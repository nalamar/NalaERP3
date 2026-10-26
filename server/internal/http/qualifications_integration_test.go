package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung der Nachweise und Anforderungen (Backlog F.2.5.2). Helfer
// aus qualification_types_integration_test.go (newCatalogTenant) und den
// Testdateien der Zeiterfassung.
//
// Alle Erwerbsdaten liegen fest in der Vergangenheit.

type qualPerson struct {
	Token      string
	EmployeeID uuid.UUID
}

// employee legt einen Mitarbeiter im Mandanten an.
func (c *catalogTenant) employee(lastName string) uuid.UUID {
	c.t.Helper()
	emp, err := hr.NewEmployeeService(c.env.PG).Create(context.Background(), hr.EmployeeInput{FirstName: "Quali", LastName: lastName}, c.company)
	if err != nil {
		c.t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return emp.ID
}

// linkedUser legt einen Benutzer mit der Rolle an, dazu einen Mitarbeiter,
// mit dem er verknuepft ist.
func (c *catalogTenant) linkedUser(role, lastName string) qualPerson {
	c.t.Helper()
	empID := c.employee(lastName)
	email := c.company + "-" + role + "-" + uuid.NewString()[:6] + "@example.com"
	userID := testutil.SeedAuthUser(c.t, c.env, email, "Secret123!", role)
	ctx := context.Background()
	if _, err := c.env.PG.Exec(ctx, `UPDATE users SET company_id=$2 WHERE id=$1`, userID, c.company); err != nil {
		c.t.Fatalf("Benutzer dem Mandanten zuordnen: %v", err)
	}
	if _, err := c.env.PG.Exec(ctx, `UPDATE hr_employees SET user_id=$1 WHERE id=$2`, userID, empID); err != nil {
		c.t.Fatalf("Benutzer verknuepfen: %v", err)
	}
	return qualPerson{Token: loginIntegrationUser(c.t, c.handler, email, "Secret123!"), EmployeeID: empID}
}

func (c *catalogTenant) qualType(token, code string, months string) qualificationTypeResponse {
	c.t.Helper()
	kategorie := "qualifikation"
	if months != "null" {
		kategorie = "unterweisung"
	}
	return decodeQualificationType(c.t, doAuthedRequest(c.t, c.handler, token, http.MethodPost, qtPath,
		`{"code":"`+code+`","name":"Art `+code+`","kategorie":"`+kategorie+`","gueltigkeit_monate":`+months+`}`), http.StatusCreated)
}

type qualRecordResponse struct {
	ID              uuid.UUID `json:"id"`
	EmployeeID      uuid.UUID `json:"employee_id"`
	TypeID          uuid.UUID `json:"type_id"`
	ErworbenAm      string    `json:"erworben_am"`
	GueltigBis      *string   `json:"gueltig_bis"`
	Status          string    `json:"status"`
	StornoReason    string    `json:"storno_reason"`
	CreatedByUserID *string   `json:"created_by_user_id"`
}

func decodeQualRecords(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) []qualRecordResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out []qualRecordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeQualRecord(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) qualRecordResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out qualRecordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

// day liefert den Kalendertag aus einem JSON-Zeitstempel ("2027-03-15T00:00:00Z").
func jsonDay(s *string) string {
	if s == nil {
		return ""
	}
	return strings.SplitN(*s, "T", 2)[0]
}

type requirementResponse struct {
	TypeID    uuid.UUID `json:"type_id"`
	Code      string    `json:"code"`
	TypeAktiv bool      `json:"type_aktiv"`
}

func decodeRequirements(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("Anforderungen: erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []requirementResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Anforderungen nicht lesbar: %v", err)
	}
	codes := make([]string, 0, len(out))
	for _, r := range out {
		codes = append(codes, r.Code)
	}
	return codes
}

const qualPath = "/api/v1/qualifications/"

func qualBody(typeID uuid.UUID, erworben string, employees ...uuid.UUID) string {
	ids := make([]string, 0, len(employees))
	for _, e := range employees {
		ids = append(ids, `"`+e.String()+`"`)
	}
	return `{"employee_ids":[` + strings.Join(ids, ",") + `],"type_id":"` + typeID.String() + `","erworben_am":"` + erworben + `","aussteller":"Meister"}`
}

func TestQualificationsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}
	a, b := tenant.employee("FlowA"), tenant.employee("FlowB")
	annual := tenant.qualType(hrToken, "ARBEITSSCHUTZ", "12")

	// Gruppenerfassung: je Mitarbeiter ein Nachweis, Gueltigkeit berechnet.
	created := decodeQualRecords(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "2026-03-15", a, b)), http.StatusCreated)
	if len(created) != 2 {
		t.Fatalf("erwartet 2 Nachweise, bekommen %d", len(created))
	}
	for _, q := range created {
		if jsonDay(q.GueltigBis) != "2027-03-15" || q.Status != "active" || q.CreatedByUserID == nil {
			t.Fatalf("Nachweis nicht wie erwartet: %+v", q)
		}
	}
	// Ausdrueckliches Gueltig-bis.
	explicit := decodeQualRecords(t, do(http.MethodPost, qualPath,
		`{"employee_ids":["`+a.String()+`"],"type_id":"`+annual.ID.String()+`","erworben_am":"2026-01-10","gueltig_bis":"2026-07-10"}`), http.StatusCreated)
	if jsonDay(explicit[0].GueltigBis) != "2026-07-10" {
		t.Fatalf("angegebenes Gueltig-bis nicht uebernommen: %+v", explicit[0])
	}

	var recA qualRecordResponse
	for _, q := range created {
		if q.EmployeeID == a {
			recA = q
		}
	}
	got := decodeQualRecord(t, do(http.MethodGet, qualPath+recA.ID.String(), ""), http.StatusOK)
	if got.ID != recA.ID || jsonDay(&got.ErworbenAm) != "2026-03-15" {
		t.Fatalf("Einzelabruf liefert nicht den erfassten Nachweis: %+v", got)
	}

	// Liste je Mitarbeiter, juengster zuerst; Storno; mit und ohne stornierte.
	if list := decodeQualRecords(t, do(http.MethodGet, qualPath+"?employee_id="+a.String(), ""), http.StatusOK); len(list) != 2 || list[0].ID != recA.ID {
		t.Fatalf("Liste fuer A: erwartet 2, juengster zuerst; bekommen %+v", list)
	}
	storniert := decodeQualRecord(t, do(http.MethodPost, qualPath+recA.ID.String()+"/storno", `{"reason":"Falscher Tag"}`), http.StatusOK)
	if storniert.Status != "storniert" || storniert.StornoReason != "Falscher Tag" {
		t.Fatalf("Storno nicht wie erwartet: %+v", storniert)
	}
	if list := decodeQualRecords(t, do(http.MethodGet, qualPath+"?employee_id="+a.String(), ""), http.StatusOK); len(list) != 1 {
		t.Fatalf("Standardliste nach Storno: erwartet 1, bekommen %d", len(list))
	}
	if list := decodeQualRecords(t, do(http.MethodGet, qualPath+"?employee_id="+a.String()+"&include_storniert=true", ""), http.StatusOK); len(list) != 2 {
		t.Fatalf("Liste mit stornierten: erwartet 2, bekommen %d", len(list))
	}

	// Fehlerfaelle.
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "2026-03-15", b)), http.StatusConflict, "conflict", "Nachweis für diesen Tag ist bereits erfasst")
	wantAPIError(t, do(http.MethodPost, qualPath+recA.ID.String()+"/storno", `{"reason":"Nochmal"}`), http.StatusConflict, "conflict", "Nachweis ist bereits storniert")
	wantAPIError(t, do(http.MethodPost, qualPath+created[1].ID.String()+"/storno", `{"reason":"  "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "2099-01-01", a)), http.StatusBadRequest, "validation_error", "Erwerbsdatum darf nicht in der Zukunft liegen")
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "15.03.2026", a)), http.StatusBadRequest, "validation_error", "Ungültiges Datum in erworben_am (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, qualPath, `{"employee_ids":["`+a.String()+`"],"type_id":"`+annual.ID.String()+`"}`), http.StatusBadRequest, "validation_error", "Erwerbsdatum erforderlich")
	wantAPIError(t, do(http.MethodPost, qualPath, `{"employee_ids":[],"type_id":"`+annual.ID.String()+`","erworben_am":"2026-03-15"}`), http.StatusBadRequest, "validation_error", "Mindestens ein Mitarbeiter erforderlich")
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(uuid.New(), "2026-03-15", a)), http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "2026-04-15", a, uuid.New())), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodGet, qualPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Nachweis nicht gefunden")
	wantAPIError(t, do(http.MethodGet, qualPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Nachweis-ID")
	wantAPIError(t, do(http.MethodGet, qualPath+"?employee_id=xyz", ""), http.StatusBadRequest, "validation_error", "Ungültige employee_id")

	// Stillgelegte Art: keine neuen Nachweise.
	decodeQualificationType(t, do(http.MethodPut, qtPath+annual.ID.String(),
		`{"name":"Art ARBEITSSCHUTZ","kategorie":"unterweisung","gueltigkeit_monate":12,"bestaetigung_monate":null,"pflicht_fuer_alle":false,"aktiv":false,"hinweis":""}`), http.StatusOK)
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "2026-05-15", a)), http.StatusConflict, "conflict", "Qualifikationsart ist stillgelegt")
}

func TestQualificationRequirementsHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}
	emp := tenant.employee("Anforderung")
	stapler := tenant.qualType(hrToken, "STAPLER_SCHEIN", "null")
	ersthelfer := tenant.qualType(hrToken, "ERSTHELFER", "24")
	reqPath := qualPath + "requirements?employee_id=" + emp.String()

	if codes := decodeRequirements(t, do(http.MethodGet, reqPath, "")); len(codes) != 0 {
		t.Fatalf("erwartet keine Anforderungen, bekommen %v", codes)
	}
	codes := decodeRequirements(t, do(http.MethodPut, reqPath, `{"type_ids":["`+stapler.ID.String()+`","`+ersthelfer.ID.String()+`"]}`))
	if len(codes) != 2 {
		t.Fatalf("erwartet 2 Anforderungen, bekommen %v", codes)
	}
	if codes := decodeRequirements(t, do(http.MethodGet, reqPath, "")); len(codes) != 2 {
		t.Fatalf("Anforderungen nicht gespeichert: %v", codes)
	}

	// "requirements" ist eine eigene Route, nicht die ID eines Nachweises.
	// Eine fehlende Liste ist KEINE leere Liste - sie wuerde sonst alles
	// entfernen.
	wantAPIError(t, do(http.MethodPut, reqPath, `{}`), http.StatusBadRequest, "validation_error", "Feld erforderlich: type_ids")
	if codes := decodeRequirements(t, do(http.MethodGet, reqPath, "")); len(codes) != 2 {
		t.Fatalf("PUT ohne type_ids hat die Anforderungen veraendert: %v", codes)
	}
	wantAPIError(t, do(http.MethodPut, qualPath+"requirements", `{"type_ids":[]}`), http.StatusBadRequest, "validation_error", "employee_id erforderlich")
	wantAPIError(t, do(http.MethodPut, reqPath, `{"type_ids":["`+uuid.NewString()+`"]}`), http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")
	wantAPIError(t, do(http.MethodPut, qualPath+"requirements?employee_id="+uuid.NewString(), `{"type_ids":[]}`), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	still := tenant.qualType(hrToken, "STILL", "12")
	decodeQualificationType(t, do(http.MethodPut, qtPath+still.ID.String(),
		`{"name":"Art STILL","kategorie":"unterweisung","gueltigkeit_monate":12,"bestaetigung_monate":null,"pflicht_fuer_alle":false,"aktiv":false,"hinweis":""}`), http.StatusOK)
	wantAPIError(t, do(http.MethodPut, reqPath, `{"type_ids":["`+still.ID.String()+`"]}`), http.StatusConflict, "conflict", "Qualifikationsart ist stillgelegt")

	// Eine ausdrücklich leere Liste entfernt alle.
	if codes := decodeRequirements(t, do(http.MethodPut, reqPath, `{"type_ids":[]}`)); len(codes) != 0 {
		t.Fatalf("leere Liste muss alle Anforderungen entfernen, bekommen %v", codes)
	}
}

// Kein Selbsteintrag; eigene Daten mit self, fremde mit read; schreiben nur
// mit write. Geprueft mit Rollen, die genau eines der Rechte haben.
func TestQualificationsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-qual-readonly", "qualifications.read")
	seedRoleWithPermissions(t, env, "itest-qual-writeonly", "qualifications.write")
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	worker := tenant.linkedUser("production", "Werker")
	colleague := tenant.linkedUser("production", "Kollege")
	reader := tenant.user("itest-qual-readonly")
	writer := tenant.user("itest-qual-writeonly")
	annual := tenant.qualType(hrToken, "ARBEITSSCHUTZ", "12")
	stapler := tenant.qualType(hrToken, "STAPLER_SCHEIN", "null")

	own := decodeQualRecords(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath, qualBody(annual.ID, "2026-03-15", worker.EmployeeID)), http.StatusCreated)[0]
	theirs := decodeQualRecords(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath, qualBody(annual.ID, "2026-03-15", colleague.EmployeeID)), http.StatusCreated)[0]
	decodeRequirements(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, qualPath+"requirements?employee_id="+worker.EmployeeID.String(), `{"type_ids":["`+stapler.ID.String()+`"]}`))
	reqOf := func(emp uuid.UUID) string { return qualPath + "requirements?employee_id=" + emp.String() }
	forbidden := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}

	t.Run("self: eigene lesen ja, fremde nein, schreiben nein", func(t *testing.T) {
		as := func(method, path, body string) *httptest.ResponseRecorder {
			return doAuthedRequest(t, handler, worker.Token, method, path, body)
		}
		// Ohne employee_id: die eigenen.
		if list := decodeQualRecords(t, as(http.MethodGet, qualPath, ""), http.StatusOK); len(list) != 1 || list[0].ID != own.ID {
			t.Fatalf("eigene Liste: erwartet den eigenen Nachweis, bekommen %+v", list)
		}
		decodeQualRecord(t, as(http.MethodGet, qualPath+own.ID.String(), ""), http.StatusOK)
		if codes := decodeRequirements(t, as(http.MethodGet, qualPath+"requirements", "")); len(codes) != 1 || codes[0] != "STAPLER_SCHEIN" {
			t.Fatalf("eigene Anforderungen: erwartet STAPLER_SCHEIN, bekommen %v", codes)
		}
		forbidden(t, as(http.MethodGet, qualPath+theirs.ID.String(), ""))
		forbidden(t, as(http.MethodGet, qualPath+"?employee_id="+colleague.EmployeeID.String(), ""))
		forbidden(t, as(http.MethodGet, reqOf(colleague.EmployeeID), ""))
		// Kein Selbsteintrag - auch nicht fuer den eigenen Mitarbeiter.
		forbidden(t, as(http.MethodPost, qualPath, qualBody(stapler.ID, "2026-02-01", worker.EmployeeID)))
		forbidden(t, as(http.MethodPost, qualPath+own.ID.String()+"/storno", `{"reason":"Selbst"}`))
		forbidden(t, as(http.MethodPut, reqOf(worker.EmployeeID), `{"type_ids":[]}`))
	})

	t.Run("nur read: fremde lesen ja, schreiben nein", func(t *testing.T) {
		as := func(method, path, body string) *httptest.ResponseRecorder {
			return doAuthedRequest(t, handler, reader, method, path, body)
		}
		decodeQualRecord(t, as(http.MethodGet, qualPath+theirs.ID.String(), ""), http.StatusOK)
		if list := decodeQualRecords(t, as(http.MethodGet, qualPath+"?employee_id="+worker.EmployeeID.String(), ""), http.StatusOK); len(list) != 1 {
			t.Fatalf("Leserolle: erwartet den Nachweis des Mitarbeiters, bekommen %d", len(list))
		}
		decodeRequirements(t, as(http.MethodGet, reqOf(worker.EmployeeID), ""))
		forbidden(t, as(http.MethodPost, qualPath, qualBody(stapler.ID, "2026-02-01", worker.EmployeeID)))
		forbidden(t, as(http.MethodPost, qualPath+own.ID.String()+"/storno", `{"reason":"Nur Leserecht"}`))
		forbidden(t, as(http.MethodPut, reqOf(worker.EmployeeID), `{"type_ids":[]}`))
	})

	t.Run("nur write: schreiben ja, fremde lesen nein", func(t *testing.T) {
		as := func(method, path, body string) *httptest.ResponseRecorder {
			return doAuthedRequest(t, handler, writer, method, path, body)
		}
		created := decodeQualRecords(t, as(http.MethodPost, qualPath, qualBody(stapler.ID, "2026-02-01", colleague.EmployeeID)), http.StatusCreated)
		decodeQualRecord(t, as(http.MethodPost, qualPath+created[0].ID.String()+"/storno", `{"reason":"Doppelt"}`), http.StatusOK)
		decodeRequirements(t, as(http.MethodPut, reqOf(colleague.EmployeeID), `{"type_ids":["`+annual.ID.String()+`"]}`))
		forbidden(t, as(http.MethodGet, qualPath+theirs.ID.String(), ""))
		forbidden(t, as(http.MethodGet, qualPath+"?employee_id="+colleague.EmployeeID.String(), ""))
		forbidden(t, as(http.MethodGet, reqOf(colleague.EmployeeID), ""))
	})

	// Nichts davon hat den Nachweis des Mitarbeiters veraendert.
	if got := decodeQualRecord(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, qualPath+own.ID.String(), ""), http.StatusOK); got.Status != "active" {
		t.Fatalf("abgelehnte Zugriffe haben den Nachweis veraendert: %+v", got)
	}
	if codes := decodeRequirements(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, reqOf(worker.EmployeeID), "")); len(codes) != 1 {
		t.Fatalf("abgelehnte Zugriffe haben die Anforderungen veraendert: %v", codes)
	}
}

func TestQualificationsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("hr"), two.user("admin")
	emp := one.employee("Isoliert")
	annual := one.qualType(tokenOne, "ARBEITSSCHUTZ", "12")
	rec := decodeQualRecords(t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, qualPath, qualBody(annual.ID, "2026-03-15", emp)), http.StatusCreated)[0]
	foreignType := two.qualType(tokenTwo, "ARBEITSSCHUTZ", "12")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, tokenTwo, method, path, body)
	}

	wantAPIError(t, do(http.MethodGet, qualPath+rec.ID.String(), ""), http.StatusNotFound, "not_found", "Nachweis nicht gefunden")
	wantAPIError(t, do(http.MethodPost, qualPath+rec.ID.String()+"/storno", `{"reason":"Fremd"}`), http.StatusNotFound, "not_found", "Nachweis nicht gefunden")
	wantAPIError(t, do(http.MethodGet, qualPath+"?employee_id="+emp.String(), ""), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodGet, qualPath+"requirements?employee_id="+emp.String(), ""), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPut, qualPath+"requirements?employee_id="+emp.String(), `{"type_ids":[]}`), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	// Mit der eigenen Art, aber fuer einen fremden Mitarbeiter.
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(foreignType.ID, "2026-04-15", emp)), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	// Mit der fremden Art.
	wantAPIError(t, do(http.MethodPost, qualPath, qualBody(annual.ID, "2026-04-15", emp)), http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")

	if got := decodeQualRecord(t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, qualPath+rec.ID.String(), ""), http.StatusOK); got.Status != "active" {
		t.Fatalf("Zugriff aus fremdem Mandanten hat den Nachweis veraendert: %+v", got)
	}
	if list := decodeQualRecords(t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, qualPath+"?employee_id="+emp.String()+"&include_storniert=true", ""), http.StatusOK); len(list) != 1 {
		t.Fatalf("Zugriff aus fremdem Mandanten hat Nachweise hinzugefuegt: %d", len(list))
	}
}

// Jeder Fehlertext der Nachweise und Anforderungen muss als 4xx
// klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversQualificationMessages(t *testing.T) {
	cases := map[string]int{
		"Mindestens ein Mitarbeiter erforderlich":           http.StatusBadRequest,
		"type_id erforderlich":                              http.StatusBadRequest,
		"Nachweis-ID erforderlich":                          http.StatusBadRequest,
		"Erwerbsdatum erforderlich":                         http.StatusBadRequest,
		"Erwerbsdatum darf nicht in der Zukunft liegen":     http.StatusBadRequest,
		"Gültig-bis darf nicht vor dem Erwerbsdatum liegen": http.StatusBadRequest,
		"Storno-Begründung erforderlich":                    http.StatusBadRequest,
		"Nachweis nicht gefunden":                           http.StatusNotFound,
		"Nachweis für diesen Tag ist bereits erfasst":       http.StatusConflict,
		"Nachweis ist bereits storniert":                    http.StatusConflict,
		"Qualifikationsart ist stillgelegt":                 http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
