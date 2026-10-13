package hr

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Diese Tests laufen gegen echtes Postgres: Mandantenpruefung,
// Ueberschneidung und "hoechstens ein laufender Eintrag" sind Abfragen
// bzw. Sperren und ohne Datenbank nicht pruefbar.
//
// Alle Zeiten liegen fest in der Vergangenheit (2026-03), damit kein Fall
// versehentlich an "in der Zukunft" scheitert. Jeder Test legt einen
// eigenen Mitarbeiter an; die Tests teilen sich die Datenbank, aber keine
// Zeiteintraege.

func seedTimeEmployee(t *testing.T, env *testutil.IntegrationEnv, lastName string) Employee {
	t.Helper()
	emp, err := NewEmployeeService(env.PG).Create(context.Background(), Employee{FirstName: "Zeit", LastName: lastName, Active: true}, "default")
	if err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return *emp
}

func seedOtherCompany(t *testing.T, env *testutil.IntegrationEnv) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ('other-company','Andere Firma GmbH') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("zweiten Mandanten anlegen: %v", err)
	}
}

func wantTimeEntryError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("erwartet Fehler %q, bekommen nil", want)
	}
	if err.Error() != want {
		t.Fatalf("erwartet %q, bekommen %q", want, err.Error())
	}
}

func TestTimeEntryCreatePersistsClosedEntry(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Nachtrag")
	userID := testutil.SeedAuthUser(t, env, "zeit-nachtrag@example.invalid", "geheim-123456", "hr")

	start, end := berlin(t, "2026-03-02 07:00"), berlin(t, "2026-03-02 16:00")
	created, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: start, EndedAt: &end, BreakMinutes: 45, Note: "Werkstatt"}, userID, "default")
	if err != nil {
		t.Fatalf("Nachtrag anlegen: %v", err)
	}

	// Erneut laden: erst das beweist die Persistenz und nicht nur eine
	// stimmige Rueckgabe.
	got, err := svc.Get(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("Eintrag laden: %v", err)
	}
	if got.EmployeeID != emp.ID || !got.StartedAt.Equal(start) || got.EndedAt == nil || !got.EndedAt.Equal(end) {
		t.Fatalf("Zeiten nicht wie erfasst: %+v", got)
	}
	if got.WorkDate.Format("2006-01-02") != "2026-03-02" {
		t.Fatalf("erwartet Arbeitstag 2026-03-02, bekommen %s", got.WorkDate.Format("2006-01-02"))
	}
	if got.BreakMinutes != 45 || got.Note != "Werkstatt" || got.Status != TimeEntryStatusActive {
		t.Fatalf("Pause/Notiz/Status nicht wie erfasst: %+v", got)
	}
	if got.CreatedByUserID == nil || *got.CreatedByUserID != userID {
		t.Fatalf("erwartet created_by_user_id=%s, bekommen %v", userID, got.CreatedByUserID)
	}
	if got.StorniertAt != nil || got.ReplacesEntryID != nil || got.ProjectID != nil {
		t.Fatalf("erwartet keine Storno-/Korrektur-/Projektangaben, bekommen %+v", got)
	}
}

// Kernentscheidung der ADR, hier gegen den echten Schreibpfad samt
// Datenbank-Constraints: ein Verstoss gegen das ArbZG wird gespeichert.
func TestTimeEntryCreateAcceptsArbZGViolation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Elfstunden")

	start, end := berlin(t, "2026-03-02 06:00"), berlin(t, "2026-03-02 17:00")
	created, err := svc.Create(context.Background(), TimeEntryInput{EmployeeID: emp.ID, StartedAt: start, EndedAt: &end}, "", "default")
	if err != nil {
		t.Fatalf("11 Stunden ohne Pause muessen angenommen werden, bekommen %v", err)
	}
	if created.CreatedByUserID != nil {
		t.Fatalf("leerer Akteur muss NULL ergeben, bekommen %v", *created.CreatedByUserID)
	}
}

