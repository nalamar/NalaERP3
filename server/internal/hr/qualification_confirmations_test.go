package hr

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Zwischenbestaetigungen (Backlog F.10.3.2b): Regeln und Eingabepruefung
// ohne Datenbank (nil-Pool).

func TestValidateQualificationConfirmation(t *testing.T) {
	erworben := qDay(t, "2026-01-15")
	gueltigBis := qDayPtr(t, "2029-01-15")
	today := qDay(t, "2026-10-27")
	cases := []struct {
		name       string
		am         string
		durch      string
		gueltigBis bool
		wantErr    string
	}{
		{"gueltig", "2026-07-10", "Schweissaufsicht", true, ""},
		{"am Erwerbstag", "2026-01-15", "Schweissaufsicht", true, ""},
		{"heute", "2026-10-27", "Schweissaufsicht", true, ""},
		{"unbefristeter Nachweis", "2026-07-10", "Schweissaufsicht", false, ""},
		{"Datum fehlt", "", "Schweissaufsicht", true, "Bestätigungsdatum erforderlich"},
		{"durch fehlt", "2026-07-10", " \t", true, "Bestätigt durch erforderlich"},
		{"in der Zukunft", "2026-10-28", "Schweissaufsicht", true, "Bestätigungsdatum darf nicht in der Zukunft liegen"},
		{"vor dem Erwerb", "2026-01-14", "Schweissaufsicht", true, "Bestätigungsdatum darf nicht vor dem Erwerbsdatum liegen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := QualificationConfirmationInput{BestaetigtDurch: tc.durch}
			if tc.am != "" {
				in.BestaetigtAm = qDay(t, tc.am)
			}
			gb := gueltigBis
			if !tc.gueltigBis {
				gb = nil
			}
			err := ValidateQualificationConfirmation(in, erworben, gb, today)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("erwartet ok, bekommen %v", err)
			}
			if tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr) {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
	// Nach dem Ablauf des Zertifikats: neuer Nachweis statt Bestaetigung.
	short := qDayPtr(t, "2026-07-15")
	if err := ValidateQualificationConfirmation(QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-15"), BestaetigtDurch: "x"}, erworben, short, today); err != nil {
		t.Fatalf("am Ablauftag: %v", err)
	}
	if err := ValidateQualificationConfirmation(QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-16"), BestaetigtDurch: "x"}, erworben, short, today); err == nil || err.Error() != "Bestätigungsdatum darf nicht nach dem Gültig-bis liegen" {
		t.Fatalf("nach Ablauf: %v", err)
	}
}

func TestQualificationConfirmationsValidateBeforeTouchingDatabase(t *testing.T) {
	s := NewQualificationService(nil)
	ctx := context.Background()
	id := uuid.New()
	valid := QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-10"), BestaetigtDurch: "Schweissaufsicht"}

	create := []struct {
		name      string
		id        uuid.UUID
		in        QualificationConfirmationInput
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", id, valid, " ", "Mandant erforderlich"},
		{"Nachweis fehlt", uuid.Nil, valid, "default", "Nachweis-ID erforderlich"},
		{"Datum fehlt", id, QualificationConfirmationInput{BestaetigtDurch: "Schweissaufsicht"}, "default", "Bestätigungsdatum erforderlich"},
		{"durch fehlt", id, QualificationConfirmationInput{BestaetigtAm: valid.BestaetigtAm}, "default", "Bestätigt durch erforderlich"},
	}
	for _, tc := range create {
		t.Run("Erfassen "+tc.name, func(t *testing.T) {
			got, err := s.CreateConfirmation(ctx, tc.id, tc.in, "", tc.companyID)
			if got != nil || err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %+v %v", tc.wantErr, got, err)
			}
		})
	}

	storno := []struct {
		name      string
		id        uuid.UUID
		reason    string
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", id, "Falsches Datum", "", "Mandant erforderlich"},
		{"ID fehlt", uuid.Nil, "Falsches Datum", "default", "Bestätigungs-ID erforderlich"},
		{"Begruendung nur Leerzeichen", id, " \t ", "default", "Storno-Begründung erforderlich"},
	}
	for _, tc := range storno {
		t.Run("Storno "+tc.name, func(t *testing.T) {
			if _, err := s.StornoConfirmation(ctx, tc.id, tc.reason, "", tc.companyID); err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}

	if _, err := s.ListConfirmations(ctx, id, false, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Liste: %v", err)
	}
	if _, err := s.ListConfirmations(ctx, uuid.Nil, false, "default"); err == nil || err.Error() != "Nachweis-ID erforderlich" {
		t.Fatalf("Liste: %v", err)
	}
}

// Waechter fuer die Unveraenderbarkeit der Bestaetigungen (wie
// TestQualificationServiceHasNoOtherWritePaths). Nicht entfernen.
func TestQualificationConfirmationsHaveNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("qualification_confirmations.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)
	if got := strings.Count(code, "UPDATE hr_qualification_confirmations"); got != 1 {
		t.Fatalf("erwartet genau 1 UPDATE auf hr_qualification_confirmations (Stornieren), gefunden %d", got)
	}
	for _, want := range []string{
		"UPDATE hr_qualification_confirmations SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3",
		"WHERE id=$1 AND status='active'",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("erwartetes Fragment fehlt: %s", want)
		}
	}
	if strings.Contains(code, "DELETE FROM") || strings.Contains(code, "UPDATE hr_employee_qualifications") {
		t.Fatal("Bestaetigungen duerfen nicht geloescht werden und Nachweise nicht veraendern")
	}
}
