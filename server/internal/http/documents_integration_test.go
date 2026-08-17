package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/auth"
	"nalaerp3/internal/testutil"
)

// uploadIntegrationDocument laedt eine Datei per multipart/form-data an die
// gegebene Upload-URL hoch (z.B. /api/v1/contacts/{id}/documents oder
// /api/v1/materials/{id}/documents) und liefert die GridFS-DocumentID zurueck.
func uploadIntegrationDocument(t *testing.T, handler http.Handler, accessToken, uploadURL, filename string, content []byte) string {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, uploadURL, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected document upload 201, got %d with body %s", rec.Code, rec.Body.String())
	}
	var uploaded struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploaded.DocumentID == "" {
		t.Fatal("expected uploaded document id")
	}
	return uploaded.DocumentID
}

// TestDocumentDownloadIsScopedToCompany belegt Backlog 0.22: der generische
// Download-Endpunkt GET /api/v1/documents/{docID} pruefte vorher NICHT, ob
// das referenzierte Dokument (material_documents/contact_documents) zum
// Mandanten des anfragenden Nutzers gehoert - jede bekannte oder erratene
// GridFS-ObjectID war mandantenuebergreifend abrufbar, unabhaengig davon, ob
// ueberhaupt eine Verknuepfung zu einem eigenen Datensatz bestand. Deckt
// beide betroffenen Domaenen ab (materials UND contacts, wie im
// Backlog-Text benannt).
func TestDocumentDownloadIsScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	testutil.SeedAuthUser(t, env, "integration-docs-owner@example.com", "Secret123!", "admin")

	if _, err := env.PG.Exec(ctx, `
		INSERT INTO company_profiles (id, name) VALUES ('itest-other-company-docs', 'Andere Firma Dokumente')
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		t.Fatalf("seed other company: %v", err)
	}
	passwordHash, err := auth.HashPassword("Secret123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	const otherCompanyUserID = "itest-other-company-docs-user"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO users (id, email, username, password_hash, first_name, last_name, display_name, locale, timezone, is_active, is_locked, company_id)
		VALUES ($1,$2,$2,$3,'Andere','Firma','Andere Firma','de-DE','Europe/Berlin',true,false,'itest-other-company-docs')
		ON CONFLICT (email) DO UPDATE SET company_id=EXCLUDED.company_id
	`, otherCompanyUserID, "integration-other-company-docs-user@example.com", passwordHash); err != nil {
		t.Fatalf("seed other-company user: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, r.id FROM roles r WHERE r.code = 'admin'
		ON CONFLICT DO NOTHING
	`, otherCompanyUserID); err != nil {
		t.Fatalf("seed other-company user role: %v", err)
	}

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	ownerToken := loginIntegrationUser(t, handler, "integration-docs-owner@example.com", "Secret123!")
	otherToken := loginIntegrationUser(t, handler, "integration-other-company-docs-user@example.com", "Secret123!")

	// --- Kontakt-Dokument ---
	contactID := createIntegrationContact(t, handler, ownerToken, map[string]any{
		"typ":   "org",
		"rolle": "customer",
		"name":  "Dokument-Mandant-Test GmbH",
	})
	contactDocID := uploadIntegrationDocument(t, handler, ownerToken, "/api/v1/contacts/"+contactID+"/documents", "kontakt.txt", []byte("Kontaktdokument"))

	assertDocumentDownload(t, handler, ownerToken, contactDocID, http.StatusOK)
	assertDocumentDownload(t, handler, otherToken, contactDocID, http.StatusNotFound)

	// --- Material-Dokument ---
	materialID := createIntegrationMaterial(t, handler, ownerToken, map[string]any{
		"nummer":      "MAT-DOC-0001",
		"bezeichnung": "Dokument-Mandant-Test Material",
		"typ":         "profil",
		"einheit":     "Stk",
		"dichte":      2.7,
	})
	materialDocID := uploadIntegrationDocument(t, handler, ownerToken, "/api/v1/materials/"+materialID+"/documents", "material.txt", []byte("Materialdokument"))

	assertDocumentDownload(t, handler, ownerToken, materialDocID, http.StatusOK)
	assertDocumentDownload(t, handler, otherToken, materialDocID, http.StatusNotFound)
}

func assertDocumentDownload(t *testing.T, handler http.Handler, accessToken, documentID string, wantStatus int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+documentID, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("expected %d for document download, got %d with body %s", wantStatus, rec.Code, rec.Body.String())
	}
}
