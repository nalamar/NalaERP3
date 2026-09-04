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

func supplierProfileSeriesTestContactBody(name, rolle string) map[string]any {
	return map[string]any{
		"typ":      "org",
		"rolle":    rolle,
		"name":     name,
		"waehrung": "EUR",
	}
}

func TestSupplierProfileSeriesCreateListAndDeleteFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-supplier-series@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-supplier-series@example.com", "Secret123!")

	supplierID := createIntegrationContact(t, handler, accessToken, supplierProfileSeriesTestContactBody("Schüco Vertrieb GmbH", "supplier"))

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+supplierID+"/profile-series", bytes.NewReader([]byte(`{
		"profilserie":"Schüco AWS 75",
		"notiz":"zertifizierter Partner"
	}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID          string `json:"id"`
		Profilserie string `json:"profilserie"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Profilserie != "Schüco AWS 75" {
		t.Fatalf("expected profilserie, got %q", created.Profilserie)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+supplierID+"/profile-series", nil)
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
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected exactly the created binding in list, got %#v", listed)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/contacts/"+supplierID+"/profile-series/"+created.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteRec.Code, deleteRec.Body.String())
	}

	listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+supplierID+"/profile-series", nil)
	listAfterDeleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAfterDeleteRec := httptest.NewRecorder()
	handler.ServeHTTP(listAfterDeleteRec, listAfterDeleteReq)
	if listAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
	}
	var listedAfterDelete []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listAfterDeleteRec.Body.Bytes(), &listedAfterDelete); err != nil {
		t.Fatalf("decode list-after-delete response: %v", err)
	}
	if len(listedAfterDelete) != 0 {
		t.Fatalf("expected no bindings after delete, got %#v", listedAfterDelete)
	}
}

func TestSupplierProfileSeriesCreateRejectsNonSupplierContact(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-supplier-series-non-supplier@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-supplier-series-non-supplier@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, supplierProfileSeriesTestContactBody("Nur Kunde GmbH", "customer"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+customerID+"/profile-series", bytes.NewReader([]byte(`{"profilserie":"Aluprof MB-70"}`)))
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
}

func TestSupplierProfileSeriesCreateAllowsBothRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-supplier-series-both@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-supplier-series-both@example.com", "Secret123!")

	bothID := createIntegrationContact(t, handler, accessToken, supplierProfileSeriesTestContactBody("Kunde und Lieferant GmbH", "both"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+bothID+"/profile-series", bytes.NewReader([]byte(`{"profilserie":"Wicona Wictec 50"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestSupplierProfileSeriesCreateRejectsDuplicateBinding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-supplier-series-dup@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-supplier-series-dup@example.com", "Secret123!")

	supplierID := createIntegrationContact(t, handler, accessToken, supplierProfileSeriesTestContactBody("Duplikat-Lieferant GmbH", "supplier"))

	body := []byte(`{"profilserie":"Reynaers CS 77"}`)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+supplierID+"/profile-series", bytes.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first binding, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+supplierID+"/profile-series", bytes.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate binding, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}
}

func TestSupplierProfileSeriesReverseLookupReturnsMatchingSuppliersOnly(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-supplier-series-reverse@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-supplier-series-reverse@example.com", "Secret123!")

	supplierA := createIntegrationContact(t, handler, accessToken, supplierProfileSeriesTestContactBody("Lieferant A GmbH", "supplier"))
	supplierB := createIntegrationContact(t, handler, accessToken, supplierProfileSeriesTestContactBody("Lieferant B GmbH", "supplier"))

	for _, tc := range []struct {
		contactID   string
		profilserie string
	}{
		{supplierA, "Internorm HF 410"},
		{supplierB, "Internorm HF 410"},
		{supplierB, "Andere Serie"},
	} {
		body := []byte(fmt.Sprintf(`{"profilserie":%q}`, tc.profilserie))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+tc.contactID+"/profile-series", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for %#v, got %d with body %s", tc, rec.Code, rec.Body.String())
		}
	}

	lookupReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/profile-series/Internorm%20HF%20410/suppliers", nil)
	lookupReq.Header.Set("Authorization", "Bearer "+accessToken)
	lookupRec := httptest.NewRecorder()
	handler.ServeHTTP(lookupRec, lookupReq)
	if lookupRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", lookupRec.Code, lookupRec.Body.String())
	}
	var suppliers []struct {
		ContactID   string `json:"contact_id"`
		Profilserie string `json:"profilserie"`
	}
	if err := json.Unmarshal(lookupRec.Body.Bytes(), &suppliers); err != nil {
		t.Fatalf("decode reverse lookup response: %v", err)
	}
	if len(suppliers) != 2 {
		t.Fatalf("expected exactly 2 suppliers for Internorm HF 410, got %#v", suppliers)
	}
	seen := map[string]bool{}
	for _, s := range suppliers {
		seen[s.ContactID] = true
	}
	if !seen[supplierA] || !seen[supplierB] {
		t.Fatalf("expected both supplierA and supplierB in result, got %#v", suppliers)
	}
}
