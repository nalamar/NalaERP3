package auth

import (
	"context"
	"testing"
)

func TestCreateUserRejectsMissingOrInvalidEmail(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	cases := []UserCreate{
		{Email: "", Password: "geheim123"},
		{Email: "   ", Password: "geheim123"},
		{Email: "no-at-sign", Password: "geheim123"},
	}
	for _, in := range cases {
		if _, err := svc.CreateUser(ctx, in, "default"); err == nil {
			t.Fatalf("expected error for email %q, got nil", in.Email)
		}
	}
}

func TestCreateUserRejectsMissingCompanyID(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.CreateUser(ctx, UserCreate{Email: "user@example.com", Password: "geheim123"}, "")
	if err == nil {
		t.Fatal("expected error for missing companyID, got nil")
	}
	if err.Error() != "mandant erforderlich" {
		t.Fatalf("expected 'mandant erforderlich', got %q", err.Error())
	}
}

func TestCreateUserRejectsEmptyPassword(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.CreateUser(ctx, UserCreate{Email: "user@example.com", Password: "   "}, "default")
	if err == nil {
		t.Fatal("expected error for empty password, got nil")
	}
	if err.Error() != "passwort erforderlich" {
		t.Fatalf("expected 'passwort erforderlich', got %q", err.Error())
	}
}
