package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Haertung des EmployeeService vor der HTTP-Anbindung (ADR 0025, Backlog
// F.4.3). Jeder Test hier gehoert zu einem Befund der ADR: was ohne
// HTTP-Grenze nicht auffiel, waere ueber HTTP ein sichtbarer Fehler.

func wantEmployeeError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("erwartet Fehler %q, bekommen nil", want)
	}
	if err.Error() != want {
		t.Fatalf("erwartet %q, bekommen %q", want, err.Error())
	}
}

func seedTeam(t *testing.T, env *testutil.IntegrationEnv, name, companyID string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO hr_teams (id, name, company_id) VALUES ($1,$2,$3)`, id, name, companyID); err != nil {
		t.Fatalf("Team anlegen: %v", err)
	}
	return id
}

// seedUserInCompany legt einen Login an und ordnet ihn dem Mandanten zu.
func seedUserInCompany(t *testing.T, env *testutil.IntegrationEnv, email, companyID string) string {
	t.Helper()
	ctx := context.Background()
	userID := testutil.SeedAuthUser(t, env, email, "geheim-123456", "production")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET company_id=$2 WHERE id=$1`, userID, companyID); err != nil {
		t.Fatalf("Benutzer dem Mandanten zuordnen: %v", err)
	}
	// Wiederholte Laeufe gegen dieselbe DB: alte Verknuepfung loesen.
	if _, err := env.PG.Exec(ctx, `UPDATE hr_employees SET user_id=NULL WHERE user_id=$1`, userID); err != nil {
		t.Fatalf("alte Verknuepfung loesen: %v", err)
	}
	return userID
}

// Entscheidung des Nutzers vom 2026-10-13: neue Mitarbeiter sind aktiv,
// solange nicht ausdruecklich etwas anderes angegeben ist.
func TestEmployeeCreateDefaultsToActive(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)
	no, yes := false, true

	cases := []struct {
		name   string
		active *bool
		want   bool
	}{
		{"nicht angegeben", nil, true},
		{"ausdruecklich false", &no, false},
		{"ausdruecklich true", &yes, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.Create(ctx, EmployeeInput{FirstName: "Aktiv", LastName: tc.name, Active: tc.active}, "default")
			if err != nil {
				t.Fatalf("Anlegen: %v", err)
			}
			if created.Active != tc.want {
				t.Fatalf("Rueckgabe: erwartet active=%v, bekommen %v", tc.want, created.Active)
			}
			// Erneut laden: die Rueckgabe allein beweist nicht, was gespeichert wurde.
			got, err := svc.Get(ctx, created.ID, "default")
			if err != nil {
				t.Fatalf("Laden: %v", err)
			}
			if got.Active != tc.want {
				t.Fatalf("gespeichert: erwartet active=%v, bekommen %v", tc.want, got.Active)
			}
		})
	}
}

func TestEmployeeGetAndUpdateReportNotFound(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)
	seedOtherCompany(t, env)

	emp, err := svc.Create(ctx, EmployeeInput{FirstName: "Nicht", LastName: "Gefunden"}, "default")
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}

	// Vorher: roher pgx.ErrNoRows ("no rows in result set") -> ueber HTTP ein 500.
	_, err = svc.Get(ctx, uuid.New(), "default")
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	_, err = svc.Get(ctx, emp.ID, "other-company")
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")

	// Vorher: nil - die Aenderung "gelang", obwohl nichts geschrieben wurde.
	wantEmployeeError(t, svc.Update(ctx, uuid.New(), map[string]any{"first_name": "X"}, "default"), "Mitarbeiter nicht gefunden")
	wantEmployeeError(t, svc.Update(ctx, emp.ID, map[string]any{"first_name": "Fremd"}, "other-company"), "Mitarbeiter nicht gefunden")

	got, err := svc.Get(ctx, emp.ID, "default")
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if got.FirstName != "Nicht" {
		t.Fatalf("Aenderung aus fremdem Mandanten wurde geschrieben: %q", got.FirstName)
	}
}

func TestEmployeePersonalnummerIsUniquePerCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)
	seedOtherCompany(t, env)
	pn := "PN-" + uuid.NewString()[:8]
	padded := "  " + pn + "  "
	blank := "   "

	first, err := svc.Create(ctx, EmployeeInput{FirstName: "Erste", LastName: "Nummer", PersonalNr: &padded}, "default")
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	if first.PersonalNr == nil || *first.PersonalNr != pn {
		t.Fatalf("erwartet getrimmte Personalnummer %q, bekommen %v", pn, first.PersonalNr)
	}

	// Fachliche Meldung statt rohem UNIQUE-Fehler. Die Meldung nennt die
	// Nummer bewusst nicht - sie landet im Log.
	_, err = svc.Create(ctx, EmployeeInput{FirstName: "Zweite", LastName: "Nummer", PersonalNr: &pn}, "default")
	wantEmployeeError(t, err, "Personalnummer existiert bereits")

	// Ein anderer Mandant darf dieselbe Nummer vergeben (Migration 087).
	if _, err := svc.Create(ctx, EmployeeInput{FirstName: "Andere", LastName: "Firma", PersonalNr: &pn}, "other-company"); err != nil {
		t.Fatalf("dieselbe Nummer bei anderem Mandanten muss zulaessig sein, bekommen %v", err)
	}

	// Eine leere Personalnummer ist keine: mehrere davon sind zulaessig
	// und werden als NULL gespeichert, nicht als leerer String.
	for i := 0; i < 2; i++ {
		e, err := svc.Create(ctx, EmployeeInput{FirstName: "Ohne", LastName: "Nummer", PersonalNr: &blank}, "default")
		if err != nil {
			t.Fatalf("Mitarbeiter %d ohne Personalnummer: %v", i+1, err)
		}
		got, err := svc.Get(ctx, e.ID, "default")
		if err != nil {
			t.Fatalf("Laden: %v", err)
		}
		if got.PersonalNr != nil {
			t.Fatalf("erwartet keine Personalnummer, bekommen %q", *got.PersonalNr)
		}
	}
}

func TestEmployeeTeamMustBelongToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)
	seedOtherCompany(t, env)
	own := seedTeam(t, env, "Werkstatt", "default")
	foreign := seedTeam(t, env, "Fremdes Team", "other-company")
	unknown := uuid.New()

	_, err := svc.Create(ctx, EmployeeInput{FirstName: "Team", LastName: "Fremd", TeamID: &foreign}, "default")
	wantEmployeeError(t, err, "Team nicht gefunden")
	_, err = svc.Create(ctx, EmployeeInput{FirstName: "Team", LastName: "Unbekannt", TeamID: &unknown}, "default")
	wantEmployeeError(t, err, "Team nicht gefunden")

	emp, err := svc.Create(ctx, EmployeeInput{FirstName: "Team", LastName: "Eigen", TeamID: &own}, "default")
	if err != nil {
		t.Fatalf("Anlegen mit eigenem Team: %v", err)
	}

	// Aendern: fremdes Team abgelehnt, und der Rest des Patches wird nicht
	// teilweise angewendet.
	err = svc.Update(ctx, emp.ID, map[string]any{"team_id": foreign.String(), "first_name": "Sollte nicht ankommen"}, "default")
	wantEmployeeError(t, err, "Team nicht gefunden")
	got, err := svc.Get(ctx, emp.ID, "default")
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if got.TeamID == nil || *got.TeamID != own || got.FirstName != "Team" {
		t.Fatalf("abgelehnter Patch wurde teilweise angewendet: %+v", got)
	}

	// team_id: nil loest die Zuordnung; als Text (so kommt es aus JSON) setzt sie.
	if err := svc.Update(ctx, emp.ID, map[string]any{"team_id": nil}, "default"); err != nil {
		t.Fatalf("Team loesen: %v", err)
	}
	if got, _ := svc.Get(ctx, emp.ID, "default"); got == nil || got.TeamID != nil {
		t.Fatalf("erwartet kein Team, bekommen %+v", got)
	}
	if err := svc.Update(ctx, emp.ID, map[string]any{"team_id": own.String()}, "default"); err != nil {
		t.Fatalf("Team als Text setzen: %v", err)
	}
	if got, _ := svc.Get(ctx, emp.ID, "default"); got == nil || got.TeamID == nil || *got.TeamID != own {
		t.Fatalf("erwartet eigenes Team, bekommen %+v", got)
	}
}

