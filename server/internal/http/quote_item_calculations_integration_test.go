package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func calculationsTestCustomerBody(name string) map[string]any {
	return map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     name,
		"waehrung": "EUR",
	}
}

// firstQuoteItemID holt die ID der ersten Position eines Angebots per GET
// - createIntegrationQuote liefert nur die Angebots-ID zurueck.
func firstQuoteItemID(t *testing.T, handler http.Handler, accessToken, quoteID string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote get, got %d with body %s", rec.Code, rec.Body.String())
	}
	var quote struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &quote); err != nil {
		t.Fatalf("decode quote response: %v", err)
	}
	if len(quote.Items) == 0 {
		t.Fatal("expected at least one quote item")
	}
	return quote.Items[0].ID
}

func TestQuoteItemCalculationUpsertGetApplyFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-calc@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-calc@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, calculationsTestCustomerBody("Kalkulation Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)
	itemID := firstQuoteItemID(t, handler, accessToken, quoteID)

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/calculation", bytes.NewReader([]byte(`{
		"material_cost":100,
		"material_zuschlag_percent":15,
		"lohn_stunden":2,
		"lohn_stundensatz":40,
		"lohn_zuschlag_percent":80,
		"fremdleistung_cost":50,
		"fremdleistung_zuschlag_percent":10
	}`)))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+accessToken)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", putRec.Code, putRec.Body.String())
	}
	var upserted struct {
		MaterialTotal       float64 `json:"material_total"`
		LohnCost            float64 `json:"lohn_cost"`
		LohnTotal           float64 `json:"lohn_total"`
		FremdleistungTotal  float64 `json:"fremdleistung_total"`
		CalculatedUnitPrice float64 `json:"calculated_unit_price"`
	}
	if err := json.Unmarshal(putRec.Body.Bytes(), &upserted); err != nil {
		t.Fatalf("decode upsert response: %v", err)
	}
	if upserted.MaterialTotal != 115 {
		t.Fatalf("expected material_total 115, got %v", upserted.MaterialTotal)
	}
	if upserted.LohnCost != 80 {
		t.Fatalf("expected lohn_cost 80, got %v", upserted.LohnCost)
	}
	if upserted.LohnTotal != 144 {
		t.Fatalf("expected lohn_total 144, got %v", upserted.LohnTotal)
	}
	if upserted.FremdleistungTotal != 55 {
		t.Fatalf("expected fremdleistung_total 55, got %v", upserted.FremdleistungTotal)
	}
	if upserted.CalculatedUnitPrice != 314 {
		t.Fatalf("expected calculated_unit_price 314, got %v", upserted.CalculatedUnitPrice)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/calculation", nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}
	var fetched struct {
		CalculatedUnitPrice float64 `json:"calculated_unit_price"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.CalculatedUnitPrice != 314 {
		t.Fatalf("expected persisted calculated_unit_price 314, got %v", fetched.CalculatedUnitPrice)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/apply-calculation", nil)
	applyReq.Header.Set("Authorization", "Bearer "+accessToken)
	applyRec := httptest.NewRecorder()
	handler.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", applyRec.Code, applyRec.Body.String())
	}
	var applied struct {
		Items []struct {
			ID        string  `json:"id"`
			UnitPrice float64 `json:"unit_price"`
		} `json:"items"`
	}
	if err := json.Unmarshal(applyRec.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode apply response: %v", err)
	}
	found := false
	for _, item := range applied.Items {
		if item.ID == itemID {
			found = true
			if item.UnitPrice != 314 {
				t.Fatalf("expected applied unit_price 314, got %v", item.UnitPrice)
			}
		}
	}
	if !found {
		t.Fatalf("expected item %q in applied quote response, got %#v", itemID, applied.Items)
	}

	// Direkte SQL-Pruefung: die Anwenden-Entscheidung muss in der
	// bestehenden quote_item_price_decisions protokolliert sein, mit dem
	// neuen decision_type - kein zweites, unabhaengiges Protokoll.
	var decisionType string
	var appliedUnitPrice float64
	if err := env.PG.QueryRow(t.Context(), `
		SELECT decision_type, applied_unit_price
		FROM quote_item_price_decisions
		WHERE quote_item_id=$1 AND decision_type='calculation_scheme_applied'
		ORDER BY created_at DESC LIMIT 1
	`, itemID).Scan(&decisionType, &appliedUnitPrice); err != nil {
		t.Fatalf("expected a calculation_scheme_applied price decision row: %v", err)
	}
	if appliedUnitPrice != 314 {
		t.Fatalf("expected price decision applied_unit_price 314, got %v", appliedUnitPrice)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/calculation", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteRec.Code, deleteRec.Body.String())
	}

	// "keine Kalkulation..." klassifiziert wie das analoge, bereits
	// bestehende "keine Preisentscheidung..." (ApplyTargetUnitPriceForQuoteItem)
	// als 400, nicht 404 - konsistent mit dem etablierten Muster fuer
	// "Voraussetzung fehlt" statt "Ressource nicht gefunden".
	getAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/calculation", nil)
	getAfterDeleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	getAfterDeleteRec := httptest.NewRecorder()
	handler.ServeHTTP(getAfterDeleteRec, getAfterDeleteReq)
	if getAfterDeleteRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 after delete, got %d with body %s", getAfterDeleteRec.Code, getAfterDeleteRec.Body.String())
	}
}

