package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// setupInvoiceInFixture legt Material, Lager, Lieferant und eine
// Bestellung mit einer Position an - der gemeinsame Ausgangszustand fuer
// alle /invoices-in-Tests (D.3.3.3, ADR 0018).
func setupInvoiceInFixture(t *testing.T, handler http.Handler, accessToken string, poQty, poPrice float64) (materialID, warehouseID, supplierID, poItemID string) {
	t.Helper()
	nonce := uuid.NewString()[:8]

	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-INV-`+nonce+`","bezeichnung":"Eingangsrechnung-Testmaterial","einheit":"Stk"}`)))
	createMatReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMatReq.Header.Set("Content-Type", "application/json")
	createMatRec := httptest.NewRecorder()
	handler.ServeHTTP(createMatRec, createMatReq)
	if createMatRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for material create, got %d with body %s", createMatRec.Code, createMatRec.Body.String())
	}
	var mat struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMatRec.Body.Bytes(), &mat); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-INV-`+nonce+`","name":"Eingangsrechnung-Testlager"}`)))
	createWhReq.Header.Set("Authorization", "Bearer "+accessToken)
	createWhReq.Header.Set("Content-Type", "application/json")
	createWhRec := httptest.NewRecorder()
	handler.ServeHTTP(createWhRec, createWhReq)
	if createWhRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for warehouse create, got %d with body %s", createWhRec.Code, createWhRec.Body.String())
	}
	var wh struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createWhRec.Body.Bytes(), &wh); err != nil {
		t.Fatalf("decode warehouse create response: %v", err)
	}

	supplierID = createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":    "org",
		"rolle":  "supplier",
		"status": "active",
		"name":   "Eingangsrechnung-Testlieferant " + nonce,
		"email":  "invoice-in-" + nonce + "@example.com",
	})

	createPoReq := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/", bytes.NewReader([]byte(`{
		"lieferant_id":"`+supplierID+`",
		"waehrung":"EUR",
		"status":"draft",
		"positionen":[
			{"material_id":"`+mat.ID+`","bezeichnung":"Testposition","menge":`+jsonFloat(poQty)+`,"einheit":"Stk","preis":`+jsonFloat(poPrice)+`,"waehrung":"EUR"}
		]
	}`)))
	createPoReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPoReq.Header.Set("Content-Type", "application/json")
	createPoRec := httptest.NewRecorder()
	handler.ServeHTTP(createPoRec, createPoReq)
	if createPoRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for purchase order create, got %d with body %s", createPoRec.Code, createPoRec.Body.String())
	}
	var createdPo struct {
		Positionen []struct {
			ID string `json:"id"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(createPoRec.Body.Bytes(), &createdPo); err != nil {
		t.Fatalf("decode purchase order create response: %v", err)
	}
	if len(createdPo.Positionen) != 1 {
		t.Fatalf("expected exactly 1 purchase order item, got %+v", createdPo.Positionen)
	}

	return mat.ID, wh.ID, supplierID, createdPo.Positionen[0].ID
}

func receiveGoods(t *testing.T, handler http.Handler, accessToken, materialID, warehouseID, poItemID string, qty float64) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{
		"material_id":"`+materialID+`",
		"warehouse_id":"`+warehouseID+`",
		"menge":`+jsonFloat(qty)+`,
		"einheit":"Stk",
		"typ":"purchase",
		"purchase_order_item_id":"`+poItemID+`"
	}`)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for goods receipt stock movement, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func createInvoiceIn(t *testing.T, handler http.Handler, accessToken, supplierID, poItemID string, qty, unitPrice float64) string {
	t.Helper()
	body := `{"lieferant_id":"` + supplierID + `","rechnungsnummer":"RE-TEST","positionen":[{"bestellposition_id":"` + poItemID + `","bezeichnung":"Testposition","menge":` + jsonFloat(qty) + `,"preis":` + jsonFloat(unitPrice) + `}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-in/", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice-in create, got %d with body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Rechnung struct {
			ID string `json:"id"`
		} `json:"rechnung"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode invoice-in create response: %v", err)
	}
	return created.Rechnung.ID
}

func matchInvoiceIn(t *testing.T, handler http.Handler, accessToken, invoiceInID string) struct {
	Positionen []struct {
		Abgleichbar    bool     `json:"abgleichbar"`
		BestelltMenge  *float64 `json:"bestellt_menge"`
		ErhaltenMenge  *float64 `json:"erhalten_menge"`
		BerechnetMenge float64  `json:"berechnet_menge"`
		BestelltPreis  *float64 `json:"bestellt_preis"`
		BerechnetPreis float64  `json:"berechnet_preis"`
		MengeStimmt    *bool    `json:"menge_stimmt"`
		PreisStimmt    *bool    `json:"preis_stimmt"`
	} `json:"positionen"`
} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-in/"+invoiceInID+"/match", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for match, got %d with body %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Positionen []struct {
			Abgleichbar    bool     `json:"abgleichbar"`
			BestelltMenge  *float64 `json:"bestellt_menge"`
			ErhaltenMenge  *float64 `json:"erhalten_menge"`
			BerechnetMenge float64  `json:"berechnet_menge"`
			BestelltPreis  *float64 `json:"bestellt_preis"`
			BerechnetPreis float64  `json:"berechnet_preis"`
			MengeStimmt    *bool    `json:"menge_stimmt"`
			PreisStimmt    *bool    `json:"preis_stimmt"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode match response: %v", err)
	}
	return result
}

func TestInvoicesInCreateGetListFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "invoices-in-crud@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "invoices-in-crud@example.com", "Secret123!")

	_, _, supplierID, poItemID := setupInvoiceInFixture(t, handler, accessToken, 10, 5.5)
	invoiceID := createInvoiceIn(t, handler, accessToken, supplierID, poItemID, 10, 5.5)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-in/"+invoiceID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices-in/?lieferant_id="+supplierID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, inv := range listed {
		if inv.ID == invoiceID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected list filtered by supplier to contain the created invoice, got %+v", listed)
	}
}

func TestInvoicesInMatchFullReceiptSucceeds(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "invoices-in-fullmatch@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "invoices-in-fullmatch@example.com", "Secret123!")

	matID, whID, supplierID, poItemID := setupInvoiceInFixture(t, handler, accessToken, 10, 5.5)
	receiveGoods(t, handler, accessToken, matID, whID, poItemID, 10)
	invoiceID := createInvoiceIn(t, handler, accessToken, supplierID, poItemID, 10, 5.5)

	result := matchInvoiceIn(t, handler, accessToken, invoiceID)
	if len(result.Positionen) != 1 {
		t.Fatalf("expected 1 match line, got %+v", result.Positionen)
	}
	line := result.Positionen[0]
	if !line.Abgleichbar {
		t.Fatalf("expected line to be matchable, got %+v", line)
	}
	if line.BestelltMenge == nil || *line.BestelltMenge != 10 || line.ErhaltenMenge == nil || *line.ErhaltenMenge != 10 {
		t.Fatalf("unexpected quantities: %+v", line)
	}
	if line.MengeStimmt == nil || !*line.MengeStimmt {
		t.Fatalf("expected menge_stimmt=true, got %+v", line.MengeStimmt)
	}
	if line.PreisStimmt == nil || !*line.PreisStimmt {
		t.Fatalf("expected preis_stimmt=true, got %+v", line.PreisStimmt)
	}
}

func TestInvoicesInMatchDetectsMismatch(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "invoices-in-mismatch@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "invoices-in-mismatch@example.com", "Secret123!")

	matID, whID, supplierID, poItemID := setupInvoiceInFixture(t, handler, accessToken, 10, 5.5)
	// Nur 6 von 10 bestellten Stueck tatsaechlich eingegangen.
	receiveGoods(t, handler, accessToken, matID, whID, poItemID, 6)
	// Rechnung ueber alle 10 bestellten Stueck zu einem abweichenden Preis (6.0 statt 5.5).
	invoiceID := createInvoiceIn(t, handler, accessToken, supplierID, poItemID, 10, 6.0)

	result := matchInvoiceIn(t, handler, accessToken, invoiceID)
	if len(result.Positionen) != 1 {
		t.Fatalf("expected 1 match line, got %+v", result.Positionen)
	}
	line := result.Positionen[0]
	if line.ErhaltenMenge == nil || *line.ErhaltenMenge != 6 {
		t.Fatalf("expected erhalten_menge=6, got %+v", line.ErhaltenMenge)
	}
	if line.MengeStimmt == nil || *line.MengeStimmt {
		t.Fatalf("expected menge_stimmt=false, got %+v", line.MengeStimmt)
	}
	if line.PreisStimmt == nil || *line.PreisStimmt {
		t.Fatalf("expected preis_stimmt=false, got %+v", line.PreisStimmt)
	}
}

func TestInvoicesInMatchMarksUnlinkedItemNotMatchable(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "invoices-in-unlinked@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "invoices-in-unlinked@example.com", "Secret123!")

	_, _, supplierID, _ := setupInvoiceInFixture(t, handler, accessToken, 10, 5.5)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-in/", bytes.NewReader([]byte(`{
		"lieferant_id":"`+supplierID+`",
		"rechnungsnummer":"RE-NEBENKOSTEN",
		"positionen":[{"bezeichnung":"Nebenkosten ohne Bestellbezug","menge":1,"preis":42}]
	}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invoice-in create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Rechnung struct {
			ID string `json:"id"`
		} `json:"rechnung"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode invoice-in create response: %v", err)
	}

	result := matchInvoiceIn(t, handler, accessToken, created.Rechnung.ID)
	if len(result.Positionen) != 1 {
		t.Fatalf("expected 1 match line, got %+v", result.Positionen)
	}
	line := result.Positionen[0]
	if line.Abgleichbar {
		t.Fatalf("expected line without bestellposition_id to be not matchable, got %+v", line)
	}
}

func TestInvoicesInCreateRejectsUnknownSupplier(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "invoices-in-nosupplier@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "invoices-in-nosupplier@example.com", "Secret123!")

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-in/", bytes.NewReader([]byte(`{
		"lieferant_id":"00000000-0000-0000-0000-000000000000",
		"positionen":[{"bezeichnung":"Test","menge":1,"preis":10}]
	}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown supplier, got %d with body %s", createRec.Code, createRec.Body.String())
	}
}
