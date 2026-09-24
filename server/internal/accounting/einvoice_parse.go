package accounting

// E-Rechnung Eingang, gemeinsames Zielmodell und CII-Parser (ADR 0023,
// Backlog E.5.2).
//
// Bewusst OHNE Datenbankzugriff: die Zuordnung zu einem Lieferanten und
// die Anzeige sind Sache von E.5.5, die Uebernahme nach invoices_in ist
// laut ADR 0023 ausdruecklich NICHT Teil von Task E.5 (Backlog E.6).
//
// WICHTIG - der Unterschied zum Ausgang (einvoice_cii.go): dort werden
// Namensraum-Praefixe fest in die Elementnamen geschrieben, weil
// encoding/xml beim SCHREIBEN keine Praefixe ausgeben kann. Beim LESEN
// funktioniert das nicht: ein Absender darf beliebige Praefixe waehlen
// ("rsm"/"ram" sind Konvention, nicht Vorschrift). Die Structs hier
// matchen deshalb ueber vollqualifizierte Namensraeume
// (xml:"<namespace-uri> <local-name>") und sind mit den Schreib-Structs
// NICHT austauschbar.

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Namensraeume der beiden EN-16931-Syntaxen. Die UBL-Konstanten werden
// bereits hier definiert, weil die Formaterkennung (E.5.3) sie braucht.
const (
	// Nur die Wurzel-Namensraeume stehen als Konstanten: sie werden fuer
	// die Formaterkennung im Code gebraucht. Die uebrigen Namensraeume
	// (ram/udt bzw. cbc/cac) erscheinen ausschliesslich in Struct-Tags,
	// und Struct-Tags koennen keine Konstanten referenzieren - eine
	// Konstante dafuer waere toter Code.
	nsCIIRSM = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"

	nsUBLInvoice = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
)

// Formatkennungen im Zielmodell.
const (
	EInvoiceFormatCII = "cii"
	EInvoiceFormatUBL = "ubl"
)

// ParsedParty ist eine Geschaeftspartei einer eingegangenen Rechnung.
type ParsedParty struct {
	Name       string `json:"name"`
	Street     string `json:"strasse"`
	PostalCode string `json:"plz"`
	City       string `json:"ort"`
	Country    string `json:"land"`
	VatID      string `json:"ust_id"`
	TaxNo      string `json:"steuernummer"`
	Email      string `json:"email"`
}

// ParsedLine ist eine Position einer eingegangenen Rechnung.
type ParsedLine struct {
	LineID          string  `json:"position"`
	Name            string  `json:"bezeichnung"`
	Qty             float64 `json:"menge"`
	UnitCode        string  `json:"einheit"`
	NetUnitPrice    float64 `json:"einzelpreis"`
	LineTotalAmount float64 `json:"positionssumme"`
	TaxCategoryCode string  `json:"steuerkategorie"`
	TaxRatePercent  float64 `json:"steuersatz"`
}

// ParsedTax ist eine Zeile der Steueraufschluesselung.
type ParsedTax struct {
	CategoryCode     string  `json:"steuerkategorie"`
	RatePercent      float64 `json:"steuersatz"`
	BasisAmount      float64 `json:"nettobetrag"`
	CalculatedAmount float64 `json:"steuerbetrag"`
	ExemptionReason  string  `json:"befreiungsgrund"`
}

// ParsedEInvoice ist das gemeinsame Zielmodell BEIDER Syntaxen (ADR 0023):
// der CII-Parser hier und der UBL-Parser aus E.5.3 liefern dasselbe
// Ergebnis, damit alles Nachgelagerte syntaxunabhaengig bleibt.
//
// Hinweise sammelt Auffaelligkeiten, die KEINE Parse-Fehler sind - etwa
// eine Summe, die nicht zu den Positionen passt. Laut ADR 0023 wird an
// gelieferten Werten nichts nachgerechnet und nichts korrigiert: es ist
// die Rechnung des Absenders, wir melden nur, was auffaellt.
type ParsedEInvoice struct {
	Format         string     `json:"format"`
	Number         string     `json:"rechnungsnummer"`
	TypeCode       string     `json:"rechnungstyp"`
	IssueDate      time.Time  `json:"rechnungsdatum"`
	DueDate        *time.Time `json:"faelligkeitsdatum,omitempty"`
	Currency       string     `json:"waehrung"`
	BuyerReference string     `json:"kaeuferreferenz"`

	Seller ParsedParty `json:"verkaeufer"`
	Buyer  ParsedParty `json:"kaeufer"`

	Lines        []ParsedLine `json:"positionen"`
	TaxBreakdown []ParsedTax  `json:"steueraufschluesselung"`

	LineTotalAmount     float64 `json:"summe_positionen"`
	TaxBasisTotalAmount float64 `json:"steuerbasis"`
	TaxTotalAmount      float64 `json:"steuerbetrag"`
	GrandTotalAmount    float64 `json:"bruttobetrag"`
	PrepaidAmount       float64 `json:"bereits_gezahlt"`
	DuePayableAmount    float64 `json:"zahlbetrag"`

	Hinweise []string `json:"hinweise,omitempty"`
}

