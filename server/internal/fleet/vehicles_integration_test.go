package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Fahrzeugstamm gegen echtes Postgres (Backlog G.1.2). Jeder Test in
// einem eigenen Mandanten: Kennzeichen und FIN sind je Mandant eindeutig.

func seedCompany(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	id := "fl-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Fuhrparktest "+id); err != nil {
		t.Fatalf("Mandant anlegen: %v", err)
	}
	return id
}

func seedCostCenter(t *testing.T, env *testutil.IntegrationEnv, companyID string) string {
	t.Helper()
	id := "cc-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO cost_centers (id, company_id, code, name) VALUES ($1, $2, $3, 'Montage')`, id, companyID, id); err != nil {
		t.Fatalf("Kostenstelle anlegen: %v", err)
	}
	return id
}

func today() time.Time { return hr.WorkDateOf(time.Now()) }

func TestVehicleCreateGetListUpdateFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewVehicleService(env.PG)
	company := seedCompany(t, env)
	cc := seedCostCenter(t, env, company)
	actor := testutil.SeedAuthUser(t, env, "fleet-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")

	v, err := svc.Create(ctx, VehicleInput{
		Kennzeichen: " m - ab  1234 ", Art: ArtTransporter, Bezeichnung: " Montagebus ", Hersteller: "VW", Modell: "Crafter",
		FIN: strPtr(" wvwzzz1jz3w386752 "), Erstzulassung: datePtr(2021, 3, 15), ZulaessigeGesamtmasseKg: intPtr(3500),
		Standort: "Hof", KostenstelleID: strPtr(cc), Fahrtenbuch: true, Notiz: "Dachträger",
	}, company, actor)
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	if v.Kennzeichen != "M-AB 1234" || v.Bezeichnung != "Montagebus" || v.FIN == nil || *v.FIN != "WVWZZZ1JZ3W386752" ||
		v.Erstzulassung == nil || !v.Erstzulassung.Equal(day(2021, 3, 15)) || v.ZulaessigeGesamtmasseKg == nil || *v.ZulaessigeGesamtmasseKg != 3500 ||
		v.KostenstelleID == nil || *v.KostenstelleID != cc || !v.Fahrtenbuch || v.AbgemeldetAm != nil {
		t.Fatalf("angelegt: %+v", v)
	}
	var createdBy *string
	if err := env.PG.QueryRow(ctx, `SELECT created_by_user_id FROM fleet_vehicles WHERE id=$1`, v.ID).Scan(&createdBy); err != nil || createdBy == nil || *createdBy != actor {
		t.Fatalf("Urheber: %v %v", createdBy, err)
	}
	anh, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 99", Art: ArtAnhaenger}, company, "")
	if err != nil || anh.FIN != nil || anh.Erstzulassung != nil || anh.Fahrtenbuch {
		t.Fatalf("Anhaenger ohne Optionales: %+v %v", anh, err)
	}

	got, err := svc.Get(ctx, v.ID, company)
	if err != nil || got.Kennzeichen != "M-AB 1234" {
		t.Fatalf("Einzelabruf: %+v %v", got, err)
	}
	list, err := svc.List(ctx, company, ListFilter{})
	if err != nil || len(list) != 2 || list[0].ID != v.ID || list[1].ID != anh.ID {
		t.Fatalf("Liste nach Kennzeichen: %+v %v", list, err)
	}
	if l, _ := svc.List(ctx, company, ListFilter{Art: ArtAnhaenger}); len(l) != 1 || l[0].ID != anh.ID {
		t.Fatalf("Liste nach Art: %+v", l)
	}

	// Aendern ist vollstaendiger Ersatz: weggelassene Angaben werden
	// entfernt; Umkennzeichnung erlaubt.
	upd, err := svc.Update(ctx, v.ID, VehicleInput{Kennzeichen: "m-xy 1", Art: ArtTransporter, Bezeichnung: "Bus"}, company)
	if err != nil {
		t.Fatalf("Aendern: %v", err)
	}
	if upd.Kennzeichen != "M-XY 1" || upd.FIN != nil || upd.KostenstelleID != nil || upd.Fahrtenbuch || upd.Notiz != "" || upd.Erstzulassung != nil {
		t.Fatalf("geaendert: %+v", upd)
	}
}