func TestTimeEntryNightShiftBelongsToStartDay(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Nachtschicht")

	start, end := berlin(t, "2026-03-02 22:00"), berlin(t, "2026-03-03 06:00")
	created, err := svc.Create(context.Background(), TimeEntryInput{EmployeeID: emp.ID, StartedAt: start, EndedAt: &end, BreakMinutes: 30}, "", "default")
	if err != nil {
		t.Fatalf("Nachtschicht anlegen: %v", err)
	}
	if created.WorkDate.Format("2006-01-02") != "2026-03-02" {
		t.Fatalf("erwartet Arbeitstag 2026-03-02, bekommen %s", created.WorkDate.Format("2006-01-02"))
	}
}

func TestTimeEntryStartAndStopFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Stempeln")

	start := berlin(t, "2026-03-02 07:00")
	running, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: start}, "", "default")
	if err != nil {
		t.Fatalf("Stempeln: %v", err)
	}
	if running.EndedAt != nil {
		t.Fatalf("erwartet laufenden Eintrag, bekommen Ende %v", running.EndedAt)
	}

	// Ein ungueltiges Beenden aendert nichts: der Eintrag laeuft weiter.
	_, err = svc.Stop(ctx, running.ID, berlin(t, "2026-03-02 06:00"), 0, "default")
	wantTimeEntryError(t, err, "Ende muss nach dem Beginn liegen")
	_, err = svc.Stop(ctx, running.ID, berlin(t, "2026-03-02 08:00"), 60, "default")
	wantTimeEntryError(t, err, "Pause muss kürzer sein als die Anwesenheit")
	still, err := svc.Get(ctx, running.ID, "default")
	if err != nil {
		t.Fatalf("Eintrag laden: %v", err)
	}
	if still.EndedAt != nil || still.BreakMinutes != 0 {
		t.Fatalf("abgelehntes Beenden darf nichts schreiben, bekommen %+v", still)
	}

	end := berlin(t, "2026-03-02 15:30")
	stopped, err := svc.Stop(ctx, running.ID, end, 30, "default")
	if err != nil {
		t.Fatalf("Beenden: %v", err)
	}
	if stopped.EndedAt == nil || !stopped.EndedAt.Equal(end) || stopped.BreakMinutes != 30 {
		t.Fatalf("Ende/Pause nicht wie angegeben: %+v", stopped)
	}
	if !stopped.StartedAt.Equal(start) {
		t.Fatalf("Beginn darf sich beim Beenden nicht aendern: %v", stopped.StartedAt)
	}

	// Unveraenderbarkeit: ein beendeter Eintrag laesst sich nicht erneut
	// beenden, und die gespeicherten Werte bleiben stehen.
	_, err = svc.Stop(ctx, running.ID, berlin(t, "2026-03-02 18:00"), 0, "default")
	wantTimeEntryError(t, err, "Zeiteintrag ist bereits beendet")
	final, err := svc.Get(ctx, running.ID, "default")
	if err != nil {
		t.Fatalf("Eintrag laden: %v", err)
	}
	if !final.EndedAt.Equal(end) || final.BreakMinutes != 30 {
		t.Fatalf("zweites Beenden hat den Eintrag veraendert: %+v", final)
	}

	// Nach dem Beenden darf wieder gestempelt werden.
	if _, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-03 07:00")}, "", "default"); err != nil {
		t.Fatalf("erneutes Stempeln nach dem Beenden: %v", err)
	}
}

func TestTimeEntryStopRejectsUnknownAndStorniertEntry(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Stornostop")

	_, err := svc.Stop(ctx, uuid.New(), berlin(t, "2026-03-02 16:00"), 0, "default")
	wantTimeEntryError(t, err, "Zeiteintrag nicht gefunden")

	running, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00")}, "", "default")
	if err != nil {
		t.Fatalf("Stempeln: %v", err)
	}
	// Der Storno-Pfad des Service folgt in F.1.4.2; hier direkt gesetzt.
	if _, err := env.PG.Exec(ctx, `UPDATE hr_time_entries SET status='storniert', storniert_at=now(), storno_reason='Fehlbuchung' WHERE id=$1`, running.ID); err != nil {
		t.Fatalf("Eintrag stornieren: %v", err)
	}
	_, err = svc.Stop(ctx, running.ID, berlin(t, "2026-03-02 16:00"), 0, "default")
	wantTimeEntryError(t, err, "Zeiteintrag ist storniert")
}

