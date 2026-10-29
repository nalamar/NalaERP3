package noticemail

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/mail"
	"nalaerp3/internal/notices"
	"nalaerp3/internal/testutil"
)

// Versand der Sammelmails gegen echtes Postgres (Backlog F.14.4a).
// Hinweise, Zugang, Rechte und Versand sind Attrappen; Benutzer,
// Abbestellungen und Protokoll liegen in der Datenbank. Erfundene
// Adressen unter example.test.

type fakeNotices struct {
	byUser map[string][]notices.Notice
	seen   []notices.Recipient
	mu     sync.Mutex
}

func (f *fakeNotices) For(_ context.Context, r notices.Recipient, _ time.Time) ([]notices.Notice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen = append(f.seen, r)
	return f.byUser[r.UserID], nil
}

type fakeSettings struct {
	cfg map[string]*mail.SMTPConfig
}

func (f fakeSettings) Config(_ context.Context, companyID string) (*mail.SMTPConfig, error) {
	if c, ok := f.cfg[companyID]; ok {
		return c, nil
	}
	return nil, errors.New("E-Mail-Versand ist nicht vollständig eingerichtet")
}

type fakeTransport struct {
	mu   sync.Mutex
	sent []mail.Message
	fail map[string]error // je Empfaenger
}

func (f *fakeTransport) Send(_ context.Context, _ mail.SMTPConfig, msg mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail[msg.To[0]]; err != nil {
		return err
	}
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeTransport) to() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	for i, m := range f.sent {
		out[i] = m.To[0]
	}
	return strings.Join(out, ",")
}

type fakePerms map[string][]string

func (f fakePerms) ListPermissionCodesByUserID(_ context.Context, userID string) ([]string, error) {
	return f[userID], nil
}

type fakeEmployees map[string]uuid.UUID

func (f fakeEmployees) EmployeeIDForUser(_ context.Context, userID, _ string) (uuid.UUID, bool, error) {
	id, ok := f[userID]
	return id, ok, nil
}

func seedCompany(t *testing.T, env *testutil.IntegrationEnv, aktiv bool) string {
	t.Helper()
	ctx := context.Background()
	id := "nm-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Hinweismail "+id); err != nil {
		t.Fatal(err)
	}
	sealed := append([]byte{1}, make([]byte, 28)...)
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_mail_settings (company_id, aktiv, smtp_host, smtp_port, sicherheit, benutzer, password_enc, absender_adresse)
		VALUES ($1,$2,'smtp.example.test',465,'ssl','erp@example.test',$3,'erp@example.test')`, id, aktiv, sealed); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedUser(t *testing.T, env *testutil.IntegrationEnv, company, name string, active, locked bool) (id, email string) {
	t.Helper()
	id = "nm-" + name + "-" + uuid.NewString()[:8]
	email = id + "@example.test"
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO users (id, email, username, password_hash, is_active, is_locked, company_id)
		VALUES ($1,$2,$1,'x',$3,$4,$5)`, id, email, active, locked, company); err != nil {
		t.Fatal(err)
	}
	return id, email
}

type logRow struct {
	ergebnis, stufe string
	finished        bool
}

func logOf(t *testing.T, env *testutil.IntegrationEnv, company string) map[string]logRow {
	t.Helper()
	rows, err := env.PG.Query(context.Background(), `SELECT user_id, ergebnis, fehler_stufe, finished_at IS NOT NULL FROM notice_mail_log WHERE company_id=$1`, company)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]logRow{}
	for rows.Next() {
		var u string
		var r logRow
		if err := rows.Scan(&u, &r.ergebnis, &r.stufe, &r.finished); err != nil {
			t.Fatal(err)
		}
		out[u] = r
	}
	return out
}

var dueNotice = []notices.Notice{{Kind: notices.KindFleetSchedulesDue, Title: "Fällige Fahrzeugtermine", Severity: notices.SeverityKritisch, Total: 1, Counts: map[string]int{"ueberfaellig": 1}}}

