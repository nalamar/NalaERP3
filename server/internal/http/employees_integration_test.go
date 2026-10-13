package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung des Mitarbeiterstamms (Backlog F.4.4). Helfer
// (seedTimeEntryUser, doAuthedRequest, wantAPIError) aus
// time_entries_integration_test.go.

type employeeResponse struct {
	ID         uuid.UUID  `json:"id"`
	PersonalNr *string    `json:"personalnummer"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	Email      string     `json:"email"`
	TeamID     *uuid.UUID `json:"team_id"`
	Active     bool       `json:"active"`
	UserID     *string    `json:"user_id"`
}

func decodeEmployee(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) employeeResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out employeeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestEmployeesCreateGetListUpdateFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	hrUser := seedTimeEntryUser(t, env, handler, "emp-crud-hr@example.com", "hr", false)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrUser.Token, method, path, body)
	}
	pn := "PN-" + uuid.NewString()[:8]
	teamID := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_teams (id, name, company_id) VALUES ($1,$2,'default')`, teamID, "Werkstatt "+pn); err != nil {
		t.Fatalf("Team anlegen: %v", err)
	}

	// Anlegen OHNE active: der Mitarbeiter ist aktiv (Entscheidung des
	// Nutzers vom 2026-10-13), hier ueber die HTTP-Grenze belegt.
	created := decodeEmployee(t, do(http.MethodPost, "/api/v1/employees/",
		`{"first_name":"Erika","last_name":"Musterfrau","email":"erika@example.invalid","personalnummer":"`+pn+`","team_id":"`+teamID.String()+`"}`), http.StatusCreated)
	if !created.Active {
		t.Fatalf("erwartet aktiven Mitarbeiter, bekommen %+v", created)
	}
	if created.PersonalNr == nil || *created.PersonalNr != pn || created.TeamID == nil || *created.TeamID != teamID {
		t.Fatalf("Personalnummer/Team nicht wie angegeben: %+v", created)
	}
	inactive := decodeEmployee(t, do(http.MethodPost, "/api/v1/employees/", `{"first_name":"Ina","last_name":"Aktiv","active":false}`), http.StatusCreated)
	if inactive.Active {
		t.Fatalf("ausdruecklich inaktiv angelegter Mitarbeiter ist aktiv: %+v", inactive)
	}

	got := decodeEmployee(t, do(http.MethodGet, "/api/v1/employees/"+created.ID.String(), ""), http.StatusOK)
	if got.ID != created.ID || got.LastName != "Musterfrau" || !got.Active {
		t.Fatalf("Einzelabruf liefert nicht den angelegten Mitarbeiter: %+v", got)
	}

	listRec := do(http.MethodGet, "/api/v1/employees/?limit=500", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("Liste: erwartet 200, bekommen %d mit Body %s", listRec.Code, listRec.Body.String())
	}
	var list []employeeResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Liste nicht lesbar: %v", err)
	}
	var found bool
	for _, e := range list {
		found = found || e.ID == created.ID
	}
	if !found {
		t.Fatalf("Liste enthaelt den angelegten Mitarbeiter nicht (%d Eintraege)", len(list))
	}

	// "teams" ist eine eigene Route und darf nicht als Mitarbeiter-ID
	// verstanden werden.
	teamsRec := do(http.MethodGet, "/api/v1/employees/teams", "")
	if teamsRec.Code != http.StatusOK {
		t.Fatalf("Teams: erwartet 200, bekommen %d mit Body %s", teamsRec.Code, teamsRec.Body.String())
	}
	var teams []struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(teamsRec.Body.Bytes(), &teams); err != nil {
		t.Fatalf("Teams nicht lesbar: %v", err)
	}
	var teamFound bool
	for _, tm := range teams {
		teamFound = teamFound || tm.ID == teamID
	}
	if !teamFound {
		t.Fatal("Teamliste enthaelt das angelegte Team nicht")
	}

	updated := decodeEmployee(t, do(http.MethodPatch, "/api/v1/employees/"+created.ID.String(),
		`{"first_name":"Erika-Maria","active":false,"team_id":null}`), http.StatusOK)
	if updated.FirstName != "Erika-Maria" || updated.Active || updated.TeamID != nil {
		t.Fatalf("Aenderung nicht wie angegeben: %+v", updated)
	}

	// Fehlerfaelle - jeder mit Status, Code und Text.
	wantAPIError(t, do(http.MethodPost, "/api/v1/employees/", `{"first_name":"Nur"}`),
		http.StatusBadRequest, "validation_error", "Vor- und Nachname erforderlich")
	wantAPIError(t, do(http.MethodPost, "/api/v1/employees/", `{"first_name":"Doppelt","last_name":"Nummer","personalnummer":"`+pn+`"}`),
		http.StatusBadRequest, "validation_error", "Personalnummer existiert bereits")
	wantAPIError(t, do(http.MethodPost, "/api/v1/employees/", `{"first_name":"Team","last_name":"Fremd","team_id":"`+uuid.NewString()+`"}`),
		http.StatusNotFound, "not_found", "Team nicht gefunden")
	wantAPIError(t, do(http.MethodPost, "/api/v1/employees/", `{`),
		http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPatch, "/api/v1/employees/"+created.ID.String(), `{"activ":true}`),
		http.StatusBadRequest, "validation_error", "unbekanntes Feld: activ")
	wantAPIError(t, do(http.MethodPatch, "/api/v1/employees/"+created.ID.String(), `{"active":"ja"}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Wert für Feld: active")
	// Die Verknuepfung laesst sich NICHT ueber den Patch setzen.
	wantAPIError(t, do(http.MethodPatch, "/api/v1/employees/"+created.ID.String(), `{"user_id":"`+hrUser.UserID+`"}`),
		http.StatusBadRequest, "validation_error", "unbekanntes Feld: user_id")
	wantAPIError(t, do(http.MethodPatch, "/api/v1/employees/"+uuid.NewString(), `{"first_name":"X"}`),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodGet, "/api/v1/employees/"+uuid.NewString(), ""),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodGet, "/api/v1/employees/kein-uuid", ""),
		http.StatusBadRequest, "validation_error", "Ungültige Mitarbeiter-ID")
}

// Der Zweck von F.4: ein ueber HTTP angelegter und verknuepfter
// Mitarbeiter kann stempeln - ohne einen einzigen Direktzugriff auf die
// Datenbank.
func TestEmployeesLinkUserEnablesTimeTracking(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	hrUser := seedTimeEntryUser(t, env, handler, "emp-link-hr@example.com", "hr", false)
	worker := seedTimeEntryUser(t, env, handler, "emp-link-worker@example.com", "production", false)
	asHR := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrUser.Token, method, path, body)
	}
	stamp := func() *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/", `{}`)
	}

	// Vorher: der Login hat keinen Mitarbeiter und kann nicht stempeln.
	wantAPIError(t, stamp(), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")

	emp := decodeEmployee(t, asHR(http.MethodPost, "/api/v1/employees/", `{"first_name":"Werner","last_name":"Werkstatt"}`), http.StatusCreated)
	if emp.UserID != nil {
		t.Fatalf("neuer Mitarbeiter darf nicht verknuepft sein: %+v", emp)
	}
	linked := decodeEmployee(t, asHR(http.MethodPut, "/api/v1/employees/"+emp.ID.String()+"/user", `{"user_id":"`+worker.UserID+`"}`), http.StatusOK)
	if linked.UserID == nil || *linked.UserID != worker.UserID {
		t.Fatalf("erwartet user_id=%s, bekommen %v", worker.UserID, linked.UserID)
	}

	// Nachher: der Mitarbeiter stempelt, und zwar fuer genau diesen Datensatz.
	entry := decodeTimeEntry(t, stamp(), http.StatusCreated)
	if entry.EmployeeID != emp.ID {
		t.Fatalf("erwartet Zeiteintrag fuer %s, bekommen %s", emp.ID, entry.EmployeeID)
	}

	// Ein Login gehoert zu hoechstens einem Mitarbeiter.
	other := decodeEmployee(t, asHR(http.MethodPost, "/api/v1/employees/", `{"first_name":"Zweiter","last_name":"Mitarbeiter"}`), http.StatusCreated)
	wantAPIError(t, asHR(http.MethodPut, "/api/v1/employees/"+other.ID.String()+"/user", `{"user_id":"`+worker.UserID+`"}`),
		http.StatusConflict, "conflict", "Benutzer ist bereits mit einem anderen Mitarbeiter verknüpft")
	wantAPIError(t, asHR(http.MethodPut, "/api/v1/employees/"+other.ID.String()+"/user", `{"user_id":"gibt-es-nicht"}`),
		http.StatusNotFound, "not_found", "Benutzer nicht gefunden")
	wantAPIError(t, asHR(http.MethodPut, "/api/v1/employees/"+uuid.NewString()+"/user", `{"user_id":null}`),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")

	// Loesen: danach ist wieder Schluss mit Stempeln.
	unlinked := decodeEmployee(t, asHR(http.MethodPut, "/api/v1/employees/"+emp.ID.String()+"/user", `{"user_id":null}`), http.StatusOK)
	if unlinked.UserID != nil {
		t.Fatalf("erwartet geloeste Verknuepfung, bekommen %v", *unlinked.UserID)
	}
	wantAPIError(t, stamp(), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")
	// Der bereits erfasste Eintrag bleibt beim Mitarbeiter.
	var count int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM hr_time_entries WHERE employee_id=$1`, emp.ID).Scan(&count); err != nil {
		t.Fatalf("Eintraege zaehlen: %v", err)
	}
	if count != 1 {
		t.Fatalf("erwartet weiterhin einen Zeiteintrag, bekommen %d", count)
	}
}

