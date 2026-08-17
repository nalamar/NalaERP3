package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Nachdem eine Zahlung erfasst wurde, darf die Rechnung nicht mehr
	// storniert werden (siehe Task 0.3.1: Zahlungs-Rückabwicklung ist
	// bewusst nicht Teil dieses Storno-Konzepts).
	stornoAfterPaymentReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+createdInvoice.ID+"/storno", bytes.NewReader([]byte(`{"reason":"Testfall"}`)))
	stornoAfterPaymentReq.Header.Set("Authorization", "Bearer "+accessToken)
	stornoAfterPaymentReq.Header.Set("Content-Type", "application/json")
	stornoAfterPaymentRec := httptest.NewRecorder()
	handler.ServeHTTP(stornoAfterPaymentRec, stornoAfterPaymentReq)
	if stornoAfterPaymentRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for storno after payment, got %d with body %s", stornoAfterPaymentRec.Code, stornoAfterPaymentRec.Body.String())
	}
}

// TestInvoiceOutPaymentGuardsRejectInvalidPayments deckt die drei fachlich
// zentralen Regeln aus PaymentService.apply() ab (Backlog 0.8), die vorher
// weder unit- noch integrationsgetestet waren (sie liegen nach dem
// tx.QueryRow-Laden der Rechnung, sind also ohne echte DB-Transaktion nicht
// unit-testbar): Statusguard ("Rechnung ist nicht gebucht"),
// Währungsabgleich ("Währung stimmt nicht mit Rechnung überein") und
// Überzahlungsschutz ("Zahlung übersteigt offenen Betrag").
func TestInvoiceOutPaymentGuardsRejectInvalidPayments(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-finance-guards@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-finance-guards@example.com", "Secret123!")

	createContactBody := []byte(`{
		"name":"Payment Guard Kunde",
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

	// net 300 + USt 19% (57) = gross 357.
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
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createInvoiceRec.Body.Bytes(), &createdInvoice); err != nil {
		t.Fatalf("decode invoice create response: %v", err)
	}

	applyPayment := func(body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+createdInvoice.ID+"/payments", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	decodeErrorMessage := func(t *testing.T, rec *httptest.ResponseRecorder) string {
		t.Helper()
		var body struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error response: %v", err)
		}
		return body.Error.Message
	}

	// Guard 1: Statusguard - die Rechnung ist noch im Status "draft" (nicht
	// gebucht), eine Zahlung darauf muss abgelehnt werden.
	notBookedRec := applyPayment([]byte(`{"amount":100,"currency":"EUR","method":"bank"}`))
	if notBookedRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for payment on unbooked invoice, got %d with body %s", notBookedRec.Code, notBookedRec.Body.String())
	}
	if msg := decodeErrorMessage(t, notBookedRec); msg != "Rechnung ist nicht gebucht" {
		t.Fatalf("expected 'Rechnung ist nicht gebucht', got %q", msg)
	}

	bookReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+createdInvoice.ID+"/book", nil)
	bookReq.Header.Set("Authorization", "Bearer "+accessToken)
	bookRec := httptest.NewRecorder()
	handler.ServeHTTP(bookRec, bookReq)
	if bookRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice book, got %d with body %s", bookRec.Code, bookRec.Body.String())
	}

	// Guard 2: Waehrungsabgleich - die Rechnung lautet auf EUR, eine Zahlung
	// in einer anderen Waehrung muss abgelehnt werden.
	wrongCurrencyRec := applyPayment([]byte(`{"amount":100,"currency":"USD","method":"bank"}`))
	if wrongCurrencyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for currency mismatch, got %d with body %s", wrongCurrencyRec.Code, wrongCurrencyRec.Body.String())
	}
	if msg := decodeErrorMessage(t, wrongCurrencyRec); msg != "Währung stimmt nicht mit Rechnung überein" {
		t.Fatalf("expected 'Währung stimmt nicht mit Rechnung überein', got %q", msg)
	}

	// Guard 3: Ueberzahlungsschutz - gross_amount ist 357, eine Zahlung
	// darueber muss abgelehnt werden.
	overpaymentRec := applyPayment([]byte(`{"amount":1000,"currency":"EUR","method":"bank"}`))
	if overpaymentRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for overpayment, got %d with body %s", overpaymentRec.Code, overpaymentRec.Body.String())
	}
	if msg := decodeErrorMessage(t, overpaymentRec); msg != "Zahlung übersteigt offenen Betrag" {
		t.Fatalf("expected 'Zahlung übersteigt offenen Betrag', got %q", msg)
	}

	// Regressionscheck: eine gueltige Zahlung innerhalb des offenen Betrags
	// wird trotz der drei vorherigen Ablehnungen weiterhin akzeptiert - die
	// Guards blockieren nur die ungueltigen Faelle, nicht den Normalfall.
	validRec := applyPayment([]byte(`{"amount":100,"currency":"EUR","method":"bank"}`))
	if validRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid payment, got %d with body %s", validRec.Code, validRec.Body.String())
	}
}

func TestInvoiceOutStornoFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-finance-storno@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-finance-storno@example.com", "Secret123!")

	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader([]byte(`{
		"name":"Storno Test Kunde",
		"rolle":"customer",
		"status":"active",
		"typ":"org"
	}`)))
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)
	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for contact create, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}
	var contact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &contact); err != nil {
		t.Fatalf("decode contact create response: %v", err)
	}

	createInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/", bytes.NewReader([]byte(`{
		"contact_id":"`+contact.ID+`",
		"currency":"EUR",
		"items":[{"description":"Montageleistung","qty":2,"unit_price":150,"tax_code":"DE19","account_code":"8000"}]
	}`)))
	createInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	createInvoiceReq.Header.Set("Content-Type", "application/json")
	createInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(createInvoiceRec, createInvoiceReq)
	if createInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice create, got %d with body %s", createInvoiceRec.Code, createInvoiceRec.Body.String())
	}
	var invoice struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createInvoiceRec.Body.Bytes(), &invoice); err != nil {
		t.Fatalf("decode invoice create response: %v", err)
	}

	// Storno eines noch nicht gebuchten (draft) Belegs muss abgelehnt werden.
	stornoDraftReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+invoice.ID+"/storno", bytes.NewReader([]byte(`{"reason":"Testfall"}`)))
	stornoDraftReq.Header.Set("Authorization", "Bearer "+accessToken)
	stornoDraftReq.Header.Set("Content-Type", "application/json")
	stornoDraftRec := httptest.NewRecorder()
	handler.ServeHTTP(stornoDraftRec, stornoDraftReq)
	if stornoDraftRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for storno of draft invoice, got %d with body %s", stornoDraftRec.Code, stornoDraftRec.Body.String())
	}

	bookReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+invoice.ID+"/book", nil)
	bookReq.Header.Set("Authorization", "Bearer "+accessToken)
	bookRec := httptest.NewRecorder()
	handler.ServeHTTP(bookRec, bookReq)
	if bookRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice book, got %d with body %s", bookRec.Code, bookRec.Body.String())
	}

	// Fehlender Stornogrund muss abgelehnt werden.
	stornoNoReasonReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+invoice.ID+"/storno", bytes.NewReader([]byte(`{"reason":""}`)))
	stornoNoReasonReq.Header.Set("Authorization", "Bearer "+accessToken)
	stornoNoReasonReq.Header.Set("Content-Type", "application/json")
	stornoNoReasonRec := httptest.NewRecorder()
	handler.ServeHTTP(stornoNoReasonRec, stornoNoReasonReq)
	if stornoNoReasonRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for storno without reason, got %d with body %s", stornoNoReasonRec.Code, stornoNoReasonRec.Body.String())
	}

	stornoReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+invoice.ID+"/storno", bytes.NewReader([]byte(`{"reason":"Kunde hat storniert"}`)))
	stornoReq.Header.Set("Authorization", "Bearer "+accessToken)
	stornoReq.Header.Set("Content-Type", "application/json")
	stornoRec := httptest.NewRecorder()
	handler.ServeHTTP(stornoRec, stornoReq)
	if stornoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for storno, got %d with body %s", stornoRec.Code, stornoRec.Body.String())
	}
	var stornoed struct {
		Status               string  `json:"status"`
		StornoGrund          string  `json:"storno_grund"`
		StorniertAm          *string `json:"storniert_am"`
		StornoJournalEntryID *string `json:"storno_journal_entry_id"`
	}
	if err := json.Unmarshal(stornoRec.Body.Bytes(), &stornoed); err != nil {
		t.Fatalf("decode storno response: %v", err)
	}
	if stornoed.Status != "storniert" {
		t.Fatalf("expected status storniert, got %q", stornoed.Status)
	}
	if stornoed.StornoGrund != "Kunde hat storniert" {
		t.Fatalf("expected storno reason to persist, got %q", stornoed.StornoGrund)
	}
	if stornoed.StorniertAm == nil || *stornoed.StorniertAm == "" {
		t.Fatal("expected storniert_am to be set")
	}
	if stornoed.StornoJournalEntryID == nil || *stornoed.StornoJournalEntryID == "" {
		t.Fatal("expected storno_journal_entry_id to be set")
	}

	// Ein bereits stornierter Beleg darf nicht erneut storniert werden.
	stornoAgainReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+invoice.ID+"/storno", bytes.NewReader([]byte(`{"reason":"Nochmal"}`)))
	stornoAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	stornoAgainReq.Header.Set("Content-Type", "application/json")
	stornoAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(stornoAgainRec, stornoAgainReq)
	if stornoAgainRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for repeated storno, got %d with body %s", stornoAgainRec.Code, stornoAgainRec.Body.String())
	}

	// Task 0.3.3.1: das generische Aenderungsprotokoll muss "gebucht" und
	// "storniert" als je einen Eintrag enthalten, neueste zuerst.
	auditReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/"+invoice.ID+"/audit-log", nil)
	auditReq.Header.Set("Authorization", "Bearer "+accessToken)
	auditRec := httptest.NewRecorder()
	handler.ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for audit log, got %d with body %s", auditRec.Code, auditRec.Body.String())
	}
	var auditEntries []struct {
		Action      string `json:"action"`
		ActorUserID string `json:"actor_user_id"`
		Note        string `json:"note"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &auditEntries); err != nil {
		t.Fatalf("decode audit log response: %v", err)
	}
	if len(auditEntries) != 2 {
		t.Fatalf("expected 2 audit log entries, got %+v", auditEntries)
	}
	if auditEntries[0].Action != "storniert" || auditEntries[1].Action != "gebucht" {
		t.Fatalf("expected [storniert, gebucht] newest-first, got %+v", auditEntries)
	}
	if auditEntries[0].ActorUserID == "" {
		t.Fatal("expected actor_user_id to be set on storno entry")
	}
	if auditEntries[0].Note != "Kunde hat storniert" {
		t.Fatalf("expected storno note to persist, got %q", auditEntries[0].Note)
	}
}

