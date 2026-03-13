package settings

import "testing"

func TestNormalizeCurrencyDefaultsToEUR(t *testing.T) {
    if got := normalizeCurrency("  "); got != "EUR" {
        t.Fatalf("expected EUR, got %q", got)
    }
    if got := normalizeCurrency("usd"); got != "USD" {
        t.Fatalf("expected USD, got %q", got)
    }
}

func TestNormalizeLocaleDefaults(t *testing.T) {
    if got := normalizeLocale("  "); got != "de-DE" {
        t.Fatalf("expected de-DE, got %q", got)
    }
    if got := normalizeTimezone(""); got != "Europe/Berlin" {
        t.Fatalf("expected Europe/Berlin, got %q", got)
    }
    if got := normalizeDateFormat(""); got != "dd.MM.yyyy" {
        t.Fatalf("expected dd.MM.yyyy, got %q", got)
    }
}
