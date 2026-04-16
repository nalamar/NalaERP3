package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestProjectQuotePDFFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-project-quote@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-project-quote@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Projektkunde Metallbau GmbH",
		"email":    "projektkunde@example.com",
		"telefon":  "+49 30 123456",
		"waehrung": "EUR",
	})

	templateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/pdf/quote", bytes.NewReader([]byte(`{
		"header_text":"Angebotskopf Projekt",
		"footer_text":"Angebotsfuss Projekt",
		"top_first_mm":32,
		"top_other_mm":21
	}`)))
	templateReq.Header.Set("Authorization", "Bearer "+accessToken)
	templateReq.Header.Set("Content-Type", "application/json")
	templateRec := httptest.NewRecorder()
	handler.ServeHTTP(templateRec, templateReq)
	if templateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for quote template update, got %d with body %s", templateRec.Code, templateRec.Body.String())
	}

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Wohnanlage Nord",
		"kunde_id":"`+customerID+`",
		"status":"angebot"
	}`)))
	createProjectReq.Header.Set("Authorization", "Bearer "+accessToken)
	createProjectReq.Header.Set("Content-Type", "application/json")
	createProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(createProjectRec, createProjectReq)
	if createProjectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", createProjectRec.Code, createProjectRec.Body.String())
	}

	var createdProject struct {
		ID     string `json:"id"`
		Nummer string `json:"nummer"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &createdProject); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}
	if createdProject.ID == "" {
		t.Fatal("expected project id")
	}

	createPhaseReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+createdProject.ID+"/phases", bytes.NewReader([]byte(`{
		"nummer":"1",
		"name":"Los Fassade",
		"beschreibung":"Fassadenarbeiten",
		"sort_order":10
	}`)))
	createPhaseReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPhaseReq.Header.Set("Content-Type", "application/json")
	createPhaseRec := httptest.NewRecorder()
	handler.ServeHTTP(createPhaseRec, createPhaseReq)
	if createPhaseRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for phase create, got %d with body %s", createPhaseRec.Code, createPhaseRec.Body.String())
	}

	var createdPhase struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createPhaseRec.Body.Bytes(), &createdPhase); err != nil {
		t.Fatalf("decode phase create response: %v", err)
	}
	if createdPhase.ID == "" {
		t.Fatal("expected phase id")
	}

	createElevationReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+createdProject.ID+"/phases/"+createdPhase.ID+"/elevations", bytes.NewReader([]byte(`{
		"nummer":"1.1",
		"name":"Fensterelement EG",
		"beschreibung":"Pfosten-Riegel mit RAL 7016",
		"menge":4,
		"width_mm":1500,
		"height_mm":2400
	}`)))
	createElevationReq.Header.Set("Authorization", "Bearer "+accessToken)
	createElevationReq.Header.Set("Content-Type", "application/json")
	createElevationRec := httptest.NewRecorder()
	handler.ServeHTTP(createElevationRec, createElevationReq)
	if createElevationRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for elevation create, got %d with body %s", createElevationRec.Code, createElevationRec.Body.String())
	}

	pdfReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+createdProject.ID+"/quote-pdf", nil)
	pdfReq.Header.Set("Authorization", "Bearer "+accessToken)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)
	if pdfRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote pdf, got %d with body %s", pdfRec.Code, pdfRec.Body.String())
	}
	if ct := pdfRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected pdf content type, got %q", ct)
	}
	if disp := pdfRec.Header().Get("Content-Disposition"); disp == "" {
		t.Fatal("expected content disposition header")
	}
	if pdfRec.Body.Len() == 0 {
		t.Fatal("expected non-empty pdf response")
	}
}

func TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-project-context@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-project-context@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Projekt Kontext Kunde GmbH",
		"email":    "project-context@example.com",
		"telefon":  "+49 30 998877",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Projekt Kontext",
		"kunde_id":"`+customerID+`",
		"status":"angebot"
	}`)))
	createProjectReq.Header.Set("Authorization", "Bearer "+accessToken)
	createProjectReq.Header.Set("Content-Type", "application/json")
	createProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(createProjectRec, createProjectReq)
	if createProjectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", createProjectRec.Code, createProjectRec.Body.String())
	}

	var createdProject struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &createdProject); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}
	if createdProject.ID == "" {
		t.Fatal("expected project id")
	}

	createDirectQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[
			{"description":"Direktrechnungs-Element","qty":1,"unit":"Stk","unit_price":1200,"tax_code":"DE19"}
		]
	}`)))
	createDirectQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createDirectQuoteReq.Header.Set("Content-Type", "application/json")
	createDirectQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createDirectQuoteRec, createDirectQuoteReq)
	if createDirectQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for direct quote create, got %d with body %s", createDirectQuoteRec.Code, createDirectQuoteRec.Body.String())
	}

	var directQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createDirectQuoteRec.Body.Bytes(), &directQuote); err != nil {
		t.Fatalf("decode direct quote create response: %v", err)
	}

	convertDirectQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+directQuote.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-04-02T00:00:00Z",
		"due_date":"2026-04-16T00:00:00Z",
		"revenue_account":"8000"
	}`)))
	convertDirectQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertDirectQuoteReq.Header.Set("Content-Type", "application/json")
	convertDirectQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(convertDirectQuoteRec, convertDirectQuoteReq)
	if convertDirectQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for direct quote conversion, got %d with body %s", convertDirectQuoteRec.Code, convertDirectQuoteRec.Body.String())
	}

	createOrderQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[
			{"description":"Auftrags-Element","qty":2,"unit":"Stk","unit_price":750,"tax_code":"DE19"}
		]
	}`)))
	createOrderQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOrderQuoteReq.Header.Set("Content-Type", "application/json")
	createOrderQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createOrderQuoteRec, createOrderQuoteReq)
	if createOrderQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for order quote create, got %d with body %s", createOrderQuoteRec.Code, createOrderQuoteRec.Body.String())
	}

	var orderQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createOrderQuoteRec.Body.Bytes(), &orderQuote); err != nil {
		t.Fatalf("decode order quote create response: %v", err)
	}

	acceptQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+orderQuote.ID+"/accept", bytes.NewReader([]byte(`{
		"project_status":"beauftragt"
	}`)))
	acceptQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptQuoteReq.Header.Set("Content-Type", "application/json")
	acceptQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptQuoteRec, acceptQuoteReq)
	if acceptQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote accept, got %d with body %s", acceptQuoteRec.Code, acceptQuoteRec.Body.String())
	}

	convertSalesOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+orderQuote.ID+"/convert-to-sales-order", nil)
	convertSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderRec, convertSalesOrderReq)
	if convertSalesOrderRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sales order conversion, got %d with body %s", convertSalesOrderRec.Code, convertSalesOrderRec.Body.String())
	}

	var createdSalesOrder struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(convertSalesOrderRec.Body.Bytes(), &createdSalesOrder); err != nil {
		t.Fatalf("decode sales order create response: %v", err)
	}

	convertSalesOrderToInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-04-03T00:00:00Z",
		"due_date":"2026-04-17T00:00:00Z",
		"revenue_account":"8000"
	}`)))
	convertSalesOrderToInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderToInvoiceReq.Header.Set("Content-Type", "application/json")
	convertSalesOrderToInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderToInvoiceRec, convertSalesOrderToInvoiceReq)
	if convertSalesOrderToInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sales order invoice conversion, got %d with body %s", convertSalesOrderToInvoiceRec.Code, convertSalesOrderToInvoiceRec.Body.String())
	}

	contextReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+createdProject.ID+"/commercial-context", nil)
	contextReq.Header.Set("Authorization", "Bearer "+accessToken)
	contextRec := httptest.NewRecorder()
	handler.ServeHTTP(contextRec, contextReq)
	if contextRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for project commercial context, got %d with body %s", contextRec.Code, contextRec.Body.String())
	}

	var contextResp struct {
		ProjectID string `json:"project_id"`
		Quotes    []struct {
			ProjectID string `json:"project_id"`
		} `json:"quotes"`
		SalesOrders []struct {
			ProjectID string `json:"project_id"`
		} `json:"sales_orders"`
		InvoicesOut []struct {
			SourceQuoteID      *string `json:"source_quote_id"`
			SourceSalesOrderID *string `json:"source_sales_order_id"`
			GrossAmount        float64 `json:"gross_amount"`
			PaidAmount         float64 `json:"paid_amount"`
		} `json:"invoices_out"`
		Stats struct {
			QuoteCount           int     `json:"quote_count"`
			SalesOrderCount      int     `json:"sales_order_count"`
			InvoiceCount         int     `json:"invoice_count"`
			OpenInvoiceCount     int     `json:"open_invoice_count"`
			QuoteGrossTotal      float64 `json:"quote_gross_total"`
			SalesOrderGrossTotal float64 `json:"sales_order_gross_total"`
			InvoiceGrossTotal    float64 `json:"invoice_gross_total"`
			InvoiceOpenTotal     float64 `json:"invoice_open_total"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(contextRec.Body.Bytes(), &contextResp); err != nil {
		t.Fatalf("decode project commercial context response: %v", err)
	}

	if contextResp.ProjectID != createdProject.ID {
		t.Fatalf("expected project id %q, got %q", createdProject.ID, contextResp.ProjectID)
	}
	if len(contextResp.Quotes) != 2 {
		t.Fatalf("expected 2 quotes, got %d", len(contextResp.Quotes))
	}
	if len(contextResp.SalesOrders) != 1 {
		t.Fatalf("expected 1 sales order, got %d", len(contextResp.SalesOrders))
	}
	if len(contextResp.InvoicesOut) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(contextResp.InvoicesOut))
	}
	if contextResp.Stats.QuoteCount != 2 || contextResp.Stats.SalesOrderCount != 1 || contextResp.Stats.InvoiceCount != 2 {
		t.Fatalf("unexpected stats counts: %+v", contextResp.Stats)
	}
	if contextResp.Stats.OpenInvoiceCount != 2 {
		t.Fatalf("expected 2 open invoices, got %d", contextResp.Stats.OpenInvoiceCount)
	}
	if contextResp.Stats.QuoteGrossTotal <= 0 || contextResp.Stats.SalesOrderGrossTotal <= 0 || contextResp.Stats.InvoiceGrossTotal <= 0 || contextResp.Stats.InvoiceOpenTotal <= 0 {
		t.Fatalf("expected positive commercial totals, got %+v", contextResp.Stats)
	}

	foundDirectInvoice := false
	foundSalesOrderInvoice := false
	for _, inv := range contextResp.InvoicesOut {
		if inv.SourceQuoteID != nil && *inv.SourceQuoteID == directQuote.ID {
			foundDirectInvoice = true
		}
		if inv.SourceSalesOrderID != nil && *inv.SourceSalesOrderID == createdSalesOrder.ID {
			foundSalesOrderInvoice = true
		}
	}
	if !foundDirectInvoice {
		t.Fatal("expected direct quote invoice in project context")
	}
	if !foundSalesOrderInvoice {
		t.Fatal("expected sales order invoice in project context")
	}
}
