package apihttp

import (
	"net/http"
	"reflect"
	"testing"

	"nalaerp3/internal/testutil"
)

// Bundesland und Arbeitstage am Mitarbeiter ueber HTTP, und die Zaehlung
// eines Urlaubsantrags danach (Backlog F.8.2.3).

type workweekEmployee struct {
	ID          string  `json:"id"`
	Bundesland  *string `json:"bundesland"`
	Arbeitstage []int   `json:"arbeitstage"`
}

func TestEmployeeWorkweekHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	const emps = "/api/v1/employees/"

	var std workweekEmployee
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, emps, `{"first_name":"Voll","last_name":"Zeit"}`), http.StatusCreated, &std)
	if std.Bundesland != nil || !reflect.DeepEqual(std.Arbeitstage, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("Vorgaben: %+v", std)
	}
	var part workweekEmployee
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, emps,
		`{"first_name":"Teil","last_name":"Zeit","bundesland":"nw","arbeitstage":[3,1,2]}`), http.StatusCreated, &part)
	if part.Bundesland == nil || *part.Bundesland != "NW" || !reflect.DeepEqual(part.Arbeitstage, []int{1, 2, 3}) {
		t.Fatalf("Teilzeit: %+v", part)
	}

	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, emps, `{"first_name":"a","last_name":"b","arbeitstage":[]}`),
		http.StatusBadRequest, "validation_error", "Mindestens ein Arbeitstag erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, emps, `{"first_name":"a","last_name":"b","arbeitstage":[1,9]}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Arbeitstag: 9 (1 = Montag bis 7 = Sonntag)")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, emps, `{"first_name":"a","last_name":"b","bundesland":"XX"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges Bundesland")

	var patched workweekEmployee
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, emps+std.ID, `{"bundesland":"sn","arbeitstage":[5,4]}`), http.StatusOK, &patched)
	if patched.Bundesland == nil || *patched.Bundesland != "SN" || !reflect.DeepEqual(patched.Arbeitstage, []int{4, 5}) {
		t.Fatalf("Patch: %+v", patched)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, emps+std.ID, `{"arbeitstage":"1-5"}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Wert für Feld: arbeitstage")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, emps+std.ID, `{"arbeitstage":[1.5]}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Wert für Feld: arbeitstage")
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, emps+std.ID, `{"bundesland":null}`), http.StatusOK, &patched)
	if patched.Bundesland != nil || !reflect.DeepEqual(patched.Arbeitstage, []int{4, 5}) {
		t.Fatalf("Zuruecksetzen: %+v", patched)
	}

	// Urlaubsantrag fuer den Mitarbeiter mit Do/Fr: Woche 01.-07.06.2026
	// in NW (Fronleichnam Do) ergibt nur den Freitag.
	decodeJSONStatus(t, doAuthedRequest(t, handler, tenant.user("admin"), http.MethodPut, "/api/v1/settings/hr-calendar/", `{"bundesland":"NW"}`), http.StatusOK, &calendarSettingsResponse{})
	var lr struct {
		Days float64 `json:"days"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, "/api/v1/leave-requests/",
		`{"employee_id":"`+std.ID+`","start_date":"2026-06-01","end_date":"2026-06-07"}`), http.StatusCreated, &lr)
	if lr.Days != 1 {
		t.Fatalf("Antrag %v Tage, erwartet 1", lr.Days)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, "/api/v1/leave-requests/",
		`{"employee_id":"`+part.ID+`","start_date":"2026-06-04","end_date":"2026-06-07"}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Zeitraum: er enthält keinen Arbeitstag")
}
