package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type capturedRequest struct {
	path, auth, contentType string
	body                    map[string]any
}

func openAIServer(t *testing.T, status int, response string) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var got []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		got = append(got, capturedRequest{path: r.URL.Path, auth: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type"), body: body})
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

const okResponse = `{"model":"modell-1","choices":[{"message":{"role":"assistant","content":"{\"code\":\"FE\",\"score\":0.8}"}}],"usage":{"prompt_tokens":42,"completion_tokens":7}}`

func TestOpenAICompatibleRequestAndResponse(t *testing.T) {
	srv, got := openAIServer(t, http.StatusOK, okResponse)
	p := &OpenAICompatible{BaseURL: srv.URL + "/v1/", APIKey: "sk-geheim", Model: "modell-1"}
	resp, err := p.Complete(context.Background(), Request{
		System:      "Ordne zu.",
		Messages:    []Message{{Role: "user", Content: "Fenster"}, {Role: "assistant", Content: "?"}, {Role: "user", Content: "Alu"}},
		JSONSchema:  map[string]any{"type": "object"},
		MaxTokens:   200,
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp != (Response{Text: `{"code":"FE","score":0.8}`, Model: "modell-1", Usage: Usage{InputTokens: 42, OutputTokens: 7}}) {
		t.Fatalf("Antwort: %+v", resp)
	}
	r := (*got)[0]
	if r.path != "/v1/chat/completions" || r.auth != "Bearer sk-geheim" || r.contentType != "application/json" {
		t.Fatalf("Anfrage: %+v", r)
	}
	msgs, _ := r.body["messages"].([]any)
	first, _ := msgs[0].(map[string]any)
	last, _ := msgs[3].(map[string]any)
	if r.body["model"] != "modell-1" || r.body["max_tokens"] != 200.0 || r.body["temperature"] != 0.2 || len(msgs) != 4 ||
		first["role"] != "system" || first["content"] != "Ordne zu." || last["role"] != "user" || last["content"] != "Alu" {
		t.Fatalf("Body: %+v", r.body)
	}
	rf, _ := r.body["response_format"].(map[string]any)
	js, _ := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["name"] != "antwort" || js["schema"] == nil {
		t.Fatalf("response_format: %+v", rf)
	}
}

func TestOpenAICompatibleOptionalParts(t *testing.T) {
	srv, got := openAIServer(t, http.StatusOK, okResponse)
	// ohne Schluessel, Systemtext, Schema und Hoechstlaenge
	p := &OpenAICompatible{BaseURL: srv.URL, Model: "lokal"}
	if _, err := p.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "x"}}}); err != nil {
		t.Fatal(err)
	}
	r := (*got)[0]
	msgs, _ := r.body["messages"].([]any)
	_, hasMax := r.body["max_tokens"]
	_, hasFormat := r.body["response_format"]
	if r.path != "/chat/completions" || r.auth != "" || len(msgs) != 1 || hasMax || hasFormat {
		t.Fatalf("ohne Optionen: %+v", r)
	}
	// json_object fuer Dienste ohne Schema
	p.SchemaMode = "json_object"
	if _, err := p.Complete(context.Background(), Request{JSONSchema: map[string]any{"type": "object"}}); err != nil {
		t.Fatal(err)
	}
	rf, _ := (*got)[1].body["response_format"].(map[string]any)
	if rf["type"] != "json_object" || rf["json_schema"] != nil {
		t.Fatalf("json_object: %+v", rf)
	}
	if p.Name() != "openai_compatible" {
		t.Fatal(p.Name())
	}
}

func TestOpenAICompatibleErrors(t *testing.T) {
	for _, c := range []struct {
		name      string
		status    int
		body      string
		transient bool
		want      string
	}{
		{"429", http.StatusTooManyRequests, `{"error":"rate limit"}`, true, "Status 429"},
		{"503", http.StatusServiceUnavailable, `overloaded`, true, "Status 503"},
		{"500", http.StatusInternalServerError, `kaputt`, true, "Status 500"},
		{"401 mit Schluessel im Text", http.StatusUnauthorized, `invalid key sk-geheim`, false, "Status 401: invalid key ***"},
		{"400", http.StatusBadRequest, strings.Repeat("ä", 400), false, strings.Repeat("ä", 300) + "…"},
		{"kein JSON", http.StatusOK, `<html>`, false, "nicht lesbar"},
		{"keine Auswahl", http.StatusOK, `{"choices":[]}`, false, "ohne Inhalt"},
		{"Inhalt null", http.StatusOK, `{"choices":[{"message":{"content":null}}]}`, false, "ohne Inhalt"},
	} {
		srv, _ := openAIServer(t, c.status, c.body)
		p := &OpenAICompatible{BaseURL: srv.URL, APIKey: "sk-geheim", Model: "m"}
		_, err := p.Complete(context.Background(), Request{})
		var te *TransientError
		if err == nil || errors.As(err, &te) != c.transient || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "sk-geheim") {
			t.Fatalf("%s: %v (vorübergehend %v)", c.name, err, errors.As(err, &te))
		}
		if c.name == "400" && strings.Contains(err.Error(), strings.Repeat("ä", 301)) {
			t.Fatalf("400: Meldung nicht gekürzt")
		}
	}
	// Netzfehler: voruebergehend
	srv, _ := openAIServer(t, http.StatusOK, okResponse)
	url := srv.URL
	srv.Close()
	_, err := (&OpenAICompatible{BaseURL: url, Model: "m"}).Complete(context.Background(), Request{})
	var te *TransientError
	if !errors.As(err, &te) {
		t.Fatalf("Netzfehler: %v", err)
	}
	// unvollstaendig eingerichtet
	for _, p := range []*OpenAICompatible{{Model: "m"}, {BaseURL: " ", Model: "m"}, {BaseURL: "http://x"}} {
		if _, err := p.Complete(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "nicht vollständig eingerichtet") {
			t.Fatalf("Einrichtung %+v: %v", p, err)
		}
	}
}

