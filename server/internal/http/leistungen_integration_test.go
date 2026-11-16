package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/leistungen"
	"nalaerp3/internal/testutil"
)

// Routen /leistungen (Backlog I.4.4b). Codes mit Zufallsanteil, damit der
// Test auf derselben Datenbank wiederholbar bleibt.

func leistungenRequest(t *testing.T, h http.Handler, token, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader([]byte(body)))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// csvUpload baut ein Upload-Formular mit der Datei im Feld feld.
func csvUpload(t *testing.T, feld, inhalt string) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(feld, "leistungen.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte(inhalt))
	_ = mw.Close()
	return mw.FormDataContentType(), buf.String()
}

func TestLeistungenRoutes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, env.Cfg, V1RouterOptions{})
	suffix := uuid.NewString()[:8]
	testutil.SeedAuthUser(t, env, "lst-sales-"+suffix+"@example.test", "Secret123!", "sales")
	testutil.SeedAuthUser(t, env, "lst-einkauf-"+suffix+"@example.test", "Secret123!", "procurement")
	// eigene Rolle: nur quotes.read
	leseRolle := "lst-lesen-" + suffix
	if _, err := env.PG.Exec(ctx, `INSERT INTO roles (id, code, name, description, is_system) VALUES ($1, $1, 'Nur lesen', 'Test', false)`, leseRolle); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) SELECT $1, id FROM permissions WHERE code='quotes.read'`, leseRolle); err != nil {
		t.Fatal(err)
	}
	testutil.SeedAuthUser(t, env, "lst-lesen-"+suffix+"@example.test", "Secret123!", leseRolle)
	sales := loginIntegrationUser(t, h, "lst-sales-"+suffix+"@example.test", "Secret123!")
	einkauf := loginIntegrationUser(t, h, "lst-einkauf-"+suffix+"@example.test", "Secret123!")
	leser := loginIntegrationUser(t, h, "lst-lesen-"+suffix+"@example.test", "Secret123!")
	code := "FE-" + suffix

	// Rechte: ohne Anmeldung 401; Einkauf ohne quotes.read 403; Leser ohne leistungen.write 403 beim Schreiben
	if rec := leistungenRequest(t, h, "", http.MethodGet, "/leistungen/", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	for _, c := range []struct{ token, method, path, body string }{
		{einkauf, http.MethodGet, "/leistungen/", ""},
		{einkauf, http.MethodGet, "/leistungen/x", ""},
		{leser, http.MethodPost, "/leistungen/", `{"code":"X","kurztext":"x","einheit":"Stk"}`},
		{leser, http.MethodPut, "/leistungen/x", `{"code":"X","kurztext":"x","einheit":"Stk"}`},
		{leser, http.MethodPost, "/leistungen/import", `{"leistungen":[]}`},
	} {
		if rec := leistungenRequest(t, h, c.token, c.method, c.path, "application/json", c.body); rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: %d", c.method, c.path, rec.Code)
		}
	}

	// Anlegen 201, doppelt 400, ungültig 400, kaputtes JSON 400
	rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/", "application/json",
		`{"code":"`+code+`","kurztext":"Aluminiumfenster","einheit":"Stk","rc_klasse":"RC2","u_wert":0.9}`)
	var l leistungen.Leistung
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &l) != nil || l.Code != code || l.UpdatedByUserID == nil || !l.Aktiv {
		t.Fatalf("Anlegen: %d %s", rec.Code, rec.Body.String())
	}
	for body, want := range map[string]string{
		`{"code":"` + code + `","kurztext":"x","einheit":"Stk"}`: `Code "` + code + `" bereits vorhanden`,
		`{"code":"Y","kurztext":"","einheit":"Stk"}`:             "Kurztext erforderlich",
		`{"code":"Y","kurztext":"y","einheit":"Stk","u_wert":0}`: "U-Wert ungültig (über 0 bis 10)",
		`{"code":`: "Ungültige Eingabe",
	} {
		if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/", "application/json", body); rec.Code != http.StatusBadRequest || mailErrorMessage(t, rec) != want {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}

	// Lesen und Ändern (auch der Leser darf lesen); fremde ID 404
	if rec := leistungenRequest(t, h, leser, http.MethodGet, "/leistungen/"+l.ID, "", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), code) {
		t.Fatalf("Lesen: %d %s", rec.Code, rec.Body.String())
	}
	if rec := leistungenRequest(t, h, sales, http.MethodPut, "/leistungen/"+l.ID, "application/json",
		`{"code":"`+code+`","kurztext":"Fenster geändert","einheit":"Stk","aktiv":false}`); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"aktiv":false`) || !strings.Contains(rec.Body.String(), "Fenster geändert") {
		t.Fatalf("Ändern: %d %s", rec.Code, rec.Body.String())
	}
	if rec := leistungenRequest(t, h, sales, http.MethodPut, "/leistungen/"+l.ID, "application/json", `{"code":`); rec.Code != http.StatusBadRequest {
		t.Fatalf("Ändern kaputt: %d", rec.Code)
	}
	if rec := leistungenRequest(t, h, sales, http.MethodGet, "/leistungen/"+uuid.NewString(), "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unbekannt: %d", rec.Code)
	}
	// anderer Mandant: Leistung mit gleichem Code unsichtbar, ID dort 404
	fremd := "lst-" + suffix
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, $1)`, fremd); err != nil {
		t.Fatal(err)
	}
	fremdeLeistung, err := leistungen.NewService(env.PG).Anlegen(ctx, leistungen.Eingabe{Code: code + "-F", Kurztext: "fremd", Einheit: "Stk"}, "", fremd)
	if err != nil {
		t.Fatal(err)
	}
	if rec := leistungenRequest(t, h, sales, http.MethodGet, "/leistungen/"+fremdeLeistung.ID, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("fremder Mandant lesen: %d", rec.Code)
	}
	// Nutzer des anderen Mandanten sieht dessen Leistung, nicht die von default
	fremdUser := testutil.SeedAuthUser(t, env, "lst-fremd-"+suffix+"@example.test", "Secret123!", "sales")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET company_id=$1 WHERE id=$2`, fremd, fremdUser); err != nil {
		t.Fatal(err)
	}
	fremdToken := loginIntegrationUser(t, h, "lst-fremd-"+suffix+"@example.test", "Secret123!")
	if rec := leistungenRequest(t, h, fremdToken, http.MethodGet, "/leistungen/"+fremdeLeistung.ID, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("eigener Mandant lesen: %d %s", rec.Code, rec.Body.String())
	}
	if rec := leistungenRequest(t, h, fremdToken, http.MethodGet, "/leistungen/"+l.ID, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("default-Leistung aus fremdem Mandanten: %d", rec.Code)
	}
	if rec := leistungenRequest(t, h, fremdToken, http.MethodGet, "/leistungen/?q="+code, "", ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), code+"-F") || strings.Count(rec.Body.String(), `"code"`) != 1 {
		t.Fatalf("Liste fremder Mandant: %d %s", rec.Code, rec.Body.String())
	}
	if rec := leistungenRequest(t, h, sales, http.MethodPut, "/leistungen/"+fremdeLeistung.ID, "application/json",
		`{"code":"x","kurztext":"x","einheit":"Stk"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("fremder Mandant ändern: %d", rec.Code)
	}

	// Import JSON: neu und geändert; unbekanntes Feld (z. B. Preis) abgelehnt
	rec = leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", "application/json",
		`{"leistungen":[{"code":"`+code+`","kurztext":"per Import","einheit":"Stk"},{"code":"`+code+`-2","kurztext":"neu","einheit":"m"}]}`)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"neu\":1,\"geaendert\":1}\n" {
		t.Fatalf("Import JSON: %d %s", rec.Code, rec.Body.String())
	}
	for body, want := range map[string]string{
		`{"leistungen":[{"code":"P","kurztext":"p","einheit":"Stk","preis":12}]}`: "Ungültige Eingabe: json: unknown field \"preis\"",
		`{"leistungen":[{"code":"P","kurztext":"","einheit":"Stk"}]}`:             "Zeile 1: Kurztext erforderlich",
		`{"leistungen":[]}`: "Import ungültig: keine Leistungen",
	} {
		if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", "application/json", body); rec.Code != http.StatusBadRequest || mailErrorMessage(t, rec) != want {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}

	// Import CSV: Semikolon, Dezimalkomma, aktiv; Meldung mit Dateizeile (Leerzeile mitgezählt)
	csvDatei := "code;kurztext;einheit;u_wert;aktiv\n" + code + "-3;Fenster CSV;Stk;1,1;ja\n\n" + code + "-4;Tür CSV;Stk;;nein\n"
	ct, body := csvUpload(t, "file", csvDatei)
	rec = leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", ct, body)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"neu\":2,\"geaendert\":0}\n" {
		t.Fatalf("Import CSV: %d %s", rec.Code, rec.Body.String())
	}
	kaputt := "code;kurztext;einheit\n" + code + "-5;ok;Stk\n\n" + code + "-6;;Stk\n"
	ct, body = csvUpload(t, "file", kaputt)
	if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", ct, body); rec.Code != http.StatusBadRequest ||
		mailErrorMessage(t, rec) != "Zeile 4: Kurztext erforderlich" {
		t.Fatalf("CSV Fehler: %d %s", rec.Code, rec.Body.String())
	}
	ct, body = csvUpload(t, "file", "code;preis\nX;1\n")
	if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", ct, body); rec.Code != http.StatusBadRequest ||
		mailErrorMessage(t, rec) != `CSV ungültig: Zeile 1: unbekannte Spalte "preis"` {
		t.Fatalf("CSV Spalte: %d %s", rec.Code, rec.Body.String())
	}
	// falsches Feld: 400
	ct, body = csvUpload(t, "datei", csvDatei)
	if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", ct, body); rec.Code != http.StatusBadRequest ||
		mailErrorMessage(t, rec) != "Datei fehlt (Feld 'file')" {
		t.Fatalf("Feld: %d %s", rec.Code, rec.Body.String())
	}
	// zu groß: 413
	gross := "code;kurztext;einheit\n" + strings.Repeat("X;"+strings.Repeat("k", 100)+";Stk\n", 60000)
	ct, body = csvUpload(t, "file", gross)
	if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", ct, body); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("zu groß: %d %s", rec.Code, rec.Body.String())
	}

	grossJSON := `{"leistungen":[` + strings.Repeat(`{"code":"X","kurztext":"`+strings.Repeat("k", 100)+`","einheit":"Stk"},`, 60000) + `{}]}`
	if rec := leistungenRequest(t, h, sales, http.MethodPost, "/leistungen/import", "application/json", grossJSON); rec.Code != http.StatusRequestEntityTooLarge ||
		mailErrorMessage(t, rec) != "Importdatei ist größer als 5 MB" {
		t.Fatalf("JSON zu groß: %d %s", rec.Code, rec.Body.String())
	}

	// Liste: Suche, nur aktive, Limit; fremder Mandant nicht dabei; ungültige Parameter 400
	liste := func(query string) []leistungen.Leistung {
		rec := leistungenRequest(t, h, leser, http.MethodGet, "/leistungen/?"+query, "", "")
		var ls []leistungen.Leistung
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &ls) != nil {
			t.Fatalf("Liste %s: %d %s", query, rec.Code, rec.Body.String())
		}
		return ls
	}
	codes := func(ls []leistungen.Leistung) string {
		var cs []string
		for _, l := range ls {
			cs = append(cs, l.Code)
		}
		return strings.Join(cs, ",")
	}
	if got := codes(liste("q=" + code)); got != code+","+code+"-2,"+code+"-3,"+code+"-4" {
		t.Fatalf("Suche: %s", got)
	}
	if got := codes(liste("q=" + code + "&aktiv=1")); got != code+"-2,"+code+"-3" {
		t.Fatalf("nur aktive: %s", got)
	}
	if got := codes(liste("q=" + code + "&limit=1&offset=1")); got != code+"-2" {
		t.Fatalf("Seite: %s", got)
	}
	if ls := liste("q=" + code + "-3"); len(ls) != 1 || ls[0].UWert == nil || *ls[0].UWert != 1.1 {
		t.Fatalf("U-Wert aus CSV: %+v", ls)
	}
	for _, q := range []string{"limit=x", "offset=-1"} {
		if rec := leistungenRequest(t, h, leser, http.MethodGet, "/leistungen/?"+q, "", ""); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", q, rec.Code)
		}
	}
}
