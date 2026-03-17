package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestQuoteFlowWithPricingAndPDF(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-quotes@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-quotes@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Quote Test Kunde GmbH",
		"email":    "quote-customer@example.com",
		"telefon":  "+49 211 111111",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Anbau Sued",
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

	templateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/pdf/quote", bytes.NewReader([]byte(`{
		"header_text":"Angebotskopf",
		"footer_text":"Angebotsfuss",
		"top_first_mm":31,
		"top_other_mm":20
	}`)))
	templateReq.Header.Set("Authorization", "Bearer "+accessToken)
	templateReq.Header.Set("Content-Type", "application/json")
	templateRec := httptest.NewRecorder()
	handler.ServeHTTP(templateRec, templateReq)
	if templateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for quote template update, got %d with body %s", templateRec.Code, templateRec.Body.String())
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"note":"Einmalige Sonderkonditionen",
		"items":[
			{"description":"Fensterelement A","qty":2,"unit":"Stk","unit_price":1200,"tax_code":"DE19"},
			{"description":"Montage","qty":6,"unit":"Std","unit_price":85,"tax_code":"DE19"}
		]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID          string  `json:"id"`
		Number      string  `json:"number"`
		ProjectID   string  `json:"project_id"`
		ContactID   string  `json:"contact_id"`
		ContactName string  `json:"contact_name"`
		GrossAmount float64 `json:"gross_amount"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if createdQuote.ID == "" || createdQuote.Number == "" {
		t.Fatal("expected created quote id and number")
	}
	if createdQuote.ProjectID != createdProject.ID {
		t.Fatalf("expected project id %q, got %q", createdProject.ID, createdQuote.ProjectID)
	}
	if createdQuote.ContactID != customerID {
		t.Fatalf("expected contact id %q, got %q", customerID, createdQuote.ContactID)
	}
	if createdQuote.ContactName != "Quote Test Kunde GmbH" {
		t.Fatalf("expected contact name, got %q", createdQuote.ContactName)
	}
	if createdQuote.GrossAmount <= 0 {
		t.Fatalf("expected gross amount > 0, got %v", createdQuote.GrossAmount)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/?project_id="+createdProject.ID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote list, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var list []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode quote list response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one quote list item, got %d", len(list))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote get, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Items  []struct {
			Description string  `json:"description"`
			Qty         float64 `json:"qty"`
			UnitPrice   float64 `json:"unit_price"`
		} `json:"items"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode quote get response: %v", err)
	}
	if fetched.Status != "draft" {
		t.Fatalf("expected draft status, got %q", fetched.Status)
	}
	if len(fetched.Items) != 2 {
		t.Fatalf("expected 2 quote items, got %d", len(fetched.Items))
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"contact_id":"`+customerID+`",
		"currency":"EUR",
		"note":"Aktualisierte Konditionen",
		"items":[
			{"description":"Fensterelement A","qty":3,"unit":"Stk","unit_price":1150,"tax_code":"DE19"}
		]
	}`)))
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote update, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/status", bytes.NewReader([]byte(`{"status":"sent"}`)))
	statusReq.Header.Set("Authorization", "Bearer "+accessToken)
	statusReq.Header.Set("Content-Type", "application/json")
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote status update, got %d with body %s", statusRec.Code, statusRec.Body.String())
	}

	pdfReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/pdf", nil)
	pdfReq.Header.Set("Authorization", "Bearer "+accessToken)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)
	if pdfRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote pdf, got %d with body %s", pdfRec.Code, pdfRec.Body.String())
	}
	if ct := pdfRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected pdf content type, got %q", ct)
	}
	if pdfRec.Body.Len() == 0 {
		t.Fatal("expected non-empty quote pdf response")
	}
}