// TestSalesOrderStatusChangeAndConvertToInvoiceAreAuditLogged belegt Subtask
// 0.3.3.3 (Anbindung sales_orders an das Aenderungsprotokoll). Der Weg ueber
// quotes-Annahme -> convert-to-sales-order -> Status "released" -> Rechnung
// wird bewusst gewaehlt, um zwei bereits bekannte, unabhaengige Vorbefunde
// zu umgehen (Backlog 0.24: DELETE des letzten Postens liefert 500 statt
// 400; Backlog 0.25: direkte quotes/{id}/convert-to-invoice ohne
// vorherigen Status-Uebergang schlaegt fehl) - beide sind in dieser Route
// nicht involviert.
func TestSalesOrderStatusChangeAndConvertToInvoiceAreAuditLogged(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-finance-sales-audit@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-finance-sales-audit@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Sales Audit Kunde GmbH",
		"email":    "sales-audit@example.com",
		"telefon":  "+49 211 444444",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Sales Audit Projekt",
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
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"project_id":"`+project.ID+`",
		"currency":"EUR",
		"items":[{"description":"Wartungsvertrag","qty":1,"unit":"Pauschale","unit_price":650,"tax_code":"DE19"}]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}
	var quote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &quote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}

	acceptReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quote.ID+"/accept", bytes.NewReader([]byte(`{}`)))
	acceptReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote accept, got %d with body %s", acceptRec.Code, acceptRec.Body.String())
	}

	convertToSalesOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quote.ID+"/convert-to-sales-order", nil)
	convertToSalesOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertToSalesOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(convertToSalesOrderRec, convertToSalesOrderReq)
	if convertToSalesOrderRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote conversion to sales order, got %d with body %s", convertToSalesOrderRec.Code, convertToSalesOrderRec.Body.String())
	}
	var salesOrder struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Items  []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(convertToSalesOrderRec.Body.Bytes(), &salesOrder); err != nil {
		t.Fatalf("decode sales order create response: %v", err)
	}
	if salesOrder.Status != "open" {
		t.Fatalf("expected sales order status open, got %q", salesOrder.Status)
	}

	releaseReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+salesOrder.ID+"/status", bytes.NewReader([]byte(`{"status":"released"}`)))
	releaseReq.Header.Set("Authorization", "Bearer "+accessToken)
	releaseReq.Header.Set("Content-Type", "application/json")
	releaseRec := httptest.NewRecorder()
	handler.ServeHTTP(releaseRec, releaseReq)
	if releaseRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order release, got %d with body %s", releaseRec.Code, releaseRec.Body.String())
	}

	convertToInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+salesOrder.ID+"/convert-to-invoice", bytes.NewReader([]byte(`{"revenue_account":"8000"}`)))
	convertToInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertToInvoiceReq.Header.Set("Content-Type", "application/json")
	convertToInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(convertToInvoiceRec, convertToInvoiceReq)
	if convertToInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sales order conversion to invoice, got %d with body %s", convertToInvoiceRec.Code, convertToInvoiceRec.Body.String())
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+salesOrder.ID, nil)
	auditReq.Header.Set("Authorization", "Bearer "+accessToken)
	auditRec := httptest.NewRecorder()
	handler.ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sales order get, got %d with body %s", auditRec.Code, auditRec.Body.String())
	}
	var finalOrder struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &finalOrder); err != nil {
		t.Fatalf("decode sales order get response: %v", err)
	}
	if finalOrder.Status != "invoiced" {
		t.Fatalf("expected final status invoiced, got %q", finalOrder.Status)
	}

	// Es existiert (noch) kein GET .../sales-orders/{id}/audit-log
	// Endpunkt (nur für invoices_out, siehe Subtask 0.3.3.1) - daher wird
	// das Protokoll hier direkt per SQL geprüft.
	rows, err := env.PG.Query(context.Background(), `SELECT action, actor_user_id, before_data, after_data FROM entity_change_log WHERE entity_type='sales_order' AND entity_id=$1 ORDER BY created_at`, salesOrder.ID)
	if err != nil {
		t.Fatalf("query entity_change_log: %v", err)
	}
	defer rows.Close()
	type logRow struct {
		Action      string
		ActorUserID string
		Before      []byte
		After       []byte
	}
	var logRows []logRow
	for rows.Next() {
		var r logRow
		if err := rows.Scan(&r.Action, &r.ActorUserID, &r.Before, &r.After); err != nil {
			t.Fatalf("scan entity_change_log row: %v", err)
		}
		logRows = append(logRows, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate entity_change_log rows: %v", err)
	}
	if len(logRows) != 2 {
		t.Fatalf("expected 2 audit log entries for sales_order, got %+v", logRows)
	}
	if logRows[0].Action != "status_geaendert" || logRows[1].Action != "in_rechnung_ueberfuehrt" {
		t.Fatalf("expected [status_geaendert, in_rechnung_ueberfuehrt], got %+v", logRows)
	}
	for _, r := range logRows {
		if r.ActorUserID == "" {
			t.Fatalf("expected actor_user_id to be set on entry %+v", r)
		}
	}
}