func TestQuoteItemCalculationUpsertRejectsNegativeValues(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-calc-negative@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-calc-negative@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, calculationsTestCustomerBody("Negativ Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)
	itemID := firstQuoteItemID(t, handler, accessToken, quoteID)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/calculation", bytes.NewReader([]byte(`{"material_cost":-1}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestQuoteItemCalculationApplyRejectsWithoutExistingCalculation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-calc-noexist@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-calc-noexist@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, calculationsTestCustomerBody("Ohne Kalkulation Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)
	itemID := firstQuoteItemID(t, handler, accessToken, quoteID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/apply-calculation", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestQuoteItemCalculationRejectedOnNonDraftQuote(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-calc-nondraft@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-calc-nondraft@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, calculationsTestCustomerBody("Nicht-Entwurf Kalkulation Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)
	itemID := firstQuoteItemID(t, handler, accessToken, quoteID)

	if _, err := env.PG.Exec(t.Context(), `UPDATE quotes SET status='sent' WHERE id=$1`, quoteID); err != nil {
		t.Fatalf("seed sent quote status: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/quotes/"+quoteID+"/items/"+itemID+"/calculation", bytes.NewReader([]byte(`{"material_cost":10}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if body.Error.Message != "nur Entwürfe sind bearbeitbar" {
		t.Fatalf("unexpected validation message, got %q", body.Error.Message)
	}
}

func TestQuoteCalculationSettingsRoundtripIncludesNewDefaultFields(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-calc-settings@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-calc-settings@example.com", "Secret123!")

	// target_margin_percent bewusst NICHT gesetzt (0 -> Upsert-interner
	// Fallback auf 20, siehe settings.validateQuoteCalculationDefaults) -
	// quote_calculation_settings ist eine globale Singleton-Zeile, die
	// zwischen Tests DESSELBEN Testlaufs nicht zurueckgesetzt wird; dieser
	// Test soll nur die vier NEUEN B.3-Felder pruefen, ohne die von
	// TestQuoteCalculationSettingsFlow vorausgesetzte Zielmarge zu stoeren.
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/quote-calculation", bytes.NewReader([]byte(`{
		"default_material_zuschlag_percent":15,
		"default_lohn_zuschlag_percent":80,
		"default_fremdleistung_zuschlag_percent":10,
		"default_lohn_stundensatz":45.5
	}`)))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+accessToken)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/quote-calculation", nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}
	var fetched struct {
		TargetMarginPercent                 float64 `json:"target_margin_percent"`
		DefaultMaterialZuschlagPercent      float64 `json:"default_material_zuschlag_percent"`
		DefaultLohnZuschlagPercent          float64 `json:"default_lohn_zuschlag_percent"`
		DefaultFremdleistungZuschlagPercent float64 `json:"default_fremdleistung_zuschlag_percent"`
		DefaultLohnStundensatz              float64 `json:"default_lohn_stundensatz"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.DefaultMaterialZuschlagPercent != 15 || fetched.DefaultLohnZuschlagPercent != 80 || fetched.DefaultFremdleistungZuschlagPercent != 10 || fetched.DefaultLohnStundensatz != 45.5 {
		t.Fatalf("expected persisted default calculation settings, got %#v", fetched)
	}
}
