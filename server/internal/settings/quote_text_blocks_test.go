package settings

import (
	"context"
	"testing"
)

func TestQuoteTextBlockUpsertRejectsMissingCode(t *testing.T) {
	svc := NewQuoteTextBlockService(nil)

	err := svc.Upsert(context.Background(), QuoteTextBlock{
		Name:     "Einleitung",
		Category: "intro",
		Body:     "Text",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "code erforderlich" {
		t.Fatalf("expected code erforderlich, got %q", err.Error())
	}
}

func TestQuoteTextBlockUpsertRejectsInvalidCategory(t *testing.T) {
	svc := NewQuoteTextBlockService(nil)

	err := svc.Upsert(context.Background(), QuoteTextBlock{
		Code:     "intro-standard",
		Name:     "Einleitung",
		Category: "invalid",
		Body:     "Text",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ungültige category" {
		t.Fatalf("expected ungültige category, got %q", err.Error())
	}
}

func TestQuoteTextBlockDeleteRejectsInvalidID(t *testing.T) {
	svc := NewQuoteTextBlockService(nil)

	err := svc.Delete(context.Background(), "not-a-uuid")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ungültige id" {
		t.Fatalf("expected ungültige id, got %q", err.Error())
	}
}
