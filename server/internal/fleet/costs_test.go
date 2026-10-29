package fleet

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Kosten-Service ohne Datenbank (Backlog G.3d.1).

func errCost(_ *Cost, err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Erfassen und Storno pruefen Mandant, IDs, Eingabe und Begruendung vor
// Postgres (nil-Pool).
func TestCostServiceValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewCostService(nil)
	ctx := context.Background()
	in := CostInput{VehicleID: uuid.New(), Datum: day(2026, 1, 1), Kostenart: KostenartWartung, BetragNetto: 10}
	noVehicle, badInput := in, in
	noVehicle.VehicleID = uuid.Nil
	badInput.BetragNetto = 0
	for name, tc := range map[string][2]string{
		"ohne Mandant":         {errCost(s.RecordCost(ctx, in, "", " ")), "Mandant erforderlich"},
		"ohne Fahrzeug":        {errCost(s.RecordCost(ctx, noVehicle, "", "c1")), "vehicle_id erforderlich"},
		"ungueltig":            {errCost(s.RecordCost(ctx, badInput, "", "c1")), "Betrag muss größer als 0 sein"},
		"Storno ohne Mandant":  {errCost(s.StornoCost(ctx, uuid.New(), "x", "", "")), "Mandant erforderlich"},
		"Storno ohne ID":       {errCost(s.StornoCost(ctx, uuid.Nil, "x", "", "c1")), "Kosten-ID erforderlich"},
		"Storno ohne Begruend": {errCost(s.StornoCost(ctx, uuid.New(), "  ", "", "c1")), "Storno-Begründung erforderlich"},
	} {
		if tc[0] != tc[1] {
			t.Fatalf("%s: %q", name, tc[0])
		}
	}
}

// Waechter: geschlossene Form - Kosten werden nur storniert, nie geaendert
// und nie geloescht.
func TestCostsHaveNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("costs.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)
	want := "UPDATE fleet_costs c SET status='storniert', storniert_at=now(), storno_reason=$3, storniert_by_user_id=$4"
	if strings.Count(code, "UPDATE fleet_costs") != 1 || !strings.Contains(code, want) || !strings.Contains(code, "c.status='active'`,\n\t\tid, companyID, reason") {
		t.Fatal("costs.go: erwartet genau ein UPDATE, das Storno nicht stornierter Kosten")
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
	if strings.Contains(all, "DELETE FROM fleet_costs") || strings.Count(all, "UPDATE fleet_costs") != 1 {
		t.Fatal("Kosten duerfen nur in costs.go storniert und nie geaendert oder geloescht werden")
	}
}
