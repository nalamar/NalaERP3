package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// TestQuoteImportBewertung: Gesamtbewertung bei der Freigabe (ADR 0046
// Frage 3, Backlog I.8.3b). Erfundenes LV aus quotes/eval/testdata.
func TestQuoteImportBewertung(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	user := "integration-bewertung-" + suffix + "@example.com"
	userID := testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Bewertung Kunde "+suffix))
	projectID := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, kunde_id, status, company_id) VALUES ($1,$2,'Bewertung Projekt',$3,'angebot','default')`,
		projectID, "PRJ-BEW-"+suffix, customerID); err != nil {
		t.Fatal(err)
	}
	svc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	raw, err := os.ReadFile("../quotes/eval/testdata/lv-02.x83")
	if err != nil {
		t.Fatal(err)
	}
	neuerLauf := func() string {
		t.Helper()
		imp, err := svc.CreateGAEBImport(ctx, quotes.QuoteImportCreateInput{ProjectID: projectID, ContactID: customerID}, bytes.NewReader(raw), "lv-02.x83", "default")
		if err != nil {
			t.Fatal(err)
		}
		result, err := (quotes.GAEBDAXMLParser{}).ParseGAEB(ctx, bytes.NewReader(raw), "lv-02.x83")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SaveImportParse(ctx, imp.ID, result, "default"); err != nil {
			t.Fatal(err)
		}
		if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET review_status='accepted' WHERE import_id=$1`, imp.ID); err != nil {
			t.Fatal(err)
		}
		return imp.ID
	}
	call := func(method, path, body string) (int, quotes.QuoteImport) {
		t.Helper()
		var rd io.Reader // "-": ohne Body
		if body != "-" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		if rd != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var out quotes.QuoteImport
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	freigeben := func(id, body string) (int, quotes.QuoteImport) {
		return call(http.MethodPatch, "/api/v1/quotes/imports/"+id+"/review", body)
	}
	a, b, c := neuerLauf(), neuerLauf(), neuerLauf()

	// falsche Eingaben: 400, der Lauf bleibt unfreigegeben
	for _, body := range []string{
		`{"bewertung":0}`, `{"bewertung":6}`, `{"nacharbeit_min":-1}`, `{"nacharbeit_min":100001}`,
		`{"bewertung_text":"` + strings.Repeat("ä", 2001) + `"}`, `{"bewertung":"gut"}`, `{kaputt`,
	} {
		if code, _ := freigeben(a, body); code != http.StatusBadRequest {
			t.Fatalf("%.40s: %d", body, code)
		}
	}
	if code, g := call(http.MethodGet, "/api/v1/quotes/imports/"+a, "-"); code != http.StatusOK || g.Status != "parsed" || g.BewertetAm != nil {
		t.Fatalf("nach Ablehnung: %d %+v", code, g)
	}

	// vollständige Bewertung, Freitext getrimmt, wer und wann festgehalten
	code, g := freigeben(a, `{"bewertung":4,"nacharbeit_min":30,"bewertung_text":"  Fenster passten, Türen nachgearbeitet "}`)
	if code != http.StatusOK || g.Status != "reviewed" || g.Bewertung == nil || *g.Bewertung != 4 || g.NacharbeitMin == nil || *g.NacharbeitMin != 30 ||
		g.BewertungText != "Fenster passten, Türen nachgearbeitet" || g.BewertetVonUserID == nil || *g.BewertetVonUserID != userID || g.BewertetAm == nil {
		t.Fatalf("Freigabe mit Bewertung: %d %+v", code, g)
	}
	// Detail und Liste liefern dasselbe
	if _, d := call(http.MethodGet, "/api/v1/quotes/imports/"+a, "-"); d.Bewertung == nil || *d.Bewertung != 4 || d.BewertungText != g.BewertungText || d.BewertetAm == nil {
		t.Fatalf("Detail: %+v", d)
	}
	liste, err := svc.ListImports(ctx, quotes.QuoteImportFilter{ProjectID: projectID}, "default")
	if err != nil || len(liste) != 3 {
		t.Fatalf("Liste: %d %v", len(liste), err)
	}
	for _, l := range liste {
		if (l.ID == a) != (l.Bewertung != nil && *l.Bewertung == 4 && *l.NacharbeitMin == 30 && l.BewertetVonUserID != nil) {
			t.Fatalf("Liste: %+v", l)
		}
	}
	// eine zweite Freigabe ändert die Bewertung nicht
	if code, _ := freigeben(a, `{"bewertung":1}`); code != http.StatusConflict {
		t.Fatalf("zweite Freigabe: %d", code)
	}
	if _, d := call(http.MethodGet, "/api/v1/quotes/imports/"+a, "-"); d.Bewertung == nil || *d.Bewertung != 4 {
		t.Fatalf("Bewertung überschrieben: %+v", d)
	}

	// nur Nacharbeit 0 Minuten zählt als Angabe
	if code, g := freigeben(b, `{"nacharbeit_min":0,"bewertung_text":"   "}`); code != http.StatusOK || g.Bewertung != nil || g.NacharbeitMin == nil ||
		*g.NacharbeitMin != 0 || g.BewertungText != "" || g.BewertetAm == nil || g.BewertetVonUserID == nil {
		t.Fatalf("nur Nacharbeit: %d %+v", code, g)
	}
	// ohne Body wie bisher: keine Bewertung, kein wer und wann
	if code, g := freigeben(c, "-"); code != http.StatusOK || g.Status != "reviewed" || g.Bewertung != nil || g.NacharbeitMin != nil ||
		g.BewertetAm != nil || g.BewertetVonUserID != nil {
		t.Fatalf("ohne Body: %d %+v", code, g)
	}

	// die Datenbank sichert dasselbe ab
	for _, c := range []struct{ sql, id string }{
		{`UPDATE quote_imports SET bewertung=6 WHERE id=$1`, a},
		{`UPDATE quote_imports SET nacharbeit_min=-1 WHERE id=$1`, a},
		{`UPDATE quote_imports SET bewertung_text=repeat('x', 2001) WHERE id=$1`, a},
		{`UPDATE quote_imports SET bewertet_at=NULL, bewertet_von_user_id=NULL WHERE id=$1`, a},
		{`UPDATE quote_imports SET bewertet_at=now() WHERE id=$1`, c},
	} {
		if tag, err := env.PG.Exec(ctx, c.sql, c.id); err == nil {
			t.Fatalf("Check fehlt (%d Zeilen): %s", tag.RowsAffected(), c.sql)
		}
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET bewertet_at=NULL, nacharbeit_min=NULL, bewertet_von_user_id=NULL WHERE id=$1`, b); err != nil {
		t.Fatalf("Bewertung zurücknehmen: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET bewertet_von_user_id=$2 WHERE id=$1`, c, userID); err == nil {
		t.Fatal("Check fehlt: wer ohne wann")
	}
}
