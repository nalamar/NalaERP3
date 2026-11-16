package apihttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"nalaerp3/internal/llm"
	"nalaerp3/internal/testutil"
)

// Routen /settings/llm (Backlog I.3.5a2). Erfundener Schluessel; der
// Verbindungstest geht an einen lokalen Test-Server.

const llmTestKey = "erfundener-Schluessel-42"

func TestLLMSettingsRoutes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	cleanup := func() {
		_, _ = env.PG.Exec(context.Background(), `DELETE FROM company_llm_settings WHERE company_id='default'`)
		_, _ = env.PG.Exec(context.Background(), `DELETE FROM company_llm_usage WHERE company_id='default'`)
	}
	cleanup()
	t.Cleanup(cleanup)
	cfg := *env.Cfg
	cfg.SettingsEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &cfg, V1RouterOptions{})
	testutil.SeedAuthUser(t, env, "llm-admin@example.test", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "llm-sales@example.test", "Secret123!", "sales")
	admin := loginIntegrationUser(t, h, "llm-admin@example.test", "Secret123!")
	sales := loginIntegrationUser(t, h, "llm-sales@example.test", "Secret123!")

	// Test-Server: antwortet wie ein OpenAI-kompatibler Endpunkt; fail -> 400.
	var calls, fail int32
	var lastAuth, lastBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		lastAuth.Store(r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		lastBody.Store(string(body))
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"model not found"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"model":"modell-1","choices":[{"message":{"role":"assistant","content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":12,"completion_tokens":3}}`)
	}))
	defer srv.Close()

	// Ohne Anmeldung 401, ohne settings.manage 403 - auch fuer den Test.
	if rec := mailRequest(t, h, "", http.MethodGet, "/settings/llm/", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/settings/llm/", ""},
		{http.MethodPut, "/settings/llm/", `{"aktiv":false}`},
		{http.MethodPost, "/settings/llm/test", ""},
	} {
		if rec := mailRequest(t, h, sales, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Fatalf("ohne Recht %s %s: %d", c.method, c.path, rec.Code)
		}
	}

	rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/", "")
	var got llm.Settings
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Eingerichtet || got.TimeoutS != 300 || got.Tageslimit != 500 || !got.SchluesselEingerichtet {
		t.Fatalf("Standard: %d %s", rec.Code, rec.Body.String())
	}
	// Test ohne Einstellung: 409, keine Anfrage nach aussen.
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/test", ""); rec.Code != http.StatusConflict ||
		mailErrorMessage(t, rec) != "Sprachmodell nicht vollständig eingerichtet" || atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("Test ohne Einstellung: %d %s", rec.Code, rec.Body.String())
	}

	// Ungueltige Eingaben: 400, nichts gespeichert.
	for _, body := range []string{`{"anbieter":"gpt"}`, `{"timeout_s":1801}`, `{"aktiv":true}`, `{"aktiv":`} {
		if rec := mailRequest(t, h, admin, http.MethodPut, "/settings/llm/", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/", ""); !strings.Contains(rec.Body.String(), `"eingerichtet":false`) {
		t.Fatalf("nach Fehlern: %s", rec.Body.String())
	}

	// Speichern (nicht eingeschaltet); Schluessel nie in der Antwort.
	full := `{"anbieter":"openai_compatible","endpunkt":"` + srv.URL + `","modell":"lokal","api_key":"` + llmTestKey + `","timeout_s":20,"tageslimit":2}`
	rec = mailRequest(t, h, admin, http.MethodPut, "/settings/llm/", full)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), llmTestKey) || json.Unmarshal(rec.Body.Bytes(), &got) != nil ||
		!got.Eingerichtet || got.Aktiv || !got.APIKeyGesetzt || got.TimeoutS != 20 || got.UpdatedByUserID == nil {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/", ""); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), llmTestKey) ||
		!strings.Contains(rec.Body.String(), `"api_key_gesetzt":true`) {
		t.Fatalf("lesen: %d %s", rec.Code, rec.Body.String())
	}

	// Verbindungstest auch ausgeschaltet: 200 ohne Inhalt der Antwort.
	rec = mailRequest(t, h, admin, http.MethodPost, "/settings/llm/test", "")
	var res map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &res) != nil || res["ok"] != true || res["modell"] != "modell-1" ||
		res["eingabe_tokens"] != float64(12) || res["ausgabe_tokens"] != float64(3) || res["dauer_ms"] == nil || strings.Contains(rec.Body.String(), `\"ok\"`) {
		t.Fatalf("Test: %d %s", rec.Code, rec.Body.String())
	}
	sent, _ := lastBody.Load().(string)
	if lastAuth.Load() != "Bearer "+llmTestKey || atomic.LoadInt32(&calls) != 1 || !strings.Contains(sent, `"max_tokens":1024`) ||
		!strings.Contains(sent, `"response_format"`) || !strings.Contains(sent, `"model":"lokal"`) {
		t.Fatalf("Anfrage: %v %d %s", lastAuth.Load(), calls, sent)
	}

	// Dienst lehnt ab: 502 mit Meldung, ohne Schluessel; zaehlt zum Tageslimit.
	atomic.StoreInt32(&fail, 1)
	rec = mailRequest(t, h, admin, http.MethodPost, "/settings/llm/test", "")
	if rec.Code != http.StatusBadGateway || !strings.HasPrefix(mailErrorMessage(t, rec), "Test des Sprachmodells fehlgeschlagen: ") ||
		strings.Contains(rec.Body.String(), llmTestKey) {
		t.Fatalf("abgelehnt: %d %s", rec.Code, rec.Body.String())
	}
	atomic.StoreInt32(&fail, 0)

	// Tageslimit (2) erreicht: 429, keine Anfrage nach aussen.
	before := atomic.LoadInt32(&calls)
	rec = mailRequest(t, h, admin, http.MethodPost, "/settings/llm/test", "")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(mailErrorMessage(t, rec), "Tageslimit des Sprachmodells erreicht") || atomic.LoadInt32(&calls) != before {
		t.Fatalf("Tageslimit: %d %s", rec.Code, rec.Body.String())
	}

	// Anderer Verschluesselungsschluessel: Schluessel nicht lesbar -> 400, keine Anfrage.
	rotated := cfg
	rotated.SettingsEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	h2 := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &rotated, V1RouterOptions{})
	admin2 := loginIntegrationUser(t, h2, "llm-admin@example.test", "Secret123!")
	if rec := mailRequest(t, h2, admin2, http.MethodPost, "/settings/llm/test", ""); rec.Code != http.StatusBadRequest ||
		!strings.Contains(mailErrorMessage(t, rec), "nicht lesbar") || atomic.LoadInt32(&calls) != before {
		t.Fatalf("anderer Schlüssel: %d %s", rec.Code, rec.Body.String())
	}
}
