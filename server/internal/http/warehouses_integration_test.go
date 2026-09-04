package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func TestWarehouseLocationStockMovementFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "warehouse-flow@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "warehouse-flow@example.com", "Secret123!")

	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-WH-0001","bezeichnung":"Testmaterial","einheit":"Stk"}`)))
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

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH1","name":"Lager 1"}`)))
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

	listWhReq := httptest.NewRequest(http.MethodGet, "/api/v1/warehouses/", nil)
	listWhReq.Header.Set("Authorization", "Bearer "+accessToken)
	listWhRec := httptest.NewRecorder()
	handler.ServeHTTP(listWhRec, listWhReq)
	if listWhRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for warehouse list, got %d with body %s", listWhRec.Code, listWhRec.Body.String())
	}
	var warehouses []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listWhRec.Body.Bytes(), &warehouses); err != nil {
		t.Fatalf("decode warehouse list response: %v", err)
	}
	found := false
	for _, w := range warehouses {
		if w.ID == wh.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warehouse list to contain the created warehouse, got %+v", warehouses)
	}

	createLocReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/"+wh.ID+"/locations", bytes.NewReader([]byte(`{"code":"L1","name":"Regal 1"}`)))
	createLocReq.Header.Set("Authorization", "Bearer "+accessToken)
	createLocReq.Header.Set("Content-Type", "application/json")
	createLocRec := httptest.NewRecorder()
	handler.ServeHTTP(createLocRec, createLocReq)
	if createLocRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for location create, got %d with body %s", createLocRec.Code, createLocRec.Body.String())
	}

	listLocReq := httptest.NewRequest(http.MethodGet, "/api/v1/warehouses/"+wh.ID+"/locations", nil)
	listLocReq.Header.Set("Authorization", "Bearer "+accessToken)
	listLocRec := httptest.NewRecorder()
	handler.ServeHTTP(listLocRec, listLocReq)
	if listLocRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for location list, got %d with body %s", listLocRec.Code, listLocRec.Body.String())
	}

	createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{"material_id":"`+mat.ID+`","warehouse_id":"`+wh.ID+`","menge":5,"einheit":"Stk","typ":"in"}`)))
	createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMoveReq.Header.Set("Content-Type", "application/json")
	createMoveRec := httptest.NewRecorder()
	handler.ServeHTTP(createMoveRec, createMoveReq)
	if createMoveRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for stock movement create, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
	}

	stockReq := httptest.NewRequest(http.MethodGet, "/api/v1/materials/"+mat.ID+"/stock", nil)
	stockReq.Header.Set("Authorization", "Bearer "+accessToken)
	stockRec := httptest.NewRecorder()
	handler.ServeHTTP(stockRec, stockReq)
	if stockRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stock lookup, got %d with body %s", stockRec.Code, stockRec.Body.String())
	}
	var stock []struct {
		WarehouseID string  `json:"warehouse_id"`
		Menge       float64 `json:"menge"`
	}
	if err := json.Unmarshal(stockRec.Body.Bytes(), &stock); err != nil {
		t.Fatalf("decode stock response: %v", err)
	}
	if len(stock) != 1 || stock[0].WarehouseID != wh.ID || stock[0].Menge != 5 {
		t.Fatalf("unexpected stock response: %+v", stock)
	}
}

func TestStockMovementCreateRejectsUnknownMaterial(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "warehouse-flow-neg@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "warehouse-flow-neg@example.com", "Secret123!")

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-NEG","name":"Lager Neg"}`)))
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

	createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{"material_id":"00000000-0000-0000-0000-000000000000","warehouse_id":"`+wh.ID+`","menge":5,"einheit":"Stk","typ":"in"}`)))
	createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMoveReq.Header.Set("Content-Type", "application/json")
	createMoveRec := httptest.NewRecorder()
	handler.ServeHTTP(createMoveRec, createMoveReq)
	if createMoveRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown material, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
	}
}

// setupPurchaseOrderFixtureForMovementLink legt Material, Lager,
// Lieferant und eine Bestellung mit einer Position an - der gemeinsame
// Ausgangszustand fuer die stock_movements<->purchase_order_item_id-Tests
// aus D.3.3.1 (ADR 0018).
func setupPurchaseOrderFixtureForMovementLink(t *testing.T, handler http.Handler, accessToken string) (materialID, warehouseID, poItemID string) {
	t.Helper()
	nonce := uuid.NewString()[:8]

	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-POI-`+nonce+`","bezeichnung":"PO-Link-Testmaterial","einheit":"Stk"}`)))
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

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-POI-`+nonce+`","name":"PO-Link-Testlager"}`)))
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

	supplierID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":    "org",
		"rolle":  "supplier",
		"status": "active",
		"name":   "PO-Link-Testlieferant " + nonce,
		"email":  "po-link-" + nonce + "@example.com",
	})

	createPoReq := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/", bytes.NewReader([]byte(`{
		"lieferant_id":"`+supplierID+`",
		"waehrung":"EUR",
		"status":"draft",
		"positionen":[
			{"material_id":"`+mat.ID+`","bezeichnung":"Testposition","menge":10,"einheit":"Stk","preis":5.5,"waehrung":"EUR"}
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

	return mat.ID, wh.ID, createdPo.Positionen[0].ID
}

func TestStockMovementCreateAcceptsPurchaseOrderItemLink(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "warehouse-po-link@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "warehouse-po-link@example.com", "Secret123!")

	matID, whID, poItemID := setupPurchaseOrderFixtureForMovementLink(t, handler, accessToken)

	createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"menge":4,
		"einheit":"Stk",
		"typ":"purchase",
		"purchase_order_item_id":"`+poItemID+`"
	}`)))
	createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMoveReq.Header.Set("Content-Type", "application/json")
	createMoveRec := httptest.NewRecorder()
	handler.ServeHTTP(createMoveRec, createMoveReq)
	if createMoveRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for stock movement with purchase_order_item_id, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMoveRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode stock movement create response: %v", err)
	}

	var linkedPoItemID string
	if err := env.PG.QueryRow(context.Background(), `SELECT purchase_order_item_id FROM stock_movements WHERE id=$1`, created.ID).Scan(&linkedPoItemID); err != nil {
		t.Fatalf("query stock_movements.purchase_order_item_id: %v", err)
	}
	if linkedPoItemID != poItemID {
		t.Fatalf("expected purchase_order_item_id=%q persisted, got %q", poItemID, linkedPoItemID)
	}
}

func TestStockMovementCreateRejectsUnknownPurchaseOrderItem(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "warehouse-po-link-neg@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "warehouse-po-link-neg@example.com", "Secret123!")

	matID, whID, _ := setupPurchaseOrderFixtureForMovementLink(t, handler, accessToken)

	createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"menge":4,
		"einheit":"Stk",
		"typ":"purchase",
		"purchase_order_item_id":"00000000-0000-0000-0000-000000000000"
	}`)))
	createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMoveReq.Header.Set("Content-Type", "application/json")
	createMoveRec := httptest.NewRecorder()
	handler.ServeHTTP(createMoveRec, createMoveReq)
	if createMoveRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown purchase_order_item_id, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
	}
}
