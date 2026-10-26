package apihttp

import (
	"context"
	"encoding/json"
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

// Fahrzeugstamm ueber HTTP (Backlog G.1.3). Helfer aus
// qualification_types_integration_test.go (newCatalogTenant) und
// leave_requests_integration_test.go (seedRoleWithPermissions).

const vehiclesPath = "/api/v1/fleet/vehicles/"

type vehicleResponse struct {
	ID                      uuid.UUID `json:"id"`
	Kennzeichen             string    `json:"kennzeichen"`
	Art                     string    `json:"art"`
	Bezeichnung             string    `json:"bezeichnung"`
	FIN                     *string   `json:"fin"`
	Erstzulassung           *string   `json:"erstzulassung"`
	ZulaessigeGesamtmasseKg *int      `json:"zulaessige_gesamtmasse_kg"`
	KostenstelleID          *string   `json:"kostenstelle_id"`
	Fahrtenbuch             bool      `json:"fahrtenbuch"`
	Notiz                   string    `json:"notiz"`
	AbgemeldetAm            *string   `json:"abgemeldet_am"`
	AbgemeldetGrund         string    `json:"abgemeldet_grund"`
}

func decodeVehicle(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) vehicleResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out vehicleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeVehicles(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []vehicleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	ks := make([]string, len(out))
	for i, v := range out {
		ks[i] = v.Kennzeichen
	}
	return ks
}

// fullVehicle: ein vollstaendiger PUT-Body.
func fullVehicle(kennzeichen, art string) string {
	return `{"kennzeichen":"` + kennzeichen + `","art":"` + art + `","bezeichnung":"","hersteller":"","modell":"","fin":null,"erstzulassung":null,` +
		`"zulaessige_gesamtmasse_kg":null,"standort":"","kostenstelle_id":null,"fahrtenbuch":false,"notiz":""}`
}

func TestFleetVehiclesHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}

	bus := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":" m - ab  1234 ","art":"transporter","bezeichnung":"Montagebus",
		"fin":"wvwzzz1jz3w386752","erstzulassung":"2021-03-15","zulaessige_gesamtmasse_kg":3500,"fahrtenbuch":true,"notiz":"Dachträger"}`), http.StatusCreated)
	if bus.Kennzeichen != "M-AB 1234" || bus.FIN == nil || *bus.FIN != "WVWZZZ1JZ3W386752" || bus.Erstzulassung == nil || (*bus.Erstzulassung)[:10] != "2021-03-15" ||
		bus.ZulaessigeGesamtmasseKg == nil || *bus.ZulaessigeGesamtmasseKg != 3500 || !bus.Fahrtenbuch || bus.AbgemeldetAm != nil {
		t.Fatalf("angelegt: %+v", bus)
	}
	anh := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 99","art":"anhaenger"}`), http.StatusCreated)
	decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"A-B 1","art":"pkw"}`), http.StatusCreated)

	if got := decodeVehicle(t, do(http.MethodGet, vehiclesPath+bus.ID.String(), ""), http.StatusOK); got.Bezeichnung != "Montagebus" {
		t.Fatalf("Einzelabruf: %+v", got)
	}
	if got := strings.Join(decodeVehicles(t, do(http.MethodGet, vehiclesPath, "")), ","); got != "A-B 1,M-AB 1234,M-AB 99" {
		t.Fatalf("Liste nach Kennzeichen: %s", got)
	}
	if got := strings.Join(decodeVehicles(t, do(http.MethodGet, vehiclesPath+"?art=anhaenger", "")), ","); got != "M-AB 99" {
		t.Fatalf("Filter Art: %s", got)
	}

	// PUT ersetzt alles: weggelassene Werte als null/leer entfernen sie.
	upd := decodeVehicle(t, do(http.MethodPut, vehiclesPath+bus.ID.String(), fullVehicle("m-xy 1", "transporter")), http.StatusOK)
	if upd.Kennzeichen != "M-XY 1" || upd.FIN != nil || upd.Erstzulassung != nil || upd.Fahrtenbuch || upd.Notiz != "" {
		t.Fatalf("geaendert: %+v", upd)
	}

	// Abmelden: einmalig.
	dereg := decodeVehicle(t, do(http.MethodPost, vehiclesPath+anh.ID.String()+"/deregister", `{"datum":"2026-01-15","grund":" verkauft "}`), http.StatusOK)
	if dereg.AbgemeldetAm == nil || (*dereg.AbgemeldetAm)[:10] != "2026-01-15" || dereg.AbgemeldetGrund != "verkauft" {
		t.Fatalf("abgemeldet: %+v", dereg)
	}
	wantAPIError(t, do(http.MethodPost, vehiclesPath+anh.ID.String()+"/deregister", `{"datum":"2026-01-16","grund":"x"}`), http.StatusConflict, "conflict", "Fahrzeug ist bereits abgemeldet")
	if got := strings.Join(decodeVehicles(t, do(http.MethodGet, vehiclesPath, "")), ","); got != "A-B 1,M-XY 1" {
		t.Fatalf("aktive Liste: %s", got)
	}
	if got := strings.Join(decodeVehicles(t, do(http.MethodGet, vehiclesPath+"?include_abgemeldete=true", "")), ","); got != "A-B 1,M-AB 99,M-XY 1" {
		t.Fatalf("mit abgemeldeten: %s", got)
	}
	// Das Kennzeichen des abgemeldeten Anhaengers ist wieder frei.
	decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 99","art":"anhaenger"}`), http.StatusCreated)
}

