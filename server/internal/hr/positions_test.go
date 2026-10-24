package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Stellen pruefen ihre Eingaben, bevor sie Postgres anfassen (nil-Pool).

func TestPositionServiceValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewPositionService(nil)
	ctx := context.Background()
	typ := uuid.New()
	for _, tc := range []struct {
		name      string
		in        PositionInput
		companyID string
		want      string
	}{
		{"ohne Mandant", PositionInput{Code: "M", Name: "M"}, " ", "Mandant erforderlich"},
		{"Code fehlt", PositionInput{Code: "  ", Name: "M"}, "c1", "Code erforderlich"},
		{"Code ungueltig", PositionInput{Code: "MÖ", Name: "M"}, "c1", "Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)"},
		{"Name fehlt", PositionInput{Code: "M", Name: " "}, "c1", "Name erforderlich"},
		{"Art leer", PositionInput{Code: "M", Name: "M", TypeIDs: []uuid.UUID{typ, uuid.Nil}}, "c1", "type_id erforderlich"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Create(ctx, tc.in, tc.companyID)
			if err == nil || err.Error() != tc.want || got != nil {
				t.Fatalf("erwartet %q, bekommen %v / %+v", tc.want, err, got)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		in        PositionUpdate
		companyID string
		want      string
	}{
		{"ohne Mandant", PositionUpdate{Name: "M"}, "", "Mandant erforderlich"},
		{"Name fehlt", PositionUpdate{Name: ""}, "c1", "Name erforderlich"},
		{"Art leer", PositionUpdate{Name: "M", TypeIDs: []uuid.UUID{uuid.Nil}}, "c1", "type_id erforderlich"},
	} {
		t.Run("Update "+tc.name, func(t *testing.T) {
			got, err := s.Update(ctx, uuid.New(), tc.in, tc.companyID)
			if err == nil || err.Error() != tc.want || got != nil {
				t.Fatalf("erwartet %q, bekommen %v / %+v", tc.want, err, got)
			}
		})
	}
}

func TestNormalizePositionTypeIDs(t *testing.T) {
	a := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	b := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	got, err := normalizePositionTypeIDs([]uuid.UUID{b, a, b})
	if err != nil || len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("bekommen %v, %v", got, err)
	}
	if got, err := normalizePositionTypeIDs(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("leer: %v, %v", got, err)
	}
}
