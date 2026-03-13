package apihttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestReadyzReportsDependencyChecks(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status string `json:"status"`
		Checks map[string]struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}
	for _, dep := range []string{"postgres", "mongo", "redis"} {
		check, ok := body.Checks[dep]
		if !ok {
			t.Fatalf("missing dependency check %q", dep)
		}
		if check.Status != "ok" {
			t.Fatalf("expected %s to be ok, got status=%q error=%q", dep, check.Status, check.Error)
		}
	}
}

func TestLivezIsAvailableWithoutDependencies(t *testing.T) {
	handler := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
