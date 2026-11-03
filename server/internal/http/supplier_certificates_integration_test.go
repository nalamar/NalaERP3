package apihttp

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Zertifikate am Lieferanten ueber HTTP (Backlog H.4.2b). Helfer aus
// qualification_types_integration_test.go (newCatalogTenant),
// asset_inspections_integration_test.go (decodeJSON) und
// time_entries_integration_test.go (wantAPIError).

const supplierCertificatesAPI = "/api/v1/supplier-certificates/"

type supplierCertificateResponse struct {
	ID           uuid.UUID `json:"id"`
	SupplierID   string    `json:"lieferant_id"`
	Art          string    `json:"art"`
	ExcBis       *string   `json:"exc_bis"`
	GueltigBis   *string   `json:"gueltig_bis"`
	StornoReason string    `json:"storno_grund"`
	StorniertAt  *string   `json:"storniert_am"`
	Hinweise     []string  `json:"hinweise"`
}

func uploadMultipart(t *testing.T, handler http.Handler, token, url, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestSupplierCertificatesHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	buyer := c.user("procurement")
	supplier := "sup-http-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Zulieferer',$2)`, supplier, c.company); err != nil {
		t.Fatal(err)
	}
	do := func(token, method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}

	created := decodeJSON[supplierCertificateResponse](t, do(buyer, http.MethodPost, supplierCertificatesAPI,
		`{"lieferant_id":"`+supplier+`","art":"en1090","nummer":"1090-CPR-1","exc_bis":"exc2","gueltig_bis":"2030-12-31"}`), http.StatusCreated)
	if created.Art != "en1090" || created.ExcBis == nil || *created.ExcBis != "EXC2" || created.GueltigBis == nil ||
		!strings.HasPrefix(*created.GueltigBis, "2030-12-31") || created.SupplierID != supplier || created.Hinweise == nil {
		t.Fatalf("angelegt: %+v", created)
	}

	// Eingaben: alle Meldungen der Regeln kommen als 400, nicht als 500.
	sup := `"lieferant_id":"` + supplier + `",`
	for body, want := range map[string]struct {
		status int
		msg    string
	}{
		`{"art":"iso3834"}`:            {http.StatusBadRequest, "Lieferant erforderlich (lieferant_id)"},
		`{`:                            {http.StatusBadRequest, "Ungültige Eingabe"},
		`{` + sup + `"art":"iso9001"}`: {http.StatusBadRequest, "Ungültige Art des Zertifikats"},
		`{` + sup + `"art":"en1090"}`:  {http.StatusBadRequest, "Ausführungsklasse erforderlich"},
		`{` + sup + `"art":"en1090","exc_bis":"EXC5"}`:                                       {http.StatusBadRequest, "Ungültige Ausführungsklasse"},
		`{` + sup + `"art":"iso3834","exc_bis":"EXC2"}`:                                      {http.StatusBadRequest, "Ungültige Ausführungsklasse: nur bei EN 1090"},
		`{` + sup + `"art":"iso3834","gueltig_von":"2026-02-01","gueltig_bis":"2026-01-31"}`: {http.StatusBadRequest, "Ungültiger Zeitraum: gültig bis liegt vor gültig von"},
		`{` + sup + `"art":"iso3834","gueltig_bis":"31.12.2030"}`:                            {http.StatusBadRequest, "Ungültiges Datum in gueltig_bis (erwartet JJJJ-MM-TT)"},
		`{"lieferant_id":"kein-kontakt","art":"iso3834"}`:                                    {http.StatusNotFound, "Lieferant nicht gefunden"},
	} {
		code := "validation_error"
		if want.status == http.StatusNotFound {
			code = "not_found"
		}
		wantAPIError(t, do(buyer, http.MethodPost, supplierCertificatesAPI, body), want.status, code, want.msg)
	}

	list := decodeJSON[[]supplierCertificateResponse](t, do(buyer, http.MethodGet, supplierCertificatesAPI+"?lieferant_id="+supplier, ""), http.StatusOK)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("Liste: %+v", list)
	}
	wantAPIError(t, do(buyer, http.MethodGet, supplierCertificatesAPI, ""), http.StatusBadRequest, "validation_error", "Lieferant erforderlich (lieferant_id)")
	path := supplierCertificatesAPI + created.ID.String()
	if g := decodeJSON[supplierCertificateResponse](t, do(buyer, http.MethodGet, path, ""), http.StatusOK); g.ID != created.ID {
		t.Fatalf("Einzelabruf: %+v", g)
	}
	wantAPIError(t, do(buyer, http.MethodGet, supplierCertificatesAPI+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Zertifikat-ID")
	wantAPIError(t, do(buyer, http.MethodGet, supplierCertificatesAPI+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Zertifikat nicht gefunden")

	// Dateien
	pdf := []byte("%PDF-1.4\nZertifikat\n")
	rec := uploadMultipart(t, handler, buyer, path+"/documents", "zert.pdf", pdf)
	doc := decodeJSON[struct {
		ID       uuid.UUID `json:"id"`
		Filename string    `json:"filename"`
	}](t, rec, http.StatusCreated)
	wantAPIError(t, uploadMultipart(t, handler, buyer, path+"/documents", "x.txt", []byte("Text")), http.StatusBadRequest, "validation_error", "Ungültige Datei: erlaubt sind PDF, JPEG und PNG")
	wantAPIError(t, do(buyer, http.MethodPost, path+"/documents", `{}`), http.StatusBadRequest, "validation_error", "Ungültiges Upload-Formular oder Datei zu groß")
	if docs := decodeJSON[[]struct {
		ID uuid.UUID `json:"id"`
	}](t, do(buyer, http.MethodGet, path+"/documents", ""), http.StatusOK); len(docs) != 1 || docs[0].ID != doc.ID {
		t.Fatalf("Dateien: %+v", docs)
	}
	dl := do(buyer, http.MethodGet, path+"/documents/"+doc.ID.String(), "")
	if dl.Code != http.StatusOK || !bytes.Equal(dl.Body.Bytes(), pdf) || dl.Header().Get("Content-Type") != "application/pdf" ||
		!strings.Contains(dl.Header().Get("Content-Disposition"), "zert.pdf") {
		t.Fatalf("Download: %d %q %v", dl.Code, dl.Body.String(), dl.Header())
	}
	wantAPIError(t, do(buyer, http.MethodGet, path+"/documents/"+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Dokument nicht gefunden")

	// Rechte: Buchhaltung liest (purchase_orders.read), schreibt nicht;
	// Vertrieb sieht nichts.
	finance := c.user("finance")
	if l := decodeJSON[[]supplierCertificateResponse](t, do(finance, http.MethodGet, supplierCertificatesAPI+"?lieferant_id="+supplier, ""), http.StatusOK); len(l) != 1 {
		t.Fatalf("Buchhaltung liest: %+v", l)
	}
	if dl := do(finance, http.MethodGet, path+"/documents/"+doc.ID.String(), ""); dl.Code != http.StatusOK {
		t.Fatalf("Buchhaltung Download: %d", dl.Code)
	}
	for _, r := range []*httptest.ResponseRecorder{
		do(finance, http.MethodPost, supplierCertificatesAPI, `{"lieferant_id":"`+supplier+`","art":"iso3834"}`),
		do(finance, http.MethodPost, path+"/storno", `{"reason":"x"}`),
		uploadMultipart(t, handler, finance, path+"/documents", "zert.pdf", pdf),
	} {
		if r.Code != http.StatusForbidden {
			t.Fatalf("Buchhaltung schreibt: %d %s", r.Code, r.Body.String())
		}
	}
	sales := c.user("sales")
	for _, p := range []string{supplierCertificatesAPI + "?lieferant_id=" + supplier, path, path + "/documents", path + "/documents/" + doc.ID.String()} {
		if r := do(sales, http.MethodGet, p, ""); r.Code != http.StatusForbidden {
			t.Fatalf("Vertrieb liest %s: %d", p, r.Code)
		}
	}
	// Anderer Mandant: Lieferant und Zertifikat gibt es dort nicht.
	stranger := newCatalogTenant(t, env, handler).user("procurement")
	wantAPIError(t, do(stranger, http.MethodGet, path, ""), http.StatusNotFound, "not_found", "Zertifikat nicht gefunden")
	wantAPIError(t, do(stranger, http.MethodGet, supplierCertificatesAPI+"?lieferant_id="+supplier, ""), http.StatusNotFound, "not_found", "Lieferant nicht gefunden")

	// Storno
	wantAPIError(t, do(buyer, http.MethodPost, path+"/storno", `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	wantAPIError(t, do(buyer, http.MethodPost, path+"/storno", `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	st := decodeJSON[supplierCertificateResponse](t, do(buyer, http.MethodPost, path+"/storno", `{"reason":"ersetzt"}`), http.StatusOK)
	if st.StorniertAt == nil || st.StornoReason != "ersetzt" {
		t.Fatalf("storniert: %+v", st)
	}
	wantAPIError(t, do(buyer, http.MethodPost, path+"/storno", `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Zertifikat ist bereits storniert")
	wantAPIError(t, uploadMultipart(t, handler, buyer, path+"/documents", "zert.pdf", pdf), http.StatusConflict, "conflict", "Zertifikat ist storniert")
	if l := decodeJSON[[]supplierCertificateResponse](t, do(buyer, http.MethodGet, supplierCertificatesAPI+"?lieferant_id="+supplier, ""), http.StatusOK); len(l) != 0 {
		t.Fatalf("ohne stornierte: %+v", l)
	}
	if l := decodeJSON[[]supplierCertificateResponse](t, do(buyer, http.MethodGet, supplierCertificatesAPI+"?lieferant_id="+supplier+"&include_storniert=true", ""), http.StatusOK); len(l) != 1 {
		t.Fatalf("mit stornierten: %+v", l)
	}
}
