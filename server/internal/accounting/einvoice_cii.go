package accounting

// E-Rechnung Ausgang, CII-Serialisierung (ADR 0022, Backlog E.4.3.1).
//
// Baut aus einem bereits vollstaendig aufgeloesten Eingabemodell eine
// UN/CEFACT-Cross-Industry-Invoice (CII) im XRechnung-3.0-Profil. Bewusst
// OHNE Datenbankzugriff: das Befuellen des Eingabemodells aus invoices_out/
// contacts/company_profiles ist Aufgabe von E.4.3.2, das HTTP-Wiring von
// E.4.3.3 (XRechnung) bzw. E.4.3.4 (ZUGFeRD). Dieselbe Ausgabe dient beiden
// Wegen - als eigenstaendige .xml (XRechnung) und als PDF-Anhang
// factur-x.xml (ZUGFeRD), siehe ADR 0022.
//
// Struktur, Elementnamen, Reihenfolge und Attribute stammen NICHT aus der
// Erinnerung, sondern wurden gegen die offizielle KoSIT-Testsuite
// (itplr-kosit/xrechnung-testsuite, Geschaeftsfaelle 01.01a und 01.21a in
// uncefact-Syntax) geprueft. Die Reihenfolge der Kindelemente ist in CII
// durch eine XSD-<sequence> festgelegt und daher NICHT beliebig - jede
// Umsortierung macht das Dokument schemaungueltig.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CII-Konstanten aus der recherchierten KoSIT-Beispieldatei (ADR 0022).
const (
	ciiGuidelineID = "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0"
	ciiProfileID   = "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0"

	ciiNamespaceRSM = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
	ciiNamespaceRAM = "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
	ciiNamespaceQDT = "urn:un:unece:uncefact:data:standard:QualifiedDataType:100"
	ciiNamespaceUDT = "urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100"

	// UNTDID 1001 (BT-3), Zuordnung laut ADR 0022.
	CIITypeCodeCommercialInvoice = "380"
	CIITypeCodePrepaymentInvoice = "386"

	// UNTDID 5305 (BT-118), Zuordnung laut ADR 0022.
	CIITaxCategoryStandard      = "S"
	CIITaxCategoryReverseCharge = "AE"
	CIITaxCategoryExempt        = "E"

	// UN/ECE Rec. 20: "Stueck/nicht naeher spezifizierte Einheit". Fester
	// Fallback fuer alle Positionen, da invoice_out_items keine Einheit
	// traegt (bewusste, in ADR 0022 offengelegte Vereinfachung).
	CIIDefaultUnitCode = "C62"

	// UNTDID 4461: SEPA-Ueberweisung. Wird nur gesetzt, wenn eine IBAN
	// vorliegt - ohne Zahlungsweg wird der gesamte Block weggelassen,
	// statt einen Zahlungsweg zu behaupten, den es nicht gibt.
	ciiPaymentMeansSEPACreditTransfer = "58"

	// udt:DateTimeString-Formatkennung fuer JJJJMMTT (CCYYMMDD).
	ciiDateFormat102 = "102"
)

// CIIParty ist eine Geschaeftspartei (Verkaeufer oder Kaeufer) mit der fuer
// EN 16931 erforderlichen Postanschrift (BG-5 bzw. BG-8).
type CIIParty struct {
	ID         string
	Name       string
	Street     string
	PostalCode string
	City       string
	// Country ist der ISO-3166-1-alpha-2-Code (BT-40/BT-55). Leer =>
	// Fallback "DE", da das Feld in EN 16931 Pflicht ist und der
	// Datenbestand durchgaengig deutsch ist.
	Country string
	Email   string
	Phone   string
	// VatID (BT-31/BT-48) und TaxNo (BT-32) werden als
	// SpecifiedTaxRegistration mit schemeID "VA" bzw. "FC" ausgegeben.
	VatID string
	TaxNo string
}

