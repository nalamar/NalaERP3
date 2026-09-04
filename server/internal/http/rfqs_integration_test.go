package apihttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func createIntegrationSupplier(t *testing.T, handler http.Handler, accessToken, name string) string {
	t.Helper()
	return createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "supplier",
		"name":     name,
		"email":    name + "@example.com",
		"waehrung": "EUR",
	})
}

func createRFQWithSingleItem(t *testing.T, handler http.Handler, accessToken, materialID string, qty float64) (rfqID, itemID string) {
	t.Helper()
	body := fmt.Sprintf(`{
		"positionen":[
			{"material_id":%q,"bezeichnung":"Testposition","menge":%v,"einheit":"Stk"}
		]
	}`, materialID, qty)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for rfq create, got %d with body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		RFQ struct {
			ID string `json:"id"`
		} `json:"rfq"`
		Positionen []struct {
			ID string `json:"id"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode rfq create response: %v", err)
	}
	if len(created.Positionen) != 1 {
		t.Fatalf("expected exactly 1 item, got %+v", created.Positionen)
	}
	return created.RFQ.ID, created.Positionen[0].ID
}

func registerQuote(t *testing.T, handler http.Handler, accessToken, itemID, supplierID string, price float64) int {
	t.Helper()
	body := fmt.Sprintf(`{"lieferant_id":%q,"preis":%v,"waehrung":"EUR"}`, supplierID, price)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/items/"+itemID+"/quotes", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

func TestRFQCreateListGetFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "rfq-crud@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "rfq-crud@example.com", "Secret123!")

	matID := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-RFQ-" + uuid.NewString()[:8],
		"bezeichnung": "RFQ-Testmaterial",
		"einheit":     "Stk",
	})

	rfqID, _ := createRFQWithSingleItem(t, handler, accessToken, matID, 5)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs/", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rfq list, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID     string `json:"id"`
		Nummer string `json:"nummer"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode rfq list response: %v", err)
	}
	found := false
	for _, r := range listed {
		if r.ID == rfqID {
			found = true
			if r.Status != "offen" || r.Nummer == "" {
				t.Fatalf("unexpected rfq in list: %+v", r)
			}
		}
	}
	if !found {
		t.Fatalf("expected list to contain the created rfq, got %+v", listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs/"+rfqID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rfq get, got %d with body %s", getRec.Code, getRec.Body.String())
	}
	var got struct {
		Positionen []struct {
			Qty float64 `json:"menge"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode rfq get response: %v", err)
	}
	if len(got.Positionen) != 1 || got.Positionen[0].Qty != 5 {
		t.Fatalf("unexpected rfq positions: %+v", got.Positionen)
	}
}

func TestRFQRegisterQuoteAndConvertToPurchaseOrderFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "rfq-convert@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "rfq-convert@example.com", "Secret123!")

	matID := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-RFQ-" + uuid.NewString()[:8],
		"bezeichnung": "RFQ-Testmaterial",
		"einheit":     "Stk",
	})
	supplierA := createIntegrationSupplier(t, handler, accessToken, "rfq-supplier-a-"+uuid.NewString()[:8])
	supplierB := createIntegrationSupplier(t, handler, accessToken, "rfq-supplier-b-"+uuid.NewString()[:8])

	rfqID, itemID := createRFQWithSingleItem(t, handler, accessToken, matID, 3)

	if code := registerQuote(t, handler, accessToken, itemID, supplierA, 25); code != http.StatusCreated {
		t.Fatalf("expected 201 for supplier A quote, got %d", code)
	}
	if code := registerQuote(t, handler, accessToken, itemID, supplierB, 19.5); code != http.StatusCreated {
		t.Fatalf("expected 201 for supplier B quote, got %d", code)
	}

	quotesReq := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs/"+rfqID+"/quotes", nil)
	quotesReq.Header.Set("Authorization", "Bearer "+accessToken)
	quotesRec := httptest.NewRecorder()
	handler.ServeHTTP(quotesRec, quotesReq)
	if quotesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quotes list, got %d with body %s", quotesRec.Code, quotesRec.Body.String())
	}
	var quotes []struct {
		SupplierID string  `json:"lieferant_id"`
		UnitPrice  float64 `json:"preis"`
	}
	if err := json.Unmarshal(quotesRec.Body.Bytes(), &quotes); err != nil {
		t.Fatalf("decode quotes list response: %v", err)
	}
	if len(quotes) != 2 {
		t.Fatalf("expected 2 quotes, got %+v", quotes)
	}
	if quotes[0].SupplierID != supplierB || quotes[0].UnitPrice != 19.5 {
		t.Fatalf("expected cheapest quote (supplier B, 19.5) first, got %+v", quotes)
	}

	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/"+rfqID+"/convert-to-purchase-order", bytes.NewReader([]byte(`{"lieferant_id":"`+supplierB+`"}`)))
	convertReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertReq.Header.Set("Content-Type", "application/json")
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for convert to purchase order, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}
	var converted struct {
		Bestellung struct {
			SupplierID string `json:"lieferant_id"`
		} `json:"bestellung"`
		Positionen []struct {
			UnitPrice float64 `json:"preis"`
			Qty       float64 `json:"menge"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(convertRec.Body.Bytes(), &converted); err != nil {
		t.Fatalf("decode convert response: %v", err)
	}
	if converted.Bestellung.SupplierID != supplierB {
		t.Fatalf("expected purchase order for supplier B, got %+v", converted.Bestellung)
	}
	if len(converted.Positionen) != 1 || converted.Positionen[0].UnitPrice != 19.5 || converted.Positionen[0].Qty != 3 {
		t.Fatalf("unexpected purchase order items: %+v", converted.Positionen)
	}

	getRfqReq := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs/"+rfqID, nil)
	getRfqReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRfqRec := httptest.NewRecorder()
	handler.ServeHTTP(getRfqRec, getRfqReq)
	if getRfqRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rfq get after convert, got %d with body %s", getRfqRec.Code, getRfqRec.Body.String())
	}
	var afterConvert struct {
		RFQ struct {
			Status string `json:"status"`
		} `json:"rfq"`
	}
	if err := json.Unmarshal(getRfqRec.Body.Bytes(), &afterConvert); err != nil {
		t.Fatalf("decode rfq get response after convert: %v", err)
	}
	if afterConvert.RFQ.Status != "abgeschlossen" {
		t.Fatalf("expected rfq status=abgeschlossen after convert, got %+v", afterConvert.RFQ)
	}
}

func TestRFQConvertToPurchaseOrderRejectsIncompleteQuotes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "rfq-incomplete@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "rfq-incomplete@example.com", "Secret123!")

	mat1 := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-RFQ-" + uuid.NewString()[:8],
		"bezeichnung": "RFQ-Testmaterial 1",
		"einheit":     "Stk",
	})
	mat2 := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-RFQ-" + uuid.NewString()[:8],
		"bezeichnung": "RFQ-Testmaterial 2",
		"einheit":     "Stk",
	})
	supplier := createIntegrationSupplier(t, handler, accessToken, "rfq-supplier-partial-"+uuid.NewString()[:8])

	body := fmt.Sprintf(`{
		"positionen":[
			{"material_id":%q,"bezeichnung":"Position 1","menge":1,"einheit":"Stk"},
			{"material_id":%q,"bezeichnung":"Position 2","menge":1,"einheit":"Stk"}
		]
	}`, mat1, mat2)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/", bytes.NewReader([]byte(body)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for rfq create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		RFQ struct {
			ID string `json:"id"`
		} `json:"rfq"`
		Positionen []struct {
			ID string `json:"id"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode rfq create response: %v", err)
	}
	if len(created.Positionen) != 2 {
		t.Fatalf("expected 2 positions, got %+v", created.Positionen)
	}

	if code := registerQuote(t, handler, accessToken, created.Positionen[0].ID, supplier, 10); code != http.StatusCreated {
		t.Fatalf("expected 201 for quote on first item, got %d", code)
	}

	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/"+created.RFQ.ID+"/convert-to-purchase-order", bytes.NewReader([]byte(`{"lieferant_id":"`+supplier+`"}`)))
	convertReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertReq.Header.Set("Content-Type", "application/json")
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for conversion with incomplete quotes, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}
}

func TestRFQCancelRejectsFurtherQuotes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "rfq-cancel@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "rfq-cancel@example.com", "Secret123!")

	matID := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-RFQ-" + uuid.NewString()[:8],
		"bezeichnung": "RFQ-Testmaterial",
		"einheit":     "Stk",
	})
	supplier := createIntegrationSupplier(t, handler, accessToken, "rfq-supplier-cancel-"+uuid.NewString()[:8])

	rfqID, itemID := createRFQWithSingleItem(t, handler, accessToken, matID, 1)

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/"+rfqID+"/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer "+accessToken)
	cancelRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cancel, got %d with body %s", cancelRec.Code, cancelRec.Body.String())
	}
	var cancelled struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelled); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if cancelled.Status != "storniert" {
		t.Fatalf("expected status=storniert, got %+v", cancelled)
	}

	if code := registerQuote(t, handler, accessToken, itemID, supplier, 10); code != http.StatusBadRequest {
		t.Fatalf("expected 400 for quote on a cancelled rfq, got %d", code)
	}
}
