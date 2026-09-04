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

// setupInventoryFixture legt Material und Lager an und bucht einen
// Wareneingang (Menge initialQty) - der gemeinsame Ausgangszustand fuer
// alle Inventur-Tests.
func setupInventoryFixture(t *testing.T, handler http.Handler, accessToken string, initialQty float64) (materialID, warehouseID string) {
	t.Helper()

	nonce := uuid.NewString()[:8]

	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-INV-`+nonce+`","bezeichnung":"Inventur-Testmaterial","einheit":"Stk"}`)))
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

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-INV-`+nonce+`","name":"Inventur-Testlager"}`)))
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

	if initialQty != 0 {
		createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{"material_id":"`+mat.ID+`","warehouse_id":"`+wh.ID+`","menge":`+jsonFloat(initialQty)+`,"einheit":"Stk","typ":"in"}`)))
		createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
		createMoveReq.Header.Set("Content-Type", "application/json")
		createMoveRec := httptest.NewRecorder()
		handler.ServeHTTP(createMoveRec, createMoveReq)
		if createMoveRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for stock movement create, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
		}
	}

	return mat.ID, wh.ID
}

func stockForMaterial(t *testing.T, handler http.Handler, accessToken, materialID string) float64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/materials/"+materialID+"/stock", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stock lookup, got %d with body %s", rec.Code, rec.Body.String())
	}
	var rows []struct {
		Menge float64 `json:"menge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode stock response: %v", err)
	}
	var total float64
	for _, r := range rows {
		total += r.Menge
	}
	return total
}

func startInventory(t *testing.T, handler http.Handler, accessToken, warehouseID string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/", bytes.NewReader([]byte(`{"warehouse_id":"`+warehouseID+`"}`)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for inventory start, got %d with body %s", rec.Code, rec.Body.String())
	}
	var inv struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatalf("decode inventory start response: %v", err)
	}
	if inv.Status != "laufend" {
		t.Fatalf("expected status=laufend after start, got %+v", inv)
	}
	return inv.ID
}

func TestInventoryStartAddLineCloseFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "inv-flow@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "inv-flow@example.com", "Secret123!")

	matID, whID := setupInventoryFixture(t, handler, accessToken, 10)
	invID := startInventory(t, handler, accessToken, whID)

	addLineReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/lines", bytes.NewReader([]byte(`{"material_id":"`+matID+`","ist_qty":7}`)))
	addLineReq.Header.Set("Authorization", "Bearer "+accessToken)
	addLineReq.Header.Set("Content-Type", "application/json")
	addLineRec := httptest.NewRecorder()
	handler.ServeHTTP(addLineRec, addLineReq)
	if addLineRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for add line, got %d with body %s", addLineRec.Code, addLineRec.Body.String())
	}
	var line struct {
		SollQty float64 `json:"soll_qty"`
		IstQty  float64 `json:"ist_qty"`
	}
	if err := json.Unmarshal(addLineRec.Body.Bytes(), &line); err != nil {
		t.Fatalf("decode add line response: %v", err)
	}
	if line.SollQty != 10 || line.IstQty != 7 {
		t.Fatalf("expected soll_qty=10 ist_qty=7, got %+v", line)
	}

	listLinesReq := httptest.NewRequest(http.MethodGet, "/api/v1/inventories/"+invID+"/lines", nil)
	listLinesReq.Header.Set("Authorization", "Bearer "+accessToken)
	listLinesRec := httptest.NewRecorder()
	handler.ServeHTTP(listLinesRec, listLinesReq)
	if listLinesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list lines, got %d with body %s", listLinesRec.Code, listLinesRec.Body.String())
	}
	var lines []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listLinesRec.Body.Bytes(), &lines); err != nil {
		t.Fatalf("decode list lines response: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 line, got %+v", lines)
	}

	if got := stockForMaterial(t, handler, accessToken, matID); got != 10 {
		t.Fatalf("expected stock unchanged at 10 before close, got %v", got)
	}

	closeReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/close", nil)
	closeReq.Header.Set("Authorization", "Bearer "+accessToken)
	closeRec := httptest.NewRecorder()
	handler.ServeHTTP(closeRec, closeReq)
	if closeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for close, got %d with body %s", closeRec.Code, closeRec.Body.String())
	}
	var closed struct {
		Status   string  `json:"status"`
		ClosedAt *string `json:"closed_at"`
	}
	if err := json.Unmarshal(closeRec.Body.Bytes(), &closed); err != nil {
		t.Fatalf("decode close response: %v", err)
	}
	if closed.Status != "abgeschlossen" || closed.ClosedAt == nil {
		t.Fatalf("unexpected inventory after close: %+v", closed)
	}

	if got := stockForMaterial(t, handler, accessToken, matID); got != 7 {
		t.Fatalf("expected stock corrected to 7 after close, got %v", got)
	}
}

