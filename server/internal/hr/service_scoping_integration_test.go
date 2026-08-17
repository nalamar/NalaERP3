package hr

import (
	"context"
	"testing"
	"time"

	"nalaerp3/internal/testutil"
)

// Der hr-Domaenen-Code ist aktuell an KEINEN HTTP-Handler angebunden (siehe
// docs/state.md, Subtask 0.2.2.1.6), daher gibt es keine HTTP-Integrationstests
// dafuer. Diese Tests verifizieren das in dieser Subtask ergaenzte
// company_id-Scoping direkt gegen echtes Postgres.

func TestEmployeeGetIsScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)

	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ('other-company','Andere Firma GmbH') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed other company: %v", err)
	}

	created, err := svc.Create(ctx, Employee{FirstName: "Max", LastName: "Mustermann"}, "default")
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	if _, err := svc.Get(ctx, created.ID, "default"); err != nil {
		t.Fatalf("expected owner company to find employee, got %v", err)
	}

	if _, err := svc.Get(ctx, created.ID, "other-company"); err == nil {
		t.Fatal("expected other company lookup to fail, got nil error")
	}

	list, err := svc.List(ctx, 50, 0, "other-company")
	if err != nil {
		t.Fatalf("list for other company: %v", err)
	}
	for _, e := range list {
		if e.ID == created.ID {
			t.Fatalf("expected other company's list to not contain employee from default company, got %+v", e)
		}
	}
}

// TestEmployeeUpdateAppliesKnownFieldsAndRejectsUnknownKey belegt gegen
// echtes Postgres, dass die in Backlog 0.10 umgebaute Update()-Validierung
// (Loop statt drei identische switch-Faelle) die bekannten Felder weiterhin
// korrekt anwendet UND dass ein Patch mit einem unbekannten Key abgelehnt
// wird, OHNE die bekannten Felder im selben Patch teilweise anzuwenden.
func TestEmployeeUpdateAppliesKnownFieldsAndRejectsUnknownKey(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewEmployeeService(env.PG)

	created, err := svc.Create(ctx, Employee{FirstName: "Erika", LastName: "Musterfrau", Active: true}, "default")
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	if err := svc.Update(ctx, created.ID, map[string]any{
		"first_name": "Erika-Update",
		"active":     false,
	}, "default"); err != nil {
		t.Fatalf("expected known-fields update to succeed, got %v", err)
	}
	afterKnownUpdate, err := svc.Get(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("get after known-fields update: %v", err)
	}
	if afterKnownUpdate.FirstName != "Erika-Update" || afterKnownUpdate.Active != false {
		t.Fatalf("expected first_name/active to be updated, got %+v", afterKnownUpdate)
	}

	// Ein Patch mit EINEM unbekannten Key (Tippfehler) wird komplett
	// abgelehnt - first_name darf NICHT auf "Sollte nicht ankommen" wechseln.
	err = svc.Update(ctx, created.ID, map[string]any{
		"first_name": "Sollte nicht ankommen",
		"activ":      true,
	}, "default")
	if err == nil {
		t.Fatal("expected error for patch containing an unknown key, got nil")
	}
	afterRejectedUpdate, err := svc.Get(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("get after rejected update: %v", err)
	}
	if afterRejectedUpdate.FirstName != "Erika-Update" {
		t.Fatalf("expected first_name unchanged after rejected patch, got %q", afterRejectedUpdate.FirstName)
	}
}

func TestLeaveCreateRejectsEmployeeFromOtherCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	empSvc := NewEmployeeService(env.PG)
	leaveSvc := NewLeaveService(env.PG)

	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ('other-company','Andere Firma GmbH') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed other company: %v", err)
	}

	created, err := empSvc.Create(ctx, Employee{FirstName: "Erika", LastName: "Musterfrau"}, "default")
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	_, err = leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: created.ID,
		StartDate:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
	}, "other-company")
	if err == nil {
		t.Fatal("expected error creating leave request for employee of a different company, got nil")
	}
	if err.Error() != "Mitarbeiter nicht gefunden" {
		t.Fatalf("expected 'Mitarbeiter nicht gefunden', got %q", err.Error())
	}
}

