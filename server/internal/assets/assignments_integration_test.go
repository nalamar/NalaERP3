package assets

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Ausgabe und Ruecknahme gegen echtes Postgres (Backlog F.3.4).
//
// Die Tests verwenden feste Daten in der Vergangenheit (2026-01-..); nur
// die Faelle "in der Zukunft" rechnen mit heute. So haengt kein Ergebnis
// vom Tagesdatum ab.

func seedEmployee(t *testing.T, env *testutil.IntegrationEnv, companyID, lastName string, active bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO hr_employees (id, first_name, last_name, company_id, active) VALUES ($1, 'Max', $2, $3, $4)`, id, lastName, companyID, active); err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return id
}

func seedAsset(t *testing.T, svc *Service, companyID, nr string) *Asset {
	t.Helper()
	a, err := svc.Create(context.Background(), AssetInput{Inventarnummer: nr, Bezeichnung: "Leiter " + nr, Kategorie: KategorieLeiter}, companyID, "")
	if err != nil {
		t.Fatalf("Asset anlegen: %v", err)
	}
	return a
}

func day(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

func wantErr(t *testing.T, what string, err error, contains string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("%s: erwartet Fehler mit %q, bekommen %v", what, contains, err)
	}
}

func TestAssignmentIssueReturnFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "ausgabe-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "hr")
	taker := testutil.SeedAuthUser(t, env, "ruecknahme-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "hr")
	anna := seedEmployee(t, env, company, "Anna", true)
	bert := seedEmployee(t, env, company, "Bert", true)
	leiter := seedAsset(t, svc, company, "L-1")
	andere := seedAsset(t, svc, company, "L-2")

	issued, err := svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna, AusgegebenAm: time.Date(2026, 1, 5, 23, 30, 0, 0, time.FixedZone("x", 3600)), Notiz: " mit Haken "}, actor, company)
	if err != nil {
		t.Fatalf("Ausgabe: %v", err)
	}
	if issued.AssetID != leiter.ID || issued.EmployeeID != anna || !issued.AusgegebenAm.Equal(day(5)) ||
		issued.ZurueckgegebenAm != nil || issued.Status != AssignmentActive || issued.Notiz != "mit Haken" ||
		issued.Inventarnummer != "L-1" || issued.Bezeichnung != "Leiter L-1" || issued.MitarbeiterName != "Max Anna" ||
		issued.AusgegebenByUserID == nil || *issued.AusgegebenByUserID != actor || issued.ZurueckgenommenByUserID != nil {
		t.Fatalf("Ausgabe nicht wie erwartet: %+v", issued)
	}

	// Zweite Ausgabe desselben Stuecks: abgelehnt, auch an andere.
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: bert, AusgegebenAm: day(6)}, actor, company)
	wantErr(t, "zweite offene Ausgabe", err, "bereits ausgegeben")
	// Ein anderes Stueck an dieselbe Person: erlaubt.
	if _, err := svc.Issue(ctx, IssueInput{AssetID: andere.ID, EmployeeID: anna, AusgegebenAm: day(6)}, actor, company); err != nil {
		t.Fatalf("anderes Stueck: %v", err)
	}

	// Rueckgabe: nicht vor der Ausgabe, nicht in der Zukunft.
	_, err = svc.Return(ctx, issued.ID, day(4), "ok", taker, company)
	wantErr(t, "Rueckgabe vor Ausgabe", err, "vor dem Ausgabedatum")
	_, err = svc.Return(ctx, issued.ID, today().AddDate(0, 0, 1), "ok", taker, company)
	wantErr(t, "Rueckgabe in der Zukunft", err, "Zukunft")

	returned, err := svc.Return(ctx, issued.ID, day(10), "  Holm verbogen ", taker, company)
	if err != nil {
		t.Fatalf("Rueckgabe: %v", err)
	}
	if returned.ZurueckgegebenAm == nil || !returned.ZurueckgegebenAm.Equal(day(10)) || returned.RueckgabeZustand != "Holm verbogen" ||
		returned.ZurueckgenommenByUserID == nil || *returned.ZurueckgenommenByUserID != taker ||
		returned.AusgegebenByUserID == nil || *returned.AusgegebenByUserID != actor || !returned.AusgegebenAm.Equal(day(5)) {
		t.Fatalf("Rueckgabe nicht wie erwartet: %+v", returned)
	}
	_, err = svc.Return(ctx, issued.ID, day(11), "nochmal", taker, company)
	wantErr(t, "zweite Rueckgabe", err, "bereits zurückgegeben")
	again, _ := svc.GetAssignment(ctx, issued.ID, company)
	if !again.ZurueckgegebenAm.Equal(day(10)) || again.RueckgabeZustand != "Holm verbogen" {
		t.Fatalf("abgelehnte zweite Rueckgabe hat geschrieben: %+v", again)
	}
	// Die Ausgabe des anderen Stuecks ist weiter offen.
	open, _ := svc.ListAssignments(ctx, company, AssignmentFilter{AssetID: &andere.ID, NurOffen: true})
	if len(open) != 1 {
		t.Fatalf("Rueckgabe hat eine andere Ausgabe getroffen: %+v", open)
	}

	// Nachgetragene Ausgabe in den Zeitraum der frueheren: abgelehnt.
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: bert, AusgegebenAm: day(9)}, actor, company)
	wantErr(t, "Ueberschneidung (Rueckgabe danach)", err, "überschneidet sich")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: bert, AusgegebenAm: day(2)}, actor, company)
	wantErr(t, "Ueberschneidung (Ausgabe danach)", err, "überschneidet sich")
	// Am Rueckgabetag an den Naechsten: erlaubt.
	next, err := svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: bert, AusgegebenAm: day(10)}, actor, company)
	if err != nil {
		t.Fatalf("Ausgabe am Rueckgabetag: %v", err)
	}
	if next.EmployeeID != bert || next.Notiz != "" || next.AusgegebenByUserID == nil {
		t.Fatalf("Folgeausgabe nicht wie erwartet: %+v", next)
	}
	// Ohne Akteur wird kein Benutzer eingetragen.
	nobody, err := svc.Issue(ctx, IssueInput{AssetID: seedAsset(t, svc, company, "L-3").ID, EmployeeID: bert, AusgegebenAm: day(10)}, "", company)
	if err != nil || nobody.AusgegebenByUserID != nil {
		t.Fatalf("Ausgabe ohne Akteur: %+v, %v", nobody, err)
	}
}

func TestAssignmentIssueRejections(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	anna := seedEmployee(t, env, company, "Anna", true)
	weg := seedEmployee(t, env, company, "Ausgeschieden", false)
	fremd := seedEmployee(t, env, other, "Fremd", true)
	leiter := seedAsset(t, svc, company, "L-1")
	fremdesAsset := seedAsset(t, svc, other, "L-1")

	_, err := svc.Issue(ctx, IssueInput{AssetID: uuid.New(), EmployeeID: anna, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "unbekanntes Asset", err, "Asset nicht gefunden")
	_, err = svc.Issue(ctx, IssueInput{AssetID: fremdesAsset.ID, EmployeeID: anna, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "Asset eines anderen Mandanten", err, "Asset nicht gefunden")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: fremd, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "Mitarbeiter eines anderen Mandanten", err, "Mitarbeiter nicht gefunden")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: weg, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "inaktiver Mitarbeiter", err, "nicht aktiv")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna, AusgegebenAm: today().AddDate(0, 0, 1)}, "", company)
	wantErr(t, "Zukunft", err, "Zukunft")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna}, "", company)
	wantErr(t, "ohne Datum", err, "Ausgabedatum erforderlich")
	_, err = svc.Issue(ctx, IssueInput{EmployeeID: anna, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "ohne Asset", err, "asset_id erforderlich")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "ohne Mitarbeiter", err, "employee_id erforderlich")
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna, AusgegebenAm: day(5)}, "", " ")
	wantErr(t, "ohne Mandant", err, "Mandant erforderlich")

	// Vor der Anschaffung: abgelehnt.
	neu, err := svc.Create(ctx, AssetInput{Inventarnummer: "N-1", Bezeichnung: "Neu", Kategorie: KategorieWerkzeug, Anschaffungsdatum: datePtr(2026, 1, 10)}, company, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Issue(ctx, IssueInput{AssetID: neu.ID, EmployeeID: anna, AusgegebenAm: day(9)}, "", company)
	wantErr(t, "vor Anschaffung", err, "vor dem Anschaffungsdatum")

	// Ausgemustert: abgelehnt.
	if _, err := svc.Retire(ctx, leiter.ID, day(3), "Holm gebrochen", company); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna, AusgegebenAm: day(5)}, "", company)
	wantErr(t, "ausgemustert", err, "ausgemustert")

	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM asset_assignments x JOIN assets a ON a.id=x.asset_id WHERE a.company_id IN ($1,$2)`, company, other).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("abgelehnte Ausgaben duerfen nichts schreiben, gefunden %d", n)
	}
}

