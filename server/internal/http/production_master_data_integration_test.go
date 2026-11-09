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

// Arbeitsplaetze und Arbeitsgangvorlagen ueber HTTP (Backlog H.2.2).
// Helfer wie in production_orders_integration_test.go.

const (
	workCentersPath = "/api/v1/production/work-centers/"
	opTemplatesPath = "/api/v1/production/operation-templates/"
)

type workCenterResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	StundenJeTag float64   `json:"stunden_je_tag"`
	Aktiv        bool      `json:"aktiv"`
}

type opTemplateResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	WorkCenterID    uuid.UUID `json:"work_center_id"`
	WorkCenterName  string    `json:"work_center_name"`
	Reihenfolge     int       `json:"reihenfolge"`
	RuestzeitMin    float64   `json:"ruestzeit_min"`
	MinutenJeStueck float64   `json:"minuten_je_stueck"`
	Aktiv           bool      `json:"aktiv"`
}

func workCenterNames(list []workCenterResponse) string {
	names := []string{}
	for _, x := range list {
		names = append(names, x.Name)
	}
	return strings.Join(names, ",")
}

func templateNames(list []opTemplateResponse) string {
	names := []string{}
	for _, x := range list {
		names = append(names, x.Name)
	}
	return strings.Join(names, ",")
}

func TestProductionMasterDataHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("production")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}

	// Arbeitsplaetze
	zu := decodeJSON[workCenterResponse](t, do(http.MethodPost, workCentersPath, `{"name":"  Zuschnitt ","stunden_je_tag":8}`), http.StatusCreated)
	if zu.Name != "Zuschnitt" || zu.StundenJeTag != 8 || !zu.Aktiv {
		t.Fatalf("Arbeitsplatz: %+v", zu)
	}
	mo := decodeJSON[workCenterResponse](t, do(http.MethodPost, workCentersPath, `{"name":"Montage","stunden_je_tag":7.5}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, workCentersPath, `{"name":" zuschnitt ","stunden_je_tag":8}`), http.StatusBadRequest, "validation_error", "Arbeitsplatz mit diesem Namen bereits vorhanden")
	wantAPIError(t, do(http.MethodPost, workCentersPath, `{"name":"Null","stunden_je_tag":0}`), http.StatusBadRequest, "validation_error", "Ungültige Stunden je Tag (größer als 0, höchstens 24)")
	wantAPIError(t, do(http.MethodPost, workCentersPath, `{"name":" ","stunden_je_tag":8}`), http.StatusBadRequest, "validation_error", "Name des Arbeitsplatzes erforderlich")
	wantAPIError(t, do(http.MethodPost, workCentersPath, `{"name":`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPut, workCentersPath+uuid.NewString(), `{"name":"x","stunden_je_tag":8}`), http.StatusNotFound, "not_found", "Arbeitsplatz nicht gefunden")
	wantAPIError(t, do(http.MethodPut, workCentersPath+mo.ID.String(), `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusBadRequest, "validation_error", "Arbeitsplatz mit diesem Namen bereits vorhanden")

	// Vorlage auf Montage, solange aktiv; danach Montage deaktivieren.
	glas := decodeJSON[opTemplateResponse](t, do(http.MethodPost, opTemplatesPath,
		`{"name":" Verglasung ","work_center_id":"`+mo.ID.String()+`","reihenfolge":3,"ruestzeit_min":10,"minuten_je_stueck":20}`), http.StatusCreated)
	if glas.Name != "Verglasung" || glas.WorkCenterName != "Montage" || glas.RuestzeitMin != 10 || glas.MinutenJeStueck != 20 || !glas.Aktiv {
		t.Fatalf("Vorlage: %+v", glas)
	}
	off := decodeJSON[workCenterResponse](t, do(http.MethodPut, workCentersPath+mo.ID.String(), `{"name":"Montage","stunden_je_tag":6,"aktiv":false}`), http.StatusOK)
	if off.Aktiv || off.StundenJeTag != 6 {
		t.Fatalf("deaktivieren: %+v", off)
	}
	if still := decodeJSON[workCenterResponse](t, do(http.MethodPut, workCentersPath+mo.ID.String(), `{"name":"Montage","stunden_je_tag":6}`), http.StatusOK); still.Aktiv {
		t.Fatal("PUT ohne aktiv hat wieder aktiviert")
	}
	if got := workCenterNames(decodeJSON[[]workCenterResponse](t, do(http.MethodGet, workCentersPath, ""), http.StatusOK)); got != "Zuschnitt" {
		t.Fatalf("Liste aktiv: %s", got)
	}
	if got := workCenterNames(decodeJSON[[]workCenterResponse](t, do(http.MethodGet, workCentersPath+"?include_inactive=true", ""), http.StatusOK)); got != "Montage,Zuschnitt" {
		t.Fatalf("Liste mit deaktivierten: %s", got)
	}

	// Vorlagen
	saege := decodeJSON[opTemplateResponse](t, do(http.MethodPost, opTemplatesPath,
		`{"name":"Sägen","work_center_id":"`+zu.ID.String()+`","reihenfolge":1,"minuten_je_stueck":4.5}`), http.StatusCreated)
	// alphabetisch vorn, in der Reihenfolge in der Mitte
	decodeJSON[opTemplateResponse](t, do(http.MethodPost, opTemplatesPath,
		`{"name":"Abkanten","work_center_id":"`+zu.ID.String()+`","reihenfolge":2}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, opTemplatesPath, `{"name":"Kleben","work_center_id":"`+mo.ID.String()+`","reihenfolge":2}`),
		http.StatusConflict, "conflict", "Arbeitsplatz ist deaktiviert")
	wantAPIError(t, do(http.MethodPost, opTemplatesPath, `{"name":"Kleben","work_center_id":"`+uuid.NewString()+`","reihenfolge":2}`),
		http.StatusNotFound, "not_found", "Arbeitsplatz nicht gefunden")
	wantAPIError(t, do(http.MethodPost, opTemplatesPath, `{"name":"SÄGEN","work_center_id":"`+zu.ID.String()+`","reihenfolge":2}`),
		http.StatusBadRequest, "validation_error", "Arbeitsgangvorlage mit diesem Namen bereits vorhanden")
	wantAPIError(t, do(http.MethodPost, opTemplatesPath, `{"name":"Kleben","work_center_id":"`+zu.ID.String()+`","reihenfolge":0}`),
		http.StatusBadRequest, "validation_error", "Reihenfolge muss größer als 0 sein")
	wantAPIError(t, do(http.MethodPost, opTemplatesPath, `{"name":"Kleben","work_center_id":"`+zu.ID.String()+`","reihenfolge":2,"ruestzeit_min":-1}`),
		http.StatusBadRequest, "validation_error", "Zeiten dürfen nicht negativ sein")
	wantAPIError(t, do(http.MethodPost, opTemplatesPath, `{"name":"Kleben","reihenfolge":2}`),
		http.StatusBadRequest, "validation_error", "Arbeitsplatz erforderlich")

	// Verglasung bleibt auf dem deaktivierten Arbeitsplatz aenderbar;
	// Saegen darf nicht dorthin wechseln.
	g2 := decodeJSON[opTemplateResponse](t, do(http.MethodPut, opTemplatesPath+glas.ID.String(),
		`{"name":"Verglasung","work_center_id":"`+mo.ID.String()+`","reihenfolge":3,"ruestzeit_min":5,"minuten_je_stueck":25}`), http.StatusOK)
	if g2.RuestzeitMin != 5 || g2.MinutenJeStueck != 25 || !g2.Aktiv {
		t.Fatalf("Vorlage ändern: %+v", g2)
	}
	wantAPIError(t, do(http.MethodPut, opTemplatesPath+saege.ID.String(), `{"name":"Sägen","work_center_id":"`+mo.ID.String()+`","reihenfolge":1}`),
		http.StatusConflict, "conflict", "Arbeitsplatz ist deaktiviert")
	wantAPIError(t, do(http.MethodPut, opTemplatesPath+uuid.NewString(), `{"name":"x","work_center_id":"`+zu.ID.String()+`","reihenfolge":1}`),
		http.StatusNotFound, "not_found", "Arbeitsgangvorlage nicht gefunden")
	if got := templateNames(decodeJSON[[]opTemplateResponse](t, do(http.MethodGet, opTemplatesPath, ""), http.StatusOK)); got != "Sägen,Abkanten,Verglasung" {
		t.Fatalf("Vorlagen in Reihenfolge: %s", got)
	}
	decodeJSON[opTemplateResponse](t, do(http.MethodPut, opTemplatesPath+glas.ID.String(),
		`{"name":"Verglasung","work_center_id":"`+mo.ID.String()+`","reihenfolge":3,"aktiv":false}`), http.StatusOK)
	if got := templateNames(decodeJSON[[]opTemplateResponse](t, do(http.MethodGet, opTemplatesPath, ""), http.StatusOK)); got != "Sägen,Abkanten" {
		t.Fatalf("Vorlagen aktiv: %s", got)
	}
	if got := templateNames(decodeJSON[[]opTemplateResponse](t, do(http.MethodGet, opTemplatesPath+"?include_inactive=true", ""), http.StatusOK)); got != "Sägen,Abkanten,Verglasung" {
		t.Fatalf("Vorlagen mit deaktivierten: %s", got)
	}
}

func TestProductionMasterDataHTTPPermissionsAndTenants(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pr-readonly", "production.read")
	seedRoleWithPermissions(t, env, "itest-pr-writeonly", "production.write")
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	wc := decodeJSON[workCenterResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, workCentersPath, `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusCreated)
	tpl := decodeJSON[opTemplateResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, opTemplatesPath,
		`{"name":"Sägen","work_center_id":"`+wc.ID.String()+`","reihenfolge":1}`), http.StatusCreated)

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
			n := uuid.NewString()[:6]
			for _, ch := range []struct {
				method, path, body string
				allowed            bool
				ok                 int
			}{
				{http.MethodGet, workCentersPath, "", tc.canRead, http.StatusOK},
				{http.MethodGet, opTemplatesPath, "", tc.canRead, http.StatusOK},
				{http.MethodPost, workCentersPath, `{"name":"WC ` + n + `","stunden_je_tag":8}`, tc.canWrite, http.StatusCreated},
				{http.MethodPut, workCentersPath + wc.ID.String(), `{"name":"Zuschnitt","stunden_je_tag":9}`, tc.canWrite, http.StatusOK},
				{http.MethodPost, opTemplatesPath, `{"name":"T ` + n + `","work_center_id":"` + wc.ID.String() + `","reihenfolge":2}`, tc.canWrite, http.StatusCreated},
				{http.MethodPut, opTemplatesPath + tpl.ID.String(), `{"name":"Sägen","work_center_id":"` + wc.ID.String() + `","reihenfolge":1,"minuten_je_stueck":3}`, tc.canWrite, http.StatusOK},
			} {
				rec := doAuthedRequest(t, handler, token, ch.method, ch.path, ch.body)
				if !ch.allowed {
					wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				} else if rec.Code != ch.ok {
					t.Fatalf("%s %s: %d %s", ch.method, ch.path, rec.Code, rec.Body.String())
				}
			}
			if !tc.canWrite {
				for _, x := range decodeJSON[[]workCenterResponse](t, doAuthedRequest(t, handler, prod, http.MethodGet, workCentersPath, ""), http.StatusOK) {
					if strings.HasSuffix(x.Name, n) {
						t.Fatalf("verbotenes Anlegen hat gewirkt: %+v", x)
					}
				}
			}
		})
	}

	// Anderer Mandant: sieht nichts, aendert nichts, kann nicht auf den
	// fremden Arbeitsplatz verweisen.
	two := newCatalogTenant(t, env, handler).user("admin")
	wantAPIError(t, doAuthedRequest(t, handler, two, http.MethodPut, workCentersPath+wc.ID.String(), `{"name":"x","stunden_je_tag":1}`),
		http.StatusNotFound, "not_found", "Arbeitsplatz nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, two, http.MethodPut, opTemplatesPath+tpl.ID.String(), `{"name":"x","work_center_id":"`+wc.ID.String()+`","reihenfolge":1}`),
		http.StatusNotFound, "not_found", "Arbeitsgangvorlage nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, two, http.MethodPost, opTemplatesPath, `{"name":"Fremd","work_center_id":"`+wc.ID.String()+`","reihenfolge":1}`),
		http.StatusNotFound, "not_found", "Arbeitsplatz nicht gefunden")
	if list := decodeJSON[[]workCenterResponse](t, doAuthedRequest(t, handler, two, http.MethodGet, workCentersPath+"?include_inactive=true", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("fremde Arbeitsplätze sichtbar: %+v", list)
	}
	if list := decodeJSON[[]opTemplateResponse](t, doAuthedRequest(t, handler, two, http.MethodGet, opTemplatesPath+"?include_inactive=true", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("fremde Vorlagen sichtbar: %+v", list)
	}
	// Gleicher Name im anderen Mandanten ist erlaubt.
	decodeJSON[workCenterResponse](t, doAuthedRequest(t, handler, two, http.MethodPost, workCentersPath, `{"name":"Zuschnitt","stunden_je_tag":8}`), http.StatusCreated)
	var hours float64
	if err := env.PG.QueryRow(t.Context(), `SELECT stunden_je_tag::float8 FROM production_work_centers WHERE id=$1`, wc.ID).Scan(&hours); err != nil || hours != 9 {
		t.Fatalf("Arbeitsplatz verändert: %v %v", hours, err)
	}
}

// Jeder Fehlertext der Stammdaten muss als 4xx klassifiziert werden.
func TestClassifyDomainErrorCoversProductionMasterDataMessages(t *testing.T) {
	for msg, want := range map[string]int{
		"Name des Arbeitsplatzes erforderlich":                  http.StatusBadRequest,
		"Ungültige Stunden je Tag (größer als 0, höchstens 24)": http.StatusBadRequest,
		"Name des Arbeitsgangs erforderlich":                    http.StatusBadRequest,
		"Arbeitsplatz erforderlich":                             http.StatusBadRequest,
		"Reihenfolge muss größer als 0 sein":                    http.StatusBadRequest,
		"Zeiten dürfen nicht negativ sein":                      http.StatusBadRequest,
		"Zeit ist zu groß":                                      http.StatusBadRequest,
		"Arbeitsplatz mit diesem Namen bereits vorhanden":       http.StatusBadRequest,
		"Arbeitsgangvorlage mit diesem Namen bereits vorhanden": http.StatusBadRequest,
		"Arbeitsplatz nicht gefunden":                           http.StatusNotFound,
		"Arbeitsgangvorlage nicht gefunden":                     http.StatusNotFound,
		"Arbeitsplatz ist deaktiviert":                          http.StatusConflict,
	} {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
