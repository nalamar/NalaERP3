package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung des Katalogs der Qualifikationsarten (Backlog F.2.5.1).
// Helfer (doAuthedRequest, wantAPIError, loginIntegrationUser,
// seedRoleWithPermissions) aus den Testdateien der Zeiterfassung und der
// Urlaubsantraege.
//
// Der Katalog ist mandantenweit, und der Vorschlagskatalog legt immer
// dieselben Codes an. Jeder Test arbeitet deshalb in eigenen, frisch
// angelegten Mandanten.

type catalogTenant struct {
	company string
	seq     int
	t       *testing.T
	env     *testutil.IntegrationEnv
	handler http.Handler
}

func newCatalogTenant(t *testing.T, env *testutil.IntegrationEnv, handler http.Handler) *catalogTenant {
	t.Helper()
	company := "qt-http-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1,$2)`, company, "Katalogtest "+company); err != nil {
		t.Fatalf("Mandant anlegen: %v", err)
	}
	return &catalogTenant{company: company, t: t, env: env, handler: handler}
}

// user legt einen Benutzer dieses Mandanten mit der Rolle an und meldet ihn an.
func (c *catalogTenant) user(role string) string {
	c.t.Helper()
	c.seq++
	email := c.company + "-" + role + "-" + uuid.NewString()[:6] + "@example.com"
	userID := testutil.SeedAuthUser(c.t, c.env, email, "Secret123!", role)
	if _, err := c.env.PG.Exec(context.Background(), `UPDATE users SET company_id=$2 WHERE id=$1`, userID, c.company); err != nil {
		c.t.Fatalf("Benutzer dem Mandanten zuordnen: %v", err)
	}
	return loginIntegrationUser(c.t, c.handler, email, "Secret123!")
}

