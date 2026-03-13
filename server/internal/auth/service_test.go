package auth

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nalaerp3/internal/config"
)

func newTestService() *Service {
	return NewService(nil, nil, &config.Config{
		JWTSecret:             "test-secret",
		AccessTokenTTLMinutes: 15,
		SessionTTLHours:       12,
	})
}

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	got, err := HashPassword("   ")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != "" {
		t.Fatalf("expected empty hash, got %q", got)
	}
	if err.Error() != "passwort erforderlich" {
		t.Fatalf("expected passwort erforderlich, got %q", err.Error())
	}
}

func TestHashPasswordReturnsBCryptHash(t *testing.T) {
	hash, err := HashPassword("geheim123")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("geheim123")); err != nil {
		t.Fatalf("expected bcrypt hash to match input password, got %v", err)
	}
}

func TestIssueTokenPairAndParseTokenRoundTrip(t *testing.T) {
	svc := newTestService()
	now := time.Now().UTC()

	pair, err := svc.issueTokenPair("user-1", "session-1", []string{"admin", "sales"}, now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("expected both tokens to be set, got %#v", pair)
	}
	if pair.SessionID != "session-1" {
		t.Fatalf("expected session-1, got %q", pair.SessionID)
	}

	accessClaims, err := svc.parseToken(pair.AccessToken, "access")
	if err != nil {
		t.Fatalf("expected access token to parse, got %v", err)
	}
	if accessClaims.Subject != "user-1" {
		t.Fatalf("expected user-1, got %q", accessClaims.Subject)
	}
	if accessClaims.SessionID != "session-1" {
		t.Fatalf("expected session-1, got %q", accessClaims.SessionID)
	}
	if accessClaims.TokenType != "access" {
		t.Fatalf("expected access token type, got %q", accessClaims.TokenType)
	}
	if len(accessClaims.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %#v", accessClaims.Roles)
	}

	refreshClaims, err := svc.parseToken(pair.RefreshToken, "refresh")
	if err != nil {
		t.Fatalf("expected refresh token to parse, got %v", err)
	}
	if refreshClaims.TokenType != "refresh" {
		t.Fatalf("expected refresh token type, got %q", refreshClaims.TokenType)
	}
	if !refreshClaims.ExpiresAt.After(accessClaims.ExpiresAt) {
		t.Fatalf("expected refresh token to outlive access token: access=%s refresh=%s", accessClaims.ExpiresAt, refreshClaims.ExpiresAt)
	}
}

func TestParseTokenRejectsWrongExpectedType(t *testing.T) {
	svc := newTestService()
	now := time.Now().UTC()

	pair, err := svc.issueTokenPair("user-1", "session-1", []string{"admin"}, now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	_, err = svc.parseToken(pair.AccessToken, "refresh")
	if err == nil {
		t.Fatal("expected invalid token error, got nil")
	}
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseTokenRejectsTamperedToken(t *testing.T) {
	svc := newTestService()
	now := time.Now().UTC()

	pair, err := svc.issueTokenPair("user-1", "session-1", []string{"admin"}, now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	tampered := pair.AccessToken + "tampered"
	_, err = svc.parseToken(tampered, "access")
	if err == nil {
		t.Fatal("expected invalid token error, got nil")
	}
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
