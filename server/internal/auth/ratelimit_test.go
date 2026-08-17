package auth

import (
	"context"
	"testing"
	"time"
)

func TestLoginRateLimitKeyStripsPort(t *testing.T) {
	cases := map[string]string{
		"203.0.113.5:4821": "login_rl:203.0.113.5",
		"203.0.113.5":      "login_rl:203.0.113.5",
		"":                 "login_rl:",
	}
	for in, want := range cases {
		if got := loginRateLimitKey(in); got != want {
			t.Fatalf("loginRateLimitKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoginRateLimiterNilIsNoop(t *testing.T) {
	var l *LoginRateLimiter
	ctx := context.Background()

	blocked, err := l.Blocked(ctx, "203.0.113.5:1234")
	if err != nil || blocked {
		t.Fatalf("expected nil limiter to never block, got blocked=%v err=%v", blocked, err)
	}
	if err := l.RegisterFailure(ctx, "203.0.113.5:1234"); err != nil {
		t.Fatalf("expected no error registering failure on nil limiter, got %v", err)
	}
	if err := l.Reset(ctx, "203.0.113.5:1234"); err != nil {
		t.Fatalf("expected no error resetting nil limiter, got %v", err)
	}
}

func TestLoginRateLimiterWithZeroMaxAttemptsIsDisabled(t *testing.T) {
	l := NewLoginRateLimiter(nil, 0, time.Minute)
	ctx := context.Background()

	blocked, err := l.Blocked(ctx, "203.0.113.5:1234")
	if err != nil || blocked {
		t.Fatalf("expected disabled limiter (maxAttempts<=0) to never block, got blocked=%v err=%v", blocked, err)
	}
}
