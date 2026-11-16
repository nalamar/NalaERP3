package sales

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Abruf von Bedarfspositionen als Nachtrag (ADR 0042 Frage 3, Backlog
// I.2.6b): die Position wird aus dem Quellangebot des Auftrags vorbelegt
// (Text, Einheit, Preis, Steuer, OZ), die Menge ist frei. Mehrfacher Abruf
// ist erlaubt; ListProvisionalItems zeigt die abgerufenen Mengen.

// AddendumCallOffInput: Qty 0 = Menge aus dem Angebot.
type AddendumCallOffInput struct {
	QuoteItemID string  `json:"quote_item_id"`
	Qty         float64 `json:"qty"`
}

// ProvisionalItem ist eine Bedarfsposition des Quellangebots mit den
// Mengen aus angenommenen (CalledOffQty) und offenen Nachträgen
// (PendingQty: Entwurf oder beantragt).
type ProvisionalItem struct {
	QuoteItemID  string  `json:"quote_item_id"`
	OutlineNo    string  `json:"outline_no"`
	Description  string  `json:"description"`
	ItemKind     string  `json:"item_kind"`
	Unit         string  `json:"unit"`
	UnitPrice    float64 `json:"unit_price"`
	TaxCode      string  `json:"tax_code"`
	QuotedQty    float64 `json:"quoted_qty"`
	CalledOffQty float64 `json:"called_off_qty"`
	PendingQty   float64 `json:"pending_qty"`
}

const provisionalKindsSQL = "qi.item_kind IN ('bedarfsposition_mit_gb', 'bedarfsposition_ohne_gb')"

func (s *Service) CreateAddendumCallOff(ctx context.Context, orderID, addendumID uuid.UUID, in AddendumCallOffInput, companyID string) (*SalesOrderAddendumItem, error) {
	quoteItemID, err := uuid.Parse(strings.TrimSpace(in.QuoteItemID))
	if err != nil {
		return nil, errors.New("quote_item_id ist ungültig")
	}
	if in.Qty < 0 {
		return nil, errors.New("Menge darf nicht negativ sein")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureAddendumEditableTx(ctx, tx, orderID, addendumID, companyID); err != nil {
		return nil, err
	}
	var description, unit, taxCode, outlineNo string
	var qty, unitPrice float64
	err = tx.QueryRow(ctx, `
        SELECT qi.description, qi.qty, qi.unit, qi.unit_price, COALESCE(qi.tax_code,''), qi.outline_no
        FROM sales_orders so
        JOIN quote_items qi ON qi.quote_id = so.source_quote_id
        WHERE so.id=$1 AND so.company_id=$2 AND qi.id=$3 AND `+provisionalKindsSQL,
		orderID, companyID, quoteItemID).Scan(&description, &qty, &unit, &unitPrice, &taxCode, &outlineNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("quote_item_id ist ungültig: keine Bedarfsposition des Angebots zu diesem Auftrag")
		}
		return nil, err
	}
	if in.Qty > 0 {
		qty = in.Qty
	}
	codes, err := loadTaxCodesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	rate, err := taxRate(codes, taxCode)
	if err != nil {
		return nil, err
	}
	var item SalesOrderAddendumItem
	err = tx.QueryRow(ctx, `
        INSERT INTO sales_order_addendum_items (id, addendum_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code, outline_no, source_quote_item_id)
        VALUES (
            $1, $2,
            (SELECT COALESCE(MAX(position), 0) + 1 FROM sales_order_addendum_items WHERE addendum_id=$2),
            $3, $4, $5, $6, $7, $8, $9, $10, $11
        )
        RETURNING id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,''), outline_no, COALESCE(source_quote_item_id::text,'')
    `, uuid.New(), addendumID, description, qty, unit, unitPrice, qty*unitPrice, qty*unitPrice*rate, nullIfEmpty(taxCode), outlineNo, quoteItemID).Scan(
		&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode, &item.OutlineNo, &item.SourceQuoteItemID)
	if err != nil {
		return nil, err
	}
	if err := refreshAddendumTotalsTx(ctx, tx, addendumID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListProvisionalItems(ctx context.Context, orderID uuid.UUID, companyID string) ([]ProvisionalItem, error) {
	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sales_orders WHERE id=$1 AND company_id=$2)`, orderID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Auftrag nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT qi.id::text, qi.outline_no, qi.description, qi.item_kind, qi.unit, qi.unit_price, COALESCE(qi.tax_code,''), qi.qty,
               COALESCE(SUM(sai.qty) FILTER (WHERE soa.status = 'angenommen'), 0),
               COALESCE(SUM(sai.qty) FILTER (WHERE soa.status IN ('entwurf', 'beantragt')), 0)
        FROM sales_orders so
        JOIN quote_items qi ON qi.quote_id = so.source_quote_id
        LEFT JOIN sales_order_addendum_items sai ON sai.source_quote_item_id = qi.id
        LEFT JOIN sales_order_addenda soa ON soa.id = sai.addendum_id AND soa.sales_order_id = so.id
        WHERE so.id=$1 AND so.company_id=$2 AND `+provisionalKindsSQL+`
        GROUP BY qi.id, qi.outline_no, qi.description, qi.item_kind, qi.unit, qi.unit_price, qi.tax_code, qi.qty, qi.position
        ORDER BY qi.position
    `, orderID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProvisionalItem, 0)
	for rows.Next() {
		var p ProvisionalItem
		if err := rows.Scan(&p.QuoteItemID, &p.OutlineNo, &p.Description, &p.ItemKind, &p.Unit, &p.UnitPrice, &p.TaxCode, &p.QuotedQty, &p.CalledOffQty, &p.PendingQty); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
