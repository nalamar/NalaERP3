package production

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/testutil"
)

// countingQuerier zaehlt die Abfragen an die Datenbank.
type countingQuerier struct {
	q querier
	n int
}

func (c *countingQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.n++
	return c.q.Query(ctx, sql, args...)
}

func (c *countingQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.n++
	return c.q.QueryRow(ctx, sql, args...)
}

// Hinweise der Liste am Stueck (Backlog H.5.4a): dieselben Hinweise wie
// in der Einzelansicht, mit einer festen Zahl von Abfragen.
func TestOrderListHintsBatched(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	seedSalesOrder(t, env, p, "open")
	// Zweites Projekt im selben Mandanten ohne Kundenauftrag: Position E2
	// (Menge 3) mit Variante V2, Position E3 mit Variante V3.
	p2, phase, e2, v2, e3, v3 := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, 'P-2', 'Zweitprojekt', $2)`, []any{p2, p.company}},
		{`INSERT INTO project_phases (id, project_id, nummer, name) VALUES ($1, $2, '1', 'Haus B')`, []any{phase, p2}},
		{`INSERT INTO project_elevations (id, phase_id, nummer, name, menge) VALUES ($1, $2, '7', 'Tuer', 3)`, []any{e2, phase}},
		{`INSERT INTO project_single_elevations (id, elevation_id, name, menge) VALUES ($1, $2, 'rechts', 3)`, []any{v2, e2}},
		{`INSERT INTO project_elevations (id, phase_id, nummer, name, menge) VALUES ($1, $2, '8', 'Tor', 1)`, []any{e3, phase}},
		{`INSERT INTO project_single_elevations (id, elevation_id, name, menge) VALUES ($1, $2, 'mitte', 1)`, []any{v3, e3}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	mk := func(project, elevation, variant uuid.UUID, n int) *Order {
		t.Helper()
		o, err := svc.Create(ctx, OrderInput{ProjectID: project, ElevationID: elevation, SingleElevationID: variant, Stueckzahl: n}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	// Projekt 1 (Menge 10): 5 + 5 genau voll; der stornierte zaehlt nicht.
	a, b, x := mk(p.project, p.elevation, p.variant, 5), mk(p.project, p.elevation, p.variant, 5), mk(p.project, p.elevation, p.variant, 5)
	if _, err := svc.Cancel(ctx, x.ID, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Release(ctx, b.ID, "", "", p.company); err != nil {
		t.Fatal(err)
	}
	// Projekt 2: 2 + 2 auf Menge 3, und ein Auftrag, dessen Variante weg ist.
	c, d, g := mk(p2, e2, v2, 2), mk(p2, e2, v2, 2), mk(p2, e3, v3, 1)
	if _, err := svc.Release(ctx, d.ID, "", "", p.company); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE single_elevation_profiles SET qty=3 WHERE single_elevation_id=$1`, []any{p.variant}},
		{`UPDATE project_elevations SET name='Tuer neu' WHERE id=$1`, []any{e2}},
		{`DELETE FROM project_single_elevations WHERE id=$1`, []any{v3}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}

	want := map[uuid.UUID][]string{
		a.ID: {},
		b.ID: {HintPositionGeaendert},
		x.ID: {},
		c.ID: {HintKeinKundenauftrag, HintMengeUeberschritten},
		d.ID: {HintKeinKundenauftrag, HintMengeUeberschritten, HintPositionGeaendert},
		g.ID: {HintKeinKundenauftrag, HintPositionEntfernt},
	}
	list, err := svc.List(ctx, p.company, OrderFilter{IncludeStorniert: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(want) {
		t.Fatalf("Liste: %d Auftraege, erwartet %d", len(list), len(want))
	}
	for _, o := range list {
		if w, ok := want[o.ID]; !ok || !slices.Equal(o.Hinweise, w) {
			t.Fatalf("%s: Hinweise %v, erwartet %v", o.Nummer, o.Hinweise, w)
		}
		single, err := svc.Get(ctx, o.ID, p.company)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(single.Hinweise, o.Hinweise) || single.PositionName != o.PositionName {
			t.Fatalf("%s: Einzelansicht %v %q, Liste %v %q", o.Nummer, single.Hinweise, single.PositionName, o.Hinweise, o.PositionName)
		}
		if o.ID == c.ID && o.PositionName != "Tuer neu" {
			t.Fatalf("Entwurf zeigt nicht den aktuellen Namen: %q", o.PositionName)
		}
		if o.ID == d.ID && o.PositionName != "Tuer" {
			t.Fatalf("Freigegebener zeigt nicht die Kopie: %q", o.PositionName)
		}
	}

	// Feste Zahl von Abfragen: ein Auftrag wie alle; nur stornierte keine.
	orders := make([]*Order, len(list))
	var cancelled []*Order
	for i := range list {
		orders[i] = &list[i]
		if list[i].Status == StatusStorniert {
			cancelled = append(cancelled, &list[i])
		}
	}
	for _, cs := range []struct {
		name   string
		orders []*Order
		want   int
	}{
		{"ein Auftrag", orders[:1], 4},
		{"alle Auftraege", orders, 4},
		{"nur stornierte", cancelled, 0},
	} {
		cq := &countingQuerier{q: env.PG}
		if err := decorate(ctx, cq, cs.orders, p.company); err != nil {
			t.Fatal(err)
		}
		if cq.n != cs.want {
			t.Fatalf("%s: %d Abfragen, erwartet %d", cs.name, cq.n, cs.want)
		}
	}
	if len(cancelled) != 1 || len(cancelled[0].Hinweise) != 0 {
		t.Fatalf("stornierte: %v", cancelled)
	}
}
