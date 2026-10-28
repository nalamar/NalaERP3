package fleet

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Fahrtenbuch-Service ohne Datenbank (Backlog G.3c.2).

func errTrip(_ *Trip, err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Erfassen und Storno pruefen Mandant, IDs und Begruendung vor Postgres
// (nil-Pool).
func TestTripServiceValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewTripService(nil)
	ctx := context.Background()
	in := TripInput{VehicleID: uuid.New(), EmployeeID: uuid.New(), Datum: day(2026, 1, 1), KmBeginn: 1, KmEnde: 2, Fahrtart: FahrtartPrivat}
	noVehicle, noEmployee := in, in
	noVehicle.VehicleID = uuid.Nil
	noEmployee.EmployeeID = uuid.Nil
	for name, tc := range map[string][2]string{
		"ohne Mandant":         {errTrip(s.RecordTrip(ctx, in, "", " ")), "Mandant erforderlich"},
		"ohne Fahrzeug":        {errTrip(s.RecordTrip(ctx, noVehicle, "", "c1")), "vehicle_id erforderlich"},
		"ohne Mitarbeiter":     {errTrip(s.RecordTrip(ctx, noEmployee, "", "c1")), "employee_id erforderlich"},
		"Storno ohne Mandant":  {errTrip(s.StornoTrip(ctx, uuid.New(), "x", "", "")), "Mandant erforderlich"},
		"Storno ohne ID":       {errTrip(s.StornoTrip(ctx, uuid.Nil, "x", "", "c1")), "Fahrt-ID erforderlich"},
		"Storno ohne Begruend": {errTrip(s.StornoTrip(ctx, uuid.New(), "  ", "", "c1")), "Storno-Begründung erforderlich"},
	} {
		if tc[0] != tc[1] {
			t.Fatalf("%s: %q", name, tc[0])
		}
	}
}

// Waechter: geschlossene Form - Fahrten werden nur storniert, nie
// geaendert und nie geloescht.
func TestTripsHaveNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("trips.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)
	want := "UPDATE fleet_trips SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3\n\t\t WHERE id=$1 AND status='active'"
	if strings.Count(code, "UPDATE fleet_trips") != 1 || !strings.Contains(code, want) {
		t.Fatal("trips.go: erwartet genau ein UPDATE, das Storno")
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
	if strings.Contains(all, "DELETE FROM fleet_trips") || strings.Count(all, "UPDATE fleet_trips") != 1 {
		t.Fatal("Fahrten duerfen nur in trips.go storniert und nie geaendert oder geloescht werden")
	}
}
