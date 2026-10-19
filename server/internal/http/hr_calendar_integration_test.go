package apihttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Feiertagskalender ueber HTTP (Backlog F.8.2.2). Helfer newCatalogTenant,
// doAuthedRequest, wantAPIError, seedRoleWithPermissions aus den
// Testdateien der Qualifikationen, Zeiterfassung und Urlaubsantraege.

type calendarSettingsResponse struct {
	Bundesland             string  `json:"bundesland"`
	MariaeHimmelfahrt      bool    `json:"mariae_himmelfahrt"`
	AugsburgerFriedensfest bool    `json:"augsburger_friedensfest"`
	UpdatedAt              *string `json:"updated_at"`
}

type calendarResponse struct {
	Year       int    `json:"year"`
	Bundesland string `json:"bundesland"`
	Days       []struct {
		Date string `json:"date"`
		Name string `json:"name"`
		Art  string `json:"art"`
	} `json:"days"`
}

type companyDayResponse struct {
	ID    uuid.UUID `json:"id"`
	Datum string    `json:"datum"`
	Name  string    `json:"name"`
}

func decodeJSONStatus(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, out any) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
}

// dayOf liefert Name und Art des Tages im Kalender ("" wenn nicht frei).
func (c calendarResponse) dayOf(date string) (string, string) {
	for _, d := range c.Days {
		if strings.HasPrefix(d.Date, date) {
			return d.Name, d.Art
		}
	}
	return "", ""
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", status, rec.Code, rec.Body.String())
	}
}

func TestHRCalendarSettingsAndHolidays(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "kalender-ohne-rechte")
	admin := tenant.user("admin")
	hrUser := tenant.user("hr")
	plain := tenant.user("kalender-ohne-rechte")
	otherAdmin := other.user("admin")
	const settings = "/api/v1/settings/hr-calendar/"
	const holidays = "/api/v1/holidays/"

	var st calendarSettingsResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, admin, http.MethodGet, settings, ""), http.StatusOK, &st)
	if st.Bundesland != "" || st.MariaeHimmelfahrt || st.UpdatedAt != nil {
		t.Fatalf("neuer Mandant hat schon eine Einstellung: %+v", st)
	}

	// Ohne Bundesland: nur die neun bundesweiten Feiertage - fuer jeden
	// angemeldeten Benutzer, auch ohne jedes Recht.
	var cal calendarResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, plain, http.MethodGet, holidays+"?year=2026", ""), http.StatusOK, &cal)
	if cal.Year != 2026 || cal.Bundesland != "" || len(cal.Days) != 9 {
		t.Fatalf("Kalender ohne Bundesland: %+v", cal)
	}
	for _, d := range cal.Days {
		if d.Art != "gesetzlich" {
			t.Fatalf("Art %q, erwartet gesetzlich: %+v", d.Art, d)
		}
	}
	if name, _ := cal.dayOf("2026-06-04"); name != "" {
		t.Fatalf("Fronleichnam ohne Bundesland: %q", name)
	}

	// Einstellung nur mit settings.manage.
	wantStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodPut, settings, `{"bundesland":"BY"}`), http.StatusForbidden)
	wantStatus(t, doAuthedRequest(t, handler, plain, http.MethodGet, settings, ""), http.StatusForbidden)

	// Kleinschreibung wird angenommen; Bayern mit Mariae Himmelfahrt.
	decodeJSONStatus(t, doAuthedRequest(t, handler, admin, http.MethodPut, settings, `{"bundesland":" by ","mariae_himmelfahrt":true}`), http.StatusOK, &st)
	if st.Bundesland != "BY" || !st.MariaeHimmelfahrt || st.AugsburgerFriedensfest || st.UpdatedAt == nil {
		t.Fatalf("Einstellung nicht gespeichert: %+v", st)
	}
	cal = calendarResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, plain, http.MethodGet, holidays+"?year=2026", ""), http.StatusOK, &cal)
	if cal.Bundesland != "BY" || len(cal.Days) != 13 {
		t.Fatalf("Kalender Bayern: %d Tage, %+v", len(cal.Days), cal)
	}
	if name, _ := cal.dayOf("2026-08-15"); name != "Mariä Himmelfahrt" {
		t.Fatalf("Mariä Himmelfahrt fehlt: %+v", cal)
	}
	if name, _ := cal.dayOf("2026-06-04"); name != "Fronleichnam" {
		t.Fatalf("Fronleichnam fehlt: %+v", cal)
	}

	// Ungueltiges Land: abgelehnt, Einstellung unveraendert.
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPut, settings, `{"bundesland":"XX"}`), http.StatusBadRequest, "validation_error", "Ungültiges Bundesland")
	wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodPut, settings, `[]`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	decodeJSONStatus(t, doAuthedRequest(t, handler, admin, http.MethodGet, settings, ""), http.StatusOK, &st)
	if st.Bundesland != "BY" || !st.MariaeHimmelfahrt {
		t.Fatalf("abgelehnte Eingabe hat die Einstellung veraendert: %+v", st)
	}

	// Der andere Mandant ist unberuehrt.
	var otherSt calendarSettingsResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, otherAdmin, http.MethodGet, settings, ""), http.StatusOK, &otherSt)
	if otherSt.Bundesland != "" {
		t.Fatalf("Einstellung ist in einen anderen Mandanten gelangt: %+v", otherSt)
	}

	// Ungueltige Jahre.
	for _, q := range []string{"?year=abc", "?year=1800", "?year=2201"} {
		wantAPIError(t, doAuthedRequest(t, handler, plain, http.MethodGet, holidays+q, ""), http.StatusBadRequest, "validation_error", "Ungültiges Jahr")
	}
	// Ohne Jahr: das laufende.
	cal = calendarResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, plain, http.MethodGet, holidays, ""), http.StatusOK, &cal)
	if cal.Year < 2026 || len(cal.Days) < 9 {
		t.Fatalf("Kalender ohne Jahr: %+v", cal)
	}

	// Zuruecksetzen: wieder nur bundesweite Feiertage.
	decodeJSONStatus(t, doAuthedRequest(t, handler, admin, http.MethodPut, settings, `{"bundesland":""}`), http.StatusOK, &st)
	if st.Bundesland != "" || st.MariaeHimmelfahrt {
		t.Fatalf("Zuruecksetzen: %+v", st)
	}
	cal = calendarResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, plain, http.MethodGet, holidays+"?year=2026", ""), http.StatusOK, &cal)
	if len(cal.Days) != 9 {
		t.Fatalf("nach Zuruecksetzen %d Tage", len(cal.Days))
	}
}

