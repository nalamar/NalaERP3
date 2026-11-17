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

	"github.com/google/uuid"

	"nalaerp3/internal/llm"
	"nalaerp3/internal/testutil"
)

// Routen /settings/llm/profile und /settings/llm/mandant (Backlog I.8.1b).
// Erfundener Schluessel; Verbindungstests gehen an einen lokalen Test-Server.

func llmCleanupDefault(env *testutil.IntegrationEnv) {
	for _, table := range []string{"company_llm_profiles", "company_llm_settings", "company_llm_usage"} {
		_, _ = env.PG.Exec(context.Background(), `DELETE FROM `+table+` WHERE company_id='default'`)
	}
}

func TestLLMProfileRoutes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	llmCleanupDefault(env)
	t.Cleanup(func() { llmCleanupDefault(env) })
	cfg := *env.Cfg
	cfg.SettingsEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &cfg, V1RouterOptions{})
	testutil.SeedAuthUser(t, env, "llmp-admin@example.test", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "llmp-sales@example.test", "Secret123!", "sales")
	admin := loginIntegrationUser(t, h, "llmp-admin@example.test", "Secret123!")
	sales := loginIntegrationUser(t, h, "llmp-sales@example.test", "Secret123!")

	var calls int32
	var lastAuth, lastBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		lastAuth.Store(r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		lastBody.Store(string(body))
		_, _ = io.WriteString(w, `{"model":"modell-1","choices":[{"message":{"role":"assistant","content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":12,"completion_tokens":3}}`)
	}))
	defer srv.Close()

	// fremdes Profil eines anderen Mandanten
	other := "llmp-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, other, "LLM-Test "+other); err != nil {
		t.Fatal(err)
	}
	fremd, err := llm.NewSettingsService(env.PG, nil).CreateProfile(context.Background(), llm.ProfileInput{Name: "Fremd"}, "", other)
	if err != nil {
		t.Fatal(err)
	}

	// ohne Anmeldung 401, ohne settings.manage 403
	if rec := mailRequest(t, h, "", http.MethodGet, "/settings/llm/profile", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/settings/llm/mandant", ""},
		{http.MethodPut, "/settings/llm/mandant", `{"aktiv":false}`},
		{http.MethodGet, "/settings/llm/profile", ""},
		{http.MethodPost, "/settings/llm/profile", `{"name":"x"}`},
		{http.MethodGet, "/settings/llm/profile/" + fremd.ID, ""},
		{http.MethodPut, "/settings/llm/profile/" + fremd.ID, `{"name":"x"}`},
		{http.MethodPost, "/settings/llm/profile/" + fremd.ID + "/standard", ""},
		{http.MethodPost, "/settings/llm/profile/" + fremd.ID + "/test", ""},
	} {
		if rec := mailRequest(t, h, sales, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Fatalf("ohne Recht %s %s: %d", c.method, c.path, rec.Code)
		}
	}

	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/profile", ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("leere Liste: %d %s", rec.Code, rec.Body.String())
	}
	var m llm.Mandant
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/mandant", ""); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &m) != nil ||
		m.Aktiv || m.Tageslimit != 500 || !m.SchluesselEingerichtet {
		t.Fatalf("Mandant: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodPut, "/settings/llm/mandant", `{"aktiv":true}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(mailErrorMessage(t, rec), "fehlt ein eingeschaltetes Profil") {
		t.Fatalf("an ohne Profil: %d %s", rec.Code, rec.Body.String())
	}

	// ungueltige Eingaben: 400, nichts angelegt
	for _, body := range []string{`{"name":""}`, `{"name":"x","schwelle":1.5}`, `{"name":"x","anbieter":"gpt"}`, `{"name":`} {
		if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := mailRequest(t, h, admin, http.MethodPut, "/settings/llm/mandant", `{"tageslimit":100001}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("Tageslimit: %d %s", rec.Code, rec.Body.String())
	}

	// Anlegen: erstes Profil wird Standard; Schluessel nie in der Antwort
	var lokal, zweit llm.Profile
	rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile",
		`{"name":"Lokal","aktiv":true,"endpunkt":"`+srv.URL+`","modell":"lokal","api_key":"`+llmTestKey+`"}`)
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), llmTestKey) || json.Unmarshal(rec.Body.Bytes(), &lokal) != nil ||
		!lokal.Standard || !lokal.APIKeyLesbar || lokal.Schwelle != 0.9 || lokal.UpdatedByUserID == nil {
		t.Fatalf("Lokal: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile", `{"name":"LOKAL"}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(mailErrorMessage(t, rec), "bereits vorhanden") {
		t.Fatalf("Name doppelt: %d %s", rec.Code, rec.Body.String())
	}
	rec = mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile", `{"name":"Zweit","endpunkt":"`+srv.URL+`","modell":"z","schwelle":0.5}`)
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &zweit) != nil || zweit.Standard || zweit.Schwelle != 0.5 || zweit.Aktiv {
		t.Fatalf("Zweit: %d %s", rec.Code, rec.Body.String())
	}
	var list []llm.Profile
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/profile", ""); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), llmTestKey) ||
		json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 2 || list[0].ID != lokal.ID || list[1].ID != zweit.ID {
		t.Fatalf("Liste: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/profile/"+zweit.ID, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Zweit"`) {
		t.Fatalf("Lesen: %d %s", rec.Code, rec.Body.String())
	}

	// fremd, unbekannt, ungueltig: 404 ohne Aenderung
	for _, id := range []string{fremd.ID, uuid.NewString(), "kein-uuid"} {
		for _, c := range []struct{ method, path, body string }{
			{http.MethodGet, "/settings/llm/profile/" + id, ""},
			{http.MethodPut, "/settings/llm/profile/" + id, `{"name":"Gekapert"}`},
			{http.MethodPost, "/settings/llm/profile/" + id + "/standard", ""},
			{http.MethodPost, "/settings/llm/profile/" + id + "/test", ""},
		} {
			if rec := mailRequest(t, h, admin, c.method, c.path, c.body); rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body.String())
			}
		}
	}
	if p, _ := llm.NewSettingsService(env.PG, nil).GetProfile(context.Background(), other, fremd.ID); p.Name != "Fremd" || !p.Standard {
		t.Fatalf("fremdes Profil verändert: %+v", p)
	}

	// Hauptschalter an; letztes eingeschaltetes Profil nicht ausschaltbar
	if rec := mailRequest(t, h, admin, http.MethodPut, "/settings/llm/mandant", `{"aktiv":true,"tageslimit":2}`); rec.Code != http.StatusOK ||
		json.Unmarshal(rec.Body.Bytes(), &m) != nil || !m.Aktiv || m.Tageslimit != 2 || m.UpdatedByUserID == nil {
		t.Fatalf("Mandant an: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodPut, "/settings/llm/profile/"+lokal.ID, `{"name":"Lokal","endpunkt":"`+srv.URL+`","modell":"lokal"}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(mailErrorMessage(t, rec), "darf nicht ausgeschaltet werden") {
		t.Fatalf("letztes Profil: %d %s", rec.Code, rec.Body.String())
	}

	// Verbindungstest je Profil, auch ausgeschaltet; Tageslimit gilt fuer alle
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile/"+zweit.ID+"/test", ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("Test Zweit: %d %s", rec.Code, rec.Body.String())
	}
	if sent, _ := lastBody.Load().(string); !strings.Contains(sent, `"model":"z"`) || lastAuth.Load() != "" {
		t.Fatalf("Anfrage Zweit: %v %s", lastAuth.Load(), sent)
	}
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile/"+lokal.ID+"/test", ""); rec.Code != http.StatusOK {
		t.Fatalf("Test Lokal: %d %s", rec.Code, rec.Body.String())
	}
	if sent, _ := lastBody.Load().(string); !strings.Contains(sent, `"model":"lokal"`) || lastAuth.Load() != "Bearer "+llmTestKey {
		t.Fatalf("Anfrage Lokal: %v %s", lastAuth.Load(), sent)
	}
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile/"+zweit.ID+"/test", ""); rec.Code != http.StatusTooManyRequests ||
		atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("Tageslimit: %d %s %d", rec.Code, rec.Body.String(), calls)
	}

	// Standard wechseln: bisherige Einstellung folgt
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/llm/profile/"+zweit.ID+"/standard", ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"standard":true`) {
		t.Fatalf("Standard: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/llm/", ""); !strings.Contains(rec.Body.String(), `"modell":"z"`) {
		t.Fatalf("Einstellung nach Wechsel: %s", rec.Body.String())
	}
	// Ersetzen ohne Schluessel behaelt ihn; Bearbeiter wird neu gesetzt
	if _, err := env.PG.Exec(context.Background(), `UPDATE company_llm_profiles SET updated_by_user_id=NULL WHERE id=$1`, lokal.ID); err != nil {
		t.Fatal(err)
	}
	var got llm.Profile
	rec = mailRequest(t, h, admin, http.MethodPut, "/settings/llm/profile/"+lokal.ID,
		`{"name":"Lokal neu","aktiv":true,"endpunkt":"`+srv.URL+`","modell":"lokal","schwelle":0.6}`)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Name != "Lokal neu" || got.Schwelle != 0.6 || !got.APIKeyLesbar || got.Standard ||
		got.UpdatedByUserID == nil {
		t.Fatalf("Ersetzen: %d %s", rec.Code, rec.Body.String())
	}
}
