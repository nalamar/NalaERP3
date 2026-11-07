package apihttp

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Der Lagerplatz einer Lagerbewegung muss zum gewaehlten Lager gehoeren,
// einzeln und beim Wareneingang (Backlog H.5.1).
func TestStockMovementLocationBelongsToWarehouse(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	admin := c.user("admin")
	n := uuid.NewString()[:8]
	material, wh, wh2, foreignWh := "mat-loc-"+n, "wh-loc-"+n, "wh-loc2-"+n, "wh-locf-"+n
	loc, loc2, foreignLoc := "loc-"+n, "loc2-"+n, "locf-"+n
	supplier, order, item := "sup-loc-"+n, "po-loc-"+n, "poi-loc-"+n
	ctx := context.Background()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Profil','profil','Stk',$2)`, []any{material, c.company}},
		{`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Lager',$2)`, []any{wh, c.company}},
		{`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Zweitlager',$2)`, []any{wh2, c.company}},
		{`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Fremdlager',$2)`, []any{foreignWh, other.company}},
		{`INSERT INTO locations (id, warehouse_id, code, name) VALUES ($1,$2,'A1','Regal A1')`, []any{loc, wh}},
		{`INSERT INTO locations (id, warehouse_id, code, name) VALUES ($1,$2,'B1','Regal B1')`, []any{loc2, wh2}},
		{`INSERT INTO locations (id, warehouse_id, code, name) VALUES ($1,$2,'F1','Regal F1')`, []any{foreignLoc, foreignWh}},
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Zulieferer',$2)`, []any{supplier, c.company}},
		{`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$1,'EUR','ordered',$3)`, []any{order, supplier, c.company}},
		{`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,1,$3,2,'Stk',1,'EUR')`, []any{item, order, material}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	move := func(location string) string {
		return `{"material_id":"` + material + `","warehouse_id":"` + wh + `","location_id":"` + location + `","menge":1,"einheit":"Stk","typ":"in","referenz":"loc-` + n + `"}`
	}
	locationsOf := func(where string, arg string) []string {
		t.Helper()
		rows, err := env.PG.Query(ctx, `SELECT COALESCE(location_id,'-') FROM stock_movements WHERE `+where+` ORDER BY location_id NULLS LAST`, arg)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var l string
			if err := rows.Scan(&l); err != nil {
				t.Fatal(err)
			}
			out = append(out, l)
		}
		return out
	}

	// Einzeln: Platz eines anderen Lagers oder Mandanten abgelehnt.
	for _, bad := range []string{loc2, foreignLoc, "kein-platz"} {
		rec := doAuthedRequest(t, handler, admin, http.MethodPost, "/api/v1/stock-movements/", move(bad))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Lagerort nicht gefunden") {
			t.Fatalf("Platz %s: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	// Eigener Platz gespeichert, leerer Platz heisst ohne Platz.
	for _, good := range []string{loc, " "} {
		if rec := doAuthedRequest(t, handler, admin, http.MethodPost, "/api/v1/stock-movements/", move(good)); rec.Code != http.StatusCreated {
			t.Fatalf("Platz %q: %d %s", good, rec.Code, rec.Body.String())
		}
	}
	if got := locationsOf("reference=$1", "loc-"+n); len(got) != 2 || got[0] != loc || got[1] != "-" {
		t.Fatalf("gespeicherte Plaetze: %v", got)
	}

	// Wareneingang: fremder Platz 404, nichts gebucht; eigener Platz gebucht.
	path := "/api/v1/purchase-orders/" + order + "/receive"
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPost, path, `{"warehouse_id":"`+wh+`","location_id":"`+foreignLoc+`"}`), http.StatusNotFound, "not_found", "Lagerort nicht gefunden")
	if got := locationsOf("purchase_order_item_id=$1", item); len(got) != 0 {
		t.Fatalf("nach Fehler gebucht: %v", got)
	}
	if rec := doAuthedRequest(t, handler, admin, http.MethodPost, path, `{"warehouse_id":"`+wh+`","location_id":"`+loc+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("Wareneingang: %d %s", rec.Code, rec.Body.String())
	}
	if got := locationsOf("purchase_order_item_id=$1", item); len(got) != 1 || got[0] != loc {
		t.Fatalf("Platz im Wareneingang: %v", got)
	}
}
