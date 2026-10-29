package apihttp

import (
	"context"
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

// Fahrtenbuch ueber HTTP (Backlog G.3c.3). Helfer aus
// fleet_integration_test.go (vehiclesPath, decodeVehicle),
// qualification_types_integration_test.go (newCatalogTenant),
// leave_requests_integration_test.go (seedRoleWithPermissions) und
// asset_inspections_integration_test.go (decodeJSON).

const fleetTripsPath = "/api/v1/fleet/trips/"

type fleetTripResponse struct {
	ID                uuid.UUID `json:"id"`
	VehicleID         uuid.UUID `json:"vehicle_id"`
	Kennzeichen       string    `json:"kennzeichen"`
	EmployeeID        uuid.UUID `json:"employee_id"`
	MitarbeiterName   string    `json:"mitarbeiter_name"`
	Datum             string    `json:"datum"`
	KmBeginn          int       `json:"km_beginn"`
	KmEnde            int       `json:"km_ende"`
	Km                int       `json:"km"`
	Fahrtart          string    `json:"fahrtart"`
	Ziel              string    `json:"ziel"`
	Zweck             string    `json:"zweck"`
	Geschaeftspartner string    `json:"geschaeftspartner"`
	Notiz             string    `json:"notiz"`
	Status            string    `json:"status"`
	StornoReason      string    `json:"storno_reason"`
	StorniertByUserID *string   `json:"storniert_by_user_id"`
	CreatedByUserID   *string   `json:"created_by_user_id"`
	CreatedAt         string    `json:"created_at"`
}

type fleetTripChainResponse struct {
	KmStand *int `json:"km_stand"`
	Luecken []struct {
		VonKm int `json:"von_km"`
		BisKm int `json:"bis_km"`
	} `json:"luecken"`
}

// tripBody: Privatfahrt; employee == uuid.Nil laesst employee_id weg.
func tripBody(vehicle, employee uuid.UUID, datum string, beginn, ende int) string {
	emp := ""
	if employee != uuid.Nil {
		emp = `"employee_id":"` + employee.String() + `",`
	}
	return `{"vehicle_id":"` + vehicle.String() + `",` + emp + `"datum":"` + datum + `","km_beginn":` + strconv.Itoa(beginn) + `,"km_ende":` + strconv.Itoa(ende) + `,"fahrtart":"privat"}`
}

func fleetTripRanges(list []fleetTripResponse) string {
	out := make([]string, len(list))
	for i, x := range list {
		out[i] = strconv.Itoa(x.KmBeginn) + "-" + strconv.Itoa(x.KmEnde)
	}
	return strings.Join(out, ",")
}

func logbookVehicle(t *testing.T, handler http.Handler, token, kennzeichen string) uuid.UUID {
	t.Helper()
	return decodeVehicle(t, doAuthedRequest(t, handler, token, http.MethodPost, vehiclesPath, `{"kennzeichen":"`+kennzeichen+`","art":"pkw","fahrtenbuch":true}`), http.StatusCreated).ID
}

func TestFleetTripsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	anna := c.employee("Anna")
	bert := c.employee("Bert")
	v := logbookVehicle(t, handler, token, "M-FB 1")
	chainPath := fleetTripsPath + "chain?vehicle_id=" + v.String()

	if st := decodeJSON[fleetTripChainResponse](t, do(http.MethodGet, chainPath, ""), http.StatusOK); st.KmStand != nil || len(st.Luecken) != 0 {
		t.Fatalf("Stand ohne Fahrt: %+v", st)
	}

	// Dienstfahrt mit allen Angaben.
	a := decodeJSON[fleetTripResponse](t, do(http.MethodPost, fleetTripsPath, `{"vehicle_id":"`+v.String()+`","employee_id":"`+anna.String()+`","datum":"2026-03-02","km_beginn":1000,"km_ende":1050,"fahrtart":" Dienstlich ","ziel":" Augsburg ","zweck":"Aufmaß","geschaeftspartner":"Kunde A","notiz":"Stau"}`), http.StatusCreated)
	if a.VehicleID != v || a.Kennzeichen != "M-FB 1" || a.EmployeeID != anna || a.MitarbeiterName != "Quali Anna" || !strings.HasPrefix(a.Datum, "2026-03-02") ||
		a.KmBeginn != 1000 || a.KmEnde != 1050 || a.Km != 50 || a.Fahrtart != "dienstlich" || a.Ziel != "Augsburg" || a.Zweck != "Aufmaß" ||
		a.Geschaeftspartner != "Kunde A" || a.Notiz != "Stau" || a.Status != "active" || a.CreatedByUserID == nil || a.CreatedAt == "" {
		t.Fatalf("Fahrt: %+v", a)
	}
	if got := decodeJSON[fleetTripResponse](t, do(http.MethodGet, fleetTripsPath+a.ID.String(), ""), http.StatusOK); got.ID != a.ID || got.Km != 50 {
		t.Fatalf("Einzelabruf: %+v", got)
	}
	b := decodeJSON[fleetTripResponse](t, do(http.MethodPost, fleetTripsPath, tripBody(v, bert, "2026-03-03", 1050, 1100)), http.StatusCreated)
	decodeJSON[fleetTripResponse](t, do(http.MethodPost, fleetTripsPath, tripBody(v, bert, "2026-03-04", 1100, 1180)), http.StatusCreated)

	// Kette und Eingaben.
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "2026-03-05", 1190, 1200)), http.StatusBadRequest, "validation_error", "Kilometerstand Beginn muss 1180 km sein (Ende der vorigen Fahrt)")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "2026-03-01", 900, 990)), http.StatusBadRequest, "validation_error", "Kilometerstand Ende muss 1000 km sein (Beginn der folgenden Fahrt)")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "2026-03-05", 1020, 1080)), http.StatusConflict, "conflict", "Kilometerbereich überschneidet sich mit der Fahrt vom 02.03.2026 (1000–1050 km)")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "2026-03-01", 1180, 1200)), http.StatusBadRequest, "validation_error", "Datum darf nicht vor der vorigen Fahrt (04.03.2026) liegen")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "2026-03-05", 1180, 1180)), http.StatusBadRequest, "validation_error", "Kilometerstand Ende muss über dem Beginn liegen")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, `{"vehicle_id":"`+v.String()+`","employee_id":"`+anna.String()+`","datum":"2026-03-05","km_beginn":1180,"km_ende":1200,"fahrtart":"dienstlich","zweck":"x"}`), http.StatusBadRequest, "validation_error", "Reiseziel ist bei Dienstfahrten erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, `{"vehicle_id":"`+v.String()+`","employee_id":"`+anna.String()+`","datum":"2026-03-05","km_beginn":1180,"km_ende":1200,"fahrtart":"urlaub"}`), http.StatusBadRequest, "validation_error", "Ungültige Fahrtart")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "", 1180, 1200)), http.StatusBadRequest, "validation_error", "Datum erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "5.3.2026", 1180, 1200)), http.StatusBadRequest, "validation_error", "Ungültiges Datum in datum (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, time.Now().AddDate(0, 0, 2).Format("2006-01-02"), 1180, 1200)), http.StatusBadRequest, "validation_error", "Datum darf nicht in der Zukunft liegen")

	// Storno der mittleren Fahrt hinterlaesst eine Luecke.
	stornoPath := fleetTripsPath + b.ID.String() + "/storno"
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	st := decodeJSON[fleetTripResponse](t, do(http.MethodPost, stornoPath, `{"reason":"falsches Fahrzeug"}`), http.StatusOK)
	if st.Status != "storniert" || st.StornoReason != "falsches Fahrzeug" || st.StorniertByUserID == nil {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Fahrt ist bereits storniert")
	chain := decodeJSON[fleetTripChainResponse](t, do(http.MethodGet, chainPath, ""), http.StatusOK)
	if chain.KmStand == nil || *chain.KmStand != 1180 || len(chain.Luecken) != 1 || chain.Luecken[0].VonKm != 1050 || chain.Luecken[0].BisKm != 1100 {
		t.Fatalf("Stand nach Storno: %+v", chain)
	}
	decodeJSON[fleetTripResponse](t, do(http.MethodPost, fleetTripsPath, tripBody(v, anna, "2026-03-03", 1050, 1100)), http.StatusCreated)
	if chain := decodeJSON[fleetTripChainResponse](t, do(http.MethodGet, chainPath, ""), http.StatusOK); len(chain.Luecken) != 0 {
		t.Fatalf("Luecke nach Ersatz: %+v", chain)
	}

	// Liste mit Filtern (juengste zuerst).
	w := logbookVehicle(t, handler, token, "M-FB 2")
	decodeJSON[fleetTripResponse](t, do(http.MethodPost, fleetTripsPath, tripBody(w, bert, "2026-03-10", 5, 25)), http.StatusCreated)
	list := func(query string) string {
		return fleetTripRanges(decodeJSON[[]fleetTripResponse](t, do(http.MethodGet, fleetTripsPath+query, ""), http.StatusOK))
	}
	for query, want := range map[string]string{
		"":                               "5-25,1100-1180,1050-1100,1000-1050",
		"?include_storniert=true":        "5-25,1100-1180,1050-1100,1050-1100,1000-1050",
		"?vehicle_id=" + v.String():      "1100-1180,1050-1100,1000-1050",
		"?employee_id=" + bert.String():  "5-25,1100-1180",
		"?von=2026-03-03&bis=2026-03-04": "1100-1180,1050-1100",
		"?von=2026-03-04":                "5-25,1100-1180",
		"?bis=2026-03-02":                "1000-1050",
		"?vehicle_id=" + w.String() + "&bis=2026-03-09": "",
	} {
		if got := list(query); got != want {
			t.Fatalf("Liste %q: %s", query, got)
		}
	}

	// Fehlerfaelle ohne Wirkung.
	ohne := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-FB 3","art":"pkw"}`), http.StatusCreated).ID
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(ohne, anna, "2026-03-01", 0, 10)), http.StatusConflict, "conflict", "Für dieses Fahrzeug wird kein Fahrtenbuch geführt")
	weg := c.employee("Weg")
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_employees SET active=false WHERE id=$1`, weg); err != nil {
		t.Fatal(err)
	}
	frei := logbookVehicle(t, handler, token, "M-FB 4")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(frei, weg, "2026-03-01", 0, 10)), http.StatusConflict, "conflict", "Mitarbeiter ist nicht aktiv")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(frei, uuid.New(), "2026-03-01", 0, 10)), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(uuid.New(), anna, "2026-03-01", 0, 10)), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, `{"employee_id":"`+anna.String()+`","datum":"2026-03-01","km_ende":10,"fahrtart":"privat"}`), http.StatusBadRequest, "validation_error", "vehicle_id erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, tripBody(frei, uuid.Nil, "2026-03-01", 0, 10)), http.StatusBadRequest, "validation_error", "employee_id erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Fahrt nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath+uuid.NewString()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Fahrt nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath+a.ID.String()+"/storno", `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Fahrt-ID")
	wantAPIError(t, do(http.MethodPost, fleetTripsPath+"kein-uuid/storno", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Fahrt-ID")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"chain", ""), http.StatusBadRequest, "validation_error", "vehicle_id erforderlich")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"chain?vehicle_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige vehicle_id")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"chain?vehicle_id="+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"?vehicle_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige vehicle_id")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"?employee_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige employee_id")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"?von=1.3.2026", ""), http.StatusBadRequest, "validation_error", "Ungültiges Datum in von (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodGet, fleetTripsPath+"?bis=2026-13-01", ""), http.StatusBadRequest, "validation_error", "Ungültiges Datum in bis (erwartet JJJJ-MM-TT)")
	if n := countFleetTrips(t, env, ohne, frei); n != 0 {
		t.Fatalf("nach Fehlern %d Fahrten", n)
	}
	if a2 := decodeJSON[fleetTripResponse](t, do(http.MethodGet, fleetTripsPath+a.ID.String(), ""), http.StatusOK); a2.Status != "active" {
		t.Fatalf("fehlgeschlagenes Storno hat gewirkt: %+v", a2)
	}
}

