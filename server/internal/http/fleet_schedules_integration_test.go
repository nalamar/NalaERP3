package apihttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Fuhrpark-Termine ueber HTTP (Backlog G.2.3). Helfer aus
// fleet_integration_test.go (vehiclesPath, decodeVehicle),
// qualification_types_integration_test.go (newCatalogTenant) und
// leave_requests_integration_test.go (seedRoleWithPermissions) und
// asset_inspections_integration_test.go (decodeJSON).

const (
	schedulesPath   = "/api/v1/fleet/schedules/"
	completionsPath = "/api/v1/fleet/completions/"
)

type scheduleResponse struct {
	ID              uuid.UUID `json:"id"`
	VehicleID       uuid.UUID `json:"vehicle_id"`
	Art             string    `json:"art"`
	Bezeichnung     string    `json:"bezeichnung"`
	Starttermin     *string   `json:"starttermin"`
	IntervallMonate *int      `json:"intervall_monate"`
	Notiz           string    `json:"notiz"`
	Aktiv           bool      `json:"aktiv"`
	Standing        struct {
		Status         string  `json:"status"`
		FaelligAm      *string `json:"faellig_am"`
		TageBisFaellig *int    `json:"tage_bis_faellig"`
	} `json:"standing"`
}

type completionResponse struct {
	ID              uuid.UUID `json:"id"`
	ScheduleID      uuid.UUID `json:"schedule_id"`
	Ergebnis        string    `json:"ergebnis"`
	NaechsterTermin *string   `json:"naechster_termin"`
	NaechsterKm     *int      `json:"naechster_km"`
	Status          string    `json:"status"`
	StornoReason    string    `json:"storno_reason"`
}

func fullSchedulePut(starttermin, notiz string, aktiv bool) string {
	s := "null"
	if starttermin != "" {
		s = `"` + starttermin + `"`
	}
	a := "false"
	if aktiv {
		a = "true"
	}
	return `{"starttermin":` + s + `,"start_km":null,"intervall_monate":12,"intervall_km":null,"notiz":"` + notiz + `","aktiv":` + a + `}`
}

func TestFleetSchedulesHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	now := hr.WorkDateOf(time.Now())
	v := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-H 1","art":"lkw"}`), http.StatusCreated)
	in10 := now.AddDate(0, 0, 10).Format("2006-01-02")

	hu := decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"hu","starttermin":"2030-05-07","notiz":"Plakette"}`), http.StatusCreated)
	if hu.Art != "hu" || hu.Starttermin == nil || (*hu.Starttermin)[:10] != "2030-05-31" || hu.IntervallMonate == nil || *hu.IntervallMonate != 24 {
		t.Fatalf("HU: %+v", hu)
	}
	uvv := decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"uvv","starttermin":"`+in10+`"}`), http.StatusCreated)
	decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"sonstige","bezeichnung":"Tachograph"}`), http.StatusCreated)

	list := decodeJSON[[]scheduleResponse](t, do(http.MethodGet, vehiclesPath+v.ID.String()+"/schedules", ""), http.StatusOK)
	got := make([]string, len(list))
	for i, s := range list {
		got[i] = s.Art + "=" + s.Standing.Status
	}
	if strings.Join(got, ",") != "hu=ok,uvv=faellig_bald,sonstige=ohne_termin" {
		t.Fatalf("Termine: %v", got)
	}
	// Kuerzere Vorwarnzeit: UVV in 10 Tagen ist "ok".
	if short := decodeJSON[[]scheduleResponse](t, do(http.MethodGet, vehiclesPath+v.ID.String()+"/schedules?warn_tage=5", ""), http.StatusOK); short[1].Standing.Status != "ok" {
		t.Fatalf("warn_tage: %+v", short[1].Standing)
	}
	due := decodeJSON[[]struct {
		Kennzeichen string `json:"kennzeichen"`
		Art         string `json:"art"`
	}](t, do(http.MethodGet, schedulesPath+"due", ""), http.StatusOK)
	if len(due) != 1 || due[0].Kennzeichen != "M-H 1" || due[0].Art != "uvv" {
		t.Fatalf("Faelligkeitsliste: %+v", due)
	}

	// Erledigen: Vorschlag aus dem Intervall; HU mit Maengeln sperrt.
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	ok1 := decodeJSON[completionResponse](t, do(http.MethodPost, completionsPath, `{"schedule_id":"`+uvv.ID.String()+`","erledigt_am":"`+yesterday+`","ergebnis":"bestanden","pruefer":"Werkstatt"}`), http.StatusCreated)
	if ok1.NaechsterTermin == nil || (*ok1.NaechsterTermin)[:10] != now.AddDate(1, 0, -1).Format("2006-01-02") {
		t.Fatalf("UVV-Vorschlag: %+v", ok1)
	}
	bad := decodeJSON[completionResponse](t, do(http.MethodPost, completionsPath, `{"schedule_id":"`+hu.ID.String()+`","erledigt_am":"`+yesterday+`","ergebnis":"maengel","naechster_termin":"`+in10+`"}`), http.StatusCreated)
	if bad.NaechsterTermin == nil || (*bad.NaechsterTermin)[:10] != endOfMonthString(now.AddDate(0, 0, 10)) {
		t.Fatalf("HU-Nachpruefung zum Monatsende: %+v", bad)
	}
	list = decodeJSON[[]scheduleResponse](t, do(http.MethodGet, vehiclesPath+v.ID.String()+"/schedules", ""), http.StatusOK)
	if list[0].Standing.Status != "gesperrt" || list[1].Standing.Status != "ok" {
		t.Fatalf("nach Erledigungen: %+v / %+v", list[0].Standing, list[1].Standing)
	}
	comps := decodeJSON[[]completionResponse](t, do(http.MethodGet, schedulesPath+hu.ID.String()+"/completions", ""), http.StatusOK)
	if len(comps) != 1 || comps[0].ID != bad.ID {
		t.Fatalf("Erledigungen: %+v", comps)
	}

	// Storno hebt die Sperre auf; zweiter Storno 409.
	st := decodeJSON[completionResponse](t, do(http.MethodPost, completionsPath+bad.ID.String()+"/storno", `{"reason":" falsches Fahrzeug "}`), http.StatusOK)
	if st.Status != "storniert" || st.StornoReason != "falsches Fahrzeug" {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, do(http.MethodPost, completionsPath+bad.ID.String()+"/storno", `{"reason":"x"}`), http.StatusConflict, "conflict", "Erledigung ist bereits storniert")
	if l := decodeJSON[[]scheduleResponse](t, do(http.MethodGet, vehiclesPath+v.ID.String()+"/schedules", ""), http.StatusOK); l[0].Standing.Status != "ok" {
		t.Fatalf("nach Storno: %+v", l[0].Standing)
	}
	if c := decodeJSON[[]completionResponse](t, do(http.MethodGet, schedulesPath+hu.ID.String()+"/completions", ""), http.StatusOK); len(c) != 0 {
		t.Fatalf("ohne stornierte: %+v", c)
	}
	if c := decodeJSON[[]completionResponse](t, do(http.MethodGet, schedulesPath+hu.ID.String()+"/completions?include_storniert=true", ""), http.StatusOK); len(c) != 1 || c[0].Status != "storniert" {
		t.Fatalf("mit stornierten: %+v", c)
	}

	// PUT ersetzt die Planung; stilllegen nimmt den Termin aus der Liste.
	upd := decodeJSON[scheduleResponse](t, do(http.MethodPut, schedulesPath+uvv.ID.String(), fullSchedulePut("", "neu", false)), http.StatusOK)
	if upd.Aktiv || upd.Notiz != "neu" || upd.Starttermin != nil || *upd.IntervallMonate != 12 {
		t.Fatalf("PUT: %+v", upd)
	}
	if l := decodeJSON[[]scheduleResponse](t, do(http.MethodGet, vehiclesPath+v.ID.String()+"/schedules", ""), http.StatusOK); len(l) != 2 {
		t.Fatalf("ohne stillgelegten: %d", len(l))
	}
	if l := decodeJSON[[]scheduleResponse](t, do(http.MethodGet, vehiclesPath+v.ID.String()+"/schedules?include_inactive=true", ""), http.StatusOK); len(l) != 3 {
		t.Fatalf("mit stillgelegtem: %d", len(l))
	}
	wantAPIError(t, do(http.MethodPost, completionsPath, `{"schedule_id":"`+uvv.ID.String()+`","erledigt_am":"`+yesterday+`","ergebnis":"bestanden"}`), http.StatusConflict, "conflict", "Termin ist stillgelegt")
}

func endOfMonthString(d time.Time) string {
	y, m, _ := d.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func TestFleetSchedulesHTTPErrors(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	v := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-E 1","art":"pkw","erstzulassung":"2020-06-01"}`), http.StatusCreated)
	hu := decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"hu"}`), http.StatusCreated)
	wartung := decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"wartung"}`), http.StatusCreated)
	gone := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-E 2","art":"pkw"}`), http.StatusCreated)
	goneUVV := decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+gone.ID.String()+`","art":"uvv"}`), http.StatusCreated)
	decodeVehicle(t, do(http.MethodPost, vehiclesPath+gone.ID.String()+"/deregister", `{"datum":"2026-01-02","grund":"verkauft"}`), http.StatusOK)
	other := newCatalogTenant(t, env, handler)
	otherToken := other.user("fleet")
	fv := decodeVehicle(t, doAuthedRequest(t, handler, otherToken, http.MethodPost, vehiclesPath, `{"kennzeichen":"F-1","art":"pkw"}`), http.StatusCreated)
	fs := decodeJSON[scheduleResponse](t, doAuthedRequest(t, handler, otherToken, http.MethodPost, schedulesPath, `{"vehicle_id":"`+fv.ID.String()+`","art":"hu"}`), http.StatusCreated)
	fc := decodeJSON[completionResponse](t, doAuthedRequest(t, handler, otherToken, http.MethodPost, completionsPath, `{"schedule_id":"`+fs.ID.String()+`","erledigt_am":"2026-01-02","ergebnis":"bestanden"}`), http.StatusCreated)
	tomorrow := hr.WorkDateOf(time.Now()).AddDate(0, 0, 1).Format("2006-01-02")
	vid, huID, wID := v.ID.String(), hu.ID.String(), wartung.ID.String()

	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code, msg                string
	}{
		{"Liste Fahrzeug-ID ungueltig", http.MethodGet, vehiclesPath + "abc/schedules", "", 400, "validation_error", "Ungültige Fahrzeug-ID"},
		{"Liste warn_tage ungueltig", http.MethodGet, vehiclesPath + vid + "/schedules?warn_tage=-1", "", 400, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)"},
		{"Liste fremdes Fahrzeug", http.MethodGet, vehiclesPath + fv.ID.String() + "/schedules", "", 404, "not_found", "Fahrzeug nicht gefunden"},
		{"due warn_tage ungueltig", http.MethodGet, schedulesPath + "due?warn_tage=x", "", 400, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)"},
		{"Termin kein JSON", http.MethodPost, schedulesPath, `{`, 400, "validation_error", "Ungültige Eingabe"},
		{"Termin ohne Fahrzeug", http.MethodPost, schedulesPath, `{"art":"hu"}`, 400, "validation_error", "vehicle_id erforderlich"},
		{"Termin Art unbekannt", http.MethodPost, schedulesPath, `{"vehicle_id":"` + vid + `","art":"tuev"}`, 400, "validation_error", "Ungültige Terminart"},
		{"Termin doppelt", http.MethodPost, schedulesPath, `{"vehicle_id":"` + vid + `","art":"hu"}`, 400, "validation_error", "Termin dieser Art existiert bereits für das Fahrzeug"},
		{"Termin Bezeichnung bei HU", http.MethodPost, schedulesPath, `{"vehicle_id":"` + vid + `","art":"uvv","bezeichnung":"x"}`, 400, "validation_error", "Bezeichnung nur bei sonstigen Terminen"},
		{"Termin Datum falsch", http.MethodPost, schedulesPath, `{"vehicle_id":"` + vid + `","art":"uvv","starttermin":"1.1.2027"}`, 400, "validation_error", "Ungültiges Datum in starttermin (erwartet JJJJ-MM-TT)"},
		{"Termin abgemeldet", http.MethodPost, schedulesPath, `{"vehicle_id":"` + gone.ID.String() + `","art":"hu"}`, 409, "conflict", "Fahrzeug ist abgemeldet"},
		{"Termin fremdes Fahrzeug", http.MethodPost, schedulesPath, `{"vehicle_id":"` + fv.ID.String() + `","art":"uvv"}`, 404, "not_found", "Fahrzeug nicht gefunden"},
		{"PUT ohne aktiv", http.MethodPut, schedulesPath + huID, strings.Replace(fullSchedulePut("", "", true), `,"aktiv":true`, "", 1), 400, "validation_error", "Feld erforderlich: aktiv"},
		{"PUT mit Art", http.MethodPut, schedulesPath + huID, strings.Replace(fullSchedulePut("", "", true), `"aktiv":true`, `"aktiv":true,"art":"uvv"`, 1), 400, "validation_error", "unbekanntes Feld: art"},
		{"PUT ID ungueltig", http.MethodPut, schedulesPath + "abc", fullSchedulePut("", "", true), 400, "validation_error", "Ungültige Termin-ID"},
		{"PUT fremder Termin", http.MethodPut, schedulesPath + fs.ID.String(), fullSchedulePut("", "", true), 404, "not_found", "Termin nicht gefunden"},
		{"PUT am abgemeldeten", http.MethodPut, schedulesPath + goneUVV.ID.String(), fullSchedulePut("", "", true), 409, "conflict", "Fahrzeug ist abgemeldet"},
		{"Erledigungen fremder Termin", http.MethodGet, schedulesPath + fs.ID.String() + "/completions", "", 404, "not_found", "Termin nicht gefunden"},
		{"Erledigung ohne Datum", http.MethodPost, completionsPath, `{"schedule_id":"` + huID + `","ergebnis":"bestanden"}`, 400, "validation_error", "Datum erforderlich"},
		{"Erledigung Datum falsch", http.MethodPost, completionsPath, `{"schedule_id":"` + huID + `","erledigt_am":"2.1.2026","ergebnis":"bestanden"}`, 400, "validation_error", "Ungültiges Datum in erledigt_am (erwartet JJJJ-MM-TT)"},
		{"Erledigung morgen", http.MethodPost, completionsPath, `{"schedule_id":"` + huID + `","erledigt_am":"` + tomorrow + `","ergebnis":"bestanden"}`, 400, "validation_error", "Datum darf nicht in der Zukunft liegen"},
		{"Erledigung vor Erstzulassung", http.MethodPost, completionsPath, `{"schedule_id":"` + huID + `","erledigt_am":"2020-05-31","ergebnis":"bestanden"}`, 400, "validation_error", "Datum darf nicht vor der Erstzulassung liegen"},
		{"Erledigung HU erledigt", http.MethodPost, completionsPath, `{"schedule_id":"` + huID + `","erledigt_am":"2026-01-02","ergebnis":"erledigt"}`, 400, "validation_error", "Ungültiges Prüfergebnis"},
		{"Erledigung Wartung bestanden", http.MethodPost, completionsPath, `{"schedule_id":"` + wID + `","erledigt_am":"2026-01-02","ergebnis":"bestanden"}`, 400, "validation_error", "Ungültiges Ergebnis (erwartet: erledigt)"},
		{"Erledigung Folgetermin davor", http.MethodPost, completionsPath, `{"schedule_id":"` + wID + `","erledigt_am":"2026-01-02","ergebnis":"erledigt","naechster_termin":"2026-01-01"}`, 400, "validation_error", "Folgetermin muss nach dem Datum liegen"},
		{"Erledigung Folge-km", http.MethodPost, completionsPath, `{"schedule_id":"` + wID + `","erledigt_am":"2026-01-02","ergebnis":"erledigt","km_stand":5000,"naechster_km":5000}`, 400, "validation_error", "Folgetermin nach Kilometern muss über dem Kilometerstand liegen"},
		{"Erledigung abgemeldet", http.MethodPost, completionsPath, `{"schedule_id":"` + goneUVV.ID.String() + `","erledigt_am":"2026-01-01","ergebnis":"bestanden"}`, 409, "conflict", "Fahrzeug ist abgemeldet"},
		{"Erledigung fremder Termin", http.MethodPost, completionsPath, `{"schedule_id":"` + fs.ID.String() + `","erledigt_am":"2026-01-02","ergebnis":"bestanden"}`, 404, "not_found", "Termin nicht gefunden"},
		{"Storno ohne Grund", http.MethodPost, completionsPath + fc.ID.String() + "/storno", `{"reason":" "}`, 400, "validation_error", "Storno-Begründung erforderlich"},
		{"Storno fremde Erledigung", http.MethodPost, completionsPath + fc.ID.String() + "/storno", `{"reason":"x"}`, 404, "not_found", "Erledigung nicht gefunden"},
		{"Storno ID ungueltig", http.MethodPost, completionsPath + "abc/storno", `{"reason":"x"}`, 400, "validation_error", "Ungültige Erledigungs-ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantAPIError(t, do(tc.method, tc.path, tc.body), tc.status, tc.code, tc.msg)
		})
	}
	if comps := decodeJSON[[]completionResponse](t, do(http.MethodGet, schedulesPath+huID+"/completions?include_storniert=true", ""), http.StatusOK); len(comps) != 0 {
		t.Fatalf("nach Fehlern Erledigungen: %+v", comps)
	}
	if comps := decodeJSON[[]completionResponse](t, doAuthedRequest(t, handler, otherToken, http.MethodGet, schedulesPath+fs.ID.String()+"/completions", ""), http.StatusOK); len(comps) != 1 || comps[0].Status != "active" {
		t.Fatalf("fremde Erledigung veraendert: %+v", comps)
	}
}

func TestFleetSchedulesHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	admin := c.user("admin")
	v := decodeVehicle(t, doAuthedRequest(t, handler, admin, http.MethodPost, vehiclesPath, `{"kennzeichen":"M-P 1","art":"pkw"}`), http.StatusCreated)
	hu := decodeJSON[scheduleResponse](t, doAuthedRequest(t, handler, admin, http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"hu"}`), http.StatusCreated)
	comp := decodeJSON[completionResponse](t, doAuthedRequest(t, handler, admin, http.MethodPost, completionsPath, `{"schedule_id":"`+hu.ID.String()+`","erledigt_am":"2026-01-02","ergebnis":"bestanden"}`), http.StatusCreated)
	seedRoleWithPermissions(t, env, "itest-fleet-read", "fleet.read")

	reads := []string{vehiclesPath + v.ID.String() + "/schedules", schedulesPath + "due", schedulesPath + hu.ID.String() + "/completions"}
	writes := []struct{ method, path, body string }{
		{http.MethodPost, schedulesPath, `{"vehicle_id":"` + v.ID.String() + `","art":"uvv"}`},
		{http.MethodPut, schedulesPath + hu.ID.String(), fullSchedulePut("", "x", false)},
		{http.MethodPost, completionsPath, `{"schedule_id":"` + hu.ID.String() + `","erledigt_am":"2026-01-03","ergebnis":"bestanden"}`},
		{http.MethodPost, completionsPath + comp.ID.String() + "/storno", `{"reason":"x"}`},
	}
	for _, tc := range []struct {
		role    string
		canRead bool
	}{{"hr", false}, {"production", false}, {"itest-fleet-read", true}} {
		token := c.user(tc.role)
		for _, p := range reads {
			rec := doAuthedRequest(t, handler, token, http.MethodGet, p, "")
			if tc.canRead && rec.Code != http.StatusOK {
				t.Fatalf("%s GET %s: %d %s", tc.role, p, rec.Code, rec.Body.String())
			}
			if !tc.canRead {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
			}
		}
		for _, w := range writes {
			wantAPIError(t, doAuthedRequest(t, handler, token, w.method, w.path, w.body), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		}
	}
	l := decodeJSON[[]scheduleResponse](t, doAuthedRequest(t, handler, admin, http.MethodGet, vehiclesPath+v.ID.String()+"/schedules?include_inactive=true", ""), http.StatusOK)
	if len(l) != 1 || !l[0].Aktiv {
		t.Fatalf("ohne Recht veraendert: %+v", l)
	}
	if cs := decodeJSON[[]completionResponse](t, doAuthedRequest(t, handler, admin, http.MethodGet, schedulesPath+hu.ID.String()+"/completions?include_storniert=true", ""), http.StatusOK); len(cs) != 1 || cs[0].Status != "active" {
		t.Fatalf("ohne Recht erfasst/storniert: %+v", cs)
	}
}

func TestClassifyDomainErrorCoversFleetScheduleMessages(t *testing.T) {
	cases := map[string]int{
		"Fahrzeug ist abgemeldet":                                         http.StatusConflict,
		"Termin ist stillgelegt":                                          http.StatusConflict,
		"Erledigung ist bereits storniert":                                http.StatusConflict,
		"Termin nicht gefunden":                                           http.StatusNotFound,
		"Erledigung nicht gefunden":                                       http.StatusNotFound,
		"Termin dieser Art existiert bereits für das Fahrzeug":            http.StatusBadRequest,
		"Ungültige Terminart":                                             http.StatusBadRequest,
		"Bezeichnung erforderlich":                                        http.StatusBadRequest,
		"Bezeichnung nur bei sonstigen Terminen":                          http.StatusBadRequest,
		"Kilometerstand muss größer als 0 sein":                           http.StatusBadRequest,
		"Intervall muss größer als 0 sein":                                http.StatusBadRequest,
		"Datum erforderlich":                                              http.StatusBadRequest,
		"Datum darf nicht in der Zukunft liegen":                          http.StatusBadRequest,
		"Datum darf nicht vor der Erstzulassung liegen":                   http.StatusBadRequest,
		"Ungültiges Prüfergebnis":                                         http.StatusBadRequest,
		"Ungültiges Ergebnis (erwartet: erledigt)":                        http.StatusBadRequest,
		"Folgetermin muss nach dem Datum liegen":                          http.StatusBadRequest,
		"Kilometerstand darf nicht negativ sein":                          http.StatusBadRequest,
		"Folgetermin nach Kilometern muss über dem Kilometerstand liegen": http.StatusBadRequest,
		"Storno-Begründung erforderlich":                                  http.StatusBadRequest,
		"Erledigungs-ID erforderlich":                                     http.StatusBadRequest,
		"schedule_id erforderlich":                                        http.StatusBadRequest,
		"vehicle_id erforderlich":                                         http.StatusBadRequest,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
