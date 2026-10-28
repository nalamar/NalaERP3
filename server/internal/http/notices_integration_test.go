package apihttp

import (
	"net/http"
	"strings"
	"testing"

	"nalaerp3/internal/testutil"
)

// GET /notices gegen echtes Postgres (Backlog F.10.4.1).

type noticeResponse struct {
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Total    int            `json:"total"`
	Counts   map[string]int `json:"counts"`
}

func TestNoticesHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-notices-qualread", "qualifications.read")
	seedRoleWithPermissions(t, env, "itest-notices-none")
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	worker := tenant.linkedUser("production", "Werkerhinweis")
	colleague := tenant.linkedUser("production", "Kollegehinweis")
	reader := tenant.user("itest-notices-qualread")
	nobody := tenant.user("itest-notices-none")
	foreignReader := other.user("itest-notices-qualread")

	annual := tenant.qualType(hrToken, "ARBEITSSCHUTZ", "12")
	// Werker: erforderlich, aber kein Nachweis (fehlt). Kollege: abgelaufen.
	decodeRequirements(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, qualPath+"requirements?employee_id="+worker.EmployeeID.String(), `{"type_ids":["`+annual.ID.String()+`"]}`))
	decodeQualRecords(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath, qualBody(annual.ID, "2025-01-10", colleague.EmployeeID)), http.StatusCreated)

	get := func(token string) ([]noticeResponse, string) {
		t.Helper()
		rec := doAuthedRequest(t, handler, token, http.MethodGet, "/api/v1/notices", "")
		var out []noticeResponse
		decodeJSONStatus(t, rec, http.StatusOK, &out)
		return out, rec.Body.String()
	}
	byKind := func(ns []noticeResponse) map[string]noticeResponse {
		m := map[string]noticeResponse{}
		for _, n := range ns {
			m[n.Kind] = n
		}
		return m
	}

	// Personalabteilung (nur qualifications.read, kein Mitarbeiter): die
	// betriebsweite Anzahl, ohne Namen.
	ns, body := get(reader)
	if len(ns) != 1 {
		t.Fatalf("Leser: %s", body)
	}
	if n := ns[0]; n.Kind != "qualifications_due" || n.Total != 2 || n.Counts["fehlt"] != 1 || n.Counts["abgelaufen"] != 1 || n.Severity != "kritisch" {
		t.Fatalf("Leser: %+v", n)
	}
	for _, name := range []string{"Werkerhinweis", "Kollegehinweis"} {
		if strings.Contains(body, name) {
			t.Fatalf("Name %q im Hinweis: %s", name, body)
		}
	}

	// Mitarbeiter mit qualifications.self: nur die eigenen.
	ns, body = get(worker.Token)
	kinds := byKind(ns)
	if _, ok := kinds["qualifications_due"]; ok {
		t.Fatalf("Mitarbeiter sieht die betriebsweite Liste: %s", body)
	}
	if n, ok := kinds["own_qualifications"]; !ok || n.Total != 1 || n.Counts["fehlt"] != 1 {
		t.Fatalf("Mitarbeiter: %s", body)
	}
	if n := byKind(func() []noticeResponse { ns, _ := get(colleague.Token); return ns }())["own_qualifications"]; n.Total != 1 || n.Counts["abgelaufen"] != 1 {
		t.Fatalf("Kollege: %+v", n)
	}

	// Ohne Rechte: leere Liste.
	if _, body := get(nobody); strings.TrimSpace(body) != "[]" {
		t.Fatalf("ohne Rechte: %s", body)
	}
	// Anderer Mandant: dessen Betrieb hat nichts Faelliges.
	if ns, body := get(foreignReader); len(ns) != 0 {
		t.Fatalf("fremder Mandant: %s", body)
	}

	// Nach dem Nachweis faellt der Hinweis des Werkers weg.
	decodeQualRecords(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath, qualBody(annual.ID, "2026-10-01", worker.EmployeeID)), http.StatusCreated)
	if n, ok := byKind(func() []noticeResponse { ns, _ := get(worker.Token); return ns }())["own_qualifications"]; ok {
		t.Fatalf("Werker nach Nachweis: %+v", n)
	}
	if ns, _ := get(reader); len(ns) != 1 || ns[0].Total != 1 {
		t.Fatalf("Leser nach Nachweis: %+v", ns)
	}

	// Ohne Anmeldung kein Zugriff.
	wantStatus(t, doAuthedRequest(t, handler, "", http.MethodGet, "/api/v1/notices", ""), http.StatusUnauthorized)
}
