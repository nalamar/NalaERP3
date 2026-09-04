package materials

import (
	"context"
	"testing"
)

func TestCreateReservationRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	got, err := svc.CreateReservation(context.Background(), StockReservationCreate{MaterialID: "mat-1", WarehouseID: "wh-1", ProjectID: "proj-1", Qty: 1}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil reservation, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCreateReservationRejectsMissingRequiredFields(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreateReservation(context.Background(), StockReservationCreate{Qty: 1}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "MaterialID, WarehouseID und ProjectID erforderlich" {
		t.Fatalf("expected required-fields error, got %q", err.Error())
	}
}

func TestCreateReservationRejectsNonPositiveQty(t *testing.T) {
	svc := NewService(nil, nil, "")

	for _, qty := range []float64{0, -1, -0.5} {
		_, err := svc.CreateReservation(context.Background(), StockReservationCreate{MaterialID: "mat-1", WarehouseID: "wh-1", ProjectID: "proj-1", Qty: qty}, "default")
		if err == nil {
			t.Fatalf("expected validation error for qty=%v, got nil", qty)
		}
		if err.Error() != "Menge muss größer als 0 sein" {
			t.Fatalf("expected Menge muss größer als 0 sein for qty=%v, got %q", qty, err.Error())
		}
	}
}

func TestReleaseReservationRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ReleaseReservation(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestReleaseReservationRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ReleaseReservation(context.Background(), "resv-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestListReservationsRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ListReservations(context.Background(), StockReservationFilter{}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestAvailableStockRejectsMissingIDs(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.AvailableStock(context.Background(), "", "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "MaterialID und WarehouseID erforderlich" {
		t.Fatalf("expected required-fields error, got %q", err.Error())
	}
}
