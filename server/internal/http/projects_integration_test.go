package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestProjectQuotePDFFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-project-quote@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-project-quote@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Projektkunde Metallbau GmbH",
		"email":    "projektkunde@example.com",
		"telefon":  "+49 30 123456",
		"waehrung": "EUR",
	})

	templateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/pdf/quote", bytes.NewReader([]byte(`{
		"header_text":"Angebotskopf Projekt",
		"footer_text":"Angebotsfuss Projekt",
		"top_first_mm":32,
		"top_other_mm":21
	}`)))
	templateReq.Header.Set("Authorization", "Bearer "+accessToken)
	templateReq.Header.Set("Content-Type", "application/json")
	templateRec := httptest.NewRecorder()
	handler.ServeHTTP(templateRec, templateReq)
	if templateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for quote template update, got %d with body %s", templateRec.Code, templateRec.Body.String())
	}

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Wohnanlage Nord",
		"kunde_id":"`+customerID+`",
		"status":"angebot"
	}`)))
	createProjectReq.Header.Set("Authorization", "Bearer "+accessToken)
	createProjectReq.Header.Set("Content-Type", "application/json")
	createProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(createProjectRec, createProjectReq)
	if createProjectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", createProjectRec.Code, createProjectRec.Body.String())
	}

	var createdProject struct {
		ID     string `json:"id"`
		Nummer string `json:"nummer"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &createdProject); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}
	if createdProject.ID == "" {
		t.Fatal("expected project id")
	}

	createPhaseReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+createdProject.ID+"/phases", bytes.NewReader([]byte(`{
		"nummer":"1",
		"name":"Los Fassade",
		"beschreibung":"Fassadenarbeiten",
		"sort_order":10
	}`)))
	createPhaseReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPhaseReq.Header.Set("Content-Type", "application/json")
	createPhaseRec := httptest.NewRecorder()
	handler.ServeHTTP(createPhaseRec, createPhaseReq)
	if createPhaseRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for phase create, got %d with body %s", createPhaseRec.Code, createPhaseRec.Body.String())
	}

	var createdPhase struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createPhaseRec.Body.Bytes(), &createdPhase); err != nil {
		t.Fatalf("decode phase create response: %v", err)
	}
	if createdPhase.ID == "" {
		t.Fatal("expected phase id")
	}

	createElevationReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+createdProject.ID+"/phases/"+createdPhase.ID+"/elevations", bytes.NewReader([]byte(`{
		"nummer":"1.1",
		"name":"Fensterelement EG",
		"beschreibung":"Pfosten-Riegel mit RAL 7016",
		"menge":4,
		"width_mm":1500,
		"height_mm":2400
	}`)))
	createElevationReq.Header.Set("Authorization", "Bearer "+accessToken)
	createElevationReq.Header.Set("Content-Type", "application/json")
	createElevationRec := httptest.NewRecorder()
	handler.ServeHTTP(createElevationRec, createElevationReq)
	if createElevationRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for elevation create, got %d with body %s", createElevationRec.Code, createElevationRec.Body.String())
	}

	pdfReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+createdProject.ID+"/quote-pdf", nil)
	pdfReq.Header.Set("Authorization", "Bearer "+accessToken)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)
	if pdfRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote pdf, got %d with body %s", pdfRec.Code, pdfRec.Body.String())
	}
	if ct := pdfRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected pdf content type, got %q", ct)
	}
	if disp := pdfRec.Header().Get("Content-Disposition"); disp == "" {
		t.Fatal("expected content disposition header")
	}
	if pdfRec.Body.Len() == 0 {
		t.Fatal("expected non-empty pdf response")
	}
}
