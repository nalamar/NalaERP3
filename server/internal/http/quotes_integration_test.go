package apihttp

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

func TestQuoteFlowWithPricingAndPDF(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-quotes@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-sales@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-quotes@example.com", "Secret123!")
	salesAccessToken := loginIntegrationUser(t, handler, "integration-sales@example.com", "Secret123!")

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

	salesOrderTemplateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/pdf/sales_order", bytes.NewReader([]byte(`{
		"header_text":"Auftragskopf",
		"footer_text":"Auftragsfuss",
		"top_first_mm":32,
		"top_other_mm":20
	}`)))
	salesOrderTemplateReq.Header.Set("Authorization", "Bearer "+accessToken)
	salesOrderTemplateReq.Header.Set("Content-Type", "application/json")
	salesOrderTemplateRec := httptest.NewRecorder()
	handler.ServeHTTP(salesOrderTemplateRec, salesOrderTemplateReq)
	if salesOrderTemplateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for sales_order template update, got %d with body %s", salesOrderTemplateRec.Code, salesOrderTemplateRec.Body.String())
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
		ID                  string  `json:"id"`
		Number              string  `json:"number"`
		RootQuoteID         string  `json:"root_quote_id"`
		RevisionNo          int     `json:"revision_no"`
		SupersededByQuoteID string  `json:"superseded_by_quote_id"`
		ProjectID           string  `json:"project_id"`
		ContactID           string  `json:"contact_id"`
		ContactName         string  `json:"contact_name"`
		GrossAmount         float64 `json:"gross_amount"`
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
	if createdQuote.RootQuoteID != createdQuote.ID {
		t.Fatalf("expected root quote id %q, got %q", createdQuote.ID, createdQuote.RootQuoteID)
	}
	if createdQuote.RevisionNo != 1 {
		t.Fatalf("expected revision_no 1, got %d", createdQuote.RevisionNo)
	}
	if createdQuote.SupersededByQuoteID != "" {
		t.Fatalf("expected no superseded_by_quote_id on new quote, got %q", createdQuote.SupersededByQuoteID)
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

	var list []struct {
		ID                  string `json:"id"`
		RootQuoteID         string `json:"root_quote_id"`
		RevisionNo          int    `json:"revision_no"`
		SupersededByQuoteID string `json:"superseded_by_quote_id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode quote list response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one quote list item, got %d", len(list))
	}
	if list[0].ID != createdQuote.ID || list[0].RootQuoteID != createdQuote.ID || list[0].RevisionNo != 1 || list[0].SupersededByQuoteID != "" {
		t.Fatalf("expected revision metadata on quote list item, got %+v", list[0])
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote get, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID                  string `json:"id"`
		RootQuoteID         string `json:"root_quote_id"`
		RevisionNo          int    `json:"revision_no"`
		SupersededByQuoteID string `json:"superseded_by_quote_id"`
		Status              string `json:"status"`
		Items               []struct {
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
	if fetched.RootQuoteID != createdQuote.ID || fetched.RevisionNo != 1 || fetched.SupersededByQuoteID != "" {
		t.Fatalf("expected revision metadata on quote get, got root=%q rev=%d superseded=%q", fetched.RootQuoteID, fetched.RevisionNo, fetched.SupersededByQuoteID)
	}
	if len(fetched.Items) != 2 {
		t.Fatalf("expected 2 quote items, got %d", len(fetched.Items))
	}

	createAcceptQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"note":"Annahme ohne Sofortrechnung",
		"items":[
			{"description":"Wartungsvertrag","qty":1,"unit":"Pauschale","unit_price":650,"tax_code":"DE19"}
		]
	}`)))
	createAcceptQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createAcceptQuoteReq.Header.Set("Content-Type", "application/json")
	createAcceptQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createAcceptQuoteRec, createAcceptQuoteReq)
	if createAcceptQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for second quote create, got %d with body %s", createAcceptQuoteRec.Code, createAcceptQuoteRec.Body.String())
	}

	var acceptQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAcceptQuoteRec.Body.Bytes(), &acceptQuote); err != nil {
		t.Fatalf("decode second quote create response: %v", err)
	}
	if acceptQuote.ID == "" {
		t.Fatal("expected second quote id")
	}

	forbiddenAcceptReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+acceptQuote.ID+"/accept", bytes.NewReader([]byte(`{
		"project_status":"beauftragt"
	}`)))
	forbiddenAcceptReq.Header.Set("Authorization", "Bearer "+salesAccessToken)
	forbiddenAcceptReq.Header.Set("Content-Type", "application/json")
	forbiddenAcceptRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenAcceptRec, forbiddenAcceptReq)
	if forbiddenAcceptRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for quote acceptance with project update without projects.write, got %d with body %s", forbiddenAcceptRec.Code, forbiddenAcceptRec.Body.String())
	}

	acceptReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+acceptQuote.ID+"/accept", bytes.NewReader([]byte(`{
		"project_status":"beauftragt"
	}`)))
	acceptReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote accept, got %d with body %s", acceptRec.Code, acceptRec.Body.String())
	}

	var accepted struct {
		Quote struct {
			ID                 string `json:"id"`
			Status             string `json:"status"`
			AcceptedAt         string `json:"accepted_at"`
			ProjectID          string `json:"project_id"`
			LinkedSalesOrderID string `json:"linked_sales_order_id"`
		} `json:"quote"`
		Project struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"project"`
	}
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode quote accept response: %v", err)
	}
	if accepted.Quote.Status != "accepted" {
		t.Fatalf("expected accepted status after explicit acceptance, got %q", accepted.Quote.Status)
	}
	if accepted.Quote.AcceptedAt == "" {
		t.Fatal("expected accepted_at on accepted quote")
	}
	if accepted.Project.ID != createdProject.ID || accepted.Project.Status != "beauftragt" {
		t.Fatalf("expected project %q in status beauftragt, got id=%q status=%q", createdProject.ID, accepted.Project.ID, accepted.Project.Status)
	}
	if accepted.Quote.LinkedSalesOrderID != "" {
		t.Fatalf("expected no sales order link immediately after acceptance, got %q", accepted.Quote.LinkedSalesOrderID)
	}

	convertSalesOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+acceptQuote.ID+"/convert-to-sales-order", nil)
	convertSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderRec, convertSalesOrderReq)
	if convertSalesOrderRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote conversion to sales order, got %d with body %s", convertSalesOrderRec.Code, convertSalesOrderRec.Body.String())
	}

	var createdSalesOrder struct {
		ID            string  `json:"id"`
		Number        string  `json:"number"`
		SourceQuoteID string  `json:"source_quote_id"`
		ProjectID     string  `json:"project_id"`
		ContactID     string  `json:"contact_id"`
		Status        string  `json:"status"`
		GrossAmount   float64 `json:"gross_amount"`
	}
	if err := json.Unmarshal(convertSalesOrderRec.Body.Bytes(), &createdSalesOrder); err != nil {
		t.Fatalf("decode sales order create response: %v", err)
	}
	if createdSalesOrder.ID == "" || createdSalesOrder.Number == "" {
		t.Fatal("expected sales order id and number")
	}
	if createdSalesOrder.SourceQuoteID != acceptQuote.ID {
		t.Fatalf("expected source quote id %q, got %q", acceptQuote.ID, createdSalesOrder.SourceQuoteID)
	}
	if createdSalesOrder.ProjectID != createdProject.ID {
		t.Fatalf("expected sales order project id %q, got %q", createdProject.ID, createdSalesOrder.ProjectID)
	}
	if createdSalesOrder.ContactID != customerID {
		t.Fatalf("expected sales order contact id %q, got %q", customerID, createdSalesOrder.ContactID)
	}
	if createdSalesOrder.Status != "open" {
		t.Fatalf("expected sales order status open, got %q", createdSalesOrder.Status)
	}
	if createdSalesOrder.GrossAmount <= 0 {
		t.Fatalf("expected sales order gross amount > 0, got %v", createdSalesOrder.GrossAmount)
	}

	getAcceptedQuoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+acceptQuote.ID, nil)
	getAcceptedQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	getAcceptedQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(getAcceptedQuoteRec, getAcceptedQuoteReq)
	if getAcceptedQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for accepted quote get after sales order conversion, got %d with body %s", getAcceptedQuoteRec.Code, getAcceptedQuoteRec.Body.String())
	}

	var acceptedWithSalesOrder struct {
		ID                 string `json:"id"`
		Status             string `json:"status"`
		LinkedSalesOrderID string `json:"linked_sales_order_id"`
	}
	if err := json.Unmarshal(getAcceptedQuoteRec.Body.Bytes(), &acceptedWithSalesOrder); err != nil {
		t.Fatalf("decode accepted quote after sales order conversion: %v", err)
	}
	if acceptedWithSalesOrder.LinkedSalesOrderID != createdSalesOrder.ID {
		t.Fatalf("expected linked sales order id %q, got %q", createdSalesOrder.ID, acceptedWithSalesOrder.LinkedSalesOrderID)
	}

	getSalesOrderReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+createdSalesOrder.ID, nil)
	getSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	getSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(getSalesOrderRec, getSalesOrderReq)
	if getSalesOrderRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order get, got %d with body %s", getSalesOrderRec.Code, getSalesOrderRec.Body.String())
	}

	var fetchedSalesOrder struct {
		ID            string `json:"id"`
		SourceQuoteID string `json:"source_quote_id"`
		Status        string `json:"status"`
		Items         []struct {
			Description string `json:"description"`
		} `json:"items"`
	}
	if err := json.Unmarshal(getSalesOrderRec.Body.Bytes(), &fetchedSalesOrder); err != nil {
		t.Fatalf("decode sales order get response: %v", err)
	}
	if fetchedSalesOrder.ID != createdSalesOrder.ID {
		t.Fatalf("expected sales order id %q, got %q", createdSalesOrder.ID, fetchedSalesOrder.ID)
	}
	if fetchedSalesOrder.SourceQuoteID != acceptQuote.ID {
		t.Fatalf("expected fetched sales order source quote id %q, got %q", acceptQuote.ID, fetchedSalesOrder.SourceQuoteID)
	}
	if fetchedSalesOrder.Status != "open" {
		t.Fatalf("expected fetched sales order status open, got %q", fetchedSalesOrder.Status)
	}
	if len(fetchedSalesOrder.Items) != 1 {
		t.Fatalf("expected 1 sales order item, got %d", len(fetchedSalesOrder.Items))
	}

	updateSalesOrderReq := httptest.NewRequest(http.MethodPatch, "/api/v1/sales-orders/"+createdSalesOrder.ID, bytes.NewReader([]byte(`{
		"number":"AUF-MANUELL-001",
		"order_date":"2026-03-19T00:00:00Z",
		"currency":"chf",
		"note":"Montage vor Ort abstimmen"
	}`)))
	updateSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateSalesOrderReq.Header.Set("Content-Type", "application/json")
	updateSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(updateSalesOrderRec, updateSalesOrderReq)
	if updateSalesOrderRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order update, got %d with body %s", updateSalesOrderRec.Code, updateSalesOrderRec.Body.String())
	}

	var updatedSalesOrder struct {
		ID          string  `json:"id"`
		Number      string  `json:"number"`
		OrderDate   string  `json:"order_date"`
		Currency    string  `json:"currency"`
		Note        string  `json:"note"`
		NetAmount   float64 `json:"net_amount"`
		TaxAmount   float64 `json:"tax_amount"`
		GrossAmount float64 `json:"gross_amount"`
	}
	if err := json.Unmarshal(updateSalesOrderRec.Body.Bytes(), &updatedSalesOrder); err != nil {
		t.Fatalf("decode sales order update response: %v", err)
	}
	if updatedSalesOrder.ID != createdSalesOrder.ID {
		t.Fatalf("expected updated sales order id %q, got %q", createdSalesOrder.ID, updatedSalesOrder.ID)
	}
	if updatedSalesOrder.Number != "AUF-MANUELL-001" {
		t.Fatalf("expected updated sales order number, got %q", updatedSalesOrder.Number)
	}
	if updatedSalesOrder.Currency != "CHF" {
		t.Fatalf("expected normalized sales order currency CHF, got %q", updatedSalesOrder.Currency)
	}
	if updatedSalesOrder.Note != "Montage vor Ort abstimmen" {
		t.Fatalf("expected updated sales order note, got %q", updatedSalesOrder.Note)
	}
	if updatedSalesOrder.GrossAmount != createdSalesOrder.GrossAmount {
		t.Fatalf("expected unchanged gross amount %v after header update, got %v", createdSalesOrder.GrossAmount, updatedSalesOrder.GrossAmount)
	}

	createSalesOrderItemReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items", bytes.NewReader([]byte(`{
		"description":"Montagepauschale",
		"qty":2,
		"unit":"Std",
		"unit_price":150,
		"tax_code":"DE19"
	}`)))
	createSalesOrderItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	createSalesOrderItemReq.Header.Set("Content-Type", "application/json")
	createSalesOrderItemRec := httptest.NewRecorder()
	handler.ServeHTTP(createSalesOrderItemRec, createSalesOrderItemReq)
	if createSalesOrderItemRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sales order item create, got %d with body %s", createSalesOrderItemRec.Code, createSalesOrderItemRec.Body.String())
	}

	var createdSalesOrderItem struct {
		Item struct {
			ID          string  `json:"id"`
			Position    int     `json:"position"`
			Description string  `json:"description"`
			Qty         float64 `json:"qty"`
			UnitPrice   float64 `json:"unit_price"`
		} `json:"item"`
		SalesOrder struct {
			ID          string  `json:"id"`
			NetAmount   float64 `json:"net_amount"`
			TaxAmount   float64 `json:"tax_amount"`
			GrossAmount float64 `json:"gross_amount"`
			Items       []struct {
				ID          string `json:"id"`
				Description string `json:"description"`
			} `json:"items"`
		} `json:"sales_order"`
	}
	if err := json.Unmarshal(createSalesOrderItemRec.Body.Bytes(), &createdSalesOrderItem); err != nil {
		t.Fatalf("decode sales order item create response: %v", err)
	}
	if createdSalesOrderItem.Item.ID == "" || createdSalesOrderItem.Item.Position != 2 {
		t.Fatalf("expected created sales order item id and position 2, got %#v", createdSalesOrderItem.Item)
	}
	if len(createdSalesOrderItem.SalesOrder.Items) != 2 {
		t.Fatalf("expected 2 sales order items after create, got %d", len(createdSalesOrderItem.SalesOrder.Items))
	}

	updateSalesOrderItemReq := httptest.NewRequest(http.MethodPatch, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items/"+createdSalesOrderItem.Item.ID, bytes.NewReader([]byte(`{
		"qty":3,
		"unit_price":175,
		"description":"Montagepauschale erweitert"
	}`)))
	updateSalesOrderItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateSalesOrderItemReq.Header.Set("Content-Type", "application/json")
	updateSalesOrderItemRec := httptest.NewRecorder()
	handler.ServeHTTP(updateSalesOrderItemRec, updateSalesOrderItemReq)
	if updateSalesOrderItemRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order item update, got %d with body %s", updateSalesOrderItemRec.Code, updateSalesOrderItemRec.Body.String())
	}

	var updatedSalesOrderItem struct {
		Item struct {
			ID          string  `json:"id"`
			Description string  `json:"description"`
			Qty         float64 `json:"qty"`
			UnitPrice   float64 `json:"unit_price"`
		} `json:"item"`
		SalesOrder struct {
			GrossAmount float64 `json:"gross_amount"`
		} `json:"sales_order"`
	}
	if err := json.Unmarshal(updateSalesOrderItemRec.Body.Bytes(), &updatedSalesOrderItem); err != nil {
		t.Fatalf("decode sales order item update response: %v", err)
	}
	if updatedSalesOrderItem.Item.Description != "Montagepauschale erweitert" || updatedSalesOrderItem.Item.Qty != 3 || updatedSalesOrderItem.Item.UnitPrice != 175 {
		t.Fatalf("expected updated sales order item payload, got %#v", updatedSalesOrderItem.Item)
	}

	deleteSalesOrderItemReq := httptest.NewRequest(http.MethodDelete, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items/"+createdSalesOrderItem.Item.ID, nil)
	deleteSalesOrderItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteSalesOrderItemRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteSalesOrderItemRec, deleteSalesOrderItemReq)
	if deleteSalesOrderItemRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order item delete, got %d with body %s", deleteSalesOrderItemRec.Code, deleteSalesOrderItemRec.Body.String())
	}

	var salesOrderAfterDelete struct {
		ID          string  `json:"id"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			ID       string `json:"id"`
			Position int    `json:"position"`
		} `json:"items"`
	}
	if err := json.Unmarshal(deleteSalesOrderItemRec.Body.Bytes(), &salesOrderAfterDelete); err != nil {
		t.Fatalf("decode sales order item delete response: %v", err)
	}
	if len(salesOrderAfterDelete.Items) != 1 {
		t.Fatalf("expected 1 sales order item after delete, got %d", len(salesOrderAfterDelete.Items))
	}
	if salesOrderAfterDelete.Items[0].Position != 1 {
		t.Fatalf("expected remaining sales order item to be resequenced to position 1, got %d", salesOrderAfterDelete.Items[0].Position)
	}
	if salesOrderAfterDelete.GrossAmount != createdSalesOrder.GrossAmount {
		t.Fatalf("expected gross amount to return to original %v after delete, got %v", createdSalesOrder.GrossAmount, salesOrderAfterDelete.GrossAmount)
	}

	invalidSalesOrderItemReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items", bytes.NewReader([]byte(`{
		"description":"Ungültige Steuerposition",
		"qty":1,
		"unit":"Stk",
		"unit_price":10,
		"tax_code":"XX99"
	}`)))
	invalidSalesOrderItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	invalidSalesOrderItemReq.Header.Set("Content-Type", "application/json")
	invalidSalesOrderItemRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidSalesOrderItemRec, invalidSalesOrderItemReq)
	if invalidSalesOrderItemRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sales order tax code, got %d with body %s", invalidSalesOrderItemRec.Code, invalidSalesOrderItemRec.Body.String())
	}

	deleteLastSalesOrderItemReq := httptest.NewRequest(http.MethodDelete, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items/"+salesOrderAfterDelete.Items[0].ID, nil)
	deleteLastSalesOrderItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteLastSalesOrderItemRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteLastSalesOrderItemRec, deleteLastSalesOrderItemReq)
	if deleteLastSalesOrderItemRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for deleting last sales order item, got %d with body %s", deleteLastSalesOrderItemRec.Code, deleteLastSalesOrderItemRec.Body.String())
	}

	createPartialSalesOrderItemReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items", bytes.NewReader([]byte(`{
		"description":"Teilfaktura Position",
		"qty":4,
		"unit":"Std",
		"unit_price":100,
		"tax_code":"DE19"
	}`)))
	createPartialSalesOrderItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPartialSalesOrderItemReq.Header.Set("Content-Type", "application/json")
	createPartialSalesOrderItemRec := httptest.NewRecorder()
	handler.ServeHTTP(createPartialSalesOrderItemRec, createPartialSalesOrderItemReq)
	if createPartialSalesOrderItemRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for partial sales order item create, got %d with body %s", createPartialSalesOrderItemRec.Code, createPartialSalesOrderItemRec.Body.String())
	}

	var partialSalesOrderItem struct {
		Item struct {
			ID       string  `json:"id"`
			Position int     `json:"position"`
			Qty      float64 `json:"qty"`
		} `json:"item"`
	}
	if err := json.Unmarshal(createPartialSalesOrderItemRec.Body.Bytes(), &partialSalesOrderItem); err != nil {
		t.Fatalf("decode partial sales order item create response: %v", err)
	}
	if partialSalesOrderItem.Item.ID == "" || partialSalesOrderItem.Item.Position != 2 || partialSalesOrderItem.Item.Qty != 4 {
		t.Fatalf("expected partial sales order item with id, position 2 and qty 4, got %#v", partialSalesOrderItem.Item)
	}

	salesOrderPDFReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/pdf", nil)
	salesOrderPDFReq.Header.Set("Authorization", "Bearer "+accessToken)
	salesOrderPDFRec := httptest.NewRecorder()
	handler.ServeHTTP(salesOrderPDFRec, salesOrderPDFReq)
	if salesOrderPDFRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order pdf, got %d with body %s", salesOrderPDFRec.Code, salesOrderPDFRec.Body.String())
	}
	if ct := salesOrderPDFRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected sales order pdf content type, got %q", ct)
	}
	if salesOrderPDFRec.Body.Len() == 0 {
		t.Fatal("expected non-empty sales order pdf response")
	}

	releaseSalesOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/status", bytes.NewReader([]byte(`{"status":"released"}`)))
	releaseSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	releaseSalesOrderReq.Header.Set("Content-Type", "application/json")
	releaseSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(releaseSalesOrderRec, releaseSalesOrderReq)
	if releaseSalesOrderRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order release, got %d with body %s", releaseSalesOrderRec.Code, releaseSalesOrderRec.Body.String())
	}

	var releasedSalesOrder struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(releaseSalesOrderRec.Body.Bytes(), &releasedSalesOrder); err != nil {
		t.Fatalf("decode released sales order response: %v", err)
	}
	if releasedSalesOrder.ID != createdSalesOrder.ID || releasedSalesOrder.Status != "released" {
		t.Fatalf("expected released sales order %q, got id=%q status=%q", createdSalesOrder.ID, releasedSalesOrder.ID, releasedSalesOrder.Status)
	}

	forbiddenSalesOrderConvertReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"revenue_account":"8000"
	}`)))
	forbiddenSalesOrderConvertReq.Header.Set("Authorization", "Bearer "+salesAccessToken)
	forbiddenSalesOrderConvertReq.Header.Set("Content-Type", "application/json")
	forbiddenSalesOrderConvertRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenSalesOrderConvertRec, forbiddenSalesOrderConvertReq)
	if forbiddenSalesOrderConvertRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for sales order conversion without invoices_out.write, got %d with body %s", forbiddenSalesOrderConvertRec.Code, forbiddenSalesOrderConvertRec.Body.String())
	}

	convertSalesOrderToInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-03-18T00:00:00Z",
		"due_date":"2026-04-01T00:00:00Z",
		"revenue_account":"8000",
		"items":[
			{"sales_order_item_id":"`+partialSalesOrderItem.Item.ID+`","qty":2}
		]
	}`)))
	convertSalesOrderToInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderToInvoiceReq.Header.Set("Content-Type", "application/json")
	convertSalesOrderToInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderToInvoiceRec, convertSalesOrderToInvoiceReq)
	if convertSalesOrderToInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sales order conversion to invoice, got %d with body %s", convertSalesOrderToInvoiceRec.Code, convertSalesOrderToInvoiceRec.Body.String())
	}

	var convertedSalesOrder struct {
		SalesOrder struct {
			ID                 string `json:"id"`
			Status             string `json:"status"`
			LinkedInvoiceOutID string `json:"linked_invoice_out_id"`
		} `json:"sales_order"`
		Invoice struct {
			ID                 string  `json:"id"`
			Status             string  `json:"status"`
			ContactID          string  `json:"contact_id"`
			Currency           string  `json:"currency"`
			GrossAmount        float64 `json:"gross_amount"`
			SourceQuoteID      *string `json:"source_quote_id"`
			SourceSalesOrderID *string `json:"source_sales_order_id"`
		} `json:"invoice"`
	}
	if err := json.Unmarshal(convertSalesOrderToInvoiceRec.Body.Bytes(), &convertedSalesOrder); err != nil {
		t.Fatalf("decode sales order conversion response: %v", err)
	}
	if convertedSalesOrder.SalesOrder.ID != createdSalesOrder.ID || convertedSalesOrder.SalesOrder.Status != "invoiced" {
		t.Fatalf("expected invoiced sales order %q, got id=%q status=%q", createdSalesOrder.ID, convertedSalesOrder.SalesOrder.ID, convertedSalesOrder.SalesOrder.Status)
	}
	if convertedSalesOrder.SalesOrder.LinkedInvoiceOutID == "" {
		t.Fatal("expected linked invoice id on converted sales order")
	}
	if convertedSalesOrder.Invoice.ID == "" || convertedSalesOrder.Invoice.Status != "draft" {
		t.Fatalf("expected draft invoice from sales order conversion, got id=%q status=%q", convertedSalesOrder.Invoice.ID, convertedSalesOrder.Invoice.Status)
	}
	if convertedSalesOrder.Invoice.ContactID != customerID {
		t.Fatalf("expected invoice contact id %q, got %q", customerID, convertedSalesOrder.Invoice.ContactID)
	}
	if convertedSalesOrder.Invoice.Currency != "CHF" {
		t.Fatalf("expected invoice currency CHF from updated sales order, got %q", convertedSalesOrder.Invoice.Currency)
	}
	if convertedSalesOrder.Invoice.GrossAmount < 237.99 || convertedSalesOrder.Invoice.GrossAmount > 238.01 {
		t.Fatalf("expected partial invoice gross amount about 238, got %v", convertedSalesOrder.Invoice.GrossAmount)
	}
	if convertedSalesOrder.Invoice.SourceQuoteID == nil || *convertedSalesOrder.Invoice.SourceQuoteID != acceptQuote.ID {
		t.Fatalf("expected invoice source quote id %q, got %v", acceptQuote.ID, convertedSalesOrder.Invoice.SourceQuoteID)
	}
	if convertedSalesOrder.Invoice.SourceSalesOrderID == nil || *convertedSalesOrder.Invoice.SourceSalesOrderID != createdSalesOrder.ID {
		t.Fatalf("expected invoice source sales order id %q, got %v", createdSalesOrder.ID, convertedSalesOrder.Invoice.SourceSalesOrderID)
	}
	if convertedSalesOrder.SalesOrder.LinkedInvoiceOutID != convertedSalesOrder.Invoice.ID {
		t.Fatalf("expected sales order linked invoice id %q, got %q", convertedSalesOrder.Invoice.ID, convertedSalesOrder.SalesOrder.LinkedInvoiceOutID)
	}

	getConvertedSalesOrderReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+createdSalesOrder.ID, nil)
	getConvertedSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConvertedSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(getConvertedSalesOrderRec, getConvertedSalesOrderReq)
	if getConvertedSalesOrderRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for converted sales order get, got %d with body %s", getConvertedSalesOrderRec.Code, getConvertedSalesOrderRec.Body.String())
	}

	var fetchedConvertedSalesOrder struct {
		Status             string `json:"status"`
		LinkedInvoiceOutID string `json:"linked_invoice_out_id"`
	}
	if err := json.Unmarshal(getConvertedSalesOrderRec.Body.Bytes(), &fetchedConvertedSalesOrder); err != nil {
		t.Fatalf("decode converted sales order get response: %v", err)
	}
	if fetchedConvertedSalesOrder.Status != "invoiced" {
		t.Fatalf("expected invoiced sales order on get, got %q", fetchedConvertedSalesOrder.Status)
	}
	if fetchedConvertedSalesOrder.LinkedInvoiceOutID != convertedSalesOrder.Invoice.ID {
		t.Fatalf("expected persisted linked invoice id %q, got %q", convertedSalesOrder.Invoice.ID, fetchedConvertedSalesOrder.LinkedInvoiceOutID)
	}

	createItemAfterInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/items", bytes.NewReader([]byte(`{
		"description":"Sperrtest nach Faktura",
		"qty":1,
		"unit":"Std",
		"unit_price":25,
		"tax_code":"DE19"
	}`)))
	createItemAfterInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	createItemAfterInvoiceReq.Header.Set("Content-Type", "application/json")
	createItemAfterInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(createItemAfterInvoiceRec, createItemAfterInvoiceReq)
	if createItemAfterInvoiceRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for item create after invoice, got %d with body %s", createItemAfterInvoiceRec.Code, createItemAfterInvoiceRec.Body.String())
	}

	convertSalesOrderToInvoiceAgainReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-03-20T00:00:00Z",
		"due_date":"2026-04-03T00:00:00Z",
		"revenue_account":"8000"
	}`)))
	convertSalesOrderToInvoiceAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderToInvoiceAgainReq.Header.Set("Content-Type", "application/json")
	convertSalesOrderToInvoiceAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderToInvoiceAgainRec, convertSalesOrderToInvoiceAgainReq)
	if convertSalesOrderToInvoiceAgainRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for second sales order invoice conversion, got %d with body %s", convertSalesOrderToInvoiceAgainRec.Code, convertSalesOrderToInvoiceAgainRec.Body.String())
	}

	var secondConvertedSalesOrder struct {
		SalesOrder struct {
			ID                 string `json:"id"`
			Status             string `json:"status"`
			LinkedInvoiceOutID string `json:"linked_invoice_out_id"`
		} `json:"sales_order"`
		Invoice struct {
			ID          string  `json:"id"`
			GrossAmount float64 `json:"gross_amount"`
		} `json:"invoice"`
	}
	if err := json.Unmarshal(convertSalesOrderToInvoiceAgainRec.Body.Bytes(), &secondConvertedSalesOrder); err != nil {
		t.Fatalf("decode second sales order conversion response: %v", err)
	}
	if secondConvertedSalesOrder.Invoice.ID == "" || secondConvertedSalesOrder.SalesOrder.LinkedInvoiceOutID != secondConvertedSalesOrder.Invoice.ID {
		t.Fatalf("expected latest linked invoice id %q, got %q", secondConvertedSalesOrder.Invoice.ID, secondConvertedSalesOrder.SalesOrder.LinkedInvoiceOutID)
	}
	if secondConvertedSalesOrder.Invoice.GrossAmount < 1011.49 || secondConvertedSalesOrder.Invoice.GrossAmount > 1011.51 {
		t.Fatalf("expected remaining invoice gross amount about 1011.5, got %v", secondConvertedSalesOrder.Invoice.GrossAmount)
	}

	listInvoicesBySalesOrderReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/?source_sales_order_id="+createdSalesOrder.ID, nil)
	listInvoicesBySalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	listInvoicesBySalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(listInvoicesBySalesOrderRec, listInvoicesBySalesOrderReq)
	if listInvoicesBySalesOrderRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice list by source sales order, got %d with body %s", listInvoicesBySalesOrderRec.Code, listInvoicesBySalesOrderRec.Body.String())
	}

	var invoicesBySalesOrder []struct {
		ID                 string  `json:"id"`
		SourceSalesOrderID *string `json:"source_sales_order_id"`
		GrossAmount        float64 `json:"gross_amount"`
	}
	if err := json.Unmarshal(listInvoicesBySalesOrderRec.Body.Bytes(), &invoicesBySalesOrder); err != nil {
		t.Fatalf("decode invoice list by source sales order: %v", err)
	}
	if len(invoicesBySalesOrder) != 2 {
		t.Fatalf("expected two invoices for source sales order, got %d", len(invoicesBySalesOrder))
	}
	if invoicesBySalesOrder[0].ID != secondConvertedSalesOrder.Invoice.ID {
		t.Fatalf("expected latest invoice %q first in source sales order list, got %q", secondConvertedSalesOrder.Invoice.ID, invoicesBySalesOrder[0].ID)
	}
	if invoicesBySalesOrder[1].ID != convertedSalesOrder.Invoice.ID {
		t.Fatalf("expected first partial invoice %q second in source sales order list, got %q", convertedSalesOrder.Invoice.ID, invoicesBySalesOrder[1].ID)
	}
	for idx, inv := range invoicesBySalesOrder {
		if inv.SourceSalesOrderID == nil || *inv.SourceSalesOrderID != createdSalesOrder.ID {
			t.Fatalf("expected invoice %d source sales order id %q, got %v", idx, createdSalesOrder.ID, inv.SourceSalesOrderID)
		}
	}

	listSalesOrdersReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/?project_id="+createdProject.ID, nil)
	listSalesOrdersReq.Header.Set("Authorization", "Bearer "+accessToken)
	listSalesOrdersRec := httptest.NewRecorder()
	handler.ServeHTTP(listSalesOrdersRec, listSalesOrdersReq)
	if listSalesOrdersRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order list, got %d with body %s", listSalesOrdersRec.Code, listSalesOrdersRec.Body.String())
	}

	var listedSalesOrders []struct {
		ID                   string  `json:"id"`
		RelatedInvoiceCount  int     `json:"related_invoice_count"`
		RemainingGrossAmount float64 `json:"remaining_gross_amount"`
	}
	if err := json.Unmarshal(listSalesOrdersRec.Body.Bytes(), &listedSalesOrders); err != nil {
		t.Fatalf("decode sales order list: %v", err)
	}
	foundListedSalesOrder := false
	for _, listed := range listedSalesOrders {
		if listed.ID != createdSalesOrder.ID {
			continue
		}
		foundListedSalesOrder = true
		if listed.RelatedInvoiceCount != 2 {
			t.Fatalf("expected related invoice count 2 for sales order list item, got %d", listed.RelatedInvoiceCount)
		}
		if listed.RemainingGrossAmount < -0.01 || listed.RemainingGrossAmount > 0.01 {
			t.Fatalf("expected remaining gross amount about 0 for fully invoiced sales order list item, got %v", listed.RemainingGrossAmount)
		}
	}
	if !foundListedSalesOrder {
		t.Fatalf("expected created sales order %q in list response", createdSalesOrder.ID)
	}

	getAcceptedQuoteAfterOrderInvoiceReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+acceptQuote.ID, nil)
	getAcceptedQuoteAfterOrderInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	getAcceptedQuoteAfterOrderInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(getAcceptedQuoteAfterOrderInvoiceRec, getAcceptedQuoteAfterOrderInvoiceReq)
	if getAcceptedQuoteAfterOrderInvoiceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for accepted quote get after sales order invoice conversion, got %d with body %s", getAcceptedQuoteAfterOrderInvoiceRec.Code, getAcceptedQuoteAfterOrderInvoiceRec.Body.String())
	}

	var acceptedQuoteAfterOrderInvoice struct {
		Status             string `json:"status"`
		LinkedInvoiceOutID string `json:"linked_invoice_out_id"`
	}
	if err := json.Unmarshal(getAcceptedQuoteAfterOrderInvoiceRec.Body.Bytes(), &acceptedQuoteAfterOrderInvoice); err != nil {
		t.Fatalf("decode accepted quote after sales order invoice conversion: %v", err)
	}
	if acceptedQuoteAfterOrderInvoice.Status != "accepted" {
		t.Fatalf("expected accepted quote status after sales order invoice conversion, got %q", acceptedQuoteAfterOrderInvoice.Status)
	}
	if acceptedQuoteAfterOrderInvoice.LinkedInvoiceOutID != secondConvertedSalesOrder.Invoice.ID {
		t.Fatalf("expected quote linked invoice id %q, got %q", secondConvertedSalesOrder.Invoice.ID, acceptedQuoteAfterOrderInvoice.LinkedInvoiceOutID)
	}

	completeSalesOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/status", bytes.NewReader([]byte(`{"status":"completed"}`)))
	completeSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	completeSalesOrderReq.Header.Set("Content-Type", "application/json")
	completeSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(completeSalesOrderRec, completeSalesOrderReq)
	if completeSalesOrderRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order completion, got %d with body %s", completeSalesOrderRec.Code, completeSalesOrderRec.Body.String())
	}

	reopenSalesOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/status", bytes.NewReader([]byte(`{"status":"open"}`)))
	reopenSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	reopenSalesOrderReq.Header.Set("Content-Type", "application/json")
	reopenSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(reopenSalesOrderRec, reopenSalesOrderReq)
	if reopenSalesOrderRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for reopening completed sales order, got %d with body %s", reopenSalesOrderRec.Code, reopenSalesOrderRec.Body.String())
	}

	convertSalesOrderToInvoiceThirdReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+createdSalesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{}`)))
	convertSalesOrderToInvoiceThirdReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderToInvoiceThirdReq.Header.Set("Content-Type", "application/json")
	convertSalesOrderToInvoiceThirdRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderToInvoiceThirdRec, convertSalesOrderToInvoiceThirdReq)
	if convertSalesOrderToInvoiceThirdRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for fully billed sales order invoice conversion, got %d with body %s", convertSalesOrderToInvoiceThirdRec.Code, convertSalesOrderToInvoiceThirdRec.Body.String())
	}

	revertAcceptedReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+acceptQuote.ID+"/status", bytes.NewReader([]byte(`{"status":"rejected"}`)))
	revertAcceptedReq.Header.Set("Authorization", "Bearer "+accessToken)
	revertAcceptedReq.Header.Set("Content-Type", "application/json")
	revertAcceptedRec := httptest.NewRecorder()
	handler.ServeHTTP(revertAcceptedRec, revertAcceptedReq)
	if revertAcceptedRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for manual status change after sales order conversion, got %d with body %s", revertAcceptedRec.Code, revertAcceptedRec.Body.String())
	}

	convertSalesOrderAgainReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+acceptQuote.ID+"/convert-to-sales-order", nil)
	convertSalesOrderAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderAgainRec, convertSalesOrderAgainReq)
	if convertSalesOrderAgainRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate sales order conversion, got %d with body %s", convertSalesOrderAgainRec.Code, convertSalesOrderAgainRec.Body.String())
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

	forbiddenConvertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"revenue_account":"8000"
	}`)))
	forbiddenConvertReq.Header.Set("Authorization", "Bearer "+salesAccessToken)
	forbiddenConvertReq.Header.Set("Content-Type", "application/json")
	forbiddenConvertRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenConvertRec, forbiddenConvertReq)
	if forbiddenConvertRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for quote conversion without invoices_out.write, got %d with body %s", forbiddenConvertRec.Code, forbiddenConvertRec.Body.String())
	}

	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-03-17T00:00:00Z",
		"due_date":"2026-03-31T00:00:00Z",
		"revenue_account":"8000"
	}`)))
	convertReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertReq.Header.Set("Content-Type", "application/json")
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote conversion, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}

	var converted struct {
		Quote struct {
			ID                 string `json:"id"`
			Status             string `json:"status"`
			LinkedInvoiceOutID string `json:"linked_invoice_out_id"`
		} `json:"quote"`
		Invoice struct {
			ID                 string  `json:"id"`
			Status             string  `json:"status"`
			ContactID          string  `json:"contact_id"`
			SourceQuoteID      *string `json:"source_quote_id"`
			SourceSalesOrderID *string `json:"source_sales_order_id"`
		} `json:"invoice"`
	}
	if err := json.Unmarshal(convertRec.Body.Bytes(), &converted); err != nil {
		t.Fatalf("decode quote conversion response: %v", err)
	}
	if converted.Quote.Status != "accepted" {
		t.Fatalf("expected accepted quote after conversion, got %q", converted.Quote.Status)
	}
	if converted.Quote.LinkedInvoiceOutID == "" {
		t.Fatal("expected linked invoice id on quote")
	}
	if converted.Invoice.ID == "" || converted.Invoice.Status != "draft" {
		t.Fatalf("expected draft invoice from conversion, got id=%q status=%q", converted.Invoice.ID, converted.Invoice.Status)
	}
	if converted.Invoice.ContactID != customerID {
		t.Fatalf("expected invoice contact id %q, got %q", customerID, converted.Invoice.ContactID)
	}
	if converted.Invoice.SourceQuoteID == nil || *converted.Invoice.SourceQuoteID != createdQuote.ID {
		t.Fatalf("expected invoice source quote id %q, got %v", createdQuote.ID, converted.Invoice.SourceQuoteID)
	}
	if converted.Invoice.SourceSalesOrderID != nil {
		t.Fatalf("expected no source sales order on direct quote conversion, got %v", converted.Invoice.SourceSalesOrderID)
	}
	if converted.Quote.LinkedInvoiceOutID != converted.Invoice.ID {
		t.Fatalf("expected quote linked invoice id %q, got %q", converted.Invoice.ID, converted.Quote.LinkedInvoiceOutID)
	}

	getConvertedQuoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	getConvertedQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConvertedQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(getConvertedQuoteRec, getConvertedQuoteReq)
	if getConvertedQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for converted quote get, got %d with body %s", getConvertedQuoteRec.Code, getConvertedQuoteRec.Body.String())
	}

	var convertedQuote struct {
		Status             string `json:"status"`
		LinkedInvoiceOutID string `json:"linked_invoice_out_id"`
	}
	if err := json.Unmarshal(getConvertedQuoteRec.Body.Bytes(), &convertedQuote); err != nil {
		t.Fatalf("decode converted quote get response: %v", err)
	}
	if convertedQuote.Status != "accepted" {
		t.Fatalf("expected accepted quote on get, got %q", convertedQuote.Status)
	}
	if convertedQuote.LinkedInvoiceOutID != converted.Invoice.ID {
		t.Fatalf("expected persisted linked invoice id %q, got %q", converted.Invoice.ID, convertedQuote.LinkedInvoiceOutID)
	}

	getInvoiceReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/"+converted.Invoice.ID, nil)
	getInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	getInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(getInvoiceRec, getInvoiceReq)
	if getInvoiceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for converted invoice get, got %d with body %s", getInvoiceRec.Code, getInvoiceRec.Body.String())
	}

	var convertedInvoice struct {
		ID                 string  `json:"id"`
		ContactID          string  `json:"contact_id"`
		SourceQuoteID      *string `json:"source_quote_id"`
		SourceSalesOrderID *string `json:"source_sales_order_id"`
	}
	if err := json.Unmarshal(getInvoiceRec.Body.Bytes(), &convertedInvoice); err != nil {
		t.Fatalf("decode converted invoice get response: %v", err)
	}
	if convertedInvoice.ID != converted.Invoice.ID {
		t.Fatalf("expected invoice id %q, got %q", converted.Invoice.ID, convertedInvoice.ID)
	}
	if convertedInvoice.ContactID != customerID {
		t.Fatalf("expected invoice contact id %q, got %q", customerID, convertedInvoice.ContactID)
	}
	if convertedInvoice.SourceQuoteID == nil || *convertedInvoice.SourceQuoteID != createdQuote.ID {
		t.Fatalf("expected persisted source quote id %q, got %v", createdQuote.ID, convertedInvoice.SourceQuoteID)
	}
	if convertedInvoice.SourceSalesOrderID != nil {
		t.Fatalf("expected no source sales order on persisted quote conversion, got %v", convertedInvoice.SourceSalesOrderID)
	}

	revertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/status", bytes.NewReader([]byte(`{"status":"draft"}`)))
	revertReq.Header.Set("Authorization", "Bearer "+accessToken)
	revertReq.Header.Set("Content-Type", "application/json")
	revertRec := httptest.NewRecorder()
	handler.ServeHTTP(revertRec, revertReq)
	if revertRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for manual status revert after conversion, got %d with body %s", revertRec.Code, revertRec.Body.String())
	}

	convertAgainReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{}`)))
	convertAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertAgainReq.Header.Set("Content-Type", "application/json")
	convertAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(convertAgainRec, convertAgainReq)
	if convertAgainRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate quote conversion, got %d with body %s", convertAgainRec.Code, convertAgainRec.Body.String())
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

