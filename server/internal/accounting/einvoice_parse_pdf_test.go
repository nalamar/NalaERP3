package accounting

// Unit-Tests fuer den ZUGFeRD-/Factur-X-Eingang (Backlog E.5.4).
// DB-los: die Test-PDFs werden zur Laufzeit erzeugt.
//
// Die Fixtures entstehen ueber dieselbe Renderfunktion, die auch unser
// ZUGFeRD-Ausgang nutzt (pdfgen.RenderInvoiceOutWithAttachments, E.4.3.4).
// Damit pruefen Ausgang und Eingang einander, statt dass beide nur gegen
// eine selbst gebaute Vorstellung von "so sieht eine ZUGFeRD-PDF aus"
// laufen.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"nalaerp3/internal/pdfgen"
)

// buildTestPDF erzeugt eine Rechnungs-PDF mit den angegebenen Anhaengen.
// mg ist nil und imageDocIDs leer - ohne Bild-IDs greift die Renderung
// nicht auf GridFS zu, es wird also keine Mongo-Verbindung gebraucht.
func buildTestPDF(t *testing.T, attachments ...pdfgen.Attachment) []byte {
	t.Helper()
	data := pdfgen.InvoiceOutData{
		Number:      "ZF-TEST-1",
		InvoiceDate: "18.05.2026",
		Currency:    "EUR",
		Status:      "booked",
		ContactName: "Profilwerk Süd GmbH",
		NetAmount:   546,
		TaxAmount:   103.74,
		GrossAmount: 649.74,
	}
	pdf, err := pdfgen.RenderInvoiceOutWithAttachments(
		context.Background(), nil, "", data, pdfgen.TemplateOptions{}, nil, attachments)
	if err != nil {
		t.Fatalf("Test-PDF konnte nicht erzeugt werden: %v", err)
	}
	return pdf
}

func ciiAttachment(name string) pdfgen.Attachment {
	return pdfgen.Attachment{
		Filename:    name,
		Description: "Rechnungsdaten",
		Content:     []byte(ciiInboundSample),
	}
}

// TestExtractEInvoiceFromPDFReadsEmbeddedCII ist der Kerntest: eine PDF
// mit eingebetteter CII-XML muss dieselbe Rechnung liefern wie das
// direkte Parsen derselben XML.
func TestExtractEInvoiceFromPDFReadsEmbeddedCII(t *testing.T) {
	pdf := buildTestPDF(t, ciiAttachment(ZugferdAttachmentName))

	got, err := ExtractEInvoiceFromPDF(pdf)
	if err != nil {
		t.Fatalf("ExtractEInvoiceFromPDF: %v", err)
	}

	want, err := ParseCIIInvoice([]byte(ciiInboundSample))
	if err != nil {
		t.Fatalf("ParseCIIInvoice: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("aus der PDF gelesene Rechnung weicht vom direkten Parsen ab.\nPDF: %+v\nXML: %+v", got, want)
	}
	if got.Number != "LIEF-2026-0815" || got.GrandTotalAmount != 649.74 {
		t.Errorf("Inhalt falsch gelesen: %+v", got)
	}
}

// TestExtractEInvoiceFromPDFReadsEmbeddedUBL beweist, dass der Anhang
// ueber ParseEInvoiceXML laeuft und nicht fest gegen CII: eine PDF mit
// UBL-Anhang wird ebenso gelesen. Das ist kein akademischer Fall - der
// Absender bestimmt die Syntax, auch innerhalb einer PDF.
func TestExtractEInvoiceFromPDFReadsEmbeddedUBL(t *testing.T) {
	pdf := buildTestPDF(t, pdfgen.Attachment{
		Filename: ZugferdAttachmentName,
		Content:  []byte(ublInboundSample),
	})

	got, err := ExtractEInvoiceFromPDF(pdf)
	if err != nil {
		t.Fatalf("ExtractEInvoiceFromPDF: %v", err)
	}
	if got.Format != EInvoiceFormatUBL {
		t.Errorf("Format = %q, erwartet %q", got.Format, EInvoiceFormatUBL)
	}
	if got.Number != "LIEF-2026-0815" {
		t.Errorf("Rechnungsnummer = %q", got.Number)
	}
}

// TestExtractEInvoiceFromPDFAcceptsOtherFilenames belegt die in E.5.4
// getroffene Entscheidung zur offenen Frage aus ADR 0023: es wird NICHT
// auf einen festen Dateinamen bestanden. Aeltere ZUGFeRD-Staende nutzen
// andere Namen, fuer die hier keine belastbare Quelle vorliegt - eine
// geratene Namensliste waere genau die Art Annahme, die vermieden werden
// soll.
func TestExtractEInvoiceFromPDFAcceptsOtherFilenames(t *testing.T) {
	for _, name := range []string{"ZUGFeRD-invoice.xml", "xrechnung.xml", "beliebig.dat"} {
		t.Run(name, func(t *testing.T) {
			pdf := buildTestPDF(t, ciiAttachment(name))

			got, err := ExtractEInvoiceFromPDF(pdf)
			if err != nil {
				t.Fatalf("Anhang %q muss trotzdem gefunden werden: %v", name, err)
			}
			if got.Number != "LIEF-2026-0815" {
				t.Errorf("Rechnungsnummer = %q", got.Number)
			}
		})
	}
}

// TestExtractEInvoiceFromPDFPrefersKnownName: liegen mehrere Anhaenge vor,
// gewinnt der mit dem bekannten ZUGFeRD-Namen - auch wenn ein anderer
// Anhang ebenfalls eine gueltige Rechnung waere.
func TestExtractEInvoiceFromPDFPrefersKnownName(t *testing.T) {
	andereRechnung := strings.Replace(ciiInboundSample, "LIEF-2026-0815", "FALSCHE-RECHNUNG", 1)

	pdf := buildTestPDF(t,
		pdfgen.Attachment{Filename: "anhang-a.xml", Content: []byte(andereRechnung)},
		ciiAttachment(ZugferdAttachmentName),
	)

	got, err := ExtractEInvoiceFromPDF(pdf)
	if err != nil {
		t.Fatalf("ExtractEInvoiceFromPDF: %v", err)
	}
	if got.Number != "LIEF-2026-0815" {
		t.Errorf("erwartet den Anhang %q, gelesen wurde %q", ZugferdAttachmentName, got.Number)
	}
}

// TestExtractEInvoiceFromPDFSkipsUnreadableAttachments: ein Anhang, der
// keine Rechnung ist, darf den Fund nicht verhindern.
func TestExtractEInvoiceFromPDFSkipsUnreadableAttachments(t *testing.T) {
	pdf := buildTestPDF(t,
		pdfgen.Attachment{Filename: "lieferschein.txt", Content: []byte("Das ist keine XML.")},
		pdfgen.Attachment{Filename: "rechnung.xml", Content: []byte(ciiInboundSample)},
	)

	got, err := ExtractEInvoiceFromPDF(pdf)
	if err != nil {
		t.Fatalf("ein unlesbarer Anhang darf den Fund nicht verhindern: %v", err)
	}
	if got.Number != "LIEF-2026-0815" {
		t.Errorf("Rechnungsnummer = %q", got.Number)
	}
}

func TestExtractEInvoiceFromPDFRejectsBrokenInput(t *testing.T) {
	cases := []struct {
		name    string
		input   []byte
		wantSub string
	}{
		{"leere Datei", nil, "leere PDF-Datei"},
		{"keine PDF", []byte("Das ist einfach nur Text."), "PDF konnte nicht gelesen werden"},
		// Dieser Fall ist zugleich der WAECHTER ueber den Textabgleich in
		// isPdfcpuNoAttachmentsError (Backlog E.7): er laeuft gegen die
		// echte Bibliothek. Aendert pdfcpu bei einer Versionsanhebung den
		// Meldungstext, schlaegt genau hier fehl - der Workaround kann
		// also nicht unbemerkt brechen. Nicht entfernen.
		{"PDF ohne Anhang", buildTestPDF(t), "keine eingebettete Rechnungsdatei"},
		{
			"PDF mit fremdartigem Anhang",
			buildTestPDF(t, pdfgen.Attachment{Filename: "notiz.txt", Content: []byte("nur eine Notiz")}),
			"ist eine lesbare E-Rechnung",
		},
		{
			"PDF mit XML, aber keiner Rechnung",
			buildTestPDF(t, pdfgen.Attachment{
				Filename: "daten.xml",
				Content:  []byte(`<?xml version="1.0"?><Lieferschein><Nr>1</Nr></Lieferschein>`),
			}),
			"ist eine lesbare E-Rechnung",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractEInvoiceFromPDF(c.input)
			if err == nil {
				t.Fatalf("erwartet Fehler, got %+v", got)
			}
			if got != nil {
				t.Error("im Fehlerfall darf kein Teilergebnis zurückgegeben werden")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("Fehlermeldung %q nennt %q nicht", err.Error(), c.wantSub)
			}
		})
	}
}