func TestAssignmentStorno(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "storno-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "hr")
	anna := seedEmployee(t, env, company, "Anna", true)
	bert := seedEmployee(t, env, company, "Bert", true)
	leiter := seedAsset(t, svc, company, "L-1")

	wrong, err := svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna, AusgegebenAm: day(5)}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.StornoAssignment(ctx, wrong.ID, "  ", actor, company)
	wantErr(t, "ohne Begruendung", err, "Begründung")
	_, err = svc.StornoAssignment(ctx, wrong.ID, "Irrtum", actor, other)
	wantErr(t, "fremder Mandant", err, "Ausgabe nicht gefunden")
	_, err = svc.Return(ctx, wrong.ID, day(6), "", actor, other)
	wantErr(t, "Rueckgabe durch fremden Mandanten", err, "Ausgabe nicht gefunden")
	if _, err := svc.GetAssignment(ctx, wrong.ID, other); err == nil {
		t.Fatal("fremder Mandant darf die Ausgabe nicht lesen")
	}

	st, err := svc.StornoAssignment(ctx, wrong.ID, " falsche Person ", actor, company)
	if err != nil {
		t.Fatalf("Storno: %v", err)
	}
	if st.Status != AssignmentStorniert || st.StornoReason != "falsche Person" || st.StorniertAt == nil ||
		st.StorniertByUserID == nil || *st.StorniertByUserID != actor || !st.AusgegebenAm.Equal(day(5)) || st.EmployeeID != anna {
		t.Fatalf("Storno nicht wie erwartet: %+v", st)
	}
	_, err = svc.StornoAssignment(ctx, wrong.ID, "nochmal", actor, company)
	wantErr(t, "zweites Storno", err, "bereits storniert")
	if again, _ := svc.GetAssignment(ctx, wrong.ID, company); again.StornoReason != "falsche Person" || !again.StorniertAt.Equal(*st.StorniertAt) {
		t.Fatalf("abgelehntes zweites Storno hat geschrieben: %+v", again)
	}
	_, err = svc.Return(ctx, wrong.ID, day(6), "", actor, company)
	wantErr(t, "Rueckgabe einer stornierten Ausgabe", err, "storniert")

	// Die stornierte Ausgabe gibt das Stueck frei - auch fuer denselben
	// Zeitraum: sie zaehlt nicht fuer die Ueberschneidung.
	right, err := svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: bert, AusgegebenAm: day(5)}, "", company)
	if err != nil {
		t.Fatalf("Ausgabe nach Storno: %v", err)
	}
	// Auch eine zurueckgegebene Ausgabe kann storniert werden; danach
	// gilt sie nicht mehr als Ueberschneidung.
	if _, err := svc.Return(ctx, right.ID, day(20), "", "", company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StornoAssignment(ctx, right.ID, "doppelt erfasst", "", company); err != nil {
		t.Fatalf("Storno einer zurueckgegebenen Ausgabe: %v", err)
	}
	if _, err := svc.Issue(ctx, IssueInput{AssetID: leiter.ID, EmployeeID: anna, AusgegebenAm: day(7)}, "", company); err != nil {
		t.Fatalf("Ausgabe im Zeitraum einer stornierten: %v", err)
	}
}

