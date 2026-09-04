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

func TestCostCentersCreateGetListUpdateFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "cost-centers-crud@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "cost-centers-crud@example.com", "Secret123!")

	nonce := uuid.NewString()[:8]
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-centers/", bytes.NewReader([]byte(`{"code":"WERK-`+nonce+`","name":"Werkstatt Nord","note":"Testkostenstelle"}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for cost center create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID    string `json:"id"`
		Code  string `json:"code"`
		Aktiv bool   `json:"aktiv"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode cost center create response: %v", err)
	}
	if !created.Aktiv {
		t.Fatalf("expected new cost center to default to aktiv=true, got %+v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/cost-centers/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cost center get, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/cost-centers/", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cost center list, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode cost center list response: %v", err)
	}
	found := false
	for _, cc := range listed {
		if cc.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected list to contain the created cost center, got %+v", listed)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/cost-centers/"+created.ID, bytes.NewReader([]byte(`{"aktiv":false}`)))
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cost center update, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}
	var updated struct {
		Aktiv bool `json:"aktiv"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode cost center update response: %v", err)
	}
	if updated.Aktiv {
		t.Fatalf("expected aktiv=false after update, got %+v", updated)
	}

	listActiveOnlyReq := httptest.NewRequest(http.MethodGet, "/api/v1/cost-centers/", nil)
	listActiveOnlyReq.Header.Set("Authorization", "Bearer "+accessToken)
	listActiveOnlyRec := httptest.NewRecorder()
	handler.ServeHTTP(listActiveOnlyRec, listActiveOnlyReq)
	if listActiveOnlyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for active-only list, got %d with body %s", listActiveOnlyRec.Code, listActiveOnlyRec.Body.String())
	}
	var activeOnly []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listActiveOnlyRec.Body.Bytes(), &activeOnly); err != nil {
		t.Fatalf("decode active-only list response: %v", err)
	}
	for _, cc := range activeOnly {
		if cc.ID == created.ID {
			t.Fatalf("expected deactivated cost center to be excluded from default list, got %+v", activeOnly)
		}
	}

	listIncludeInactiveReq := httptest.NewRequest(http.MethodGet, "/api/v1/cost-centers/?include_inactive=true", nil)
	listIncludeInactiveReq.Header.Set("Authorization", "Bearer "+accessToken)
	listIncludeInactiveRec := httptest.NewRecorder()
	handler.ServeHTTP(listIncludeInactiveRec, listIncludeInactiveReq)
	if listIncludeInactiveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for include_inactive list, got %d with body %s", listIncludeInactiveRec.Code, listIncludeInactiveRec.Body.String())
	}
	var includeInactive []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listIncludeInactiveRec.Body.Bytes(), &includeInactive); err != nil {
		t.Fatalf("decode include_inactive list response: %v", err)
	}
	foundInactive := false
	for _, cc := range includeInactive {
		if cc.ID == created.ID {
			foundInactive = true
		}
	}
	if !foundInactive {
		t.Fatalf("expected include_inactive=true to show the deactivated cost center, got %+v", includeInactive)
	}
}

func TestCostCentersCreateRejectsDuplicateCode(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "cost-centers-dup@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "cost-centers-dup@example.com", "Secret123!")

	nonce := uuid.NewString()[:8]
	body := `{"code":"WERK-DUP-` + nonce + `","name":"Werkstatt"}`

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-centers/", bytes.NewReader([]byte(body)))
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first cost center, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-centers/", bytes.NewReader([]byte(body)))
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate code, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}
}