func TestOpenAICompatibleThroughClient(t *testing.T) {
	// Zeitlimit des Clients greift auch fuer den echten HTTP-Aufruf
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(block)
	rec := &memRecorder{}
	c := NewClient(&OpenAICompatible{BaseURL: srv.URL, Model: "m"}, Limits{Timeout: 50 * time.Millisecond, MaxRetries: 2}, rec)
	if _, err := c.Complete(context.Background(), Request{}); !errors.Is(err, ErrTimeout) || rec.records[0].Outcome != OutcomeTimeout || rec.records[0].Attempts != 1 {
		t.Fatalf("Zeitlimit: %v %+v", err, rec.records)
	}
	// 503 zweimal, dann ok: drei Versuche
	calls := 0
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, okResponse)
	}))
	defer srv2.Close()
	c = NewClient(&OpenAICompatible{BaseURL: srv2.URL, Model: "m"}, Limits{MaxRetries: 2}, rec)
	c.sleep = func(time.Duration) {}
	resp, err := c.Complete(context.Background(), Request{JSONSchema: map[string]any{"type": "object", "required": []any{"code"}}})
	if err != nil || calls != 3 || resp.Usage.InputTokens != 42 {
		t.Fatalf("Wiederholung: %v %d %+v", err, calls, resp)
	}
}

func TestMistral(t *testing.T) {
	p := NewMistral("mk-geheim", "")
	if p.BaseURL != "https://api.mistral.ai/v1" || p.Model != "mistral-large-latest" || p.Name() != "mistral" || !p.RequireAPIKey {
		t.Fatalf("NewMistral: %+v", p)
	}
	if q := NewMistral("k", " ministral-8b-latest "); q.Model != " ministral-8b-latest " {
		t.Fatalf("eigenes Modell: %+v", q)
	}
	// gleiche Schnittstelle: gegen den Testserver
	srv, got := openAIServer(t, http.StatusOK, okResponse)
	p.BaseURL = srv.URL + "/v1"
	if _, err := p.Complete(context.Background(), Request{JSONSchema: map[string]any{"type": "object"}}); err != nil {
		t.Fatal(err)
	}
	r := (*got)[0]
	rf, _ := r.body["response_format"].(map[string]any)
	if r.path != "/v1/chat/completions" || r.auth != "Bearer mk-geheim" || r.body["model"] != "mistral-large-latest" || rf["type"] != "json_schema" {
		t.Fatalf("Mistral-Anfrage: %+v", r)
	}
	// ohne Schluessel: Fehler vor dem Aufruf
	for _, key := range []string{"", "  "} {
		m := NewMistral(key, "")
		m.BaseURL = srv.URL
		if _, err := m.Complete(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "API-Schlüssel") {
			t.Fatalf("ohne Schlüssel %q: %v", key, err)
		}
	}
	if len(*got) != 1 {
		t.Fatalf("%d Aufrufe", len(*got))
	}
	// Protokoll unter dem Namen "mistral"
	rec := &memRecorder{}
	if _, err := NewClient(p, Limits{}, rec).Complete(context.Background(), Request{}); err != nil || rec.records[0].Provider != "mistral" {
		t.Fatalf("Protokoll: %v %+v", err, rec.records)
	}
	// ohne ProviderName bleibt der allgemeine Name, ohne RequireAPIKey geht es ohne Schluessel
	plain := &OpenAICompatible{BaseURL: srv.URL, Model: "m"}
	if _, err := plain.Complete(context.Background(), Request{}); err != nil || plain.Name() != "openai_compatible" {
		t.Fatalf("allgemein: %v %s", err, plain.Name())
	}
}
