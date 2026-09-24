package accounting

// Unit-Tests fuer den CII-Serialisierer (Backlog E.4.3.1). Bewusst
// vollstaendig DB-los - der Baustein hat per Design keinen
// Datenbankzugriff, die Tests laufen daher ohne NALA_INTEGRATION=1.
//
// Die Soll-Werte stammen aus den in ADR 0022 recherchierten offiziellen
// KoSIT-Beispieldateien (01.01a und 01.21a in uncefact-Syntax), nicht aus
// der Erwartungshaltung des Codes selbst.

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func ciiTestInvoice() CIIInvoice {
	due := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	return CIIInvoice{
		Number:         "RE-2026-0042",
		TypeCode:       CIITypeCodeCommercialInvoice,
		IssueDate:      time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		DueDate:        &due,
		Currency:       "EUR",
		BuyerReference: "991-12345-67",
		Seller: CIIParty{
			Name:       "Metallbau Muster GmbH",
			Street:     "Werkstrasse 4",
			PostalCode: "40213",
			City:       "Duesseldorf",
			Country:    "DE",
			Email:      "rechnung@metallbau-muster.de",
			Phone:      "+49 211 1234567",
			VatID:      "DE123456789",
			TaxNo:      "133/8150/1234",
		},
		Buyer: CIIParty{
			ID:         "K-1001",
			Name:       "Bauamt Beispielstadt",
			Street:     "Rathausplatz 1",
			PostalCode: "50667",
			City:       "Koeln",
			Country:    "DE",
			Email:      "rechnungseingang@beispielstadt.de",
		},
		IBAN:          "DE02120300000000202051",
		BIC:           "BYLADEM1001",
		AccountHolder: "Metallbau Muster GmbH",
		PaymentTerms:  "Zahlbar innerhalb von 30 Tagen ohne Abzug.",
		Lines: []CIILineItem{
			{
				LineID:          "1",
				Name:            "Fensterelement RC2, Profilserie MB-70",
				Qty:             3,
				NetUnitPrice:    1250,
				LineTotalAmount: 3750,
				TaxCategoryCode: CIITaxCategoryStandard,
				TaxRatePercent:  19,
			},
			{
				LineID:          "2",
				Name:            "Montage vor Ort",
				Qty:             8.5,
				UnitCode:        "HUR",
				NetUnitPrice:    68,
				LineTotalAmount: 578,
				TaxCategoryCode: CIITaxCategoryStandard,
				TaxRatePercent:  19,
			},
		},
		TaxBreakdown: []CIITaxBreakdown{
			{CategoryCode: CIITaxCategoryStandard, RatePercent: 19, BasisAmount: 4328, CalculatedAmount: 822.32},
		},
		LineTotalAmount:     4328,
		TaxBasisTotalAmount: 4328,
		TaxTotalAmount:      822.32,
		GrandTotalAmount:    5150.32,
		DuePayableAmount:    5150.32,
	}
}

func buildCIIForTest(t *testing.T, inv CIIInvoice) string {
	t.Helper()
	out, err := BuildCrossIndustryInvoice(inv)
	if err != nil {
		t.Fatalf("BuildCrossIndustryInvoice: unerwarteter Fehler: %v", err)
	}
	return string(out)
}

// TestBuildCIIProducesWellFormedXML beweist, dass die Ausgabe ueberhaupt
// wohlgeformtes XML ist - der Praefix-Trick (Praefixe als Teil des
// Elementnamens) darf kein kaputtes Dokument erzeugen.
func TestBuildCIIProducesWellFormedXML(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	dec := xml.NewDecoder(strings.NewReader(got))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("Ausgabe ist kein wohlgeformtes XML: %v", err)
		}
	}

	if !strings.HasPrefix(got, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("XML-Deklaration fehlt oder falsch, Anfang: %q", got[:min(60, len(got))])
	}
}

