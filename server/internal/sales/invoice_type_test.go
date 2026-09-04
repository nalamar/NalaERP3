package sales

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"nalaerp3/internal/accounting"
)

func TestConvertToInvoiceRejectsMissingInvoiceType(t *testing.T) {
	s := &Service{}
	arSvc := &accounting.ARService{}

	_, err := s.ConvertToInvoice(context.Background(), uuid.New(), arSvc, ConvertToInvoiceInput{}, "company-1", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "invoice_type ist ungültig (gültig: abschlagsrechnung, schlussrechnung)" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestConvertToInvoiceRejectsUnknownInvoiceType(t *testing.T) {
	s := &Service{}
	arSvc := &accounting.ARService{}

	_, err := s.ConvertToInvoice(context.Background(), uuid.New(), arSvc, ConvertToInvoiceInput{InvoiceType: "proformarechnung"}, "company-1", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "invoice_type ist ungültig (gültig: abschlagsrechnung, schlussrechnung)" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestVOBInvoiceTypesContainsExactlyTheTwoVOBTypes(t *testing.T) {
	if !vobInvoiceTypes["abschlagsrechnung"] {
		t.Fatal("expected abschlagsrechnung to be a valid VOB invoice type")
	}
	if !vobInvoiceTypes["schlussrechnung"] {
		t.Fatal("expected schlussrechnung to be a valid VOB invoice type")
	}
	if vobInvoiceTypes["rechnung"] {
		t.Fatal("expected the neutral 'rechnung' type to NOT be accepted for sales-order conversion (must be explicit)")
	}
	if len(vobInvoiceTypes) != 2 {
		t.Fatalf("expected exactly 2 valid VOB invoice types, got %d", len(vobInvoiceTypes))
	}
}
