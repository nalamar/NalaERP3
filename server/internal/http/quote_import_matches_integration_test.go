package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/leistungen"
	"nalaerp3/internal/llm"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/quotes/eval"
	"nalaerp3/internal/testutil"
)

// Routen der Zuordnung am GAEB-Import (Backlog I.5.3). Erfundene LVs; der
// Lauf laeuft im Test synchron (MatchRunner), ohne Sprachmodell
// (Wortabgleich).

func TestQuoteImportMatchRoutes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	var runs int
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, env.Cfg, V1RouterOptions{
		MatchRunner: func(run func(context.Context) error) {
			runs++
			if err := run(ctx); err != nil {
				t.Errorf("Lauf: %v", err)
			}
		},
	})
	testutil.SeedAuthUser(t, env, "match-admin@example.test", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "match-proc@example.test", "Secret123!", "procurement")
	admin := loginIntegrationUser(t, h, "match-admin@example.test", "Secret123!")
	proc := loginIntegrationUser(t, h, "match-proc@example.test", "Secret123!")
	// nur quotes.read: lesen ja, starten und entscheiden nein
	for _, q := range []string{
		`INSERT INTO roles (id, code, name, description, is_system) VALUES ('role-itest-quotes-read','itest-quotes-read','Test: Angebote lesen','',false)
			ON CONFLICT (code) DO NOTHING`,
		`INSERT INTO role_permissions (role_id, permission_id) SELECT 'role-itest-quotes-read', id FROM permissions WHERE code='quotes.read'
			ON CONFLICT DO NOTHING`,
	} {
		if _, err := env.PG.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	testutil.SeedAuthUser(t, env, "match-read@example.test", "Secret123!", "itest-quotes-read")
	reader := loginIntegrationUser(t, h, "match-read@example.test", "Secret123!")

	// Projekt, Katalog (eigene Codes je Lauf) und Import im Mandanten "default"
	prefix := "M" + uuid.NewString()[:6] + "-"
	project := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, $2, 'Zuordnung', 'default')`,
		project, "P-"+prefix); err != nil {
		t.Fatal(err)
	}
	k, err := eval.LoadKatalog("../quotes/eval/testdata/katalog.json")
	if err != nil {
		t.Fatal(err)
	}
	lst := leistungen.NewService(env.PG)
	var ersteLeistung string
	for _, l := range k.Leistungen {
		neu, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: prefix + l.Code, Kurztext: l.Kurztext, Einheit: l.Einheit}, "", "default")
		if err != nil {
			t.Fatal(err)
		}
		if ersteLeistung == "" {
			ersteLeistung = neu.ID
		}
	}
	imp := matchImport(t, env, project, "../quotes/eval/testdata/lv-01.x83")
	base := "/quotes/imports/" + imp

	// ohne Anmeldung 401, ohne quotes-Rechte 403
	if rec := mailRequest(t, h, "", http.MethodGet, base+"/match", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, base + "/match", ""},
		{http.MethodGet, base + "/match", ""},
		{http.MethodGet, base + "/matches", ""},
		{http.MethodPost, base + "/matches/bestaetigen", ""},
		{http.MethodPut, base + "/matches/" + uuid.NewString(), `{"aktion":"verwerfen"}`},
	} {
		if rec := mailRequest(t, h, proc, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Fatalf("ohne Recht %s %s: %d", c.method, c.path, rec.Code)
		}
	}

	for _, c := range []struct {
		method, path, body string
		code               int
	}{
		{http.MethodGet, base + "/match", "", http.StatusOK},
		{http.MethodGet, base + "/matches", "", http.StatusOK},
		{http.MethodPost, base + "/match", "", http.StatusForbidden},
		{http.MethodPost, base + "/matches/bestaetigen", "", http.StatusForbidden},
		{http.MethodPut, base + "/matches/" + uuid.NewString(), `{"aktion":"verwerfen"}`, http.StatusForbidden},
	} {
		if rec := mailRequest(t, h, reader, c.method, c.path, c.body); rec.Code != c.code {
			t.Fatalf("nur lesen %s %s: %d", c.method, c.path, rec.Code)
		}
	}

	// Status vor dem Lauf
	var st matching.LaufStatus
	if rec := mailRequest(t, h, admin, http.MethodGet, base+"/match", ""); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &st) != nil ||
		st.Status != "" {
		t.Fatalf("Status vorher: %d %s", rec.Code, rec.Body.String())
	}
	// ungueltige Eingaben
	for _, c := range []struct {
		body string
		code int
	}{
		{`{"wortabgleich":`, http.StatusBadRequest},
		{`{"wortabgleich":true,"profil_id":"` + uuid.NewString() + `"}`, http.StatusBadRequest},
		{`{"profil_id":"` + uuid.NewString() + `"}`, http.StatusNotFound},
	} {
		if rec := mailRequest(t, h, admin, http.MethodPost, base+"/match", c.body); rec.Code != c.code {
			t.Fatalf("%s: %d %s", c.body, rec.Code, rec.Body.String())
		}
	}
	if runs != 0 {
		t.Fatalf("Lauf trotz Fehler: %d", runs)
	}
	// Start ohne Body: Standardprofil fehlt, also Wortabgleich; 202
	rec := mailRequest(t, h, admin, http.MethodPost, base+"/match", "")
	if rec.Code != http.StatusAccepted || json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Status != "laeuft" || st.Quelle != "wortabgleich" ||
		runs != 1 {
		t.Fatalf("Start: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodGet, base+"/match", ""); json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Status != "fertig" ||
		st.Fortschritt != st.Gesamt || st.Gesamt == 0 {
		t.Fatalf("Status nachher: %s", rec.Body.String())
	}

	// Pruefliste
	var liste []matching.PruefZeile
	if rec := mailRequest(t, h, admin, http.MethodGet, base+"/matches", ""); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &liste) != nil ||
		len(liste) != st.Gesamt {
		t.Fatalf("Prüfliste: %d %s", rec.Code, rec.Body.String())
	}
	var mitCode, ohneCode string
	// Im Mandanten "default" liegen auch Leistungen anderer Tests.
	for _, z := range liste {
		if z.Code != "" && mitCode == "" {
			mitCode = z.ItemID
		}
		if z.Code == "" && ohneCode == "" {
			ohneCode = z.ItemID
		}
	}
	if mitCode == "" || ohneCode == "" {
		t.Fatalf("Prüfliste ohne beide Fälle: %s / %s", mitCode, ohneCode)
	}

	// Entscheiden
	var z matching.PruefZeile
	rec = mailRequest(t, h, admin, http.MethodPut, base+"/matches/"+ohneCode, `{"aktion":"aendern","leistung_id":"`+ersteLeistung+`"}`)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &z) != nil || z.Entscheidung != "geaendert" || z.EntschiedenVonUserID == nil {
		t.Fatalf("ändern: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range []struct {
		item, body string
		code       int
	}{
		{mitCode, `{"aktion":"annehmen"}`, http.StatusBadRequest},
		{mitCode, `{"aktion":"aendern"}`, http.StatusBadRequest},
		{mitCode, `{"aktion":`, http.StatusBadRequest},
		{ohneCode, `{"aktion":"bestaetigen"}`, http.StatusBadRequest},
		{uuid.NewString(), `{"aktion":"verwerfen"}`, http.StatusNotFound},
		{mitCode, `{"aktion":"aendern","leistung_id":"gibt-es-nicht"}`, http.StatusNotFound},
	} {
		if rec := mailRequest(t, h, admin, http.MethodPut, base+"/matches/"+c.item, c.body); rec.Code != c.code {
			t.Fatalf("%s: %d %s", c.body, rec.Code, rec.Body.String())
		}
	}
	// Sammelbestaetigung
	var sammel struct {
		Bestaetigt int `json:"bestaetigt"`
	}
	if rec := mailRequest(t, h, admin, http.MethodPost, base+"/matches/bestaetigen", ""); rec.Code != http.StatusOK ||
		json.Unmarshal(rec.Body.Bytes(), &sammel) != nil {
		t.Fatalf("Sammel: %d %s", rec.Code, rec.Body.String())
	}
	var bestaetigt int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_import_item_matches WHERE import_id=$1 AND entscheidung='bestaetigt'`, imp).
		Scan(&bestaetigt); err != nil || bestaetigt != sammel.Bestaetigt || bestaetigt == 0 {
		t.Fatalf("Sammel: %d bestätigt, gemeldet %d %v", bestaetigt, sammel.Bestaetigt, err)
	}
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_import_item_matches WHERE import_id=$1 AND entscheidung='bestaetigt'
		AND entschieden_von_user_id IS NOT NULL`, imp).Scan(&bestaetigt); err != nil || bestaetigt != sammel.Bestaetigt {
		t.Fatalf("Sammel: %d bestätigt, gemeldet %d %v", bestaetigt, sammel.Bestaetigt, err)
	}

	// laufender Lauf: 409; Freigabe mit offenen Vorschlaegen: 400, waehrend des Laufs: 409
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET review_status='accepted' WHERE import_id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	if rec := mailRequest(t, h, admin, http.MethodPatch, base+"/review", ""); rec.Code != http.StatusBadRequest ||
		!strings.Contains(mailErrorMessage(t, rec), "offene Vorschläge der Zuordnung") {
		t.Fatalf("Freigabe mit offenen: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET match_status='laeuft' WHERE id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	if rec := mailRequest(t, h, admin, http.MethodPost, base+"/match", `{"wortabgleich":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("doppelter Start: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodPatch, base+"/review", ""); rec.Code != http.StatusConflict {
		t.Fatalf("Freigabe während des Laufs: %d %s", rec.Code, rec.Body.String())
	}
	for _, sql := range []string{
		`UPDATE quote_imports SET match_status='fertig' WHERE id=$1`,
		`UPDATE quote_import_item_matches SET entscheidung='verworfen', entschieden_at=now() WHERE import_id=$1 AND entscheidung='offen' AND code<>''`,
	} {
		if _, err := env.PG.Exec(ctx, sql, imp); err != nil {
			t.Fatal(err)
		}
	}
	if rec := mailRequest(t, h, admin, http.MethodPatch, base+"/review", ""); rec.Code != http.StatusOK {
		t.Fatalf("Freigabe: %d %s", rec.Code, rec.Body.String())
	}
	// nach der Freigabe: Start und Entscheidung 409
	if rec := mailRequest(t, h, admin, http.MethodPost, base+"/match", `{"wortabgleich":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("Start nach Freigabe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := mailRequest(t, h, admin, http.MethodPut, base+"/matches/"+mitCode, `{"aktion":"verwerfen"}`); rec.Code != http.StatusConflict {
		t.Fatalf("Entscheidung nach Freigabe: %d %s", rec.Code, rec.Body.String())
	}

	// fremder Mandant: 404
	other := "match-" + uuid.NewString()[:8]
	otherProject := uuid.NewString()
	otherImp := uuid.NewString()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, []any{other, "Fremd " + other}},
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, $2, 'Fremd', $3)`, []any{otherProject, "P-" + other, other}},
		{`INSERT INTO quote_imports (id, project_id, source_kind, source_filename, source_document_id, status)
			VALUES ($1, $2, 'gaeb', 'x.x83', 'test', 'parsed')`, []any{otherImp, otherProject}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/quotes/imports/" + otherImp + "/match", ""},
		{http.MethodGet, "/quotes/imports/" + otherImp + "/match", ""},
		{http.MethodGet, "/quotes/imports/" + otherImp + "/matches", ""},
		{http.MethodPost, "/quotes/imports/" + otherImp + "/matches/bestaetigen", ""},
		{http.MethodPut, "/quotes/imports/" + otherImp + "/matches/" + uuid.NewString(), `{"aktion":"verwerfen"}`},
		{http.MethodGet, "/quotes/imports/kein-uuid/match", ""},
	} {
		if rec := mailRequest(t, h, admin, c.method, c.path, c.body); rec.Code != http.StatusNotFound {
			t.Fatalf("fremd %s %s: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}

	// gewaehltes Profil ausgeschaltet: 409, kein Lauf
	imp2 := matchImport(t, env, project, "../quotes/eval/testdata/lv-03.x83")
	base2 := "/quotes/imports/" + imp2
	profil, err := llm.NewSettingsService(env.PG, nil).CreateProfile(ctx, llm.ProfileInput{Name: "Zuordnung " + prefix}, "", "default")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = env.PG.Exec(context.Background(), `DELETE FROM company_llm_profiles WHERE id=$1`, profil.ID)
	})
	if rec := mailRequest(t, h, admin, http.MethodPost, base2+"/match", `{"profil_id":"`+profil.ID+`"}`); rec.Code != http.StatusConflict ||
		!strings.Contains(mailErrorMessage(t, rec), "nicht eingeschaltet") || runs != 1 {
		t.Fatalf("Profil aus: %d %s", rec.Code, rec.Body.String())
	}

	// ohne MatchRunner: Lauf im Hintergrund, Anfrage kehrt sofort zurueck
	h2 := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, env.Cfg, V1RouterOptions{})
	if rec := mailRequest(t, h2, admin, http.MethodPost, base2+"/match", `{"wortabgleich":true}`); rec.Code != http.StatusAccepted {
		t.Fatalf("Hintergrund: %d %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		rec := mailRequest(t, h2, admin, http.MethodGet, base2+"/match", "")
		if json.Unmarshal(rec.Body.Bytes(), &st) != nil {
			t.Fatalf("Status: %s", rec.Body.String())
		}
		if st.Status == "fertig" && st.Fortschritt == st.Gesamt && st.Gesamt > 0 {
			break
		}
		if st.Status != "laeuft" || time.Now().After(deadline) {
			t.Fatalf("Hintergrundlauf: %+v", st)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if runs != 1 {
		t.Fatalf("Testrunner benutzt: %d", runs)
	}
}

// matchImport legt einen GAEB-Importlauf im Mandanten "default" an und
// verarbeitet die Datei.
func matchImport(t *testing.T, env *testutil.IntegrationEnv, project, datei string) string {
	t.Helper()
	ctx := context.Background()
	imp := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `INSERT INTO quote_imports (id, project_id, source_kind, source_filename, source_document_id, status)
		VALUES ($1, $2, 'gaeb', 'lv.x83', 'test', 'uploaded')`, imp, project); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(datei)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := quotes.GAEBDAXMLParser{}.ParseGAEB(ctx, f, "lv.x83")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := quotes.NewService(env.PG, nil).SaveImportParse(ctx, imp, res, "default"); err != nil {
		t.Fatal(err)
	}
	return imp
}
