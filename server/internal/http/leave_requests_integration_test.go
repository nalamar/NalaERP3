package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung der Urlaubsantraege (Backlog F.4.6). Helfer
// (seedTimeEntryUser, doAuthedRequest, wantAPIError) aus
// time_entries_integration_test.go.
//
// Jeder Test legt eigene Mitarbeiter an; Zeitraeume duerfen sich deshalb
// zwischen den Tests wiederholen.

type leaveResponse struct {
	ID              uuid.UUID  `json:"id"`
	EmployeeID      uuid.UUID  `json:"employee_id"`
	Typ             string     `json:"typ"`
	Status          string     `json:"status"`
	StartDate       time.Time  `json:"start_date"`
	EndDate         time.Time  `json:"end_date"`
	Days            float64    `json:"days"`
	Reason          string     `json:"reason"`
	ApproverID      *uuid.UUID `json:"approver_id"`
	DecidedByUserID *string    `json:"decided_by_user_id"`
	DecidedAt       *time.Time `json:"decided_at"`
}

func decodeLeave(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) leaveResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out leaveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeLeaveList(t *testing.T, rec *httptest.ResponseRecorder) map[uuid.UUID]leaveResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("Liste: erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var list []leaveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Liste nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	out := make(map[uuid.UUID]leaveResponse, len(list))
	for _, lr := range list {
		out[lr.ID] = lr
	}
	return out
}