// TestBuildCIIRootAndNamespaces prueft Wurzelelement, Praefixe und die vier
// Namensraeume exakt gegen die KoSIT-Beispieldatei.
func TestBuildCIIRootAndNamespaces(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	for _, want := range []string{
		`<rsm:CrossIndustryInvoice`,
		`xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"`,
		`xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"`,
		`xmlns:qdt="urn:un:unece:uncefact:data:standard:QualifiedDataType:100"`,
		`xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100"`,
		`</rsm:CrossIndustryInvoice>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Ausgabe enthaelt %q nicht", want)
		}
	}
}

// TestBuildCIIContextIDs prueft die beiden Kennungen, die eine Datei
// ueberhaupt erst als XRechnung 3.0 ausweisen.
func TestBuildCIIContextIDs(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	if !strings.Contains(got, `<ram:ID>urn:fdc:peppol.eu:2017:poacc:billing:01:1.0</ram:ID>`) {
		t.Error("ProfileID (Peppol BIS Billing 3.0) fehlt")
	}
	if !strings.Contains(got, `<ram:ID>urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0</ram:ID>`) {
		t.Error("GuidelineID (XRechnung 3.0) fehlt")
	}
	if idx := strings.Index(got, "BusinessProcessSpecifiedDocumentContextParameter"); idx == -1 {
		t.Error("BusinessProcessSpecifiedDocumentContextParameter fehlt")
	} else if idx > strings.Index(got, "GuidelineSpecifiedDocumentContextParameter") {
		t.Error("BusinessProcess muss laut XSD-sequence VOR Guideline stehen")
	}
}

// TestBuildCIIDocumentHeader prueft Rechnungsnummer, Typ-Code und das
// Datumsformat 102 (CCYYMMDD).
func TestBuildCIIDocumentHeader(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	if !strings.Contains(got, `<ram:ID>RE-2026-0042</ram:ID>`) {
		t.Error("Rechnungsnummer fehlt")
	}
	if !strings.Contains(got, `<ram:TypeCode>380</ram:TypeCode>`) {
		t.Error("Rechnungstyp-Code 380 fehlt")
	}
	if !strings.Contains(got, `<udt:DateTimeString format="102">20260320</udt:DateTimeString>`) {
		t.Error("Rechnungsdatum im Format 102 fehlt")
	}
}

// TestBuildCIIPrepaymentTypeCode beweist die Zuordnung aus ADR 0022:
// Abschlagsrechnung => 386.
func TestBuildCIIPrepaymentTypeCode(t *testing.T) {
	inv := ciiTestInvoice()
	inv.TypeCode = CIITypeCodePrepaymentInvoice

	got := buildCIIForTest(t, inv)

	if !strings.Contains(got, `<ram:TypeCode>386</ram:TypeCode>`) {
		t.Error("Abschlagsrechnung muss Typ-Code 386 tragen")
	}
}

// TestBuildCIISellerAndBuyer prueft beide Parteien inkl. Anschrift,
// USt-IdNr. (schemeID VA), Steuernummer (schemeID FC) und der Regel, dass
// die Steuernummer NUR beim Verkaeufer ausgegeben wird.
func TestBuildCIISellerAndBuyer(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	for _, want := range []string{
		`<ram:BuyerReference>991-12345-67</ram:BuyerReference>`,
		`<ram:Name>Metallbau Muster GmbH</ram:Name>`,
		`<ram:PostcodeCode>40213</ram:PostcodeCode>`,
		`<ram:LineOne>Werkstrasse 4</ram:LineOne>`,
		`<ram:CityName>Duesseldorf</ram:CityName>`,
		`<ram:CountryID>DE</ram:CountryID>`,
		`<ram:ID schemeID="FC">133/8150/1234</ram:ID>`,
		`<ram:ID schemeID="VA">DE123456789</ram:ID>`,
		`<ram:CompleteNumber>+49 211 1234567</ram:CompleteNumber>`,
		`<ram:ID>K-1001</ram:ID>`,
		`<ram:Name>Bauamt Beispielstadt</ram:Name>`,
		`<ram:PostcodeCode>50667</ram:PostcodeCode>`,
		`<ram:URIID schemeID="EM">rechnungseingang@beispielstadt.de</ram:URIID>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Ausgabe enthaelt %q nicht", want)
		}
	}

	if strings.Count(got, `schemeID="FC"`) != 1 {
		t.Error("Steuernummer (FC) darf genau einmal - nur beim Verkaeufer - vorkommen")
	}
	buyerBlock := got[strings.Index(got, "<ram:BuyerTradeParty>"):strings.Index(got, "</ram:BuyerTradeParty>")]
	if strings.Contains(buyerBlock, "DefinedTradeContact") {
		t.Error("DefinedTradeContact ist laut Design nur fuer den Verkaeufer vorgesehen")
	}
}

