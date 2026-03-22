package sales

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/accounting"
	"nalaerp3/internal/settings"
)

type SalesOrderItem struct {
	ID           string  `json:"id"`
	Position     int     `json:"position"`
	Description  string  `json:"description"`
	Qty          float64 `json:"qty"`
	Unit         string  `json:"unit"`
	UnitPrice    float64 `json:"unit_price"`
	TaxCode      string  `json:"tax_code"`
	InvoicedQty  float64 `json:"invoiced_qty"`
	RemainingQty float64 `json:"remaining_qty"`
}

type SalesOrder struct {
	ID                 uuid.UUID        `json:"id"`
	Number             string           `json:"number"`
	SourceQuoteID      uuid.UUID        `json:"source_quote_id"`
	LinkedInvoiceOutID string           `json:"linked_invoice_out_id,omitempty"`
	ProjectID          string           `json:"project_id"`
	ProjectName        string           `json:"project_name"`
	ContactID          string           `json:"contact_id"`
	ContactName        string           `json:"contact_name"`
	Status             string           `json:"status"`
	OrderDate          time.Time        `json:"order_date"`
	Currency           string           `json:"currency"`
	Note               string           `json:"note"`
	NetAmount          float64          `json:"net_amount"`
	TaxAmount          float64          `json:"tax_amount"`
	GrossAmount        float64          `json:"gross_amount"`
	Items              []SalesOrderItem `json:"items"`
}

type SalesOrderListItem struct {
	ID                   uuid.UUID `json:"id"`
	Number               string    `json:"number"`
	SourceQuoteID        uuid.UUID `json:"source_quote_id"`
	LinkedInvoiceOutID   string    `json:"linked_invoice_out_id,omitempty"`
	ProjectID            string    `json:"project_id"`
	ProjectName          string    `json:"project_name"`
	ContactID            string    `json:"contact_id"`
	ContactName          string    `json:"contact_name"`
	Status               string    `json:"status"`
	OrderDate            time.Time `json:"order_date"`
	Currency             string    `json:"currency"`
	GrossAmount          float64   `json:"gross_amount"`
	RelatedInvoiceCount  int       `json:"related_invoice_count"`
	RemainingGrossAmount float64   `json:"remaining_gross_amount"`
}

type SalesOrderFilter struct {
	Status    string
	ContactID string
	ProjectID string
	Search    string
	Limit     int
	Offset    int
}

type Service struct {
	pg  *pgxpool.Pool
	num *settings.NumberingService
}

type ConvertToInvoiceInput struct {
	RevenueAccount string                      `json:"revenue_account"`
	InvoiceDate    time.Time                   `json:"invoice_date"`
	DueDate        *time.Time                  `json:"due_date,omitempty"`
	Items          []ConvertToInvoiceItemInput `json:"items,omitempty"`
}

type ConvertToInvoiceItemInput struct {
	SalesOrderItemID string  `json:"sales_order_item_id"`
	Qty              float64 `json:"qty"`
}

type SalesOrderUpdate struct {
	Number    *string    `json:"number"`
	OrderDate *time.Time `json:"order_date"`
	Currency  *string    `json:"currency"`
	Note      *string    `json:"note"`
}

type SalesOrderItemInput struct {
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TaxCode     string  `json:"tax_code"`
}

type SalesOrderItemUpdate struct {
	Description *string  `json:"description"`
	Qty         *float64 `json:"qty"`
	Unit        *string  `json:"unit"`
	UnitPrice   *float64 `json:"unit_price"`
	TaxCode     *string  `json:"tax_code"`
}

type ConvertToInvoiceResult struct {
	SalesOrder *SalesOrder            `json:"sales_order"`
	Invoice    *accounting.InvoiceOut `json:"invoice"`
}

func NewService(pg *pgxpool.Pool, num *settings.NumberingService) *Service {
	return &Service{pg: pg, num: num}
}

func Statuses() []string {
	return []string{"open", "released", "invoiced", "completed", "canceled"}
}

