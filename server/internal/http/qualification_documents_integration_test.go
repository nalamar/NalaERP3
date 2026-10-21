package apihttp

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Nachweisdokumente zu Qualifikationen (Backlog F.10.1).

type qualDocResponse struct {
	ID              uuid.UUID `json:"id"`
	QualificationID uuid.UUID `json:"qualification_id"`
	DocumentID      string    `json:"document_id"`
	Filename        string    `json:"filename"`
	ContentType     string    `json:"content_type"`
	Length          int64     `json:"length"`
}

var (
	pdfBytes  = []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << >>\n%%EOF\n")
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0}, bytes.Repeat([]byte{1}, 32)...)
)

func uploadQualDoc(t *testing.T, handler http.Handler, token, qualID, field, filename, partType string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	h["Content-Type"] = []string{partType}
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qualifications/"+qualID+"/documents", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestQualificationDocumentsHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	worker := tenant.linkedUser("sales", "Schweisser")
	colleague := tenant.linkedUser("sales", "Kollege")
	otherHR := other.user("hr")
	typ := tenant.qualType(hrToken, "SCHW_DOK", "null")
	own := decodeQualRecords(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath, qualBody(typ.ID, "2026-03-15", worker.EmployeeID)), http.StatusCreated)[0]
	theirs := decodeQualRecords(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath, qualBody(typ.ID, "2026-03-15", colleague.EmployeeID)), http.StatusCreated)[0]
	ownID := own.ID.String()

	// Hochladen: Art am Inhalt erkannt, Pfad aus dem Dateinamen entfernt.
	var pdf qualDocResponse
	decodeJSONStatus(t, uploadQualDoc(t, handler, hrToken, ownID, "file", `C:\Scans\zertifikat.pdf`, "application/octet-stream", pdfBytes), http.StatusCreated, &pdf)
	if pdf.ContentType != "application/pdf" || pdf.Filename != "zertifikat.pdf" || pdf.Length != int64(len(pdfBytes)) || pdf.QualificationID != own.ID {
		t.Fatalf("PDF: %+v", pdf)
	}
	var png, jpg qualDocResponse
	decodeJSONStatus(t, uploadQualDoc(t, handler, hrToken, ownID, "file", "liste.png", "image/png", pngBytes), http.StatusCreated, &png)
	decodeJSONStatus(t, uploadQualDoc(t, handler, hrToken, ownID, "file", "../../foto.jpg", "image/jpeg", jpegBytes), http.StatusCreated, &jpg)
	if png.ContentType != "image/png" || jpg.ContentType != "image/jpeg" || jpg.Filename != "foto.jpg" {
		t.Fatalf("PNG/JPEG: %+v %+v", png, jpg)
	}

	// Abgelehnt: falsche Art (auch bei falscher Angabe), leer, zu gross, ohne Feld.
	wantAPIError(t, uploadQualDoc(t, handler, hrToken, ownID, "file", "x.pdf", "application/pdf", []byte("nur Text, kein PDF")),
		http.StatusBadRequest, "validation_error", "Ungültige Datei: erlaubt sind PDF, JPEG und PNG")
	wantAPIError(t, uploadQualDoc(t, handler, hrToken, ownID, "file", "leer.pdf", "application/pdf", nil),
		http.StatusBadRequest, "validation_error", "Datei erforderlich")
	big := append(append([]byte{}, pdfBytes...), bytes.Repeat([]byte{'x'}, 10<<20)...)
	wantAPIError(t, uploadQualDoc(t, handler, hrToken, ownID, "file", "gross.pdf", "application/pdf", big),
		http.StatusBadRequest, "validation_error", "Ungültige Datei: höchstens 10 MB")
	wantAPIError(t, uploadQualDoc(t, handler, hrToken, ownID, "datei", "x.pdf", "application/pdf", pdfBytes),
		http.StatusBadRequest, "validation_error", "Datei fehlt (Feld 'file')")
	wantAPIError(t, uploadQualDoc(t, handler, hrToken, "kein-uuid", "file", "x.pdf", "application/pdf", pdfBytes),
		http.StatusBadRequest, "validation_error", "Ungültige Nachweis-ID")
	// Hochladen nur mit qualifications.write; fremder Mandant findet nichts.
	wantStatus(t, uploadQualDoc(t, handler, worker.Token, ownID, "file", "x.pdf", "application/pdf", pdfBytes), http.StatusForbidden)
	wantAPIError(t, uploadQualDoc(t, handler, otherHR, ownID, "file", "x.pdf", "application/pdf", pdfBytes),
		http.StatusNotFound, "not_found", "Nachweis nicht gefunden")

	// Liste: eigene mit .self, alle mit .read; Reihenfolge des Hochladens.
	var list []qualDocResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, qualPath+ownID+"/documents", ""), http.StatusOK, &list)
	if len(list) != 3 || list[0].ID != pdf.ID || list[2].ID != jpg.ID {
		t.Fatalf("Liste: %+v", list)
	}
	wantStatus(t, doAuthedRequest(t, handler, colleague.Token, http.MethodGet, qualPath+ownID+"/documents", ""), http.StatusForbidden)
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, qualPath+theirs.ID.String()+"/documents", ""), http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("fremder Nachweis hat Dokumente: %+v", list)
	}

	// Download: Inhalt, Kopfzeilen; nur ueber den richtigen Nachweis.
	rec := doAuthedRequest(t, handler, worker.Token, http.MethodGet, qualPath+ownID+"/documents/"+pdf.ID.String(), "")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pdfBytes) || rec.Header().Get("Content-Type") != "application/pdf" ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "zertifikat.pdf") {
		t.Fatalf("Download: %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"))
	}
	wantStatus(t, doAuthedRequest(t, handler, colleague.Token, http.MethodGet, qualPath+ownID+"/documents/"+pdf.ID.String(), ""), http.StatusForbidden)
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, qualPath+theirs.ID.String()+"/documents/"+pdf.ID.String(), ""),
		http.StatusNotFound, "not_found", "Dokument nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, qualPath+ownID+"/documents/kein-uuid", ""),
		http.StatusBadRequest, "validation_error", "Ungültige Dokument-ID")
	wantAPIError(t, doAuthedRequest(t, handler, otherHR, http.MethodGet, qualPath+ownID+"/documents/"+pdf.ID.String(), ""),
		http.StatusNotFound, "not_found", "Nachweis nicht gefunden")

	// Der allgemeine Download kennt Nachweisdokumente nicht - auch mit
	// documents.read (die Rolle sales hat es).
	wantStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/documents/"+pdf.DocumentID, ""), http.StatusNotFound)

	// Zu einem stornierten Nachweis kommt nichts mehr hinzu; die
	// vorhandenen Dokumente bleiben lesbar.
	decodeQualRecord(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, qualPath+ownID+"/storno", `{"reason":"falsch erfasst"}`), http.StatusOK)
	wantAPIError(t, uploadQualDoc(t, handler, hrToken, ownID, "file", "x.pdf", "application/pdf", pdfBytes), http.StatusConflict, "conflict", "Nachweis ist storniert")
	list = nil
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, qualPath+ownID+"/documents", ""), http.StatusOK, &list)
	if len(list) != 3 {
		t.Fatalf("nach Storno: %+v", list)
	}
	var raw json.RawMessage
	decodeJSONStatus(t, doAuthedRequest(t, handler, hrToken, http.MethodGet, qualPath+ownID, ""), http.StatusOK, &raw)
}