func TestRunDaySendsOncePerUserAndLogsWithoutContent(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env, true)
	off := seedCompany(t, env, false)
	notReady := seedCompany(t, env, true) // aktiv, aber Config scheitert
	switched := seedCompany(t, env, true) // aktiv in der Tabelle, Config meldet inzwischen aus

	anna, annaMail := seedUser(t, env, company, "anna", true, false)
	bert, bertMail := seedUser(t, env, company, "bert", true, false)
	nichts, _ := seedUser(t, env, company, "nichts", true, false)
	weg, _ := seedUser(t, env, company, "weg", true, false)
	inaktiv, _ := seedUser(t, env, company, "inaktiv", false, false)
	gesperrt, _ := seedUser(t, env, company, "gesperrt", true, true)
	kaputt, kaputtMail := seedUser(t, env, company, "kaputt", true, false)
	offUser, _ := seedUser(t, env, off, "off", true, false)
	notReadyUser, _ := seedUser(t, env, notReady, "nr", true, false)
	switchedUser, _ := seedUser(t, env, switched, "sw", true, false)
	if _, err := env.PG.Exec(ctx, `INSERT INTO notice_mail_optouts (user_id) VALUES ($1)`, weg); err != nil {
		t.Fatal(err)
	}

	emp := uuid.New()
	nf := &fakeNotices{byUser: map[string][]notices.Notice{}}
	for _, u := range []string{anna, bert, weg, inaktiv, gesperrt, kaputt, offUser, notReadyUser, switchedUser} {
		nf.byUser[u] = dueNotice
	}
	tr := &fakeTransport{fail: map[string]error{kaputtMail: errors.New("Empfängeradresse ungültig")}}
	cfg := &mail.SMTPConfig{Aktiv: true, Host: "smtp.example.test"}
	r := &Runner{
		PG: env.PG, Notices: nf, Transport: tr,
		Settings:    fakeSettings{cfg: map[string]*mail.SMTPConfig{company: cfg, off: cfg, switched: {Aktiv: false}}},
		Permissions: fakePerms{anna: {"fleet.read"}, bert: {"admin.superuser"}},
		Employees:   fakeEmployees{anna: emp},
	}
	day := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)

	res, err := r.RunDay(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	// Mehrere Mandanten in der Test-DB: nur die eigenen zaehlen.
	sent := map[string]bool{}
	for _, m := range tr.sent {
		sent[m.To[0]] = true
	}
	if !sent[annaMail] || !sent[bertMail] || len(tr.sent) != 2 || res.Gesendet < 2 || res.Fehler < 1 {
		t.Fatalf("versandt an %s, Ergebnis %+v", tr.to(), res)
	}
	m := tr.sent[0]
	if len(m.To) != 1 || m.Subject != "Nala ERP: 1 fällige Hinweise" || !strings.Contains(m.Text, "- Fällige Fahrzeugtermine: 1 (1 überfällig) [dringend]") {
		t.Fatalf("Nachricht: %+v", m)
	}

	// Empfaenger mit Rechten und Mitarbeiter.
	byUser := map[string]notices.Recipient{}
	for _, rc := range nf.seen {
		byUser[rc.UserID] = rc
	}
	if a := byUser[anna]; a.CompanyID != company || a.EmployeeID == nil || *a.EmployeeID != emp || !a.Can("fleet.read") || a.Can("assets.read") {
		t.Fatalf("Empfaenger Anna: %+v", a)
	}
	if b := byUser[bert]; b.EmployeeID != nil || !b.Can("assets.read") {
		t.Fatalf("Superuser: %+v", b)
	}
	for _, u := range []string{weg, inaktiv, gesperrt, offUser, notReadyUser, switchedUser} {
		if _, asked := byUser[u]; asked {
			t.Errorf("%s haette nicht beruecksichtigt werden duerfen", u)
		}
	}

	// Protokoll: ohne Inhalt, mit Stufe bei Fehler; nichts Faelliges ohne Eintrag.
	lg := logOf(t, env, company)
	if len(lg) != 3 || lg[anna] != (logRow{ErgebnisGesendet, "", true}) || lg[bert] != (logRow{ErgebnisGesendet, "", true}) ||
		lg[kaputt] != (logRow{ErgebnisFehler, StufeEingabe, true}) {
		t.Fatalf("Protokoll: %+v", lg)
	}
	if _, ok := lg[nichts]; ok {
		t.Fatal("ohne Faelliges ein Protokolleintrag")
	}
	if n := len(logOf(t, env, off)) + len(logOf(t, env, notReady)) + len(logOf(t, env, switched)); n != 0 {
		t.Fatalf("Protokoll anderer Mandanten: %d", n)
	}

	// Zweiter Lauf am selben Tag: nichts mehr, auch kein neuer Versuch nach Fehler.
	tr.fail = nil
	if _, err := r.RunDay(ctx, day); err != nil {
		t.Fatal(err)
	}
	if len(tr.sent) != 2 {
		t.Fatalf("zweiter Lauf hat versandt: %s", tr.to())
	}
	// Schon protokollierte Benutzer werden nicht erneut ausgewertet.
	asked := 0
	for _, rc := range nf.seen {
		if rc.UserID == anna {
			asked++
		}
	}
	if asked != 1 {
		t.Fatalf("Anna am selben Tag %d-mal ausgewertet", asked)
	}
	// Naechster Tag: wieder.
	if _, err := r.RunDay(ctx, day.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tr.to(), kaputtMail) || len(tr.sent) != 5 {
		t.Fatalf("naechster Tag: %s", tr.to())
	}
}

