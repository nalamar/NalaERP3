package apihttp

// End-to-End-Test fuer POST /invoices-in/parse-e-invoice (Backlog E.5.5,
// letzte Subtask von Task E.5).
//
// Die Parser selbst sind in internal/accounting DB-los abgedeckt (E.5.2
// bis E.5.4). Dieser Test beweist das Wiring: Route, Permission,
// Multipart-Upload, die Weiche XML/PDF, den Lieferanten-Vorschlag aus
// echten Stammdaten, die Groessenbegrenzung - und dass NICHTS
// persistiert wird.

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/accounting"
	"nalaerp3/internal/pdfgen"
	"nalaerp3/internal/testutil"
)

const inboundCIIForUpload = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
    <rsm:ExchangedDocument>
        <ram:ID>EIN-2026-0042</ram:ID>
        <ram:TypeCode>380</ram:TypeCode>
        <ram:IssueDateTime>
            <udt:DateTimeString format="102">20260610</udt:DateTimeString>
        </ram:IssueDateTime>
    </rsm:ExchangedDocument>
    <rsm:SupplyChainTradeTransaction>
        <ram:IncludedSupplyChainTradeLineItem>
            <ram:AssociatedDocumentLineDocument>
                <ram:LineID>1</ram:LineID>
            </ram:AssociatedDocumentLineDocument>
            <ram:SpecifiedTradeProduct>
                <ram:Name>Dichtungsprofil EPDM</ram:Name>
            </ram:SpecifiedTradeProduct>
            <ram:SpecifiedLineTradeAgreement>
                <ram:NetPriceProductTradePrice>
                    <ram:ChargeAmount>4.00</ram:ChargeAmount>
                </ram:NetPriceProductTradePrice>
            </ram:SpecifiedLineTradeAgreement>
            <ram:SpecifiedLineTradeDelivery>
                <ram:BilledQuantity unitCode="MTR">50</ram:BilledQuantity>
            </ram:SpecifiedLineTradeDelivery>
            <ram:SpecifiedLineTradeSettlement>
                <ram:ApplicableTradeTax>
                    <ram:TypeCode>VAT</ram:TypeCode>
                    <ram:CategoryCode>S</ram:CategoryCode>
                    <ram:RateApplicablePercent>19.00</ram:RateApplicablePercent>
                </ram:ApplicableTradeTax>
                <ram:SpecifiedTradeSettlementLineMonetarySummation>
                    <ram:LineTotalAmount>200.00</ram:LineTotalAmount>
                </ram:SpecifiedTradeSettlementLineMonetarySummation>
            </ram:SpecifiedLineTradeSettlement>
        </ram:IncludedSupplyChainTradeLineItem>
        <ram:ApplicableHeaderTradeAgreement>
            <ram:SellerTradeParty>
                <ram:Name>Dichtungshandel Nord GmbH</ram:Name>
                <ram:PostalTradeAddress>
                    <ram:PostcodeCode>24103</ram:PostcodeCode>
                    <ram:LineOne>Hafenweg 3</ram:LineOne>
                    <ram:CityName>Kiel</ram:CityName>
                    <ram:CountryID>DE</ram:CountryID>
                </ram:PostalTradeAddress>
                <ram:SpecifiedTaxRegistration>
                    <ram:ID schemeID="VA">DE811122233</ram:ID>
                </ram:SpecifiedTaxRegistration>
            </ram:SellerTradeParty>
            <ram:BuyerTradeParty>
                <ram:Name>Metallbau Muster GmbH</ram:Name>
            </ram:BuyerTradeParty>
        </ram:ApplicableHeaderTradeAgreement>
        <ram:ApplicableHeaderTradeDelivery/>
        <ram:ApplicableHeaderTradeSettlement>
            <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
            <ram:ApplicableTradeTax>
                <ram:CalculatedAmount>38.00</ram:CalculatedAmount>
                <ram:TypeCode>VAT</ram:TypeCode>
                <ram:BasisAmount>200.00</ram:BasisAmount>
                <ram:CategoryCode>S</ram:CategoryCode>
                <ram:RateApplicablePercent>19.00</ram:RateApplicablePercent>
            </ram:ApplicableTradeTax>
            <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
                <ram:LineTotalAmount>200.00</ram:LineTotalAmount>
                <ram:TaxBasisTotalAmount>200.00</ram:TaxBasisTotalAmount>
                <ram:TaxTotalAmount currencyID="EUR">38.00</ram:TaxTotalAmount>
                <ram:GrandTotalAmount>238.00</ram:GrandTotalAmount>
                <ram:DuePayableAmount>238.00</ram:DuePayableAmount>
            </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        </ram:ApplicableHeaderTradeSettlement>
    </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`

// uploadEInvoice schickt eine Datei an den Parse-Endpunkt.
func uploadEInvoice(t *testing.T, handler http.Handler, token, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Datei schreiben: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Writer schließen: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-in/parse-e-invoice", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type parseEInvoiceResult struct {
	Quelle   string `json:"quelle"`
	Rechnung struct {
		Format          string  `json:"format"`
		Rechnungsnummer string  `json:"rechnungsnummer"`
		Waehrung        string  `json:"waehrung"`
		Bruttobetrag    float64 `json:"bruttobetrag"`
		Verkaeufer      struct {
			Name  string `json:"name"`
			UstID string `json:"ust_id"`
			Ort   string `json:"ort"`
		} `json:"verkaeufer"`
		Positionen []struct {
			Bezeichnung string  `json:"bezeichnung"`
			Menge       float64 `json:"menge"`
		} `json:"positionen"`
		Hinweise []string `json:"hinweise"`
	} `json:"rechnung"`
	Lieferantensuche struct {
		Art       string `json:"art"`
		Eindeutig bool   `json:"eindeutig"`
		Vorschlag *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"vorschlag"`
		Kandidaten []struct {
			ID string `json:"id"`
		} `json:"kandidaten"`
		Hinweis string `json:"hinweis"`
	} `json:"lieferantensuche"`
}

