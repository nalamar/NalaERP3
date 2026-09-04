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

func createIntegrationMaterialWithMindestbestand(t *testing.T, handler http.Handler, accessToken string, mindestbestand *float64) string {
	t.Helper()
	nonce := uuid.NewString()[:8]
	body := map[string]any{
		"nummer":      "MAT-DEM-" + nonce,
		"bezeichnung": "Bedarfsermittlung-Testmaterial",
		"einheit":     "Stk",
	}
	if mindestbestand != nil {
		body["mindestbestand"] = *mindestbestand
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal material body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for material create, got %d with body %s", rec.Code, rec.Body.String())
	}
	var mat struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mat); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}
	return mat.ID
}

func TestMinStockShortfallsFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "demand-minstock@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "demand-minstock@example.com", "Secret123!")

	minStock := 10.0
	matID := createIntegrationMaterialWithMindestbestand(t, handler, accessToken, &minStock)

	shortfallReq := httptest.NewRequest(http.MethodGet, "/api/v1/purchasing/demand/min-stock-shortfalls", nil)
	shortfallReq.Header.Set("Authorization", "Bearer "+accessToken)
	shortfallRec := httptest.NewRecorder()
	handler.ServeHTTP(shortfallRec, shortfallReq)
	if shortfallRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for shortfall list, got %d with body %s", shortfallRec.Code, shortfallRec.Body.String())
	}
	var shortfalls []struct {
		MaterialID string  `json:"material_id"`
		FehlMenge  float64 `json:"fehlmenge"`
	}
	if err := json.Unmarshal(shortfallRec.Body.Bytes(), &shortfalls); err != nil {
		t.Fatalf("decode shortfall list response: %v", err)
	}
	found := false
	for _, sf := range shortfalls {
		if sf.MaterialID == matID {
			found = true
			if sf.FehlMenge != 10 {
				t.Fatalf("expected fehlmenge=10 for material with zero stock, got %v", sf.FehlMenge)
			}
		}
	}
	if !found {
		t.Fatalf("expected material with unmet mindestbestand to appear in shortfall list, got %+v", shortfalls)
	}

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-DEM-`+matID[:8]+`","name":"Bedarfstestlager"}`)))
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

	createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{"material_id":"`+matID+`","warehouse_id":"`+wh.ID+`","menge":15,"einheit":"Stk","typ":"in"}`)))
	createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMoveReq.Header.Set("Content-Type", "application/json")
	createMoveRec := httptest.NewRecorder()
	handler.ServeHTTP(createMoveRec, createMoveReq)
	if createMoveRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for stock movement create, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
	}

	secondShortfallReq := httptest.NewRequest(http.MethodGet, "/api/v1/purchasing/demand/min-stock-shortfalls", nil)
	secondShortfallReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondShortfallRec := httptest.NewRecorder()
	handler.ServeHTTP(secondShortfallRec, secondShortfallReq)
	if secondShortfallRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for second shortfall list, got %d with body %s", secondShortfallRec.Code, secondShortfallRec.Body.String())
	}
	var afterRestock []struct {
		MaterialID string `json:"material_id"`
	}
	if err := json.Unmarshal(secondShortfallRec.Body.Bytes(), &afterRestock); err != nil {
		t.Fatalf("decode second shortfall list response: %v", err)
	}
	for _, sf := range afterRestock {
		if sf.MaterialID == matID {
			t.Fatalf("expected material to no longer be in shortfall list after restocking above mindestbestand, got %+v", afterRestock)
		}
	}
}

func TestQuoteDemandFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "demand-quote@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "demand-quote@example.com", "Secret123!")

	matID := createIntegrationMaterialWithMindestbestand(t, handler, accessToken, nil)
	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":    "org",
		"rolle":  "customer",
		"status": "active",
		"name":   "Bedarfsermittlung-Testkunde " + matID[:8],
		"email":  "demand-quote-" + matID[:8] + "@example.com",
	})

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(fmt.Sprintf(`{
		"contact_id":%q,
		"currency":"EUR",
		"items":[
			{"description":"Testposition","qty":5,"unit":"Stk","unit_price":100,"tax_code":"DE19","material_id":%q}
		]
	}`, customerID, matID))))
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

	demandBeforeSendReq := httptest.NewRequest(http.MethodGet, "/api/v1/purchasing/demand/quote-demand", nil)
	demandBeforeSendReq.Header.Set("Authorization", "Bearer "+accessToken)
	demandBeforeSendRec := httptest.NewRecorder()
	handler.ServeHTTP(demandBeforeSendRec, demandBeforeSendReq)
	if demandBeforeSendRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote demand list, got %d with body %s", demandBeforeSendRec.Code, demandBeforeSendRec.Body.String())
	}
	var demandBeforeSend []struct {
		MaterialID string `json:"material_id"`
	}
	if err := json.Unmarshal(demandBeforeSendRec.Body.Bytes(), &demandBeforeSend); err != nil {
		t.Fatalf("decode quote demand list response: %v", err)
	}
	for _, d := range demandBeforeSend {
		if d.MaterialID == matID {
			t.Fatalf("expected draft quote to NOT contribute to demand yet, got %+v", demandBeforeSend)
		}
	}

	statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quote.ID+"/status", bytes.NewReader([]byte(`{"status":"sent"}`)))
	statusReq.Header.Set("Authorization", "Bearer "+accessToken)
	statusReq.Header.Set("Content-Type", "application/json")
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote status update, got %d with body %s", statusRec.Code, statusRec.Body.String())
	}

	demandAfterSendReq := httptest.NewRequest(http.MethodGet, "/api/v1/purchasing/demand/quote-demand", nil)
	demandAfterSendReq.Header.Set("Authorization", "Bearer "+accessToken)
	demandAfterSendRec := httptest.NewRecorder()
	handler.ServeHTTP(demandAfterSendRec, demandAfterSendReq)
	if demandAfterSendRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote demand list after send, got %d with body %s", demandAfterSendRec.Code, demandAfterSendRec.Body.String())
	}
	var demandAfterSend []struct {
		MaterialID string  `json:"material_id"`
		DemandQty  float64 `json:"demand_qty"`
	}
	if err := json.Unmarshal(demandAfterSendRec.Body.Bytes(), &demandAfterSend); err != nil {
		t.Fatalf("decode quote demand list response after send: %v", err)
	}
	found := false
	for _, d := range demandAfterSend {
		if d.MaterialID == matID {
			found = true
			if d.DemandQty != 5 {
				t.Fatalf("expected demand_qty=5, got %v", d.DemandQty)
			}
		}
	}
	if !found {
		t.Fatalf("expected sent quote to contribute to demand, got %+v", demandAfterSend)
	}
}
