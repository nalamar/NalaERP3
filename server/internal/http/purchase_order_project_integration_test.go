package apihttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Projekt und Lieferant an der Bestellung ueber HTTP (Backlog H.4.3c):
// Feld projekt_id, Meldungen als 400/404 statt 500.
func TestPurchaseOrderProjectHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	buyer := c.user("procurement")
	n := uuid.NewString()[:8]
	supplier, customer, project := "sup-prj-"+n, "cus-prj-"+n, uuid.NewString()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Zulieferer',$2)`, []any{supplier, c.company}},
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','customer','Kunde',$2)`, []any{customer, c.company}},
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1,'P-PRJ','Projekt',$2)`, []any{project, c.company}},
	} {
		if _, err := env.PG.Exec(context.Background(), q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	type order struct {
		Bestellung struct {
			ID        string   `json:"id"`
			ProjectID *string  `json:"projekt_id"`
			Hinweise  []string `json:"hinweise"`
		} `json:"bestellung"`
	}
	created := decodeJSON[order](t, doAuthedRequest(t, handler, buyer, http.MethodPost, "/api/v1/purchase-orders/",
		`{"nummer":"PO-PRJ-`+n+`","lieferant_id":"`+supplier+`","projekt_id":"`+project+`"}`), http.StatusCreated)
	// hinweise: leere Liste statt null (Backlog H.4.3d).
	if created.Bestellung.ProjectID == nil || *created.Bestellung.ProjectID != project || created.Bestellung.Hinweise == nil {
		t.Fatalf("angelegt: %+v", created)
	}
	path := "/api/v1/purchase-orders/" + created.Bestellung.ID
	if g := decodeJSON[order](t, doAuthedRequest(t, handler, buyer, http.MethodGet, path, ""), http.StatusOK); g.Bestellung.ProjectID == nil || g.Bestellung.Hinweise == nil {
		t.Fatalf("Einzelabruf: %+v", g)
	}
	if u := decodeJSON[order](t, doAuthedRequest(t, handler, buyer, http.MethodPatch, path, `{"projekt_id":""}`), http.StatusOK); u.Bestellung.ProjectID != nil {
		t.Fatalf("entfernt: %+v", u)
	}

	for body, want := range map[string]struct {
		status    int
		code, msg string
	}{
		`{"nummer":"PO-PRJ-X1-` + n + `","lieferant_id":"` + customer + `"}`:                                         {http.StatusNotFound, "not_found", "Lieferant nicht gefunden"},
		`{"nummer":"PO-PRJ-X2-` + n + `","lieferant_id":"` + supplier + `","projekt_id":"kein-uuid"}`:                {http.StatusBadRequest, "validation_error", "Ungültige Projekt-ID"},
		`{"nummer":"PO-PRJ-X3-` + n + `","lieferant_id":"` + supplier + `","projekt_id":"` + uuid.NewString() + `"}`: {http.StatusNotFound, "not_found", "Projekt nicht gefunden"},
	} {
		wantAPIError(t, doAuthedRequest(t, handler, buyer, http.MethodPost, "/api/v1/purchase-orders/", body), want.status, want.code, want.msg)
	}
	wantAPIError(t, doAuthedRequest(t, handler, buyer, http.MethodPatch, path, `{"projekt_id":"`+uuid.NewString()+`"}`), http.StatusNotFound, "not_found", "Projekt nicht gefunden")
}
