package apihttp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/settings"
	"nalaerp3/internal/testutil"
)

type gaebProcessParser struct {
	calls int
}

func (p *gaebProcessParser) ParseGAEB(_ context.Context, source io.Reader, _ string) (quotes.GAEBImportParseResult, error) {
	if _, err := io.ReadAll(source); err != nil {
		return quotes.GAEBImportParseResult{}, err
	}
	p.calls++
	return quotes.GAEBImportParseResult{
		ParserVersion:  "http-parser-v1",
		DetectedFormat: "x83",
		Items: []quotes.QuoteImportItemInput{
			{PositionNo: "01.001", Description: "Fenster", Qty: 1, Unit: "Stk", SortOrder: 1},
		},
	}, nil
}

func TestGAEBImportProcessEndpoint(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "gaeb-process-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "gaeb-process-procurement@example.com", "Secret123!", "procurement")
	parser := &gaebProcessParser{}
	handler := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, env.Cfg, V1RouterOptions{GAEBImportParser: parser})
	adminToken := loginIntegrationUser(t, handler, "gaeb-process-admin@example.com", "Secret123!")
	procurementToken := loginIntegrationUser(t, handler, "gaeb-process-procurement@example.com", "Secret123!")
	contactID, projectID := uuid.NewString(), uuid.NewString()
	if _, err := env.PG.Exec(context.Background(), "INSERT INTO contacts (id, typ, rolle, status, name, email, phone, waehrung) VALUES ($1,'org','customer','active',$2,$3,$4,'EUR')", contactID, "GAEB Prozess Kunde", "gaeb-process@example.com", "+49 211 555555"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(context.Background(), "INSERT INTO projects (id, nummer, name, kunde_id, status, company_id) VALUES ($1,$2,$3,$4,'angebot','default')", projectID, "PRJ-GAEB-PROCESS-0001", "GAEB Prozess Projekt", contactID); err != nil {
		t.Fatal(err)
	}
	svc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	create := func(t *testing.T) *quotes.QuoteImport {
		t.Helper()
		imp, err := svc.CreateGAEBImport(context.Background(), quotes.QuoteImportCreateInput{ProjectID: projectID, ContactID: contactID}, strings.NewReader("source"), "process.x83", "default")
		if err != nil {
			t.Fatal(err)
		}
		return imp
	}
	call := func(h http.Handler, token, id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/"+id+"/process", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	success := create(t)
	if rec := call(handler, adminToken, success.ID); rec.Code != http.StatusOK {
		t.Fatalf("success: %d %s", rec.Code, rec.Body.String())
	}
	if parser.calls != 1 {
		t.Fatalf("parser calls: %d", parser.calls)
	}
	if rec := call(handler, adminToken, success.ID); rec.Code != http.StatusConflict || parser.calls != 1 {
		t.Fatalf("conflict: %d calls=%d", rec.Code, parser.calls)
	}
	if rec := call(handler, procurementToken, create(t).ID); rec.Code != http.StatusForbidden || parser.calls != 1 {
		t.Fatalf("forbidden: %d calls=%d", rec.Code, parser.calls)
	}
	defaultImport, err := svc.CreateGAEBImport(context.Background(), quotes.QuoteImportCreateInput{ProjectID: projectID, ContactID: contactID}, strings.NewReader("<gaeb><item position_no=\"01.001\" qty=\"1\" unit=\"Stk\"><description>Fenster</description></item></gaeb>"), "default.xml", "default")
	if err != nil {
		t.Fatalf("create default XML import: %v", err)
	}
	if rec := call(NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg), adminToken, defaultImport.ID); rec.Code != http.StatusOK {
		t.Fatalf("default parser: %d %s", rec.Code, rec.Body.String())
	}
	defaultParsed, err := svc.GetImport(context.Background(), defaultImport.ID, "default")
	if err != nil || defaultParsed.Status != "parsed" || defaultParsed.ParserVersion != "gaeb-xml-subset-v1" || defaultParsed.ItemCount != 1 {
		t.Fatalf("default parser result: %+v err=%v", defaultParsed, err)
	}
	noParser := create(t)
	noParserHandler := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, env.Cfg, V1RouterOptions{})
	if rec := call(noParserHandler, adminToken, noParser.ID); rec.Code != http.StatusInternalServerError {
		t.Fatalf("missing parser: %d %s", rec.Code, rec.Body.String())
	}
	after, err := svc.GetImport(context.Background(), noParser.ID, "default")
	if err != nil || after.Status != "uploaded" {
		t.Fatalf("no-parser import: %+v err=%v", after, err)
	}
}

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

func TestQuoteApprovalDecisionEndpointsRequireApprovePermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	adminID := testutil.SeedAuthUser(t, env, "integration-approval-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-approval-sales@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-approval-admin@example.com", "Secret123!")
	salesToken := loginIntegrationUser(t, handler, "integration-approval-sales@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Approval Endpoint Kunde GmbH",
		"email":    "approval-endpoint@example.com",
		"telefon":  "+49 211 555100",
		"waehrung": "EUR",
	})

	approveQuoteID, approveItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-APPROVE", 50, 60)
	createApprovalRequestReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+approveQuoteID.String()+"/items/"+approveItemID.String()+"/approval-requests", bytes.NewReader([]byte(`{
		"comment":"Zielmarge pruefen"
	}`)))
	createApprovalRequestReq.Header.Set("Authorization", "Bearer "+adminToken)
	createApprovalRequestReq.Header.Set("Content-Type", "application/json")
	createApprovalRequestRec := httptest.NewRecorder()
	handler.ServeHTTP(createApprovalRequestRec, createApprovalRequestReq)
	if createApprovalRequestRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval request create, got %d with body %s", createApprovalRequestRec.Code, createApprovalRequestRec.Body.String())
	}

	forbiddenApproveReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+approveQuoteID.String()+"/items/"+approveItemID.String()+"/approval-requests/approve", bytes.NewReader([]byte(`{
		"comment":"sales darf nicht freigeben"
	}`)))
	forbiddenApproveReq.Header.Set("Authorization", "Bearer "+salesToken)
	forbiddenApproveReq.Header.Set("Content-Type", "application/json")
	forbiddenApproveRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenApproveRec, forbiddenApproveReq)
	if forbiddenApproveRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for approval without quotes.approve, got %d with body %s", forbiddenApproveRec.Code, forbiddenApproveRec.Body.String())
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+approveQuoteID.String()+"/items/"+approveItemID.String()+"/approval-requests/approve", bytes.NewReader([]byte(`{
		"comment":"wirtschaftlich freigegeben"
	}`)))
	approveReq.Header.Set("Authorization", "Bearer "+adminToken)
	approveReq.Header.Set("Content-Type", "application/json")
	approveRec := httptest.NewRecorder()
	handler.ServeHTTP(approveRec, approveReq)
	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval decision, got %d with body %s", approveRec.Code, approveRec.Body.String())
	}
	var approved struct {
		Status                      string   `json:"status"`
		DecidedBy                   string   `json:"decided_by"`
		DecidedAt                   string   `json:"decided_at"`
		DecisionComment             string   `json:"decision_comment"`
		CurrentUnitPriceSnapshot    float64  `json:"current_unit_price_snapshot"`
		TargetMarginPercentSnapshot float64  `json:"target_margin_percent_snapshot"`
		ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
		ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
	}
	if err := json.Unmarshal(approveRec.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved response: %v", err)
	}
	if approved.Status != "approved" || approved.DecidedBy != adminID || approved.DecidedAt == "" || approved.DecisionComment != "wirtschaftlich freigegeben" {
		t.Fatalf("unexpected approved decision metadata: %+v", approved)
	}
	if approved.ApprovedUnitPriceSnapshot == nil || *approved.ApprovedUnitPriceSnapshot != approved.CurrentUnitPriceSnapshot {
		t.Fatalf("expected approved unit price snapshot from request snapshot, got %+v", approved)
	}
	if approved.ApprovedTargetMarginPercent == nil || *approved.ApprovedTargetMarginPercent != approved.TargetMarginPercentSnapshot {
		t.Fatalf("expected approved target margin snapshot from request snapshot, got %+v", approved)
	}

	approvedHistoryReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+approveQuoteID.String()+"/items/"+approveItemID.String()+"/approval-requests", nil)
	approvedHistoryReq.Header.Set("Authorization", "Bearer "+salesToken)
	approvedHistoryRec := httptest.NewRecorder()
	handler.ServeHTTP(approvedHistoryRec, approvedHistoryReq)
	if approvedHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval history with quotes.read, got %d with body %s", approvedHistoryRec.Code, approvedHistoryRec.Body.String())
	}
	var approvedHistory []struct {
		Status                      string   `json:"status"`
		DecisionComment             string   `json:"decision_comment"`
		RequestedBy                 string   `json:"requested_by"`
		RequestedByName             string   `json:"requested_by_name"`
		DecidedBy                   string   `json:"decided_by"`
		DecidedByName               string   `json:"decided_by_name"`
		ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
		ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
	}
	if err := json.Unmarshal(approvedHistoryRec.Body.Bytes(), &approvedHistory); err != nil {
		t.Fatalf("decode approved history response: %v", err)
	}
	if len(approvedHistory) != 1 {
		t.Fatalf("expected one approved history entry, got %+v", approvedHistory)
	}
	if approvedHistory[0].Status != "approved" || approvedHistory[0].DecisionComment != "wirtschaftlich freigegeben" || approvedHistory[0].DecidedBy != adminID {
		t.Fatalf("unexpected approved history entry: %+v", approvedHistory[0])
	}
	if approvedHistory[0].RequestedBy != adminID || approvedHistory[0].RequestedByName != "Integration Test" || approvedHistory[0].DecidedByName != "Integration Test" {
		t.Fatalf("expected approved history user display names, got %+v", approvedHistory[0])
	}
	if approvedHistory[0].ApprovedUnitPriceSnapshot == nil || approvedHistory[0].ApprovedTargetMarginPercent == nil {
		t.Fatalf("expected approved snapshots in history entry, got %+v", approvedHistory[0])
	}

	approvedReloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+approveQuoteID.String(), nil)
	approvedReloadReq.Header.Set("Authorization", "Bearer "+salesToken)
	approvedReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(approvedReloadRec, approvedReloadReq)
	if approvedReloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approved quote reload, got %d with body %s", approvedReloadRec.Code, approvedReloadRec.Body.String())
	}
	var approvedReload struct {
		Items []struct {
			ActiveApprovalRequest *struct {
				ID string `json:"id"`
			} `json:"active_approval_request"`
			LatestApprovalDecision *struct {
				ID                          string   `json:"id"`
				Status                      string   `json:"status"`
				ReasonCode                  string   `json:"reason_code"`
				ReasonText                  string   `json:"reason_text"`
				DecidedBy                   string   `json:"decided_by"`
				DecidedByName               string   `json:"decided_by_name"`
				DecidedAt                   string   `json:"decided_at"`
				DecisionComment             string   `json:"decision_comment"`
				ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
				ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
			} `json:"latest_approval_decision"`
		} `json:"items"`
	}
	if err := json.Unmarshal(approvedReloadRec.Body.Bytes(), &approvedReload); err != nil {
		t.Fatalf("decode approved quote reload: %v", err)
	}
	if len(approvedReload.Items) != 1 {
		t.Fatalf("expected one approved quote item on reload, got %+v", approvedReload.Items)
	}
	if approvedReload.Items[0].ActiveApprovalRequest != nil {
		t.Fatalf("expected no active approval request after approval, got %+v", approvedReload.Items[0].ActiveApprovalRequest)
	}
	if approvedReload.Items[0].LatestApprovalDecision == nil {
		t.Fatalf("expected latest approval decision badge after approval, got %+v", approvedReload.Items[0])
	}
	approvedBadge := approvedReload.Items[0].LatestApprovalDecision
	if approvedBadge.Status != "approved" || approvedBadge.DecidedBy != adminID || approvedBadge.DecidedByName != "Integration Test" || approvedBadge.DecidedAt == "" || approvedBadge.DecisionComment != "wirtschaftlich freigegeben" {
		t.Fatalf("unexpected approved latest approval decision badge: %+v", approvedBadge)
	}
	// unitPrice=50 < costBasis=60 => reasonCode "negative_margin" (siehe
	// Backlog 0.28, gleiches Muster wie bei
	// TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests).
	if approvedBadge.ReasonCode != "negative_margin" || approvedBadge.ReasonText != "Zielmarge pruefen" {
		t.Fatalf("unexpected approved latest approval decision reason: %+v", approvedBadge)
	}
	if approvedBadge.ApprovedUnitPriceSnapshot == nil || approvedBadge.ApprovedTargetMarginPercent == nil {
		t.Fatalf("expected approved snapshots in latest approval decision badge, got %+v", approvedBadge)
	}

	duplicateRejectReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+approveQuoteID.String()+"/items/"+approveItemID.String()+"/approval-requests/reject", nil)
	duplicateRejectReq.Header.Set("Authorization", "Bearer "+adminToken)
	duplicateRejectRec := httptest.NewRecorder()
	handler.ServeHTTP(duplicateRejectRec, duplicateRejectReq)
	if duplicateRejectRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate decision, got %d with body %s", duplicateRejectRec.Code, duplicateRejectRec.Body.String())
	}

	rejectQuoteID, rejectItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REJECT", 50, 60)
	createRejectApprovalReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+rejectQuoteID.String()+"/items/"+rejectItemID.String()+"/approval-requests", nil)
	createRejectApprovalReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRejectApprovalRec := httptest.NewRecorder()
	handler.ServeHTTP(createRejectApprovalRec, createRejectApprovalReq)
	if createRejectApprovalRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for rejection approval request, got %d with body %s", createRejectApprovalRec.Code, createRejectApprovalRec.Body.String())
	}

	rejectReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+rejectQuoteID.String()+"/items/"+rejectItemID.String()+"/approval-requests/reject", bytes.NewReader([]byte(`{
		"comment":"Preis nacharbeiten"
	}`)))
	rejectReq.Header.Set("Authorization", "Bearer "+adminToken)
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectRec := httptest.NewRecorder()
	handler.ServeHTTP(rejectRec, rejectReq)
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rejection decision, got %d with body %s", rejectRec.Code, rejectRec.Body.String())
	}
	var rejected struct {
		Status                      string   `json:"status"`
		DecidedBy                   string   `json:"decided_by"`
		DecidedAt                   string   `json:"decided_at"`
		DecisionComment             string   `json:"decision_comment"`
		ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
		ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
	}
	if err := json.Unmarshal(rejectRec.Body.Bytes(), &rejected); err != nil {
		t.Fatalf("decode rejected response: %v", err)
	}
	if rejected.Status != "rejected" || rejected.DecidedBy != adminID || rejected.DecidedAt == "" || rejected.DecisionComment != "Preis nacharbeiten" {
		t.Fatalf("unexpected rejected decision metadata: %+v", rejected)
	}
	if rejected.ApprovedUnitPriceSnapshot != nil || rejected.ApprovedTargetMarginPercent != nil {
		t.Fatalf("rejected decision must not carry approved snapshots: %+v", rejected)
	}

	rejectedHistoryReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+rejectQuoteID.String()+"/items/"+rejectItemID.String()+"/approval-requests", nil)
	rejectedHistoryReq.Header.Set("Authorization", "Bearer "+salesToken)
	rejectedHistoryRec := httptest.NewRecorder()
	handler.ServeHTTP(rejectedHistoryRec, rejectedHistoryReq)
	if rejectedHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rejected approval history, got %d with body %s", rejectedHistoryRec.Code, rejectedHistoryRec.Body.String())
	}
	var rejectedHistory []struct {
		Status                      string   `json:"status"`
		DecisionComment             string   `json:"decision_comment"`
		RequestedBy                 string   `json:"requested_by"`
		RequestedByName             string   `json:"requested_by_name"`
		DecidedBy                   string   `json:"decided_by"`
		DecidedByName               string   `json:"decided_by_name"`
		ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
		ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
	}
	if err := json.Unmarshal(rejectedHistoryRec.Body.Bytes(), &rejectedHistory); err != nil {
		t.Fatalf("decode rejected history response: %v", err)
	}
	if len(rejectedHistory) != 1 || rejectedHistory[0].Status != "rejected" || rejectedHistory[0].DecisionComment != "Preis nacharbeiten" {
		t.Fatalf("unexpected rejected history response: %+v", rejectedHistory)
	}
	if rejectedHistory[0].RequestedBy != adminID || rejectedHistory[0].RequestedByName != "Integration Test" || rejectedHistory[0].DecidedBy != adminID || rejectedHistory[0].DecidedByName != "Integration Test" {
		t.Fatalf("expected rejected history user display names, got %+v", rejectedHistory[0])
	}
	if rejectedHistory[0].ApprovedUnitPriceSnapshot != nil || rejectedHistory[0].ApprovedTargetMarginPercent != nil {
		t.Fatalf("rejected history must not carry approved snapshots: %+v", rejectedHistory[0])
	}

	rejectedReloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+rejectQuoteID.String(), nil)
	rejectedReloadReq.Header.Set("Authorization", "Bearer "+salesToken)
	rejectedReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(rejectedReloadRec, rejectedReloadReq)
	if rejectedReloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rejected quote reload, got %d with body %s", rejectedReloadRec.Code, rejectedReloadRec.Body.String())
	}
	var rejectedReload struct {
		Items []struct {
			ActiveApprovalRequest *struct {
				ID string `json:"id"`
			} `json:"active_approval_request"`
			LatestApprovalDecision *struct {
				Status                      string   `json:"status"`
				DecidedBy                   string   `json:"decided_by"`
				DecidedByName               string   `json:"decided_by_name"`
				DecidedAt                   string   `json:"decided_at"`
				DecisionComment             string   `json:"decision_comment"`
				ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
				ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
			} `json:"latest_approval_decision"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rejectedReloadRec.Body.Bytes(), &rejectedReload); err != nil {
		t.Fatalf("decode rejected quote reload: %v", err)
	}
	if len(rejectedReload.Items) != 1 {
		t.Fatalf("expected one rejected quote item on reload, got %+v", rejectedReload.Items)
	}
	if rejectedReload.Items[0].ActiveApprovalRequest != nil {
		t.Fatalf("expected no active approval request after rejection, got %+v", rejectedReload.Items[0].ActiveApprovalRequest)
	}
	if rejectedReload.Items[0].LatestApprovalDecision == nil {
		t.Fatalf("expected latest approval decision badge after rejection, got %+v", rejectedReload.Items[0])
	}
	rejectedBadge := rejectedReload.Items[0].LatestApprovalDecision
	if rejectedBadge.Status != "rejected" || rejectedBadge.DecidedBy != adminID || rejectedBadge.DecidedByName != "Integration Test" || rejectedBadge.DecidedAt == "" || rejectedBadge.DecisionComment != "Preis nacharbeiten" {
		t.Fatalf("unexpected rejected latest approval decision badge: %+v", rejectedBadge)
	}
	if rejectedBadge.ApprovedUnitPriceSnapshot != nil || rejectedBadge.ApprovedTargetMarginPercent != nil {
		t.Fatalf("rejected latest approval decision badge must not carry approved snapshots: %+v", rejectedBadge)
	}

	forbiddenResolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+rejectQuoteID.String()+"/items/"+rejectItemID.String()+"/approval-rework/resolve", bytes.NewReader([]byte(`{
		"comment":"sales darf Nacharbeit nicht abschliessen"
	}`)))
	forbiddenResolveReq.Header.Set("Authorization", "Bearer "+salesToken)
	forbiddenResolveReq.Header.Set("Content-Type", "application/json")
	forbiddenResolveRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenResolveRec, forbiddenResolveReq)
	if forbiddenResolveRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for rework resolve without quotes.approve, got %d with body %s", forbiddenResolveRec.Code, forbiddenResolveRec.Body.String())
	}

	blockedResolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+rejectQuoteID.String()+"/items/"+rejectItemID.String()+"/approval-rework/resolve", bytes.NewReader([]byte(`{
		"comment":"noch nicht erledigt"
	}`)))
	blockedResolveReq.Header.Set("Authorization", "Bearer "+adminToken)
	blockedResolveReq.Header.Set("Content-Type", "application/json")
	blockedResolveRec := httptest.NewRecorder()
	handler.ServeHTTP(blockedResolveRec, blockedResolveReq)
	if blockedResolveRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unresolved rework below target margin, got %d with body %s", blockedResolveRec.Code, blockedResolveRec.Body.String())
	}
	if !strings.Contains(blockedResolveRec.Body.String(), "Zielmarge noch nicht") {
		t.Fatalf("expected target margin validation for blocked rework resolve, got body %s", blockedResolveRec.Body.String())
	}

	if _, err := env.PG.Exec(context.Background(), `UPDATE quote_items SET unit_price = 72.00 WHERE id = $1`, rejectItemID); err != nil {
		t.Fatalf("raise rejected quote item to target margin: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+rejectQuoteID.String()+"/items/"+rejectItemID.String()+"/approval-rework/resolve", bytes.NewReader([]byte(`{
		"comment":"Zielmarge nach Nacharbeit erreicht"
	}`)))
	resolveReq.Header.Set("Authorization", "Bearer "+adminToken)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	handler.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rework resolve, got %d with body %s", resolveRec.Code, resolveRec.Body.String())
	}
	var resolved struct {
		Status                      string   `json:"status"`
		DecidedBy                   string   `json:"decided_by"`
		DecidedAt                   string   `json:"decided_at"`
		DecisionComment             string   `json:"decision_comment"`
		CurrentUnitPriceSnapshot    float64  `json:"current_unit_price_snapshot"`
		TargetMarginPercentSnapshot float64  `json:"target_margin_percent_snapshot"`
		ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
		ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
	}
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("decode rework resolved response: %v", err)
	}
	if resolved.Status != "rework_resolved" || resolved.DecidedBy != adminID || resolved.DecidedAt == "" || resolved.DecisionComment != "Zielmarge nach Nacharbeit erreicht" {
		t.Fatalf("unexpected rework resolved metadata: %+v", resolved)
	}
	if resolved.CurrentUnitPriceSnapshot != 72 {
		t.Fatalf("expected current unit price snapshot 72 after rework, got %+v", resolved)
	}
	if resolved.ApprovedUnitPriceSnapshot == nil || *resolved.ApprovedUnitPriceSnapshot != resolved.CurrentUnitPriceSnapshot {
		t.Fatalf("expected rework resolved unit price snapshot from current price, got %+v", resolved)
	}
	if resolved.ApprovedTargetMarginPercent == nil || *resolved.ApprovedTargetMarginPercent != resolved.TargetMarginPercentSnapshot {
		t.Fatalf("expected rework resolved target margin snapshot, got %+v", resolved)
	}

	resolvedReloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+rejectQuoteID.String(), nil)
	resolvedReloadReq.Header.Set("Authorization", "Bearer "+salesToken)
	resolvedReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(resolvedReloadRec, resolvedReloadReq)
	if resolvedReloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rework resolved quote reload, got %d with body %s", resolvedReloadRec.Code, resolvedReloadRec.Body.String())
	}
	var resolvedReload struct {
		Items []struct {
			LatestApprovalDecision *struct {
				Status                      string   `json:"status"`
				DecisionComment             string   `json:"decision_comment"`
				ApprovedUnitPriceSnapshot   *float64 `json:"approved_unit_price_snapshot"`
				ApprovedTargetMarginPercent *float64 `json:"approved_target_margin_percent_snapshot"`
			} `json:"latest_approval_decision"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resolvedReloadRec.Body.Bytes(), &resolvedReload); err != nil {
		t.Fatalf("decode rework resolved quote reload: %v", err)
	}
	if len(resolvedReload.Items) != 1 || resolvedReload.Items[0].LatestApprovalDecision == nil {
		t.Fatalf("expected rework resolved latest decision on reload, got %+v", resolvedReload.Items)
	}
	resolvedBadge := resolvedReload.Items[0].LatestApprovalDecision
	if resolvedBadge.Status != "rework_resolved" || resolvedBadge.DecisionComment != "Zielmarge nach Nacharbeit erreicht" {
		t.Fatalf("unexpected rework resolved latest decision badge: %+v", resolvedBadge)
	}
	if resolvedBadge.ApprovedUnitPriceSnapshot == nil || resolvedBadge.ApprovedTargetMarginPercent == nil {
		t.Fatalf("expected resolved badge snapshots, got %+v", resolvedBadge)
	}
}

