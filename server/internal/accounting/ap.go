package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Eingangsrechnungsprüfung / 3-Way-Match (ADR 0018). APService ist das
// Accounts-Payable-Schwester-Konzept zu ARService - bewusst OHNE Buchung,
// Storno, journal_entries-Anbindung oder USt.-Behandlung (siehe ADR 0018,
// Option C verworfen). MatchInvoiceIn ist eine reine Lese-/
// Berechnungsfunktion, kein Zustand wird persistiert.

type APService struct {
	pg *pgxpool.Pool
}

func NewAPService(pg *pgxpool.Pool) *APService {
	return &APService{pg: pg}
}

type InvoiceIn struct {
	ID              string    `json:"id"`
	SupplierID      string    `json:"lieferant_id"`
	PurchaseOrderID *string   `json:"bestellung_id"`
	InvoiceNumber   string    `json:"rechnungsnummer"`
	InvoiceDate     time.Time `json:"rechnungsdatum"`
	Currency        string    `json:"waehrung"`
	Status          string    `json:"status"`
	Note            string    `json:"notiz"`
	CreatedAt       time.Time `json:"angelegt_am"`
}

type InvoiceInItem struct {
	ID                  string  `json:"id"`
	InvoiceInID         string  `json:"rechnung_id"`
	PurchaseOrderItemID *string `json:"bestellposition_id"`
	Description         string  `json:"bezeichnung"`
	Qty                 float64 `json:"menge"`
	UnitPrice           float64 `json:"preis"`
	Currency            string  `json:"waehrung"`
}

type InvoiceInCreate struct {
	SupplierID      string               `json:"lieferant_id"`
	PurchaseOrderID *string              `json:"bestellung_id"`
	InvoiceNumber   string               `json:"rechnungsnummer"`
	InvoiceDate     *time.Time           `json:"rechnungsdatum"`
	Currency        string               `json:"waehrung"`
	Note            string               `json:"notiz"`
	Items           []InvoiceInItemInput `json:"positionen"`
}

type InvoiceInItemInput struct {
	PurchaseOrderItemID *string `json:"bestellposition_id"`
	Description         string  `json:"bezeichnung"`
	Qty                 float64 `json:"menge"`
	UnitPrice           float64 `json:"preis"`
	Currency            string  `json:"waehrung"`
}

type InvoiceInFilter struct {
	SupplierID      string
	PurchaseOrderID string
}