// Der Mitarbeiterstamm ist der Personalabteilung vorbehalten: ohne
// employees.read/write gibt es nichts zu sehen - auch nicht den eigenen
// Datensatz.
func TestEmployeesRequirePermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "emp-perm-worker@example.com", "production", true)
	own := "/api/v1/employees/" + worker.EmployeeID.String()

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/employees/", ""},
		{http.MethodGet, "/api/v1/employees/teams", ""},
		{http.MethodPost, "/api/v1/employees/", `{"first_name":"X","last_name":"Y"}`},
		{http.MethodGet, own, ""},
		{http.MethodPatch, own, `{"first_name":"Selbst"}`},
		{http.MethodPut, own + "/user", `{"user_id":null}`},
	} {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			wantAPIError(t, doAuthedRequest(t, handler, worker.Token, c.method, c.path, c.body), http.StatusForbidden, "forbidden", "Keine Berechtigung")
			req := httptest.NewRequest(c.method, c.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
			}
		})
	}

	// Nichts davon hat etwas veraendert.
	var firstName string
	var userID *string
	if err := env.PG.QueryRow(context.Background(), `SELECT first_name, user_id FROM hr_employees WHERE id=$1`, worker.EmployeeID).Scan(&firstName, &userID); err != nil {
		t.Fatalf("Mitarbeiter lesen: %v", err)
	}
	if firstName != "Zeit" || userID == nil {
		t.Fatalf("abgelehnte Aufrufe haben den Datensatz veraendert: first_name=%q user_id=%v", firstName, userID)
	}
}