func decodeParseResult(t *testing.T, rec *httptest.ResponseRecorder) parseEInvoiceResult {
	t.Helper()
	var out parseEInvoiceResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort dekodieren: %v (Body: %s)", err, rec.Body.String())
	}
	return out
}

func TestParseEInvoiceEndpointEndToEnd(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "einvoice-in@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "einvoice-in@example.com", "Secret123!")
	ctx := context.Background()

	// Lieferant mit passender USt-IdNr. - bewusst mit Leerzeichen
	// gepflegt, damit der normalisierte Vergleich mitgeprüft wird.
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, vat_id, company_id)
        VALUES ('c-ein-nord','org','supplier','Dichtungshandel Nord GmbH','DE 811 122 233','default')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}

	// Bestand VOR dem Upload festhalten. Eine absolute Zaehlung waere
	// nicht aussagekraeftig: andere Tests desselben Pakets legen ebenfalls
	// Eingangsrechnungen an, und testutil setzt die DB zwischen Tests
	// nicht zurueck. Nur die Differenz ist diesem Endpunkt zurechenbar.
	invoicesInVorher := countRows(t, env.PG, "invoices_in")
	itemsVorher := countRows(t, env.PG, "invoice_in_items")

	// --- Happy Path: XML-Upload ---
	rec := uploadEInvoice(t, handler, accessToken, "rechnung.xml", []byte(inboundCIIForUpload))
	if rec.Code != http.StatusOK {
		t.Fatalf("XML-Upload: erwartet 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeParseResult(t, rec)

	if got.Quelle != "xml" {
		t.Errorf("Quelle = %q, erwartet xml", got.Quelle)
	}
	if got.Rechnung.Format != accounting.EInvoiceFormatCII {
		t.Errorf("Format = %q", got.Rechnung.Format)
	}
	if got.Rechnung.Rechnungsnummer != "EIN-2026-0042" || got.Rechnung.Bruttobetrag != 238 {
		t.Errorf("Rechnungsdaten falsch: %+v", got.Rechnung)
	}
	if got.Rechnung.Verkaeufer.Name != "Dichtungshandel Nord GmbH" || got.Rechnung.Verkaeufer.Ort != "Kiel" {
		t.Errorf("Verkäufer falsch: %+v", got.Rechnung.Verkaeufer)
	}
	if len(got.Rechnung.Positionen) != 1 || got.Rechnung.Positionen[0].Menge != 50 {
		t.Errorf("Positionen falsch: %+v", got.Rechnung.Positionen)
	}

	// Lieferantenvorschlag über die USt-IdNr., trotz abweichender
	// Schreibweise in den Stammdaten.
	if got.Lieferantensuche.Art != accounting.SupplierMatchVatID {
		t.Errorf("Zuordnungsart = %q, erwartet %q", got.Lieferantensuche.Art, accounting.SupplierMatchVatID)
	}
	if !got.Lieferantensuche.Eindeutig {
		t.Error("ein Treffer über die USt-IdNr. muss als eindeutig gelten")
	}
	if got.Lieferantensuche.Vorschlag == nil || got.Lieferantensuche.Vorschlag.ID != "c-ein-nord" {
		t.Errorf("Lieferantenvorschlag falsch: %+v", got.Lieferantensuche.Vorschlag)
	}

	// --- Es darf NICHTS persistiert worden sein (ADR 0023) ---
	if nachher := countRows(t, env.PG, "invoices_in"); nachher != invoicesInVorher {
		t.Errorf("der Parse-Endpunkt darf nichts anlegen: invoices_in ging von %d auf %d", invoicesInVorher, nachher)
	}
	if nachher := countRows(t, env.PG, "invoice_in_items"); nachher != itemsVorher {
		t.Errorf("der Parse-Endpunkt darf nichts anlegen: invoice_in_items ging von %d auf %d", itemsVorher, nachher)
	}

	// --- Happy Path: ZUGFeRD-PDF-Upload ---
	// Die PDF wird über den eigenen Ausgangsweg erzeugt, damit Ein- und
	// Ausgang einander prüfen.
	pdf := buildZugferdTestPDF(t, []byte(inboundCIIForUpload))
	pdfRec := uploadEInvoice(t, handler, accessToken, "beliebiger-name.bin", pdf)
	if pdfRec.Code != http.StatusOK {
		t.Fatalf("PDF-Upload: erwartet 200, got %d: %s", pdfRec.Code, pdfRec.Body.String())
	}
	pdfGot := decodeParseResult(t, pdfRec)
	if pdfGot.Quelle != "pdf" {
		t.Errorf("Quelle = %q, erwartet pdf - die Erkennung muss am Inhalt hängen, nicht am Dateinamen", pdfGot.Quelle)
	}
	if pdfGot.Rechnung.Rechnungsnummer != "EIN-2026-0042" {
		t.Errorf("aus der PDF gelesene Rechnungsnummer = %q", pdfGot.Rechnung.Rechnungsnummer)
	}
	if pdfGot.Lieferantensuche.Vorschlag == nil || pdfGot.Lieferantensuche.Vorschlag.ID != "c-ein-nord" {
		t.Errorf("Lieferantenvorschlag beim PDF-Weg falsch: %+v", pdfGot.Lieferantensuche)
	}

	// --- Unbekannter Lieferant ---
	fremd := strings.Replace(inboundCIIForUpload, "DE811122233", "DE999888777", 1)
	fremd = strings.Replace(fremd, "Dichtungshandel Nord GmbH", "Voellig Unbekannt AG", 1)
	fremdRec := uploadEInvoice(t, handler, accessToken, "fremd.xml", []byte(fremd))
	if fremdRec.Code != http.StatusOK {
		t.Fatalf("erwartet 200 auch ohne Lieferantentreffer, got %d: %s", fremdRec.Code, fremdRec.Body.String())
	}
	fremdGot := decodeParseResult(t, fremdRec)
	if fremdGot.Lieferantensuche.Art != accounting.SupplierMatchNone {
		t.Errorf("Zuordnungsart = %q, erwartet %q", fremdGot.Lieferantensuche.Art, accounting.SupplierMatchNone)
	}
	if fremdGot.Lieferantensuche.Eindeutig {
		t.Error("ohne Treffer darf nicht eindeutig gemeldet werden")
	}
	if !strings.Contains(fremdGot.Lieferantensuche.Hinweis, "kein Lieferant") {
		t.Errorf("Hinweis nennt die Ursache nicht: %q", fremdGot.Lieferantensuche.Hinweis)
	}

	// --- Mehrdeutige USt-IdNr. ---
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, vat_id, company_id)
        VALUES ('c-ein-nord-2','org','supplier','Dichtungshandel Nord GmbH (alt)','DE811122233','default')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed second supplier: %v", err)
	}
	ambigRec := uploadEInvoice(t, handler, accessToken, "rechnung.xml", []byte(inboundCIIForUpload))
	if ambigRec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, got %d: %s", ambigRec.Code, ambigRec.Body.String())
	}
	ambigGot := decodeParseResult(t, ambigRec)
	if ambigGot.Lieferantensuche.Art != accounting.SupplierMatchAmbig {
		t.Errorf("Zuordnungsart = %q, erwartet %q", ambigGot.Lieferantensuche.Art, accounting.SupplierMatchAmbig)
	}
	if ambigGot.Lieferantensuche.Eindeutig {
		t.Error("bei mehreren Treffern darf nicht eindeutig gemeldet werden")
	}
	if ambigGot.Lieferantensuche.Vorschlag != nil {
		t.Error("bei Mehrdeutigkeit darf kein einzelner Vorschlag gesetzt werden - das würde eine Entscheidung vortäuschen")
	}
	if len(ambigGot.Lieferantensuche.Kandidaten) != 2 {
		t.Errorf("erwartet 2 Kandidaten, got %d", len(ambigGot.Lieferantensuche.Kandidaten))
	}
}

