package accounting

import (
	"context"
	"testing"
)

func TestCostCenterCreateRejectsMissingCompanyID(t *testing.T) {
	svc := NewCostCenterService(nil)

	got, err := svc.Create(context.Background(), CostCenterCreate{Code: "WERK-1", Name: "Werkstatt"}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil cost center, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCostCenterCreateRejectsMissingRequiredFields(t *testing.T) {
	svc := NewCostCenterService(nil)

	_, err := svc.Create(context.Background(), CostCenterCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Code und Name sind erforderlich" {
		t.Fatalf("expected Code und Name sind erforderlich, got %q", err.Error())
	}
}

func TestCostCenterGetRejectsMissingID(t *testing.T) {
	svc := NewCostCenterService(nil)

	_, err := svc.Get(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestCostCenterGetRejectsMissingCompanyID(t *testing.T) {
	svc := NewCostCenterService(nil)

	_, err := svc.Get(context.Background(), "cc-1", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCostCenterListRejectsMissingCompanyID(t *testing.T) {
	svc := NewCostCenterService(nil)

	_, err := svc.List(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCostCenterUpdateRejectsMissingID(t *testing.T) {
	svc := NewCostCenterService(nil)

	_, err := svc.Update(context.Background(), "", CostCenterUpdate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestCostCenterUpdateRejectsMissingCompanyID(t *testing.T) {
	svc := NewCostCenterService(nil)

	_, err := svc.Update(context.Background(), "cc-1", CostCenterUpdate{}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCostCenterUpdateRejectsEmptyCode(t *testing.T) {
	svc := NewCostCenterService(nil)
	empty := ""

	_, err := svc.Update(context.Background(), "cc-1", CostCenterUpdate{Code: &empty}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Code erforderlich" {
		t.Fatalf("expected Code erforderlich, got %q", err.Error())
	}
}

func TestCostCenterUpdateRejectsEmptyName(t *testing.T) {
	svc := NewCostCenterService(nil)
	empty := ""

	_, err := svc.Update(context.Background(), "cc-1", CostCenterUpdate{Name: &empty}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Name erforderlich" {
		t.Fatalf("expected Name erforderlich, got %q", err.Error())
	}
}