func TestQuoteUpdateAllowsManualMaterialMappingOnItems(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-quote-mapping@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-quote-mapping@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Quote Mapping Kunde GmbH",
		"email":    "quote-mapping@example.com",
		"telefon":  "+49 211 333333",
		"waehrung": "EUR",
	})

	createMaterialReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-GAEB-0001",
		"bezeichnung":"Aluminium Profil 70mm",
		"einheit":"m"
	}`)))
	createMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMaterialReq.Header.Set("Content-Type", "application/json")
	createMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(createMaterialRec, createMaterialReq)
	if createMaterialRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for material create, got %d with body %s", createMaterialRec.Code, createMaterialRec.Body.String())
	}

	var createdMaterial struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMaterialRec.Body.Bytes(), &createdMaterial); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"contact_id":"`+customerID+`",
		"currency":"EUR",
		"note":"Quote fuer manuelles Mapping",
		"items":[{"description":"GAEB Position","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID        string `json:"id"`
		ContactID string `json:"contact_id"`
		QuoteDate string `json:"quote_date"`
		Items     []struct {
			PriceMappingStatus      string `json:"price_mapping_status"`
			MaterialID              string `json:"material_id"`
			MaterialCandidateStatus string `json:"material_candidate_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if createdQuote.ID == "" || createdQuote.ContactID != customerID || createdQuote.QuoteDate == "" {
		t.Fatalf("unexpected created quote payload: %+v", createdQuote)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].PriceMappingStatus != "open" || createdQuote.Items[0].MaterialID != "" || createdQuote.Items[0].MaterialCandidateStatus != "none" {
		t.Fatalf("expected default open mapping status without material, got %+v", createdQuote.Items)
	}

	updateQuoteReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"contact_id":"`+customerID+`",
		"quote_date":"`+createdQuote.QuoteDate+`",
		"currency":"EUR",
		"note":"Mapping gesetzt",
		"items":[{"description":"GAEB Position","qty":1,"unit":"Stk","unit_price":0,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	updateQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateQuoteReq.Header.Set("Content-Type", "application/json")
	updateQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(updateQuoteRec, updateQuoteReq)
	if updateQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote update with mapping, got %d with body %s", updateQuoteRec.Code, updateQuoteRec.Body.String())
	}

	var updatedQuote struct {
		ID    string `json:"id"`
		Items []struct {
			Description             string `json:"description"`
			MaterialID              string `json:"material_id"`
			PriceMappingStatus      string `json:"price_mapping_status"`
			MaterialCandidateStatus string `json:"material_candidate_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(updateQuoteRec.Body.Bytes(), &updatedQuote); err != nil {
		t.Fatalf("decode quote update response: %v", err)
	}
	if updatedQuote.ID != createdQuote.ID || len(updatedQuote.Items) != 1 {
		t.Fatalf("unexpected updated quote payload: %+v", updatedQuote)
	}
	if updatedQuote.Items[0].MaterialID != createdMaterial.ID || updatedQuote.Items[0].PriceMappingStatus != "manual" || updatedQuote.Items[0].MaterialCandidateStatus != "none" {
		t.Fatalf("expected manual mapping on quote item, got %+v", updatedQuote.Items[0])
	}

	getQuoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	getQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	getQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(getQuoteRec, getQuoteReq)
	if getQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote get, got %d with body %s", getQuoteRec.Code, getQuoteRec.Body.String())
	}

	var fetchedQuote struct {
		Items []struct {
			MaterialID              string `json:"material_id"`
			PriceMappingStatus      string `json:"price_mapping_status"`
			MaterialCandidateStatus string `json:"material_candidate_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(getQuoteRec.Body.Bytes(), &fetchedQuote); err != nil {
		t.Fatalf("decode quote get response: %v", err)
	}
	if len(fetchedQuote.Items) != 1 || fetchedQuote.Items[0].MaterialID != createdMaterial.ID || fetchedQuote.Items[0].PriceMappingStatus != "manual" || fetchedQuote.Items[0].MaterialCandidateStatus != "none" {
		t.Fatalf("expected fetched quote item mapping, got %+v", fetchedQuote.Items)
	}

	invalidStatusReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"contact_id":"`+customerID+`",
		"quote_date":"`+createdQuote.QuoteDate+`",
		"currency":"EUR",
		"note":"Ungueltiger Mapping-Status",
		"items":[{"description":"GAEB Position","qty":1,"unit":"Stk","unit_price":0,"tax_code":"","price_mapping_status":"auto"}]
	}`)))
	invalidStatusReq.Header.Set("Authorization", "Bearer "+accessToken)
	invalidStatusReq.Header.Set("Content-Type", "application/json")
	invalidStatusRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidStatusRec, invalidStatusReq)
	if invalidStatusRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid price_mapping_status, got %d with body %s", invalidStatusRec.Code, invalidStatusRec.Body.String())
	}

	invalidMaterialReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"contact_id":"`+customerID+`",
		"quote_date":"`+createdQuote.QuoteDate+`",
		"currency":"EUR",
		"note":"Ungueltiges Material",
		"items":[{"description":"GAEB Position","qty":1,"unit":"Stk","unit_price":0,"tax_code":"","material_id":"missing-material","price_mapping_status":"manual"}]
	}`)))
	invalidMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	invalidMaterialReq.Header.Set("Content-Type", "application/json")
	invalidMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidMaterialRec, invalidMaterialReq)
	if invalidMaterialRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid material_id, got %d with body %s", invalidMaterialRec.Code, invalidMaterialRec.Body.String())
	}
}

