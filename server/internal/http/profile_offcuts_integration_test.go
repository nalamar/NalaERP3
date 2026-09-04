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

// setupOffcutFixture legt ein Profil-Material (profilserie gesetzt) und ein
// Lager an - der gemeinsame Ausgangszustand fuer alle Reststueck-Tests.
func setupOffcutFixture(t *testing.T, handler http.Handler, accessToken string) (materialID, warehouseID string) {
	t.Helper()

	nonce := uuid.NewString()[:8]

	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-OFF-`+nonce+`","bezeichnung":"Reststueck-Testprofil","einheit":"Stk","profilserie":"Internorm HF 410"}`)))
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

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-OFF-`+nonce+`","name":"Reststueck-Testlager"}`)))
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

	return mat.ID, wh.ID
}

func registerOffcut(t *testing.T, handler http.Handler, accessToken, materialID, warehouseID string, lengthMM float64) string {
	t.Helper()
	body := fmt.Sprintf(`{"material_id":"%s","warehouse_id":"%s","length_mm":%s}`, materialID, warehouseID, jsonFloat(lengthMM))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for offcut register, got %d with body %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode offcut register response: %v", err)
	}
	return out.ID
}

func TestProfileOffcutRegisterListConsumeFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "offcut-flow@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "offcut-flow@example.com", "Secret123!")

	matID, whID := setupOffcutFixture(t, handler, accessToken)
	offcutID := registerOffcut(t, handler, accessToken, matID, whID, 2000)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/profile-offcuts/?material_id="+matID+"&min_length_mm=1500", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for offcut list, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode offcut list response: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != offcutID {
		t.Fatalf("expected list filtered by min_length_mm to contain exactly the registered offcut, got %+v", listed)
	}

	consumeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/"+offcutID+"/consume", bytes.NewReader([]byte(`{"used_length_mm":1200}`)))
	consumeReq.Header.Set("Authorization", "Bearer "+accessToken)
	consumeReq.Header.Set("Content-Type", "application/json")
	consumeRec := httptest.NewRecorder()
	handler.ServeHTTP(consumeRec, consumeReq)
	if consumeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for offcut consume, got %d with body %s", consumeRec.Code, consumeRec.Body.String())
	}
	var result struct {
		Consumed struct {
			Status       string  `json:"status"`
			UsedLengthMM float64 `json:"used_length_mm"`
		} `json:"consumed"`
		Remainder *struct {
			ID             string  `json:"id"`
			LengthMM       float64 `json:"length_mm"`
			Status         string  `json:"status"`
			SourceOffcutID string  `json:"source_offcut_id"`
		} `json:"remainder"`
	}
	if err := json.Unmarshal(consumeRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode offcut consume response: %v", err)
	}
	if result.Consumed.Status != "verbraucht" || result.Consumed.UsedLengthMM != 1200 {
		t.Fatalf("unexpected consumed offcut: %+v", result.Consumed)
	}
	if result.Remainder == nil {
		t.Fatal("expected a remainder offcut to be created")
	}
	if result.Remainder.LengthMM != 800 || result.Remainder.Status != "verfügbar" || result.Remainder.SourceOffcutID != offcutID {
		t.Fatalf("unexpected remainder offcut: %+v", result.Remainder)
	}

	listAvailableReq := httptest.NewRequest(http.MethodGet, "/api/v1/profile-offcuts/?material_id="+matID+"&status=verfügbar", nil)
	listAvailableReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAvailableRec := httptest.NewRecorder()
	handler.ServeHTTP(listAvailableRec, listAvailableReq)
	if listAvailableRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for available offcut list, got %d with body %s", listAvailableRec.Code, listAvailableRec.Body.String())
	}
	var available []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listAvailableRec.Body.Bytes(), &available); err != nil {
		t.Fatalf("decode available offcut list response: %v", err)
	}
	if len(available) != 1 || available[0].ID != result.Remainder.ID {
		t.Fatalf("expected only the remainder to be verfügbar, got %+v", available)
	}
}

