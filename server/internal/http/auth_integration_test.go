package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/auth"
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

func TestAuthLoginRateLimitedAfterRepeatedFailures(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-ratelimit@example.com", "Secret123!", "admin")

	// Eigene, niedrige Schwelle statt der Produktiv-Defaults (10/60s), damit
	// der Test nicht 10+ Requests braucht. Eigene IP-Adresse (nicht die von
	// httptest.NewRequest() geteilte Default-IP "192.0.2.1"), damit dieser
	// Test keine anderen Tests im selben Lauf blockiert, die ueber dieselbe
	// Default-IP erfolgreich einloggen.
	limitedCfg := *env.Cfg
	limitedCfg.LoginRateLimitMaxAttempts = 3
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, &limitedCfg)

	const attackerIP = "198.51.100.42:9999"
	const attackerIPKey = "login_rl:198.51.100.42"
	t.Cleanup(func() {
		_ = env.Redis.Del(context.Background(), attackerIPKey).Err()
	})

	doLogin := func(login, password string) *httptest.ResponseRecorder {
		body, err := json.Marshal(map[string]string{"login": login, "password": password})
		if err != nil {
			t.Fatalf("marshal login body: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = attackerIP
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 3; i++ {
		rec := doLogin("integration-ratelimit@example.com", "wrong-password")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d with body %s", i+1, rec.Code, rec.Body.String())
		}
	}

	blockedRec := doLogin("integration-ratelimit@example.com", "wrong-password")
	if blockedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after exceeding limit, got %d with body %s", blockedRec.Code, blockedRec.Body.String())
	}
	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(blockedRec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Code != "rate_limited" {
		t.Fatalf("expected rate_limited error code, got %q", errResp.Error.Code)
	}

	// Waehrend der Sperre werden auch KORREKTE Zugangsdaten abgelehnt - die
	// Sperre gilt fuer die IP-Adresse, unabhaengig vom Ausgang des Versuchs.
	stillBlockedRec := doLogin("integration-ratelimit@example.com", "Secret123!")
	if stillBlockedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for correct credentials while IP is blocked, got %d with body %s", stillBlockedRec.Code, stillBlockedRec.Body.String())
	}
}

// TestUsersCreateEndpointCreatesUserWithinCallersCompany belegt Subtask
// 0.5.4.1 (User-Management-API statt Direkt-SQL): POST /api/v1/users/ legt
// einen neuen Benutzer im Mandanten des Aufrufers an. Der End-zu-Ende-Beweis
// dafuer, dass das Passwort-Hashing korrekt verdrahtet ist, ist ein
// anschliessender echter Login mit dem gerade vergebenen Passwort - nicht
// nur eine Struktur-/Statuscode-Pruefung der Create-Antwort.
func TestUsersCreateEndpointCreatesUserWithinCallersCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-useradmin@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-useradmin@example.com", "Secret123!")

	createBody := []byte(`{
		"email":"integration-new-user@example.com",
		"password":"NeuesPasswort1!",
		"first_name":"Neue",
		"last_name":"Person"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("password_hash")) || bytes.Contains(rec.Body.Bytes(), []byte("NeuesPasswort1!")) {
		t.Fatalf("response must never expose the password hash or plaintext, got body %s", rec.Body.String())
	}

	var created struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		CompanyID string `json:"company_id"`
		IsActive  bool   `json:"is_active"`
		IsLocked  bool   `json:"is_locked"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Email != "integration-new-user@example.com" {
		t.Fatalf("expected new user's email echoed back, got %q", created.Email)
	}
	if created.CompanyID != "default" {
		t.Fatalf("expected new user scoped to caller's company 'default', got %q", created.CompanyID)
	}
	if !created.IsActive || created.IsLocked {
		t.Fatalf("expected new user active and unlocked by default, got %#v", created)
	}

	// End-zu-Ende-Beweis: der neue Nutzer kann sich mit dem soeben gesetzten
	// Passwort tatsaechlich einloggen.
	loginBody, err := json.Marshal(map[string]string{"login": "integration-new-user@example.com", "password": "NeuesPasswort1!"})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected new user to be able to log in with the password set at creation, got %d with body %s", loginRec.Code, loginRec.Body.String())
	}

	// Duplikat (gleiche E-Mail) wird abgelehnt.
	dupReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/", bytes.NewReader(createBody))
	dupReq.Header.Set("Content-Type", "application/json")
	dupReq.Header.Set("Authorization", "Bearer "+accessToken)
	dupRec := httptest.NewRecorder()
	handler.ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate email, got %d with body %s", dupRec.Code, dupRec.Body.String())
	}
}