// TestBuildCIILineItems prueft die Positionen inkl. Mengeneinheit,
// Einzelpreis und Positionssumme.
func TestBuildCIILineItems(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	if n := strings.Count(got, "<ram:IncludedSupplyChainTradeLineItem>"); n != 2 {
		t.Errorf("erwartet 2 Positionen, gefunden %d", n)
	}
	for _, want := range []string{
		`<ram:LineID>1</ram:LineID>`,
		`<ram:Name>Fensterelement RC2, Profilserie MB-70</ram:Name>`,
		`<ram:ChargeAmount>1250.00</ram:ChargeAmount>`,
		`<ram:BilledQuantity unitCode="C62">3</ram:BilledQuantity>`,
		`<ram:LineTotalAmount>3750.00</ram:LineTotalAmount>`,
		`<ram:BilledQuantity unitCode="HUR">8.5</ram:BilledQuantity>`,
		`<ram:LineTotalAmount>578.00</ram:LineTotalAmount>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Ausgabe enthaelt %q nicht", want)
		}
	}
}

// TestBuildCIIDefaultUnitCode beweist den in ADR 0022 festgelegten festen
// Fallback C62, da invoice_out_items keine Einheit traegt.
func TestBuildCIIDefaultUnitCode(t *testing.T) {
	inv := ciiTestInvoice()
	for i := range inv.Lines {
		inv.Lines[i].UnitCode = ""
	}

	got := buildCIIForTest(t, inv)

	if n := strings.Count(got, `unitCode="C62"`); n != 2 {
		t.Errorf("erwartet 2x Fallback-Einheit C62, gefunden %d", n)
	}
}

// TestBuildCIISettlementTotals prueft die Summenzeile und dass Betraege
// IMMER mit genau zwei Nachkommastellen und Punkt als Dezimaltrennzeichen
// ausgegeben werden (anders als im DATEV-Export, der Komma verlangt).
func TestBuildCIISettlementTotals(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())

	for _, want := range []string{
		`<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>`,
		`<ram:LineTotalAmount>4328.00</ram:LineTotalAmount>`,
		`<ram:TaxBasisTotalAmount>4328.00</ram:TaxBasisTotalAmount>`,
		`<ram:TaxTotalAmount currencyID="EUR">822.32</ram:TaxTotalAmount>`,
		`<ram:GrandTotalAmount>5150.32</ram:GrandTotalAmount>`,
		`<ram:DuePayableAmount>5150.32</ram:DuePayableAmount>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Ausgabe enthaelt %q nicht", want)
		}
	}
	if strings.Contains(got, "4328,00") {
		t.Error("Betraege duerfen kein Komma als Dezimaltrennzeichen verwenden")
	}
}

// TestBuildCIIPrepaidAmountOnlyWhenPaid beweist, dass TotalPrepaidAmount
// nur erscheint, wenn tatsaechlich eine Anzahlung vorliegt.
func TestBuildCIIPrepaidAmountOnlyWhenPaid(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())
	if strings.Contains(got, "TotalPrepaidAmount") {
		t.Error("ohne Anzahlung darf TotalPrepaidAmount nicht erscheinen")
	}

	inv := ciiTestInvoice()
	inv.PaidAmount = 1000
	inv.DuePayableAmount = 4150.32

	got = buildCIIForTest(t, inv)
	if !strings.Contains(got, `<ram:TotalPrepaidAmount>1000.00</ram:TotalPrepaidAmount>`) {
		t.Error("mit Anzahlung muss TotalPrepaidAmount erscheinen")
	}
	if !strings.Contains(got, `<ram:DuePayableAmount>4150.32</ram:DuePayableAmount>`) {
		t.Error("Restbetrag muss uebernommen werden")
	}
}

