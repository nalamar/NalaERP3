package apihttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func invoiceTypeTestCustomerBody(name string) map[string]any {
	return map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     name,
		"waehrung": "EUR",
	}
}

func firstSalesOrderItemID(t *testing.T, handler http.Handler, accessToken, orderID string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+orderID, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order get, got %d with body %s", rec.Code, rec.Body.String())
	}
	var order struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode sales order response: %v", err)
	}
	if len(order.Items) == 0 {
		t.Fatal("expected at least one sales order item")
	}
	return order.Items[0].ID
}

func TestSalesOrderConvertToInvoiceRejectsMissingInvoiceType(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-invtype-missing@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-invtype-missing@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, invoiceTypeTestCustomerBody("VOB Fehlender Typ Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/convert-to-invoice", bytes.NewReader([]byte(`{"invoice_date":"2026-03-01T00:00:00Z"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if body.Error.Message != "invoice_type ist ungültig (gültig: abschlagsrechnung, schlussrechnung)" {
		t.Fatalf("unexpected validation message, got %q", body.Error.Message)
	}
}

func TestSalesOrderConvertToInvoiceSchlussrechnungRejectsPartialQuantity(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-invtype-partial-schluss@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-invtype-partial-schluss@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, invoiceTypeTestCustomerBody("VOB Teilmenge Schluss Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)
	itemID := firstSalesOrderItemID(t, handler, accessToken, orderID)

	body := fmt.Sprintf(`{
		"invoice_date":"2026-03-01T00:00:00Z",
		"invoice_type":"schlussrechnung",
		"items":[{"sales_order_item_id":%q,"qty":0.5}]
	}`, itemID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/convert-to-invoice", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
	var respBody struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if respBody.Error.Message != "Schlussrechnung muss die gesamte Restmenge aller Positionen abrechnen" {
		t.Fatalf("unexpected validation message, got %q", respBody.Error.Message)
	}
}

func TestSalesOrderConvertToInvoiceAbschlagThenSchlussrechnungFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-invtype-flow@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-invtype-flow@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, invoiceTypeTestCustomerBody("VOB Flow Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)
	itemID := firstSalesOrderItemID(t, handler, accessToken, orderID)

	// 1. Abschlagsrechnung ueber die Haelfte der Menge (qty=1 gesamt).
	abschlagBody := fmt.Sprintf(`{
		"invoice_date":"2026-03-01T00:00:00Z",
		"invoice_type":"abschlagsrechnung",
		"items":[{"sales_order_item_id":%q,"qty":0.5}]
	}`, itemID)
	abschlagReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/convert-to-invoice", bytes.NewReader([]byte(abschlagBody)))
	abschlagReq.Header.Set("Content-Type", "application/json")
	abschlagReq.Header.Set("Authorization", "Bearer "+accessToken)
	abschlagRec := httptest.NewRecorder()
	handler.ServeHTTP(abschlagRec, abschlagReq)
	if abschlagRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for Abschlagsrechnung, got %d with body %s", abschlagRec.Code, abschlagRec.Body.String())
	}
	var abschlagResult struct {
		Invoice struct {
			ID          string `json:"id"`
			InvoiceType string `json:"invoice_type"`
		} `json:"invoice"`
	}
	if err := json.Unmarshal(abschlagRec.Body.Bytes(), &abschlagResult); err != nil {
		t.Fatalf("decode abschlag response: %v", err)
	}
	if abschlagResult.Invoice.InvoiceType != "abschlagsrechnung" {
		t.Fatalf("expected invoice_type abschlagsrechnung, got %q", abschlagResult.Invoice.InvoiceType)
	}

	// 2. Versuch einer weiteren Abschlagsrechnung ist erlaubt (noch keine Schlussrechnung).
	// 3. Schlussrechnung ueber die verbleibende Restmenge (ohne explizite items -> voller Rest).
	schlussBody := `{"invoice_date":"2026-03-02T00:00:00Z","invoice_type":"schlussrechnung"}`
	schlussReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/convert-to-invoice", bytes.NewReader([]byte(schlussBody)))
	schlussReq.Header.Set("Content-Type", "application/json")
	schlussReq.Header.Set("Authorization", "Bearer "+accessToken)
	schlussRec := httptest.NewRecorder()
	handler.ServeHTTP(schlussRec, schlussReq)
	if schlussRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for Schlussrechnung, got %d with body %s", schlussRec.Code, schlussRec.Body.String())
	}
	var schlussResult struct {
		Invoice struct {
			ID          string  `json:"id"`
			InvoiceType string  `json:"invoice_type"`
			NetAmount   float64 `json:"net_amount"`
		} `json:"invoice"`
	}
	if err := json.Unmarshal(schlussRec.Body.Bytes(), &schlussResult); err != nil {
		t.Fatalf("decode schluss response: %v", err)
	}
	if schlussResult.Invoice.InvoiceType != "schlussrechnung" {
		t.Fatalf("expected invoice_type schlussrechnung, got %q", schlussResult.Invoice.InvoiceType)
	}
	// Restmenge war 0.5 * 100 = 50 - die Schlussrechnung darf NUR den Rest
	// enthalten, nicht nochmal die volle Summe (Beweis, dass keine
	// Doppelverrechnung stattfindet).
	if schlussResult.Invoice.NetAmount != 50 {
		t.Fatalf("expected schlussrechnung net_amount 50 (only the remaining half), got %v", schlussResult.Invoice.NetAmount)
	}

	// 4. Jede weitere Konvertierung (Abschlag ODER erneute Schlussrechnung) wird jetzt abgelehnt.
	furtherBody := `{"invoice_date":"2026-03-03T00:00:00Z","invoice_type":"abschlagsrechnung"}`
	furtherReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/convert-to-invoice", bytes.NewReader([]byte(furtherBody)))
	furtherReq.Header.Set("Content-Type", "application/json")
	furtherReq.Header.Set("Authorization", "Bearer "+accessToken)
	furtherRec := httptest.NewRecorder()
	handler.ServeHTTP(furtherRec, furtherReq)
	if furtherRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for conversion after Schlussrechnung, got %d with body %s", furtherRec.Code, furtherRec.Body.String())
	}
	var furtherBodyResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(furtherRec.Body.Bytes(), &furtherBodyResp); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if furtherBodyResp.Error.Message != "für diesen Auftrag existiert bereits eine Schlussrechnung" {
		t.Fatalf("unexpected validation message, got %q", furtherBodyResp.Error.Message)
	}

	// 5. Direkte SQL-Pruefung: beide Rechnungen tragen den korrekten Typ.
	var types []string
	rows, err := env.PG.Query(t.Context(), `SELECT invoice_type FROM invoices_out WHERE source_sales_order_id=$1 ORDER BY invoice_date`, orderID)
	if err != nil {
		t.Fatalf("query invoice types: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it string
		if err := rows.Scan(&it); err != nil {
			t.Fatalf("scan invoice type: %v", err)
		}
		types = append(types, it)
	}
	if len(types) != 2 || types[0] != "abschlagsrechnung" || types[1] != "schlussrechnung" {
		t.Fatalf("expected [abschlagsrechnung schlussrechnung], got %#v", types)
	}
}

func TestSalesOrderConvertToInvoiceListFilterByInvoiceType(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-invtype-filter@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-invtype-filter@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, invoiceTypeTestCustomerBody("VOB Filter Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)

	convertBody := `{"invoice_date":"2026-03-01T00:00:00Z","invoice_type":"schlussrechnung"}`
	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/convert-to-invoice", bytes.NewReader([]byte(convertBody)))
	convertReq.Header.Set("Content-Type", "application/json")
	convertReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/?invoice_type=schlussrechnung&source_sales_order_id="+orderID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		InvoiceType string `json:"invoice_type"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 || listed[0].InvoiceType != "schlussrechnung" {
		t.Fatalf("expected exactly one schlussrechnung in filtered list, got %#v", listed)
	}
}