// Ein Versandfehler des Mail-Servers steht mit seiner Stufe im Protokoll.
func TestRunDayLogsSendErrorStage(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env, true)
	u, email := seedUser(t, env, company, "stufe", true, false)
	// Ein echter mail.SendError: Anmeldung an einem nicht erreichbaren
	// Server scheitert an der Verbindung.
	var sendErr error
	{
		s := &mail.Sender{Timeout: 200 * time.Millisecond}
		sendErr = s.Send(ctx, mail.SMTPConfig{Aktiv: true, Host: "127.0.0.1", Port: 1, Sicherheit: "ssl", Benutzer: "x", Passwort: "x",
			AbsenderAdresse: "erp@example.test"}, mail.Message{To: []string{email}, Subject: "x", Text: "x"})
		var se *mail.SendError
		if !errors.As(sendErr, &se) || se.Stage != mail.StageVerbindung {
			t.Fatalf("Vorbedingung: %v", sendErr)
		}
	}
	r := &Runner{
		PG: env.PG, Notices: &fakeNotices{byUser: map[string][]notices.Notice{u: dueNotice}},
		Transport:   &fakeTransport{fail: map[string]error{email: sendErr}},
		Settings:    fakeSettings{cfg: map[string]*mail.SMTPConfig{company: {Aktiv: true}}},
		Permissions: fakePerms{}, Employees: fakeEmployees{},
	}
	if _, err := r.RunDay(ctx, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if lg := logOf(t, env, company); lg[u] != (logRow{ErgebnisFehler, mail.StageVerbindung, true}) {
		t.Fatalf("Protokoll: %+v", lg)
	}
}

// Zwei gleichzeitige Laeufe verschicken jede Mail genau einmal.
func TestRunDayConcurrentRunsSendOnce(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env, true)
	nf := &fakeNotices{byUser: map[string][]notices.Notice{}}
	for i := 0; i < 5; i++ {
		u, _ := seedUser(t, env, company, "par", true, false)
		nf.byUser[u] = dueNotice
	}
	tr := &fakeTransport{}
	r := &Runner{PG: env.PG, Notices: nf, Transport: tr,
		Settings: fakeSettings{cfg: map[string]*mail.SMTPConfig{company: {Aktiv: true}}}, Permissions: fakePerms{}, Employees: fakeEmployees{}}
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.RunDay(ctx, day); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(tr.sent) != 5 {
		t.Fatalf("versandt: %d", len(tr.sent))
	}
	if lg := logOf(t, env, company); len(lg) != 5 {
		t.Fatalf("Protokoll: %d", len(lg))
	}
}

// CHECKs der Migration 106 gegen Direkt-SQL.
func TestNoticeMailLogConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env, false)
	u, _ := seedUser(t, env, company, "chk", true, false)
	ins := func(user string, tag, ergebnis, stufe string) error {
		_, err := env.PG.Exec(ctx, `INSERT INTO notice_mail_log (id, company_id, user_id, versand_tag, ergebnis, fehler_stufe) VALUES ($1,$2,$3,$4,$5,$6)`,
			uuid.New(), company, user, tag, ergebnis, stufe)
		return err
	}
	// Geordnet: "zweimal am Tag" braucht den ersten Eintrag.
	for _, tc := range []struct {
		name, tag, ergebnis, stufe string
		ok                         bool
	}{
		{"versand", "2026-01-01", "versand", "", true},
		{"gesendet", "2026-01-02", "gesendet", "", true},
		{"fehler mit Stufe", "2026-01-03", "fehler", "anmeldung", true},
		{"fehler ohne Stufe", "2026-01-04", "fehler", "", false},
		{"Stufe ohne Fehler", "2026-01-05", "gesendet", "tls", false},
		{"Ergebnis", "2026-01-06", "egal", "", false},
		{"zweimal am Tag", "2026-01-01", "gesendet", "", false},
	} {
		if err := ins(u, tc.tag, tc.ergebnis, tc.stufe); (err == nil) != tc.ok {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	// Benutzer entfernt: Protokoll bleibt, Abbestellung geht mit.
	if _, err := env.PG.Exec(ctx, `INSERT INTO notice_mail_optouts (user_id) VALUES ($1)`, u); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO notice_mail_optouts (user_id) VALUES ($1)`, u); err == nil {
		t.Fatal("doppelte Abbestellung angenommen")
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM users WHERE id=$1`, u); err != nil {
		t.Fatal(err)
	}
	var logs, optouts int
	_ = env.PG.QueryRow(ctx, `SELECT count(*) FROM notice_mail_log WHERE company_id=$1 AND user_id IS NULL`, company).Scan(&logs)
	_ = env.PG.QueryRow(ctx, `SELECT count(*) FROM notice_mail_optouts WHERE user_id=$1`, u).Scan(&optouts)
	if logs != 3 || optouts != 0 {
		t.Fatalf("nach Entfernen: Protokoll %d, Abbestellungen %d", logs, optouts)
	}
}
