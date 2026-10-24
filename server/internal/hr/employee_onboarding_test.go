package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// CreateEmployeeWithQualifications prueft Mandant, Anforderungen und
// Nachweise, bevor es Postgres anfasst (nil-Pool) - ein Fehler hier
// darf nicht erst nach angelegtem Mitarbeiter auffallen.
func TestEmployeeWithQualificationsValidatesBeforeTouchingDatabase(t *testing.T) {
	typ := uuid.New()
	past := time.Now().AddDate(0, -1, 0)
	future := time.Now().AddDate(0, 0, 3)
	emp := EmployeeInput{FirstName: "Neu", LastName: "Person"}
	cases := []struct {
		name      string
		in        EmployeeWithQualificationsInput
		companyID string
		want      string
	}{
		{"ohne Mandant", EmployeeWithQualificationsInput{Employee: emp}, " ", "Mandant erforderlich"},
		{"Anforderung ohne Art", EmployeeWithQualificationsInput{Employee: emp, RequiredTypeIDs: []uuid.UUID{typ, uuid.Nil}}, "c1", "type_id erforderlich"},
		{"Stelle leer", EmployeeWithQualificationsInput{Employee: emp, PositionIDs: []uuid.UUID{uuid.New(), uuid.Nil}}, "c1", "position_id erforderlich"},
		{"Nachweis ohne Art", EmployeeWithQualificationsInput{Employee: emp, Qualifications: []InitialQualification{{ErworbenAm: past}}}, "c1", "type_id erforderlich"},
		{"Nachweis ohne Datum", EmployeeWithQualificationsInput{Employee: emp, Qualifications: []InitialQualification{{TypeID: typ}}}, "c1", "Erwerbsdatum erforderlich"},
		{"Nachweis in der Zukunft", EmployeeWithQualificationsInput{Employee: emp, Qualifications: []InitialQualification{
			{TypeID: typ, ErworbenAm: past},
			{TypeID: typ, ErworbenAm: future},
		}}, "c1", "Erwerbsdatum darf nicht in der Zukunft liegen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewQualificationService(nil).CreateEmployeeWithQualifications(context.Background(), tc.in, "", tc.companyID)
			if err == nil || err.Error() != tc.want || got != nil {
				t.Fatalf("erwartet %q, bekommen %v / %+v", tc.want, err, got)
			}
		})
	}
}

func TestInitialQualificationInputCarriesAllFields(t *testing.T) {
	emp, typ := uuid.New(), uuid.New()
	erworben := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	bis := time.Date(2028, 3, 15, 0, 0, 0, 0, time.UTC)
	got := InitialQualification{TypeID: typ, ErworbenAm: erworben, GueltigBis: &bis, Aussteller: "DRK", NachweisNr: "N-1", Note: "x"}.input(emp)
	if len(got.EmployeeIDs) != 1 || got.EmployeeIDs[0] != emp || got.TypeID != typ || !got.ErworbenAm.Equal(erworben) ||
		got.GueltigBis == nil || !got.GueltigBis.Equal(bis) || got.Aussteller != "DRK" || got.NachweisNr != "N-1" || got.Note != "x" {
		t.Fatalf("Felder nicht uebernommen: %+v", got)
	}
}
