package apihttp

// E-Rechnung Eingang, HTTP-Anbindung (ADR 0023, Backlog E.5.5).
//
// Der Endpunkt PARST und ZEIGT - er persistiert nichts. Laut ADR 0023 ist
// die Uebernahme nach invoices_in ausdruecklich nicht Teil von Task E.5
// (Backlog E.6), weil invoices_in.supplier_id ein Pflicht-Fremdschluessel
// auf contacts ist und eine automatische Anlage Stammdaten aus einer von
// aussen zugestellten Datei erzeugen wuerde.

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"nalaerp3/internal/accounting"
)

// maxEInvoiceUploadBytes begrenzt den Upload. Die Datei stammt von einem
// Dritten; ohne Grenze koennte ein einzelner Request den Speicher fuellen.
// 24 MiB ist fuer eine E-Rechnung reichlich - eine reine XML liegt im
// niedrigen dreistelligen Kilobyte-Bereich, eine ZUGFeRD-PDF mit Logos
// und Bildern selten ueber wenigen Megabyte.
const maxEInvoiceUploadBytes = 24 << 20

// pdfMagic ist die Signatur, an der eine PDF erkannt wird.
var pdfMagic = []byte("%PDF-")

// eInvoiceParseResponse ist die Antwort des Endpunkts: die geparste
// Rechnung plus der Lieferanten-Zuordnungsvorschlag.
type eInvoiceParseResponse struct {
	Quelle           string                         `json:"quelle"`
	Rechnung         *accounting.ParsedEInvoice     `json:"rechnung"`
	Lieferantensuche *accounting.SupplierSuggestion `json:"lieferantensuche"`
}

// parseUploadedEInvoice erkennt anhand der Dateisignatur, ob eine PDF
// oder eine XML vorliegt, und parst entsprechend. Eine Stelle fuer beide
// Aufrufer (Vorschau und Uebernahme), damit die Erkennung nicht zweimal
// existiert und auseinanderlaufen kann.
func parseUploadedEInvoice(data []byte) (*accounting.ParsedEInvoice, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("E-Rechnung Eingang: die hochgeladene Datei ist leer")
	}
	if bytes.HasPrefix(data, pdfMagic) {
		return accounting.ExtractEInvoiceFromPDF(data)
	}
	return accounting.ParseEInvoiceXML(data)
}

// parseInboundEInvoice erkennt anhand des DATEIINHALTS, ob eine PDF oder
// eine XML hochgeladen wurde, parst entsprechend und sucht einen
// passenden Lieferanten.
//
// Die Erkennung laeuft bewusst ueber die Dateisignatur und NICHT ueber
// Dateiname, Endung oder Content-Type: beim Eingang bestimmt der Absender
// alle drei, sie sind also keine verlaessliche Aussage ueber den Inhalt.
// Dieselbe Ueberlegung wie bei der Syntaxerkennung in E.5.3, die den
// Wurzelelement-Namensraum statt des Dateinamens nutzt.
func parseInboundEInvoice(
	ctx context.Context,
	data []byte,
	companyID string,
	apSvc *accounting.APService,
) (*eInvoiceParseResponse, error) {
	parsed, err := parseUploadedEInvoice(data)
	if err != nil {
		return nil, err
	}
	quelle := "xml"
	if bytes.HasPrefix(data, pdfMagic) {
		quelle = "pdf"
	}

	suggestion, err := apSvc.SuggestSupplier(ctx, parsed.Seller, companyID)
	if err != nil {
		return nil, err
	}

	return &eInvoiceParseResponse{
		Quelle:           quelle,
		Rechnung:         parsed,
		Lieferantensuche: suggestion,
	}, nil
}

// readLimitedUpload liest hoechstens limit Bytes und meldet einen klaren
// Fehler, wenn die Datei groesser ist - statt sie stillschweigend
// abzuschneiden und anschliessend an einem unverstaendlichen XML-Fehler
// zu scheitern.
func readLimitedUpload(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("Datei konnte nicht gelesen werden: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Die Datei ist größer als die zulässigen %d MiB", limit>>20)
	}
	return data, nil
}

// takeOverInboundEInvoice parst die hochgeladene Datei erneut und legt
// daraus eine Eingangsrechnung an (Backlog E.6).
//
// Bewusst wird die DATEI verarbeitet und nicht ein vom Client
// geschicktes Datenmodell: sonst koennte der Client Betraege oder die
// Rechnungsnummer abweichend von der tatsaechlichen Rechnung setzen. Der
// Client steuert ausschliesslich die Entscheidungen, die ein Mensch
// treffen muss - Lieferant und Bestellbezug.
func takeOverInboundEInvoice(
	ctx context.Context,
	data []byte,
	decision accounting.EInvoiceTakeoverDecision,
	companyID string,
	apSvc *accounting.APService,
) (*accounting.InvoiceIn, []accounting.InvoiceInItem, error) {
	parsed, err := parseUploadedEInvoice(data)
	if err != nil {
		return nil, nil, err
	}
	return apSvc.TakeOverEInvoice(ctx, parsed, decision, companyID)
}
