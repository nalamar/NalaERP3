package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung des Asset-Bestands (Backlog F.3.6.1). Helfer
// (newCatalogTenant, doAuthedRequest, wantAPIError, seedRoleWithPermissions)
// aus den Testdateien der Qualifikationen, Zeiterfassung und
// Urlaubsantraege. Jeder Test in eigenen, frisch angelegten Mandanten.

type assetResponse struct {
	ID                   uuid.UUID `json:"id"`
	Inventarnummer       string    `json:"inventarnummer"`
	Bezeichnung          string    `json:"bezeichnung"`
	Kategorie            string    `json:"kategorie"`
	Seriennummer         string    `json:"seriennummer"`
	Hersteller           string    `json:"hersteller"`
	Modell               string    `json:"modell"`
	Standort             string    `json:"standort"`
	Anschaffungsdatum    *string   `json:"anschaffungsdatum"`
	Anschaffungspreis    *float64  `json:"anschaffungspreis"`
	KostenstelleID       *string   `json:"kostenstelle_id"`
	PruefintervallMonate *int      `json:"pruefintervall_monate"`
	Notiz                string    `json:"notiz"`
	AusgemustertAm       *string   `json:"ausgemustert_am"`
	AusgemustertGrund    string    `json:"ausgemustert_grund"`
}

func decodeAsset(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) assetResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out assetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeAssetList(t *testing.T, rec *httptest.ResponseRecorder) []assetResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []assetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Liste nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func assetNumbers(list []assetResponse) string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Inventarnummer)
	}
	return strings.Join(out, ",")
}

const assetPath = "/api/v1/assets/"

func seedHTTPCostCenter(t *testing.T, env *testutil.IntegrationEnv, company string) string {
	t.Helper()
	id := "cc-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO cost_centers (id, company_id, code, name) VALUES ($1,$2,$3,'Werkstatt')`, id, company, id); err != nil {
		t.Fatalf("Kostenstelle anlegen: %v", err)
	}
	return id
}

func TestAssetsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	cc := seedHTTPCostCenter(t, env, tenant.company)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}

	created := decodeAsset(t, do(http.MethodPost, assetPath,
		`{"inventarnummer":" WZ-17 ","bezeichnung":"Bohrhammer","kategorie":"elektrowerkzeug","seriennummer":"SN1","hersteller":"Hilti","modell":"TE 30","standort":"Werkstatt",
		  "anschaffungsdatum":"2025-03-01","anschaffungspreis":499.9,"kostenstelle_id":"`+cc+`","pruefintervall_monate":6,"notiz":"Koffer rot"}`), http.StatusCreated)
	if created.Inventarnummer != "WZ-17" || created.Anschaffungsdatum == nil || !strings.HasPrefix(*created.Anschaffungsdatum, "2025-03-01") ||
		created.Anschaffungspreis == nil || *created.Anschaffungspreis != 499.9 || created.KostenstelleID == nil || *created.KostenstelleID != cc ||
		created.PruefintervallMonate == nil || *created.PruefintervallMonate != 6 || created.Hersteller != "Hilti" || created.AusgemustertAm != nil {
		t.Fatalf("Asset nicht wie angelegt: %+v", created)
	}
	var createdBy *string
	if err := env.PG.QueryRow(context.Background(), `SELECT created_by_user_id FROM assets WHERE id=$1`, created.ID).Scan(&createdBy); err != nil || createdBy == nil {
		t.Fatalf("Urheber nicht gespeichert: %v, %v", createdBy, err)
	}
	got := decodeAsset(t, do(http.MethodGet, assetPath+created.ID.String(), ""), http.StatusOK)
	if got.ID != created.ID || got.Notiz != "Koffer rot" || got.Modell != "TE 30" {
		t.Fatalf("Einzelabruf: %+v", got)
	}

	// PUT ersetzt vollstaendig - optionale Angaben auch auf null.
	put := `{"inventarnummer":"WZ-17","bezeichnung":"Bohrhammer alt","kategorie":"werkzeug","seriennummer":"","hersteller":"","modell":"","standort":"",
	         "anschaffungsdatum":null,"anschaffungspreis":null,"kostenstelle_id":null,"pruefintervall_monate":null,"notiz":""}`
	updated := decodeAsset(t, do(http.MethodPut, assetPath+created.ID.String(), put), http.StatusOK)
	if updated.Bezeichnung != "Bohrhammer alt" || updated.Kategorie != "werkzeug" || updated.Anschaffungsdatum != nil ||
		updated.Anschaffungspreis != nil || updated.KostenstelleID != nil || updated.PruefintervallMonate != nil || updated.Hersteller != "" {
		t.Fatalf("PUT hat nicht alles ersetzt: %+v", updated)
	}

	// Ein PUT, dem ein Feld fehlt, wird abgelehnt; unbekannte Felder (auch
	// die Ausmusterung) ebenso. Nichts aendert sich.
	for _, field := range []string{"pruefintervall_monate", "kostenstelle_id", "notiz"} {
		var raw map[string]json.RawMessage
		_ = json.Unmarshal([]byte(put), &raw)
		delete(raw, field)
		b, _ := json.Marshal(raw)
		wantAPIError(t, do(http.MethodPut, assetPath+created.ID.String(), string(b)), http.StatusBadRequest, "validation_error", "Feld erforderlich: "+field)
	}
	wantAPIError(t, do(http.MethodPut, assetPath+created.ID.String(), strings.Replace(put, `"notiz":""`, `"notiz":"","ausgemustert_am":"2026-01-01"`, 1)),
		http.StatusBadRequest, "validation_error", "unbekanntes Feld: ausgemustert_am")
	wantAPIError(t, do(http.MethodPut, assetPath+created.ID.String(), `[]`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodPut, assetPath+created.ID.String(), strings.Replace(put, `"anschaffungsdatum":null`, `"anschaffungsdatum":"01.03.2025"`, 1)),
		http.StatusBadRequest, "validation_error", "Ungültiges Datum in anschaffungsdatum (erwartet JJJJ-MM-TT)")
	if still := decodeAsset(t, do(http.MethodGet, assetPath+created.ID.String(), ""), http.StatusOK); still.Bezeichnung != "Bohrhammer alt" || still.AusgemustertAm != nil {
		t.Fatalf("abgelehnte PUTs haben das Asset veraendert: %+v", still)
	}

	// Liste: sortiert, Filter, Ausgemusterte nur auf Wunsch.
	decodeAsset(t, do(http.MethodPost, assetPath, `{"inventarnummer":"L-1","bezeichnung":"Leiter","kategorie":"leiter"}`), http.StatusCreated)
	laptop := decodeAsset(t, do(http.MethodPost, assetPath, `{"inventarnummer":"IT-1","bezeichnung":"Laptop","kategorie":"it"}`), http.StatusCreated)
	if nums := assetNumbers(decodeAssetList(t, do(http.MethodGet, assetPath, ""))); nums != "IT-1,L-1,WZ-17" {
		t.Fatalf("Liste: %s", nums)
	}
	if nums := assetNumbers(decodeAssetList(t, do(http.MethodGet, assetPath+"?kategorie=leiter", ""))); nums != "L-1" {
		t.Fatalf("Filter Kategorie: %s", nums)
	}
	wantAPIError(t, do(http.MethodGet, assetPath+"?kategorie=fahrzeug", ""), http.StatusBadRequest, "validation_error", "Ungültige Kategorie")

	// Ausmustern.
	wantAPIError(t, do(http.MethodPost, assetPath+laptop.ID.String()+"/retire", `{"datum":"2026-01-10"}`), http.StatusBadRequest, "validation_error", "Grund der Ausmusterung erforderlich")
	wantAPIError(t, do(http.MethodPost, assetPath+laptop.ID.String()+"/retire", `{"grund":"defekt"}`), http.StatusBadRequest, "validation_error", "Ausmusterungsdatum erforderlich")
	wantAPIError(t, do(http.MethodPost, assetPath+laptop.ID.String()+"/retire", `{"datum":"10.01.2026","grund":"defekt"}`), http.StatusBadRequest, "validation_error", "Ungültiges Datum in datum (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, assetPath+laptop.ID.String()+"/retire", `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	retired := decodeAsset(t, do(http.MethodPost, assetPath+laptop.ID.String()+"/retire", `{"datum":"2026-01-10","grund":"Display defekt"}`), http.StatusOK)
	if retired.AusgemustertAm == nil || !strings.HasPrefix(*retired.AusgemustertAm, "2026-01-10") || retired.AusgemustertGrund != "Display defekt" {
		t.Fatalf("Ausmusterung: %+v", retired)
	}
	wantAPIError(t, do(http.MethodPost, assetPath+laptop.ID.String()+"/retire", `{"datum":"2026-01-11","grund":"nochmal"}`), http.StatusConflict, "conflict", "Asset ist bereits ausgemustert")
	if nums := assetNumbers(decodeAssetList(t, do(http.MethodGet, assetPath, ""))); nums != "L-1,WZ-17" {
		t.Fatalf("ohne Ausgemusterte: %s", nums)
	}
	if nums := assetNumbers(decodeAssetList(t, do(http.MethodGet, assetPath+"?include_ausgemustert=true", ""))); nums != "IT-1,L-1,WZ-17" {
		t.Fatalf("mit Ausgemusterten: %s", nums)
	}

	// Noch ausgegeben: 409. Die Ausgabe wird hier direkt angelegt (HTTP
	// dafuer folgt mit F.3.6.2).
	emp := uuid.New()
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO hr_employees (id, first_name, last_name, company_id) VALUES ($1,'A','B',$2)`, emp, tenant.company); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO asset_assignments (id, asset_id, employee_id, ausgegeben_am) VALUES ($1,$2,$3,'2026-01-05')`, uuid.New(), created.ID, emp); err != nil {
		t.Fatal(err)
	}
	wantAPIError(t, do(http.MethodPost, assetPath+created.ID.String()+"/retire", `{"datum":"2026-01-10","grund":"weg"}`), http.StatusConflict, "conflict", "Asset ist noch ausgegeben - erst zurücknehmen")

	// Weitere Fehlerfaelle mit Status, Code und Text.
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":"L-1","bezeichnung":"Doppelt","kategorie":"leiter"}`),
		http.StatusBadRequest, "validation_error", "Asset mit dieser Inventarnummer existiert bereits")
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":"X","bezeichnung":"X","kategorie":"fahrzeug"}`), http.StatusBadRequest, "validation_error", "Ungültige Kategorie")
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":" ","bezeichnung":"X","kategorie":"it"}`), http.StatusBadRequest, "validation_error", "Inventarnummer erforderlich")
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":"X","bezeichnung":"X","kategorie":"it","anschaffungspreis":-1}`), http.StatusBadRequest, "validation_error", "Anschaffungspreis darf nicht negativ sein")
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":"X","bezeichnung":"X","kategorie":"it","pruefintervall_monate":0}`), http.StatusBadRequest, "validation_error", "Prüfintervall muss größer als 0 sein")
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":"X","bezeichnung":"X","kategorie":"it","anschaffungsdatum":"gestern"}`), http.StatusBadRequest, "validation_error", "Ungültiges Datum in anschaffungsdatum (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, assetPath, `{"inventarnummer":"X","bezeichnung":"X","kategorie":"it","kostenstelle_id":"gibt-es-nicht"}`), http.StatusNotFound, "not_found", "Kostenstelle nicht gefunden")
	wantAPIError(t, do(http.MethodPost, assetPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodGet, assetPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, do(http.MethodGet, assetPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Asset-ID")
	wantAPIError(t, do(http.MethodPut, assetPath+"kein-uuid", put), http.StatusBadRequest, "validation_error", "Ungültige Asset-ID")
	wantAPIError(t, do(http.MethodPost, assetPath+"kein-uuid/retire", `{}`), http.StatusBadRequest, "validation_error", "Ungültige Asset-ID")
	wantAPIError(t, do(http.MethodPut, assetPath+uuid.NewString(), put), http.StatusNotFound, "not_found", "Asset nicht gefunden")
}

// Lesen nur mit assets.read, schreiben nur mit assets.write. assets.self
// allein (alle Systemrollen, auch das Lager) reicht fuer den Bestand nicht
// - Entscheidung des Nutzers vom 2026-10-15: vorerst nur die
// Personalabteilung.
func TestAssetsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-assets-readonly", "assets.read")
	seedRoleWithPermissions(t, env, "itest-assets-writeonly", "assets.write")
	seedRoleWithPermissions(t, env, "itest-assets-self", "assets.self")
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	existing := decodeAsset(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, assetPath, `{"inventarnummer":"E-1","bezeichnung":"Flex","kategorie":"elektrowerkzeug"}`), http.StatusCreated)
	put := `{"inventarnummer":"E-1","bezeichnung":"Flex","kategorie":"elektrowerkzeug","seriennummer":"","hersteller":"","modell":"","standort":"",
	         "anschaffungsdatum":null,"anschaffungspreis":null,"kostenstelle_id":null,"pruefintervall_monate":6,"notiz":""}`

	type call struct{ method, path, body string }
	reads := []call{{http.MethodGet, assetPath, ""}, {http.MethodGet, assetPath + existing.ID.String(), ""}}
	writes := func() []call {
		other := decodeAsset(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, assetPath, `{"inventarnummer":"R-`+uuid.NewString()[:8]+`","bezeichnung":"R","kategorie":"it"}`), http.StatusCreated)
		return []call{
			{http.MethodPost, assetPath, `{"inventarnummer":"NEU-` + uuid.NewString()[:8] + `","bezeichnung":"Neu","kategorie":"it"}`},
			{http.MethodPut, assetPath + existing.ID.String(), put},
			{http.MethodPost, assetPath + other.ID.String() + "/retire", `{"datum":"2026-01-10","grund":"weg"}`},
		}
	}

	for _, tc := range []struct {
		role              string
		canRead, canWrite bool
	}{
		{"hr", true, true},
		{"admin", true, true},
		{"inventory", false, false},  // Lager: vorerst nur assets.self
		{"production", false, false}, // nur assets.self
		{"itest-assets-self", false, false},
		{"itest-assets-readonly", true, false},
		{"itest-assets-writeonly", false, true},
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
			for _, c := range writes() {
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

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, assetPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestAssetsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("hr"), two.user("admin")
	ccOne := seedHTTPCostCenter(t, env, one.company)

	own := decodeAsset(t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, assetPath, `{"inventarnummer":"L-1","bezeichnung":"Leiter","kategorie":"leiter"}`), http.StatusCreated)
	put := `{"inventarnummer":"L-1","bezeichnung":"Fremd","kategorie":"leiter","seriennummer":"","hersteller":"","modell":"","standort":"",
	         "anschaffungsdatum":null,"anschaffungspreis":null,"kostenstelle_id":null,"pruefintervall_monate":null,"notiz":""}`

	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, assetPath+own.ID.String(), ""), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPut, assetPath+own.ID.String(), put), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, assetPath+own.ID.String()+"/retire", `{"datum":"2026-01-10","grund":"weg"}`), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	if list := decodeAssetList(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, assetPath+"?include_ausgemustert=true", "")); len(list) != 0 {
		t.Fatalf("Bestand des anderen Mandanten muss leer sein: %+v", list)
	}
	// Dieselbe Inventarnummer ist im anderen Mandanten frei; die
	// Kostenstelle des ersten Mandanten ist dort unbekannt.
	decodeAsset(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, assetPath, `{"inventarnummer":"L-1","bezeichnung":"Leiter 2","kategorie":"leiter"}`), http.StatusCreated)
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, assetPath, `{"inventarnummer":"L-2","bezeichnung":"X","kategorie":"leiter","kostenstelle_id":"`+ccOne+`"}`),
		http.StatusNotFound, "not_found", "Kostenstelle nicht gefunden")
	if still := decodeAsset(t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, assetPath+own.ID.String(), ""), http.StatusOK); still.Bezeichnung != "Leiter" || still.AusgemustertAm != nil {
		t.Fatalf("anderer Mandant hat das Asset veraendert: %+v", still)
	}
}

