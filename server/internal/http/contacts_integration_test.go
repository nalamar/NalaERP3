package apihttp

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestContactsCreateListAndGetFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts@example.com", "Secret123!")

	createBody := []byte(`{
		"typ":"org",
		"rolle":"partner",
		"status":"lead",
		"name":"Integration Metallbau GmbH",
		"email":"kontakt@integration.example",
		"telefon":"+49 123 4567",
		"waehrung":"EUR"
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()

	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Rolle  string `json:"rolle"`
		Status string `json:"status"`
		Typ    string `json:"typ"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created contact id")
	}
	if created.Name != "Integration Metallbau GmbH" {
		t.Fatalf("expected contact name, got %q", created.Name)
	}

	if created.Rolle != "partner" {
		t.Fatalf("expected partner role, got %q", created.Rolle)
	}
	if created.Status != "lead" {
		t.Fatalf("expected lead status, got %q", created.Status)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/?q=Integration%20Metallbau&rolle=partner&status=lead", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()

	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, item := range listed {
		if item.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected created contact %q in list response %#v", created.ID, listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()

	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Rolle  string `json:"rolle"`
		Status string `json:"status"`
		Typ    string `json:"typ"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected fetched id %q, got %q", created.ID, fetched.ID)
	}
	if fetched.Rolle != "partner" {
		t.Fatalf("expected partner role, got %q", fetched.Rolle)
	}
	if fetched.Status != "lead" {
		t.Fatalf("expected lead status, got %q", fetched.Status)
	}
	if fetched.Typ != "org" {
		t.Fatalf("expected org type, got %q", fetched.Typ)
	}
}

func TestContactsDeleteSoftSetsInactiveStatus(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-delete@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-delete@example.com", "Secret123!")

	createBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Delete Test GmbH"
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/contacts/"+created.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteRec.Code, deleteRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Aktiv  bool   `json:"aktiv"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Status != "inactive" {
		t.Fatalf("expected inactive status, got %q", fetched.Status)
	}
	if fetched.Aktiv {
		t.Fatal("expected aktiv=false after soft delete")
	}
}

func TestContactNotesCreateListUpdateAndDeleteFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contact-notes@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contact-notes@example.com", "Secret123!")

	createContactBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Notes Contact GmbH"
	}`)
	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createContactBody))
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)

	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}

	var createdContact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &createdContact); err != nil {
		t.Fatalf("decode create contact response: %v", err)
	}

	createNoteBody := []byte(`{
		"titel":"Erstgespräch",
		"inhalt":"Kunde wünscht Rückruf zur Projektabstimmung."
	}`)
	createNoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/notes", bytes.NewReader(createNoteBody))
	createNoteReq.Header.Set("Content-Type", "application/json")
	createNoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createNoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createNoteRec, createNoteReq)

	if createNoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createNoteRec.Code, createNoteRec.Body.String())
	}

	var createdNote struct {
		ID      string `json:"id"`
		Titel   string `json:"titel"`
		Inhalt  string `json:"inhalt"`
		Contact string `json:"contact_id"`
	}
	if err := json.Unmarshal(createNoteRec.Body.Bytes(), &createdNote); err != nil {
		t.Fatalf("decode create note response: %v", err)
	}
	if createdNote.ID == "" {
		t.Fatal("expected note id")
	}
	if createdNote.Contact != createdContact.ID {
		t.Fatalf("expected contact id %q, got %q", createdContact.ID, createdNote.Contact)
	}
	if createdNote.Titel != "Erstgespräch" {
		t.Fatalf("expected note title, got %q", createdNote.Titel)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/notes", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID     string `json:"id"`
		Titel  string `json:"titel"`
		Inhalt string `json:"inhalt"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list notes response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 note, got %d", len(listed))
	}
	if listed[0].ID != createdNote.ID {
		t.Fatalf("expected listed note id %q, got %q", createdNote.ID, listed[0].ID)
	}

	updateBody := []byte(`{
		"titel":"Nachfassen",
		"inhalt":"Rueckruf wurde fuer naechste Woche vereinbart."
	}`)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+createdContact.ID+"/notes/"+createdNote.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	var updated struct {
		ID     string `json:"id"`
		Titel  string `json:"titel"`
		Inhalt string `json:"inhalt"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update note response: %v", err)
	}
	if updated.Titel != "Nachfassen" {
		t.Fatalf("expected updated title, got %q", updated.Titel)
	}
	if updated.Inhalt != "Rueckruf wurde fuer naechste Woche vereinbart." {
		t.Fatalf("expected updated content, got %q", updated.Inhalt)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/contacts/"+createdContact.ID+"/notes/"+createdNote.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteRec.Code, deleteRec.Body.String())
	}

	listAgainReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/notes", nil)
	listAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(listAgainRec, listAgainReq)

	if listAgainRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listAgainRec.Code, listAgainRec.Body.String())
	}

	var listedAgain []map[string]any
	if err := json.Unmarshal(listAgainRec.Body.Bytes(), &listedAgain); err != nil {
		t.Fatalf("decode second list notes response: %v", err)
	}
	if len(listedAgain) != 0 {
		t.Fatalf("expected 0 notes after delete, got %d", len(listedAgain))
	}
}

func TestContactTasksCreateListUpdateAndDeleteFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contact-tasks@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contact-tasks@example.com", "Secret123!")

	createContactBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Tasks Contact GmbH"
	}`)
	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createContactBody))
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)

	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}

	var createdContact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &createdContact); err != nil {
		t.Fatalf("decode create contact response: %v", err)
	}

	createTaskBody := []byte(`{
		"titel":"Angebot nachfassen",
		"beschreibung":"Kundenrueckmeldung bis Freitag einholen.",
		"status":"open",
		"faellig_am":"2026-04-10T00:00:00Z"
	}`)
	createTaskReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/tasks", bytes.NewReader(createTaskBody))
	createTaskReq.Header.Set("Content-Type", "application/json")
	createTaskReq.Header.Set("Authorization", "Bearer "+accessToken)
	createTaskRec := httptest.NewRecorder()
	handler.ServeHTTP(createTaskRec, createTaskReq)

	if createTaskRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createTaskRec.Code, createTaskRec.Body.String())
	}

	var createdTask struct {
		ID           string  `json:"id"`
		ContactID    string  `json:"contact_id"`
		Titel        string  `json:"titel"`
		Beschreibung string  `json:"beschreibung"`
		Status       string  `json:"status"`
		FaelligAm    *string `json:"faellig_am"`
		ErledigtAm   *string `json:"erledigt_am"`
	}
	if err := json.Unmarshal(createTaskRec.Body.Bytes(), &createdTask); err != nil {
		t.Fatalf("decode create task response: %v", err)
	}
	if createdTask.ID == "" {
		t.Fatal("expected task id")
	}
	if createdTask.ContactID != createdContact.ID {
		t.Fatalf("expected contact id %q, got %q", createdContact.ID, createdTask.ContactID)
	}
	if createdTask.Status != "open" {
		t.Fatalf("expected open status, got %q", createdTask.Status)
	}
	if createdTask.FaelligAm == nil || *createdTask.FaelligAm != "2026-04-10T00:00:00Z" {
		t.Fatalf("expected due date to roundtrip, got %#v", createdTask.FaelligAm)
	}
	if createdTask.ErledigtAm != nil {
		t.Fatalf("expected no completion timestamp on create, got %#v", createdTask.ErledigtAm)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/tasks", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID        string  `json:"id"`
		Titel     string  `json:"titel"`
		Status    string  `json:"status"`
		FaelligAm *string `json:"faellig_am"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list tasks response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(listed))
	}
	if listed[0].ID != createdTask.ID {
		t.Fatalf("expected listed task id %q, got %q", createdTask.ID, listed[0].ID)
	}

	updateBody := []byte(`{
		"titel":"Angebot final nachfassen",
		"beschreibung":"Rueckmeldung und Freigabe final einholen.",
		"status":"done",
		"faellig_am":""
	}`)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+createdContact.ID+"/tasks/"+createdTask.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	var updated struct {
		ID           string  `json:"id"`
		Titel        string  `json:"titel"`
		Beschreibung string  `json:"beschreibung"`
		Status       string  `json:"status"`
		FaelligAm    *string `json:"faellig_am"`
		ErledigtAm   *string `json:"erledigt_am"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update task response: %v", err)
	}
	if updated.Titel != "Angebot final nachfassen" {
		t.Fatalf("expected updated title, got %q", updated.Titel)
	}
	if updated.Status != "done" {
		t.Fatalf("expected done status, got %q", updated.Status)
	}
	if updated.FaelligAm != nil {
		t.Fatalf("expected due date to be cleared, got %#v", updated.FaelligAm)
	}
	if updated.ErledigtAm == nil || *updated.ErledigtAm == "" {
		t.Fatal("expected completion timestamp after done update")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/contacts/"+createdContact.ID+"/tasks/"+createdTask.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", deleteRec.Code, deleteRec.Body.String())
	}

	listAgainReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/tasks", nil)
	listAgainReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAgainRec := httptest.NewRecorder()
	handler.ServeHTTP(listAgainRec, listAgainReq)

	if listAgainRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listAgainRec.Code, listAgainRec.Body.String())
	}

	var listedAgain []map[string]any
	if err := json.Unmarshal(listAgainRec.Body.Bytes(), &listedAgain); err != nil {
		t.Fatalf("decode second list tasks response: %v", err)
	}
	if len(listedAgain) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", len(listedAgain))
	}
}

func TestContactDocumentsUploadListAndDownloadFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contact-documents@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contact-documents@example.com", "Secret123!")

	createContactBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Documents Contact GmbH"
	}`)
	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createContactBody))
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)

	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}

	var createdContact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &createdContact); err != nil {
		t.Fatalf("decode create contact response: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "kontaktinfo.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	expectedContent := []byte("Kontaktdokument aus Integrationstest")
	if _, err := part.Write(expectedContent); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/documents", &body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploaded struct {
		ID          string `json:"id"`
		ContactID   string `json:"contact_id"`
		DocumentID  string `json:"document_id"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		Length      int64  `json:"length"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploaded.ID == "" || uploaded.DocumentID == "" {
		t.Fatal("expected uploaded document ids")
	}
	if uploaded.ContactID != createdContact.ID {
		t.Fatalf("expected contact id %q, got %q", createdContact.ID, uploaded.ContactID)
	}
	if uploaded.Filename != "kontaktinfo.txt" {
		t.Fatalf("expected filename kontaktinfo.txt, got %q", uploaded.Filename)
	}
	if uploaded.Length != int64(len(expectedContent)) {
		t.Fatalf("expected length %d, got %d", len(expectedContent), uploaded.Length)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/documents", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID         string `json:"id"`
		DocumentID string `json:"document_id"`
		Filename   string `json:"filename"`
		Length     int64  `json:"length"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list documents response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 document, got %d", len(listed))
	}
	if listed[0].DocumentID != uploaded.DocumentID {
		t.Fatalf("expected listed document %q, got %q", uploaded.DocumentID, listed[0].DocumentID)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+uploaded.DocumentID, nil)
	downloadReq.Header.Set("Authorization", "Bearer "+accessToken)
	downloadRec := httptest.NewRecorder()
	handler.ServeHTTP(downloadRec, downloadReq)

	if downloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", downloadRec.Code, downloadRec.Body.String())
	}
	if got := downloadRec.Header().Get("Content-Disposition"); got == "" {
		t.Fatal("expected content disposition header")
	}
	if got := downloadRec.Header().Get("Content-Length"); got != strconv.Itoa(len(expectedContent)) {
		t.Fatalf("expected content length %d, got %q", len(expectedContent), got)
	}
	if !bytes.Equal(downloadRec.Body.Bytes(), expectedContent) {
		t.Fatalf("expected downloaded body %q, got %q", string(expectedContent), downloadRec.Body.String())
	}
}

