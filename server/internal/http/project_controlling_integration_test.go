package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/accounting"
	"nalaerp3/internal/projects"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/settings"
	"nalaerp3/internal/testutil"
)

// buildProjectControlling hat zum Zeitpunkt von E.2.3.2 noch KEIN
// HTTP-Wiring (das folgt in E.2.3.3) - diese Datei ruft die Funktion
// direkt auf (unexported, aber im selben Package "apihttp" wie dieser
// Test), analog zum bereits etablierten Direkt-Aufruf-Muster fuer
// Funktionen ohne eigenen Handler (D.3.3.2, E.1.3). Die Ist-Kosten-Seite
// braucht eine Buchung mit kostenstelle_id - da JournalService selbst
// keinen HTTP-Handler hat, wird sie direkt ueber accounting.JournalService
// gebucht (hybrider Test: Fixtures ueber HTTP, Buchung + Aufruf der zu
// testenden Funktion direkt in Go).

func TestBuildProjectControllingAggregatesSollUndIstKosten(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "controlling-full@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "controlling-full@example.com", "Secret123!")
	ctx := context.Background()

	nonce := uuid.NewString()[:8]

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Controlling-Testkunde " + nonce,
		"email":    "controlling-" + nonce + "@example.com",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{"name":"Controlling-Testprojekt","kunde_id":"`+customerID+`","status":"angebot"}`)))
	createProjectReq.Header.Set("Authorization", "Bearer "+accessToken)
	createProjectReq.Header.Set("Content-Type", "application/json")
	createProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(createProjectRec, createProjectReq)
	if createProjectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", createProjectRec.Code, createProjectRec.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"contact_id":"`+customerID+`",
		"project_id":"`+project.ID+`",
		"currency":"EUR",
		"items":[
			{"description":"Testposition","qty":2,"unit":"Stk","unit_price":500,"tax_code":"DE19"}
		]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}
	var createdQuote struct {
		ID          string  `json:"id"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 {
		t.Fatalf("expected exactly 1 quote item, got %+v", createdQuote.Items)
	}
	itemID := createdQuote.Items[0].ID

	calcReq := httptest.NewRequest(http.MethodPut, "/api/v1/quotes/"+createdQuote.ID+"/items/"+itemID+"/calculation", bytes.NewReader([]byte(`{
		"material_cost":100,
		"material_zuschlag_percent":15,
		"lohn_stunden":2,
		"lohn_stundensatz":40,
		"lohn_zuschlag_percent":80,
		"fremdleistung_cost":50,
		"fremdleistung_zuschlag_percent":10
	}`)))
	calcReq.Header.Set("Authorization", "Bearer "+accessToken)
	calcReq.Header.Set("Content-Type", "application/json")
	calcRec := httptest.NewRecorder()
	handler.ServeHTTP(calcRec, calcReq)
	if calcRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for calculation upsert, got %d with body %s", calcRec.Code, calcRec.Body.String())
	}
	// Erwartete Kalkulation je Einheit: material_total=115, lohn_total=144,
	// fremdleistung_total=55 -> calculated_unit_price=314; bei qty=2 also
	// soll_kosten=628 (siehe B.3.3, dieselbe Formel bereits dort exakt
	// nachgerechnet).
	const expectedSollKosten = 628.0

	projSvc := projects.NewService(env.PG)
	quoteSvc := quotes.NewService(env.PG, settings.NewNumberingService(env.PG))
	companyID := "default"

	beforeKostenstelle, err := buildProjectControlling(ctx, project.ID, env.PG, projSvc, quoteSvc, companyID)
	if err != nil {
		t.Fatalf("buildProjectControlling (ohne Kostenstelle): %v", err)
	}
	if beforeKostenstelle.SollErloes != createdQuote.GrossAmount {
		t.Fatalf("expected soll_erloes=%v (Angebots-Bruttosumme), got %v", createdQuote.GrossAmount, beforeKostenstelle.SollErloes)
	}
	if beforeKostenstelle.IstErloes != 0 {
		t.Fatalf("expected ist_erloes=0 (keine Rechnung gestellt), got %v", beforeKostenstelle.IstErloes)
	}
	if beforeKostenstelle.SollKosten != expectedSollKosten {
		t.Fatalf("expected soll_kosten=%v, got %v", expectedSollKosten, beforeKostenstelle.SollKosten)
	}
	if beforeKostenstelle.IstKosten != nil {
		t.Fatalf("expected ist_kosten=nil ohne zugeordnete Kostenstelle, got %v", *beforeKostenstelle.IstKosten)
	}
	if beforeKostenstelle.DeckungsbeitragIst != nil {
		t.Fatalf("expected deckungsbeitrag_ist=nil ohne zugeordnete Kostenstelle, got %v", *beforeKostenstelle.DeckungsbeitragIst)
	}
	expectedDeckungsbeitragSoll := createdQuote.GrossAmount - expectedSollKosten
	if beforeKostenstelle.DeckungsbeitragSoll != expectedDeckungsbeitragSoll {
		t.Fatalf("expected deckungsbeitrag_soll=%v, got %v", expectedDeckungsbeitragSoll, beforeKostenstelle.DeckungsbeitragSoll)
	}

	createCcReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-centers/", bytes.NewReader([]byte(`{"code":"WERK-CTRL-`+nonce+`","name":"Werkstatt Controlling-Test"}`)))
	createCcReq.Header.Set("Authorization", "Bearer "+accessToken)
	createCcReq.Header.Set("Content-Type", "application/json")
	createCcRec := httptest.NewRecorder()
	handler.ServeHTTP(createCcRec, createCcReq)
	if createCcRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for cost center create, got %d with body %s", createCcRec.Code, createCcRec.Body.String())
	}
	var costCenter struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createCcRec.Body.Bytes(), &costCenter); err != nil {
		t.Fatalf("decode cost center create response: %v", err)
	}

	assignReq := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/"+project.ID+"/kostenstelle", bytes.NewReader([]byte(`{"kostenstelle_id":"`+costCenter.ID+`"}`)))
	assignReq.Header.Set("Authorization", "Bearer "+accessToken)
	assignReq.Header.Set("Content-Type", "application/json")
	assignRec := httptest.NewRecorder()
	handler.ServeHTTP(assignRec, assignReq)
	if assignRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for kostenstelle assignment, got %d with body %s", assignRec.Code, assignRec.Body.String())
	}

	journalSvc := accounting.NewJournalService(env.PG)
	if _, err := journalSvc.Create(ctx, accounting.JournalEntryInput{
		Date:        time.Now(),
		Description: "Materialaufwand Controlling-Test",
		Lines: []accounting.JournalLineInput{
			{AccountCode: "3400", Debit: 250, KostenstelleID: &costCenter.ID},
			{AccountCode: "1200", Credit: 250},
		},
	}, companyID); err != nil {
		t.Fatalf("book journal entry: %v", err)
	}

	afterKostenstelle, err := buildProjectControlling(ctx, project.ID, env.PG, projSvc, quoteSvc, companyID)
	if err != nil {
		t.Fatalf("buildProjectControlling (mit Kostenstelle): %v", err)
	}
	if afterKostenstelle.IstKosten == nil || *afterKostenstelle.IstKosten != 250 {
		t.Fatalf("expected ist_kosten=250 nach Buchung auf Aufwandskonto, got %+v", afterKostenstelle.IstKosten)
	}
	expectedDeckungsbeitragIst := afterKostenstelle.IstErloes - 250
	if afterKostenstelle.DeckungsbeitragIst == nil || *afterKostenstelle.DeckungsbeitragIst != expectedDeckungsbeitragIst {
		t.Fatalf("expected deckungsbeitrag_ist=%v, got %+v", expectedDeckungsbeitragIst, afterKostenstelle.DeckungsbeitragIst)
	}
	// Soll-Seite bleibt von der Buchung unberuehrt.
	if afterKostenstelle.SollKosten != expectedSollKosten {
		t.Fatalf("expected soll_kosten unveraendert bei %v, got %v", expectedSollKosten, afterKostenstelle.SollKosten)
	}
}