func TestInventoryCloseSkipsLinesWithNoDifference(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "inv-nodiff@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "inv-nodiff@example.com", "Secret123!")

	matID, whID := setupInventoryFixture(t, handler, accessToken, 5)
	invID := startInventory(t, handler, accessToken, whID)

	addLineReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/lines", bytes.NewReader([]byte(`{"material_id":"`+matID+`","ist_qty":5}`)))
	addLineReq.Header.Set("Authorization", "Bearer "+accessToken)
	addLineReq.Header.Set("Content-Type", "application/json")
	addLineRec := httptest.NewRecorder()
	handler.ServeHTTP(addLineRec, addLineReq)
	if addLineRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for add line, got %d with body %s", addLineRec.Code, addLineRec.Body.String())
	}

	closeReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/close", nil)
	closeReq.Header.Set("Authorization", "Bearer "+accessToken)
	closeRec := httptest.NewRecorder()
	handler.ServeHTTP(closeRec, closeReq)
	if closeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for close, got %d with body %s", closeRec.Code, closeRec.Body.String())
	}

	if got := stockForMaterial(t, handler, accessToken, matID); got != 5 {
		t.Fatalf("expected stock unchanged at 5 (no adjust booking for zero-diff line), got %v", got)
	}
}

func TestInventoryAddLineRejectsAfterClose(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "inv-afterclose@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "inv-afterclose@example.com", "Secret123!")

	matID, whID := setupInventoryFixture(t, handler, accessToken, 3)
	invID := startInventory(t, handler, accessToken, whID)

	closeReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/close", nil)
	closeReq.Header.Set("Authorization", "Bearer "+accessToken)
	closeRec := httptest.NewRecorder()
	handler.ServeHTTP(closeRec, closeReq)
	if closeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for close, got %d with body %s", closeRec.Code, closeRec.Body.String())
	}

	addLineReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/lines", bytes.NewReader([]byte(`{"material_id":"`+matID+`","ist_qty":1}`)))
	addLineReq.Header.Set("Authorization", "Bearer "+accessToken)
	addLineReq.Header.Set("Content-Type", "application/json")
	addLineRec := httptest.NewRecorder()
	handler.ServeHTTP(addLineRec, addLineReq)
	if addLineRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for add line after close, got %d with body %s", addLineRec.Code, addLineRec.Body.String())
	}

	secondCloseReq := httptest.NewRequest(http.MethodPost, "/api/v1/inventories/"+invID+"/close", nil)
	secondCloseReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondCloseRec := httptest.NewRecorder()
	handler.ServeHTTP(secondCloseRec, secondCloseReq)
	if secondCloseRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for re-closing an already closed inventory, got %d with body %s", secondCloseRec.Code, secondCloseRec.Body.String())
	}
}

func TestInventoryGetAndListFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "inv-getlist@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "inv-getlist@example.com", "Secret123!")

	_, whID := setupInventoryFixture(t, handler, accessToken, 1)
	invID := startInventory(t, handler, accessToken, whID)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/inventories/"+invID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get inventory, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/inventories/?warehouse_id="+whID+"&status=laufend", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list inventories, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list inventories response: %v", err)
	}
	found := false
	for _, inv := range listed {
		if inv.ID == invID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected list to contain the started inventory, got %+v", listed)
	}
}
