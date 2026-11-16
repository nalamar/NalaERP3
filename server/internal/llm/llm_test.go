package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

type memRecorder struct{ records []CallRecord }

func (m *memRecorder) Record(_ context.Context, r CallRecord) { m.records = append(m.records, r) }

func newTestClient(f *Fake, limits Limits) (*Client, *memRecorder, *[]time.Duration) {
	rec := &memRecorder{}
	c := NewClient(f, limits, rec)
	var sleeps []time.Duration
	c.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	clock := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	c.now = func() time.Time { clock = clock.Add(250 * time.Millisecond); return clock }
	return c, rec, &sleeps
}

var schema = map[string]any{
	"type":                 "object",
	"required":             []any{"code", "score"},
	"additionalProperties": false,
	"properties": map[string]any{
		"code":   map[string]any{"type": "string"},
		"score":  map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
		"stufe":  map[string]any{"type": "string", "enum": []any{"los", "titel"}},
		"anzahl": map[string]any{"type": "integer"},
		"tags":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	},
}

func TestLimitsDefaultsAndBounds(t *testing.T) {
	for _, c := range []struct {
		in   Limits
		want Limits
	}{
		{Limits{}, Limits{Timeout: 300 * time.Second, MaxRetries: 0}},
		{Limits{Timeout: 45 * time.Second, MaxRetries: 1}, Limits{Timeout: 45 * time.Second, MaxRetries: 1}},
		{Limits{Timeout: -1, MaxRetries: -3}, Limits{Timeout: 300 * time.Second, MaxRetries: 0}},
		{Limits{Timeout: time.Hour, MaxRetries: 9}, Limits{Timeout: 30 * time.Minute, MaxRetries: 2}},
	} {
		if got := NewClient(&Fake{}, c.in, nil).Limits(); got != c.want {
			t.Fatalf("%+v: %+v, erwartet %+v", c.in, got, c.want)
		}
	}
	if DefaultTimeout != 300*time.Second || DefaultMaxRetries != 2 {
		t.Fatal("Standardwerte aus ADR 0043")
	}
}

func TestCompleteOKRecordsWithoutContent(t *testing.T) {
	f := &Fake{Respond: func(_ context.Context, _ int, _ Request) (Response, error) {
		return Response{Text: `{"code":"FE-ALU","score":0.9}`, Model: "m1", Usage: Usage{InputTokens: 120, OutputTokens: 15}}, nil
	}}
	c, rec, sleeps := newTestClient(f, Limits{MaxRetries: 2})
	req := Request{System: "Geheimer Systemtext", Messages: []Message{{Role: "user", Content: "Fenster Alu 1-flg."}}, JSONSchema: schema, Purpose: "matching"}
	resp, err := c.Complete(context.Background(), req)
	if err != nil || resp.Text != `{"code":"FE-ALU","score":0.9}` || len(f.Requests) != 1 || len(*sleeps) != 0 {
		t.Fatalf("Antwort: %+v %v", resp, err)
	}
	if len(rec.records) != 1 {
		t.Fatalf("Protokoll: %+v", rec.records)
	}
	r := rec.records[0]
	want := CallRecord{Provider: "fake", Model: "m1", Purpose: "matching", Outcome: OutcomeOK, Attempts: 1, Duration: 250 * time.Millisecond,
		Usage: Usage{InputTokens: 120, OutputTokens: 15}, InputHash: InputHash(req)}
	if r != want {
		t.Fatalf("Protokoll: %+v, erwartet %+v", r, want)
	}
	// das Log enthaelt keine Inhalte
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	LogRecorder{}.Record(context.Background(), r)
	log.SetOutput(prev)
	line := buf.String()
	if strings.Contains(line, "Fenster") || strings.Contains(line, "Geheimer") || !strings.Contains(line, "outcome=ok") ||
		!strings.Contains(line, "input_hash="+r.InputHash) || !strings.Contains(line, "purpose=matching") || !strings.Contains(line, "input_tokens=120") {
		t.Fatalf("Log: %q", line)
	}
}

