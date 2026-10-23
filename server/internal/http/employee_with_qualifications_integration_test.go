package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Anlage eines Mitarbeiters mit Anforderungen und Nachweisen in einem
// Aufruf (Backlog F.10.5.1). Helfer aus qualification_types_integration_test.go
// (newCatalogTenant), qualifications_integration_test.go und
// leave_requests_integration_test.go (seedRoleWithPermissions).

const onboardingPath = "/api/v1/employees/with-qualifications"

type onboardingResponse struct {
	Employee     employeeResponse `json:"employee"`
	Requirements []struct {
		TypeID uuid.UUID `json:"type_id"`
	} `json:"requirements"`
	Qualifications []qualRecordResponse `json:"qualifications"`
}

func decodeOnboarding(t *testing.T, rec *httptest.ResponseRecorder) onboardingResponse {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("erwartet 201, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out onboardingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

// tenantCounts zaehlt Mitarbeiter, Anforderungen und Nachweise des Mandanten.
func (c *catalogTenant) tenantCounts() [3]int {
	c.t.Helper()
	var out [3]int
	if err := c.env.PG.QueryRow(context.Background(), `SELECT
			(SELECT count(*) FROM hr_employees WHERE company_id=$1),
			(SELECT count(*) FROM hr_employee_qualification_requirements r JOIN hr_employees e ON e.id=r.employee_id WHERE e.company_id=$1),
			(SELECT count(*) FROM hr_employee_qualifications q JOIN hr_employees e ON e.id=q.employee_id WHERE e.company_id=$1)`, c.company).
		Scan(&out[0], &out[1], &out[2]); err != nil {
		c.t.Fatalf("zaehlen: %v", err)
	}
	return out
}

func TestEmployeeWithQualificationsCreatesAllAtOnce(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	annual := c.qualType(token, "EH", "12")
	open := c.qualType(token, "STAPLER", "null")
	other := c.qualType(token, "KRAN", "null")

	// Anforderung doppelt genannt: eine. Nachweis ohne gueltig_bis: aus der
	// Frist berechnet; mit gueltig_bis: wie angegeben.
	out := decodeOnboarding(t, doAuthedRequest(t, handler, token, http.MethodPost, onboardingPath, `{
		"employee": {"first_name":"Neu","last_name":"Eingestellt","personalnummer":"P-1"},
		"required_type_ids": ["`+annual.ID.String()+`","`+open.ID.String()+`","`+annual.ID.String()+`"],
		"qualifications": [
			{"type_id":"`+annual.ID.String()+`","erworben_am":"2026-03-15","aussteller":"DRK","nachweis_nr":"N-7","note":"Kurs A"},
			{"type_id":"`+other.ID.String()+`","erworben_am":"2025-01-10","gueltig_bis":"2030-01-10"}
		]}`))
	if out.Employee.FirstName != "Neu" || !out.Employee.Active || out.Employee.PersonalNr == nil || *out.Employee.PersonalNr != "P-1" {
		t.Fatalf("Mitarbeiter nicht wie angegeben: %+v", out.Employee)
	}
	reqs := map[uuid.UUID]bool{}
	for _, r := range out.Requirements {
		reqs[r.TypeID] = true
	}
	if len(out.Requirements) != 2 || !reqs[annual.ID] || !reqs[open.ID] {
		t.Fatalf("Anforderungen: %+v", out.Requirements)
	}
	if len(out.Qualifications) != 2 {
		t.Fatalf("erwartet 2 Nachweise, bekommen %+v", out.Qualifications)
	}
	first, second := out.Qualifications[0], out.Qualifications[1]
	if first.EmployeeID != out.Employee.ID || first.TypeID != annual.ID || first.GueltigBis == nil || (*first.GueltigBis)[:10] != "2027-03-15" {
		t.Fatalf("erster Nachweis: %+v", first)
	}
	if second.TypeID != other.ID || second.GueltigBis == nil || (*second.GueltigBis)[:10] != "2030-01-10" {
		t.Fatalf("zweiter Nachweis: %+v", second)
	}
	if first.CreatedByUserID == nil || *first.CreatedByUserID == "" {
		t.Fatalf("Urheber fehlt: %+v", first)
	}
	var aussteller, nr, note string
	if err := env.PG.QueryRow(context.Background(), `SELECT aussteller, nachweis_nr, note FROM hr_employee_qualifications WHERE id=$1`, first.ID).Scan(&aussteller, &nr, &note); err != nil {
		t.Fatalf("Nachweis lesen: %v", err)
	}
	if aussteller != "DRK" || nr != "N-7" || note != "Kurs A" {
		t.Fatalf("Angaben nicht uebernommen: %q %q %q", aussteller, nr, note)
	}
	if got := c.tenantCounts(); got != [3]int{1, 2, 2} {
		t.Fatalf("gespeichert: %v", got)
	}

	// Ohne Anforderungen und Nachweise: wie die einfache Anlage.
	plain := decodeOnboarding(t, doAuthedRequest(t, handler, token, http.MethodPost, onboardingPath, `{"employee":{"first_name":"Nur","last_name":"Stamm"}}`))
	if len(plain.Requirements) != 0 || len(plain.Qualifications) != 0 || plain.Employee.LastName != "Stamm" {
		t.Fatalf("Anlage ohne Qualifikationen: %+v", plain)
	}
	if got := c.tenantCounts(); got != [3]int{2, 2, 2} {
		t.Fatalf("gespeichert: %v", got)
	}
}

// Scheitert irgendein Teil, ist NICHTS gespeichert - auch nicht der
// Mitarbeiter, der in der Transaktion schon angelegt war.
func TestEmployeeWithQualificationsIsAllOrNothing(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	annual := c.qualType(token, "EH", "12")
	stillgelegt := c.qualType(token, "ALT", "null")
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_qualification_types SET aktiv=false WHERE id=$1`, stillgelegt.ID); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}
	foreign := newCatalogTenant(t, env, handler)
	foreignType := foreign.qualType(foreign.user("hr"), "FREMD", "null")
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO hr_employees (id, first_name, last_name, personalnummer, company_id) VALUES ($1,'Schon','Da','P-DOPPELT',$2)`, uuid.New(), c.company); err != nil {
		t.Fatalf("Mitarbeiter mit Personalnummer: %v", err)
	}
	before := c.tenantCounts()

	emp := `"employee":{"first_name":"Teil","last_name":"Erfolg"}`
	ok := `{"type_id":"` + annual.ID.String() + `","erworben_am":"2026-03-15"}`
	cases := []struct {
		name, body string
		status     int
		code, msg  string
	}{
		{"Anforderung unbekannt", `{` + emp + `,"required_type_ids":["` + uuid.NewString() + `"],"qualifications":[` + ok + `]}`, http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden"},
		{"Anforderung fremder Mandant", `{` + emp + `,"required_type_ids":["` + foreignType.ID.String() + `"]}`, http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden"},
		{"Anforderung stillgelegt", `{` + emp + `,"required_type_ids":["` + stillgelegt.ID.String() + `"]}`, http.StatusConflict, "conflict", "Qualifikationsart ist stillgelegt"},
		{"Nachweis fremder Mandant", `{` + emp + `,"required_type_ids":["` + annual.ID.String() + `"],"qualifications":[` + ok + `,{"type_id":"` + foreignType.ID.String() + `","erworben_am":"2026-03-15"}]}`, http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden"},
		{"Nachweis stillgelegt", `{` + emp + `,"qualifications":[` + ok + `,{"type_id":"` + stillgelegt.ID.String() + `","erworben_am":"2026-03-15"}]}`, http.StatusConflict, "conflict", "Qualifikationsart ist stillgelegt"},
		{"Nachweis doppelt", `{` + emp + `,"qualifications":[` + ok + `,` + ok + `]}`, http.StatusConflict, "conflict", "Nachweis für diesen Tag ist bereits erfasst"},
		{"Erwerbsdatum in der Zukunft", `{` + emp + `,"qualifications":[{"type_id":"` + annual.ID.String() + `","erworben_am":"2099-01-01"}]}`, http.StatusBadRequest, "validation_error", "Erwerbsdatum darf nicht in der Zukunft liegen"},
		{"Erwerbsdatum fehlt", `{` + emp + `,"qualifications":[{"type_id":"` + annual.ID.String() + `"}]}`, http.StatusBadRequest, "validation_error", "Erwerbsdatum erforderlich"},
		{"Datum falsch geschrieben", `{` + emp + `,"qualifications":[{"type_id":"` + annual.ID.String() + `","erworben_am":"15.03.2026"}]}`, http.StatusBadRequest, "validation_error", "Ungültiges Datum in erworben_am (erwartet JJJJ-MM-TT)"},
		{"Art fehlt", `{` + emp + `,"qualifications":[{"erworben_am":"2026-03-15"}]}`, http.StatusBadRequest, "validation_error", "type_id erforderlich"},
		{"Name fehlt", `{"employee":{"first_name":"Ohne"},"required_type_ids":["` + annual.ID.String() + `"]}`, http.StatusBadRequest, "validation_error", "Vor- und Nachname erforderlich"},
		{"Personalnummer doppelt", `{"employee":{"first_name":"A","last_name":"B","personalnummer":"P-DOPPELT"},"required_type_ids":["` + annual.ID.String() + `"]}`, http.StatusBadRequest, "validation_error", "Personalnummer existiert bereits"},
		{"Team unbekannt", `{"employee":{"first_name":"A","last_name":"B","team_id":"` + uuid.NewString() + `"},"required_type_ids":["` + annual.ID.String() + `"]}`, http.StatusNotFound, "not_found", "Team nicht gefunden"},
		{"kein JSON", `{`, http.StatusBadRequest, "validation_error", "Ungültige Eingabe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, onboardingPath, tc.body), tc.status, tc.code, tc.msg)
			if got := c.tenantCounts(); got != before {
				t.Fatalf("nach Fehler gespeichert: vorher %v, nachher %v", before, got)
			}
		})
	}
}

// Beide Schreibrechte sind noetig; die Systemrollen haben sie nur
// zusammen, deshalb Testrollen mit je einem.
func TestEmployeeWithQualificationsNeedsBothPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	annual := c.qualType(c.user("hr"), "EH", "12")
	seedRoleWithPermissions(t, env, "itest-onb-emp-only", "employees.write", "employees.read")
	seedRoleWithPermissions(t, env, "itest-onb-qual-only", "qualifications.write", "qualifications.read")
	body := `{"employee":{"first_name":"Ohne","last_name":"Recht"},"required_type_ids":["` + annual.ID.String() + `"]}`
	before := c.tenantCounts()
	for _, role := range []string{"itest-onb-emp-only", "itest-onb-qual-only", "sales"} {
		wantAPIError(t, doAuthedRequest(t, handler, c.user(role), http.MethodPost, onboardingPath, body), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}
	if got := c.tenantCounts(); got != before {
		t.Fatalf("ohne Recht gespeichert: vorher %v, nachher %v", before, got)
	}
}
