package production

import (
	"context"
	"math"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/materials"
	"nalaerp3/internal/testutil"
)

// Reservierungen bei der Freigabe (Backlog H.1.5): Migration 108, die
// Hilfen in materials, die in der Transaktion der Freigabe laufen (a), und
// Freigabe mit Lager, Storno und Lesen im Service (b).

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
	if _, err := svc.Release(ctx, o.ID, "", "", p.company); err != nil {
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

// addMaterial legt ein Material im Mandanten mit Bestand qty im Lager an.
func addMaterial(t *testing.T, env *testutil.IntegrationEnv, company, einheit, warehouse string, qty float64) string {
	t.Helper()
	ctx := context.Background()
	material := "mat-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(ctx, `INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Testmaterial','artikel',$2,$3)`,
		material, einheit, company); err != nil {
		t.Fatalf("Material: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO stock_movements (id, material_id, warehouse_id, quantity, uom, movement_type) VALUES ($1,$2,$3,$4,$5,'purchase')`,
		uuid.NewString(), material, warehouse, qty, einheit); err != nil {
		t.Fatalf("Bestand: %v", err)
	}
	return material
}

func itemOf(t *testing.T, o *Order, art string) Item {
	t.Helper()
	for _, it := range o.Items {
		if it.Art == art {
			return it
		}
	}
	t.Fatalf("keine Zeile %s", art)
	return Item{}
}

func near(p *float64, want float64) bool { return p != nil && math.Abs(*p-want) < 1e-9 }

func TestReleaseReservesFromWarehouse(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	profMat, wh := seedStock(t, env, p.company, "m", 10)
	artMat := addMaterial(t, env, p.company, "Stk", wh, 4)
	glasMat := addMaterial(t, env, p.company, "m²", wh, 100)
	otherMat, otherWh := seedStock(t, env, other.company, "Stk", 10)
	link := func(table, material string) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, `UPDATE `+table+` SET material_id=$2 WHERE single_elevation_id=$1`, p.variant, material); err != nil {
			t.Fatal(err)
		}
	}
	link("single_elevation_profiles", profMat)
	link("single_elevation_articles", artMat)
	// Material eines fremden Mandanten an der Projektzeile: nicht kopiert,
	// nicht reserviert (Einheit "Stk" waere sonst reservierbar).
	link("single_elevation_glass", otherMat)
	create := func(n int) *Order {
		t.Helper()
		o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: n}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	// aktive und alle Reservierungen eines Auftrags
	count := func(nummer string) (aktiv, alle int) {
		t.Helper()
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='aktiv'), count(*) FROM stock_reservations WHERE referenz=$1 AND project_id=$2`,
			nummer, p.project).Scan(&aktiv, &alle); err != nil {
			t.Fatal(err)
		}
		return aktiv, alle
	}

	o1 := create(3)
	// Negativfaelle: unbekanntes und fremdes Lager - nichts freigegeben.
	for _, lager := range []string{"gibt-es-nicht", otherWh} {
		if _, err := svc.Release(ctx, o1.ID, lager, "", p.company); err == nil || err.Error() != "Lager nicht gefunden" {
			t.Fatalf("Lager %s: %v", lager, err)
		}
	}
	if got, _ := svc.Get(ctx, o1.ID, p.company); got.Status != StatusEntwurf || got.ReservierungLagerID != nil {
		t.Fatalf("nach abgelehnter Freigabe: %s %v", got.Status, got.ReservierungLagerID)
	}
	if _, alle := count(o1.Nummer); alle != 0 {
		t.Fatalf("Reservierungen nach abgelehnter Freigabe: %d", alle)
	}

	r1, err := svc.Release(ctx, o1.ID, " "+wh+" ", "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if r1.ReservierungLagerID == nil || *r1.ReservierungLagerID != wh {
		t.Fatalf("Reservierungslager: %v", r1.ReservierungLagerID)
	}
	// Profil: 2 Stueck je Element x 3 x 1,2304 m = 7,3824 m, alles da.
	prof := itemOf(t, r1, ArtProfil)
	if prof.MaterialID == nil || *prof.MaterialID != profMat || prof.MaterialEinheit != "m" ||
		!near(prof.Bedarf, 7.3824) || !near(prof.Reserviert, 7.3824) || !near(prof.Fehlmenge, 0) {
		t.Fatalf("Profil: %+v", prof)
	}
	// Artikel: 1,5 x 3 = 4,5 Stk, 4 da - Fehlmenge 0,5.
	art := itemOf(t, r1, ArtArtikel)
	if art.MaterialID == nil || *art.MaterialID != artMat || !near(art.Bedarf, 4.5) || !near(art.Reserviert, 4) || !near(art.Fehlmenge, 0.5) {
		t.Fatalf("Artikel: %+v", art)
	}
	if glas := itemOf(t, r1, ArtGlas); glas.MaterialID != nil || glas.Bedarf != nil {
		t.Fatalf("Glas mit fremdem Material: %+v", glas)
	}
	if !slices.Contains(r1.Hinweise, HintFehlmenge) {
		t.Fatalf("Hinweise: %v", r1.Hinweise)
	}
	var grund, projekt string
	if err := env.PG.QueryRow(ctx, `SELECT DISTINCT grund, project_id::text FROM stock_reservations r
		JOIN warehouses w ON w.id = r.warehouse_id WHERE r.referenz=$1 AND w.company_id=$2`, o1.Nummer, p.company).
		Scan(&grund, &projekt); err != nil {
		t.Fatal(err)
	}
	if grund != ReservierungGrund || projekt != p.project.String() {
		t.Fatalf("Reservierung: %s %s", grund, projekt)
	}
	if aktiv, alle := count(o1.Nummer); aktiv != 2 || alle != 2 {
		t.Fatalf("Reservierungen: %d/%d", aktiv, alle)
	}

	// Ein geaenderter Materialbezug ist keine geaenderte Position.
	link("single_elevation_glass", glasMat)
	if got, _ := svc.Get(ctx, o1.ID, p.company); slices.Contains(got.Hinweise, HintPositionGeaendert) {
		t.Fatalf("Hinweise nach Materialbezug: %v", got.Hinweise)
	}

	// Zweiter Auftrag: Profil 2,4608 m vom Rest 2,6176 m, Artikel nichts
	// mehr frei; Glas auf m² wird kopiert, aber nicht reserviert.
	o2 := create(1)
	r2, err := svc.Release(ctx, o2.ID, wh, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if prof := itemOf(t, r2, ArtProfil); !near(prof.Reserviert, 2.4608) || !near(prof.Fehlmenge, 0) {
		t.Fatalf("Profil 2: %+v", prof)
	}
	if art := itemOf(t, r2, ArtArtikel); !near(art.Reserviert, 0) || !near(art.Fehlmenge, 1.5) {
		t.Fatalf("Artikel 2: %+v", art)
	}
	if glas := itemOf(t, r2, ArtGlas); glas.MaterialID == nil || *glas.MaterialID != glasMat || glas.Bedarf != nil {
		t.Fatalf("Glas 2: %+v", glas)
	}
	if aktiv, _ := count(o2.Nummer); aktiv != 1 {
		t.Fatalf("Reservierungen 2: %d", aktiv)
	}
	// Im Lager freigegeben (nicht ueber den Auftrag): zaehlt nicht mehr
	// als reserviert, der Bedarf ist Fehlmenge.
	var rid string
	if err := env.PG.QueryRow(ctx, `SELECT stock_reservation_id FROM production_order_items WHERE order_id=$1 AND art='profil'`, o2.ID).Scan(&rid); err != nil {
		t.Fatal(err)
	}
	if err := inTx(t, env, func(tx pgx.Tx) error {
		_, err := materials.ReleaseReservationTx(ctx, tx, p.company, rid)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if g2, _ := svc.Get(ctx, o2.ID, p.company); !near(itemOf(t, g2, ArtProfil).Reserviert, 0) || !near(itemOf(t, g2, ArtProfil).Fehlmenge, 2.4608) {
		t.Fatalf("Profil nach Freigabe im Lager: %+v", itemOf(t, g2, ArtProfil))
	}
	// "fertig" ohne Fehlmengen-Hinweis; Freigabe der Reservierungen beim
	// Fertigmelden: TestFinishReleasesReservations (H.3.2b).
	if _, err := svc.Start(ctx, o2.ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	f2, err := svc.Finish(ctx, o2.ID, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if aktiv, _ := count(o2.Nummer); aktiv != 0 || slices.Contains(f2.Hinweise, HintFehlmenge) {
		t.Fatalf("nach fertig: %d %v", aktiv, f2.Hinweise)
	}

	// Storno gibt frei; danach ist der Artikel fuer einen neuen Auftrag da.
	s1, err := svc.Cancel(ctx, o1.ID, "Kunde storniert", "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if aktiv, alle := count(o1.Nummer); aktiv != 0 || alle != 2 {
		t.Fatalf("nach Storno: %d/%d", aktiv, alle)
	}
	if art := itemOf(t, s1, ArtArtikel); art.MaterialID == nil || art.Bedarf != nil || art.Fehlmenge != nil {
		t.Fatalf("Artikel storniert: %+v", art)
	}
	o3 := create(1)
	r3, err := svc.Release(ctx, o3.ID, wh, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if art := itemOf(t, r3, ArtArtikel); !near(art.Reserviert, 1.5) || slices.Contains(r3.Hinweise, HintFehlmenge) {
		t.Fatalf("Artikel nach Storno: %+v %v", art, r3.Hinweise)
	}

	// Ohne Lager: Material kopiert, keine Reservierung, keine Fehlmenge.
	o4 := create(1)
	r4, err := svc.Release(ctx, o4.ID, "", "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if art := itemOf(t, r4, ArtArtikel); r4.ReservierungLagerID != nil || art.MaterialID == nil || art.Bedarf != nil {
		t.Fatalf("ohne Lager: %v %+v", r4.ReservierungLagerID, art)
	}
	if _, alle := count(o4.Nummer); alle != 0 {
		t.Fatalf("Reservierungen ohne Lager: %d", alle)
	}
	if _, err := svc.Cancel(ctx, o4.ID, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}
}
