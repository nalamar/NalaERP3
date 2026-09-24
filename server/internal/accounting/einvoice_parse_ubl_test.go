package accounting

// Unit-Tests fuer den UBL-Eingangsparser und die Formaterkennung
// (Backlog E.5.3). Vollstaendig DB-los.
//
// ublInboundSample bildet EXAKT denselben Geschaeftsvorfall ab wie
// ciiInboundSample in einvoice_parse_test.go - nur in der anderen Syntax.
// Genau das ermoeglicht den zentralen Test dieser Subtask: beide Wege
// muessen dasselbe Zielmodell ergeben.

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const ublInboundSample = `<?xml version="1.0" encoding="UTF-8"?>
<ubl:Invoice xmlns:ubl="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
             xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
             xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
    <cbc:CustomizationID>urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0</cbc:CustomizationID>
    <cbc:ID>LIEF-2026-0815</cbc:ID>
    <cbc:IssueDate>2026-05-18</cbc:IssueDate>
    <cbc:DueDate>2026-06-17</cbc:DueDate>
    <cbc:InvoiceTypeCode>380</cbc:InvoiceTypeCode>
    <cbc:DocumentCurrencyCode>EUR</cbc:DocumentCurrencyCode>
    <cbc:BuyerReference>BESTELL-4711</cbc:BuyerReference>
    <cac:AccountingSupplierParty>
        <cac:Party>
            <cbc:EndpointID schemeID="EM">rechnung@profilwerk.example</cbc:EndpointID>
            <cac:PartyName>
                <cbc:Name>Profilwerk</cbc:Name>
            </cac:PartyName>
            <cac:PostalAddress>
                <cbc:StreetName>Industriestraße 12</cbc:StreetName>
                <cbc:CityName>Stuttgart</cbc:CityName>
                <cbc:PostalZone>70173</cbc:PostalZone>
                <cac:Country>
                    <cbc:IdentificationCode>DE</cbc:IdentificationCode>
                </cac:Country>
            </cac:PostalAddress>
            <cac:PartyTaxScheme>
                <cbc:CompanyID>DE555666777</cbc:CompanyID>
                <cac:TaxScheme>
                    <cbc:ID>VAT</cbc:ID>
                </cac:TaxScheme>
            </cac:PartyTaxScheme>
            <cac:PartyTaxScheme>
                <cbc:CompanyID>99/123/45678</cbc:CompanyID>
                <cac:TaxScheme>
                    <cbc:ID>FC</cbc:ID>
                </cac:TaxScheme>
            </cac:PartyTaxScheme>
            <cac:PartyLegalEntity>
                <cbc:RegistrationName>Profilwerk Süd GmbH</cbc:RegistrationName>
            </cac:PartyLegalEntity>
            <cac:Contact>
                <cbc:ElectronicMail>rechnung@profilwerk.example</cbc:ElectronicMail>
            </cac:Contact>
        </cac:Party>
    </cac:AccountingSupplierParty>
    <cac:AccountingCustomerParty>
        <cac:Party>
            <cac:PostalAddress>
                <cbc:StreetName>Werkstrasse 4</cbc:StreetName>
                <cbc:CityName>Duesseldorf</cbc:CityName>
                <cbc:PostalZone>40213</cbc:PostalZone>
                <cac:Country>
                    <cbc:IdentificationCode>DE</cbc:IdentificationCode>
                </cac:Country>
            </cac:PostalAddress>
            <cac:PartyLegalEntity>
                <cbc:RegistrationName>Metallbau Muster GmbH</cbc:RegistrationName>
            </cac:PartyLegalEntity>
        </cac:Party>
    </cac:AccountingCustomerParty>
    <cac:TaxTotal>
        <cbc:TaxAmount currencyID="EUR">103.74</cbc:TaxAmount>
        <cac:TaxSubtotal>
            <cbc:TaxableAmount currencyID="EUR">546.00</cbc:TaxableAmount>
            <cbc:TaxAmount currencyID="EUR">103.74</cbc:TaxAmount>
            <cac:TaxCategory>
                <cbc:ID>S</cbc:ID>
                <cbc:Percent>19.00</cbc:Percent>
                <cac:TaxScheme>
                    <cbc:ID>VAT</cbc:ID>
                </cac:TaxScheme>
            </cac:TaxCategory>
        </cac:TaxSubtotal>
    </cac:TaxTotal>
    <cac:LegalMonetaryTotal>
        <cbc:LineExtensionAmount currencyID="EUR">546.00</cbc:LineExtensionAmount>
        <cbc:TaxExclusiveAmount currencyID="EUR">546.00</cbc:TaxExclusiveAmount>
        <cbc:TaxInclusiveAmount currencyID="EUR">649.74</cbc:TaxInclusiveAmount>
        <cbc:PayableAmount currencyID="EUR">649.74</cbc:PayableAmount>
    </cac:LegalMonetaryTotal>
    <cac:InvoiceLine>
        <cbc:ID>1</cbc:ID>
        <cbc:InvoicedQuantity unitCode="MTR">12</cbc:InvoicedQuantity>
        <cbc:LineExtensionAmount currencyID="EUR">546.00</cbc:LineExtensionAmount>
        <cac:Item>
            <cbc:Name>Aluminiumprofil MB-70</cbc:Name>
            <cac:ClassifiedTaxCategory>
                <cbc:ID>S</cbc:ID>
                <cbc:Percent>19.00</cbc:Percent>
                <cac:TaxScheme>
                    <cbc:ID>VAT</cbc:ID>
                </cac:TaxScheme>
            </cac:ClassifiedTaxCategory>
        </cac:Item>
        <cac:Price>
            <cbc:PriceAmount currencyID="EUR">45.50</cbc:PriceAmount>
        </cac:Price>
    </cac:InvoiceLine>
</ubl:Invoice>`

