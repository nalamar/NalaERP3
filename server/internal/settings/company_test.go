package settings

import "testing"

func TestNormalizeCountryDefaultsToDE(t *testing.T) {
    if got := normalizeCountry("  "); got != "DE" {
        t.Fatalf("expected DE, got %q", got)
    }
    if got := normalizeCountry("at"); got != "AT" {
        t.Fatalf("expected AT, got %q", got)
    }
}

func TestNormalizeCompactUpper(t *testing.T) {
    if got := normalizeCompactUpper(" de12 3456 "); got != "DE123456" {
        t.Fatalf("unexpected compact upper %q", got)
    }
}

func TestFirstNonEmpty(t *testing.T) {
    if got := firstNonEmpty("  Neu  ", "Alt"); got != "Neu" {
        t.Fatalf("expected Neu, got %q", got)
    }
    if got := firstNonEmpty("   ", "Alt"); got != "Alt" {
        t.Fatalf("expected fallback Alt, got %q", got)
    }
}
