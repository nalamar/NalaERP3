package purchasing

import (
	"context"
	"testing"
)

// TestIsEditableStatusOnlyAllowsDraft belegt den in Task 0.3.2 ergaenzten
// Festschreibungs-Mechanismus: nur draft-Bestellungen sind inhaltlich
// bearbeitbar, alle anderen Status (auch Grossschreibung/Leerraum-Varianten)
// sind schreibgeschuetzt.
func TestIsEditableStatusOnlyAllowsDraft(t *testing.T) {
	cases := map[string]bool{
		"draft":    true,
		" Draft ":  true,
		"ordered":  false,
		"received": false,
		"canceled": false,
		"":         false,
	}
	for status, want := range cases {
		if got := isEditableStatus(status); got != want {
			t.Errorf("isEditableStatus(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestCreateRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	po, items, err := svc.Create(context.Background(), PurchaseOrderCreate{SupplierID: "supplier-1"}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if po != nil {
		t.Fatalf("expected nil purchase order, got %#v", po)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %#v", items)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCreateRejectsMissingSupplier(t *testing.T) {
	svc := NewService(nil)

	po, items, err := svc.Create(context.Background(), PurchaseOrderCreate{}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if po != nil {
		t.Fatalf("expected nil purchase order, got %#v", po)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %#v", items)
	}
	if err.Error() != "Lieferant erforderlich" {
		t.Fatalf("expected Lieferant erforderlich, got %q", err.Error())
	}
}

func TestCreateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(nil)

	_, _, err := svc.Create(context.Background(), PurchaseOrderCreate{
		SupplierID: "supplier-1",
		Status:     "invalid",
	}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Status" {
		t.Fatalf("expected Ungültiger Status, got %q", err.Error())
	}
}

func TestCreateRejectsInvalidItem(t *testing.T) {
	svc := NewService(nil)

	_, _, err := svc.Create(context.Background(), PurchaseOrderCreate{
		SupplierID: "supplier-1",
		Items: []PurchaseOrderItemInput{
			{MaterialID: "", Qty: 1, UOM: "Stk"},
		},
	}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Position" {
		t.Fatalf("expected Ungültige Position, got %q", err.Error())
	}
}

func TestUpdateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(nil)
	status := "invalid"

	_, _, err := svc.Update(context.Background(), "po-1", PurchaseOrderUpdate{Status: &status}, "company-1", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Status" {
		t.Fatalf("expected Ungültiger Status, got %q", err.Error())
	}
}

func TestCreateItemRejectsInvalidItem(t *testing.T) {
	svc := NewService(nil)

	item, err := svc.CreateItem(context.Background(), "po-1", PurchaseOrderItemInput{}, "company-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if item != nil {
		t.Fatalf("expected nil item, got %#v", item)
	}
	if err.Error() != "Ungültige Position" {
		t.Fatalf("expected Ungültige Position, got %q", err.Error())
	}
}