func TestAssignmentList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	anna := seedEmployee(t, env, company, "Anna", true)
	bert := seedEmployee(t, env, company, "Bert", true)
	l1 := seedAsset(t, svc, company, "L-1")
	l2 := seedAsset(t, svc, company, "L-2")
	seedAsset(t, svc, other, "X-1")

	a1, _ := svc.Issue(ctx, IssueInput{AssetID: l1.ID, EmployeeID: anna, AusgegebenAm: day(1)}, "", company)
	if _, err := svc.Return(ctx, a1.ID, day(3), "", "", company); err != nil {
		t.Fatal(err)
	}
	a2, _ := svc.Issue(ctx, IssueInput{AssetID: l1.ID, EmployeeID: bert, AusgegebenAm: day(4)}, "", company)
	a3, _ := svc.Issue(ctx, IssueInput{AssetID: l2.ID, EmployeeID: anna, AusgegebenAm: day(2)}, "", company)
	if _, err := svc.StornoAssignment(ctx, a3.ID, "Irrtum", "", company); err != nil {
		t.Fatal(err)
	}
	a4, _ := svc.Issue(ctx, IssueInput{AssetID: l2.ID, EmployeeID: anna, AusgegebenAm: day(2)}, "", company)

	ids := func(list []Assignment) string {
		names := map[uuid.UUID]string{a1.ID: "a1", a2.ID: "a2", a3.ID: "a3", a4.ID: "a4"}
		out := make([]string, 0, len(list))
		for _, x := range list {
			out = append(out, names[x.ID])
		}
		return strings.Join(out, ",")
	}
	check := func(name string, f AssignmentFilter, want string) {
		t.Helper()
		list, err := svc.ListAssignments(ctx, company, f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := ids(list); got != want {
			t.Fatalf("%s: erwartet %s, bekommen %s", name, want, got)
		}
	}
	check("alle, neueste zuerst", AssignmentFilter{}, "a2,a4,a1")
	check("mit Stornierten", AssignmentFilter{IncludeStorniert: true}, "a2,a4,a3,a1")
	check("nur offene", AssignmentFilter{NurOffen: true}, "a2,a4")
	check("je Asset", AssignmentFilter{AssetID: &l1.ID}, "a2,a1")
	check("je Mitarbeiter", AssignmentFilter{EmployeeID: &anna}, "a4,a1")
	check("je Mitarbeiter offen", AssignmentFilter{EmployeeID: &anna, NurOffen: true}, "a4")

	if list, err := svc.ListAssignments(ctx, other, AssignmentFilter{IncludeStorniert: true}); err != nil || len(list) != 0 {
		t.Fatalf("fremder Mandant: %v, %v", list, err)
	}
}

