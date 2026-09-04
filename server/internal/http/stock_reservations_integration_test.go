package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// setupReservationFixture legt Material, Lager und Projekt an und bucht
// einen Wareneingang (Menge initialQty) - der gemeinsame Ausgangszustand
// fuer alle Reservierungs-Tests.
func setupReservationFixture(t *testing.T, handler http.Handler, accessToken string, initialQty float64) (materialID, warehouseID, projectID string) {
	t.Helper()

	nonce := uuid.NewString()[:8]

	createMatReq := httptest.NewRequest(http.MethodPost, "/api/v1/materials/", bytes.NewReader([]byte(`{"nummer":"MAT-RESV-`+nonce+`","bezeichnung":"Reservierungs-Testmaterial","einheit":"Stk"}`)))
	createMatReq.Header.Set("Authorization", "Bearer "+accessToken)
	createMatReq.Header.Set("Content-Type", "application/json")
	createMatRec := httptest.NewRecorder()
	handler.ServeHTTP(createMatRec, createMatReq)
	if createMatRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for material create, got %d with body %s", createMatRec.Code, createMatRec.Body.String())
	}
	var mat struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMatRec.Body.Bytes(), &mat); err != nil {
		t.Fatalf("decode material create response: %v", err)
	}

	createWhReq := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses/", bytes.NewReader([]byte(`{"code":"WH-RESV-`+nonce+`","name":"Reservierungs-Testlager"}`)))
	createWhReq.Header.Set("Authorization", "Bearer "+accessToken)
	createWhReq.Header.Set("Content-Type", "application/json")
	createWhRec := httptest.NewRecorder()
	handler.ServeHTTP(createWhRec, createWhReq)
	if createWhRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for warehouse create, got %d with body %s", createWhRec.Code, createWhRec.Body.String())
	}
	var wh struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createWhRec.Body.Bytes(), &wh); err != nil {
		t.Fatalf("decode warehouse create response: %v", err)
	}

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ":    "org",
		"rolle":  "customer",
		"status": "active",
		"name":   "Reservierungs-Testkunde " + nonce,
		"email":  "resv-" + nonce + "@example.com",
	})

	createProjectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{
		"name":"Reservierungs-Testprojekt",
		"kunde_id":"`+customerID+`",
		"status":"angebot"
	}`)))
	createProjectReq.Header.Set("Authorization", "Bearer "+accessToken)
	createProjectReq.Header.Set("Content-Type", "application/json")
	createProjectRec := httptest.NewRecorder()
	handler.ServeHTTP(createProjectRec, createProjectReq)
	if createProjectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for project create, got %d with body %s", createProjectRec.Code, createProjectRec.Body.String())
	}
	var proj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createProjectRec.Body.Bytes(), &proj); err != nil {
		t.Fatalf("decode project create response: %v", err)
	}

	if initialQty != 0 {
		createMoveReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-movements/", bytes.NewReader([]byte(`{"material_id":"`+mat.ID+`","warehouse_id":"`+wh.ID+`","menge":`+jsonFloat(initialQty)+`,"einheit":"Stk","typ":"in"}`)))
		createMoveReq.Header.Set("Authorization", "Bearer "+accessToken)
		createMoveReq.Header.Set("Content-Type", "application/json")
		createMoveRec := httptest.NewRecorder()
		handler.ServeHTTP(createMoveRec, createMoveReq)
		if createMoveRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for stock movement create, got %d with body %s", createMoveRec.Code, createMoveRec.Body.String())
		}
	}

	return mat.ID, wh.ID, proj.ID
}

func jsonFloat(v float64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func availableStock(t *testing.T, handler http.Handler, accessToken, materialID, warehouseID string) float64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stock-reservations/available?material_id="+materialID+"&warehouse_id="+warehouseID, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for available lookup, got %d with body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Available float64 `json:"available"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode available response: %v", err)
	}
	return body.Available
}

func TestStockReservationCreateListReleaseFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "resv-flow@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "resv-flow@example.com", "Secret123!")

	matID, whID, projID := setupReservationFixture(t, handler, accessToken, 10)

	if got := availableStock(t, handler, accessToken, matID, whID); got != 10 {
		t.Fatalf("expected available=10 before reservation, got %v", got)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"project_id":"`+projID+`",
		"qty":6,
		"grund":"Fuer Fassadenmontage"
	}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for reservation create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var reservation struct {
		ID     string  `json:"id"`
		Status string  `json:"status"`
		Qty    float64 `json:"qty"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &reservation); err != nil {
		t.Fatalf("decode reservation create response: %v", err)
	}
	if reservation.Status != "aktiv" || reservation.Qty != 6 {
		t.Fatalf("unexpected reservation after create: %+v", reservation)
	}

	if got := availableStock(t, handler, accessToken, matID, whID); got != 4 {
		t.Fatalf("expected available=4 after reserving 6 of 10, got %v", got)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/stock-reservations/?project_id="+projID, nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for reservation list, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode reservation list response: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != reservation.ID {
		t.Fatalf("expected list to contain exactly the created reservation, got %+v", listed)
	}

	releaseReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/"+reservation.ID+"/release", nil)
	releaseReq.Header.Set("Authorization", "Bearer "+accessToken)
	releaseRec := httptest.NewRecorder()
	handler.ServeHTTP(releaseRec, releaseReq)
	if releaseRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for reservation release, got %d with body %s", releaseRec.Code, releaseRec.Body.String())
	}
	var released struct {
		Status     string  `json:"status"`
		ReleasedAt *string `json:"released_at"`
	}
	if err := json.Unmarshal(releaseRec.Body.Bytes(), &released); err != nil {
		t.Fatalf("decode reservation release response: %v", err)
	}
	if released.Status != "freigegeben" || released.ReleasedAt == nil {
		t.Fatalf("unexpected reservation after release: %+v", released)
	}

	if got := availableStock(t, handler, accessToken, matID, whID); got != 10 {
		t.Fatalf("expected available=10 after release, got %v", got)
	}
}

func TestStockReservationCreateRejectsQtyOverAvailable(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "resv-over@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "resv-over@example.com", "Secret123!")

	matID, whID, projID := setupReservationFixture(t, handler, accessToken, 5)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"project_id":"`+projID+`",
		"qty":10
	}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for over-available reservation, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	if got := availableStock(t, handler, accessToken, matID, whID); got != 5 {
		t.Fatalf("expected available unchanged at 5 after rejected reservation, got %v", got)
	}
}

func TestStockReservationCreateSecondReservationRespectsFirstHold(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "resv-double@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "resv-double@example.com", "Secret123!")

	matID, whID, projID := setupReservationFixture(t, handler, accessToken, 10)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"project_id":"`+projID+`",
		"qty":6
	}`)))
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first reservation, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"project_id":"`+projID+`",
		"qty":5
	}`)))
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for second reservation exceeding remaining 4, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}
}

func TestStockReservationCreateRejectsUnknownProject(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "resv-noproject@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "resv-noproject@example.com", "Secret123!")

	matID, whID, _ := setupReservationFixture(t, handler, accessToken, 5)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"project_id":"00000000-0000-0000-0000-000000000000",
		"qty":1
	}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown project, got %d with body %s", createRec.Code, createRec.Body.String())
	}
}

func TestStockReservationReleaseRejectsAlreadyReleased(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "resv-rerelease@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "resv-rerelease@example.com", "Secret123!")

	matID, whID, projID := setupReservationFixture(t, handler, accessToken, 5)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/", bytes.NewReader([]byte(`{
		"material_id":"`+matID+`",
		"warehouse_id":"`+whID+`",
		"project_id":"`+projID+`",
		"qty":3
	}`)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for reservation create, got %d with body %s", createRec.Code, createRec.Body.String())
	}
	var reservation struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &reservation); err != nil {
		t.Fatalf("decode reservation create response: %v", err)
	}

	firstReleaseReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/"+reservation.ID+"/release", nil)
	firstReleaseReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstReleaseRec := httptest.NewRecorder()
	handler.ServeHTTP(firstReleaseRec, firstReleaseReq)
	if firstReleaseRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for first release, got %d with body %s", firstReleaseRec.Code, firstReleaseRec.Body.String())
	}

	secondReleaseReq := httptest.NewRequest(http.MethodPost, "/api/v1/stock-reservations/"+reservation.ID+"/release", nil)
	secondReleaseReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondReleaseRec := httptest.NewRecorder()
	handler.ServeHTTP(secondReleaseRec, secondReleaseReq)
	if secondReleaseRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for re-releasing an already released reservation, got %d with body %s", secondReleaseRec.Code, secondReleaseRec.Body.String())
	}
}
