package apihttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Arbeitsgaenge am Fertigungsauftrag ueber HTTP (Backlog H.2.3b). Helfer
// aus production_orders_integration_test.go und
// production_master_data_integration_test.go.

type operationResponse struct {
	ID             uuid.UUID  `json:"id"`
	Pos            int        `json:"pos"`
	Name           string     `json:"name"`
	WorkCenterName string     `json:"work_center_name"`
	TemplateID     *uuid.UUID `json:"template_id"`
	GeplanteWoche  *string    `json:"geplante_woche"`
	PlanMinuten    float64    `json:"plan_minuten"`
	Hinweise       []string   `json:"hinweise"`
}

func operationsPath(orderID uuid.UUID) string {
	return productionOrdersPath + orderID.String() + "/operations"
}

func weekOf(x operationResponse) string {
	if x.GeplanteWoche == nil {
		return "-"
	}
	return (*x.GeplanteWoche)[:10]
}

func TestProductionOperationsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("production")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	wc := decodeJSON[workCenterResponse](t, do(http.MethodPost, workCentersPath, `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusCreated)
	decodeJSON[opTemplateResponse](t, do(http.MethodPost, opTemplatesPath,
		`{"name":"Sägen","work_center_id":"`+wc.ID.String()+`","reihenfolge":1,"ruestzeit_min":30,"minuten_je_stueck":10}`), http.StatusCreated)
	p := seedProductionPosition(t, env, c.company)
	order := decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath, p.body(4, `,"liefertermin":"2026-12-02"`)), http.StatusCreated)
	path := operationsPath(order.ID)

	sug := decodeJSON[[]operationResponse](t, do(http.MethodPost, path+"/vorschlagen", ""), http.StatusCreated)
	if len(sug) != 1 || sug[0].Name != "Sägen" || weekOf(sug[0]) != "2026-11-23" || sug[0].PlanMinuten != 70 || sug[0].TemplateID == nil || sug[0].WorkCenterName != "Zuschnitt" {
		t.Fatalf("Vorschlag: %+v", sug)
	}
	wantAPIError(t, do(http.MethodPost, path+"/vorschlagen", ""), http.StatusConflict, "conflict", "Auftrag hat bereits Arbeitsgänge")

	add := decodeJSON[operationResponse](t, do(http.MethodPost, path,
		`{"name":"Endkontrolle","work_center_id":"`+wc.ID.String()+`","minuten_je_stueck":2,"geplante_woche":"2026-12-09"}`), http.StatusCreated)
	if add.Pos != 2 || weekOf(add) != "2026-12-07" || add.PlanMinuten != 8 || strings.Join(add.Hinweise, ",") != "nach_liefertermin" {
		t.Fatalf("hinzugefügt: %+v", add)
	}
	wcID := wc.ID.String()
	for _, tc := range []struct {
		body, code, msg string
		status          int
	}{
		{`{"name":"x","work_center_id":"` + wcID + `","geplante_woche":"9.12.2026"}`, "validation_error", "Ungültiges Datum in geplante_woche (erwartet JJJJ-MM-TT)", http.StatusBadRequest},
		{`{"name":`, "validation_error", "Ungültige Eingabe", http.StatusBadRequest},
		{`{"name":" ","work_center_id":"` + wcID + `"}`, "validation_error", "Name des Arbeitsgangs erforderlich", http.StatusBadRequest},
		{`{"name":"x","work_center_id":"` + wcID + `","ruestzeit_min":-1}`, "validation_error", "Zeiten dürfen nicht negativ sein", http.StatusBadRequest},
		{`{"name":"x","work_center_id":"` + uuid.NewString() + `"}`, "not_found", "Arbeitsplatz nicht gefunden", http.StatusNotFound},
	} {
		wantAPIError(t, do(http.MethodPost, path, tc.body), tc.status, tc.code, tc.msg)
	}
	wantAPIError(t, do(http.MethodPost, productionOrdersPath+"kaputt/operations", `{"name":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
	wantAPIError(t, do(http.MethodPost, operationsPath(uuid.New()), `{"name":"x","work_center_id":"`+wcID+`"}`), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")

	ch := decodeJSON[operationResponse](t, do(http.MethodPut, path+"/"+add.ID.String(),
		`{"name":"Endkontrolle","work_center_id":"`+wcID+`","minuten_je_stueck":3}`), http.StatusOK)
	if weekOf(ch) != "-" || len(ch.Hinweise) != 0 || ch.PlanMinuten != 12 {
		t.Fatalf("geändert: %+v", ch)
	}
	wantAPIError(t, do(http.MethodPut, path+"/"+uuid.NewString(), `{"name":"x","work_center_id":"`+wcID+`"}`), http.StatusNotFound, "not_found", "Arbeitsgang nicht gefunden")
	wantAPIError(t, do(http.MethodPut, path+"/kaputt", `{"name":"x","work_center_id":"`+wcID+`"}`), http.StatusBadRequest, "validation_error", "Ungültige Arbeitsgang-ID")

	if list := decodeJSON[[]operationResponse](t, do(http.MethodGet, path, ""), http.StatusOK); len(list) != 2 || list[0].Pos != 1 || list[1].Name != "Endkontrolle" {
		t.Fatalf("Liste: %+v", list)
	}
	detail := decodeJSON[struct {
		Arbeitsgaenge []operationResponse `json:"arbeitsgaenge"`
	}](t, do(http.MethodGet, productionOrdersPath+order.ID.String(), ""), http.StatusOK)
	if len(detail.Arbeitsgaenge) != 2 {
		t.Fatalf("Einzelansicht ohne Arbeitsgänge: %+v", detail)
	}

	if rec := do(http.MethodDelete, path+"/"+add.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("entfernen: %d %s", rec.Code, rec.Body.String())
	}
	wantAPIError(t, do(http.MethodDelete, path+"/"+add.ID.String(), ""), http.StatusNotFound, "not_found", "Arbeitsgang nicht gefunden")

	// Ohne aktive Vorlagen kein Vorschlag.
	empty := newCatalogTenant(t, env, handler)
	emptyToken := empty.user("production")
	q := seedProductionPosition(t, env, empty.company)
	o2 := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, emptyToken, http.MethodPost, productionOrdersPath, q.body(1, "")), http.StatusCreated)
	wantAPIError(t, doAuthedRequest(t, handler, emptyToken, http.MethodPost, operationsPath(o2.ID)+"/vorschlagen", ""),
		http.StatusConflict, "conflict", "Keine aktiven Arbeitsgangvorlagen vorhanden")

	// Nach Storno gesperrt.
	decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath+order.ID.String()+"/storno", `{"reason":"Maß falsch"}`), http.StatusOK)
	const locked = "Auftrag ist abgeschlossen oder storniert, Arbeitsgänge nicht änderbar"
	wantAPIError(t, do(http.MethodPost, path, `{"name":"x","work_center_id":"`+wcID+`"}`), http.StatusConflict, "conflict", locked)
	wantAPIError(t, do(http.MethodPut, path+"/"+sug[0].ID.String(), `{"name":"x","work_center_id":"`+wcID+`"}`), http.StatusConflict, "conflict", locked)
	wantAPIError(t, do(http.MethodDelete, path+"/"+sug[0].ID.String(), ""), http.StatusConflict, "conflict", locked)
	if list := decodeJSON[[]operationResponse](t, do(http.MethodGet, path, ""), http.StatusOK); len(list) != 1 || list[0].Name != "Sägen" {
		t.Fatalf("gesperrter Auftrag verändert: %+v", list)
	}
}