func TestContactActivityFeedAggregatesNotesTasksAndDocuments(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contact-activity@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contact-activity@example.com", "Secret123!")

	createContactBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Activity Contact GmbH"
	}`)
	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createContactBody))
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)

	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}

	var createdContact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &createdContact); err != nil {
		t.Fatalf("decode create contact response: %v", err)
	}

	createNoteBody := []byte(`{
		"titel":"Kickoff",
		"inhalt":"Erster Kontakt hergestellt."
	}`)
	createNoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/notes", bytes.NewReader(createNoteBody))
	createNoteReq.Header.Set("Content-Type", "application/json")
	createNoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	createNoteRec := httptest.NewRecorder()
	handler.ServeHTTP(createNoteRec, createNoteReq)

	if createNoteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createNoteRec.Code, createNoteRec.Body.String())
	}

	var createdNote struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createNoteRec.Body.Bytes(), &createdNote); err != nil {
		t.Fatalf("decode create note response: %v", err)
	}

	updateNoteBody := []byte(`{
		"titel":"Kickoff aktualisiert",
		"inhalt":"Erster Kontakt hergestellt und Bedarfe aufgenommen."
	}`)
	updateNoteReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+createdContact.ID+"/notes/"+createdNote.ID, bytes.NewReader(updateNoteBody))
	updateNoteReq.Header.Set("Content-Type", "application/json")
	updateNoteReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateNoteRec := httptest.NewRecorder()
	handler.ServeHTTP(updateNoteRec, updateNoteReq)

	if updateNoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updateNoteRec.Code, updateNoteRec.Body.String())
	}

	createTaskBody := []byte(`{
		"titel":"Rueckruf erledigen",
		"beschreibung":"Rueckruf fuer morgen einplanen.",
		"status":"open"
	}`)
	createTaskReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/tasks", bytes.NewReader(createTaskBody))
	createTaskReq.Header.Set("Content-Type", "application/json")
	createTaskReq.Header.Set("Authorization", "Bearer "+accessToken)
	createTaskRec := httptest.NewRecorder()
	handler.ServeHTTP(createTaskRec, createTaskReq)

	if createTaskRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createTaskRec.Code, createTaskRec.Body.String())
	}

	var createdTask struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createTaskRec.Body.Bytes(), &createdTask); err != nil {
		t.Fatalf("decode create task response: %v", err)
	}

	updateTaskBody := []byte(`{
		"status":"done"
	}`)
	updateTaskReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+createdContact.ID+"/tasks/"+createdTask.ID, bytes.NewReader(updateTaskBody))
	updateTaskReq.Header.Set("Content-Type", "application/json")
	updateTaskReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateTaskRec := httptest.NewRecorder()
	handler.ServeHTTP(updateTaskRec, updateTaskReq)

	if updateTaskRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updateTaskRec.Code, updateTaskRec.Body.String())
	}

	const expectedContent = "Aktivitaetsfeed-Dokument"
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "aktivitaet.txt")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte(expectedContent)); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/documents", &body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", uploadRec.Code, uploadRec.Body.String())
	}

	activityReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/activity", nil)
	activityReq.Header.Set("Authorization", "Bearer "+accessToken)
	activityRec := httptest.NewRecorder()
	handler.ServeHTTP(activityRec, activityReq)

	if activityRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", activityRec.Code, activityRec.Body.String())
	}

	var activity []map[string]any
	if err := json.Unmarshal(activityRec.Body.Bytes(), &activity); err != nil {
		t.Fatalf("decode activity response: %v", err)
	}
	if len(activity) < 4 {
		t.Fatalf("expected at least 4 activity items, got %d", len(activity))
	}

	assertActivity := func(source, action string) map[string]any {
		t.Helper()
		for _, item := range activity {
			if item["quelle"] == source && item["aktion"] == action {
				return item
			}
		}
		t.Fatalf("expected activity item %s/%s in %#v", source, action, activity)
		return nil
	}

	contactItem := assertActivity("contact", "created")
	if contactItem["referenz_id"] != createdContact.ID {
		t.Fatalf("expected contact reference id %q, got %#v", createdContact.ID, contactItem["referenz_id"])
	}

	noteItem := assertActivity("note", "updated")
	if noteItem["referenz_id"] != createdNote.ID {
		t.Fatalf("expected note reference id %q, got %#v", createdNote.ID, noteItem["referenz_id"])
	}
	if noteItem["beschreibung"] == "" {
		t.Fatal("expected note activity description")
	}

	taskItem := assertActivity("task", "completed")
	if taskItem["referenz_id"] != createdTask.ID {
		t.Fatalf("expected task reference id %q, got %#v", createdTask.ID, taskItem["referenz_id"])
	}

	documentItem := assertActivity("document", "uploaded")
	if documentItem["beschreibung"] != "aktivitaet.txt" {
		t.Fatalf("expected document description aktivitaet.txt, got %#v", documentItem["beschreibung"])
	}
}

func TestContactPersonsRoleAndChannelRoundtripFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contact-persons@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contact-persons@example.com", "Secret123!")

	createContactBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Persons Contact GmbH"
	}`)
	createContactReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createContactBody))
	createContactReq.Header.Set("Content-Type", "application/json")
	createContactReq.Header.Set("Authorization", "Bearer "+accessToken)
	createContactRec := httptest.NewRecorder()
	handler.ServeHTTP(createContactRec, createContactReq)

	if createContactRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createContactRec.Code, createContactRec.Body.String())
	}

	var createdContact struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createContactRec.Body.Bytes(), &createdContact); err != nil {
		t.Fatalf("decode create contact response: %v", err)
	}

	createPersonBody := []byte(`{
		"anrede":"Frau",
		"vorname":"Julia",
		"nachname":"Becker",
		"position":"Leitung Einkauf",
		"rolle":"purchasing",
		"bevorzugter_kanal":"teams",
		"email":"j.becker@example.com",
		"telefon":"+49 40 123456",
		"mobil":"+49 171 1111111",
		"is_primary":true
	}`)
	createPersonReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/"+createdContact.ID+"/persons", bytes.NewReader(createPersonBody))
	createPersonReq.Header.Set("Content-Type", "application/json")
	createPersonReq.Header.Set("Authorization", "Bearer "+accessToken)
	createPersonRec := httptest.NewRecorder()
	handler.ServeHTTP(createPersonRec, createPersonReq)

	if createPersonRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createPersonRec.Code, createPersonRec.Body.String())
	}

	var createdPerson struct {
		ID               string `json:"id"`
		ContactID        string `json:"contact_id"`
		Vorname          string `json:"vorname"`
		Nachname         string `json:"nachname"`
		Rolle            string `json:"rolle"`
		BevorzugterKanal string `json:"bevorzugter_kanal"`
		Primary          bool   `json:"is_primary"`
	}
	if err := json.Unmarshal(createPersonRec.Body.Bytes(), &createdPerson); err != nil {
		t.Fatalf("decode create person response: %v", err)
	}
	if createdPerson.ID == "" {
		t.Fatal("expected person id")
	}
	if createdPerson.ContactID != createdContact.ID {
		t.Fatalf("expected contact id %q, got %q", createdContact.ID, createdPerson.ContactID)
	}
	if createdPerson.Rolle != "purchasing" {
		t.Fatalf("expected purchasing role, got %q", createdPerson.Rolle)
	}
	if createdPerson.BevorzugterKanal != "teams" {
		t.Fatalf("expected teams channel, got %q", createdPerson.BevorzugterKanal)
	}
	if !createdPerson.Primary {
		t.Fatal("expected primary contact person")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+createdContact.ID+"/persons", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID               string `json:"id"`
		Rolle            string `json:"rolle"`
		BevorzugterKanal string `json:"bevorzugter_kanal"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list persons response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 person, got %d", len(listed))
	}
	if listed[0].ID != createdPerson.ID {
		t.Fatalf("expected listed person id %q, got %q", createdPerson.ID, listed[0].ID)
	}
	if listed[0].Rolle != "purchasing" {
		t.Fatalf("expected listed role purchasing, got %q", listed[0].Rolle)
	}
	if listed[0].BevorzugterKanal != "teams" {
		t.Fatalf("expected listed channel teams, got %q", listed[0].BevorzugterKanal)
	}

	updatePersonBody := []byte(`{
		"rolle":"project",
		"bevorzugter_kanal":"email",
		"position":"Projektkoordination"
	}`)
	updatePersonReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+createdContact.ID+"/persons/"+createdPerson.ID, bytes.NewReader(updatePersonBody))
	updatePersonReq.Header.Set("Content-Type", "application/json")
	updatePersonReq.Header.Set("Authorization", "Bearer "+accessToken)
	updatePersonRec := httptest.NewRecorder()
	handler.ServeHTTP(updatePersonRec, updatePersonReq)

	if updatePersonRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updatePersonRec.Code, updatePersonRec.Body.String())
	}

	var updated struct {
		ID               string `json:"id"`
		Position         string `json:"position"`
		Rolle            string `json:"rolle"`
		BevorzugterKanal string `json:"bevorzugter_kanal"`
	}
	if err := json.Unmarshal(updatePersonRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update person response: %v", err)
	}
	if updated.Rolle != "project" {
		t.Fatalf("expected updated role project, got %q", updated.Rolle)
	}
	if updated.BevorzugterKanal != "email" {
		t.Fatalf("expected updated channel email, got %q", updated.BevorzugterKanal)
	}
	if updated.Position != "Projektkoordination" {
		t.Fatalf("expected updated position, got %q", updated.Position)
	}
}

func TestContactsCommercialFieldsRoundtripAndUpdateFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-commercial@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-commercial@example.com", "Secret123!")

	createBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Commercial Test GmbH",
		"zahlungsbedingungen":"30 Tage netto",
		"debitor_nr":"D-1001",
		"kreditor_nr":"K-2001",
		"steuer_land":"at",
		"steuerbefreit":true
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID                  string `json:"id"`
		Zahlungsbedingungen string `json:"zahlungsbedingungen"`
		DebitorNr           string `json:"debitor_nr"`
		KreditorNr          string `json:"kreditor_nr"`
		SteuerLand          string `json:"steuer_land"`
		Steuerbefreit       bool   `json:"steuerbefreit"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created contact id")
	}
	if created.Zahlungsbedingungen != "30 Tage netto" {
		t.Fatalf("expected payment terms, got %q", created.Zahlungsbedingungen)
	}
	if created.DebitorNr != "D-1001" {
		t.Fatalf("expected debtor no, got %q", created.DebitorNr)
	}
	if created.KreditorNr != "K-2001" {
		t.Fatalf("expected creditor no, got %q", created.KreditorNr)
	}
	if created.SteuerLand != "AT" {
		t.Fatalf("expected tax country normalized to AT, got %q", created.SteuerLand)
	}
	if !created.Steuerbefreit {
		t.Fatal("expected tax_exempt=true")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/?q=Commercial%20Test", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}

	var listed []struct {
		ID                  string `json:"id"`
		Zahlungsbedingungen string `json:"zahlungsbedingungen"`
		DebitorNr           string `json:"debitor_nr"`
		SteuerLand          string `json:"steuer_land"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, item := range listed {
		if item.ID == created.ID {
			found = true
			if item.Zahlungsbedingungen != "30 Tage netto" {
				t.Fatalf("expected list payment terms, got %q", item.Zahlungsbedingungen)
			}
			if item.DebitorNr != "D-1001" {
				t.Fatalf("expected list debtor no, got %q", item.DebitorNr)
			}
			if item.SteuerLand != "AT" {
				t.Fatalf("expected list tax country AT, got %q", item.SteuerLand)
			}
		}
	}
	if !found {
		t.Fatalf("expected created contact %q in list response %#v", created.ID, listed)
	}

	updateBody := []byte(`{
		"zahlungsbedingungen":"14 Tage 2% Skonto",
		"debitor_nr":"D-1002",
		"kreditor_nr":"K-2002",
		"steuer_land":"ch",
		"steuerbefreit":false
	}`)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+created.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var fetched struct {
		ID                  string `json:"id"`
		Zahlungsbedingungen string `json:"zahlungsbedingungen"`
		DebitorNr           string `json:"debitor_nr"`
		KreditorNr          string `json:"kreditor_nr"`
		SteuerLand          string `json:"steuer_land"`
		Steuerbefreit       bool   `json:"steuerbefreit"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Zahlungsbedingungen != "14 Tage 2% Skonto" {
		t.Fatalf("expected updated payment terms, got %q", fetched.Zahlungsbedingungen)
	}
	if fetched.DebitorNr != "D-1002" {
		t.Fatalf("expected updated debtor no, got %q", fetched.DebitorNr)
	}
	if fetched.KreditorNr != "K-2002" {
		t.Fatalf("expected updated creditor no, got %q", fetched.KreditorNr)
	}
	if fetched.SteuerLand != "CH" {
		t.Fatalf("expected updated tax country CH, got %q", fetched.SteuerLand)
	}
	if fetched.Steuerbefreit {
		t.Fatal("expected tax_exempt=false after update")
	}
}

func TestContactsCreateReturnsValidationErrorForDuplicateVAT(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-duplicate@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-duplicate@example.com", "Secret123!")

	firstBody := []byte(`{
		"typ":"org",
		"rolle":"supplier",
		"status":"active",
		"name":"Duplicate VAT GmbH",
		"email":"dup-vat@example.com",
		"ust_id":"DE123456789"
	}`)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondBody := []byte(`{
		"typ":"org",
		"rolle":"supplier",
		"status":"active",
		"name":"Other Supplier GmbH",
		"email":"other-vat@example.com",
		"ust_id":"de123456789"
	}`)
	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "Kontakt mit gleicher USt-IdNr. bereits vorhanden" {
		t.Fatalf("unexpected error message %q", resp.Error.Message)
	}
}

func TestContactsCreateReturnsValidationErrorForDuplicateNameAndEmail(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-duplicate-name@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-duplicate-name@example.com", "Secret123!")

	firstBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Duplicate Name GmbH",
		"email":"duplicate-name@example.com"
	}`)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondBody := []byte(`{
		"typ":"org",
		"rolle":"partner",
		"status":"lead",
		"name":" duplicate name gmbh ",
		"email":"DUPLICATE-NAME@example.com"
	}`)
	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "Kontakt mit gleichem Namen und gleicher E-Mail bereits vorhanden" {
		t.Fatalf("unexpected error message %q", resp.Error.Message)
	}
}

func TestContactsUpdateReturnsValidationErrorForDuplicateConflict(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-update-conflict@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-update-conflict@example.com", "Secret123!")

	firstBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Conflict One GmbH",
		"email":"conflict-one@example.com"
	}`)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected first create 201, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}
	var firstCreated struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(firstRec.Body.Bytes(), &firstCreated); err != nil {
		t.Fatalf("decode first create response: %v", err)
	}

	secondBody := []byte(`{
		"typ":"org",
		"rolle":"supplier",
		"status":"active",
		"name":"Conflict Two GmbH",
		"email":"conflict-two@example.com"
	}`)
	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusCreated {
		t.Fatalf("expected second create 201, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}
	var secondCreated struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondCreated); err != nil {
		t.Fatalf("decode second create response: %v", err)
	}

	updateBody := []byte(`{
		"name":"Conflict One GmbH",
		"email":"conflict-one@example.com"
	}`)
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/"+secondCreated.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "Kontakt mit gleichem Namen und gleicher E-Mail bereits vorhanden" {
		t.Fatalf("unexpected error message %q", resp.Error.Message)
	}

	if firstCreated.ID == secondCreated.ID {
		t.Fatal("expected distinct contacts for conflict test")
	}
}

