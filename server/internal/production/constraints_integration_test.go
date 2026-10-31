package production

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

type seededPosition struct {
	company, nummer             string
	project, elevation, variant uuid.UUID
}

func seedPosition(t *testing.T, env *testutil.IntegrationEnv) seededPosition {
	t.Helper()
	ctx := context.Background()
	s := seededPosition{company: "pr-" + uuid.NewString()[:12], project: uuid.New(), elevation: uuid.New(), variant: uuid.New()}
	phase := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, []any{s.company, "Fertigungstest " + s.company}},
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, 'P-1', 'Testprojekt', $2)`, []any{s.project, s.company}},
		{`INSERT INTO project_phases (id, project_id, nummer, name) VALUES ($1, $2, '1', 'Haus A')`, []any{phase, s.project}},
		{`INSERT INTO project_elevations (id, phase_id, nummer, name, menge) VALUES ($1, $2, '3', 'Fenster', 10)`, []any{s.elevation, phase}},
		{`INSERT INTO project_single_elevations (id, elevation_id, name, menge) VALUES ($1, $2, 'links', 10)`, []any{s.variant, s.elevation}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("Testdaten: %v", err)
		}
	}
	return s
}

// Die Datenbank selbst schuetzt die Regeln der Fertigungsauftraege
// (Migration 107), auch gegen Direkt-SQL (Backlog H.1.1).
func TestProductionOrderTableConstraints(t *testing.T) {
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
	n := 0
	num := func() string { n++; return fmt.Sprintf("FA-T-%d", n) }
	ins := `INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl,
		ausfuehrungsklasse, status, position_nummer, inhalt_hash, freigegeben_at, begonnen_at, fertig_at, storno_reason, storniert_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	t1, t2, t3 := "2026-11-02T08:00:00Z", "2026-11-03T08:00:00Z", "2026-11-04T08:00:00Z"
	row := func(nummer, status string, stueck int, exc any, posNr, hash string, frei, beg, fer any, grund string, storno any) []any {
		return []any{uuid.New(), p.company, nummer, p.project, p.elevation, p.variant, stueck, exc, status, posNr, hash, frei, beg, fer, grund, storno}
	}

	released := uuid.New()
	ok("Entwurf", ins, row(num(), "entwurf", 4, nil, "", "", nil, nil, nil, "", nil)...)
	args := row("FA-T-REL", "freigegeben", 6, "EXC2", "3", "h", t1, nil, nil, "", nil)
	args[0] = released
	ok("freigegeben mit Kopie", ins, args...)
	ok("in Arbeit", ins, row(num(), "in_arbeit", 1, "EXC1", "3", "h", t1, t2, nil, "", nil)...)
	ok("fertig", ins, row(num(), "fertig", 1, nil, "3", "h", t1, t2, t3, "", nil)...)
	ok("fertig, alles am selben Zeitpunkt", ins, row(num(), "fertig", 1, nil, "3", "h", t1, t1, t1, "", nil)...)
	ok("Entwurf storniert", ins, row(num(), "storniert", 1, nil, "", "", nil, nil, nil, "doppelt", t2)...)
	ok("in Arbeit storniert", ins, row(num(), "storniert", 1, nil, "3", "h", t1, t2, nil, "Maß falsch", t3)...)

	bad("Nummer doppelt", ins, row("FA-T-REL", "entwurf", 1, nil, "", "", nil, nil, nil, "", nil)...)
	bad("Nummer leer", ins, row(" ", "entwurf", 1, nil, "", "", nil, nil, nil, "", nil)...)
	bad("Stückzahl 0", ins, row(num(), "entwurf", 0, nil, "", "", nil, nil, nil, "", nil)...)
	bad("Klasse unbekannt", ins, row(num(), "entwurf", 1, "EXC5", "", "", nil, nil, nil, "", nil)...)
	bad("Klasse klein", ins, row(num(), "entwurf", 1, "exc2", "", "", nil, nil, nil, "", nil)...)
	bad("Status unbekannt", ins, row(num(), "geloescht", 1, nil, "", "", nil, nil, nil, "", nil)...)
	bad("Entwurf mit Freigabezeit", ins, row(num(), "entwurf", 1, nil, "3", "h", t1, nil, nil, "", nil)...)
	bad("freigegeben ohne Zeit", ins, row(num(), "freigegeben", 1, nil, "3", "h", nil, nil, nil, "", nil)...)
	bad("freigegeben ohne Kopie", ins, row(num(), "freigegeben", 1, nil, "3", "", t1, nil, nil, "", nil)...)
	bad("freigegeben ohne Positionsnummer", ins, row(num(), "freigegeben", 1, nil, " ", "h", t1, nil, nil, "", nil)...)
	bad("freigegeben mit Beginn", ins, row(num(), "freigegeben", 1, nil, "3", "h", t1, t2, nil, "", nil)...)
	bad("in Arbeit ohne Beginn", ins, row(num(), "in_arbeit", 1, nil, "3", "h", t1, nil, nil, "", nil)...)
	bad("in Arbeit mit Ende", ins, row(num(), "in_arbeit", 1, nil, "3", "h", t1, t2, t3, "", nil)...)
	bad("fertig ohne Ende", ins, row(num(), "fertig", 1, nil, "3", "h", t1, t2, nil, "", nil)...)
	bad("fertig ohne Beginn", ins, row(num(), "fertig", 1, nil, "3", "h", t1, nil, t3, "", nil)...)
	bad("Beginn vor Freigabe", ins, row(num(), "in_arbeit", 1, nil, "3", "h", t2, t1, nil, "", nil)...)
	bad("Ende vor Beginn", ins, row(num(), "fertig", 1, nil, "3", "h", t1, t3, t2, "", nil)...)
	bad("fertig storniert", ins, row(num(), "storniert", 1, nil, "3", "h", t1, t2, t3, "zu spät", t3)...)
	bad("storniert, Beginn ohne Freigabe", ins, row(num(), "storniert", 1, nil, "", "", nil, t2, nil, "x", t3)...)
	bad("storniert ohne Grund", ins, row(num(), "storniert", 1, nil, "", "", nil, nil, nil, " ", t2)...)
	bad("storniert ohne Zeit", ins, row(num(), "storniert", 1, nil, "", "", nil, nil, nil, "x", nil)...)
	bad("aktiv mit Storno-Grund", ins, row(num(), "entwurf", 1, nil, "", "", nil, nil, nil, "x", nil)...)
	bad("aktiv mit Storno-Zeit", ins, row(num(), "entwurf", 1, nil, "", "", nil, nil, nil, "", t2)...)
	unknownProject := row(num(), "entwurf", 1, nil, "", "", nil, nil, nil, "", nil)
	unknownProject[3] = uuid.New()
	bad("Projekt unbekannt", ins, unknownProject...)
	unknownCompany := row(num(), "entwurf", 1, nil, "", "", nil, nil, nil, "", nil)
	unknownCompany[1] = "gibt-es-nicht"
	bad("Mandant unbekannt", ins, unknownCompany...)
	// gleiche Nummer bei einem anderen Mandanten ist erlaubt
	other := seedPosition(t, env)
	ok("Nummer bei anderem Mandanten", ins, uuid.New(), other.company, "FA-T-REL", other.project, other.elevation, other.variant, 1,
		nil, "entwurf", "", "", nil, nil, nil, "", nil)

	insI := `INSERT INTO production_order_items (id, order_id, pos, art, configuration, length_mm, width_mm, height_mm, area_m2, menge_je_stueck)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	ok("Profil", insI, uuid.New(), released, 1, "profil", "", 1230.5, nil, nil, nil, 2)
	ok("Artikel", insI, uuid.New(), released, 2, "artikel", "", nil, nil, nil, nil, 0)
	ok("Glas", insI, uuid.New(), released, 3, "glas", "4/16/4", nil, 1100, 1350, 1.485, 1)
	bad("Position doppelt", insI, uuid.New(), released, 1, "artikel", "", nil, nil, nil, nil, 1)
	bad("Position 0", insI, uuid.New(), released, 0, "artikel", "", nil, nil, nil, nil, 1)
	bad("Art unbekannt", insI, uuid.New(), released, 4, "beschlag", "", nil, nil, nil, nil, 1)
	bad("Menge negativ", insI, uuid.New(), released, 5, "artikel", "", nil, nil, nil, nil, -1)
	bad("Länge am Artikel", insI, uuid.New(), released, 6, "artikel", "", 100, nil, nil, nil, 1)
	bad("Länge negativ", insI, uuid.New(), released, 7, "profil", "", -1, nil, nil, nil, 1)
	bad("Glasmaß am Profil", insI, uuid.New(), released, 8, "profil", "", nil, 100, nil, nil, 1)
	bad("Fläche am Artikel", insI, uuid.New(), released, 9, "artikel", "", nil, nil, nil, 1, 1)
	bad("Glasaufbau am Profil", insI, uuid.New(), released, 10, "profil", "4/16/4", nil, nil, nil, nil, 1)
	bad("Glasbreite negativ", insI, uuid.New(), released, 11, "glas", "", nil, -1, nil, nil, 1)
	bad("Glashöhe negativ", insI, uuid.New(), released, 12, "glas", "", nil, nil, -1, nil, 1)
	bad("Glasfläche negativ", insI, uuid.New(), released, 13, "glas", "", nil, nil, nil, -1, 1)
	bad("Auftrag unbekannt", insI, uuid.New(), uuid.New(), 1, "artikel", "", nil, nil, nil, nil, 1)

	// RESTRICT: kein Loeschen von Auftrag mit Stueckliste und Projekt mit
	// Auftraegen.
	bad("Auftrag mit Stückliste löschen", `DELETE FROM production_orders WHERE id=$1`, released)
	bad("Projekt mit Aufträgen löschen", `DELETE FROM projects WHERE id=$1`, p.project)
	// ohne Stueckliste: das RESTRICT am Projekt selbst greift
	bad("Projekt mit Entwurf löschen", `DELETE FROM projects WHERE id=$1`, other.project)

	// SET NULL: ein Re-Import, der Variante und Position entfernt, laesst
	// den Auftrag stehen.
	ok("Variante löschen", `DELETE FROM project_single_elevations WHERE id=$1`, p.variant)
	ok("Position löschen", `DELETE FROM project_elevations WHERE id=$1`, p.elevation)
	var elev, variant *uuid.UUID
	var posNr string
	if err := env.PG.QueryRow(ctx, `SELECT elevation_id, single_elevation_id, position_nummer FROM production_orders WHERE id=$1`, released).
		Scan(&elev, &variant, &posNr); err != nil {
		t.Fatalf("Auftrag nach Re-Import: %v", err)
	}
	if elev != nil || variant != nil || posNr != "3" {
		t.Fatalf("nach Entfernen der Position: %v %v %q", elev, variant, posNr)
	}
}

// Migration 107 legt Nummernkreis und Rechte an.
func TestProductionSeeds(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	var pattern string
	if err := env.PG.QueryRow(ctx, `SELECT pattern FROM number_sequences WHERE company_id='default' AND entity='production_order'`).Scan(&pattern); err != nil || pattern != "FA-{YYYY}-{NNNN}" {
		t.Fatalf("Nummernkreis: %q, %v", pattern, err)
	}
	has := func(role, code string) bool {
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id=$1 AND p.code=$2`, role, code).Scan(&n); err != nil {
			t.Fatalf("Rechte: %v", err)
		}
		return n == 1
	}
	for _, c := range []struct {
		role, code string
		want       bool
	}{
		{"role-admin", "production.read", true},
		{"role-admin", "production.write", true},
		{"role-production", "production.read", true},
		{"role-production", "production.write", true},
		{"role-sales", "production.read", true},
		{"role-sales", "production.write", false},
		{"role-hr", "production.read", false},
		{"role-fleet", "production.read", false},
		{"role-inventory", "production.write", false},
	} {
		if got := has(c.role, c.code); got != c.want {
			t.Errorf("%s %s: %v", c.role, c.code, got)
		}
	}
}
