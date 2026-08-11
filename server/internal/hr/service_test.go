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
// vor dem DB-Zugriff, Update() liefert bei leerem oder ausschliesslich aus
// unbekannten Feldern bestehendem Patch ein no-op OHNE Postgres anzufassen -
// beides ist damit ueber die echten Einstiegspunkte mit nil-Pool testbar.

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

			e, err := s.Create(context.Background(), tc.input)
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

	if err := s.Update(context.Background(), uuid.New(), map[string]any{}); err != nil {
		t.Fatalf("expected nil error for empty patch, got %v", err)
	}
	if err := s.Update(context.Background(), uuid.New(), nil); err != nil {
		t.Fatalf("expected nil error for nil patch, got %v", err)
	}
}

// Update() ignoriert unbekannte Patch-Keys still, statt einen Fehler
// zurueckzugeben (server/internal/hr/service.go:128-146: nur first_name,
// last_name, email, phone, role, location, cost_center, active, team_id
// werden in die SET-Klausel uebernommen; alles andere faellt durch den
// switch und wird verworfen). Ein Patch, der ausschliesslich unbekannte Keys
// enthaelt, fuehrt dadurch zu einem stillen No-Op statt zu einem Fehler -
// z.B. bei einem Tippfehler wie "activ" statt "active" wuerde der Aufruf
// erfolgreich zurueckkehren, ohne irgendetwas zu aendern. Siehe
// docs/backlog.md 0.10 fuer die daraus abgeleitete Backlog-Position.
func TestUpdateWithOnlyUnknownKeysIsSilentNoOp(t *testing.T) {
	s := NewEmployeeService(nil)

	err := s.Update(context.Background(), uuid.New(), map[string]any{
		"activ":            true, // Tippfehler statt "active"
		"unbekanntes_feld": "x",
	})
	if err != nil {
		t.Fatalf("expected silent no-op (nil error) for unknown-only patch, got %v", err)
	}
}

// LeaveService.Create() validiert employee_id und start/end VOR dem
// DB-Zugriff (server/internal/hr/service.go:170-179) - beides mit nil-Pool
// testbar. Alles danach (Tage-Berechnung, INSERT) laeuft ungeprueft bis zum
// echten s.pg.Exec durch und ist daher NICHT mit nil-Pool aufrufbar, ohne zu
// panicen (gleiches Muster wie der vorbestehende Bug in
// server/internal/purchasing, siehe docs/backlog.md 0.6) - deshalb wird der
// unten dokumentierte Tage-Berechnungsfehler separat ueber die reine
// Zeitarithmetik nachgewiesen statt ueber Create() selbst.

func TestLeaveCreateRejectsMissingEmployeeID(t *testing.T) {
	s := NewLeaveService(nil)

	lr, err := s.Create(context.Background(), LeaveRequest{
		StartDate: time.Now(),
		EndDate:   time.Now().Add(24 * time.Hour),
	})
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
			})
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

// TestLeaveCreateDaysFormulaCanProduceNonPositiveDaysForInvertedDateRange
// belegt einen bei dieser Subtask gefundenen Mangel: Create() prueft nicht,
// ob EndDate nach StartDate liegt (server/internal/hr/service.go:177-182).
// Bei vertauschten Daten berechnet dieselbe Formel wie in Create() einen
// Days-Wert <= 0, der ungeprueft in die DB geschrieben wuerde. Der Test ruft
// bewusst NICHT Create() selbst auf (das wuerde ohne echten DB-Zugriff mit
// nil-Pool panicen, siehe Kommentar oben), sondern reproduziert exakt die
// Formel aus service.go, um den Fund nachvollziehbar zu belegen. Siehe
// docs/backlog.md 0.11.
func TestLeaveCreateDaysFormulaCanProduceNonPositiveDaysForInvertedDateRange(t *testing.T) {
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC) // vor start

	days := end.Sub(start).Hours()/24 + 1

	if days > 0 {
		t.Fatalf("Fund widerlegt: days=%v ist positiv trotz vertauschter Daten - docs/backlog.md 0.11 pruefen", days)
	}
}
