package apihttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// TestQuoteFollowUpDocumentsSkipAlternativeAndProvisionalItems: Auftrag und
// Rechnung aus einem Angebot übernehmen nur Normal- und Grundpositionen
// (ADR 0041 Frage 5, Backlog I.2.4e2).
func TestQuoteFollowUpDocumentsSkipAlternativeAndProvisionalItems(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	suffix := uuid.NewString()[:8]
	user := "integration-quote-followup-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Folgebeleg Kunde "+suffix))

	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	items := `[
		{"description":"Normal","qty":2,"unit":"Stk","unit_price":100,"tax_code":"DE19"},
		{"description":"Grund","qty":1,"unit":"Stk","unit_price":50,"tax_code":"DE19","outline_no":"01.0010","item_kind":"grundposition"},
		{"description":"Alternative","qty":1,"unit":"Stk","unit_price":60,"tax_code":"DE19","outline_no":"01.0020","item_kind":"alternativposition","base_outline_no":"01.0010"},
		{"description":"Bedarf mit GB","qty":1,"unit":"Stk","unit_price":30,"tax_code":"DE19","item_kind":"bedarfsposition_mit_gb"},
		{"description":"Bedarf ohne GB","qty":1,"unit":"Stk","unit_price":20,"tax_code":"DE19","item_kind":"bedarfsposition_ohne_gb"}
	]`
	newQuote := func(status string) string {
		t.Helper()
		rec := do(http.MethodPost, "/api/v1/quotes/", fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":%s}`, customerID, items))
		var q struct {
			ID string `json:"id"`
		}
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &q) != nil {
			t.Fatalf("Angebot: %d %s", rec.Code, rec.Body.String())
		}
		if rec := do(http.MethodPost, "/api/v1/quotes/"+q.ID+"/status", fmt.Sprintf(`{"status":%q}`, status)); rec.Code != http.StatusOK {
			t.Fatalf("Status: %d %s", rec.Code, rec.Body.String())
		}
		return q.ID
	}
	type doc struct {
		NetAmount   float64 `json:"net_amount"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			Description string  `json:"description"`
			Qty         float64 `json:"qty"`
			UnitPrice   float64 `json:"unit_price"`
		} `json:"items"`
	}
	check := func(stage string, d doc) {
		t.Helper()
		if len(d.Items) != 2 || d.Items[0].Description != "Normal" || d.Items[1].Description != "Grund" ||
			math.Abs(d.NetAmount-250) > 0.005 || math.Abs(d.GrossAmount-297.5) > 0.005 {
			t.Fatalf("%s: %+v", stage, d)
		}
	}

	// Auftrag aus angenommenem Angebot
	rec := do(http.MethodPost, "/api/v1/quotes/"+newQuote("accepted")+"/convert-to-sales-order", "")
	var order doc
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &order) != nil {
		t.Fatalf("Auftrag: %d %s", rec.Code, rec.Body.String())
	}
	check("Auftrag", order)

	// Rechnung aus versendetem Angebot
	rec = do(http.MethodPost, "/api/v1/quotes/"+newQuote("sent")+"/convert-to-invoice", `{"revenue_account":"8000"}`)
	var invoice struct {
		Invoice doc `json:"invoice"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &invoice) != nil {
		t.Fatalf("Rechnung: %d %s", rec.Code, rec.Body.String())
	}
	check("Rechnung", invoice.Invoice)
}
