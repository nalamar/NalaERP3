package auth

import (
	"context"
	"testing"
)

func TestReplaceUserRolesRejectsMissingUserID(t *testing.T) {
	svc := newTestService()
	_, err := svc.ReplaceUserRoles(context.Background(), "  ", []string{"admin"}, "default", "actor-1")
	if err == nil {
		t.Fatal("expected error for empty userID, got nil")
	}
	if err.Error() != "benutzer-id erforderlich" {
		t.Fatalf("expected 'benutzer-id erforderlich', got %q", err.Error())
	}
}

func TestReplaceUserRolesRejectsMissingCompanyID(t *testing.T) {
	svc := newTestService()
	_, err := svc.ReplaceUserRoles(context.Background(), "user-1", []string{"admin"}, "  ", "actor-1")
	if err == nil {
		t.Fatal("expected error for empty companyID, got nil")
	}
	if err.Error() != "mandant erforderlich" {
		t.Fatalf("expected 'mandant erforderlich', got %q", err.Error())
	}
}