// Ausmustern und Ausgeben nehmen dieselbe Sperre auf die Asset-Zeile. Laufen
// beide gleichzeitig, darf nie ein ausgemustertes Stueck mit offener
// Ausgabe entstehen - genau eines von beiden gelingt.
func TestAssignmentIssueAndRetireConcurrently(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	anna := seedEmployee(t, env, company, "Anna", true)

	for i := 0; i < 10; i++ {
		a := seedAsset(t, svc, company, "P-"+uuid.NewString()[:8])
		var wg sync.WaitGroup
		var issueErr, retireErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, issueErr = svc.Issue(ctx, IssueInput{AssetID: a.ID, EmployeeID: anna, AusgegebenAm: day(5)}, "", company)
		}()
		go func() {
			defer wg.Done()
			_, retireErr = svc.Retire(ctx, a.ID, day(5), "weg", company)
		}()
		wg.Wait()
		if (issueErr == nil) == (retireErr == nil) {
			t.Fatalf("Durchlauf %d: genau eines muss gelingen, Ausgabe: %v, Ausmusterung: %v", i, issueErr, retireErr)
		}
		var bad bool
		if err := env.PG.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assets a JOIN asset_assignments x ON x.asset_id=a.id
			WHERE a.id=$1 AND a.ausgemustert_am IS NOT NULL AND x.zurueckgegeben_am IS NULL AND x.status='active')`, a.ID).Scan(&bad); err != nil {
			t.Fatal(err)
		}
		if bad {
			t.Fatalf("Durchlauf %d: ausgemustertes Stueck mit offener Ausgabe", i)
		}
	}
}

// Zwei gleichzeitige Ausgaben desselben Stuecks: genau eine gelingt, die
// andere bekommt die fachliche Meldung.
func TestAssignmentConcurrentIssue(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	anna := seedEmployee(t, env, company, "Anna", true)
	bert := seedEmployee(t, env, company, "Bert", true)
	a := seedAsset(t, svc, company, "C-1")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, emp := range []uuid.UUID{anna, bert} {
		wg.Add(1)
		go func(i int, emp uuid.UUID) {
			defer wg.Done()
			_, errs[i] = svc.Issue(ctx, IssueInput{AssetID: a.ID, EmployeeID: emp, AusgegebenAm: day(5)}, "", company)
		}(i, emp)
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		} else if e.Error() != "Asset ist bereits ausgegeben" {
			t.Fatalf("roher Fehler statt fachlicher Meldung: %v", e)
		}
	}
	if ok != 1 {
		t.Fatalf("erwartet genau eine Ausgabe, bekommen %d", ok)
	}
}
