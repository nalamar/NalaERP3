package accounting

// Unit-Tests fuer den CII-Eingangsparser (Backlog E.5.2). Vollstaendig
// DB-los - der Parser hat per Design keinen Datenbankzugriff.
//
// Die Testdokumente sind hier selbst geschrieben (nicht aus der
// KoSIT-Testsuite kopiert), damit gezielt Randfaelle konstruiert werden
// koennen und keine fremden Dateien ins Repo wandern. Die Struktur folgt
// der in ADR 0022/0023 gegen die KoSIT-Beispiele verifizierten Form.

import (
	"strings"
	"testing"
	"time"
)

// ciiInboundSample ist eine vollstaendige CII-Rechnung mit den ueblichen
// Praefixen.
const ciiInboundSample = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
    <rsm:ExchangedDocument>
        <ram:ID>LIEF-2026-0815</ram:ID>
        <ram:TypeCode>380</ram:TypeCode>
        <ram:IssueDateTime>
            <udt:DateTimeString format="102">20260518</udt:DateTimeString>
        </ram:IssueDateTime>
    </rsm:ExchangedDocument>
    <rsm:SupplyChainTradeTransaction>
        <ram:IncludedSupplyChainTradeLineItem>
            <ram:AssociatedDocumentLineDocument>
                <ram:LineID>1</ram:LineID>
            </ram:AssociatedDocumentLineDocument>
            <ram:SpecifiedTradeProduct>
                <ram:Name>Aluminiumprofil MB-70</ram:Name>
            </ram:SpecifiedTradeProduct>
            <ram:SpecifiedLineTradeAgreement>
                <ram:NetPriceProductTradePrice>
                    <ram:ChargeAmount>45.50</ram:ChargeAmount>
                </ram:NetPriceProductTradePrice>
            </ram:SpecifiedLineTradeAgreement>
            <ram:SpecifiedLineTradeDelivery>
                <ram:BilledQuantity unitCode="MTR">12</ram:BilledQuantity>
            </ram:SpecifiedLineTradeDelivery>
            <ram:SpecifiedLineTradeSettlement>
                <ram:ApplicableTradeTax>
                    <ram:TypeCode>VAT</ram:TypeCode>
                    <ram:CategoryCode>S</ram:CategoryCode>
                    <ram:RateApplicablePercent>19.00</ram:RateApplicablePercent>
                </ram:ApplicableTradeTax>
                <ram:SpecifiedTradeSettlementLineMonetarySummation>
                    <ram:LineTotalAmount>546.00</ram:LineTotalAmount>
                </ram:SpecifiedTradeSettlementLineMonetarySummation>
            </ram:SpecifiedLineTradeSettlement>
        </ram:IncludedSupplyChainTradeLineItem>
        <ram:ApplicableHeaderTradeAgreement>
            <ram:BuyerReference>BESTELL-4711</ram:BuyerReference>
            <ram:SellerTradeParty>
                <ram:Name>Profilwerk Süd GmbH</ram:Name>
                <ram:PostalTradeAddress>
                    <ram:PostcodeCode>70173</ram:PostcodeCode>
                    <ram:LineOne>Industriestraße 12</ram:LineOne>
                    <ram:CityName>Stuttgart</ram:CityName>
                    <ram:CountryID>DE</ram:CountryID>
                </ram:PostalTradeAddress>
                <ram:URIUniversalCommunication>
                    <ram:URIID schemeID="EM">rechnung@profilwerk.example</ram:URIID>
                </ram:URIUniversalCommunication>
                <ram:SpecifiedTaxRegistration>
                    <ram:ID schemeID="FC">99/123/45678</ram:ID>
                </ram:SpecifiedTaxRegistration>
                <ram:SpecifiedTaxRegistration>
                    <ram:ID schemeID="VA">DE555666777</ram:ID>
                </ram:SpecifiedTaxRegistration>
            </ram:SellerTradeParty>
            <ram:BuyerTradeParty>
                <ram:Name>Metallbau Muster GmbH</ram:Name>
                <ram:PostalTradeAddress>
                    <ram:PostcodeCode>40213</ram:PostcodeCode>
                    <ram:LineOne>Werkstrasse 4</ram:LineOne>
                    <ram:CityName>Duesseldorf</ram:CityName>
                    <ram:CountryID>DE</ram:CountryID>
                </ram:PostalTradeAddress>
            </ram:BuyerTradeParty>
        </ram:ApplicableHeaderTradeAgreement>
        <ram:ApplicableHeaderTradeDelivery/>
        <ram:ApplicableHeaderTradeSettlement>
            <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
            <ram:ApplicableTradeTax>
                <ram:CalculatedAmount>103.74</ram:CalculatedAmount>
                <ram:TypeCode>VAT</ram:TypeCode>
                <ram:BasisAmount>546.00</ram:BasisAmount>
                <ram:CategoryCode>S</ram:CategoryCode>
                <ram:RateApplicablePercent>19.00</ram:RateApplicablePercent>
            </ram:ApplicableTradeTax>
            <ram:SpecifiedTradePaymentTerms>
                <ram:Description>30 Tage netto</ram:Description>
                <ram:DueDateDateTime>
                    <udt:DateTimeString format="102">20260617</udt:DateTimeString>
                </ram:DueDateDateTime>
            </ram:SpecifiedTradePaymentTerms>
            <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
                <ram:LineTotalAmount>546.00</ram:LineTotalAmount>
                <ram:TaxBasisTotalAmount>546.00</ram:TaxBasisTotalAmount>
                <ram:TaxTotalAmount currencyID="EUR">103.74</ram:TaxTotalAmount>
                <ram:GrandTotalAmount>649.74</ram:GrandTotalAmount>
                <ram:DuePayableAmount>649.74</ram:DuePayableAmount>
            </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        </ram:ApplicableHeaderTradeSettlement>
    </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`

func TestParseCIIInvoiceReadsAllFields(t *testing.T) {
	got, err := ParseCIIInvoice([]byte(ciiInboundSample))
	if err != nil {
		t.Fatalf("ParseCIIInvoice: %v", err)
	}

	if got.Format != EInvoiceFormatCII {
		t.Errorf("Format = %q, erwartet %q", got.Format, EInvoiceFormatCII)
	}
	if got.Number != "LIEF-2026-0815" {
		t.Errorf("Rechnungsnummer = %q", got.Number)
	}
	if got.TypeCode != "380" {
		t.Errorf("Typ-Code = %q", got.TypeCode)
	}
	if !got.IssueDate.Equal(time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Rechnungsdatum = %v", got.IssueDate)
	}
	if got.DueDate == nil || !got.DueDate.Equal(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Fälligkeitsdatum = %v", got.DueDate)
	}
	if got.Currency != "EUR" {
		t.Errorf("Währung = %q", got.Currency)
	}
	if got.BuyerReference != "BESTELL-4711" {
		t.Errorf("Käuferreferenz = %q", got.BuyerReference)
	}

	if got.Seller.Name != "Profilwerk Süd GmbH" || got.Seller.City != "Stuttgart" ||
		got.Seller.PostalCode != "70173" || got.Seller.Street != "Industriestraße 12" {
		t.Errorf("Verkäufer falsch gelesen: %+v", got.Seller)
	}
	if got.Seller.VatID != "DE555666777" {
		t.Errorf("USt-IdNr. (schemeID VA) = %q", got.Seller.VatID)
	}
	if got.Seller.TaxNo != "99/123/45678" {
		t.Errorf("Steuernummer (schemeID FC) = %q", got.Seller.TaxNo)
	}
	if got.Seller.Email != "rechnung@profilwerk.example" {
		t.Errorf("E-Mail = %q", got.Seller.Email)
	}
	if got.Buyer.Name != "Metallbau Muster GmbH" || got.Buyer.City != "Duesseldorf" {
		t.Errorf("Käufer falsch gelesen: %+v", got.Buyer)
	}

	if len(got.Lines) != 1 {
		t.Fatalf("erwartet 1 Position, got %d", len(got.Lines))
	}
	l := got.Lines[0]
	if l.LineID != "1" || l.Name != "Aluminiumprofil MB-70" || l.Qty != 12 ||
		l.UnitCode != "MTR" || l.NetUnitPrice != 45.50 || l.LineTotalAmount != 546.00 ||
		l.TaxCategoryCode != "S" || l.TaxRatePercent != 19 {
		t.Errorf("Position falsch gelesen: %+v", l)
	}

	if len(got.TaxBreakdown) != 1 {
		t.Fatalf("erwartet 1 Steuergruppe, got %d", len(got.TaxBreakdown))
	}
	tb := got.TaxBreakdown[0]
	if tb.CategoryCode != "S" || tb.RatePercent != 19 || tb.BasisAmount != 546 || tb.CalculatedAmount != 103.74 {
		t.Errorf("Steuergruppe falsch gelesen: %+v", tb)
	}

	if got.LineTotalAmount != 546 || got.TaxBasisTotalAmount != 546 ||
		got.TaxTotalAmount != 103.74 || got.GrandTotalAmount != 649.74 || got.DuePayableAmount != 649.74 {
		t.Errorf("Summen falsch gelesen: %+v", got)
	}

	if len(got.Hinweise) != 0 {
		t.Errorf("eine in sich stimmige Rechnung darf keine Hinweise erzeugen: %v", got.Hinweise)
	}
}

// TestParseCIIInvoiceIsPrefixIndependent ist der wichtigste Test dieser
// Subtask: er beweist die Kernbehauptung aus ADR 0023, dass beim Lesen
// ueber echte Namensraeume und NICHT ueber Praefixe gematcht wird. Ein
// Absender darf beliebige Praefixe waehlen - hier bewusst "a"/"b"/"c"
// statt "rsm"/"ram"/"udt", und zusaetzlich ein Default-Namensraum ohne
// jedes Praefix.
func TestParseCIIInvoiceIsPrefixIndependent(t *testing.T) {
	// Praefixe UND ihre Deklarationen umbenennen - sonst waere das
	// Praefix gar nicht deklariert und der Test wuerde etwas anderes
	// pruefen als beabsichtigt.
	weird := strings.NewReplacer(
		"xmlns:rsm=", "xmlns:a=",
		"xmlns:ram=", "xmlns:b=",
		"xmlns:udt=", "xmlns:c=",
		"rsm:", "a:",
		"ram:", "b:",
		"udt:", "c:",
	).Replace(ciiInboundSample)

	got, err := ParseCIIInvoice([]byte(weird))
	if err != nil {
		t.Fatalf("Dokument mit anderen Präfixen muss lesbar sein: %v", err)
	}
	if got.Number != "LIEF-2026-0815" || got.Seller.VatID != "DE555666777" || got.GrandTotalAmount != 649.74 {
		t.Errorf("Inhalt bei geänderten Präfixen nicht korrekt gelesen: %+v", got)
	}

	// Gegenprobe ohne jedes Präfix: alle Elemente im Default-Namensraum.
	// Damit ist ausgeschlossen, dass das Matching heimlich doch am
	// Präfixnamen hängt.
	noPrefix := `<?xml version="1.0" encoding="UTF-8"?>