func TestQuoteApprovalReworkQueueEndpointListsOnlyOpenLatestRejections(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-rework-queue-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-rework-queue-sales@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-rework-queue-admin@example.com", "Secret123!")
	salesToken := loginIntegrationUser(t, handler, "integration-rework-queue-sales@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Approval Queue Kunde GmbH",
		"email":    "approval-queue@example.com",
		"telefon":  "+49 211 555105",
		"waehrung": "EUR",
	})
	projectID := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO projects (id, nummer, name, kunde_id, status)
		VALUES ($1, 'PRJ-APPROVAL-QUEUE', 'Approval Queue Projekt', $2, 'angebot')
	`, projectID, customerID); err != nil {
		t.Fatalf("seed approval rework queue project: %v", err)
	}

	rejectedQuoteID, rejectedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-QUEUE-OPEN", 50, 60)
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET project_id = $1 WHERE id = $2`, projectID, rejectedQuoteID); err != nil {
		t.Fatalf("assign project to rejected queue quote: %v", err)
	}
	createHTTPApprovalRequest(t, handler, adminToken, rejectedQuoteID, rejectedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, rejectedQuoteID, rejectedItemID, "reject", "Preis nacharbeiten")

	approvedQuoteID, approvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-QUEUE-APPROVED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "reject", "zuerst abgelehnt")
	createHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "Nacharbeit erneut pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "approve", "nach Nacharbeit freigegeben")

	resolvedQuoteID, resolvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-QUEUE-RESOLVED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "reject", "Preis nacharbeiten")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quote_items SET unit_price = 72.00 WHERE id = $1`, resolvedItemID); err != nil {
		t.Fatalf("raise resolved queue quote item to target margin: %v", err)
	}
	resolveHTTPApprovalRework(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge nach Nacharbeit erreicht")

	historicalQuoteID, historicalItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-QUEUE-HISTORIC", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, historicalQuoteID, historicalItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, historicalQuoteID, historicalItemID, "reject", "historische Version")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET superseded_by_quote_id = $1 WHERE id = $2`, rejectedQuoteID, historicalQuoteID); err != nil {
		t.Fatalf("mark historical queue quote superseded: %v", err)
	}

	queueReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-rework", nil)
	queueReq.Header.Set("Authorization", "Bearer "+salesToken)
	queueRec := httptest.NewRecorder()
	handler.ServeHTTP(queueRec, queueReq)
	if queueRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval rework queue, got %d with body %s", queueRec.Code, queueRec.Body.String())
	}
	var queue struct {
		Items []struct {
			QuoteID                     string   `json:"quote_id"`
			QuoteNumber                 string   `json:"quote_number"`
			QuoteStatus                 string   `json:"quote_status"`
			ProjectID                   string   `json:"project_id"`
			ProjectName                 string   `json:"project_name"`
			ContactID                   string   `json:"contact_id"`
			ContactName                 string   `json:"contact_name"`
			QuoteItemID                 string   `json:"quote_item_id"`
			Position                    int      `json:"position"`
			Description                 string   `json:"description"`
			CurrentUnitPrice            float64  `json:"current_unit_price"`
			Currency                    string   `json:"currency"`
			ApprovalRequestID           string   `json:"approval_request_id"`
			ReasonCode                  string   `json:"reason_code"`
			ReasonText                  string   `json:"reason_text"`
			DecisionComment             string   `json:"decision_comment"`
			DecidedByName               string   `json:"decided_by_name"`
			DecidedAt                   string   `json:"decided_at"`
			CurrentUnitPriceSnapshot    float64  `json:"current_unit_price_snapshot"`
			CostBasisUnitPriceSnapshot  float64  `json:"cost_basis_unit_price_snapshot"`
			TargetUnitPriceSnapshot     float64  `json:"target_unit_price_snapshot"`
			TargetMarginPercentSnapshot float64  `json:"target_margin_percent_snapshot"`
			TargetDifferenceSnapshot    float64  `json:"target_difference_snapshot"`
			MarginPercentSnapshot       *float64 `json:"margin_percent_snapshot"`
			PriceDecisionID             string   `json:"price_decision_id"`
			CurrentTargetStatus         string   `json:"current_target_status"`
			CurrentTargetDifference     *float64 `json:"current_target_difference"`
			CurrentTargetUnitPrice      *float64 `json:"current_target_unit_price"`
		} `json:"items"`
	}
	if err := json.Unmarshal(queueRec.Body.Bytes(), &queue); err != nil {
		t.Fatalf("decode approval rework queue response: %v", err)
	}
	if len(queue.Items) != 1 {
		t.Fatalf("expected exactly one open approval rework queue item, got %+v", queue.Items)
	}
	item := queue.Items[0]
	if item.QuoteID != rejectedQuoteID.String() || item.QuoteItemID != rejectedItemID.String() || item.QuoteNumber != "ANG-HTTP-REWORK-QUEUE-OPEN" {
		t.Fatalf("unexpected queue item identity: %+v", item)
	}
	if item.ProjectID != projectID.String() || item.ProjectName != "Approval Queue Projekt" || item.ContactID != customerID || item.ContactName != "Approval Queue Kunde GmbH" {
		t.Fatalf("unexpected queue context: %+v", item)
	}
	if item.QuoteStatus != "draft" || item.Position != 1 || item.Description != "Freigabeposition" || item.Currency != "EUR" {
		t.Fatalf("unexpected queue quote or item fields: %+v", item)
	}
	if item.ApprovalRequestID == "" || item.DecidedAt == "" || item.DecidedByName != "Integration Test" {
		t.Fatalf("expected decision metadata in queue item, got %+v", item)
	}
	// unitPrice=50 < costBasis=60 => reasonCode "negative_margin" (siehe
	// Backlog 0.28, gleiches Muster wie bei
	// TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests).
	if item.ReasonCode != "negative_margin" || item.ReasonText != "Zielmarge pruefen" || item.DecisionComment != "Preis nacharbeiten" {
		t.Fatalf("unexpected rejection details in queue item: %+v", item)
	}
	if item.CurrentUnitPrice != 50 || item.CurrentUnitPriceSnapshot != 50 || item.CostBasisUnitPriceSnapshot != 60 || item.TargetMarginPercentSnapshot != 20 {
		t.Fatalf("unexpected queue price snapshots: %+v", item)
	}
	if item.TargetUnitPriceSnapshot != 72 || item.TargetDifferenceSnapshot != -22 {
		t.Fatalf("unexpected target snapshot values: %+v", item)
	}
	if item.MarginPercentSnapshot == nil || *item.MarginPercentSnapshot > -16.66 || *item.MarginPercentSnapshot < -16.67 {
		t.Fatalf("expected margin percent snapshot about -16.67, got %+v", item)
	}
	if item.PriceDecisionID == "" || item.CurrentTargetStatus != "below_cost" {
		t.Fatalf("expected current target metadata in queue item, got %+v", item)
	}
	if item.CurrentTargetUnitPrice == nil || *item.CurrentTargetUnitPrice != 72 || item.CurrentTargetDifference == nil || *item.CurrentTargetDifference != -22 {
		t.Fatalf("unexpected current target values: %+v", item)
	}

	projectFilterReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-rework?project_id="+projectID.String(), nil)
	projectFilterReq.Header.Set("Authorization", "Bearer "+salesToken)
	projectFilterRec := httptest.NewRecorder()
	handler.ServeHTTP(projectFilterRec, projectFilterReq)
	if projectFilterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for project-filtered approval rework queue, got %d with body %s", projectFilterRec.Code, projectFilterRec.Body.String())
	}
	var projectFiltered struct {
		Items []struct {
			QuoteID string `json:"quote_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(projectFilterRec.Body.Bytes(), &projectFiltered); err != nil {
		t.Fatalf("decode project-filtered queue response: %v", err)
	}
	if len(projectFiltered.Items) != 1 || projectFiltered.Items[0].QuoteID != rejectedQuoteID.String() {
		t.Fatalf("unexpected project-filtered queue response: %+v", projectFiltered.Items)
	}

	contactFilterReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-rework?contact_id="+customerID, nil)
	contactFilterReq.Header.Set("Authorization", "Bearer "+salesToken)
	contactFilterRec := httptest.NewRecorder()
	handler.ServeHTTP(contactFilterRec, contactFilterReq)
	if contactFilterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for contact-filtered approval rework queue, got %d with body %s", contactFilterRec.Code, contactFilterRec.Body.String())
	}
	var contactFiltered struct {
		Items []struct {
			QuoteID string `json:"quote_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(contactFilterRec.Body.Bytes(), &contactFiltered); err != nil {
		t.Fatalf("decode contact-filtered queue response: %v", err)
	}
	if len(contactFiltered.Items) != 1 || contactFiltered.Items[0].QuoteID != rejectedQuoteID.String() {
		t.Fatalf("unexpected contact-filtered queue response: %+v", contactFiltered.Items)
	}

	approvedQuoteFilterReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-rework?quote_id="+approvedQuoteID.String(), nil)
	approvedQuoteFilterReq.Header.Set("Authorization", "Bearer "+salesToken)
	approvedQuoteFilterRec := httptest.NewRecorder()
	handler.ServeHTTP(approvedQuoteFilterRec, approvedQuoteFilterReq)
	if approvedQuoteFilterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approved quote-filtered queue, got %d with body %s", approvedQuoteFilterRec.Code, approvedQuoteFilterRec.Body.String())
	}
	var approvedQuoteFiltered struct {
		Items []struct {
			QuoteID string `json:"quote_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(approvedQuoteFilterRec.Body.Bytes(), &approvedQuoteFiltered); err != nil {
		t.Fatalf("decode approved quote-filtered queue response: %v", err)
	}
	if len(approvedQuoteFiltered.Items) != 0 {
		t.Fatalf("expected no queue entry for latest approved quote, got %+v", approvedQuoteFiltered.Items)
	}

	invalidProjectReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-rework?project_id=not-a-uuid", nil)
	invalidProjectReq.Header.Set("Authorization", "Bearer "+salesToken)
	invalidProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidProjectRec, invalidProjectReq)
	if invalidProjectRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid approval rework project filter, got %d with body %s", invalidProjectRec.Code, invalidProjectRec.Body.String())
	}
}

func TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-approval-request-queue-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-approval-request-queue-sales@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-approval-request-queue-admin@example.com", "Secret123!")
	salesToken := loginIntegrationUser(t, handler, "integration-approval-request-queue-sales@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Approval Request Queue Kunde GmbH",
		"email":    "approval-request-queue@example.com",
		"telefon":  "+49 211 555106",
		"waehrung": "EUR",
	})
	projectID := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO projects (id, nummer, name, kunde_id, status)
		VALUES ($1, 'PRJ-APPROVAL-REQUEST-QUEUE', 'Approval Request Queue Projekt', $2, 'angebot')
	`, projectID, customerID); err != nil {
		t.Fatalf("seed approval request queue project: %v", err)
	}

	requestedQuoteID, requestedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-APPROVAL-REQUEST-QUEUE-OPEN", 50, 60)
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET project_id = $1 WHERE id = $2`, projectID, requestedQuoteID); err != nil {
		t.Fatalf("assign project to requested queue quote: %v", err)
	}
	createHTTPApprovalRequest(t, handler, adminToken, requestedQuoteID, requestedItemID, "Zielmarge pruefen")

	approvedQuoteID, approvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-APPROVAL-REQUEST-QUEUE-APPROVED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "approve", "wirtschaftlich freigegeben")

	rejectedQuoteID, rejectedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-APPROVAL-REQUEST-QUEUE-REJECTED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, rejectedQuoteID, rejectedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, rejectedQuoteID, rejectedItemID, "reject", "Preis nacharbeiten")

	cancelledQuoteID, cancelledItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-APPROVAL-REQUEST-QUEUE-CANCELLED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, cancelledQuoteID, cancelledItemID, "Zielmarge pruefen")
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+cancelledQuoteID.String()+"/items/"+cancelledItemID.String()+"/approval-requests/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer "+adminToken)
	cancelRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval request cancel in queue setup, got %d with body %s", cancelRec.Code, cancelRec.Body.String())
	}

	historicalQuoteID, historicalItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-APPROVAL-REQUEST-QUEUE-HISTORIC", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, historicalQuoteID, historicalItemID, "historische Freigabe pruefen")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET superseded_by_quote_id = $1 WHERE id = $2`, requestedQuoteID, historicalQuoteID); err != nil {
		t.Fatalf("mark historical approval request queue quote superseded: %v", err)
	}

	queueReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-requests", nil)
	queueReq.Header.Set("Authorization", "Bearer "+salesToken)
	queueRec := httptest.NewRecorder()
	handler.ServeHTTP(queueRec, queueReq)
	if queueRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval request queue, got %d with body %s", queueRec.Code, queueRec.Body.String())
	}
	var queue struct {
		Items []struct {
			QuoteID                     string   `json:"quote_id"`
			QuoteNumber                 string   `json:"quote_number"`
			QuoteStatus                 string   `json:"quote_status"`
			ProjectID                   string   `json:"project_id"`
			ProjectName                 string   `json:"project_name"`
			ContactID                   string   `json:"contact_id"`
			ContactName                 string   `json:"contact_name"`
			QuoteItemID                 string   `json:"quote_item_id"`
			Position                    int      `json:"position"`
			Description                 string   `json:"description"`
			CurrentUnitPrice            float64  `json:"current_unit_price"`
			Currency                    string   `json:"currency"`
			ApprovalRequestID           string   `json:"approval_request_id"`
			ReasonCode                  string   `json:"reason_code"`
			ReasonText                  string   `json:"reason_text"`
			RequestedBy                 string   `json:"requested_by"`
			RequestedByName             string   `json:"requested_by_name"`
			RequestedAt                 string   `json:"requested_at"`
			CurrentUnitPriceSnapshot    float64  `json:"current_unit_price_snapshot"`
			CostBasisUnitPriceSnapshot  float64  `json:"cost_basis_unit_price_snapshot"`
			TargetUnitPriceSnapshot     float64  `json:"target_unit_price_snapshot"`
			TargetMarginPercentSnapshot float64  `json:"target_margin_percent_snapshot"`
			TargetDifferenceSnapshot    float64  `json:"target_difference_snapshot"`
			MarginPercentSnapshot       *float64 `json:"margin_percent_snapshot"`
			PriceDecisionID             string   `json:"price_decision_id"`
			CurrentTargetStatus         string   `json:"current_target_status"`
			CurrentTargetDifference     *float64 `json:"current_target_difference"`
			CurrentTargetUnitPrice      *float64 `json:"current_target_unit_price"`
		} `json:"items"`
	}
	if err := json.Unmarshal(queueRec.Body.Bytes(), &queue); err != nil {
		t.Fatalf("decode approval request queue response: %v", err)
	}
	if len(queue.Items) != 1 {
		t.Fatalf("expected exactly one active approval request queue item, got %+v", queue.Items)
	}
	item := queue.Items[0]
	if item.QuoteID != requestedQuoteID.String() || item.QuoteItemID != requestedItemID.String() || item.QuoteNumber != "ANG-HTTP-APPROVAL-REQUEST-QUEUE-OPEN" {
		t.Fatalf("unexpected request queue item identity: %+v", item)
	}
	if item.ProjectID != projectID.String() || item.ProjectName != "Approval Request Queue Projekt" || item.ContactID != customerID || item.ContactName != "Approval Request Queue Kunde GmbH" {
		t.Fatalf("unexpected request queue context: %+v", item)
	}
	if item.QuoteStatus != "draft" || item.Position != 1 || item.Description != "Freigabeposition" || item.Currency != "EUR" {
		t.Fatalf("unexpected request queue quote or item fields: %+v", item)
	}
	if item.ApprovalRequestID == "" || item.RequestedBy == "" || item.RequestedAt == "" || item.RequestedByName != "Integration Test" {
		t.Fatalf("expected request metadata in queue item, got %+v", item)
	}
	// unitPrice=50 < costBasis=60 => absoluteMargin=-10 < 0 => reasonCode
	// "negative_margin" (server/internal/quotes/service.go); "below_target_margin"
	// wuerde unitPrice zwischen costBasis und Zielpreis voraussetzen, was
	// bereits durch die uebrigen Assertions unten (TargetDifferenceSnapshot,
	// MarginPercentSnapshot, CurrentTargetStatus="below_cost") widerlegt wird
	// (Backlog 0.28).
	if item.ReasonCode != "negative_margin" || item.ReasonText != "Zielmarge pruefen" {
		t.Fatalf("unexpected request details in queue item: %+v", item)
	}
	if item.CurrentUnitPrice != 50 || item.CurrentUnitPriceSnapshot != 50 || item.CostBasisUnitPriceSnapshot != 60 || item.TargetMarginPercentSnapshot != 20 {
		t.Fatalf("unexpected request queue price snapshots: %+v", item)
	}
	if item.TargetUnitPriceSnapshot != 72 || item.TargetDifferenceSnapshot != -22 {
		t.Fatalf("unexpected request queue target snapshot values: %+v", item)
	}
	if item.MarginPercentSnapshot == nil || *item.MarginPercentSnapshot > -16.66 || *item.MarginPercentSnapshot < -16.67 {
		t.Fatalf("expected request queue margin percent snapshot about -16.67, got %+v", item)
	}
	if item.PriceDecisionID == "" || item.CurrentTargetStatus != "below_cost" {
		t.Fatalf("expected current target metadata in request queue item, got %+v", item)
	}
	if item.CurrentTargetUnitPrice == nil || *item.CurrentTargetUnitPrice != 72 || item.CurrentTargetDifference == nil || *item.CurrentTargetDifference != -22 {
		t.Fatalf("unexpected current target values in request queue item: %+v", item)
	}

	projectFilterReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-requests?project_id="+projectID.String(), nil)
	projectFilterReq.Header.Set("Authorization", "Bearer "+salesToken)
	projectFilterRec := httptest.NewRecorder()
	handler.ServeHTTP(projectFilterRec, projectFilterReq)
	if projectFilterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for project-filtered approval request queue, got %d with body %s", projectFilterRec.Code, projectFilterRec.Body.String())
	}
	var projectFiltered struct {
		Items []struct {
			QuoteID string `json:"quote_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(projectFilterRec.Body.Bytes(), &projectFiltered); err != nil {
		t.Fatalf("decode project-filtered approval request queue response: %v", err)
	}
	if len(projectFiltered.Items) != 1 || projectFiltered.Items[0].QuoteID != requestedQuoteID.String() {
		t.Fatalf("unexpected project-filtered approval request queue response: %+v", projectFiltered.Items)
	}

	contactFilterReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-requests?contact_id="+customerID, nil)
	contactFilterReq.Header.Set("Authorization", "Bearer "+salesToken)
	contactFilterRec := httptest.NewRecorder()
	handler.ServeHTTP(contactFilterRec, contactFilterReq)
	if contactFilterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for contact-filtered approval request queue, got %d with body %s", contactFilterRec.Code, contactFilterRec.Body.String())
	}
	var contactFiltered struct {
		Items []struct {
			QuoteID string `json:"quote_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(contactFilterRec.Body.Bytes(), &contactFiltered); err != nil {
		t.Fatalf("decode contact-filtered approval request queue response: %v", err)
	}
	if len(contactFiltered.Items) != 1 || contactFiltered.Items[0].QuoteID != requestedQuoteID.String() {
		t.Fatalf("unexpected contact-filtered approval request queue response: %+v", contactFiltered.Items)
	}

	approvedQuoteFilterReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-requests?quote_id="+approvedQuoteID.String(), nil)
	approvedQuoteFilterReq.Header.Set("Authorization", "Bearer "+salesToken)
	approvedQuoteFilterRec := httptest.NewRecorder()
	handler.ServeHTTP(approvedQuoteFilterRec, approvedQuoteFilterReq)
	if approvedQuoteFilterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approved quote-filtered request queue, got %d with body %s", approvedQuoteFilterRec.Code, approvedQuoteFilterRec.Body.String())
	}
	var approvedQuoteFiltered struct {
		Items []struct {
			QuoteID string `json:"quote_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(approvedQuoteFilterRec.Body.Bytes(), &approvedQuoteFiltered); err != nil {
		t.Fatalf("decode approved quote-filtered request queue response: %v", err)
	}
	if len(approvedQuoteFiltered.Items) != 0 {
		t.Fatalf("expected no request queue entry for approved quote, got %+v", approvedQuoteFiltered.Items)
	}

	invalidProjectReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-requests?project_id=not-a-uuid", nil)
	invalidProjectReq.Header.Set("Authorization", "Bearer "+salesToken)
	invalidProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidProjectRec, invalidProjectReq)
	if invalidProjectRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid approval request project filter, got %d with body %s", invalidProjectRec.Code, invalidProjectRec.Body.String())
	}

	invalidQuoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/approval-requests?quote_id=not-a-uuid", nil)
	invalidQuoteReq.Header.Set("Authorization", "Bearer "+salesToken)
	invalidQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidQuoteRec, invalidQuoteReq)
	if invalidQuoteRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid approval request quote filter, got %d with body %s", invalidQuoteRec.Code, invalidQuoteRec.Body.String())
	}
}

func TestQuoteStatusBlocksOpenApprovalRework(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-approval-status-admin@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-approval-status-admin@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Approval Status Kunde GmbH",
		"email":    "approval-status@example.com",
		"telefon":  "+49 211 555101",
		"waehrung": "EUR",
	})

	rejectQuoteID, rejectItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-LOCK", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, rejectQuoteID, rejectItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, rejectQuoteID, rejectItemID, "reject", "Preis nacharbeiten")

	sentRec := updateHTTPQuoteStatus(t, handler, adminToken, rejectQuoteID, "sent")
	if sentRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for sent status with open approval rework, got %d with body %s", sentRec.Code, sentRec.Body.String())
	}
	if !strings.Contains(sentRec.Body.String(), "Nacharbeit vor Versand") {
		t.Fatalf("expected approval rework validation message, got body %s", sentRec.Body.String())
	}

	acceptedRec := updateHTTPQuoteStatus(t, handler, adminToken, rejectQuoteID, "accepted")
	if acceptedRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for accepted status with open approval rework, got %d with body %s", acceptedRec.Code, acceptedRec.Body.String())
	}

	rejectedRec := updateHTTPQuoteStatus(t, handler, adminToken, rejectQuoteID, "rejected")
	if rejectedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote-level rejected status despite open approval rework, got %d with body %s", rejectedRec.Code, rejectedRec.Body.String())
	}

	approvedQuoteID, approvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-APPROVED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "reject", "zuerst abgelehnt")
	createHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "nachgearbeitet erneut pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, approvedQuoteID, approvedItemID, "approve", "nach Nacharbeit freigegeben")

	approvedSentRec := updateHTTPQuoteStatus(t, handler, adminToken, approvedQuoteID, "sent")
	if approvedSentRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sent status after later approval decision, got %d with body %s", approvedSentRec.Code, approvedSentRec.Body.String())
	}

	resolvedQuoteID, resolvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-RESOLVED-STATUS", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "reject", "Preis nacharbeiten")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quote_items SET unit_price=72, net_amount=72 WHERE id=$1`, resolvedItemID); err != nil {
		t.Fatalf("seed resolved approval rework item price: %v", err)
	}
	resolveHTTPApprovalRework(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge nach Nacharbeit erreicht")

	resolvedSentRec := updateHTTPQuoteStatus(t, handler, adminToken, resolvedQuoteID, "sent")
	if resolvedSentRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sent status after resolved approval rework, got %d with body %s", resolvedSentRec.Code, resolvedSentRec.Body.String())
	}
}

