package apihttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func addendaTestCustomerBody(name string) map[string]any {
	return map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     name,
		"waehrung": "EUR",
	}
}

// createIntegrationSalesOrder erstellt ein Angebot mit einer Position,
// nimmt es an und ueberfuehrt es in einen Auftrag - der einzige bestehende
// Weg, um im Test einen echten sales_orders-Datensatz zu erhalten (siehe
// quotes_integration_test.go, keine direkte POST /sales-orders/-Route).
func createIntegrationSalesOrder(t *testing.T, handler http.Handler, accessToken, contactID string) string {
	t.Helper()
	quoteID := createIntegrationQuote(t, handler, accessToken, contactID)

	acceptReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/accept", nil)
	acceptReq.Header.Set("Authorization", "Bearer "+accessToken)
	acceptRec := httptest.NewRecorder()
	handler.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for quote accept, got %d with body %s", acceptRec.Code, acceptRec.Body.String())
	}

	convertReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/convert-to-sales-order", nil)
	convertReq.Header.Set("Authorization", "Bearer "+accessToken)
	convertRec := httptest.NewRecorder()
	handler.ServeHTTP(convertRec, convertReq)
	if convertRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for convert-to-sales-order, got %d with body %s", convertRec.Code, convertRec.Body.String())
	}
	var order struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(convertRec.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode sales order response: %v", err)
	}
	return order.ID
}

func TestSalesOrderAddendaCreateSubmitAndAcceptFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-addenda@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-addenda@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, addendaTestCustomerBody("Nachtrag Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda", bytes.NewReader([]byte(`{"begruendung":"Anordnung des AG: zusaetzliche Steckdosen"}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var addendum struct {
		ID         string `json:"id"`
		NachtragNo int    `json:"nachtrag_no"`
		Status     string `json:"status"`
		Currency   string `json:"currency"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &addendum); err != nil {
		t.Fatalf("decode addendum create response: %v", err)
	}
	if addendum.NachtragNo != 1 {
		t.Fatalf("expected first addendum to be nachtrag_no 1, got %d", addendum.NachtragNo)
	}
	if addendum.Status != "entwurf" {
		t.Fatalf("expected entwurf status, got %q", addendum.Status)
	}
	if addendum.Currency != "EUR" {
		t.Fatalf("expected currency inherited from order (EUR), got %q", addendum.Currency)
	}

	itemReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendum.ID+"/items", bytes.NewReader([]byte(`{
		"description":"Zusaetzliche Steckdosen",
		"qty":4,
		"unit":"Stk",
		"unit_price":25,
		"tax_code":"DE19"
	}`)))
	itemReq.Header.Set("Content-Type", "application/json")
	itemReq.Header.Set("Authorization", "Bearer "+accessToken)
	itemRec := httptest.NewRecorder()
	handler.ServeHTTP(itemRec, itemReq)
	if itemRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", itemRec.Code, itemRec.Body.String())
	}

	submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendum.ID+"/submit", nil)
	submitReq.Header.Set("Authorization", "Bearer "+accessToken)
	submitRec := httptest.NewRecorder()
	handler.ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", submitRec.Code, submitRec.Body.String())
	}
	var submitted struct {
		Status      string  `json:"status"`
		NetAmount   float64 `json:"net_amount"`
		GrossAmount float64 `json:"gross_amount"`
	}
	if err := json.Unmarshal(submitRec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if submitted.Status != "beantragt" {
		t.Fatalf("expected beantragt status, got %q", submitted.Status)
	}
	if submitted.NetAmount != 100 {
		t.Fatalf("expected net_amount 100 (4*25), got %v", submitted.NetAmount)
	}

	decideReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendum.ID+"/decide", bytes.NewReader([]byte(`{"accepted":true}`)))
	decideReq.Header.Set("Content-Type", "application/json")
	decideReq.Header.Set("Authorization", "Bearer "+accessToken)
	decideRec := httptest.NewRecorder()
	handler.ServeHTTP(decideRec, decideReq)
	if decideRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", decideRec.Code, decideRec.Body.String())
	}
	var decided struct {
		Status        string `json:"status"`
		EntschiedenAm string `json:"entschieden_am"`
	}
	if err := json.Unmarshal(decideRec.Body.Bytes(), &decided); err != nil {
		t.Fatalf("decode decide response: %v", err)
	}
	if decided.Status != "angenommen" {
		t.Fatalf("expected angenommen status, got %q", decided.Status)
	}
	if decided.EntschiedenAm == "" {
		t.Fatal("expected entschieden_am to be set")
	}

	// Effektive Auftragssumme muss jetzt den Nachtrag zusaetzlich zur
	// urspruenglichen Auftragssumme (100 aus der Grundposition) enthalten.
	totalsReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+orderID+"/effective-totals", nil)
	totalsReq.Header.Set("Authorization", "Bearer "+accessToken)
	totalsRec := httptest.NewRecorder()
	handler.ServeHTTP(totalsRec, totalsReq)
	if totalsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", totalsRec.Code, totalsRec.Body.String())
	}
	var totals struct {
		BaseNetAmount            float64 `json:"base_net_amount"`
		ApprovedAddendaNetAmount float64 `json:"approved_addenda_net_amount"`
		EffectiveNetAmount       float64 `json:"effective_net_amount"`
	}
	if err := json.Unmarshal(totalsRec.Body.Bytes(), &totals); err != nil {
		t.Fatalf("decode effective totals response: %v", err)
	}
	if totals.BaseNetAmount != 100 {
		t.Fatalf("expected base_net_amount 100, got %v", totals.BaseNetAmount)
	}
	if totals.ApprovedAddendaNetAmount != 100 {
		t.Fatalf("expected approved_addenda_net_amount 100, got %v", totals.ApprovedAddendaNetAmount)
	}
	if totals.EffectiveNetAmount != 200 {
		t.Fatalf("expected effective_net_amount 200 (100 base + 100 approved addendum), got %v", totals.EffectiveNetAmount)
	}

	// Der Grundauftrag selbst darf durch den angenommenen Nachtrag NICHT
	// veraendert worden sein (ADR 0010: keine stille Mutation).
	orderReq := httptest.NewRequest(http.MethodGet, "/api/v1/sales-orders/"+orderID, nil)
	orderReq.Header.Set("Authorization", "Bearer "+accessToken)
	orderRec := httptest.NewRecorder()
	handler.ServeHTTP(orderRec, orderReq)
	var order struct {
		NetAmount float64 `json:"net_amount"`
	}
	if err := json.Unmarshal(orderRec.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode order response: %v", err)
	}
	if order.NetAmount != 100 {
		t.Fatalf("expected base order net_amount to remain unchanged at 100, got %v", order.NetAmount)
	}
}

