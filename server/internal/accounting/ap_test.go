package accounting

import (
	"context"
	"testing"
)

func TestCreateInvoiceInRejectsMissingCompanyID(t *testing.T) {
	svc := NewAPService(nil)

	got, items, err := svc.CreateInvoiceIn(context.Background(), InvoiceInCreate{
		SupplierID: "sup-1",
		Items:      []InvoiceInItemInput{{Qty: 1, UnitPrice: 10}},
	}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil || items != nil {
		t.Fatalf("expected nil results, got invoice=%#v items=%#v", got, items)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCreateInvoiceInRejectsMissingSupplierID(t *testing.T) {
	svc := NewAPService(nil)

	_, _, err := svc.CreateInvoiceIn(context.Background(), InvoiceInCreate{
		Items: []InvoiceInItemInput{{Qty: 1, UnitPrice: 10}},
	}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Lieferant erforderlich" {
		t.Fatalf("expected Lieferant erforderlich, got %q", err.Error())
	}
}

func TestCreateInvoiceInRejectsNoItems(t *testing.T) {
	svc := NewAPService(nil)

	_, _, err := svc.CreateInvoiceIn(context.Background(), InvoiceInCreate{SupplierID: "sup-1"}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "mindestens eine Position erforderlich" {
		t.Fatalf("expected mindestens eine Position erforderlich, got %q", err.Error())
	}
}

func TestCreateInvoiceInRejectsNonPositiveQty(t *testing.T) {
	svc := NewAPService(nil)

	for _, qty := range []float64{0, -1} {
		_, _, err := svc.CreateInvoiceIn(context.Background(), InvoiceInCreate{
			SupplierID: "sup-1",
			Items:      []InvoiceInItemInput{{Qty: qty, UnitPrice: 10}},
		}, "default")
		if err == nil {
			t.Fatalf("expected validation error for qty=%v, got nil", qty)
		}
		if err.Error() != "Menge muss größer als 0 sein" {
			t.Fatalf("expected Menge muss größer als 0 sein for qty=%v, got %q", qty, err.Error())
		}
	}
}

func TestGetInvoiceInRejectsMissingID(t *testing.T) {
	svc := NewAPService(nil)

	_, _, err := svc.GetInvoiceIn(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestGetInvoiceInRejectsMissingCompanyID(t *testing.T) {
	svc := NewAPService(nil)

	_, _, err := svc.GetInvoiceIn(context.Background(), "invoice-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestListInvoicesInRejectsMissingCompanyID(t *testing.T) {
	svc := NewAPService(nil)

	_, err := svc.ListInvoicesIn(context.Background(), InvoiceInFilter{}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestMatchInvoiceInRejectsMissingID(t *testing.T) {
	svc := NewAPService(nil)

	_, err := svc.MatchInvoiceIn(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestMatchInvoiceInRejectsMissingCompanyID(t *testing.T) {
	svc := NewAPService(nil)

	_, err := svc.MatchInvoiceIn(context.Background(), "invoice-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}