func TestQuoteGAEBImportFlowCreatesImportRunAndListsIt(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-import@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-import@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Import Kunde GmbH",
		"email":    "gaeb-import@example.com",
		"telefon":  "+49 211 222222",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Import Projekt",
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

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	if err := writer.WriteField("contact_id", customerID); err != nil {
		t.Fatalf("write contact_id field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "lv.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte("GAEB-DUMMY-CONTENT")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	createImportReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", &body)
	createImportReq.Header.Set("Authorization", "Bearer "+accessToken)
	createImportReq.Header.Set("Content-Type", writer.FormDataContentType())
	createImportRec := httptest.NewRecorder()
	handler.ServeHTTP(createImportRec, createImportReq)
	if createImportRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import create, got %d with body %s", createImportRec.Code, createImportRec.Body.String())
	}

	var createdImport struct {
		ID               string `json:"id"`
		ProjectID        string `json:"project_id"`
		ContactID        string `json:"contact_id"`
		SourceKind       string `json:"source_kind"`
		SourceFilename   string `json:"source_filename"`
		SourceDocumentID string `json:"source_document_id"`
		Status           string `json:"status"`
		DetectedFormat   string `json:"detected_format"`
		ErrorMessage     string `json:"error_message"`
		CreatedQuoteID   string `json:"created_quote_id"`
	}
	if err := json.Unmarshal(createImportRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode import create response: %v", err)
	}
	if createdImport.ID == "" {
		t.Fatal("expected import id")
	}
	if createdImport.ProjectID != createdProject.ID {
		t.Fatalf("expected project_id %q, got %q", createdProject.ID, createdImport.ProjectID)
	}
	if createdImport.ContactID != customerID {
		t.Fatalf("expected contact_id %q, got %q", customerID, createdImport.ContactID)
	}
	if createdImport.SourceKind != "gaeb" {
		t.Fatalf("expected source_kind gaeb, got %q", createdImport.SourceKind)
	}
	if createdImport.SourceFilename != "lv.x83" {
		t.Fatalf("expected source_filename lv.x83, got %q", createdImport.SourceFilename)
	}
	if createdImport.SourceDocumentID == "" {
		t.Fatal("expected source_document_id")
	}
	if createdImport.Status != "uploaded" {
		t.Fatalf("expected status uploaded, got %q", createdImport.Status)
	}
	if createdImport.DetectedFormat != "" || createdImport.ErrorMessage != "" || createdImport.CreatedQuoteID != "" {
		t.Fatalf("expected empty parser fields on fresh import, got %+v", createdImport)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports?project_id="+createdProject.ID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import list, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var list []struct {
		ID         string `json:"id"`
		ProjectID  string `json:"project_id"`
		ContactID  string `json:"contact_id"`
		Status     string `json:"status"`
		SourceKind string `json:"source_kind"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode import list response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one import in list, got %d", len(list))
	}
	if list[0].ID != createdImport.ID || list[0].Status != "uploaded" || list[0].SourceKind != "gaeb" {
		t.Fatalf("unexpected import list entry: %+v", list[0])
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports/"+createdImport.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import get, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID               string `json:"id"`
		ProjectID        string `json:"project_id"`
		ContactID        string `json:"contact_id"`
		SourceFilename   string `json:"source_filename"`
		SourceDocumentID string `json:"source_document_id"`
		Status           string `json:"status"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode import get response: %v", err)
	}
	if fetched.ID != createdImport.ID || fetched.ProjectID != createdProject.ID || fetched.ContactID != customerID || fetched.SourceFilename != "lv.x83" || fetched.SourceDocumentID == "" || fetched.Status != "uploaded" {
		t.Fatalf("unexpected import detail: %+v", fetched)
	}
}

func TestQuoteGAEBImportRejectsNonGAEBFiles(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-quote-import-invalid@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-quote-import-invalid@example.com", "Secret123!")

	projectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Invalid Projekt",
		"status":"angebot"
	}`)))
	projectReq.Header.Set("Authorization", "Bearer "+accessToken)
	projectReq.Header.Set("Content-Type", "application/json")
	projectRec := httptest.NewRecorder()
	handler.ServeHTTP(projectRec, projectReq)
	if projectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", projectRec.Code, projectRec.Body.String())
	}

	var createdProject struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(projectRec.Body.Bytes(), &createdProject); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "not-gaeb.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("%PDF-1.4 invalid gaeb")); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", &body)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid GAEB file type, got %d with body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Nur GAEB-Dateien") {
		t.Fatalf("expected GAEB file type validation message, got %s", rec.Body.String())
	}
}

func TestQuoteGAEBImportListAndDetailExposeReviewSummaryCounts(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-quote-import-summary@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-quote-import-summary@example.com", "Secret123!")

	projectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Summary Projekt",
		"status":"angebot"
	}`)))
	projectReq.Header.Set("Authorization", "Bearer "+accessToken)
	projectReq.Header.Set("Content-Type", "application/json")
	projectRec := httptest.NewRecorder()
	handler.ServeHTTP(projectRec, projectReq)
	if projectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", projectRec.Code, projectRec.Body.String())
	}

	var createdProject struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(projectRec.Body.Bytes(), &createdProject); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "summary.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte("GAEB-SUMMARY-CONTENT")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", &body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode import create response: %v", err)
	}

	quoteSvc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Akzeptierte Position",
			Qty:         1,
			Unit:        "Stk",
			SortOrder:   1,
		},
		{
			PositionNo:  "01.002",
			OutlineNo:   "01",
			Description: "Abgelehnte Position",
			Qty:         2,
			Unit:        "Std",
			SortOrder:   2,
		},
		{
			PositionNo:  "01.003",
			OutlineNo:   "01",
			Description: "Offene Position",
			Qty:         3,
			Unit:        "m",
			SortOrder:   3,
		},
	}); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID)
	if err != nil || len(items) != 3 {
		t.Fatalf("expected 3 import items, got %d err=%v", len(items), err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[0].ID, "accepted", "Übernehmen"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[1].ID, "rejected", "Nicht übernehmen"); err != nil {
		t.Fatalf("reject import item: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports?project_id="+createdProject.ID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import list, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var list []struct {
		ID            string `json:"id"`
		ItemCount     int    `json:"item_count"`
		AcceptedCount int    `json:"accepted_count"`
		RejectedCount int    `json:"rejected_count"`
		PendingCount  int    `json:"pending_count"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode import list response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one import in list, got %d", len(list))
	}
	if list[0].ID != createdImport.ID || list[0].ItemCount != 3 || list[0].AcceptedCount != 1 || list[0].RejectedCount != 1 || list[0].PendingCount != 1 {
		t.Fatalf("unexpected import list summary: %+v", list[0])
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports/"+createdImport.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import detail, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID            string `json:"id"`
		ItemCount     int    `json:"item_count"`
		AcceptedCount int    `json:"accepted_count"`
		RejectedCount int    `json:"rejected_count"`
		PendingCount  int    `json:"pending_count"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode import detail response: %v", err)
	}
	if fetched.ID != createdImport.ID || fetched.ItemCount != 3 || fetched.AcceptedCount != 1 || fetched.RejectedCount != 1 || fetched.PendingCount != 1 {
		t.Fatalf("unexpected import detail summary: %+v", fetched)
	}
}

func TestQuoteGAEBImportItemReadEndpointsExposeParsedItems(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-items@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-items@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Item Test Kunde GmbH",
		"email":    "gaeb-items@example.com",
		"telefon":  "+49 211 666666",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Item Test Projekt",
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	if err := writer.WriteField("contact_id", customerID); err != nil {
		t.Fatalf("write contact_id field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "parsed-items.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy-gaeb-content")); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if createdImport.ID == "" {
		t.Fatal("expected created import id")
	}

	quoteSvc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	parsedImport, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Fensterelement Aluminium",
			Qty:         2,
			Unit:        "Stk",
			ParserHint:  "lv-position",
			SortOrder:   1,
		},
		{
			PositionNo:  "01.002",
			OutlineNo:   "01",
			Description: "Montage vor Ort",
			Qty:         6,
			Unit:        "Std",
			IsOptional:  true,
			ParserHint:  "optionale-position",
			SortOrder:   2,
		},
	})
	if err != nil {
		t.Fatalf("save import parse result: %v", err)
	}
	if parsedImport.Status != "parsed" || parsedImport.ItemCount != 2 {
		t.Fatalf("expected parsed import with item_count 2, got %+v", parsedImport)
	}

	itemsReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports/"+createdImport.ID+"/items", nil)
	itemsReq.Header.Set("Authorization", "Bearer "+accessToken)
	itemsRec := httptest.NewRecorder()
	handler.ServeHTTP(itemsRec, itemsReq)
	if itemsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import items list, got %d with body %s", itemsRec.Code, itemsRec.Body.String())
	}

	var items []struct {
		ID                string  `json:"id"`
		ImportID          string  `json:"import_id"`
		PositionNo        string  `json:"position_no"`
		OutlineNo         string  `json:"outline_no"`
		Description       string  `json:"description"`
		Qty               float64 `json:"qty"`
		Unit              string  `json:"unit"`
		IsOptional        bool    `json:"is_optional"`
		ParserHint        string  `json:"parser_hint"`
		ReviewStatus      string  `json:"review_status"`
		SortOrder         int     `json:"sort_order"`
		LinkedQuoteID     string  `json:"linked_quote_id"`
		LinkedQuoteItemID string  `json:"linked_quote_item_id"`
		LinkedQuotePos    int     `json:"linked_quote_position"`
	}
	if err := json.Unmarshal(itemsRec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode import items response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 import items, got %d with body %s", len(items), itemsRec.Body.String())
	}
	if items[0].ImportID != createdImport.ID || items[0].PositionNo != "01.001" || items[0].ReviewStatus != "pending" || items[0].SortOrder != 1 {
		t.Fatalf("unexpected first import item: %+v", items[0])
	}
	if items[0].LinkedQuoteID != "" || items[0].LinkedQuoteItemID != "" || items[0].LinkedQuotePos != 0 {
		t.Fatalf("expected no quote link on parsed import item, got %+v", items[0])
	}
	if items[1].PositionNo != "01.002" || !items[1].IsOptional || items[1].SortOrder != 2 {
		t.Fatalf("unexpected second import item: %+v", items[1])
	}

	itemReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports/"+createdImport.ID+"/items/"+items[1].ID, nil)
	itemReq.Header.Set("Authorization", "Bearer "+accessToken)
	itemRec := httptest.NewRecorder()
	handler.ServeHTTP(itemRec, itemReq)
	if itemRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import item detail, got %d with body %s", itemRec.Code, itemRec.Body.String())
	}

	var itemDetail struct {
		ID                string `json:"id"`
		ImportID          string `json:"import_id"`
		PositionNo        string `json:"position_no"`
		Description       string `json:"description"`
		ParserHint        string `json:"parser_hint"`
		ReviewStatus      string `json:"review_status"`
		LinkedQuoteID     string `json:"linked_quote_id"`
		LinkedQuoteItemID string `json:"linked_quote_item_id"`
		LinkedQuotePos    int    `json:"linked_quote_position"`
	}
	if err := json.Unmarshal(itemRec.Body.Bytes(), &itemDetail); err != nil {
		t.Fatalf("decode import item detail response: %v", err)
	}
	if itemDetail.ID != items[1].ID || itemDetail.ImportID != createdImport.ID || itemDetail.PositionNo != "01.002" || itemDetail.ParserHint != "optionale-position" || itemDetail.ReviewStatus != "pending" {
		t.Fatalf("unexpected import item detail: %+v", itemDetail)
	}
	if itemDetail.LinkedQuoteID != "" || itemDetail.LinkedQuoteItemID != "" || itemDetail.LinkedQuotePos != 0 {
		t.Fatalf("expected no quote link on parsed import item detail, got %+v", itemDetail)
	}
}