// CIITaxBreakdown ist eine Zeile der Steueraufschluesselung (BG-23), also
// genau eine Kombination aus Steuerkategorie und Steuersatz.
type CIITaxBreakdown struct {
	CategoryCode     string
	RatePercent      float64
	BasisAmount      float64
	CalculatedAmount float64
	// ExemptionReason (BT-120) ist fuer die Kategorien AE und E laut
	// EN 16931 erforderlich und fuer S unzulaessig.
	ExemptionReason string
}

// CIILineItem ist eine Rechnungsposition (BG-25).
type CIILineItem struct {
	LineID          string
	Name            string
	Qty             float64
	UnitCode        string
	NetUnitPrice    float64
	LineTotalAmount float64
	TaxCategoryCode string
	TaxRatePercent  float64
}

// CIIInvoice ist das vollstaendig aufgeloeste Eingabemodell. Alle Betraege
// sind bereits berechnet - dieser Baustein rechnet bewusst nichts nach und
// erfindet nichts, er serialisiert nur.
type CIIInvoice struct {
	Number         string
	TypeCode       string
	IssueDate      time.Time
	DueDate        *time.Time
	Currency       string
	BuyerReference string
	Note           string

	Seller CIIParty
	Buyer  CIIParty

	// Zahlungsweg (optional, siehe ciiPaymentMeansSEPACreditTransfer).
	IBAN          string
	BIC           string
	AccountHolder string

	PaymentTerms string

	Lines        []CIILineItem
	TaxBreakdown []CIITaxBreakdown

	LineTotalAmount     float64
	TaxBasisTotalAmount float64
	TaxTotalAmount      float64
	GrandTotalAmount    float64
	PaidAmount          float64
	DuePayableAmount    float64
}

// --- XML-Strukturen -------------------------------------------------------
//
// encoding/xml kennt keine Ausgabe von Namensraum-Praefixen. Bewaehrter und
// hier genutzter Weg: die Praefixe werden als feste Bestandteile der
// Elementnamen geschrieben und die xmlns-Deklarationen einmalig als
// Attribute am Wurzelelement gesetzt. Ergebnis ist exakt die Praefix-Form
// der offiziellen Beispieldateien.

type ciiDateTimeString struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type ciiDateTime struct {
	DateTimeString ciiDateTimeString `xml:"udt:DateTimeString"`
}

type ciiAmount struct {
	CurrencyID string `xml:"currencyID,attr,omitempty"`
	Value      string `xml:",chardata"`
}

type ciiSchemeID struct {
	SchemeID string `xml:"schemeID,attr,omitempty"`
	Value    string `xml:",chardata"`
}

type ciiQuantity struct {
	UnitCode string `xml:"unitCode,attr"`
	Value    string `xml:",chardata"`
}

type ciiDocumentContextParameter struct {
	ID string `xml:"ram:ID"`
}

type ciiExchangedDocumentContext struct {
	BusinessProcess *ciiDocumentContextParameter `xml:"ram:BusinessProcessSpecifiedDocumentContextParameter,omitempty"`
	Guideline       ciiDocumentContextParameter  `xml:"ram:GuidelineSpecifiedDocumentContextParameter"`
}

type ciiIncludedNote struct {
	Content string `xml:"ram:Content"`
}

type ciiExchangedDocument struct {
	ID            string           `xml:"ram:ID"`
	TypeCode      string           `xml:"ram:TypeCode"`
	IssueDateTime ciiDateTime      `xml:"ram:IssueDateTime"`
	IncludedNote  *ciiIncludedNote `xml:"ram:IncludedNote,omitempty"`
}

type ciiTradeAddress struct {
	PostcodeCode string `xml:"ram:PostcodeCode,omitempty"`
	LineOne      string `xml:"ram:LineOne,omitempty"`
	CityName     string `xml:"ram:CityName,omitempty"`
	CountryID    string `xml:"ram:CountryID"`
}

