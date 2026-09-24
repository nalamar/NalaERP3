package accounting

// E-Rechnung Ausgang, DB-Abfrage und Mapping (ADR 0022, Backlog E.4.3.2).
//
// Loest eine Rechnung aus invoices_out/invoice_out_items/contacts/
// contact_addresses/tax_codes/company_profiles vollstaendig auf und baut
// daraus das Eingabemodell CIIInvoice fuer den Serialisierer aus E.4.3.1.
// Reine Lesefunktion, keine Schreiblogik.
//
// Die Serialisierung selbst liegt bewusst in einer anderen Datei: dieser
// Baustein entscheidet WAS in die Rechnung gehoert, der andere WIE es
// aussieht.

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/settings"
)

// Befreiungsbegruendungen (BT-120). EN 16931 verlangt fuer die
// steuerfreien Kategorien eine Begruendung; der Serialisierer erzwingt das
// (E.4.3.1), die fachlich zutreffenden Texte werden hier gesetzt.
const (
	ciiExemptionReasonReverseCharge = "Steuerschuldnerschaft des Leistungsempfängers"
	ciiExemptionReasonExempt        = "Steuerbefreit"
)

// EInvoiceService baut aus einer gebuchten Ausgangsrechnung das
// CII-Eingabemodell.
type EInvoiceService struct {
	pg      *pgxpool.Pool
	company *settings.CompanyService
}

func NewEInvoiceService(pg *pgxpool.Pool, company *settings.CompanyService) *EInvoiceService {
	return &EInvoiceService{pg: pg, company: company}
}

// einvoiceTaxCode ist ein Steuerkennzeichen samt der fuer die
// EN-16931-Kategorie noetigen reverse_charge-Angabe. Bewusst eine eigene,
// enge Abfrage statt einer Erweiterung des geteilten taxCodeInfo: das wird
// nur hier gebraucht, und taxCodeInfo haengt an den Buchungspfaden
// (buildJournal/calcTotals), die diese Subtask nicht anfassen soll.
type einvoiceTaxCode struct {
	rate          float64 // Bruchteil, z.B. 0.19 - NICHT Prozent
	reverseCharge bool
}

// einvoiceItem ist eine Rechnungsposition, wie sie in der Datenbank steht.
type einvoiceItem struct {
	position    int
	description string
	qty         float64
	unitPrice   float64
	netAmount   float64
	taxAmount   float64
	taxCode     string
}