func TestQuoteGAEBImportItemReviewEndpointUpdatesReviewFields(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-review@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-review@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Review Kunde GmbH",
		"email":    "gaeb-review@example.com",
		"telefon":  "+49 211 777777",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Review Projekt",
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "review-items.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy-gaeb-content")); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	quoteSvc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Fassadenelement",
			Qty:         3,
			Unit:        "Stk",
			SortOrder:   1,
		},
	}); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected one import item, got %d err=%v", len(items), err)
	}

	reviewReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/imports/"+createdImport.ID+"/items/"+items[0].ID+"/review", bytes.NewReader([]byte(`{
		"review_status":"accepted",
		"review_note":"Fachlich freigegeben"
	}`)))
	reviewReq.Header.Set("Authorization", "Bearer "+accessToken)
	reviewReq.Header.Set("Content-Type", "application/json")
	reviewRec := httptest.NewRecorder()
	handler.ServeHTTP(reviewRec, reviewReq)
	if reviewRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for item review patch, got %d with body %s", reviewRec.Code, reviewRec.Body.String())
	}

	var reviewed struct {
		ID           string `json:"id"`
		ImportID     string `json:"import_id"`
		ReviewStatus string `json:"review_status"`
		ReviewNote   string `json:"review_note"`
	}
	if err := json.Unmarshal(reviewRec.Body.Bytes(), &reviewed); err != nil {
		t.Fatalf("decode review response: %v", err)
	}
	if reviewed.ID != items[0].ID || reviewed.ImportID != createdImport.ID || reviewed.ReviewStatus != "accepted" || reviewed.ReviewNote != "Fachlich freigegeben" {
		t.Fatalf("unexpected review patch response: %+v", reviewed)
	}
}

func TestQuoteGAEBImportItemReviewEndpointRejectsNonParsedImport(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-review-guard@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-review-guard@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Review Guard Kunde GmbH",
		"email":    "gaeb-review-guard@example.com",
		"telefon":  "+49 211 888888",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Review Guard Projekt",
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "review-guard.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy-gaeb-content")); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	itemID := uuid.NewString()
	if _, err := env.PG.Exec(uploadReq.Context(), `
		INSERT INTO quote_import_items (
			id, import_id, position_no, outline_no, description, qty, unit,
			is_optional, parser_hint, review_status, review_note, sort_order
		) VALUES ($1,$2,'01.001','01','Nur Rohposition',1,'Stk',false,'seed','pending','',1)
	`, itemID, createdImport.ID); err != nil {
		t.Fatalf("seed import item: %v", err)
	}

	reviewReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/imports/"+createdImport.ID+"/items/"+itemID+"/review", bytes.NewReader([]byte(`{
		"review_status":"accepted",
		"review_note":"Sollte scheitern"
	}`)))
	reviewReq.Header.Set("Authorization", "Bearer "+accessToken)
	reviewReq.Header.Set("Content-Type", "application/json")
	reviewRec := httptest.NewRecorder()
	handler.ServeHTTP(reviewRec, reviewReq)
	if reviewRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-parsed import review, got %d with body %s", reviewRec.Code, reviewRec.Body.String())
	}
	if !strings.Contains(reviewRec.Body.String(), "Nur geparste Importläufe") {
		t.Fatalf("expected parsed import guard message, got %s", reviewRec.Body.String())
	}
}

func TestQuoteGAEBImportReviewEndpointRejectsPendingItems(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-import-review@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-import-review@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Import Review Kunde GmbH",
		"email":    "gaeb-import-review@example.com",
		"telefon":  "+49 211 999991",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Import Review Projekt",
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "import-review.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy-gaeb-content")); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	quoteSvc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Nur teilweise bewertet",
			Qty:         1,
			Unit:        "Stk",
			SortOrder:   1,
		},
	}); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	reviewImportReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/imports/"+createdImport.ID+"/review", nil)
	reviewImportReq.Header.Set("Authorization", "Bearer "+accessToken)
	reviewImportRec := httptest.NewRecorder()
	handler.ServeHTTP(reviewImportRec, reviewImportReq)
	if reviewImportRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for import review with pending items, got %d with body %s", reviewImportRec.Code, reviewImportRec.Body.String())
	}
	if !strings.Contains(reviewImportRec.Body.String(), "offene Review-Positionen") {
		t.Fatalf("expected pending review guard message, got %s", reviewImportRec.Body.String())
	}
}

func TestQuoteGAEBImportApplyCreatesDraftQuoteFromAcceptedItems(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-apply@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-apply@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Apply Kunde GmbH",
		"email":    "gaeb-apply@example.com",
		"telefon":  "+49 211 999992",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Apply Projekt",
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	if err := writer.WriteField("contact_id", customerID); err != nil {
		t.Fatalf("write contact_id field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "import-apply.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy-gaeb-content")); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	quoteSvc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Akzeptierte Position",
			Qty:         2,
			Unit:        "Stk",
			SortOrder:   1,
		},
		{
			PositionNo:  "01.002",
			OutlineNo:   "01",
			Description: "Abgelehnte Position",
			Qty:         5,
			Unit:        "Std",
			SortOrder:   2,
		},
	}); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("expected two import items, got %d err=%v", len(items), err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[0].ID, "accepted", "Übernehmen"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[1].ID, "rejected", "Nicht übernehmen"); err != nil {
		t.Fatalf("reject import item: %v", err)
	}

	reviewImportReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/imports/"+createdImport.ID+"/review", nil)
	reviewImportReq.Header.Set("Authorization", "Bearer "+accessToken)
	reviewImportRec := httptest.NewRecorder()
	handler.ServeHTTP(reviewImportRec, reviewImportReq)
	if reviewImportRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for import review, got %d with body %s", reviewImportRec.Code, reviewImportRec.Body.String())
	}

	var reviewedImport struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(reviewImportRec.Body.Bytes(), &reviewedImport); err != nil {
		t.Fatalf("decode import review response: %v", err)
	}
	if reviewedImport.ID != createdImport.ID || reviewedImport.Status != "reviewed" {
		t.Fatalf("unexpected reviewed import response: %+v", reviewedImport)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/"+createdImport.ID+"/apply", nil)
	applyReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyRec := httptest.NewRecorder()
	handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for import apply, got %d with body %s", applyRec.Code, applyRec.Body.String())
	}

	var applied struct {
		Import struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			CreatedQuoteID string `json:"created_quote_id"`
		} `json:"import"`
		Quote struct {
			ID        string `json:"id"`
			ProjectID string `json:"project_id"`
			ContactID string `json:"contact_id"`
			Status    string `json:"status"`
			Note      string `json:"note"`
			Items     []struct {
				Description             string  `json:"description"`
				Qty                     float64 `json:"qty"`
				Unit                    string  `json:"unit"`
				UnitPrice               float64 `json:"unit_price"`
				MaterialID              string  `json:"material_id"`
				PriceMappingStatus      string  `json:"price_mapping_status"`
				MaterialCandidateStatus string  `json:"material_candidate_status"`
			} `json:"items"`
		} `json:"quote"`
	}
	if err := json.Unmarshal(applyRec.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode import apply response: %v", err)
	}
	if applied.Import.ID != createdImport.ID || applied.Import.Status != "applied" {
		t.Fatalf("unexpected applied import response: %+v", applied.Import)
	}
	if applied.Import.CreatedQuoteID == "" || applied.Import.CreatedQuoteID != applied.Quote.ID {
		t.Fatalf("expected created quote link, got import=%+v quote=%+v", applied.Import, applied.Quote)
	}
	if applied.Quote.ProjectID != createdProject.ID || applied.Quote.ContactID != customerID || applied.Quote.Status != "draft" {
		t.Fatalf("unexpected created quote metadata: %+v", applied.Quote)
	}
	if len(applied.Quote.Items) != 1 {
		t.Fatalf("expected exactly one accepted quote item, got %d", len(applied.Quote.Items))
	}
	if applied.Quote.Items[0].Description != "Akzeptierte Position" || applied.Quote.Items[0].Qty != 2 || applied.Quote.Items[0].Unit != "Stk" || applied.Quote.Items[0].UnitPrice != 0 {
		t.Fatalf("unexpected created quote item: %+v", applied.Quote.Items[0])
	}
	if applied.Quote.Items[0].MaterialID != "" || applied.Quote.Items[0].PriceMappingStatus != "open" || applied.Quote.Items[0].MaterialCandidateStatus != "available" {
		t.Fatalf("expected open imported quote item with available candidate anchor, got %+v", applied.Quote.Items[0])
	}
	if !strings.Contains(applied.Quote.Note, createdImport.ID) {
		t.Fatalf("expected import reference in quote note, got %q", applied.Quote.Note)
	}

	var linkCount int
	if err := env.PG.QueryRow(applyReq.Context(), `
		SELECT COUNT(*)
		FROM quote_import_item_links
		WHERE quote_id=$1::uuid
	`, applied.Quote.ID).Scan(&linkCount); err != nil {
		t.Fatalf("count import item links: %v", err)
	}
	if linkCount != 1 {
		t.Fatalf("expected exactly one import item link, got %d", linkCount)
	}

	var linkedImportItemID string
	var linkedQuoteID string
	var linkedQuoteItemDescription string
	if err := env.PG.QueryRow(applyReq.Context(), `
		SELECT qil.quote_import_item_id::text, qil.quote_id::text, qi.description
		FROM quote_import_item_links qil
		JOIN quote_items qi ON qi.id = qil.quote_item_id
		WHERE qil.quote_id=$1::uuid
	`, applied.Quote.ID).Scan(&linkedImportItemID, &linkedQuoteID, &linkedQuoteItemDescription); err != nil {
		t.Fatalf("load import item link: %v", err)
	}
	if linkedQuoteID != applied.Quote.ID {
		t.Fatalf("expected linked quote id %q, got %q", applied.Quote.ID, linkedQuoteID)
	}
	if linkedQuoteItemDescription != "Akzeptierte Position" {
		t.Fatalf("unexpected linked quote item description: %q", linkedQuoteItemDescription)
	}
	if linkedImportItemID != items[0].ID {
		t.Fatalf("expected accepted import item %q to be linked, got %q", items[0].ID, linkedImportItemID)
	}

	appliedItemsReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports/"+createdImport.ID+"/items", nil)
	appliedItemsReq.Header.Set("Authorization", "Bearer "+accessToken)
	appliedItemsRec := httptest.NewRecorder()
	handler.ServeHTTP(appliedItemsRec, appliedItemsReq)
	if appliedItemsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for applied import items list, got %d with body %s", appliedItemsRec.Code, appliedItemsRec.Body.String())
	}

	var appliedItems []struct {
		ID                string `json:"id"`
		ReviewStatus      string `json:"review_status"`
		LinkedQuoteID     string `json:"linked_quote_id"`
		LinkedQuoteItemID string `json:"linked_quote_item_id"`
		LinkedQuotePos    int    `json:"linked_quote_position"`
	}
	if err := json.Unmarshal(appliedItemsRec.Body.Bytes(), &appliedItems); err != nil {
		t.Fatalf("decode applied import items response: %v", err)
	}
	if len(appliedItems) != 2 {
		t.Fatalf("expected 2 applied import items, got %d", len(appliedItems))
	}
	if appliedItems[0].ID != items[0].ID || appliedItems[0].ReviewStatus != "accepted" || appliedItems[0].LinkedQuoteID != applied.Quote.ID || appliedItems[0].LinkedQuoteItemID == "" || appliedItems[0].LinkedQuotePos != 1 {
		t.Fatalf("unexpected accepted import item link view: %+v", appliedItems[0])
	}
	if appliedItems[1].ID != items[1].ID || appliedItems[1].ReviewStatus != "rejected" || appliedItems[1].LinkedQuoteID != "" || appliedItems[1].LinkedQuoteItemID != "" || appliedItems[1].LinkedQuotePos != 0 {
		t.Fatalf("unexpected rejected import item link view: %+v", appliedItems[1])
	}

	appliedItemReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/imports/"+createdImport.ID+"/items/"+items[0].ID, nil)
	appliedItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	appliedItemRec := httptest.NewRecorder()
	handler.ServeHTTP(appliedItemRec, appliedItemReq)
	if appliedItemRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for applied import item detail, got %d with body %s", appliedItemRec.Code, appliedItemRec.Body.String())
	}

	var appliedItemDetail struct {
		ID                string `json:"id"`
		ReviewStatus      string `json:"review_status"`
		LinkedQuoteID     string `json:"linked_quote_id"`
		LinkedQuoteItemID string `json:"linked_quote_item_id"`
		LinkedQuotePos    int    `json:"linked_quote_position"`
	}
	if err := json.Unmarshal(appliedItemRec.Body.Bytes(), &appliedItemDetail); err != nil {
		t.Fatalf("decode applied import item detail: %v", err)
	}
	if appliedItemDetail.ID != items[0].ID || appliedItemDetail.ReviewStatus != "accepted" || appliedItemDetail.LinkedQuoteID != applied.Quote.ID || appliedItemDetail.LinkedQuoteItemID == "" || appliedItemDetail.LinkedQuotePos != 1 {
		t.Fatalf("unexpected applied import item detail: %+v", appliedItemDetail)
	}
}

func TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-candidates@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-candidates@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "GAEB Kandidaten Kunde GmbH",
		"email":    "gaeb-candidates@example.com",
		"telefon":  "+49 211 999993",
		"waehrung": "EUR",
	})

	createMaterialReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-GAEB-CAND-0001",
		"bezeichnung":"Aluminium Profil 70mm",
		"einheit":"m"
	}`)))
	createMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMaterialReq.Header.Set("Content-Type", "application/json")
	createMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(createMaterialRec, createMaterialReq)
	if createMaterialRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for material create, got %d with body %s", createMaterialRec.Code, createMaterialRec.Body.String())
	}

	var createdMaterial struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMaterialRec.Body.Bytes(), &createdMaterial); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"GAEB Kandidaten Projekt",
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	if err := writer.WriteField("contact_id", customerID); err != nil {
		t.Fatalf("write contact_id field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "import-candidates.x83")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy-gaeb-content")); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for gaeb import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	quoteSvc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Aluminium Profil 70mm",
			Qty:         4,
			Unit:        "m",
			SortOrder:   1,
		},
	}); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected one import item, got %d err=%v", len(items), err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[0].ID, "accepted", "Übernehmen"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.MarkImportReviewed(uploadReq.Context(), createdImport.ID); err != nil {
		t.Fatalf("mark import reviewed: %v", err)
	}

	applied, err := quoteSvc.ApplyImportToDraftQuote(uploadReq.Context(), createdImport.ID)
	if err != nil {
		t.Fatalf("apply import: %v", err)
	}
	if applied.Quote == nil || len(applied.Quote.Items) != 1 {
		t.Fatalf("unexpected applied quote payload: %+v", applied)
	}

	item := applied.Quote.Items[0]
	if item.MaterialCandidateStatus != "available" {
		t.Fatalf("expected available material candidate status, got %+v", item)
	}
	if len(item.MaterialCandidates) != 1 {
		t.Fatalf("expected one material candidate, got %+v", item.MaterialCandidates)
	}
	if item.MaterialCandidates[0].MaterialID != createdMaterial.ID ||
		item.MaterialCandidates[0].MaterialNo != "MAT-GAEB-CAND-0001" ||
		item.MaterialCandidates[0].MaterialLabel != "Aluminium Profil 70mm" {
		t.Fatalf("unexpected material candidate payload: %+v", item.MaterialCandidates[0])
	}

	getQuoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+applied.Quote.ID.String(), nil)
	getQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	getQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(getQuoteRec, getQuoteReq)
	if getQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote get, got %d with body %s", getQuoteRec.Code, getQuoteRec.Body.String())
	}

	var fetched struct {
		Items []struct {
			MaterialCandidateStatus string `json:"material_candidate_status"`
			MaterialCandidates      []struct {
				MaterialID    string `json:"material_id"`
				MaterialNo    string `json:"material_no"`
				MaterialLabel string `json:"material_label"`
			} `json:"material_candidates"`
		} `json:"items"`
	}
	if err := json.Unmarshal(getQuoteRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode quote get response: %v", err)
	}
	if len(fetched.Items) != 1 || fetched.Items[0].MaterialCandidateStatus != "available" {
		t.Fatalf("unexpected fetched quote candidate status: %+v", fetched.Items)
	}
	if len(fetched.Items[0].MaterialCandidates) != 1 ||
		fetched.Items[0].MaterialCandidates[0].MaterialID != createdMaterial.ID ||
		fetched.Items[0].MaterialCandidates[0].MaterialNo != "MAT-GAEB-CAND-0001" ||
		fetched.Items[0].MaterialCandidates[0].MaterialLabel != "Aluminium Profil 70mm" {
		t.Fatalf("unexpected fetched material candidates: %+v", fetched.Items[0].MaterialCandidates)
	}
}