type ciiUniversalCommunication struct {
	URIID ciiSchemeID `xml:"ram:URIID"`
}

type ciiPhoneCommunication struct {
	CompleteNumber string `xml:"ram:CompleteNumber"`
}

type ciiEmailCommunication struct {
	URIID string `xml:"ram:URIID"`
}

type ciiTradeContact struct {
	PersonName string                 `xml:"ram:PersonName,omitempty"`
	Phone      *ciiPhoneCommunication `xml:"ram:TelephoneUniversalCommunication,omitempty"`
	Email      *ciiEmailCommunication `xml:"ram:EmailURIUniversalCommunication,omitempty"`
}

type ciiTaxRegistration struct {
	ID ciiSchemeID `xml:"ram:ID"`
}

type ciiTradeParty struct {
	ID               string                     `xml:"ram:ID,omitempty"`
	Name             string                     `xml:"ram:Name"`
	DefinedContact   *ciiTradeContact           `xml:"ram:DefinedTradeContact,omitempty"`
	PostalAddress    ciiTradeAddress            `xml:"ram:PostalTradeAddress"`
	URICommunication *ciiUniversalCommunication `xml:"ram:URIUniversalCommunication,omitempty"`
	TaxRegistrations []ciiTaxRegistration       `xml:"ram:SpecifiedTaxRegistration,omitempty"`
}

type ciiHeaderTradeAgreement struct {
	BuyerReference string        `xml:"ram:BuyerReference,omitempty"`
	SellerParty    ciiTradeParty `xml:"ram:SellerTradeParty"`
	BuyerParty     ciiTradeParty `xml:"ram:BuyerTradeParty"`
}

type ciiCreditorFinancialAccount struct {
	IBANID      string `xml:"ram:IBANID"`
	AccountName string `xml:"ram:AccountName,omitempty"`
}

type ciiCreditorFinancialInstitution struct {
	BICID string `xml:"ram:BICID"`
}

type ciiPaymentMeans struct {
	TypeCode              string                           `xml:"ram:TypeCode"`
	CreditorAccount       *ciiCreditorFinancialAccount     `xml:"ram:PayeePartyCreditorFinancialAccount,omitempty"`
	CreditorFinancialInst *ciiCreditorFinancialInstitution `xml:"ram:PayeeSpecifiedCreditorFinancialInstitution,omitempty"`
}

// ciiHeaderTradeTax bildet die Steueraufschluesselung auf Belegebene ab.
// Feldreihenfolge exakt wie in der KoSIT-Beispieldatei 01.21a:
// CalculatedAmount, TypeCode, ExemptionReason, BasisAmount, CategoryCode,
// RateApplicablePercent.
type ciiHeaderTradeTax struct {
	CalculatedAmount      string `xml:"ram:CalculatedAmount"`
	TypeCode              string `xml:"ram:TypeCode"`
	ExemptionReason       string `xml:"ram:ExemptionReason,omitempty"`
	BasisAmount           string `xml:"ram:BasisAmount"`
	CategoryCode          string `xml:"ram:CategoryCode"`
	RateApplicablePercent string `xml:"ram:RateApplicablePercent"`
}

type ciiPaymentTerms struct {
	Description string       `xml:"ram:Description,omitempty"`
	DueDate     *ciiDateTime `xml:"ram:DueDateDateTime,omitempty"`
}

type ciiHeaderMonetarySummation struct {
	LineTotalAmount     string    `xml:"ram:LineTotalAmount"`
	TaxBasisTotalAmount string    `xml:"ram:TaxBasisTotalAmount"`
	TaxTotalAmount      ciiAmount `xml:"ram:TaxTotalAmount"`
	GrandTotalAmount    string    `xml:"ram:GrandTotalAmount"`
	TotalPrepaidAmount  string    `xml:"ram:TotalPrepaidAmount,omitempty"`
	DuePayableAmount    string    `xml:"ram:DuePayableAmount"`
}

