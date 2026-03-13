package settings

import (
	"testing"
	"time"
)

func TestPadAddsLeadingZeros(t *testing.T) {
	if got := pad(7, 4); got != "0007" {
		t.Fatalf("expected 0007, got %q", got)
	}
}

func TestPadDoesNotTrimLongerNumbers(t *testing.T) {
	if got := pad(12345, 4); got != "12345" {
		t.Fatalf("expected 12345, got %q", got)
	}
}

func TestFormatWithPatternReplacesDateAndNumberTokens(t *testing.T) {
	ts := time.Date(2026, time.March, 13, 10, 30, 0, 0, time.UTC)

	got := formatWithPattern("PO-{YYYY}-{MM}-{DD}-{NNNN}", 12, ts)
	want := "PO-2026-03-13-0012"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatWithPatternSupportsShortYearAndTripleDigits(t *testing.T) {
	ts := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)

	got := formatWithPattern("ANG-{YY}-{NNN}", 3, ts)
	want := "ANG-26-003"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatWithPatternLeavesPatternWithoutNumberTokenUntouched(t *testing.T) {
	ts := time.Date(2026, time.December, 24, 0, 0, 0, 0, time.UTC)

	got := formatWithPattern("CFG-{YYYY}", 99, ts)
	want := "CFG-2026"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
