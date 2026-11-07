package apihttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Anfrage prueft den Lieferanten auf Rolle supplier/both, beim Erfassen
// der Offerte und bei der Umwandlung (Backlog H.5.2, ADR 0017
// Praezisierung H.5.2).
func TestRFQChecksSupplierRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	admin := c.user("admin")
	ctx := context.Background()
	n := uuid.NewString()[:8]
	material := "mat-rfq-" + n
	contact := func(company, rolle string) string {
		t.Helper()
		id := "ct-" + rolle + "-" + uuid.NewString()[:8]
		if _, err := env.PG.Exec(ctx, `INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org',$2,'Kontakt',$3)`, id, rolle, company); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Profil','profil','Stk',$2)`, material, c.company); err != nil {
		t.Fatal(err)
	}
	// Eigener Nummernkreis: ohne ihn bekaeme der neue Mandant "PO-{YYYY}-0001"
	// und stiesse mit anderen Mandanten zusammen (Befund H.5.5).
	if _, err := env.PG.Exec(ctx, `INSERT INTO number_sequences (entity, pattern, next_value, company_id) VALUES ('purchase_order', $1, 1, $2)`,
		"BE-T-"+n+"-{NNNN}", c.company); err != nil {
		t.Fatal(err)
	}
	customer, partner, supplier, both := contact(c.company, "customer"), contact(c.company, "partner"), contact(c.company, "supplier"), contact(c.company, "both")
	foreign := contact(other.company, "supplier")

	type created struct {
		RFQ struct {
			ID string `json:"id"`
		} `json:"rfq"`
		Positionen []struct {
			ID string `json:"id"`
		} `json:"positionen"`
	}
	newRFQ := func() (string, string) {
		t.Helper()
		r := decodeJSON[created](t, doAuthedRequest(t, handler, admin, http.MethodPost, "/api/v1/rfqs/",
			`{"nummer":"RFQ-T-`+n+`","positionen":[{"material_id":"`+material+`","bezeichnung":"Profil","menge":2,"einheit":"Stk"}]}`), http.StatusCreated)
		return r.RFQ.ID, r.Positionen[0].ID
	}
	quote := func(item, supplierID string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, admin, http.MethodPost, "/api/v1/rfqs/items/"+item+"/quotes",
			fmt.Sprintf(`{"lieferant_id":%q,"preis":10,"waehrung":"EUR"}`, supplierID))
	}
	convert := func(rfq, supplierID string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, admin, http.MethodPost, "/api/v1/rfqs/"+rfq+"/convert-to-purchase-order",
			`{"lieferant_id":"`+supplierID+`"}`)
	}
	count := func(sql string, arg string) int {
		t.Helper()
		var k int
		if err := env.PG.QueryRow(ctx, sql, arg).Scan(&k); err != nil {
			t.Fatal(err)
		}
		return k
	}

	rfq, item := newRFQ()
	// Offerte: Kunde, Partner, fremder Mandant und unbekannt abgelehnt.
	for _, bad := range []string{customer, partner, foreign, "gibt-es-nicht"} {
		wantAPIError(t, quote(item, bad), http.StatusNotFound, "not_found", "Lieferant nicht gefunden")
	}
	if k := count(`SELECT count(*) FROM rfq_supplier_quotes WHERE rfq_item_id=$1`, item); k != 0 {
		t.Fatalf("nach Ablehnung %d Offerten", k)
	}
	for _, good := range []string{supplier, both} {
		if r := quote(item, good); r.Code != http.StatusCreated {
			t.Fatalf("Offerte %s: %d %s", good, r.Code, r.Body.String())
		}
	}

	// Umwandlung: eine Offerte eines Kunden aus der Zeit vor H.5.2 reicht
	// nicht; ein fremder Lieferant ist nicht gefunden (statt "kein Angebot").
	if _, err := env.PG.Exec(ctx, `INSERT INTO rfq_supplier_quotes (id, rfq_item_id, supplier_id, unit_price, currency) VALUES ($1,$2,$3,9,'EUR')`,
		uuid.NewString(), item, customer); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{customer, foreign} {
		wantAPIError(t, convert(rfq, bad), http.StatusNotFound, "not_found", "Lieferant nicht gefunden")
	}
	if k := count(`SELECT count(*) FROM purchase_orders WHERE company_id=$1`, c.company); k != 0 {
		t.Fatalf("nach Ablehnung %d Bestellungen", k)
	}
	if r := convert(rfq, both); r.Code != http.StatusCreated {
		t.Fatalf("Umwandlung mit both: %d %s", r.Code, r.Body.String())
	}
	if k := count(`SELECT count(*) FROM purchase_orders WHERE company_id=$1 AND supplier_id='`+both+`'`, c.company); k != 1 {
		t.Fatalf("Bestellungen fuer both: %d", k)
	}
}
