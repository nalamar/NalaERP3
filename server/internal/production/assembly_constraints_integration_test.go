package production

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Die Datenbank schuetzt Montagetermine (Migration 115, Backlog H.3.6a)
// auch gegen Direkt-SQL.
func TestAssemblyTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	p := seedPosition(t, env)
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
	user := testutil.SeedAuthUser(t, env, "mt-"+uuid.NewString()[:8]+"@example.com", "Passwort-123!", "production")

	order, team, vehicle, project2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ok("Auftrag", `INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl)
		VALUES ($1,$2,'FA-MT-1',$3,$4,$5,1)`, order, p.company, p.project, p.elevation, p.variant)
	ok("Team", `INSERT INTO hr_teams (id, name, company_id) VALUES ($1,'Montage 1',$2)`, team, p.company)
	ok("Fahrzeug", `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art) VALUES ($1,$2,$3,'transporter')`, vehicle, p.company, "MT-"+uuid.NewString()[:6])
	ok("Projekt ohne Auftrag", `INSERT INTO projects (id, nummer, name, company_id) VALUES ($1,'P-MT','Montageprojekt',$2)`, project2, p.company)

	ins := `INSERT INTO production_assembly_appointments (id, company_id, project_id, order_id, von, bis, team_id, vehicle_id, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	full, onlyProject := uuid.New(), uuid.New()
	ok("mit Auftrag, Team und Fahrzeug", ins, full, p.company, p.project, order, "2026-12-07", "2026-12-09", team, vehicle, user)
	ok("nur Projekt, ein Tag (Vorbereitung (b))", ins, onlyProject, p.company, project2, nil, "2026-12-07", "2026-12-07", nil, nil, nil)
	ok("60 Tage", ins, uuid.New(), p.company, p.project, order, "2026-12-07", "2027-02-04", nil, nil, nil)
	bad("61 Tage", ins, uuid.New(), p.company, p.project, order, "2026-12-07", "2027-02-05", nil, nil, nil)
	bad("bis vor von", ins, uuid.New(), p.company, p.project, order, "2026-12-07", "2026-12-06", nil, nil, nil)
	bad("ohne von", ins, uuid.New(), p.company, p.project, order, nil, "2026-12-06", nil, nil, nil)
	bad("ohne Projekt", ins, uuid.New(), p.company, nil, order, "2026-12-07", "2026-12-07", nil, nil, nil)
	bad("unbekanntes Projekt", ins, uuid.New(), p.company, uuid.New(), nil, "2026-12-07", "2026-12-07", nil, nil, nil)
	bad("unbekannter Auftrag", ins, uuid.New(), p.company, p.project, uuid.New(), "2026-12-07", "2026-12-07", nil, nil, nil)
	bad("unbekanntes Team", ins, uuid.New(), p.company, p.project, order, "2026-12-07", "2026-12-07", uuid.New(), nil, nil)
	bad("unbekanntes Fahrzeug", ins, uuid.New(), p.company, p.project, order, "2026-12-07", "2026-12-07", nil, uuid.New(), nil)
	bad("unbekannter Mandant", ins, uuid.New(), "kein-mandant", p.project, order, "2026-12-07", "2026-12-07", nil, nil, nil)
	bad("bis vor von per UPDATE", `UPDATE production_assembly_appointments SET bis='2026-12-01' WHERE id=$1`, full)

	// Storno nur vollstaendig.
	storno := `UPDATE production_assembly_appointments SET storniert_at=$2, storno_reason=$3, storniert_by_user_id=$4 WHERE id=$1`
	bad("Storno ohne Begruendung", storno, onlyProject, "2026-11-12T10:00:00Z", "  ", user)
	bad("Begruendung ohne Zeitpunkt", storno, onlyProject, nil, "verschoben", nil)
	bad("Benutzer ohne Storno", storno, onlyProject, nil, "", user)
	ok("Storno vollstaendig", storno, onlyProject, "2026-11-12T10:00:00Z", "verschoben", user)
	ok("Storno ohne Benutzer", storno, onlyProject, "2026-11-12T10:00:00Z", "verschoben", nil)

	// Auftrag und Projekt mit Termin nicht loeschbar; Team und Fahrzeug
	// loeschen den Bezug.
	bad("Auftrag loeschen", `DELETE FROM production_orders WHERE id=$1`, order)
	bad("Projekt loeschen", `DELETE FROM projects WHERE id=$1`, project2)
	ok("Team loeschen", `DELETE FROM hr_teams WHERE id=$1`, team)
	ok("Fahrzeug loeschen", `DELETE FROM fleet_vehicles WHERE id=$1`, vehicle)
	var teamID, vehicleID *uuid.UUID
	if err := env.PG.QueryRow(ctx, `SELECT team_id, vehicle_id FROM production_assembly_appointments WHERE id=$1`, full).Scan(&teamID, &vehicleID); err != nil ||
		teamID != nil || vehicleID != nil {
		t.Fatalf("nach dem Loeschen: Team %v, Fahrzeug %v, %v", teamID, vehicleID, err)
	}
}
