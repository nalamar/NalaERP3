package apihttp

import (
	"net/http"
	"testing"
	"time"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Auslastung ueber HTTP (Backlog H.2.4): Zeitraum, Fehler, Rechte,
// Mandant. Die Rechnung selbst prueft production/capacity_integration_test.go.

const capacityPath = "/api/v1/production/capacity"

type capacityResponse struct {
	Von            string `json:"von"`
	Bis            string `json:"bis"`
	Arbeitsplaetze []struct {
		Name   string `json:"name"`
		Wochen []struct {
			Woche         string  `json:"woche"`
			VerfuegbarMin float64 `json:"verfuegbar_min"`
		} `json:"wochen"`
	} `json:"arbeitsplaetze"`
}

func TestProductionCapacityHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pr-readonly", "production.read")
	seedRoleWithPermissions(t, env, "itest-pr-writeonly", "production.write")
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	decodeJSON[workCenterResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, workCentersPath, `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusCreated)

	got := decodeJSON[capacityResponse](t, doAuthedRequest(t, handler, prod, http.MethodGet, capacityPath+"?von=2026-11-25&bis=2026-12-09", ""), http.StatusOK)
	if got.Von[:10] != "2026-11-23" || got.Bis[:10] != "2026-12-07" || len(got.Arbeitsplaetze) != 1 ||
		got.Arbeitsplaetze[0].Name != "Zuschnitt" || len(got.Arbeitsplaetze[0].Wochen) != 3 || got.Arbeitsplaetze[0].Wochen[0].VerfuegbarMin != 2400 {
		t.Fatalf("Auslastung: %+v", got)
	}
	// Ohne Angaben: ab der aktuellen Woche acht Wochen; ohne bis acht
	// Wochen ab von.
	monday := func(d time.Time) string {
		back := (int(d.Weekday()) + 6) % 7
		return d.AddDate(0, 0, -back).Format("2006-01-02")
	}
	today := hr.WorkDateOf(time.Now())
	def := decodeJSON[capacityResponse](t, doAuthedRequest(t, handler, prod, http.MethodGet, capacityPath, ""), http.StatusOK)
	if def.Von[:10] != monday(today) || len(def.Arbeitsplaetze[0].Wochen) != 8 {
		t.Fatalf("ohne Angaben: von %s, %d Wochen", def.Von, len(def.Arbeitsplaetze[0].Wochen))
	}
	onlyVon := decodeJSON[capacityResponse](t, doAuthedRequest(t, handler, prod, http.MethodGet, capacityPath+"?von=2026-11-25", ""), http.StatusOK)
	if onlyVon.Bis[:10] != "2027-01-11" {
		t.Fatalf("nur von: bis %s", onlyVon.Bis)
	}

	wantAPIError(t, doAuthedRequest(t, handler, prod, http.MethodGet, capacityPath+"?von=25.11.2026", ""), http.StatusBadRequest, "validation_error", "Ungültiges Datum in von (erwartet JJJJ-MM-TT)")
	wantAPIError(t, doAuthedRequest(t, handler, prod, http.MethodGet, capacityPath+"?von=2026-12-09&bis=2026-11-25", ""), http.StatusBadRequest, "validation_error", "Ungültiger Zeitraum: bis liegt vor von")
	wantAPIError(t, doAuthedRequest(t, handler, prod, http.MethodGet, capacityPath+"?von=2026-01-05&bis=2026-12-28", ""), http.StatusBadRequest, "validation_error", "Zeitraum ist zu groß (höchstens 26 Wochen)")

	for role, allowed := range map[string]bool{"itest-pr-readonly": true, "sales": true, "admin": true, "itest-pr-writeonly": false, "hr": false, "inventory": false} {
		rec := doAuthedRequest(t, handler, c.user(role), http.MethodGet, capacityPath, "")
		if !allowed {
			wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		} else if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", role, rec.Code, rec.Body.String())
		}
	}

	two := newCatalogTenant(t, env, handler).user("admin")
	if other := decodeJSON[capacityResponse](t, doAuthedRequest(t, handler, two, http.MethodGet, capacityPath, ""), http.StatusOK); len(other.Arbeitsplaetze) != 0 {
		t.Fatalf("fremde Arbeitsplätze sichtbar: %+v", other)
	}
}