func TestCompleteRetriesTransientErrors(t *testing.T) {
	// zweimal voruebergehend, dann ok: 3 Versuche, Verbrauch summiert
	f := &Fake{Respond: func(_ context.Context, call int, _ Request) (Response, error) {
		if call < 3 {
			return Response{}, &TransientError{Err: errors.New("503")}
		}
		return Response{Text: "frei", Model: "m2", Usage: Usage{InputTokens: 10, OutputTokens: 2}}, nil
	}}
	c, rec, sleeps := newTestClient(f, Limits{MaxRetries: 2})
	resp, err := c.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "x"}}})
	if err != nil || resp.Text != "frei" || len(f.Requests) != 3 || rec.records[0].Attempts != 3 || rec.records[0].Outcome != OutcomeOK {
		t.Fatalf("Wiederholung: %+v %v %+v", resp, err, rec.records)
	}
	if len(*sleeps) != 2 || (*sleeps)[0] != time.Second || (*sleeps)[1] != 2*time.Second {
		t.Fatalf("Wartezeiten: %v", *sleeps)
	}
	// immer voruebergehend: nach 3 Versuchen Schluss
	f = &Fake{Respond: func(context.Context, int, Request) (Response, error) {
		return Response{}, &TransientError{Err: errors.New("überlastet")}
	}}
	c, rec, _ = newTestClient(f, Limits{MaxRetries: 2})
	_, err = c.Complete(context.Background(), Request{})
	var te *TransientError
	if !errors.As(err, &te) || len(f.Requests) != 3 || rec.records[0].Outcome != OutcomeError || rec.records[0].Attempts != 3 {
		t.Fatalf("zu viele Fehler: %v %d %+v", err, len(f.Requests), rec.records)
	}
	// ohne Wiederholungen: ein Versuch
	f = &Fake{Respond: func(context.Context, int, Request) (Response, error) {
		return Response{}, &TransientError{Err: errors.New("503")}
	}}
	c, _, _ = newTestClient(f, Limits{MaxRetries: 0})
	if _, err := c.Complete(context.Background(), Request{}); err == nil || len(f.Requests) != 1 {
		t.Fatalf("MaxRetries 0: %v %d", err, len(f.Requests))
	}
	// dauerhafter Fehler: keine Wiederholung
	f = &Fake{Respond: func(context.Context, int, Request) (Response, error) {
		return Response{}, errors.New("401 Schlüssel ungültig")
	}}
	c, rec, sleeps = newTestClient(f, Limits{MaxRetries: 2})
	if _, err := c.Complete(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "401") || len(f.Requests) != 1 ||
		rec.records[0].Outcome != OutcomeError || len(*sleeps) != 0 {
		t.Fatalf("dauerhafter Fehler: %v %d", err, len(f.Requests))
	}
}

func TestCompleteTimeout(t *testing.T) {
	f := &Fake{Respond: func(ctx context.Context, _ int, _ Request) (Response, error) {
		<-ctx.Done()
		return Response{}, &TransientError{Err: ctx.Err()}
	}}
	c, rec, _ := newTestClient(f, Limits{Timeout: 20 * time.Millisecond, MaxRetries: 2})
	_, err := c.Complete(context.Background(), Request{})
	if !errors.Is(err, ErrTimeout) || len(f.Requests) != 1 || rec.records[0].Outcome != OutcomeTimeout {
		t.Fatalf("Zeitlimit: %v %d %+v", err, len(f.Requests), rec.records)
	}
	// abgebrochener Aufrufer: kein Zeitlimit-Fehler, keine Wiederholung
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f = &Fake{Respond: func(ctx context.Context, _ int, _ Request) (Response, error) {
		return Response{}, &TransientError{Err: ctx.Err()}
	}}
	c, rec, _ = newTestClient(f, Limits{Timeout: time.Second, MaxRetries: 2})
	if _, err := c.Complete(ctx, Request{}); errors.Is(err, ErrTimeout) || len(f.Requests) != 1 || rec.records[0].Outcome != OutcomeError {
		t.Fatalf("Abbruch: %v %d %+v", err, len(f.Requests), rec.records)
	}
}

