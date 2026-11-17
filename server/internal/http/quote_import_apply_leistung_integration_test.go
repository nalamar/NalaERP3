package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/leistungen"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/testutil"
)

// Leistung an der Angebotsposition (Backlog I.5.4): Uebernahme aus der
// Zuordnung, Lesen, Speichern und Revision behalten sie. Erfundene LVs.

func TestQuoteImportApplyLeistung(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, env.Cfg, V1RouterOptions{
		MatchRunner: func(run func(context.Context) error) {
			if err := run(ctx); err != nil {
				t.Errorf("Lauf: %v", err)
			}
		},
	})
	testutil.SeedAuthUser(t, env, "apply-admin@example.test", "Secret123!", "admin")
	admin := loginIntegrationUser(t, h, "apply-admin@example.test", "Secret123!")

	prefix := "A" + uuid.NewString()[:6] + "-"
	project := uuid.NewString()
	kunde := createIntegrationContact(t, h, admin, itemGroupsTestCustomerBody("Übernahme Kunde "+prefix))
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, kunde_id, status, company_id) VALUES ($1, $2, 'Übernahme', $3, 'angebot', 'default')`,
		project, "P-"+prefix, kunde); err != nil {
		t.Fatal(err)
	}
	lst := leistungen.NewService(env.PG)
	if _, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: prefix + "FE", Kurztext: "Aluminiumfenster einflügelig Dreh-Kipp", Einheit: "Stk"}, "", "default"); err != nil {
		t.Fatal(err)
	}
	gewaehlt, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: prefix + "SO", Kurztext: "Sonderleistung", Einheit: "Stk"}, "", "default")
	if err != nil {
		t.Fatal(err)
	}
	other := "apply-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, other, "Fremd "+other); err != nil {
		t.Fatal(err)
	}
	fremd, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: prefix + "FR", Kurztext: "fremd", Einheit: "Stk"}, "", other)
	if err != nil {
		t.Fatal(err)
	}
	// der fremde Mandant hat selbst ein Angebot (Pruefung gegen DIESES Angebot)
	fremdKontakt := "c-" + other
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1, 'org', 'customer', 'Fremdkunde', $2)`, []any{fremdKontakt, other}},
		{`INSERT INTO quotes (id, nummer, root_quote_id, revision_no, contact_id, status, quote_date, currency, net_amount, tax_amount, gross_amount, company_id)
			VALUES ($1, $2, $1, 1, $3, 'draft', now(), 'EUR', 0, 0, 0, $4)`, []any{uuid.NewString(), "ANG-" + other, fremdKontakt, other}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}

	imp := matchImport(t, env, project, "../quotes/eval/testdata/lv-01.x83")
	base := "/quotes/imports/" + imp
	if rec := mailRequest(t, h, admin, http.MethodPost, base+"/match", `{"wortabgleich":true}`); rec.Code != http.StatusAccepted {
		t.Fatalf("Start: %d %s", rec.Code, rec.Body.String())
	}
	var liste []matching.PruefZeile
	if rec := mailRequest(t, h, admin, http.MethodGet, base+"/matches", ""); json.Unmarshal(rec.Body.Bytes(), &liste) != nil {
		t.Fatalf("Prüfliste: %s", rec.Body.String())
	}
	// bestaetigt, geaendert, verworfen, ohne Vorschlag; eine bestaetigte Position nicht ins Angebot
	var bestaetigt, geaendert, verworfen, ohne, abgelehnt string
	var bestaetigtLeistung string
	for _, z := range liste {
		switch {
		case z.Code != "" && bestaetigt == "":
			bestaetigt, bestaetigtLeistung = z.ItemID, *z.LeistungID
		case z.Code != "" && abgelehnt == "":
			abgelehnt = z.ItemID
		case z.Code != "" && verworfen == "":
			verworfen = z.ItemID
		case z.Code == "" && geaendert == "":
			geaendert = z.ItemID
		case z.Code == "" && ohne == "":
			ohne = z.ItemID
		}
	}
	if bestaetigt == "" || abgelehnt == "" || verworfen == "" || geaendert == "" || ohne == "" {
		t.Fatalf("Korpus deckt die Fälle nicht ab: %q %q %q %q %q", bestaetigt, abgelehnt, verworfen, geaendert, ohne)
	}
	for _, c := range []struct{ item, body string }{
		{bestaetigt, `{"aktion":"bestaetigen"}`},
		{abgelehnt, `{"aktion":"bestaetigen"}`},
		{verworfen, `{"aktion":"verwerfen"}`},
		{geaendert, `{"aktion":"aendern","leistung_id":"` + gewaehlt.ID + `"}`},
	} {
		if rec := mailRequest(t, h, admin, http.MethodPut, base+"/matches/"+c.item, c.body); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.body, rec.Code, rec.Body.String())
		}
	}
	for _, q := range []string{
		`UPDATE quote_import_item_matches SET entscheidung='verworfen', entschieden_at=now() WHERE import_id=$1 AND entscheidung='offen' AND code<>''`,
		`UPDATE quote_import_items SET review_status=CASE WHEN id=$2 THEN 'rejected' ELSE 'accepted' END WHERE import_id=$1`,
	} {
		args := []any{imp}
		if strings.Contains(q, "$2") {
			args = append(args, abgelehnt)
		}
		if _, err := env.PG.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	if rec := mailRequest(t, h, admin, http.MethodPatch, base+"/review", ""); rec.Code != http.StatusOK {
		t.Fatalf("Freigabe: %d %s", rec.Code, rec.Body.String())
	}
	rec := mailRequest(t, h, admin, http.MethodPost, base+"/apply", "")
	var applied struct {
		Quote struct {
			ID string `json:"id"`
		} `json:"quote"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &applied) != nil || applied.Quote.ID == "" {
		t.Fatalf("Übernahme: %d %s", rec.Code, rec.Body.String())
	}

	// Leistung je uebernommener Position, Preis 0
	leistungJePosition := func(quoteID string) map[string]string {
		t.Helper()
		rows, err := env.PG.Query(ctx, `SELECT l.quote_import_item_id::text, COALESCE(qi.leistung_id, ''), qi.unit_price::float8
			FROM quote_import_item_links l JOIN quote_items qi ON qi.id = l.quote_item_id WHERE l.quote_id=$1`, quoteID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var item, leistung string
			var preis float64
			if err := rows.Scan(&item, &leistung, &preis); err != nil {
				t.Fatal(err)
			}
			if preis != 0 {
				t.Fatalf("Preis aus der Übernahme: %v", preis)
			}
			out[item] = leistung
		}
		return out
	}
	got := leistungJePosition(applied.Quote.ID)
	if len(got) != len(liste)-1 {
		t.Fatalf("übernommen: %d von %d", len(got), len(liste)-1)
	}
	if got[bestaetigt] != bestaetigtLeistung || got[geaendert] != gewaehlt.ID || got[verworfen] != "" || got[ohne] != "" {
		t.Fatalf("Leistungen: bestätigt %q (%q), geändert %q (%q), verworfen %q, ohne %q", got[bestaetigt], bestaetigtLeistung,
			got[geaendert], gewaehlt.ID, got[verworfen], got[ohne])
	}
	if _, drin := got[abgelehnt]; drin {
		t.Fatal("abgelehnte Position übernommen")
	}

	// Lesen und Speichern: Leistung kommt mit und bleibt; fremde Leistung abgelehnt
	var quote map[string]any
	rec = mailRequest(t, h, admin, http.MethodGet, "/quotes/"+applied.Quote.ID, "")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &quote) != nil {
		t.Fatalf("Lesen: %d %s", rec.Code, rec.Body.String())
	}
	items := quote["items"].([]any)
	mitLeistung := 0
	for _, it := range items {
		if v, _ := it.(map[string]any)["leistung_id"].(string); v != "" {
			mitLeistung++
		}
	}
	if mitLeistung != 2 {
		t.Fatalf("Leistungen beim Lesen: %d", mitLeistung)
	}
	body := func(items []any) string {
		b, _ := json.Marshal(map[string]any{"project_id": quote["project_id"], "contact_id": quote["contact_id"], "currency": "EUR",
			"quote_date": quote["quote_date"], "items": items})
		return string(b)
	}
	if rec := mailRequest(t, h, admin, http.MethodPatch, "/quotes/"+applied.Quote.ID, body(items)); rec.Code != http.StatusOK {
		t.Fatalf("Speichern: %d %s", rec.Code, rec.Body.String())
	}
	var nachSpeichern int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_items WHERE quote_id=$1 AND leistung_id IN ($2, $3)`,
		applied.Quote.ID, bestaetigtLeistung, gewaehlt.ID).Scan(&nachSpeichern); err != nil || nachSpeichern != 2 {
		t.Fatalf("nach Speichern: %d %v", nachSpeichern, err)
	}
	mitFremd := append([]any(nil), items...)
	erste := map[string]any{}
	for k, v := range items[0].(map[string]any) {
		erste[k] = v
	}
	erste["leistung_id"] = fremd.ID
	mitFremd[0] = erste
	if rec := mailRequest(t, h, admin, http.MethodPatch, "/quotes/"+applied.Quote.ID, body(mitFremd)); rec.Code != http.StatusBadRequest ||
		!strings.Contains(mailErrorMessage(t, rec), "leistung_id ist ungültig") {
		t.Fatalf("fremde Leistung: %d %s", rec.Code, rec.Body.String())
	}

	// Revision behaelt die Leistung
	if rec := mailRequest(t, h, admin, http.MethodPost, "/quotes/"+applied.Quote.ID+"/revise", ""); rec.Code != http.StatusOK &&
		rec.Code != http.StatusCreated {
		t.Fatalf("Revision: %d %s", rec.Code, rec.Body.String())
	}
	var revidiert int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_items qi JOIN quotes q ON q.id = qi.quote_id
		WHERE q.id = (SELECT superseded_by_quote_id FROM quotes WHERE id=$1) AND qi.leistung_id IN ($2, $3)`,
		applied.Quote.ID, bestaetigtLeistung, gewaehlt.ID).Scan(&revidiert); err != nil || revidiert != 2 {
		t.Fatalf("Revision: %d %v", revidiert, err)
	}

	// Leistung entfernt (nur direkt in der Datenbank moeglich): Position bleibt ohne Bezug
	var vorher, ohneBezug int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_items WHERE quote_id=$1`, applied.Quote.ID).Scan(&vorher); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM leistungen WHERE id=$1`, gewaehlt.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.PG.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE leistung_id IS NULL) FROM quote_items WHERE quote_id=$1`,
		applied.Quote.ID).Scan(&nachSpeichern, &ohneBezug); err != nil || nachSpeichern != vorher || ohneBezug != vorher-1 {
		t.Fatalf("nach Entfernen der Leistung: %d von %d, ohne Bezug %d %v", nachSpeichern, vorher, ohneBezug, err)
	}
}
