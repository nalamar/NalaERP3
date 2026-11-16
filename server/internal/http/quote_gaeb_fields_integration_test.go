package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// TestQuoteGAEBFieldsAndDeeperHierarchy: OZ, Kurz-/Langtext, Art, Pauschale
// und Grundposition an Angebotspositionen, OZ an Gruppen, Untertitel unter
// Untertiteln (ADR 0041 Fragen 1 bis 3, Backlog I.2.4a).
func TestQuoteGAEBFieldsAndDeeperHierarchy(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	suffix := uuid.NewString()[:8]
	user := "integration-quote-gaeb-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("GAEB Felder Kunde "+suffix))

	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	var quote struct {
		ID    string                  `json:"id"`
		Items []quotes.QuoteItemInput `json:"items"`
	}
	items := `[
		{"description":"Fenster\n\nAluminium","qty":2,"unit":"Stk","unit_price":100,"tax_code":"DE19",
		 "outline_no":" 01.0010 ","short_text":"Fenster","long_text":"Aluminium","item_kind":"grundposition"},
		{"description":"wie vor, jedoch zweiflügelig","qty":2,"unit":"Stk","unit_price":120,"tax_code":"DE19",
		 "outline_no":"01.0020","short_text":"wie vor, jedoch zweiflügelig","item_kind":" Alternativposition ","base_outline_no":" 01.0010 "},
		{"description":"Baustelleneinrichtung","qty":1,"unit":"psch","unit_price":500,"tax_code":"DE19",
		 "outline_no":"01.0030","item_kind":"bedarfsposition_ohne_gb","is_lump_sum":true},
		{"description":"Von Hand","qty":1,"unit":"Stk","unit_price":10,"tax_code":"DE19"}
	]`
	rec := do(http.MethodPost, "/api/v1/quotes/", fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":%s}`, customerID, items))
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &quote) != nil {
		t.Fatalf("Anlegen: %d %s", rec.Code, rec.Body.String())
	}
	check := func(stage string, got []quotes.QuoteItemInput) {
		t.Helper()
		if len(got) != 4 {
			t.Fatalf("%s: %d Positionen", stage, len(got))
		}
		g, a, b, h := got[0], got[1], got[2], got[3]
		if g.OutlineNo != "01.0010" || g.ShortText != "Fenster" || g.LongText != "Aluminium" || g.ItemKind != "grundposition" || g.IsLumpSum || g.BaseOutlineNo != "" {
			t.Fatalf("%s Grundposition: %+v", stage, g)
		}
		if a.ItemKind != "alternativposition" || a.BaseOutlineNo != "01.0010" || a.OutlineNo != "01.0020" || a.LongText != "" {
			t.Fatalf("%s Alternative: %+v", stage, a)
		}
		if b.ItemKind != "bedarfsposition_ohne_gb" || !b.IsLumpSum || b.ShortText != "" {
			t.Fatalf("%s Bedarf: %+v", stage, b)
		}
		if h.ItemKind != "normal" || h.OutlineNo != "" || h.IsLumpSum {
			t.Fatalf("%s von Hand: %+v", stage, h)
		}
	}
	check("Anlegen", quote.Items)
	// Speichern des Entwurfs übernimmt die Angaben
	rec = do(http.MethodPatch, "/api/v1/quotes/"+quote.ID, fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":%s}`, customerID, items))
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &quote) != nil {
		t.Fatalf("Speichern: %d %s", rec.Code, rec.Body.String())
	}
	check("Speichern", quote.Items)
	rec = do(http.MethodGet, "/api/v1/quotes/"+quote.ID, "")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &quote) != nil {
		t.Fatalf("Lesen: %d", rec.Code)
	}
	check("Lesen", quote.Items)

	// Fehler: unbekannte Art, Grundposition bei Nicht-Alternative
	for name, item := range map[string]string{
		"Art":              `{"description":"x","item_kind":"eventual"}`,
		"Grund bei Bedarf": `{"description":"x","item_kind":"bedarfsposition_mit_gb","base_outline_no":"01.0010"}`,
		"Grund ohne Art":   `{"description":"x","base_outline_no":"01.0010"}`,
	} {
		rec = do(http.MethodPatch, "/api/v1/quotes/"+quote.ID, fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":[%s]}`, customerID, item))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	rec = do(http.MethodGet, "/api/v1/quotes/"+quote.ID, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &quote)
	check("nach abgelehntem Speichern", quote.Items)

	// Gliederung: OZ an Gruppen, Untertitel unter Untertitel erlaubt, nicht unter Los
	createGroup := func(body string) (int, quotes.QuoteItemGroup) {
		t.Helper()
		rec := do(http.MethodPost, "/api/v1/quotes/"+quote.ID+"/item-groups", body)
		var g quotes.QuoteItemGroup
		_ = json.Unmarshal(rec.Body.Bytes(), &g)
		return rec.Code, g
	}
	code, los := createGroup(`{"kind":"los","bezeichnung":"Los","outline_no":" 1 "}`)
	if code != http.StatusCreated || los.OutlineNo != "1" {
		t.Fatalf("Los: %d %+v", code, los)
	}
	_, titel := createGroup(fmt.Sprintf(`{"kind":"titel","bezeichnung":"Titel","outline_no":"1.01","parent_group_id":%q}`, los.ID))
	_, unter := createGroup(fmt.Sprintf(`{"kind":"untertitel","bezeichnung":"Unter","parent_group_id":%q}`, titel.ID))
	code, tiefer := createGroup(fmt.Sprintf(`{"kind":"untertitel","bezeichnung":"Tiefer","outline_no":"1.01.02.03","parent_group_id":%q}`, unter.ID))
	if code != http.StatusCreated || tiefer.OutlineNo != "1.01.02.03" || unter.OutlineNo != "" {
		t.Fatalf("Untertitel unter Untertitel: %d %+v %+v", code, tiefer, unter)
	}
	if code, _ := createGroup(fmt.Sprintf(`{"kind":"untertitel","bezeichnung":"Falsch","parent_group_id":%q}`, los.ID)); code != http.StatusBadRequest {
		t.Fatalf("Untertitel unter Los: %d", code)
	}
	var groups []quotes.QuoteItemGroup
	rec = do(http.MethodGet, "/api/v1/quotes/"+quote.ID+"/item-groups", "")
	if json.Unmarshal(rec.Body.Bytes(), &groups) != nil || len(groups) != 4 || groups[1].OutlineNo != "1.01" {
		t.Fatalf("Liste: %s", rec.Body.String())
	}
	rec = do(http.MethodPatch, "/api/v1/quotes/"+quote.ID+"/item-groups/"+titel.ID.String(), `{"bezeichnung":"Titel neu"}`)
	var updated quotes.QuoteItemGroup
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &updated) != nil || updated.OutlineNo != "1.01" || updated.Bezeichnung != "Titel neu" {
		t.Fatalf("Ändern: %d %s", rec.Code, rec.Body.String())
	}

	// Prüfungen der Datenbank (Migration 117)
	for name, sql := range map[string]string{
		"Art":              `UPDATE quote_items SET item_kind='eventual' WHERE quote_id=$1 AND base_outline_no=''`,
		"Grund bei Normal": `UPDATE quote_items SET base_outline_no='01.0010' WHERE quote_id=$1 AND item_kind='normal'`,
	} {
		if _, err := env.PG.Exec(context.Background(), sql, quote.ID); err == nil || !strings.Contains(err.Error(), "check") {
			t.Fatalf("Datenbank nimmt %s an: %v", name, err)
		}
	}
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM quote_items WHERE quote_id=$1 AND item_kind='normal' AND outline_no='' AND base_outline_no=''`, quote.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("Standardwerte: %d %v", n, err)
	}
}