func TestProfileOffcutConsumeEntireLengthCreatesNoRemainder(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "offcut-full@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "offcut-full@example.com", "Secret123!")

	matID, whID := setupOffcutFixture(t, handler, accessToken)
	offcutID := registerOffcut(t, handler, accessToken, matID, whID, 500)

	consumeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/"+offcutID+"/consume", bytes.NewReader([]byte(`{"used_length_mm":500}`)))
	consumeReq.Header.Set("Authorization", "Bearer "+accessToken)
	consumeReq.Header.Set("Content-Type", "application/json")
	consumeRec := httptest.NewRecorder()
	handler.ServeHTTP(consumeRec, consumeReq)
	if consumeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for offcut consume, got %d with body %s", consumeRec.Code, consumeRec.Body.String())
	}
	var result struct {
		Remainder *struct {
			ID string `json:"id"`
		} `json:"remainder"`
	}
	if err := json.Unmarshal(consumeRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode offcut consume response: %v", err)
	}
	if result.Remainder != nil {
		t.Fatalf("expected no remainder when the entire length is used, got %+v", result.Remainder)
	}
}

func TestProfileOffcutConsumeRejectsExceedingLength(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "offcut-exceed@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "offcut-exceed@example.com", "Secret123!")

	matID, whID := setupOffcutFixture(t, handler, accessToken)
	offcutID := registerOffcut(t, handler, accessToken, matID, whID, 300)

	consumeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/"+offcutID+"/consume", bytes.NewReader([]byte(`{"used_length_mm":500}`)))
	consumeReq.Header.Set("Authorization", "Bearer "+accessToken)
	consumeReq.Header.Set("Content-Type", "application/json")
	consumeRec := httptest.NewRecorder()
	handler.ServeHTTP(consumeRec, consumeReq)
	if consumeRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for consuming more than the piece length, got %d with body %s", consumeRec.Code, consumeRec.Body.String())
	}
}

func TestProfileOffcutConsumeRejectsAlreadyConsumed(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "offcut-reconsume@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "offcut-reconsume@example.com", "Secret123!")

	matID, whID := setupOffcutFixture(t, handler, accessToken)
	offcutID := registerOffcut(t, handler, accessToken, matID, whID, 400)

	firstConsumeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/"+offcutID+"/consume", bytes.NewReader([]byte(`{"used_length_mm":100}`)))
	firstConsumeReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstConsumeReq.Header.Set("Content-Type", "application/json")
	firstConsumeRec := httptest.NewRecorder()
	handler.ServeHTTP(firstConsumeRec, firstConsumeReq)
	if firstConsumeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for first consume, got %d with body %s", firstConsumeRec.Code, firstConsumeRec.Body.String())
	}

	secondConsumeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/"+offcutID+"/consume", bytes.NewReader([]byte(`{"used_length_mm":100}`)))
	secondConsumeReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondConsumeReq.Header.Set("Content-Type", "application/json")
	secondConsumeRec := httptest.NewRecorder()
	handler.ServeHTTP(secondConsumeRec, secondConsumeReq)
	if secondConsumeRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for consuming an already consumed offcut, got %d with body %s", secondConsumeRec.Code, secondConsumeRec.Body.String())
	}
}

func TestProfileOffcutRegisterRejectsNonProfileMaterial(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "offcut-nonprofile@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "offcut-nonprofile@example.com", "Secret123!")

	nonce := uuid.NewString()[:8]
	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-NOPROF-`+nonce+`","bezeichnung":"Schraube","einheit":"Stk"}`)))
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

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-NOPROF-`+nonce+`","name":"Testlager"}`)))
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

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile-offcuts/", bytes.NewReader([]byte(`{"material_id":"`+mat.ID+`","warehouse_id":"`+wh.ID+`","length_mm":500}`)))
	registerReq.Header.Set("Authorization", "Bearer "+accessToken)
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for registering an offcut on a non-profile material, got %d with body %s", registerRec.Code, registerRec.Body.String())
	}
}