// --- CII-Lesestrukturen ---------------------------------------------------

type ciiInDateTimeString struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type ciiInDateTime struct {
	DateTimeString ciiInDateTimeString `xml:"urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100 DateTimeString"`
}

type ciiInIDWithScheme struct {
	SchemeID string `xml:"schemeID,attr"`
	Value    string `xml:",chardata"`
}

type ciiInAddress struct {
	PostcodeCode string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 PostcodeCode"`
	LineOne      string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 LineOne"`
	CityName     string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 CityName"`
	CountryID    string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 CountryID"`
}

type ciiInURIComm struct {
	URIID ciiInIDWithScheme `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 URIID"`
}

type ciiInTaxRegistration struct {
	ID ciiInIDWithScheme `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ID"`
}

type ciiInTradeParty struct {
	Name             string                 `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 Name"`
	PostalAddress    ciiInAddress           `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 PostalTradeAddress"`
	URIComm          ciiInURIComm           `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 URIUniversalCommunication"`
	TaxRegistrations []ciiInTaxRegistration `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedTaxRegistration"`
}

type ciiInHeaderTradeAgreement struct {
	BuyerReference string          `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 BuyerReference"`
	SellerParty    ciiInTradeParty `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SellerTradeParty"`
	BuyerParty     ciiInTradeParty `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 BuyerTradeParty"`
}

type ciiInTradeTax struct {
	CalculatedAmount      string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 CalculatedAmount"`
	TypeCode              string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 TypeCode"`
	ExemptionReason       string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ExemptionReason"`
	BasisAmount           string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 BasisAmount"`
	CategoryCode          string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 CategoryCode"`
	RateApplicablePercent string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 RateApplicablePercent"`
}

type ciiInPaymentTerms struct {
	DueDate *ciiInDateTime `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 DueDateDateTime"`
}

type ciiInSummation struct {
	LineTotalAmount     string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 LineTotalAmount"`
	TaxBasisTotalAmount string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 TaxBasisTotalAmount"`
	TaxTotalAmount      string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 TaxTotalAmount"`
	GrandTotalAmount    string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 GrandTotalAmount"`
	TotalPrepaidAmount  string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 TotalPrepaidAmount"`
	DuePayableAmount    string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 DuePayableAmount"`
}

type ciiInHeaderTradeSettlement struct {
	InvoiceCurrencyCode string             `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 InvoiceCurrencyCode"`
	TradeTax            []ciiInTradeTax    `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ApplicableTradeTax"`
	PaymentTerms        *ciiInPaymentTerms `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedTradePaymentTerms"`
	Summation           ciiInSummation     `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type ciiInQuantity struct {
	UnitCode string `xml:"unitCode,attr"`
	Value    string `xml:",chardata"`
}

type ciiInLineItem struct {
	LineDocument struct {
		LineID string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 LineID"`
	} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 AssociatedDocumentLineDocument"`
	Product struct {
		Name string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 Name"`
	} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedTradeProduct"`
	Agreement struct {
		NetPrice struct {
			ChargeAmount string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ChargeAmount"`
		} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 NetPriceProductTradePrice"`
	} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedLineTradeAgreement"`
	Delivery struct {
		BilledQuantity ciiInQuantity `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 BilledQuantity"`
	} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedLineTradeDelivery"`
	Settlement struct {
		TradeTax struct {
			CategoryCode          string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 CategoryCode"`
			RateApplicablePercent string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 RateApplicablePercent"`
		} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ApplicableTradeTax"`
		Summation struct {
			LineTotalAmount string `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 LineTotalAmount"`
		} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedTradeSettlementLineMonetarySummation"`
	} `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 SpecifiedLineTradeSettlement"`
}

type ciiInDocument struct {
	XMLName  xml.Name
	Document struct {
		ID            string        `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ID"`
		TypeCode      string        `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 TypeCode"`
		IssueDateTime ciiInDateTime `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 IssueDateTime"`
	} `xml:"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100 ExchangedDocument"`
	Transaction struct {
		LineItems  []ciiInLineItem            `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 IncludedSupplyChainTradeLineItem"`
		Agreement  ciiInHeaderTradeAgreement  `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ApplicableHeaderTradeAgreement"`
		Settlement ciiInHeaderTradeSettlement `xml:"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100 ApplicableHeaderTradeSettlement"`
	} `xml:"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100 SupplyChainTradeTransaction"`
}

