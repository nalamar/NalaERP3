package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeSettingsInput(t *testing.T) {
	got := NormalizeSettingsInput(SettingsInput{Anbieter: " Mistral ", Endpunkt: " http://x/v1 ", Modell: " m ", APIKey: " k "})
	want := SettingsInput{Anbieter: "mistral", Endpunkt: "http://x/v1", Modell: "m", APIKey: " k ", TimeoutS: 300, Tageslimit: 500}
	if got != want {
		t.Fatalf("Normalize: %+v", got)
	}
	if NormalizeSettingsInput(SettingsInput{}).Anbieter != ProviderOpenAICompatible {
		t.Fatal("Standardanbieter")
	}
	if got := NormalizeSettingsInput(SettingsInput{TimeoutS: 45, Tageslimit: 7}); got.TimeoutS != 45 || got.Tageslimit != 7 {
		t.Fatalf("eigene Werte: %+v", got)
	}
}

func TestValidateSettingsInput(t *testing.T) {
	ok := func(in SettingsInput) SettingsInput { return NormalizeSettingsInput(in) }
	for _, in := range []SettingsInput{
		ok(SettingsInput{}),
		ok(SettingsInput{Aktiv: true, Endpunkt: "http://localhost:11434/v1", Modell: "llama3"}),
		ok(SettingsInput{Aktiv: true, Anbieter: "anthropic"}),
		ok(SettingsInput{Anbieter: "mistral", Endpunkt: "https://api.mistral.ai/v1", TimeoutS: 1800, Tageslimit: 100000}),
		ok(SettingsInput{TimeoutS: 1, Tageslimit: 1, Modell: strings.Repeat("m", 200), APIKey: strings.Repeat("k", 1000)}),
	} {
		if err := ValidateSettingsInput(in); err != nil {
			t.Fatalf("%+v: %v", in, err)
		}
	}
	for name, c := range map[string]struct {
		in   SettingsInput
		want string
	}{
		"Anbieter":             {ok(SettingsInput{Anbieter: "openai"}), "Anbieter ungültig"},
		"Endpunkt ohne Schema": {ok(SettingsInput{Endpunkt: "localhost:11434"}), "Endpunkt ungültig"},
		"Endpunkt ftp":         {ok(SettingsInput{Endpunkt: "ftp://x/v1"}), "Endpunkt ungültig"},
		"Endpunkt ohne Host":   {ok(SettingsInput{Endpunkt: "http:///v1"}), "Endpunkt ungültig"},
		"Endpunkt mit Zugang":  {ok(SettingsInput{Endpunkt: "https://user:pw@x/v1"}), "Endpunkt ungültig"},
		"Endpunkt Leerraum":    {SettingsInput{Anbieter: "anthropic", Endpunkt: "http://x/ v1", TimeoutS: 1, Tageslimit: 1}, "Endpunkt ungültig"},
		"Endpunkt zu lang":     {ok(SettingsInput{Endpunkt: "http://x/" + strings.Repeat("a", 500)}), "Endpunkt ungültig"},
		"Modell Leerraum":      {ok(SettingsInput{Modell: "a b"}), "Modell ungültig"},
		"Modell zu lang":       {ok(SettingsInput{Modell: strings.Repeat("m", 201)}), "Modell ungültig"},
		"Schlüssel Umbruch":    {ok(SettingsInput{APIKey: "a\nb"}), "API-Schlüssel ungültig"},
		"Schlüssel zu lang":    {ok(SettingsInput{APIKey: strings.Repeat("k", 1001)}), "API-Schlüssel ungültig"},
		"setzen und löschen":   {ok(SettingsInput{APIKey: "k", APIKeyLoeschen: true}), "entweder neu setzen oder löschen"},
		"Zeitlimit hoch":       {ok(SettingsInput{TimeoutS: 1801}), "Zeitlimit ungültig"},
		"Zeitlimit negativ":    {ok(SettingsInput{TimeoutS: -1}), "Zeitlimit ungültig"},
		"Tageslimit hoch":      {ok(SettingsInput{Tageslimit: 100001}), "Tageslimit ungültig"},
		"Tageslimit negativ":   {ok(SettingsInput{Tageslimit: -5}), "Tageslimit ungültig"},
		"aktiv ohne Endpunkt":  {ok(SettingsInput{Aktiv: true, Modell: "m"}), "Endpunkt und Modell"},
		"aktiv ohne Modell":    {ok(SettingsInput{Aktiv: true, Endpunkt: "http://x/v1"}), "Endpunkt und Modell"},
	} {
		if err := ValidateSettingsInput(c.in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestNewProvider(t *testing.T) {
	oa, ok := newProvider(Settings{Anbieter: ProviderOpenAICompatible, Endpunkt: "http://l/v1", Modell: "m"}, "k").(*OpenAICompatible)
	if !ok || oa.BaseURL != "http://l/v1" || oa.Model != "m" || oa.APIKey != "k" || oa.Name() != "openai_compatible" {
		t.Fatalf("openai: %+v", oa)
	}
	an, ok := newProvider(Settings{Anbieter: ProviderAnthropic, Endpunkt: "http://a", Modell: "claude-sonnet-5-5"}, "ak").(*Anthropic)
	if !ok || an.BaseURL != "http://a" || an.Model != "claude-sonnet-5-5" || an.APIKey != "ak" {
		t.Fatalf("anthropic: %+v", an)
	}
	mi, ok := newProvider(Settings{Anbieter: ProviderMistral}, "mk").(*OpenAICompatible)
	if !ok || mi.Name() != "mistral" || mi.BaseURL != MistralBaseURL || mi.Model != DefaultMistralModel || mi.APIKey != "mk" || !mi.RequireAPIKey {
		t.Fatalf("mistral: %+v", mi)
	}
	mi2, _ := newProvider(Settings{Anbieter: ProviderMistral, Endpunkt: "http://eigener/v1"}, "mk").(*OpenAICompatible)
	if mi2.BaseURL != "http://eigener/v1" {
		t.Fatalf("mistral eigener Endpunkt: %+v", mi2)
	}
}

func TestClientBudget(t *testing.T) {
	// Budget verweigert: keine Anfrage, Ergebnis "limit"
	f := &Fake{}
	c, rec, _ := newTestClient(f, Limits{MaxRetries: 2})
	c.WithBudget(func(context.Context) error { return errors.New("Tageslimit erreicht") })
	if _, err := c.Complete(context.Background(), Request{Purpose: "p"}); err == nil || !strings.Contains(err.Error(), "Tageslimit") ||
		len(f.Requests) != 0 || rec.records[0].Outcome != OutcomeLimit || rec.records[0].Attempts != 0 || rec.records[0].Purpose != "p" {
		t.Fatalf("Budget verweigert: %v %d %+v", err, len(f.Requests), rec.records)
	}
	// Budget einmal je Anfrage, auch bei Wiederholungen
	calls := 0
	f = &Fake{Respond: func(_ context.Context, call int, _ Request) (Response, error) {
		if call < 3 {
			return Response{}, &TransientError{Err: errors.New("503")}
		}
		return Response{Text: "ok"}, nil
	}}
	c, _, _ = newTestClient(f, Limits{MaxRetries: 2})
	c.WithBudget(func(context.Context) error { calls++; return nil })
	if _, err := c.Complete(context.Background(), Request{}); err != nil || calls != 1 || len(f.Requests) != 3 {
		t.Fatalf("Budget je Anfrage: %v %d %d", err, calls, len(f.Requests))
	}
}
