package accounting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/settings"
)

type InvoiceItemInput struct {
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	UnitPrice   float64 `json:"unit_price"`
	TaxCode     string  `json:"tax_code"`
	AccountCode string  `json:"account_code"`
}

type InvoiceOutInput struct {
	ContactID   string             `json:"contact_id"`
	InvoiceDate time.Time          `json:"invoice_date"`
	DueDate     *time.Time         `json:"due_date,omitempty"`
	Currency    string             `json:"currency"`
	Items       []InvoiceItemInput `json:"items"`
}

type InvoiceOut struct {
	ID          uuid.UUID          `json:"id"`
	Number      *string            `json:"number,omitempty"`
	Status      string             `json:"status"`
	ContactID   string             `json:"contact_id"`
	InvoiceDate time.Time          `json:"invoice_date"`
	DueDate     *time.Time         `json:"due_date,omitempty"`
	Currency    string             `json:"currency"`
	NetAmount   float64            `json:"net_amount"`
	TaxAmount   float64            `json:"tax_amount"`
	GrossAmount float64            `json:"gross_amount"`
	PaidAmount  float64            `json:"paid_amount"`
	Items       []InvoiceItemInput `json:"items"`
}

type InvoiceListItem struct {
	ID          uuid.UUID  `json:"id"`
	Number      *string    `json:"number,omitempty"`
	Status      string     `json:"status"`
	ContactID   string     `json:"contact_id"`
	ContactName string     `json:"contact_name"`
	InvoiceDate time.Time  `json:"invoice_date"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Currency    string     `json:"currency"`
	GrossAmount float64    `json:"gross_amount"`
	PaidAmount  float64    `json:"paid_amount"`
}

type InvoiceFilter struct {
	Status    string
	ContactID string
	Search    string
	Limit     int
	Offset    int
}

type ARService struct {
	pg      *pgxpool.Pool
	num     *settings.NumberingService
	journal *JournalService
}

func NewARService(pg *pgxpool.Pool, num *settings.NumberingService, journal *JournalService) *ARService {
	return &ARService{pg: pg, num: num, journal: journal}
}

func (s *ARService) Get(ctx context.Context, id uuid.UUID) (*InvoiceOut, error) {
	var inv InvoiceOut
	var number sql.NullString
	var due sql.NullTime
	err := s.pg.QueryRow(ctx, `SELECT id, nummer, status, contact_id, invoice_date, due_date, currency, net_amount, tax_amount, gross_amount, paid_amount
		FROM invoices_out WHERE id=$1`, id).Scan(
		&inv.ID, &number, &inv.Status, &inv.ContactID, &inv.InvoiceDate, &due, &inv.Currency, &inv.NetAmount, &inv.TaxAmount, &inv.GrossAmount, &inv.PaidAmount,
	)
	if err != nil {
		return nil, err
	}
	if number.Valid {
		inv.Number = &number.String
	}
	if due.Valid {
		t := due.Time
		inv.DueDate = &t
	}
	rows, err := s.pg.Query(ctx, `SELECT description, qty, unit_price, tax_code, account_code FROM invoice_out_items WHERE invoice_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it InvoiceItemInput
		if err := rows.Scan(&it.Description, &it.Qty, &it.UnitPrice, &it.TaxCode, &it.AccountCode); err != nil {
			return nil, err
		}
		inv.Items = append(inv.Items, it)
	}
	return &inv, nil
}

func (s *ARService) List(ctx context.Context, f InvoiceFilter) ([]InvoiceListItem, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args := make([]any, 0)
	conds := make([]string, 0)
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("i.status=$%d", len(args)))
	}
	if f.ContactID != "" {
		args = append(args, f.ContactID)
		conds = append(conds, fmt.Sprintf("i.contact_id=$%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
		conds = append(conds, fmt.Sprintf("(LOWER(i.nummer) LIKE $%d OR LOWER(i.id::text) LIKE $%d)", len(args), len(args)))
	}
	args = append(args, f.Limit)
	args = append(args, f.Offset)
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	query := `
SELECT i.id, i.nummer, i.status, i.contact_id, COALESCE(c.name,''), i.invoice_date, i.due_date, i.currency, i.gross_amount, i.paid_amount
FROM invoices_out i
LEFT JOIN contacts c ON c.id = i.contact_id
` + where + `
ORDER BY i.invoice_date DESC, i.created_at DESC
LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InvoiceListItem, 0)
	for rows.Next() {
		var it InvoiceListItem
		var num sql.NullString
		var due sql.NullTime
		if err := rows.Scan(&it.ID, &num, &it.Status, &it.ContactID, &it.ContactName, &it.InvoiceDate, &due, &it.Currency, &it.GrossAmount, &it.PaidAmount); err != nil {
			return nil, err
		}
		if num.Valid {
			it.Number = &num.String
		}
		if due.Valid {
			t := due.Time
			it.DueDate = &t
		}
		out = append(out, it)
	}
	return out, nil
}

// Create draft invoice
func (s *ARService) Create(ctx context.Context, in InvoiceOutInput) (*InvoiceOut, error) {
	if in.ContactID == "" {
		return nil, errors.New("contact_id fehlt")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("keine Positionen")
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
	if in.InvoiceDate.IsZero() {
		in.InvoiceDate = time.Now()
	}
	id := uuid.New()
	netSum, taxSum := calcTotals(in.Items)
	gross := netSum + taxSum
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO invoices_out (id, contact_id, status, invoice_date, due_date, currency, net_amount, tax_amount, gross_amount)
	VALUES ($1,$2,'draft',$3,$4,$5,$6,$7,$8)`,
		id, in.ContactID, in.InvoiceDate, in.DueDate, in.Currency, netSum, taxSum, gross)
	if err != nil {
		return nil, err
	}
	for idx, it := range in.Items {
		lineID := uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO invoice_out_items (id, invoice_id, position, description, qty, unit_price, net_amount, tax_amount, tax_code, account_code)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			lineID, id, idx+1, it.Description, it.Qty, it.UnitPrice, it.Qty*it.UnitPrice, it.UnitPrice*it.Qty*taxRate(it.TaxCode), it.TaxCode, it.AccountCode)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &InvoiceOut{
		ID:          id,
		Status:      "draft",
		ContactID:   in.ContactID,
		InvoiceDate: in.InvoiceDate,
		DueDate:     in.DueDate,
		Currency:    in.Currency,
		NetAmount:   netSum,
		TaxAmount:   taxSum,
		GrossAmount: gross,
		Items:       in.Items,
	}, nil
}