// seedRoleWithPermissions legt eine Testrolle mit genau den genannten
// Rechten an. Die Systemrollen vergeben read und decide immer zusammen;
// eine Verwechslung der beiden faellt nur mit einer Rolle auf, die eines
// davon NICHT hat (Lehre aus F.4.4).
func seedRoleWithPermissions(t *testing.T, env *testutil.IntegrationEnv, code string, permissions ...string) {
	t.Helper()
	ctx := context.Background()
	roleID := "role-" + code
	if _, err := env.PG.Exec(ctx, `INSERT INTO roles (id, code, name, description, is_system) VALUES ($1,$2,$3,'',false) ON CONFLICT (code) DO NOTHING`, roleID, code, "Test: "+code); err != nil {
		t.Fatalf("Testrolle anlegen: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		t.Fatalf("Rechte der Testrolle leeren: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) SELECT $1, id FROM permissions WHERE code = ANY($2)`, roleID, permissions); err != nil {
		t.Fatalf("Rechte der Testrolle setzen: %v", err)
	}
}

const leavePath = "/api/v1/leave-requests/"

// Der Normalfall: ein Mitarbeiter stellt mit leave_requests.self einen
// Antrag, ohne employee_id, und sieht ihn in seiner Liste.
func TestLeaveRequestsSelfCreateAndRead(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "leave-self@example.com", "production", true)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, worker.Token, method, path, body)
	}

	// Mitgeschickte Entscheidungsangaben werden nicht gelesen: der Antrag
	// ist offen, egal was der Aufrufer behauptet.
	created := decodeLeave(t, do(http.MethodPost, leavePath,
		`{"start_date":"2027-03-01","end_date":"2027-03-05","reason":"Familienurlaub","status":"approved","approver_id":"`+worker.EmployeeID.String()+`","decided_by_user_id":"`+worker.UserID+`"}`), http.StatusCreated)
	if created.EmployeeID != worker.EmployeeID {
		t.Fatalf("erwartet Antrag fuer den eigenen Mitarbeiter %s, bekommen %s", worker.EmployeeID, created.EmployeeID)
	}
	if created.Status != "pending" || created.Typ != "vacation" || created.Days != 5 {
		t.Fatalf("erwartet offenen Urlaubsantrag ueber 5 Tage, bekommen %+v", created)
	}

	got := decodeLeave(t, do(http.MethodGet, leavePath+created.ID.String(), ""), http.StatusOK)
	if got.Status != "pending" || got.ApproverID != nil || got.DecidedByUserID != nil || got.DecidedAt != nil {
		t.Fatalf("gespeicherter Antrag traegt eine Entscheidung: %+v", got)
	}
	if got.Reason != "Familienurlaub" || got.StartDate.Format("2006-01-02") != "2027-03-01" || got.EndDate.Format("2006-01-02") != "2027-03-05" {
		t.Fatalf("Antrag nicht wie gestellt: %+v", got)
	}

	list := decodeLeaveList(t, do(http.MethodGet, leavePath, ""))
	if _, ok := list[created.ID]; !ok || len(list) != 1 {
		t.Fatalf("eigene Liste: erwartet genau den eigenen Antrag, bekommen %d Eintraege", len(list))
	}

	// Mit ausdruecklich genanntem EIGENEN Mitarbeiter geht es ebenfalls.
	decodeLeave(t, do(http.MethodPost, leavePath,
		`{"employee_id":"`+worker.EmployeeID.String()+`","start_date":"2027-04-01","end_date":"2027-04-01","typ":"unpaid","days":0.5}`), http.StatusCreated)

	// Entscheiden darf er nicht - auch nicht den eigenen Antrag.
	wantAPIError(t, do(http.MethodPost, leavePath+created.ID.String()+"/decide", `{"approve":true}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if still := decodeLeave(t, do(http.MethodGet, leavePath+created.ID.String(), ""), http.StatusOK); still.Status != "pending" {
		t.Fatalf("abgelehnter Entscheidungsversuch hat den Antrag entschieden: %+v", still)
	}
}

func TestLeaveRequestsSelfCannotTouchOtherEmployees(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "leave-self-a@example.com", "production", true)
	colleague := seedTimeEntryUser(t, env, handler, "leave-self-b@example.com", "production", true)
	unlinked := seedTimeEntryUser(t, env, handler, "leave-self-unlinked@example.com", "production", false)

	theirs := decodeLeave(t, doAuthedRequest(t, handler, colleague.Token, http.MethodPost, leavePath, `{"start_date":"2027-03-01","end_date":"2027-03-05"}`), http.StatusCreated)

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"Antrag fuer den Kollegen stellen": doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"employee_id":"`+colleague.EmployeeID.String()+`","start_date":"2027-06-01","end_date":"2027-06-02"}`),
		"Antrag des Kollegen lesen":        doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+theirs.ID.String(), ""),
		"Liste des Kollegen lesen":         doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+"?employee_id="+colleague.EmployeeID.String(), ""),
	} {
		t.Run(name, func(t *testing.T) {
			wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}
	// Die eigene Liste enthaelt den fremden Antrag nicht.
	if own := decodeLeaveList(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath, "")); len(own) != 0 {
		t.Fatalf("eigene Liste enthaelt fremde Antraege: %d", len(own))
	}
	// Fuer den Kollegen ist kein zweiter Antrag entstanden.
	var count int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM hr_leave_requests WHERE employee_id=$1`, colleague.EmployeeID).Scan(&count); err != nil {
		t.Fatalf("Antraege zaehlen: %v", err)
	}
	if count != 1 {
		t.Fatalf("erwartet genau einen Antrag des Kollegen, bekommen %d", count)
	}

	// Ohne verknuepften Mitarbeiter gibt es keine "eigenen" Antraege.
	wantAPIError(t, doAuthedRequest(t, handler, unlinked.Token, http.MethodPost, leavePath, `{"start_date":"2027-03-01","end_date":"2027-03-05"}`),
		http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")
	wantAPIError(t, doAuthedRequest(t, handler, unlinked.Token, http.MethodGet, leavePath, ""),
		http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")

	// Ohne Anmeldung: 401.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, leavePath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

// Die Personalabteilung sieht alle Antraege und entscheidet. Festgehalten
// wird der Login; approver_id nur, wenn der Entscheider einen
// Mitarbeiterdatensatz hat.
func TestLeaveRequestsHRListsAndDecides(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	hrLinked := seedTimeEntryUser(t, env, handler, "leave-hr-linked@example.com", "hr", true)
	hrUnlinked := seedTimeEntryUser(t, env, handler, "leave-hr-unlinked@example.com", "hr", false)
	a := seedTimeEntryUser(t, env, handler, "leave-hr-a@example.com", "production", true)
	b := seedTimeEntryUser(t, env, handler, "leave-hr-b@example.com", "production", true)

	first := decodeLeave(t, doAuthedRequest(t, handler, a.Token, http.MethodPost, leavePath, `{"start_date":"2027-03-01","end_date":"2027-03-05"}`), http.StatusCreated)
	second := decodeLeave(t, doAuthedRequest(t, handler, b.Token, http.MethodPost, leavePath, `{"start_date":"2027-03-08","end_date":"2027-03-09"}`), http.StatusCreated)

	// Ohne employee_id: alle Antraege des Mandanten.
	all := decodeLeaveList(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodGet, leavePath+"?limit=500", ""))
	if _, ok := all[first.ID]; !ok {
		t.Fatal("Gesamtliste enthaelt den ersten Antrag nicht")
	}
	if _, ok := all[second.ID]; !ok {
		t.Fatal("Gesamtliste enthaelt den zweiten Antrag nicht")
	}
	// Mit employee_id: nur dessen Antraege.
	onlyA := decodeLeaveList(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodGet, leavePath+"?employee_id="+a.EmployeeID.String(), ""))
	if _, ok := onlyA[first.ID]; !ok || len(onlyA) != 1 {
		t.Fatalf("Liste fuer A: erwartet genau dessen Antrag, bekommen %d Eintraege", len(onlyA))
	}
	decodeLeave(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodGet, leavePath+first.ID.String(), ""), http.StatusOK)

	// Entscheider MIT Mitarbeiterdatensatz: approver_id wird mitgefuehrt.
	approved := decodeLeave(t, doAuthedRequest(t, handler, hrLinked.Token, http.MethodPost, leavePath+first.ID.String()+"/decide", `{"approve":true}`), http.StatusOK)
	if approved.Status != "approved" || approved.DecidedAt == nil {
		t.Fatalf("erwartet genehmigten Antrag mit Zeitpunkt, bekommen %+v", approved)
	}
	if approved.DecidedByUserID == nil || *approved.DecidedByUserID != hrLinked.UserID {
		t.Fatalf("erwartet decided_by_user_id=%s, bekommen %v", hrLinked.UserID, approved.DecidedByUserID)
	}
	if approved.ApproverID == nil || *approved.ApproverID != hrLinked.EmployeeID {
		t.Fatalf("erwartet approver_id=%s, bekommen %v", hrLinked.EmployeeID, approved.ApproverID)
	}

	// Entscheider OHNE Mitarbeiterdatensatz: geht, approver_id bleibt leer.
	rejected := decodeLeave(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodPost, leavePath+second.ID.String()+"/decide", `{"approve":false}`), http.StatusOK)
	if rejected.Status != "rejected" || rejected.ApproverID != nil {
		t.Fatalf("erwartet abgelehnten Antrag ohne approver_id, bekommen %+v", rejected)
	}
	if rejected.DecidedByUserID == nil || *rejected.DecidedByUserID != hrUnlinked.UserID {
		t.Fatalf("erwartet decided_by_user_id=%s, bekommen %v", hrUnlinked.UserID, rejected.DecidedByUserID)
	}

	// Einmalig: ein zweiter Versuch - auch durch einen anderen Entscheider
	// und in die andere Richtung - aendert nichts.
	wantAPIError(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodPost, leavePath+first.ID.String()+"/decide", `{"approve":false}`),
		http.StatusConflict, "conflict", "Urlaubsantrag ist bereits entschieden")
	still := decodeLeave(t, doAuthedRequest(t, handler, a.Token, http.MethodGet, leavePath+first.ID.String(), ""), http.StatusOK)
	if still.Status != "approved" || still.DecidedByUserID == nil || *still.DecidedByUserID != hrLinked.UserID {
		t.Fatalf("zweiter Entscheidungsversuch hat die erste Entscheidung veraendert: %+v", still)
	}

	// Ein vergessenes approve ist keine Ablehnung.
	third := decodeLeave(t, doAuthedRequest(t, handler, a.Token, http.MethodPost, leavePath, `{"start_date":"2027-05-03","end_date":"2027-05-04"}`), http.StatusCreated)
	wantAPIError(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodPost, leavePath+third.ID.String()+"/decide", `{}`),
		http.StatusBadRequest, "validation_error", "approve erforderlich")
	if got := decodeLeave(t, doAuthedRequest(t, handler, a.Token, http.MethodGet, leavePath+third.ID.String(), ""), http.StatusOK); got.Status != "pending" {
		t.Fatalf("Aufruf ohne approve hat den Antrag entschieden: %+v", got)
	}

	// Die Personalabteilung stellt einen Antrag FUER einen Mitarbeiter
	// (leave_requests.decide), z. B. eine telefonische Krankmeldung.
	onBehalf := decodeLeave(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodPost, leavePath,
		`{"employee_id":"`+b.EmployeeID.String()+`","typ":"sick","start_date":"2027-06-01","end_date":"2027-06-02"}`), http.StatusCreated)
	if onBehalf.EmployeeID != b.EmployeeID || onBehalf.Typ != "sick" || onBehalf.Status != "pending" {
		t.Fatalf("Antrag im Auftrag nicht wie gestellt: %+v", onBehalf)
	}
	// Ohne eigenen Mitarbeiter und ohne employee_id ist unklar, fuer wen.
	wantAPIError(t, doAuthedRequest(t, handler, hrUnlinked.Token, http.MethodPost, leavePath, `{"start_date":"2027-07-01","end_date":"2027-07-02"}`),
		http.StatusBadRequest, "validation_error", "employee_id erforderlich")
}

