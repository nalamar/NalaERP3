package apihttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Fahrerzuordnung ueber HTTP (Backlog G.3a.2). Helfer aus
// fleet_integration_test.go (vehiclesPath, decodeVehicle),
// fleet_schedules_integration_test.go (schedulesPath, completionsPath),
// qualification_types_integration_test.go (newCatalogTenant),
// leave_requests_integration_test.go (seedRoleWithPermissions) und
// asset_inspections_integration_test.go (decodeJSON).

const fleetAssignmentsPath = "/api/v1/fleet/assignments/"

type fleetAssignmentResponse struct {
	ID                 uuid.UUID `json:"id"`
	VehicleID          uuid.UUID `json:"vehicle_id"`
	Kennzeichen        string    `json:"kennzeichen"`
	EmployeeID         uuid.UUID `json:"employee_id"`
	MitarbeiterName    string    `json:"mitarbeiter_name"`
	Beginn             string    `json:"beginn"`
	Ende               *string   `json:"ende"`
	Notiz              string    `json:"notiz"`
	Status             string    `json:"status"`
	StornoReason       string    `json:"storno_reason"`
	StorniertByUserID  *string   `json:"storniert_by_user_id"`
	ZugeordnetByUserID *string   `json:"zugeordnet_by_user_id"`
	BeendetByUserID    *string   `json:"beendet_by_user_id"`
	Hinweise           []string  `json:"hinweise"`
}

func assignBody(vehicle, employee uuid.UUID, beginn string) string {
	return `{"vehicle_id":"` + vehicle.String() + `","employee_id":"` + employee.String() + `","beginn":"` + beginn + `"}`
}

func fleetAssignmentPlates(list []fleetAssignmentResponse) string {
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = a.Kennzeichen
	}
	return strings.Join(out, ",")
}

func TestFleetAssignmentsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("fleet")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	anna := c.employee("Anna")
	bert := c.employee("Bert")
	v := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-ZH 1","art":"pkw"}`), http.StatusCreated)
	decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+v.ID.String()+`","art":"uvv","starttermin":"2026-02-28"}`), http.StatusCreated)

	// Zuordnen mit Hinweisen: UVV ueberfaellig, keine Kontrolle.
	a := decodeJSON[fleetAssignmentResponse](t, do(http.MethodPost, fleetAssignmentsPath, `{"vehicle_id":"`+v.ID.String()+`","employee_id":"`+anna.String()+`","beginn":"2026-03-01","notiz":" Dienstwagen "}`), http.StatusCreated)
	if a.VehicleID != v.ID || a.Kennzeichen != "M-ZH 1" || a.EmployeeID != anna || a.MitarbeiterName != "Quali Anna" || !strings.HasPrefix(a.Beginn, "2026-03-01") ||
		a.Ende != nil || a.Notiz != "Dienstwagen" || a.Status != "active" || a.ZugeordnetByUserID == nil {
		t.Fatalf("Zuordnung: %+v", a)
	}
	if got := strings.Join(a.Hinweise, "|"); got != "UVV-Prüfung überfällig seit 28.02.2026|Keine gültige Führerscheinkontrolle erfasst" {
		t.Fatalf("Hinweise: %q", got)
	}
	if got := decodeJSON[fleetAssignmentResponse](t, do(http.MethodGet, fleetAssignmentsPath+a.ID.String(), ""), http.StatusOK); got.ID != a.ID || len(got.Hinweise) != 0 {
		t.Fatalf("Einzelabruf: %+v", got)
	}
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(v.ID, bert, "2026-03-02")), http.StatusConflict, "conflict", "Fahrzeug ist bereits zugeordnet")
	wantAPIError(t, do(http.MethodPost, vehiclesPath+v.ID.String()+"/deregister", `{"datum":"2026-03-05","grund":"verkauft"}`), http.StatusConflict, "conflict", "Fahrzeug ist noch zugeordnet - erst die Zuordnung beenden")

	// Beenden.
	endPath := fleetAssignmentsPath + a.ID.String() + "/end"
	wantAPIError(t, do(http.MethodPost, endPath, `{}`), http.StatusBadRequest, "validation_error", "Ende erforderlich")
	wantAPIError(t, do(http.MethodPost, endPath, `{"ende":"10.03.2026"}`), http.StatusBadRequest, "validation_error", "Ungültiges Datum in ende (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, endPath, `{"ende":"2026-02-28"}`), http.StatusBadRequest, "validation_error", "Ende darf nicht vor dem Beginn liegen")
	wantAPIError(t, do(http.MethodPost, endPath, `{"ende":"`+time.Now().AddDate(0, 0, 2).Format("2006-01-02")+`"}`), http.StatusBadRequest, "validation_error", "Ende darf nicht in der Zukunft liegen")
	ended := decodeJSON[fleetAssignmentResponse](t, do(http.MethodPost, endPath, `{"ende":"2026-03-10"}`), http.StatusOK)
	if ended.Ende == nil || !strings.HasPrefix(*ended.Ende, "2026-03-10") || ended.BeendetByUserID == nil {
		t.Fatalf("Ende: %+v", ended)
	}
	wantAPIError(t, do(http.MethodPost, endPath, `{"ende":"2026-03-11"}`), http.StatusConflict, "conflict", "Zuordnung ist bereits beendet")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(v.ID, bert, "2026-03-09")), http.StatusConflict, "conflict", "Zuordnung überschneidet sich mit einer früheren Zuordnung dieses Fahrzeugs")
	wantAPIError(t, do(http.MethodPost, vehiclesPath+v.ID.String()+"/deregister", `{"datum":"2026-03-09","grund":"verkauft"}`), http.StatusBadRequest, "validation_error", "Abmeldedatum darf nicht vor dem Ende einer Zuordnung liegen")

	// Storno.
	b := decodeJSON[fleetAssignmentResponse](t, do(http.MethodPost, fleetAssignmentsPath, assignBody(v.ID, bert, "2026-03-10")), http.StatusCreated)
	stornoPath := fleetAssignmentsPath + b.ID.String() + "/storno"
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	st := decodeJSON[fleetAssignmentResponse](t, do(http.MethodPost, stornoPath, `{"reason":"falscher Fahrer"}`), http.StatusOK)
	if st.Status != "storniert" || st.StornoReason != "falscher Fahrer" || st.StorniertByUserID == nil {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Zuordnung ist bereits storniert")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath+b.ID.String()+"/end", `{"ende":"2026-03-12"}`), http.StatusConflict, "conflict", "Zuordnung ist storniert")

	// Gesperrt: HU mit Maengeln.
	w := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-ZH 2","art":"transporter"}`), http.StatusCreated)
	hu := decodeJSON[scheduleResponse](t, do(http.MethodPost, schedulesPath, `{"vehicle_id":"`+w.ID.String()+`","art":"hu"}`), http.StatusCreated)
	decodeJSON[completionResponse](t, do(http.MethodPost, completionsPath, `{"schedule_id":"`+hu.ID.String()+`","erledigt_am":"2026-02-10","ergebnis":"maengel","pruefer":"TÜV"}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(w.ID, anna, "2026-03-01")), http.StatusConflict, "conflict", "Fahrzeug ist gesperrt: Hauptuntersuchung mit Mängeln")

	// Liste mit Filtern (jüngste zuerst).
	x := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-ZH 3","art":"pkw"}`), http.StatusCreated)
	decodeJSON[fleetAssignmentResponse](t, do(http.MethodPost, fleetAssignmentsPath, assignBody(x.ID, bert, "2026-04-01")), http.StatusCreated)
	list := func(query string) string {
		return fleetAssignmentPlates(decodeJSON[[]fleetAssignmentResponse](t, do(http.MethodGet, fleetAssignmentsPath+query, ""), http.StatusOK))
	}
	for query, want := range map[string]string{
		"":                              "M-ZH 3,M-ZH 1",
		"?include_storniert=true":       "M-ZH 3,M-ZH 1,M-ZH 1",
		"?nur_offen=true":               "M-ZH 3",
		"?vehicle_id=" + v.ID.String():  "M-ZH 1",
		"?employee_id=" + bert.String(): "M-ZH 3",
		"?employee_id=" + anna.String(): "M-ZH 1",
		"?vehicle_id=" + w.ID.String():  "",
	} {
		if got := list(query); got != want {
			t.Fatalf("Liste %q: %s", query, got)
		}
	}

	// Fehlerfaelle.
	weg := c.employee("Weg")
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_employees SET active=false WHERE id=$1`, weg); err != nil {
		t.Fatal(err)
	}
	frei := decodeVehicle(t, do(http.MethodPost, vehiclesPath, `{"kennzeichen":"M-ZH 4","art":"pkw"}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(frei.ID, weg, "2026-03-01")), http.StatusConflict, "conflict", "Mitarbeiter ist nicht aktiv")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(frei.ID, uuid.New(), "2026-03-01")), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(uuid.New(), anna, "2026-03-01")), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, `{"vehicle_id":"`+frei.ID.String()+`","employee_id":"`+anna.String()+`"}`), http.StatusBadRequest, "validation_error", "Beginn erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(frei.ID, anna, "1.3.2026")), http.StatusBadRequest, "validation_error", "Ungültiges Datum in beginn (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, assignBody(frei.ID, anna, time.Now().AddDate(0, 0, 2).Format("2006-01-02"))), http.StatusBadRequest, "validation_error", "Beginn darf nicht in der Zukunft liegen")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, `{"employee_id":"`+anna.String()+`","beginn":"2026-03-01"}`), http.StatusBadRequest, "validation_error", "vehicle_id erforderlich")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodGet, fleetAssignmentsPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Zuordnung nicht gefunden")
	wantAPIError(t, do(http.MethodPost, fleetAssignmentsPath+uuid.NewString()+"/end", `{"ende":"2026-03-01"}`), http.StatusNotFound, "not_found", "Zuordnung nicht gefunden")
	for _, p := range []string{"kein-uuid", "kein-uuid/end", "kein-uuid/storno"} {
		method := http.MethodPost
		if p == "kein-uuid" {
			method = http.MethodGet
		}
		wantAPIError(t, do(method, fleetAssignmentsPath+p, `{}`), http.StatusBadRequest, "validation_error", "Ungültige Zuordnungs-ID")
	}
	wantAPIError(t, do(http.MethodGet, fleetAssignmentsPath+"?vehicle_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige vehicle_id")
	wantAPIError(t, do(http.MethodGet, fleetAssignmentsPath+"?employee_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige employee_id")
	if n := countFleetAssignments(t, env, frei.ID, w.ID); n != 0 {
		t.Fatalf("nach Fehlern %d Zuordnungen", n)
	}
}

func countFleetAssignments(t *testing.T, env *testutil.IntegrationEnv, vehicleIDs ...uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM fleet_vehicle_assignments WHERE vehicle_id = ANY($1)`, vehicleIDs).Scan(&n); err != nil {
		t.Fatalf("zaehlen: %v", err)
	}
	return n
}

// Schreiben nur mit fleet.write. Lesen: mit fleet.read alles, mit
// fleet.self nur die eigenen Zuordnungen.
func TestFleetAssignmentsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-fa-readonly", "fleet.read")
	seedRoleWithPermissions(t, env, "itest-fa-writeonly", "fleet.write")
	seedRoleWithPermissions(t, env, "itest-fa-none")
	c := newCatalogTenant(t, env, handler)
	fleetToken := c.user("fleet")
	vehicle := func() uuid.UUID {
		return decodeVehicle(t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, vehiclesPath, `{"kennzeichen":"M-P `+uuid.NewString()[:4]+`","art":"pkw"}`), http.StatusCreated).ID
	}
	monteur := c.linkedUser("production", "Monteur")
	personal := c.linkedUser("hr", "Personal")
	fremd := c.employee("Fremd")
	own := decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetAssignmentsPath, assignBody(vehicle(), monteur.EmployeeID, "2026-03-01")), http.StatusCreated)
	foreign := decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetAssignmentsPath, assignBody(vehicle(), fremd, "2026-03-01")), http.StatusCreated)

	// Selbstauskunft mit fleet.self (Produktion; auch die
	// Personalabteilung hat nur fleet.self).
	if list := decodeJSON[[]fleetAssignmentResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetAssignmentsPath, ""), http.StatusOK); len(list) != 1 || list[0].ID != own.ID {
		t.Fatalf("eigene Liste: %+v", list)
	}
	if list := decodeJSON[[]fleetAssignmentResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetAssignmentsPath+"?vehicle_id="+foreign.VehicleID.String(), ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("fremdes Fahrzeug: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetAssignmentsPath+"?employee_id="+fremd.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetAssignmentsPath+own.ID.String(), ""), http.StatusOK)
	wantAPIError(t, doAuthedRequest(t, handler, monteur.Token, http.MethodGet, fleetAssignmentsPath+foreign.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if list := decodeJSON[[]fleetAssignmentResponse](t, doAuthedRequest(t, handler, personal.Token, http.MethodGet, fleetAssignmentsPath, ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("Personalabteilung sieht nur eigene: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, c.user("production"), http.MethodGet, fleetAssignmentsPath, ""), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")

	readOnly := c.user("itest-fa-readonly")
	if list := decodeJSON[[]fleetAssignmentResponse](t, doAuthedRequest(t, handler, readOnly, http.MethodGet, fleetAssignmentsPath, ""), http.StatusOK); len(list) != 2 {
		t.Fatalf("read: alle: %+v", list)
	}
	decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, readOnly, http.MethodGet, fleetAssignmentsPath+foreign.ID.String(), ""), http.StatusOK)
	writeOnly := c.user("itest-fa-writeonly")
	none := c.user("itest-fa-none")
	for _, tok := range []string{writeOnly, none} {
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, fleetAssignmentsPath, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, fleetAssignmentsPath+uuid.NewString(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}

	for _, tc := range []struct {
		name     string
		token    string
		canWrite bool
	}{
		{"production (self)", monteur.Token, false},
		{"hr (self)", personal.Token, false},
		{"nur read", readOnly, false},
		{"keines", none, false},
		{"nur write", writeOnly, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vid := vehicle()
			rec := doAuthedRequest(t, handler, tc.token, http.MethodPost, fleetAssignmentsPath, assignBody(vid, fremd, "2026-03-01"))
			if !tc.canWrite {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				x := decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, fleetToken, http.MethodPost, fleetAssignmentsPath, assignBody(vid, fremd, "2026-03-01")), http.StatusCreated)
				wantAPIError(t, doAuthedRequest(t, handler, tc.token, http.MethodPost, fleetAssignmentsPath+x.ID.String()+"/end", `{"ende":"2026-03-02"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
				wantAPIError(t, doAuthedRequest(t, handler, tc.token, http.MethodPost, fleetAssignmentsPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			x := decodeJSON[fleetAssignmentResponse](t, rec, http.StatusCreated)
			decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, tc.token, http.MethodPost, fleetAssignmentsPath+x.ID.String()+"/end", `{"ende":"2026-03-02"}`), http.StatusOK)
			decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, tc.token, http.MethodPost, fleetAssignmentsPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusOK)
		})
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fleetAssignmentsPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestFleetAssignmentsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("fleet"), two.user("admin")
	empOne, empTwo := one.employee("Eins"), two.employee("Zwei")
	vOne := decodeVehicle(t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, vehiclesPath, `{"kennzeichen":"M-I 1","art":"pkw"}`), http.StatusCreated)
	vTwo := decodeVehicle(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, vehiclesPath, `{"kennzeichen":"M-I 1","art":"pkw"}`), http.StatusCreated)
	x := decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, fleetAssignmentsPath, assignBody(vOne.ID, empOne, "2026-03-01")), http.StatusCreated)

	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetAssignmentsPath+x.ID.String(), ""), http.StatusNotFound, "not_found", "Zuordnung nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetAssignmentsPath+x.ID.String()+"/end", `{"ende":"2026-03-02"}`), http.StatusNotFound, "not_found", "Zuordnung nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetAssignmentsPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Zuordnung nicht gefunden")
	if list := decodeJSON[[]fleetAssignmentResponse](t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, fleetAssignmentsPath+"?include_storniert=true", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("Liste des anderen Mandanten: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetAssignmentsPath, assignBody(vOne.ID, empTwo, "2026-03-01")), http.StatusNotFound, "not_found", "Fahrzeug nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, fleetAssignmentsPath, assignBody(vTwo.ID, empOne, "2026-03-01")), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	if still := decodeJSON[fleetAssignmentResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, fleetAssignmentsPath+x.ID.String(), ""), http.StatusOK); still.Ende != nil || still.Status != "active" {
		t.Fatalf("anderer Mandant hat die Zuordnung veraendert: %+v", still)
	}
}