func (s *APService) CreateInvoiceIn(ctx context.Context, in InvoiceInCreate, companyID string) (*InvoiceIn, []InvoiceInItem, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.SupplierID) == "" {
		return nil, nil, errors.New("Lieferant erforderlich")
	}
	if len(in.Items) == 0 {
		return nil, nil, errors.New("mindestens eine Position erforderlich")
	}
	for _, it := range in.Items {
		if it.Qty <= 0 {
			return nil, nil, errors.New("Menge muss größer als 0 sein")
		}
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
	invoiceDate := time.Now()
	if in.InvoiceDate != nil {
		invoiceDate = *in.InvoiceDate
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var supplierOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contacts WHERE id=$1 AND company_id=$2)`, in.SupplierID, companyID).Scan(&supplierOwned); err != nil {
		return nil, nil, err
	}
	if !supplierOwned {
		return nil, nil, errors.New("Lieferant nicht gefunden")
	}

	if in.PurchaseOrderID != nil && strings.TrimSpace(*in.PurchaseOrderID) != "" {
		var poOwned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_orders WHERE id=$1 AND company_id=$2)`, *in.PurchaseOrderID, companyID).Scan(&poOwned); err != nil {
			return nil, nil, err
		}
		if !poOwned {
			return nil, nil, errors.New("Bestellung nicht gefunden")
		}
	}

	for _, it := range in.Items {
		if it.PurchaseOrderItemID != nil && strings.TrimSpace(*it.PurchaseOrderItemID) != "" {
			var poItemOwned bool
			if err := tx.QueryRow(ctx, `
                SELECT EXISTS(
                    SELECT 1 FROM purchase_order_items poi
                    JOIN purchase_orders po ON po.id = poi.order_id
                    WHERE poi.id=$1 AND po.company_id=$2
                )
            `, *it.PurchaseOrderItemID, companyID).Scan(&poItemOwned); err != nil {
				return nil, nil, err
			}
			if !poItemOwned {
				return nil, nil, errors.New("Bestellposition nicht gefunden")
			}
		}
	}

	id := uuid.NewString()
	var invoice InvoiceIn
	if err := tx.QueryRow(ctx, `
        INSERT INTO invoices_in (id, company_id, supplier_id, purchase_order_id, invoice_number, invoice_date, currency, note)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING id, supplier_id, purchase_order_id, invoice_number, invoice_date, currency, status, COALESCE(note,''), created_at
    `, id, companyID, in.SupplierID, in.PurchaseOrderID, in.InvoiceNumber, invoiceDate, in.Currency, in.Note).Scan(
		&invoice.ID, &invoice.SupplierID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate, &invoice.Currency, &invoice.Status, &invoice.Note, &invoice.CreatedAt,
	); err != nil {
		return nil, nil, err
	}

	items := make([]InvoiceInItem, 0, len(in.Items))
	for _, it := range in.Items {
		if it.Currency == "" {
			it.Currency = invoice.Currency
		}
		iid := uuid.NewString()
		var out InvoiceInItem
		if err := tx.QueryRow(ctx, `
            INSERT INTO invoice_in_items (id, invoice_in_id, purchase_order_item_id, description, qty, unit_price, currency)
            VALUES ($1,$2,$3,$4,$5,$6,$7)
            RETURNING id, invoice_in_id, purchase_order_item_id, COALESCE(description,''), qty, unit_price, currency
        `, iid, invoice.ID, it.PurchaseOrderItemID, it.Description, it.Qty, it.UnitPrice, it.Currency).Scan(
			&out.ID, &out.InvoiceInID, &out.PurchaseOrderItemID, &out.Description, &out.Qty, &out.UnitPrice, &out.Currency,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, out)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &invoice, items, nil
}

func (s *APService) GetInvoiceIn(ctx context.Context, id string, companyID string) (*InvoiceIn, []InvoiceInItem, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	var invoice InvoiceIn
	err := s.pg.QueryRow(ctx, `
        SELECT id, supplier_id, purchase_order_id, invoice_number, invoice_date, currency, status, COALESCE(note,''), created_at
        FROM invoices_in WHERE id=$1 AND company_id=$2
    `, id, companyID).Scan(&invoice.ID, &invoice.SupplierID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate, &invoice.Currency, &invoice.Status, &invoice.Note, &invoice.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, errors.New("Eingangsrechnung nicht gefunden")
		}
		return nil, nil, err
	}

	rows, err := s.pg.Query(ctx, `
        SELECT id, invoice_in_id, purchase_order_item_id, COALESCE(description,''), qty, unit_price, currency
        FROM invoice_in_items WHERE invoice_in_id=$1
    `, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]InvoiceInItem, 0)
	for rows.Next() {
		var it InvoiceInItem
		if err := rows.Scan(&it.ID, &it.InvoiceInID, &it.PurchaseOrderItemID, &it.Description, &it.Qty, &it.UnitPrice, &it.Currency); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	return &invoice, items, nil
}

func (s *APService) ListInvoicesIn(ctx context.Context, f InvoiceInFilter, companyID string) ([]InvoiceIn, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	query := `
        SELECT id, supplier_id, purchase_order_id, invoice_number, invoice_date, currency, status, COALESCE(note,''), created_at
        FROM invoices_in WHERE company_id=$1
    `
	args := []any{companyID}
	if strings.TrimSpace(f.SupplierID) != "" {
		args = append(args, f.SupplierID)
		query += fmt.Sprintf(" AND supplier_id=$%d", len(args))
	}
	if strings.TrimSpace(f.PurchaseOrderID) != "" {
		args = append(args, f.PurchaseOrderID)
		query += fmt.Sprintf(" AND purchase_order_id=$%d", len(args))
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InvoiceIn, 0)
	for rows.Next() {
		var invoice InvoiceIn
		if err := rows.Scan(&invoice.ID, &invoice.SupplierID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate, &invoice.Currency, &invoice.Status, &invoice.Note, &invoice.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, invoice)
	}
	return out, nil
}