func TestQuoteConvertToInvoiceBlocksOpenApprovalRework(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-approval-invoice-admin@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-approval-invoice-admin@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Approval Invoice Kunde GmbH",
		"email":    "approval-invoice@example.com",
		"telefon":  "+49 211 555102",
		"waehrung": "EUR",
	})

	quoteID, itemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-INVOICE", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, quoteID, itemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, quoteID, itemID, "reject", "Preis nacharbeiten")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET status='sent' WHERE id=$1`, quoteID); err != nil {
		t.Fatalf("seed sent quote status: %v", err)
	}

	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID.String()+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-03-17T00:00:00Z",
		"due_date":"2026-03-31T00:00:00Z",
		"revenue_account":"8000"
	}`)))
	convertReq.Header.Set("Authorization", "Bearer "+adminToken)
	convertReq.Header.Set("Content-Type", "application/json")
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invoice conversion with open approval rework, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}
	if !strings.Contains(convertRec.Body.String(), "Nacharbeit vor Versand") {
		t.Fatalf("expected approval rework validation message, got body %s", convertRec.Body.String())
	}

	var linkedInvoiceOutID sql.NullString
	var status string
	if err := env.PG.QueryRow(context.Background(), `SELECT status, linked_invoice_out_id::text FROM quotes WHERE id=$1`, quoteID).Scan(&status, &linkedInvoiceOutID); err != nil {
		t.Fatalf("reload quote after blocked invoice conversion: %v", err)
	}
	if status != "sent" || linkedInvoiceOutID.Valid {
		t.Fatalf("expected quote to remain sent without linked invoice, got status=%q linked=%v", status, linkedInvoiceOutID)
	}

	resolvedQuoteID, resolvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-INVOICE-RESOLVED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "reject", "Preis nacharbeiten")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quote_items SET unit_price=72, net_amount=72 WHERE id=$1`, resolvedItemID); err != nil {
		t.Fatalf("seed resolved invoice approval rework item price: %v", err)
	}
	resolveHTTPApprovalRework(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge nach Nacharbeit erreicht")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET status='sent' WHERE id=$1`, resolvedQuoteID); err != nil {
		t.Fatalf("seed resolved sent quote status: %v", err)
	}

	resolvedConvertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+resolvedQuoteID.String()+"/convert-to-invoice", bytes.NewReader([]byte(`{
		"invoice_date":"2026-03-18T00:00:00Z",
		"due_date":"2026-04-01T00:00:00Z",
		"revenue_account":"8000"
	}`)))
	resolvedConvertReq.Header.Set("Authorization", "Bearer "+adminToken)
	resolvedConvertReq.Header.Set("Content-Type", "application/json")
	resolvedConvertRec := httptest.NewRecorder()
	handler.ServeHTTP(resolvedConvertRec, resolvedConvertReq)
	if resolvedConvertRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice conversion after resolved approval rework, got %d with body %s", resolvedConvertRec.Code, resolvedConvertRec.Body.String())
	}

	linkedInvoiceOutID = sql.NullString{}
	if err := env.PG.QueryRow(context.Background(), `SELECT linked_invoice_out_id::text FROM quotes WHERE id=$1`, resolvedQuoteID).Scan(&linkedInvoiceOutID); err != nil {
		t.Fatalf("reload resolved quote after invoice conversion: %v", err)
	}
	if !linkedInvoiceOutID.Valid {
		t.Fatalf("expected linked invoice after resolved approval rework conversion")
	}
}