func TestContactsCreateReturnsValidationErrorForDuplicateDebtorNumber(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-duplicate-debtor@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-duplicate-debtor@example.com", "Secret123!")

	firstBody := []byte(`{
		"typ":"org",
		"rolle":"customer",
		"status":"active",
		"name":"Debtor One GmbH",
		"debitor_nr":"D-9001"
	}`)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondBody := []byte(`{
		"typ":"org",
		"rolle":"partner",
		"status":"lead",
		"name":"Debtor Two GmbH",
		"debitor_nr":" d-9001 "
	}`)
	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "Kontakt mit gleicher Debitor-Nr. bereits vorhanden" {
		t.Fatalf("unexpected error message %q", resp.Error.Message)
	}
}

func TestContactsCreateReturnsValidationErrorForDuplicateCreditorNumber(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-contacts-duplicate-creditor@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-contacts-duplicate-creditor@example.com", "Secret123!")

	firstBody := []byte(`{
		"typ":"org",
		"rolle":"supplier",
		"status":"active",
		"name":"Creditor One GmbH",
		"kreditor_nr":"K-7001"
	}`)
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer "+accessToken)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", firstRec.Code, firstRec.Body.String())
	}

	secondBody := []byte(`{
		"typ":"org",
		"rolle":"both",
		"status":"active",
		"name":"Creditor Two GmbH",
		"kreditor_nr":" k-7001 "
	}`)
	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer "+accessToken)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", secondRec.Code, secondRec.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "Kontakt mit gleicher Kreditor-Nr. bereits vorhanden" {
		t.Fatalf("unexpected error message %q", resp.Error.Message)
	}
}

func loginIntegrationUser(t *testing.T, handler http.Handler, login, password string) string {
	t.Helper()

	loginBody, err := json.Marshal(map[string]string{
		"login":    login,
		"password": password,
	})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d with body %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"access_token"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResp.Data.Tokens.AccessToken == "" {
		t.Fatal("expected access token")
	}
	return loginResp.Data.Tokens.AccessToken
}
