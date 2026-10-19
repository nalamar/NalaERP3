package apihttp

import (
	"net/http"
	"testing"

	"nalaerp3/internal/testutil"
)

// Zurueckziehen und Storno ueber HTTP (Backlog F.8.4.1): zurueckziehen der
// Antragsteller oder wer entscheidet; stornieren und entscheiden nur, wer
// entscheiden darf - geprueft an der einen Stelle im Service.

func TestLeaveWithdrawStornoHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "urlaub-self", "leave_requests.self")
	worker := tenant.linkedUser("urlaub-self", "Antragsteller")
	colleague := tenant.linkedUser("urlaub-self", "Kollege")
	hrToken := tenant.user("hr")

	mine := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"start_date":"2026-06-08","end_date":"2026-06-12"}`), http.StatusCreated)
	// Kollege darf nicht zurueckziehen, entscheiden oder stornieren.
	wantStatus(t, doAuthedRequest(t, handler, colleague.Token, http.MethodPost, leavePath+mine.ID.String()+"/withdraw", ""), http.StatusForbidden)
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath+mine.ID.String()+"/decide", `{"approve":true}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	// Der Antragsteller zieht seinen offenen Antrag zurueck.
	got := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath+mine.ID.String()+"/withdraw", ""), http.StatusOK)
	if got.Status != "withdrawn" {
		t.Fatalf("zurueckziehen: %+v", got)
	}
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath+mine.ID.String()+"/withdraw", ""),
		http.StatusConflict, "conflict", "Urlaubsantrag ist bereits entschieden")

	// Neuer Antrag fuer denselben Zeitraum, genehmigt, dann storniert.
	again := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"start_date":"2026-06-08","end_date":"2026-06-12"}`), http.StatusCreated)
	if d := decodeLeave(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+again.ID.String()+"/decide", `{"approve":true}`), http.StatusOK); d.Status != "approved" {
		t.Fatalf("genehmigen: %+v", d)
	}
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath+again.ID.String()+"/storno", `{"reason":"will doch nicht"}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+again.ID.String()+"/storno", `{"reason":""}`),
		http.StatusBadRequest, "validation_error", "Begründung erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+again.ID.String()+"/storno", `[]`),
		http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	var st struct {
		Status       string `json:"status"`
		StornoReason string `json:"storno_reason"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+again.ID.String()+"/storno", `{"reason":"Auftrag vorgezogen"}`), http.StatusOK, &st)
	if st.Status != "storniert" || st.StornoReason != "Auftrag vorgezogen" {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+again.ID.String()+"/storno", `{"reason":"x"}`),
		http.StatusConflict, "conflict", "Urlaubsantrag ist bereits storniert")
	// Die Personalabteilung darf auch einen fremden offenen Antrag zurueckziehen.
	third := decodeLeave(t, doAuthedRequest(t, handler, colleague.Token, http.MethodPost, leavePath, `{"start_date":"2026-07-06","end_date":"2026-07-07"}`), http.StatusCreated)
	if w := decodeLeave(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+third.ID.String()+"/withdraw", ""), http.StatusOK); w.Status != "withdrawn" {
		t.Fatalf("Personalabteilung zieht zurueck: %+v", w)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+"kein-uuid/storno", `{"reason":"x"}`),
		http.StatusBadRequest, "validation_error", "Ungültige Urlaubsantrag-ID")
}

// Statusfilter der Antragsliste (F.8.6.2.1): die Personalabteilung holt
// alle offenen Antraege.
func TestLeaveRequestsStatusFilterHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "filter-self", "leave_requests.self")
	worker := tenant.linkedUser("filter-self", "Filter")
	hrToken := tenant.user("hr")
	open := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"start_date":"2026-06-08","end_date":"2026-06-09"}`), http.StatusCreated)
	decided := decodeLeave(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, leavePath, `{"start_date":"2026-07-06","end_date":"2026-07-07"}`), http.StatusCreated)
	decodeLeave(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, leavePath+decided.ID.String()+"/decide", `{"approve":true}`), http.StatusOK)

	pending := decodeLeaveList(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, leavePath+"?status=pending", ""))
	if _, ok := pending[open.ID]; !ok || len(pending) != 1 {
		t.Fatalf("offene: %+v", pending)
	}
	approved := decodeLeaveList(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, leavePath+"?status=approved", ""))
	if _, ok := approved[decided.ID]; !ok || len(approved) != 1 {
		t.Fatalf("genehmigte (eigene): %+v", approved)
	}
	if all := decodeLeaveList(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, leavePath, "")); len(all) != 2 {
		t.Fatalf("ohne Filter: %+v", all)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, leavePath+"?status=offen", ""), http.StatusBadRequest, "validation_error", "Ungültiger Status")
}