// TestUsersCreateEndpointIsForbiddenWithoutUsersManagePermission belegt,
// dass POST /api/v1/users/ weiterhin korrekt per 'users.manage' geschuetzt
// ist (insb. relevant nach Subtask 0.5.3, die 'users.manage' von einem
// impliziten Universal-Bypass zu einem eng skopierten Recht gemacht hat).
func TestUsersCreateEndpointIsForbiddenWithoutUsersManagePermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-sales-nouseradmin@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-sales-nouseradmin@example.com", "Secret123!")

	createBody := []byte(`{"email":"integration-should-not-exist@example.com","password":"NeuesPasswort1!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", rec.Code, rec.Body.String())
	}
}

// TestUsersLockEndpointBlocksNewLoginsAndInvalidatesExistingSessions belegt
// Subtask 0.5.4.2 (Sperren/Entsperren): POST /users/{id}/lock schaltet
// is_locked um. Die Sperre wirkt nicht nur gegen NEUE Logins, sondern auch
// gegen ein BEREITS ausgestelltes Access-Token - das folgt aus
// auth.Service.AuthenticateAccessToken, das user.IsLocked bei JEDEM Request
// frisch aus der DB liest (nicht nur beim Login), weshalb dieser Test das
// explizit end-to-end nachweist statt es nur anhand des Codes anzunehmen.
func TestUsersLockEndpointBlocksNewLoginsAndInvalidatesExistingSessions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-lockadmin@example.com", "Secret123!", "admin")
	targetUserID := testutil.SeedAuthUser(t, env, "integration-locktarget@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-lockadmin@example.com", "Secret123!")
	targetToken := loginIntegrationUser(t, handler, "integration-locktarget@example.com", "Secret123!")

	callMe := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	tryLogin := func() *httptest.ResponseRecorder {
		body, err := json.Marshal(map[string]string{"login": "integration-locktarget@example.com", "password": "Secret123!"})
		if err != nil {
			t.Fatalf("marshal login body: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := callMe(targetToken); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 before lock, got %d with body %s", rec.Code, rec.Body.String())
	}

	lockReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+targetUserID+"/lock", nil)
	lockReq.Header.Set("Authorization", "Bearer "+adminToken)
	lockRec := httptest.NewRecorder()
	handler.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for lock, got %d with body %s", lockRec.Code, lockRec.Body.String())
	}
	var lockedUser struct {
		IsLocked bool `json:"is_locked"`
	}
	if err := json.Unmarshal(lockRec.Body.Bytes(), &lockedUser); err != nil {
		t.Fatalf("decode lock response: %v", err)
	}
	if !lockedUser.IsLocked {
		t.Fatalf("expected is_locked=true in lock response, got %#v", lockedUser)
	}

	if rec := callMe(targetToken); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for pre-existing access token after lock, got %d with body %s", rec.Code, rec.Body.String())
	}
	if rec := tryLogin(); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 login while locked, got %d with body %s", rec.Code, rec.Body.String())
	}

	unlockReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+targetUserID+"/unlock", nil)
	unlockReq.Header.Set("Authorization", "Bearer "+adminToken)
	unlockRec := httptest.NewRecorder()
	handler.ServeHTTP(unlockRec, unlockReq)
	if unlockRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unlock, got %d with body %s", unlockRec.Code, unlockRec.Body.String())
	}
	var unlockedUser struct {
		IsLocked bool `json:"is_locked"`
	}
	if err := json.Unmarshal(unlockRec.Body.Bytes(), &unlockedUser); err != nil {
		t.Fatalf("decode unlock response: %v", err)
	}
	if unlockedUser.IsLocked {
		t.Fatalf("expected is_locked=false in unlock response, got %#v", unlockedUser)
	}

	if rec := tryLogin(); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 login after unlock, got %d with body %s", rec.Code, rec.Body.String())
	}
}

// TestUsersLockEndpointReturnsNotFoundForUserInAnotherCompany belegt das
// Mandanten-Scoping aus Task 0.2: ein Sperr-Versuch gegen einen Benutzer
// eines ANDEREN Mandanten liefert 404 (nicht 403 o.ae.) - verraet also nicht
// einmal die Existenz des fremden Kontos.
func TestUsersLockEndpointReturnsNotFoundForUserInAnotherCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	testutil.SeedAuthUser(t, env, "integration-lockadmin-crosstenant@example.com", "Secret123!", "admin")

	if _, err := env.PG.Exec(ctx, `
		INSERT INTO company_profiles (id, name) VALUES ('itest-other-company', 'Andere Firma')
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		t.Fatalf("seed other company: %v", err)
	}
	passwordHash, err := auth.HashPassword("Secret123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	const otherCompanyUserID = "itest-other-company-user"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO users (id, email, username, password_hash, first_name, last_name, display_name, locale, timezone, is_active, is_locked, company_id)
		VALUES ($1,$2,$2,$3,'Andere','Firma','Andere Firma','de-DE','Europe/Berlin',true,false,'itest-other-company')
		ON CONFLICT (email) DO UPDATE SET company_id=EXCLUDED.company_id
	`, otherCompanyUserID, "integration-other-company-user@example.com", passwordHash); err != nil {
		t.Fatalf("seed other-company user: %v", err)
	}

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-lockadmin-crosstenant@example.com", "Secret123!")

	lockReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+otherCompanyUserID+"/lock", nil)
	lockReq.Header.Set("Authorization", "Bearer "+adminToken)
	lockRec := httptest.NewRecorder()
	handler.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for user in another company, got %d with body %s", lockRec.Code, lockRec.Body.String())
	}
}

// TestUsersLockEndpointIsForbiddenWithoutUsersManagePermission belegt, dass
// Sperren/Entsperren wie das Anlegen per 'users.manage' geschuetzt ist.
func TestUsersLockEndpointIsForbiddenWithoutUsersManagePermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	targetUserID := testutil.SeedAuthUser(t, env, "integration-locktarget-forbidden@example.com", "Secret123!", "sales")
	testutil.SeedAuthUser(t, env, "integration-lockactor-forbidden@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	actorToken := loginIntegrationUser(t, handler, "integration-lockactor-forbidden@example.com", "Secret123!")

	lockReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+targetUserID+"/lock", nil)
	lockReq.Header.Set("Authorization", "Bearer "+actorToken)
	lockRec := httptest.NewRecorder()
	handler.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", lockRec.Code, lockRec.Body.String())
	}
}

// TestUsersRolesEndpointListsAvailableRoles belegt den lesenden Teil von
// Subtask 0.5.4.3: GET /users/roles listet alle im System bekannten Rollen
// (Grundlage fuer eine kuenftige Rollenauswahl in einer Verwaltungs-UI).
func TestUsersRolesEndpointListsAvailableRoles(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-rolesreader@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	token := loginIntegrationUser(t, handler, "integration-rolesreader@example.com", "Secret123!")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/roles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}
	var roles []struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatalf("decode roles response: %v", err)
	}
	found := false
	for _, r := range roles {
		if r.Code == "inventory" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected seeded role 'inventory' in list, got %#v", roles)
	}
}

// TestUsersRolesEndpointReplacesAssignmentAndTakesEffectOnNextLogin belegt
// den zentralen Teil von Subtask 0.5.4.3: PUT /users/{id}/roles ERSETZT die
// komplette Rollenmenge eines Nutzers (nicht inkrementell). Der Beweis, dass
// die neue Zuweisung nicht nur in der Antwort steht, sondern tatsaechlich
// wirkt, ist ein ECHTER Login danach, dessen Rollen/Berechtigungen die neue
// Zuweisung widerspiegeln muessen. Zusaetzlich wird geprueft, dass doppelte/
// leere Eintraege in der Eingabe normalisiert werden.
func TestUsersRolesEndpointReplacesAssignmentAndTakesEffectOnNextLogin(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-rolesadmin@example.com", "Secret123!", "admin")
	targetUserID := testutil.SeedAuthUser(t, env, "integration-rolestarget@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-rolesadmin@example.com", "Secret123!")

	// Vor der Umzuweisung hat der Zielnutzer die 'sales'-Rolle mit
	// 'quotes.write', aber ohne 'materials.write'.
	beforeToken := loginIntegrationUser(t, handler, "integration-rolestarget@example.com", "Secret123!")
	if !containsPermission(t, handler, beforeToken, "quotes.write") {
		t.Fatal("expected sales role to include quotes.write before reassignment")
	}

	putBody, err := json.Marshal(map[string]any{"role_codes": []string{"inventory", " inventory ", "", "  "}})
	if err != nil {
		t.Fatalf("marshal roles body: %v", err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+targetUserID+"/roles", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+adminToken)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", putRec.Code, putRec.Body.String())
	}
	var replaced struct {
		RoleCodes []string `json:"role_codes"`
	}
	if err := json.Unmarshal(putRec.Body.Bytes(), &replaced); err != nil {
		t.Fatalf("decode roles response: %v", err)
	}
	if len(replaced.RoleCodes) != 1 || replaced.RoleCodes[0] != "inventory" {
		t.Fatalf("expected normalized role_codes=['inventory'], got %#v", replaced.RoleCodes)
	}

	// Nach der Umzuweisung: neuer Login zeigt die NEUE Rolle, nicht mehr die alte.
	afterToken := loginIntegrationUser(t, handler, "integration-rolestarget@example.com", "Secret123!")
	if containsPermission(t, handler, afterToken, "quotes.write") {
		t.Fatal("expected old sales permission quotes.write to be gone after reassignment")
	}
	if !containsPermission(t, handler, afterToken, "materials.write") {
		t.Fatal("expected new inventory permission materials.write after reassignment")
	}
}

func containsPermission(t *testing.T, handler http.Handler, token, permission string) bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /auth/me, got %d with body %s", rec.Code, rec.Body.String())
	}
	var me struct {
		Data struct {
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode /auth/me response: %v", err)
	}
	for _, p := range me.Data.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// TestUsersRolesEndpointRejectsUnknownRoleCode belegt die
// Alles-oder-nichts-Validierung: ein unbekannter Rollen-Code lehnt die
// GESAMTE Anfrage ab, statt teilweise anzuwenden.
func TestUsersRolesEndpointRejectsUnknownRoleCode(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-rolesadmin-unknown@example.com", "Secret123!", "admin")
	targetUserID := testutil.SeedAuthUser(t, env, "integration-rolestarget-unknown@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-rolesadmin-unknown@example.com", "Secret123!")

	putBody, err := json.Marshal(map[string]any{"role_codes": []string{"does-not-exist"}})
	if err != nil {
		t.Fatalf("marshal roles body: %v", err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+targetUserID+"/roles", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+adminToken)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown role code, got %d with body %s", putRec.Code, putRec.Body.String())
	}
}

// TestUsersRolesEndpointReturnsNotFoundForUserInAnotherCompany belegt das
// Mandanten-Scoping analog zu TestUsersLockEndpointReturnsNotFoundForUserInAnotherCompany.
func TestUsersRolesEndpointReturnsNotFoundForUserInAnotherCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	testutil.SeedAuthUser(t, env, "integration-rolesadmin-crosstenant@example.com", "Secret123!", "admin")

	if _, err := env.PG.Exec(ctx, `
		INSERT INTO company_profiles (id, name) VALUES ('itest-other-company-roles', 'Andere Firma Rollen')
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		t.Fatalf("seed other company: %v", err)
	}
	passwordHash, err := auth.HashPassword("Secret123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	const otherCompanyUserID = "itest-other-company-roles-user"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO users (id, email, username, password_hash, first_name, last_name, display_name, locale, timezone, is_active, is_locked, company_id)
		VALUES ($1,$2,$2,$3,'Andere','Firma','Andere Firma','de-DE','Europe/Berlin',true,false,'itest-other-company-roles')
		ON CONFLICT (email) DO UPDATE SET company_id=EXCLUDED.company_id
	`, otherCompanyUserID, "integration-other-company-roles-user@example.com", passwordHash); err != nil {
		t.Fatalf("seed other-company user: %v", err)
	}

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-rolesadmin-crosstenant@example.com", "Secret123!")

	putBody, err := json.Marshal(map[string]any{"role_codes": []string{"inventory"}})
	if err != nil {
		t.Fatalf("marshal roles body: %v", err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+otherCompanyUserID+"/roles", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+adminToken)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for user in another company, got %d with body %s", putRec.Code, putRec.Body.String())
	}
}

// TestUsersRolesEndpointIsForbiddenWithoutUsersManagePermission belegt, dass
// Rollenzuweisung wie Anlegen/Sperren per 'users.manage' geschuetzt ist.
func TestUsersRolesEndpointIsForbiddenWithoutUsersManagePermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	targetUserID := testutil.SeedAuthUser(t, env, "integration-rolestarget-forbidden@example.com", "Secret123!", "sales")
	testutil.SeedAuthUser(t, env, "integration-rolesactor-forbidden@example.com", "Secret123!", "sales")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	actorToken := loginIntegrationUser(t, handler, "integration-rolesactor-forbidden@example.com", "Secret123!")

	putBody, err := json.Marshal(map[string]any{"role_codes": []string{"inventory"}})
	if err != nil {
		t.Fatalf("marshal roles body: %v", err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+targetUserID+"/roles", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+actorToken)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", putRec.Code, putRec.Body.String())
	}
}

// TestUsersManagePermissionNoLongerBypassesOtherPermissionChecks belegt die
// Subtask-0.5.3-Korrektur: 'users.manage' war zuvor ein impliziter
// Universal-Bypass in requirePermission() (server/internal/http/v1.go) - JEDE
// Rolle mit diesem engen "Benutzer/Rollen verwalten"-Recht erhielt dadurch
// ungewollt auch Zugriff auf alle anderen Module (Materialien, Lager,
// Bestellungen, ...). Es gibt keine vorbestehende Rolle mit AUSSCHLIESSLICH
// 'users.manage' (nur 'admin' hat es, und 'admin' hat ohnehin bereits jede
// Einzelberechtigung direkt zugewiesen) - daher wird hier gezielt eine
// Test-Rolle mit genau diesem einen Recht angelegt, um den frueheren
// Bypass-Pfad zu pruefen.
func TestUsersManagePermissionNoLongerBypassesOtherPermissionChecks(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()

	const roleID = "role-itest-users-manage-only"
	const roleCode = "itest-users-manage-only"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO roles (id, code, name, description, is_system)
		VALUES ($1, $2, 'Test: nur users.manage', '', false)
		ON CONFLICT (code) DO NOTHING
	`, roleID, roleCode); err != nil {
		t.Fatalf("seed test role: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT $1, p.id FROM permissions p WHERE p.code = 'users.manage'
		ON CONFLICT DO NOTHING
	`, roleID); err != nil {
		t.Fatalf("seed test role permission: %v", err)
	}

	testutil.SeedAuthUser(t, env, "integration-users-manage-only@example.com", "Secret123!", roleCode)

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-users-manage-only@example.com", "Secret123!")

	createBody := []byte(`{
		"nummer":"MAT-IT-USERSMANAGE",
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
		t.Fatalf("expected 403 (users.manage must not bypass materials.write), got %d with body %s", rec.Code, rec.Body.String())
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
