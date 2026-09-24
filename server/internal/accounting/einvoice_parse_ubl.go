package accounting

// E-Rechnung Eingang, UBL-Parser und Formaterkennung (ADR 0023,
// Backlog E.5.3).
//
// UBL 2.1 ist neben CII die zweite gleichwertig zugelassene Ausdrucksform
// desselben EN-16931-Datenmodells. Beim Ausgang durften wir uns auf CII
// festlegen (ADR 0022), weil wir dort die Syntax bestimmen - beim Eingang
// bestimmt sie der Absender, deshalb muessen beide gelesen werden.
//
// Struktur gegen die Primaerquelle geprueft (KoSIT-Testsuite,
// Geschaeftsfaelle 01.01a/01.04a/01.21a/02.01a/04.01a in UBL-Syntax):
// UBL ist strukturell voellig anders als CII - flache cbc:-Felder auf
// Dokumentebene, cac:-Gruppen fuer alles Zusammengesetzte, Datumsangaben
// als YYYY-MM-DD statt im CII-Format 102.
//
// Wie beim CII-Parser wird ueber vollqualifizierte Namensraeume gematcht,
// nicht ueber Praefixe.

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// UBL-Datumsfelder sind nach XML-Schema xs:date formatiert.
const ublDateLayout = "2006-01-02"

// --- UBL-Lesestrukturen ---------------------------------------------------

type ublInIDWithScheme struct {
	SchemeID string `xml:"schemeID,attr"`
	Value    string `xml:",chardata"`
}

type ublInCountry struct {
	IdentificationCode string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 IdentificationCode"`
}

type ublInPostalAddress struct {
	StreetName string       `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 StreetName"`
	CityName   string       `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 CityName"`
	PostalZone string       `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 PostalZone"`
	Country    ublInCountry `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 Country"`
}

type ublInTaxScheme struct {
	ID string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ID"`
}

type ublInPartyTaxScheme struct {
	CompanyID string         `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 CompanyID"`
	TaxScheme ublInTaxScheme `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 TaxScheme"`
}

type ublInParty struct {
	EndpointID ublInIDWithScheme `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 EndpointID"`
	PartyName  struct {
		Name string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 Name"`
	} `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 PartyName"`
	PostalAddress   ublInPostalAddress    `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 PostalAddress"`
	PartyTaxSchemes []ublInPartyTaxScheme `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 PartyTaxScheme"`
	LegalEntity     struct {
		RegistrationName string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 RegistrationName"`
	} `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 PartyLegalEntity"`
	Contact struct {
		ElectronicMail string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ElectronicMail"`
	} `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 Contact"`
}

type ublInSupplierParty struct {
	Party ublInParty `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 Party"`
}

type ublInTaxCategory struct {
	ID                 string         `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ID"`
	Percent            string         `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 Percent"`
	TaxExemptionReason string         `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 TaxExemptionReason"`
	TaxScheme          ublInTaxScheme `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 TaxScheme"`
}

type ublInTaxSubtotal struct {
	TaxableAmount string           `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 TaxableAmount"`
	TaxAmount     string           `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 TaxAmount"`
	TaxCategory   ublInTaxCategory `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 TaxCategory"`
}

type ublInTaxTotal struct {
	TaxAmount    string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 TaxAmount"`
	TaxSubtotals []ublInTaxSubtotal `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 TaxSubtotal"`
}

type ublInMonetaryTotal struct {
	LineExtensionAmount string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 LineExtensionAmount"`
	TaxExclusiveAmount  string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 TaxExclusiveAmount"`
	TaxInclusiveAmount  string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 TaxInclusiveAmount"`
	PrepaidAmount       string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 PrepaidAmount"`
	PayableAmount       string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 PayableAmount"`
}

type ublInQuantity struct {
	UnitCode string `xml:"unitCode,attr"`
	Value    string `xml:",chardata"`
}

type ublInInvoiceLine struct {
	ID                  string        `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ID"`
	InvoicedQuantity    ublInQuantity `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 InvoicedQuantity"`
	LineExtensionAmount string        `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 LineExtensionAmount"`
	Item                struct {
		Name               string           `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 Name"`
		ClassifiedTaxCateg ublInTaxCategory `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 ClassifiedTaxCategory"`
	} `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 Item"`
	Price struct {
		PriceAmount string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 PriceAmount"`
	} `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 Price"`
}