type MatchLineResult struct {
	InvoiceInItemID     string   `json:"rechnungsposition_id"`
	PurchaseOrderItemID *string  `json:"bestellposition_id"`
	Matchable           bool     `json:"abgleichbar"`
	BestelltQty         *float64 `json:"bestellt_menge"`
	ErhaltenQty         *float64 `json:"erhalten_menge"`
	BerechnetQty        float64  `json:"berechnet_menge"`
	BestelltPreis       *float64 `json:"bestellt_preis"`
	BerechnetPreis      float64  `json:"berechnet_preis"`
	MengeStimmt         *bool    `json:"menge_stimmt"`
	PreisStimmt         *bool    `json:"preis_stimmt"`
}

type MatchResult struct {
	InvoiceInID string            `json:"rechnung_id"`
	Lines       []MatchLineResult `json:"positionen"`
}

// MatchInvoiceIn vergleicht je Rechnungsposition mit Bestellpositions-
// Bezug drei Datenquellen (ADR 0018 - Kernregel): bestellte Menge/Preis
// (purchase_order_items), tatsächlich erhaltene Menge (SUM der
// stock_movements mit purchase_order_item_id-Bezug) und die berechnete
// Menge/Preis der Rechnungsposition selbst. MengeStimmt vergleicht
// berechnet gegen ERHALTEN (der eigentliche AP-Kontrollpunkt: es darf
// nur bezahlt werden, was tatsächlich eingegangen ist), PreisStimmt
// vergleicht berechnet gegen BESTELLT. Exakter Vergleich, keine
// Toleranzschwelle (siehe ADR 0018). Positionen ohne Bestellpositions-
// Bezug sind explizit nicht abgleichbar.
func (s *APService) MatchInvoiceIn(ctx context.Context, invoiceInID string, companyID string) (*MatchResult, error) {
	if strings.TrimSpace(invoiceInID) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}

	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices_in WHERE id=$1 AND company_id=$2)`, invoiceInID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Eingangsrechnung nicht gefunden")
	}

	rows, err := s.pg.Query(ctx, `
        SELECT id, purchase_order_item_id, qty, unit_price
        FROM invoice_in_items WHERE invoice_in_id=$1
    `, invoiceInID)
	if err != nil {
		return nil, err
	}
	type invoiceLine struct {
		id        string
		poItemID  *string
		qty       float64
		unitPrice float64
	}
	var lines []invoiceLine
	for rows.Next() {
		var l invoiceLine
		if err := rows.Scan(&l.id, &l.poItemID, &l.qty, &l.unitPrice); err != nil {
			rows.Close()
			return nil, err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := &MatchResult{InvoiceInID: invoiceInID, Lines: make([]MatchLineResult, 0, len(lines))}
	for _, l := range lines {
		line := MatchLineResult{
			InvoiceInItemID:     l.id,
			PurchaseOrderItemID: l.poItemID,
			BerechnetQty:        l.qty,
			BerechnetPreis:      l.unitPrice,
		}
		if l.poItemID == nil || strings.TrimSpace(*l.poItemID) == "" {
			result.Lines = append(result.Lines, line)
			continue
		}
		line.Matchable = true

		var bestelltQty, bestelltPreis float64
		if err := s.pg.QueryRow(ctx, `SELECT qty, unit_price FROM purchase_order_items WHERE id=$1`, *l.poItemID).Scan(&bestelltQty, &bestelltPreis); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				result.Lines = append(result.Lines, line)
				continue
			}
			return nil, err
		}
		line.BestelltQty = &bestelltQty
		line.BestelltPreis = &bestelltPreis

		var erhaltenQty float64
		if err := s.pg.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0) FROM stock_movements WHERE purchase_order_item_id=$1`, *l.poItemID).Scan(&erhaltenQty); err != nil {
			return nil, err
		}
		line.ErhaltenQty = &erhaltenQty

		mengeStimmt := l.qty == erhaltenQty
		preisStimmt := l.unitPrice == bestelltPreis
		line.MengeStimmt = &mengeStimmt
		line.PreisStimmt = &preisStimmt

		result.Lines = append(result.Lines, line)
	}
	return result, nil
}
