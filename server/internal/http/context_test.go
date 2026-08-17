package apihttp

import (
	"context"
	"testing"

	"nalaerp3/internal/auth"
)

// companyIDFromContext/branchIDFromContext sind reine Context-Extraktoren
// ohne DB-Zugriff (Subtask 0.2.2.1.1, Voraussetzung fuer die Mandanten-
// Scoping-Filterung in den Domaenen-Packages).

func TestCompanyIDFromContextReturnsValueWhenSet(t *testing.T) {
	ctx := context.WithValue(context.Background(), authUserKey, auth.User{CompanyID: "acme"})

	got, ok := companyIDFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "acme" {
		t.Fatalf("expected 'acme', got %q", got)
	}
}

func TestCompanyIDFromContextReturnsFalseWhenMissing(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
	}{
		{"kein User im Context", context.Background()},
		{"User ohne CompanyID", context.WithValue(context.Background(), authUserKey, auth.User{})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := companyIDFromContext(tc.ctx)
			if ok {
				t.Fatalf("expected ok=false, got value %q", got)
			}
			if got != "" {
				t.Fatalf("expected empty string, got %q", got)
			}
		})
	}
}

func TestBranchIDFromContextReturnsValueWhenSet(t *testing.T) {
	branch := "hq"
	ctx := context.WithValue(context.Background(), authUserKey, auth.User{CompanyID: "acme", BranchID: &branch})

	got, ok := branchIDFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "hq" {
		t.Fatalf("expected 'hq', got %q", got)
	}
}

func TestBranchIDFromContextReturnsFalseWhenNilOrEmpty(t *testing.T) {
	empty := ""
	cases := []struct {
		name string
		user auth.User
	}{
		{"BranchID ist nil", auth.User{CompanyID: "acme"}},
		{"BranchID zeigt auf leeren String", auth.User{CompanyID: "acme", BranchID: &empty}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), authUserKey, tc.user)
			got, ok := branchIDFromContext(ctx)
			if ok {
				t.Fatalf("expected ok=false, got value %q", got)
			}
			if got != "" {
				t.Fatalf("expected empty string, got %q", got)
			}
		})
	}
}
