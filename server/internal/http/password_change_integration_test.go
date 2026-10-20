package apihttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Erzwungener Passwortwechsel (Backlog 0.49): solange must_change_password
// gesetzt ist, sind alle geschuetzten Routen gesperrt - ausser /auth/me und
// /auth/change-password.

func TestPasswordChangeRequiredHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	ctx := context.Background()
	email := "pwwechsel-" + uuid.NewString()[:8] + "@example.com"
	userID := testutil.SeedAuthUser(t, env, email, "Secret123!", "hr")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET must_change_password=true WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}

	// Anmelden geht; die Antwort traegt das Kennzeichen.
	var login struct {
		Data struct {
			User struct {
				MustChangePassword bool `json:"must_change_password"`
			} `json:"user"`
			Tokens struct {
				AccessToken string `json:"access_token"`
			} `json:"tokens"`
		} `json:"data"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, "", http.MethodPost, "/api/v1/auth/login", `{"login":"`+email+`","password":"Secret123!"}`), http.StatusOK, &login)
	if !login.Data.User.MustChangePassword || login.Data.Tokens.AccessToken == "" {
		t.Fatalf("Login: %+v", login)
	}
	token := login.Data.Tokens.AccessToken

	// Gesperrt: jede geschuetzte Route.
	for _, path := range []string{"/api/v1/employees/", "/api/v1/holidays/", "/api/v1/leave-requests/"} {
		wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodGet, path, ""), http.StatusForbidden, "password_change_required", "Passwort muss geändert werden")
	}
	// Erlaubt: /auth/me mit Kennzeichen.
	var me struct {
		Data struct {
			User struct {
				MustChangePassword bool `json:"must_change_password"`
			} `json:"user"`
		} `json:"data"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, token, http.MethodGet, "/api/v1/auth/me", ""), http.StatusOK, &me)
	if !me.Data.User.MustChangePassword {
		t.Fatalf("/auth/me ohne Kennzeichen: %+v", me)
	}

	const path = "/api/v1/auth/change-password"
	wantStatus(t, doAuthedRequest(t, handler, "", http.MethodPost, path, `{"current_password":"Secret123!","new_password":"NeuesPasswort1"}`), http.StatusUnauthorized)
	wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, path, `{"current_password":"falsch","new_password":"NeuesPasswort1"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges aktuelles Passwort")
	wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, path, `{"current_password":"Secret123!","new_password":"kurz"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges neues Passwort: mindestens 10 Zeichen")
	wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, path, `{"current_password":"Secret123!","new_password":"Secret123!"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges neues Passwort: es muss sich vom bisherigen unterscheiden")
	wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, path, `{"current_password":"Secret123!"}`),
		http.StatusBadRequest, "validation_error", "Aktuelles und neues Passwort erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, path, `[]`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	// Abgelehnte Versuche heben die Sperre nicht auf.
	wantStatus(t, doAuthedRequest(t, handler, token, http.MethodGet, "/api/v1/holidays/", ""), http.StatusForbidden)

	wantStatus(t, doAuthedRequest(t, handler, token, http.MethodPost, path, `{"current_password":"Secret123!","new_password":"NeuesPasswort1"}`), http.StatusNoContent)
	// Mit demselben Token sofort frei.
	wantStatus(t, doAuthedRequest(t, handler, token, http.MethodGet, "/api/v1/holidays/", ""), http.StatusOK)
	var flag bool
	var changedAt *string
	if err := env.PG.QueryRow(ctx, `SELECT must_change_password, password_changed_at::text FROM users WHERE id=$1`, userID).Scan(&flag, &changedAt); err != nil || flag || changedAt == nil {
		t.Fatalf("nach Wechsel: flag=%v changedAt=%v err=%v", flag, changedAt, err)
	}
	// Altes Passwort gilt nicht mehr, das neue schon.
	wantStatus(t, doAuthedRequest(t, handler, "", http.MethodPost, "/api/v1/auth/login", `{"login":"`+email+`","password":"Secret123!"}`), http.StatusUnauthorized)
	wantStatus(t, doAuthedRequest(t, handler, "", http.MethodPost, "/api/v1/auth/login", `{"login":"`+email+`","password":"NeuesPasswort1"}`), http.StatusOK)
}
