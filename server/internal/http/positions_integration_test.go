package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Stellen als Vorlagen (Backlog F.11.2, ADR 0031). Helfer aus
// qualification_types_integration_test.go (newCatalogTenant, qualType) und
// leave_requests_integration_test.go (seedRoleWithPermissions).

const positionsPath = "/api/v1/positions/"

type positionResponse struct {
	ID      uuid.UUID   `json:"id"`
	Code    string      `json:"code"`
	Name    string      `json:"name"`
	Aktiv   bool        `json:"aktiv"`
	Hinweis string      `json:"hinweis"`
	TypeIDs []uuid.UUID `json:"type_ids"`
}

func decodePosition(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) positionResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out positionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodePositions(t *testing.T, rec *httptest.ResponseRecorder) []positionResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []positionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func sortedIDs(ids ...uuid.UUID) []uuid.UUID {
	out := append([]uuid.UUID(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func idsJSON(ids ...uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = `"` + id.String() + `"`
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func sameIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPositionsCreateListUpdate(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	eh := c.qualType(token, "EH", "24")
	schweiss := c.qualType(token, "SCHWEISS", "36")
	brand := c.qualType(token, "BRAND", "36")

	// Code wird normalisiert, doppelte Arten zaehlen einmal, sortiert.
	welder := decodePosition(t, do(http.MethodPost, positionsPath,
		`{"code":" schweisser ","name":" Schweißer ","hinweis":"Werkstatt","type_ids":`+idsJSON(schweiss.ID, brand.ID, schweiss.ID)+`}`), http.StatusCreated)
	if welder.Code != "SCHWEISSER" || welder.Name != "Schweißer" || !welder.Aktiv || welder.Hinweis != "Werkstatt" || !sameIDs(welder.TypeIDs, sortedIDs(schweiss.ID, brand.ID)) {
		t.Fatalf("angelegt: %+v", welder)
	}
	// Ohne Arten: leere Liste, nicht null.
	rec := do(http.MethodPost, positionsPath, `{"code":"BUERO","name":"Bürokraft"}`)
	office := decodePosition(t, rec, http.StatusCreated)
	if office.TypeIDs == nil || len(office.TypeIDs) != 0 || !strings.Contains(rec.Body.String(), `"type_ids":[]`) {
		t.Fatalf("ohne Arten: %s", rec.Body.String())
	}

	got := decodePosition(t, do(http.MethodGet, positionsPath+welder.ID.String(), ""), http.StatusOK)
	if got.ID != welder.ID || !sameIDs(got.TypeIDs, welder.TypeIDs) {
		t.Fatalf("Einzelabruf: %+v", got)
	}
	// Nach Name sortiert (ohne Gross-/Kleinschreibung).
	list := decodePositions(t, do(http.MethodGet, positionsPath, ""))
	if len(list) != 2 || list[0].Code != "BUERO" || list[1].Code != "SCHWEISSER" {
		t.Fatalf("Liste: %+v", list)
	}

	// PUT ersetzt alles: neue Arten, Name, Hinweis.
	upd := decodePosition(t, do(http.MethodPut, positionsPath+welder.ID.String(),
		`{"name":"Schweißer (MAG)","aktiv":true,"hinweis":"","type_ids":`+idsJSON(eh.ID, schweiss.ID)+`}`), http.StatusOK)
	if upd.Name != "Schweißer (MAG)" || upd.Hinweis != "" || upd.Code != "SCHWEISSER" || !sameIDs(upd.TypeIDs, sortedIDs(eh.ID, schweiss.ID)) {
		t.Fatalf("geaendert: %+v", upd)
	}
	// Eine zugeordnete Art wird stillgelegt: die Stelle bleibt speicherbar.
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_qualification_types SET aktiv=false WHERE id=$1`, eh.ID); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}
	kept := decodePosition(t, do(http.MethodPut, positionsPath+welder.ID.String(),
		`{"name":"Schweißer (MAG)","aktiv":true,"hinweis":"x","type_ids":`+idsJSON(eh.ID, schweiss.ID)+`}`), http.StatusOK)
	if !sameIDs(kept.TypeIDs, sortedIDs(eh.ID, schweiss.ID)) {
		t.Fatalf("stillgelegte Art entfernt: %+v", kept)
	}
	// Neu hinzufuegen laesst sie sich nicht.
	wantAPIError(t, do(http.MethodPut, positionsPath+office.ID.String(), `{"name":"Bürokraft","aktiv":true,"hinweis":"","type_ids":`+idsJSON(eh.ID)+`}`),
		http.StatusConflict, "conflict", "Qualifikationsart ist stillgelegt")
	if still := decodePosition(t, do(http.MethodGet, positionsPath+office.ID.String(), ""), http.StatusOK); len(still.TypeIDs) != 0 {
		t.Fatalf("abgelehnte Aenderung teilweise gespeichert: %+v", still)
	}

	// Stilllegen: nur noch mit include_inactive in der Liste.
	decodePosition(t, do(http.MethodPut, positionsPath+office.ID.String(), `{"name":"Bürokraft","aktiv":false,"hinweis":"","type_ids":[]}`), http.StatusOK)
	if l := decodePositions(t, do(http.MethodGet, positionsPath, "")); len(l) != 1 || l[0].ID != welder.ID {
		t.Fatalf("aktive Liste: %+v", l)
	}
	if l := decodePositions(t, do(http.MethodGet, positionsPath+"?include_inactive=true", "")); len(l) != 2 {
		t.Fatalf("Liste mit stillgelegten: %+v", l)
	}
}

// Sortiert nach Name ohne Gross-/Kleinschreibung - die Daten sind so
// gewaehlt, dass Code, Name mit und Name ohne Schreibung verschieden
// sortieren.
func TestPositionsListOrder(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	for _, body := range []string{`{"code":"B_MONTEUR","name":"Monteur"}`, `{"code":"0_ZUSCHNITT","name":"Zuschnitt"}`, `{"code":"A_LAGER","name":"lagerist"}`} {
		decodePosition(t, doAuthedRequest(t, handler, token, http.MethodPost, positionsPath, body), http.StatusCreated)
	}
	list := decodePositions(t, doAuthedRequest(t, handler, token, http.MethodGet, positionsPath, ""))
	got := make([]string, len(list))
	for i, p := range list {
		got[i] = p.Code
	}
	if strings.Join(got, ",") != "A_LAGER,B_MONTEUR,0_ZUSCHNITT" {
		t.Fatalf("Reihenfolge: %v", got)
	}
}

func TestPositionsValidationAndTenantIsolation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	typ := c.qualType(token, "EH", "24")
	stillgelegt := c.qualType(token, "ALT", "null")
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_qualification_types SET aktiv=false WHERE id=$1`, stillgelegt.ID); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}
	other := newCatalogTenant(t, env, handler)
	otherToken := other.user("hr")
	foreignType := other.qualType(otherToken, "FREMD", "null")
	foreignPos := decodePosition(t, doAuthedRequest(t, handler, otherToken, http.MethodPost, positionsPath, `{"code":"MONTEUR","name":"Monteur"}`), http.StatusCreated)

	pos := decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"MONTEUR","name":"Monteur"}`), http.StatusCreated)
	countBefore := func() int {
		var n int
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM hr_positions WHERE company_id=$1`, c.company).Scan(&n); err != nil {
			t.Fatalf("zaehlen: %v", err)
		}
		return n
	}
	before := countBefore()

	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code, msg                string
	}{
		{"Code fehlt", http.MethodPost, positionsPath, `{"name":"X"}`, 400, "validation_error", "Code erforderlich"},
		{"Code ungueltig", http.MethodPost, positionsPath, `{"code":"MONT EUR","name":"X"}`, 400, "validation_error", "Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)"},
		{"Name fehlt", http.MethodPost, positionsPath, `{"code":"X","name":"  "}`, 400, "validation_error", "Name erforderlich"},
		{"Code doppelt", http.MethodPost, positionsPath, `{"code":"monteur","name":"Anderer"}`, 400, "validation_error", "Stelle mit diesem Code existiert bereits"},
		{"Name doppelt, andere Schreibung", http.MethodPost, positionsPath, `{"code":"M2","name":"MONTEUR"}`, 400, "validation_error", "Stelle mit diesem Namen existiert bereits"},
		{"Art fremder Mandant", http.MethodPost, positionsPath, `{"code":"M3","name":"M3","type_ids":` + idsJSON(typ.ID, foreignType.ID) + `}`, 404, "not_found", "Qualifikationsart nicht gefunden"},
		{"Art unbekannt", http.MethodPost, positionsPath, `{"code":"M4","name":"M4","type_ids":` + idsJSON(uuid.New()) + `}`, 404, "not_found", "Qualifikationsart nicht gefunden"},
		{"Art stillgelegt", http.MethodPost, positionsPath, `{"code":"M5","name":"M5","type_ids":` + idsJSON(stillgelegt.ID) + `}`, 409, "conflict", "Qualifikationsart ist stillgelegt"},
		{"Art leer", http.MethodPost, positionsPath, `{"code":"M6","name":"M6","type_ids":["00000000-0000-0000-0000-000000000000"]}`, 400, "validation_error", "type_id erforderlich"},
		{"kein JSON", http.MethodPost, positionsPath, `{`, 400, "validation_error", "Ungültige Eingabe"},
		{"PUT ohne type_ids", http.MethodPut, positionsPath + pos.ID.String(), `{"name":"Monteur","aktiv":true,"hinweis":""}`, 400, "validation_error", "Feld erforderlich: type_ids"},
		{"PUT ohne aktiv", http.MethodPut, positionsPath + pos.ID.String(), `{"name":"Monteur","hinweis":"","type_ids":[]}`, 400, "validation_error", "Feld erforderlich: aktiv"},
		{"PUT mit Code", http.MethodPut, positionsPath + pos.ID.String(), `{"code":"NEU","name":"Monteur","aktiv":true,"hinweis":"","type_ids":[]}`, 400, "validation_error", "Der Code einer Stelle ist nicht änderbar"},
		{"PUT unbekanntes Feld", http.MethodPut, positionsPath + pos.ID.String(), `{"name":"Monteur","aktiv":true,"hinweis":"","type_ids":[],"farbe":"rot"}`, 400, "validation_error", "unbekanntes Feld: farbe"},
		{"PUT Name leer", http.MethodPut, positionsPath + pos.ID.String(), `{"name":"","aktiv":true,"hinweis":"","type_ids":[]}`, 400, "validation_error", "Name erforderlich"},
		{"PUT ungueltige ID", http.MethodPut, positionsPath + "abc", `{}`, 400, "validation_error", "Ungültige Stellen-ID"},
		{"PUT unbekannt", http.MethodPut, positionsPath + uuid.NewString(), `{"name":"X","aktiv":true,"hinweis":"","type_ids":[]}`, 404, "not_found", "Stelle nicht gefunden"},
		{"PUT fremde Stelle", http.MethodPut, positionsPath + foreignPos.ID.String(), `{"name":"Gekapert","aktiv":true,"hinweis":"","type_ids":[]}`, 404, "not_found", "Stelle nicht gefunden"},
		{"GET fremde Stelle", http.MethodGet, positionsPath + foreignPos.ID.String(), "", 404, "not_found", "Stelle nicht gefunden"},
		{"GET ungueltige ID", http.MethodGet, positionsPath + "abc", "", 400, "validation_error", "Ungültige Stellen-ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantAPIError(t, do(tc.method, tc.path, tc.body), tc.status, tc.code, tc.msg)
		})
	}
	if got := countBefore(); got != before {
		t.Fatalf("nach Fehlern %d statt %d Stellen", got, before)
	}
	// Derselbe Name in einem anderen Mandanten ist erlaubt (siehe oben),
	// eine Umbenennung auf einen vorhandenen Namen nicht; die fremde Stelle
	// blieb bei allem unveraendert.
	decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"LAGER","name":"Lagerist"}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPut, positionsPath+pos.ID.String(), `{"name":"lagerist","aktiv":true,"hinweis":"","type_ids":[]}`), 400, "validation_error", "Stelle mit diesem Namen existiert bereits")
	if g := decodePosition(t, doAuthedRequest(t, handler, otherToken, http.MethodGet, positionsPath+foreignPos.ID.String(), ""), http.StatusOK); g.Name != "Monteur" {
		t.Fatalf("fremde Stelle veraendert: %+v", g)
	}
}

func TestPositionsPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	pos := decodePosition(t, doAuthedRequest(t, handler, c.user("hr"), http.MethodPost, positionsPath, `{"code":"MONTEUR","name":"Monteur"}`), http.StatusCreated)
	seedRoleWithPermissions(t, env, "itest-pos-none", "employees.read")
	seedRoleWithPermissions(t, env, "itest-pos-read", "qualifications.read")

	// Lesen mit jedem Qualifikationsrecht, auch self (production).
	for _, role := range []string{"production", "itest-pos-read"} {
		token := c.user(role)
		if l := decodePositions(t, doAuthedRequest(t, handler, token, http.MethodGet, positionsPath, "")); len(l) != 1 {
			t.Fatalf("%s: Liste %+v", role, l)
		}
		decodePosition(t, doAuthedRequest(t, handler, token, http.MethodGet, positionsPath+pos.ID.String(), ""), http.StatusOK)
		wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPost, positionsPath, `{"code":"X","name":"X"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodPut, positionsPath+pos.ID.String(), `{"name":"X","aktiv":false,"hinweis":"","type_ids":[]}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}
	none := c.user("itest-pos-none")
	wantAPIError(t, doAuthedRequest(t, handler, none, http.MethodGet, positionsPath, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, none, http.MethodGet, positionsPath+pos.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if still := decodePosition(t, doAuthedRequest(t, handler, c.user("hr"), http.MethodGet, positionsPath+pos.ID.String(), ""), http.StatusOK); still.Name != "Monteur" || !still.Aktiv {
		t.Fatalf("ohne Recht geaendert: %+v", still)
	}
}

func TestClassifyDomainErrorCoversPositionMessages(t *testing.T) {
	cases := map[string]int{
		"Stelle nicht gefunden":                     http.StatusNotFound,
		"Stelle mit diesem Code existiert bereits":  http.StatusBadRequest,
		"Stelle mit diesem Namen existiert bereits": http.StatusBadRequest,
		"Ungültige Stellen-ID":                      http.StatusBadRequest,
		"type_id erforderlich":                      http.StatusBadRequest,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}

type adoptedPositionResponse struct {
	positionResponse
	FehlendeArten []string `json:"fehlende_arten"`
}

func decodeAdoptedPositions(t *testing.T, rec *httptest.ResponseRecorder) map[string]adoptedPositionResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var list []adoptedPositionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	out := map[string]adoptedPositionResponse{}
	for _, p := range list {
		out[p.Code] = p
	}
	return out
}

// Vorschlagskatalog (F.11.4): ohne Qualifikationskatalog entstehen die
// Stellen ohne Arten und nennen die fehlenden; ein zweiter Aufruf legt
// nichts an.
func TestPositionsAdoptDefaultsWithoutQualificationCatalog(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	got := decodeAdoptedPositions(t, doAuthedRequest(t, handler, token, http.MethodPost, positionsPath+"adopt-defaults", ""))
	if len(got) != 6 {
		t.Fatalf("erwartet 6 Stellen, bekommen %d", len(got))
	}
	kran := got["KRANFUEHRER"]
	if len(kran.TypeIDs) != 0 || strings.Join(kran.FehlendeArten, ",") != "KRANFUEHRER,ANSCHLAEGER" || kran.Name != "Kranführer" || !kran.Aktiv || !strings.HasPrefix(kran.Hinweis, "Vorschlag, nicht recherchiert") {
		t.Fatalf("Kranführer: %+v", kran)
	}
	if again := decodeAdoptedPositions(t, doAuthedRequest(t, handler, token, http.MethodPost, positionsPath+"adopt-defaults", "")); len(again) != 0 {
		t.Fatalf("zweiter Aufruf legt an: %v", again)
	}
	if l := decodePositions(t, doAuthedRequest(t, handler, token, http.MethodGet, positionsPath, "")); len(l) != 6 {
		t.Fatalf("Liste: %d", len(l))
	}
}

// Mit Qualifikationskatalog: Arten ueber den Code, nur aktive; was der
// Mandant schon hat (Code oder Name), bleibt unberuehrt.
func TestPositionsAdoptDefaultsMapsTypesAndKeepsExisting(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	token := c.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	types := map[string]uuid.UUID{}
	for _, qt := range decodeQualificationTypeList(t, do(http.MethodPost, qtPath+"adopt-defaults", "")) {
		types[qt.Code] = qt.ID
	}
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_qualification_types SET aktiv=false WHERE id=$1`, types["GEFAHRSTOFFE"]); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}
	ownLager := decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"LAGERIST","name":"Lager (eigen)"}`), http.StatusCreated)
	ownMonteur := decodePosition(t, do(http.MethodPost, positionsPath, `{"code":"EIGEN","name":"monteur","type_ids":`+idsJSON(types["ERSTHELFER"])+`}`), http.StatusCreated)
	// Ein anderer Mandant hat schon alles - das zaehlt hier nicht.
	other := newCatalogTenant(t, env, handler)
	decodeAdoptedPositions(t, doAuthedRequest(t, handler, other.user("hr"), http.MethodPost, positionsPath+"adopt-defaults", ""))

	got := decodeAdoptedPositions(t, do(http.MethodPost, positionsPath+"adopt-defaults", ""))
	if len(got) != 4 {
		t.Fatalf("erwartet SCHWEISSER, STAPLERFAHRER, KRANFUEHRER, FAHRER, bekommen %v", got)
	}
	// Fahrer: die Fuehrerscheinkontrolle aus dem Qualifikationskatalog.
	if f := got["FAHRER"]; !sameIDs(f.TypeIDs, []uuid.UUID{types["FUEHRERSCHEINKONTROLLE"]}) || len(f.FehlendeArten) != 0 {
		t.Fatalf("Fahrer: %+v", f)
	}
	welder := got["SCHWEISSER"]
	if !sameIDs(welder.TypeIDs, sortedIDs(types["SCHWEISSERPRUEFUNG"], types["BRANDSCHUTZHELFER"])) || strings.Join(welder.FehlendeArten, ",") != "GEFAHRSTOFFE" {
		t.Fatalf("Schweißer: %+v", welder)
	}
	if st := got["STAPLERFAHRER"]; !sameIDs(st.TypeIDs, sortedIDs(types["STAPLER_SCHEIN"], types["STAPLER_UNTERWEISUNG"], types["STAPLER_BEAUFTRAGUNG"])) || len(st.FehlendeArten) != 0 || st.FehlendeArten == nil {
		t.Fatalf("Staplerfahrer: %+v", st)
	}
	for id, wantName := range map[uuid.UUID]string{ownLager.ID: "Lager (eigen)", ownMonteur.ID: "monteur"} {
		p := decodePosition(t, do(http.MethodGet, positionsPath+id.String(), ""), http.StatusOK)
		if p.Name != wantName {
			t.Fatalf("eigene Stelle veraendert: %+v", p)
		}
	}
	if p := decodePosition(t, do(http.MethodGet, positionsPath+ownLager.ID.String(), ""), http.StatusOK); len(p.TypeIDs) != 0 {
		t.Fatalf("eigene Stelle LAGERIST um Arten ergaenzt: %+v", p)
	}
	if p := decodePosition(t, do(http.MethodGet, positionsPath+ownMonteur.ID.String(), ""), http.StatusOK); !sameIDs(p.TypeIDs, []uuid.UUID{types["ERSTHELFER"]}) {
		t.Fatalf("eigene Stelle monteur veraendert: %+v", p)
	}
	if l := decodePositions(t, do(http.MethodGet, positionsPath, "")); len(l) != 6 {
		t.Fatalf("Liste: %d", len(l))
	}
}

func TestPositionsAdoptDefaultsNeedsWritePermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	seedRoleWithPermissions(t, env, "itest-pos-read", "qualifications.read")
	for _, role := range []string{"production", "itest-pos-read"} {
		wantAPIError(t, doAuthedRequest(t, handler, c.user(role), http.MethodPost, positionsPath+"adopt-defaults", ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM hr_positions WHERE company_id=$1`, c.company).Scan(&n); err != nil || n != 0 {
		t.Fatalf("ohne Recht angelegt: %d (%v)", n, err)
	}
}
