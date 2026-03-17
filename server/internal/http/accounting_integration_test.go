package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestInvoiceOutFlowWithPDFAndPayments(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-finance@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-finance@example.com", "Secret123!")

	createContactBody := []byte(`{
		"name":"Invoice Test Kunde",
		"rolle":"customer",
		"status":"active",
		"typ":"org"
	}`)
	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createContactBody))
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)
	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for contact create, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}

	var createdContact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &createdContact); err != nil {
		t.Fatalf("decode contact create response: %v", err)
	}
	if createdContact.ID == "" {
		t.Fatal("expected contact id")
	}

	templateBody := []byte(`{
		"header_text":"Rechnungs-Kopf",
		"footer_text":"Rechnungs-Fuss",
		"top_first_mm":35,
		"top_other_mm":22
	}`)
	templateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/pdf/invoice_out", bytes.NewReader(templateBody))
	templateReq.Header.Set("Authorization", "Bearer "+accessToken)
	templateReq.Header.Set("Content-Type", "application/json")
	templateRec := httptest.NewRecorder()
	handler.ServeHTTP(templateRec, templateReq)
	if templateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for invoice template update, got %d with body %s", templateRec.Code, templateRec.Body.String())
	}

	createInvoiceBody := []byte(`{
		"contact_id":"` + createdContact.ID + `",
		"currency":"EUR",
		"items":[
			{
				"description":"Montageleistung",
				"qty":2,
				"unit_price":150,
				"tax_code":"DE19",
				"account_code":"8000"
			}
		]
	}`)
	createInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/", bytes.NewReader(createInvoiceBody))
	createInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	createInvoiceReq.Header.Set("Content-Type", "application/json")
	createInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(createInvoiceRec, createInvoiceReq)
	if createInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice create, got %d with body %s", createInvoiceRec.Code, createInvoiceRec.Body.String())
	}

	var createdInvoice struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(createInvoiceRec.Body.Bytes(), &createdInvoice); err != nil {
		t.Fatalf("decode invoice create response: %v", err)
	}
	if createdInvoice.ID == "" {
		t.Fatal("expected invoice id")
	}
	if createdInvoice.Status != "draft" {
		t.Fatalf("expected draft invoice, got %q", createdInvoice.Status)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/?q="+createdInvoice.ID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice list, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/"+createdInvoice.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice get, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var invoiceDetail struct {
		ID          string `json:"id"`
		ContactID   string `json:"contact_id"`
		ContactName string `json:"contact_name"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &invoiceDetail); err != nil {
		t.Fatalf("decode invoice detail response: %v", err)
	}
	if invoiceDetail.ContactID != createdContact.ID {
		t.Fatalf("expected contact id %q, got %q", createdContact.ID, invoiceDetail.ContactID)
	}
	if invoiceDetail.ContactName != "Invoice Test Kunde" {
		t.Fatalf("expected contact name, got %q", invoiceDetail.ContactName)
	}

	bookReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+createdInvoice.ID+"/book", nil)
	bookReq.Header.Set("Authorization", "Bearer "+accessToken)
	bookRec := httptest.NewRecorder()
	handler.ServeHTTP(bookRec, bookReq)
	if bookRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice book, got %d with body %s", bookRec.Code, bookRec.Body.String())
	}

	var bookedInvoice struct {
		Status string  `json:"status"`
		Number *string `json:"number"`
	}
	if err := json.Unmarshal(bookRec.Body.Bytes(), &bookedInvoice); err != nil {
		t.Fatalf("decode booked invoice response: %v", err)
	}
	if bookedInvoice.Status != "booked" {
		t.Fatalf("expected booked status, got %q", bookedInvoice.Status)
	}
	if bookedInvoice.Number == nil || *bookedInvoice.Number == "" {
		t.Fatal("expected booked invoice number")
	}

	pdfReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/"+createdInvoice.ID+"/pdf", nil)
	pdfReq.Header.Set("Authorization", "Bearer "+accessToken)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)
	if pdfRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice pdf, got %d with body %s", pdfRec.Code, pdfRec.Body.String())
	}
	if ct := pdfRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected pdf content type, got %q", ct)
	}
	if pdfRec.Body.Len() == 0 {
		t.Fatal("expected non-empty pdf response")
	}

	paymentBody := []byte(`{
		"amount":100,
		"currency":"EUR",
		"method":"bank",
		"reference":"Teilzahlung 1"
	}`)
	paymentReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+createdInvoice.ID+"/payments", bytes.NewReader(paymentBody))
	paymentReq.Header.Set("Authorization", "Bearer "+accessToken)
	paymentReq.Header.Set("Content-Type", "application/json")
	paymentRec := httptest.NewRecorder()
	handler.ServeHTTP(paymentRec, paymentReq)
	if paymentRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice payment, got %d with body %s", paymentRec.Code, paymentRec.Body.String())
	}

	listPaymentsReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/"+createdInvoice.ID+"/payments", nil)
	listPaymentsReq.Header.Set("Authorization", "Bearer "+accessToken)
	listPaymentsRec := httptest.NewRecorder()
	handler.ServeHTTP(listPaymentsRec, listPaymentsReq)
	if listPaymentsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for payments list, got %d with body %s", listPaymentsRec.Code, listPaymentsRec.Body.String())
	}

	var payments []map[string]any
	if err := json.Unmarshal(listPaymentsRec.Body.Bytes(), &payments); err != nil {
		t.Fatalf("decode payments list: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected one payment, got %d", len(payments))
	}
}
