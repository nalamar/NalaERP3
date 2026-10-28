package fleet

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Fahrerzuordnung gegen echtes Postgres (Backlog G.3a.1a). Feste Daten in
// der Vergangenheit (2026-01 bis 2026-09); nur "in der Zukunft" rechnet
// mit heute.

func seedDriver(t *testing.T, env *testutil.IntegrationEnv, company, lastName string, active bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO hr_employees (id, first_name, last_name, company_id, active) VALUES ($1, 'Max', $2, $3, $4)`, id, lastName, company, active); err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return id
}

// seedLicenseCheck legt die Art (6 Monate) an, falls noetig, und einen
// Nachweis vom Tag erworben.
func seedLicenseCheck(t *testing.T, env *testutil.IntegrationEnv, company string, employeeID uuid.UUID, erworben string) hr.EmployeeQualification {
	t.Helper()
	ctx := context.Background()
	var typeID uuid.UUID
	err := env.PG.QueryRow(ctx, `SELECT id FROM hr_qualification_types WHERE company_id=$1 AND code=$2`, company, LicenseCheckCode).Scan(&typeID)
	if err != nil {
		six := 6
		qt, err := hr.NewQualificationTypeService(env.PG).Create(ctx, hr.QualificationTypeInput{Code: LicenseCheckCode, Name: "Führerscheinkontrolle", Kategorie: hr.QualificationKategorieQualifikation, GueltigkeitMonate: &six}, company)
		if err != nil {
			t.Fatalf("Art: %v", err)
		}
		typeID = qt.ID
	}
	d, _ := parseDay(erworben)
	recs, err := hr.NewQualificationService(env.PG).Create(ctx, hr.QualificationInput{EmployeeIDs: []uuid.UUID{employeeID}, TypeID: typeID, ErworbenAm: d, NachweisNr: "B, BE"}, "", company)
	if err != nil || len(recs) != 1 {
		t.Fatalf("Nachweis: %v", err)
	}
	return recs[0]
}

func parseDay(s string) (d time.Time, err error) { return time.Parse("2006-01-02", s) }

func countAssignments(t *testing.T, env *testutil.IntegrationEnv, vehicleIDs ...uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM fleet_vehicle_assignments WHERE vehicle_id = ANY($1)`, vehicleIDs).Scan(&n); err != nil {
		t.Fatalf("zaehlen: %v", err)
	}
	return n
}

