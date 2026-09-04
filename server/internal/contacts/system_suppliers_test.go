package contacts

import (
	"context"
	"testing"
)

func TestCreateSupplierProfileSeriesRejectsMissingProfilserie(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreateSupplierProfileSeries(context.Background(), "contact-1", SupplierProfileSeriesCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "profilserie erforderlich" {
		t.Fatalf("expected profilserie erforderlich, got %q", err.Error())
	}
}

func TestCreateSupplierProfileSeriesRejectsBlankProfilserie(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreateSupplierProfileSeries(context.Background(), "contact-1", SupplierProfileSeriesCreate{Profilserie: "   "}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "profilserie erforderlich" {
		t.Fatalf("expected profilserie erforderlich, got %q", err.Error())
	}
}

func TestListSuppliersByProfileSeriesRejectsMissingProfilserie(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.ListSuppliersByProfileSeries(context.Background(), "  ", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "profilserie erforderlich" {
		t.Fatalf("expected profilserie erforderlich, got %q", err.Error())
	}
}