// TestExtractEInvoiceFromPDFErrorNamesAttachments: scheitert die Suche,
// muss die Meldung benennen, was geprueft wurde - sonst steht der
// Anwender vor einem "geht nicht" ohne Anhaltspunkt.
func TestExtractEInvoiceFromPDFErrorNamesAttachments(t *testing.T) {
	pdf := buildTestPDF(t,
		pdfgen.Attachment{Filename: "notiz.txt", Content: []byte("nur eine Notiz")},
		pdfgen.Attachment{Filename: "bild.dat", Content: []byte{0x00, 0x01, 0x02}},
	)

	_, err := ExtractEInvoiceFromPDF(pdf)
	if err == nil {
		t.Fatal("erwartet Fehler")
	}
	msg := err.Error()
	if !strings.Contains(msg, "notiz.txt") || !strings.Contains(msg, "bild.dat") {
		t.Errorf("Fehlermeldung nennt die geprüften Anhänge nicht: %v", err)
	}
}

// TestExtractEInvoiceFromPDFDoesNotResolveExternalEntities: der XXE-Schutz
// muss auch auf dem PDF-Weg greifen - der Anhang stammt genauso von einem
// Dritten wie eine hochgeladene XML.
func TestExtractEInvoiceFromPDFDoesNotResolveExternalEntities(t *testing.T) {
	xxe := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [ <!ENTITY xxe SYSTEM "file:///etc/passwd"> ]>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
    <rsm:ExchangedDocument>
        <ram:ID>&xxe;</ram:ID>
        <ram:IssueDateTime>
            <udt:DateTimeString format="102">20260518</udt:DateTimeString>
        </ram:IssueDateTime>
    </rsm:ExchangedDocument>
</rsm:CrossIndustryInvoice>`

	pdf := buildTestPDF(t, pdfgen.Attachment{Filename: ZugferdAttachmentName, Content: []byte(xxe)})

	got, err := ExtractEInvoiceFromPDF(pdf)
	if err != nil {
		return
	}
	if strings.Contains(got.Number, "root:") || strings.Contains(got.Number, "/bin/") {
		t.Fatalf("externe Entität wurde aufgelöst - Dateiinhalt im Ergebnis: %q", got.Number)
	}
}
