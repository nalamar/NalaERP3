package settings

import (
	"context"
	"testing"
)

func TestUnitUpsertRejectsEmptyCode(t *testing.T) {
	svc := NewUnitService(nil)

	err := svc.Upsert(context.Background(), "   ", "Stueck")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "code erforderlich" {
		t.Fatalf("expected code erforderlich, got %q", err.Error())
	}
}

func TestUnitDeleteRejectsEmptyCode(t *testing.T) {
	svc := NewUnitService(nil)

	err := svc.Delete(context.Background(), "\n\t ")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "code erforderlich" {
		t.Fatalf("expected code erforderlich, got %q", err.Error())
	}
}

func TestTrimRemovesLeadingAndTrailingWhitespace(t *testing.T) {
	got := trim(" \t\n kg \r\n")
	if got != "kg" {
		t.Fatalf("expected kg, got %q", got)
	}
}