func TestTimeEntryCreateRejectsOverlap(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Ueberschneidung")
	other := seedTimeEmployee(t, env, "Kollege")

	closed := func(start, end string) TimeEntryInput {
		e := berlin(t, end)
		return TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, start), EndedAt: &e}
	}
	const overlap = "Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag"

	base, err := svc.Create(ctx, closed("2026-03-02 08:00", "2026-03-02 12:00"), "", "default")
	if err != nil {
		t.Fatalf("Ausgangseintrag: %v", err)
	}

	for name, in := range map[string]TimeEntryInput{
		"ragt hinten hinein":   closed("2026-03-02 07:00", "2026-03-02 09:00"),
		"ragt vorne hinein":    closed("2026-03-02 11:00", "2026-03-02 13:00"),
		"liegt innerhalb":      closed("2026-03-02 09:00", "2026-03-02 10:00"),
		"umschliesst":          closed("2026-03-02 07:00", "2026-03-02 13:00"),
		"deckungsgleich":       closed("2026-03-02 08:00", "2026-03-02 12:00"),
		"Stempeln mittendrin":  {EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 10:00")},
		"Stempeln davor":       {EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00")},
		"um eine Minute davor": closed("2026-03-02 07:00", "2026-03-02 08:01"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Create(ctx, in, "", "default")
			wantTimeEntryError(t, err, overlap)
		})
	}

	// Halboffene Zeitraeume: direkt anschliessend ist keine Ueberschneidung.
	if _, err := svc.Create(ctx, closed("2026-03-02 06:00", "2026-03-02 08:00"), "", "default"); err != nil {
		t.Fatalf("direkt davor anschliessend muss erlaubt sein, bekommen %v", err)
	}
	if _, err := svc.Create(ctx, closed("2026-03-02 12:00", "2026-03-02 13:00"), "", "default"); err != nil {
		t.Fatalf("direkt danach anschliessend muss erlaubt sein, bekommen %v", err)
	}

	// Die Pruefung gilt je Mitarbeiter: ein Kollege darf zur selben Zeit arbeiten.
	end := berlin(t, "2026-03-02 12:00")
	if _, err := svc.Create(ctx, TimeEntryInput{EmployeeID: other.ID, StartedAt: berlin(t, "2026-03-02 08:00"), EndedAt: &end}, "", "default"); err != nil {
		t.Fatalf("anderer Mitarbeiter zur selben Zeit muss erlaubt sein, bekommen %v", err)
	}

	// Ein stornierter Eintrag blockiert seinen Zeitraum nicht mehr.
	if _, err := env.PG.Exec(ctx, `UPDATE hr_time_entries SET status='storniert', storniert_at=now(), storno_reason='Fehlbuchung' WHERE id=$1`, base.ID); err != nil {
		t.Fatalf("Eintrag stornieren: %v", err)
	}
	if _, err := svc.Create(ctx, closed("2026-03-02 08:00", "2026-03-02 12:00"), "", "default"); err != nil {
		t.Fatalf("Zeitraum eines stornierten Eintrags muss wieder frei sein, bekommen %v", err)
	}
}

func TestTimeEntryRunningEntryBlocksLaterEntries(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Laufend")

	if _, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00")}, "", "default"); err != nil {
		t.Fatalf("Stempeln: %v", err)
	}

	_, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 09:00")}, "", "default")
	wantTimeEntryError(t, err, "Es läuft bereits ein Zeiteintrag")
	// Auch ein frueher beginnender zweiter laufender Eintrag ist einer zu viel.
	_, err = svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-01 09:00")}, "", "default")
	wantTimeEntryError(t, err, "Es läuft bereits ein Zeiteintrag")

	// Ein laufender Eintrag reicht bis unendlich: ein Nachtrag danach
	// ueberschneidet sich, auch Tage spaeter.
	lateEnd := berlin(t, "2026-03-05 12:00")
	_, err = svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-05 08:00"), EndedAt: &lateEnd}, "", "default")
	wantTimeEntryError(t, err, "Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag")

	// Ein Nachtrag, der vollstaendig VOR dem laufenden Eintrag liegt, ist
	// dagegen zulaessig - der vergessene Vortag muss nachtragbar bleiben.
	earlyEnd := berlin(t, "2026-03-01 15:00")
	if _, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-01 07:00"), EndedAt: &earlyEnd}, "", "default"); err != nil {
		t.Fatalf("Nachtrag vor dem laufenden Eintrag muss erlaubt sein, bekommen %v", err)
	}
}

