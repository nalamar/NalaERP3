package fleet

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// Regeln der Fahrerzuordnung ohne Datenbank (Backlog G.3a.1a, G.3a.1b).

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

func TestValidateAssignmentEnd(t *testing.T) {
	today := day(2026, 5, 10)
	beginn := day(2026, 3, 1)
	for name, tc := range map[string]struct {
		ende time.Time
		want string
	}{
		"leer":          {time.Time{}, "Ende erforderlich"},
		"Zukunft":       {day(2026, 5, 11), "Ende darf nicht in der Zukunft liegen"},
		"vor Beginn":    {day(2026, 2, 28), "Ende darf nicht vor dem Beginn liegen"},
		"Tag d. Beginn": {beginn, ""},
		"heute":         {today, ""},
	} {
		err := ValidateAssignmentEnd(tc.ende, beginn, today)
		if (tc.want == "" && err != nil) || (tc.want != "" && (err == nil || err.Error() != tc.want)) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// Ende und Storno pruefen Mandant, ID und Begruendung vor Postgres.
func TestEndAndStornoValidateBeforeTouchingDatabase(t *testing.T) {
	s := NewAssignmentService(nil)
	ctx := context.Background()
	for name, err := range map[string][2]any{
		"Ende ohne Mandant":    {errOf(s.End(ctx, uuid.New(), day(2026, 1, 1), "", "")), "Mandant erforderlich"},
		"Ende ohne ID":         {errOf(s.End(ctx, uuid.Nil, day(2026, 1, 1), "", "c1")), "Zuordnungs-ID erforderlich"},
		"Storno ohne Mandant":  {errOf(s.StornoAssignment(ctx, uuid.New(), "x", "", " ")), "Mandant erforderlich"},
		"Storno ohne ID":       {errOf(s.StornoAssignment(ctx, uuid.Nil, "x", "", "c1")), "Zuordnungs-ID erforderlich"},
		"Storno ohne Begruend": {errOf(s.StornoAssignment(ctx, uuid.New(), "  ", "", "c1")), "Storno-Begründung erforderlich"},
	} {
		if err[0] != err[1] {
			t.Fatalf("%s: %v", name, err[0])
		}
	}
}

func errOf(_ *Assignment, err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Waechter: Zuordnungen werden nur ueber Ende und Storno geaendert und nie
// geloescht (Muster der Ausgaben, ADR 0024/0028).
func TestAssignmentsHaveNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("assignments.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)
	if got := strings.Count(code, "UPDATE fleet_vehicle_assignments"); got != 2 {
		t.Fatalf("erwartet genau 2 UPDATE (Ende, Storno), gefunden %d", got)
	}
	for _, want := range []string{
		"UPDATE fleet_vehicle_assignments SET ende=$2, beendet_by_user_id=$3\n\t\t WHERE id=$1 AND status='active' AND ende IS NULL",
		"UPDATE fleet_vehicle_assignments SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3\n\t\t WHERE id=$1 AND status='active'",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("erwartetes Fragment fehlt: %s", want)
		}
	}
	all := ""
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		all += string(b)
	}
	if strings.Contains(all, "DELETE FROM fleet_vehicle_assignments") || strings.Count(all, "UPDATE fleet_vehicle_assignments") != 2 {
		t.Fatal("Zuordnungen duerfen nur in assignments.go beendet oder storniert und nie geloescht werden")
	}
}
