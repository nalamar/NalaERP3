package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// TestQuoteGAEBImportStructureEndpoints: Gliederung, Vorbemerkungen und
// die neuen Positionsfelder über die API (Backlog I.2.2b).
func TestQuoteGAEBImportStructureEndpoints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-gaeb-structure@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-gaeb-structure@example.com", "Secret123!")

	get := func(path string, out any) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if out != nil && rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		}
		return rec.Code
	}

	customerID := createIntegrationContact(t, handler, accessToken, map[string]any{
		"typ": "org", "rolle": "customer", "status": "active", "name": "GAEB Struktur Kunde GmbH",
		"email": "gaeb-structure@example.com", "telefon": "+49 211 777777", "waehrung": "EUR",
	})
	projectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/", bytes.NewReader([]byte(`{"name":"GAEB Struktur Projekt","kunde_id":"`+customerID+`","status":"angebot"}`)))
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

	raw, err := os.ReadFile("../quotes/eval/testdata/lv-02.x83")
	if err != nil {
		t.Fatal(err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("project_id", project.ID)
	_ = writer.WriteField("contact_id", customerID)
	fw, _ := writer.CreateFormFile("file", "lv-02.x83")
	_, _ = fw.Write(raw)
	_ = writer.Close()
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/quotes/imports/gaeb", body)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)
	var created struct {
		ID string `json:"id"`
	}
	if uploadRec.Code != http.StatusCreated || json.Unmarshal(uploadRec.Body.Bytes(), &created) != nil {
		t.Fatalf("Upload: %d %s", uploadRec.Code, uploadRec.Body.String())
	}

	// vor dem Parsen: leere Gliederung und Vorbemerkungen, keine null-Werte
	var leer map[string]any
	if code := get("/api/v1/quotes/imports/"+created.ID+"/structure", &leer); code != http.StatusOK {
		t.Fatalf("Struktur vor dem Parsen: %d", code)
	}
	if g, ok := leer["groups"].([]any); !ok || len(g) != 0 {
		t.Fatalf("groups vor dem Parsen: %v", leer)
	}
	if v, ok := leer["vorbemerkungen"].([]any); !ok || len(v) != 0 {
		t.Fatalf("vorbemerkungen vor dem Parsen: %v", leer)
	}

	ctx := context.Background()
	result, err := (quotes.GAEBDAXMLParser{}).ParseGAEB(ctx, bytes.NewReader(raw), "lv-02.x83")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := quotes.NewService(env.PG, nil).SaveImportParse(ctx, created.ID, result, "default"); err != nil {
		t.Fatal(err)
	}

	var structure quotes.QuoteImportStructure
	if code := get("/api/v1/quotes/imports/"+created.ID+"/structure", &structure); code != http.StatusOK {
		t.Fatalf("Struktur: %d", code)
	}
	if structure.ImportID != created.ID || len(structure.Groups) != len(result.Groups) || len(structure.Vorbemerkungen) != 1 ||
		structure.Vorbemerkungen[0] != result.Vorbemerkungen[0] {
		t.Fatalf("Struktur: %+v", structure)
	}
	byOutline := map[string]quotes.QuoteImportGroup{}
	for i, g := range structure.Groups {
		if g.SortOrder != i+1 || g.ID == "" {
			t.Fatalf("Reihenfolge: %+v", structure.Groups)
		}
		byOutline[g.OutlineNo] = g
	}
	los, titel := byOutline["1"], byOutline["1.01"]
	if los.Kind != "los" || los.Bezeichnung != "Fassade" || los.ParentGroupID != "" || titel.Kind != "titel" || titel.ParentGroupID != los.ID {
		t.Fatalf("Los/Titel: %+v %+v", los, titel)
	}

	var items []quotes.QuoteImportItem
	if code := get("/api/v1/quotes/imports/"+created.ID+"/items", &items); code != http.StatusOK || len(items) != 26 {
		t.Fatalf("Positionen: %d %d", code, len(items))
	}
	first := items[0]
	if first.PositionNo != "1.01.0010" || first.ShortText != "Pfosten-Riegel-Fassade Aluminium" || first.LongText == "" ||
		first.ItemKind != "normal" || first.GroupID != titel.ID || first.BasePositionNo != "" {
		t.Fatalf("erste Position: %+v", first)
	}
	var alt, lumpSum *quotes.QuoteImportItem
	for i := range items {
		if items[i].ItemKind == "alternativposition" && alt == nil {
			alt = &items[i]
		}
		if items[i].IsLumpSum && lumpSum == nil {
			lumpSum = &items[i]
		}
	}
	if alt == nil || alt.BasePositionNo == "" || !alt.IsOptional || lumpSum == nil {
		t.Fatalf("Alternative/Pauschale: %+v %+v", alt, lumpSum)
	}
	var single quotes.QuoteImportItem
	if code := get("/api/v1/quotes/imports/"+created.ID+"/items/"+alt.ID, &single); code != http.StatusOK ||
		single.ItemKind != "alternativposition" || single.BasePositionNo != alt.BasePositionNo || single.ShortText != alt.ShortText ||
		single.LongText != alt.LongText || single.GroupID != alt.GroupID || single.IsLumpSum != alt.IsLumpSum {
		t.Fatalf("Einzelposition: %d %+v", code, single)
	}
	if code := get("/api/v1/quotes/imports/"+created.ID+"/items/"+lumpSum.ID, &single); code != http.StatusOK || !single.IsLumpSum {
		t.Fatalf("Pauschale einzeln: %d %+v", code, single)
	}

	// unbekannter und ungültiger Importlauf
	if code := get("/api/v1/quotes/imports/"+uuid.NewString()+"/structure", nil); code != http.StatusNotFound {
		t.Fatalf("unbekannter Importlauf: %d", code)
	}
	if code := get("/api/v1/quotes/imports/kein-uuid/structure", nil); code != http.StatusBadRequest {
		t.Fatalf("ungültige id: %d", code)
	}
}