// Lesen und Schreiben sind getrennte Rechte. Keine der Systemrollen hat
// das eine ohne das andere - deshalb eine eigene Rolle nur mit
// employees.read, sonst bliebe unbemerkt, wenn eine Schreibroute
// versehentlich nur das Leserecht verlangt.
func TestEmployeesReadPermissionDoesNotAllowWriting(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	if _, err := env.PG.Exec(ctx, `INSERT INTO roles (id, code, name, description, is_system) VALUES ('role-itest-emp-readonly','itest-emp-readonly','Test: Mitarbeiter nur lesen','',false) ON CONFLICT (code) DO NOTHING`); err != nil {
		t.Fatalf("Leserolle anlegen: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) SELECT 'role-itest-emp-readonly', id FROM permissions WHERE code='employees.read' ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("Leserecht zuordnen: %v", err)
	}
	hrUser := seedTimeEntryUser(t, env, handler, "emp-ro-hr@example.com", "hr", false)
	reader := seedTimeEntryUser(t, env, handler, "emp-ro-reader@example.com", "itest-emp-readonly", false)
	emp := decodeEmployee(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/employees/", `{"first_name":"Nur","last_name":"Lesbar"}`), http.StatusCreated)
	path := "/api/v1/employees/" + emp.ID.String()
	do := func(method, p, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, reader.Token, method, p, body)
	}

	// Lesen geht.
	decodeEmployee(t, do(http.MethodGet, path, ""), http.StatusOK)
	for _, p := range []string{"/api/v1/employees/", "/api/v1/employees/teams"} {
		if rec := do(http.MethodGet, p, ""); rec.Code != http.StatusOK {
			t.Fatalf("GET %s: erwartet 200, bekommen %d mit Body %s", p, rec.Code, rec.Body.String())
		}
	}
	// Schreiben nicht.
	wantAPIError(t, do(http.MethodPost, "/api/v1/employees/", `{"first_name":"X","last_name":"Y"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, do(http.MethodPatch, path, `{"first_name":"Geaendert"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, do(http.MethodPut, path+"/user", `{"user_id":"`+reader.UserID+`"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")

	still := decodeEmployee(t, do(http.MethodGet, path, ""), http.StatusOK)
	if still.FirstName != "Nur" || still.UserID != nil {
		t.Fatalf("abgelehnte Schreibzugriffe haben den Datensatz veraendert: %+v", still)
	}
}

func TestEmployeesAreIsolatedBetweenCompaniesOverHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	hrUser := seedTimeEntryUser(t, env, handler, "emp-iso-hr@example.com", "hr", false)
	emp := decodeEmployee(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/employees/", `{"first_name":"Mandant","last_name":"Eins"}`), http.StatusCreated)

	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ('other-company','Andere Firma GmbH') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("zweiten Mandanten anlegen: %v", err)
	}
	foreignID := testutil.SeedAuthUser(t, env, "emp-iso-admin@other.example.com", "Secret123!", "admin")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET company_id='other-company' WHERE id=$1`, foreignID); err != nil {
		t.Fatalf("Benutzer dem zweiten Mandanten zuordnen: %v", err)
	}
	foreign := loginIntegrationUser(t, handler, "emp-iso-admin@other.example.com", "Secret123!")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, foreign, method, path, body)
	}
	path := "/api/v1/employees/" + emp.ID.String()

	wantAPIError(t, do(http.MethodGet, path, ""), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPatch, path, `{"first_name":"Fremd"}`), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPut, path+"/user", `{"user_id":null}`), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")

	listRec := do(http.MethodGet, "/api/v1/employees/?limit=500", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("Liste: erwartet 200, bekommen %d", listRec.Code)
	}
	var list []employeeResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Liste nicht lesbar: %v", err)
	}
	for _, e := range list {
		if e.ID == emp.ID {
			t.Fatal("Liste des fremden Mandanten enthaelt den Mitarbeiter")
		}
	}

	// Der fremde Administrator kann einen Login des ERSTEN Mandanten nicht
	// an einen eigenen Mitarbeiter binden.
	mine := decodeEmployee(t, do(http.MethodPost, "/api/v1/employees/", `{"first_name":"Mandant","last_name":"Zwei"}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPut, "/api/v1/employees/"+mine.ID.String()+"/user", `{"user_id":"`+hrUser.UserID+`"}`),
		http.StatusNotFound, "not_found", "Benutzer nicht gefunden")

	still := decodeEmployee(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodGet, path, ""), http.StatusOK)
	if still.FirstName != "Mandant" {
		t.Fatalf("Zugriff aus fremdem Mandanten hat den Mitarbeiter veraendert: %+v", still)
	}
}

// Jeder Fehlertext des Mitarbeiterstamms muss als 4xx klassifiziert
// werden (siehe TestClassifyDomainErrorCoversTimeEntryMessages). DB-los.
func TestClassifyDomainErrorCoversEmployeeMessages(t *testing.T) {
	cases := map[string]int{
		"Vor- und Nachname erforderlich":                               http.StatusBadRequest,
		"Mitarbeiter-ID erforderlich":                                  http.StatusBadRequest,
		"unbekanntes Feld: activ":                                      http.StatusBadRequest,
		"Ungültiger Wert für Feld: active":                             http.StatusBadRequest,
		"Personalnummer existiert bereits":                             http.StatusBadRequest,
		"Mitarbeiter nicht gefunden":                                   http.StatusNotFound,
		"Team nicht gefunden":                                          http.StatusNotFound,
		"Benutzer nicht gefunden":                                      http.StatusNotFound,
		"Benutzer ist bereits mit einem anderen Mitarbeiter verknüpft": http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
