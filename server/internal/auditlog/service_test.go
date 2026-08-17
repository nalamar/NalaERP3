package auditlog

import (
	"context"
	"testing"
)

func TestRecordRejectsMissingCompanyID(t *testing.T) {
	s := NewService(nil)

	err := s.Record(context.Background(), nil, "", RecordInput{
		EntityType: "invoice_out",
		EntityID:   "inv-1",
		Action:     "gebucht",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected 'Mandant erforderlich', got %q", err.Error())
	}
}

func TestRecordRejectsMissingEntityType(t *testing.T) {
	s := NewService(nil)

	err := s.Record(context.Background(), nil, "company-1", RecordInput{
		EntityID: "inv-1",
		Action:   "gebucht",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "entity_type erforderlich" {
		t.Fatalf("expected 'entity_type erforderlich', got %q", err.Error())
	}
}

func TestRecordRejectsMissingEntityID(t *testing.T) {
	s := NewService(nil)

	err := s.Record(context.Background(), nil, "company-1", RecordInput{
		EntityType: "invoice_out",
		Action:     "gebucht",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "entity_id erforderlich" {
		t.Fatalf("expected 'entity_id erforderlich', got %q", err.Error())
	}
}

func TestRecordRejectsMissingAction(t *testing.T) {
	s := NewService(nil)

	err := s.Record(context.Background(), nil, "company-1", RecordInput{
		EntityType: "invoice_out",
		EntityID:   "inv-1",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "action erforderlich" {
		t.Fatalf("expected 'action erforderlich', got %q", err.Error())
	}
}

func TestListRejectsMissingCompanyID(t *testing.T) {
	s := NewService(nil)

	entries, err := s.List(context.Background(), "invoice_out", "inv-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %#v", entries)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected 'Mandant erforderlich', got %q", err.Error())
	}
}
