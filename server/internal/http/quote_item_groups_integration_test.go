package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func createIntegrationQuote(t *testing.T, handler http.Handler, accessToken, contactID string) string {
	t.Helper()
	body := []byte(fmt.Sprintf(`{
		"contact_id":%q,
		"currency":"EUR",
		"items":[
			{"description":"Testposition","qty":1,"unit":"Stk","unit_price":100,"tax_code":"DE19"}
		]
	}`, contactID))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for quote create, got %d with body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode quote create response: %v", err)
	}
	return created.ID
}

func itemGroupsTestCustomerBody(name string) map[string]any {
	return map[string]any{
		"typ":      "org",
		"rolle":    "customer",
		"status":   "active",
		"name":     name,
		"waehrung": "EUR",
	}
}

func TestQuoteItemGroupsCreateListUpdateAndDeleteFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-item-groups@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-item-groups@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("LV-Gruppen Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)

	losReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/item-groups", bytes.NewReader([]byte(`{"kind":"los","bezeichnung":"Los 1"}`)))
	losReq.Header.Set("Content-Type", "application/json")
	losReq.Header.Set("Authorization", "Bearer "+accessToken)
	losRec := httptest.NewRecorder()
	handler.ServeHTTP(losRec, losReq)
	if losRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for los create, got %d with body %s", losRec.Code, losRec.Body.String())
	}
	var los struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(losRec.Body.Bytes(), &los); err != nil {
		t.Fatalf("decode los create response: %v", err)
	}

	titelBody := fmt.Sprintf(`{"kind":"titel","bezeichnung":"Titel 1.1","parent_group_id":%q}`, los.ID)
	titelReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/item-groups", bytes.NewReader([]byte(titelBody)))
	titelReq.Header.Set("Content-Type", "application/json")
	titelReq.Header.Set("Authorization", "Bearer "+accessToken)
	titelRec := httptest.NewRecorder()
	handler.ServeHTTP(titelRec, titelReq)
	if titelRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for titel create, got %d with body %s", titelRec.Code, titelRec.Body.String())
	}
	var titel struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(titelRec.Body.Bytes(), &titel); err != nil {
		t.Fatalf("decode titel create response: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID+"/item-groups", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}
	var listed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 groups, got %#v", listed)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+quoteID+"/item-groups/"+titel.ID, bytes.NewReader([]byte(`{"bezeichnung":"Titel 1.1 umbenannt"}`)))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Authorization", "Bearer "+accessToken)
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", patchRec.Code, patchRec.Body.String())
	}
	var patched struct {
		Bezeichnung string `json:"bezeichnung"`
	}
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if patched.Bezeichnung != "Titel 1.1 umbenannt" {
		t.Fatalf("expected renamed bezeichnung, got %q", patched.Bezeichnung)
	}

	deleteTitelReq := httptest.NewRequest(http.MethodDelete, "/api/v1/quotes/"+quoteID+"/item-groups/"+titel.ID, nil)
	deleteTitelReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteTitelRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteTitelRec, deleteTitelReq)
	if deleteTitelRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteTitelRec.Code, deleteTitelRec.Body.String())
	}

	listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID+"/item-groups", nil)
	listAfterDeleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAfterDeleteRec := httptest.NewRecorder()
	handler.ServeHTTP(listAfterDeleteRec, listAfterDeleteReq)
	if listAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
	}
	var listedAfterDelete []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listAfterDeleteRec.Body.Bytes(), &listedAfterDelete); err != nil {
		t.Fatalf("decode list-after-delete response: %v", err)
	}
	if len(listedAfterDelete) != 1 {
		t.Fatalf("expected 1 remaining group (los), got %#v", listedAfterDelete)
	}
}

