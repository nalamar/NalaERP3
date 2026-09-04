package purchasing

import (
	"context"
	"testing"
)

func TestCreateRFQRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	got, items, err := svc.CreateRFQ(context.Background(), RFQCreate{Items: []RFQItemInput{{MaterialID: "mat-1", Qty: 1, UOM: "Stk"}}}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil || items != nil {
		t.Fatalf("expected nil results, got rfq=%#v items=%#v", got, items)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCreateRFQRejectsNoItems(t *testing.T) {
	svc := NewService(nil)

	_, _, err := svc.CreateRFQ(context.Background(), RFQCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "mindestens eine Position erforderlich" {
		t.Fatalf("expected mindestens eine Position erforderlich, got %q", err.Error())
	}
}

func TestCreateRFQRejectsInvalidItem(t *testing.T) {
	svc := NewService(nil)

	_, _, err := svc.CreateRFQ(context.Background(), RFQCreate{Items: []RFQItemInput{{MaterialID: "", Qty: 1, UOM: "Stk"}}}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige Position" {
		t.Fatalf("expected Ungültige Position, got %q", err.Error())
	}
}

func TestGetRFQRejectsMissingID(t *testing.T) {
	svc := NewService(nil)

	_, _, err := svc.GetRFQ(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestListRFQsRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.ListRFQs(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestRegisterSupplierQuoteRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.RegisterSupplierQuote(context.Background(), "item-1", SupplierQuoteInput{SupplierID: "sup-1", UnitPrice: 10}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestRegisterSupplierQuoteRejectsMissingFields(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.RegisterSupplierQuote(context.Background(), "", SupplierQuoteInput{UnitPrice: 10}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Anfrage-Position und Lieferant erforderlich" {
		t.Fatalf("expected Anfrage-Position und Lieferant erforderlich, got %q", err.Error())
	}
}

func TestRegisterSupplierQuoteRejectsNonPositivePrice(t *testing.T) {
	svc := NewService(nil)

	for _, price := range []float64{0, -5} {
		_, err := svc.RegisterSupplierQuote(context.Background(), "item-1", SupplierQuoteInput{SupplierID: "sup-1", UnitPrice: price}, "default")
		if err == nil {
			t.Fatalf("expected validation error for price=%v, got nil", price)
		}
		if err.Error() != "Preis muss größer als 0 sein" {
			t.Fatalf("expected Preis muss größer als 0 sein for price=%v, got %q", price, err.Error())
		}
	}
}

func TestListSupplierQuotesRejectsMissingID(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.ListSupplierQuotes(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestCancelRFQRejectsMissingID(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CancelRFQ(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestConvertToPurchaseOrderRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	po, items, err := svc.ConvertToPurchaseOrder(context.Background(), "rfq-1", "sup-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if po != nil || items != nil {
		t.Fatalf("expected nil results, got po=%#v items=%#v", po, items)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestConvertToPurchaseOrderRejectsMissingIDs(t *testing.T) {
	svc := NewService(nil)

	_, _, err := svc.ConvertToPurchaseOrder(context.Background(), "", "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Anfrage und Lieferant erforderlich" {
		t.Fatalf("expected Anfrage und Lieferant erforderlich, got %q", err.Error())
	}
}
