package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// TestQuoteGAEBImportUploadLimits: Größengrenze, GAEB 90/2000 und leere
// Dateien beim Upload (ADR 0040 Entscheidungen 5 und 6, Backlog I.2.3a).
func TestQuoteGAEBImportUploadLimits(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-upload@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-upload@example.com", "Secret123!")

	suffix := uuid.NewString()[:8]
	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ": "org", "rolle": "customer", "status": "active", "name": "GAEB Upload Kunde " + suffix,
		"email": "gaeb-upload-" + suffix + "@example.com", "telefon": "+49 211 888888", "waehrung": "EUR",
	})
	projectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{"name":"GAEB Upload Projekt","kunde_id":"`+customerID+`","status":"angebot"}`)))
	projectReq.Header.Set("Authorization", "Bearer "+accessToken)
	projectReq.Header.Set("Content-Type", "application/json")
	projectRec := httptest.NewRecorder()
	handler.ServeHTTP(projectRec, projectReq)
	var project struct {
		ID string `json:"id"`
	}
	if projectRec.Code != http.StatusCreated || json.Unmarshal(projectRec.Body.Bytes(), &project) != nil {
		t.Fatalf("Projekt: %d %s", projectRec.Code, projectRec.Body.String())
	}

	upload := func(filename string, content []byte) *httptest.ResponseRecorder {
		t.Helper()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("project_id", project.ID)
		fw, _ := writer.CreateFormFile("file", filename)
		_, _ = fw.Write(content)
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	want := func(name string, rec *httptest.ResponseRecorder, status int, text string) {
		t.Helper()
		if rec.Code != status || !strings.Contains(rec.Body.String(), text) {
			t.Fatalf("%s: %d %s, erwartet %d mit %q", name, rec.Code, rec.Body.String(), status, text)
		}
	}
	count := func() int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM quote_imports WHERE project_id=$1`, project.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	gaeb90 := "GAEB 90/2000 (.d83, .p83) wird nicht unterstützt"
	want("d83", upload("lv.d83", []byte("00 GAEB 90")), http.StatusBadRequest, gaeb90)
	want("p83", upload("lv.p83", []byte("00 GAEB 2000")), http.StatusBadRequest, gaeb90)
	want("D83 groß", upload("LV.D83", []byte("00 GAEB 90")), http.StatusBadRequest, gaeb90)
	want("andere Endung", upload("lv.pdf", []byte("%PDF")), http.StatusBadRequest, "Nur GAEB-Dateien mit den Endungen .x83, .x84, .gaeb oder .xml")
	want("leer", upload("lv.x83", nil), http.StatusBadRequest, "GAEB-Datei ist leer")
	// eine Byte über der Grenze: der Service lehnt ab
	want("über der Grenze", upload("lv.x83", bytes.Repeat([]byte("a"), quotes.MaxGAEBImportBytes+1)), http.StatusRequestEntityTooLarge, "GAEB-Datei ist größer als 20 MB")
	// weit darüber: schon der Request wird abgeschnitten
	want("weit darüber", upload("lv.x83", bytes.Repeat([]byte("a"), quotes.MaxGAEBImportBytes+(2<<20))), http.StatusRequestEntityTooLarge, "Upload ist größer als 20 MB")
	if n := count(); n != 0 {
		t.Fatalf("%d Importläufe nach abgelehnten Uploads", n)
	}
	// kein Multipart: 400, nicht 413
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	want("kein Formular", rec, http.StatusBadRequest, "Ungültiges Upload-Formular")

	// genau an der Grenze und kleine Datei: angenommen
	want("an der Grenze", upload("lv.x83", bytes.Repeat([]byte("a"), quotes.MaxGAEBImportBytes)), http.StatusCreated, `"status":"uploaded"`)
	want("xml", upload("lv.xml", []byte("<GAEB/>")), http.StatusCreated, `"source_filename":"lv.xml"`)
	if n := count(); n != 2 {
		t.Fatalf("%d Importläufe, erwartet 2", n)
	}
}
