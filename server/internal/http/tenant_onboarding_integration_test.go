package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

// TestPlatformTenantsCreateOnboardsNewTenant deckt Backlog 0.32 (Subtask
// 0.32.2b) ab: POST /platform/tenants legt atomar company_profiles-Zeile,
// eine Kopie des Kontenrahmens der 'default'-Company und den ersten
// Admin-Benutzer an. Verifiziert außerdem, dass ein regulärer Nutzer ohne
// admin.superuser den Endpunkt nicht aufrufen darf, dass der neue Mandant
// sofort funktionsfähig ist (Login, eigenes Firmenprofil, eigener
// Kontenrahmen) und dass doppelte IDs/E-Mail-Adressen sauber abgelehnt
// werden.
func TestPlatformTenantsCreateOnboardsNewTenant(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	testutil.SeedAuthUser(t, env, "integration-platform-superuser@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-platform-procurement@example.com", "Secret123!", "procurement")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	superuserToken := loginIntegrationUser(t, handler, "integration-platform-superuser@example.com", "Secret123!")
	procurementToken := loginIntegrationUser(t, handler, "integration-platform-procurement@example.com", "Secret123!")

	var defaultAccountCount int
	if err := env.PG.QueryRow(ctx, `SELECT COUNT(*) FROM accounts WHERE company_id='default'`).Scan(&defaultAccountCount); err != nil {
		t.Fatalf("count default accounts: %v", err)
	}
	if defaultAccountCount == 0 {
		t.Fatal("expected the 'default' company to have a seeded chart of accounts")
	}

	createBody := []byte(`{
		"id":"acme-metallbau",
		"name":"Acme Metallbau GmbH",
		"admin_email":"admin@acme-metallbau.example",
		"admin_password":"Secret123!",
		"admin_display_name":"Acme Admin"
	}`)

	forbiddenReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tenants/", bytes.NewReader(createBody))
	forbiddenReq.Header.Set("Authorization", "Bearer "+procurementToken)
	forbiddenReq.Header.Set("Content-Type", "application/json")
	forbiddenRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for tenant create without admin.superuser, got %d with body %s", forbiddenRec.Code, forbiddenRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tenants/", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+superuserToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for tenant create, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		CompanyID   string `json:"company_id"`
		Name        string `json:"name"`
		AdminUserID string `json:"admin_user_id"`
		AdminEmail  string `json:"admin_email"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode tenant create response: %v", err)
	}
	if created.CompanyID != "acme-metallbau" {
		t.Fatalf("expected company_id acme-metallbau, got %q", created.CompanyID)
	}
	if created.Name != "Acme Metallbau GmbH" {
		t.Fatalf("expected company name, got %q", created.Name)
	}
	if created.AdminUserID == "" {
		t.Fatal("expected admin_user_id")
	}
	if created.AdminEmail != "admin@acme-metallbau.example" {
		t.Fatalf("expected admin email, got %q", created.AdminEmail)
	}

	// Der neue Mandant muss sofort funktionsfaehig sein: Login, eigenes
	// Firmenprofil, eigener (kopierter) Kontenrahmen.
	newTenantToken := loginIntegrationUser(t, handler, "admin@acme-metallbau.example", "Secret123!")

	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/", nil)
	profileReq.Header.Set("Authorization", "Bearer "+newTenantToken)
	profileRec := httptest.NewRecorder()
	handler.ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for new tenant's own company profile, got %d with body %s", profileRec.Code, profileRec.Body.String())
	}
	var profile struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(profileRec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode company profile: %v", err)
	}
	if profile.Name != "Acme Metallbau GmbH" {
		t.Fatalf("expected new tenant's own profile name, got %q", profile.Name)
	}

	var newTenantAccountCount int
	if err := env.PG.QueryRow(ctx, `SELECT COUNT(*) FROM accounts WHERE company_id='acme-metallbau'`).Scan(&newTenantAccountCount); err != nil {
		t.Fatalf("count new tenant accounts: %v", err)
	}
	if newTenantAccountCount != defaultAccountCount {
		t.Fatalf("expected new tenant to have a copy of the default chart of accounts (%d accounts), got %d", defaultAccountCount, newTenantAccountCount)
	}

	// Stichprobe: dieselbe Kontonummer existiert jetzt fuer BEIDE Mandanten,
	// ohne Kollision (Backlog 0.32.2a, PK-Umbau auf (company_id, code)).
	var sharedCodeCount int
	if err := env.PG.QueryRow(ctx, `
		SELECT COUNT(DISTINCT company_id) FROM accounts WHERE code='1400' AND company_id IN ('default','acme-metallbau')
	`).Scan(&sharedCodeCount); err != nil {
		t.Fatalf("count shared account code: %v", err)
	}
	if sharedCodeCount != 2 {
		t.Fatalf("expected account code 1400 to exist independently for both tenants, got %d", sharedCodeCount)
	}

	duplicateIDReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tenants/", bytes.NewReader([]byte(`{
		"id":"acme-metallbau",
		"name":"Andere Firma",
		"admin_email":"other@example.com",
		"admin_password":"Secret123!"
	}`)))
	duplicateIDReq.Header.Set("Authorization", "Bearer "+superuserToken)
	duplicateIDReq.Header.Set("Content-Type", "application/json")
	duplicateIDRec := httptest.NewRecorder()
	handler.ServeHTTP(duplicateIDRec, duplicateIDReq)
	if duplicateIDRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate tenant id, got %d with body %s", duplicateIDRec.Code, duplicateIDRec.Body.String())
	}

	duplicateEmailReq := httptest.NewRequest(http.MethodPost, "/api/v1/platform/tenants/", bytes.NewReader([]byte(`{
		"name":"Wieder Andere Firma",
		"admin_email":"admin@acme-metallbau.example",
		"admin_password":"Secret123!"
	}`)))
	duplicateEmailReq.Header.Set("Authorization", "Bearer "+superuserToken)
	duplicateEmailReq.Header.Set("Content-Type", "application/json")
	duplicateEmailRec := httptest.NewRecorder()
	handler.ServeHTTP(duplicateEmailRec, duplicateEmailReq)
	if duplicateEmailRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate admin email, got %d with body %s", duplicateEmailRec.Code, duplicateEmailRec.Body.String())
	}
}
