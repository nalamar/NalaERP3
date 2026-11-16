package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type anthropicCapture struct {
	path, apiKey, version string
	body                  map[string]any
}

func anthropicServer(t *testing.T, status int, response string) (*httptest.Server, *[]anthropicCapture, *int32) {
	t.Helper()
	var got []anthropicCapture
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		got = append(got, anthropicCapture{path: r.URL.Path, apiKey: r.Header.Get("x-api-key"), version: r.Header.Get("anthropic-version"), body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return srv, &got, &calls
}

const anthropicOK = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
	"content":[{"type":"thinking","thinking":"","signature":"x"},{"type":"text","text":"{\"code\":"},{"type":"text","text":"\"FE\"}"}],
	"stop_reason":"end_turn","usage":{"input_tokens":30,"output_tokens":9}}`

func anthropicError(kind string) string {
	return `{"type":"error","error":{"type":"` + kind + `","message":"sk-ant-geheim abgelehnt"}}`
}

func TestAnthropicRequestAndResponse(t *testing.T) {
	srv, got, _ := anthropicServer(t, http.StatusOK, anthropicOK)
	p := &Anthropic{BaseURL: srv.URL, APIKey: "sk-ant-geheim"}
	schema := map[string]any{
		"type":     "object",
		"required": []any{"code"},
		"properties": map[string]any{
			"code":  map[string]any{"type": "string", "maxLength": 20.0},
			"score": map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
		},
	}
	resp, err := p.Complete(context.Background(), Request{
		System:      "Ordne zu.",
		Messages:    []Message{{Role: "user", Content: "Fenster"}, {Role: "assistant", Content: "?"}, {Role: "user", Content: "Alu"}},
		JSONSchema:  schema,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Textbloecke zusammengesetzt, Denkblock ignoriert
	if resp != (Response{Text: `{"code":"FE"}`, Model: "claude-opus-5-5", Usage: Usage{InputTokens: 30, OutputTokens: 9}}) {
		t.Fatalf("Antwort: %+v", resp)
	}
	r := (*got)[0]
	if r.path != "/v1/messages" || r.apiKey != "sk-ant-geheim" || r.version == "" {
		t.Fatalf("Anfrage: %+v", r)
	}
	b := r.body
	if b["model"] != DefaultAnthropicModel || b["max_tokens"] != 16000.0 {
		t.Fatalf("Modell/Höchstlänge: %v %v", b["model"], b["max_tokens"])
	}
	if _, hasTemp := b["temperature"]; hasTemp {
		t.Fatal("temperature gesendet")
	}
	system, _ := b["system"].([]any)
	sys0, _ := system[0].(map[string]any)
	msgs, _ := b["messages"].([]any)
	m1, _ := msgs[1].(map[string]any)
	m2, _ := msgs[2].(map[string]any)
	content, _ := m2["content"].([]any)
	c0, _ := content[0].(map[string]any)
	if len(system) != 1 || sys0["text"] != "Ordne zu." || len(msgs) != 3 || m1["role"] != "assistant" || m2["role"] != "user" || c0["text"] != "Alu" {
		t.Fatalf("Nachrichten: %+v / %+v", b["system"], b["messages"])
	}
	oc, _ := b["output_config"].(map[string]any)
	format, _ := oc["format"].(map[string]any)
	sent, _ := format["schema"].(map[string]any)
	props, _ := sent["properties"].(map[string]any)
	score, _ := props["score"].(map[string]any)
	code, _ := props["code"].(map[string]any)
	if format["type"] != "json_schema" || sent["additionalProperties"] != false || score["type"] != "number" || score["minimum"] != nil ||
		score["maximum"] != nil || code["maxLength"] != nil {
		t.Fatalf("Schema: %+v", format)
	}
	// das Original bleibt unveraendert (der Client prueft dagegen)
	if (schema["properties"].(map[string]any))["score"].(map[string]any)["minimum"] != 0.0 {
		t.Fatal("Original-Schema verändert")
	}
}

func TestAnthropicOptionalParts(t *testing.T) {
	srv, got, _ := anthropicServer(t, http.StatusOK, anthropicOK)
	p := &Anthropic{BaseURL: srv.URL, APIKey: "k", Model: " claude-sonnet-5-5 "}
	if _, err := p.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "x"}}, MaxTokens: 300}); err != nil {
		t.Fatal(err)
	}
	b := (*got)[0].body
	_, hasSystem := b["system"]
	_, hasOC := b["output_config"]
	if b["model"] != "claude-sonnet-5-5" || b["max_tokens"] != 300.0 || hasSystem || hasOC {
		t.Fatalf("ohne Optionen: %+v", b)
	}
	if p.Name() != "anthropic" {
		t.Fatal(p.Name())
	}
}

func TestAnthropicErrors(t *testing.T) {
	for _, c := range []struct {
		name      string
		status    int
		body      string
		transient bool
		want      string
	}{
		{"429", 429, anthropicError("rate_limit_error"), true, "Status 429 (rate_limit_error)"},
		{"529", 529, anthropicError("overloaded_error"), true, "Status 529 (overloaded_error)"},
		{"500", 500, anthropicError("api_error"), true, "Status 500"},
		{"401", 401, anthropicError("authentication_error"), false, "Status 401 (authentication_error)"},
		{"400", 400, anthropicError("invalid_request_error"), false, "Status 400"},
		{"Ablehnung", 200, `{"id":"m","type":"message","role":"assistant","model":"m","content":[],"stop_reason":"refusal","usage":{"input_tokens":1,"output_tokens":0}}`, false, "abgelehnt"},
		{"ohne Text", 200, `{"id":"m","type":"message","role":"assistant","model":"m","content":[{"type":"thinking","thinking":"","signature":"s"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":0}}`, false, "ohne Inhalt"},
	} {
		srv, _, calls := anthropicServer(t, c.status, c.body)
		_, err := (&Anthropic{BaseURL: srv.URL, APIKey: "sk-ant-geheim"}).Complete(context.Background(), Request{})
		var te *TransientError
		if err == nil || errors.As(err, &te) != c.transient || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "sk-ant-geheim") {
			t.Fatalf("%s: %v (vorübergehend %v)", c.name, err, errors.As(err, &te))
		}
		// das SDK wiederholt nicht selbst
		if atomic.LoadInt32(calls) != 1 {
			t.Fatalf("%s: %d Aufrufe", c.name, *calls)
		}
	}
	// Netzfehler: voruebergehend, ohne Schluessel in der Meldung
	srv, _, _ := anthropicServer(t, 200, anthropicOK)
	url := srv.URL
	srv.Close()
	_, err := (&Anthropic{BaseURL: url, APIKey: "sk-ant-geheim"}).Complete(context.Background(), Request{})
	var te *TransientError
	if !errors.As(err, &te) || strings.Contains(err.Error(), "sk-ant-geheim") {
		t.Fatalf("Netzfehler: %v", err)
	}
	// ohne Schluessel
	if _, err := (&Anthropic{BaseURL: url}).Complete(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "API-Schlüssel") {
		t.Fatalf("ohne Schlüssel: %v", err)
	}
}

func TestAnthropicThroughClient(t *testing.T) {
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
	c := NewClient(&Anthropic{BaseURL: srv.URL, APIKey: "k"}, Limits{Timeout: 50 * time.Millisecond, MaxRetries: 2}, rec)
	if _, err := c.Complete(context.Background(), Request{}); !errors.Is(err, ErrTimeout) || rec.records[0].Attempts != 1 {
		t.Fatalf("Zeitlimit: %v %+v", err, rec.records)
	}
	// Schema-Minimum wird vom Client geprueft, obwohl es nicht gesendet wurde
	srv2, _, _ := anthropicServer(t, 200, `{"id":"m","type":"message","role":"assistant","model":"m","content":[{"type":"text","text":"{\"score\":2}"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	c = NewClient(&Anthropic{BaseURL: srv2.URL, APIKey: "k"}, Limits{}, rec)
	schema := map[string]any{"type": "object", "properties": map[string]any{"score": map[string]any{"type": "number", "maximum": 1.0}}}
	if _, err := c.Complete(context.Background(), Request{JSONSchema: schema}); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("Maximum: %v", err)
	}
}

func TestAPISchema(t *testing.T) {
	in := map[string]any{
		"type":     "object",
		"required": []any{"maximum"},
		"properties": map[string]any{
			// Feldname "maximum" bleibt, sein Schluesselwort minimum faellt weg
			"maximum": map[string]any{"type": "integer", "minimum": 1.0},
			"liste":   map[string]any{"type": "array", "minItems": 1.0, "items": map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string", "minLength": 1.0}}}},
			"wahl":    map[string]any{"anyOf": []any{map[string]any{"type": "string", "maxLength": 3.0}, map[string]any{"type": "null"}}},
		},
		"$defs": map[string]any{"teil": map[string]any{"type": "object", "properties": map[string]any{}}},
	}
	want := map[string]any{
		"type":                 "object",
		"required":             []any{"maximum"},
		"additionalProperties": false,
		"properties": map[string]any{
			"maximum": map[string]any{"type": "integer"},
			"liste":   map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"x": map[string]any{"type": "string"}}}},
			"wahl":    map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}},
		},
		"$defs": map[string]any{"teil": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}}},
	}
	if got := apiSchema(in); !reflect.DeepEqual(got, want) {
		gj, _ := json.Marshal(got)
		t.Fatalf("apiSchema: %s", gj)
	}
}