// TestProjectControllingEndpointEndToEnd ist die E.2.3.3-Subtask: Anders
// als TestBuildProjectControllingAggregatesSollUndIstKosten (E.2.3.2, ruft
// buildProjectControlling direkt auf) geht dieser Test ausschliesslich
// ueber die echte HTTP-Route GET /projects/{id}/controlling und beweist
// damit das eigentliche Wiring (Route, Permission, JSON-Serialisierung),
// nicht nochmal die Aggregationslogik selbst.
func TestProjectControllingEndpointEndToEnd(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "controlling-endpoint@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "controlling-endpoint@example.com", "Secret123!")
	ctx := context.Background()

	nonce := uuid.NewString()[:8]

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     "Controlling-Endpoint-Kunde " + nonce,
		"email":    "controlling-endpoint-" + nonce + "@example.com",
		"waehrung": "EUR",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{"name":"Controlling-Endpoint-Projekt","kunde_id":"`+customerID+`","status":"angebot"}`)))
	createProjectReq.Header.Set("Authorization", "Bearer "+accessToken)
	createProjectReq.Header.Set("Content-Type", "application/json")
	createProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(createProjectRec, createProjectReq)
	if createProjectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", createProjectRec.Code, createProjectRec.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}

	createQuoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader([]byte(`{
		"contact_id":"`+customerID+`",
		"project_id":"`+project.ID+`",
		"currency":"EUR",
		"items":[
			{"description":"Testposition","qty":1,"unit":"Stk","unit_price":400,"tax_code":"DE19"}
		]
	}`)))
	createQuoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createQuoteReq.Header.Set("Content-Type", "application/json")
	createQuoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createQuoteRec, createQuoteReq)
	if createQuoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", createQuoteRec.Code, createQuoteRec.Body.String())
	}
	var createdQuote struct {
		ID          string  `json:"id"`
		GrossAmount float64 `json:"gross_amount"`
		Items       []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(createQuoteRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	if len(createdQuote.Items) != 1 {
		t.Fatalf("expected exactly 1 quote item, got %+v", createdQuote.Items)
	}
	itemID := createdQuote.Items[0].ID

	calcReq := httptest.NewRequest(http.MethodPut, "/api/v1/quotes/"+createdQuote.ID+"/items/"+itemID+"/calculation", bytes.NewReader([]byte(`{
		"material_cost":200,
		"material_zuschlag_percent":10,
		"lohn_stunden":1,
		"lohn_stundensatz":50,
		"lohn_zuschlag_percent":20,
		"fremdleistung_cost":20,
		"fremdleistung_zuschlag_percent":0
	}`)))
	calcReq.Header.Set("Authorization", "Bearer "+accessToken)
	calcReq.Header.Set("Content-Type", "application/json")
	calcRec := httptest.NewRecorder()
	handler.ServeHTTP(calcRec, calcReq)
	if calcRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for calculation upsert, got %d with body %s", calcRec.Code, calcRec.Body.String())
	}
	// material_total=220, lohn_total=60, fremdleistung_total=20 -> 300 je
	// Einheit, bei qty=1 also soll_kosten=300.
	const expectedSollKosten = 300.0

	getControlling := func() controllingResponseForTest {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/controlling", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for controlling endpoint, got %d with body %s", rec.Code, rec.Body.String())
		}
		var out controllingResponseForTest
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode controlling response: %v", err)
		}
		return out
	}

	before := getControlling()
	if before.SollErloes != createdQuote.GrossAmount {
		t.Fatalf("expected soll_erloes=%v, got %v", createdQuote.GrossAmount, before.SollErloes)
	}
	if before.SollKosten != expectedSollKosten {
		t.Fatalf("expected soll_kosten=%v, got %v", expectedSollKosten, before.SollKosten)
	}
	if before.IstKosten != nil {
		t.Fatalf("expected ist_kosten=nil ohne Kostenstelle, got %v", *before.IstKosten)
	}
	if before.DeckungsbeitragIst != nil {
		t.Fatalf("expected deckungsbeitrag_ist=nil ohne Kostenstelle, got %v", *before.DeckungsbeitragIst)
	}

	createCcReq := httptest.NewRequest(http.MethodPost, "/api/v1/cost-centers/", bytes.NewReader([]byte(`{"code":"WERK-EP-`+nonce+`","name":"Werkstatt Endpoint-Test"}`)))
	createCcReq.Header.Set("Authorization", "Bearer "+accessToken)
	createCcReq.Header.Set("Content-Type", "application/json")
	createCcRec := httptest.NewRecorder()
	handler.ServeHTTP(createCcRec, createCcReq)
	if createCcRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for cost center create, got %d with body %s", createCcRec.Code, createCcRec.Body.String())
	}
	var costCenter struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createCcRec.Body.Bytes(), &costCenter); err != nil {
		t.Fatalf("decode cost center create response: %v", err)
	}

	assignReq := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/"+project.ID+"/kostenstelle", bytes.NewReader([]byte(`{"kostenstelle_id":"`+costCenter.ID+`"}`)))
	assignReq.Header.Set("Authorization", "Bearer "+accessToken)
	assignReq.Header.Set("Content-Type", "application/json")
	assignRec := httptest.NewRecorder()
	handler.ServeHTTP(assignRec, assignReq)
	if assignRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for kostenstelle assignment, got %d with body %s", assignRec.Code, assignRec.Body.String())
	}

	journalSvc := accounting.NewJournalService(env.PG)
	if _, err := journalSvc.Create(ctx, accounting.JournalEntryInput{
		Date:        time.Now(),
		Description: "Materialaufwand Controlling-Endpoint-Test",
		Lines: []accounting.JournalLineInput{
			{AccountCode: "3400", Debit: 100, KostenstelleID: &costCenter.ID},
			{AccountCode: "1200", Credit: 100},
		},
	}, "default"); err != nil {
		t.Fatalf("book journal entry: %v", err)
	}

	after := getControlling()
	if after.IstKosten == nil || *after.IstKosten != 100 {
		t.Fatalf("expected ist_kosten=100 nach Buchung, got %+v", after.IstKosten)
	}
	expectedDeckungsbeitragIst := after.IstErloes - 100
	if after.DeckungsbeitragIst == nil || *after.DeckungsbeitragIst != expectedDeckungsbeitragIst {
		t.Fatalf("expected deckungsbeitrag_ist=%v, got %+v", expectedDeckungsbeitragIst, after.DeckungsbeitragIst)
	}
	if after.SollKosten != expectedSollKosten {
		t.Fatalf("expected soll_kosten unveraendert bei %v, got %v", expectedSollKosten, after.SollKosten)
	}

	// Negativfall: unbekannte Projekt-ID liefert einen Domain-Error (404),
	// keinen rohen 500.
	unknownReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+uuid.NewString()+"/controlling", nil)
	unknownReq.Header.Set("Authorization", "Bearer "+accessToken)
	unknownRec := httptest.NewRecorder()
	handler.ServeHTTP(unknownRec, unknownReq)
	if unknownRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown project id, got %d with body %s", unknownRec.Code, unknownRec.Body.String())
	}
}

type controllingResponseForTest struct {
	ProjectID           string   `json:"project_id"`
	SollErloes          float64  `json:"soll_erloes"`
	IstErloes           float64  `json:"ist_erloes"`
	SollKosten          float64  `json:"soll_kosten"`
	IstKosten           *float64 `json:"ist_kosten"`
	DeckungsbeitragSoll float64  `json:"deckungsbeitrag_soll"`
	DeckungsbeitragIst  *float64 `json:"deckungsbeitrag_ist"`
}
