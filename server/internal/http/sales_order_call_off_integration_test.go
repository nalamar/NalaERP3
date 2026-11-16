package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/auth"
	"nalaerp3/internal/testutil"
)

// TestSalesOrderProvisionalItemCallOff: Bedarfspositionen werden als
// vorbelegter Nachtrag abgerufen, die Übersicht zeigt abgerufene und offene
// Mengen (ADR 0042 Frage 3, Backlog I.2.6b).
func TestSalesOrderProvisionalItemCallOff(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	user := "integration-call-off-" + suffix + "@example.com"
	testutil.SeedAuthUser(t, env, user, "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	token := loginIntegrationUser(t, handler, user, "Secret123!")
	customerID := createIntegrationContact(t, handler, token, itemGroupsTestCustomerBody("Abruf Kunde "+suffix))

	do := func(tok, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	items := `[
		{"description":"Normal","qty":2,"unit":"Stk","unit_price":100,"tax_code":"DE19"},
		{"description":"Altfenster ausbauen","qty":4,"unit":"Stk","unit_price":30,"tax_code":"DE19","outline_no":"01.0040","item_kind":"bedarfsposition_mit_gb"},
		{"description":"Stundenlohn","qty":8,"unit":"h","unit_price":40,"tax_code":"DE19","outline_no":"01.0050","item_kind":"bedarfsposition_ohne_gb"}
	]`
	rec := do(token, http.MethodPost, "/api/v1/quotes/", fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":%s}`, customerID, items))
	var quote struct {
		ID    string `json:"id"`
		Items []struct {
			ID        string `json:"id"`
			OutlineNo string `json:"outline_no"`
		} `json:"items"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &quote) != nil {
		t.Fatalf("Angebot: %d %s", rec.Code, rec.Body.String())
	}
	qi := map[string]string{}
	for _, it := range quote.Items {
		qi[it.OutlineNo] = it.ID
	}
	if rec := do(token, http.MethodPost, "/api/v1/quotes/"+quote.ID+"/status", `{"status":"accepted"}`); rec.Code != http.StatusOK {
		t.Fatalf("Status: %d", rec.Code)
	}
	rec = do(token, http.MethodPost, "/api/v1/quotes/"+quote.ID+"/convert-to-sales-order", "")
	var order struct {
		ID    string `json:"id"`
		Items []any  `json:"items"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &order) != nil || len(order.Items) != 1 {
		t.Fatalf("Auftrag: %d %s", rec.Code, rec.Body.String())
	}
	base := "/api/v1/sales-orders/" + order.ID
	newAddendum := func() string {
		t.Helper()
		rec := do(token, http.MethodPost, base+"/addenda", `{"begruendung":"Anordnung des AG"}`)
		var a struct {
			ID string `json:"id"`
		}
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &a) != nil {
			t.Fatalf("Nachtrag: %d %s", rec.Code, rec.Body.String())
		}
		return a.ID
	}
	type item struct {
		Description       string  `json:"description"`
		Qty               float64 `json:"qty"`
		Unit              string  `json:"unit"`
		UnitPrice         float64 `json:"unit_price"`
		TaxCode           string  `json:"tax_code"`
		OutlineNo         string  `json:"outline_no"`
		SourceQuoteItemID string  `json:"source_quote_item_id"`
	}
	callOff := func(addendumID, quoteItemID string, qty float64) (int, item) {
		t.Helper()
		rec := do(token, http.MethodPost, base+"/addenda/"+addendumID+"/call-offs", fmt.Sprintf(`{"quote_item_id":%q,"qty":%v}`, quoteItemID, qty))
		var it item
		_ = json.Unmarshal(rec.Body.Bytes(), &it)
		return rec.Code, it
	}
	type provisional struct {
		QuoteItemID  string  `json:"quote_item_id"`
		OutlineNo    string  `json:"outline_no"`
		ItemKind     string  `json:"item_kind"`
		QuotedQty    float64 `json:"quoted_qty"`
		CalledOffQty float64 `json:"called_off_qty"`
		PendingQty   float64 `json:"pending_qty"`
	}
	overview := func(stage string, want map[string][2]float64) {
		t.Helper()
		rec := do(token, http.MethodGet, base+"/provisional-items", "")
		var list []provisional
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 2 {
			t.Fatalf("%s: %d %s", stage, rec.Code, rec.Body.String())
		}
		for _, p := range list {
			w := want[p.OutlineNo]
			if p.QuoteItemID != qi[p.OutlineNo] || p.CalledOffQty != w[0] || p.PendingQty != w[1] {
				t.Fatalf("%s: %+v, erwartet abgerufen %v, offen %v", stage, p, w[0], w[1])
			}
		}
		if list[0].OutlineNo != "01.0040" || list[0].QuotedQty != 4 || list[1].ItemKind != "bedarfsposition_ohne_gb" {
			t.Fatalf("%s: Reihenfolge/Angaben %+v", stage, list)
		}
	}
	overview("vor dem Abruf", map[string][2]float64{"01.0040": {0, 0}, "01.0050": {0, 0}})

	a1 := newAddendum()
	code, got := callOff(a1, qi["01.0040"], 2)
	if code != http.StatusCreated || got != (item{Description: "Altfenster ausbauen", Qty: 2, Unit: "Stk", UnitPrice: 30, TaxCode: "DE19", OutlineNo: "01.0040", SourceQuoteItemID: qi["01.0040"]}) {
		t.Fatalf("Abruf mit Menge: %d %+v", code, got)
	}
	if code, got := callOff(a1, qi["01.0050"], 0); code != http.StatusCreated || got.Qty != 8 || got.Unit != "h" {
		t.Fatalf("Abruf ohne Menge: %d %+v", code, got)
	}
	rec = do(token, http.MethodGet, base+"/addenda/"+a1, "")
	var addendum struct {
		NetAmount float64 `json:"net_amount"`
		Items     []item  `json:"items"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &addendum) != nil || len(addendum.Items) != 2 ||
		math.Abs(addendum.NetAmount-(2*30+8*40)) > 0.005 || addendum.Items[1].OutlineNo != "01.0050" || addendum.Items[1].SourceQuoteItemID != qi["01.0050"] {
		t.Fatalf("Nachtrag lesen: %d %s", rec.Code, rec.Body.String())
	}
	overview("im Entwurf", map[string][2]float64{"01.0040": {0, 2}, "01.0050": {0, 8}})
	if rec := do(token, http.MethodPost, base+"/addenda/"+a1+"/submit", ""); rec.Code != http.StatusOK {
		t.Fatalf("beantragen: %d", rec.Code)
	}
	overview("beantragt", map[string][2]float64{"01.0040": {0, 2}, "01.0050": {0, 8}})
	if rec := do(token, http.MethodPost, base+"/addenda/"+a1+"/decide", `{"accepted":true}`); rec.Code != http.StatusOK {
		t.Fatalf("annehmen: %d %s", rec.Code, rec.Body.String())
	}
	// mehrfacher Abruf
	a2 := newAddendum()
	if code, _ := callOff(a2, qi["01.0040"], 1.5); code != http.StatusCreated {
		t.Fatalf("zweiter Abruf: %d", code)
	}
	overview("angenommen plus Entwurf", map[string][2]float64{"01.0040": {2, 1.5}, "01.0050": {8, 0}})

	// Fehler: keine Bedarfsposition, unbekannt, kein UUID, negative Menge, entschiedener Nachtrag
	var normalID string
	for oz, id := range qi {
		if oz == "" {
			normalID = id
		}
	}
	for name, c := range map[string]struct {
		addendum, quoteItem string
		qty                 float64
		want                int
	}{
		"Normalposition":        {a2, normalID, 1, http.StatusBadRequest},
		"unbekannt":             {a2, uuid.NewString(), 1, http.StatusBadRequest},
		"kein UUID":             {a2, "abc", 1, http.StatusBadRequest},
		"negative Menge":        {a2, qi["01.0040"], -1, http.StatusBadRequest},
		"angenommener Nachtrag": {a1, qi["01.0040"], 1, http.StatusBadRequest},
	} {
		if code, _ := callOff(c.addendum, c.quoteItem, c.qty); code != c.want {
			t.Fatalf("%s: %d, erwartet %d", name, code, c.want)
		}
	}
	overview("nach Fehlern unverändert", map[string][2]float64{"01.0040": {2, 1.5}, "01.0050": {8, 0}})

	// fremder Mandant
	otherCompany := "itest-call-off-" + suffix
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, 'Andere Firma')`, otherCompany); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("Secret123!")
	if err != nil {
		t.Fatal(err)
	}
	otherUser, otherEmail := "itest-call-off-user-"+suffix, "integration-call-off-other-"+suffix+"@example.com"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO users (id, email, username, password_hash, first_name, last_name, display_name, locale, timezone, is_active, is_locked, company_id)
		VALUES ($1,$2,$2,$3,'Andere','Firma','Andere Firma','de-DE','Europe/Berlin',true,false,$4)`, otherUser, otherEmail, hash, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, r.id FROM roles r WHERE r.code = 'admin'`, otherUser); err != nil {
		t.Fatal(err)
	}
	other := loginIntegrationUser(t, handler, otherEmail, "Secret123!")
	if rec := do(other, http.MethodGet, base+"/provisional-items", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("fremder Mandant Übersicht: %d", rec.Code)
	}
	if rec := do(other, http.MethodPost, base+"/addenda/"+a2+"/call-offs", fmt.Sprintf(`{"quote_item_id":%q,"qty":1}`, qi["01.0040"])); rec.Code != http.StatusNotFound {
		t.Fatalf("fremder Mandant Abruf: %d", rec.Code)
	}
	overview("nach fremdem Mandanten unverändert", map[string][2]float64{"01.0040": {2, 1.5}, "01.0050": {8, 0}})
}
