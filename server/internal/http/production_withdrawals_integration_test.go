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

// Entnahmen am Fertigungsauftrag ueber HTTP (Backlog H.3.2c). Helfer aus
// production_orders_integration_test.go.

type withdrawalResponse struct {
	ID           uuid.UUID  `json:"id"`
	ItemID       *uuid.UUID `json:"item_id"`
	MaterialID   string     `json:"material_id"`
	Menge        float64    `json:"menge"`
	StornoReason string     `json:"storno_reason"`
	StorniertAt  *string    `json:"storniert_at"`
	Hinweise     []string   `json:"hinweise"`
}

func withdrawalsPath(orderID uuid.UUID) string {
	return productionOrdersPath + orderID.String() + "/withdrawals"
}

// seedWithdrawalStock legt Profil-Material (m) mit 10 m Bestand und ein
// Lager an und haengt das Material an die Profile der Variante.
func seedWithdrawalStock(t *testing.T, env *testutil.IntegrationEnv, company string, p productionPosition) (mat, wh string) {
	t.Helper()
	mat, wh = "mat-"+uuid.NewString()[:12], "wh-"+uuid.NewString()[:12]
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Profil','profil','m',$2)`, []any{mat, company}},
		{`INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Halle',$2)`, []any{wh, company}},
		{`INSERT INTO stock_movements (id, material_id, warehouse_id, quantity, uom, movement_type) VALUES ($1,$2,$3,10,'m','purchase')`,
			[]any{uuid.NewString(), mat, wh}},
		{`UPDATE single_elevation_profiles SET material_id=$2 WHERE single_elevation_id=$1`, []any{p.variant, mat}},
	} {
		if _, err := env.PG.Exec(context.Background(), q.sql, q.args...); err != nil {
			t.Fatalf("Testdaten: %v", err)
		}
	}
	return mat, wh
}

func TestProductionWithdrawalsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("production")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	ctx := context.Background()
	p := seedProductionPosition(t, env, c.company)
	mat, wh := seedWithdrawalStock(t, env, c.company, p)
	other := "mat-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(ctx, `INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Schraube','artikel','Stk',$2)`,
		other, c.company); err != nil {
		t.Fatal(err)
	}
	countWithdrawals := func() int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM production_order_withdrawals WHERE company_id=$1`, c.company).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// 2 Stueck: Bedarf 4,8 m.
	o := decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath, p.body(2, "")), http.StatusCreated)
	path := withdrawalsPath(o.ID)
	body := `{"material_id":"` + mat + `","warehouse_id":"` + wh + `","menge":1}`
	wantAPIError(t, do(http.MethodPost, path, body), http.StatusConflict, "conflict", "Entnahme nur bei freigegebenen oder begonnenen Aufträgen möglich")
	detail := decodeJSON[struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
	}](t, do(http.MethodPost, productionOrdersPath+o.ID.String()+"/freigeben", `{"lager_id":"`+wh+`"}`), http.StatusOK)
	if len(detail.Items) != 1 {
		t.Fatalf("Stückliste: %+v", detail)
	}
	item := detail.Items[0].ID.String()

	for _, tc := range []struct {
		body, code, msg string
		status          int
	}{
		{`{"item_id":`, "validation_error", "Ungültige Eingabe", http.StatusBadRequest},
		{`{"item_id":"` + item + `","warehouse_id":"` + wh + `","menge":0}`, "validation_error", "Ungültige Menge (größer als 0, höchstens 1.000.000)", http.StatusBadRequest},
		{`{"warehouse_id":"` + wh + `","menge":1}`, "validation_error", "Bitte eine Zeile der Stückliste oder ein Material angeben", http.StatusBadRequest},
		{`{"item_id":"` + item + `","menge":1}`, "validation_error", "Bitte ein Lager angeben", http.StatusBadRequest},
		{`{"item_id":"` + item + `","warehouse_id":"` + wh + `","menge":1,"notiz":"` + strings.Repeat("x", 501) + `"}`, "validation_error", "Notiz ist zu lang (höchstens 500 Zeichen)", http.StatusBadRequest},
		{`{"item_id":"` + item + `","material_id":"` + other + `","warehouse_id":"` + wh + `","menge":1}`, "validation_error", "Material passt nicht zur Zeile", http.StatusBadRequest},
		{`{"item_id":"` + uuid.NewString() + `","warehouse_id":"` + wh + `","menge":1}`, "not_found", "Zeile nicht im Fertigungsauftrag", http.StatusNotFound},
		{`{"item_id":"` + item + `","warehouse_id":"gibt-es-nicht","menge":1}`, "not_found", "Lager nicht gefunden", http.StatusNotFound},
	} {
		wantAPIError(t, do(http.MethodPost, path, tc.body), tc.status, tc.code, tc.msg)
	}
	wantAPIError(t, do(http.MethodPost, productionOrdersPath+"kaputt/withdrawals", body), http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
	wantAPIError(t, do(http.MethodPost, withdrawalsPath(uuid.New()), body), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
	if n := countWithdrawals(); n != 0 {
		t.Fatalf("Entnahmen nach Fehlern: %d", n)
	}

	w1 := decodeJSON[withdrawalResponse](t, do(http.MethodPost, path, `{"item_id":"`+item+`","warehouse_id":"`+wh+`","menge":2}`), http.StatusCreated)
	if w1.ItemID == nil || w1.ItemID.String() != item || w1.MaterialID != mat || w1.Menge != 2 || len(w1.Hinweise) != 0 {
		t.Fatalf("Entnahme: %+v", w1)
	}
	w2 := decodeJSON[withdrawalResponse](t, do(http.MethodPost, path, `{"item_id":"`+item+`","warehouse_id":"`+wh+`","menge":3}`), http.StatusCreated)
	if strings.Join(w2.Hinweise, ",") != "mehrentnahme" {
		t.Fatalf("Mehrentnahme: %+v", w2)
	}
	if list := decodeJSON[[]withdrawalResponse](t, do(http.MethodGet, path, ""), http.StatusOK); len(list) != 2 || list[0].ID != w2.ID {
		t.Fatalf("Liste: %+v", list)
	}

	storno := path + "/" + w2.ID.String() + "/storno"
	wantAPIError(t, do(http.MethodPost, storno, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPost, storno, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno der Entnahme nur mit Begründung")
	wantAPIError(t, do(http.MethodPost, path+"/kaputt/storno", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Entnahme-ID")
	wantAPIError(t, do(http.MethodPost, path+"/"+uuid.NewString()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Entnahme nicht gefunden")
	s := decodeJSON[withdrawalResponse](t, do(http.MethodPost, storno, `{"reason":"Zählfehler"}`), http.StatusOK)
	if s.StorniertAt == nil || s.StornoReason != "Zählfehler" {
		t.Fatalf("Storno: %+v", s)
	}
	wantAPIError(t, do(http.MethodPost, storno, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Entnahme ist bereits storniert")
	var bestand float64
	if err := env.PG.QueryRow(ctx, `SELECT SUM(quantity)::float8 FROM stock_movements WHERE material_id=$1`, mat).Scan(&bestand); err != nil {
		t.Fatal(err)
	}
	if bestand != 8 {
		t.Fatalf("Bestand: %v", bestand)
	}
}

func TestProductionWithdrawalsHTTPPermissionsAndTenants(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pw-readonly", "production.read")
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	p := seedProductionPosition(t, env, c.company)
	mat, wh := seedWithdrawalStock(t, env, c.company, p)
	o := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated)
	decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, productionOrdersPath+o.ID.String()+"/freigeben", `{"lager_id":"`+wh+`"}`), http.StatusOK)
	path := withdrawalsPath(o.ID)
	body := `{"material_id":"` + mat + `","warehouse_id":"` + wh + `","menge":1}`
	w := decodeJSON[withdrawalResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, path, body), http.StatusCreated)
	storno := path + "/" + w.ID.String() + "/storno"
	count := func() (alle, storniert int) {
		t.Helper()
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*), count(storniert_at) FROM production_order_withdrawals WHERE order_id=$1`, o.ID).
			Scan(&alle, &storniert); err != nil {
			t.Fatal(err)
		}
		return alle, storniert
	}

	for name, tc := range map[string]struct {
		role              string
		canRead, canWrite bool
	}{
		"nur read": {"itest-pw-readonly", true, false},
		"Vertrieb": {"sales", true, false},
		"Personal": {"hr", false, false},
		"Lager":    {"inventory", false, false},
	} {
		t.Run(name, func(t *testing.T) {
			token := c.user(tc.role)
			if rec := doAuthedRequest(t, handler, token, http.MethodGet, path, ""); tc.canRead != (rec.Code == http.StatusOK) {
				t.Fatalf("lesen: %d %s", rec.Code, rec.Body.String())
			}
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, path, body), http.StatusForbidden, "forbidden", "Keine Berechtigung")
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, storno, `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}

	// Anderer Mandant: Auftrag unbekannt, nichts gebucht oder storniert.
	two := newCatalogTenant(t, env, handler).user("admin")
	for _, ch := range []struct{ method, path, body string }{
		{http.MethodGet, path, ""},
		{http.MethodPost, path, body},
		{http.MethodPost, storno, `{"reason":"x"}`},
	} {
		wantAPIError(t, doAuthedRequest(t, handler, two, ch.method, ch.path, ch.body), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
	}
	if alle, storniert := count(); alle != 1 || storniert != 0 {
		t.Fatalf("verbotene Änderung hat gewirkt: %d/%d", alle, storniert)
	}
}

// Jeder Fehlertext der Entnahme muss als 4xx klassifiziert werden.
func TestClassifyDomainErrorCoversProductionWithdrawalMessages(t *testing.T) {
	for msg, want := range map[string]int{
		"Entnahme nur bei freigegebenen oder begonnenen Aufträgen möglich": http.StatusConflict,
		"Entnahme ist bereits storniert":                                   http.StatusConflict,
		"Entnahme nicht gefunden":                                          http.StatusNotFound,
		"Zeile nicht im Fertigungsauftrag":                                 http.StatusNotFound,
		"Zeile hat kein Lagermaterial":                                     http.StatusBadRequest,
		"Material passt nicht zur Zeile":                                   http.StatusBadRequest,
		"Ungültige Menge (größer als 0, höchstens 1.000.000)":              http.StatusBadRequest,
		"Notiz ist zu lang (höchstens 500 Zeichen)":                        http.StatusBadRequest,
		"Bitte eine Zeile der Stückliste oder ein Material angeben":        http.StatusBadRequest,
		"Bitte ein Lager angeben":                                          http.StatusBadRequest,
		"Storno der Entnahme nur mit Begründung":                           http.StatusBadRequest,
		"Lagerort nicht gefunden":                                          http.StatusNotFound,
	} {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