type qualificationTypeResponse struct {
	ID                uuid.UUID `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	Kategorie         string    `json:"kategorie"`
	GueltigkeitMonate *int      `json:"gueltigkeit_monate"`
	PflichtFuerAlle   bool      `json:"pflicht_fuer_alle"`
	Aktiv             bool      `json:"aktiv"`
	Hinweis           string    `json:"hinweis"`
}

func decodeQualificationType(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) qualificationTypeResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out qualificationTypeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeQualificationTypeList(t *testing.T, rec *httptest.ResponseRecorder) []qualificationTypeResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []qualificationTypeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Liste nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

const qtPath = "/api/v1/qualification-types/"

func TestQualificationTypesHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}

	created := decodeQualificationType(t, do(http.MethodPost, qtPath,
		`{"code":" schweisser_135 ","name":"Schweißerprüfung MAG","kategorie":"qualifikation","gueltigkeit_monate":36,"hinweis":"DIN EN ISO 9606-1"}`), http.StatusCreated)
	if created.Code != "SCHWEISSER_135" || !created.Aktiv || created.GueltigkeitMonate == nil || *created.GueltigkeitMonate != 36 {
		t.Fatalf("Art nicht wie angelegt: %+v", created)
	}
	got := decodeQualificationType(t, do(http.MethodGet, qtPath+created.ID.String(), ""), http.StatusOK)
	if got.ID != created.ID || got.Hinweis != "DIN EN ISO 9606-1" {
		t.Fatalf("Einzelabruf liefert nicht die angelegte Art: %+v", got)
	}

	// PUT ersetzt vollstaendig - auch die Frist auf "keine" (null).
	updated := decodeQualificationType(t, do(http.MethodPut, qtPath+created.ID.String(),
		`{"name":"Schweißerprüfung MAG (135)","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"aktiv":true,"hinweis":"geaendert"}`), http.StatusOK)
	if updated.GueltigkeitMonate != nil || updated.Name != "Schweißerprüfung MAG (135)" || updated.Code != "SCHWEISSER_135" {
		t.Fatalf("Aenderung nicht wie angegeben: %+v", updated)
	}

	// Ein PUT, dem ein Feld fehlt, wird abgelehnt - nicht mit einem
	// stillen Standardwert ausgefuehrt. Und nichts aendert sich.
	for field, body := range map[string]string{
		"aktiv":              `{"name":"X","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"hinweis":""}`,
		"gueltigkeit_monate": `{"name":"X","kategorie":"qualifikation","pflicht_fuer_alle":false,"aktiv":true,"hinweis":""}`,
		"hinweis":            `{"name":"X","kategorie":"qualifikation","gueltigkeit_monate":12,"pflicht_fuer_alle":false,"aktiv":true}`,
	} {
		t.Run("PUT ohne "+field, func(t *testing.T) {
			wantAPIError(t, do(http.MethodPut, qtPath+created.ID.String(), body), http.StatusBadRequest, "validation_error", "Feld erforderlich: "+field)
		})
	}
	wantAPIError(t, do(http.MethodPut, qtPath+created.ID.String(),
		`{"code":"NEU","name":"X","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"aktiv":true,"hinweis":""}`),
		http.StatusBadRequest, "validation_error", "Der Code einer Qualifikationsart ist nicht änderbar")
	wantAPIError(t, do(http.MethodPut, qtPath+created.ID.String(),
		`{"name":"X","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"aktiv":true,"hinweis":"","farbe":"rot"}`),
		http.StatusBadRequest, "validation_error", "unbekanntes Feld: farbe")
	if still := decodeQualificationType(t, do(http.MethodGet, qtPath+created.ID.String(), ""), http.StatusOK); still.Name != "Schweißerprüfung MAG (135)" || !still.Aktiv || still.Hinweis != "geaendert" {
		t.Fatalf("abgelehnte PUTs haben die Art veraendert: %+v", still)
	}

	// Stilllegen, Liste mit und ohne inaktive.
	decodeQualificationType(t, do(http.MethodPut, qtPath+created.ID.String(),
		`{"name":"Schweißerprüfung MAG (135)","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"aktiv":false,"hinweis":"geaendert"}`), http.StatusOK)
	if list := decodeQualificationTypeList(t, do(http.MethodGet, qtPath, "")); len(list) != 0 {
		t.Fatalf("Standardliste darf die stillgelegte Art nicht enthalten: %+v", list)
	}
	if list := decodeQualificationTypeList(t, do(http.MethodGet, qtPath+"?include_inactive=true", "")); len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("Liste mit inaktiven: erwartet die stillgelegte Art, bekommen %+v", list)
	}

	// Vorschlagskatalog uebernehmen: einmal alles, beim zweiten Mal nichts.
	adopted := decodeQualificationTypeList(t, do(http.MethodPost, qtPath+"adopt-defaults", ""))
	if len(adopted) != 18 {
		t.Fatalf("erwartet 18 uebernommene Arten, bekommen %d", len(adopted))
	}
	var arbeitsschutz *qualificationTypeResponse
	for i := range adopted {
		if adopted[i].Code == "ARBEITSSCHUTZ" {
			arbeitsschutz = &adopted[i]
		}
	}
	if arbeitsschutz == nil || !arbeitsschutz.PflichtFuerAlle || arbeitsschutz.GueltigkeitMonate == nil || *arbeitsschutz.GueltigkeitMonate != 12 {
		t.Fatalf("ARBEITSSCHUTZ nicht wie im Vorschlag: %+v", arbeitsschutz)
	}
	if again := decodeQualificationTypeList(t, do(http.MethodPost, qtPath+"adopt-defaults", "")); len(again) != 0 {
		t.Fatalf("zweite Uebernahme darf nichts anlegen, bekommen %d", len(again))
	}
	if list := decodeQualificationTypeList(t, do(http.MethodGet, qtPath, "")); len(list) != 18 {
		t.Fatalf("erwartet 18 aktive Arten im Katalog, bekommen %d", len(list))
	}

	// Fehlerfaelle mit Status, Code und Text.
	wantAPIError(t, do(http.MethodPost, qtPath, `{"code":"ARBEITSSCHUTZ","name":"Doppelt","kategorie":"unterweisung","gueltigkeit_monate":12}`),
		http.StatusBadRequest, "validation_error", "Qualifikationsart mit diesem Code existiert bereits")
	wantAPIError(t, do(http.MethodPost, qtPath, `{"code":"X","name":"X","kategorie":"vorsorge"}`),
		http.StatusBadRequest, "validation_error", "Ungültige Kategorie")
	wantAPIError(t, do(http.MethodPost, qtPath, `{"code":"X Y","name":"X","kategorie":"unterweisung"}`),
		http.StatusBadRequest, "validation_error", "Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)")
	wantAPIError(t, do(http.MethodPost, qtPath, `{"code":"X","name":"X","kategorie":"unterweisung","gueltigkeit_monate":0}`),
		http.StatusBadRequest, "validation_error", "Gültigkeit muss größer als 0 sein")
	wantAPIError(t, do(http.MethodPost, qtPath, `{"code":"X","name":" ","kategorie":"unterweisung"}`),
		http.StatusBadRequest, "validation_error", "Name erforderlich")
	wantAPIError(t, do(http.MethodPost, qtPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodGet, qtPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")
	wantAPIError(t, do(http.MethodGet, qtPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Qualifikationsart-ID")
	wantAPIError(t, do(http.MethodPut, qtPath+uuid.NewString(),
		`{"name":"X","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"aktiv":true,"hinweis":""}`),
		http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")
}

// Lesen darf, wer eines der drei Rechte hat; schreiben nur mit
// qualifications.write. Geprueft auch mit Rollen, die genau eines haben.
func TestQualificationTypesHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-qual-readonly", "qualifications.read")
	seedRoleWithPermissions(t, env, "itest-qual-writeonly", "qualifications.write")
	seedRoleWithPermissions(t, env, "itest-qual-none")
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	existing := decodeQualificationType(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qtPath, `{"code":"ERSTHELFER","name":"Ersthelfer","kategorie":"qualifikation","gueltigkeit_monate":24}`), http.StatusCreated)
	put := `{"name":"Geaendert","kategorie":"qualifikation","gueltigkeit_monate":24,"pflicht_fuer_alle":false,"aktiv":true,"hinweis":""}`

	type call struct{ method, path, body string }
	reads := []call{{http.MethodGet, qtPath, ""}, {http.MethodGet, qtPath + existing.ID.String(), ""}}
	writes := []call{
		{http.MethodPost, qtPath, `{"code":"NEU","name":"Neu","kategorie":"qualifikation"}`},
		{http.MethodPut, qtPath + existing.ID.String(), put},
		{http.MethodPost, qtPath + "adopt-defaults", ""},
	}

	for _, tc := range []struct {
		role              string
		canRead, canWrite bool
	}{
		{"production", true, false},          // nur qualifications.self
		{"itest-qual-readonly", true, false}, // nur read
		{"itest-qual-writeonly", true, true}, // nur write - darf den Katalog auch lesen
		{"itest-qual-none", false, false},    // keines
	} {
		t.Run(tc.role, func(t *testing.T) {
			token := tenant.user(tc.role)
			for _, c := range reads {
				rec := doAuthedRequest(t, handler, token, c.method, c.path, c.body)
				if tc.canRead && rec.Code != http.StatusOK {
					t.Fatalf("%s %s: erwartet 200, bekommen %d (%s)", c.method, c.path, rec.Code, rec.Body.String())
				}
				if !tc.canRead {
					wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				}
			}
			for _, c := range writes {
				rec := doAuthedRequest(t, handler, token, c.method, c.path, c.body)
				if tc.canWrite {
					if rec.Code >= 300 {
						t.Fatalf("%s %s: erwartet Erfolg, bekommen %d (%s)", c.method, c.path, rec.Code, rec.Body.String())
					}
					continue
				}
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
			}
		})
	}

	// Ohne Anmeldung: 401.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, qtPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestQualificationTypesHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("hr"), two.user("admin")

	own := decodeQualificationType(t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, qtPath, `{"code":"STAPLER_SCHEIN","name":"Stapler","kategorie":"qualifikation"}`), http.StatusCreated)

	// Der Administrator des anderen Mandanten sieht die Art nicht und kann
	// sie nicht aendern.
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, qtPath+own.ID.String(), ""), http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPut, qtPath+own.ID.String(),
		`{"name":"Fremd","kategorie":"qualifikation","gueltigkeit_monate":null,"pflicht_fuer_alle":false,"aktiv":false,"hinweis":""}`),
		http.StatusNotFound, "not_found", "Qualifikationsart nicht gefunden")
	if list := decodeQualificationTypeList(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, qtPath+"?include_inactive=true", "")); len(list) != 0 {
		t.Fatalf("Katalog des anderen Mandanten muss leer sein, bekommen %+v", list)
	}

	// Derselbe Code im anderen Mandanten ist ein eigener Eintrag.
	decodeQualificationType(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, qtPath, `{"code":"STAPLER_SCHEIN","name":"Stapler (Mandant 2)","kategorie":"qualifikation"}`), http.StatusCreated)

	// Die Uebernahme des Vorschlags wirkt nur im eigenen Mandanten - und
	// laesst die schon vorhandene Art dort unberuehrt.
	adopted := decodeQualificationTypeList(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, qtPath+"adopt-defaults", ""))
	if len(adopted) != 17 {
		t.Fatalf("erwartet 17 neue Arten (STAPLER_SCHEIN war schon da), bekommen %d", len(adopted))
	}
	if list := decodeQualificationTypeList(t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, qtPath, "")); len(list) != 1 || list[0].Name != "Stapler" {
		t.Fatalf("Uebernahme im anderen Mandanten hat diesen Katalog veraendert: %+v", list)
	}
}

// Jeder Fehlertext des Katalogs muss als 4xx klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversQualificationTypeMessages(t *testing.T) {
	cases := map[string]int{
		"Code erforderlich": http.StatusBadRequest,
		"Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)": http.StatusBadRequest,
		"Name erforderlich":                                   http.StatusBadRequest,
		"Ungültige Kategorie":                                 http.StatusBadRequest,
		"Gültigkeit muss größer als 0 sein":                   http.StatusBadRequest,
		"Qualifikationsart mit diesem Code existiert bereits": http.StatusBadRequest,
		"Qualifikationsart nicht gefunden":                    http.StatusNotFound,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