func TestFleetVehiclesHTTPErrors(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	other := newCatalogTenant(t, env, handler)
	foreign := decodeVehicle(t, doAuthedRequest(t, handler, other.user("fleet"), http.MethodPost, vehiclesPath, `{"kennzeichen":"F-X 1","art":"pkw"}`), http.StatusCreated)
	foreignCC := "cc-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO cost_centers (id, company_id, code, name) VALUES ($1,$2,$1,'Fremd')`, foreignCC, other.company); err != nil {
		t.Fatalf("Kostenstelle: %v", err)
	}
	v := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 1","art":"pkw","erstzulassung":"2020-06-01"}`), http.StatusCreated)
	path := vehiclesPath + v.ID.String()
	tomorrow := hr.WorkDateOf(time.Now()).AddDate(0, 0, 1).Format("2006-01-02")

	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code, msg                string
	}{
		{"kein JSON", http.MethodPost, vehiclesPath, `{`, 400, "validation_error", "Ungültige Eingabe"},
		{"Datum falsch geschrieben", http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"pkw","erstzulassung":"15.03.2021"}`, 400, "validation_error", "Ungültiges Datum in erstzulassung (erwartet JJJJ-MM-TT)"},
		{"Kennzeichen fehlt", http.MethodPost, vehiclesPath, `{"art":"pkw"}`, 400, "validation_error", "Kennzeichen erforderlich"},
		{"Kennzeichen ungueltig", http.MethodPost, vehiclesPath, `{"kennzeichen":"M_AB 2","art":"pkw"}`, 400, "validation_error", "Ungültiges Kennzeichen (Buchstaben, Ziffern, Bindestrich und Leerzeichen, höchstens 15 Zeichen)"},
		{"Stapler", http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"stapler"}`, 400, "validation_error", "Ungültige Fahrzeugart"},
		{"FIN ungueltig", http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"pkw","fin":"123"}`, 400, "validation_error", "Ungültige FIN (17 Zeichen, Ziffern und Buchstaben ohne I, O, Q)"},
		{"Erstzulassung morgen", http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"pkw","erstzulassung":"` + tomorrow + `"}`, 400, "validation_error", "Erstzulassung darf nicht in der Zukunft liegen"},
		{"Masse 0", http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"pkw","zulaessige_gesamtmasse_kg":0}`, 400, "validation_error", "Zulässige Gesamtmasse muss größer als 0 sein"},
		{"Kennzeichen doppelt", http.MethodPost, vehiclesPath, `{"kennzeichen":"m-ab 1","art":"lkw"}`, 400, "validation_error", "Fahrzeug mit diesem Kennzeichen existiert bereits"},
		{"fremde Kostenstelle", http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"pkw","kostenstelle_id":"` + foreignCC + `"}`, 404, "not_found", "Kostenstelle nicht gefunden"},
		{"Liste ungueltige Art", http.MethodGet, vehiclesPath + "?art=bagger", "", 400, "validation_error", "Ungültige Fahrzeugart"},
		{"ID ungueltig", http.MethodGet, vehiclesPath + "abc", "", 400, "validation_error", "Ungültige Fahrzeug-ID"},
		{"unbekannt", http.MethodGet, vehiclesPath + uuid.NewString(), "", 404, "not_found", "Fahrzeug nicht gefunden"},
		{"fremder Mandant lesen", http.MethodGet, vehiclesPath + foreign.ID.String(), "", 404, "not_found", "Fahrzeug nicht gefunden"},
		{"fremder Mandant aendern", http.MethodPut, vehiclesPath + foreign.ID.String(), fullVehicle("F-X 2", "pkw"), 404, "not_found", "Fahrzeug nicht gefunden"},
		{"fremder Mandant abmelden", http.MethodPost, vehiclesPath + foreign.ID.String() + "/deregister", `{"datum":"2026-01-01","grund":"x"}`, 404, "not_found", "Fahrzeug nicht gefunden"},
		{"PUT ohne fahrtenbuch", http.MethodPut, path, strings.Replace(fullVehicle("M-AB 1", "pkw"), `"fahrtenbuch":false,`, "", 1), 400, "validation_error", "Feld erforderlich: fahrtenbuch"},
		{"PUT ohne kostenstelle_id", http.MethodPut, path, strings.Replace(fullVehicle("M-AB 1", "pkw"), `"kostenstelle_id":null,`, "", 1), 400, "validation_error", "Feld erforderlich: kostenstelle_id"},
		{"PUT unbekanntes Feld", http.MethodPut, path, strings.Replace(fullVehicle("M-AB 1", "pkw"), `"notiz":""`, `"notiz":"","farbe":"rot"`, 1), 400, "validation_error", "unbekanntes Feld: farbe"},
		{"PUT mit Abmeldung", http.MethodPut, path, strings.Replace(fullVehicle("M-AB 1", "pkw"), `"notiz":""`, `"notiz":"","abgemeldet_am":"2026-01-01"`, 1), 400, "validation_error", "unbekanntes Feld: abgemeldet_am"},
		{"Abmelden ohne Datum", http.MethodPost, path + "/deregister", `{"grund":"x"}`, 400, "validation_error", "Abmeldedatum erforderlich"},
		{"Abmelden Datum falsch", http.MethodPost, path + "/deregister", `{"datum":"1.1.2026","grund":"x"}`, 400, "validation_error", "Ungültiges Datum in datum (erwartet JJJJ-MM-TT)"},
		{"Abmelden ohne Grund", http.MethodPost, path + "/deregister", `{"datum":"2026-01-01"}`, 400, "validation_error", "Grund der Abmeldung erforderlich"},
		{"Abmelden morgen", http.MethodPost, path + "/deregister", `{"datum":"` + tomorrow + `","grund":"x"}`, 400, "validation_error", "Abmeldedatum darf nicht in der Zukunft liegen"},
		{"Abmelden vor Erstzulassung", http.MethodPost, path + "/deregister", `{"datum":"2020-05-31","grund":"x"}`, 400, "validation_error", "Abmeldedatum darf nicht vor der Erstzulassung liegen"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantAPIError(t, do(tc.method, tc.path, tc.body), tc.status, tc.code, tc.msg)
		})
	}
	if got := decodeVehicle(t, do(http.MethodGet, path, ""), http.StatusOK); got.AbgemeldetAm != nil || got.Kennzeichen != "M-AB 1" {
		t.Fatalf("nach Fehlern veraendert: %+v", got)
	}
	if got := decodeVehicle(t, doAuthedRequest(t, handler, other.user("fleet"), http.MethodGet, vehiclesPath+foreign.ID.String(), ""), http.StatusOK); got.Kennzeichen != "F-X 1" || got.AbgemeldetAm != nil {
		t.Fatalf("fremdes Fahrzeug veraendert: %+v", got)
	}
}