type ublInDocument struct {
	XMLName              xml.Name
	ID                   string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ID"`
	IssueDate            string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 IssueDate"`
	DueDate              string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 DueDate"`
	InvoiceTypeCode      string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 InvoiceTypeCode"`
	DocumentCurrencyCode string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 DocumentCurrencyCode"`
	BuyerReference       string             `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 BuyerReference"`
	SupplierParty        ublInSupplierParty `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 AccountingSupplierParty"`
	CustomerParty        ublInSupplierParty `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 AccountingCustomerParty"`
	TaxTotals            []ublInTaxTotal    `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 TaxTotal"`
	MonetaryTotal        ublInMonetaryTotal `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 LegalMonetaryTotal"`
	InvoiceLines         []ublInInvoiceLine `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2 InvoiceLine"`
}

// --- Parser ---------------------------------------------------------------

// ParseUBLInvoice liest eine eingegangene Rechnung in UBL-2.1-Syntax.
// Liefert dasselbe Zielmodell wie ParseCIIInvoice (ADR 0023), damit alles
// Nachgelagerte syntaxunabhaengig bleibt.
func ParseUBLInvoice(data []byte) (*ParsedEInvoice, error) {
	var doc ublInDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("E-Rechnung Eingang: XML konnte nicht gelesen werden: %w", err)
	}
	if doc.XMLName.Space != nsUBLInvoice || doc.XMLName.Local != "Invoice" {
		return nil, fmt.Errorf("E-Rechnung Eingang: kein UBL-Dokument (Wurzelelement %q im Namensraum %q)", doc.XMLName.Local, doc.XMLName.Space)
	}

	out := &ParsedEInvoice{
		Format:         EInvoiceFormatUBL,
		Number:         strings.TrimSpace(doc.ID),
		TypeCode:       strings.TrimSpace(doc.InvoiceTypeCode),
		Currency:       strings.TrimSpace(doc.DocumentCurrencyCode),
		BuyerReference: strings.TrimSpace(doc.BuyerReference),
	}

	if out.Number == "" {
		return nil, fmt.Errorf("E-Rechnung Eingang: Rechnungsnummer fehlt")
	}

	issue, err := parseUBLDate(doc.IssueDate)
	if err != nil {
		return nil, fmt.Errorf("E-Rechnung Eingang: Rechnungsdatum: %w", err)
	}
	out.IssueDate = issue

	if strings.TrimSpace(doc.DueDate) != "" {
		due, err := parseUBLDate(doc.DueDate)
		if err != nil {
			return nil, fmt.Errorf("E-Rechnung Eingang: Fälligkeitsdatum: %w", err)
		}
		out.DueDate = &due
	}

	out.Seller = parseUBLParty(doc.SupplierParty.Party)
	out.Buyer = parseUBLParty(doc.CustomerParty.Party)

	for _, l := range doc.InvoiceLines {
		out.Lines = append(out.Lines, ParsedLine{
			LineID:          strings.TrimSpace(l.ID),
			Name:            strings.TrimSpace(l.Item.Name),
			Qty:             parseEInvoiceAmount(l.InvoicedQuantity.Value),
			UnitCode:        strings.TrimSpace(l.InvoicedQuantity.UnitCode),
			NetUnitPrice:    parseEInvoiceAmount(l.Price.PriceAmount),
			LineTotalAmount: parseEInvoiceAmount(l.LineExtensionAmount),
			TaxCategoryCode: strings.TrimSpace(l.Item.ClassifiedTaxCateg.ID),
			TaxRatePercent:  parseEInvoiceAmount(l.Item.ClassifiedTaxCateg.Percent),
		})
	}

	// Steueraufschluesselung: in UBL steckt der Gesamtsteuerbetrag im
	// cbc:TaxAmount des cac:TaxTotal, die Aufschluesselung in dessen
	// cac:TaxSubtotal-Elementen.
	for _, tt := range doc.TaxTotals {
		out.TaxTotalAmount += parseEInvoiceAmount(tt.TaxAmount)
		for _, sub := range tt.TaxSubtotals {
			out.TaxBreakdown = append(out.TaxBreakdown, ParsedTax{
				CategoryCode:     strings.TrimSpace(sub.TaxCategory.ID),
				RatePercent:      parseEInvoiceAmount(sub.TaxCategory.Percent),
				BasisAmount:      parseEInvoiceAmount(sub.TaxableAmount),
				CalculatedAmount: parseEInvoiceAmount(sub.TaxAmount),
				ExemptionReason:  strings.TrimSpace(sub.TaxCategory.TaxExemptionReason),
			})
		}
	}

	out.LineTotalAmount = parseEInvoiceAmount(doc.MonetaryTotal.LineExtensionAmount)
	out.TaxBasisTotalAmount = parseEInvoiceAmount(doc.MonetaryTotal.TaxExclusiveAmount)
	out.GrandTotalAmount = parseEInvoiceAmount(doc.MonetaryTotal.TaxInclusiveAmount)
	out.PrepaidAmount = parseEInvoiceAmount(doc.MonetaryTotal.PrepaidAmount)
	out.DuePayableAmount = parseEInvoiceAmount(doc.MonetaryTotal.PayableAmount)

	// Bewusst dieselbe, syntaxunabhaengige Pruefung wie beim CII-Parser -
	// nicht dupliziert, sondern wiederverwendet.
	out.Hinweise = eInvoicePlausibilityHinweise(out)

	return out, nil
}