// Lesen und Entscheiden sind getrennte Rechte. Geprueft mit Rollen, die
// genau eines davon haben.
func TestLeaveRequestsReadAndDecideAreSeparatePermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-leave-readonly", "leave_requests.read")
	seedRoleWithPermissions(t, env, "itest-leave-decideonly", "leave_requests.decide")
	reader := seedTimeEntryUser(t, env, handler, "leave-ro@example.com", "itest-leave-readonly", true)
	decider := seedTimeEntryUser(t, env, handler, "leave-do@example.com", "itest-leave-decideonly", false)
	worker := seedTimeEntryUser(t, env, handler, "leave-ro-worker@example.com", "production", true)
	lr := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"start_date":"2027-03-01","end_date":"2027-03-05"}`), http.StatusCreated)
	body := `{"employee_id":"` + worker.EmployeeID.String() + `","start_date":"2027-08-02","end_date":"2027-08-03"}`

	// Nur read: fremde Antraege lesen ja; entscheiden und fuer andere
	// stellen nein. Und ohne self auch keinen eigenen Antrag.
	decodeLeave(t, doAuthedRequest(t, handler, reader.Token, http.MethodGet, leavePath+lr.ID.String(), ""), http.StatusOK)
	if list := decodeLeaveList(t, doAuthedRequest(t, handler, reader.Token, http.MethodGet, leavePath+"?employee_id="+worker.EmployeeID.String(), "")); len(list) != 1 {
		t.Fatalf("Leserolle: erwartet den Antrag des Mitarbeiters, bekommen %d Eintraege", len(list))
	}
	wantAPIError(t, doAuthedRequest(t, handler, reader.Token, http.MethodPost, leavePath+lr.ID.String()+"/decide", `{"approve":true}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, reader.Token, http.MethodPost, leavePath, body),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, reader.Token, http.MethodPost, leavePath, `{"start_date":"2027-08-02","end_date":"2027-08-03"}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")

	// Nur decide: entscheiden und fuer andere stellen ja; fremde Antraege
	// LESEN nein.
	wantAPIError(t, doAuthedRequest(t, handler, decider.Token, http.MethodGet, leavePath+lr.ID.String(), ""),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, decider.Token, http.MethodGet, leavePath+"?employee_id="+worker.EmployeeID.String(), ""),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, decider.Token, http.MethodGet, leavePath, ""),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	decodeLeave(t, doAuthedRequest(t, handler, decider.Token, http.MethodPost, leavePath, body), http.StatusCreated)
	if got := decodeLeave(t, doAuthedRequest(t, handler, decider.Token, http.MethodPost, leavePath+lr.ID.String()+"/decide", `{"approve":true}`), http.StatusOK); got.Status != "approved" {
		t.Fatalf("erwartet genehmigten Antrag, bekommen %+v", got)
	}
}

func TestLeaveRequestsHTTPErrorMapping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "leave-errors@example.com", "production", true)
	hrUser := seedTimeEntryUser(t, env, handler, "leave-errors-hr@example.com", "hr", false)
	post := func(body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, body)
	}

	decodeLeave(t, post(`{"start_date":"2027-03-01","end_date":"2027-03-05"}`), http.StatusCreated)

	wantAPIError(t, post(`{"start_date":"2027-03-04","end_date":"2027-03-08"}`),
		http.StatusConflict, "conflict", "Zeitraum überschneidet sich mit einem bestehenden Urlaubsantrag")
	wantAPIError(t, post(`{"start_date":"2027-04-05","end_date":"2027-04-01"}`),
		http.StatusBadRequest, "validation_error", "Enddatum darf nicht vor Startdatum liegen")
	wantAPIError(t, post(`{"start_date":"2027-04-05"}`),
		http.StatusBadRequest, "validation_error", "start/end erforderlich")
	wantAPIError(t, post(`{"start_date":"05.04.2027","end_date":"2027-04-06"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges Datum in start_date (erwartet JJJJ-MM-TT)")
	wantAPIError(t, post(`{"start_date":"2027-04-05","end_date":"2027-04-06T00:00:00Z"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges Datum in end_date (erwartet JJJJ-MM-TT)")
	wantAPIError(t, post(`{"start_date":"2027-04-05","end_date":"2027-04-06","typ":"sabbatical"}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Antragstyp")
	wantAPIError(t, post(`{"start_date":"2027-04-05","end_date":"2027-04-06","days":3}`),
		http.StatusBadRequest, "validation_error", "Tage dürfen die Länge des Zeitraums nicht überschreiten")
	wantAPIError(t, post(`{"employee_id":"kein-uuid"}`),
		http.StatusBadRequest, "validation_error", "Ungültige Eingabe")

	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+uuid.NewString(), ""),
		http.StatusNotFound, "not_found", "Urlaubsantrag nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+"kein-uuid", ""),
		http.StatusBadRequest, "validation_error", "Ungültige Urlaubsantrag-ID")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+"?employee_id=xyz", ""),
		http.StatusBadRequest, "validation_error", "Ungültige employee_id")
	wantAPIError(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, leavePath+uuid.NewString()+"/decide", `{"approve":true}`),
		http.StatusNotFound, "not_found", "Urlaubsantrag nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodGet, leavePath+"?employee_id="+uuid.NewString(), ""),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
}

