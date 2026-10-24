package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Stellen am Mitarbeiter pruefen ihre Eingaben, bevor sie Postgres
// anfassen (nil-Pool).
func TestEmployeePositionsValidateBeforeTouchingDatabase(t *testing.T) {
	s := NewQualificationService(nil)
	ctx := context.Background()
	emp := uuid.New()
	for _, tc := range []struct {
		name      string
		employee  uuid.UUID
		positions []uuid.UUID
		companyID string
		want      string
	}{
		{"ohne Mandant", emp, nil, " ", "Mandant erforderlich"},
		{"ohne Mitarbeiter", uuid.Nil, nil, "c1", "employee_id erforderlich"},
		{"Stelle leer", emp, []uuid.UUID{uuid.New(), uuid.Nil}, "c1", "position_id erforderlich"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.SetEmployeePositions(ctx, tc.employee, tc.positions, "", tc.companyID)
			if err == nil || err.Error() != tc.want || got != nil {
				t.Fatalf("erwartet %q, bekommen %v / %+v", tc.want, err, got)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		employee  uuid.UUID
		companyID string
		want      string
	}{
		{"ohne Mandant", emp, "", "Mandant erforderlich"},
		{"ohne Mitarbeiter", uuid.Nil, "c1", "employee_id erforderlich"},
	} {
		t.Run("Liste "+tc.name, func(t *testing.T) {
			if _, err := s.ListEmployeePositions(ctx, tc.employee, tc.companyID); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
}
