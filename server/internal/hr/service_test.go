package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// EmployeeService.List/Get/ListTeams greifen ohne jede Vorab-Validierung
// sofort auf Postgres zu und sind daher ohne DB nicht testbar (kein Mock im
// Repo, siehe docs/adr/0001-baseline.md). Create() validiert Vor-/Nachname
// vor dem DB-Zugriff, Update() validiert einen leeren Patch (no-op) sowie
// unbekannte Patch-Keys (Fehler, Backlog 0.10) OHNE Postgres anzufassen -
// beides ist damit ueber die echten Einstiegspunkte mit nil-Pool testbar.

func TestEmployeeCreateRejectsMissingCompanyID(t *testing.T) {
	s := NewEmployeeService(nil)

	e, err := s.Create(context.Background(), Employee{FirstName: "Max", LastName: "Muster"}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if e != nil {
		t.Fatalf("expected nil employee, got %#v", e)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected 'Mandant erforderlich', got %q", err.Error())
	}
}

func TestCreateRejectsMissingNames(t *testing.T) {
	cases := []struct {
		name  string
		input Employee
	}{
		{"leerer Vor- und Nachname", Employee{}},
		{"leerer Vorname", Employee{LastName: "Muster"}},
		{"leerer Nachname", Employee{FirstName: "Max"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewEmployeeService(nil)

			e, err := s.Create(context.Background(), tc.input, "default")
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if e != nil {
				t.Fatalf("expected nil employee, got %#v", e)
			}
			if err.Error() != "Vor- und Nachname erforderlich" {
				t.Fatalf("expected 'Vor- und Nachname erforderlich', got %q", err.Error())
			}
		})
	}
}

func TestUpdateWithEmptyPatchIsNoOp(t *testing.T) {
	s := NewEmployeeService(nil)

	if err := s.Update(context.Background(), uuid.New(), map[string]any{}, "default"); err != nil {
		t.Fatalf("expected nil error for empty patch, got %v", err)
	}
	if err := s.Update(context.Background(), uuid.New(), nil, "default"); err != nil {
		t.Fatalf("expected nil error for nil patch, got %v", err)
	}
}

// Update() lehnt unbekannte Patch-Keys jetzt mit einem Fehler ab, statt sie
// still zu ignorieren (Backlog 0.10 - vorher fiel z.B. ein Tippfehler wie
// "activ" statt "active" stillschweigend durch und der Aufruf kehrte ohne
// Fehler zurueck, obwohl nichts geaendert wurde). Beide Faelle sind mit
// nil-Pool testbar, da die Validierung VOR jedem DB-Zugriff passiert.
func TestUpdateRejectsUnknownKeys(t *testing.T) {
	s := NewEmployeeService(nil)

	err := s.Update(context.Background(), uuid.New(), map[string]any{
		"activ":            true, // Tippfehler statt "active"
		"unbekanntes_feld": "x",
	}, "default")
	if err == nil {
		t.Fatal("expected error for unknown patch keys, got nil")
	}
}

// Ein gemischter Patch mit EINEM bekannten und EINEM unbekannten Key wird
// ebenfalls komplett abgelehnt (alles-oder-nichts) - kein teilweises
// Anwenden nur der bekannten Felder.
func TestUpdateRejectsPatchWithAnySingleUnknownKey(t *testing.T) {
	s := NewEmployeeService(nil)

	err := s.Update(context.Background(), uuid.New(), map[string]any{
		"first_name": "Max",
		"activ":      true,
	}, "default")
	if err == nil {
		t.Fatal("expected error for patch containing an unknown key, got nil")
	}
}

// LeaveService.Create() validiert employee_id, start/end sowie (seit
// Backlog 0.11) EndDate>=StartDate VOR dem DB-Zugriff
// (server/internal/hr/service.go:172-184) - alles davon mit nil-Pool
// testbar. Alles danach (employeeOwned-Lookup, Tage-Berechnung, INSERT)
// laeuft bis zum echten s.pg.Exec durch und ist daher NICHT mit nil-Pool
// aufrufbar, ohne zu panicen (gleiches Muster wie der vorbestehende Bug in
// server/internal/purchasing, siehe docs/backlog.md 0.6) - der Normalfall
// (gueltige Daten, korrekte Tage-Berechnung) wird deshalb per
// Integrationstest gegen echtes Postgres nachgewiesen, siehe
// service_scoping_integration_test.go.

func TestLeaveCreateRejectsMissingCompanyID(t *testing.T) {
	s := NewLeaveService(nil)

	lr, err := s.Create(context.Background(), LeaveRequest{
		EmployeeID: uuid.New(),
		StartDate:  time.Now(),
		EndDate:    time.Now().Add(24 * time.Hour),
	}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if lr != nil {
		t.Fatalf("expected nil leave request, got %#v", lr)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected 'Mandant erforderlich', got %q", err.Error())
	}
}

func TestLeaveCreateRejectsMissingEmployeeID(t *testing.T) {
	s := NewLeaveService(nil)

	lr, err := s.Create(context.Background(), LeaveRequest{
		StartDate: time.Now(),
		EndDate:   time.Now().Add(24 * time.Hour),
	}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if lr != nil {
		t.Fatalf("expected nil leave request, got %#v", lr)
	}
	if err.Error() != "employee_id erforderlich" {
		t.Fatalf("expected 'employee_id erforderlich', got %q", err.Error())
	}
}

func TestLeaveCreateRejectsMissingStartOrEndDate(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"beide Daten fehlen", time.Time{}, time.Time{}},
		{"Startdatum fehlt", time.Time{}, now},
		{"Enddatum fehlt", now, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewLeaveService(nil)

			lr, err := s.Create(context.Background(), LeaveRequest{
				EmployeeID: uuid.New(),
				StartDate:  tc.start,
				EndDate:    tc.end,
			}, "default")
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if lr != nil {
				t.Fatalf("expected nil leave request, got %#v", lr)
			}
			if err.Error() != "start/end erforderlich" {
				t.Fatalf("expected 'start/end erforderlich', got %q", err.Error())
			}
		})
	}
}

// TestLeaveCreateRejectsEndDateBeforeStartDate belegt die in Backlog 0.11
// ergaenzte Validierung: Create() prueft jetzt VOR jedem DB-Zugriff, ob
// EndDate vor StartDate liegt, und lehnt vertauschte Daten explizit ab -
// vorher berechnete dieselbe Formel bei vertauschten Daten einen Days-Wert
// <= 0, der ungeprueft in die DB geschrieben worden waere. Da die neue
// Pruefung VOR dem employeeOwned-DB-Zugriff passiert, ist sie jetzt (anders
// als vorher) direkt ueber Create() mit nil-Pool testbar.
func TestLeaveCreateRejectsEndDateBeforeStartDate(t *testing.T) {
	s := NewLeaveService(nil)

	_, err := s.Create(context.Background(), LeaveRequest{
		EmployeeID: uuid.New(),
		StartDate:  time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), // vor StartDate
	}, "default")
	if err == nil {
		t.Fatal("expected error for end date before start date, got nil")
	}
	if err.Error() != "Enddatum darf nicht vor Startdatum liegen" {
		t.Fatalf("expected 'Enddatum darf nicht vor Startdatum liegen', got %q", err.Error())
	}
}
