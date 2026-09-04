package apihttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func priceListTestMaterialBody(nummer string) map[string]any {
	return map[string]any{
		"nummer":      nummer,
		"bezeichnung": "Preislisten-Testmaterial",
		"typ":         "profil",
		"einheit":     "Stk",
		"dichte":      2.7,
	}
}

func TestPriceListsCreateListAndGetFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelists@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelists@example.com", "Secret123!")

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Schüco Preisliste 2026 Q1",
		"lieferant":"Schüco",
		"currency":"EUR",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Lieferant string `json:"lieferant"`
		Aktiv     bool   `json:"aktiv"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created price list id")
	}
	if !created.Aktiv {
		t.Fatal("expected new price list to be active by default")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/price-lists/?q=Sch%C3%BCco", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, item := range listed {
		if item.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected created price list %q in list response %#v", created.ID, listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/price-lists/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}
}

func TestPriceListsCreateIsForbiddenForSalesRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelists-sales@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelists-sales@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Nicht erlaubt",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestPriceListsCreateRejectsInvalidDateRange(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelists-invalid-dates@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelists-invalid-dates@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Ungueltiger Zeitraum",
		"gueltig_von":"2026-06-01T00:00:00Z",
		"gueltig_bis":"2026-01-01T00:00:00Z"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", body.Error.Code)
	}
	if body.Error.Message != "Gültig-bis darf nicht vor Gültig-von liegen" {
		t.Fatalf("unexpected validation message, got %q", body.Error.Message)
	}
}

func TestPriceListsUpdateAndSoftDelete(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelists-update@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelists-update@example.com", "Secret123!")

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Vor Update",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/price-lists/"+created.ID, bytes.NewReader([]byte(`{"name":"Nach Update"}`)))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Authorization", "Bearer "+accessToken)
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", patchRec.Code, patchRec.Body.String())
	}
	var patched struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if patched.Name != "Nach Update" {
		t.Fatalf("expected updated name, got %q", patched.Name)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/price-lists/"+created.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteRec.Code, deleteRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/price-lists/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}
	var fetched struct {
		Aktiv bool `json:"aktiv"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Aktiv {
		t.Fatal("expected price list to be deactivated after soft delete")
	}
}

func TestPriceListItemsCreateListAndDeleteFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelistitems@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelistitems@example.com", "Secret123!")

	materialID := createIntegrationMaterial(t, handler, accessToken, priceListTestMaterialBody("MAT-PL-ITEM-0001"))

	createListReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Staffel-Testliste",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	createListReq.Header.Set("Content-Type", "application/json")
	createListReq.Header.Set("Authorization", "Bearer "+accessToken)
	createListRec := httptest.NewRecorder()
	handler.ServeHTTP(createListRec, createListReq)
	if createListRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createListRec.Code, createListRec.Body.String())
	}
	var priceList struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createListRec.Body.Bytes(), &priceList); err != nil {
		t.Fatalf("decode price list create response: %v", err)
	}

	itemBody := []byte(fmt.Sprintf(`{"material_id":%q,"min_menge":10,"unit_price":9.5}`, materialID))
	createItemReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/"+priceList.ID+"/items", bytes.NewReader(itemBody))
	createItemReq.Header.Set("Content-Type", "application/json")
	createItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	createItemRec := httptest.NewRecorder()
	handler.ServeHTTP(createItemRec, createItemReq)
	if createItemRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createItemRec.Code, createItemRec.Body.String())
	}
	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createItemRec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item create response: %v", err)
	}

	listItemsReq := httptest.NewRequest(http.MethodGet, "/api/v1/price-lists/"+priceList.ID+"/items", nil)
	listItemsReq.Header.Set("Authorization", "Bearer "+accessToken)
	listItemsRec := httptest.NewRecorder()
	handler.ServeHTTP(listItemsRec, listItemsReq)
	if listItemsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listItemsRec.Code, listItemsRec.Body.String())
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listItemsRec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode item list response: %v", err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("expected exactly the created item in list, got %#v", items)
	}

	deleteItemReq := httptest.NewRequest(http.MethodDelete, "/api/v1/price-lists/"+priceList.ID+"/items/"+item.ID, nil)
	deleteItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteItemRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteItemRec, deleteItemReq)
	if deleteItemRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteItemRec.Code, deleteItemRec.Body.String())
	}

	listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/v1/price-lists/"+priceList.ID+"/items", nil)
	listAfterDeleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAfterDeleteRec := httptest.NewRecorder()
	handler.ServeHTTP(listAfterDeleteRec, listAfterDeleteReq)
	if listAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
	}
	var itemsAfterDelete []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listAfterDeleteRec.Body.Bytes(), &itemsAfterDelete); err != nil {
		t.Fatalf("decode item list after delete: %v", err)
	}
	if len(itemsAfterDelete) != 0 {
		t.Fatalf("expected no items after delete, got %#v", itemsAfterDelete)
	}
}