func TestLeaveRequestsAreIsolatedBetweenCompaniesOverHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "leave-iso-worker@example.com", "production", true)
	lr := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"start_date":"2027-03-01","end_date":"2027-03-05"}`), http.StatusCreated)

	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ('other-company','Andere Firma GmbH') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("zweiten Mandanten anlegen: %v", err)
	}
	foreignID := testutil.SeedAuthUser(t, env, "leave-iso-admin@other.example.com", "Secret123!", "admin")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET company_id='other-company' WHERE id=$1`, foreignID); err != nil {
		t.Fatalf("Benutzer dem zweiten Mandanten zuordnen: %v", err)
	}
	foreign := loginIntegrationUser(t, handler, "leave-iso-admin@other.example.com", "Secret123!")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, foreign, method, path, body)
	}

	wantAPIError(t, do(http.MethodGet, leavePath+lr.ID.String(), ""), http.StatusNotFound, "not_found", "Urlaubsantrag nicht gefunden")
	wantAPIError(t, do(http.MethodPost, leavePath+lr.ID.String()+"/decide", `{"approve":true}`), http.StatusNotFound, "not_found", "Urlaubsantrag nicht gefunden")
	wantAPIError(t, do(http.MethodGet, leavePath+"?employee_id="+worker.EmployeeID.String(), ""), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPost, leavePath, `{"employee_id":"`+worker.EmployeeID.String()+`","start_date":"2027-09-01","end_date":"2027-09-02"}`),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	if all := decodeLeaveList(t, do(http.MethodGet, leavePath+"?limit=500", "")); len(all) != 0 {
		if _, leaked := all[lr.ID]; leaked {
			t.Fatal("Gesamtliste des fremden Mandanten enthaelt den Antrag")
		}
	}

	still := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+lr.ID.String(), ""), http.StatusOK)
	if still.Status != "pending" {
		t.Fatalf("Zugriff aus fremdem Mandanten hat den Antrag entschieden: %+v", still)
	}
}

// Jeder Fehlertext der Urlaubsantraege muss als 4xx klassifiziert werden.
// DB-los.
func TestClassifyDomainErrorCoversLeaveRequestMessages(t *testing.T) {
	cases := map[string]int{
		"start/end erforderlich":                                          http.StatusBadRequest,
		"Urlaubsantrag-ID erforderlich":                                   http.StatusBadRequest,
		"Entscheider erforderlich":                                        http.StatusBadRequest,
		"Enddatum darf nicht vor Startdatum liegen":                       http.StatusBadRequest,
		"Ungültiger Antragstyp":                                           http.StatusBadRequest,
		"Tage dürfen die Länge des Zeitraums nicht überschreiten":         http.StatusBadRequest,
		"Urlaubsantrag nicht gefunden":                                    http.StatusNotFound,
		"Urlaubsantrag ist bereits entschieden":                           http.StatusConflict,
		"Zeitraum überschneidet sich mit einem bestehenden Urlaubsantrag": http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