func TestParseEInvoiceEndpointRejectsBadInput(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "einvoice-in-neg@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "einvoice-in-neg@example.com", "Secret123!")

	cases := []struct {
		name     string
		content  []byte
		wantCode int
		wantSub  string
	}{
		{"leere Datei", []byte{}, http.StatusBadRequest, "leer"},
		{"kein XML", []byte("Guten Tag, anbei meine Rechnung."), http.StatusBadRequest, "nicht gelesen werden"},
		{
			"XML, aber keine E-Rechnung",
			[]byte(`<?xml version="1.0"?><Lieferschein><Nr>7</Nr></Lieferschein>`),
			http.StatusBadRequest, "unbekanntes Format",
		},
		{
			"PDF ohne eingebettete Rechnung",
			buildZugferdTestPDF(t, nil),
			http.StatusBadRequest, "keine eingebettete Rechnungsdatei",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := uploadEInvoice(t, handler, accessToken, "upload.dat", c.content)
			if rec.Code != c.wantCode {
				t.Fatalf("erwartet %d, got %d: %s", c.wantCode, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.wantSub) {
				t.Errorf("Antwort nennt %q nicht: %s", c.wantSub, rec.Body.String())
			}
		})
	}
}

// TestParseEInvoiceEndpointRejectsOversizedUpload weist die in ADR 0023
// geforderte Groessenbegrenzung nach - ausdruecklich als Nachweis, nicht
// als Behauptung.
func TestParseEInvoiceEndpointRejectsOversizedUpload(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "einvoice-in-big@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "einvoice-in-big@example.com", "Secret123!")

	// Knapp über der Grenze, damit der Test nicht unnötig Speicher zieht.
	oversized := bytes.Repeat([]byte("A"), maxEInvoiceUploadBytes+1024)

	rec := uploadEInvoice(t, handler, accessToken, "riesig.xml", oversized)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("erwartet 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestParseEInvoiceEndpointRequiresPermission stellt sicher, dass der
// Endpunkt nicht ohne das Schreibrecht fuer Eingangsrechnungen nutzbar
// ist.
func TestParseEInvoiceEndpointRequiresPermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-in/parse-e-invoice", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Fatalf("ohne Anmeldung erwartet 401/403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// buildZugferdTestPDF erzeugt eine Rechnungs-PDF. Ist xml nicht nil, wird
// sie als factur-x.xml eingebettet - also genau der Weg, den auch unser
// eigener ZUGFeRD-Ausgang nimmt (E.4.3.4). Damit pruefen Ausgang und
// Eingang einander, statt beide gegen eine selbst gebaute Vorstellung zu
// laufen. Ohne xml entsteht eine gewoehnliche PDF ohne Anhang.
func buildZugferdTestPDF(t *testing.T, xml []byte) []byte {
	t.Helper()
	var attachments []pdfgen.Attachment
	if xml != nil {
		attachments = []pdfgen.Attachment{{
			Filename: "factur-x.xml",
			Content:  xml,
		}}
	}
	data := pdfgen.InvoiceOutData{
		Number:      "EIN-TEST",
		InvoiceDate: "10.06.2026",
		Currency:    "EUR",
		ContactName: "Dichtungshandel Nord GmbH",
	}
	pdf, err := pdfgen.RenderInvoiceOutWithAttachments(
		context.Background(), nil, "", data, pdfgen.TemplateOptions{}, nil, attachments)
	if err != nil {
		t.Fatalf("Test-PDF konnte nicht erzeugt werden: %v", err)
	}
	return pdf
}

// countRows zaehlt Zeilen einer Tabelle - fuer Vorher/Nachher-Vergleiche,
// die im gemeinsamen Testlauf aussagekraeftig bleiben.
func countRows(t *testing.T, pg *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pg.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
