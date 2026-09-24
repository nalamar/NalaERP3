package accounting

// E-Rechnung Eingang, Uebernahme nach invoices_in (Backlog E.6, Folge
// aus ADR 0023).
//
// Grundsatz: die Rechnungsdaten stammen AUSSCHLIESSLICH aus der geparsten
// Datei, nicht vom Client. Der Client steuert nur die Entscheidungen, die
// ein Mensch treffen muss - welcher Lieferant es ist und ob es einen
// Bestellbezug gibt. Wuerde der Client Betraege oder Nummern mitschicken,
// koennte er sie abweichend von der Rechnung setzen; in einem
// buchungsrelevanten Kontext waere das nicht vertretbar.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// EInvoiceTakeoverDecision buendelt, was ein Mensch entscheiden muss.
type EInvoiceTakeoverDecision struct {
	SupplierID      string
	PurchaseOrderID *string
	Note            string
}

// FindInvoiceInByNumber sucht eine bereits erfasste Eingangsrechnung
// desselben Lieferanten mit derselben Rechnungsnummer.
func (s *APService) FindInvoiceInByNumber(ctx context.Context, supplierID, invoiceNumber, companyID string) (string, bool, error) {
	invoiceNumber = strings.TrimSpace(invoiceNumber)
	if invoiceNumber == "" {
		return "", false, nil
	}
	var id string
	err := s.pg.QueryRow(ctx, `
        SELECT id FROM invoices_in
         WHERE company_id = $1 AND supplier_id = $2 AND btrim(invoice_number) = btrim($3)
         LIMIT 1
    `, companyID, supplierID, invoiceNumber).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

// TakeOverEInvoice legt aus einer geparsten Eingangs-E-Rechnung eine
// Eingangsrechnung an.
//
// Die Pruefung auf Doppelerfassung (gleicher Lieferant, gleiche
// Rechnungsnummer) sitzt bewusst im Anwendungscode und nicht als
// Datenbank-Constraint: `invoices_in` traegt bereits manuell erfasste
// Bestaende, fuer die eine nachtraegliche Unique-Constraint fehlschlagen
// koennte, und die Regel gilt fachlich nur fuer den Uebernahmeweg -
// dasselbe Muster wie bei den VOB-Regeln aus B.4.3 (siehe Kommentar in
// 074_invoices_out_invoice_type.sql).
func (s *APService) TakeOverEInvoice(
	ctx context.Context,
	parsed *ParsedEInvoice,
	decision EInvoiceTakeoverDecision,
	companyID string,
) (*InvoiceIn, []InvoiceInItem, error) {
	if parsed == nil {
		return nil, nil, fmt.Errorf("E-Rechnung Eingang: keine geparste Rechnung übergeben")
	}
	if strings.TrimSpace(decision.SupplierID) == "" {
		return nil, nil, fmt.Errorf("E-Rechnung Eingang: Lieferant muss bestätigt werden - eine automatische Zuordnung findet bewusst nicht statt")
	}
	if len(parsed.Lines) == 0 {
		return nil, nil, fmt.Errorf("E-Rechnung Eingang: die Rechnung enthält keine Positionen und kann nicht übernommen werden")
	}

	// Mengen: invoices_in verlangt Menge > 0. Eine Position ohne Menge
	// wird mit Nennung der Position abgelehnt, statt sie stillschweigend
	// auf 1 zu setzen - eine erfundene Menge waere ein inhaltlicher
	// Fehler in einer Eingangsrechnung.
	for i, l := range parsed.Lines {
		if l.Qty <= 0 {
			bezeichnung := strings.TrimSpace(l.Name)
			if bezeichnung == "" {
				bezeichnung = "ohne Bezeichnung"
			}
			return nil, nil, fmt.Errorf(
				"E-Rechnung Eingang: Position %d (%s) hat keine Menge größer 0 und kann nicht übernommen werden",
				i+1, bezeichnung)
		}
	}

	if existing, found, err := s.FindInvoiceInByNumber(ctx, decision.SupplierID, parsed.Number, companyID); err != nil {
		return nil, nil, err
	} else if found {
		return nil, nil, fmt.Errorf(
			"E-Rechnung Eingang: zu diesem Lieferanten ist die Rechnungsnummer %q bereits erfasst (Eingangsrechnung %s) - doppelte Erfassung wird verhindert",
			parsed.Number, existing)
	}

	in := InvoiceInCreate{
		SupplierID:      decision.SupplierID,
		PurchaseOrderID: decision.PurchaseOrderID,
		InvoiceNumber:   parsed.Number,
		Currency:        parsed.Currency,
		Note:            buildEInvoiceTakeoverNote(parsed, decision.Note),
	}
	if !parsed.IssueDate.IsZero() {
		d := parsed.IssueDate
		in.InvoiceDate = &d
	}

	for _, l := range parsed.Lines {
		in.Items = append(in.Items, InvoiceInItemInput{
			Description: l.Name,
			Qty:         l.Qty,
			UnitPrice:   l.NetUnitPrice,
			Currency:    parsed.Currency,
		})
	}

	// Bewusste Wiederverwendung: CreateInvoiceIn prueft bereits die
	// Mandanten-Zugehoerigkeit von Lieferant, Bestellung und
	// Bestellpositionen und schreibt in einer Transaktion. Diese Logik
	// hier zu wiederholen waere eine zweite Quelle der Wahrheit.
	return s.CreateInvoiceIn(ctx, in, companyID)
}

// buildEInvoiceTakeoverNote haelt fest, woher die Rechnung stammt, und
// rettet die Angaben, die das heutige invoices_in-Datenmodell nicht
// abbilden kann.
//
// BEKANNTE LUECKE, bewusst dokumentiert statt verschwiegen:
// `invoice_in_items` hat weder ein Steuerkennzeichen noch eine
// Mengeneinheit, und `invoices_in` traegt keine Summenfelder. Die
// geparste Steueraufschluesselung und die ausgewiesenen Summen gehen bei
// der Uebernahme daher verloren. Sie hier als Klartext in die Notiz zu
// schreiben ist eine Notloesung, damit die Information dem Sachbearbeiter
// wenigstens erhalten bleibt - strukturiert gehoert sie ins Datenmodell
// (Backlog E.8). Es wird ausdruecklich nichts gerechnet und nichts
// ergaenzt, nur uebernommen was in der Rechnung steht.
func buildEInvoiceTakeoverNote(parsed *ParsedEInvoice, userNote string) string {
	var b strings.Builder
	if n := strings.TrimSpace(userNote); n != "" {
		b.WriteString(n)
		b.WriteString("\n\n")
	}

	fmt.Fprintf(&b, "Übernommen aus E-Rechnung (%s).", strings.ToUpper(parsed.Format))
	if v := strings.TrimSpace(parsed.Seller.VatID); v != "" {
		fmt.Fprintf(&b, " USt-IdNr. des Verkäufers: %s.", v)
	}
	if parsed.GrandTotalAmount != 0 {
		fmt.Fprintf(&b, "\nAusgewiesen: netto %.2f, Steuer %.2f, brutto %.2f %s.",
			parsed.TaxBasisTotalAmount, parsed.TaxTotalAmount, parsed.GrandTotalAmount, parsed.Currency)
	}
	if parsed.DueDate != nil {
		fmt.Fprintf(&b, "\nFällig am %s.", parsed.DueDate.Format("02.01.2006"))
	}
	for _, t := range parsed.TaxBreakdown {
		fmt.Fprintf(&b, "\nSteuer %s %.2f%%: Basis %.2f, Betrag %.2f.",
			t.CategoryCode, t.RatePercent, t.BasisAmount, t.CalculatedAmount)
	}
	for _, h := range parsed.Hinweise {
		fmt.Fprintf(&b, "\nHinweis aus der Rechnung: %s", h)
	}
	return b.String()
}
