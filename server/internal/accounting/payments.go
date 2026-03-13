package accounting

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentInput struct {
	InvoiceID uuid.UUID `json:"invoice_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Method    string    `json:"method"` // bank | cash | other
	Reference string    `json:"reference"`
	Date      time.Time `json:"date"`
}

type Payment struct {
	ID        uuid.UUID `json:"id"`
	InvoiceID uuid.UUID `json:"invoice_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Method    string    `json:"method"`
	Reference string    `json:"reference"`
	PaidAt    time.Time `json:"paid_at"`
}

type PaymentService struct {
	pg      *pgxpool.Pool
	journal *JournalService
}

func NewPaymentService(pg *pgxpool.Pool, journal *JournalService) *PaymentService {
	return &PaymentService{pg: pg, journal: journal}
}

func (s *PaymentService) Apply(ctx context.Context, in PaymentInput) (*Payment, error) {
	return s.apply(ctx, nil, in)
}

// apply allows reusing an existing transaction when provided.
func (s *PaymentService) apply(ctx context.Context, tx pgx.Tx, in PaymentInput) (*Payment, error) {
	var err error
	if in.Amount <= 0 {
		return nil, errors.New("Betrag muss > 0 sein")
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
	if in.Method == "" {
		in.Method = "bank"
	}
	if in.Date.IsZero() {
		in.Date = time.Now()
	}
	commit := false
	if tx == nil {
		txx, err := s.pg.Begin(ctx)
		if err != nil {
			return nil, err
		}
		tx = txx
		defer tx.Rollback(ctx)
		commit = true
	}
	var status string
	var currency string
	var gross, paid float64
	err = tx.QueryRow(ctx, `SELECT status, currency, gross_amount, paid_amount FROM invoices_out WHERE id=$1 FOR UPDATE`, in.InvoiceID).Scan(&status, &currency, &gross, &paid)
	if err != nil {
		return nil, err
	}
	if status != "booked" && status != "partial" && status != "paid" {
		return nil, errors.New("Rechnung ist nicht gebucht")
	}
	if currency != in.Currency {
		return nil, errors.New("Währung stimmt nicht mit Rechnung überein")
	}
	open := gross - paid
	if in.Amount > open+0.0001 {
		return nil, errors.New("Zahlung übersteigt offenen Betrag")
	}
	entry, err := s.journal.CreateTx(ctx, tx, paymentJournal(in, in.InvoiceID))
	if err != nil {
		return nil, err
	}
	payID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO invoice_out_payments (id, invoice_id, amount, currency, method, reference, paid_at, journal_entry_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		payID, in.InvoiceID, in.Amount, in.Currency, in.Method, in.Reference, in.Date, entry.ID)
	if err != nil {
		return nil, err
	}
	newPaid := paid + in.Amount
	newStatus := "partial"
	if newPaid >= gross-0.0001 {
		newStatus = "paid"
	}
	_, err = tx.Exec(ctx, `UPDATE invoices_out SET paid_amount=$2, status=$3 WHERE id=$1`, in.InvoiceID, newPaid, newStatus)
	if err != nil {
		return nil, err
	}
	if commit {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return &Payment{
		ID:        payID,
		InvoiceID: in.InvoiceID,
		Amount:    in.Amount,
		Currency:  in.Currency,
		Method:    in.Method,
		Reference: in.Reference,
		PaidAt:    in.Date,
	}, nil
}

func (s *PaymentService) List(ctx context.Context, invoiceID uuid.UUID) ([]Payment, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, invoice_id, amount, currency, method, reference, paid_at FROM invoice_out_payments WHERE invoice_id=$1 ORDER BY paid_at DESC`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Payment, 0)
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.InvoiceID, &p.Amount, &p.Currency, &p.Method, &p.Reference, &p.PaidAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func paymentJournal(in PaymentInput, invoiceID uuid.UUID) JournalEntryInput {
	bankAccount := "1200"
	if in.Method == "cash" {
		bankAccount = "1000"
	}
	return JournalEntryInput{
		Date:        in.Date,
		Description: "Zahlung AR " + invoiceID.String(),
		Currency:    in.Currency,
		Source:      "payment_out",
		SourceID:    invoiceID.String(),
		Lines: []JournalLineInput{
			{AccountCode: bankAccount, Debit: in.Amount, Credit: 0, Memo: in.Reference},
			{AccountCode: "1400", Debit: 0, Credit: in.Amount, Memo: "Ausgleich AR"},
		},
	}
}