func TestEmployeeSetUserLinksAndUnlinks(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)
	timeSvc := NewTimeEntryService(env.PG)
	seedOtherCompany(t, env)

	userID := seedUserInCompany(t, env, "f43-link@example.invalid", "default")
	foreignUserID := seedUserInCompany(t, env, "f43-foreign@other.example.invalid", "other-company")
	a, err := svc.Create(ctx, EmployeeInput{FirstName: "Link", LastName: "A"}, "default")
	if err != nil {
		t.Fatalf("Anlegen A: %v", err)
	}
	b, err := svc.Create(ctx, EmployeeInput{FirstName: "Link", LastName: "B"}, "default")
	if err != nil {
		t.Fatalf("Anlegen B: %v", err)
	}

	if a.UserID != nil {
		t.Fatalf("neuer Mitarbeiter darf nicht verknuepft sein, bekommen %v", *a.UserID)
	}

	if err := svc.SetUser(ctx, a.ID, userID, "default"); err != nil {
		t.Fatalf("Verknuepfen: %v", err)
	}
	got, err := svc.Get(ctx, a.ID, "default")
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if got.UserID == nil || *got.UserID != userID {
		t.Fatalf("erwartet user_id=%s, bekommen %v", userID, got.UserID)
	}
	// Die Verknuepfung ist genau das, was die Zeiterfassung liest.
	if emp, linked, err := timeSvc.EmployeeIDForUser(ctx, userID, "default"); err != nil || !linked || emp != a.ID {
		t.Fatalf("EmployeeIDForUser: erwartet %s, bekommen %s (linked=%v, err=%v)", a.ID, emp, linked, err)
	}
	// Auch die Liste traegt die Verknuepfung.
	list, err := svc.List(ctx, 1000, 0, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	var inList bool
	for _, e := range list {
		if e.ID == a.ID {
			inList = e.UserID != nil && *e.UserID == userID
		}
	}
	if !inList {
		t.Fatal("Liste zeigt die Verknuepfung nicht")
	}

	// Erneut dieselbe Verknuepfung: kein Fehler.
	if err := svc.SetUser(ctx, a.ID, userID, "default"); err != nil {
		t.Fatalf("dieselbe Verknuepfung erneut setzen: %v", err)
	}

	// Ein Login gehoert zu hoechstens einem Mitarbeiter.
	wantEmployeeError(t, svc.SetUser(ctx, b.ID, userID, "default"), "Benutzer ist bereits mit einem anderen Mitarbeiter verknüpft")
	if got, _ := svc.Get(ctx, b.ID, "default"); got == nil || got.UserID != nil {
		t.Fatalf("abgelehnte Verknuepfung wurde geschrieben: %+v", got)
	}

	// Benutzer muss existieren und zum selben Mandanten gehoeren.
	wantEmployeeError(t, svc.SetUser(ctx, b.ID, "gibt-es-nicht", "default"), "Benutzer nicht gefunden")
	wantEmployeeError(t, svc.SetUser(ctx, b.ID, foreignUserID, "default"), "Benutzer nicht gefunden")
	// Mitarbeiter muss existieren und zum Mandanten gehoeren.
	wantEmployeeError(t, svc.SetUser(ctx, uuid.New(), "", "default"), "Mitarbeiter nicht gefunden")
	wantEmployeeError(t, svc.SetUser(ctx, b.ID, foreignUserID, "other-company"), "Mitarbeiter nicht gefunden")
	wantEmployeeError(t, svc.SetUser(ctx, a.ID, "", "other-company"), "Mitarbeiter nicht gefunden")
	if got, _ := svc.Get(ctx, a.ID, "default"); got == nil || got.UserID == nil {
		t.Fatalf("fremder Mandant hat die Verknuepfung geloest: %+v", got)
	}

	// Loesen, danach ist der Login fuer einen anderen Mitarbeiter frei.
	if err := svc.SetUser(ctx, a.ID, "  ", "default"); err != nil {
		t.Fatalf("Loesen: %v", err)
	}
	if got, _ := svc.Get(ctx, a.ID, "default"); got == nil || got.UserID != nil {
		t.Fatalf("erwartet geloeste Verknuepfung, bekommen %+v", got)
	}
	if _, linked, _ := timeSvc.EmployeeIDForUser(ctx, userID, "default"); linked {
		t.Fatal("EmployeeIDForUser findet nach dem Loesen noch einen Mitarbeiter")
	}
	if err := svc.SetUser(ctx, b.ID, userID, "default"); err != nil {
		t.Fatalf("Login nach dem Loesen neu verknuepfen: %v", err)
	}
}
