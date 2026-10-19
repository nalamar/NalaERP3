package apihttp

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Teams ueber HTTP (Backlog F.8.5): pflegen mit employees.write.

func TestTeamsHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "team-read", "employees.read")
	reader := tenant.user("team-read")
	hrToken := tenant.user("hr")
	lead := tenant.employee("Leiter")
	const path = "/api/v1/employees/teams"

	type team struct {
		ID         uuid.UUID  `json:"id"`
		Name       string     `json:"name"`
		Lead       *uuid.UUID `json:"lead_employee_id"`
		Mitglieder int        `json:"mitglieder"`
	}
	wantStatus(t, doAuthedRequest(t, handler, reader, http.MethodPost, path, `{"name":"Montage"}`), http.StatusForbidden)
	var created team
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path, `{"name":"Montage","lead_employee_id":"`+lead.String()+`"}`), http.StatusCreated, &created)
	if created.Name != "Montage" || created.Lead == nil || *created.Lead != lead {
		t.Fatalf("anlegen: %+v", created)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path, `{"name":"montage"}`), http.StatusBadRequest, "validation_error", "Team mit diesem Namen existiert bereits")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path, `{"name":""}`), http.StatusBadRequest, "validation_error", "Teamname erforderlich")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, path, `{"name":"X","lead_employee_id":"`+uuid.NewString()+`"}`), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")

	wantStatus(t, doAuthedRequest(t, handler, reader, http.MethodPut, path+"/"+created.ID.String(), `{"name":"Y"}`), http.StatusForbidden)
	var upd team
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, path+"/"+created.ID.String(), `{"name":"Montage Süd","lead_employee_id":null}`), http.StatusOK, &upd)
	if upd.Name != "Montage Süd" || upd.Lead != nil {
		t.Fatalf("aendern: %+v", upd)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, path+"/x", `{"name":"Y"}`), http.StatusBadRequest, "validation_error", "Ungültige Team-ID")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, path+"/"+uuid.NewString(), `{"name":"Y"}`), http.StatusNotFound, "not_found", "Team nicht gefunden")

	// Mitglied zuordnen: Loeschen 409; Liste zeigt die Zahl.
	wantStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, "/api/v1/employees/"+lead.String(), `{"team_id":"`+created.ID.String()+`"}`), http.StatusOK)
	var list []team
	decodeJSONStatus(t, doAuthedRequest(t, handler, reader, http.MethodGet, path, ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].Mitglieder != 1 {
		t.Fatalf("Liste: %+v", list)
	}
	wantStatus(t, doAuthedRequest(t, handler, reader, http.MethodDelete, path+"/"+created.ID.String(), ""), http.StatusForbidden)
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodDelete, path+"/"+created.ID.String(), ""), http.StatusConflict, "conflict", "Team hat noch Mitglieder - zuerst umsetzen")
	wantStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodPatch, "/api/v1/employees/"+lead.String(), `{"team_id":null}`), http.StatusOK)
	wantStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodDelete, path+"/"+created.ID.String(), ""), http.StatusNoContent)
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodDelete, path+"/"+created.ID.String(), ""), http.StatusNotFound, "not_found", "Team nicht gefunden")
}
