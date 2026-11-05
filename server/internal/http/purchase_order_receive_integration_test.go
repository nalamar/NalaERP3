package apihttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Wareneingang am Stueck ueber HTTP (Backlog H.4.z1): beide Rechte noetig,
// Meldungen als 400/404/409 statt 500.
func TestPurchaseOrderReceiveHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	admin, buyer, stock := c.user("admin"), c.user("procurement"), c.user("inventory")
	n := uuid.NewString()[:8]
	supplier, material, wh, order := "sup-rc-"+n, "mat-rc-"+n, "wh-rc-"+n, "po-rc-"+n
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Zulieferer',$2)`, []any{supplier, c.company}},
		{`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Profil','profil','Stk',$2)`, []any{material, c.company}},
		{`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Lager',$2)`, []any{wh, c.company}},
		{`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$1,'EUR','ordered',$3)`, []any{order, supplier, c.company}},
		{`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,1,$3,4,'Stk',2,'EUR')`, []any{order + "-1", order, material}},
	} {
		if _, err := env.PG.Exec(context.Background(), q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v1/purchase-orders/" + order + "/receive"
	body := `{"warehouse_id":"` + wh + `","status_received":true}`

	// Einkauf ohne Lagerrecht und Lager ohne Bestellrecht: 403, nichts gebucht.
	for _, user := range []string{buyer, stock} {
		wantAPIError(t, doAuthedRequest(t, handler, user, http.MethodPost, path, body), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPost, path, `{"warehouse_id":" "}`), http.StatusBadRequest, "validation_error", "Lager erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPost, "/api/v1/purchase-orders/po-unbekannt/receive", body), http.StatusNotFound, "not_found", "Bestellung nicht gefunden")

	type received struct {
		Bestellung struct {
			Status string `json:"status"`
		} `json:"bestellung"`
		Bewegungen int `json:"bewegungen"`
	}
	got := decodeJSON[received](t, doAuthedRequest(t, handler, admin, http.MethodPost, path, body), http.StatusOK)
	if got.Bestellung.Status != "received" || got.Bewegungen != 1 {
		t.Fatalf("gebucht: %+v", got)
	}
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPost, path, body), http.StatusConflict, "conflict", "Wareneingang ist im aktuellen Status nicht erlaubt (aktuell: received)")
	if _, err := env.PG.Exec(context.Background(), `UPDATE purchase_orders SET status='ordered' WHERE id=$1`, order); err != nil {
		t.Fatal(err)
	}
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPost, path, body), http.StatusConflict, "conflict", "Wareneingang zu dieser Bestellung wurde bereits erfasst")

	var moves, audits int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM stock_movements WHERE purchase_order_item_id=$1`, order+"-1").Scan(&moves); err != nil {
		t.Fatal(err)
	}
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM entity_change_log WHERE entity_type='purchase_order' AND entity_id=$1 AND action='status_geaendert'`, order).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if moves != 1 || audits != 1 {
		t.Fatalf("Bewegungen %d, Protokolleintraege %d", moves, audits)
	}
}