// Jeder Fehlertext der Zuordnung muss als 4xx klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversFleetAssignmentMessages(t *testing.T) {
	cases := map[string]int{
		"vehicle_id erforderlich":                                                    http.StatusBadRequest,
		"employee_id erforderlich":                                                   http.StatusBadRequest,
		"Zuordnungs-ID erforderlich":                                                 http.StatusBadRequest,
		"Beginn erforderlich":                                                        http.StatusBadRequest,
		"Beginn darf nicht in der Zukunft liegen":                                    http.StatusBadRequest,
		"Beginn darf nicht vor der Erstzulassung liegen":                             http.StatusBadRequest,
		"Ende erforderlich":                                                          http.StatusBadRequest,
		"Ende darf nicht in der Zukunft liegen":                                      http.StatusBadRequest,
		"Ende darf nicht vor dem Beginn liegen":                                      http.StatusBadRequest,
		"Storno-Begründung erforderlich":                                             http.StatusBadRequest,
		"Fahrzeug nicht gefunden":                                                    http.StatusNotFound,
		"Mitarbeiter nicht gefunden":                                                 http.StatusNotFound,
		"Zuordnung nicht gefunden":                                                   http.StatusNotFound,
		"Fahrzeug ist abgemeldet":                                                    http.StatusConflict,
		"Mitarbeiter ist nicht aktiv":                                                http.StatusConflict,
		"Fahrzeug ist bereits zugeordnet":                                            http.StatusConflict,
		"Fahrzeug ist gesperrt: Hauptuntersuchung mit Mängeln":                       http.StatusConflict,
		"Fahrzeug ist gesperrt: UVV-Prüfung nicht bestanden":                         http.StatusConflict,
		"Zuordnung überschneidet sich mit einer früheren Zuordnung dieses Fahrzeugs": http.StatusConflict,
		"Zuordnung ist bereits beendet":                                              http.StatusConflict,
		"Zuordnung ist bereits storniert":                                            http.StatusConflict,
		"Zuordnung ist storniert":                                                    http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