// BuildCIIInvoice laedt die Rechnung und liefert das fertige
// CII-Eingabemodell. Jede fehlende Pflichtangabe fuehrt zu einem Fehler,
// der das fehlende Feld benennt - es wird nichts geraten und nichts
// ergaenzt.
func (s *EInvoiceService) BuildCIIInvoice(ctx context.Context, invoiceID uuid.UUID, companyID string) (CIIInvoice, error) {
	var (
		out            CIIInvoice
		number         sql.NullString
		buyerReference sql.NullString
		due            sql.NullTime
		status         string
		invoiceType    string
		storniertAm    sql.NullTime
		netAmount      float64
		taxAmount      float64
		paidAmount     float64
		contactID      string
		contactName    string
		contactEmail   string
		contactVatID   string
		currency       string
		invoiceDate    time.Time
	)

	err := s.pg.QueryRow(ctx, `
        SELECT i.nummer, i.buyer_reference, i.status, i.invoice_type, i.invoice_date, i.due_date,
               i.currency, i.net_amount, i.tax_amount, i.paid_amount, i.storniert_am,
               i.contact_id, COALESCE(c.name,''), COALESCE(c.email,''), COALESCE(c.vat_id,'')
          FROM invoices_out i
          LEFT JOIN contacts c ON c.id = i.contact_id
         WHERE i.id = $1 AND i.company_id = $2
    `, invoiceID, companyID).Scan(
		&number, &buyerReference, &status, &invoiceType, &invoiceDate, &due,
		&currency, &netAmount, &taxAmount, &paidAmount, &storniertAm,
		&contactID, &contactName, &contactEmail, &contactVatID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return out, fmt.Errorf("Rechnung nicht gefunden")
		}
		return out, err
	}

	// Status-Guard (ADR 0022): nur festgeschriebene, nicht stornierte
	// Rechnungen duerfen als rechtsverbindliche E-Rechnung das Haus
	// verlassen. Ein draft ist per Definition noch aenderbar; eine
	// stornierte Rechnung ist keine gueltige Forderung mehr (eine echte
	// Rechnungskorrektur, UNTDID-1001-Typ 381, ist laut ADR 0022 nicht
	// Teil dieser Task).
	if storniertAm.Valid || status == "storniert" {
		return out, fmt.Errorf("E-Rechnung: stornierte Rechnung kann nicht als E-Rechnung exportiert werden")
	}
	if status != "booked" && status != "paid" {
		return out, fmt.Errorf("E-Rechnung: nur gebuchte Rechnungen koennen exportiert werden (Status: %s)", status)
	}
	if !number.Valid || strings.TrimSpace(number.String) == "" {
		return out, fmt.Errorf("E-Rechnung: Rechnung hat keine Rechnungsnummer")
	}
	if !buyerReference.Valid || strings.TrimSpace(buyerReference.String) == "" {
		return out, fmt.Errorf("E-Rechnung: Kaeuferreferenz (Leitweg-ID bzw. Kundenreferenz) ist erforderlich, aber an der Rechnung nicht gepflegt")
	}

	items, err := s.loadItems(ctx, invoiceID)
	if err != nil {
		return out, err
	}
	if len(items) == 0 {
		return out, fmt.Errorf("E-Rechnung: Rechnung hat keine Positionen")
	}

	taxCodes, err := s.loadTaxCodesForEInvoice(ctx)
	if err != nil {
		return out, err
	}

	profile, err := s.company.Get(ctx, companyID)
	if err != nil {
		return out, fmt.Errorf("E-Rechnung: Firmenprofil konnte nicht geladen werden: %w", err)
	}

	buyerAddr, err := s.loadBuyerAddress(ctx, contactID)
	if err != nil {
		return out, err
	}

	out = CIIInvoice{
		Number:         number.String,
		TypeCode:       ciiTypeCodeFor(invoiceType),
		IssueDate:      invoiceDate,
		Currency:       currency,
		BuyerReference: buyerReference.String,
		Seller: CIIParty{
			Name:       profile.Name,
			Street:     profile.Street,
			PostalCode: profile.PostalCode,
			City:       profile.City,
			Country:    profile.Country,
			Email:      ciiSellerEmail(profile),
			Phone:      profile.Phone,
			VatID:      profile.VatID,
			TaxNo:      profile.TaxNo,
		},
		Buyer: CIIParty{
			ID:         contactID,
			Name:       contactName,
			Street:     buyerAddr.street,
			PostalCode: buyerAddr.postalCode,
			City:       buyerAddr.city,
			Country:    buyerAddr.country,
			Email:      contactEmail,
			VatID:      contactVatID,
		},
		IBAN:          profile.IBAN,
		BIC:           profile.BIC,
		AccountHolder: profile.AccountHolder,
		PaidAmount:    paidAmount,
	}
	if due.Valid {
		d := due.Time
		out.DueDate = &d
	}

	lineTotal, breakdown, err := ciiBuildLinesAndTax(items, taxCodes, &out)
	if err != nil {
		return CIIInvoice{}, err
	}

	taxTotal := 0.0
	for _, b := range breakdown {
		taxTotal += b.CalculatedAmount
	}
	taxTotal = ciiRound2(taxTotal)

	// Gegen die gespeicherten Kopfsummen abgleichen. Eine Abweichung waere
	// ein echter Datenfehler (Positionen passen nicht zum gebuchten Beleg)
	// - dann lieber gar keine E-Rechnung als eine, die dem gebuchten
	// Beleg widerspricht.
	if math.Abs(lineTotal-ciiRound2(netAmount)) > 0.01 {
		return CIIInvoice{}, fmt.Errorf("E-Rechnung: Summe der Positionen (%.2f) weicht vom gebuchten Nettobetrag (%.2f) ab", lineTotal, netAmount)
	}
	if math.Abs(taxTotal-ciiRound2(taxAmount)) > 0.01 {
		return CIIInvoice{}, fmt.Errorf("E-Rechnung: Summe der Steuerbetraege (%.2f) weicht vom gebuchten Steuerbetrag (%.2f) ab", taxTotal, taxAmount)
	}

	out.TaxBreakdown = breakdown
	out.LineTotalAmount = lineTotal
	out.TaxBasisTotalAmount = lineTotal
	out.TaxTotalAmount = taxTotal
	out.GrandTotalAmount = ciiRound2(lineTotal + taxTotal)
	out.DuePayableAmount = ciiRound2(out.GrandTotalAmount - ciiRound2(paidAmount))

	return out, nil
}