type ciiHeaderTradeSettlement struct {
	InvoiceCurrencyCode string                     `xml:"ram:InvoiceCurrencyCode"`
	PaymentMeans        *ciiPaymentMeans           `xml:"ram:SpecifiedTradeSettlementPaymentMeans,omitempty"`
	TradeTax            []ciiHeaderTradeTax        `xml:"ram:ApplicableTradeTax"`
	PaymentTerms        *ciiPaymentTerms           `xml:"ram:SpecifiedTradePaymentTerms,omitempty"`
	MonetarySummation   ciiHeaderMonetarySummation `xml:"ram:SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type ciiLineDocument struct {
	LineID string `xml:"ram:LineID"`
}

type ciiTradeProduct struct {
	Name string `xml:"ram:Name"`
}

type ciiTradePrice struct {
	ChargeAmount string `xml:"ram:ChargeAmount"`
}

type ciiLineTradeAgreement struct {
	NetPrice ciiTradePrice `xml:"ram:NetPriceProductTradePrice"`
}

type ciiLineTradeDelivery struct {
	BilledQuantity ciiQuantity `xml:"ram:BilledQuantity"`
}

type ciiLineTradeTax struct {
	TypeCode              string `xml:"ram:TypeCode"`
	CategoryCode          string `xml:"ram:CategoryCode"`
	RateApplicablePercent string `xml:"ram:RateApplicablePercent"`
}

type ciiLineMonetarySummation struct {
	LineTotalAmount string `xml:"ram:LineTotalAmount"`
}

type ciiLineTradeSettlement struct {
	TradeTax          ciiLineTradeTax          `xml:"ram:ApplicableTradeTax"`
	MonetarySummation ciiLineMonetarySummation `xml:"ram:SpecifiedTradeSettlementLineMonetarySummation"`
}

type ciiTradeLineItem struct {
	LineDocument   ciiLineDocument        `xml:"ram:AssociatedDocumentLineDocument"`
	Product        ciiTradeProduct        `xml:"ram:SpecifiedTradeProduct"`
	LineAgreement  ciiLineTradeAgreement  `xml:"ram:SpecifiedLineTradeAgreement"`
	LineDelivery   ciiLineTradeDelivery   `xml:"ram:SpecifiedLineTradeDelivery"`
	LineSettlement ciiLineTradeSettlement `xml:"ram:SpecifiedLineTradeSettlement"`
}

type ciiSupplyChainTradeTransaction struct {
	LineItems  []ciiTradeLineItem       `xml:"ram:IncludedSupplyChainTradeLineItem"`
	Agreement  ciiHeaderTradeAgreement  `xml:"ram:ApplicableHeaderTradeAgreement"`
	Delivery   struct{}                 `xml:"ram:ApplicableHeaderTradeDelivery"`
	Settlement ciiHeaderTradeSettlement `xml:"ram:ApplicableHeaderTradeSettlement"`
}

type ciiCrossIndustryInvoice struct {
	XMLName xml.Name `xml:"rsm:CrossIndustryInvoice"`

	XmlnsRSM string `xml:"xmlns:rsm,attr"`
	XmlnsRAM string `xml:"xmlns:ram,attr"`
	XmlnsQDT string `xml:"xmlns:qdt,attr"`
	XmlnsUDT string `xml:"xmlns:udt,attr"`

	Context     ciiExchangedDocumentContext    `xml:"rsm:ExchangedDocumentContext"`
	Document    ciiExchangedDocument           `xml:"rsm:ExchangedDocument"`
	Transaction ciiSupplyChainTradeTransaction `xml:"rsm:SupplyChainTradeTransaction"`
}

// --- Formatierung ---------------------------------------------------------

// ciiAmountString formatiert einen Betrag mit genau zwei Nachkommastellen
// und Punkt als Dezimaltrennzeichen (XML-Schema xs:decimal - anders als im
// DATEV-Export, der Komma verlangt).
func ciiAmountString(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// ciiQuantityString formatiert eine Menge mit bis zu vier Nachkommastellen
// (Genauigkeit von invoice_out_items.qty) ohne ueberfluessige Nullen.
func ciiQuantityString(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

// ciiDateString formatiert ein Datum als CCYYMMDD (udt-Format 102).
func ciiDateString(t time.Time) string {
	return t.Format("20060102")
}

func ciiDateTimeOf(t time.Time) ciiDateTime {
	return ciiDateTime{DateTimeString: ciiDateTimeString{Format: ciiDateFormat102, Value: ciiDateString(t)}}
}

// --- Aufbau ---------------------------------------------------------------

// BuildCrossIndustryInvoice serialisiert eine Rechnung als CII-XML im
// XRechnung-3.0-Profil. Fehlende Pflichtangaben fuehren zu einem Fehler mit
// klarer Benennung des fehlenden Feldes - es wird kein Platzhalter erfunden,
// weil ein Dokument mit erfundenen Pflichtangaben beim Empfaenger als
// gueltige Rechnung gelten wuerde.
func BuildCrossIndustryInvoice(inv CIIInvoice) ([]byte, error) {
	if err := validateCIIInvoice(inv); err != nil {
		return nil, err
	}

	doc := ciiCrossIndustryInvoice{
		XmlnsRSM: ciiNamespaceRSM,
		XmlnsRAM: ciiNamespaceRAM,
		XmlnsQDT: ciiNamespaceQDT,
		XmlnsUDT: ciiNamespaceUDT,
	}

	doc.Context.BusinessProcess = &ciiDocumentContextParameter{ID: ciiProfileID}
	doc.Context.Guideline = ciiDocumentContextParameter{ID: ciiGuidelineID}

	doc.Document = ciiExchangedDocument{
		ID:            inv.Number,
		TypeCode:      inv.TypeCode,
		IssueDateTime: ciiDateTimeOf(inv.IssueDate),
	}
	if strings.TrimSpace(inv.Note) != "" {
		doc.Document.IncludedNote = &ciiIncludedNote{Content: inv.Note}
	}

	for _, l := range inv.Lines {
		unit := l.UnitCode
		if unit == "" {
			unit = CIIDefaultUnitCode
		}
		doc.Transaction.LineItems = append(doc.Transaction.LineItems, ciiTradeLineItem{
			LineDocument:  ciiLineDocument{LineID: l.LineID},
			Product:       ciiTradeProduct{Name: l.Name},
			LineAgreement: ciiLineTradeAgreement{NetPrice: ciiTradePrice{ChargeAmount: ciiAmountString(l.NetUnitPrice)}},
			LineDelivery: ciiLineTradeDelivery{BilledQuantity: ciiQuantity{
				UnitCode: unit,
				Value:    ciiQuantityString(l.Qty),
			}},
			LineSettlement: ciiLineTradeSettlement{
				TradeTax: ciiLineTradeTax{
					TypeCode:              "VAT",
					CategoryCode:          l.TaxCategoryCode,
					RateApplicablePercent: ciiAmountString(l.TaxRatePercent),
				},
				MonetarySummation: ciiLineMonetarySummation{LineTotalAmount: ciiAmountString(l.LineTotalAmount)},
			},
		})
	}

	doc.Transaction.Agreement = ciiHeaderTradeAgreement{
		BuyerReference: inv.BuyerReference,
		SellerParty:    ciiTradePartyOf(inv.Seller, true),
		BuyerParty:     ciiTradePartyOf(inv.Buyer, false),
	}

	doc.Transaction.Settlement = ciiHeaderTradeSettlement{
		InvoiceCurrencyCode: inv.Currency,
		MonetarySummation: ciiHeaderMonetarySummation{
			LineTotalAmount:     ciiAmountString(inv.LineTotalAmount),
			TaxBasisTotalAmount: ciiAmountString(inv.TaxBasisTotalAmount),
			TaxTotalAmount:      ciiAmount{CurrencyID: inv.Currency, Value: ciiAmountString(inv.TaxTotalAmount)},
			GrandTotalAmount:    ciiAmountString(inv.GrandTotalAmount),
			DuePayableAmount:    ciiAmountString(inv.DuePayableAmount),
		},
	}
	if inv.PaidAmount != 0 {
		doc.Transaction.Settlement.MonetarySummation.TotalPrepaidAmount = ciiAmountString(inv.PaidAmount)
	}

	if strings.TrimSpace(inv.IBAN) != "" {
		pm := &ciiPaymentMeans{
			TypeCode:        ciiPaymentMeansSEPACreditTransfer,
			CreditorAccount: &ciiCreditorFinancialAccount{IBANID: inv.IBAN, AccountName: inv.AccountHolder},
		}
		if strings.TrimSpace(inv.BIC) != "" {
			pm.CreditorFinancialInst = &ciiCreditorFinancialInstitution{BICID: inv.BIC}
		}
		doc.Transaction.Settlement.PaymentMeans = pm
	}

	for _, t := range inv.TaxBreakdown {
		doc.Transaction.Settlement.TradeTax = append(doc.Transaction.Settlement.TradeTax, ciiHeaderTradeTax{
			CalculatedAmount:      ciiAmountString(t.CalculatedAmount),
			TypeCode:              "VAT",
			ExemptionReason:       t.ExemptionReason,
			BasisAmount:           ciiAmountString(t.BasisAmount),
			CategoryCode:          t.CategoryCode,
			RateApplicablePercent: ciiAmountString(t.RatePercent),
		})
	}

	if inv.DueDate != nil || strings.TrimSpace(inv.PaymentTerms) != "" {
		terms := &ciiPaymentTerms{Description: inv.PaymentTerms}
		if inv.DueDate != nil {
			dt := ciiDateTimeOf(*inv.DueDate)
			terms.DueDate = &dt
		}
		doc.Transaction.Settlement.PaymentTerms = terms
	}

	body, err := xml.MarshalIndent(doc, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("CII-XML konnte nicht erzeugt werden: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.Write(body)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

// ciiTradePartyOf bildet eine Partei ab. Die Steuernummer (schemeID FC) wird
// nur fuer den Verkaeufer ausgegeben; beim Kaeufer ist ausschliesslich die
// USt-IdNr. (schemeID VA) vorgesehen.
func ciiTradePartyOf(p CIIParty, seller bool) ciiTradeParty {
	country := strings.ToUpper(strings.TrimSpace(p.Country))
	if country == "" {
		country = "DE"
	}
	out := ciiTradeParty{
		ID:   p.ID,
		Name: p.Name,
		PostalAddress: ciiTradeAddress{
			PostcodeCode: p.PostalCode,
			LineOne:      p.Street,
			CityName:     p.City,
			CountryID:    country,
		},
	}
	if strings.TrimSpace(p.Email) != "" {
		out.URICommunication = &ciiUniversalCommunication{URIID: ciiSchemeID{SchemeID: "EM", Value: p.Email}}
	}
	if seller {
		if strings.TrimSpace(p.Email) != "" || strings.TrimSpace(p.Phone) != "" {
			contact := &ciiTradeContact{}
			if strings.TrimSpace(p.Phone) != "" {
				contact.Phone = &ciiPhoneCommunication{CompleteNumber: p.Phone}
			}
			if strings.TrimSpace(p.Email) != "" {
				contact.Email = &ciiEmailCommunication{URIID: p.Email}
			}
			out.DefinedContact = contact
		}
		if strings.TrimSpace(p.TaxNo) != "" {
			out.TaxRegistrations = append(out.TaxRegistrations, ciiTaxRegistration{ID: ciiSchemeID{SchemeID: "FC", Value: p.TaxNo}})
		}
	}
	if strings.TrimSpace(p.VatID) != "" {
		out.TaxRegistrations = append(out.TaxRegistrations, ciiTaxRegistration{ID: ciiSchemeID{SchemeID: "VA", Value: p.VatID}})
	}
	return out
}

// validateCIIInvoice prueft die EN-16931-Pflichtangaben, die aus unserem
// Datenmodell stammen muessen. Bewusst hier und nicht erst beim Empfaenger:
// ein unvollstaendiges Dokument wuerde entweder abgelehnt oder - schlimmer -
// mit fehlenden Angaben akzeptiert.
func validateCIIInvoice(inv CIIInvoice) error {
	if strings.TrimSpace(inv.Number) == "" {
		return fmt.Errorf("E-Rechnung: Rechnungsnummer fehlt")
	}
	if strings.TrimSpace(inv.TypeCode) == "" {
		return fmt.Errorf("E-Rechnung: Rechnungstyp-Code fehlt")
	}
	if inv.IssueDate.IsZero() {
		return fmt.Errorf("E-Rechnung: Rechnungsdatum fehlt")
	}
	if strings.TrimSpace(inv.Currency) == "" {
		return fmt.Errorf("E-Rechnung: Waehrung fehlt")
	}
	if strings.TrimSpace(inv.BuyerReference) == "" {
		return fmt.Errorf("E-Rechnung: Kaeuferreferenz (Leitweg-ID bzw. Kundenreferenz) ist erforderlich")
	}
	if err := validateCIIParty("Verkaeufer", inv.Seller); err != nil {
		return err
	}
	if err := validateCIIParty("Kaeufer", inv.Buyer); err != nil {
		return err
	}
	if strings.TrimSpace(inv.Seller.VatID) == "" && strings.TrimSpace(inv.Seller.TaxNo) == "" {
		return fmt.Errorf("E-Rechnung: Verkaeufer benoetigt USt-IdNr. oder Steuernummer")
	}
	if len(inv.Lines) == 0 {
		return fmt.Errorf("E-Rechnung: mindestens eine Rechnungsposition ist erforderlich")
	}
	for i, l := range inv.Lines {
		if strings.TrimSpace(l.Name) == "" {
			return fmt.Errorf("E-Rechnung: Position %d hat keine Bezeichnung", i+1)
		}
		if strings.TrimSpace(l.TaxCategoryCode) == "" {
			return fmt.Errorf("E-Rechnung: Position %d hat keine Steuerkategorie", i+1)
		}
	}
	if len(inv.TaxBreakdown) == 0 {
		return fmt.Errorf("E-Rechnung: Steueraufschluesselung fehlt")
	}
	for _, t := range inv.TaxBreakdown {
		switch t.CategoryCode {
		case CIITaxCategoryStandard:
			if strings.TrimSpace(t.ExemptionReason) != "" {
				return fmt.Errorf("E-Rechnung: Steuerkategorie S darf keine Befreiungsbegruendung tragen")
			}
		case CIITaxCategoryReverseCharge, CIITaxCategoryExempt:
			if strings.TrimSpace(t.ExemptionReason) == "" {
				return fmt.Errorf("E-Rechnung: Steuerkategorie %s erfordert eine Befreiungsbegruendung", t.CategoryCode)
			}
		default:
			return fmt.Errorf("E-Rechnung: unbekannte Steuerkategorie %q", t.CategoryCode)
		}
	}
	return nil
}

func validateCIIParty(role string, p CIIParty) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("E-Rechnung: Name des %ss fehlt", role)
	}
	if strings.TrimSpace(p.Street) == "" || strings.TrimSpace(p.City) == "" || strings.TrimSpace(p.PostalCode) == "" {
		return fmt.Errorf("E-Rechnung: vollstaendige Anschrift des %ss (Strasse, PLZ, Ort) ist erforderlich", role)
	}
	return nil
}
