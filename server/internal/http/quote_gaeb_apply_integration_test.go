package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// TestQuoteGAEBImportApplyCopiesStructure: die Übernahme eines GAEB-Imports
// ins Angebot übernimmt Gliederung, OZ, Texte, Art, Pauschale und
// Grundposition (ADR 0041, Backlog I.2.4c).
func TestQuoteGAEBImportApplyCopiesStructure(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	user := "integration-gaeb-apply-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("GAEB Übernahme Kunde "+suffix))
	projectID := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, kunde_id, status, company_id) VALUES ($1,$2,'GAEB Übernahme Projekt',$3,'angebot','default')`,
		projectID, "PRJ-GAEB-APPLY-"+suffix, customerID); err != nil {
		t.Fatal(err)
	}
	svc := quotes.NewService(env.PG, nil).WithMongo(env.Mongo, env.Cfg.MongoDB)
	apply := func(importID string) (int, quotes.QuoteImportApplyResult) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/"+importID+"/apply", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var out quotes.QuoteImportApplyResult
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	listGroups := func(quoteID string) []quotes.QuoteItemGroup {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID+"/item-groups", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var out []quotes.QuoteItemGroup
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("Gruppen: %d %s", rec.Code, rec.Body.String())
		}
		return out
	}

	raw, err := os.ReadFile("../quotes/eval/testdata/lv-02.x83")
	if err != nil {
		t.Fatal(err)
	}
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
	// Grund- und Alternativposition suchen; die erste Position ablehnen, eine Bezeichnung leeren
	var alt, grund string
	if err := env.PG.QueryRow(ctx, `SELECT position_no, base_position_no FROM quote_import_items WHERE import_id=$1 AND item_kind='alternativposition' ORDER BY sort_order LIMIT 1`,
		imp.ID).Scan(&alt, &grund); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET review_status = CASE WHEN position_no='1.01.0010' THEN 'rejected' ELSE 'accepted' END WHERE import_id=$1`, imp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_groups SET bezeichnung='' WHERE import_id=$1 AND outline_no='1.02'`, imp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkImportReviewed(ctx, imp.ID, "default"); err != nil {
		t.Fatal(err)
	}

	code, applied := apply(imp.ID)
	if code != http.StatusCreated || applied.Quote == nil || applied.Import == nil || applied.Import.Status != "applied" {
		t.Fatalf("Übernahme: %d %+v", code, applied)
	}
	quoteID := applied.Quote.ID.String()

	// Gliederung vollständig, in Reihenfolge, mit Eltern und OZ
	groups := listGroups(quoteID)
	if len(groups) != len(result.Groups) {
		t.Fatalf("%d Gruppen, erwartet %d", len(groups), len(result.Groups))
	}
	byOutline := map[string]quotes.QuoteItemGroup{}
	for i, g := range groups {
		want := result.Groups[i]
		if g.OutlineNo != want.OutlineNo || g.Kind != want.Kind || g.SortOrder != i+1 {
			t.Fatalf("Gruppe %d: %+v, erwartet %+v", i, g, want)
		}
		byOutline[g.OutlineNo] = g
	}
	for _, want := range result.Groups {
		g := byOutline[want.OutlineNo]
		if want.ParentOutlineNo == "" {
			if g.ParentGroupID != nil {
				t.Fatalf("%s hat Eltern", want.OutlineNo)
			}
		} else if g.ParentGroupID == nil || *g.ParentGroupID != byOutline[want.ParentOutlineNo].ID {
			t.Fatalf("%s: falsche Eltern", want.OutlineNo)
		}
	}
	if byOutline["1"].Bezeichnung != "Fassade" || byOutline["1.02"].Bezeichnung != "1.02" {
		t.Fatalf("Bezeichnungen: %q %q", byOutline["1"].Bezeichnung, byOutline["1.02"].Bezeichnung)
	}

	// Positionen: nur angenommene, mit Gruppe, OZ, Texten, Art, Pauschale, Grundposition
	items := applied.Quote.Items
	if len(items) != len(result.Items)-1 {
		t.Fatalf("%d Positionen, erwartet %d", len(items), len(result.Items)-1)
	}
	byOZ := map[string]quotes.QuoteItemInput{}
	for _, it := range items {
		byOZ[it.OutlineNo] = it
	}
	if _, ok := byOZ["1.01.0010"]; ok {
		t.Fatal("abgelehnte Position übernommen")
	}
	lumpSums := 0
	for _, want := range result.Items[1:] {
		got, ok := byOZ[want.PositionNo]
		if !ok {
			t.Fatalf("Position %s fehlt", want.PositionNo)
		}
		if got.ShortText != want.ShortText || got.LongText != want.LongText || got.ItemKind != want.ItemKind || got.IsLumpSum != want.IsLumpSum ||
			got.BaseOutlineNo != want.BasePositionNo || got.Description != want.Description || got.GroupID != byOutline[want.OutlineNo].ID.String() {
			t.Fatalf("Position %s: %+v, erwartet %+v", want.PositionNo, got, want)
		}
		if got.IsLumpSum {
			lumpSums++
		}
	}
	if byOZ[alt].BaseOutlineNo != grund || byOZ[alt].ItemKind != "alternativposition" || byOZ[grund].ItemKind != "grundposition" || lumpSums == 0 {
		t.Fatalf("Alternative/Grund/Pauschale: %+v %+v %d", byOZ[alt], byOZ[grund], lumpSums)
	}
	// die Verknüpfung Import -> Angebot bleibt
	var links int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_import_item_links WHERE quote_id=$1`, quoteID).Scan(&links); err != nil || links != len(items) {
		t.Fatalf("Verknüpfungen: %d %v", links, err)
	}
	// zweite Übernahme abgelehnt, keine weiteren Gruppen
	if code, _ := apply(imp.ID); code != http.StatusConflict {
		t.Fatalf("zweite Übernahme: %d", code)
	}
	var allGroups int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_item_groups WHERE quote_id=$1`, quoteID).Scan(&allGroups); err != nil || allGroups != len(groups) {
		t.Fatalf("Gruppen nach zweiter Übernahme: %d %v", allGroups, err)
	}

	// Import ohne Gliederung (bisheriger Aufruf): Positionen ohne Gruppe, Art normal
	flat, err := svc.CreateGAEBImport(ctx, quotes.QuoteImportCreateInput{ProjectID: projectID, ContactID: customerID}, strings.NewReader("x"), "flat.x83", "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveImportParseResult(ctx, flat.ID, "v", "f", []quotes.QuoteImportItemInput{{PositionNo: "1", OutlineNo: "01", Description: "Fenster", Qty: 2, Unit: "Stk"}}, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET review_status='accepted' WHERE import_id=$1`, flat.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkImportReviewed(ctx, flat.ID, "default"); err != nil {
		t.Fatal(err)
	}
	code, flatApplied := apply(flat.ID)
	if code != http.StatusCreated || len(flatApplied.Quote.Items) != 1 {
		t.Fatalf("ohne Gliederung: %d %+v", code, flatApplied)
	}
	if it := flatApplied.Quote.Items[0]; it.GroupID != "" || it.ItemKind != "normal" || it.OutlineNo != "1" || it.BaseOutlineNo != "" {
		t.Fatalf("ohne Gliederung: %+v", it)
	}
	if g := listGroups(flatApplied.Quote.ID.String()); len(g) != 0 {
		t.Fatalf("ohne Gliederung: %d Gruppen", len(g))
	}
}