func TestParseUBLInvoiceReadsAllFields(t *testing.T) {
	got, err := ParseUBLInvoice([]byte(ublInboundSample))
	if err != nil {
		t.Fatalf("ParseUBLInvoice: %v", err)
	}

	if got.Format != EInvoiceFormatUBL {
		t.Errorf("Format = %q, erwartet %q", got.Format, EInvoiceFormatUBL)
	}
	if got.Number != "LIEF-2026-0815" || got.TypeCode != "380" || got.Currency != "EUR" {
		t.Errorf("Kopfdaten falsch: %+v", got)
	}
	if !got.IssueDate.Equal(time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Rechnungsdatum = %v", got.IssueDate)
	}
	if got.DueDate == nil || !got.DueDate.Equal(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Fälligkeitsdatum = %v", got.DueDate)
	}
	if got.BuyerReference != "BESTELL-4711" {
		t.Errorf("Käuferreferenz = %q", got.BuyerReference)
	}

	if got.Seller.Name != "Profilwerk Süd GmbH" {
		t.Errorf("der eingetragene Name (RegistrationName) muss Vorrang vor dem Handelsnamen haben, got %q", got.Seller.Name)
	}
	if got.Seller.Street != "Industriestraße 12" || got.Seller.PostalCode != "70173" ||
		got.Seller.City != "Stuttgart" || got.Seller.Country != "DE" {
		t.Errorf("Verkäuferanschrift falsch: %+v", got.Seller)
	}
	if got.Seller.VatID != "DE555666777" {
		t.Errorf("USt-IdNr. (TaxScheme VAT) = %q", got.Seller.VatID)
	}
	if got.Seller.TaxNo != "99/123/45678" {
		t.Errorf("Steuernummer (TaxScheme FC) = %q", got.Seller.TaxNo)
	}
	if got.Buyer.Name != "Metallbau Muster GmbH" || got.Buyer.City != "Duesseldorf" {
		t.Errorf("Käufer falsch: %+v", got.Buyer)
	}

	if len(got.Lines) != 1 {
		t.Fatalf("erwartet 1 Position, got %d", len(got.Lines))
	}
	l := got.Lines[0]
	if l.LineID != "1" || l.Name != "Aluminiumprofil MB-70" || l.Qty != 12 || l.UnitCode != "MTR" ||
		l.NetUnitPrice != 45.50 || l.LineTotalAmount != 546.00 || l.TaxCategoryCode != "S" || l.TaxRatePercent != 19 {
		t.Errorf("Position falsch: %+v", l)
	}

	if len(got.TaxBreakdown) != 1 {
		t.Fatalf("erwartet 1 Steuergruppe, got %d", len(got.TaxBreakdown))
	}
	if tb := got.TaxBreakdown[0]; tb.CategoryCode != "S" || tb.RatePercent != 19 ||
		tb.BasisAmount != 546 || tb.CalculatedAmount != 103.74 {
		t.Errorf("Steuergruppe falsch: %+v", got.TaxBreakdown[0])
	}

	if got.LineTotalAmount != 546 || got.TaxBasisTotalAmount != 546 || got.TaxTotalAmount != 103.74 ||
		got.GrandTotalAmount != 649.74 || got.DuePayableAmount != 649.74 {
		t.Errorf("Summen falsch: %+v", got)
	}
	if len(got.Hinweise) != 0 {
		t.Errorf("stimmige Rechnung darf keine Hinweise erzeugen: %v", got.Hinweise)
	}
}