// --- Parser ---------------------------------------------------------------

// ParseCIIInvoice liest eine eingegangene Rechnung in CII-Syntax
// (UN/CEFACT Cross Industry Invoice, auch der eingebettete Teil einer
// ZUGFeRD-PDF).
//
// Grundhaltung: streng genug, um Unsinn zu erkennen, nachsichtig genug,
// um an einem fehlenden Kann-Feld nicht zu scheitern. Es ist die Rechnung
// eines Dritten - wir duerfen sie nicht zurueckweisen, nur weil sie nicht
// so aussieht, wie wir selbst schreiben wuerden.
func ParseCIIInvoice(data []byte) (*ParsedEInvoice, error) {
	var doc ciiInDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("E-Rechnung Eingang: XML konnte nicht gelesen werden: %w", err)
	}
	if doc.XMLName.Space != nsCIIRSM || doc.XMLName.Local != "CrossIndustryInvoice" {
		return nil, fmt.Errorf("E-Rechnung Eingang: kein CII-Dokument (Wurzelelement %q im Namensraum %q)", doc.XMLName.Local, doc.XMLName.Space)
	}

	out := &ParsedEInvoice{
		Format:         EInvoiceFormatCII,
		Number:         strings.TrimSpace(doc.Document.ID),
		TypeCode:       strings.TrimSpace(doc.Document.TypeCode),
		Currency:       strings.TrimSpace(doc.Transaction.Settlement.InvoiceCurrencyCode),
		BuyerReference: strings.TrimSpace(doc.Transaction.Agreement.BuyerReference),
	}

	if out.Number == "" {
		return nil, fmt.Errorf("E-Rechnung Eingang: Rechnungsnummer fehlt")
	}

	issue, err := parseCIIDate(doc.Document.IssueDateTime)
	if err != nil {
		return nil, fmt.Errorf("E-Rechnung Eingang: Rechnungsdatum: %w", err)
	}
	out.IssueDate = issue

	if pt := doc.Transaction.Settlement.PaymentTerms; pt != nil && pt.DueDate != nil {
		due, err := parseCIIDate(*pt.DueDate)
		if err != nil {
			return nil, fmt.Errorf("E-Rechnung Eingang: Fälligkeitsdatum: %w", err)
		}
		out.DueDate = &due
	}

	out.Seller = parseCIIParty(doc.Transaction.Agreement.SellerParty)
	out.Buyer = parseCIIParty(doc.Transaction.Agreement.BuyerParty)

	for _, l := range doc.Transaction.LineItems {
		out.Lines = append(out.Lines, ParsedLine{
			LineID:          strings.TrimSpace(l.LineDocument.LineID),
			Name:            strings.TrimSpace(l.Product.Name),
			Qty:             parseEInvoiceAmount(l.Delivery.BilledQuantity.Value),
			UnitCode:        strings.TrimSpace(l.Delivery.BilledQuantity.UnitCode),
			NetUnitPrice:    parseEInvoiceAmount(l.Agreement.NetPrice.ChargeAmount),
			LineTotalAmount: parseEInvoiceAmount(l.Settlement.Summation.LineTotalAmount),
			TaxCategoryCode: strings.TrimSpace(l.Settlement.TradeTax.CategoryCode),
			TaxRatePercent:  parseEInvoiceAmount(l.Settlement.TradeTax.RateApplicablePercent),
		})
	}

	for _, t := range doc.Transaction.Settlement.TradeTax {
		out.TaxBreakdown = append(out.TaxBreakdown, ParsedTax{
			CategoryCode:     strings.TrimSpace(t.CategoryCode),
			RatePercent:      parseEInvoiceAmount(t.RateApplicablePercent),
			BasisAmount:      parseEInvoiceAmount(t.BasisAmount),
			CalculatedAmount: parseEInvoiceAmount(t.CalculatedAmount),
			ExemptionReason:  strings.TrimSpace(t.ExemptionReason),
		})
	}

	sum := doc.Transaction.Settlement.Summation
	out.LineTotalAmount = parseEInvoiceAmount(sum.LineTotalAmount)
	out.TaxBasisTotalAmount = parseEInvoiceAmount(sum.TaxBasisTotalAmount)
	out.TaxTotalAmount = parseEInvoiceAmount(sum.TaxTotalAmount)
	out.GrandTotalAmount = parseEInvoiceAmount(sum.GrandTotalAmount)
	out.PrepaidAmount = parseEInvoiceAmount(sum.TotalPrepaidAmount)
	out.DuePayableAmount = parseEInvoiceAmount(sum.DuePayableAmount)

	out.Hinweise = eInvoicePlausibilityHinweise(out)

	return out, nil
}

