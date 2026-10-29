package apihttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Fahrzeugkosten ueber HTTP (Backlog G.3d.2). Helfer aus
// fleet_integration_test.go (vehiclesPath, decodeVehicle),
// fleet_trips_integration_test.go (logbookVehicle),
// qualification_types_integration_test.go (newCatalogTenant),
// leave_requests_integration_test.go (seedRoleWithPermissions),
// assets_integration_test.go (Kostenstelle) und
// asset_inspections_integration_test.go (decodeJSON).

const fleetCostsPath = "/api/v1/fleet/costs/"

type fleetCostResponse struct {
	ID                uuid.UUID `json:"id"`
	VehicleID         uuid.UUID `json:"vehicle_id"`
	Kennzeichen       string    `json:"kennzeichen"`
	Datum             string    `json:"datum"`
	Kostenart         string    `json:"kostenart"`
	Bezeichnung       string    `json:"bezeichnung"`
	BetragNetto       float64   `json:"betrag_netto"`
	Lieferant         string    `json:"lieferant"`
	Belegnummer       string    `json:"belegnummer"`
	KmStand           *int      `json:"km_stand"`
	Liter             *float64  `json:"liter"`
	KostenstelleID    *string   `json:"kostenstelle_id"`
	Notiz             string    `json:"notiz"`
	Status            string    `json:"status"`
	StornoReason      string    `json:"storno_reason"`
	StorniertByUserID *string   `json:"storniert_by_user_id"`
	CreatedByUserID   *string   `json:"created_by_user_id"`
}

type fleetCostSummaryResponse struct {
	Summen []struct {
		Kostenart   string  `json:"kostenart"`
		Anzahl      int     `json:"anzahl"`
		BetragNetto float64 `json:"betrag_netto"`
	} `json:"summen"`
	Gesamt float64 `json:"gesamt"`
}

func costBody(vehicle uuid.UUID, datum, art string, betrag float64) string {
	return `{"vehicle_id":"` + vehicle.String() + `","datum":"` + datum + `","kostenart":"` + art + `","betrag_netto":` + strconv.FormatFloat(betrag, 'f', -1, 64) + `}`
}

func fleetCostList(list []fleetCostResponse) string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.Kostenart + ":" + strconv.FormatFloat(c.BetragNetto, 'f', -1, 64)
	}
	return strings.Join(out, ",")
}

func TestFleetCostsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	v := logbookVehicle(t, handler, token, "M-KO 1")
	w := logbookVehicle(t, handler, token, "M-KO 2")

	a := decodeJSON[fleetCostResponse](t, do(http.MethodPost, fleetCostsPath, `{"vehicle_id":"`+v.String()+`","datum":"2026-03-02","kostenart":" Kraftstoff ","betrag_netto":80.12,
		"lieferant":" Tanke ","belegnummer":"B-1","km_stand":12000,"liter":45.5,"notiz":"voll"}`), http.StatusCreated)
	if a.VehicleID != v || a.Kennzeichen != "M-KO 1" || !strings.HasPrefix(a.Datum, "2026-03-02") || a.Kostenart != "kraftstoff" || a.BetragNetto != 80.12 ||
		a.Lieferant != "Tanke" || a.Belegnummer != "B-1" || a.KmStand == nil || *a.KmStand != 12000 || a.Liter == nil || *a.Liter != 45.5 ||
		a.KostenstelleID != nil || a.Notiz != "voll" || a.Status != "active" || a.CreatedByUserID == nil {
		t.Fatalf("Beleg: %+v", a)
	}
	if got := decodeJSON[fleetCostResponse](t, do(http.MethodGet, fleetCostsPath+a.ID.String(), ""), http.StatusOK); got.ID != a.ID || got.BetragNetto != 80.12 {
		t.Fatalf("Einzelabruf: %+v", got)
	}
	b := decodeJSON[fleetCostResponse](t, do(http.MethodPost, fleetCostsPath, costBody(v, "2026-03-05", "reparatur", 300)), http.StatusCreated)
	decodeJSON[fleetCostResponse](t, do(http.MethodPost, fleetCostsPath, `{"vehicle_id":"`+w.String()+`","datum":"2026-03-04","kostenart":"sonstiges","bezeichnung":"Vignette","betrag_netto":0.3}`), http.StatusCreated)

	// Eingaben.
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, "", "wartung", 1)), http.StatusBadRequest, "validation_error", "Datum erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, "2.3.2026", "wartung", 1)), http.StatusBadRequest, "validation_error", "Ungültiges Datum in datum (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, time.Now().AddDate(0, 0, 2).Format("2006-01-02"), "wartung", 1)), http.StatusBadRequest, "validation_error", "Datum darf nicht in der Zukunft liegen")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, "2026-03-02", "bussgeld", 1)), http.StatusBadRequest, "validation_error", "Ungültige Kostenart")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, "2026-03-02", "sonstiges", 1)), http.StatusBadRequest, "validation_error", "Bezeichnung ist bei sonstigen Kosten erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, "2026-03-02", "wartung", 0)), http.StatusBadRequest, "validation_error", "Betrag muss größer als 0 sein")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(v, "2026-03-02", "wartung", 1.005)), http.StatusBadRequest, "validation_error", "Betrag darf höchstens zwei Nachkommastellen haben")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, `{"vehicle_id":"`+v.String()+`","datum":"2026-03-02","kostenart":"wartung","betrag_netto":1,"liter":5}`), http.StatusBadRequest, "validation_error", "Liter nur bei Kraftstoff")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, `{"vehicle_id":"`+v.String()+`","datum":"2026-03-02","kostenart":"wartung","betrag_netto":1,"km_stand":-1}`), http.StatusBadRequest, "validation_error", "Kilometerstand darf nicht negativ sein")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, `{"vehicle_id":"`+v.String()+`","datum":"2026-03-02","kostenart":"wartung","betrag_netto":1,"kostenstelle_id":"gibt-es-nicht"}`), http.StatusNotFound, "not_found", "Kostenstelle nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, costBody(uuid.New(), "2026-03-02", "wartung", 1)), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, `{"datum":"2026-03-02","kostenart":"wartung","betrag_netto":1}`), http.StatusBadRequest, "validation_error", "vehicle_id erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")

	// Liste und Auswertung mit Filtern.
	list := func(query string) string {
		return fleetCostList(decodeJSON[[]fleetCostResponse](t, do(http.MethodGet, fleetCostsPath+query, ""), http.StatusOK))
	}
	for query, want := range map[string]string{
		"":                               "reparatur:300,sonstiges:0.3,kraftstoff:80.12",
		"?vehicle_id=" + v.String():      "reparatur:300,kraftstoff:80.12",
		"?kostenart=Reparatur":           "reparatur:300",
		"?von=2026-03-03&bis=2026-03-04": "sonstiges:0.3",
	} {
		if got := list(query); got != want {
			t.Fatalf("Liste %q: %s", query, got)
		}
	}
	sum := decodeJSON[fleetCostSummaryResponse](t, do(http.MethodGet, fleetCostsPath+"summary", ""), http.StatusOK)
	if len(sum.Summen) != 3 || sum.Summen[0].Kostenart != "kraftstoff" || sum.Summen[1].Kostenart != "reparatur" || sum.Summen[2].Kostenart != "sonstiges" || sum.Gesamt != 380.42 {
		t.Fatalf("Auswertung: %+v", sum)
	}
	if s := decodeJSON[fleetCostSummaryResponse](t, do(http.MethodGet, fleetCostsPath+"summary?vehicle_id="+v.String()+"&bis=2026-03-02", ""), http.StatusOK); s.Gesamt != 80.12 || len(s.Summen) != 1 || s.Summen[0].Anzahl != 1 {
		t.Fatalf("Auswertung gefiltert: %+v", s)
	}
	for _, p := range []string{"", "summary"} {
		wantAPIError(t, do(http.MethodGet, fleetCostsPath+p+"?vehicle_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige vehicle_id")
		wantAPIError(t, do(http.MethodGet, fleetCostsPath+p+"?von=1.3.2026", ""), http.StatusBadRequest, "validation_error", "Ungültiges Datum in von (erwartet JJJJ-MM-TT)")
		wantAPIError(t, do(http.MethodGet, fleetCostsPath+p+"?bis=2026-13-01", ""), http.StatusBadRequest, "validation_error", "Ungültiges Datum in bis (erwartet JJJJ-MM-TT)")
		wantAPIError(t, do(http.MethodGet, fleetCostsPath+p+"?kostenart=bussgeld", ""), http.StatusBadRequest, "validation_error", "Ungültige Kostenart")
	}

	// Storno.
	stornoPath := fleetCostsPath + b.ID.String() + "/storno"
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	wantAPIError(t, do(http.MethodPost, stornoPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	st := decodeJSON[fleetCostResponse](t, do(http.MethodPost, stornoPath, `{"reason":"doppelt"}`), http.StatusOK)
	if st.Status != "storniert" || st.StornoReason != "doppelt" || st.StorniertByUserID == nil {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Kosten sind bereits storniert")
	if got := list("?include_storniert=true&vehicle_id=" + v.String()); got != "reparatur:300,kraftstoff:80.12" {
		t.Fatalf("mit stornierten: %s", got)
	}
	if got := list("?vehicle_id=" + v.String()); got != "kraftstoff:80.12" {
		t.Fatalf("nach Storno: %s", got)
	}
	if s := decodeJSON[fleetCostSummaryResponse](t, do(http.MethodGet, fleetCostsPath+"summary?include_storniert=true", ""), http.StatusOK); s.Gesamt != 80.42 {
		t.Fatalf("Auswertung nach Storno: %+v", s)
	}

	wantAPIError(t, do(http.MethodGet, fleetCostsPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Kosten nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath+uuid.NewString()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Kosten nicht gefunden")
	wantAPIError(t, do(http.MethodGet, fleetCostsPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Kosten-ID")
	wantAPIError(t, do(http.MethodPost, fleetCostsPath+"kein-uuid/storno", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Kosten-ID")
}

// Lesen mit fleet.read, schreiben mit fleet.write; fleet.self allein
// nichts.
func TestFleetCostsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-fc-readonly", "fleet.read")
	seedRoleWithPermissions(t, env, "itest-fc-writeonly", "fleet.write")
	seedRoleWithPermissions(t, env, "itest-fc-self", "fleet.self")
	c := newCatalogTenant(t, env, handler)
	fleetToken := c.user("fleet")
	v := logbookVehicle(t, handler, fleetToken, "M-KP 1")
	x := decodeJSON[fleetCostResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetCostsPath, costBody(v, "2026-03-02", "wartung", 10)), http.StatusCreated)

	readOnly := c.user("itest-fc-readonly")
	writeOnly := c.user("itest-fc-writeonly")
	self := c.linkedUser("itest-fc-self", "Fahrer").Token
	for name, tc := range map[string]struct {
		token             string
		canRead, canWrite bool
	}{
		"nur read":  {readOnly, true, false},
		"nur write": {writeOnly, false, true},
		"nur self":  {self, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			checks := []struct {
				method, path, body string
				allowed            bool
				ok                 int
			}{
				{http.MethodGet, fleetCostsPath, "", tc.canRead, http.StatusOK},
				{http.MethodGet, fleetCostsPath + "summary", "", tc.canRead, http.StatusOK},
				{http.MethodGet, fleetCostsPath + x.ID.String(), "", tc.canRead, http.StatusOK},
				{http.MethodPost, fleetCostsPath, costBody(v, "2026-03-03", "wartung", 1), tc.canWrite, http.StatusCreated},
			}
			for _, ch := range checks {
				rec := doAuthedRequest(t, handler, tc.token, ch.method, ch.path, ch.body)
				if !ch.allowed {
					wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				} else if rec.Code != ch.ok {
					t.Fatalf("%s %s: %d %s", ch.method, ch.path, rec.Code, rec.Body.String())
				}
			}
			y := decodeJSON[fleetCostResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetCostsPath, costBody(v, "2026-03-04", "wartung", 2)), http.StatusCreated)
			rec := doAuthedRequest(t, handler, tc.token, http.MethodPost, fleetCostsPath+y.ID.String()+"/storno", `{"reason":"x"}`)
			if !tc.canWrite {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				if got := decodeJSON[fleetCostResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodGet, fleetCostsPath+y.ID.String(), ""), http.StatusOK); got.Status != "active" {
					t.Fatalf("verbotenes Storno hat gewirkt: %+v", got)
				}
			} else if rec.Code != http.StatusOK {
				t.Fatalf("Storno: %d %s", rec.Code, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fleetCostsPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestFleetCostsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("fleet"), two.user("admin")
	vOne := logbookVehicle(t, handler, tokenOne, "M-KI 1")
	x := decodeJSON[fleetCostResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, fleetCostsPath, costBody(vOne, "2026-03-02", "wartung", 10)), http.StatusCreated)

	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetCostsPath+x.ID.String(), ""), http.StatusNotFound, "not_found", "Kosten nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetCostsPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Kosten nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetCostsPath, costBody(vOne, "2026-03-02", "wartung", 1)), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	if list := decodeJSON[[]fleetCostResponse](t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetCostsPath+"?include_storniert=true", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("Liste des anderen Mandanten: %+v", list)
	}
	if s := decodeJSON[fleetCostSummaryResponse](t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetCostsPath+"summary", ""), http.StatusOK); s.Gesamt != 0 || len(s.Summen) != 0 {
		t.Fatalf("Auswertung des anderen Mandanten: %+v", s)
	}
	if still := decodeJSON[fleetCostResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, fleetCostsPath+x.ID.String(), ""), http.StatusOK); still.Status != "active" {
		t.Fatalf("anderer Mandant hat storniert: %+v", still)
	}
}

// Jeder Fehlertext der Kosten muss als 4xx klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversFleetCostMessages(t *testing.T) {
	cases := map[string]int{
		"Datum erforderlich":                                http.StatusBadRequest,
		"Datum darf nicht in der Zukunft liegen":            http.StatusBadRequest,
		"Ungültige Kostenart":                               http.StatusBadRequest,
		"Bezeichnung ist bei sonstigen Kosten erforderlich": http.StatusBadRequest,
		"Betrag muss größer als 0 sein":                     http.StatusBadRequest,
		"Betrag darf höchstens zwei Nachkommastellen haben": http.StatusBadRequest,
		"Kilometerstand darf nicht negativ sein":            http.StatusBadRequest,
		"Liter nur bei Kraftstoff":                          http.StatusBadRequest,
		"Liter muss größer als 0 sein":                      http.StatusBadRequest,
		"Liter darf höchstens zwei Nachkommastellen haben":  http.StatusBadRequest,
		"vehicle_id erforderlich":                           http.StatusBadRequest,
		"Kosten-ID erforderlich":                            http.StatusBadRequest,
		"Storno-Begründung erforderlich":                    http.StatusBadRequest,
		"Fahrzeug nicht gefunden":                           http.StatusNotFound,
		"Kostenstelle nicht gefunden":                       http.StatusNotFound,
		"Kosten nicht gefunden":                             http.StatusNotFound,
		"Kosten sind bereits storniert":                     http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