func TestQuoteApplyVisibleMaterialCandidateSetsManualMapping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-candidate-apply@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-candidate-apply@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Kandidatenaktion Kunde GmbH",
		"email":    "candidate-apply@example.com",
		"telefon":  "+49 211 888888",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Kandidatenaktion Projekt",
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

	createMaterialReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-GAEB-APPLY-0001",
		"bezeichnung":"Aluminium Profil 90mm",
		"einheit":"Stk",
		"aktiv":true
	}`)))
	createMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMaterialReq.Header.Set("Content-Type", "application/json")
	createMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(createMaterialRec, createMaterialReq)
	if createMaterialRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for material create, got %d with body %s", createMaterialRec.Code, createMaterialRec.Body.String())
	}

	var createdMaterial struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMaterialRec.Body.Bytes(), &createdMaterial); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}

	createOtherMaterialReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-GAEB-APPLY-0002",
		"bezeichnung":"Stahl Profil 90mm",
		"einheit":"Stk",
		"aktiv":true
	}`)))
	createOtherMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOtherMaterialReq.Header.Set("Content-Type", "application/json")
	createOtherMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(createOtherMaterialRec, createOtherMaterialReq)
	if createOtherMaterialRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for other material create, got %d with body %s", createOtherMaterialRec.Code, createOtherMaterialRec.Body.String())
	}

	var otherMaterial struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createOtherMaterialRec.Body.Bytes(), &otherMaterial); err != nil {
		t.Fatalf("decode other material create response: %v", err)
	}

	uploadBody := &bytes.Buffer{}
	uploadWriter := multipart.NewWriter(uploadBody)
	if err := uploadWriter.WriteField("project_id", createdProject.ID); err != nil {
		t.Fatalf("write project_id field: %v", err)
	}
	if err := uploadWriter.WriteField("contact_id", customerID); err != nil {
		t.Fatalf("write contact_id field: %v", err)
	}
	fileWriter, err := uploadWriter.CreateFormFile("file", "gaeb-candidate-apply.x83")
	if err != nil {
		t.Fatalf("create upload form file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("dummy gaeb content")); err != nil {
		t.Fatalf("write upload file: %v", err)
	}
	if err := uploadWriter.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports", uploadBody)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", uploadWriter.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for import upload, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var createdImport struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &createdImport); err != nil {
		t.Fatalf("decode import upload response: %v", err)
	}

	quoteSvc := quotes.NewService(env.PG, nil)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "itest-parser", "GAEB-X83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01",
			Description: "Aluminium Profil 90mm",
			Qty:         1,
			Unit:        "Stk",
			SortOrder:   1,
		},
	}); err != nil {
		t.Fatalf("save parse result: %v", err)
	}
	importItems, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID)
	if err != nil {
		t.Fatalf("list import items: %v", err)
	}
	if len(importItems) != 1 {
		t.Fatalf("expected one import item, got %+v", importItems)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, importItems[0].ID, "accepted", "Passender Kandidat sichtbar"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.MarkImportReviewed(uploadReq.Context(), createdImport.ID); err != nil {
		t.Fatalf("mark import reviewed: %v", err)
	}

	applied, err := quoteSvc.ApplyImportToDraftQuote(uploadReq.Context(), createdImport.ID)
	if err != nil {
		t.Fatalf("apply import: %v", err)
	}
	if applied.Quote == nil || len(applied.Quote.Items) != 1 {
		t.Fatalf("unexpected applied quote payload: %+v", applied)
	}

	itemID := applied.Quote.Items[0].ID
	if itemID == "" {
		t.Fatalf("expected quote item id in applied quote payload: %+v", applied.Quote.Items[0])
	}
	if len(applied.Quote.Items[0].MaterialCandidates) != 1 {
		t.Fatalf("expected one visible material candidate, got %+v", applied.Quote.Items[0].MaterialCandidates)
	}

	invalidApplyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+applied.Quote.ID.String()+"/items/"+itemID+"/apply-material-candidate", bytes.NewReader([]byte(`{
		"material_id":"`+otherMaterial.ID+`"
	}`)))
	invalidApplyReq.Header.Set("Authorization", "Bearer "+accessToken)
	invalidApplyReq.Header.Set("Content-Type", "application/json")
	invalidApplyRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidApplyRec, invalidApplyReq)
	if invalidApplyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-visible candidate, got %d with body %s", invalidApplyRec.Code, invalidApplyRec.Body.String())
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+applied.Quote.ID.String()+"/items/"+itemID+"/apply-material-candidate", bytes.NewReader([]byte(`{
		"material_id":"`+createdMaterial.ID+`"
	}`)))
	applyReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyReq.Header.Set("Content-Type", "application/json")
	applyRec := httptest.NewRecorder()
	handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for visible candidate apply, got %d with body %s", applyRec.Code, applyRec.Body.String())
	}

	var updatedQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID                      string `json:"id"`
			MaterialID              string `json:"material_id"`
			PriceMappingStatus      string `json:"price_mapping_status"`
			MaterialCandidateStatus string `json:"material_candidate_status"`
			MaterialCandidates      []struct {
				MaterialID string `json:"material_id"`
			} `json:"material_candidates"`
		} `json:"items"`
	}
	if err := json.Unmarshal(applyRec.Body.Bytes(), &updatedQuote); err != nil {
		t.Fatalf("decode candidate apply response: %v", err)
	}
	if len(updatedQuote.Items) != 1 {
		t.Fatalf("expected one quote item after candidate apply, got %+v", updatedQuote.Items)
	}
	if updatedQuote.Items[0].MaterialID != createdMaterial.ID {
		t.Fatalf("expected material_id %q after candidate apply, got %+v", createdMaterial.ID, updatedQuote.Items[0])
	}
	if updatedQuote.Items[0].PriceMappingStatus != "manual" {
		t.Fatalf("expected price_mapping_status manual, got %+v", updatedQuote.Items[0])
	}
	if updatedQuote.Items[0].MaterialCandidateStatus != "none" {
		t.Fatalf("expected candidate status none after candidate apply, got %+v", updatedQuote.Items[0])
	}
	if len(updatedQuote.Items[0].MaterialCandidates) != 0 {
		t.Fatalf("expected no visible candidates after candidate apply, got %+v", updatedQuote.Items[0].MaterialCandidates)
	}
}

func TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-revise@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-revise@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Revisionskunde GmbH",
		"email":    "revision@example.com",
		"telefon":  "+49 211 222222",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Revisionstest Projekt",
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

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"note":"Basisangebot fuer Revision",
		"items":[
			{"description":"Position A","qty":2,"unit":"Stk","unit_price":500,"tax_code":"DE19"},
			{"description":"Position B","qty":1,"unit":"Std","unit_price":150,"tax_code":"DE19"}
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
		ID          string `json:"id"`
		Number      string `json:"number"`
		RootQuoteID string `json:"root_quote_id"`
		RevisionNo  int    `json:"revision_no"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}

	reviseReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/revise", nil)
	reviseReq.Header.Set("Authorization", "Bearer "+accessToken)
	reviseRec := httptest.NewRecorder()
	handler.ServeHTTP(reviseRec, reviseReq)
	if reviseRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote revise, got %d with body %s", reviseRec.Code, reviseRec.Body.String())
	}

	var revised struct {
		SourceQuote struct {
			ID                  string `json:"id"`
			Number              string `json:"number"`
			RootQuoteID         string `json:"root_quote_id"`
			RevisionNo          int    `json:"revision_no"`
			SupersededByQuoteID string `json:"superseded_by_quote_id"`
			Status              string `json:"status"`
			Items               []struct {
				Description string  `json:"description"`
				Qty         float64 `json:"qty"`
			} `json:"items"`
		} `json:"source_quote"`
		RevisedQuote struct {
			ID                  string `json:"id"`
			Number              string `json:"number"`
			RootQuoteID         string `json:"root_quote_id"`
			RevisionNo          int    `json:"revision_no"`
			SupersededByQuoteID string `json:"superseded_by_quote_id"`
			Status              string `json:"status"`
			Items               []struct {
				Description string  `json:"description"`
				Qty         float64 `json:"qty"`
			} `json:"items"`
		} `json:"revised_quote"`
	}
	if err := json.Unmarshal(reviseRec.Body.Bytes(), &revised); err != nil {
		t.Fatalf("decode quote revise response: %v", err)
	}

	if revised.SourceQuote.ID != createdQuote.ID {
		t.Fatalf("expected source quote id %q, got %q", createdQuote.ID, revised.SourceQuote.ID)
	}
	if revised.SourceQuote.RootQuoteID != createdQuote.RootQuoteID {
		t.Fatalf("expected source root quote id %q, got %q", createdQuote.RootQuoteID, revised.SourceQuote.RootQuoteID)
	}
	if revised.SourceQuote.RevisionNo != 1 {
		t.Fatalf("expected source revision_no 1, got %d", revised.SourceQuote.RevisionNo)
	}
	if revised.SourceQuote.SupersededByQuoteID != revised.RevisedQuote.ID {
		t.Fatalf("expected source superseded_by_quote_id %q, got %q", revised.RevisedQuote.ID, revised.SourceQuote.SupersededByQuoteID)
	}
	if revised.RevisedQuote.ID == "" || revised.RevisedQuote.ID == revised.SourceQuote.ID {
		t.Fatalf("expected different revised quote id, got %q", revised.RevisedQuote.ID)
	}
	if revised.RevisedQuote.Number != createdQuote.Number {
		t.Fatalf("expected revised quote number %q, got %q", createdQuote.Number, revised.RevisedQuote.Number)
	}
	if revised.RevisedQuote.RootQuoteID != createdQuote.RootQuoteID {
		t.Fatalf("expected revised root quote id %q, got %q", createdQuote.RootQuoteID, revised.RevisedQuote.RootQuoteID)
	}
	if revised.RevisedQuote.RevisionNo != 2 {
		t.Fatalf("expected revised revision_no 2, got %d", revised.RevisedQuote.RevisionNo)
	}
	if revised.RevisedQuote.Status != "draft" {
		t.Fatalf("expected revised quote status draft, got %q", revised.RevisedQuote.Status)
	}
	if revised.RevisedQuote.SupersededByQuoteID != "" {
		t.Fatalf("expected revised quote without superseded_by_quote_id, got %q", revised.RevisedQuote.SupersededByQuoteID)
	}
	if len(revised.SourceQuote.Items) != 2 || len(revised.RevisedQuote.Items) != 2 {
		t.Fatalf("expected 2 items on source and revised quote, got source=%d revised=%d", len(revised.SourceQuote.Items), len(revised.RevisedQuote.Items))
	}
	if revised.RevisedQuote.Items[0].Description != revised.SourceQuote.Items[0].Description || revised.RevisedQuote.Items[0].Qty != revised.SourceQuote.Items[0].Qty {
		t.Fatalf("expected copied first quote item, got source=%+v revised=%+v", revised.SourceQuote.Items[0], revised.RevisedQuote.Items[0])
	}

	reviseAgainReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/revise", nil)
	reviseAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	reviseAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(reviseAgainRec, reviseAgainReq)
	if reviseAgainRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for revising superseded quote, got %d with body %s", reviseAgainRec.Code, reviseAgainRec.Body.String())
	}

	updateSupersededReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"contact_id":"`+customerID+`",
		"currency":"EUR",
		"note":"Darf nicht gespeichert werden",
		"items":[
			{"description":"Position A","qty":2,"unit":"Stk","unit_price":500,"tax_code":"DE19"}
		]
	}`)))
	updateSupersededReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateSupersededReq.Header.Set("Content-Type", "application/json")
	updateSupersededRec := httptest.NewRecorder()
	handler.ServeHTTP(updateSupersededRec, updateSupersededReq)
	if updateSupersededRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for updating superseded quote, got %d with body %s", updateSupersededRec.Code, updateSupersededRec.Body.String())
	}

	statusSupersededReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/status", bytes.NewReader([]byte(`{"status":"sent"}`)))
	statusSupersededReq.Header.Set("Authorization", "Bearer "+accessToken)
	statusSupersededReq.Header.Set("Content-Type", "application/json")
	statusSupersededRec := httptest.NewRecorder()
	handler.ServeHTTP(statusSupersededRec, statusSupersededReq)
	if statusSupersededRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for status update on superseded quote, got %d with body %s", statusSupersededRec.Code, statusSupersededRec.Body.String())
	}

	convertInvoiceSupersededReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{"revenue_account":"8000"}`)))
	convertInvoiceSupersededReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertInvoiceSupersededReq.Header.Set("Content-Type", "application/json")
	convertInvoiceSupersededRec := httptest.NewRecorder()
	handler.ServeHTTP(convertInvoiceSupersededRec, convertInvoiceSupersededReq)
	if convertInvoiceSupersededRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invoice conversion on superseded quote, got %d with body %s", convertInvoiceSupersededRec.Code, convertInvoiceSupersededRec.Body.String())
	}

	acceptRevisedReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+revised.RevisedQuote.ID+"/accept", bytes.NewReader([]byte(`{}`)))
	acceptRevisedReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptRevisedReq.Header.Set("Content-Type", "application/json")
	acceptRevisedRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptRevisedRec, acceptRevisedReq)
	if acceptRevisedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for accepting revised quote, got %d with body %s", acceptRevisedRec.Code, acceptRevisedRec.Body.String())
	}

	convertSalesOrderSupersededReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/convert-to-sales-order", nil)
	convertSalesOrderSupersededReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertSalesOrderSupersededRec := httptest.NewRecorder()
	handler.ServeHTTP(convertSalesOrderSupersededRec, convertSalesOrderSupersededReq)
	if convertSalesOrderSupersededRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for sales order conversion on superseded quote, got %d with body %s", convertSalesOrderSupersededRec.Code, convertSalesOrderSupersededRec.Body.String())
	}
}

func TestCommercialWorkflowEndpointListsOpenFollowActions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-workflow@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-workflow@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Workflow Kunde GmbH",
		"email":    "workflow@example.com",
		"telefon":  "+49 211 333333",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Workflow Projekt",
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

	createSentQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Sent Position","qty":1,"unit":"Stk","unit_price":1000,"tax_code":"DE19"}]
	}`)))
	createSentQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createSentQuoteReq.Header.Set("Content-Type", "application/json")
	createSentQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createSentQuoteRec, createSentQuoteReq)
	if createSentQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sent quote create, got %d with body %s", createSentQuoteRec.Code, createSentQuoteRec.Body.String())
	}
	var sentQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createSentQuoteRec.Body.Bytes(), &sentQuote); err != nil {
		t.Fatalf("decode sent quote create response: %v", err)
	}
	sentQuoteStatusReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+sentQuote.ID+"/status", bytes.NewReader([]byte(`{"status":"sent"}`)))
	sentQuoteStatusReq.Header.Set("Authorization", "Bearer "+accessToken)
	sentQuoteStatusReq.Header.Set("Content-Type", "application/json")
	sentQuoteStatusRec := httptest.NewRecorder()
	handler.ServeHTTP(sentQuoteStatusRec, sentQuoteStatusReq)
	if sentQuoteStatusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sent quote status update, got %d with body %s", sentQuoteStatusRec.Code, sentQuoteStatusRec.Body.String())
	}

	createAcceptedQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Accepted Position","qty":1,"unit":"Stk","unit_price":900,"tax_code":"DE19"}]
	}`)))
	createAcceptedQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createAcceptedQuoteReq.Header.Set("Content-Type", "application/json")
	createAcceptedQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createAcceptedQuoteRec, createAcceptedQuoteReq)
	if createAcceptedQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for accepted quote create, got %d with body %s", createAcceptedQuoteRec.Code, createAcceptedQuoteRec.Body.String())
	}
	var acceptedQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAcceptedQuoteRec.Body.Bytes(), &acceptedQuote); err != nil {
		t.Fatalf("decode accepted quote create response: %v", err)
	}
	acceptQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+acceptedQuote.ID+"/accept", bytes.NewReader([]byte(`{}`)))
	acceptQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptQuoteReq.Header.Set("Content-Type", "application/json")
	acceptQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptQuoteRec, acceptQuoteReq)
	if acceptQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote accept, got %d with body %s", acceptQuoteRec.Code, acceptQuoteRec.Body.String())
	}

	createPendingOrderQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Order Position","qty":2,"unit":"Stk","unit_price":700,"tax_code":"DE19"}]
	}`)))
	createPendingOrderQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPendingOrderQuoteReq.Header.Set("Content-Type", "application/json")
	createPendingOrderQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createPendingOrderQuoteRec, createPendingOrderQuoteReq)
	if createPendingOrderQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for pending-order quote create, got %d with body %s", createPendingOrderQuoteRec.Code, createPendingOrderQuoteRec.Body.String())
	}
	var pendingOrderQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createPendingOrderQuoteRec.Body.Bytes(), &pendingOrderQuote); err != nil {
		t.Fatalf("decode pending-order quote create response: %v", err)
	}
	acceptPendingOrderQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+pendingOrderQuote.ID+"/accept", bytes.NewReader([]byte(`{}`)))
	acceptPendingOrderQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptPendingOrderQuoteReq.Header.Set("Content-Type", "application/json")
	acceptPendingOrderQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptPendingOrderQuoteRec, acceptPendingOrderQuoteReq)
	if acceptPendingOrderQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for pending-order quote accept, got %d with body %s", acceptPendingOrderQuoteRec.Code, acceptPendingOrderQuoteRec.Body.String())
	}
	convertPendingOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+pendingOrderQuote.ID+"/convert-to-sales-order", nil)
	convertPendingOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertPendingOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(convertPendingOrderRec, convertPendingOrderReq)
	if convertPendingOrderRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for pending sales order conversion, got %d with body %s", convertPendingOrderRec.Code, convertPendingOrderRec.Body.String())
	}
	var pendingSalesOrder struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(convertPendingOrderRec.Body.Bytes(), &pendingSalesOrder); err != nil {
		t.Fatalf("decode pending sales order response: %v", err)
	}

	createPartialOrderQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Partial Position","qty":3,"unit":"Stk","unit_price":400,"tax_code":"DE19"}]
	}`)))
	createPartialOrderQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPartialOrderQuoteReq.Header.Set("Content-Type", "application/json")
	createPartialOrderQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createPartialOrderQuoteRec, createPartialOrderQuoteReq)
	if createPartialOrderQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for partial-order quote create, got %d with body %s", createPartialOrderQuoteRec.Code, createPartialOrderQuoteRec.Body.String())
	}
	var partialOrderQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createPartialOrderQuoteRec.Body.Bytes(), &partialOrderQuote); err != nil {
		t.Fatalf("decode partial-order quote create response: %v", err)
	}
	acceptPartialOrderQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+partialOrderQuote.ID+"/accept", bytes.NewReader([]byte(`{}`)))
	acceptPartialOrderQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptPartialOrderQuoteReq.Header.Set("Content-Type", "application/json")
	acceptPartialOrderQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptPartialOrderQuoteRec, acceptPartialOrderQuoteReq)
	if acceptPartialOrderQuoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for partial-order quote accept, got %d with body %s", acceptPartialOrderQuoteRec.Code, acceptPartialOrderQuoteRec.Body.String())
	}
	convertPartialOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+partialOrderQuote.ID+"/convert-to-sales-order", nil)
	convertPartialOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertPartialOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(convertPartialOrderRec, convertPartialOrderReq)
	if convertPartialOrderRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for partial sales order conversion, got %d with body %s", convertPartialOrderRec.Code, convertPartialOrderRec.Body.String())
	}
	var partialSalesOrder struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(convertPartialOrderRec.Body.Bytes(), &partialSalesOrder); err != nil {
		t.Fatalf("decode partial sales order response: %v", err)
	}
	if len(partialSalesOrder.Items) != 1 {
		t.Fatalf("expected 1 partial sales order item, got %d", len(partialSalesOrder.Items))
	}
	partialInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+partialSalesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-04-03T00:00:00Z",
		"due_date":"2026-04-17T00:00:00Z",
		"revenue_account":"8000",
		"items":[{"sales_order_item_id":"`+partialSalesOrder.Items[0].ID+`","qty":1}]
	}`)))
	partialInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	partialInvoiceReq.Header.Set("Content-Type", "application/json")
	partialInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(partialInvoiceRec, partialInvoiceReq)
	if partialInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for partial invoice conversion, got %d with body %s", partialInvoiceRec.Code, partialInvoiceRec.Body.String())
	}

	createSupersededQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Superseded Position","qty":1,"unit":"Stk","unit_price":500,"tax_code":"DE19"}]
	}`)))
	createSupersededQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createSupersededQuoteReq.Header.Set("Content-Type", "application/json")
	createSupersededQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createSupersededQuoteRec, createSupersededQuoteReq)
	if createSupersededQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for superseded quote create, got %d with body %s", createSupersededQuoteRec.Code, createSupersededQuoteRec.Body.String())
	}
	var supersededQuote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createSupersededQuoteRec.Body.Bytes(), &supersededQuote); err != nil {
		t.Fatalf("decode superseded quote create response: %v", err)
	}
	supersededQuoteStatusReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+supersededQuote.ID+"/status", bytes.NewReader([]byte(`{"status":"sent"}`)))
	supersededQuoteStatusReq.Header.Set("Authorization", "Bearer "+accessToken)
	supersededQuoteStatusReq.Header.Set("Content-Type", "application/json")
	supersededQuoteStatusRec := httptest.NewRecorder()
	handler.ServeHTTP(supersededQuoteStatusRec, supersededQuoteStatusReq)
	if supersededQuoteStatusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for superseded quote status update, got %d with body %s", supersededQuoteStatusRec.Code, supersededQuoteStatusRec.Body.String())
	}
	supersededReviseReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+supersededQuote.ID+"/revise", nil)
	supersededReviseReq.Header.Set("Authorization", "Bearer "+accessToken)
	supersededReviseRec := httptest.NewRecorder()
	handler.ServeHTTP(supersededReviseRec, supersededReviseReq)
	if supersededReviseRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for superseded quote revise, got %d with body %s", supersededReviseRec.Code, supersededReviseRec.Body.String())
	}

	workflowReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflow/commercial?project_id="+createdProject.ID, nil)
	workflowReq.Header.Set("Authorization", "Bearer "+accessToken)
	workflowRec := httptest.NewRecorder()
	handler.ServeHTTP(workflowRec, workflowReq)
	if workflowRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for commercial workflow, got %d with body %s", workflowRec.Code, workflowRec.Body.String())
	}

	var workflowResp struct {
		Items []struct {
			Kind            string  `json:"kind"`
			Priority        string  `json:"priority"`
			QuoteID         string  `json:"quote_id"`
			SalesOrderID    string  `json:"sales_order_id"`
			OpenGrossTotal  float64 `json:"open_gross_total"`
			NextActionLabel string  `json:"next_action_label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(workflowRec.Body.Bytes(), &workflowResp); err != nil {
		t.Fatalf("decode commercial workflow response: %v", err)
	}
	if len(workflowResp.Items) != 4 {
		t.Fatalf("expected 4 workflow items, got %d: %s", len(workflowResp.Items), workflowRec.Body.String())
	}

	kinds := make(map[string]bool)
	foundPendingOrder := false
	foundPartialOrder := false
	for _, item := range workflowResp.Items {
		kinds[item.Kind] = true
		if item.QuoteID == supersededQuote.ID {
			t.Fatalf("expected superseded quote %q to be excluded from workflow items", supersededQuote.ID)
		}
		if item.SalesOrderID == pendingSalesOrder.ID {
			foundPendingOrder = item.Kind == "sales_order_pending_invoice" && item.Priority == "high"
		}
		if item.SalesOrderID == partialSalesOrder.ID {
			foundPartialOrder = item.Kind == "sales_order_partially_invoiced" && item.OpenGrossTotal > 0 && item.NextActionLabel == "Restbetrag fakturieren"
		}
	}
	if !kinds["quote_sent_pending"] {
		t.Fatal("expected quote_sent_pending workflow item")
	}
	if !kinds["quote_accepted_pending_followup"] {
		t.Fatal("expected quote_accepted_pending_followup workflow item")
	}
	if !kinds["sales_order_pending_invoice"] {
		t.Fatal("expected sales_order_pending_invoice workflow item")
	}
	if !kinds["sales_order_partially_invoiced"] {
		t.Fatal("expected sales_order_partially_invoiced workflow item")
	}
	if !foundPendingOrder {
		t.Fatal("expected pending sales order item with high priority")
	}
	if !foundPartialOrder {
		t.Fatal("expected partial sales order item with positive open amount and follow-up label")
	}

	filterReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflow/commercial?project_id="+createdProject.ID+"&kind=quote_accepted_pending_followup", nil)
	filterReq.Header.Set("Authorization", "Bearer "+accessToken)
	filterRec := httptest.NewRecorder()
	handler.ServeHTTP(filterRec, filterReq)
	if filterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for filtered commercial workflow, got %d with body %s", filterRec.Code, filterRec.Body.String())
	}
	var filtered struct {
		Items []struct {
			Kind string `json:"kind"`
		} `json:"items"`
	}
	if err := json.Unmarshal(filterRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("decode filtered workflow response: %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].Kind != "quote_accepted_pending_followup" {
		t.Fatalf("expected exactly one filtered accepted-quote item, got %+v", filtered.Items)
	}
}