func TestHRCalendarCompanyDays(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "kalender-nur-self", "leave_requests.self")
	hrUser := tenant.user("hr")
	self := tenant.user("kalender-nur-self")
	otherHR := other.user("hr")
	const days = "/api/v1/holidays/company-days"

	var created companyDayResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodPost, days, `{"datum":"2026-12-24","name":" Heiligabend "}`), http.StatusCreated, &created)
	if created.ID == uuid.Nil || created.Name != "Heiligabend" || !strings.HasPrefix(created.Datum, "2026-12-24") {
		t.Fatalf("Zusatztag nicht wie angelegt: %+v", created)
	}

	wantAPIError(t, doAuthedRequest(t, handler, hrUser, http.MethodPost, days, `{"datum":"2026-12-24","name":"doppelt"}`),
		http.StatusConflict, "conflict", "Betrieblicher freier Tag für dieses Datum ist bereits erfasst")
	wantAPIError(t, doAuthedRequest(t, handler, hrUser, http.MethodPost, days, `{"name":"ohne Datum"}`), http.StatusBadRequest, "validation_error", "Datum erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, hrUser, http.MethodPost, days, `{"datum":"24.12.2026","name":"x"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges Datum in datum (erwartet JJJJ-MM-TT)")
	wantAPIError(t, doAuthedRequest(t, handler, hrUser, http.MethodPost, days, `{"datum":"2026-12-31","name":"  "}`), http.StatusBadRequest, "validation_error", "Bezeichnung erforderlich")

	// Im Kalender als betrieblicher Tag, vor dem 1. Weihnachtstag.
	var cal calendarResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, self, http.MethodGet, "/api/v1/holidays/?year=2026", ""), http.StatusOK, &cal)
	if name, art := cal.dayOf("2026-12-24"); name != "Heiligabend" || art != "betrieblich" {
		t.Fatalf("Heiligabend im Kalender: %q/%q", name, art)
	}
	if len(cal.Days) != 10 || !strings.HasPrefix(cal.Days[7].Date, "2026-12-24") || cal.Days[8].Name != "1. Weihnachtsfeiertag" {
		t.Fatalf("Kalender nicht sortiert oder unvollstaendig: %+v", cal.Days)
	}

	var list []companyDayResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodGet, days+"?year=2026", ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("Liste 2026: %+v", list)
	}
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodGet, days+"?year=2027", ""), http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("Liste 2027: %+v", list)
	}
	// Ein Jahr davor: der Tag liegt nach dem 31.12.2025, also nicht dabei.
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodGet, days+"?year=2025", ""), http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("Liste 2025: %+v", list)
	}

	// Pflegen nur mit leave_requests.decide.
	wantStatus(t, doAuthedRequest(t, handler, self, http.MethodPost, days, `{"datum":"2026-12-31","name":"Silvester"}`), http.StatusForbidden)
	wantStatus(t, doAuthedRequest(t, handler, self, http.MethodGet, days, ""), http.StatusForbidden)
	wantStatus(t, doAuthedRequest(t, handler, self, http.MethodDelete, days+"/"+created.ID.String(), ""), http.StatusForbidden)

	// Fremder Mandant: sieht den Tag nicht und kann ihn nicht loeschen.
	wantAPIError(t, doAuthedRequest(t, handler, otherHR, http.MethodDelete, days+"/"+created.ID.String(), ""),
		http.StatusNotFound, "not_found", "Betrieblicher freier Tag nicht gefunden")
	cal = calendarResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, otherHR, http.MethodGet, "/api/v1/holidays/?year=2026", ""), http.StatusOK, &cal)
	if name, _ := cal.dayOf("2026-12-24"); name != "" {
		t.Fatalf("Zusatztag ist in einen anderen Mandanten gelangt: %q", name)
	}
	// Derselbe Tag ist im anderen Mandanten frei anlegbar.
	wantStatus(t, doAuthedRequest(t, handler, otherHR, http.MethodPost, days, `{"datum":"2026-12-24","name":"Heiligabend"}`), http.StatusCreated)

	wantAPIError(t, doAuthedRequest(t, handler, hrUser, http.MethodDelete, days+"/kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige ID")
	wantStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodDelete, days+"/"+created.ID.String(), ""), http.StatusNoContent)
	wantAPIError(t, doAuthedRequest(t, handler, hrUser, http.MethodDelete, days+"/"+created.ID.String(), ""),
		http.StatusNotFound, "not_found", "Betrieblicher freier Tag nicht gefunden")
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrUser, http.MethodGet, days+"?year=2026", ""), http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("nach Loeschen: %+v", list)
	}
}
