package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Stellen am Mitarbeiter und ihre Wirkung auf Stand und Fälligkeitsliste
// (Backlog F.11.3.1, ADR 0031 Entscheidung B: dauerhaft gelten). Helfer
// aus positions_integration_test.go, qualification_standing_integration_test.go,
// qualifications_integration_test.go und qualification_types_integration_test.go.
//
// Alle Arten sind unbefristet; der Stand haengt damit nicht vom Datum ab.

func decodeEmployeePositions(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []positionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	codes := make([]string, len(out))
	for i, p := range out {
		codes[i] = p.Code
		if !p.Aktiv {
			codes[i] += "(stillgelegt)"
		}
	}
	return codes
}

// standingCodes: die Codes des Stands, sortiert; gueltige mit "+", nicht
// erforderliche offene mit "?".
func standingCodes(st []standingResponse) string {
	out := make([]string, 0, len(st))
	for _, s := range st {
		if !s.Erforderlich && s.Status != "gueltig" {
			out = append(out, s.Code+"?")
			continue
		}
		if s.Status == "gueltig" {
			out = append(out, "+"+s.Code)
		} else {
			out = append(out, s.Code)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func dueCodes(list []dueResponse, emp uuid.UUID) string {
	out := make([]string, 0)
	for _, d := range list {
		if d.EmployeeID == emp {
			out = append(out, d.Code)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestEmployeePositionsDriveStandingAndDueList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	typ := map[string]uuid.UUID{}
	for _, code := range []string{"A", "B", "C", "D", "E"} {
		typ[code] = c.qualType(token, code, "null").ID
	}
	monteur := decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"MONTEUR","name":"Monteur","type_ids":`+idsJSON(typ["A"], typ["B"])+`}`), http.StatusCreated)
	kran := decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"KRAN","name":"Kranführer","type_ids":`+idsJSON(typ["B"], typ["C"])+`}`), http.StatusCreated)
	x := c.employee("Xaver")
	y := c.employee("Ypsilon")
	posPath := qualPath + "positions?employee_id=" + x.String()
	standing := func(emp uuid.UUID) string {
		return standingCodes(decodeStanding(t, do(http.MethodGet, qualPath+"standing?employee_id="+emp.String(), "")))
	}
	due := func(emp uuid.UUID) string { return dueCodes(decodeDue(t, do(http.MethodGet, qualPath+"due", "")), emp) }

	// Zwei Stellen (eine doppelt genannt), dazu eine eigene Anforderung.
	if got := decodeEmployeePositions(t, do(http.MethodPut, posPath, `{"position_ids":`+idsJSON(monteur.ID, kran.ID, monteur.ID)+`}`)); strings.Join(got, ",") != "KRAN,MONTEUR" {
		t.Fatalf("Stellen: %v", got)
	}
	decodeRequirements(t, do(http.MethodPut, qualPath+"requirements?employee_id="+x.String(), `{"type_ids":`+idsJSON(typ["D"])+`}`))
	if got, want := standing(x), "A,B,C,D"; got != want {
		t.Fatalf("Stand: %s, erwartet %s", got, want)
	}
	if got := due(x); got != "A,B,C,D" {
		t.Fatalf("Faelligkeitsliste: %s", got)
	}
	if got := standing(y); got != "" {
		t.Fatalf("Mitarbeiter ohne Stelle: %s", got)
	}
	// Die eigene Anforderung bleibt getrennt: die Anforderungsliste kennt
	// nur sie, nicht die Arten der Stellen.
	if got := decodeRequirements(t, do(http.MethodGet, qualPath+"requirements?employee_id="+x.String(), "")); len(got) != 1 {
		t.Fatalf("eigene Anforderungen: %v", got)
	}

	// Ein Nachweis fuer B: gueltig, nicht mehr in der Liste.
	decodeQualRecords(t, do(http.MethodPost, qualPath, qualBody(typ["B"], "2020-01-01", x)), http.StatusCreated)
	if got := standing(x); got != "+B,A,C,D" {
		t.Fatalf("Stand mit Nachweis: %s", got)
	}

	// Dauerhaft gelten: die Stelle aendert sich, der Stand sofort mit.
	decodePosition(t, do(http.MethodPut, positionsPath+monteur.ID.String(), `{"name":"Monteur","aktiv":true,"hinweis":"","type_ids":`+idsJSON(typ["A"], typ["E"])+`}`), http.StatusOK)
	decodePosition(t, do(http.MethodPut, positionsPath+kran.ID.String(), `{"name":"Kranführer","aktiv":true,"hinweis":"","type_ids":`+idsJSON(typ["B"])+`}`), http.StatusOK)
	if got := standing(x); got != "+B,A,D,E" {
		t.Fatalf("Stand nach Aenderung der Stellen: %s", got)
	}
	if got := due(x); got != "A,D,E" {
		t.Fatalf("Liste nach Aenderung der Stellen: %s", got)
	}

	// Stillgelegte Stelle: zaehlt nicht mehr, bleibt aber zugeordnet und
	// speicherbar; neu zuordnen laesst sie sich nicht.
	decodePosition(t, do(http.MethodPut, positionsPath+monteur.ID.String(), `{"name":"Monteur","aktiv":false,"hinweis":"","type_ids":`+idsJSON(typ["A"], typ["E"])+`}`), http.StatusOK)
	if got := standing(x); got != "+B,D" {
		t.Fatalf("Stand mit stillgelegter Stelle: %s", got)
	}
	if got := due(x); got != "D" {
		t.Fatalf("Liste mit stillgelegter Stelle: %s", got)
	}
	if got := decodeEmployeePositions(t, do(http.MethodGet, posPath, "")); strings.Join(got, ",") != "KRAN,MONTEUR(stillgelegt)" {
		t.Fatalf("Stellen mit stillgelegter: %v", got)
	}
	decodeEmployeePositions(t, do(http.MethodPut, posPath, `{"position_ids":`+idsJSON(monteur.ID, kran.ID)+`}`))
	wantAPIError(t, do(http.MethodPut, qualPath+"positions?employee_id="+y.String(), `{"position_ids":`+idsJSON(monteur.ID)+`}`), http.StatusConflict, "conflict", "Stelle ist stillgelegt")
	if got := decodeEmployeePositions(t, do(http.MethodGet, qualPath+"positions?employee_id="+y.String(), "")); len(got) != 0 {
		t.Fatalf("abgelehnte Zuordnung gespeichert: %v", got)
	}

	// Alle Stellen entfernen: nur die eigene Anforderung bleibt.
	if got := decodeEmployeePositions(t, do(http.MethodPut, posPath, `{"position_ids":[]}`)); len(got) != 0 {
		t.Fatalf("nach Entfernen: %v", got)
	}
	if got := standing(x); got != "+B,D" {
		t.Fatalf("Stand ohne Stellen: %s", got)
	}
}

func TestEmployeePositionsValidationAndPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	hrToken := c.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}
	pos := decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"MONTEUR","name":"Monteur"}`), http.StatusCreated)
	worker := c.linkedUser("production", "Werker")
	colleague := c.employee("Kollege")
	other := newCatalogTenant(t, env, handler)
	otherToken := other.user("hr")
	foreignPos := decodePosition(t, doAuthedRequest(t, handler, otherToken, http.MethodPost, positionsPath, `{"code":"FREMD","name":"Fremd"}`), http.StatusCreated)
	foreignEmp := other.employee("Fremd")
	path := qualPath + "positions?employee_id=" + worker.EmployeeID.String()

	countAssignments := func() int {
		var n int
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM hr_employee_positions ep JOIN hr_employees e ON e.id=ep.employee_id WHERE e.company_id IN ($1,$2)`, c.company, other.company).Scan(&n); err != nil {
			t.Fatalf("zaehlen: %v", err)
		}
		return n
	}
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code, msg                string
	}{
		{"ohne employee_id", http.MethodPut, qualPath + "positions", `{"position_ids":[]}`, 400, "validation_error", "employee_id erforderlich"},
		{"employee_id ungueltig", http.MethodPut, qualPath + "positions?employee_id=xyz", `{"position_ids":[]}`, 400, "validation_error", "Ungültige employee_id"},
		{"Feld fehlt", http.MethodPut, path, `{}`, 400, "validation_error", "Feld erforderlich: position_ids"},
		{"kein JSON", http.MethodPut, path, `{`, 400, "validation_error", "Ungültige Eingabe"},
		{"Stelle leer", http.MethodPut, path, `{"position_ids":["00000000-0000-0000-0000-000000000000"]}`, 400, "validation_error", "position_id erforderlich"},
		{"Stelle unbekannt", http.MethodPut, path, `{"position_ids":` + idsJSON(pos.ID, uuid.New()) + `}`, 404, "not_found", "Stelle nicht gefunden"},
		{"Stelle fremder Mandant", http.MethodPut, path, `{"position_ids":` + idsJSON(foreignPos.ID) + `}`, 404, "not_found", "Stelle nicht gefunden"},
		{"Mitarbeiter fremder Mandant", http.MethodPut, qualPath + "positions?employee_id=" + foreignEmp.String(), `{"position_ids":` + idsJSON(pos.ID) + `}`, 404, "not_found", "Mitarbeiter nicht gefunden"},
		{"Mitarbeiter unbekannt", http.MethodPut, qualPath + "positions?employee_id=" + uuid.NewString(), `{"position_ids":[]}`, 404, "not_found", "Mitarbeiter nicht gefunden"},
		{"Lesen fremder Mitarbeiter", http.MethodGet, qualPath + "positions?employee_id=" + foreignEmp.String(), "", 404, "not_found", "Mitarbeiter nicht gefunden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantAPIError(t, do(tc.method, tc.path, tc.body), tc.status, tc.code, tc.msg)
		})
	}
	if n := countAssignments(); n != 0 {
		t.Fatalf("nach Fehlern %d Zuordnungen", n)
	}

	decodeEmployeePositions(t, do(http.MethodPut, path, `{"position_ids":`+idsJSON(pos.ID)+`}`))
	// self: die eigenen Stellen lesen (ohne employee_id), fremde nicht,
	// setzen gar nicht.
	if got := decodeEmployeePositions(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, qualPath+"positions", "")); strings.Join(got, ",") != "MONTEUR" {
		t.Fatalf("eigene Stellen: %v", got)
	}
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, qualPath+"positions?employee_id="+colleague.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPut, path, `{"position_ids":[]}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if got := decodeEmployeePositions(t, do(http.MethodGet, path, "")); strings.Join(got, ",") != "MONTEUR" {
		t.Fatalf("nach verweigertem Setzen: %v", got)
	}
	// Die fremde Stelle blieb unberuehrt und ohne Inhaber.
	if n := countAssignments(); n != 1 {
		t.Fatalf("Zuordnungen: %d", n)
	}
}

func TestClassifyDomainErrorCoversEmployeePositionMessages(t *testing.T) {
	cases := map[string]int{
		"Stelle ist stillgelegt":          http.StatusConflict,
		"position_id erforderlich":        http.StatusBadRequest,
		"Feld erforderlich: position_ids": http.StatusBadRequest,
		"Stelle nicht gefunden":           http.StatusNotFound,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
