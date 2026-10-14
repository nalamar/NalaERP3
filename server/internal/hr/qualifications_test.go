package hr

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Create und Storno pruefen ihre Eingaben, bevor sie Postgres anfassen
// (nil-Pool).

func TestQualificationCreateValidatesBeforeTouchingDatabase(t *testing.T) {
	emp, typ := uuid.New(), uuid.New()
	past := time.Now().AddDate(0, -1, 0)
	future := time.Now().AddDate(0, 0, 3)
	beforePast := past.AddDate(0, 0, -1)
	valid := QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: typ, ErworbenAm: past}
	with := func(mod func(*QualificationInput)) QualificationInput {
		in := valid
		mod(&in)
		return in
	}

	cases := []struct {
		name      string
		in        QualificationInput
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", valid, " ", "Mandant erforderlich"},
		{"kein Mitarbeiter", with(func(in *QualificationInput) { in.EmployeeIDs = nil }), "default", "Mindestens ein Mitarbeiter erforderlich"},
		{"leere Mitarbeiter-ID in der Liste", with(func(in *QualificationInput) { in.EmployeeIDs = []uuid.UUID{emp, uuid.Nil} }), "default", "employee_id erforderlich"},
		{"Art fehlt", with(func(in *QualificationInput) { in.TypeID = uuid.Nil }), "default", "type_id erforderlich"},
		{"Erwerbsdatum fehlt", with(func(in *QualificationInput) { in.ErworbenAm = time.Time{} }), "default", "Erwerbsdatum erforderlich"},
		{"Erwerb in der Zukunft", with(func(in *QualificationInput) { in.ErworbenAm = future }), "default", "Erwerbsdatum darf nicht in der Zukunft liegen"},
		{"gueltig bis vor Erwerb", with(func(in *QualificationInput) { in.GueltigBis = &beforePast }), "default", "Gültig-bis darf nicht vor dem Erwerbsdatum liegen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewQualificationService(nil).Create(context.Background(), tc.in, "", tc.companyID)
			if got != nil {
				t.Fatalf("erwartet kein Ergebnis, bekommen %+v", got)
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

func TestQualificationStornoAndListValidateBeforeTouchingDatabase(t *testing.T) {
	s := NewQualificationService(nil)
	ctx := context.Background()
	id := uuid.New()

	cases := []struct {
		name      string
		id        uuid.UUID
		reason    string
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", id, "Falsches Datum", "", "Mandant erforderlich"},
		{"ID fehlt", uuid.Nil, "Falsches Datum", "default", "Nachweis-ID erforderlich"},
		{"Begruendung fehlt", id, "", "default", "Storno-Begründung erforderlich"},
		{"Begruendung nur Leerzeichen", id, " \t ", "default", "Storno-Begründung erforderlich"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Storno(ctx, tc.id, tc.reason, "", tc.companyID); err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
	if _, err := s.ListForEmployee(ctx, id, false, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Liste: erwartet 'Mandant erforderlich', bekommen %v", err)
	}
	if _, err := s.ListForEmployee(ctx, uuid.Nil, false, "default"); err == nil || err.Error() != "employee_id erforderlich" {
		t.Fatalf("Liste: erwartet 'employee_id erforderlich', bekommen %v", err)
	}
}

func TestQualificationRequirementsAndStandingValidateBeforeTouchingDatabase(t *testing.T) {
	s := NewQualificationService(nil)
	ctx := context.Background()
	emp, typ := uuid.New(), uuid.New()

	if _, err := s.SetRequirements(ctx, emp, []uuid.UUID{typ}, "", " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("SetRequirements: erwartet 'Mandant erforderlich', bekommen %v", err)
	}
	if _, err := s.SetRequirements(ctx, uuid.Nil, []uuid.UUID{typ}, "", "default"); err == nil || err.Error() != "employee_id erforderlich" {
		t.Fatalf("SetRequirements: erwartet 'employee_id erforderlich', bekommen %v", err)
	}
	if _, err := s.SetRequirements(ctx, emp, []uuid.UUID{typ, uuid.Nil}, "", "default"); err == nil || err.Error() != "type_id erforderlich" {
		t.Fatalf("SetRequirements: erwartet 'type_id erforderlich', bekommen %v", err)
	}
	if _, err := s.ListRequirements(ctx, emp, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ListRequirements: erwartet 'Mandant erforderlich', bekommen %v", err)
	}
	if _, err := s.ListRequirements(ctx, uuid.Nil, "default"); err == nil || err.Error() != "employee_id erforderlich" {
		t.Fatalf("ListRequirements: erwartet 'employee_id erforderlich', bekommen %v", err)
	}
	if _, err := s.Standing(ctx, emp, time.Now(), 60, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Standing: erwartet 'Mandant erforderlich', bekommen %v", err)
	}
	if _, err := s.Standing(ctx, uuid.Nil, time.Now(), 60, "default"); err == nil || err.Error() != "employee_id erforderlich" {
		t.Fatalf("Standing: erwartet 'employee_id erforderlich', bekommen %v", err)
	}
}

func TestQualificationDueListValidatesBeforeTouchingDatabase(t *testing.T) {
	if _, err := NewQualificationService(nil).DueList(context.Background(), time.Now(), 60, " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("erwartet 'Mandant erforderlich', bekommen %v", err)
	}
}

func TestUniqueEmployeeIDsDedupesAndSorts(t *testing.T) {
	a, b, c := uuid.MustParse("00000000-0000-0000-0000-00000000000a"), uuid.MustParse("00000000-0000-0000-0000-00000000000b"), uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	got := uniqueEmployeeIDs([]uuid.UUID{c, a, b, a, c})
	if len(got) != 3 || got[0] != a || got[1] != b || got[2] != c {
		t.Fatalf("erwartet [a b c], bekommen %v", got)
	}
}

// Waechter fuer die Unveraenderbarkeit der Nachweise (ADR 0027). Nicht
// entfernen.
//
// Der Service darf genau EINEN UPDATE auf hr_employee_qualifications
// enthalten - das einmalige Stornieren, beschraenkt auf aktive Nachweise
// und auf die vier Storno-Spalten - und keinen DELETE. Wer einen weiteren
// Schreibpfad ergaenzt, soll hier anhalten und begruenden, warum er die
// Beweisfunktion eines Nachweises nicht beruehrt.
func TestQualificationServiceHasNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("qualifications.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)

	if got := strings.Count(code, "UPDATE hr_employee_qualifications"); got != 1 {
		t.Fatalf("erwartet genau 1 UPDATE auf hr_employee_qualifications (Stornieren), gefunden %d", got)
	}
	for _, want := range []string{
		"UPDATE hr_employee_qualifications SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3",
		"WHERE id=$1 AND status='active'",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("erwartetes Fragment fehlt im Service: %s", want)
		}
	}
	if strings.Contains(code, "DELETE FROM") {
		t.Fatal("Nachweise duerfen nicht geloescht werden")
	}
}