func TestSalesOrderAddendaDecideRejectsMissingAblehnungsgrund(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-addenda-reject@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-addenda-reject@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, addendaTestCustomerBody("Ablehnungs-Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)

	addendumID := createDraftAddendumWithItem(t, handler, accessToken, orderID, "Testbegruendung", 1, 10)

	submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendumID+"/submit", nil)
	submitReq.Header.Set("Authorization", "Bearer "+accessToken)
	submitRec := httptest.NewRecorder()
	handler.ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", submitRec.Code, submitRec.Body.String())
	}

	decideReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendumID+"/decide", bytes.NewReader([]byte(`{"accepted":false}`)))
	decideReq.Header.Set("Content-Type", "application/json")
	decideReq.Header.Set("Authorization", "Bearer "+accessToken)
	decideRec := httptest.NewRecorder()
	handler.ServeHTTP(decideRec, decideReq)
	if decideRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", decideRec.Code, decideRec.Body.String())
	}
}

func TestSalesOrderAddendaSubmitRejectsEmptyAddendum(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-addenda-empty@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-addenda-empty@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, addendaTestCustomerBody("Leerer Nachtrag Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda", bytes.NewReader([]byte(`{"begruendung":"Ohne Positionen"}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var addendum struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &addendum); err != nil {
		t.Fatalf("decode addendum create response: %v", err)
	}

	submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendum.ID+"/submit", nil)
	submitReq.Header.Set("Authorization", "Bearer "+accessToken)
	submitRec := httptest.NewRecorder()
	handler.ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for addendum without items, got %d with body %s", submitRec.Code, submitRec.Body.String())
	}
}

func TestSalesOrderAddendaItemMutationRejectedAfterSubmit(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-addenda-frozen@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-addenda-frozen@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, addendaTestCustomerBody("Eingefrorener Nachtrag Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, accessToken, customerID)

	addendumID := createDraftAddendumWithItem(t, handler, accessToken, orderID, "Wird eingefroren", 1, 10)

	submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendumID+"/submit", nil)
	submitReq.Header.Set("Authorization", "Bearer "+accessToken)
	submitRec := httptest.NewRecorder()
	handler.ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", submitRec.Code, submitRec.Body.String())
	}

	itemReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendumID+"/items", bytes.NewReader([]byte(`{
		"description":"Sollte scheitern",
		"qty":1,
		"unit":"Stk",
		"unit_price":10,
		"tax_code":"DE19"
	}`)))
	itemReq.Header.Set("Content-Type", "application/json")
	itemReq.Header.Set("Authorization", "Bearer "+accessToken)
	itemRec := httptest.NewRecorder()
	handler.ServeHTTP(itemRec, itemReq)
	if itemRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for item mutation on non-draft addendum, got %d with body %s", itemRec.Code, itemRec.Body.String())
	}
}

func TestSalesOrderAddendaCreateIsForbiddenForProcurementRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-addenda-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-addenda-procurement@example.com", "Secret123!", "procurement")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-addenda-admin@example.com", "Secret123!")
	procurementToken := loginIntegrationUser(t, handler, "integration-addenda-procurement@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, addendaTestCustomerBody("Forbidden Nachtrag Kunde GmbH"))
	orderID := createIntegrationSalesOrder(t, handler, adminToken, customerID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda", bytes.NewReader([]byte(`{"begruendung":"Test"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+procurementToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func createDraftAddendumWithItem(t *testing.T, handler http.Handler, accessToken, orderID, begruendung string, qty, unitPrice float64) string {
	t.Helper()
	createBody := fmt.Sprintf(`{"begruendung":%q}`, begruendung)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda", bytes.NewReader([]byte(createBody)))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for addendum create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var addendum struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &addendum); err != nil {
		t.Fatalf("decode addendum create response: %v", err)
	}

	itemBody := fmt.Sprintf(`{"description":"Testposition","qty":%v,"unit":"Stk","unit_price":%v,"tax_code":"DE19"}`, qty, unitPrice)
	itemReq := httptest.NewRequest(http.MethodPost, "/api/v1/sales-orders/"+orderID+"/addenda/"+addendum.ID+"/items", bytes.NewReader([]byte(itemBody)))
	itemReq.Header.Set("Content-Type", "application/json")
	itemReq.Header.Set("Authorization", "Bearer "+accessToken)
	itemRec := httptest.NewRecorder()
	handler.ServeHTTP(itemRec, itemReq)
	if itemRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for addendum item create, got %d with body %s", itemRec.Code, itemRec.Body.String())
	}
	return addendum.ID
}