func TestQuoteConvertToSalesOrderBlocksOpenApprovalRework(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-approval-order-admin@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-approval-order-admin@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Approval Order Kunde GmbH",
		"email":    "approval-order@example.com",
		"telefon":  "+49 211 555103",
		"waehrung": "EUR",
	})

	quoteID, itemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-ORDER", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, quoteID, itemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, quoteID, itemID, "reject", "Preis nacharbeiten")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET status='accepted', accepted_at=now() WHERE id=$1`, quoteID); err != nil {
		t.Fatalf("seed accepted quote status: %v", err)
	}

	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID.String()+"/convert-to-sales-order", nil)
	convertReq.Header.Set("Authorization", "Bearer "+adminToken)
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for sales order conversion with open approval rework, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}
	if !strings.Contains(convertRec.Body.String(), "Nacharbeit vor Versand") {
		t.Fatalf("expected approval rework validation message, got body %s", convertRec.Body.String())
	}

	var linkedSalesOrderID sql.NullString
	var status string
	if err := env.PG.QueryRow(context.Background(), `SELECT status, linked_sales_order_id::text FROM quotes WHERE id=$1`, quoteID).Scan(&status, &linkedSalesOrderID); err != nil {
		t.Fatalf("reload quote after blocked sales order conversion: %v", err)
	}
	if status != "accepted" || linkedSalesOrderID.Valid {
		t.Fatalf("expected quote to remain accepted without linked sales order, got status=%q linked=%v", status, linkedSalesOrderID)
	}

	var salesOrderCount int
	if err := env.PG.QueryRow(context.Background(), `SELECT COUNT(*) FROM sales_orders WHERE source_quote_id=$1`, quoteID).Scan(&salesOrderCount); err != nil {
		t.Fatalf("count sales orders after blocked conversion: %v", err)
	}
	if salesOrderCount != 0 {
		t.Fatalf("expected no sales order after blocked conversion, got %d", salesOrderCount)
	}

	resolvedQuoteID, resolvedItemID := seedHTTPApprovalDecisionQuote(t, env, customerID, "ANG-HTTP-REWORK-ORDER-RESOLVED", 50, 60)
	createHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge pruefen")
	decideHTTPApprovalRequest(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "reject", "Preis nacharbeiten")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quote_items SET unit_price=72, net_amount=72 WHERE id=$1`, resolvedItemID); err != nil {
		t.Fatalf("seed resolved sales order approval rework item price: %v", err)
	}
	resolveHTTPApprovalRework(t, handler, adminToken, resolvedQuoteID, resolvedItemID, "Zielmarge nach Nacharbeit erreicht")
	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET status='accepted', accepted_at=now() WHERE id=$1`, resolvedQuoteID); err != nil {
		t.Fatalf("seed resolved accepted quote status: %v", err)
	}

	resolvedConvertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+resolvedQuoteID.String()+"/convert-to-sales-order", nil)
	resolvedConvertReq.Header.Set("Authorization", "Bearer "+adminToken)
	resolvedConvertRec := httptest.NewRecorder()
	handler.ServeHTTP(resolvedConvertRec, resolvedConvertReq)
	if resolvedConvertRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sales order conversion after resolved approval rework, got %d with body %s", resolvedConvertRec.Code, resolvedConvertRec.Body.String())
	}

	linkedSalesOrderID = sql.NullString{}
	if err := env.PG.QueryRow(context.Background(), `SELECT linked_sales_order_id::text FROM quotes WHERE id=$1`, resolvedQuoteID).Scan(&linkedSalesOrderID); err != nil {
		t.Fatalf("reload resolved quote after sales order conversion: %v", err)
	}
	if !linkedSalesOrderID.Valid {
		t.Fatalf("expected linked sales order after resolved approval rework conversion")
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
	}, "default"); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID, "default")
	if err != nil || len(items) != 3 {
		t.Fatalf("expected 3 import items, got %d err=%v", len(items), err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[0].ID, "accepted", "Übernehmen", "default"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[1].ID, "rejected", "Nicht übernehmen", "default"); err != nil {
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
	}, "default")
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
	}, "default"); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID, "default")
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
	}, "default"); err != nil {
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
	}, "default"); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID, "default")
	if err != nil || len(items) != 2 {
		t.Fatalf("expected two import items, got %d err=%v", len(items), err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[0].ID, "accepted", "Übernehmen", "default"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[1].ID, "rejected", "Nicht übernehmen", "default"); err != nil {
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

	quoteSvc := quotes.NewService(env.PG, settings.NewNumberingService(env.PG)).WithMongo(env.Mongo, env.Cfg.MongoDB)
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "parser-v1", "x83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01.001",
			OutlineNo:   "01",
			Description: "Aluminium Profil 70mm",
			Qty:         4,
			Unit:        "m",
			SortOrder:   1,
		},
	}, "default"); err != nil {
		t.Fatalf("save import parse result: %v", err)
	}

	items, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID, "default")
	if err != nil || len(items) != 1 {
		t.Fatalf("expected one import item, got %d err=%v", len(items), err)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, items[0].ID, "accepted", "Übernehmen", "default"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.MarkImportReviewed(uploadReq.Context(), createdImport.ID, "default"); err != nil {
		t.Fatalf("mark import reviewed: %v", err)
	}

	applied, err := quoteSvc.ApplyImportToDraftQuote(uploadReq.Context(), createdImport.ID, "default")
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

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", uploadBody)
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

	quoteSvc := quotes.NewService(env.PG, settings.NewNumberingService(env.PG))
	if _, err := quoteSvc.SaveImportParseResult(uploadReq.Context(), createdImport.ID, "itest-parser", "GAEB-X83", []quotes.QuoteImportItemInput{
		{
			PositionNo:  "01",
			Description: "Aluminium Profil 90mm",
			Qty:         1,
			Unit:        "Stk",
			SortOrder:   1,
		},
	}, "default"); err != nil {
		t.Fatalf("save parse result: %v", err)
	}
	importItems, err := quoteSvc.ListImportItems(uploadReq.Context(), createdImport.ID, "default")
	if err != nil {
		t.Fatalf("list import items: %v", err)
	}
	if len(importItems) != 1 {
		t.Fatalf("expected one import item, got %+v", importItems)
	}
	if _, err := quoteSvc.UpdateImportItemReview(uploadReq.Context(), createdImport.ID, importItems[0].ID, "accepted", "Passender Kandidat sichtbar", "default"); err != nil {
		t.Fatalf("accept import item: %v", err)
	}
	if _, err := quoteSvc.MarkImportReviewed(uploadReq.Context(), createdImport.ID, "default"); err != nil {
		t.Fatalf("mark import reviewed: %v", err)
	}

	applied, err := quoteSvc.ApplyImportToDraftQuote(uploadReq.Context(), createdImport.ID, "default")
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

func TestQuoteMaterialSearchEndpointSupportsOpenDraftItemSearch(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-material-search@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-material-search@example.com", "Secret123!")
	ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Materialsuche Kunde GmbH",
		"email":    "material-search@example.com",
		"telefon":  "+49 211 777777",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Materialsuche Projekt",
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

	createMaterial := func(number, label string) string {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
			"nummer":"`+number+`",
			"bezeichnung":"`+label+`",
			"einheit":"Stk",
			"kategorie":"profile"
		}`)))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for material create, got %d with body %s", rec.Code, rec.Body.String())
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode material create response: %v", err)
		}
		return created.ID
	}

	firstMaterialID := createMaterial("MAT-SEARCH-0001", "Alpha Profil 90")
	secondMaterialID := createMaterial("MAT-SEARCH-0002", "Alpha Verbinder")
	_ = createMaterial("MAT-SEARCH-9999", "Beta Blech")

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Freie Suchposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].ID == "" {
		t.Fatalf("expected one quote item with id, got %+v", createdQuote.Items)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/material-search?q=Alpha", nil)
	searchReq.Header.Set("Authorization", "Bearer "+accessToken)
	searchRec := httptest.NewRecorder()
	handler.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for material search, got %d with body %s", searchRec.Code, searchRec.Body.String())
	}

	var searchResults []struct {
		MaterialID    string `json:"material_id"`
		MaterialNo    string `json:"material_no"`
		MaterialLabel string `json:"material_label"`
	}
	if err := json.Unmarshal(searchRec.Body.Bytes(), &searchResults); err != nil {
		t.Fatalf("decode material search response: %v", err)
	}
	if len(searchResults) != 2 {
		t.Fatalf("expected 2 material search results, got %+v", searchResults)
	}
	if searchResults[0].MaterialID != firstMaterialID || searchResults[1].MaterialID != secondMaterialID {
		t.Fatalf("unexpected search result ids: %+v", searchResults)
	}
	if searchResults[0].MaterialLabel != "Alpha Profil 90" || searchResults[1].MaterialLabel != "Alpha Verbinder" {
		t.Fatalf("unexpected search result labels: %+v", searchResults)
	}

	emptySearchReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/material-search?q=", nil)
	emptySearchReq.Header.Set("Authorization", "Bearer "+accessToken)
	emptySearchRec := httptest.NewRecorder()
	handler.ServeHTTP(emptySearchRec, emptySearchReq)
	if emptySearchRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty material search query, got %d with body %s", emptySearchRec.Code, emptySearchRec.Body.String())
	}

	mappedQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Bereits gemappt","qty":1,"unit":"Stk","unit_price":0,"tax_code":"","material_id":"`+firstMaterialID+`","price_mapping_status":"manual"}]
	}`)))
	mappedQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	mappedQuoteReq.Header.Set("Content-Type", "application/json")
	mappedQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(mappedQuoteRec, mappedQuoteReq)
	if mappedQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for mapped quote create, got %d with body %s", mappedQuoteRec.Code, mappedQuoteRec.Body.String())
	}

	var mappedQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(mappedQuoteRec.Body.Bytes(), &mappedQuote); err != nil {
		t.Fatalf("decode mapped quote response: %v", err)
	}
	if len(mappedQuote.Items) != 1 || mappedQuote.Items[0].ID == "" {
		t.Fatalf("expected one mapped quote item with id, got %+v", mappedQuote.Items)
	}

	mappedSearchReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+mappedQuote.ID+"/items/"+mappedQuote.Items[0].ID+"/material-search?q=Alpha", nil)
	mappedSearchReq.Header.Set("Authorization", "Bearer "+accessToken)
	mappedSearchRec := httptest.NewRecorder()
	handler.ServeHTTP(mappedSearchRec, mappedSearchReq)
	if mappedSearchRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for material search on already mapped item, got %d with body %s", mappedSearchRec.Code, mappedSearchRec.Body.String())
	}
}

func TestQuoteMaterialSearchApplyEndpointSupportsVisibleSearchResultApply(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-material-search-apply@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-material-search-apply@example.com", "Secret123!")
	ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Materialsuchtreffer Kunde GmbH",
		"email":    "material-search-apply@example.com",
		"telefon":  "+49 211 888888",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Materialsuchtreffer Projekt",
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

	createMaterial := func(number, label string) string {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
			"nummer":"`+number+`",
			"bezeichnung":"`+label+`",
			"einheit":"Stk",
			"kategorie":"profile"
		}`)))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for material create, got %d with body %s", rec.Code, rec.Body.String())
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode material create response: %v", err)
		}
		return created.ID
	}

	firstMaterialID := createMaterial("MAT-SEARCH-APPLY-0001", "Alpha Traeger")
	_ = createMaterial("MAT-SEARCH-APPLY-0002", "Alpha Verbinder")
	otherMaterialID := createMaterial("MAT-SEARCH-APPLY-9999", "Beta Blech")

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Freie Suchposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].ID == "" {
		t.Fatalf("expected one quote item with id, got %+v", createdQuote.Items)
	}

	invalidApplyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/apply-material-search-result", bytes.NewReader([]byte(`{
		"query":"Alpha",
		"material_id":"`+otherMaterialID+`"
	}`)))
	invalidApplyReq.Header.Set("Authorization", "Bearer "+accessToken)
	invalidApplyReq.Header.Set("Content-Type", "application/json")
	invalidApplyRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidApplyRec, invalidApplyReq)
	if invalidApplyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-visible search result apply, got %d with body %s", invalidApplyRec.Code, invalidApplyRec.Body.String())
	}

	missingQueryReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/apply-material-search-result", bytes.NewReader([]byte(`{
		"query":"",
		"material_id":"`+firstMaterialID+`"
	}`)))
	missingQueryReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingQueryReq.Header.Set("Content-Type", "application/json")
	missingQueryRec := httptest.NewRecorder()
	handler.ServeHTTP(missingQueryRec, missingQueryReq)
	if missingQueryRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty search query during apply, got %d with body %s", missingQueryRec.Code, missingQueryRec.Body.String())
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/apply-material-search-result", bytes.NewReader([]byte(`{
		"query":"Alpha",
		"material_id":"`+firstMaterialID+`"
	}`)))
	applyReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyReq.Header.Set("Content-Type", "application/json")
	applyRec := httptest.NewRecorder()
	handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for visible search result apply, got %d with body %s", applyRec.Code, applyRec.Body.String())
	}

	var updatedQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID                 string `json:"id"`
			MaterialID         string `json:"material_id"`
			PriceMappingStatus string `json:"price_mapping_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(applyRec.Body.Bytes(), &updatedQuote); err != nil {
		t.Fatalf("decode search apply response: %v", err)
	}
	if len(updatedQuote.Items) != 1 {
		t.Fatalf("expected one updated quote item, got %+v", updatedQuote.Items)
	}
	if updatedQuote.Items[0].MaterialID != firstMaterialID {
		t.Fatalf("expected material_id %q after search apply, got %+v", firstMaterialID, updatedQuote.Items[0])
	}
	if updatedQuote.Items[0].PriceMappingStatus != "manual" {
		t.Fatalf("expected price_mapping_status manual after search apply, got %+v", updatedQuote.Items[0])
	}
}

func TestQuotePriceSuggestionEndpointSupportsMappedDraftItem(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-price-suggestion@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-price-suggestion@example.com", "Secret123!")
	ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Preisvorschlag Kunde GmbH",
		"email":    "price-suggestion@example.com",
		"telefon":  "+49 211 999999",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Preisvorschlag Projekt",
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
		"nummer":"MAT-PRICE-0001",
		"bezeichnung":"Preisprofil 100",
		"einheit":"Stk",
		"kategorie":"profile"
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

	if _, err := env.PG.Exec(context.Background(), `UPDATE materials SET avg_purchase_price = 42.75, currency = 'EUR' WHERE id = $1`, createdMaterial.ID); err != nil {
		t.Fatalf("seed material avg purchase price: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Gemappte Preisposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].ID == "" {
		t.Fatalf("expected one mapped quote item with id, got %+v", createdQuote.Items)
	}

	suggestionReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-suggestion", nil)
	suggestionReq.Header.Set("Authorization", "Bearer "+accessToken)
	suggestionRec := httptest.NewRecorder()
	handler.ServeHTTP(suggestionRec, suggestionReq)
	if suggestionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price suggestion, got %d with body %s", suggestionRec.Code, suggestionRec.Body.String())
	}

	var suggestion struct {
		MaterialID         string  `json:"material_id"`
		SuggestedUnitPrice float64 `json:"suggested_unit_price"`
		Currency           string  `json:"currency"`
		SourceLabel        string  `json:"source_label"`
	}
	if err := json.Unmarshal(suggestionRec.Body.Bytes(), &suggestion); err != nil {
		t.Fatalf("decode price suggestion response: %v", err)
	}
	if suggestion.MaterialID != createdMaterial.ID {
		t.Fatalf("expected material_id %q in suggestion, got %+v", createdMaterial.ID, suggestion)
	}
	if suggestion.SuggestedUnitPrice != 42.75 {
		t.Fatalf("expected suggested_unit_price 42.75, got %+v", suggestion)
	}
	if suggestion.Currency != "EUR" {
		t.Fatalf("expected EUR currency in suggestion, got %+v", suggestion)
	}
	if suggestion.SourceLabel != "Durchschnittlicher Einkaufspreis" {
		t.Fatalf("unexpected source label in suggestion: %+v", suggestion)
	}

	createOpenQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Offene Preisposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createOpenQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOpenQuoteReq.Header.Set("Content-Type", "application/json")
	createOpenQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createOpenQuoteRec, createOpenQuoteReq)
	if createOpenQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for open quote create, got %d with body %s", createOpenQuoteRec.Code, createOpenQuoteRec.Body.String())
	}

	var openQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createOpenQuoteRec.Body.Bytes(), &openQuote); err != nil {
		t.Fatalf("decode open quote response: %v", err)
	}
	if len(openQuote.Items) != 1 || openQuote.Items[0].ID == "" {
		t.Fatalf("expected one open quote item with id, got %+v", openQuote.Items)
	}

	missingMaterialReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/price-suggestion", nil)
	missingMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialRec, missingMaterialReq)
	if missingMaterialRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for price suggestion without mapped material, got %d with body %s", missingMaterialRec.Code, missingMaterialRec.Body.String())
	}
}

