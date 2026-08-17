package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestPurchaseOrdersCreateAndGetFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-po@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-po@example.com", "Secret123!")

	supplierID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "supplier",
		"name":     "Integration Lieferant GmbH",
		"email":    "supplier@integration.example",
		"telefon":  "+49 111 222",
		"waehrung": "EUR",
	})

	materialID := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-PO-0001",
		"bezeichnung": "PO-Integrationsmaterial",
		"typ":         "profil",
		"einheit":     "Stk",
		"dichte":      2.7,
		"kategorie":   "integration",
	})

	createBody, err := json.Marshal(map[string]any{
		"lieferant_id": supplierID,
		"waehrung":     "EUR",
		"status":       "draft",
		"notiz":        "Integrationstest Bestellung",
		"positionen": []map[string]any{
			{
				"material_id": materialID,
				"bezeichnung": "Profilzuschnitt",
				"menge":       2,
				"einheit":     "Stk",
				"preis":       42.5,
				"waehrung":    "EUR",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal purchase order body: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()

	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		Bestellung struct {
			ID          string `json:"id"`
			LieferantID string `json:"lieferant_id"`
			Nummer      string `json:"nummer"`
			Status      string `json:"status"`
		} `json:"bestellung"`
		Positionen []struct {
			ID         string  `json:"id"`
			MaterialID string  `json:"material_id"`
			Menge      float64 `json:"menge"`
			Preis      float64 `json:"preis"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Bestellung.ID == "" {
		t.Fatal("expected purchase order id")
	}
	if created.Bestellung.LieferantID != supplierID {
		t.Fatalf("expected supplier id %q, got %q", supplierID, created.Bestellung.LieferantID)
	}
	if created.Bestellung.Nummer == "" {
		t.Fatal("expected generated or assigned purchase order number")
	}
	if len(created.Positionen) != 1 {
		t.Fatalf("expected 1 item, got %#v", created.Positionen)
	}
	if created.Positionen[0].MaterialID != materialID {
		t.Fatalf("expected material id %q, got %q", materialID, created.Positionen[0].MaterialID)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/purchase-orders/"+created.Bestellung.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()

	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		Bestellung struct {
			ID          string `json:"id"`
			LieferantID string `json:"lieferant_id"`
			Status      string `json:"status"`
			Notiz       string `json:"notiz"`
		} `json:"bestellung"`
		Positionen []struct {
			ID         string  `json:"id"`
			MaterialID string  `json:"material_id"`
			Menge      float64 `json:"menge"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Bestellung.ID != created.Bestellung.ID {
		t.Fatalf("expected id %q, got %q", created.Bestellung.ID, fetched.Bestellung.ID)
	}
	if fetched.Bestellung.Status != "draft" {
		t.Fatalf("expected draft status, got %q", fetched.Bestellung.Status)
	}
	if fetched.Bestellung.Notiz != "Integrationstest Bestellung" {
		t.Fatalf("expected note to roundtrip, got %q", fetched.Bestellung.Notiz)
	}
	if len(fetched.Positionen) != 1 {
		t.Fatalf("expected 1 fetched item, got %#v", fetched.Positionen)
	}
}

func TestPurchaseOrdersCreateReturnsStructuredValidationError(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-po-validation@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-po-validation@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/", bytes.NewReader([]byte(`{
		"lieferant_id":"",
		"waehrung":"EUR",
		"status":"draft",
		"positionen":[]
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
	if body.Error.Message != "Lieferant erforderlich" {
		t.Fatalf("expected Lieferant erforderlich, got %q", body.Error.Message)
	}
}

func TestPurchaseOrdersCreateIsForbiddenForInventoryRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-po-forbidden@example.com", "Secret123!", "inventory")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-po-forbidden@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/", bytes.NewReader([]byte(`{
		"lieferant_id":"supplier-missing",
		"waehrung":"EUR",
		"status":"draft",
		"positionen":[]
	}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode forbidden response: %v", err)
	}
	if body.Error.Code != "forbidden" {
		t.Fatalf("expected forbidden error code, got %q", body.Error.Code)
	}
}

// TestPurchaseOrdersAreLockedAfterLeavingDraftStatus belegt den in Task
// 0.3.2 ergaenzten Festschreibungs-Mechanismus: sobald eine Bestellung den
// Entwurfsstatus verlaesst (hier: "ordered", entspricht einer an den
// Lieferanten versendeten Bestellung), duerfen weder ihre Kopfdaten noch
// ihre Positionen mehr inhaltlich veraendert werden - der Statuswechsel
// selbst bleibt aber weiterhin moeglich.
func TestPurchaseOrdersAreLockedAfterLeavingDraftStatus(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-po-lock@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-po-lock@example.com", "Secret123!")

	supplierID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "supplier",
		"name":     "Festschreibung Lieferant GmbH",
		"email":    "festschreibung-supplier@integration.example",
		"telefon":  "+49 111 333",
		"waehrung": "EUR",
	})

	// Kategorie bewusst leer, um den unabhaengigen, bereits bekannten
	// Vorbefund Backlog 0.21 (leere material_groups-Tabelle auf frischer
	// DB) nicht zu beruehren - siehe TestPurchaseOrdersCreateAndGetFlow,
	// das dort mit einer festen Kategorie ("integration") bereits scheitert.
	materialID := createIntegrationMaterial(t, handler, accessToken, map[string]any{
		"nummer":      "MAT-PO-LOCK-0001",
		"bezeichnung": "Festschreibung-Integrationsmaterial",
		"typ":         "profil",
		"einheit":     "Stk",
		"dichte":      2.7,
	})

	createBody, err := json.Marshal(map[string]any{
		"lieferant_id": supplierID,
		"waehrung":     "EUR",
		"status":       "draft",
		"notiz":        "Vor dem Versand",
		"positionen": []map[string]any{
			{
				"material_id": materialID,
				"bezeichnung": "Profilzuschnitt",
				"menge":       2,
				"einheit":     "Stk",
				"preis":       42.5,
				"waehrung":    "EUR",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal purchase order body: %v", err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for purchase order create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Bestellung struct {
			ID string `json:"id"`
		} `json:"bestellung"`
		Positionen []struct {
			ID string `json:"id"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	orderID := created.Bestellung.ID
	itemID := created.Positionen[0].ID

	// Waehrend draft: Kopfdaten aendern funktioniert noch.
	preOrderPatchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/purchase-orders/"+orderID, bytes.NewReader([]byte(`{"notiz":"Noch im Entwurf geaendert"}`)))
	preOrderPatchReq.Header.Set("Content-Type", "application/json")
	preOrderPatchReq.Header.Set("Authorization", "Bearer "+accessToken)
	preOrderPatchRec := httptest.NewRecorder()
	handler.ServeHTTP(preOrderPatchRec, preOrderPatchReq)
	if preOrderPatchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for draft note update, got %d with body %s", preOrderPatchRec.Code, preOrderPatchRec.Body.String())
	}

	// Statuswechsel draft -> ordered (entspricht "an Lieferanten versendet").
	statusReq := httptest.NewRequest(http.MethodPatch, "/api/v1/purchase-orders/"+orderID, bytes.NewReader([]byte(`{"status":"ordered"}`)))
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq.Header.Set("Authorization", "Bearer "+accessToken)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for status transition to ordered, got %d with body %s", statusRec.Code, statusRec.Body.String())
	}
	var afterStatus struct {
		Bestellung struct {
			Status string `json:"status"`
		} `json:"bestellung"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &afterStatus); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if afterStatus.Bestellung.Status != "ordered" {
		t.Fatalf("expected status ordered, got %q", afterStatus.Bestellung.Status)
	}

	// Nach dem Versand: Kopfdatenaenderung muss abgelehnt werden.
	patchAfterOrderReq := httptest.NewRequest(http.MethodPatch, "/api/v1/purchase-orders/"+orderID, bytes.NewReader([]byte(`{"notiz":"Nachtraeglich manipuliert"}`)))
	patchAfterOrderReq.Header.Set("Content-Type", "application/json")
	patchAfterOrderReq.Header.Set("Authorization", "Bearer "+accessToken)
	patchAfterOrderRec := httptest.NewRecorder()
	handler.ServeHTTP(patchAfterOrderRec, patchAfterOrderReq)
	if patchAfterOrderRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for content change after ordered, got %d with body %s", patchAfterOrderRec.Code, patchAfterOrderRec.Body.String())
	}

	// Nach dem Versand: neue Position hinzufuegen muss abgelehnt werden.
	addItemReq := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-orders/"+orderID+"/items", bytes.NewReader([]byte(`{
		"material_id":"`+materialID+`","bezeichnung":"Nachtraegliche Position","menge":1,"einheit":"Stk","preis":10,"waehrung":"EUR"
	}`)))
	addItemReq.Header.Set("Content-Type", "application/json")
	addItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	addItemRec := httptest.NewRecorder()
	handler.ServeHTTP(addItemRec, addItemReq)
	if addItemRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for adding item after ordered, got %d with body %s", addItemRec.Code, addItemRec.Body.String())
	}

	// Nach dem Versand: bestehende Position aendern muss abgelehnt werden.
	updateItemReq := httptest.NewRequest(http.MethodPatch, "/api/v1/purchase-orders/"+orderID+"/items/"+itemID, bytes.NewReader([]byte(`{"menge":99}`)))
	updateItemReq.Header.Set("Content-Type", "application/json")
	updateItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateItemRec := httptest.NewRecorder()
	handler.ServeHTTP(updateItemRec, updateItemReq)
	if updateItemRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for item update after ordered, got %d with body %s", updateItemRec.Code, updateItemRec.Body.String())
	}

	// Nach dem Versand: Position loeschen muss abgelehnt werden.
	deleteItemReq := httptest.NewRequest(http.MethodDelete, "/api/v1/purchase-orders/"+orderID+"/items/"+itemID, nil)
	deleteItemReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteItemRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteItemRec, deleteItemReq)
	if deleteItemRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for item delete after ordered, got %d with body %s", deleteItemRec.Code, deleteItemRec.Body.String())
	}

	// Zur Kontrolle: die Position ist tatsaechlich unveraendert erhalten geblieben.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/purchase-orders/"+orderID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get after locked attempts, got %d with body %s", getRec.Code, getRec.Body.String())
	}
	var afterAttempts struct {
		Positionen []struct {
			ID    string  `json:"id"`
			Menge float64 `json:"menge"`
		} `json:"positionen"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &afterAttempts); err != nil {
		t.Fatalf("decode get-after-attempts response: %v", err)
	}
	if len(afterAttempts.Positionen) != 1 || afterAttempts.Positionen[0].Menge != 2 {
		t.Fatalf("expected item to remain unchanged (menge=2), got %+v", afterAttempts.Positionen)
	}
}

func createIntegrationContact(t *testing.T, handler http.Handler, accessToken string, body map[string]any) string {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal contact body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected contact create 201, got %d with body %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode contact create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected contact id")
	}
	return created.ID
}

// ensureIntegrationMaterialGroup legt eine Materialgruppe an (idempotent per
// Upsert), falls sie noch nicht existiert. material_groups wird nur
// rueckwirkend aus bereits vorhandenen materials.kategorie-Werten befuellt
// (039_material_groups.sql) - auf einer wirklich leeren DB ist die Tabelle
// daher leer, und die Materialanlage lehnt jede Kategorie ab, die weder in
// material_groups noch bereits in materials vorkommt (Backlog 0.21). Der
// reale, vorgesehene Weg fuer eine NEUE Kategorie ist, sie zuerst ueber die
// Materialgruppen-Verwaltung (`POST /settings/material-groups/`)
// anzulegen - genau das bildet dieser Helper nach.
func ensureIntegrationMaterialGroup(t *testing.T, handler http.Handler, accessToken, code string) {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"code": code, "name": code, "is_active": true})
	if err != nil {
		t.Fatalf("marshal material group body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/material-groups/", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for material group create, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func createIntegrationMaterial(t *testing.T, handler http.Handler, accessToken string, body map[string]any) string {
	t.Helper()

	if kategorie, ok := body["kategorie"].(string); ok && strings.TrimSpace(kategorie) != "" {
		ensureIntegrationMaterialGroup(t, handler, accessToken, kategorie)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal material body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected material create 201, got %d with body %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected material id")
	}
	return created.ID
}
