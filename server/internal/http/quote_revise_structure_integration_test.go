package apihttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// TestQuoteReviseKeepsStructureAndGAEBFields: eine Revision nimmt Gliederung
// und GAEB-Felder mit (ADR 0041 Frage 6, Backlog I.2.4e1).
func TestQuoteReviseKeepsStructureAndGAEBFields(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	suffix := uuid.NewString()[:8]
	user := "integration-quote-revise-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Revision Kunde "+suffix))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)

	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	createGroup := func(qID, body string) quotes.QuoteItemGroup {
		t.Helper()
		rec := do(http.MethodPost, "/api/v1/quotes/"+qID+"/item-groups", body)
		var g quotes.QuoteItemGroup
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &g) != nil {
			t.Fatalf("Gruppe: %d %s", rec.Code, rec.Body.String())
		}
		return g
	}
	listGroups := func(qID string) []quotes.QuoteItemGroup {
		t.Helper()
		rec := do(http.MethodGet, "/api/v1/quotes/"+qID+"/item-groups", "")
		var out []quotes.QuoteItemGroup
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("Gruppen: %d %s", rec.Code, rec.Body.String())
		}
		return out
	}
	// Eltern mit höherer sort_order als ihre Kinder
	los := createGroup(quoteID, `{"kind":"los","bezeichnung":"Los","outline_no":"1","sort_order":9}`)
	titel := createGroup(quoteID, fmt.Sprintf(`{"kind":"titel","bezeichnung":"Titel","outline_no":"1.01","sort_order":5,"parent_group_id":%q}`, los.ID))
	unter := createGroup(quoteID, fmt.Sprintf(`{"kind":"untertitel","bezeichnung":"Unter","outline_no":"1.01.01","sort_order":1,"parent_group_id":%q}`, titel.ID))
	createGroup(quoteID, `{"kind":"titel","bezeichnung":"Leer","sort_order":2}`)

	items := fmt.Sprintf(`[
		{"description":"Grund","qty":1,"unit":"Stk","unit_price":100,"tax_code":"DE19","group_id":%q,"outline_no":"1.01.01.0010","short_text":"Grund","long_text":"Lang","item_kind":"grundposition"},
		{"description":"Alternative","qty":1,"unit":"Stk","unit_price":120,"tax_code":"DE19","group_id":%q,"outline_no":"1.01.01.0020","item_kind":"alternativposition","base_outline_no":"1.01.01.0010"},
		{"description":"Pauschale","qty":1,"unit":"psch","unit_price":50,"tax_code":"DE19","group_id":%q,"outline_no":"1.01.0030","item_kind":"bedarfsposition_mit_gb","is_lump_sum":true},
		{"description":"Ohne Gruppe","qty":2,"unit":"Stk","unit_price":10,"tax_code":"DE19"}
	]`, unter.ID, unter.ID, titel.ID)
	var source struct {
		NetAmount float64                 `json:"net_amount"`
		Items     []quotes.QuoteItemInput `json:"items"`
	}
	rec := do(http.MethodPatch, "/api/v1/quotes/"+quoteID, fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":%s}`, customerID, items))
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &source) != nil {
		t.Fatalf("Speichern: %d %s", rec.Code, rec.Body.String())
	}
	sourceGroups := listGroups(quoteID)

	revise := func(qID string) quotes.ReviseResult {
		t.Helper()
		rec := do(http.MethodPost, "/api/v1/quotes/"+qID+"/revise", "")
		var out quotes.ReviseResult
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.RevisedQuote == nil {
			t.Fatalf("Revision: %d %s", rec.Code, rec.Body.String())
		}
		return out
	}
	check := func(stage, fromID, toID string, fromItems, toItems []quotes.QuoteItemInput, toNet float64) {
		t.Helper()
		from, to := listGroups(fromID), listGroups(toID)
		if len(to) != len(from) || len(to) != 4 {
			t.Fatalf("%s: %d Gruppen, Quelle %d", stage, len(to), len(from))
		}
		fromByID := map[uuid.UUID]quotes.QuoteItemGroup{}
		for _, g := range from {
			fromByID[g.ID] = g
		}
		toByID := map[uuid.UUID]quotes.QuoteItemGroup{}
		for _, g := range to {
			toByID[g.ID] = g
		}
		mapped := map[uuid.UUID]uuid.UUID{} // alt -> neu über die Bezeichnung (eindeutig im Test)
		for _, g := range to {
			if _, clash := fromByID[g.ID]; clash {
				t.Fatalf("%s: Gruppen-ID übernommen statt neu", stage)
			}
			for _, f := range from {
				if f.Bezeichnung == g.Bezeichnung {
					if f.Kind != g.Kind || f.OutlineNo != g.OutlineNo || f.SortOrder != g.SortOrder {
						t.Fatalf("%s: Gruppe %+v, Quelle %+v", stage, g, f)
					}
					mapped[f.ID] = g.ID
				}
			}
		}
		for _, f := range from {
			g := toByID[mapped[f.ID]]
			switch {
			case f.ParentGroupID == nil && g.ParentGroupID != nil,
				f.ParentGroupID != nil && (g.ParentGroupID == nil || *g.ParentGroupID != mapped[*f.ParentGroupID]):
				t.Fatalf("%s: Eltern von %s falsch", stage, f.Bezeichnung)
			}
		}
		if len(toItems) != len(fromItems) {
			t.Fatalf("%s: %d Positionen", stage, len(toItems))
		}
		for i := range fromItems {
			f, g := fromItems[i], toItems[i]
			wantGroup := ""
			if f.GroupID != "" {
				wantGroup = mapped[uuid.MustParse(f.GroupID)].String()
			}
			if g.GroupID != wantGroup || g.OutlineNo != f.OutlineNo || g.ShortText != f.ShortText || g.LongText != f.LongText ||
				g.ItemKind != f.ItemKind || g.IsLumpSum != f.IsLumpSum || g.BaseOutlineNo != f.BaseOutlineNo || g.Description != f.Description {
				t.Fatalf("%s: Position %d %+v, Quelle %+v", stage, i, g, f)
			}
		}
		if toNet != source.NetAmount || source.NetAmount != 120 {
			t.Fatalf("%s: Summe %v, Quelle %v", stage, toNet, source.NetAmount)
		}
	}
	first := revise(quoteID)
	check("erste Revision", quoteID, first.RevisedQuote.ID.String(), source.Items, first.RevisedQuote.Items, first.RevisedQuote.NetAmount)
	// Quelle unverändert
	if after := listGroups(quoteID); len(after) != len(sourceGroups) {
		t.Fatalf("Quelle verändert: %d Gruppen", len(after))
	}
	// Revision der Revision
	second := revise(first.RevisedQuote.ID.String())
	check("zweite Revision", first.RevisedQuote.ID.String(), second.RevisedQuote.ID.String(), first.RevisedQuote.Items, second.RevisedQuote.Items, second.RevisedQuote.NetAmount)
}