func TestCompleteValidatesJSON(t *testing.T) {
	for name, text := range map[string]string{
		"kein JSON":        `Der Code ist FE-ALU`,
		"Feld fehlt":       `{"code":"FE-ALU"}`,
		"falscher Typ":     `{"code":1,"score":0.5}`,
		"Score zu hoch":    `{"code":"X","score":1.5}`,
		"Score negativ":    `{"code":"X","score":-0.1}`,
		"Enum":             `{"code":"X","score":0.5,"stufe":"kapitel"}`,
		"keine ganze Zahl": `{"code":"X","score":0.5,"anzahl":1.5}`,
		"Array-Element":    `{"code":"X","score":0.5,"tags":["a",2]}`,
		"Zusatzfeld":       `{"code":"X","score":0.5,"preis":100}`,
		"kein Objekt":      `["X"]`,
	} {
		f := &Fake{Respond: func(context.Context, int, Request) (Response, error) {
			return Response{Text: text, Model: "m"}, nil
		}}
		c, rec, _ := newTestClient(f, Limits{MaxRetries: 2})
		if _, err := c.Complete(context.Background(), Request{JSONSchema: schema}); !errors.Is(err, ErrInvalidResponse) ||
			len(f.Requests) != 1 || rec.records[0].Outcome != OutcomeInvalidResponse {
			t.Fatalf("%s: %v %d %+v", name, err, len(f.Requests), rec.records)
		}
	}
	ok := `{"code":"X","score":1,"stufe":"titel","anzahl":3,"tags":["a","b"]}`
	f := &Fake{Respond: func(context.Context, int, Request) (Response, error) { return Response{Text: ok}, nil }}
	c, _, _ := newTestClient(f, Limits{})
	if resp, err := c.Complete(context.Background(), Request{JSONSchema: schema}); err != nil || resp.Text != ok {
		t.Fatalf("gültig: %v", err)
	}
	// ohne Schema: freier Text
	f = &Fake{Respond: func(context.Context, int, Request) (Response, error) { return Response{Text: "kein JSON"}, nil }}
	c, _, _ = newTestClient(f, Limits{})
	if _, err := c.Complete(context.Background(), Request{}); err != nil {
		t.Fatalf("freier Text: %v", err)
	}
}

func TestValidateJSONTypes(t *testing.T) {
	decode := func(s string) any {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, c := range []struct {
		value, typ string
		ok         bool
	}{
		{`true`, "boolean", true}, {`1`, "boolean", false},
		{`null`, "null", true}, {`0`, "null", false},
		{`2`, "integer", true}, {`2.5`, "integer", false}, {`"2"`, "integer", false},
		{`2.5`, "number", true}, {`"x"`, "number", false},
		{`[]`, "array", true}, {`{}`, "array", false},
		{`{}`, "object", true}, {`"x"`, "string", true}, {`1`, "string", false},
		{`1`, "unbekannt", true},
	} {
		err := ValidateJSON(decode(c.value), map[string]any{"type": c.typ})
		if (err == nil) != c.ok {
			t.Fatalf("%s als %s: %v", c.value, c.typ, err)
		}
	}
	// Zusatzfelder ohne additionalProperties: erlaubt; Fehlerpfad nennt das Feld
	if err := ValidateJSON(decode(`{"a":1,"x":2}`), map[string]any{"properties": map[string]any{"a": map[string]any{"type": "number"}}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(decode(`{"a":{"b":"x"}}`), map[string]any{"properties": map[string]any{"a": map[string]any{"properties": map[string]any{"b": map[string]any{"type": "number"}}}}}); err == nil || !strings.Contains(err.Error(), "$.a.b") {
		t.Fatalf("Pfad: %v", err)
	}
}

func TestInputHash(t *testing.T) {
	a := Request{System: "s", Messages: []Message{{Role: "user", Content: "ab"}}}
	b := Request{System: "s", Messages: []Message{{Role: "user", Content: "ab"}}, Purpose: "anders", MaxTokens: 9}
	c := Request{System: "sa", Messages: []Message{{Role: "user", Content: "b"}}}
	d := Request{System: "s", Messages: []Message{{Role: "assistant", Content: "ab"}}}
	e := Request{System: "t", Messages: []Message{{Role: "user", Content: "ab"}}}
	// ohne Laengen waeren eine Nachricht "xusery" und zwei Nachrichten "x", "y" gleich
	f := Request{Messages: []Message{{Role: "user", Content: "xusery"}}}
	g := Request{Messages: []Message{{Role: "user", Content: "x"}, {Role: "user", Content: "y"}}}
	if InputHash(a) == InputHash(e) || InputHash(f) == InputHash(g) {
		t.Fatal("Systemtext bzw. Nachrichtengrenzen gehen nicht in die Prüfsumme ein")
	}
	if InputHash(a) != InputHash(b) || InputHash(a) == InputHash(c) || InputHash(a) == InputHash(d) || len(InputHash(a)) != 16 {
		t.Fatalf("Prüfsumme: %s %s %s %s", InputHash(a), InputHash(b), InputHash(c), InputHash(d))
	}
}

func TestFakeDefaultAndNewClientRecorder(t *testing.T) {
	f := &Fake{}
	resp, err := f.Complete(context.Background(), Request{Purpose: "p"})
	if err != nil || resp.Text != "{}" || len(f.Requests) != 1 || f.Name() != "fake" {
		t.Fatalf("Fake: %+v %v", resp, err)
	}
	if _, ok := NewClient(f, Limits{}, nil).recorder.(LogRecorder); !ok {
		t.Fatal("Standard-Recorder")
	}
}
