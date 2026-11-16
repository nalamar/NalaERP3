package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/auth"
	"nalaerp3/internal/testutil"
)

// TestQuoteToSalesOrderWithAlternativeSelection: gewählte Alternativen
// ersetzen im Auftrag ihre Grundposition, OZ und Herkunft stehen an der
// Auftragsposition; fremder Mandant kann nicht umwandeln (ADR 0042, I.2.6a).
func TestQuoteToSalesOrderWithAlternativeSelection(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	user := "integration-order-selection-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Auswahl Kunde "+suffix))

	do := func(token, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	type quoteItem struct {
		ID        string `json:"id"`
		OutlineNo string `json:"outline_no"`
	}
	items := `[
		{"description":"Normal","qty":2,"unit":"Stk","unit_price":100,"tax_code":"DE19"},
		{"description":"Grund","qty":1,"unit":"Stk","unit_price":50,"tax_code":"DE19","outline_no":"01.0010","item_kind":"grundposition"},
		{"description":"Alternative A","qty":1,"unit":"Stk","unit_price":60,"tax_code":"DE19","outline_no":"01.0020","item_kind":"alternativposition","base_outline_no":"01.0010"},
		{"description":"Alternative B","qty":1,"unit":"Stk","unit_price":70,"tax_code":"DE19","outline_no":"01.0030","item_kind":"alternativposition","base_outline_no":"01.0010"},
		{"description":"Bedarf","qty":1,"unit":"Stk","unit_price":30,"tax_code":"DE19","outline_no":"01.0040","item_kind":"bedarfsposition_mit_gb"}
	]`
	newAcceptedQuote := func() (string, map[string]string) {
		t.Helper()
		rec := do(accessToken, http.MethodPost, "/api/v1/quotes/", fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":%s}`, customerID, items))
		var q struct {
			ID    string      `json:"id"`
			Items []quoteItem `json:"items"`
		}
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &q) != nil {
			t.Fatalf("Angebot: %d %s", rec.Code, rec.Body.String())
		}
		if rec := do(accessToken, http.MethodPost, "/api/v1/quotes/"+q.ID+"/status", `{"status":"accepted"}`); rec.Code != http.StatusOK {
			t.Fatalf("Status: %d %s", rec.Code, rec.Body.String())
		}
		byOZ := map[string]string{}
		for _, it := range q.Items {
			byOZ[it.OutlineNo] = it.ID
		}
		return q.ID, byOZ
	}
	type orderItem struct {
		Description       string `json:"description"`
		OutlineNo         string `json:"outline_no"`
		SourceQuoteItemID string `json:"source_quote_item_id"`
	}
	type order struct {
		ID        string      `json:"id"`
		NetAmount float64     `json:"net_amount"`
		Items     []orderItem `json:"items"`
	}

	// Alternative B statt Grundposition
	quoteID, byOZ := newAcceptedQuote()
	rec := do(accessToken, http.MethodPost, "/api/v1/quotes/"+quoteID+"/convert-to-sales-order", fmt.Sprintf(`{"alternatives":[%q]}`, byOZ["01.0030"]))
	var o order
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &o) != nil {
		t.Fatalf("Auftrag: %d %s", rec.Code, rec.Body.String())
	}
	if len(o.Items) != 2 || o.Items[0].Description != "Normal" || o.Items[1].Description != "Alternative B" ||
		o.Items[1].OutlineNo != "01.0030" || o.Items[1].SourceQuoteItemID != byOZ["01.0030"] ||
		o.Items[0].OutlineNo != "" || o.Items[0].SourceQuoteItemID == "" || math.Abs(o.NetAmount-270) > 0.005 {
		t.Fatalf("Auftrag mit Alternative: %+v", o)
	}
	// Lesen liefert dieselbe Herkunft
	rec = do(accessToken, http.MethodGet, "/api/v1/sales-orders/"+o.ID, "")
	var read order
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &read) != nil || len(read.Items) != 2 ||
		read.Items[1].OutlineNo != "01.0030" || read.Items[1].SourceQuoteItemID != byOZ["01.0030"] {
		t.Fatalf("Auftrag lesen: %d %s", rec.Code, rec.Body.String())
	}

	// ohne Auswahl: Grundposition, keine Alternative
	quoteID, byOZ = newAcceptedQuote()
	rec = do(accessToken, http.MethodPost, "/api/v1/quotes/"+quoteID+"/convert-to-sales-order", "")
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &o) != nil || len(o.Items) != 2 ||
		o.Items[1].Description != "Grund" || o.Items[1].SourceQuoteItemID != byOZ["01.0010"] || math.Abs(o.NetAmount-250) > 0.005 {
		t.Fatalf("ohne Auswahl: %d %s", rec.Code, rec.Body.String())
	}

	// ungültige Auswahl: 400, kein Auftrag
	quoteID, byOZ = newAcceptedQuote()
	for name, body := range map[string]string{
		"zwei Alternativen": fmt.Sprintf(`{"alternatives":[%q,%q]}`, byOZ["01.0020"], byOZ["01.0030"]),
		"Bedarfsposition":   fmt.Sprintf(`{"alternatives":[%q]}`, byOZ["01.0040"]),
		"fremde Position":   fmt.Sprintf(`{"alternatives":[%q]}`, uuid.NewString()),
		"kein JSON":         `{"alternatives":`,
	} {
		if rec := do(accessToken, http.MethodPost, "/api/v1/quotes/"+quoteID+"/convert-to-sales-order", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	var linked *string
	if err := env.PG.QueryRow(ctx, `SELECT linked_sales_order_id::text FROM quotes WHERE id=$1`, quoteID).Scan(&linked); err != nil || linked != nil {
		t.Fatalf("Angebot nach abgelehnter Auswahl verknüpft: %v %v", linked, err)
	}

	// fremder Mandant: Angebot nicht gefunden, kein Auftrag
	otherCompany := "itest-order-selection-" + suffix
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, 'Andere Firma')`, otherCompany); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("Secret123!")
	if err != nil {
		t.Fatal(err)
	}
	otherUser := "itest-order-selection-user-" + suffix
	otherEmail := "integration-order-selection-other-" + suffix + "@example.com"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO users (id, email, username, password_hash, first_name, last_name, display_name, locale, timezone, is_active, is_locked, company_id)
		VALUES ($1,$2,$2,$3,'Andere','Firma','Andere Firma','de-DE','Europe/Berlin',true,false,$4)`, otherUser, otherEmail, hash, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, r.id FROM roles r WHERE r.code = 'admin'`, otherUser); err != nil {
		t.Fatal(err)
	}
	otherToken := loginIntegrationUser(t, handler, otherEmail, "Secret123!")
	if rec := do(otherToken, http.MethodPost, "/api/v1/quotes/"+quoteID+"/convert-to-sales-order", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("fremder Mandant: %d %s", rec.Code, rec.Body.String())
	}
	var foreignOrders int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM sales_orders WHERE source_quote_id=$1`, quoteID).Scan(&foreignOrders); err != nil || foreignOrders != 0 {
		t.Fatalf("Aufträge für das Angebot: %d %v", foreignOrders, err)
	}
}