func (s *Service) CreateFromQuote(ctx context.Context, quoteID uuid.UUID) (*SalesOrder, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	var projectID sql.NullString
	var contactID string
	var currency string
	var note string
	var linkedInvoiceID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	err = tx.QueryRow(ctx, `SELECT status, project_id::text, contact_id, currency, COALESCE(note,''), linked_invoice_out_id, linked_sales_order_id
		FROM quotes WHERE id=$1 FOR UPDATE`, quoteID).Scan(
		&status, &projectID, &contactID, &currency, &note, &linkedInvoiceID, &linkedSalesOrderID,
	)
	if err != nil {
		return nil, err
	}
	if status != "accepted" {
		return nil, errors.New("nur angenommene Angebote können in Aufträge überführt werden")
	}
	if linkedInvoiceID.Valid {
		return nil, errors.New("Angebot wurde bereits in eine Rechnung überführt")
	}
	if linkedSalesOrderID.Valid {
		return nil, errors.New("Angebot wurde bereits in einen Auftrag überführt")
	}

	rows, err := tx.Query(ctx, `SELECT description, qty, unit, unit_price, COALESCE(tax_code,'') FROM quote_items WHERE quote_id=$1 ORDER BY position`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SalesOrderItem, 0)
	var netAmount float64
	var taxAmount float64
	for rows.Next() {
		var item SalesOrderItem
		if err := rows.Scan(&item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode); err != nil {
			return nil, err
		}
		items = append(items, item)
		lineNet := item.Qty * item.UnitPrice
		netAmount += lineNet
		taxAmount += lineNet * taxRate(item.TaxCode)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("keine Positionen")
	}

	number, err := s.num.Next(ctx, "sales_order")
	if err != nil {
		return nil, err
	}
	orderID := uuid.New()
	orderDate := time.Now()
	grossAmount := netAmount + taxAmount

	_, err = tx.Exec(ctx, `INSERT INTO sales_orders (id, nummer, source_quote_id, project_id, contact_id, status, order_date, currency, note, net_amount, tax_amount, gross_amount)
		VALUES ($1,$2,$3,$4,$5,'open',$6,$7,$8,$9,$10,$11)`,
		orderID, number, quoteID, nullIfEmpty(projectID.String), contactID, orderDate, currency, note, netAmount, taxAmount, grossAmount)
	if err != nil {
		return nil, err
	}

	for idx, item := range items {
		lineID := uuid.New()
		lineNet := item.Qty * item.UnitPrice
		lineTax := lineNet * taxRate(item.TaxCode)
		_, err = tx.Exec(ctx, `INSERT INTO sales_order_items (id, sales_order_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			lineID, orderID, idx+1, item.Description, item.Qty, item.Unit, item.UnitPrice, lineNet, lineTax, nullIfEmpty(item.TaxCode))
		if err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE quotes SET linked_sales_order_id=$2 WHERE id=$1`, quoteID, orderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, orderID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*SalesOrder, error) {
	var out SalesOrder
	var projectID sql.NullString
	var linkedInvoiceOutID uuid.NullUUID
	err := s.pg.QueryRow(ctx, `SELECT so.id, so.nummer, so.source_quote_id, so.linked_invoice_out_id, so.project_id::text, COALESCE(p.name,''), so.contact_id, COALESCE(c.name,''), so.status, so.order_date, so.currency, COALESCE(so.note,''), so.net_amount, so.tax_amount, so.gross_amount
		FROM sales_orders so
		LEFT JOIN projects p ON p.id = so.project_id
		LEFT JOIN contacts c ON c.id = so.contact_id
		WHERE so.id=$1`, id).Scan(
		&out.ID, &out.Number, &out.SourceQuoteID, &linkedInvoiceOutID, &projectID, &out.ProjectName, &out.ContactID, &out.ContactName, &out.Status, &out.OrderDate, &out.Currency, &out.Note, &out.NetAmount, &out.TaxAmount, &out.GrossAmount,
	)
	if err != nil {
		return nil, err
	}
	if linkedInvoiceOutID.Valid {
		out.LinkedInvoiceOutID = linkedInvoiceOutID.UUID.String()
	}
	if projectID.Valid {
		out.ProjectID = projectID.String
	}
	rows, err := s.pg.Query(ctx, `SELECT id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'') FROM sales_order_items WHERE sales_order_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item SalesOrderItem
		if err := rows.Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	remainingByItem, err := remainingQtyByItem(ctx, s.pg, id)
	if err != nil {
		return nil, err
	}
	for i := range out.Items {
		out.Items[i].RemainingQty = remainingByItem[out.Items[i].ID]
		out.Items[i].InvoicedQty = out.Items[i].Qty - out.Items[i].RemainingQty
		if out.Items[i].InvoicedQty < 0 {
			out.Items[i].InvoicedQty = 0
		}
	}
	return &out, nil
}

func (s *Service) List(ctx context.Context, f SalesOrderFilter) ([]SalesOrderListItem, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args := make([]any, 0)
	conds := make([]string, 0)
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("so.status=$%d", len(args)))
	}
	if strings.TrimSpace(f.ContactID) != "" {
		args = append(args, f.ContactID)
		conds = append(conds, fmt.Sprintf("so.contact_id=$%d", len(args)))
	}
	if strings.TrimSpace(f.ProjectID) != "" {
		args = append(args, f.ProjectID)
		conds = append(conds, fmt.Sprintf("so.project_id::text=$%d", len(args)))
	}
	if strings.TrimSpace(f.Search) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(f.Search))+"%")
		conds = append(conds, fmt.Sprintf("(LOWER(so.nummer) LIKE $%d OR LOWER(COALESCE(c.name,'')) LIKE $%d)", len(args), len(args)))
	}
	args = append(args, f.Limit, f.Offset)
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	query := `SELECT so.id, so.nummer, so.source_quote_id, COALESCE(so.linked_invoice_out_id::text,''), COALESCE(so.project_id::text,''), COALESCE(p.name,''), so.contact_id, COALESCE(c.name,''), so.status, so.order_date, so.currency, so.gross_amount,
		COALESCE(inv_stats.invoice_count, 0),
		COALESCE(rem_stats.remaining_gross_amount, 0)
		FROM sales_orders so
		LEFT JOIN projects p ON p.id = so.project_id
		LEFT JOIN contacts c ON c.id = so.contact_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::int AS invoice_count
			FROM invoices_out io
			WHERE io.source_sales_order_id = so.id
		) inv_stats ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(
				GREATEST(soi.qty - COALESCE(inv_item.invoiced_qty, 0), 0) * soi.unit_price *
				(1 + CASE COALESCE(soi.tax_code, '') WHEN 'DE19' THEN 0.19 WHEN 'DE7' THEN 0.07 ELSE 0 END)
			), 0) AS remaining_gross_amount
			FROM sales_order_items soi
			LEFT JOIN (
				SELECT ioi.source_sales_order_item_id, SUM(ioi.qty) AS invoiced_qty
				FROM invoice_out_items ioi
				JOIN invoices_out io ON io.id = ioi.invoice_id
				WHERE io.source_sales_order_id = so.id
				GROUP BY ioi.source_sales_order_item_id
			) inv_item ON inv_item.source_sales_order_item_id = soi.id
			WHERE soi.sales_order_id = so.id
		) rem_stats ON true` + where + `
		ORDER BY so.order_date DESC, so.created_at DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SalesOrderListItem, 0)
	for rows.Next() {
		var item SalesOrderListItem
		if err := rows.Scan(&item.ID, &item.Number, &item.SourceQuoteID, &item.LinkedInvoiceOutID, &item.ProjectID, &item.ProjectName, &item.ContactID, &item.ContactName, &item.Status, &item.OrderDate, &item.Currency, &item.GrossAmount, &item.RelatedInvoiceCount, &item.RemainingGrossAmount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in SalesOrderUpdate) (*SalesOrder, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM sales_orders WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus)
	if err != nil {
		return nil, err
	}
	if !isEditableStatus(currentStatus) {
		return nil, errors.New("nur offene oder freigegebene Aufträge können bearbeitet werden")
	}

	netAmount, taxAmount, grossAmount, err := recalculateTotalsTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	sets := []string{"net_amount=$2", "tax_amount=$3", "gross_amount=$4"}
	args := []any{id, netAmount, taxAmount, grossAmount}
	argPos := 5
	add := func(field string, value any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", field, argPos))
		args = append(args, value)
		argPos++
	}
	if in.Number != nil {
		number := strings.TrimSpace(*in.Number)
		if number == "" {
			return nil, errors.New("Auftragsnummer erforderlich")
		}
		add("nummer", number)
	}
	if in.OrderDate != nil {
		add("order_date", *in.OrderDate)
	}
	if in.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*in.Currency))
		if currency == "" {
			return nil, errors.New("Währung erforderlich")
		}
		add("currency", currency)
	}
	if in.Note != nil {
		add("note", strings.TrimSpace(*in.Note))
	}

	if _, err := tx.Exec(ctx, `UPDATE sales_orders SET `+strings.Join(sets, ", ")+` WHERE id=$1`, args...); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) CreateItem(ctx context.Context, orderID uuid.UUID, in SalesOrderItemInput) (*SalesOrderItem, *SalesOrder, error) {
	if err := validateItemInput(in.Description, in.Qty, in.Unit, in.UnitPrice, in.TaxCode); err != nil {
		return nil, nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureOrderEditableTx(ctx, tx, orderID); err != nil {
		return nil, nil, err
	}

	var item SalesOrderItem
	itemID := uuid.New()
	taxCode := normalizeTaxCode(in.TaxCode)
	err = tx.QueryRow(ctx, `
		INSERT INTO sales_order_items (id, sales_order_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code)
		VALUES (
			$1, $2,
			(SELECT COALESCE(MAX(position), 0) + 1 FROM sales_order_items WHERE sales_order_id=$2),
			$3, $4, $5, $6, $7, $8, $9
		)
		RETURNING id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'')
	`,
		itemID,
		orderID,
		strings.TrimSpace(in.Description),
		in.Qty,
		strings.TrimSpace(in.Unit),
		in.UnitPrice,
		in.Qty*in.UnitPrice,
		in.Qty*in.UnitPrice*taxRate(taxCode),
		nullIfEmpty(taxCode),
	).Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode)
	if err != nil {
		return nil, nil, err
	}

	if err := refreshOrderTotalsTx(ctx, tx, orderID); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	order, err := s.Get(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return &item, order, nil
}

func (s *Service) UpdateItem(ctx context.Context, orderID, itemID uuid.UUID, in SalesOrderItemUpdate) (*SalesOrderItem, *SalesOrder, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureOrderEditableTx(ctx, tx, orderID); err != nil {
		return nil, nil, err
	}
	current, err := getItemTx(ctx, tx, orderID, itemID)
	if err != nil {
		return nil, nil, err
	}

	description := current.Description
	qty := current.Qty
	unit := current.Unit
	unitPrice := current.UnitPrice
	taxCode := current.TaxCode
	if in.Description != nil {
		description = strings.TrimSpace(*in.Description)
	}
	if in.Qty != nil {
		qty = *in.Qty
	}
	if in.Unit != nil {
		unit = strings.TrimSpace(*in.Unit)
	}
	if in.UnitPrice != nil {
		unitPrice = *in.UnitPrice
	}
	if in.TaxCode != nil {
		taxCode = normalizeTaxCode(*in.TaxCode)
	}
	if err := validateItemInput(description, qty, unit, unitPrice, taxCode); err != nil {
		return nil, nil, err
	}

	var item SalesOrderItem
	err = tx.QueryRow(ctx, `
		UPDATE sales_order_items
		SET description=$3, qty=$4, unit=$5, unit_price=$6, net_amount=$7, tax_amount=$8, tax_code=$9
		WHERE sales_order_id=$1 AND id=$2
		RETURNING id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'')
	`,
		orderID,
		itemID,
		description,
		qty,
		unit,
		unitPrice,
		qty*unitPrice,
		qty*unitPrice*taxRate(taxCode),
		nullIfEmpty(taxCode),
	).Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode)
	if err != nil {
		return nil, nil, err
	}

	if err := refreshOrderTotalsTx(ctx, tx, orderID); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	order, err := s.Get(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return &item, order, nil
}

func (s *Service) DeleteItem(ctx context.Context, orderID, itemID uuid.UUID) (*SalesOrder, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureOrderEditableTx(ctx, tx, orderID); err != nil {
		return nil, err
	}
	itemCount, err := countItemsTx(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if itemCount <= 1 {
		return nil, errors.New("Auftrag muss mindestens eine Position enthalten")
	}
	tag, err := tx.Exec(ctx, `DELETE FROM sales_order_items WHERE sales_order_id=$1 AND id=$2`, orderID, itemID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	if err := resequenceItemsTx(ctx, tx, orderID); err != nil {
		return nil, err
	}
	if err := refreshOrderTotalsTx(ctx, tx, orderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, orderID)
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*SalesOrder, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if !isStatus(status) {
		return nil, errors.New("ungültiger Auftragsstatus")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var linkedInvoiceOutID uuid.NullUUID
	err = tx.QueryRow(ctx, `SELECT status, linked_invoice_out_id FROM sales_orders WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus, &linkedInvoiceOutID)
	if err != nil {
		return nil, err
	}
	if err := validateStatusTransition(currentStatus, status, linkedInvoiceOutID.Valid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE sales_orders SET status=$2 WHERE id=$1`, id, status); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) ConvertToInvoice(ctx context.Context, id uuid.UUID, arSvc *accounting.ARService, in ConvertToInvoiceInput) (*ConvertToInvoiceResult, error) {
	if arSvc == nil {
		return nil, errors.New("invoice service fehlt")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, items, err := s.loadForInvoiceTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("keine Positionen")
	}
	remainingByItem, err := remainingQtyByItemTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	revenueAccount := strings.TrimSpace(in.RevenueAccount)
	if revenueAccount == "" {
		revenueAccount = "8000"
	}
	invoiceItems := make([]accounting.InvoiceItemInput, 0, len(items))
	selectedQtyByItemID, err := selectInvoiceQuantities(items, remainingByItem, in.Items)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		qtyToInvoice := selectedQtyByItemID[item.ID]
		if qtyToInvoice <= 0 {
			continue
		}
		sourceSalesOrderItemID, parseErr := uuid.Parse(item.ID)
		if parseErr != nil {
			return nil, parseErr
		}
		invoiceItems = append(invoiceItems, accounting.InvoiceItemInput{
			Description:            item.Description,
			Qty:                    qtyToInvoice,
			UnitPrice:              item.UnitPrice,
			TaxCode:                item.TaxCode,
			AccountCode:            revenueAccount,
			SourceSalesOrderItemID: &sourceSalesOrderItemID,
		})
	}
	if len(invoiceItems) == 0 {
		return nil, errors.New("Auftrag ist bereits vollständig fakturiert")
	}
	if in.InvoiceDate.IsZero() {
		in.InvoiceDate = time.Now()
	}
	invoice, err := arSvc.CreateFromSalesOrderTx(ctx, tx, order.ID, &order.SourceQuoteID, accounting.InvoiceOutInput{
		ContactID:   order.ContactID,
		InvoiceDate: in.InvoiceDate,
		DueDate:     in.DueDate,
		Currency:    order.Currency,
		Items:       invoiceItems,
	})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE sales_orders SET linked_invoice_out_id=$2, status='invoiced' WHERE id=$1`, id, invoice.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE quotes SET status='accepted', accepted_at=COALESCE(accepted_at, now()), linked_invoice_out_id=$2 WHERE id=$1`, order.SourceQuoteID, invoice.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	updatedOrder, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ConvertToInvoiceResult{
		SalesOrder: updatedOrder,
		Invoice:    invoice,
	}, nil
}

func (s *Service) loadForInvoiceTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*SalesOrder, []SalesOrderItem, error) {
	var order SalesOrder
	var projectID sql.NullString
	var linkedInvoiceOutID uuid.NullUUID
	err := tx.QueryRow(ctx, `SELECT so.id, so.nummer, so.source_quote_id, so.linked_invoice_out_id, so.project_id::text, COALESCE(p.name,''), so.contact_id, COALESCE(c.name,''), so.status, so.order_date, so.currency, COALESCE(so.note,''), so.net_amount, so.tax_amount, so.gross_amount
		FROM sales_orders so
		LEFT JOIN projects p ON p.id = so.project_id
		LEFT JOIN contacts c ON c.id = so.contact_id
		WHERE so.id=$1 FOR UPDATE`, id).Scan(
		&order.ID, &order.Number, &order.SourceQuoteID, &linkedInvoiceOutID, &projectID, &order.ProjectName, &order.ContactID, &order.ContactName, &order.Status, &order.OrderDate, &order.Currency, &order.Note, &order.NetAmount, &order.TaxAmount, &order.GrossAmount,
	)
	if err != nil {
		return nil, nil, err
	}
	if linkedInvoiceOutID.Valid {
		order.LinkedInvoiceOutID = linkedInvoiceOutID.UUID.String()
	}
	if projectID.Valid {
		order.ProjectID = projectID.String
	}
	switch order.Status {
	case "open", "released", "invoiced":
	default:
		return nil, nil, errors.New("nur offene oder freigegebene Aufträge können in Rechnungen überführt werden")
	}

	rows, err := tx.Query(ctx, `SELECT id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'') FROM sales_order_items WHERE sales_order_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]SalesOrderItem, 0)
	for rows.Next() {
		var item SalesOrderItem
		if err := rows.Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return &order, items, nil
}

func isStatus(status string) bool {
	for _, candidate := range Statuses() {
		if candidate == status {
			return true
		}
	}
	return false
}

func isEditableStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open", "released":
		return true
	default:
		return false
	}
}

func validateItemInput(description string, qty float64, unit string, unitPrice float64, taxCode string) error {
	if strings.TrimSpace(description) == "" {
		return errors.New("Positionsbeschreibung erforderlich")
	}
	if qty <= 0 {
		return errors.New("Menge muss größer als 0 sein")
	}
	if strings.TrimSpace(unit) == "" {
		return errors.New("Einheit erforderlich")
	}
	if unitPrice < 0 {
		return errors.New("Einzelpreis darf nicht negativ sein")
	}
	if !isSupportedTaxCode(taxCode) {
		return errors.New("Steuercode ungültig")
	}
	return nil
}

func isSupportedTaxCode(v string) bool {
	switch normalizeTaxCode(v) {
	case "", "DE19", "DE7":
		return true
	default:
		return false
	}
}

func validateStatusTransition(currentStatus, nextStatus string, hasInvoice bool) error {
	if currentStatus == nextStatus {
		return nil
	}
	switch currentStatus {
	case "open":
		switch nextStatus {
		case "released", "canceled":
			return nil
		}
	case "released":
		switch nextStatus {
		case "open", "canceled":
			return nil
		case "invoiced", "completed":
			if hasInvoice {
				return nil
			}
		}
	case "invoiced":
		if nextStatus == "completed" && hasInvoice {
			return nil
		}
	case "completed", "canceled":
		return errors.New("abgeschlossene oder stornierte Aufträge können nicht erneut umgestellt werden")
	}
	return errors.New("Auftragsstatus darf nicht in den gewünschten Status wechseln")
}

func taxRate(code string) float64 {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "DE19":
		return 0.19
	case "DE7":
		return 0.07
	default:
		return 0
	}
}

func recalculateTotalsTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (float64, float64, float64, error) {
	rows, err := tx.Query(ctx, `SELECT qty, unit_price, COALESCE(tax_code,'') FROM sales_order_items WHERE sales_order_id=$1`, orderID)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()

	var netAmount float64
	var taxAmount float64
	var count int
	for rows.Next() {
		var qty float64
		var unitPrice float64
		var taxCode string
		if err := rows.Scan(&qty, &unitPrice, &taxCode); err != nil {
			return 0, 0, 0, err
		}
		lineNet := qty * unitPrice
		netAmount += lineNet
		taxAmount += lineNet * taxRate(taxCode)
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	if count == 0 {
		return 0, 0, 0, errors.New("keine Positionen")
	}
	return netAmount, taxAmount, netAmount + taxAmount, nil
}

func refreshOrderTotalsTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	netAmount, taxAmount, grossAmount, err := recalculateTotalsTx(ctx, tx, orderID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE sales_orders SET net_amount=$2, tax_amount=$3, gross_amount=$4 WHERE id=$1`, orderID, netAmount, taxAmount, grossAmount)
	return err
}

func ensureOrderEditableTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM sales_orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&status); err != nil {
		return err
	}
	if !isEditableStatus(status) {
		return errors.New("nur offene oder freigegebene Aufträge können bearbeitet werden")
	}
	return nil
}

