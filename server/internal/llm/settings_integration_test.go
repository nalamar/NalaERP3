package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/secretbox"
	"nalaerp3/internal/testutil"
)

func llmTestCompany(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	id := "llm-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "LLM-Test "+id); err != nil {
		t.Fatal(err)
	}
	return id
}

func testBox(t *testing.T, b byte) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{b}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestSettingsServiceSaveAndGet(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := llmTestCompany(t, env)
	svc := NewSettingsService(env.PG, testBox(t, 1))

	got, err := svc.Get(ctx, company)
	if err != nil || got.Eingerichtet || got.Aktiv || got.Anbieter != ProviderOpenAICompatible || got.TimeoutS != 300 || got.Tageslimit != 500 ||
		got.APIKeyGesetzt || !got.SchluesselEingerichtet {
		t.Fatalf("Standard: %+v %v", got, err)
	}
	// Anthropic aktiv mit Schluessel; Schluessel nie in der Antwort, nur verschluesselt gespeichert
	got, err = svc.Save(ctx, SettingsInput{Aktiv: true, Anbieter: "anthropic", APIKey: "sk-ant-geheim", TimeoutS: 120, Tageslimit: 50}, "", company)
	if err != nil || !got.Eingerichtet || !got.Aktiv || got.Anbieter != "anthropic" || !got.APIKeyGesetzt || !got.APIKeyLesbar ||
		got.TimeoutS != 120 || got.Tageslimit != 50 || got.UpdatedAt == nil {
		t.Fatalf("Speichern: %+v %v", got, err)
	}
	if raw, _ := json.Marshal(got); strings.Contains(string(raw), "sk-ant-geheim") {
		t.Fatalf("Schlüssel im JSON: %s", raw)
	}
	var stored []byte
	if err := env.PG.QueryRow(ctx, `SELECT api_key_enc FROM company_llm_settings WHERE company_id=$1`, company).Scan(&stored); err != nil ||
		bytes.Contains(stored, []byte("sk-ant-geheim")) || len(stored) < 29 {
		t.Fatalf("gespeichert: %v %d", err, len(stored))
	}
	// leerer Schluessel laesst den gespeicherten stehen
	if got, err = svc.Save(ctx, SettingsInput{Aktiv: true, Anbieter: "anthropic", Modell: "claude-sonnet-5-5"}, "", company); err != nil || !got.APIKeyGesetzt || got.Modell != "claude-sonnet-5-5" {
		t.Fatalf("Schlüssel behalten: %+v %v", got, err)
	}
	// Schluessel loeschen geht nur, wenn nicht aktiv
	if _, err := svc.Save(ctx, SettingsInput{Aktiv: true, Anbieter: "anthropic", APIKeyLoeschen: true}, "", company); err == nil || !strings.Contains(err.Error(), "fehlt: API-Schlüssel") {
		t.Fatalf("aktiv ohne Schlüssel: %v", err)
	}
	if got, err = svc.Save(ctx, SettingsInput{Anbieter: "anthropic", APIKeyLoeschen: true}, "", company); err != nil || got.APIKeyGesetzt || got.Aktiv {
		t.Fatalf("löschen: %+v %v", got, err)
	}
	// ungueltige Eingabe aendert nichts
	if _, err := svc.Save(ctx, SettingsInput{Anbieter: "gpt"}, "", company); err == nil {
		t.Fatal("ungültiger Anbieter angenommen")
	}
	if got, _ = svc.Get(ctx, company); got.Anbieter != "anthropic" {
		t.Fatalf("nach Fehler: %+v", got)
	}
	// anderer Schluessel: nicht lesbar, Client verweigert
	if _, err := svc.Save(ctx, SettingsInput{Aktiv: true, Anbieter: "mistral", APIKey: "mk"}, "", company); err != nil {
		t.Fatal(err)
	}
	other := NewSettingsService(env.PG, testBox(t, 2))
	if got, _ = other.Get(ctx, company); !got.APIKeyGesetzt || got.APIKeyLesbar {
		t.Fatalf("fremder Schlüssel: %+v", got)
	}
	if _, err := other.Client(ctx, company, nil); err == nil || !strings.Contains(err.Error(), "nicht lesbar") {
		t.Fatalf("Client mit fremdem Schlüssel: %v", err)
	}
	// ohne Schluessel eingerichtet: Speichern eines API-Schluessels scheitert mit Meldung
	nokey := NewSettingsService(env.PG, nil)
	if _, err := nokey.Save(ctx, SettingsInput{Anbieter: "mistral", APIKey: "x"}, "", company); err == nil || !strings.Contains(err.Error(), "SETTINGS_ENCRYPTION_KEY") {
		t.Fatalf("ohne Verschlüsselungsschlüssel: %v", err)
	}
	if got, _ = nokey.Get(ctx, company); got.SchluesselEingerichtet || got.APIKeyLesbar {
		t.Fatalf("ohne Verschlüsselungsschlüssel lesen: %+v", got)
	}
	// Mandanten getrennt
	second := llmTestCompany(t, env)
	if got, _ = svc.Get(ctx, second); got.Eingerichtet {
		t.Fatalf("anderer Mandant sieht Einstellung: %+v", got)
	}
	// ein in einen anderen Mandanten kopiertes Chiffrat ist dort nicht lesbar (Mandant als AAD)
	if _, err := svc.Save(ctx, SettingsInput{}, "", second); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE company_llm_settings SET api_key_enc=(SELECT api_key_enc FROM company_llm_settings WHERE company_id=$1) WHERE company_id=$2`, company, second); err != nil {
		t.Fatal(err)
	}
	if got, _ = svc.Get(ctx, second); !got.APIKeyGesetzt || got.APIKeyLesbar {
		t.Fatalf("kopiertes Chiffrat: %+v", got)
	}
	// Pruefungen der Datenbank
	for name, sql := range map[string]string{
		"Anbieter":         `UPDATE company_llm_settings SET anbieter='gpt' WHERE company_id=$1`,
		"Zeitlimit":        `UPDATE company_llm_settings SET timeout_s=1801 WHERE company_id=$1`,
		"Tageslimit":       `UPDATE company_llm_settings SET tageslimit=0 WHERE company_id=$1`,
		"Schlüssel kaputt": `UPDATE company_llm_settings SET api_key_enc='\x0201'::bytea WHERE company_id=$1`,
		"aktiv ohne Key":   `UPDATE company_llm_settings SET api_key_enc=NULL WHERE company_id=$1`,
		"aktiv OpenAI":     `UPDATE company_llm_settings SET anbieter='openai_compatible', endpunkt='' WHERE company_id=$1`,
	} {
		if _, err := env.PG.Exec(ctx, sql, company); err == nil {
			t.Fatalf("Datenbank nimmt %s an", name)
		}
	}
}

func TestSettingsServiceClientAndDailyLimit(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := llmTestCompany(t, env)
	svc := NewSettingsService(env.PG, testBox(t, 1))

	var calls int32
	var lastAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		lastAuth.Store(r.Header.Get("Authorization"))
		_, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, okResponse)
	}))
	defer srv.Close()

	// nicht eingeschaltet: kein Client
	if _, err := svc.Client(ctx, company, nil); err == nil || !strings.Contains(err.Error(), "nicht eingeschaltet") {
		t.Fatalf("aus: %v", err)
	}
	if _, err := svc.Save(ctx, SettingsInput{Aktiv: true, Endpunkt: srv.URL, Modell: "lokal", APIKey: "lk", TimeoutS: 7, Tageslimit: 2}, "", company); err != nil {
		t.Fatal(err)
	}
	rec := &memRecorder{}
	c, err := svc.Client(ctx, company, rec)
	if err != nil || c.Limits() != (Limits{Timeout: 7 * time.Second, MaxRetries: 2}) {
		t.Fatalf("Client: %v %+v", err, c)
	}
	for i := 0; i < 2; i++ {
		if _, err := c.Complete(ctx, Request{}); err != nil {
			t.Fatalf("Aufruf %d: %v", i+1, err)
		}
	}
	if lastAuth.Load() != "Bearer lk" || rec.records[0].Provider != "openai_compatible" {
		t.Fatalf("Anbindung: %v %+v", lastAuth.Load(), rec.records)
	}
	// dritter Aufruf: Tageslimit, keine Anfrage nach aussen
	if _, err := c.Complete(ctx, Request{}); err == nil || !strings.Contains(err.Error(), "Tageslimit des Sprachmodells erreicht (2 Aufrufe)") {
		t.Fatalf("Tageslimit: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 || rec.records[2].Outcome != OutcomeLimit {
		t.Fatalf("nach Tageslimit: %d %+v", calls, rec.records)
	}
	got, _ := svc.Get(ctx, company)
	if got.AufrufeHeute != 2 {
		t.Fatalf("Aufrufe heute: %+v", got)
	}
	// hoeheres Limit: weiter
	if _, err := svc.Save(ctx, SettingsInput{Aktiv: true, Endpunkt: srv.URL, Modell: "lokal", Tageslimit: 3}, "", company); err != nil {
		t.Fatal(err)
	}
	c, _ = svc.Client(ctx, company, rec)
	if _, err := c.Complete(ctx, Request{}); err != nil || atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("nach Erhöhung: %v %d", err, calls)
	}
	// Zaehler je Mandant
	second := llmTestCompany(t, env)
	if _, err := svc.Save(ctx, SettingsInput{Aktiv: true, Endpunkt: srv.URL, Modell: "lokal", Tageslimit: 1}, "", second); err != nil {
		t.Fatal(err)
	}
	c2, _ := svc.Client(ctx, second, rec)
	if _, err := c2.Complete(ctx, Request{}); err != nil {
		t.Fatalf("zweiter Mandant: %v", err)
	}
	if got2, _ := svc.Get(ctx, second); got2.AufrufeHeute != 1 {
		t.Fatalf("Zähler zweiter Mandant: %+v", got2)
	}
}
