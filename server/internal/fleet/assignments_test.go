package fleet

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// Regeln der Fahrerzuordnung ohne Datenbank (Backlog G.3a.1a).

func TestValidateAssignmentStart(t *testing.T) {
	today := day(2026, 5, 10)
	ez := datePtr(2026, 2, 1)
	if err := ValidateAssignmentStart(time.Time{}, ez, today); err == nil || err.Error() != "Beginn erforderlich" {
		t.Fatalf("leer: %v", err)
	}
	if err := ValidateAssignmentStart(day(2026, 5, 11), ez, today); err == nil || err.Error() != "Beginn darf nicht in der Zukunft liegen" {
		t.Fatalf("Zukunft: %v", err)
	}
	if err := ValidateAssignmentStart(day(2026, 1, 31), ez, today); err == nil || err.Error() != "Beginn darf nicht vor der Erstzulassung liegen" {
		t.Fatalf("vor Erstzulassung: %v", err)
	}
	for name, err := range map[string]error{
		"heute":                 ValidateAssignmentStart(today, ez, today),
		"Tag der Erstzulassung": ValidateAssignmentStart(day(2026, 2, 1), ez, today),
		"ohne Erstzulassung":    ValidateAssignmentStart(day(2020, 1, 1), nil, today),
	} {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func withStanding(art, bezeichnung, status, ergebnis string, faellig *time.Time) ScheduleWithStanding {
	return ScheduleWithStanding{
		Schedule: Schedule{ID: uuid.New(), Art: art, Bezeichnung: bezeichnung},
		Standing: ScheduleStanding{Status: status, LetztesErgebnis: ergebnis, FaelligAm: faellig},
	}
}

func TestAssignmentBlockAndScheduleHints(t *testing.T) {
	list := []ScheduleWithStanding{
		withStanding(TerminHU, "", TerminstandOK, ErgebnisBestanden, datePtr(2027, 3, 31)),
		withStanding(TerminUVV, "", TerminstandUeberfaellig, "", datePtr(2026, 4, 30)),
		withStanding(TerminWartung, "", TerminstandFaelligBald, "", datePtr(2026, 5, 20)),
		withStanding(TerminSonstige, "Tachograph", TerminstandUeberfaellig, "", datePtr(2026, 1, 2)),
		withStanding(TerminVersicherung, "", TerminstandUeberfaellig, "", nil),
	}
	if err := assignmentBlock(list); err != nil {
		t.Fatalf("ohne Sperre: %v", err)
	}
	want := []string{"UVV-Prüfung überfällig seit 30.04.2026", "Tachograph überfällig seit 02.01.2026"}
	if got := scheduleHints(list); !reflect.DeepEqual(got, want) {
		t.Fatalf("Hinweise: %q", got)
	}
	if got := scheduleHints(nil); got == nil || len(got) != 0 {
		t.Fatalf("ohne Termine: %#v", got)
	}

	blocked := []ScheduleWithStanding{
		withStanding(TerminHU, "", TerminstandOK, ErgebnisBestanden, nil),
		withStanding(TerminUVV, "", TerminstandGesperrt, ErgebnisMaengel, datePtr(2026, 6, 1)),
	}
	if err := assignmentBlock(blocked); err == nil || err.Error() != "Fahrzeug ist gesperrt: UVV-Prüfung mit Mängeln" {
		t.Fatalf("UVV Maengel: %v", err)
	}
	blocked[0] = withStanding(TerminHU, "", TerminstandGesperrt, ErgebnisNichtBestanden, nil)
	if err := assignmentBlock(blocked); err == nil || err.Error() != "Fahrzeug ist gesperrt: Hauptuntersuchung nicht bestanden" {
		t.Fatalf("HU zuerst: %v", err)
	}
}

func licenseStanding(code, status string, gueltigBis *time.Time) hr.EmployeeQualificationStanding {
	return hr.EmployeeQualificationStanding{Code: code, QualificationStanding: hr.QualificationStanding{Status: status, GueltigBis: gueltigBis}}
}

func TestLicenseHint(t *testing.T) {
	other := licenseStanding("STAPLER", hr.QualificationStandGueltig, nil)
	for name, tc := range map[string]struct {
		in   []hr.EmployeeQualificationStanding
		want string
	}{
		"gueltig":           {[]hr.EmployeeQualificationStanding{other, licenseStanding(LicenseCheckCode, hr.QualificationStandGueltig, datePtr(2026, 9, 1))}, ""},
		"laeuft ab":         {[]hr.EmployeeQualificationStanding{licenseStanding(LicenseCheckCode, hr.QualificationStandLaeuftAb, datePtr(2026, 5, 20))}, ""},
		"abgelaufen":        {[]hr.EmployeeQualificationStanding{licenseStanding(LicenseCheckCode, hr.QualificationStandAbgelaufen, datePtr(2026, 4, 30))}, "Führerscheinkontrolle abgelaufen seit 30.04.2026"},
		"abgelaufen o. Tag": {[]hr.EmployeeQualificationStanding{licenseStanding(LicenseCheckCode, hr.QualificationStandAbgelaufen, nil)}, "Führerscheinkontrolle abgelaufen"},
		"fehlt":             {[]hr.EmployeeQualificationStanding{licenseStanding(LicenseCheckCode, hr.QualificationStandFehlt, nil)}, "Keine gültige Führerscheinkontrolle erfasst"},
		"nur andere Arten":  {[]hr.EmployeeQualificationStanding{other}, "Keine gültige Führerscheinkontrolle erfasst"},
		"nichts":            {nil, "Keine gültige Führerscheinkontrolle erfasst"},
	} {
		if got := licenseHint(tc.in); got != tc.want {
			t.Fatalf("%s: %q", name, got)
		}
	}
}

// Assign prueft Mandant und IDs, bevor es Postgres anfasst (nil-Pool).
func TestAssignValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewAssignmentService(nil)
	ctx := context.Background()
	in := AssignInput{VehicleID: uuid.New(), EmployeeID: uuid.New(), Beginn: day(2026, 1, 1)}
	if _, err := s.Assign(ctx, in, "", " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant: %v", err)
	}
	noVehicle := in
	noVehicle.VehicleID = uuid.Nil
	if _, err := s.Assign(ctx, noVehicle, "", "c1"); err == nil || err.Error() != "vehicle_id erforderlich" {
		t.Fatalf("ohne Fahrzeug: %v", err)
	}
	noEmployee := in
	noEmployee.EmployeeID = uuid.Nil
	if _, err := s.Assign(ctx, noEmployee, "", "c1"); err == nil || err.Error() != "employee_id erforderlich" {
		t.Fatalf("ohne Mitarbeiter: %v", err)
	}
}