func parseCIIParty(p ciiInTradeParty) ParsedParty {
	out := ParsedParty{
		Name:       strings.TrimSpace(p.Name),
		Street:     strings.TrimSpace(p.PostalAddress.LineOne),
		PostalCode: strings.TrimSpace(p.PostalAddress.PostcodeCode),
		City:       strings.TrimSpace(p.PostalAddress.CityName),
		Country:    strings.TrimSpace(p.PostalAddress.CountryID),
		Email:      strings.TrimSpace(p.URIComm.URIID.Value),
	}
	// schemeID "VA" = USt-IdNr., "FC" = Steuernummer. Ein Absender darf
	// beide, eine oder keine angeben.
	for _, reg := range p.TaxRegistrations {
		switch strings.ToUpper(strings.TrimSpace(reg.ID.SchemeID)) {
		case "VA":
			out.VatID = strings.TrimSpace(reg.ID.Value)
		case "FC":
			out.TaxNo = strings.TrimSpace(reg.ID.Value)
		}
	}
	return out
}

// parseCIIDate liest ein udt:DateTimeString. Ein anderes Format als 102
// (CCYYMMDD) wird mit Fehler abgelehnt statt geraten - eine falsch
// interpretierte Datumsangabe auf einer Rechnung waere schlimmer als eine
// abgelehnte Datei.
func parseCIIDate(dt ciiInDateTime) (time.Time, error) {
	raw := strings.TrimSpace(dt.DateTimeString.Value)
	if raw == "" {
		return time.Time{}, fmt.Errorf("fehlt")
	}
	format := strings.TrimSpace(dt.DateTimeString.Format)
	if format != "" && format != ciiDateFormat102 {
		return time.Time{}, fmt.Errorf("nicht unterstütztes Datumsformat %q (nur 102 = CCYYMMDD)", format)
	}
	t, err := time.Parse("20060102", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("unlesbar: %q", raw)
	}
	return t, nil
}

// parseEInvoiceAmount liest einen Betrag. Ein leeres oder unlesbares Feld
// ergibt 0 - Betragsfelder sind in EN 16931 grossteils optional, und ein
// fehlender Vorauszahlungsbetrag darf die ganze Rechnung nicht
// unbrauchbar machen. Auffaellige Summen werden stattdessen ueber
// Hinweise gemeldet.
func parseEInvoiceAmount(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// eInvoicePlausibilityHinweise meldet Auffaelligkeiten, ohne etwas zu
// korrigieren (ADR 0023): es ist die Rechnung des Absenders. Toleranz
// bewusst 1 Cent, damit uebliche Rundungen keine Hinweisflut erzeugen.
func eInvoicePlausibilityHinweise(inv *ParsedEInvoice) []string {
	var hinweise []string

	if len(inv.Lines) == 0 {
		hinweise = append(hinweise, "Die Rechnung enthält keine Positionen.")
	} else {
		var sum float64
		for _, l := range inv.Lines {
			sum += l.LineTotalAmount
		}
		if inv.LineTotalAmount != 0 && math.Abs(sum-inv.LineTotalAmount) > 0.01 {
			hinweise = append(hinweise, fmt.Sprintf(
				"Summe der Positionen (%.2f) weicht von der ausgewiesenen Positionssumme (%.2f) ab.",
				sum, inv.LineTotalAmount))
		}
	}

	if inv.GrandTotalAmount != 0 {
		erwartet := inv.TaxBasisTotalAmount + inv.TaxTotalAmount
		if math.Abs(erwartet-inv.GrandTotalAmount) > 0.01 {
			hinweise = append(hinweise, fmt.Sprintf(
				"Netto (%.2f) plus Steuer (%.2f) ergibt %.2f, ausgewiesen ist ein Bruttobetrag von %.2f.",
				inv.TaxBasisTotalAmount, inv.TaxTotalAmount, erwartet, inv.GrandTotalAmount))
		}
	}

	if strings.TrimSpace(inv.Seller.Name) == "" {
		hinweise = append(hinweise, "Die Rechnung nennt keinen Verkäufernamen.")
	}
	if strings.TrimSpace(inv.Seller.VatID) == "" && strings.TrimSpace(inv.Seller.TaxNo) == "" {
		hinweise = append(hinweise, "Der Verkäufer ist weder mit USt-IdNr. noch mit Steuernummer angegeben - eine eindeutige Lieferantenzuordnung ist dadurch erschwert.")
	}
	if strings.TrimSpace(inv.Currency) == "" {
		hinweise = append(hinweise, "Die Rechnung nennt keine Währung.")
	}

	return hinweise
}
