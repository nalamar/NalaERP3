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

// Montagetermine ueber HTTP (Backlog H.3.6d). Helfer aus
// production_orders_integration_test.go und
// qualification_types_integration_test.go.

const assemblyOverviewPath = "/api/v1/production/montage"

type assemblyResponse struct {
	ID           uuid.UUID  `json:"id"`
	OrderID      *uuid.UUID `json:"order_id"`
	Von          string     `json:"von"`
	Bis          string     `json:"bis"`
	TeamID       *uuid.UUID `json:"team_id"`
	TeamName     string     `json:"team_name"`
	Kennzeichen  string     `json:"kennzeichen"`
	Baustelle    string     `json:"baustelle"`
	StornoReason string     `json:"storno_reason"`
	StorniertAt  *string    `json:"storniert_at"`
	Hinweise     []string   `json:"hinweise"`
}

type assemblyOverviewResponse struct {
	Von   string `json:"von"`
	Bis   string `json:"bis"`
	Teams []struct {
		TeamID   *uuid.UUID         `json:"team_id"`
		TeamName string             `json:"team_name"`
		Termine  []assemblyResponse `json:"termine"`
	} `json:"teams"`
}

type assemblyChoicesResponse struct {
	Teams []struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"teams"`
	Fahrzeuge []struct {
		ID           uuid.UUID `json:"id"`
		Kennzeichen  string    `json:"kennzeichen"`
		AbgemeldetAm *string   `json:"abgemeldet_am"`
	} `json:"fahrzeuge"`
}

type assemblySeed struct {
	order, team, vehicle, offVehicle, laterVehicle uuid.UUID
}

// seedAssembly legt einen Auftrag (Liefertermin 01.03.2030), ein Team und
// drei Fahrzeuge an: aktiv, abgemeldet und erst 2031 abgemeldet.
func seedAssembly(t *testing.T, env *testutil.IntegrationEnv, handler http.Handler, token, company string) assemblySeed {
	t.Helper()
	p := seedProductionPosition(t, env, company)
	s := assemblySeed{team: uuid.New(), vehicle: uuid.New(), offVehicle: uuid.New(), laterVehicle: uuid.New()}
	s.order = decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, token, http.MethodPost, productionOrdersPath,
		p.body(1, `,"liefertermin":"2030-03-01"`)), http.StatusCreated).ID
	ctx := context.Background()
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_teams (id, name, company_id) VALUES ($1,'Montage Nord',$2)`, s.team, company); err != nil {
		t.Fatal(err)
	}
	ins := `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art, abgemeldet_am, abgemeldet_grund) VALUES ($1,$2,$3,'transporter',$4,$5)`
	for _, v := range []struct {
		id         uuid.UUID
		kz, ab, gr any
	}{
		{s.vehicle, "MA-1", nil, ""},
		{s.offVehicle, "MA-2", "2026-01-31", "verkauft"},
		{s.laterVehicle, "MA-3", "2031-01-31", "Leasingende"},
	} {
		if _, err := env.PG.Exec(ctx, ins, v.id, company, v.kz, v.ab, v.gr); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestProductionAssemblyHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("production")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	s := seedAssembly(t, env, handler, token, c.company)
	foreign := newCatalogTenant(t, env, handler)
	foreignTeam := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO hr_teams (id, name, company_id) VALUES ($1,'Fremd',$2)`, foreignTeam, foreign.company); err != nil {
		t.Fatal(err)
	}
	base := productionOrdersPath + s.order.String() + "/montage"
	body := func(von, bis, extra string) string {
		return `{"von":"` + von + `","bis":"` + bis + `","team_id":"` + s.team.String() + `","vehicle_id":"` + s.vehicle.String() + `"` + extra + `}`
	}
	countRows := func() int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM production_assembly_appointments WHERE company_id=$1`, c.company).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	ch := decodeJSON[assemblyChoicesResponse](t, do(http.MethodGet, assemblyOverviewPath+"/auswahl", ""), http.StatusOK)
	if len(ch.Teams) != 1 || ch.Teams[0].ID != s.team || ch.Teams[0].Name != "Montage Nord" || len(ch.Fahrzeuge) != 2 ||
		ch.Fahrzeuge[0].ID != s.vehicle || ch.Fahrzeuge[1].ID != s.laterVehicle || ch.Fahrzeuge[1].AbgemeldetAm == nil {
		t.Fatalf("Auswahl: %+v", ch)
	}

	for _, tc := range []struct {
		path, body, code, msg string
		status                int
	}{
		{base, `{`, "validation_error", "Ungültige Eingabe", http.StatusBadRequest},
		{base, body("07.03.2030", "2030-03-08", ""), "validation_error", "Ungültiges Datum in von (erwartet JJJJ-MM-TT)", http.StatusBadRequest},
		{base, body("2030-03-07", "8.3.2030", ""), "validation_error", "Ungültiges Datum in bis (erwartet JJJJ-MM-TT)", http.StatusBadRequest},
		{base, body("", "2030-03-08", ""), "validation_error", "Zeitraum der Montage erforderlich (von und bis)", http.StatusBadRequest},
		{base, body("2030-03-07", "2030-03-06", ""), "validation_error", "Ungültiger Zeitraum der Montage: bis liegt vor von", http.StatusBadRequest},
		{base, body("2030-03-01", "2030-04-30", ""), "validation_error", "Montage ist zu lang (höchstens 60 Tage)", http.StatusBadRequest},
		{base, body("2030-03-07", "2030-03-08", `,"baustelle":"`+strings.Repeat("x", 301)+`"`), "validation_error", "Baustelle ist zu lang (höchstens 300 Zeichen)", http.StatusBadRequest},
		{base, body("2030-03-07", "2030-03-08", `,"notiz":"`+strings.Repeat("x", 501)+`"`), "validation_error", "Notiz ist zu lang (höchstens 500 Zeichen)", http.StatusBadRequest},
		{base, `{"von":"2030-03-07","bis":"2030-03-08","team_id":"` + foreignTeam.String() + `"}`, "not_found", "Team nicht gefunden", http.StatusNotFound},
		{base, `{"von":"2030-03-07","bis":"2030-03-08","vehicle_id":"` + uuid.NewString() + `"}`, "not_found", "Fahrzeug nicht gefunden", http.StatusNotFound},
		{base, `{"von":"2030-03-07","bis":"2030-03-08","vehicle_id":"` + s.offVehicle.String() + `"}`, "conflict", "Fahrzeug ist abgemeldet", http.StatusConflict},
		{productionOrdersPath + "kaputt/montage", body("2030-03-07", "2030-03-08", ""), "validation_error", "Ungültige Auftrags-ID", http.StatusBadRequest},
		{productionOrdersPath + uuid.NewString() + "/montage", body("2030-03-07", "2030-03-08", ""), "not_found", "Fertigungsauftrag nicht gefunden", http.StatusNotFound},
	} {
		wantAPIError(t, do(http.MethodPost, tc.path, tc.body), tc.status, tc.code, tc.msg)
	}
	if n := countRows(); n != 0 {
		t.Fatalf("Termine nach Fehlern: %d", n)
	}

	a := decodeJSON[assemblyResponse](t, do(http.MethodPost, base, body("2030-03-04", "2030-03-06", `,"baustelle":" Hafenstr. 2 "`)), http.StatusCreated)
	if a.OrderID == nil || *a.OrderID != s.order || a.Von[:10] != "2030-03-04" || a.Bis[:10] != "2030-03-06" || a.TeamName != "Montage Nord" ||
		a.Kennzeichen != "MA-1" || a.Baustelle != "Hafenstr. 2" || len(a.Hinweise) != 1 || a.Hinweise[0] != "nach_liefertermin" {
		t.Fatalf("angelegt: %+v", a)
	}
	if list := decodeJSON[[]assemblyResponse](t, do(http.MethodGet, base, ""), http.StatusOK); len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("Liste: %+v", list)
	}
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+"kaputt/montage", ""), http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+uuid.NewString()+"/montage", ""), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")

	one := base + "/" + a.ID.String()
	wantAPIError(t, do(http.MethodPut, base+"/kaputt", body("2030-02-25", "2030-02-27", "")), http.StatusBadRequest, "validation_error", "Ungültige Termin-ID")
	wantAPIError(t, do(http.MethodPut, base+"/"+uuid.NewString(), body("2030-02-25", "2030-02-27", "")), http.StatusNotFound, "not_found", "Montagetermin nicht gefunden")
	wantAPIError(t, do(http.MethodPut, one, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPut, one, body("2030-02-25", "2030-02-24", "")), http.StatusBadRequest, "validation_error", "Ungültiger Zeitraum der Montage: bis liegt vor von")
	if x := decodeJSON[assemblyResponse](t, do(http.MethodPut, one, `{"von":"2030-02-25","bis":"2030-02-27","vehicle_id":"`+s.laterVehicle.String()+`"}`), http.StatusOK); x.Von[:10] != "2030-02-25" || x.TeamID != nil || x.Kennzeichen != "MA-3" || len(x.Hinweise) != 0 {
		t.Fatalf("geändert: %+v", x)
	}

	ov := decodeJSON[assemblyOverviewResponse](t, do(http.MethodGet, assemblyOverviewPath+"?von=2030-02-27&bis=2030-03-05", ""), http.StatusOK)
	if ov.Von[:10] != "2030-02-25" || ov.Bis[:10] != "2030-03-10" || len(ov.Teams) != 2 || ov.Teams[0].TeamName != "Montage Nord" || len(ov.Teams[0].Termine) != 0 ||
		ov.Teams[1].TeamID != nil || len(ov.Teams[1].Termine) != 1 || ov.Teams[1].Termine[0].ID != a.ID {
		t.Fatalf("Übersicht: %+v", ov)
	}
	if ov = decodeJSON[assemblyOverviewResponse](t, do(http.MethodGet, assemblyOverviewPath, ""), http.StatusOK); len(ov.Teams) != 1 {
		t.Fatalf("Übersicht ohne Angabe: %+v", ov)
	}
	for q, msg := range map[string]string{
		"?von=1.3.2030":                  "Ungültiges Datum in von (erwartet JJJJ-MM-TT)",
		"?bis=x":                         "Ungültiges Datum in bis (erwartet JJJJ-MM-TT)",
		"?von=2030-03-10&bis=2030-03-01": "Ungültiger Zeitraum: bis liegt vor von",
		"?von=2030-01-07&bis=2030-07-08": "Zeitraum ist zu groß (höchstens 26 Wochen)",
	} {
		wantAPIError(t, do(http.MethodGet, assemblyOverviewPath+q, ""), http.StatusBadRequest, "validation_error", msg)
	}

	cancel := one + "/stornieren"
	wantAPIError(t, do(http.MethodPost, cancel, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPost, cancel, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno des Montagetermins nur mit Begründung")
	wantAPIError(t, do(http.MethodPost, cancel, `{"reason":"`+strings.Repeat("x", 501)+`"}`), http.StatusBadRequest, "validation_error", "Begründung ist zu lang (höchstens 500 Zeichen)")
	wantAPIError(t, do(http.MethodPost, base+"/kaputt/stornieren", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Termin-ID")
	if x := decodeJSON[assemblyResponse](t, do(http.MethodPost, cancel, `{"reason":"verschoben"}`), http.StatusOK); x.StorniertAt == nil || x.StornoReason != "verschoben" {
		t.Fatalf("storniert: %+v", x)
	}
	wantAPIError(t, do(http.MethodPost, cancel, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Montagetermin ist bereits storniert")
	wantAPIError(t, do(http.MethodPut, one, body("2030-02-25", "2030-02-27", "")), http.StatusConflict, "conflict", "Montagetermin ist storniert")

	decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath+s.order.String()+"/storno", `{"reason":"Kunde abgesprungen"}`), http.StatusOK)
	wantAPIError(t, do(http.MethodPost, base, body("2030-03-07", "2030-03-08", "")), http.StatusConflict, "conflict", "Montagetermin für stornierte Aufträge nicht möglich")
}

func TestProductionAssemblyHTTPPermissionsAndTenants(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pa-readonly", "production.read")
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	s := seedAssembly(t, env, handler, prod, c.company)
	base := productionOrdersPath + s.order.String() + "/montage"
	a := decodeJSON[assemblyResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, base, `{"von":"2030-03-04","bis":"2030-03-06","team_id":"`+s.team.String()+`"}`), http.StatusCreated)
	one := base + "/" + a.ID.String()
	upd := `{"von":"2030-03-05","bis":"2030-03-06"}`
	state := func() (von string, storniert bool) {
		t.Helper()
		if err := env.PG.QueryRow(context.Background(), `SELECT von::text, storniert_at IS NOT NULL FROM production_assembly_appointments WHERE id=$1`, a.ID).Scan(&von, &storniert); err != nil {
			t.Fatal(err)
		}
		return von, storniert
	}
	reads := []string{base, assemblyOverviewPath + "?von=2030-03-04", assemblyOverviewPath + "/auswahl"}

	for name, tc := range map[string]struct {
		role    string
		canRead bool
	}{
		"nur read": {"itest-pa-readonly", true},
		"Vertrieb": {"sales", true},
		"Personal": {"hr", false},
		"Lager":    {"inventory", false},
	} {
		t.Run(name, func(t *testing.T) {
			token := c.user(tc.role)
			for _, path := range reads {
				if rec := doAuthedRequest(t, handler, token, http.MethodGet, path, ""); tc.canRead != (rec.Code == http.StatusOK) {
					t.Fatalf("lesen %s: %d %s", path, rec.Code, rec.Body.String())
				}
			}
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, base, upd), http.StatusForbidden, "forbidden", "Keine Berechtigung")
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPut, one, upd), http.StatusForbidden, "forbidden", "Keine Berechtigung")
			wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, one+"/stornieren", `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}

	// Anderer Mandant: Auftrag unbekannt, Uebersicht und Auswahl ohne
	// fremde Teams, Fahrzeuge und Termine.
	two := newCatalogTenant(t, env, handler).user("admin")
	for _, ch := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, upd},
		{http.MethodPut, one, upd},
		{http.MethodPost, one + "/stornieren", `{"reason":"x"}`},
	} {
		wantAPIError(t, doAuthedRequest(t, handler, two, ch.method, ch.path, ch.body), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
	}
	if ov := decodeJSON[assemblyOverviewResponse](t, doAuthedRequest(t, handler, two, http.MethodGet, assemblyOverviewPath+"?von=2030-03-04", ""), http.StatusOK); len(ov.Teams) != 0 {
		t.Fatalf("Übersicht anderer Mandant: %+v", ov)
	}
	if ch := decodeJSON[assemblyChoicesResponse](t, doAuthedRequest(t, handler, two, http.MethodGet, assemblyOverviewPath+"/auswahl", ""), http.StatusOK); len(ch.Teams) != 0 || len(ch.Fahrzeuge) != 0 {
		t.Fatalf("Auswahl anderer Mandant: %+v", ch)
	}
	if von, storniert := state(); von != "2030-03-04" || storniert {
		t.Fatalf("verbotene Änderung hat gewirkt: %s %v", von, storniert)
	}
}

// Jeder Fehlertext der Montagetermine muss als 4xx klassifiziert werden.
func TestClassifyDomainErrorCoversProductionAssemblyMessages(t *testing.T) {
	for msg, want := range map[string]int{
		"Fertigungsauftrag für den Montagetermin erforderlich": http.StatusBadRequest,
		"Zeitraum der Montage erforderlich (von und bis)":      http.StatusBadRequest,
		"Ungültiger Zeitraum der Montage: bis liegt vor von":   http.StatusBadRequest,
		"Montage ist zu lang (höchstens 60 Tage)":              http.StatusBadRequest,
		"Baustelle ist zu lang (höchstens 300 Zeichen)":        http.StatusBadRequest,
		"Notiz ist zu lang (höchstens 500 Zeichen)":            http.StatusBadRequest,
		"Storno des Montagetermins nur mit Begründung":         http.StatusBadRequest,
		"Begründung ist zu lang (höchstens 500 Zeichen)":       http.StatusBadRequest,
		"Montagetermin für stornierte Aufträge nicht möglich":  http.StatusConflict,
		"Montagetermin ist storniert":                          http.StatusConflict,
		"Montagetermin ist bereits storniert":                  http.StatusConflict,
		"Fahrzeug ist abgemeldet":                              http.StatusConflict,
		"Montagetermin nicht gefunden":                         http.StatusNotFound,
		"Team nicht gefunden":                                  http.StatusNotFound,
		"Fahrzeug nicht gefunden":                              http.StatusNotFound,
	} {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
