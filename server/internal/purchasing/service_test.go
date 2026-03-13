package purchasing

import (
	"context"
	"testing"
)

func TestCreateRejectsMissingSupplier(t *testing.T) {
	svc := NewService(nil)

	po, items, err := svc.Create(context.Background(), PurchaseOrderCreate{})
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
	})
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
	})
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

	_, _, err := svc.Update(context.Background(), "po-1", PurchaseOrderUpdate{Status: &status})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültiger Status" {
		t.Fatalf("expected Ungültiger Status, got %q", err.Error())
	}
}

func TestCreateItemRejectsInvalidItem(t *testing.T) {
	svc := NewService(nil)

	item, err := svc.CreateItem(context.Background(), "po-1", PurchaseOrderItemInput{})
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