// ciiBuildLinesAndTax fuellt die Positionen und baut daraus die
// Steueraufschluesselung je Kombination aus Kategorie und Steuersatz
// (BG-23). Reine Funktion ausser dem Schreiben nach inv.Lines.
func ciiBuildLinesAndTax(items []einvoiceItem, taxCodes map[string]einvoiceTaxCode, inv *CIIInvoice) (float64, []CIITaxBreakdown, error) {
	type groupKey struct {
		category string
		rate     float64
	}
	var order []groupKey
	basis := map[groupKey]float64{}
	calculated := map[groupKey]float64{}

	lineTotal := 0.0
	for _, it := range items {
		category, ratePercent, err := ciiTaxCategoryFor(it.taxCode, taxCodes, it.position)
		if err != nil {
			return 0, nil, err
		}

		net := ciiRound2(it.netAmount)
		lineTotal += net

		inv.Lines = append(inv.Lines, CIILineItem{
			LineID:          strconv.Itoa(it.position),
			Name:            it.description,
			Qty:             it.qty,
			UnitCode:        CIIDefaultUnitCode,
			NetUnitPrice:    it.unitPrice,
			LineTotalAmount: net,
			TaxCategoryCode: category,
			TaxRatePercent:  ratePercent,
		})

		k := groupKey{category: category, rate: ratePercent}
		if _, seen := basis[k]; !seen {
			order = append(order, k)
		}
		basis[k] += net
		calculated[k] += it.taxAmount
	}

	breakdown := make([]CIITaxBreakdown, 0, len(order))
	for _, k := range order {
		b := CIITaxBreakdown{
			CategoryCode:     k.category,
			RatePercent:      k.rate,
			BasisAmount:      ciiRound2(basis[k]),
			CalculatedAmount: ciiRound2(calculated[k]),
		}
		switch k.category {
		case CIITaxCategoryReverseCharge:
			b.ExemptionReason = ciiExemptionReasonReverseCharge
		case CIITaxCategoryExempt:
			b.ExemptionReason = ciiExemptionReasonExempt
		}
		breakdown = append(breakdown, b)
	}

	return ciiRound2(lineTotal), breakdown, nil
}

// ciiTaxCategoryFor leitet die UNTDID-5305-Kategorie aus den vorhandenen
// Steuerstammdaten ab (ADR 0022) und liefert den Steuersatz in PROZENT
// (tax_codes.rate ist ein Bruchteil, z.B. 0.1900 fuer 19 %).
//
// Ein leeres Steuerkennzeichen wird bewusst ABGELEHNT statt auf eine
// Kategorie geraten zu werden: invoice_out_items.tax_code ist nullable und
// gilt in den Buchungspfaden als "0 %, kein Fehler" (z.B. Durchlaufposten),
// aber EN 16931 verlangt je Position eine fachlich zutreffende Kategorie -
// "steuerbefreit" und "Durchlaufposten" sind nicht dasselbe, und eine
// falsche Kategorie waere ein inhaltlicher Fehler in einer
// rechtsverbindlichen Rechnung.
func ciiTaxCategoryFor(code string, codes map[string]einvoiceTaxCode, position int) (string, float64, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", 0, fmt.Errorf("E-Rechnung: Position %d hat kein Steuerkennzeichen - fuer eine E-Rechnung ist je Position eine Steuerkategorie erforderlich", position)
	}
	info, ok := codes[code]
	if !ok {
		return "", 0, fmt.Errorf("E-Rechnung: unbekanntes oder inaktives Steuerkennzeichen %q in Position %d", code, position)
	}
	ratePercent := ciiRound2(info.rate * 100)
	switch {
	case info.rate > 0:
		return CIITaxCategoryStandard, ratePercent, nil
	case info.reverseCharge:
		return CIITaxCategoryReverseCharge, ratePercent, nil
	default:
		return CIITaxCategoryExempt, ratePercent, nil
	}
}

