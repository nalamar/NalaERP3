package materials

import (
	"context"
	"testing"
)

func TestRegisterOffcutRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	got, err := svc.RegisterOffcut(context.Background(), ProfileOffcutCreate{MaterialID: "mat-1", WarehouseID: "wh-1", LengthMM: 500}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil offcut, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestRegisterOffcutRejectsMissingRequiredFields(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.RegisterOffcut(context.Background(), ProfileOffcutCreate{LengthMM: 500}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "MaterialID und WarehouseID erforderlich" {
		t.Fatalf("expected required-fields error, got %q", err.Error())
	}
}

func TestRegisterOffcutRejectsNonPositiveLength(t *testing.T) {
	svc := NewService(nil, nil, "")

	for _, length := range []float64{0, -1, -0.5} {
		_, err := svc.RegisterOffcut(context.Background(), ProfileOffcutCreate{MaterialID: "mat-1", WarehouseID: "wh-1", LengthMM: length}, "default")
		if err == nil {
			t.Fatalf("expected validation error for length=%v, got nil", length)
		}
		if err.Error() != "Länge muss größer als 0 sein" {
			t.Fatalf("expected Länge muss größer als 0 sein for length=%v, got %q", length, err.Error())
		}
	}
}

func TestListOffcutsRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ListOffcuts(context.Background(), ProfileOffcutFilter{}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestConsumeOffcutRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ConsumeOffcut(context.Background(), "", ProfileOffcutConsume{UsedLengthMM: 100}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestConsumeOffcutRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.ConsumeOffcut(context.Background(), "offcut-1", ProfileOffcutConsume{UsedLengthMM: 100}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestConsumeOffcutRejectsNonPositiveUsedLength(t *testing.T) {
	svc := NewService(nil, nil, "")

	for _, length := range []float64{0, -1} {
		_, err := svc.ConsumeOffcut(context.Background(), "offcut-1", ProfileOffcutConsume{UsedLengthMM: length}, "default")
		if err == nil {
			t.Fatalf("expected validation error for used_length_mm=%v, got nil", length)
		}
		if err.Error() != "Verbrauchte Länge muss größer als 0 sein" {
			t.Fatalf("expected Verbrauchte Länge muss größer als 0 sein for used_length_mm=%v, got %q", length, err.Error())
		}
	}
}
