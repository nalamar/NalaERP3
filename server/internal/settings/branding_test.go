package settings

import "testing"

func TestNormalizeHexColorAddsHashAndUppercases(t *testing.T) {
	got := normalizeHexColor("1f4b99", "#000000")
	if got != "#1F4B99" {
		t.Fatalf("expected #1F4B99, got %q", got)
	}
}

func TestNormalizeHexColorFallsBackForInvalidValue(t *testing.T) {
	got := normalizeHexColor("blue", "#ABCDEF")
	if got != "#ABCDEF" {
		t.Fatalf("expected fallback #ABCDEF, got %q", got)
	}
}

func TestApplyBrandingDefaultsUsesDocumentFallbacks(t *testing.T) {
	tmpl := PDFTemplate{}
	branding := &BrandingSettings{
		DisplayName:        "NALA Metallbau",
		Claim:              "Praezision in Metall",
		DocumentHeaderText: "Globaler Kopf",
		DocumentFooterText: "Globaler Fuss",
	}

	got := ApplyBrandingDefaults(tmpl, branding)
	if got.HeaderText != "Globaler Kopf" {
		t.Fatalf("expected branding header fallback, got %q", got.HeaderText)
	}
	if got.FooterText != "Globaler Fuss" {
		t.Fatalf("expected branding footer fallback, got %q", got.FooterText)
	}
}

func TestApplyBrandingDefaultsBuildsHeaderFromBrandNameAndClaim(t *testing.T) {
	tmpl := PDFTemplate{}
	branding := &BrandingSettings{
		DisplayName: "NALA Metallbau",
		Claim:       "Praezision in Metall",
	}

	got := ApplyBrandingDefaults(tmpl, branding)
	if got.HeaderText != "NALA Metallbau\nPraezision in Metall" {
		t.Fatalf("unexpected header fallback %q", got.HeaderText)
	}
}

func TestApplyBrandingDefaultsKeepsTemplateSpecificValues(t *testing.T) {
	tmpl := PDFTemplate{
		HeaderText: "Template-Kopf",
		FooterText: "Template-Fuss",
	}
	branding := &BrandingSettings{
		DocumentHeaderText: "Globaler Kopf",
		DocumentFooterText: "Globaler Fuss",
	}

	got := ApplyBrandingDefaults(tmpl, branding)
	if got.HeaderText != "Template-Kopf" {
		t.Fatalf("expected template header to win, got %q", got.HeaderText)
	}
	if got.FooterText != "Template-Fuss" {
		t.Fatalf("expected template footer to win, got %q", got.FooterText)
	}
}
