package production

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/materials"
	"nalaerp3/internal/testutil"
)

// Reservierungen bei der Freigabe (Backlog H.1.5a): Migration 108 und die
// Hilfen in materials, die in der Transaktion der Freigabe laufen.

// seedStock legt Material und Lager im Mandanten an, mit Bestand qty
// (0: ohne Bewegung).
func seedStock(t *testing.T, env *testutil.IntegrationEnv, company, einheit string, qty float64) (material, warehouse string) {
	t.Helper()
	ctx := context.Background()
	material, warehouse = "mat-"+uuid.NewString()[:12], "wh-"+uuid.NewString()[:12]
	if _, err := env.PG.Exec(ctx, `INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Testmaterial','artikel',$2,$3)`,
		material, einheit, company); err != nil {
		t.Fatalf("Material: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Testlager',$2)`, warehouse, company); err != nil {
		t.Fatalf("Lager: %v", err)
	}
	if qty != 0 {
		if _, err := env.PG.Exec(ctx, `INSERT INTO stock_movements (id, material_id, warehouse_id, quantity, uom, movement_type) VALUES ($1,$2,$3,$4,$5,'purchase')`,
			uuid.NewString(), material, warehouse, qty, einheit); err != nil {
			t.Fatalf("Bestand: %v", err)
		}
	}
	return material, warehouse
}

// inTx fuehrt f in einer Transaktion aus und schreibt sie fest.
func inTx(t *testing.T, env *testutil.IntegrationEnv, f func(tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := env.PG.Begin(ctx)
	if err != nil {
		t.Fatalf("Transaktion: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := f(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestReserveAvailableTx(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	mat, wh := seedStock(t, env, p.company, "Stk", 10)
	otherMat, otherWh := seedStock(t, env, other.company, "Stk", 10)
	project := p.project.String()

	reserve := func(company, m, w, proj string, want float64) (string, float64, error) {
		var id string
		var qty float64
		err := inTx(t, env, func(tx pgx.Tx) error {
			var err error
			id, qty, err = materials.ReserveAvailableTx(ctx, tx, company, m, w, proj, want, "Fertigungsauftrag", "FA-1")
			return err
		})
		return id, qty, err
	}
	release := func(company, id string) bool {
		var ok bool
		if err := inTx(t, env, func(tx pgx.Tx) error {
			var err error
			ok, err = materials.ReleaseReservationTx(ctx, tx, company, id)
			return err
		}); err != nil {
			t.Fatalf("Freigeben: %v", err)
		}
		return ok
	}
	countRows := func() int {
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM stock_reservations WHERE material_id=$1`, mat).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Negativfaelle: nichts angelegt.
	for _, c := range []struct {
		name, company, m, w, proj string
		want                      float64
		err                       string
	}{
		{"ohne Mandant", "", mat, wh, project, 1, "Mandant erforderlich"},
		{"Menge 0", p.company, mat, wh, project, 0, "Menge muss größer als 0 sein"},
		{"Menge negativ", p.company, mat, wh, project, -1, "Menge muss größer als 0 sein"},
		{"fremdes Material", p.company, otherMat, wh, project, 1, "Material nicht gefunden"},
		{"fremdes Lager", p.company, mat, otherWh, project, 1, "Lager nicht gefunden"},
		{"fremdes Projekt", p.company, mat, wh, other.project.String(), 1, "Projekt nicht gefunden"},
		{"anderer Mandant", other.company, mat, wh, project, 1, "Material nicht gefunden"},
	} {
		_, _, err := reserve(c.company, c.m, c.w, c.proj, c.want)
		wantErr(t, c.name, err, c.err)
	}
	if n := countRows(); n != 0 {
		t.Fatalf("nach Fehlern %d Reservierungen", n)
	}

	id1, q1, err := reserve(p.company, mat, wh, project, 4)
	if err != nil || id1 == "" || q1 != 4 {
		t.Fatalf("4 von 10: %q %v %v", id1, q1, err)
	}
	var status, grund, referenz, proj string
	var qty float64
	if err := env.PG.QueryRow(ctx, `SELECT status, grund, referenz, project_id::text, qty::float8 FROM stock_reservations WHERE id=$1`, id1).
		Scan(&status, &grund, &referenz, &proj, &qty); err != nil {
		t.Fatal(err)
	}
	if status != "aktiv" || grund != "Fertigungsauftrag" || referenz != "FA-1" || proj != project || qty != 4 {
		t.Fatalf("Zeile: %s %s %s %s %v", status, grund, referenz, proj, qty)
	}
	// Mehr als verfuegbar: nur der Rest.
	id2, q2, err := reserve(p.company, mat, wh, project, 10)
	if err != nil || id2 == "" || q2 != 6 {
		t.Fatalf("Rest: %q %v %v", id2, q2, err)
	}
	// Nichts mehr frei: keine Zeile, kein Fehler.
	id3, q3, err := reserve(p.company, mat, wh, project, 1)
	if err != nil || id3 != "" || q3 != 0 || countRows() != 2 {
		t.Fatalf("nichts frei: %q %v %v, %d Zeilen", id3, q3, err, countRows())
	}
	// Ein anderes Lager desselben Materials zaehlt nicht mit.
	if _, err := env.PG.Exec(ctx, `INSERT INTO warehouses (id, code, name, company_id) VALUES ('wh2-'||$1,'wh2-'||$1,'Zweitlager',$2)`, mat, p.company); err != nil {
		t.Fatal(err)
	}
	if id, q, err := reserve(p.company, mat, "wh2-"+mat, project, 1); err != nil || id != "" || q != 0 {
		t.Fatalf("leeres Zweitlager: %q %v %v", id, q, err)
	}

	// Freigeben: fremder Mandant nicht, zweimal nicht, danach wieder frei.
	if release(other.company, id1) {
		t.Fatal("fremder Mandant hat freigegeben")
	}
	if release(p.company, "gibt-es-nicht") {
		t.Fatal("unbekannte Reservierung freigegeben")
	}
	if !release(p.company, id1) {
		t.Fatal("Freigabe abgelehnt")
	}
	if release(p.company, id1) {
		t.Fatal("zweimal freigegeben")
	}
	if err := env.PG.QueryRow(ctx, `SELECT status FROM stock_reservations WHERE id=$1 AND released_at IS NOT NULL`, id1).Scan(&status); err != nil || status != "freigegeben" {
		t.Fatalf("nach Freigabe: %s %v", status, err)
	}
	if _, q, err := reserve(p.company, mat, wh, project, 5); err != nil || q != 4 {
		t.Fatalf("nach Freigabe wieder 4 frei: %v %v", q, err)
	}

	// Abrunden auf 6 Stellen: nie mehr als verfuegbar.
	// Der Bestand hat schon 6 Stellen (numeric(18,6)); mehr Stellen hat nur
	// ein umgerechneter Bedarf.
	m3, w3 := seedStock(t, env, p.company, "m", 1)
	if _, q, err := reserve(p.company, m3, w3, project, 2.0/3); err != nil || q != 0.666666 {
		t.Fatalf("Abrunden: %v %v", q, err)
	}
	// Ohne Bestand: nichts.
	m4, w4 := seedStock(t, env, p.company, "Stk", 0)
	if id, q, err := reserve(p.company, m4, w4, project, 1); err != nil || id != "" || q != 0 {
		t.Fatalf("ohne Bestand: %q %v %v", id, q, err)
	}
}

// Die Datenbank schuetzt die neuen Spalten (Migration 108), auch gegen
// Direkt-SQL.
func TestProductionReservationColumns(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	mat, wh := seedStock(t, env, p.company, "Stk", 10)
	o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 1}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}

	// Im Entwurf kein Reservierungslager.
	if _, err := env.PG.Exec(ctx, `UPDATE production_orders SET reservierung_lager_id=$2 WHERE id=$1`, o.ID, wh); err == nil {
		t.Fatal("Reservierungslager im Entwurf angenommen")
	}
	if _, err := svc.Release(ctx, o.ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE production_orders SET reservierung_lager_id=$2 WHERE id=$1`, o.ID, wh); err != nil {
		t.Fatalf("Reservierungslager nach Freigabe: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE production_orders SET reservierung_lager_id='gibt-es-nicht' WHERE id=$1`, o.ID); err == nil {
		t.Fatal("unbekanntes Lager angenommen")
	}

	var item uuid.UUID
	if err := env.PG.QueryRow(ctx, `SELECT id FROM production_order_items WHERE order_id=$1 AND art='artikel'`, o.ID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	var resv string
	if err := inTx(t, env, func(tx pgx.Tx) error {
		var err error
		resv, _, err = materials.ReserveAvailableTx(ctx, tx, p.company, mat, wh, p.project.String(), 2, "Fertigungsauftrag", o.Nummer)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE production_order_items SET material_id=$2, stock_reservation_id=$3 WHERE id=$1`, item, mat, resv); err != nil {
		t.Fatalf("Material und Reservierung an der Zeile: %v", err)
	}
	// Eine Reservierung hoechstens an einer Zeile.
	if _, err := env.PG.Exec(ctx, `UPDATE production_order_items SET stock_reservation_id=$2 WHERE order_id=$1 AND art='glas'`, o.ID, resv); err == nil {
		t.Fatal("Reservierung an zwei Zeilen angenommen")
	}
	if _, err := env.PG.Exec(ctx, `UPDATE production_order_items SET stock_reservation_id='gibt-es-nicht' WHERE order_id=$1 AND art='glas'`, o.ID); err == nil {
		t.Fatal("unbekannte Reservierung angenommen")
	}
	// Lager mit Auftrag laesst sich nicht loeschen - auch ein leeres, an dem
	// nur der Auftrag haengt.
	_, empty := seedStock(t, env, p.company, "Stk", 0)
	if _, err := env.PG.Exec(ctx, `UPDATE production_orders SET reservierung_lager_id=$2 WHERE id=$1`, o.ID, empty); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM warehouses WHERE id=$1`, empty); err == nil {
		t.Fatal("Lager mit Auftrag geloescht")
	}
	// Material geloescht: Reservierung weg (075), Zeile bleibt ohne Bezug.
	if _, err := env.PG.Exec(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, mat); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM materials WHERE id=$1`, mat); err != nil {
		t.Fatalf("Material loeschen: %v", err)
	}
	var gotMat, gotResv *string
	if err := env.PG.QueryRow(ctx, `SELECT material_id, stock_reservation_id FROM production_order_items WHERE id=$1`, item).Scan(&gotMat, &gotResv); err != nil {
		t.Fatalf("Zeile nach Materialloeschung: %v", err)
	}
	if gotMat != nil || gotResv != nil {
		t.Fatalf("Bezug nach Materialloeschung: %v %v", gotMat, gotResv)
	}
}