func TestProductionOperationsHTTPPermissionsAndTenants(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pr-readonly", "production.read")
	seedRoleWithPermissions(t, env, "itest-pr-writeonly", "production.write")
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	wc := decodeJSON[workCenterResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, workCentersPath, `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusCreated)
	decodeJSON[opTemplateResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, opTemplatesPath,
		`{"name":"Sägen","work_center_id":"`+wc.ID.String()+`","reihenfolge":1}`), http.StatusCreated)
	p := seedProductionPosition(t, env, c.company)
	newOrder := func() uuid.UUID {
		return decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated).ID
	}
	opBody := `{"name":"x","work_center_id":"` + wc.ID.String() + `"}`
	countOps := func(order uuid.UUID) int {
		return len(decodeJSON[[]operationResponse](t, doAuthedRequest(t, handler, prod, http.MethodGet, operationsPath(order), ""), http.StatusOK))
	}

	for name, tc := range map[string]struct {
		role              string
		canRead, canWrite bool
	}{
		"nur read":   {"itest-pr-readonly", true, false},
		"nur write":  {"itest-pr-writeonly", false, true},
		"Vertrieb":   {"sales", true, false},
		"Personal":   {"hr", false, false},
		"Lager":      {"inventory", false, false},
		"Produktion": {"production", true, true},
		"Admin":      {"admin", true, true},
	} {
		t.Run(name, func(t *testing.T) {
			token := c.user(tc.role)
			order := newOrder()
			path := operationsPath(order)
			existing := decodeJSON[operationResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, path, opBody), http.StatusCreated)
			check := func(method, p, body string, allowed bool, ok int) {
				t.Helper()
				rec := doAuthedRequest(t, handler, token, method, p, body)
				if !allowed {
					wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				} else if rec.Code != ok {
					t.Fatalf("%s %s: %d %s", method, p, rec.Code, rec.Body.String())
				}
			}
			check(http.MethodGet, path, "", tc.canRead, http.StatusOK)
			check(http.MethodPost, path, opBody, tc.canWrite, http.StatusCreated)
			check(http.MethodPut, path+"/"+existing.ID.String(), `{"name":"y","work_center_id":"`+wc.ID.String()+`"}`, tc.canWrite, http.StatusOK)
			check(http.MethodDelete, path+"/"+existing.ID.String(), "", tc.canWrite, http.StatusNoContent)
			fresh := newOrder()
			check(http.MethodPost, operationsPath(fresh)+"/vorschlagen", "", tc.canWrite, http.StatusCreated)
			if !tc.canWrite {
				if n, m := countOps(order), countOps(fresh); n != 1 || m != 0 {
					t.Fatalf("verbotene Änderung hat gewirkt: %d, %d", n, m)
				}
			}
		})
	}

	// Anderer Mandant: Auftrag unbekannt, nichts veraendert.
	order := newOrder()
	path := operationsPath(order)
	op := decodeJSON[operationResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, path, opBody), http.StatusCreated)
	two := newCatalogTenant(t, env, handler).user("admin")
	const notFound = "Fertigungsauftrag nicht gefunden"
	for _, ch := range []struct{ method, path, body string }{
		{http.MethodGet, path, ""},
		{http.MethodPost, path, opBody},
		{http.MethodPost, path + "/vorschlagen", ""},
		{http.MethodPut, path + "/" + op.ID.String(), opBody},
		{http.MethodDelete, path + "/" + op.ID.String(), ""},
	} {
		wantAPIError(t, doAuthedRequest(t, handler, two, ch.method, ch.path, ch.body), http.StatusNotFound, "not_found", notFound)
	}
	if list := decodeJSON[[]operationResponse](t, doAuthedRequest(t, handler, prod, http.MethodGet, path, ""), http.StatusOK); len(list) != 1 || list[0].Name != "x" {
		t.Fatalf("anderer Mandant hat geändert: %+v", list)
	}
}

// Jeder Fehlertext der Arbeitsgaenge muss als 4xx klassifiziert werden.
func TestClassifyDomainErrorCoversProductionOperationMessages(t *testing.T) {
	for msg, want := range map[string]int{
		"Arbeitsgang nicht gefunden": http.StatusNotFound,
		"Auftrag ist abgeschlossen oder storniert, Arbeitsgänge nicht änderbar": http.StatusConflict,
		"Auftrag hat bereits Arbeitsgänge":                                      http.StatusConflict,
		"Keine aktiven Arbeitsgangvorlagen vorhanden":                           http.StatusConflict,
	} {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
