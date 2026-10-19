package apihttp

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Urlaubskonto ueber HTTP (Backlog F.8.3.1): eigenes Konto mit
// leave_requests.self, fremde mit leave_requests.read, festlegen nur mit
// leave_requests.decide.

type leaveAccountResponse struct {
	EmployeeID       uuid.UUID `json:"employee_id"`
	Jahr             int       `json:"jahr"`
	AnspruchTage     float64   `json:"anspruch_tage"`
	UebertragTage    float64   `json:"uebertrag_tage"`
	GenommenTage     float64   `json:"genommen_tage"`
	VerplantTage     float64   `json:"verplant_tage"`
	RestTage         float64   `json:"rest_tage"`
	VerfuegbarTage   float64   `json:"verfuegbar_tage"`
	HinweisErteiltAm *string   `json:"hinweis_erteilt_am"`
	Notiz            string    `json:"notiz"`
	Festgelegt       bool      `json:"festgelegt"`
}

func TestLeaveAccountsHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "konto-self", "leave_requests.self")
	seedRoleWithPermissions(t, env, "konto-read", "leave_requests.read")
	worker := tenant.linkedUser("konto-self", "Arbeiter")
	colleague := tenant.employee("Kollege")
	reader := tenant.user("konto-read")
	hrToken := tenant.user("hr")
	otherHR := other.user("hr")
	const path = "/api/v1/leave-accounts/"

	// Eigenes Konto ohne employee_id.
	var acc leaveAccountResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+"?year=2026", ""), http.StatusOK, &acc)
	if acc.EmployeeID != worker.EmployeeID || acc.Jahr != 2026 || acc.AnspruchTage != 20 || acc.RestTage != 20 || acc.Festgelegt {
		t.Fatalf("eigenes Konto: %+v", acc)
	}
	// Fremdes Konto nur mit leave_requests.read.
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+"?employee_id="+colleague.String(), ""), http.StatusForbidden)
	acc = leaveAccountResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?employee_id="+colleague.String()+"&year=2026", ""), http.StatusOK, &acc)
	if acc.EmployeeID != colleague || acc.AnspruchTage != 20 {
		t.Fatalf("fremdes Konto: %+v", acc)
	}
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?employee_id=kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige employee_id")
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?employee_id="+colleague.String()+"&year=x", ""), http.StatusBadRequest, "validation_error", "Ungültiges Jahr")
	// Ohne Jahr: das laufende.
	acc = leaveAccountResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path, ""), http.StatusOK, &acc)
	if acc.Jahr < 2026 {
		t.Fatalf("laufendes Jahr: %+v", acc)
	}

	// Festlegen nur mit leave_requests.decide.
	put := path + colleague.String() + "/2026"
	body := `{"anspruch_tage":15,"uebertrag_tage":2.5,"hinweis_erteilt_am":"2026-10-01","notiz":"Eintritt Juni"}`
	wantStatus(t, doAuthedRequest(t, handler, reader, http.MethodPut, put, body), http.StatusForbidden)
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodPut, put, body), http.StatusForbidden)
	acc = leaveAccountResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, put, body), http.StatusOK, &acc)
	if acc.AnspruchTage != 15 || acc.UebertragTage != 2.5 || acc.RestTage != 17.5 || !acc.Festgelegt ||
		acc.HinweisErteiltAm == nil || !strings.HasPrefix(*acc.HinweisErteiltAm, "2026-10-01") || acc.Notiz != "Eintritt Juni" {
		t.Fatalf("festgelegtes Konto: %+v", acc)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, put, `{"uebertrag_tage":1}`), http.StatusBadRequest, "validation_error", "anspruch_tage erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, put, `{"anspruch_tage":20,"hinweis_erteilt_am":"01.10.2026"}`),
		http.StatusBadRequest, "validation_error", "Ungültiges Datum in hinweis_erteilt_am (erwartet JJJJ-MM-TT)")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, put, `{"anspruch_tage":20.3}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Wert für anspruch_tage: ganze oder halbe Tage von 0 bis 366")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, path+"kein-uuid/2026", `{"anspruch_tage":20}`), http.StatusBadRequest, "validation_error", "Ungültige employee_id")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, path+colleague.String()+"/zwei", `{"anspruch_tage":20}`), http.StatusBadRequest, "validation_error", "Ungültiges Jahr")
	wantAPIError(t, doAuthedRequest(t, handler, otherHR, http.MethodPut, put, `{"anspruch_tage":99}`), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	acc = leaveAccountResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?employee_id="+colleague.String()+"&year=2026", ""), http.StatusOK, &acc)
	if acc.AnspruchTage != 15 || acc.UebertragTage != 2.5 {
		t.Fatalf("abgelehnte Aenderungen haben gespeichert: %+v", acc)
	}

	// Jahresanspruch am Mitarbeiter ueber PATCH: Untergrenze 400.
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, "/api/v1/employees/"+worker.EmployeeID.String(), `{"urlaubstage_jahr":19}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Urlaubsanspruch: mindestens 20 Tage bei 5 Arbeitstagen je Woche (gesetzlicher Mindesturlaub)")
	var emp struct {
		UrlaubstageJahr *float64 `json:"urlaubstage_jahr"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, "/api/v1/employees/"+worker.EmployeeID.String(), `{"urlaubstage_jahr":30}`), http.StatusOK, &emp)
	if emp.UrlaubstageJahr == nil || *emp.UrlaubstageJahr != 30 {
		t.Fatalf("Jahresanspruch: %+v", emp)
	}
	acc = leaveAccountResponse{}
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+"?year=2026", ""), http.StatusOK, &acc)
	if acc.AnspruchTage != 30 {
		t.Fatalf("Konto folgt dem Jahresanspruch nicht: %+v", acc)
	}
}

func TestLeaveAccountsRemainingHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "rest-self", "leave_requests.self")
	seedRoleWithPermissions(t, env, "rest-read", "leave_requests.read")
	worker := tenant.linkedUser("rest-self", "Arbeiter")
	reader := tenant.user("rest-read")
	const path = "/api/v1/leave-accounts/remaining"

	var list []struct {
		EmployeeID uuid.UUID            `json:"employee_id"`
		LastName   string               `json:"last_name"`
		Konto      leaveAccountResponse `json:"konto"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?year=2026", ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].EmployeeID != worker.EmployeeID || list[0].LastName != "Arbeiter" || list[0].Konto.RestTage != 20 {
		t.Fatalf("Resturlaub: %+v", list)
	}
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path+"?year=2026", ""), http.StatusForbidden)
	wantAPIError(t, doAuthedRequest(t, handler, reader, http.MethodGet, path+"?year=1800", ""), http.StatusBadRequest, "validation_error", "Ungültiges Jahr")
}