func TestQuoteItemGroupsRejectsInvalidHierarchy(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-item-groups-invalid@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-item-groups-invalid@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Ungueltige Hierarchie Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/item-groups", bytes.NewReader([]byte(`{"kind":"untertitel","bezeichnung":"Ohne Titel-Elternknoten"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", body.Error.Code)
	}
}

func TestQuoteItemGroupsCreateIsForbiddenForProcurementRole(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-item-groups-admin@example.com", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "integration-item-groups-procurement@example.com", "Secret123!", "procurement")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	adminToken := loginIntegrationUser(t, handler, "integration-item-groups-admin@example.com", "Secret123!")
	procurementToken := loginIntegrationUser(t, handler, "integration-item-groups-procurement@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, adminToken, itemGroupsTestCustomerBody("Forbidden Test Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, adminToken, customerID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/item-groups", bytes.NewReader([]byte(`{"kind":"los","bezeichnung":"Los 1"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+procurementToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestQuoteItemGroupsCreateRejectsOnNonDraftQuote(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-item-groups-nondraft@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-item-groups-nondraft@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Nicht-Entwurf Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)

	if _, err := env.PG.Exec(context.Background(), `UPDATE quotes SET status='sent' WHERE id=$1`, quoteID); err != nil {
		t.Fatalf("seed sent quote status: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/item-groups", bytes.NewReader([]byte(`{"kind":"los","bezeichnung":"Los 1"}`)))
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

func TestQuoteItemUpdateAcceptsGroupIDAndRejectsForeignGroup(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-item-groups-assign@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-item-groups-assign@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Gruppenzuordnung Kunde GmbH"))
	quoteAID := createIntegrationQuote(t, handler, accessToken, customerID)
	quoteBID := createIntegrationQuote(t, handler, accessToken, customerID)

	groupReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteAID+"/item-groups", bytes.NewReader([]byte(`{"kind":"los","bezeichnung":"Los A"}`)))
	groupReq.Header.Set("Content-Type", "application/json")
	groupReq.Header.Set("Authorization", "Bearer "+accessToken)
	groupRec := httptest.NewRecorder()
	handler.ServeHTTP(groupRec, groupReq)
	if groupRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", groupRec.Code, groupRec.Body.String())
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(groupRec.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode group create response: %v", err)
	}

	// Positiver Fall: quoteA-Position referenziert quoteA-eigene Gruppe.
	patchOwnBody := fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":[
		{"description":"Zugeordnete Position","qty":1,"unit":"Stk","unit_price":50,"tax_code":"DE19","group_id":%q}
	]}`, customerID, group.ID)
	patchOwnReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+quoteAID, bytes.NewReader([]byte(patchOwnBody)))
	patchOwnReq.Header.Set("Content-Type", "application/json")
	patchOwnReq.Header.Set("Authorization", "Bearer "+accessToken)
	patchOwnRec := httptest.NewRecorder()
	handler.ServeHTTP(patchOwnRec, patchOwnReq)
	if patchOwnRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", patchOwnRec.Code, patchOwnRec.Body.String())
	}
	var patchedOwn struct {
		Items []struct {
			GroupID string `json:"group_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(patchOwnRec.Body.Bytes(), &patchedOwn); err != nil {
		t.Fatalf("decode patch-own response: %v", err)
	}
	if len(patchedOwn.Items) != 1 || patchedOwn.Items[0].GroupID != group.ID {
		t.Fatalf("expected item group_id to be persisted, got %#v", patchedOwn.Items)
	}

	// Negativer Fall: quoteB-Position versucht quoteA-Gruppe zu referenzieren.
	patchForeignBody := fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":[
		{"description":"Fremde Gruppe","qty":1,"unit":"Stk","unit_price":50,"tax_code":"DE19","group_id":%q}
	]}`, customerID, group.ID)
	patchForeignReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+quoteBID, bytes.NewReader([]byte(patchForeignBody)))
	patchForeignReq.Header.Set("Content-Type", "application/json")
	patchForeignReq.Header.Set("Authorization", "Bearer "+accessToken)
	patchForeignRec := httptest.NewRecorder()
	handler.ServeHTTP(patchForeignRec, patchForeignReq)
	if patchForeignRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for foreign group_id, got %d with body %s", patchForeignRec.Code, patchForeignRec.Body.String())
	}
}

func TestQuoteItemTreeAssemblesHierarchyCorrectly(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-item-tree@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-item-tree@example.com", "Secret123!")

	customerID := createIntegrationContact(t, handler, accessToken, itemGroupsTestCustomerBody("Baum-Test Kunde GmbH"))
	quoteID := createIntegrationQuote(t, handler, accessToken, customerID)

	createGroup := func(body string) string {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/"+quoteID+"/item-groups", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for group create, got %d with body %s", rec.Code, rec.Body.String())
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode group create response: %v", err)
		}
		return created.ID
	}

	losID := createGroup(`{"kind":"los","bezeichnung":"Los 1"}`)
	titelID := createGroup(fmt.Sprintf(`{"kind":"titel","bezeichnung":"Titel 1.1","parent_group_id":%q}`, losID))
	untertitelID := createGroup(fmt.Sprintf(`{"kind":"untertitel","bezeichnung":"Untertitel 1.1.1","parent_group_id":%q}`, titelID))

	updateBody := fmt.Sprintf(`{"contact_id":%q,"currency":"EUR","items":[
		{"description":"Position im Untertitel","qty":1,"unit":"Stk","unit_price":10,"tax_code":"DE19","group_id":%q},
		{"description":"Ungruppierte Position","qty":1,"unit":"Stk","unit_price":20,"tax_code":"DE19"}
	]}`, customerID, untertitelID)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/quotes/"+quoteID, bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	treeReq := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+quoteID+"/item-tree", nil)
	treeReq.Header.Set("Authorization", "Bearer "+accessToken)
	treeRec := httptest.NewRecorder()
	handler.ServeHTTP(treeRec, treeReq)
	if treeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", treeRec.Code, treeRec.Body.String())
	}

	var tree struct {
		Nodes []struct {
			Group struct {
				ID string `json:"id"`
			} `json:"group"`
			Children []struct {
				Group struct {
					ID string `json:"id"`
				} `json:"group"`
				Children []struct {
					Group struct {
						ID string `json:"id"`
					} `json:"group"`
					Items []struct {
						Description string `json:"description"`
					} `json:"items"`
				} `json:"children"`
			} `json:"children"`
		} `json:"nodes"`
		UngroupedItems []struct {
			Description string `json:"description"`
		} `json:"ungrouped_items"`
	}
	if err := json.Unmarshal(treeRec.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode item-tree response: %v", err)
	}

	if len(tree.Nodes) != 1 || tree.Nodes[0].Group.ID != losID {
		t.Fatalf("expected exactly one root node (los), got %#v", tree.Nodes)
	}
	if len(tree.Nodes[0].Children) != 1 || tree.Nodes[0].Children[0].Group.ID != titelID {
		t.Fatalf("expected los to have exactly one child (titel), got %#v", tree.Nodes[0].Children)
	}
	titelNode := tree.Nodes[0].Children[0]
	if len(titelNode.Children) != 1 || titelNode.Children[0].Group.ID != untertitelID {
		t.Fatalf("expected titel to have exactly one child (untertitel), got %#v", titelNode.Children)
	}
	untertitelNode := titelNode.Children[0]
	if len(untertitelNode.Items) != 1 || untertitelNode.Items[0].Description != "Position im Untertitel" {
		t.Fatalf("expected the grouped item under untertitel, got %#v", untertitelNode.Items)
	}
	if len(tree.UngroupedItems) != 1 || tree.UngroupedItems[0].Description != "Ungruppierte Position" {
		t.Fatalf("expected exactly the ungrouped item, got %#v", tree.UngroupedItems)
	}
}