func getItemTx(ctx context.Context, tx pgx.Tx, orderID, itemID uuid.UUID) (*SalesOrderItem, error) {
	var item SalesOrderItem
	err := tx.QueryRow(ctx, `SELECT id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'') FROM sales_order_items WHERE sales_order_id=$1 AND id=$2`, orderID, itemID).
		Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func countItemsTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM sales_order_items WHERE sales_order_id=$1`, orderID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func resequenceItemsTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		WITH ordered AS (
			SELECT id, ROW_NUMBER() OVER (ORDER BY position, id) AS new_position
			FROM sales_order_items
			WHERE sales_order_id=$1
		)
		UPDATE sales_order_items soi
		SET position=ordered.new_position
		FROM ordered
		WHERE soi.id=ordered.id
	`, orderID)
	return err
}

func normalizeTaxCode(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

func remainingQtyByItem(ctx context.Context, pg *pgxpool.Pool, orderID uuid.UUID) (map[string]float64, error) {
	rows, err := pg.Query(ctx, `
		SELECT soi.id::text, soi.qty, COALESCE(SUM(ioi.qty), 0)
		FROM sales_order_items soi
		LEFT JOIN invoice_out_items ioi ON ioi.source_sales_order_item_id = soi.id
		LEFT JOIN invoices_out io ON io.id = ioi.invoice_id AND io.source_sales_order_id = soi.sales_order_id
		WHERE soi.sales_order_id = $1
		GROUP BY soi.id, soi.qty
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var itemID string
		var qty float64
		var invoicedQty float64
		if err := rows.Scan(&itemID, &qty, &invoicedQty); err != nil {
			return nil, err
		}
		remaining := qty - invoicedQty
		if remaining < 0 {
			remaining = 0
		}
		out[itemID] = remaining
	}
	return out, rows.Err()
}

func remainingQtyByItemTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (map[string]float64, error) {
	rows, err := tx.Query(ctx, `
		SELECT soi.id::text, soi.qty, COALESCE(SUM(ioi.qty), 0)
		FROM sales_order_items soi
		LEFT JOIN invoice_out_items ioi ON ioi.source_sales_order_item_id = soi.id
		LEFT JOIN invoices_out io ON io.id = ioi.invoice_id AND io.source_sales_order_id = soi.sales_order_id
		WHERE soi.sales_order_id = $1
		GROUP BY soi.id, soi.qty
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var itemID string
		var qty float64
		var invoicedQty float64
		if err := rows.Scan(&itemID, &qty, &invoicedQty); err != nil {
			return nil, err
		}
		remaining := qty - invoicedQty
		if remaining < 0 {
			remaining = 0
		}
		out[itemID] = remaining
	}
	return out, rows.Err()
}

func selectInvoiceQuantities(items []SalesOrderItem, remainingByItem map[string]float64, selected []ConvertToInvoiceItemInput) (map[string]float64, error) {
	out := make(map[string]float64, len(items))
	if len(selected) == 0 {
		for _, item := range items {
			out[item.ID] = remainingByItem[item.ID]
		}
		return out, nil
	}

	itemsByID := make(map[string]SalesOrderItem, len(items))
	for _, item := range items {
		itemsByID[item.ID] = item
	}
	for _, req := range selected {
		itemID := strings.TrimSpace(req.SalesOrderItemID)
		if itemID == "" {
			return nil, errors.New("Auftragspositions-ID erforderlich")
		}
		if _, ok := itemsByID[itemID]; !ok {
			return nil, errors.New("Auftragsposition nicht gefunden")
		}
		if req.Qty <= 0 {
			return nil, errors.New("Teilfaktura-Menge muss größer als 0 sein")
		}
		remaining := remainingByItem[itemID]
		if remaining <= 0.0001 {
			return nil, errors.New("Auftragsposition ist bereits vollständig fakturiert")
		}
		if req.Qty > remaining+0.0001 {
			return nil, errors.New("Teilfaktura-Menge überschreitet die offene Restmenge")
		}
		out[itemID] = req.Qty
	}
	return out, nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