// TestBothSyntaxesYieldIdenticalModel ist der zentrale Test dieser
// Subtask: derselbe Geschaeftsvorfall in CII und in UBL muss - bis auf die
// Formatkennung - exakt dasselbe Zielmodell ergeben. Genau das ist die
// Zusage aus ADR 0023, auf die sich alles Nachgelagerte (E.5.5) verlaesst.
func TestBothSyntaxesYieldIdenticalModel(t *testing.T) {
	fromCII, err := ParseCIIInvoice([]byte(ciiInboundSample))
	if err != nil {
		t.Fatalf("ParseCIIInvoice: %v", err)
	}
	fromUBL, err := ParseUBLInvoice([]byte(ublInboundSample))
	if err != nil {
		t.Fatalf("ParseUBLInvoice: %v", err)
	}

	if fromCII.Format != EInvoiceFormatCII || fromUBL.Format != EInvoiceFormatUBL {
		t.Fatalf("Formatkennungen falsch: %q / %q", fromCII.Format, fromUBL.Format)
	}

	// Die Formatkennung ist der EINZIGE erlaubte Unterschied - danach
	// muessen die Modelle tief gleich sein.
	fromUBL.Format = fromCII.Format
	if !reflect.DeepEqual(fromCII, fromUBL) {
		t.Errorf("CII und UBL ergeben unterschiedliche Modelle.\nCII: %+v\nUBL: %+v", fromCII, fromUBL)
	}
}

// TestParseEInvoiceXMLDispatchesByNamespace beweist die Formaterkennung
// ueber den Wurzelelement-Namensraum (ADR 0023) - nicht ueber Dateiname
// oder Endung.
func TestParseEInvoiceXMLDispatchesByNamespace(t *testing.T) {
	cii, err := ParseEInvoiceXML([]byte(ciiInboundSample))
	if err != nil {
		t.Fatalf("CII über Dispatch: %v", err)
	}
	if cii.Format != EInvoiceFormatCII || cii.Number != "LIEF-2026-0815" {
		t.Errorf("CII falsch erkannt: %+v", cii)
	}

	ubl, err := ParseEInvoiceXML([]byte(ublInboundSample))
	if err != nil {
		t.Fatalf("UBL über Dispatch: %v", err)
	}
	if ubl.Format != EInvoiceFormatUBL || ubl.Number != "LIEF-2026-0815" {
		t.Errorf("UBL falsch erkannt: %+v", ubl)
	}
}

