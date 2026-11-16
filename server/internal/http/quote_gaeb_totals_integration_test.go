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

	"nalaerp3/internal/testutil"
)

// TestQuoteTotalsSkipAlternativeAndProvisionalItems: Alternativ- und
// Bedarfspositionen behalten ihren Preis, zählen aber nicht zur
// Angebotssumme und nicht zu den Soll-Kosten (ADR 0041 Frage 4, I.2.4d1).
func TestQuoteTotalsSkipAlternativeAndProvisionalItems(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	user := "integration-quote-totals-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Summen Kunde "+suffix))
	projectID := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, kunde_id, status, company_id) VALUES ($1,$2,'Summen Projekt',$3,'angebot','default')`,
		projectID, "PRJ-SUM-"+suffix, customerID); err != nil {
		t.Fatal(err)
	}

	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	type quoteResp struct {
		ID          string  `json:"id"`
		NetAmount   float64 `json:"net_amount"`
		TaxAmount   float64 `json:"tax_amount"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			ID       string `json:"id"`
			ItemKind string `json:"item_kind"`
		} `json:"items"`
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.005 }
	wantTotals := func(stage string, q quoteResp, net float64) {
		t.Helper()
		if !near(q.NetAmount, net) || !near(q.TaxAmount, net*0.19) || !near(q.GrossAmount, net*1.19) {
			t.Fatalf("%s: netto %v, steuer %v, brutto %v; erwartet netto %v", stage, q.NetAmount, q.TaxAmount, q.GrossAmount, net)
		}
	}
	items := `[
		{"description":"Normal","qty":2,"unit":"Stk","unit_price":100,"tax_code":"DE19"},
		{"description":"Grund","qty":1,"unit":"Stk","unit_price":50,"tax_code":"DE19","outline_no":"01.0010","item_kind":"grundposition"},
		{"description":"Alternative","qty":1,"unit":"Stk","unit_price":60,"tax_code":"DE19","outline_no":"01.0020","item_kind":"alternativposition","base_outline_no":"01.0010"},
		{"description":"Bedarf mit GB","qty":1,"unit":"Stk","unit_price":30,"tax_code":"DE19","item_kind":"bedarfsposition_mit_gb"},
		{"description":"Bedarf ohne GB","qty":1,"unit":"Stk","unit_price":20,"tax_code":"DE19","item_kind":"bedarfsposition_ohne_gb"}
	]`
	body := fmt.Sprintf(`{"contact_id":%q,"project_id":%q,"currency":"EUR","items":%s}`, customerID, projectID, items)
	var q quoteResp
	rec := do(http.MethodPost, "/api/v1/quotes/", body)
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &q) != nil {
		t.Fatalf("Anlegen: %d %s", rec.Code, rec.Body.String())
	}
	wantTotals("Anlegen", q, 250)
	// Gesamtbetrag je Position bleibt gespeichert, auch für nicht zählende Positionen
	var allNet float64
	if err := env.PG.QueryRow(ctx, `SELECT COALESCE(SUM(net_amount),0) FROM quote_items WHERE quote_id=$1`, q.ID).Scan(&allNet); err != nil || !near(allNet, 360) {
		t.Fatalf("Positionsbeträge: %v %v", allNet, err)
	}
	rec = do(http.MethodPatch, "/api/v1/quotes/"+q.ID, body)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &q) != nil {
		t.Fatalf("Speichern: %d %s", rec.Code, rec.Body.String())
	}
	wantTotals("Speichern", q, 250)

	itemByKind := map[string]string{}
	for _, it := range q.Items {
		if it.ItemKind == "" {
			t.Fatalf("Position ohne Art: %+v", it)
		}
		itemByKind[it.ItemKind] = it.ID
	}
	// Kalkulation (Einheitspreis 314) anwenden: Neuberechnung der Summe aus den Positionen
	calc := `{"material_cost":100,"material_zuschlag_percent":15,"lohn_stunden":2,"lohn_stundensatz":40,"lohn_zuschlag_percent":80,"fremdleistung_cost":50,"fremdleistung_zuschlag_percent":10}`
	applyCalc := func(itemID string) quoteResp {
		t.Helper()
		if rec := do(http.MethodPut, "/api/v1/quotes/"+q.ID+"/items/"+itemID+"/calculation", calc); rec.Code != http.StatusOK {
			t.Fatalf("Kalkulation: %d %s", rec.Code, rec.Body.String())
		}
		rec := do(http.MethodPost, "/api/v1/quotes/"+q.ID+"/items/"+itemID+"/apply-calculation", "")
		var out quoteResp
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("Kalkulation anwenden: %d %s", rec.Code, rec.Body.String())
		}
		return out
	}
	q = applyCalc(itemByKind["alternativposition"])
	wantTotals("Kalkulation an der Alternative", q, 250)
	q = applyCalc(itemByKind["grundposition"])
	wantTotals("Kalkulation an der Grundposition", q, 200+314)

	// Soll-Kosten im Controlling: nur die Grundposition (314 × 1), nicht die Alternative
	var controlling struct {
		SollErloes float64 `json:"soll_erloes"`
		SollKosten float64 `json:"soll_kosten"`
	}
	rec = do(http.MethodGet, "/api/v1/projects/"+projectID+"/controlling", "")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &controlling) != nil {
		t.Fatalf("Controlling: %d %s", rec.Code, rec.Body.String())
	}
	if !near(controlling.SollKosten, 314) || !near(controlling.SollErloes, (200+314)*1.19) {
		t.Fatalf("Controlling: %+v", controlling)
	}
}
