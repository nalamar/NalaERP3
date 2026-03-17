package pdfgen

import "testing"

func TestParseHexColorParsesUpperAndLowercaseValues(t *testing.T) {
	fallback := rgbColor{R: 1, G: 2, B: 3}
	got := parseHexColor("#1f4B99", fallback)
	if got != (rgbColor{R: 31, G: 75, B: 153}) {
		t.Fatalf("expected parsed color {31 75 153}, got %#v", got)
	}
}

func TestParseHexColorFallsBackForInvalidValues(t *testing.T) {
	fallback := rgbColor{R: 107, G: 114, B: 128}
	got := parseHexColor("not-a-color", fallback)
	if got != fallback {
		t.Fatalf("expected fallback %#v, got %#v", fallback, got)
	}
}

func TestTemplateColorsUseDefaultsWhenUnset(t *testing.T) {
	primary, accent := templateColors(TemplateOptions{})
	if primary != (rgbColor{R: 31, G: 75, B: 153}) {
		t.Fatalf("unexpected default primary color %#v", primary)
	}
	if accent != (rgbColor{R: 107, G: 114, B: 128}) {
		t.Fatalf("unexpected default accent color %#v", accent)
	}
}
