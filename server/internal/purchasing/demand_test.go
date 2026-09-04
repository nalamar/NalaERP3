package purchasing

import (
	"context"
	"testing"
)

func TestMinStockShortfallsRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	got, err := svc.MinStockShortfalls(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil result, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestQuoteDemandRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil)

	got, err := svc.QuoteDemand(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil result, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}