func TestQuoteApplyPriceSuggestionEndpointSupportsMappedDraftItem(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-price-apply@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-price-apply@example.com", "Secret123!")
	ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Preisuebernahme Kunde GmbH",
		"email":    "price-apply@example.com",
		"telefon":  "+49 211 888888",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Preisuebernahme Projekt",
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
		"nummer":"MAT-PRICE-APPLY-0001",
		"bezeichnung":"Preisprofil 200",
		"einheit":"Stk",
		"kategorie":"profile"
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

	if _, err := env.PG.Exec(context.Background(), `UPDATE materials SET avg_purchase_price = 58.5, currency = 'EUR' WHERE id = $1`, createdMaterial.ID); err != nil {
		t.Fatalf("seed material avg purchase price: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Preisanker Position","qty":1,"unit":"Stk","unit_price":0,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].ID == "" {
		t.Fatalf("expected one mapped quote item with id, got %+v", createdQuote.Items)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/apply-price-suggestion", nil)
	applyReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyRec := httptest.NewRecorder()
	handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price apply, got %d with body %s", applyRec.Code, applyRec.Body.String())
	}

	var updatedQuote struct {
		Items []struct {
			UnitPrice          float64 `json:"unit_price"`
			PriceMappingStatus string  `json:"price_mapping_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(applyRec.Body.Bytes(), &updatedQuote); err != nil {
		t.Fatalf("decode price apply response: %v", err)
	}
	if len(updatedQuote.Items) != 1 {
		t.Fatalf("expected one updated quote item, got %+v", updatedQuote.Items)
	}
	if updatedQuote.Items[0].UnitPrice != 58.5 {
		t.Fatalf("expected unit_price 58.5 after price apply, got %+v", updatedQuote.Items[0])
	}
	if updatedQuote.Items[0].PriceMappingStatus != "manual" {
		t.Fatalf("expected price_mapping_status manual after price apply, got %+v", updatedQuote.Items[0])
	}

	createOpenQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Offene Preisposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createOpenQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOpenQuoteReq.Header.Set("Content-Type", "application/json")
	createOpenQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createOpenQuoteRec, createOpenQuoteReq)
	if createOpenQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for open quote create, got %d with body %s", createOpenQuoteRec.Code, createOpenQuoteRec.Body.String())
	}

	var openQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createOpenQuoteRec.Body.Bytes(), &openQuote); err != nil {
		t.Fatalf("decode open quote response: %v", err)
	}
	if len(openQuote.Items) != 1 || openQuote.Items[0].ID == "" {
		t.Fatalf("expected one open quote item with id, got %+v", openQuote.Items)
	}

	missingMaterialReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/apply-price-suggestion", nil)
	missingMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialRec, missingMaterialReq)
	if missingMaterialRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for price apply without mapped material, got %d with body %s", missingMaterialRec.Code, missingMaterialRec.Body.String())
	}
}

func TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-price-history@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-price-history@example.com", "Secret123!")
	ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Preisquellen Kunde GmbH",
		"email":    "price-history-customer@example.com",
		"telefon":  "+49 211 454545",
		"waehrung": "EUR",
	})
	supplierID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "supplier",
		"status":   "active",
		"name":     "Preisquellen Lieferant GmbH",
		"email":    "price-history-supplier@example.com",
		"telefon":  "+49 211 565656",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Preisquellen Projekt",
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
		"nummer":"MAT-PRICE-HISTORY-0001",
		"bezeichnung":"Preisprofil Historie 100",
		"einheit":"Stk",
		"kategorie":"profile"
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

	if _, err := env.PG.Exec(context.Background(), `UPDATE materials SET avg_purchase_price = 61.25, currency = 'EUR' WHERE id = $1`, createdMaterial.ID); err != nil {
		t.Fatalf("seed material avg purchase price: %v", err)
	}

	orderID := uuid.NewString()
	orderItemID := uuid.NewString()
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO purchase_orders (id, supplier_id, number, order_date, currency, status, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, orderID, supplierID, "PO-HISTORY-0001", "2026-04-22", "EUR", "ordered", "Integrationsquelle"); err != nil {
		t.Fatalf("seed purchase order: %v", err)
	}
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO purchase_order_items (id, order_id, position, material_id, description, qty, uom, unit_price, currency)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, orderItemID, orderID, 1, createdMaterial.ID, "Letzte Einkaufsquelle", 5, "Stk", 59.9, "EUR"); err != nil {
		t.Fatalf("seed purchase order item: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Preisquellen Position","qty":1,"unit":"Stk","unit_price":61.25,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].ID == "" {
		t.Fatalf("expected one mapped quote item with id, got %+v", createdQuote.Items)
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-history", nil)
	historyReq.Header.Set("Authorization", "Bearer "+accessToken)
	historyRec := httptest.NewRecorder()
	handler.ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price history, got %d with body %s", historyRec.Code, historyRec.Body.String())
	}

	var history []struct {
		SourceLabel string  `json:"source_label"`
		UnitPrice   float64 `json:"unit_price"`
		Currency    string  `json:"currency"`
		Reference   string  `json:"reference"`
		Date        string  `json:"date"`
	}
	if err := json.Unmarshal(historyRec.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode price history response: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 price history entries, got %+v", history)
	}
	if history[0].SourceLabel != "Durchschnittlicher Einkaufspreis" || history[0].UnitPrice != 61.25 || history[0].Currency != "EUR" {
		t.Fatalf("unexpected avg price history entry: %+v", history[0])
	}
	if history[1].SourceLabel != "Letzter Bestellpreis" || history[1].UnitPrice != 59.9 || history[1].Currency != "EUR" {
		t.Fatalf("unexpected latest purchase price history entry: %+v", history[1])
	}
	if history[1].Reference != "Bestellung PO-HISTORY-0001" {
		t.Fatalf("expected purchase order reference in price history, got %+v", history[1])
	}
	if !strings.Contains(history[1].Date, "2026-04-22") {
		t.Fatalf("expected purchase order date in price history, got %+v", history[1])
	}

	createOpenQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Offene Quellenposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createOpenQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOpenQuoteReq.Header.Set("Content-Type", "application/json")
	createOpenQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createOpenQuoteRec, createOpenQuoteReq)
	if createOpenQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for open quote create, got %d with body %s", createOpenQuoteRec.Code, createOpenQuoteRec.Body.String())
	}

	var openQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createOpenQuoteRec.Body.Bytes(), &openQuote); err != nil {
		t.Fatalf("decode open quote response: %v", err)
	}
	if len(openQuote.Items) != 1 || openQuote.Items[0].ID == "" {
		t.Fatalf("expected one open quote item with id, got %+v", openQuote.Items)
	}

	missingMaterialReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/price-history", nil)
	missingMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialRec, missingMaterialReq)
	if missingMaterialRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for price history without mapped material, got %d with body %s", missingMaterialRec.Code, missingMaterialRec.Body.String())
	}
}

func TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-price-source-priority@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-price-source-priority@example.com", "Secret123!")
	ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Preispriorisierung Kunde GmbH",
		"email":    "price-source-priority-customer@example.com",
		"telefon":  "+49 211 676767",
		"waehrung": "EUR",
	})
	supplierID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "supplier",
		"status":   "active",
		"name":     "Preispriorisierung Lieferant GmbH",
		"email":    "price-source-priority-supplier@example.com",
		"telefon":  "+49 211 787878",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Preispriorisierung Projekt",
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
		"nummer":"MAT-PRICE-SOURCE-0001",
		"bezeichnung":"Preisprofil Priorisierung 100",
		"einheit":"Stk",
		"kategorie":"profile"
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

	if _, err := env.PG.Exec(context.Background(), `UPDATE materials SET avg_purchase_price = 61.25, currency = 'EUR' WHERE id = $1`, createdMaterial.ID); err != nil {
		t.Fatalf("seed material avg purchase price: %v", err)
	}

	orderID := uuid.NewString()
	orderItemID := uuid.NewString()
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO purchase_orders (id, supplier_id, number, order_date, currency, status, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, orderID, supplierID, "PO-PRIORITY-0001", "2026-04-22", "EUR", "ordered", "Integrationsquelle"); err != nil {
		t.Fatalf("seed purchase order: %v", err)
	}
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO purchase_order_items (id, order_id, position, material_id, description, qty, uom, unit_price, currency)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, orderItemID, orderID, 1, createdMaterial.ID, "Letzte Einkaufsquelle", 5, "Stk", 59.9, "EUR"); err != nil {
		t.Fatalf("seed purchase order item: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Preispriorisierung Position","qty":1,"unit":"Stk","unit_price":61.25,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}

	var createdQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 || createdQuote.Items[0].ID == "" {
		t.Fatalf("expected one mapped quote item with id, got %+v", createdQuote.Items)
	}

	priorityReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-source-priority", nil)
	priorityReq.Header.Set("Authorization", "Bearer "+accessToken)
	priorityRec := httptest.NewRecorder()
	handler.ServeHTTP(priorityRec, priorityReq)
	if priorityRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price source priority, got %d with body %s", priorityRec.Code, priorityRec.Body.String())
	}

	var priority []struct {
		SourceLabel    string  `json:"source_label"`
		UnitPrice      float64 `json:"unit_price"`
		Currency       string  `json:"currency"`
		Reference      string  `json:"reference"`
		Date           string  `json:"date"`
		PriorityRank   int     `json:"priority_rank"`
		PriorityReason string  `json:"priority_reason"`
	}
	if err := json.Unmarshal(priorityRec.Body.Bytes(), &priority); err != nil {
		t.Fatalf("decode price source priority response: %v", err)
	}
	if len(priority) != 2 {
		t.Fatalf("expected 2 prioritized price source entries, got %+v", priority)
	}
	if priority[0].SourceLabel != "Letzter Bestellpreis" || priority[0].PriorityRank != 1 || priority[0].UnitPrice != 59.9 {
		t.Fatalf("unexpected primary prioritized source: %+v", priority[0])
	}
	if priority[0].PriorityReason != "Juengste konkrete Einkaufsquelle" {
		t.Fatalf("unexpected primary priority reason: %+v", priority[0])
	}
	if priority[1].SourceLabel != "Durchschnittlicher Einkaufspreis" || priority[1].PriorityRank != 2 || priority[1].UnitPrice != 61.25 {
		t.Fatalf("unexpected secondary prioritized source: %+v", priority[1])
	}
	if priority[1].PriorityReason != "Fallback auf Materialdurchschnitt" {
		t.Fatalf("unexpected secondary priority reason: %+v", priority[1])
	}

	evaluationReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-evaluation", nil)
	evaluationReq.Header.Set("Authorization", "Bearer "+accessToken)
	evaluationRec := httptest.NewRecorder()
	handler.ServeHTTP(evaluationRec, evaluationReq)
	if evaluationRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price evaluation, got %d with body %s", evaluationRec.Code, evaluationRec.Body.String())
	}

	var evaluation struct {
		CurrentUnitPrice       float64  `json:"current_unit_price"`
		Currency               string   `json:"currency"`
		PrimarySourceLabel     string   `json:"primary_source_label"`
		PrimarySourceUnitPrice float64  `json:"primary_source_unit_price"`
		PrimarySourceReference string   `json:"primary_source_reference"`
		AbsoluteDelta          float64  `json:"absolute_delta"`
		RelativeDeltaPercent   *float64 `json:"relative_delta_percent"`
		EvaluationStatus       string   `json:"evaluation_status"`
		EvaluationReason       string   `json:"evaluation_reason"`
	}
	if err := json.Unmarshal(evaluationRec.Body.Bytes(), &evaluation); err != nil {
		t.Fatalf("decode price evaluation response: %v", err)
	}
	if evaluation.CurrentUnitPrice != 61.25 {
		t.Fatalf("unexpected current unit price: %+v", evaluation)
	}
	if evaluation.PrimarySourceLabel != "Letzter Bestellpreis" || evaluation.PrimarySourceUnitPrice != 59.9 {
		t.Fatalf("unexpected primary evaluation source: %+v", evaluation)
	}
	if evaluation.Currency != "EUR" || evaluation.PrimarySourceReference != "Bestellung PO-PRIORITY-0001" {
		t.Fatalf("unexpected evaluation source metadata: %+v", evaluation)
	}
	if evaluation.AbsoluteDelta < 1.34 || evaluation.AbsoluteDelta > 1.36 {
		t.Fatalf("unexpected absolute evaluation delta: %+v", evaluation)
	}
	if evaluation.RelativeDeltaPercent == nil || *evaluation.RelativeDeltaPercent < 2.25 || *evaluation.RelativeDeltaPercent > 2.26 {
		t.Fatalf("unexpected relative evaluation delta: %+v", evaluation)
	}
	if evaluation.EvaluationStatus != "above_cost_basis" {
		t.Fatalf("unexpected evaluation status: %+v", evaluation)
	}
	if evaluation.EvaluationReason != "Aktueller Preis liegt ueber der primaeren Kostenbasis" {
		t.Fatalf("unexpected evaluation reason: %+v", evaluation)
	}

	transparencyReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-decision-transparency", nil)
	transparencyReq.Header.Set("Authorization", "Bearer "+accessToken)
	transparencyRec := httptest.NewRecorder()
	handler.ServeHTTP(transparencyRec, transparencyReq)
	if transparencyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price decision transparency, got %d with body %s", transparencyRec.Code, transparencyRec.Body.String())
	}

	var transparency struct {
		CurrentUnitPrice       float64  `json:"current_unit_price"`
		Currency               string   `json:"currency"`
		PrimarySourceLabel     string   `json:"primary_source_label"`
		PrimarySourceUnitPrice float64  `json:"primary_source_unit_price"`
		PrimarySourceReference string   `json:"primary_source_reference"`
		AbsoluteDelta          float64  `json:"absolute_delta"`
		RelativeDeltaPercent   *float64 `json:"relative_delta_percent"`
		DecisionStatus         string   `json:"decision_status"`
		DecisionReason         string   `json:"decision_reason"`
	}
	if err := json.Unmarshal(transparencyRec.Body.Bytes(), &transparency); err != nil {
		t.Fatalf("decode price decision transparency response: %v", err)
	}
	if transparency.CurrentUnitPrice != 61.25 {
		t.Fatalf("unexpected transparency current unit price: %+v", transparency)
	}
	if transparency.PrimarySourceLabel != "Letzter Bestellpreis" || transparency.PrimarySourceUnitPrice != 59.9 {
		t.Fatalf("unexpected transparency primary source: %+v", transparency)
	}
	if transparency.Currency != "EUR" || transparency.PrimarySourceReference != "Bestellung PO-PRIORITY-0001" {
		t.Fatalf("unexpected transparency source metadata: %+v", transparency)
	}
	if transparency.AbsoluteDelta < 1.34 || transparency.AbsoluteDelta > 1.36 {
		t.Fatalf("unexpected transparency absolute delta: %+v", transparency)
	}
	if transparency.RelativeDeltaPercent == nil || *transparency.RelativeDeltaPercent < 2.25 || *transparency.RelativeDeltaPercent > 2.26 {
		t.Fatalf("unexpected transparency relative delta: %+v", transparency)
	}
	if transparency.DecisionStatus != "differs_from_primary_source" {
		t.Fatalf("unexpected transparency decision status: %+v", transparency)
	}
	if transparency.DecisionReason != "Aktueller Preis weicht von der primaeren Preisquelle ab" {
		t.Fatalf("unexpected transparency decision reason: %+v", transparency)
	}

	applyPrimaryReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/apply-primary-price-source", nil)
	applyPrimaryReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyPrimaryRec := httptest.NewRecorder()
	handler.ServeHTTP(applyPrimaryRec, applyPrimaryReq)
	if applyPrimaryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for primary price source apply, got %d with body %s", applyPrimaryRec.Code, applyPrimaryRec.Body.String())
	}

	var updatedQuote struct {
		NetAmount   float64 `json:"net_amount"`
		TaxAmount   float64 `json:"tax_amount"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			ID                 string  `json:"id"`
			UnitPrice          float64 `json:"unit_price"`
			PriceMappingStatus string  `json:"price_mapping_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(applyPrimaryRec.Body.Bytes(), &updatedQuote); err != nil {
		t.Fatalf("decode primary price source apply response: %v", err)
	}
	if len(updatedQuote.Items) != 1 {
		t.Fatalf("expected one updated quote item, got %+v", updatedQuote.Items)
	}
	if updatedQuote.Items[0].UnitPrice != 59.9 {
		t.Fatalf("expected primary source unit price 59.9 after apply, got %+v", updatedQuote.Items[0])
	}
	if updatedQuote.Items[0].PriceMappingStatus != "manual" {
		t.Fatalf("expected manual price mapping status after primary source apply, got %+v", updatedQuote.Items[0])
	}
	if updatedQuote.NetAmount != 59.9 || updatedQuote.TaxAmount != 0 || updatedQuote.GrossAmount != 59.9 {
		t.Fatalf("expected quote totals to follow primary source price, got %+v", updatedQuote)
	}

	var decision struct {
		Count            int
		MaterialID       string
		DecisionType     string
		SourceLabel      string
		SourceUnitPrice  float64
		AppliedUnitPrice float64
		Currency         string
		SourceReference  string
		HasSourceDate    bool
		HasDecisionTime  bool
	}
	if err := env.PG.QueryRow(context.Background(), `
		SELECT
			COUNT(*) OVER (),
			COALESCE(material_id, ''),
			decision_type,
			source_label,
			source_unit_price,
			applied_unit_price,
			BTRIM(currency),
			source_reference,
			source_date IS NOT NULL,
			created_at IS NOT NULL
		FROM quote_item_price_decisions
		WHERE quote_item_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, createdQuote.Items[0].ID).Scan(
		&decision.Count,
		&decision.MaterialID,
		&decision.DecisionType,
		&decision.SourceLabel,
		&decision.SourceUnitPrice,
		&decision.AppliedUnitPrice,
		&decision.Currency,
		&decision.SourceReference,
		&decision.HasSourceDate,
		&decision.HasDecisionTime,
	); err != nil {
		t.Fatalf("query primary price decision snapshot: %v", err)
	}
	if decision.Count != 1 {
		t.Fatalf("expected one price decision snapshot, got %+v", decision)
	}
	if decision.MaterialID != createdMaterial.ID {
		t.Fatalf("unexpected price decision material id: %+v", decision)
	}
	if decision.DecisionType != "primary_source_applied" {
		t.Fatalf("unexpected price decision type: %+v", decision)
	}
	if decision.SourceLabel != "Letzter Bestellpreis" || decision.SourceReference != "Bestellung PO-PRIORITY-0001" {
		t.Fatalf("unexpected price decision source metadata: %+v", decision)
	}
	if decision.SourceUnitPrice != 59.9 || decision.AppliedUnitPrice != 59.9 || decision.Currency != "EUR" {
		t.Fatalf("unexpected price decision price snapshot: %+v", decision)
	}
	if !decision.HasSourceDate || !decision.HasDecisionTime {
		t.Fatalf("expected source date and decision time in snapshot, got %+v", decision)
	}

	decisionHistoryReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-decision-history", nil)
	decisionHistoryReq.Header.Set("Authorization", "Bearer "+accessToken)
	decisionHistoryRec := httptest.NewRecorder()
	handler.ServeHTTP(decisionHistoryRec, decisionHistoryReq)
	if decisionHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for price decision history, got %d with body %s", decisionHistoryRec.Code, decisionHistoryRec.Body.String())
	}

	var decisionHistory []struct {
		ID               string  `json:"id"`
		DecisionType     string  `json:"decision_type"`
		MaterialID       string  `json:"material_id"`
		SourceLabel      string  `json:"source_label"`
		SourceUnitPrice  float64 `json:"source_unit_price"`
		AppliedUnitPrice float64 `json:"applied_unit_price"`
		Currency         string  `json:"currency"`
		SourceReference  string  `json:"source_reference"`
		SourceDate       string  `json:"source_date"`
		CreatedAt        string  `json:"created_at"`
	}
	if err := json.Unmarshal(decisionHistoryRec.Body.Bytes(), &decisionHistory); err != nil {
		t.Fatalf("decode price decision history response: %v", err)
	}
	if len(decisionHistory) != 1 {
		t.Fatalf("expected one price decision history entry, got %+v", decisionHistory)
	}
	if decisionHistory[0].ID == "" || decisionHistory[0].CreatedAt == "" {
		t.Fatalf("expected id and created_at in price decision history, got %+v", decisionHistory[0])
	}
	if decisionHistory[0].DecisionType != "primary_source_applied" {
		t.Fatalf("unexpected price decision history type: %+v", decisionHistory[0])
	}
	if decisionHistory[0].MaterialID != createdMaterial.ID {
		t.Fatalf("unexpected price decision history material: %+v", decisionHistory[0])
	}
	if decisionHistory[0].SourceLabel != "Letzter Bestellpreis" || decisionHistory[0].SourceReference != "Bestellung PO-PRIORITY-0001" {
		t.Fatalf("unexpected price decision history source metadata: %+v", decisionHistory[0])
	}
	if decisionHistory[0].SourceUnitPrice != 59.9 || decisionHistory[0].AppliedUnitPrice != 59.9 || decisionHistory[0].Currency != "EUR" {
		t.Fatalf("unexpected price decision history price snapshot: %+v", decisionHistory[0])
	}
	if !strings.Contains(decisionHistory[0].SourceDate, "2026-04-22") {
		t.Fatalf("expected source date in price decision history, got %+v", decisionHistory[0])
	}

	marginAnchorReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/margin-anchor", nil)
	marginAnchorReq.Header.Set("Authorization", "Bearer "+accessToken)
	marginAnchorRec := httptest.NewRecorder()
	handler.ServeHTTP(marginAnchorRec, marginAnchorReq)
	if marginAnchorRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for margin anchor, got %d with body %s", marginAnchorRec.Code, marginAnchorRec.Body.String())
	}

	var marginAnchor struct {
		CurrentUnitPrice   float64  `json:"current_unit_price"`
		CostBasisUnitPrice float64  `json:"cost_basis_unit_price"`
		Currency           string   `json:"currency"`
		AbsoluteMargin     float64  `json:"absolute_margin"`
		MarginPercent      *float64 `json:"margin_percent"`
		MarginStatus       string   `json:"margin_status"`
		DecisionID         string   `json:"decision_id"`
		DecisionType       string   `json:"decision_type"`
		SourceLabel        string   `json:"source_label"`
		SourceReference    string   `json:"source_reference"`
		SourceDate         string   `json:"source_date"`
		DecisionCreatedAt  string   `json:"decision_created_at"`
	}
	if err := json.Unmarshal(marginAnchorRec.Body.Bytes(), &marginAnchor); err != nil {
		t.Fatalf("decode margin anchor response: %v", err)
	}
	if marginAnchor.CurrentUnitPrice != 59.9 || marginAnchor.CostBasisUnitPrice != 59.9 {
		t.Fatalf("unexpected margin anchor prices: %+v", marginAnchor)
	}
	if marginAnchor.AbsoluteMargin != 0 {
		t.Fatalf("expected zero margin after primary source apply, got %+v", marginAnchor)
	}
	if marginAnchor.MarginPercent == nil || *marginAnchor.MarginPercent != 0 {
		t.Fatalf("expected zero margin percent after primary source apply, got %+v", marginAnchor)
	}
	if marginAnchor.MarginStatus != "zero_margin" {
		t.Fatalf("unexpected margin anchor status: %+v", marginAnchor)
	}
	if marginAnchor.DecisionID == "" || marginAnchor.DecisionCreatedAt == "" {
		t.Fatalf("expected decision id and timestamp in margin anchor, got %+v", marginAnchor)
	}
	if marginAnchor.DecisionType != "primary_source_applied" {
		t.Fatalf("unexpected margin anchor decision type: %+v", marginAnchor)
	}
	if marginAnchor.SourceLabel != "Letzter Bestellpreis" || marginAnchor.SourceReference != "Bestellung PO-PRIORITY-0001" {
		t.Fatalf("unexpected margin anchor source metadata: %+v", marginAnchor)
	}
	if marginAnchor.Currency != "EUR" || !strings.Contains(marginAnchor.SourceDate, "2026-04-22") {
		t.Fatalf("unexpected margin anchor currency or source date: %+v", marginAnchor)
	}

	approvalHintReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/approval-hint", nil)
	approvalHintReq.Header.Set("Authorization", "Bearer "+accessToken)
	approvalHintRec := httptest.NewRecorder()
	handler.ServeHTTP(approvalHintRec, approvalHintReq)
	if approvalHintRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval hint, got %d with body %s", approvalHintRec.Code, approvalHintRec.Body.String())
	}

	var approvalHint struct {
		ApprovalStatus     string   `json:"approval_status"`
		ApprovalReason     string   `json:"approval_reason"`
		MarginStatus       string   `json:"margin_status"`
		CurrentUnitPrice   *float64 `json:"current_unit_price"`
		CostBasisUnitPrice *float64 `json:"cost_basis_unit_price"`
		Currency           string   `json:"currency"`
		AbsoluteMargin     *float64 `json:"absolute_margin"`
		MarginPercent      *float64 `json:"margin_percent"`
		DecisionID         string   `json:"decision_id"`
		DecisionType       string   `json:"decision_type"`
		SourceLabel        string   `json:"source_label"`
		DecisionCreatedAt  string   `json:"decision_created_at"`
	}
	if err := json.Unmarshal(approvalHintRec.Body.Bytes(), &approvalHint); err != nil {
		t.Fatalf("decode approval hint response: %v", err)
	}
	if approvalHint.ApprovalStatus != "approval_not_required" {
		t.Fatalf("unexpected approval hint status: %+v", approvalHint)
	}
	if approvalHint.ApprovalReason != "Marge ist nicht negativ; keine Freigabeempfehlung" {
		t.Fatalf("unexpected approval hint reason: %+v", approvalHint)
	}
	if approvalHint.MarginStatus != "zero_margin" {
		t.Fatalf("unexpected approval hint margin status: %+v", approvalHint)
	}
	if approvalHint.CurrentUnitPrice == nil || *approvalHint.CurrentUnitPrice != 59.9 {
		t.Fatalf("expected current unit price in approval hint, got %+v", approvalHint)
	}
	if approvalHint.CostBasisUnitPrice == nil || *approvalHint.CostBasisUnitPrice != 59.9 {
		t.Fatalf("expected cost basis in approval hint, got %+v", approvalHint)
	}
	if approvalHint.AbsoluteMargin == nil || *approvalHint.AbsoluteMargin != 0 {
		t.Fatalf("expected zero absolute margin in approval hint, got %+v", approvalHint)
	}
	if approvalHint.MarginPercent == nil || *approvalHint.MarginPercent != 0 {
		t.Fatalf("expected zero margin percent in approval hint, got %+v", approvalHint)
	}
	if approvalHint.Currency != "EUR" || approvalHint.DecisionID == "" || approvalHint.DecisionCreatedAt == "" {
		t.Fatalf("expected approval hint metadata, got %+v", approvalHint)
	}
	if approvalHint.DecisionType != "primary_source_applied" || approvalHint.SourceLabel != "Letzter Bestellpreis" {
		t.Fatalf("unexpected approval hint decision metadata: %+v", approvalHint)
	}

	targetMarginReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/target-margin-anchor", nil)
	targetMarginReq.Header.Set("Authorization", "Bearer "+accessToken)
	targetMarginRec := httptest.NewRecorder()
	handler.ServeHTTP(targetMarginRec, targetMarginReq)
	if targetMarginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for target margin anchor, got %d with body %s", targetMarginRec.Code, targetMarginRec.Body.String())
	}

	var targetMargin struct {
		TargetStatus            string   `json:"target_status"`
		TargetReason            string   `json:"target_reason"`
		TargetMarginPercent     float64  `json:"target_margin_percent"`
		CurrentUnitPrice        *float64 `json:"current_unit_price"`
		CostBasisUnitPrice      *float64 `json:"cost_basis_unit_price"`
		TargetUnitPrice         *float64 `json:"target_unit_price"`
		Currency                string   `json:"currency"`
		AbsoluteMargin          *float64 `json:"absolute_margin"`
		MarginPercent           *float64 `json:"margin_percent"`
		TargetDifference        *float64 `json:"target_difference"`
		TargetDifferencePercent *float64 `json:"target_difference_percent"`
		MarginStatus            string   `json:"margin_status"`
		DecisionID              string   `json:"decision_id"`
		DecisionType            string   `json:"decision_type"`
		SourceLabel             string   `json:"source_label"`
		DecisionCreatedAt       string   `json:"decision_created_at"`
	}
	if err := json.Unmarshal(targetMarginRec.Body.Bytes(), &targetMargin); err != nil {
		t.Fatalf("decode target margin anchor response: %v", err)
	}
	if targetMargin.TargetStatus != "below_target" {
		t.Fatalf("unexpected target margin status: %+v", targetMargin)
	}
	if targetMargin.TargetReason != "Aktueller Preis erreicht den Zielaufschlag noch nicht" {
		t.Fatalf("unexpected target margin reason: %+v", targetMargin)
	}
	if targetMargin.TargetMarginPercent != 20 {
		t.Fatalf("unexpected target margin percent: %+v", targetMargin)
	}
	if targetMargin.CurrentUnitPrice == nil || *targetMargin.CurrentUnitPrice != 59.9 {
		t.Fatalf("expected current unit price in target margin anchor, got %+v", targetMargin)
	}
	if targetMargin.CostBasisUnitPrice == nil || *targetMargin.CostBasisUnitPrice != 59.9 {
		t.Fatalf("expected cost basis in target margin anchor, got %+v", targetMargin)
	}
	if targetMargin.TargetUnitPrice == nil || *targetMargin.TargetUnitPrice < 71.879 || *targetMargin.TargetUnitPrice > 71.881 {
		t.Fatalf("expected target unit price about 71.88, got %+v", targetMargin)
	}
	if targetMargin.TargetDifference == nil || *targetMargin.TargetDifference > -11.97 || *targetMargin.TargetDifference < -11.99 {
		t.Fatalf("expected target difference about -11.98, got %+v", targetMargin)
	}
	if targetMargin.TargetDifferencePercent == nil || *targetMargin.TargetDifferencePercent > -16.66 || *targetMargin.TargetDifferencePercent < -16.67 {
		t.Fatalf("expected target difference percent about -16.67, got %+v", targetMargin)
	}
	if targetMargin.MarginStatus != "zero_margin" {
		t.Fatalf("unexpected target margin source margin status: %+v", targetMargin)
	}
	if targetMargin.Currency != "EUR" || targetMargin.DecisionID == "" || targetMargin.DecisionCreatedAt == "" {
		t.Fatalf("expected target margin metadata, got %+v", targetMargin)
	}
	if targetMargin.DecisionType != "primary_source_applied" || targetMargin.SourceLabel != "Letzter Bestellpreis" {
		t.Fatalf("unexpected target margin decision metadata: %+v", targetMargin)
	}

	approvalRequestReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/approval-requests", bytes.NewReader([]byte(`{
		"comment":"Zielmarge pruefen"
	}`)))
	approvalRequestReq.Header.Set("Authorization", "Bearer "+accessToken)
	approvalRequestReq.Header.Set("Content-Type", "application/json")
	approvalRequestRec := httptest.NewRecorder()
	handler.ServeHTTP(approvalRequestRec, approvalRequestReq)
	if approvalRequestRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval request, got %d with body %s", approvalRequestRec.Code, approvalRequestRec.Body.String())
	}

	var approvalRequest struct {
		ID                          string   `json:"id"`
		QuoteID                     string   `json:"quote_id"`
		QuoteItemID                 string   `json:"quote_item_id"`
		Status                      string   `json:"status"`
		ReasonCode                  string   `json:"reason_code"`
		ReasonText                  string   `json:"reason_text"`
		CurrentUnitPriceSnapshot    float64  `json:"current_unit_price_snapshot"`
		CostBasisUnitPriceSnapshot  float64  `json:"cost_basis_unit_price_snapshot"`
		TargetUnitPriceSnapshot     float64  `json:"target_unit_price_snapshot"`
		TargetMarginPercentSnapshot float64  `json:"target_margin_percent_snapshot"`
		TargetDifferenceSnapshot    float64  `json:"target_difference_snapshot"`
		MarginPercentSnapshot       *float64 `json:"margin_percent_snapshot"`
		PriceDecisionID             string   `json:"price_decision_id"`
		RequestedBy                 string   `json:"requested_by"`
		RequestedAt                 string   `json:"requested_at"`
		CreatedAt                   string   `json:"created_at"`
		UpdatedAt                   string   `json:"updated_at"`
	}
	if err := json.Unmarshal(approvalRequestRec.Body.Bytes(), &approvalRequest); err != nil {
		t.Fatalf("decode approval request response: %v", err)
	}
	if approvalRequest.ID == "" || approvalRequest.QuoteID != createdQuote.ID || approvalRequest.QuoteItemID != createdQuote.Items[0].ID {
		t.Fatalf("unexpected approval request identity: %+v", approvalRequest)
	}
	if approvalRequest.Status != "requested" || approvalRequest.ReasonCode != "below_target_margin" || approvalRequest.ReasonText != "Zielmarge pruefen" {
		t.Fatalf("unexpected approval request state: %+v", approvalRequest)
	}
	if approvalRequest.CurrentUnitPriceSnapshot != 59.9 || approvalRequest.CostBasisUnitPriceSnapshot != 59.9 {
		t.Fatalf("unexpected approval request price snapshots: %+v", approvalRequest)
	}
	if approvalRequest.TargetMarginPercentSnapshot != 20 {
		t.Fatalf("unexpected approval request target margin snapshot: %+v", approvalRequest)
	}
	if approvalRequest.TargetUnitPriceSnapshot < 71.879 || approvalRequest.TargetUnitPriceSnapshot > 71.881 {
		t.Fatalf("expected approval request target unit price about 71.88, got %+v", approvalRequest)
	}
	if approvalRequest.TargetDifferenceSnapshot > -11.97 || approvalRequest.TargetDifferenceSnapshot < -11.99 {
		t.Fatalf("expected approval request target difference about -11.98, got %+v", approvalRequest)
	}
	if approvalRequest.MarginPercentSnapshot == nil || *approvalRequest.MarginPercentSnapshot != 0 {
		t.Fatalf("expected zero approval request margin percent, got %+v", approvalRequest)
	}
	if approvalRequest.PriceDecisionID != targetMargin.DecisionID || approvalRequest.RequestedBy == "" {
		t.Fatalf("unexpected approval request metadata: %+v", approvalRequest)
	}
	if approvalRequest.RequestedAt == "" || approvalRequest.CreatedAt == "" || approvalRequest.UpdatedAt == "" {
		t.Fatalf("expected approval request timestamps, got %+v", approvalRequest)
	}

	duplicateApprovalRequestReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/approval-requests", nil)
	duplicateApprovalRequestReq.Header.Set("Authorization", "Bearer "+accessToken)
	duplicateApprovalRequestRec := httptest.NewRecorder()
	handler.ServeHTTP(duplicateApprovalRequestRec, duplicateApprovalRequestReq)
	if duplicateApprovalRequestRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate approval request, got %d with body %s", duplicateApprovalRequestRec.Code, duplicateApprovalRequestRec.Body.String())
	}

	approvalReloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	approvalReloadReq.Header.Set("Authorization", "Bearer "+accessToken)
	approvalReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(approvalReloadRec, approvalReloadReq)
	if approvalReloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote reload with approval request, got %d with body %s", approvalReloadRec.Code, approvalReloadRec.Body.String())
	}

	var approvalReload struct {
		ID    string `json:"id"`
		Items []struct {
			ID                    string `json:"id"`
			ActiveApprovalRequest *struct {
				ID         string `json:"id"`
				Status     string `json:"status"`
				ReasonCode string `json:"reason_code"`
				ReasonText string `json:"reason_text"`
			} `json:"active_approval_request"`
		} `json:"items"`
	}
	if err := json.Unmarshal(approvalReloadRec.Body.Bytes(), &approvalReload); err != nil {
		t.Fatalf("decode approval reload response: %v", err)
	}
	if len(approvalReload.Items) != 1 {
		t.Fatalf("expected one reloaded quote item, got %+v", approvalReload.Items)
	}
	if approvalReload.Items[0].ActiveApprovalRequest == nil {
		t.Fatalf("expected active approval request on reloaded quote item, got %+v", approvalReload.Items[0])
	}
	if approvalReload.Items[0].ActiveApprovalRequest.ID != approvalRequest.ID ||
		approvalReload.Items[0].ActiveApprovalRequest.Status != "requested" ||
		approvalReload.Items[0].ActiveApprovalRequest.ReasonCode != "below_target_margin" ||
		approvalReload.Items[0].ActiveApprovalRequest.ReasonText != "Zielmarge pruefen" {
		t.Fatalf("unexpected active approval request on reload: %+v", approvalReload.Items[0].ActiveApprovalRequest)
	}

	blockedQuoteUpdateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Preisquellen Position angepasst","qty":1,"unit":"Stk","unit_price":60.00,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	blockedQuoteUpdateReq.Header.Set("Authorization", "Bearer "+accessToken)
	blockedQuoteUpdateReq.Header.Set("Content-Type", "application/json")
	blockedQuoteUpdateRec := httptest.NewRecorder()
	handler.ServeHTTP(blockedQuoteUpdateRec, blockedQuoteUpdateReq)
	if blockedQuoteUpdateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for quote update with active approval request, got %d with body %s", blockedQuoteUpdateRec.Code, blockedQuoteUpdateRec.Body.String())
	}
	if !strings.Contains(blockedQuoteUpdateRec.Body.String(), "Aktive Freigabeanforderungen muessen vor dem Speichern storniert werden") {
		t.Fatalf("expected active approval request validation message, got body %s", blockedQuoteUpdateRec.Body.String())
	}

	blockedUpdateReloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	blockedUpdateReloadReq.Header.Set("Authorization", "Bearer "+accessToken)
	blockedUpdateReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(blockedUpdateReloadRec, blockedUpdateReloadReq)
	if blockedUpdateReloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote reload after blocked approval update, got %d with body %s", blockedUpdateReloadRec.Code, blockedUpdateReloadRec.Body.String())
	}
	var blockedUpdateReload struct {
		Items []struct {
			ID                    string `json:"id"`
			Description           string `json:"description"`
			ActiveApprovalRequest *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"active_approval_request"`
		} `json:"items"`
	}
	if err := json.Unmarshal(blockedUpdateReloadRec.Body.Bytes(), &blockedUpdateReload); err != nil {
		t.Fatalf("decode blocked update reload response: %v", err)
	}
	if len(blockedUpdateReload.Items) != 1 {
		t.Fatalf("expected one quote item after blocked approval update, got %+v", blockedUpdateReload.Items)
	}
	if blockedUpdateReload.Items[0].Description != "Preisquellen Position" {
		t.Fatalf("expected quote item to remain unchanged after blocked approval update, got %+v", blockedUpdateReload.Items[0])
	}
	if blockedUpdateReload.Items[0].ActiveApprovalRequest == nil ||
		blockedUpdateReload.Items[0].ActiveApprovalRequest.ID != approvalRequest.ID ||
		blockedUpdateReload.Items[0].ActiveApprovalRequest.Status != "requested" {
		t.Fatalf("expected active approval request to survive blocked update, got %+v", blockedUpdateReload.Items[0].ActiveApprovalRequest)
	}

	updateTargetMarginReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/quote-calculation", bytes.NewReader([]byte(`{
		"target_margin_percent": 25
	}`)))
	updateTargetMarginReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateTargetMarginReq.Header.Set("Content-Type", "application/json")
	updateTargetMarginRec := httptest.NewRecorder()
	handler.ServeHTTP(updateTargetMarginRec, updateTargetMarginReq)
	if updateTargetMarginRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for target margin setting update, got %d with body %s", updateTargetMarginRec.Code, updateTargetMarginRec.Body.String())
	}

	configuredTargetMarginReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/target-margin-anchor", nil)
	configuredTargetMarginReq.Header.Set("Authorization", "Bearer "+accessToken)
	configuredTargetMarginRec := httptest.NewRecorder()
	handler.ServeHTTP(configuredTargetMarginRec, configuredTargetMarginReq)
	if configuredTargetMarginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for configured target margin anchor, got %d with body %s", configuredTargetMarginRec.Code, configuredTargetMarginRec.Body.String())
	}
	var configuredTargetMargin struct {
		TargetMarginPercent float64  `json:"target_margin_percent"`
		TargetUnitPrice     *float64 `json:"target_unit_price"`
		TargetDifference    *float64 `json:"target_difference"`
	}
	if err := json.Unmarshal(configuredTargetMarginRec.Body.Bytes(), &configuredTargetMargin); err != nil {
		t.Fatalf("decode configured target margin anchor response: %v", err)
	}
	if configuredTargetMargin.TargetMarginPercent != 25 {
		t.Fatalf("expected configured target margin percent 25, got %+v", configuredTargetMargin)
	}
	if configuredTargetMargin.TargetUnitPrice == nil || *configuredTargetMargin.TargetUnitPrice < 74.874 || *configuredTargetMargin.TargetUnitPrice > 74.876 {
		t.Fatalf("expected configured target unit price about 74.875, got %+v", configuredTargetMargin)
	}
	if configuredTargetMargin.TargetDifference == nil || *configuredTargetMargin.TargetDifference > -14.97 || *configuredTargetMargin.TargetDifference < -14.98 {
		t.Fatalf("expected configured target difference about -14.975, got %+v", configuredTargetMargin)
	}

	applyTargetPriceReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/apply-target-price", nil)
	applyTargetPriceReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyTargetPriceRec := httptest.NewRecorder()
	handler.ServeHTTP(applyTargetPriceRec, applyTargetPriceReq)
	if applyTargetPriceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for target price apply, got %d with body %s", applyTargetPriceRec.Code, applyTargetPriceRec.Body.String())
	}

	var targetPriceQuote struct {
		NetAmount   float64 `json:"net_amount"`
		TaxAmount   float64 `json:"tax_amount"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			ID                 string  `json:"id"`
			UnitPrice          float64 `json:"unit_price"`
			PriceMappingStatus string  `json:"price_mapping_status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(applyTargetPriceRec.Body.Bytes(), &targetPriceQuote); err != nil {
		t.Fatalf("decode target price apply response: %v", err)
	}
	if len(targetPriceQuote.Items) != 1 {
		t.Fatalf("expected one target price quote item, got %+v", targetPriceQuote.Items)
	}
	if targetPriceQuote.Items[0].UnitPrice != 74.88 {
		t.Fatalf("expected rounded target unit price 74.88 after apply, got %+v", targetPriceQuote.Items[0])
	}
	if targetPriceQuote.Items[0].PriceMappingStatus != "manual" {
		t.Fatalf("expected manual price mapping status after target price apply, got %+v", targetPriceQuote.Items[0])
	}
	if targetPriceQuote.NetAmount != 74.88 || targetPriceQuote.TaxAmount != 0 || targetPriceQuote.GrossAmount != 74.88 {
		t.Fatalf("expected quote totals to follow target price, got %+v", targetPriceQuote)
	}

	var targetDecision struct {
		Count            int
		MaterialID       string
		DecisionType     string
		SourceLabel      string
		SourceUnitPrice  float64
		AppliedUnitPrice float64
		Currency         string
		SourceReference  string
		HasSourceDate    bool
		HasDecisionTime  bool
	}
	if err := env.PG.QueryRow(context.Background(), `
		SELECT
			COUNT(*) OVER (),
			COALESCE(material_id, ''),
			decision_type,
			source_label,
			source_unit_price,
			applied_unit_price,
			BTRIM(currency),
			source_reference,
			source_date IS NOT NULL,
			created_at IS NOT NULL
		FROM quote_item_price_decisions
		WHERE quote_item_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, createdQuote.Items[0].ID).Scan(
		&targetDecision.Count,
		&targetDecision.MaterialID,
		&targetDecision.DecisionType,
		&targetDecision.SourceLabel,
		&targetDecision.SourceUnitPrice,
		&targetDecision.AppliedUnitPrice,
		&targetDecision.Currency,
		&targetDecision.SourceReference,
		&targetDecision.HasSourceDate,
		&targetDecision.HasDecisionTime,
	); err != nil {
		t.Fatalf("query target price decision snapshot: %v", err)
	}
	if targetDecision.Count != 2 {
		t.Fatalf("expected two price decision snapshots after target price apply, got %+v", targetDecision)
	}
	if targetDecision.MaterialID != createdMaterial.ID {
		t.Fatalf("unexpected target price decision material id: %+v", targetDecision)
	}
	if targetDecision.DecisionType != "target_price_applied" {
		t.Fatalf("unexpected target price decision type: %+v", targetDecision)
	}
	if targetDecision.SourceLabel != "Zielpreis aus Zielmarge" || targetDecision.SourceReference != "target_margin_percent=25.00" {
		t.Fatalf("unexpected target price decision source metadata: %+v", targetDecision)
	}
	if targetDecision.SourceUnitPrice != 59.9 || targetDecision.AppliedUnitPrice != 74.88 || targetDecision.Currency != "EUR" {
		t.Fatalf("unexpected target price decision price snapshot: %+v", targetDecision)
	}
	if targetDecision.HasSourceDate || !targetDecision.HasDecisionTime {
		t.Fatalf("expected no source date and a decision time in target snapshot, got %+v", targetDecision)
	}

	targetAppliedAnchorReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/target-margin-anchor", nil)
	targetAppliedAnchorReq.Header.Set("Authorization", "Bearer "+accessToken)
	targetAppliedAnchorRec := httptest.NewRecorder()
	handler.ServeHTTP(targetAppliedAnchorRec, targetAppliedAnchorReq)
	if targetAppliedAnchorRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for target-applied margin anchor, got %d with body %s", targetAppliedAnchorRec.Code, targetAppliedAnchorRec.Body.String())
	}
	var targetAppliedAnchor struct {
		TargetStatus       string   `json:"target_status"`
		CurrentUnitPrice   *float64 `json:"current_unit_price"`
		CostBasisUnitPrice *float64 `json:"cost_basis_unit_price"`
		TargetUnitPrice    *float64 `json:"target_unit_price"`
		DecisionType       string   `json:"decision_type"`
	}
	if err := json.Unmarshal(targetAppliedAnchorRec.Body.Bytes(), &targetAppliedAnchor); err != nil {
		t.Fatalf("decode target-applied margin anchor response: %v", err)
	}
	if targetAppliedAnchor.TargetStatus != "on_target" && targetAppliedAnchor.TargetStatus != "above_target" {
		t.Fatalf("expected on-target or rounded above-target status after target apply, got %+v", targetAppliedAnchor)
	}
	if targetAppliedAnchor.CurrentUnitPrice == nil || *targetAppliedAnchor.CurrentUnitPrice != 74.88 {
		t.Fatalf("expected current unit price 74.88 after target apply, got %+v", targetAppliedAnchor)
	}
	if targetAppliedAnchor.CostBasisUnitPrice == nil || *targetAppliedAnchor.CostBasisUnitPrice != 59.9 {
		t.Fatalf("expected cost basis to remain 59.9 after target apply, got %+v", targetAppliedAnchor)
	}
	if targetAppliedAnchor.TargetUnitPrice == nil || *targetAppliedAnchor.TargetUnitPrice < 74.874 || *targetAppliedAnchor.TargetUnitPrice > 74.876 {
		t.Fatalf("expected target unit price about 74.875 after target apply, got %+v", targetAppliedAnchor)
	}
	if targetAppliedAnchor.DecisionType != "target_price_applied" {
		t.Fatalf("expected target price decision metadata after apply, got %+v", targetAppliedAnchor)
	}

	if _, err := env.PG.Exec(context.Background(), `
		UPDATE quote_calculation_settings
		SET target_margin_percent = 20.00
		WHERE id = 'default'
	`); err != nil {
		t.Fatalf("reset target margin setting after configured check: %v", err)
	}

	if _, err := env.PG.Exec(context.Background(), `
		UPDATE quote_items
		SET unit_price = 71.88
		WHERE id = $1
	`, createdQuote.Items[0].ID); err != nil {
		t.Fatalf("seed on-target quote item price: %v", err)
	}

	onTargetMarginReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/target-margin-anchor", nil)
	onTargetMarginReq.Header.Set("Authorization", "Bearer "+accessToken)
	onTargetMarginRec := httptest.NewRecorder()
	handler.ServeHTTP(onTargetMarginRec, onTargetMarginReq)
	if onTargetMarginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for on-target margin anchor, got %d with body %s", onTargetMarginRec.Code, onTargetMarginRec.Body.String())
	}
	var onTargetMargin struct {
		TargetStatus     string   `json:"target_status"`
		TargetReason     string   `json:"target_reason"`
		CurrentUnitPrice *float64 `json:"current_unit_price"`
		TargetUnitPrice  *float64 `json:"target_unit_price"`
		TargetDifference *float64 `json:"target_difference"`
	}
	if err := json.Unmarshal(onTargetMarginRec.Body.Bytes(), &onTargetMargin); err != nil {
		t.Fatalf("decode on-target margin anchor response: %v", err)
	}
	if onTargetMargin.TargetStatus != "on_target" {
		t.Fatalf("unexpected on-target margin status: %+v", onTargetMargin)
	}
	if onTargetMargin.TargetReason != "Aktueller Preis erreicht den Zielaufschlag" {
		t.Fatalf("unexpected on-target margin reason: %+v", onTargetMargin)
	}
	if onTargetMargin.CurrentUnitPrice == nil || *onTargetMargin.CurrentUnitPrice != 71.88 {
		t.Fatalf("expected current unit price 71.88 in on-target anchor, got %+v", onTargetMargin)
	}
	if onTargetMargin.TargetUnitPrice == nil || *onTargetMargin.TargetUnitPrice < 71.879 || *onTargetMargin.TargetUnitPrice > 71.881 {
		t.Fatalf("expected target unit price about 71.88 in on-target anchor, got %+v", onTargetMargin)
	}
	if onTargetMargin.TargetDifference == nil || *onTargetMargin.TargetDifference < -0.005 || *onTargetMargin.TargetDifference > 0.005 {
		t.Fatalf("expected near-zero target difference in on-target anchor, got %+v", onTargetMargin)
	}

	if _, err := env.PG.Exec(context.Background(), `
		UPDATE quote_items
		SET unit_price = 59.90
		WHERE id = $1
	`, createdQuote.Items[0].ID); err != nil {
		t.Fatalf("reset quote item price after target margin check: %v", err)
	}

	if _, err := env.PG.Exec(context.Background(), `
		UPDATE quote_items
		SET unit_price = 50.00
		WHERE id = $1
	`, createdQuote.Items[0].ID); err != nil {
		t.Fatalf("seed negative margin quote item price: %v", err)
	}

	negativeApprovalHintReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/approval-hint", nil)
	negativeApprovalHintReq.Header.Set("Authorization", "Bearer "+accessToken)
	negativeApprovalHintRec := httptest.NewRecorder()
	handler.ServeHTTP(negativeApprovalHintRec, negativeApprovalHintReq)
	if negativeApprovalHintRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for negative margin approval hint, got %d with body %s", negativeApprovalHintRec.Code, negativeApprovalHintRec.Body.String())
	}

	var negativeApprovalHint struct {
		ApprovalStatus     string   `json:"approval_status"`
		ApprovalReason     string   `json:"approval_reason"`
		MarginStatus       string   `json:"margin_status"`
		CurrentUnitPrice   *float64 `json:"current_unit_price"`
		CostBasisUnitPrice *float64 `json:"cost_basis_unit_price"`
		AbsoluteMargin     *float64 `json:"absolute_margin"`
		MarginPercent      *float64 `json:"margin_percent"`
	}
	if err := json.Unmarshal(negativeApprovalHintRec.Body.Bytes(), &negativeApprovalHint); err != nil {
		t.Fatalf("decode negative margin approval hint response: %v", err)
	}
	if negativeApprovalHint.ApprovalStatus != "approval_recommended" {
		t.Fatalf("unexpected negative margin approval status: %+v", negativeApprovalHint)
	}
	if negativeApprovalHint.ApprovalReason != "Negative Marge sichtbar; spaetere Freigabe empfohlen" {
		t.Fatalf("unexpected negative margin approval reason: %+v", negativeApprovalHint)
	}
	if negativeApprovalHint.MarginStatus != "negative_margin" {
		t.Fatalf("unexpected negative margin status: %+v", negativeApprovalHint)
	}
	if negativeApprovalHint.CurrentUnitPrice == nil || *negativeApprovalHint.CurrentUnitPrice != 50 {
		t.Fatalf("expected current unit price 50 in negative approval hint, got %+v", negativeApprovalHint)
	}
	if negativeApprovalHint.CostBasisUnitPrice == nil || *negativeApprovalHint.CostBasisUnitPrice != 59.9 {
		t.Fatalf("expected cost basis 59.9 in negative approval hint, got %+v", negativeApprovalHint)
	}
	if negativeApprovalHint.AbsoluteMargin == nil || *negativeApprovalHint.AbsoluteMargin > -9.89 || *negativeApprovalHint.AbsoluteMargin < -9.91 {
		t.Fatalf("expected negative absolute margin about -9.90, got %+v", negativeApprovalHint)
	}
	if negativeApprovalHint.MarginPercent == nil || *negativeApprovalHint.MarginPercent > -16.52 || *negativeApprovalHint.MarginPercent < -16.53 {
		t.Fatalf("expected negative margin percent about -16.53, got %+v", negativeApprovalHint)
	}

	if _, err := env.PG.Exec(context.Background(), `
		UPDATE quote_items
		SET unit_price = 59.90
		WHERE id = $1
	`, createdQuote.Items[0].ID); err != nil {
		t.Fatalf("reset quote item price after negative margin check: %v", err)
	}

	matchingTransparencyReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/price-decision-transparency", nil)
	matchingTransparencyReq.Header.Set("Authorization", "Bearer "+accessToken)
	matchingTransparencyRec := httptest.NewRecorder()
	handler.ServeHTTP(matchingTransparencyRec, matchingTransparencyReq)
	if matchingTransparencyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for matching price decision transparency, got %d with body %s", matchingTransparencyRec.Code, matchingTransparencyRec.Body.String())
	}

	var matchingTransparency struct {
		AbsoluteDelta  float64 `json:"absolute_delta"`
		DecisionStatus string  `json:"decision_status"`
		DecisionReason string  `json:"decision_reason"`
	}
	if err := json.Unmarshal(matchingTransparencyRec.Body.Bytes(), &matchingTransparency); err != nil {
		t.Fatalf("decode matching price decision transparency response: %v", err)
	}
	if matchingTransparency.AbsoluteDelta != 0 {
		t.Fatalf("expected no transparency delta after primary price source apply, got %+v", matchingTransparency)
	}
	if matchingTransparency.DecisionStatus != "matches_primary_source" {
		t.Fatalf("unexpected matching transparency decision status: %+v", matchingTransparency)
	}
	if matchingTransparency.DecisionReason != "Aktueller Preis entspricht der primaeren Preisquelle" {
		t.Fatalf("unexpected matching transparency decision reason: %+v", matchingTransparency)
	}

	cancelApprovalRequestReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/approval-requests/cancel", nil)
	cancelApprovalRequestReq.Header.Set("Authorization", "Bearer "+accessToken)
	cancelApprovalRequestRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelApprovalRequestRec, cancelApprovalRequestReq)
	if cancelApprovalRequestRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval request cancel, got %d with body %s", cancelApprovalRequestRec.Code, cancelApprovalRequestRec.Body.String())
	}
	var cancelledApprovalRequest struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		CancelledBy string `json:"cancelled_by"`
		CancelledAt string `json:"cancelled_at"`
	}
	if err := json.Unmarshal(cancelApprovalRequestRec.Body.Bytes(), &cancelledApprovalRequest); err != nil {
		t.Fatalf("decode cancelled approval request response: %v", err)
	}
	if cancelledApprovalRequest.ID != approvalRequest.ID || cancelledApprovalRequest.Status != "cancelled" {
		t.Fatalf("unexpected cancelled approval request response: %+v", cancelledApprovalRequest)
	}
	if cancelledApprovalRequest.CancelledBy == "" || cancelledApprovalRequest.CancelledAt == "" {
		t.Fatalf("expected cancellation metadata, got %+v", cancelledApprovalRequest)
	}

	cancelledReloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+createdQuote.ID, nil)
	cancelledReloadReq.Header.Set("Authorization", "Bearer "+accessToken)
	cancelledReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelledReloadRec, cancelledReloadReq)
	if cancelledReloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote reload after approval cancel, got %d with body %s", cancelledReloadRec.Code, cancelledReloadRec.Body.String())
	}
	var cancelledReload struct {
		Items []struct {
			ID                    string `json:"id"`
			ActiveApprovalRequest *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"active_approval_request"`
		} `json:"items"`
	}
	if err := json.Unmarshal(cancelledReloadRec.Body.Bytes(), &cancelledReload); err != nil {
		t.Fatalf("decode cancelled approval reload response: %v", err)
	}
	if len(cancelledReload.Items) != 1 {
		t.Fatalf("expected one quote item after approval cancel, got %+v", cancelledReload.Items)
	}
	if cancelledReload.Items[0].ActiveApprovalRequest != nil {
		t.Fatalf("expected no active approval request after cancel, got %+v", cancelledReload.Items[0].ActiveApprovalRequest)
	}

	cancelApprovalRequestAgainReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+createdQuote.ID+"/items/"+createdQuote.Items[0].ID+"/approval-requests/cancel", nil)
	cancelApprovalRequestAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	cancelApprovalRequestAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelApprovalRequestAgainRec, cancelApprovalRequestAgainReq)
	if cancelApprovalRequestAgainRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate approval request cancel, got %d with body %s", cancelApprovalRequestAgainRec.Code, cancelApprovalRequestAgainRec.Body.String())
	}

	unblockedQuoteUpdateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+createdQuote.ID, bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Preisquellen Position nach Storno","qty":1,"unit":"Stk","unit_price":60.00,"tax_code":"","material_id":"`+createdMaterial.ID+`","price_mapping_status":"manual"}]
	}`)))
	unblockedQuoteUpdateReq.Header.Set("Authorization", "Bearer "+accessToken)
	unblockedQuoteUpdateReq.Header.Set("Content-Type", "application/json")
	unblockedQuoteUpdateRec := httptest.NewRecorder()
	handler.ServeHTTP(unblockedQuoteUpdateRec, unblockedQuoteUpdateReq)
	if unblockedQuoteUpdateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote update after approval cancel, got %d with body %s", unblockedQuoteUpdateRec.Code, unblockedQuoteUpdateRec.Body.String())
	}

	createOpenQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+createdProject.ID+`",
		"currency":"EUR",
		"items":[{"description":"Offene Priorisierungsposition","qty":1,"unit":"Stk","unit_price":0,"tax_code":""}]
	}`)))
	createOpenQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOpenQuoteReq.Header.Set("Content-Type", "application/json")
	createOpenQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createOpenQuoteRec, createOpenQuoteReq)
	if createOpenQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for open quote create, got %d with body %s", createOpenQuoteRec.Code, createOpenQuoteRec.Body.String())
	}

	var openQuote struct {
		ID    string `json:"id"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createOpenQuoteRec.Body.Bytes(), &openQuote); err != nil {
		t.Fatalf("decode open quote response: %v", err)
	}
	if len(openQuote.Items) != 1 || openQuote.Items[0].ID == "" {
		t.Fatalf("expected one open quote item with id, got %+v", openQuote.Items)
	}

	missingMaterialReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/price-source-priority", nil)
	missingMaterialReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialRec, missingMaterialReq)
	if missingMaterialRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for price source priority without mapped material, got %d with body %s", missingMaterialRec.Code, missingMaterialRec.Body.String())
	}

	missingMaterialEvaluationReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/price-evaluation", nil)
	missingMaterialEvaluationReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialEvaluationRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialEvaluationRec, missingMaterialEvaluationReq)
	if missingMaterialEvaluationRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for price evaluation without mapped material, got %d with body %s", missingMaterialEvaluationRec.Code, missingMaterialEvaluationRec.Body.String())
	}

	missingMaterialTransparencyReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/price-decision-transparency", nil)
	missingMaterialTransparencyReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialTransparencyRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialTransparencyRec, missingMaterialTransparencyReq)
	if missingMaterialTransparencyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for price decision transparency without mapped material, got %d with body %s", missingMaterialTransparencyRec.Code, missingMaterialTransparencyRec.Body.String())
	}

	missingMaterialApplyPrimaryReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/apply-primary-price-source", nil)
	missingMaterialApplyPrimaryReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingMaterialApplyPrimaryRec := httptest.NewRecorder()
	handler.ServeHTTP(missingMaterialApplyPrimaryRec, missingMaterialApplyPrimaryReq)
	if missingMaterialApplyPrimaryRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for primary price source apply without mapped material, got %d with body %s", missingMaterialApplyPrimaryRec.Code, missingMaterialApplyPrimaryRec.Body.String())
	}

	missingDecisionApplyTargetReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/apply-target-price", nil)
	missingDecisionApplyTargetReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingDecisionApplyTargetRec := httptest.NewRecorder()
	handler.ServeHTTP(missingDecisionApplyTargetRec, missingDecisionApplyTargetReq)
	if missingDecisionApplyTargetRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for target price apply without price decision, got %d with body %s", missingDecisionApplyTargetRec.Code, missingDecisionApplyTargetRec.Body.String())
	}

	var openDecisionCount int
	if err := env.PG.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM quote_item_price_decisions
		WHERE quote_item_id = $1
	`, openQuote.Items[0].ID).Scan(&openDecisionCount); err != nil {
		t.Fatalf("query missing-material price decision snapshots: %v", err)
	}
	if openDecisionCount != 0 {
		t.Fatalf("expected no price decision snapshot without material, got %d", openDecisionCount)
	}

	emptyDecisionHistoryReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/price-decision-history", nil)
	emptyDecisionHistoryReq.Header.Set("Authorization", "Bearer "+accessToken)
	emptyDecisionHistoryRec := httptest.NewRecorder()
	handler.ServeHTTP(emptyDecisionHistoryRec, emptyDecisionHistoryReq)
	if emptyDecisionHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty price decision history, got %d with body %s", emptyDecisionHistoryRec.Code, emptyDecisionHistoryRec.Body.String())
	}
	var emptyDecisionHistory []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(emptyDecisionHistoryRec.Body.Bytes(), &emptyDecisionHistory); err != nil {
		t.Fatalf("decode empty price decision history response: %v", err)
	}
	if len(emptyDecisionHistory) != 0 {
		t.Fatalf("expected empty price decision history without snapshot, got %+v", emptyDecisionHistory)
	}

	missingDecisionMarginReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/margin-anchor", nil)
	missingDecisionMarginReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingDecisionMarginRec := httptest.NewRecorder()
	handler.ServeHTTP(missingDecisionMarginRec, missingDecisionMarginReq)
	if missingDecisionMarginRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for margin anchor without price decision, got %d with body %s", missingDecisionMarginRec.Code, missingDecisionMarginRec.Body.String())
	}

	missingDecisionApprovalReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/approval-hint", nil)
	missingDecisionApprovalReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingDecisionApprovalRec := httptest.NewRecorder()
	handler.ServeHTTP(missingDecisionApprovalRec, missingDecisionApprovalReq)
	if missingDecisionApprovalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval hint without price decision, got %d with body %s", missingDecisionApprovalRec.Code, missingDecisionApprovalRec.Body.String())
	}
	var missingDecisionApproval struct {
		ApprovalStatus   string   `json:"approval_status"`
		ApprovalReason   string   `json:"approval_reason"`
		CurrentUnitPrice *float64 `json:"current_unit_price"`
		AbsoluteMargin   *float64 `json:"absolute_margin"`
	}
	if err := json.Unmarshal(missingDecisionApprovalRec.Body.Bytes(), &missingDecisionApproval); err != nil {
		t.Fatalf("decode missing-decision approval hint response: %v", err)
	}
	if missingDecisionApproval.ApprovalStatus != "approval_blocked_until_margin_available" {
		t.Fatalf("unexpected missing-decision approval status: %+v", missingDecisionApproval)
	}
	if missingDecisionApproval.ApprovalReason != "Keine gespeicherte Preisentscheidung als Kostenbasis vorhanden" {
		t.Fatalf("unexpected missing-decision approval reason: %+v", missingDecisionApproval)
	}
	if missingDecisionApproval.CurrentUnitPrice != nil || missingDecisionApproval.AbsoluteMargin != nil {
		t.Fatalf("expected no margin data without price decision, got %+v", missingDecisionApproval)
	}

	missingDecisionTargetMarginReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/target-margin-anchor", nil)
	missingDecisionTargetMarginReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingDecisionTargetMarginRec := httptest.NewRecorder()
	handler.ServeHTTP(missingDecisionTargetMarginRec, missingDecisionTargetMarginReq)
	if missingDecisionTargetMarginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for target margin anchor without price decision, got %d with body %s", missingDecisionTargetMarginRec.Code, missingDecisionTargetMarginRec.Body.String())
	}
	var missingDecisionTargetMargin struct {
		TargetStatus        string   `json:"target_status"`
		TargetReason        string   `json:"target_reason"`
		TargetMarginPercent float64  `json:"target_margin_percent"`
		CurrentUnitPrice    *float64 `json:"current_unit_price"`
		TargetUnitPrice     *float64 `json:"target_unit_price"`
	}
	if err := json.Unmarshal(missingDecisionTargetMarginRec.Body.Bytes(), &missingDecisionTargetMargin); err != nil {
		t.Fatalf("decode missing-decision target margin anchor response: %v", err)
	}
	if missingDecisionTargetMargin.TargetStatus != "target_blocked_until_margin_available" {
		t.Fatalf("unexpected missing-decision target margin status: %+v", missingDecisionTargetMargin)
	}
	if missingDecisionTargetMargin.TargetReason != "Keine gespeicherte Preisentscheidung als Kostenbasis vorhanden" {
		t.Fatalf("unexpected missing-decision target margin reason: %+v", missingDecisionTargetMargin)
	}
	if missingDecisionTargetMargin.TargetMarginPercent != 20 {
		t.Fatalf("unexpected missing-decision target margin percent: %+v", missingDecisionTargetMargin)
	}
	if missingDecisionTargetMargin.CurrentUnitPrice != nil || missingDecisionTargetMargin.TargetUnitPrice != nil {
		t.Fatalf("expected no target margin price data without price decision, got %+v", missingDecisionTargetMargin)
	}

	missingDecisionApprovalRequestReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+openQuote.ID+"/items/"+openQuote.Items[0].ID+"/approval-requests", nil)
	missingDecisionApprovalRequestReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingDecisionApprovalRequestRec := httptest.NewRecorder()
	handler.ServeHTTP(missingDecisionApprovalRequestRec, missingDecisionApprovalRequestReq)
	if missingDecisionApprovalRequestRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for approval request without price decision, got %d with body %s", missingDecisionApprovalRequestRec.Code, missingDecisionApprovalRequestRec.Body.String())
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