// TestBuildCIIPaymentMeans prueft den Zahlungsweg inkl. BIC - und dass der
// gesamte Block entfaellt, wenn keine IBAN gepflegt ist (statt einen
// Zahlungsweg zu behaupten, den es nicht gibt).
func TestBuildCIIPaymentMeans(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())
	for _, want := range []string{
		`<ram:TypeCode>58</ram:TypeCode>`,
		`<ram:IBANID>DE02120300000000202051</ram:IBANID>`,
		`<ram:AccountName>Metallbau Muster GmbH</ram:AccountName>`,
		`<ram:BICID>BYLADEM1001</ram:BICID>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Ausgabe enthaelt %q nicht", want)
		}
	}

	inv := ciiTestInvoice()
	inv.IBAN = ""
	got = buildCIIForTest(t, inv)
	if strings.Contains(got, "SpecifiedTradeSettlementPaymentMeans") {
		t.Error("ohne IBAN darf kein Zahlungsweg-Block ausgegeben werden")
	}

	inv = ciiTestInvoice()
	inv.BIC = ""
	got = buildCIIForTest(t, inv)
	if strings.Contains(got, "PayeeSpecifiedCreditorFinancialInstitution") {
		t.Error("ohne BIC darf kein Kreditinstitut-Block ausgegeben werden")
	}
	if !strings.Contains(got, `<ram:IBANID>DE02120300000000202051</ram:IBANID>`) {
		t.Error("IBAN muss auch ohne BIC ausgegeben werden")
	}
}

// TestBuildCIIPaymentTermsAndDueDate prueft Zahlungsziel und
// Zahlungsbedingung - inkl. des Falls, dass beides fehlt.
func TestBuildCIIPaymentTermsAndDueDate(t *testing.T) {
	got := buildCIIForTest(t, ciiTestInvoice())
	if !strings.Contains(got, `<ram:Description>Zahlbar innerhalb von 30 Tagen ohne Abzug.</ram:Description>`) {
		t.Error("Zahlungsbedingung fehlt")
	}
	if !strings.Contains(got, `<udt:DateTimeString format="102">20260420</udt:DateTimeString>`) {
		t.Error("Faelligkeitsdatum im Format 102 fehlt")
	}

	inv := ciiTestInvoice()
	inv.DueDate = nil
	inv.PaymentTerms = ""
	got = buildCIIForTest(t, inv)
	if strings.Contains(got, "SpecifiedTradePaymentTerms") {
		t.Error("ohne Faelligkeit und ohne Zahlungsbedingung darf der Block entfallen")
	}
}

// TestBuildCIITaxBreakdownFieldOrder prueft die Steueraufschluesselung und
// vor allem die XSD-Reihenfolge CalculatedAmount, TypeCode, ExemptionReason,
// BasisAmount, CategoryCode, RateApplicablePercent (KoSIT 01.21a). Eine
// Umsortierung wuerde das Dokument schemaungueltig machen.
func TestBuildCIITaxBreakdownFieldOrder(t *testing.T) {
	inv := ciiTestInvoice()
	inv.TaxBreakdown = []CIITaxBreakdown{{
		CategoryCode:     CIITaxCategoryReverseCharge,
		RatePercent:      0,
		BasisAmount:      4328,
		CalculatedAmount: 0,
		ExemptionReason:  "Steuerschuldnerschaft des Leistungsempfaengers",
	}}
	inv.TaxTotalAmount = 0
	inv.GrandTotalAmount = 4328
	inv.DuePayableAmount = 4328
	for i := range inv.Lines {
		inv.Lines[i].TaxCategoryCode = CIITaxCategoryReverseCharge
		inv.Lines[i].TaxRatePercent = 0
	}

	got := buildCIIForTest(t, inv)

	start := strings.LastIndex(got, "<ram:ApplicableTradeTax>")
	block := got[start:]
	block = block[:strings.Index(block, "</ram:ApplicableTradeTax>")]

	order := []string{
		"<ram:CalculatedAmount>0.00</ram:CalculatedAmount>",
		"<ram:TypeCode>VAT</ram:TypeCode>",
		"<ram:ExemptionReason>Steuerschuldnerschaft des Leistungsempfaengers</ram:ExemptionReason>",
		"<ram:BasisAmount>4328.00</ram:BasisAmount>",
		"<ram:CategoryCode>AE</ram:CategoryCode>",
		"<ram:RateApplicablePercent>0.00</ram:RateApplicablePercent>",
	}
	prev := -1
	for _, want := range order {
		idx := strings.Index(block, want)
		if idx == -1 {
			t.Fatalf("Steuerblock enthaelt %q nicht; Block:\n%s", want, block)
		}
		if idx <= prev {
			t.Errorf("Feldreihenfolge verletzt: %q steht zu frueh", want)
		}
		prev = idx
	}
}

// TestBuildCIIMultipleTaxBreakdowns beweist, dass mehrere
// Steuersatz-Gruppen je eine eigene Aufschluesselungszeile erzeugen.
func TestBuildCIIMultipleTaxBreakdowns(t *testing.T) {
	inv := ciiTestInvoice()
	inv.Lines[1].TaxRatePercent = 7
	inv.TaxBreakdown = []CIITaxBreakdown{
		{CategoryCode: CIITaxCategoryStandard, RatePercent: 19, BasisAmount: 3750, CalculatedAmount: 712.50},
		{CategoryCode: CIITaxCategoryStandard, RatePercent: 7, BasisAmount: 578, CalculatedAmount: 40.46},
	}

	got := buildCIIForTest(t, inv)

	settlement := got[strings.Index(got, "<ram:ApplicableHeaderTradeSettlement>"):]
	if n := strings.Count(settlement, "<ram:ApplicableTradeTax>"); n != 2 {
		t.Errorf("erwartet 2 Steueraufschluesselungszeilen auf Belegebene, gefunden %d", n)
	}
	for _, want := range []string{
		`<ram:CalculatedAmount>712.50</ram:CalculatedAmount>`,
		`<ram:CalculatedAmount>40.46</ram:CalculatedAmount>`,
	} {
		if !strings.Contains(settlement, want) {
			t.Errorf("Ausgabe enthaelt %q nicht", want)
		}
	}
}

// TestBuildCIIEscapesSpecialCharacters beweist, dass Sonderzeichen korrekt
// maskiert werden und Umlaute erhalten bleiben - ein unmaskiertes & wuerde
// die Datei beim Empfaenger unlesbar machen.
func TestBuildCIIEscapesSpecialCharacters(t *testing.T) {
	inv := ciiTestInvoice()
	inv.Buyer.Name = "Müller & Söhne <GmbH>"
	inv.Lines[0].Name = `Tür "Modell A" & Zarge`

	got := buildCIIForTest(t, inv)

	if !strings.Contains(got, "Müller &amp; Söhne &lt;GmbH&gt;") {
		t.Error("Sonderzeichen im Kaeufernamen nicht korrekt maskiert")
	}
	if !strings.Contains(got, "Tür &#34;Modell A&#34; &amp; Zarge") {
		t.Error("Sonderzeichen in der Positionsbezeichnung nicht korrekt maskiert")
	}

	dec := xml.NewDecoder(strings.NewReader(got))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("Ausgabe mit Sonderzeichen ist nicht wohlgeformt: %v", err)
		}
	}
}

// TestBuildCIICountryFallback beweist den Fallback auf DE, da CountryID in
// EN 16931 Pflicht ist.
func TestBuildCIICountryFallback(t *testing.T) {
	inv := ciiTestInvoice()
	inv.Seller.Country = ""
	inv.Buyer.Country = "at"

	got := buildCIIForTest(t, inv)

	if n := strings.Count(got, "<ram:CountryID>DE</ram:CountryID>"); n != 1 {
		t.Errorf("erwartet genau einen DE-Fallback (Verkaeufer), gefunden %d", n)
	}
	if !strings.Contains(got, "<ram:CountryID>AT</ram:CountryID>") {
		t.Error("Laendercode muss in Grossbuchstaben normalisiert werden")
	}
}

// TestBuildCIIQuantityFormatting prueft die Mengenformatierung ohne
// ueberfluessige Nullen, aber mit voller Genauigkeit.
func TestBuildCIIQuantityFormatting(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{3, "3"},
		{8.5, "8.5"},
		{0.25, "0.25"},
		{1.2345, "1.2345"},
		{10.1000, "10.1"},
	}
	for _, c := range cases {
		if got := ciiQuantityString(c.in); got != c.want {
			t.Errorf("ciiQuantityString(%v) = %q, erwartet %q", c.in, got, c.want)
		}
	}
}

// --- Negativfaelle --------------------------------------------------------

// TestBuildCIIRejectsIncompleteInvoice deckt die Pflichtangaben ab. Jeder
// Fall muss einen Fehler liefern statt ein Dokument mit erfundenen oder
// fehlenden Pflichtangaben zu erzeugen.
func TestBuildCIIRejectsIncompleteInvoice(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*CIIInvoice)
		wantSub string
	}{
		{"ohne Rechnungsnummer", func(i *CIIInvoice) { i.Number = "" }, "Rechnungsnummer"},
		{"ohne Typ-Code", func(i *CIIInvoice) { i.TypeCode = "" }, "Rechnungstyp-Code"},
		{"ohne Rechnungsdatum", func(i *CIIInvoice) { i.IssueDate = time.Time{} }, "Rechnungsdatum"},
		{"ohne Waehrung", func(i *CIIInvoice) { i.Currency = "" }, "Waehrung"},
		{"ohne Kaeuferreferenz", func(i *CIIInvoice) { i.BuyerReference = "" }, "Kaeuferreferenz"},
		{"ohne Verkaeufername", func(i *CIIInvoice) { i.Seller.Name = "" }, "Name des Verkaeufers"},
		{"ohne Kaeuferanschrift", func(i *CIIInvoice) { i.Buyer.Street = "" }, "Anschrift des Kaeufers"},
		{"ohne Kaeufer-PLZ", func(i *CIIInvoice) { i.Buyer.PostalCode = "" }, "Anschrift des Kaeufers"},
		{"ohne Steuerkennung des Verkaeufers", func(i *CIIInvoice) { i.Seller.VatID = ""; i.Seller.TaxNo = "" }, "USt-IdNr"},
		{"ohne Positionen", func(i *CIIInvoice) { i.Lines = nil }, "mindestens eine Rechnungsposition"},
		{"Position ohne Bezeichnung", func(i *CIIInvoice) { i.Lines[1].Name = "" }, "Position 2"},
		{"Position ohne Steuerkategorie", func(i *CIIInvoice) { i.Lines[0].TaxCategoryCode = "" }, "Position 1"},
		{"ohne Steueraufschluesselung", func(i *CIIInvoice) { i.TaxBreakdown = nil }, "Steueraufschluesselung"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inv := ciiTestInvoice()
			c.mutate(&inv)

			out, err := BuildCrossIndustryInvoice(inv)
			if err == nil {
				t.Fatalf("erwartet Fehler, aber es wurde ein Dokument erzeugt (%d Bytes)", len(out))
			}
			if out != nil {
				t.Error("bei einem Fehler darf kein Teildokument zurueckgegeben werden")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("Fehlermeldung %q nennt %q nicht", err.Error(), c.wantSub)
			}
		})
	}
}

// TestBuildCIIRejectsInvalidTaxCategory deckt die Steuerkategorie-Regeln aus
// EN 16931 ab: S ohne Befreiungsbegruendung, AE/E nur MIT Begruendung, und
// keine erfundenen Kategorien.
func TestBuildCIIRejectsInvalidTaxCategory(t *testing.T) {
	cases := []struct {
		name    string
		tax     CIITaxBreakdown
		wantSub string
	}{
		{
			"S mit Befreiungsbegruendung",
			CIITaxBreakdown{CategoryCode: CIITaxCategoryStandard, RatePercent: 19, ExemptionReason: "Steuerbefreit"},
			"Steuerkategorie S darf keine",
		},
		{
			"AE ohne Befreiungsbegruendung",
			CIITaxBreakdown{CategoryCode: CIITaxCategoryReverseCharge},
			"Steuerkategorie AE erfordert",
		},
		{
			"E ohne Befreiungsbegruendung",
			CIITaxBreakdown{CategoryCode: CIITaxCategoryExempt},
			"Steuerkategorie E erfordert",
		},
		{
			"unbekannte Kategorie",
			CIITaxBreakdown{CategoryCode: "X"},
			"unbekannte Steuerkategorie",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inv := ciiTestInvoice()
			inv.TaxBreakdown = []CIITaxBreakdown{c.tax}

			if _, err := BuildCrossIndustryInvoice(inv); err == nil {
				t.Fatal("erwartet Fehler, aber es wurde ein Dokument erzeugt")
			} else if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("Fehlermeldung %q nennt %q nicht", err.Error(), c.wantSub)
			}
		})
	}
}

// TestBuildCIIAcceptsExemptWithReason beweist die Gegenrichtung: E mit
// Begruendung ist gueltig.
func TestBuildCIIAcceptsExemptWithReason(t *testing.T) {
	inv := ciiTestInvoice()
	inv.TaxBreakdown = []CIITaxBreakdown{{
		CategoryCode:    CIITaxCategoryExempt,
		RatePercent:     0,
		BasisAmount:     4328,
		ExemptionReason: "Steuerbefreit",
	}}
	for i := range inv.Lines {
		inv.Lines[i].TaxCategoryCode = CIITaxCategoryExempt
		inv.Lines[i].TaxRatePercent = 0
	}

	got := buildCIIForTest(t, inv)

	if !strings.Contains(got, `<ram:CategoryCode>E</ram:CategoryCode>`) {
		t.Error("Steuerkategorie E fehlt")
	}
	if !strings.Contains(got, `<ram:ExemptionReason>Steuerbefreit</ram:ExemptionReason>`) {
		t.Error("Befreiungsbegruendung fehlt")
	}
}
