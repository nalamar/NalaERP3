package llm

import (
	"math"
	"strings"
	"testing"
)

func TestNormalizeProfileInput(t *testing.T) {
	got := NormalizeProfileInput(ProfileInput{Name: "  Lokal ", Anbieter: " Mistral ", Modell: " m "})
	if got.Name != "Lokal" || got.Anbieter != "mistral" || got.Modell != "m" || got.TimeoutS != 300 || got.Schwelle == nil || *got.Schwelle != 0.9 {
		t.Fatalf("Normalize: %+v", got)
	}
	null := 0.0
	if got := NormalizeProfileInput(ProfileInput{Schwelle: &null, TimeoutS: 45}); *got.Schwelle != 0 || got.TimeoutS != 45 {
		t.Fatalf("eigene Werte: %+v", got)
	}
}

func TestValidateProfileInput(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	ok := func(in ProfileInput) ProfileInput { return NormalizeProfileInput(in) }
	for _, in := range []ProfileInput{
		ok(ProfileInput{Name: "Lokal"}),
		ok(ProfileInput{Name: strings.Repeat("ä", 100), Schwelle: f(0)}),
		ok(ProfileInput{Name: "x", Schwelle: f(1)}),
		ok(ProfileInput{Name: "x", Aktiv: true, Endpunkt: "http://localhost:11434/v1", Modell: "llama3"}),
	} {
		if err := ValidateProfileInput(in); err != nil {
			t.Fatalf("%+v: %v", in, err)
		}
	}
	for _, c := range []struct {
		in   ProfileInput
		want string
	}{
		{ok(ProfileInput{Name: "  "}), "Name des Profils ungültig"},
		{ok(ProfileInput{Name: strings.Repeat("ä", 101)}), "Name des Profils ungültig"},
		{ok(ProfileInput{Name: "a\tb"}), "Name des Profils ungültig"},
		{ok(ProfileInput{Name: "x", Schwelle: f(-0.001)}), "Schwelle ungültig"},
		{ok(ProfileInput{Name: "x", Schwelle: f(1.001)}), "Schwelle ungültig"},
		{ok(ProfileInput{Name: "x", Schwelle: f(math.NaN())}), "Schwelle ungültig"},
		{ProfileInput{Name: "x", Anbieter: ProviderOpenAICompatible, TimeoutS: 300}, "Schwelle ungültig"},
		{ok(ProfileInput{Name: "x", Anbieter: "gpt"}), "Anbieter ungültig"},
		{ok(ProfileInput{Name: "x", TimeoutS: 1801}), "Zeitlimit ungültig"},
		{ok(ProfileInput{Name: "x", Aktiv: true, Modell: "m"}), "fehlt: Endpunkt und Modell"},
	} {
		if err := ValidateProfileInput(c.in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%+v: %v", c.in, err)
		}
	}
}