func seedHTTPApprovalDecisionQuote(t *testing.T, env *testutil.IntegrationEnv, contactID, number string, unitPrice, costBasis float64) (uuid.UUID, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	quoteID := uuid.New()
	itemID := uuid.New()
	decisionID := uuid.New()
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO quotes (id, nummer, root_quote_id, revision_no, contact_id, status, quote_date, currency, net_amount, tax_amount, gross_amount, company_id)
		VALUES ($1, $2, $1, 1, $3, 'draft', CURRENT_DATE, 'EUR', $4, 0, $4, 'default')
	`, quoteID, number, contactID, unitPrice); err != nil {
		t.Fatalf("seed approval decision quote: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount)
		VALUES ($1, $2, 1, 'Freigabeposition', 1, 'Stk', $3, $3, 0)
	`, itemID, quoteID, unitPrice); err != nil {
		t.Fatalf("seed approval decision quote item: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO quote_item_price_decisions (
			id,
			quote_id,
			quote_item_id,
			decision_type,
			source_label,
			source_unit_price,
			applied_unit_price,
			currency
		)
		VALUES ($1, $2, $3, 'primary_source_applied', 'HTTP-Test-Kostenbasis', $4, $4, 'EUR')
	`, decisionID, quoteID, itemID, costBasis); err != nil {
		t.Fatalf("seed approval decision price snapshot: %v", err)
	}
	return quoteID, itemID
}

func createHTTPApprovalRequest(t *testing.T, handler http.Handler, token string, quoteID, itemID uuid.UUID, comment string) {
	t.Helper()

	body := []byte(`{}`)
	if strings.TrimSpace(comment) != "" {
		payload, err := json.Marshal(map[string]string{"comment": comment})
		if err != nil {
			t.Fatalf("encode approval request payload: %v", err)
		}
		body = payload
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID.String()+"/items/"+itemID.String()+"/approval-requests", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval request create, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func decideHTTPApprovalRequest(t *testing.T, handler http.Handler, token string, quoteID, itemID uuid.UUID, decision, comment string) {
	t.Helper()

	payload, err := json.Marshal(map[string]string{"comment": comment})
	if err != nil {
		t.Fatalf("encode approval decision payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID.String()+"/items/"+itemID.String()+"/approval-requests/"+decision, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval %s decision, got %d with body %s", decision, rec.Code, rec.Body.String())
	}
}

func resolveHTTPApprovalRework(t *testing.T, handler http.Handler, token string, quoteID, itemID uuid.UUID, comment string) {
	t.Helper()

	payload, err := json.Marshal(map[string]string{"comment": comment})
	if err != nil {
		t.Fatalf("encode approval rework resolve payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID.String()+"/items/"+itemID.String()+"/approval-rework/resolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval rework resolve, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func updateHTTPQuoteStatus(t *testing.T, handler http.Handler, token string, quoteID uuid.UUID, status string) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		t.Fatalf("encode quote status payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID.String()+"/status", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
