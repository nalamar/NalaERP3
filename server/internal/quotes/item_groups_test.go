package quotes

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestValidateGroupHierarchyRejectsInvalidKind(t *testing.T) {
	s := &Service{}

	err := s.validateGroupHierarchy(context.Background(), uuid.New(), "abschnitt", nil)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "kind ist ungültig (gültig: los, titel, untertitel)" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestValidateGroupHierarchyRejectsLosWithParent(t *testing.T) {
	s := &Service{}
	parent := uuid.New()

	err := s.validateGroupHierarchy(context.Background(), uuid.New(), "los", &parent)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Hierarchie: los darf keinen übergeordneten Knoten haben" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestValidateGroupHierarchyAllowsTopLevelLosAndTitel(t *testing.T) {
	s := &Service{}

	if err := s.validateGroupHierarchy(context.Background(), uuid.New(), "los", nil); err != nil {
		t.Fatalf("unexpected error for top-level los: %v", err)
	}
	if err := s.validateGroupHierarchy(context.Background(), uuid.New(), "titel", nil); err != nil {
		t.Fatalf("unexpected error for top-level titel: %v", err)
	}
}

func TestValidateGroupHierarchyRejectsUntertitelWithoutParent(t *testing.T) {
	s := &Service{}

	err := s.validateGroupHierarchy(context.Background(), uuid.New(), "untertitel", nil)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Hierarchie: untertitel benötigt einen übergeordneten titel-Knoten" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestCreateQuoteItemGroupRejectsMissingBezeichnung(t *testing.T) {
	s := &Service{}

	_, err := s.CreateQuoteItemGroup(context.Background(), uuid.New(), QuoteItemGroupCreate{Kind: "los"}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "bezeichnung erforderlich" {
		t.Fatalf("expected bezeichnung erforderlich, got %q", err.Error())
	}
}
