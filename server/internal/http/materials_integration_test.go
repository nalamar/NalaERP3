package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestMaterialsCreateListAndGetFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-materials@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-materials@example.com", "Secret123!")

	createBody := []byte(`{
		"nummer":"MAT-IT-0001",
		"bezeichnung":"Integrationsprofil",
		"typ":"profil",
		"einheit":"Stk",
		"dichte":2.7,
		"kategorie":"integration",
		"attribute":{"source":"integration-test"}
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()

	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID          string `json:"id"`
		Nummer      string `json:"nummer"`
		Bezeichnung string `json:"bezeichnung"`
		Typ         string `json:"typ"`
		Einheit     string `json:"einheit"`
		Kategorie   string `json:"kategorie"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created material id")
	}
	if created.Nummer != "MAT-IT-0001" {
		t.Fatalf("expected material number MAT-IT-0001, got %q", created.Nummer)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/materials/?q=MAT-IT-0001", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()

	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID     string `json:"id"`
		Nummer string `json:"nummer"`
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
		t.Fatalf("expected created material %q in list response %#v", created.ID, listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/materials/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()

	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID          string `json:"id"`
		Nummer      string `json:"nummer"`
		Bezeichnung string `json:"bezeichnung"`
		Typ         string `json:"typ"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected fetched id %q, got %q", created.ID, fetched.ID)
	}
	if fetched.Typ != "profil" {
		t.Fatalf("expected profil type, got %q", fetched.Typ)
	}
}

func TestMaterialsCreateIsForbiddenForSalesRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-sales@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-sales@example.com", "Secret123!")

	createBody := []byte(`{
		"nummer":"MAT-IT-FORBIDDEN",
		"bezeichnung":"Nicht erlaubt",
		"typ":"profil",
		"einheit":"Stk",
		"dichte":2.7,
		"kategorie":"integration"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode forbidden response: %v", err)
	}
	if body.Error.Code != "forbidden" {
		t.Fatalf("expected forbidden error code, got %q", body.Error.Code)
	}
}

func TestMaterialsCreateReturnsStructuredValidationError(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-materials-validation@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-materials-validation@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"",
		"bezeichnung":"",
		"typ":"profil",
		"einheit":"Stk",
		"dichte":2.7,
		"kategorie":"integration"
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
	if body.Error.Message != "Nummer und Bezeichnung sind erforderlich" {
		t.Fatalf("expected validation message, got %q", body.Error.Message)
	}
}

func TestMaterialsCreateAcceptsActiveMaterialGroupCategory(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-materials-catalog@example.com", "Secret123!", "admin")

	if _, err := env.PG.Exec(t.Context(), `
        INSERT INTO material_groups (code, name, is_active, sort_order)
        VALUES ('catalog-active', 'Katalog Aktiv', TRUE, 10)
        ON CONFLICT (code) DO UPDATE
        SET name = EXCLUDED.name,
            is_active = EXCLUDED.is_active,
            sort_order = EXCLUDED.sort_order,
            updated_at = now()
    `); err != nil {
		t.Fatalf("seed material group: %v", err)
	}

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-materials-catalog@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-IT-CATALOG-0001",
		"bezeichnung":"Katalogmaterial",
		"typ":"profil",
		"einheit":"Stk",
		"dichte":2.7,
		"kategorie":"catalog-active"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestMaterialsCreateRejectsUnknownCategory(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-materials-invalid-category@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-materials-invalid-category@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-IT-INVALID-CAT",
		"bezeichnung":"Unbekannte Kategorie",
		"typ":"profil",
		"einheit":"Stk",
		"dichte":2.7,
		"kategorie":"does-not-exist-yet"
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
	if body.Error.Message != "Ungültige Materialkategorie" {
		t.Fatalf("expected material category validation message, got %q", body.Error.Message)
	}
}

func TestMaterialsCreateAllowsEmptyCategory(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-materials-empty-category@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-materials-empty-category@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{
		"nummer":"MAT-IT-NO-CAT",
		"bezeichnung":"Ohne Kategorie",
		"typ":"profil",
		"einheit":"Stk",
		"dichte":2.7,
		"kategorie":"   "
	}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestMaterialsUpdateAllowsExistingLegacyCategory(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-materials-legacy-category@example.com", "Secret123!", "admin")

	if _, err := env.PG.Exec(t.Context(), `
        INSERT INTO materials (
            id, nummer, bezeichnung, typ, einheit, dichte, kategorie, attributes
        ) VALUES (
            'mat-legacy-category-itest',
            'MAT-LEGACY-0001',
            'Legacy Material',
            'profil',
            'Stk',
            2.7,
            'legacy-existing',
            '{}'::jsonb
        )
        ON CONFLICT (id) DO UPDATE
        SET nummer = EXCLUDED.nummer,
            bezeichnung = EXCLUDED.bezeichnung,
            typ = EXCLUDED.typ,
            einheit = EXCLUDED.einheit,
            dichte = EXCLUDED.dichte,
            kategorie = EXCLUDED.kategorie,
            attributes = EXCLUDED.attributes
    `); err != nil {
		t.Fatalf("seed legacy material: %v", err)
	}

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-materials-legacy-category@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/materials/mat-legacy-category-itest", bytes.NewReader([]byte(`{
		"kategorie":"legacy-existing"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ID        string `json:"id"`
		Kategorie string `json:"kategorie"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if body.Kategorie != "legacy-existing" {
		t.Fatalf("expected legacy category to remain allowed, got %q", body.Kategorie)
	}
}