// TestLeaveCreateComputesDaysForValidDateRange belegt gegen echtes Postgres,
// dass die in Backlog 0.11 ergaenzte EndDate>=StartDate-Pruefung den
// Normalfall (gueltiger Datumsbereich) nicht versehentlich mitblockiert -
// nur die Validierungsschicht per Unit-Test zu pruefen (service_test.go)
// wuerde eine Regression im Erfolgspfad nicht aufdecken.
func TestLeaveCreateComputesDaysForValidDateRange(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	empSvc := NewEmployeeService(env.PG)
	leaveSvc := NewLeaveService(env.PG)

	emp, err := empSvc.Create(ctx, Employee{FirstName: "Urlaub", LastName: "Testperson"}, "default")
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	created, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), // 5 Tage inkl. beider Enden
	}, "default")
	if err != nil {
		t.Fatalf("expected valid date range to succeed, got %v", err)
	}
	if created.Days != 5 {
		t.Fatalf("expected 5 days, got %v", created.Days)
	}

	// Ein eintaegiger Antrag (StartDate == EndDate) ist ebenfalls gueltig -
	// EndDate.Before(StartDate) liefert dafuer false.
	sameDay, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}, "default")
	if err != nil {
		t.Fatalf("expected same-day date range to succeed, got %v", err)
	}
	if sameDay.Days != 1 {
		t.Fatalf("expected 1 day for same-day request, got %v", sameDay.Days)
	}
}

// TestLeaveCreateRejectsOverlappingDateRange belegt Backlog 0.12: ein neuer
// Urlaubsantrag, dessen Zeitraum sich mit einem bereits bestehenden
// pending/approved Antrag DESSELBEN Mitarbeiters ueberschneidet, wird
// abgelehnt - vorher waren beliebig viele (auch mehrfach genehmigte)
// ueberlappende Antraege moeglich. Da die Pruefung eine Datenbankabfrage
// erfordert, ist sie nur gegen echtes Postgres testbar.
func TestLeaveCreateRejectsOverlappingDateRange(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	empSvc := NewEmployeeService(env.PG)
	leaveSvc := NewLeaveService(env.PG)

	emp, err := empSvc.Create(ctx, Employee{FirstName: "Ueberschneidung", LastName: "Testperson"}, "default")
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	first, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
	}, "default")
	if err != nil {
		t.Fatalf("create first leave request: %v", err)
	}

	// Ueberlappender Zeitraum (5.-15. November) auf einen noch "pending"
	// Antrag wird abgelehnt.
	_, err = leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
	}, "default")
	if err == nil {
		t.Fatal("expected overlap error for pending request, got nil")
	}
	if err.Error() != "Zeitraum überschneidet sich mit einem bestehenden Urlaubsantrag" {
		t.Fatalf("expected overlap error message, got %q", err.Error())
	}

	// Direkt anschliessender, NICHT ueberlappender Zeitraum (11.-15.
	// November, beginnt erst nach dem Ende des ersten Antrags) ist weiterhin
	// erlaubt.
	if _, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
	}, "default"); err != nil {
		t.Fatalf("expected adjacent, non-overlapping date range to succeed, got %v", err)
	}

	// Ein bereits genehmigter Antrag blockiert Ueberschneidungen genauso wie
	// ein pending Antrag.
	// emp.ID als Approver: approver_id hat eine Fremdschluessel-Referenz auf
	// hr_employees(id), ein frei erfundener uuid.New() wuerde diese verletzen.
	if err := leaveSvc.Approve(ctx, first.ID, emp.ID, true, "default"); err != nil {
		t.Fatalf("approve first request: %v", err)
	}
	if _, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
	}, "default"); err == nil {
		t.Fatal("expected overlap error for approved request, got nil")
	}

	// Ein ABGELEHNTER Antrag blockiert dagegen keine neuen Antraege im
	// selben Zeitraum mehr - die Ueberschneidungspruefung ignoriert
	// status='rejected' bewusst.
	rejected, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC),
	}, "default")
	if err != nil {
		t.Fatalf("create request to be rejected: %v", err)
	}
	if err := leaveSvc.Approve(ctx, rejected.ID, emp.ID, false, "default"); err != nil {
		t.Fatalf("reject request: %v", err)
	}
	if _, err := leaveSvc.Create(ctx, LeaveRequest{
		EmployeeID: emp.ID,
		StartDate:  time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC),
	}, "default"); err != nil {
		t.Fatalf("expected same date range to succeed after prior request was rejected, got %v", err)
	}
}