func TestVehicleUniquenessAndReuseAfterDeregistration(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewVehicleService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)

	first, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, FIN: strPtr("WVWZZZ1JZ3W386752")}, company, "")
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	second, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 2", Art: ArtPkw}, company, "")
	if err != nil {
		t.Fatalf("zweites Fahrzeug: %v", err)
	}
	// Gleiches Kennzeichen, andere Schreibweise.
	if _, err := svc.Create(ctx, VehicleInput{Kennzeichen: "m-ab  1", Art: ArtLkw}, company, ""); err == nil || err.Error() != "Fahrzeug mit diesem Kennzeichen existiert bereits" {
		t.Fatalf("doppeltes Kennzeichen: %v", err)
	}
	if _, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 3", Art: ArtPkw, FIN: strPtr("wvwzzz1jz3w386752")}, company, ""); err == nil || err.Error() != "Fahrzeug mit dieser FIN existiert bereits" {
		t.Fatalf("doppelte FIN: %v", err)
	}
	if _, err := svc.Update(ctx, second.ID, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw}, company); err == nil || err.Error() != "Fahrzeug mit diesem Kennzeichen existiert bereits" {
		t.Fatalf("Umkennzeichnung auf vorhandenes: %v", err)
	}
	// Anderer Mandant: dasselbe Kennzeichen und dieselbe FIN sind erlaubt.
	if _, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, FIN: strPtr("WVWZZZ1JZ3W386752")}, other, ""); err != nil {
		t.Fatalf("anderer Mandant: %v", err)
	}

	// Nach der Abmeldung ist das Kennzeichen frei, die FIN nicht.
	if _, err := svc.Deregister(ctx, first.ID, today(), "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	reuse, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtTransporter}, company, "")
	if err != nil {
		t.Fatalf("Kennzeichen nach Abmeldung: %v", err)
	}
	if _, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 4", Art: ArtPkw, FIN: strPtr("WVWZZZ1JZ3W386752")}, company, ""); err == nil || err.Error() != "Fahrzeug mit dieser FIN existiert bereits" {
		t.Fatalf("FIN nach Abmeldung: %v", err)
	}
	old, err := svc.Get(ctx, first.ID, company)
	if err != nil || old.Kennzeichen != "M-AB 1" || reuse.ID == first.ID {
		t.Fatalf("altes Fahrzeug behaelt sein Kennzeichen: %+v %v", old, err)
	}
}

func TestVehicleDeregistration(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewVehicleService(env.PG)
	company := seedCompany(t, env)
	v, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, Erstzulassung: datePtr(2020, 6, 1)}, company, "")
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	for _, tc := range []struct {
		name  string
		datum time.Time
		grund string
		want  string
	}{
		{"ohne Grund", today(), " ", "Grund der Abmeldung erforderlich"},
		{"in der Zukunft", today().AddDate(0, 0, 1), "x", "Abmeldedatum darf nicht in der Zukunft liegen"},
		{"vor Erstzulassung", day(2020, 5, 31), "x", "Abmeldedatum darf nicht vor der Erstzulassung liegen"},
	} {
		if _, err := svc.Deregister(ctx, v.ID, tc.datum, tc.grund, company); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if still, _ := svc.Get(ctx, v.ID, company); still.AbgemeldetAm != nil {
		t.Fatalf("abgelehnte Abmeldung gespeichert: %+v", still)
	}
	out, err := svc.Deregister(ctx, v.ID, day(2024, 2, 29), " Totalschaden ", company)
	if err != nil || out.AbgemeldetAm == nil || !out.AbgemeldetAm.Equal(day(2024, 2, 29)) || out.AbgemeldetGrund != "Totalschaden" {
		t.Fatalf("abgemeldet: %+v %v", out, err)
	}
	if _, err := svc.Deregister(ctx, v.ID, today(), "nochmal", company); err == nil || err.Error() != "Fahrzeug ist bereits abgemeldet" {
		t.Fatalf("zweite Abmeldung: %v", err)
	}
	// Abgemeldete nur auf Wunsch in der Liste; berichtigen bleibt moeglich,
	// die Abmeldung bleibt.
	if l, _ := svc.List(ctx, company, ListFilter{}); len(l) != 0 {
		t.Fatalf("abgemeldetes in aktiver Liste: %+v", l)
	}
	if l, _ := svc.List(ctx, company, ListFilter{IncludeAbgemeldete: true}); len(l) != 1 {
		t.Fatalf("mit abgemeldeten: %+v", l)
	}
	upd, err := svc.Update(ctx, v.ID, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, Bezeichnung: "berichtigt", Erstzulassung: datePtr(2020, 6, 1)}, company)
	if err != nil || upd.Bezeichnung != "berichtigt" || upd.AbgemeldetAm == nil {
		t.Fatalf("Berichtigung nach Abmeldung: %+v %v", upd, err)
	}
	if _, err := svc.Update(ctx, v.ID, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, Erstzulassung: datePtr(2024, 3, 1)}, company); err == nil || err.Error() != "Erstzulassung darf nicht nach der Abmeldung liegen" {
		t.Fatalf("Erstzulassung nach Abmeldung: %v", err)
	}
}