func TestAssignFlowHintsAndList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAssignmentService(env.PG)
	schedules := NewScheduleService(env.PG)
	company := seedCompany(t, env)
	foreignCompany := seedCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "zuord-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")
	anna := seedDriver(t, env, company, "Anna", true)
	bert := seedDriver(t, env, company, "Bert", true)
	annaCheck := seedLicenseCheck(t, env, company, anna, "2026-01-10")
	v := seedVehicle(t, env, company, "M-ZU 1")
	w := seedVehicle(t, env, company, "M-ZU 2")
	x := seedVehicle(t, env, company, "M-ZU 3")
	if _, err := schedules.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminUVV, Starttermin: datePtr(2026, 2, 28)}, "", company); err != nil {
		t.Fatalf("UVV: %v", err)
	}
	if _, err := schedules.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminWartung, Starttermin: datePtr(2026, 3, 10)}, "", company); err != nil {
		t.Fatalf("Wartung: %v", err)
	}

	// Ueberfaellige UVV: Hinweis; gueltige Kontrolle: kein Hinweis; bald
	// faellige Wartung: kein Hinweis.
	a1, err := svc.Assign(ctx, AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: day(2026, 3, 1), Notiz: " Dienstwagen "}, actor, company)
	if err != nil {
		t.Fatalf("Zuordnung: %v", err)
	}
	if a1.VehicleID != v.ID || a1.Kennzeichen != "M-ZU 1" || a1.EmployeeID != anna || a1.MitarbeiterName != "Max Anna" || !a1.Beginn.Equal(day(2026, 3, 1)) ||
		a1.Ende != nil || a1.Notiz != "Dienstwagen" || a1.Status != AssignmentActive || a1.ZugeordnetByUserID == nil || *a1.ZugeordnetByUserID != actor ||
		a1.BeendetByUserID != nil || a1.StorniertAt != nil {
		t.Fatalf("Zuordnung: %+v", a1)
	}
	if want := []string{"UVV-Prüfung überfällig seit 28.02.2026"}; !reflect.DeepEqual(a1.Hinweise, want) {
		t.Fatalf("Hinweise: %q", a1.Hinweise)
	}
	if _, err := svc.Assign(ctx, AssignInput{VehicleID: v.ID, EmployeeID: bert, Beginn: day(2026, 3, 2)}, "", company); err == nil || err.Error() != "Fahrzeug ist bereits zugeordnet" {
		t.Fatalf("zweite offene Zuordnung: %v", err)
	}

	// Ohne Kontrolle: Hinweis; ein Mitarbeiter darf mehrere Fahrzeuge haben.
	a2, err := svc.Assign(ctx, AssignInput{VehicleID: w.ID, EmployeeID: bert, Beginn: day(2026, 3, 1)}, "", company)
	if err != nil || !reflect.DeepEqual(a2.Hinweise, []string{"Keine gültige Führerscheinkontrolle erfasst"}) || a2.ZugeordnetByUserID != nil {
		t.Fatalf("ohne Kontrolle: %+v %v", a2, err)
	}
	// Abgelaufene Kontrolle am Tag des Beginns.
	a3, err := svc.Assign(ctx, AssignInput{VehicleID: x.ID, EmployeeID: anna, Beginn: day(2026, 9, 1)}, "", company)
	if err != nil || len(a3.Hinweise) != 1 || a3.Hinweise[0] != "Führerscheinkontrolle abgelaufen seit "+annaCheck.GueltigBis.Format("02.01.2006") {
		t.Fatalf("abgelaufene Kontrolle: %+v %v", a3, err)
	}

	// Stornierte Zuordnung (per SQL; Storno folgt in G.3a.1b).
	if _, err := env.PG.Exec(ctx, `INSERT INTO fleet_vehicle_assignments (id, vehicle_id, employee_id, beginn, ende, status, storno_reason, storniert_at)
		VALUES ($1,$2,$3,'2026-01-01','2026-01-05','storniert','falsch',now())`, uuid.New(), w.ID, anna); err != nil {
		t.Fatalf("stornierte: %v", err)
	}
	ids := func(l []Assignment, err error) []uuid.UUID {
		t.Helper()
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		out := make([]uuid.UUID, len(l))
		for i, a := range l {
			out[i] = a.ID
		}
		return out
	}
	if got := ids(svc.ListAssignments(ctx, company, AssignmentFilter{})); len(got) != 3 || got[0] != a3.ID {
		t.Fatalf("alle, juengste zuerst: %v", got)
	}
	if got := ids(svc.ListAssignments(ctx, company, AssignmentFilter{IncludeStorniert: true})); len(got) != 4 {
		t.Fatalf("mit stornierten: %v", got)
	}
	if got := ids(svc.ListAssignments(ctx, company, AssignmentFilter{EmployeeID: &anna})); !reflect.DeepEqual(got, []uuid.UUID{a3.ID, a1.ID}) {
		t.Fatalf("je Mitarbeiter: %v", got)
	}
	if got := ids(svc.ListAssignments(ctx, company, AssignmentFilter{VehicleID: &w.ID, IncludeStorniert: true})); len(got) != 2 || got[0] != a2.ID {
		t.Fatalf("je Fahrzeug: %v", got)
	}
	if got := ids(svc.ListAssignments(ctx, foreignCompany, AssignmentFilter{IncludeStorniert: true})); len(got) != 0 {
		t.Fatalf("fremder Mandant: %v", got)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE fleet_vehicle_assignments SET ende='2026-03-20' WHERE id=$1`, a2.ID); err != nil {
		t.Fatalf("Ende per SQL: %v", err)
	}
	if got := ids(svc.ListAssignments(ctx, company, AssignmentFilter{NurOffen: true})); !reflect.DeepEqual(got, []uuid.UUID{a3.ID, a1.ID}) {
		t.Fatalf("nur offene: %v", got)
	}
	if g, err := svc.GetAssignment(ctx, a1.ID, company); err != nil || g.ID != a1.ID || g.Hinweise != nil {
		t.Fatalf("Einzelabruf: %+v %v", g, err)
	}
	if _, err := svc.GetAssignment(ctx, a1.ID, foreignCompany); err == nil || err.Error() != "Zuordnung nicht gefunden" {
		t.Fatalf("fremder Einzelabruf: %v", err)
	}
}

func TestAssignErrorsAndBlockWithoutEffect(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAssignmentService(env.PG)
	schedules := NewScheduleService(env.PG)
	vehicles := NewVehicleService(env.PG)
	company := seedCompany(t, env)
	foreignCompany := seedCompany(t, env)
	anna := seedDriver(t, env, company, "Anna", true)
	inaktiv := seedDriver(t, env, company, "Inaktiv", false)
	foreignEmp := seedDriver(t, env, foreignCompany, "Fremd", true)
	v, err := vehicles.Create(ctx, VehicleInput{Kennzeichen: "M-ER 1", Art: ArtPkw, Erstzulassung: datePtr(2026, 2, 1)}, company, "")
	if err != nil {
		t.Fatalf("Fahrzeug: %v", err)
	}
	gone := seedVehicle(t, env, company, "M-ER 2")
	if _, err := vehicles.Deregister(ctx, gone.ID, day(2026, 2, 1), "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	locked := seedVehicle(t, env, company, "M-ER 3")
	hu, _ := schedules.CreateSchedule(ctx, ScheduleInput{VehicleID: locked.ID, Art: TerminHU}, "", company)
	seedCompletion(t, env, hu.ID, "2026-02-15", ErgebnisMaengel, nil, CompletionActive)
	foreignVehicle := seedVehicle(t, env, foreignCompany, "M-ER 4")

	future := today().AddDate(0, 0, 1)
	for name, tc := range map[string]struct {
		in   AssignInput
		want string
	}{
		"Fahrzeug unbekannt":  {AssignInput{VehicleID: uuid.New(), EmployeeID: anna, Beginn: day(2026, 3, 1)}, "Fahrzeug nicht gefunden"},
		"fremdes Fahrzeug":    {AssignInput{VehicleID: foreignVehicle.ID, EmployeeID: anna, Beginn: day(2026, 3, 1)}, "Fahrzeug nicht gefunden"},
		"abgemeldet":          {AssignInput{VehicleID: gone.ID, EmployeeID: anna, Beginn: day(2026, 3, 1)}, "Fahrzeug ist abgemeldet"},
		"ohne Beginn":         {AssignInput{VehicleID: v.ID, EmployeeID: anna}, "Beginn erforderlich"},
		"Zukunft":             {AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: future}, "Beginn darf nicht in der Zukunft liegen"},
		"vor Erstzulassung":   {AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: day(2026, 1, 31)}, "Beginn darf nicht vor der Erstzulassung liegen"},
		"Mitarbeiter fehlt":   {AssignInput{VehicleID: v.ID, EmployeeID: uuid.New(), Beginn: day(2026, 3, 1)}, "Mitarbeiter nicht gefunden"},
		"fremder Mitarbeiter": {AssignInput{VehicleID: v.ID, EmployeeID: foreignEmp, Beginn: day(2026, 3, 1)}, "Mitarbeiter nicht gefunden"},
		"inaktiv":             {AssignInput{VehicleID: v.ID, EmployeeID: inaktiv, Beginn: day(2026, 3, 1)}, "Mitarbeiter ist nicht aktiv"},
		"HU mit Maengeln":     {AssignInput{VehicleID: locked.ID, EmployeeID: anna, Beginn: day(2026, 3, 1)}, "Fahrzeug ist gesperrt: Hauptuntersuchung mit Mängeln"},
		"ohne Mandant":        {AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: day(2026, 3, 1)}, "Mandant erforderlich"},
	} {
		c := company
		if name == "ohne Mandant" {
			c = ""
		}
		if _, err := svc.Assign(ctx, tc.in, "", c); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if n := countAssignments(t, env, v.ID, gone.ID, locked.ID, foreignVehicle.ID); n != 0 {
		t.Fatalf("nach Fehlern %d Zuordnungen", n)
	}

	// Sperre am Tag des Beginns: eine spaeter bestandene HU hebt sie fuer
	// den frueheren Tag nicht auf, ab ihrem Datum schon.
	seedCompletion(t, env, hu.ID, "2026-03-05", ErgebnisBestanden, nil, CompletionActive)
	if _, err := svc.Assign(ctx, AssignInput{VehicleID: locked.ID, EmployeeID: anna, Beginn: day(2026, 3, 4)}, "", company); err == nil || !strings.Contains(err.Error(), "gesperrt") {
		t.Fatalf("vor der bestandenen HU: %v", err)
	}
	if _, err := svc.Assign(ctx, AssignInput{VehicleID: locked.ID, EmployeeID: anna, Beginn: day(2026, 3, 5)}, "", company); err != nil {
		t.Fatalf("ab der bestandenen HU: %v", err)
	}

	// Eine stillgelegte UVV mit Maengeln sperrt nicht.
	uvv, _ := schedules.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminUVV}, "", company)
	seedCompletion(t, env, uvv.ID, "2026-02-10", ErgebnisNichtBestanden, nil, CompletionActive)
	if _, err := svc.Assign(ctx, AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: day(2026, 3, 1)}, "", company); err == nil || err.Error() != "Fahrzeug ist gesperrt: UVV-Prüfung nicht bestanden" {
		t.Fatalf("UVV nicht bestanden: %v", err)
	}
	if _, err := schedules.UpdateSchedule(ctx, uvv.ID, ScheduleUpdate{Aktiv: false}, company); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}

	// Ueberschneidung mit einer beendeten Zuordnung (per SQL; Ende folgt in
	// G.3a.1b); Beginn am Tag des Endes ist zulaessig.
	if _, err := env.PG.Exec(ctx, `INSERT INTO fleet_vehicle_assignments (id, vehicle_id, employee_id, beginn, ende) VALUES ($1,$2,$3,'2026-03-01','2026-03-10')`, uuid.New(), v.ID, anna); err != nil {
		t.Fatalf("beendete: %v", err)
	}
	if _, err := svc.Assign(ctx, AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: day(2026, 3, 9)}, "", company); err == nil || err.Error() != "Zuordnung überschneidet sich mit einer früheren Zuordnung dieses Fahrzeugs" {
		t.Fatalf("Ueberschneidung: %v", err)
	}
	if n := countAssignments(t, env, v.ID); n != 1 {
		t.Fatalf("nach Ueberschneidung %d", n)
	}
	// Eine stornierte Zuordnung ueberschneidet sich nicht.
	if _, err := env.PG.Exec(ctx, `INSERT INTO fleet_vehicle_assignments (id, vehicle_id, employee_id, beginn, ende, status, storno_reason, storniert_at)
		VALUES ($1,$2,$3,'2026-03-05','2026-03-25','storniert','falsch',now())`, uuid.New(), v.ID, anna); err != nil {
		t.Fatalf("stornierte: %v", err)
	}
	if _, err := svc.Assign(ctx, AssignInput{VehicleID: v.ID, EmployeeID: anna, Beginn: day(2026, 3, 10)}, "", company); err != nil {
		t.Fatalf("Beginn am Tag des Endes: %v", err)
	}
}

// CHECKs und Index aus Migration 102 gegen Direkt-SQL.
func TestAssignmentTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-CK 1")
	emp := seedDriver(t, env, company, "Check", true)
	ins := func(beginn string, ende any, status, reason string, at any) error {
		_, err := env.PG.Exec(ctx, `INSERT INTO fleet_vehicle_assignments (id, vehicle_id, employee_id, beginn, ende, status, storno_reason, storniert_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.New(), v.ID, emp, beginn, ende, status, reason, at)
		return err
	}
	if err := ins("2026-01-01", nil, "active", "", nil); err != nil {
		t.Fatalf("erste offene: %v", err)
	}
	for name, err := range map[string]error{
		"zweite offene":         ins("2026-01-02", nil, "active", "", nil),
		"Ende vor Beginn":       ins("2026-01-05", "2026-01-04", "active", "", nil),
		"Status unbekannt":      ins("2025-01-01", "2025-01-02", "beendet", "", nil),
		"Storno ohne Grund":     ins("2025-01-01", "2025-01-02", "storniert", " ", time.Now()),
		"aktiv mit Stornodatum": ins("2025-01-01", "2025-01-02", "active", "", time.Now()),
		"Storno ohne Zeitpunkt": ins("2025-01-01", "2025-01-02", "storniert", "falsch", nil),
	} {
		if err == nil {
			t.Fatalf("%s: angenommen", name)
		}
	}
	// Eine stornierte offene Zuordnung zaehlt nicht fuer den Index.
	if err := ins("2026-01-03", nil, "storniert", "falsch", time.Now()); err != nil {
		t.Fatalf("stornierte offene: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM fleet_vehicles WHERE id=$1`, v.ID); err == nil {
		t.Fatal("Fahrzeug mit Zuordnungen geloescht")
	}
}