// Zwei gleichzeitige "Kommen"-Buchungen: genau eine darf durchgehen. Ohne
// die Zeilensperre auf dem Mitarbeiter koennten beide die Vorabpruefung
// passieren; dann finge zwar der UNIQUE-Index die zweite ab, aber als
// roher Datenbankfehler statt als fachliche Meldung.
func TestTimeEntryConcurrentStartAllowsExactlyOne(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Gleichzeitig")

	const n = 8
	base := berlin(t, "2026-03-02 07:00")
	errs := make([]error, n)
	var wg sync.WaitGroup
	startGate := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-startGate
			_, errs[i] = svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: base.Add(time.Duration(i) * time.Minute)}, "", "default")
		}(i)
	}
	close(startGate)
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		if err.Error() != "Es läuft bereits ein Zeiteintrag" {
			t.Fatalf("erwartet fachliche Meldung, bekommen %q", err.Error())
		}
	}
	if ok != 1 {
		t.Fatalf("erwartet genau eine erfolgreiche Buchung, bekommen %d", ok)
	}
	var count int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM hr_time_entries WHERE employee_id=$1`, emp.ID).Scan(&count); err != nil {
		t.Fatalf("Eintraege zaehlen: %v", err)
	}
	if count != 1 {
		t.Fatalf("erwartet genau einen Eintrag in der Datenbank, bekommen %d", count)
	}
}

func TestTimeEntryIsScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	seedOtherCompany(t, env)
	emp := seedTimeEmployee(t, env, "Mandant")

	// Fuer einen Mitarbeiter eines anderen Mandanten laesst sich nichts buchen.
	_, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00")}, "", "other-company")
	wantTimeEntryError(t, err, "Mitarbeiter nicht gefunden")

	running, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00")}, "", "default")
	if err != nil {
		t.Fatalf("Stempeln: %v", err)
	}

	// Der andere Mandant sieht den Eintrag nicht und kann ihn nicht beenden.
	_, err = svc.Get(ctx, running.ID, "other-company")
	wantTimeEntryError(t, err, "Zeiteintrag nicht gefunden")
	_, err = svc.Stop(ctx, running.ID, berlin(t, "2026-03-02 16:00"), 30, "other-company")
	wantTimeEntryError(t, err, "Zeiteintrag nicht gefunden")

	still, err := svc.Get(ctx, running.ID, "default")
	if err != nil {
		t.Fatalf("Eintrag laden: %v", err)
	}
	if still.EndedAt != nil {
		t.Fatalf("fremder Mandant hat den Eintrag beendet: %+v", still)
	}
}

func TestTimeEntryProjectMustBelongToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	seedOtherCompany(t, env)
	emp := seedTimeEmployee(t, env, "Projekt")

	own, foreign := uuid.New(), uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, status, company_id) VALUES ($1,$2,'Zeiterfassung eigen','neu','default'), ($3,$4,'Zeiterfassung fremd','neu','other-company')`,
		own, "PRJ-ZEIT-"+own.String()[:8], foreign, "PRJ-ZEIT-"+foreign.String()[:8]); err != nil {
		t.Fatalf("Projekte anlegen: %v", err)
	}

	end := berlin(t, "2026-03-02 12:00")
	in := TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 08:00"), EndedAt: &end}

	in.ProjectID = &foreign
	_, err := svc.Create(ctx, in, "", "default")
	wantTimeEntryError(t, err, "Projekt nicht gefunden")

	unknown := uuid.New()
	in.ProjectID = &unknown
	_, err = svc.Create(ctx, in, "", "default")
	wantTimeEntryError(t, err, "Projekt nicht gefunden")

	in.ProjectID = &own
	created, err := svc.Create(ctx, in, "", "default")
	if err != nil {
		t.Fatalf("Eintrag mit eigenem Projekt: %v", err)
	}
	if created.ProjectID == nil || *created.ProjectID != own {
		t.Fatalf("erwartet project_id=%s, bekommen %v", own, created.ProjectID)
	}
}