func TestPriceListItemsCreateRejectsDuplicateTier(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelistitems-dup@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelistitems-dup@example.com", "Secret123!")

	materialID := createIntegrationMaterial(t, handler, accessToken, priceListTestMaterialBody("MAT-PL-ITEM-DUP-0001"))

	createListReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Duplikat-Testliste",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	createListReq.Header.Set("Content-Type", "application/json")
	createListReq.Header.Set("Authorization", "Bearer "+accessToken)
	createListRec := httptest.NewRecorder()
	handler.ServeHTTP(createListRec, createListReq)
	var priceList struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createListRec.Body.Bytes(), &priceList); err != nil {
		t.Fatalf("decode price list create response: %v", err)
	}

	itemBody := []byte(fmt.Sprintf(`{"material_id":%q,"min_menge":5,"unit_price":10}`, materialID))
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/"+priceList.ID+"/items", bytes.NewReader(itemBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first item, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/"+priceList.ID+"/items", bytes.NewReader(itemBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate tier, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", body.Error.Code)
	}
}

func TestPriceListItemsCreateRejectsUnknownMaterial(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-pricelistitems-unknown-mat@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-pricelistitems-unknown-mat@example.com", "Secret123!")

	createListReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Unbekanntes Material",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	createListReq.Header.Set("Content-Type", "application/json")
	createListReq.Header.Set("Authorization", "Bearer "+accessToken)
	createListRec := httptest.NewRecorder()
	handler.ServeHTTP(createListRec, createListReq)
	var priceList struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createListRec.Body.Bytes(), &priceList); err != nil {
		t.Fatalf("decode price list create response: %v", err)
	}

	itemBody := []byte(`{"material_id":"does-not-exist","min_menge":0,"unit_price":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/"+priceList.ID+"/items", bytes.NewReader(itemBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestEffectivePriceForMaterialReturnsHighestApplicableTierAndNotFoundOtherwise(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-effective-price@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-effective-price@example.com", "Secret123!")

	materialID := createIntegrationMaterial(t, handler, accessToken, priceListTestMaterialBody("MAT-EFFECTIVE-PRICE-0001"))

	createListReq := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/", bytes.NewReader([]byte(`{
		"name":"Effektivpreis-Testliste",
		"gueltig_von":"2026-01-01T00:00:00Z"
	}`)))
	createListReq.Header.Set("Content-Type", "application/json")
	createListReq.Header.Set("Authorization", "Bearer "+accessToken)
	createListRec := httptest.NewRecorder()
	handler.ServeHTTP(createListRec, createListReq)
	var priceList struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createListRec.Body.Bytes(), &priceList); err != nil {
		t.Fatalf("decode price list create response: %v", err)
	}

	for _, tier := range []struct {
		minMenge float64
		price    float64
	}{
		{0, 10.0},
		{10, 8.5},
		{100, 6.0},
	} {
		body := []byte(fmt.Sprintf(`{"material_id":%q,"min_menge":%v,"unit_price":%v}`, materialID, tier.minMenge, tier.price))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists/"+priceList.ID+"/items", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for tier %v, got %d with body %s", tier, rec.Code, rec.Body.String())
		}
	}

	// Menge 50 liegt zwischen den Staffeln 10 und 100 -> die 10er-Staffel muss gewinnen.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/materials/"+materialID+"/effective-price?menge=50&datum=2026-06-01", nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}
	var effective struct {
		MinMenge  float64 `json:"min_menge"`
		UnitPrice float64 `json:"unit_price"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &effective); err != nil {
		t.Fatalf("decode effective price response: %v", err)
	}
	if effective.MinMenge != 10 || effective.UnitPrice != 8.5 {
		t.Fatalf("expected the 10-unit tier (8.5) to win at quantity 50, got %#v", effective)
	}

	// Stichtag vor Gueltig-von der Liste -> keine gueltige Preisliste, 404 erwartet.
	beforeReq := httptest.NewRequest(http.MethodGet, "/api/v1/materials/"+materialID+"/effective-price?menge=50&datum=2025-01-01", nil)
	beforeReq.Header.Set("Authorization", "Bearer "+accessToken)
	beforeRec := httptest.NewRecorder()
	handler.ServeHTTP(beforeRec, beforeReq)
	if beforeRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before validity start, got %d with body %s", beforeRec.Code, beforeRec.Body.String())
	}
}
