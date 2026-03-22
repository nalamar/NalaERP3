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