func countFleetTrips(t *testing.T, env *testutil.IntegrationEnv, vehicleIDs ...uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM fleet_trips WHERE vehicle_id = ANY($1)`, vehicleIDs).Scan(&n); err != nil {
		t.Fatalf("zaehlen: %v", err)
	}
	return n
}

// Erfassen: mit fleet.self nur fuer sich, mit fleet.write fuer jeden.
// Lesen: mit fleet.read alles, mit fleet.self nur die eigenen. Storno nur
// mit fleet.write.
func TestFleetTripsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-ft-readonly", "fleet.read")
	seedRoleWithPermissions(t, env, "itest-ft-writeonly", "fleet.write")
	seedRoleWithPermissions(t, env, "itest-ft-none")
	c := newCatalogTenant(t, env, handler)
	fleetToken := c.user("fleet")
	v := logbookVehicle(t, handler, fleetToken, "M-FP 1")
	monteur := c.linkedUser("production", "Monteur")
	fremd := c.employee("Fremd")
	km := 0
	next := func() (int, int) { km += 10; return km - 10, km }

	// Fahrer mit fleet.self: ohne employee_id fuer sich selbst.
	b0, e0 := next()
	own := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodPost, fleetTripsPath, tripBody(v, uuid.Nil, "2026-03-01", b0, e0)), http.StatusCreated)
	if own.EmployeeID != monteur.EmployeeID {
		t.Fatalf("eigene Fahrt: %+v", own)
	}
	b1, e1 := next()
	decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodPost, fleetTripsPath, tripBody(v, monteur.EmployeeID, "2026-03-01", b1, e1)), http.StatusCreated)
	wantAPIError(t, doAuthedRequest(t, handler, monteur.Token, http.MethodPost, fleetTripsPath, tripBody(v, fremd, "2026-03-01", e1, e1+10)), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	b2, e2 := next()
	foreign := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetTripsPath, tripBody(v, fremd, "2026-03-01", b2, e2)), http.StatusCreated)

	if list := decodeJSON[[]fleetTripResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetTripsPath+"?vehicle_id="+v.String(), ""), http.StatusOK); len(list) != 2 || list[1].ID != own.ID {
		t.Fatalf("eigene Liste: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetTripsPath+"?employee_id="+fremd.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetTripsPath+own.ID.String(), ""), http.StatusOK)
	wantAPIError(t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetTripsPath+foreign.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if st := decodeJSON[fleetTripChainResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetTripsPath+"chain?vehicle_id="+v.String(), ""), http.StatusOK); st.KmStand == nil || *st.KmStand != e2 {
		t.Fatalf("Stand fuer den Fahrer: %+v", st)
	}
	wantAPIError(t, doAuthedRequest(t, handler, monteur.Token, http.MethodPost, fleetTripsPath+own.ID.String()+"/storno", `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	unlinked := c.user("production")
	wantAPIError(t, doAuthedRequest(t, handler, unlinked, http.MethodPost, fleetTripsPath, tripBody(v, uuid.Nil, "2026-03-01", e2, e2+10)), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")
	wantAPIError(t, doAuthedRequest(t, handler, unlinked, http.MethodGet, fleetTripsPath, ""), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")

	// Nur fleet.read: alles lesen, nichts schreiben.
	readOnly := c.user("itest-ft-readonly")
	if list := decodeJSON[[]fleetTripResponse](t, doAuthedRequest(t, handler, readOnly, http.MethodGet, fleetTripsPath, ""), http.StatusOK); len(list) != 3 {
		t.Fatalf("read: alle: %+v", list)
	}
	decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, readOnly, http.MethodGet, fleetTripsPath+foreign.ID.String(), ""), http.StatusOK)
	decodeJSON[fleetTripChainResponse](t, doAuthedRequest(t, handler, readOnly, http.MethodGet, fleetTripsPath+"chain?vehicle_id="+v.String(), ""), http.StatusOK)

	// nur_eigene=true: auch mit fleet.read nur die eigenen (G.3c.4b).
	leser := c.linkedUser("itest-ft-readonly", "Leser")
	bl, el := next()
	mine := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetTripsPath, tripBody(v, leser.EmployeeID, "2026-03-01", bl, el)), http.StatusCreated)
	if list := decodeJSON[[]fleetTripResponse](t, doAuthedRequest(t, handler, leser.Token, http.MethodGet, fleetTripsPath+"?nur_eigene=true", ""), http.StatusOK); len(list) != 1 || list[0].ID != mine.ID {
		t.Fatalf("nur_eigene mit read: %+v", list)
	}
	if list := decodeJSON[[]fleetTripResponse](t, doAuthedRequest(t, handler, leser.Token, http.MethodGet, fleetTripsPath, ""), http.StatusOK); len(list) != 4 {
		t.Fatalf("ohne nur_eigene alle: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, leser.Token, http.MethodGet, fleetTripsPath+"?nur_eigene=true&employee_id="+fremd.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, readOnly, http.MethodGet, fleetTripsPath+"?nur_eigene=true", ""), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")
	if list := decodeJSON[[]fleetTripResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetTripsPath+"?nur_eigene=true", ""), http.StatusOK); len(list) != 2 {
		t.Fatalf("nur_eigene mit self: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, readOnly, http.MethodPost, fleetTripsPath, tripBody(v, fremd, "2026-03-01", e2, e2+10)), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, readOnly, http.MethodPost, fleetTripsPath+foreign.ID.String()+"/storno", `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")

	// Nur fleet.write: fuer andere erfassen und stornieren, nicht lesen.
	writeOnly := c.user("itest-ft-writeonly")
	b3, e3 := next()
	x := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, writeOnly, http.MethodPost, fleetTripsPath, tripBody(v, fremd, "2026-03-01", b3, e3)), http.StatusCreated)
	wantAPIError(t, doAuthedRequest(t, handler, writeOnly, http.MethodPost, fleetTripsPath, tripBody(v, uuid.Nil, "2026-03-01", e3, e3+10)), http.StatusBadRequest, "validation_error", "employee_id erforderlich")
	decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, writeOnly, http.MethodPost, fleetTripsPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusOK)

	none := c.user("itest-ft-none")
	for _, tok := range []string{writeOnly, none} {
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, fleetTripsPath, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, fleetTripsPath+"?nur_eigene=true", ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, fleetTripsPath+own.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, fleetTripsPath+"chain?vehicle_id="+v.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}
	wantAPIError(t, doAuthedRequest(t, handler, none, http.MethodPost, fleetTripsPath, `{`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, none, http.MethodPost, fleetTripsPath+own.ID.String()+"/storno", `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if got := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodGet, fleetTripsPath+own.ID.String(), ""), http.StatusOK); got.Status != "active" {
		t.Fatalf("verbotenes Storno hat gewirkt: %+v", got)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fleetTripsPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestFleetTripsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("fleet"), two.user("admin")
	empOne, empTwo := one.employee("Eins"), two.employee("Zwei")
	vOne := logbookVehicle(t, handler, tokenOne, "M-FI 1")
	vTwo := logbookVehicle(t, handler, tokenTwo, "M-FI 1")
	x := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, fleetTripsPath, tripBody(vOne, empOne, "2026-03-01", 0, 10)), http.StatusCreated)

	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetTripsPath+x.ID.String(), ""), http.StatusNotFound, "not_found", "Fahrt nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetTripsPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Fahrt nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetTripsPath+"chain?vehicle_id="+vOne.String(), ""), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	if list := decodeJSON[[]fleetTripResponse](t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetTripsPath+"?include_storniert=true", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("Liste des anderen Mandanten: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetTripsPath, tripBody(vOne, empTwo, "2026-03-01", 10, 20)), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetTripsPath, tripBody(vTwo, empOne, "2026-03-01", 0, 10)), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	if still := decodeJSON[fleetTripResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, fleetTripsPath+x.ID.String(), ""), http.StatusOK); still.Status != "active" {
		t.Fatalf("anderer Mandant hat die Fahrt veraendert: %+v", still)
	}
	if n := countFleetTrips(t, env, vOne, vTwo); n != 1 {
		t.Fatalf("Fahrten nach fremden Versuchen: %d", n)
	}
}

// Jeder Fehlertext des Fahrtenbuchs muss als 4xx klassifiziert werden.
// DB-los.
func TestClassifyDomainErrorCoversFleetTripMessages(t *testing.T) {
	cases := map[string]int{
		"Datum erforderlich":                                                              http.StatusBadRequest,
		"Datum darf nicht in der Zukunft liegen":                                          http.StatusBadRequest,
		"Datum darf nicht vor der Erstzulassung liegen":                                   http.StatusBadRequest,
		"Datum darf nicht nach der Abmeldung liegen":                                      http.StatusBadRequest,
		"Kilometerstand Beginn darf nicht negativ sein":                                   http.StatusBadRequest,
		"Kilometerstand Ende muss über dem Beginn liegen":                                 http.StatusBadRequest,
		"Ungültige Fahrtart":                                                              http.StatusBadRequest,
		"Reiseziel ist bei Dienstfahrten erforderlich":                                    http.StatusBadRequest,
		"Reisezweck ist bei Dienstfahrten erforderlich":                                   http.StatusBadRequest,
		"Kilometerstand Beginn muss 1180 km sein (Ende der vorigen Fahrt)":                http.StatusBadRequest,
		"Kilometerstand Ende muss 1000 km sein (Beginn der folgenden Fahrt)":              http.StatusBadRequest,
		"Datum darf nicht vor der vorigen Fahrt (04.03.2026) liegen":                      http.StatusBadRequest,
		"Datum darf nicht nach der folgenden Fahrt (04.03.2026) liegen":                   http.StatusBadRequest,
		"vehicle_id erforderlich":                                                         http.StatusBadRequest,
		"employee_id erforderlich":                                                        http.StatusBadRequest,
		"Fahrt-ID erforderlich":                                                           http.StatusBadRequest,
		"Storno-Begründung erforderlich":                                                  http.StatusBadRequest,
		"Fahrzeug nicht gefunden":                                                         http.StatusNotFound,
		"Mitarbeiter nicht gefunden":                                                      http.StatusNotFound,
		"Fahrt nicht gefunden":                                                            http.StatusNotFound,
		"Für dieses Fahrzeug wird kein Fahrtenbuch geführt":                               http.StatusConflict,
		"Mitarbeiter ist nicht aktiv":                                                     http.StatusConflict,
		"Kilometerbereich überschneidet sich mit einer anderen Fahrt":                     http.StatusConflict,
		"Kilometerbereich überschneidet sich mit der Fahrt vom 02.03.2026 (1000–1050 km)": http.StatusConflict,
		"Fahrt ist bereits storniert":                                                     http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
