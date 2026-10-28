package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Die Datenbank schuetzt die Regeln des Fahrtenbuchs (Migration 103),
// auch gegen Direkt-SQL (Backlog G.3c.1).
func TestTripTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-FB 1")
	emp := seedDriver(t, env, company, "Fahrer", true)
	ins := `INSERT INTO fleet_trips (id, vehicle_id, employee_id, datum, km_beginn, km_ende, fahrtart, ziel, zweck, status, storno_reason, storniert_at)
		VALUES ($1,$2,$3,'2026-03-01',$4,$5,$6,$7,$8,$9,$10,$11)`
	trip := func(beginn, ende int, art, ziel, zweck, status, reason string, at any) error {
		_, err := env.PG.Exec(ctx, ins, uuid.New(), v.ID, emp, beginn, ende, art, ziel, zweck, status, reason, at)
		return err
	}
	ok := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: abgelehnt: %v", name, err)
		}
	}
	bad := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: von der Datenbank angenommen", name)
		}
	}
	ok("dienstlich", trip(1000, 1100, "dienstlich", "Ulm", "Montage", "active", "", nil))
	ok("privat ohne Ziel", trip(1100, 1150, "privat", "", "", "active", "", nil))
	ok("Arbeitsweg", trip(1150, 1170, "arbeitsweg", "", "", "active", "", nil))
	bad("gleicher Beginn", trip(1000, 1020, "privat", "", "", "active", "", nil))
	bad("gleiches Ende", trip(1090, 1100, "privat", "", "", "active", "", nil))
	ok("gleicher Beginn wie eine stornierte", trip(1000, 1100, "privat", "", "", "storniert", "falsch", time.Now()))
	bad("Ende gleich Beginn", trip(2000, 2000, "privat", "", "", "active", "", nil))
	bad("km negativ", trip(-1, 10, "privat", "", "", "active", "", nil))
	bad("Fahrtart unbekannt", trip(3000, 3010, "urlaub", "", "", "active", "", nil))
	bad("dienstlich ohne Ziel", trip(3000, 3010, "dienstlich", " ", "Montage", "active", "", nil))
	bad("dienstlich ohne Zweck", trip(3000, 3010, "dienstlich", "Ulm", "", "active", "", nil))
	bad("Status unbekannt", trip(3000, 3010, "privat", "", "", "geloescht", "", nil))
	bad("Storno ohne Grund", trip(3000, 3010, "privat", "", "", "storniert", " ", time.Now()))
	bad("Storno ohne Zeitpunkt", trip(3000, 3010, "privat", "", "", "storniert", "falsch", nil))
	bad("aktiv mit Stornodatum", trip(3000, 3010, "privat", "", "", "active", "", time.Now()))
	if _, err := env.PG.Exec(ctx, ins, uuid.New(), uuid.New(), emp, 5000, 5010, "privat", "", "", "active", "", nil); err == nil {
		t.Fatal("Fahrzeug unbekannt angenommen")
	}
	var created time.Time
	if err := env.PG.QueryRow(ctx, `SELECT created_at FROM fleet_trips WHERE vehicle_id=$1 AND km_beginn=1000 AND status='active'`, v.ID).Scan(&created); err != nil || time.Since(created) > time.Minute {
		t.Fatalf("Erfassungszeitpunkt: %v %v", created, err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM fleet_vehicles WHERE id=$1`, v.ID); err == nil {
		t.Fatal("Fahrzeug mit Fahrten geloescht")
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM hr_employees WHERE id=$1`, emp); err == nil {
		t.Fatal("Fahrer mit Fahrten geloescht")
	}
}
