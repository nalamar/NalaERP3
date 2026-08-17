package auth

import (
	"context"
	"testing"
)

func TestSetUserLockedRejectsMissingUserID(t *testing.T) {
	svc := newTestService()
	_, err := svc.SetUserLocked(context.Background(), "  ", true, "default")
	if err == nil {
		t.Fatal("expected error for empty userID, got nil")
	}
	if err.Error() != "benutzer-id erforderlich" {
		t.Fatalf("expected 'benutzer-id erforderlich', got %q", err.Error())
	}
}

func TestSetUserLockedRejectsMissingCompanyID(t *testing.T) {
	svc := newTestService()
	_, err := svc.SetUserLocked(context.Background(), "user-1", true, "  ")
	if err == nil {
		t.Fatal("expected error for empty companyID, got nil")
	}
	if err.Error() != "mandant erforderlich" {
		t.Fatalf("expected 'mandant erforderlich', got %q", err.Error())
	}
}
