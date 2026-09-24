package apihttp

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"nalaerp3/internal/testutil"
)

// TestZugferdEndpointEndToEnd deckt E.4.3.4 ab (letzte Subtask von Task
// E.4): GET /invoices-out/{id}/zugferd liefert die bestehende
// Rechnungs-PDF mit eingebetteter CII-XML (factur-x.xml).
//
// Der Test verlässt sich NICHT darauf, dass die XML irgendwo im PDF als
// Klartext auftaucht - gofpdf speichert Anhänge zlib-komprimiert. Der
// eingebettete Datenstrom wird deshalb tatsächlich aus dem PDF
// herausgelöst und entpackt, und das Ergebnis Byte für Byte mit der
// Ausgabe des XRechnung-Endpunkts verglichen. Nur so ist bewiesen, dass
// wirklich dieselbe, vollständige Rechnung eingebettet ist.
func TestZugferdEndpointEndToEnd(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "zugferd@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "zugferd@example.com", "Secret123!")
	ctx := context.Background()

	if _, err := env.PG.Exec(ctx, `
        UPDATE company_profiles
           SET street='Werkstrasse 4', postal_code='40213', city='Duesseldorf', country='DE',
               vat_id='DE123456789'
         WHERE id='default'
    `); err != nil {
		t.Fatalf("seed seller profile: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, email)
        VALUES ('c-zf-http','org','customer','Bauamt ZUGFeRD','rechnung@bauamt-zf.example')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contact_addresses (id, contact_id, art, zeile1, plz, ort, land, is_primary)
        VALUES ('a-zf-http','c-zf-http','billing','Domplatz 5','50667','Koeln','DE',true)
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed address: %v", err)
	}

	createBody := `{"contact_id":"c-zf-http","invoice_date":"2026-07-01T00:00:00Z","currency":"EUR",
        "items":[{"description":"Haustuer Aluminium","qty":1,"unit_price":2000,"tax_code":"DE19","account_code":"8000"}]}`
	createRec := doAuthedRequest(t, handler, accessToken, http.MethodPost, "/api/v1/invoices-out/", createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create invoice: expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created invoice: %v", err)
	}

	// Negativfall 1: draft -> 400, noch bevor eine PDF gebaut wird.
	draftRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/zugferd", "")
	if draftRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for draft invoice, got %d: %s", draftRec.Code, draftRec.Body.String())
	}

	if rec := doAuthedRequest(t, handler, accessToken, http.MethodPost, "/api/v1/invoices-out/"+created.ID+"/book", ""); rec.Code != http.StatusOK {
		t.Fatalf("book invoice: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Negativfall 2: gebucht, aber ohne Kaeuferreferenz -> 400.
	noRefRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/zugferd", "")
	if noRefRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without buyer reference, got %d: %s", noRefRec.Code, noRefRec.Body.String())
	}

	if rec := doAuthedRequest(t, handler, accessToken, http.MethodPatch,
		"/api/v1/invoices-out/"+created.ID+"/buyer-reference", `{"buyer_reference":"04011000-55555-01"}`); rec.Code != http.StatusOK {
		t.Fatalf("set buyer reference: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Referenz: die XML, die der XRechnung-Endpunkt liefert.
	xmlRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/xrechnung", "")
	if xmlRec.Code != http.StatusOK {
		t.Fatalf("xrechnung: expected 200, got %d: %s", xmlRec.Code, xmlRec.Body.String())
	}
	wantXML := xmlRec.Body.Bytes()

	// Happy Path: ZUGFeRD-PDF abrufen.
	rec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/zugferd", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("zugferd: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, erwartet application/pdf", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, `filename="zugferd_`) || !strings.HasSuffix(cd, `.pdf"`) {
		t.Errorf("Content-Disposition unerwartet: %q", cd)
	}

	pdfBytes := rec.Body.Bytes()
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("Antwort ist keine PDF, Anfang: %q", pdfBytes[:min(20, len(pdfBytes))])
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(pdfBytes)) {
		t.Errorf("Content-Length = %q, tatsächliche Länge %d", got, len(pdfBytes))
	}

	// Die PDF muss einen eingebetteten Dateianhang enthalten.
	if !bytes.Contains(pdfBytes, []byte("/Type /EmbeddedFile")) {
		t.Fatal("PDF enthält kein /EmbeddedFile-Objekt")
	}
	if !bytes.Contains(pdfBytes, []byte("/Type /Filespec")) {
		t.Fatal("PDF enthält kein /Filespec-Objekt")
	}

	// Pruefsumme und unkomprimierte Laenge stehen im EmbeddedFile-Objekt -
	// beides muss zur XML des XRechnung-Endpunkts passen.
	sum := md5.Sum(wantXML)
	wantChecksum := strings.ToUpper(hex.EncodeToString(sum[:]))
	if !bytes.Contains(bytes.ToUpper(pdfBytes), []byte("/CHECKSUM <"+wantChecksum+">")) {
		t.Errorf("MD5-Prüfsumme der eingebetteten Datei passt nicht zur XRechnung-XML (erwartet %s)", wantChecksum)
	}
	if !bytes.Contains(pdfBytes, []byte("/Size "+strconv.Itoa(len(wantXML)))) {
		t.Errorf("unkomprimierte Länge der eingebetteten Datei passt nicht (erwartet %d)", len(wantXML))
	}

	// Entscheidender Nachweis: den eingebetteten Strom tatsaechlich
	// herausloesen, entpacken und vergleichen.
	gotXML := extractEmbeddedFile(t, pdfBytes)
	if !bytes.Equal(gotXML, wantXML) {
		t.Fatalf("eingebettete factur-x.xml weicht von der XRechnung-XML ab.\nEingebettet (%d Bytes):\n%s\n\nErwartet (%d Bytes):\n%s",
			len(gotXML), gotXML, len(wantXML), wantXML)
	}
	for _, want := range []string{
		"<rsm:CrossIndustryInvoice",
		"<ram:BuyerReference>04011000-55555-01</ram:BuyerReference>",
		"<ram:GrandTotalAmount>2380.00</ram:GrandTotalAmount>",
	} {
		if !strings.Contains(string(gotXML), want) {
			t.Errorf("eingebettete XML enthält %q nicht", want)
		}
	}

	// Negativfall 3: unbekannte Rechnungs-ID -> 404.
	if r := doAuthedRequest(t, handler, accessToken, http.MethodGet,
		"/api/v1/invoices-out/00000000-0000-0000-0000-00000000feed/zugferd", ""); r.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown invoice, got %d: %s", r.Code, r.Body.String())
	}

	// Negativfall 4: ungültige Rechnungs-ID -> 400.
	if r := doAuthedRequest(t, handler, accessToken, http.MethodGet,
		"/api/v1/invoices-out/keine-uuid/zugferd", ""); r.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed id, got %d: %s", r.Code, r.Body.String())
	}

	// Der bestehende PDF-Endpunkt muss unverändert funktionieren und darf
	// KEINEN Anhang tragen - buildInvoiceOutPDF wurde in E.4.3.4 aus dem
	// Handler herausgezogen, das darf sein Verhalten nicht ändern.
	plainRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/pdf", "")
	if plainRec.Code != http.StatusOK {
		t.Fatalf("pdf: expected 200, got %d: %s", plainRec.Code, plainRec.Body.String())
	}
	if !bytes.HasPrefix(plainRec.Body.Bytes(), []byte("%PDF-")) {
		t.Error("PDF-Endpunkt liefert keine PDF mehr")
	}
	if bytes.Contains(plainRec.Body.Bytes(), []byte("/Type /EmbeddedFile")) {
		t.Error("die normale Rechnungs-PDF darf keinen eingebetteten Anhang tragen")
	}
	if cd := plainRec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="Rechnung_`) {
		t.Errorf("Dateiname des PDF-Endpunkts hat sich geändert: %q", cd)
	}
}

// extractEmbeddedFile löst den ersten /EmbeddedFile-Datenstrom aus einer
// PDF heraus und entpackt ihn (gofpdf schreibt ihn zlib-komprimiert als
// /Filter /FlateDecode).
func extractEmbeddedFile(t *testing.T, pdf []byte) []byte {
	t.Helper()

	idx := bytes.Index(pdf, []byte("/Type /EmbeddedFile"))
	if idx == -1 {
		t.Fatal("kein /EmbeddedFile-Objekt gefunden")
	}
	rest := pdf[idx:]

	// Auf das Objekt folgt "stream\n" ... "\nendstream".
	streamStart := bytes.Index(rest, []byte("stream\n"))
	if streamStart == -1 {
		t.Fatal("kein stream im EmbeddedFile-Objekt gefunden")
	}
	body := rest[streamStart+len("stream\n"):]
	streamEnd := bytes.Index(body, []byte("\nendstream"))
	if streamEnd == -1 {
		t.Fatal("kein endstream gefunden")
	}
	compressed := body[:streamEnd]

	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("eingebetteter Strom ist nicht zlib-komprimiert: %v", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("eingebetteten Strom entpacken: %v", err)
	}
	return out
}