// Book moves draft to booked, assigns number and creates journal entry (AR 1400 / revenue + USt)
func (s *ARService) Book(ctx context.Context, id uuid.UUID) (*InvoiceOut, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var inv InvoiceOut
	var journalID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id, nummer, status, contact_id, invoice_date, due_date, currency, net_amount, tax_amount, gross_amount, journal_entry_id, paid_amount
		FROM invoices_out WHERE id=$1 FOR UPDATE`, id).Scan(
		&inv.ID, &inv.Number, &inv.Status, &inv.ContactID, &inv.InvoiceDate, &inv.DueDate, &inv.Currency, &inv.NetAmount, &inv.TaxAmount, &inv.GrossAmount, &journalID, &inv.PaidAmount,
	)
	if err != nil {
		return nil, err
	}
	if inv.Status != "draft" {
		return nil, errors.New("Rechnung ist nicht im Status draft")
	}
	// load items
	rows, err := tx.Query(ctx, `SELECT description, qty, unit_price, tax_code, account_code FROM invoice_out_items WHERE invoice_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]InvoiceItemInput, 0)
	for rows.Next() {
		var it InvoiceItemInput
		if err := rows.Scan(&it.Description, &it.Qty, &it.UnitPrice, &it.TaxCode, &it.AccountCode); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	inv.Items = items

	nummer, err := s.num.Next(ctx, "invoice_out")
	if err != nil {
		return nil, err
	}
	entry, err := s.journal.CreateTx(ctx, tx, buildJournal(inv, nummer, items))
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE invoices_out SET status='booked', nummer=$2, journal_entry_id=$3 WHERE id=$1`, id, nummer, entry.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	inv.Status = "booked"
	inv.Number = &nummer
	return &inv, nil
}

func buildJournal(inv InvoiceOut, nummer string, items []InvoiceItemInput) JournalEntryInput {
	desc := "AR " + nummer
	debit := JournalLineInput{AccountCode: "1400", Debit: inv.GrossAmount, Credit: 0, Memo: "Forderung"}
	lines := []JournalLineInput{debit}
	for _, it := range items {
		net := it.Qty * it.UnitPrice
		tax := net * taxRate(it.TaxCode)
		lines = append(lines, JournalLineInput{
			AccountCode: it.AccountCode,
			Debit:       0,
			Credit:      net,
			Memo:        it.Description,
		})
		if tax > 0.0001 && it.TaxCode != "" {
			lines = append(lines, JournalLineInput{
				AccountCode: taxAccountFor(it.TaxCode),
				Debit:       0,
				Credit:      tax,
				Memo:        "USt",
			})
		}
	}
	return JournalEntryInput{
		Date:        inv.InvoiceDate,
		Description: desc,
		Currency:    inv.Currency,
		Source:      "invoice_out",
		SourceID:    inv.ID.String(),
		Lines:       lines,
	}
}

func calcTotals(items []InvoiceItemInput) (net, tax float64) {
	for _, it := range items {
		n := it.Qty * it.UnitPrice
		net += n
		tax += n * taxRate(it.TaxCode)
	}
	return
}

func taxRate(code string) float64 {
	switch code {
	case "DE19":
		return 0.19
	case "DE7":
		return 0.07
	default:
		return 0
	}
}

func taxAccountFor(code string) string {
	switch code {
	case "DE19":
		return "1776"
	case "DE7":
		return "1771"
	default:
		return "1776"
	}
}