<CrossIndustryInvoice xmlns="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100">
    <ExchangedDocument xmlns="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100">
        <ID xmlns="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">OHNE-PRAEFIX-1</ID>
        <TypeCode xmlns="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">380</TypeCode>
        <IssueDateTime xmlns="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">
            <DateTimeString xmlns="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100" format="102">20260101</DateTimeString>
        </IssueDateTime>
    </ExchangedDocument>
</CrossIndustryInvoice>`

	got2, err := ParseCIIInvoice([]byte(noPrefix))
	if err != nil {
		t.Fatalf("Dokument ohne Präfixe muss lesbar sein: %v", err)
	}
	if got2.Number != "OHNE-PRAEFIX-1" {
		t.Errorf("Rechnungsnummer ohne Präfixe = %q", got2.Number)
	}
	if !got2.IssueDate.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Datum ohne Präfixe = %v", got2.IssueDate)
	}
}

// TestParseCIIInvoiceReadsOwnOutput schliesst den Kreis zum Ausgang: eine
// mit BuildCrossIndustryInvoice (E.4.3.1) erzeugte Rechnung muss vom
// Eingangsparser wieder vollstaendig gelesen werden koennen. Das prueft
// Schreiber und Leser gegeneinander statt beide nur gegen die eigene
// Erwartung.
func TestParseCIIInvoiceReadsOwnOutput(t *testing.T) {
	written, err := BuildCrossIndustryInvoice(ciiTestInvoice())
	if err != nil {
		t.Fatalf("BuildCrossIndustryInvoice: %v", err)
	}

	got, err := ParseCIIInvoice(written)
	if err != nil {
		t.Fatalf("eigene Ausgabe ist nicht wieder lesbar: %v", err)
	}

	want := ciiTestInvoice()
	if got.Number != want.Number {
		t.Errorf("Rechnungsnummer: got %q, want %q", got.Number, want.Number)
	}
	if got.BuyerReference != want.BuyerReference {
		t.Errorf("Käuferreferenz: got %q, want %q", got.BuyerReference, want.BuyerReference)
	}
	if !got.IssueDate.Equal(want.IssueDate) {
		t.Errorf("Rechnungsdatum: got %v, want %v", got.IssueDate, want.IssueDate)
	}
	if got.DueDate == nil || !got.DueDate.Equal(*want.DueDate) {
		t.Errorf("Fälligkeitsdatum: got %v, want %v", got.DueDate, want.DueDate)
	}
	if got.Seller.Name != want.Seller.Name || got.Seller.VatID != want.Seller.VatID || got.Seller.TaxNo != want.Seller.TaxNo {
		t.Errorf("Verkäufer: got %+v", got.Seller)
	}
	if got.Buyer.Name != want.Buyer.Name || got.Buyer.City != want.Buyer.City {
		t.Errorf("Käufer: got %+v", got.Buyer)
	}
	if len(got.Lines) != len(want.Lines) {
		t.Fatalf("Positionen: got %d, want %d", len(got.Lines), len(want.Lines))
	}
	for i := range want.Lines {
		if got.Lines[i].Name != want.Lines[i].Name || got.Lines[i].LineTotalAmount != want.Lines[i].LineTotalAmount {
			t.Errorf("Position %d: got %+v, want %+v", i+1, got.Lines[i], want.Lines[i])
		}
	}
	if got.GrandTotalAmount != want.GrandTotalAmount {
		t.Errorf("Bruttobetrag: got %v, want %v", got.GrandTotalAmount, want.GrandTotalAmount)
	}
	if len(got.Hinweise) != 0 {
		t.Errorf("die eigene Ausgabe darf keine Plausibilitätshinweise erzeugen: %v", got.Hinweise)
	}
}

// TestParseCIIInvoiceRejectsForeignSyntax beweist, dass ein UBL-Dokument
// nicht stillschweigend als leere CII-Rechnung durchgeht. Die Erkennung
// laeuft ueber den Wurzelelement-Namensraum (ADR 0023).
func TestParseCIIInvoiceRejectsForeignSyntax(t *testing.T) {
	ubl := `<?xml version="1.0" encoding="UTF-8"?>
