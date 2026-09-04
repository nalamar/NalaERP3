package materials

import (
	"context"
	"testing"
	"time"
)

func TestCreatePriceListRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreatePriceList(context.Background(), PriceListCreate{Name: "Test", GueltigVon: time.Now()}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCreatePriceListRejectsMissingName(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreatePriceList(context.Background(), PriceListCreate{GueltigVon: time.Now()}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}

func TestCreatePriceListRejectsMissingGueltigVon(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreatePriceList(context.Background(), PriceListCreate{Name: "Test"}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Gültig-von erforderlich" {
		t.Fatalf("expected Gültig-von erforderlich, got %q", err.Error())
	}
}

func TestCreatePriceListRejectsGueltigBisBeforeGueltigVon(t *testing.T) {
	svc := NewService(nil, nil, "")
	von := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	bis := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := svc.CreatePriceList(context.Background(), PriceListCreate{Name: "Test", GueltigVon: von, GueltigBis: &bis}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Gültig-bis darf nicht vor Gültig-von liegen" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestUpdatePriceListRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.UpdatePriceList(context.Background(), "", PriceListUpdate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestDeleteSoftPriceListRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	err := svc.DeleteSoftPriceList(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestCreatePriceListItemRejectsMissingMaterialID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreatePriceListItem(context.Background(), "pl-1", PriceListItemCreate{UnitPrice: 10}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "material_id erforderlich" {
		t.Fatalf("expected material_id erforderlich, got %q", err.Error())
	}
}

func TestCreatePriceListItemRejectsNegativeMinMenge(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreatePriceListItem(context.Background(), "pl-1", PriceListItemCreate{MaterialID: "mat-1", MinMenge: -1, UnitPrice: 10}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mindestmenge darf nicht negativ sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestCreatePriceListItemRejectsNegativeUnitPrice(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.CreatePriceListItem(context.Background(), "pl-1", PriceListItemCreate{MaterialID: "mat-1", UnitPrice: -1}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Preis darf nicht negativ sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestEffectivePriceForMaterialRejectsMissingMaterialID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.EffectivePriceForMaterial(context.Background(), "", 1, time.Now(), "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "material_id erforderlich" {
		t.Fatalf("expected material_id erforderlich, got %q", err.Error())
	}
}

func TestEffectivePriceForMaterialRejectsNegativeMenge(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.EffectivePriceForMaterial(context.Background(), "mat-1", -1, time.Now(), "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Menge darf nicht negativ sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
