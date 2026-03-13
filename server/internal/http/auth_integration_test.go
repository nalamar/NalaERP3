package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestAuthLoginAndMeFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-admin@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)

	loginBody := []byte(`{"login":"integration-admin@example.com","password":"Secret123!"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp struct {
		Data struct {
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"user"`
			Roles       []string `json:"roles"`
			Permissions []string `json:"permissions"`
			Tokens      struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				TokenType    string `json:"token_type"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResp.Data.User.Email != "integration-admin@example.com" {
		t.Fatalf("expected integration-admin@example.com, got %q", loginResp.Data.User.Email)
	}
	if loginResp.Data.Tokens.AccessToken == "" || loginResp.Data.Tokens.RefreshToken == "" {
		t.Fatalf("expected token pair, got %#v", loginResp.Data.Tokens)
	}
	if loginResp.Data.Tokens.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", loginResp.Data.Tokens.TokenType)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Data.Tokens.AccessToken)
	meRec := httptest.NewRecorder()

	handler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", meRec.Code, meRec.Body.String())
	}

	var meResp struct {
		Data struct {
			User struct {
				Email string `json:"email"`
			} `json:"user"`
			Roles       []string `json:"roles"`
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResp.Data.User.Email != "integration-admin@example.com" {
		t.Fatalf("expected integration-admin@example.com, got %q", meResp.Data.User.Email)
	}
	if len(meResp.Data.Roles) == 0 {
		t.Fatal("expected at least one role")
	}
	if len(meResp.Data.Permissions) == 0 {
		t.Fatal("expected at least one permission")
	}
}
