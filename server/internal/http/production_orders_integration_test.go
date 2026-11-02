package apihttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Fertigungsauftraege ueber HTTP (Backlog H.1.3). Helfer aus
// qualification_types_integration_test.go (newCatalogTenant),
// leave_requests_integration_test.go (seedRoleWithPermissions) und
// asset_inspections_integration_test.go (decodeJSON).

const productionOrdersPath = "/api/v1/production/orders/"

type productionOrderResponse struct {
	ID                 uuid.UUID `json:"id"`
	Nummer             string    `json:"nummer"`
	ProjectID          uuid.UUID `json:"project_id"`
	Stueckzahl         int       `json:"stueckzahl"`
	Liefertermin       *string   `json:"liefertermin"`
	Ausfuehrungsklasse *string   `json:"ausfuehrungsklasse"`
	Notiz              string    `json:"notiz"`
	Status             string    `json:"status"`
	PositionNummer     string    `json:"position_nummer"`
	StornoReason       string    `json:"storno_reason"`
	FreigegebenByUser  *string   `json:"freigegeben_by_user_id"`
	Hinweise           []string  `json:"hinweise"`
	Items              []struct {
		Pos         int     `json:"pos"`
		Art         string  `json:"art"`
		MengeGesamt float64 `json:"menge_gesamt"`
	} `json:"items"`
}

type productionPosition struct {
	project, elevation, variant uuid.UUID
}

// seedProductionPosition legt im Mandanten ein Projekt mit Phase,
// Position (Menge 10) und Variante mit einem Profil an.
func seedProductionPosition(t *testing.T, env *testutil.IntegrationEnv, company string) productionPosition {
	t.Helper()
	ctx := context.Background()
	p := productionPosition{project: uuid.New(), elevation: uuid.New(), variant: uuid.New()}
	phase := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, 'P-FA', 'Fertigungsprojekt', $2)`, []any{p.project, company}},
		{`INSERT INTO project_phases (id, project_id, nummer, name) VALUES ($1, $2, '1', 'Haus A')`, []any{phase, p.project}},
		{`INSERT INTO project_elevations (id, phase_id, nummer, name, menge) VALUES ($1, $2, '3', 'Fenster', 10)`, []any{p.elevation, phase}},
		{`INSERT INTO project_single_elevations (id, elevation_id, name, menge) VALUES ($1, $2, 'links', 10)`, []any{p.variant, p.elevation}},
		{`INSERT INTO single_elevation_profiles (id, single_elevation_id, article_code, length_mm, qty) VALUES ($1, $2, 'P-1', 1200, 2)`, []any{uuid.New(), p.variant}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("Testdaten: %v", err)
		}
	}
	return p
}

func (p productionPosition) body(stueck int, extra string) string {
	return `{"project_id":"` + p.project.String() + `","elevation_id":"` + p.elevation.String() + `","single_elevation_id":"` +
		p.variant.String() + `","stueckzahl":` + strconv.Itoa(stueck) + extra + `}`
}

func productionOrderNumbers(list []productionOrderResponse) string {
	out := make([]string, len(list))
	for i, o := range list {
		out[i] = o.Nummer[strings.LastIndex(o.Nummer, "-")+1:] + ":" + o.Status
	}
	return strings.Join(out, ",")
}

func TestProductionOrdersHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("production")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	p := seedProductionPosition(t, env, c.company)
	other := seedProductionPosition(t, env, c.company)

	a := decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath,
		p.body(4, `,"liefertermin":"2026-12-01","ausfuehrungsklasse":"exc2","notiz":" Los 1 "`)), http.StatusCreated)
	if !strings.HasPrefix(a.Nummer, "FA-") || a.Status != "entwurf" || a.Stueckzahl != 4 || a.Liefertermin == nil ||
		!strings.HasPrefix(*a.Liefertermin, "2026-12-01") || a.Ausfuehrungsklasse == nil || *a.Ausfuehrungsklasse != "EXC2" ||
		a.Notiz != "Los 1" || a.PositionNummer != "3" || len(a.Items) != 0 || len(a.Hinweise) != 1 || a.Hinweise[0] != "kein_kundenauftrag" {
		t.Fatalf("Entwurf: %+v", a)
	}

	// Eingaben
	for body, want := range map[string]struct {
		status    int
		code, msg string
	}{
		p.body(0, ""):     {http.StatusBadRequest, "validation_error", "Stückzahl muss größer als 0 sein"},
		p.body(10001, ""): {http.StatusBadRequest, "validation_error", "Stückzahl ist zu groß"},
		p.body(1, `,"ausfuehrungsklasse":"EXC5"`): {http.StatusBadRequest, "validation_error", "Ungültige Ausführungsklasse"},
		p.body(1, `,"liefertermin":"1.12.2026"`):  {http.StatusBadRequest, "validation_error", "Ungültiges Datum in liefertermin (erwartet JJJJ-MM-TT)"},
		`{"stueckzahl":1}`:                        {http.StatusBadRequest, "validation_error", "Projekt erforderlich"},
		`{`:                                       {http.StatusBadRequest, "validation_error", "Ungültige Eingabe"},
		productionPosition{uuid.New(), p.elevation, p.variant}.body(1, ""):        {http.StatusNotFound, "not_found", "Projekt nicht gefunden"},
		productionPosition{p.project, other.elevation, other.variant}.body(1, ""): {http.StatusBadRequest, "validation_error", "Position nicht im Projekt"},
		productionPosition{p.project, p.elevation, other.variant}.body(1, ""):     {http.StatusBadRequest, "validation_error", "Variante nicht an der Position"},
	} {
		wantAPIError(t, do(http.MethodPost, productionOrdersPath, body), want.status, want.code, want.msg)
	}

	// Entwurf aendern
	path := productionOrdersPath + a.ID.String()
	u := decodeJSON[productionOrderResponse](t, do(http.MethodPut, path, `{"stueckzahl":6,"liefertermin":"","ausfuehrungsklasse":"","notiz":"neu"}`), http.StatusOK)
	if u.Stueckzahl != 6 || u.Liefertermin != nil || u.Ausfuehrungsklasse != nil || u.Notiz != "neu" {
		t.Fatalf("geändert: %+v", u)
	}
	wantAPIError(t, do(http.MethodPut, path, `{"stueckzahl":0}`), http.StatusBadRequest, "validation_error", "Stückzahl muss größer als 0 sein")
	wantAPIError(t, do(http.MethodPut, path, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPut, path, `{"stueckzahl":1,"liefertermin":"morgen"}`), http.StatusBadRequest, "validation_error", "Ungültiges Datum in liefertermin (erwartet JJJJ-MM-TT)")
	if d := decodeJSON[productionOrderResponse](t, do(http.MethodPut, path, `{"stueckzahl":6,"liefertermin":"2027-01-15","notiz":"neu"}`), http.StatusOK); d.Liefertermin == nil ||
		!strings.HasPrefix(*d.Liefertermin, "2027-01-15") {
		t.Fatalf("Liefertermin gesetzt: %+v", d)
	}

	// Ablauf
	wantAPIError(t, do(http.MethodPost, path+"/beginnen", ""), http.StatusConflict, "conflict", "Aktion im aktuellen Status nicht erlaubt")
	r := decodeJSON[productionOrderResponse](t, do(http.MethodPost, path+"/freigeben", ""), http.StatusOK)
	if r.Status != "freigegeben" || r.FreigegebenByUser == nil || len(r.Items) != 1 || r.Items[0].Art != "profil" || r.Items[0].MengeGesamt != 12 {
		t.Fatalf("freigegeben: %+v", r)
	}
	wantAPIError(t, do(http.MethodPost, path+"/freigeben", ""), http.StatusConflict, "conflict", "Aktion im aktuellen Status nicht erlaubt")
	wantAPIError(t, do(http.MethodPut, path, `{"stueckzahl":1}`), http.StatusConflict, "conflict", "Nur ein Entwurf kann geändert werden")
	if g := decodeJSON[productionOrderResponse](t, do(http.MethodGet, path, ""), http.StatusOK); g.Status != "freigegeben" || len(g.Items) != 1 {
		t.Fatalf("Einzelabruf: %+v", g)
	}
	if s := decodeJSON[productionOrderResponse](t, do(http.MethodPost, path+"/beginnen", ""), http.StatusOK); s.Status != "in_arbeit" {
		t.Fatalf("beginnen: %+v", s)
	}
	if f := decodeJSON[productionOrderResponse](t, do(http.MethodPost, path+"/fertigmelden", ""), http.StatusOK); f.Status != "fertig" {
		t.Fatalf("fertig: %+v", f)
	}
	wantAPIError(t, do(http.MethodPost, path+"/storno", `{"reason":"x"}`), http.StatusConflict, "conflict", "Ein fertiger Auftrag kann nicht storniert werden")

	// Storno
	b := decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated)
	bPath := productionOrdersPath + b.ID.String()
	wantAPIError(t, do(http.MethodPost, bPath+"/storno", `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Begründung für das Storno erforderlich")
	wantAPIError(t, do(http.MethodPost, bPath+"/storno", `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	if x := decodeJSON[productionOrderResponse](t, do(http.MethodPost, bPath+"/storno", `{"reason":"doppelt"}`), http.StatusOK); x.Status != "storniert" || x.StornoReason != "doppelt" {
		t.Fatalf("Storno: %+v", x)
	}
	wantAPIError(t, do(http.MethodPost, bPath+"/storno", `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Auftrag ist bereits storniert")

	// Re-Import entfernt die Variante: Freigabe eines Entwurfs scheitert.
	d := decodeJSON[productionOrderResponse](t, do(http.MethodPost, productionOrdersPath, other.body(1, `,"liefertermin":"2026-11-20"`)), http.StatusCreated)
	if _, err := env.PG.Exec(context.Background(), `DELETE FROM project_single_elevations WHERE id=$1`, other.variant); err != nil {
		t.Fatal(err)
	}
	wantAPIError(t, do(http.MethodPost, productionOrdersPath+d.ID.String()+"/freigeben", ""), http.StatusConflict, "conflict",
		"Position oder Variante ist nicht mehr im Projekt")

	// Liste und Filter
	list := func(query string) string {
		return productionOrderNumbers(decodeJSON[[]productionOrderResponse](t, do(http.MethodGet, productionOrdersPath+query, ""), http.StatusOK))
	}
	aN, dN := a.Nummer[len(a.Nummer)-4:], d.Nummer[len(d.Nummer)-4:]
	bN := b.Nummer[len(b.Nummer)-4:]
	for query, want := range map[string]string{
		"":                                      dN + ":entwurf," + aN + ":fertig",
		"?include_storniert=true":               dN + ":entwurf," + aN + ":fertig," + bN + ":storniert",
		"?status=Storniert":                     bN + ":storniert",
		"?status=fertig":                        aN + ":fertig",
		"?project_id=" + other.project.String(): dN + ":entwurf",
		"?elevation_id=" + p.elevation.String(): aN + ":fertig",
		"?project_id=" + uuid.NewString():       "",
	} {
		if got := list(query); got != want {
			t.Fatalf("Liste %q: %s, erwartet %s", query, got, want)
		}
	}
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+"?status=geloescht", ""), http.StatusBadRequest, "validation_error", "Ungültiger Status")
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+"?project_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige project_id")
	wantAPIError(t, do(http.MethodGet, productionOrdersPath+"?elevation_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige elevation_id")

	// unbekannt und kaputte ID
	for _, ch := range []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""}, {http.MethodPut, "", `{"stueckzahl":1}`}, {http.MethodPost, "/freigeben", ""},
		{http.MethodPost, "/beginnen", ""}, {http.MethodPost, "/fertigmelden", ""}, {http.MethodPost, "/storno", `{"reason":"x"}`},
	} {
		wantAPIError(t, do(ch.method, productionOrdersPath+uuid.NewString()+ch.suffix, ch.body), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
		wantAPIError(t, do(ch.method, productionOrdersPath+"kein-uuid"+ch.suffix, ch.body), http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
	}
}

// Lesen mit production.read, schreiben mit production.write; dazu die
// Systemrollen aus Migration 107.
func TestProductionOrdersHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-pr-readonly", "production.read")
	seedRoleWithPermissions(t, env, "itest-pr-writeonly", "production.write")
	c := newCatalogTenant(t, env, handler)
	prodToken := c.user("production")
	p := seedProductionPosition(t, env, c.company)
	x := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prodToken, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated)

	for name, tc := range map[string]struct {
		role              string
		canRead, canWrite bool
	}{
		"nur read":   {"itest-pr-readonly", true, false},
		"nur write":  {"itest-pr-writeonly", false, true},
		"Vertrieb":   {"sales", true, false},
		"Personal":   {"hr", false, false},
		"Fuhrpark":   {"fleet", false, false},
		"Lager":      {"inventory", false, false},
		"Produktion": {"production", true, true},
		"Admin":      {"admin", true, true},
	} {
		t.Run(name, func(t *testing.T) {
			token := c.user(tc.role)
			y := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prodToken, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated)
			yPath := productionOrdersPath + y.ID.String()
			checks := []struct {
				method, path, body string
				allowed            bool
				ok                 int
			}{
				{http.MethodGet, productionOrdersPath, "", tc.canRead, http.StatusOK},
				{http.MethodGet, productionOrdersPath + x.ID.String(), "", tc.canRead, http.StatusOK},
				{http.MethodPost, productionOrdersPath, p.body(1, ""), tc.canWrite, http.StatusCreated},
				{http.MethodPut, yPath, `{"stueckzahl":2}`, tc.canWrite, http.StatusOK},
				{http.MethodPost, yPath + "/freigeben", "", tc.canWrite, http.StatusOK},
				{http.MethodPost, yPath + "/beginnen", "", tc.canWrite, http.StatusOK},
				{http.MethodPost, yPath + "/fertigmelden", "", tc.canWrite, http.StatusOK},
			}
			for _, ch := range checks {
				rec := doAuthedRequest(t, handler, token, ch.method, ch.path, ch.body)
				if !ch.allowed {
					wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				} else if rec.Code != ch.ok {
					t.Fatalf("%s %s: %d %s", ch.method, ch.path, rec.Code, rec.Body.String())
				}
			}
			z := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prodToken, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated)
			rec := doAuthedRequest(t, handler, token, http.MethodPost, productionOrdersPath+z.ID.String()+"/storno", `{"reason":"x"}`)
			want := "storniert"
			if !tc.canWrite {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				want = "entwurf"
			} else if rec.Code != http.StatusOK {
				t.Fatalf("Storno: %d %s", rec.Code, rec.Body.String())
			}
			if got := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prodToken, http.MethodGet, productionOrdersPath+z.ID.String(), ""), http.StatusOK); got.Status != want {
				t.Fatalf("Storno-Status: %s, erwartet %s", got.Status, want)
			}
			if !tc.canWrite {
				if got := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prodToken, http.MethodGet, yPath, ""), http.StatusOK); got.Status != "entwurf" || got.Stueckzahl != 1 {
					t.Fatalf("verbotene Änderung hat gewirkt: %+v", got)
				}
			}
		})
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, productionOrdersPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestProductionOrdersHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("production"), two.user("admin")
	p := seedProductionPosition(t, env, one.company)
	x := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusCreated)
	xPath := productionOrdersPath + x.ID.String()

	for _, ch := range []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""}, {http.MethodPut, "", `{"stueckzahl":2}`}, {http.MethodPost, "/freigeben", ""},
		{http.MethodPost, "/storno", `{"reason":"x"}`},
	} {
		wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, ch.method, xPath+ch.suffix, ch.body), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
	}
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, productionOrdersPath, p.body(1, "")), http.StatusNotFound, "not_found", "Projekt nicht gefunden")
	if list := decodeJSON[[]productionOrderResponse](t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, productionOrdersPath+"?include_storniert=true", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("Liste des anderen Mandanten: %+v", list)
	}
	if still := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, xPath, ""), http.StatusOK); still.Status != "entwurf" || still.Stueckzahl != 1 {
		t.Fatalf("anderer Mandant hat geändert: %+v", still)
	}
}

// Jeder Fehlertext der Fertigungsauftraege muss als 4xx klassifiziert
// werden. DB-los.
func TestClassifyDomainErrorCoversProductionOrderMessages(t *testing.T) {
	cases := map[string]int{
		"Mandant erforderlich":                               http.StatusBadRequest,
		"Projekt erforderlich":                               http.StatusBadRequest,
		"Position erforderlich":                              http.StatusBadRequest,
		"Variante erforderlich":                              http.StatusBadRequest,
		"Stückzahl muss größer als 0 sein":                   http.StatusBadRequest,
		"Stückzahl ist zu groß":                              http.StatusBadRequest,
		"Ungültige Ausführungsklasse":                        http.StatusBadRequest,
		"Begründung für das Storno erforderlich":             http.StatusBadRequest,
		"Position nicht im Projekt":                          http.StatusBadRequest,
		"Variante nicht an der Position":                     http.StatusBadRequest,
		"Ungültige Materialart":                              http.StatusBadRequest,
		"Ungültige Materialmenge":                            http.StatusBadRequest,
		"Projekt nicht gefunden":                             http.StatusNotFound,
		"Fertigungsauftrag nicht gefunden":                   http.StatusNotFound,
		"Aktion im aktuellen Status nicht erlaubt":           http.StatusConflict,
		"Ein fertiger Auftrag kann nicht storniert werden":   http.StatusConflict,
		"Auftrag ist bereits storniert":                      http.StatusConflict,
		"Nur ein Entwurf kann geändert werden":               http.StatusConflict,
		"Position oder Variante ist nicht mehr im Projekt":   http.StatusConflict,
		"Position ohne Nummer kann nicht freigegeben werden": http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
