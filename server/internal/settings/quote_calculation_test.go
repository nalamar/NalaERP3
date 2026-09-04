package settings

import (
	"context"
	"testing"
)

func TestQuoteCalculationSettingsUpsertRejectsNegativeMaterialZuschlag(t *testing.T) {
	s := &QuoteCalculationSettingsService{}

	err := s.Upsert(context.Background(), QuoteCalculationSettings{DefaultMaterialZuschlagPercent: -1})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Material-Zuschlag ist ungueltig" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestQuoteCalculationSettingsUpsertRejectsNegativeLohnStundensatz(t *testing.T) {
	s := &QuoteCalculationSettingsService{}

	err := s.Upsert(context.Background(), QuoteCalculationSettings{DefaultLohnStundensatz: -1})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Lohn-Stundensatz ist ungueltig" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestQuoteCalculationSettingsUpsertAllowsZeroDefaults(t *testing.T) {
	// Anders als target_margin_percent (0 -> Fallback 20) gilt fuer die
	// neuen B.3-Felder 0 als korrekter, gewollter Ausgangswert (ADR 0011)
	// - dieser Test dokumentiert diese bewusste Abweichung.
	targetMarginPercent, err := validateQuoteCalculationDefaults(QuoteCalculationSettings{})
	if err != nil {
		t.Fatalf("unexpected error for all-zero defaults: %v", err)
	}
	if targetMarginPercent != 20 {
		t.Fatalf("expected target_margin_percent fallback to 20, got %v", targetMarginPercent)
	}
}