// Nur Fuhrpark und Admin lesen und schreiben - nicht die Personalabteilung
// (Entscheidung des Nutzers vom 2026-10-25). Eine Testrolle nur mit
// fleet.read belegt die Trennung von Lesen und Schreiben.
func TestFleetVehiclesHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	v := decodeVehicle(t, doAuthedRequest(t, handler, c.user("admin"), http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 1","art":"pkw"}`), http.StatusCreated)
	seedRoleWithPermissions(t, env, "itest-fleet-read", "fleet.read")

	writes := []struct{ method, path, body string }{
		{http.MethodPost, vehiclesPath, `{"kennzeichen":"M-AB 2","art":"pkw"}`},
		{http.MethodPut, vehiclesPath + v.ID.String(), fullVehicle("M-AB 3", "pkw")},
		{http.MethodPost, vehiclesPath + v.ID.String() + "/deregister", `{"datum":"2026-01-01","grund":"x"}`},
	}
	for _, tc := range []struct {
		role              string
		canRead, canWrite bool
	}{
		{"hr", false, false},
		{"production", false, false},
		{"itest-fleet-read", true, false},
	} {
		token := c.user(tc.role)
		for _, p := range []string{vehiclesPath, vehiclesPath + v.ID.String()} {
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
	if got := decodeVehicle(t, doAuthedRequest(t, handler, c.user("fleet"), http.MethodGet, vehiclesPath+v.ID.String(), ""), http.StatusOK); got.Kennzeichen != "M-AB 1" || got.AbgemeldetAm != nil {
		t.Fatalf("ohne Recht veraendert: %+v", got)
	}
	if got := decodeVehicles(t, doAuthedRequest(t, handler, c.user("fleet"), http.MethodGet, vehiclesPath, "")); len(got) != 1 {
		t.Fatalf("ohne Recht angelegt: %v", got)
	}
}

func TestClassifyDomainErrorCoversFleetMessages(t *testing.T) {
	cases := map[string]int{
		"Fahrzeug ist bereits abgemeldet":                      http.StatusConflict,
		"Fahrzeug nicht gefunden":                              http.StatusNotFound,
		"Fahrzeug mit diesem Kennzeichen existiert bereits":    http.StatusBadRequest,
		"Fahrzeug mit dieser FIN existiert bereits":            http.StatusBadRequest,
		"Erstzulassung darf nicht nach der Abmeldung liegen":   http.StatusBadRequest,
		"Abmeldedatum darf nicht vor der Erstzulassung liegen": http.StatusBadRequest,
		"Grund der Abmeldung erforderlich":                     http.StatusBadRequest,
		"Ungültige Fahrzeug-ID":                                http.StatusBadRequest,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