func TestParseEInvoiceXMLRejectsUnknownFormat(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantSub string
	}{
		{
			"fremdes Wurzelelement",
			`<?xml version="1.0"?><Rechnung><Nummer>1</Nummer></Rechnung>`,
			"unbekanntes Format",
		},
		{
			"richtiger Name, falscher Namensraum",
			`<?xml version="1.0"?><Invoice xmlns="urn:example:eigenes-format">
			   <ID>1</ID></Invoice>`,
			"unbekanntes Format",
		},
		{
			"kein XML",
			"PK\x03\x04 irgendeine Binaerdatei",
			"nicht gelesen werden",
		},
		{
			"leere Datei",
			"",
			"nicht gelesen werden",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseEInvoiceXML([]byte(c.input))
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

// TestParseUBLInvoiceIsPrefixIndependent beweist auch fuer UBL, dass ueber
// Namensraeume und nicht ueber Praefixe gematcht wird.
func TestParseUBLInvoiceIsPrefixIndependent(t *testing.T) {
	// Nur die Praefixe an den Element-Begrenzern ersetzen, NICHT per
	// blossem "ubl:" - diese Zeichenfolge kommt auch INNERHALB der
	// Namensraum-URI vor (urn:oasis:names:specification:ubl:schema:...),
	// und die darf der Test nicht veraendern, sonst prueft er etwas
	// anderes als beabsichtigt.
	weird := strings.NewReplacer(
		"xmlns:ubl=", "xmlns:x=",
		"xmlns:cac=", "xmlns:y=",
		"xmlns:cbc=", "xmlns:z=",
		"<ubl:", "<x:", "</ubl:", "</x:",
		"<cac:", "<y:", "</cac:", "</y:",
		"<cbc:", "<z:", "</cbc:", "</z:",
	).Replace(ublInboundSample)

	got, err := ParseUBLInvoice([]byte(weird))
	if err != nil {
		t.Fatalf("Dokument mit anderen Präfixen muss lesbar sein: %v", err)
	}
	if got.Number != "LIEF-2026-0815" || got.Seller.VatID != "DE555666777" || got.GrandTotalAmount != 649.74 {
		t.Errorf("Inhalt bei geänderten Präfixen nicht korrekt gelesen: %+v", got)
	}
}

// TestParseUBLInvoiceRejectsForeignSyntax: ein CII-Dokument darf nicht als
// leere UBL-Rechnung durchgehen.
func TestParseUBLInvoiceRejectsForeignSyntax(t *testing.T) {
	_, err := ParseUBLInvoice([]byte(ciiInboundSample))
	if err == nil {
		t.Fatal("ein CII-Dokument darf nicht als UBL akzeptiert werden")
	}
	if !strings.Contains(err.Error(), "kein UBL-Dokument") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
	}
}

func TestParseUBLInvoiceRejectsBrokenInput(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantSub string
	}{
		{
			"ohne Rechnungsnummer",
			strings.Replace(ublInboundSample, "<cbc:ID>LIEF-2026-0815</cbc:ID>", "<cbc:ID></cbc:ID>", 1),
			"Rechnungsnummer fehlt",
		},
		{
			"ohne Rechnungsdatum",
			strings.Replace(ublInboundSample, "<cbc:IssueDate>2026-05-18</cbc:IssueDate>", "", 1),
			"Rechnungsdatum",
		},
		{
			"unlesbares Rechnungsdatum",
			strings.Replace(ublInboundSample, "2026-05-18", "18.05.2026", 1),
			"unlesbar",
		},
		{
			"unlesbares Fälligkeitsdatum",
			strings.Replace(ublInboundSample, "<cbc:DueDate>2026-06-17</cbc:DueDate>", "<cbc:DueDate>naechsten Monat</cbc:DueDate>", 1),
			"Fälligkeitsdatum",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseUBLInvoice([]byte(c.input))
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

// TestParseUBLInvoiceIgnoresUnknownTaxScheme deckt einen Fall ab, der in
// den KoSIT-Beispielen tatsaechlich vorkommt: ein PartyTaxScheme mit einer
// Platzhalter-Kennung ("???") statt VAT oder FC. Solche Angaben duerfen
// NICHT als Steuernummer durchgereicht werden.
func TestParseUBLInvoiceIgnoresUnknownTaxScheme(t *testing.T) {
	mitPlatzhalter := strings.Replace(ublInboundSample,
		`<cac:PartyTaxScheme>
                <cbc:CompanyID>99/123/45678</cbc:CompanyID>
                <cac:TaxScheme>
                    <cbc:ID>FC</cbc:ID>
                </cac:TaxScheme>
            </cac:PartyTaxScheme>`,
		`<cac:PartyTaxScheme>
                <cbc:CompanyID>99/123/45678</cbc:CompanyID>
                <cac:TaxScheme>
                    <cbc:ID>???</cbc:ID>
                </cac:TaxScheme>
            </cac:PartyTaxScheme>`, 1)

	got, err := ParseUBLInvoice([]byte(mitPlatzhalter))
	if err != nil {
		t.Fatalf("ParseUBLInvoice: %v", err)
	}
	if got.Seller.TaxNo != "" {
		t.Errorf("unbekanntes TaxScheme darf nicht als Steuernummer übernommen werden, got %q", got.Seller.TaxNo)
	}
	if got.Seller.VatID != "DE555666777" {
		t.Errorf("die gültige USt-IdNr. muss erhalten bleiben, got %q", got.Seller.VatID)
	}
}

// TestParseUBLInvoiceReadsPrepaidAmount deckt die Anzahlung ab
// (cbc:PrepaidAmount in cac:LegalMonetaryTotal).
func TestParseUBLInvoiceReadsPrepaidAmount(t *testing.T) {
	mitAnzahlung := strings.Replace(ublInboundSample,
		`<cbc:PayableAmount currencyID="EUR">649.74</cbc:PayableAmount>`,
		`<cbc:PrepaidAmount currencyID="EUR">200.00</cbc:PrepaidAmount>
        <cbc:PayableAmount currencyID="EUR">449.74</cbc:PayableAmount>`, 1)

	got, err := ParseUBLInvoice([]byte(mitAnzahlung))
	if err != nil {
		t.Fatalf("ParseUBLInvoice: %v", err)
	}
	if got.PrepaidAmount != 200 {
		t.Errorf("Anzahlung = %v, erwartet 200", got.PrepaidAmount)
	}
	if got.DuePayableAmount != 449.74 {
		t.Errorf("Zahlbetrag = %v, erwartet 449.74", got.DuePayableAmount)
	}
}

// TestParseUBLInvoiceUsesEndpointAsEmailFallback: fehlt cac:Contact, ist
// die EndpointID mit schemeID "EM" die naechstbeste E-Mail-Quelle.
func TestParseUBLInvoiceUsesEndpointAsEmailFallback(t *testing.T) {
	ohneContact := strings.Replace(ublInboundSample,
		`<cac:Contact>
                <cbc:ElectronicMail>rechnung@profilwerk.example</cbc:ElectronicMail>
            </cac:Contact>`, "", 1)

	got, err := ParseUBLInvoice([]byte(ohneContact))
	if err != nil {
		t.Fatalf("ParseUBLInvoice: %v", err)
	}
	if got.Seller.Email != "rechnung@profilwerk.example" {
		t.Errorf("E-Mail-Fallback über EndpointID greift nicht, got %q", got.Seller.Email)
	}
}

// TestParseUBLInvoiceReportsImplausibleSums: auch fuer UBL gilt die
// Entscheidung aus ADR 0023 - melden, nicht korrigieren. Der Hinweistext
// kommt aus derselben, syntaxunabhaengigen Pruefung wie bei CII.
func TestParseUBLInvoiceReportsImplausibleSums(t *testing.T) {
	broken := strings.Replace(ublInboundSample,
		`<cbc:TaxInclusiveAmount currencyID="EUR">649.74</cbc:TaxInclusiveAmount>`,
		`<cbc:TaxInclusiveAmount currencyID="EUR">999.99</cbc:TaxInclusiveAmount>`, 1)

	got, err := ParseUBLInvoice([]byte(broken))
	if err != nil {
		t.Fatalf("unplausible Summe darf kein Parse-Fehler sein: %v", err)
	}
	if got.GrandTotalAmount != 999.99 {
		t.Errorf("gelieferter Wert muss unverändert übernommen werden, got %v", got.GrandTotalAmount)
	}
	if !strings.Contains(strings.Join(got.Hinweise, " "), "999.99") {
		t.Errorf("erwartet Hinweis auf die abweichende Summe, got %v", got.Hinweise)
	}
}

// TestParseUBLInvoiceDoesNotResolveExternalEntities: derselbe Nachweis wie
// fuer CII, weil der Eingang auch hier von Dritten stammt (ADR 0023).
func TestParseUBLInvoiceDoesNotResolveExternalEntities(t *testing.T) {
	xxe := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [ <!ENTITY xxe SYSTEM "file:///etc/passwd"> ]>
<ubl:Invoice xmlns:ubl="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
             xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
    <cbc:ID>&xxe;</cbc:ID>
    <cbc:IssueDate>2026-05-18</cbc:IssueDate>
</ubl:Invoice>`

	got, err := ParseUBLInvoice([]byte(xxe))
	if err != nil {
		return
	}
	if strings.Contains(got.Number, "root:") || strings.Contains(got.Number, "/bin/") {
		t.Fatalf("externe Entität wurde aufgelöst - Dateiinhalt im Ergebnis: %q", got.Number)
	}
}