<ubl:Invoice xmlns:ubl="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
             xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
    <cbc:ID>UBL-1</cbc:ID>
</ubl:Invoice>`

	_, err := ParseCIIInvoice([]byte(ubl))
	if err == nil {
		t.Fatal("ein UBL-Dokument darf nicht als CII akzeptiert werden")
	}
	if !strings.Contains(err.Error(), "kein CII-Dokument") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
	}
}

func TestParseCIIInvoiceRejectsBrokenInput(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantSub string
	}{
		{"kein XML", "das ist kein XML", "nicht gelesen werden"},
		{"abgeschnittenes XML", `<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100">`, "nicht gelesen werden"},
		{"leeres Dokument", "", "nicht gelesen werden"},
		{
			"fremdes Wurzelelement",
			`<?xml version="1.0"?><html><body>Rechnung</body></html>`,
			"kein CII-Dokument",
		},
		{
			"ohne Rechnungsnummer",
			strings.Replace(ciiInboundSample, "<ram:ID>LIEF-2026-0815</ram:ID>", "<ram:ID></ram:ID>", 1),
			"Rechnungsnummer fehlt",
		},
		{
			"ohne Rechnungsdatum",
			strings.Replace(ciiInboundSample, `<udt:DateTimeString format="102">20260518</udt:DateTimeString>`, "", 1),
			"Rechnungsdatum",
		},
		{
			"unlesbares Datum",
			strings.Replace(ciiInboundSample, ">20260518<", ">18.05.2026<", 1),
			"unlesbar",
		},
		{
			"nicht unterstütztes Datumsformat",
			strings.Replace(ciiInboundSample, `format="102">20260518`, `format="610">202605`, 1),
			"nicht unterstütztes Datumsformat",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseCIIInvoice([]byte(c.input))
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

// TestParseCIIInvoiceDoesNotResolveExternalEntities weist nach, was
// ADR 0023 fordert: eingehendes XML stammt von Dritten, externe Entitäten
// dürfen NICHT aufgelöst werden (XXE). Der Test belegt das Verhalten,
// statt es zu behaupten.
func TestParseCIIInvoiceDoesNotResolveExternalEntities(t *testing.T) {
	xxe := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [ <!ENTITY xxe SYSTEM "file:///etc/passwd"> ]>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
    <rsm:ExchangedDocument>
        <ram:ID>&xxe;</ram:ID>
        <ram:TypeCode>380</ram:TypeCode>
        <ram:IssueDateTime>
            <udt:DateTimeString format="102">20260518</udt:DateTimeString>
        </ram:IssueDateTime>
    </rsm:ExchangedDocument>
</rsm:CrossIndustryInvoice>`

	got, err := ParseCIIInvoice([]byte(xxe))

	// Zulaessig sind genau zwei Ausgaenge: ein Fehler (Entitaet unbekannt)
	// oder ein Ergebnis, in dem die Entitaet NICHT aufgeloest wurde. Ein
	// Dateiinhalt darf unter keinen Umstaenden im Ergebnis landen.
	if err != nil {
		return
	}
	if strings.Contains(got.Number, "root:") || strings.Contains(got.Number, "/bin/") {
		t.Fatalf("externe Entität wurde aufgelöst - Dateiinhalt im Ergebnis: %q", got.Number)
	}
}

// TestParseCIIInvoiceReportsImplausibleSums beweist die Entscheidung aus
// ADR 0023: abweichende Summen werden GEMELDET, aber nicht korrigiert -
// es ist die Rechnung des Absenders.
func TestParseCIIInvoiceReportsImplausibleSums(t *testing.T) {
	broken := strings.Replace(ciiInboundSample,
		"<ram:GrandTotalAmount>649.74</ram:GrandTotalAmount>",
		"<ram:GrandTotalAmount>999.99</ram:GrandTotalAmount>", 1)

	got, err := ParseCIIInvoice([]byte(broken))
	if err != nil {
		t.Fatalf("eine unplausible Summe darf kein Parse-Fehler sein: %v", err)
	}
	if got.GrandTotalAmount != 999.99 {
		t.Errorf("der gelieferte Wert muss unverändert übernommen werden, got %v", got.GrandTotalAmount)
	}
	if len(got.Hinweise) == 0 {
		t.Fatal("erwartet einen Hinweis auf die abweichende Summe")
	}
	if !strings.Contains(strings.Join(got.Hinweise, " "), "999.99") {
		t.Errorf("Hinweis nennt den abweichenden Betrag nicht: %v", got.Hinweise)
	}
}

func TestParseCIIInvoiceReportsMissingSellerIdentification(t *testing.T) {
	ohneSteuerIDs := strings.Replace(ciiInboundSample,
		`<ram:SpecifiedTaxRegistration>
                    <ram:ID schemeID="FC">99/123/45678</ram:ID>
                </ram:SpecifiedTaxRegistration>
                <ram:SpecifiedTaxRegistration>
                    <ram:ID schemeID="VA">DE555666777</ram:ID>
                </ram:SpecifiedTaxRegistration>`, "", 1)

	got, err := ParseCIIInvoice([]byte(ohneSteuerIDs))
	if err != nil {
		t.Fatalf("fehlende Steuerkennung darf kein Parse-Fehler sein: %v", err)
	}
	if got.Seller.VatID != "" || got.Seller.TaxNo != "" {
		t.Errorf("erwartet leere Steuerkennungen, got %+v", got.Seller)
	}
	if !strings.Contains(strings.Join(got.Hinweise, " "), "Lieferantenzuordnung") {
		t.Errorf("erwartet Hinweis auf die erschwerte Lieferantenzuordnung, got %v", got.Hinweise)
	}
}

func TestParseCIIInvoiceReportsMissingLines(t *testing.T) {
	ohnePositionen := strings.Replace(ciiInboundSample,
		ciiInboundSample[strings.Index(ciiInboundSample, "<ram:IncludedSupplyChainTradeLineItem>"):strings.Index(ciiInboundSample, "</ram:IncludedSupplyChainTradeLineItem>")+len("</ram:IncludedSupplyChainTradeLineItem>")],
		"", 1)

	got, err := ParseCIIInvoice([]byte(ohnePositionen))
	if err != nil {
		t.Fatalf("eine Rechnung ohne Positionen darf kein Parse-Fehler sein: %v", err)
	}
	if len(got.Lines) != 0 {
		t.Fatalf("erwartet keine Positionen, got %d", len(got.Lines))
	}
	if !strings.Contains(strings.Join(got.Hinweise, " "), "keine Positionen") {
		t.Errorf("erwartet Hinweis auf fehlende Positionen, got %v", got.Hinweise)
	}
}

// TestParseCIIInvoiceToleratesMissingOptionalFields beweist die
// Grundhaltung des Parsers: an einem fehlenden Kann-Feld darf eine fremde
// Rechnung nicht scheitern.
func TestParseCIIInvoiceToleratesMissingOptionalFields(t *testing.T) {
	minimal := `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
                          xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
                          xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
    <rsm:ExchangedDocument>
        <ram:ID>MINIMAL-1</ram:ID>
        <ram:IssueDateTime>
            <udt:DateTimeString format="102">20260301</udt:DateTimeString>
        </ram:IssueDateTime>
    </rsm:ExchangedDocument>
</rsm:CrossIndustryInvoice>`

	got, err := ParseCIIInvoice([]byte(minimal))
	if err != nil {
		t.Fatalf("minimales Dokument muss lesbar sein: %v", err)
	}
	if got.Number != "MINIMAL-1" {
		t.Errorf("Rechnungsnummer = %q", got.Number)
	}
	if got.DueDate != nil {
		t.Errorf("ohne Zahlungsbedingung darf kein Fälligkeitsdatum entstehen: %v", got.DueDate)
	}
	if got.TypeCode != "" || got.Currency != "" {
		t.Errorf("fehlende Felder dürfen nicht erfunden werden: TypeCode=%q Currency=%q", got.TypeCode, got.Currency)
	}
	if len(got.Hinweise) == 0 {
		t.Error("erwartet Hinweise auf die fehlenden Angaben")
	}
}
