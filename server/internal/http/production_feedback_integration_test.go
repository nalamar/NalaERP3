package apihttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Rueckmeldung je Arbeitsgang ueber HTTP (Backlog H.3.5d). Helfer aus
// production_orders_integration_test.go,
// production_operations_integration_test.go und
// production_master_data_integration_test.go.

type feedbackOperationResponse struct {
	ID                uuid.UUID `json:"id"`
	Erledigt          bool      `json:"erledigt"`
	ErledigtAt        *string   `json:"erledigt_at"`
	IstMinuten        *float64  `json:"ist_minuten"`
	RueckmeldungNotiz string    `json:"rueckmeldung_notiz"`
}

type sollIstResponse struct {
	Arbeitsgaenge   int     `json:"arbeitsgaenge"`
	Erledigt        int     `json:"erledigt"`
	SollMin         float64 `json:"soll_min"`
	SollErledigtMin float64 `json:"soll_erledigt_min"`
	MitIst          int     `json:"mit_ist"`
	SollMitIstMin   float64 `json:"soll_mit_ist_min"`
	IstMin          float64 `json:"ist_min"`
}

// seedFeedbackOrder legt einen Auftrag ueber 4 Stueck mit zwei
// Arbeitsgaengen (je 30 + 10 x 4 = 70 Minuten) an.
func seedFeedbackOrder(t *testing.T, env *testutil.IntegrationEnv, handler http.Handler, token, company string) (order uuid.UUID, ops [2]uuid.UUID) {
	t.Helper()
	wc := decodeJSON[workCenterResponse](t, doAuthedRequest(t, handler, token, http.MethodPost, workCentersPath, `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusCreated)
	p := seedProductionPosition(t, env, company)
	order = decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, token, http.MethodPost, productionOrdersPath, p.body(4, "")), http.StatusCreated).ID
	for i, name := range []string{"Zuschnitt", "Verglasung"} {
		ops[i] = decodeJSON[operationResponse](t, doAuthedRequest(t, handler, token, http.MethodPost, operationsPath(order),
			`{"name":"`+name+`","work_center_id":"`+wc.ID.String()+`","ruestzeit_min":30,"minuten_je_stueck":10}`), http.StatusCreated).ID
	}
	return order, ops
}

func TestProductionFeedbackHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("production")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	order, ops := seedFeedbackOrder(t, env, handler, token, c.company)
	opPath := func(op uuid.UUID) string { return operationsPath(order) + "/" + op.String() }
	done1, done2 := opPath(ops[0])+"/erledigt", opPath(ops[1])+"/erledigt"
	countFeedback := func() int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM production_operation_feedback WHERE company_id=$1`, c.company).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	const status = "Rückmeldung nur bei begonnenen Aufträgen möglich"
	wantAPIError(t, do(http.MethodPost, done1, ""), http.StatusConflict, "conflict", status)
	decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath+order.String()+"/freigeben", ""), http.StatusOK)
	wantAPIError(t, do(http.MethodPost, done1, ""), http.StatusConflict, "conflict", status)
	decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath+order.String()+"/beginnen", ""), http.StatusOK)

	for _, tc := range []struct {
		path, body, code, msg string
		status                int
	}{
		{done1, `{"ist_minuten":`, "validation_error", "Ungültige Eingabe", http.StatusBadRequest},
		{done1, `{"ist_minuten":0}`, "validation_error", "Ungültige Ist-Zeit (größer als 0, höchstens 100.000 Minuten)", http.StatusBadRequest},
		{done1, `{"ist_minuten":100001}`, "validation_error", "Ungültige Ist-Zeit (größer als 0, höchstens 100.000 Minuten)", http.StatusBadRequest},
		{done1, `{"notiz":"` + strings.Repeat("x", 501) + `"}`, "validation_error", "Notiz ist zu lang (höchstens 500 Zeichen)", http.StatusBadRequest},
		{operationsPath(order) + "/kaputt/erledigt", "", "validation_error", "Ungültige Arbeitsgang-ID", http.StatusBadRequest},
		{opPath(uuid.New()) + "/erledigt", "", "not_found", "Arbeitsgang nicht gefunden", http.StatusNotFound},
		{productionOrdersPath + "kaputt/operations/" + ops[0].String() + "/erledigt", "", "validation_error", "Ungültige Auftrags-ID", http.StatusBadRequest},
		{operationsPath(uuid.New()) + "/" + ops[0].String() + "/erledigt", "", "not_found", "Fertigungsauftrag nicht gefunden", http.StatusNotFound},
	} {
		wantAPIError(t, do(http.MethodPost, tc.path, tc.body), tc.status, tc.code, tc.msg)
	}
	if n := countFeedback(); n != 0 {
		t.Fatalf("Rückmeldungen nach Fehlern: %d", n)
	}

	x := decodeJSON[feedbackOperationResponse](t, do(http.MethodPost, done1, `{"ist_minuten":95.456,"notiz":" Rest morgen "}`), http.StatusOK)
	if x.ID != ops[0] || !x.Erledigt || x.ErledigtAt == nil || x.IstMinuten == nil || *x.IstMinuten != 95.46 || x.RueckmeldungNotiz != "Rest morgen" {
		t.Fatalf("gemeldet: %+v", x)
	}
	if x = decodeJSON[feedbackOperationResponse](t, do(http.MethodPost, done2, ""), http.StatusOK); !x.Erledigt || x.IstMinuten != nil {
		t.Fatalf("ohne Body: %+v", x)
	}
	wantAPIError(t, do(http.MethodPost, done1, ""), http.StatusConflict, "conflict", "Arbeitsgang ist bereits erledigt gemeldet")
	if list := decodeJSON[[]feedbackOperationResponse](t, do(http.MethodGet, operationsPath(order), ""), http.StatusOK); len(list) != 2 || !list[0].Erledigt || !list[1].Erledigt {
		t.Fatalf("Liste: %+v", list)
	}

	revert := opPath(ops[1]) + "/zuruecknehmen"
	wantAPIError(t, do(http.MethodPost, revert, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPost, revert, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Rücknahme der Rückmeldung nur mit Begründung")
	wantAPIError(t, do(http.MethodPost, revert, `{"reason":"`+strings.Repeat("x", 501)+`"}`), http.StatusBadRequest, "validation_error", "Begründung ist zu lang (höchstens 500 Zeichen)")
	wantAPIError(t, do(http.MethodPost, operationsPath(order)+"/kaputt/zuruecknehmen", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Arbeitsgang-ID")
	if x = decodeJSON[feedbackOperationResponse](t, do(http.MethodPost, revert, `{"reason":"versehentlich"}`), http.StatusOK); x.Erledigt || x.ErledigtAt != nil {
		t.Fatalf("zurückgenommen: %+v", x)
	}
	wantAPIError(t, do(http.MethodPost, revert, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Arbeitsgang ist nicht erledigt gemeldet")

	want := sollIstResponse{Arbeitsgaenge: 2, Erledigt: 1, SollMin: 140, SollErledigtMin: 70, MitIst: 1, SollMitIstMin: 70, IstMin: 95.46}
	if got := decodeJSON[sollIstResponse](t, do(http.MethodGet, productionOrdersPath+order.String()+"/soll-ist", ""), http.StatusOK); got != want {
		t.Fatalf("Soll/Ist: %+v", got)
	}
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+"kaputt/soll-ist", ""), http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+uuid.NewString()+"/soll-ist", ""), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")

	// Arbeitsgang mit Rueckmeldung (auch zurueckgenommener) nicht entfernbar.
	for _, op := range ops {
		wantAPIError(t, do(http.MethodDelete, opPath(op), ""), http.StatusConflict, "conflict", "Arbeitsgang mit Rückmeldung kann nicht entfernt werden")
	}
}

func TestProductionFeedbackHTTPPermissionsAndTenants(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pf-readonly", "production.read")
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	order, ops := seedFeedbackOrder(t, env, handler, prod, c.company)
	for _, step := range []string{"/freigeben", "/beginnen"} {
		decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, productionOrdersPath+order.String()+step, ""), http.StatusOK)
	}
	opPath := operationsPath(order) + "/" + ops[0].String()
	decodeJSON[feedbackOperationResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, opPath+"/erledigt", `{"ist_minuten":10}`), http.StatusOK)
	sollIst := productionOrdersPath + order.String() + "/soll-ist"
	done2, revert := operationsPath(order)+"/"+ops[1].String()+"/erledigt", opPath+"/zuruecknehmen"
	count := func() (alle, gueltig int) {
		t.Helper()
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*), count(*) FILTER (WHERE zurueckgenommen_at IS NULL) FROM production_operation_feedback
			WHERE company_id=$1`, c.company).Scan(&alle, &gueltig); err != nil {
			t.Fatal(err)
		}
		return alle, gueltig
	}

	for name, tc := range map[string]struct {
		role    string
		canRead bool
	}{
		"nur read": {"itest-pf-readonly", true},
		"Vertrieb": {"sales", true},
		"Personal": {"hr", false},
		"Lager":    {"inventory", false},
	} {
		t.Run(name, func(t *testing.T) {
			token := c.user(tc.role)
			if rec := doAuthedRequest(t, handler, token, http.MethodGet, sollIst, ""); tc.canRead != (rec.Code == http.StatusOK) {
				t.Fatalf("lesen: %d %s", rec.Code, rec.Body.String())
			}
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, done2, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, revert, `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}

	// Anderer Mandant: Auftrag unbekannt, nichts gemeldet oder zurueckgenommen.
	two := newCatalogTenant(t, env, handler).user("admin")
	for _, ch := range []struct{ method, path, body string }{
		{http.MethodGet, sollIst, ""},
		{http.MethodPost, done2, ""},
		{http.MethodPost, revert, `{"reason":"x"}`},
	} {
		wantAPIError(t, doAuthedRequest(t, handler, two, ch.method, ch.path, ch.body), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
	}
	if alle, gueltig := count(); alle != 1 || gueltig != 1 {
		t.Fatalf("verbotene Änderung hat gewirkt: %d/%d", alle, gueltig)
	}
}

// Jeder Fehlertext der Rueckmeldung muss als 4xx klassifiziert werden.
func TestClassifyDomainErrorCoversProductionFeedbackMessages(t *testing.T) {
	for msg, want := range map[string]int{
		"Rückmeldung nur bei begonnenen Aufträgen möglich":             http.StatusConflict,
		"Arbeitsgang ist bereits erledigt gemeldet":                    http.StatusConflict,
		"Arbeitsgang ist nicht erledigt gemeldet":                      http.StatusConflict,
		"Arbeitsgang mit Rückmeldung kann nicht entfernt werden":       http.StatusConflict,
		"Ungültige Ist-Zeit (größer als 0, höchstens 100.000 Minuten)": http.StatusBadRequest,
		"Notiz ist zu lang (höchstens 500 Zeichen)":                    http.StatusBadRequest,
		"Rücknahme der Rückmeldung nur mit Begründung":                 http.StatusBadRequest,
		"Begründung ist zu lang (höchstens 500 Zeichen)":               http.StatusBadRequest,
		"Arbeitsgang nicht gefunden":                                   http.StatusNotFound,
	} {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
