package apihttp

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Krankmeldungen ueber HTTP (Backlog F.8.4.2).

type absenceResponse struct {
	ID           uuid.UUID `json:"id"`
	EmployeeID   uuid.UUID `json:"employee_id"`
	Typ          string    `json:"typ"`
	Status       string    `json:"status"`
	Attest       bool      `json:"attest"`
	GemeldetVon  string    `json:"gemeldet_von"`
	BestaetigtAt *string   `json:"bestaetigt_at"`
	StornoReason string    `json:"storno_reason"`
}

func TestSickReportsHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "krank-self", "leave_requests.self")
	seedRoleWithPermissions(t, env, "krank-read", "leave_requests.read")
	worker := tenant.linkedUser("krank-self", "Krank")
	colleague := tenant.linkedUser("krank-self", "Gesund")
	reader := tenant.user("krank-read")
	hrToken := tenant.user("hr")
	const path = "/api/v1/absences/"

	// Selbstmeldung: Attest und Zusatzfelder werden nicht uebernommen.
	var self absenceResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, path+"sick",
		`{"start_date":"2026-03-09","end_date":"2026-03-10","attest":true,"diagnose":"Grippe"}`), http.StatusCreated, &self)
	if self.EmployeeID != worker.EmployeeID || self.Typ != "sick" || self.GemeldetVon != "selbst" || self.Attest || self.BestaetigtAt != nil {
		t.Fatalf("Selbstmeldung: %+v", self)
	}
	var stored int
	if err := env.PG.QueryRow(t.Context(), `SELECT count(*) FROM hr_absences WHERE id=$1 AND row_to_json(hr_absences)::text LIKE '%Grippe%'`, self.ID).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("Zusatzangabe gespeichert: %d, %v", stored, err)
	}
	// Nicht fuer Kollegen; fremde lesen nur mit read.
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, path+"sick",
		`{"employee_id":"`+colleague.EmployeeID.String()+`","start_date":"2026-03-09","end_date":"2026-03-09"}`), http.StatusForbidden)
	wantStatus(t, doAuthedRequest(t, handler, colleague.Token, http.MethodGet, path+self.ID.String(), ""), http.StatusForbidden)
	var got absenceResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+self.ID.String(), ""), http.StatusOK, &got)
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+self.ID.String(), ""), http.StatusOK, &got)
	if got.ID != self.ID {
		t.Fatalf("lesen: %+v", got)
	}
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, path+"sick", `{"start_date":"09.03.2026","end_date":"2026-03-10"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges Datum in start_date (erwartet JJJJ-MM-TT)")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, path+"sick", `{"start_date":"2026-03-10","end_date":"2026-03-10"}`),
		http.StatusConflict, "conflict", "Zeitraum überschneidet sich mit einer bestehenden Krankmeldung")

	// Liste "neu": nur mit read.
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+"sick/new", ""), http.StatusForbidden)
	var list []absenceResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"sick/new", ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != self.ID {
		t.Fatalf("neue Krankmeldungen: %+v", list)
	}

	// Pflegen nur mit decide.
	for _, p := range []string{"/attest", "/confirm", "/storno"} {
		wantStatus(t, doAuthedRequest(t, handler, reader, http.MethodPost, path+self.ID.String()+p, `{"attest":true,"reason":"x"}`), http.StatusForbidden)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+self.ID.String()+"/attest", `{}`), http.StatusBadRequest, "validation_error", "attest erforderlich")
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+self.ID.String()+"/attest", `{"attest":true}`), http.StatusOK, &got)
	if !got.Attest {
		t.Fatalf("Attest: %+v", got)
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+self.ID.String()+"/confirm", ""), http.StatusOK, &got)
	if got.BestaetigtAt == nil {
		t.Fatalf("bestaetigen: %+v", got)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+self.ID.String()+"/confirm", ""), http.StatusConflict, "conflict", "Krankmeldung ist bereits bestätigt")

	// Personalabteilung meldet fuer einen Mitarbeiter: bestaetigt, mit Attest.
	var byHR absenceResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+"sick",
		`{"employee_id":"`+colleague.EmployeeID.String()+`","start_date":"2026-03-16","end_date":"2026-03-16","attest":true}`), http.StatusCreated, &byHR)
	if byHR.GemeldetVon != "personal" || !byHR.Attest || byHR.BestaetigtAt == nil {
		t.Fatalf("Meldung der Personalabteilung: %+v", byHR)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+byHR.ID.String()+"/storno", `{"reason":""}`), http.StatusBadRequest, "validation_error", "Begründung erforderlich")
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+byHR.ID.String()+"/storno", `{"reason":"falsch erfasst"}`), http.StatusOK, &got)
	if got.Status != "storniert" || got.StornoReason != "falsch erfasst" {
		t.Fatalf("Storno: %+v", got)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, path+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Abwesenheits-ID")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, path+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Abwesenheit nicht gefunden")
}

func TestAbsenceOverviewHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "liste-self", "leave_requests.self")
	seedRoleWithPermissions(t, env, "liste-read", "leave_requests.read")
	worker := tenant.linkedUser("liste-self", "Eigen")
	colleague := tenant.linkedUser("liste-self", "Kollege")
	reader := tenant.user("liste-read")
	hrToken := tenant.user("hr")
	const path = "/api/v1/absences/"
	for _, emp := range []string{worker.EmployeeID.String(), colleague.EmployeeID.String()} {
		wantStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path+"sick", `{"employee_id":"`+emp+`","start_date":"2026-06-10","end_date":"2026-06-11"}`), http.StatusCreated)
	}
	type entry struct {
		EmployeeID uuid.UUID `json:"employee_id"`
		LastName   string    `json:"last_name"`
		Typ        string    `json:"typ"`
	}
	june := "?from=2026-06-01&to=2026-06-30"

	// Ohne Leserecht nur die eigenen - auch ohne employee_id.
	var list []entry
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+june, ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].EmployeeID != worker.EmployeeID || list[0].LastName != "Eigen" {
		t.Fatalf("eigene: %+v", list)
	}
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+june+"&employee_id="+colleague.EmployeeID.String(), ""), http.StatusForbidden)
	// Mit Leserecht alle, optional gefiltert.
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+june, ""), http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("alle: %+v", list)
	}
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+june+"&employee_id="+colleague.EmployeeID.String(), ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].EmployeeID != colleague.EmployeeID {
		t.Fatalf("Kollege: %+v", list)
	}
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+june+"&team_id="+uuid.NewString(), ""), http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("unbekanntes Team: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+june+"&team_id=x", ""), http.StatusBadRequest, "validation_error", "Ungültige team_id")
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?from=01.06.2026&to=2026-06-30", ""), http.StatusBadRequest, "validation_error", "Ungültiges Datum in from (erwartet JJJJ-MM-TT)")
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?to=2026-06-30", ""), http.StatusBadRequest, "validation_error", "from/to erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?from=2026-01-01&to=2027-06-30", ""), http.StatusBadRequest, "validation_error", "Ungültiger Zeitraum: höchstens 366 Tage")
}