// TestInvoiceOutCreateAcceptsEmptyTaxCodeAndRejectsEmptyAccountCode deckt
// Backlog 0.38 ab: createTx() (accounting/ar.go) übergab TaxCode/AccountCode
// bisher als rohen Go-string direkt an INSERT INTO invoice_out_items - bei
// TaxCode:"" wurde eine leere Zeichenkette statt SQL NULL eingefügt, was
// die Fremdschlüssel-Constraint invoice_out_items_tax_code_fkey verletzte
// (kein tax_codes-Eintrag mit code=”), obwohl calcTotals/taxRate einen
// leeren Code seit Backlog 0.7 bewusst als gültigen "steuerfrei"-Fall
// behandeln. AccountCode ist dagegen NOT NULL (kann nicht auf NULL
// abgebildet werden) - ein leerer AccountCode ist ein echter
// Validierungsfehler und liefert jetzt eine saubere 400-Meldung statt
// eine rohe Postgres-Fehlermeldung.
func TestInvoiceOutCreateAcceptsEmptyTaxCodeAndRejectsEmptyAccountCode(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-invoice-empty-taxcode@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-invoice-empty-taxcode@example.com", "Secret123!")

	contactID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Steuerfreie Position Kunde GmbH",
		"email":    "invoice-empty-taxcode@example.com",
		"telefon":  "+49 211 888888",
		"waehrung": "EUR",
	})

	missingAccountCodeReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/", bytes.NewReader([]byte(`{
		"contact_id":"`+contactID+`",
		"currency":"EUR",
		"items":[{"description":"Durchlaufender Posten","qty":1,"unit_price":25,"tax_code":"","account_code":""}]
	}`)))
	missingAccountCodeReq.Header.Set("Authorization", "Bearer "+accessToken)
	missingAccountCodeReq.Header.Set("Content-Type", "application/json")
	missingAccountCodeRec := httptest.NewRecorder()
	handler.ServeHTTP(missingAccountCodeRec, missingAccountCodeReq)
	if missingAccountCodeRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing account_code, got %d with body %s", missingAccountCodeRec.Code, missingAccountCodeRec.Body.String())
	}
	if !strings.Contains(missingAccountCodeRec.Body.String(), "account_code fehlt") {
		t.Fatalf("expected account_code fehlt validation message, got body %s", missingAccountCodeRec.Body.String())
	}

	createInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/", bytes.NewReader([]byte(`{
		"contact_id":"`+contactID+`",
		"currency":"EUR",
		"items":[
			{"description":"Durchlaufender Posten","qty":1,"unit_price":25,"tax_code":"","account_code":"1400"},
			{"description":"Beratungsleistung","qty":1,"unit_price":100,"tax_code":"DE19","account_code":"8000"}
		]
	}`)))
	createInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	createInvoiceReq.Header.Set("Content-Type", "application/json")
	createInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(createInvoiceRec, createInvoiceReq)
	if createInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice with empty tax_code item, got %d with body %s", createInvoiceRec.Code, createInvoiceRec.Body.String())
	}

	var createdInvoice struct {
		NetAmount   float64 `json:"net_amount"`
		TaxAmount   float64 `json:"tax_amount"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			TaxCode string `json:"tax_code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createInvoiceRec.Body.Bytes(), &createdInvoice); err != nil {
		t.Fatalf("decode invoice create response: %v", err)
	}
	if createdInvoice.NetAmount != 125 {
		t.Fatalf("expected net_amount 125, got %v", createdInvoice.NetAmount)
	}
	if createdInvoice.TaxAmount != 19 {
		t.Fatalf("expected tax_amount 19 (only the DE19 item is taxed), got %v", createdInvoice.TaxAmount)
	}
	if createdInvoice.GrossAmount != 144 {
		t.Fatalf("expected gross_amount 144, got %v", createdInvoice.GrossAmount)
	}
	if len(createdInvoice.Items) != 2 || createdInvoice.Items[0].TaxCode != "" {
		t.Fatalf("expected empty tax_code preserved on the first item, got %+v", createdInvoice.Items)
	}
}

// TestBankStatementsIngestListAndMatchFlow deckt Backlog 0.37 ab:
// BankService (Kontoauszug-Import/-Abgleich) war an keinen HTTP-Handler
// angebunden. Verifiziert die neuen Routen POST/GET /bank-statements und
// POST /bank-statements/{id}/match end-to-end, inklusive der neuen
// bank.read/bank.write-Berechtigungen.
func TestBankStatementsIngestListAndMatchFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-bank-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-bank-procurement@example.com", "Secret123!", "procurement")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-bank-admin@example.com", "Secret123!")
	procurementToken := loginIntegrationUser(t, handler, "integration-bank-procurement@example.com", "Secret123!")

	contactID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Bankabgleich Kunde GmbH",
		"email":    "bank-match@example.com",
		"telefon":  "+49 211 777777",
		"waehrung": "EUR",
	})

	createInvoiceReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/", bytes.NewReader([]byte(`{
		"contact_id":"`+contactID+`",
		"currency":"EUR",
		"items":[{"description":"Bankabgleich Position","qty":1,"unit_price":100,"tax_code":"DE19","account_code":"8000"}]
	}`)))
	createInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	createInvoiceReq.Header.Set("Content-Type", "application/json")
	createInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(createInvoiceRec, createInvoiceReq)
	if createInvoiceRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice create, got %d with body %s", createInvoiceRec.Code, createInvoiceRec.Body.String())
	}
	var createdInvoice struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createInvoiceRec.Body.Bytes(), &createdInvoice); err != nil {
		t.Fatalf("decode invoice create response: %v", err)
	}

	bookReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-out/"+createdInvoice.ID+"/book", nil)
	bookReq.Header.Set("Authorization", "Bearer "+accessToken)
	bookRec := httptest.NewRecorder()
	handler.ServeHTTP(bookRec, bookReq)
	if bookRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice book, got %d with body %s", bookRec.Code, bookRec.Body.String())
	}

	forbiddenIngestReq := httptest.NewRequest(http.MethodPost, "/api/v1/bank-statements/", bytes.NewReader([]byte(`{
		"booking_date":"2026-01-15T00:00:00Z",
		"amount":119,
		"currency":"EUR",
		"counterparty":"Bankabgleich Kunde GmbH",
		"reference":"Rechnung 119 EUR"
	}`)))
	forbiddenIngestReq.Header.Set("Authorization", "Bearer "+procurementToken)
	forbiddenIngestReq.Header.Set("Content-Type", "application/json")
	forbiddenIngestRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenIngestRec, forbiddenIngestReq)
	if forbiddenIngestRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for bank ingest without bank.write, got %d with body %s", forbiddenIngestRec.Code, forbiddenIngestRec.Body.String())
	}

	ingestReq := httptest.NewRequest(http.MethodPost, "/api/v1/bank-statements/", bytes.NewReader([]byte(`{
		"booking_date":"2026-01-15T00:00:00Z",
		"amount":119,
		"currency":"EUR",
		"counterparty":"Bankabgleich Kunde GmbH",
		"reference":"Rechnung 119 EUR"
	}`)))
	ingestReq.Header.Set("Authorization", "Bearer "+accessToken)
	ingestReq.Header.Set("Content-Type", "application/json")
	ingestRec := httptest.NewRecorder()
	handler.ServeHTTP(ingestRec, ingestReq)
	if ingestRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for bank statement ingest, got %d with body %s", ingestRec.Code, ingestRec.Body.String())
	}
	var ingested struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ingestRec.Body.Bytes(), &ingested); err != nil {
		t.Fatalf("decode bank statement ingest response: %v", err)
	}
	if ingested.ID == "" {
		t.Fatal("expected bank statement id")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/bank-statements/", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bank statement list, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode bank statement list response: %v", err)
	}
	found := false
	for _, item := range listed {
		if id, _ := item["id"].(string); id == ingested.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ingested statement %q in list, got %+v", ingested.ID, listed)
	}

	matchReq := httptest.NewRequest(http.MethodPost, "/api/v1/bank-statements/"+ingested.ID+"/match", bytes.NewReader([]byte(`{
		"invoice_id":"`+createdInvoice.ID+`"
	}`)))
	matchReq.Header.Set("Authorization", "Bearer "+accessToken)
	matchReq.Header.Set("Content-Type", "application/json")
	matchRec := httptest.NewRecorder()
	handler.ServeHTTP(matchRec, matchReq)
	if matchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bank statement match, got %d with body %s", matchRec.Code, matchRec.Body.String())
	}
	var matched struct {
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(matchRec.Body.Bytes(), &matched); err != nil {
		t.Fatalf("decode bank statement match response: %v", err)
	}
	if matched.PaymentID == "" {
		t.Fatal("expected payment id from match")
	}

	getInvoiceReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-out/"+createdInvoice.ID, nil)
	getInvoiceReq.Header.Set("Authorization", "Bearer "+accessToken)
	getInvoiceRec := httptest.NewRecorder()
	handler.ServeHTTP(getInvoiceRec, getInvoiceReq)
	if getInvoiceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invoice get after match, got %d with body %s", getInvoiceRec.Code, getInvoiceRec.Body.String())
	}
	var invoiceAfterMatch struct {
		Status     string  `json:"status"`
		PaidAmount float64 `json:"paid_amount"`
	}
	if err := json.Unmarshal(getInvoiceRec.Body.Bytes(), &invoiceAfterMatch); err != nil {
		t.Fatalf("decode invoice after match response: %v", err)
	}
	if invoiceAfterMatch.Status != "paid" {
		t.Fatalf("expected invoice status paid after full match, got %q", invoiceAfterMatch.Status)
	}
	if invoiceAfterMatch.PaidAmount != 119 {
		t.Fatalf("expected paid_amount 119 after match, got %v", invoiceAfterMatch.PaidAmount)
	}

	rematchReq := httptest.NewRequest(http.MethodPost, "/api/v1/bank-statements/"+ingested.ID+"/match", bytes.NewReader([]byte(`{
		"invoice_id":"`+createdInvoice.ID+`"
	}`)))
	rematchReq.Header.Set("Authorization", "Bearer "+accessToken)
	rematchReq.Header.Set("Content-Type", "application/json")
	rematchRec := httptest.NewRecorder()
	handler.ServeHTTP(rematchRec, rematchReq)
	if rematchRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for re-matching an already matched statement, got %d with body %s", rematchRec.Code, rematchRec.Body.String())
	}
}