// ciiTypeCodeFor bildet invoices_out.invoice_type auf UNTDID 1001 ab
// (ADR 0022): Abschlagsrechnung => 386, alles andere => 380.
func ciiTypeCodeFor(invoiceType string) string {
	if invoiceType == "abschlagsrechnung" {
		return CIITypeCodePrepaymentInvoice
	}
	return CIITypeCodeCommercialInvoice
}

// ciiSellerEmail bevorzugt die dedizierte Rechnungs-E-Mail-Adresse, faellt
// aber auf die allgemeine zurueck.
func ciiSellerEmail(p *settings.CompanyProfile) string {
	if strings.TrimSpace(p.InvoiceEmail) != "" {
		return p.InvoiceEmail
	}
	return p.Email
}

func (s *EInvoiceService) loadItems(ctx context.Context, invoiceID uuid.UUID) ([]einvoiceItem, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT position, description, qty, unit_price, net_amount, tax_amount, COALESCE(tax_code,'')
          FROM invoice_out_items
         WHERE invoice_id = $1
         ORDER BY position
    `, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []einvoiceItem
	for rows.Next() {
		var it einvoiceItem
		if err := rows.Scan(&it.position, &it.description, &it.qty, &it.unitPrice, &it.netAmount, &it.taxAmount, &it.taxCode); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *EInvoiceService) loadTaxCodesForEInvoice(ctx context.Context) (map[string]einvoiceTaxCode, error) {
	rows, err := s.pg.Query(ctx, `SELECT code, rate, reverse_charge FROM tax_codes WHERE is_active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]einvoiceTaxCode{}
	for rows.Next() {
		var code string
		var info einvoiceTaxCode
		if err := rows.Scan(&code, &info.rate, &info.reverseCharge); err != nil {
			return nil, err
		}
		out[code] = info
	}
	return out, rows.Err()
}

type einvoiceAddress struct {
	street     string
	postalCode string
	city       string
	country    string
}

// loadBuyerAddress loest die Kaeufer-Postanschrift auf (BG-8, EN-16931-
// Pflichtblock). Praeferenz laut ADR 0022: Rechnungsadresse vor
// Hauptadresse vor irgendeiner vorhandenen. Ist keine Adresse gepflegt,
// wird der Export mit klarer Meldung abgelehnt - eine erfundene Adresse in
// einer rechtsverbindlichen Rechnung waere deutlich schaedlicher als ein
// fehlgeschlagener Export.
func (s *EInvoiceService) loadBuyerAddress(ctx context.Context, contactID string) (einvoiceAddress, error) {
	var a einvoiceAddress
	err := s.pg.QueryRow(ctx, `
        SELECT COALESCE(zeile1,''), COALESCE(plz,''), COALESCE(ort,''), COALESCE(land,'')
          FROM contact_addresses
         WHERE contact_id = $1
         ORDER BY (art = 'billing') DESC, is_primary DESC, id
         LIMIT 1
    `, contactID).Scan(&a.street, &a.postalCode, &a.city, &a.country)
	if err != nil {
		if err == pgx.ErrNoRows {
			return a, fmt.Errorf("E-Rechnung: fuer den Kaeufer ist keine Anschrift gepflegt - fuer eine E-Rechnung ist die Kaeuferanschrift erforderlich")
		}
		return a, err
	}
	return a, nil
}

// ciiRound2 rundet kaufmaennisch auf zwei Nachkommastellen.
func ciiRound2(v float64) float64 {
	return math.Round(v*100) / 100
}
