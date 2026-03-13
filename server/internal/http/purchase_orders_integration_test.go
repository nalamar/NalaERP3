package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func createIntegrationMaterial(t *testing.T, handler http.Handler, accessToken string, body map[string]any) string {
	t.Helper()

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
