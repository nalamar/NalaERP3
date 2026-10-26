package fleet

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Die Datenbank selbst schuetzt die Regeln der Termine (Migration 100),
// auch gegen Direkt-SQL (Backlog G.2.1).
func TestScheduleTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	vehicle, err := NewVehicleService(env.PG).Create(ctx, VehicleInput{Kennzeichen: "M-T 1", Art: ArtPkw}, company, "")
	if err != nil {
		t.Fatalf("Fahrzeug: %v", err)
	}
	ok := func(name, sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: abgelehnt: %v", name, err)
		}
	}
	bad := func(name, sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err == nil {
			t.Fatalf("%s: von der Datenbank angenommen", name)
		}
	}
	ins := `INSERT INTO fleet_vehicle_schedules (id, vehicle_id, art, bezeichnung, starttermin, start_km, intervall_monate, intervall_km) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

	hu := uuid.New()
	ok("HU am Monatsende", ins, hu, vehicle.ID, "hu", "", "2027-05-31", nil, 24, nil)
	ok("Wartung nach km", ins, uuid.New(), vehicle.ID, "wartung", "", nil, 30000, nil, 30000)
	ok("sonstige mit Bezeichnung", ins, uuid.New(), vehicle.ID, "sonstige", "Tachograph", nil, nil, 24, nil)
	// An einem zweiten Fahrzeug - am ersten gibt es schon eine HU, die
	// Eindeutigkeit wuerde den Fehler sonst verdecken.
	second, err := NewVehicleService(env.PG).Create(ctx, VehicleInput{Kennzeichen: "M-T 2", Art: ArtPkw}, company, "")
	if err != nil {
		t.Fatalf("zweites Fahrzeug: %v", err)
	}
	bad("HU nicht am Monatsende", ins, uuid.New(), second.ID, "hu", "", "2027-05-30", nil, nil, nil)
	ok("HU am Monatsende, zweites Fahrzeug", ins, uuid.New(), second.ID, "hu", "", "2027-02-28", nil, nil, nil)
	bad("zweite HU", ins, uuid.New(), vehicle.ID, "hu", "", nil, nil, nil, nil)
	bad("sonstige gleiche Bezeichnung anders geschrieben", ins, uuid.New(), vehicle.ID, "sonstige", "TACHOGRAPH", nil, nil, nil, nil)
	ok("sonstige andere Bezeichnung", ins, uuid.New(), vehicle.ID, "sonstige", "Feuerlöscher", nil, nil, nil, nil)
	bad("Art unbekannt", ins, uuid.New(), vehicle.ID, "tuev", "", nil, nil, nil, nil)
	bad("sonstige ohne Bezeichnung", ins, uuid.New(), vehicle.ID, "sonstige", " ", nil, nil, nil, nil)
	bad("UVV mit Bezeichnung", ins, uuid.New(), vehicle.ID, "uvv", "x", nil, nil, nil, nil)
	bad("Start-km 0", ins, uuid.New(), vehicle.ID, "uvv", "", nil, 0, nil, nil)
	bad("Intervall 0", ins, uuid.New(), vehicle.ID, "uvv", "", nil, nil, 0, nil)
	bad("Intervall km 0", ins, uuid.New(), vehicle.ID, "leasing", "", nil, nil, nil, 0)
	bad("Fahrzeug unbekannt", ins, uuid.New(), uuid.New(), "uvv", "", nil, nil, nil, nil)

	insC := `INSERT INTO fleet_vehicle_completions (id, schedule_id, erledigt_am, ergebnis, km_stand, naechster_termin, naechster_km, status, storno_reason, storniert_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	ok("HU bestanden", insC, uuid.New(), hu, "2026-03-10", "bestanden", 1000, "2028-03-31", nil, "active", "", nil)
	ok("storniert mit Grund", insC, uuid.New(), hu, "2026-03-11", "maengel", nil, nil, nil, "storniert", "falsch", "2026-03-12T10:00:00Z")
	bad("Ergebnis unbekannt", insC, uuid.New(), hu, "2026-03-10", "ok", nil, nil, nil, "active", "", nil)
	bad("Folgetermin am selben Tag", insC, uuid.New(), hu, "2026-03-10", "bestanden", nil, "2026-03-10", nil, "active", "", nil)
	bad("km negativ", insC, uuid.New(), hu, "2026-03-10", "bestanden", -1, nil, nil, "active", "", nil)
	bad("Folge-km nicht ueber km", insC, uuid.New(), hu, "2026-03-10", "bestanden", 5000, nil, 5000, "active", "", nil)
	bad("Folge-km 0", insC, uuid.New(), hu, "2026-03-10", "bestanden", nil, nil, 0, "active", "", nil)
	bad("storniert ohne Grund", insC, uuid.New(), hu, "2026-03-10", "bestanden", nil, nil, nil, "storniert", "", "2026-03-12T10:00:00Z")
	bad("aktiv mit Storno-Zeitpunkt", insC, uuid.New(), hu, "2026-03-10", "bestanden", nil, nil, nil, "active", "", "2026-03-12T10:00:00Z")
	bad("Status unbekannt", insC, uuid.New(), hu, "2026-03-10", "bestanden", nil, nil, nil, "geloescht", "", nil)
	bad("Termin unbekannt", insC, uuid.New(), uuid.New(), "2026-03-10", "bestanden", nil, nil, nil, "active", "", nil)

	// RESTRICT: ein Termin mit Erledigung und ein Fahrzeug mit Termin
	// lassen sich nicht loeschen.
	bad("Termin mit Erledigung loeschen", `DELETE FROM fleet_vehicle_schedules WHERE id=$1`, hu)
	bad("Fahrzeug mit Terminen loeschen", `DELETE FROM fleet_vehicles WHERE id=$1`, vehicle.ID)
}