func TestVehicleTenantIsolationAndKostenstelle(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewVehicleService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	foreignCC := seedCostCenter(t, env, other)

	if _, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, KostenstelleID: strPtr(foreignCC)}, company, ""); err == nil || err.Error() != "Kostenstelle nicht gefunden" {
		t.Fatalf("fremde Kostenstelle: %v", err)
	}
	v, err := svc.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw}, company, "")
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	if _, err := svc.Update(ctx, v.ID, VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw, KostenstelleID: strPtr(foreignCC)}, company); err == nil || err.Error() != "Kostenstelle nicht gefunden" {
		t.Fatalf("fremde Kostenstelle beim Aendern: %v", err)
	}
	for name, err := range map[string]error{
		"Get": func() error { _, e := svc.Get(ctx, v.ID, other); return e }(),
		"Update": func() error {
			_, e := svc.Update(ctx, v.ID, VehicleInput{Kennzeichen: "X-Y 1", Art: ArtPkw}, other)
			return e
		}(),
		"Deregister": func() error { _, e := svc.Deregister(ctx, v.ID, today(), "x", other); return e }(),
		"unbekannt":  func() error { _, e := svc.Get(ctx, uuid.New(), company); return e }(),
	} {
		if err == nil || err.Error() != "Fahrzeug nicht gefunden" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if l, _ := svc.List(ctx, other, ListFilter{IncludeAbgemeldete: true}); len(l) != 0 {
		t.Fatalf("fremde Fahrzeuge sichtbar: %+v", l)
	}
	if still, _ := svc.Get(ctx, v.ID, company); still.Kennzeichen != "M-AB 1" || still.AbgemeldetAm != nil {
		t.Fatalf("durch fremden Mandanten veraendert: %+v", still)
	}
}

// Die Datenbank selbst schuetzt die Regeln, auch gegen Direkt-SQL.
func TestVehicleTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	for name, sql := range map[string]string{
		"Art Stapler":          `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art) VALUES ($1,$2,'M-1','stapler')`,
		"leeres Kennzeichen":   `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art) VALUES ($1,$2,'  ','pkw')`,
		"Abmeldung ohne Grund": `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art, abgemeldet_am) VALUES ($1,$2,'M-2','pkw','2026-01-01')`,
		"Grund ohne Abmeldung": `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art, abgemeldet_grund) VALUES ($1,$2,'M-3','pkw','x')`,
		"Masse 0":              `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art, zulaessige_gesamtmasse_kg) VALUES ($1,$2,'M-4','pkw',0)`,
	} {
		if _, err := env.PG.Exec(ctx, sql, uuid.New(), company); err == nil {
			t.Fatalf("%s: von der Datenbank angenommen", name)
		}
	}
}

// Rechte aus Migration 099: self an alle Systemrollen, read/write nur an
// Fuhrpark und Admin - NICHT an die Personalabteilung (Entscheidung des
// Nutzers vom 2026-10-25).
func TestFleetPermissionsByRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	rows, err := env.PG.Query(context.Background(), `
		SELECT r.code, p.code FROM role_permissions rp
		  JOIN roles r ON r.id = rp.role_id
		  JOIN permissions p ON p.id = rp.permission_id
		 WHERE p.code LIKE 'fleet.%' AND r.is_system`)
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	defer rows.Close()
	got := map[string][]string{}
	for rows.Next() {
		var role, perm string
		if err := rows.Scan(&role, &perm); err != nil {
			t.Fatalf("lesen: %v", err)
		}
		got[perm] = append(got[perm], role)
	}
	has := func(perm, role string) bool {
		for _, r := range got[perm] {
			if r == role {
				return true
			}
		}
		return false
	}
	for _, role := range []string{"admin", "sales", "procurement", "inventory", "finance", "hr", "production", "fleet"} {
		if !has("fleet.self", role) {
			t.Errorf("fleet.self fehlt bei %s", role)
		}
	}
	for _, perm := range []string{"fleet.read", "fleet.write"} {
		if len(got[perm]) != 2 || !has(perm, "admin") || !has(perm, "fleet") {
			t.Errorf("%s: %v, erwartet genau admin und fleet", perm, got[perm])
		}
		if has(perm, "hr") {
			t.Errorf("%s darf die Personalabteilung nicht haben", perm)
		}
	}
	if t.Failed() {
		t.Logf("Rollen je Recht: %v", got)
	}
}
