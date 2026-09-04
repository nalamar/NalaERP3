package materials

import (
	"context"
	"testing"
)

func TestStartInventoryRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	got, err := svc.StartInventory(context.Background(), InventoryCreate{WarehouseID: "wh-1"}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil inventory, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestStartInventoryRejectsMissingWarehouseID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.StartInventory(context.Background(), InventoryCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "WarehouseID erforderlich" {
		t.Fatalf("expected WarehouseID erforderlich, got %q", err.Error())
	}
}

func TestGetInventoryRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.GetInventory(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestListInventoriesRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ListInventories(context.Background(), InventoryFilter{}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestAddInventoryLineRejectsMissingRequiredFields(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.AddInventoryLine(context.Background(), "", InventoryLineCreate{MaterialID: "mat-1"}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}

	_, err = svc.AddInventoryLine(context.Background(), "inv-1", InventoryLineCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "MaterialID erforderlich" {
		t.Fatalf("expected MaterialID erforderlich, got %q", err.Error())
	}
}

func TestAddInventoryLineRejectsNegativeIstQty(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.AddInventoryLine(context.Background(), "inv-1", InventoryLineCreate{MaterialID: "mat-1", IstQty: -1}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Istmenge darf nicht negativ sein" {
		t.Fatalf("expected Istmenge darf nicht negativ sein, got %q", err.Error())
	}
}

func TestListInventoryLinesRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ListInventoryLines(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestCloseInventoryRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CloseInventory(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestCloseInventoryRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CloseInventory(context.Background(), "inv-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}