// parseUBLParty bildet eine UBL-Partei ab. Der eingetragene Name
// (PartyLegalEntity/RegistrationName, BT-27) hat Vorrang vor dem
// Handelsnamen (PartyName/Name, BT-28) - fuer die Lieferantenzuordnung in
// E.5.5 ist der eingetragene Name der belastbarere Wert.
func parseUBLParty(p ublInParty) ParsedParty {
	name := strings.TrimSpace(p.LegalEntity.RegistrationName)
	if name == "" {
		name = strings.TrimSpace(p.PartyName.Name)
	}

	email := strings.TrimSpace(p.Contact.ElectronicMail)
	if email == "" && strings.EqualFold(strings.TrimSpace(p.EndpointID.SchemeID), "EM") {
		email = strings.TrimSpace(p.EndpointID.Value)
	}

	out := ParsedParty{
		Name:       name,
		Street:     strings.TrimSpace(p.PostalAddress.StreetName),
		PostalCode: strings.TrimSpace(p.PostalAddress.PostalZone),
		City:       strings.TrimSpace(p.PostalAddress.CityName),
		Country:    strings.TrimSpace(p.PostalAddress.Country.IdentificationCode),
		Email:      email,
	}

	// In UBL unterscheidet das TaxScheme, wofuer die CompanyID steht:
	// "VAT" = USt-IdNr., "FC" = Steuernummer. Andere/unbekannte Kennungen
	// werden bewusst ignoriert statt geraten - die KoSIT-Beispiele
	// enthalten dort teils Platzhalter.
	for _, ts := range p.PartyTaxSchemes {
		id := strings.TrimSpace(ts.CompanyID)
		if id == "" {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(ts.TaxScheme.ID)) {
		case "VAT":
			out.VatID = id
		case "FC":
			out.TaxNo = id
		}
	}

	return out
}

// parseUBLDate liest ein UBL-Datum (xs:date, YYYY-MM-DD). Anders als in
// CII gibt es hier kein Formatattribut - ein abweichendes Format ist
// schlicht ein unlesbares Datum.
func parseUBLDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("fehlt")
	}
	t, err := time.Parse(ublDateLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("unlesbar: %q", raw)
	}
	return t, nil
}

// --- Formaterkennung ------------------------------------------------------

// ParseEInvoiceXML erkennt die Syntax anhand des Wurzelelement-
// Namensraums und gibt an den passenden Parser ab (ADR 0023).
//
// Bewusst NICHT anhand von Dateiname oder Dateiendung: beim Eingang ist
// beides nicht vertrauenswuerdig - der Absender bestimmt den Dateinamen,
// und eine .xml sagt nichts ueber die Syntax aus.
func ParseEInvoiceXML(data []byte) (*ParsedEInvoice, error) {
	root, err := eInvoiceRootElement(data)
	if err != nil {
		return nil, err
	}

	switch {
	case root.Space == nsCIIRSM && root.Local == "CrossIndustryInvoice":
		return ParseCIIInvoice(data)
	case root.Space == nsUBLInvoice && root.Local == "Invoice":
		return ParseUBLInvoice(data)
	default:
		return nil, fmt.Errorf(
			"E-Rechnung Eingang: unbekanntes Format - Wurzelelement %q im Namensraum %q ist weder CII (UN/CEFACT Cross Industry Invoice) noch UBL 2.1",
			root.Local, root.Space)
	}
}

// eInvoiceRootElement liest nur das Wurzelelement, ohne das gesamte
// Dokument in eine Struktur zu zwingen.
func eInvoiceRootElement(data []byte) (xml.Name, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return xml.Name{}, fmt.Errorf("E-Rechnung Eingang: XML konnte nicht gelesen werden: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name, nil
		}
	}
}
