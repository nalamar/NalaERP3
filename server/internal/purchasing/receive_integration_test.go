package purchasing

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Wareneingang am Stueck (Backlog H.4.z1): alles oder nichts, kein
// doppeltes Buchen, nur im Status ordered.
func TestPurchaseOrderReceive(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	other := seedEvidence(t, env)
	svc := NewService(env.PG)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	moves := func(orderID string) int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM stock_movements sm JOIN purchase_order_items poi ON poi.id = sm.purchase_order_item_id WHERE poi.order_id=$1`, orderID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	wh, foreignWh := "wh-rc-"+uuid.NewString()[:8], "wh-rc-"+uuid.NewString()[:8]
	exec(`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Testlager',$2)`, wh, s.company)
	exec(`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Fremdlager',$2)`, foreignWh, other.company)
	item2 := s.item + "-2"
	exec(`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,2,$3,3,'m',12.5,'usd')`, item2, s.order, s.material)
	in := ReceiveInput{WarehouseID: " " + wh + " ", SetReceived: true}

	// Nur im Status ordered, nur im eigenen Mandanten, Lager Pflicht.
	_, _, _, err := svc.Receive(ctx, s.order, in, s.company, "")
	wantErr(t, err, "Wareneingang ist im aktuellen Status nicht erlaubt (aktuell: draft)")
	exec(`UPDATE purchase_orders SET status='ordered' WHERE id IN ($1,$2)`, s.order, other.order)
	_, _, _, err = svc.Receive(ctx, s.order, in, other.company, "")
	wantErr(t, err, "Bestellung nicht gefunden")
	_, _, _, err = svc.Receive(ctx, s.order, ReceiveInput{WarehouseID: "  "}, s.company, "")
	wantErr(t, err, "Lager erforderlich")
	_, _, _, err = svc.Receive(ctx, s.order, ReceiveInput{WarehouseID: foreignWh}, s.company, "")
	wantErr(t, err, "Lager nicht gefunden")

	// Alles oder nichts: die zweite Position scheitert am fremden Material,
	// die erste ist dann auch nicht gebucht.
	exec(`UPDATE purchase_order_items SET material_id=$2 WHERE id=$1`, item2, other.material)
	_, _, _, err = svc.Receive(ctx, s.order, in, s.company, "")
	wantErr(t, err, "Material nicht gefunden")
	if n := moves(s.order); n != 0 {
		t.Fatalf("nach Fehler %d Bewegungen gebucht", n)
	}
	exec(`UPDATE purchase_order_items SET material_id=$2 WHERE id=$1`, item2, s.material)

	po, items, n, err := svc.Receive(ctx, s.order, in, s.company, "")
	if err != nil || n != 2 || po.Status != "received" || len(items) != 2 {
		t.Fatalf("gebucht: %+v %d, %v", po, n, err)
	}
	var qty, price float64
	var wid, ref, typ, cur string
	if err := env.PG.QueryRow(ctx, `SELECT quantity, warehouse_id, reference, movement_type FROM stock_movements WHERE purchase_order_item_id=$1`, item2).Scan(&qty, &wid, &ref, &typ); err != nil {
		t.Fatal(err)
	}
	if qty != 3 || wid != wh || ref != "PO-EV-"+s.order[len("po-ev-"):] || typ != "purchase" {
		t.Fatalf("Bewegung: %v %s %s %s", qty, wid, ref, typ)
	}
	if err := env.PG.QueryRow(ctx, `SELECT avg_purchase_price, currency FROM materials WHERE id=$1`, s.material).Scan(&price, &cur); err != nil {
		t.Fatal(err)
	}
	// 5 Stk zu 0 und 3 m zu 12,5: Durchschnitt 37,5 / 8.
	if price != 37.5/8 || cur != "USD" {
		t.Fatalf("Einkaufspreis: %v %s", price, cur)
	}

	// Kein zweites Mal: erst der Status, dann die vorhandenen Bewegungen.
	_, _, _, err = svc.Receive(ctx, s.order, in, s.company, "")
	wantErr(t, err, "Wareneingang ist im aktuellen Status nicht erlaubt (aktuell: received)")
	exec(`UPDATE purchase_orders SET status='ordered' WHERE id=$1`, s.order)
	_, _, _, err = svc.Receive(ctx, s.order, ReceiveInput{WarehouseID: wh}, s.company, "")
	wantErr(t, err, "Wareneingang zu dieser Bestellung wurde bereits erfasst")
	if n := moves(s.order); n != 2 {
		t.Fatalf("nach zweitem Versuch %d Bewegungen", n)
	}

	// Ohne Status: bucht, Bestellung bleibt ordered. Ohne Positionen: Fehler.
	ow := "wh-rc-" + uuid.NewString()[:8]
	exec(`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Lager',$2)`, ow, other.company)
	if po, _, n, err := svc.Receive(ctx, other.order, ReceiveInput{WarehouseID: ow}, other.company, ""); err != nil || n != 1 || po.Status != "ordered" {
		t.Fatalf("ohne Status: %+v %d, %v", po, n, err)
	}
	empty := "po-rc-" + uuid.NewString()[:8]
	exec(`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$1,'EUR','ordered',$3)`, empty, s.supplier, s.company)
	_, _, _, err = svc.Receive(ctx, empty, in, s.company, "")
	wantErr(t, err, "Bestellung hat keine Positionen")
}