// Jeder Fehlertext des Asset-Stamms muss als 4xx klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversAssetMessages(t *testing.T) {
	cases := map[string]int{
		"Mandant erforderlich":                                           http.StatusBadRequest,
		"Inventarnummer erforderlich":                                    http.StatusBadRequest,
		"Inventarnummer darf höchstens 64 Zeichen haben":                 http.StatusBadRequest,
		"Bezeichnung erforderlich":                                       http.StatusBadRequest,
		"Ungültige Kategorie":                                            http.StatusBadRequest,
		"Anschaffungspreis darf nicht negativ sein":                      http.StatusBadRequest,
		"Prüfintervall muss größer als 0 sein":                           http.StatusBadRequest,
		"Kostenstelle nicht gefunden":                                    http.StatusNotFound,
		"Asset mit dieser Inventarnummer existiert bereits":              http.StatusBadRequest,
		"Asset nicht gefunden":                                           http.StatusNotFound,
		"Ausmusterungsdatum erforderlich":                                http.StatusBadRequest,
		"Grund der Ausmusterung erforderlich":                            http.StatusBadRequest,
		"Ausmusterungsdatum darf nicht in der Zukunft liegen":            http.StatusBadRequest,
		"Ausmusterungsdatum darf nicht vor dem Anschaffungsdatum liegen": http.StatusBadRequest,
		"Asset ist bereits ausgemustert":                                 http.StatusConflict,
		"Asset ist noch ausgegeben - erst zurücknehmen":                  http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
