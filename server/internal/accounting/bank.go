package accounting

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type BankStatementInput struct {
	BookingDate  time.Time      `json:"booking_date"`
	ValueDate    *time.Time     `json:"value_date,omitempty"`
	Amount       float64        `json:"amount"`
	Currency     string         `json:"currency"`
	Counterparty string         `json:"counterparty"`
	Reference    string         `json:"reference"`
	Raw          map[string]any `json:"raw"`
	InvoiceID    *uuid.UUID     `json:"invoice_id,omitempty"`
}

type BankService struct {
	pg       *pgxpool.Pool
	payments *PaymentService
}

func NewBankService(pg *pgxpool.Pool, payments *PaymentService) *BankService {
	return &BankService{pg: pg, payments: payments}
}

func (s *BankService) Ingest(ctx context.Context, in BankStatementInput, companyID string) (uuid.UUID, error) {
	if strings.TrimSpace(companyID) == "" {
		return uuid.Nil, errors.New("Mandant erforderlich")
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
	if in.BookingDate.IsZero() {
		in.BookingDate = time.Now()
	}
	id := uuid.New()
	// in.Raw ist vom statischen Typ map[string]any - ein Vergleich der Form
	// "var raw any = in.Raw; if raw == nil" greift NIE, da eine nil-Map, die
	// in ein any-Interface verpackt wird, ein Interface mit gesetztem Typ
	// (aber nil-Wert) ergibt, das != nil ist (klassische Go-"typed nil in
	// interface"-Falle). Dadurch blieb raw bei jedem Ingest ohne explizites
	// Raw-Feld eine nil-Map, die pgx als SQL NULL sendet - Verletzung von
	// bank_statements.raw NOT NULL bei JEDEM manuell erfassten Kontoauszug
	// (Backlog 0.9, gefunden beim Schreiben der ersten Integrationstests
	// fuer Ingest). Fix: den Nil-Check auf die Map VOR dem Verpacken in ein
	// Interface anwenden.
	raw := in.Raw
	if raw == nil {
		raw = map[string]any{}
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO bank_statements (id, booking_date, value_date, amount, currency, counterparty, reference, raw, company_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, in.BookingDate, in.ValueDate, in.Amount, in.Currency, in.Counterparty, in.Reference, raw, companyID)
	if err != nil {
		return uuid.Nil, err
	}
	if in.InvoiceID != nil {
		_, err = s.payments.Apply(ctx, PaymentInput{
			InvoiceID: *in.InvoiceID,
			Amount:    in.Amount,
			Currency:  in.Currency,
			Method:    "bank",
			Reference: in.Reference,
			Date:      in.BookingDate,
		}, companyID)
		if err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *BankService) List(ctx context.Context, limit int, companyID string) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `SELECT id, booking_date, value_date, amount, currency, counterparty, reference, raw, matched_payment_id FROM bank_statements WHERE company_id=$1 ORDER BY booking_date DESC, created_at DESC LIMIT $2`, companyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var booking time.Time
		var value sql.NullTime
		var amount float64
		var currency, counterparty, reference string
		var raw map[string]any
		var matched sql.NullString
		if err := rows.Scan(&id, &booking, &value, &amount, &currency, &counterparty, &reference, &raw, &matched); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id":           id,
			"booking_date": booking,
			"amount":       amount,
			"currency":     currency,
			"counterparty": counterparty,
			"reference":    reference,
			"raw":          raw,
		}
		if value.Valid {
			m["value_date"] = value.Time
		}
		if matched.Valid {
			m["matched_payment_id"] = matched.String
		}
		out = append(out, m)
	}
	return out, nil
}

// Match tries to link a bank statement to an invoice (manual if invoiceID given, simple heuristic otherwise)
func (s *BankService) Match(ctx context.Context, statementID uuid.UUID, invoiceID *uuid.UUID, companyID string) (uuid.UUID, error) {
	if strings.TrimSpace(companyID) == "" {
		return uuid.Nil, errors.New("Mandant erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var amt float64
	var currency string
	var matched sql.NullString
	var reference string
	err = tx.QueryRow(ctx, `SELECT amount, currency, matched_payment_id, reference FROM bank_statements WHERE id=$1 AND company_id=$2 FOR UPDATE`, statementID, companyID).
		Scan(&amt, &currency, &matched, &reference)
	if err != nil {
		return uuid.Nil, err
	}
	if matched.Valid {
		return uuid.Nil, errors.New("Statement bereits gematcht")
	}
	var target uuid.UUID
	if invoiceID != nil {
		target = *invoiceID
	} else {
		if idFromRef := s.findInvoiceIDInReference(ctx, tx, reference, currency, companyID); idFromRef != uuid.Nil {
			target = idFromRef
		} else {
			target, err = s.findInvoiceByAmount(ctx, tx, amt, currency, companyID)
			if err != nil {
				return uuid.Nil, err
			}
		}
	}
	pay, err := s.payments.apply(ctx, tx, PaymentInput{
		InvoiceID: target,
		Amount:    amt,
		Currency:  currency,
		Method:    "bank",
		Reference: "Match",
		Date:      time.Now(),
	}, companyID)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE bank_statements SET matched_payment_id=$2 WHERE id=$1`, statementID, pay.ID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return pay.ID, nil
}

func (s *BankService) findInvoiceByAmount(ctx context.Context, tx pgx.Tx, amount float64, currency string, companyID string) (uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT id, nummer FROM invoices_out WHERE status IN ('booked','partial') AND currency=$1 AND company_id=$2 AND ABS((gross_amount - paid_amount) - $3) < 0.01`, currency, companyID, amount)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var num sql.NullString
		if err := rows.Scan(&id, &num); err != nil {
			return uuid.Nil, err
		}
		_ = num
		ids = append(ids, id)
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	if len(ids) == 0 {
		return uuid.Nil, errors.New("Kein passender offener Posten gefunden")
	}
	return uuid.Nil, errors.New("Mehrere mögliche offene Posten gefunden, bitte manuell zuordnen")
}

// find by invoice number in reference (simple regex)
func (s *BankService) findInvoiceIDInReference(ctx context.Context, tx pgx.Tx, reference, currency string, companyID string) uuid.UUID {
	ref := strings.ToLower(reference)
	if ref == "" {
		return uuid.Nil
	}
	// Das erste Muster erkennt explizit ein "RE"-Praefix, faengt aber nur
	// den nachfolgenden Zahlenteil ein - invoices_out.nummer enthaelt laut
	// Nummernkreis-Pattern (017_accounting_basics.sql: 'RE-{YYYY}-{NNNN}')
	// immer das volle "RE-"-Praefix. Ohne den Praefix beim ersten Muster
	// wieder anzuhaengen fand diese Suche NIE eine reale Rechnung
	// (Backlog 0.9, gefunden beim Schreiben der ersten Integrationstests
	// fuer den Referenz-Erkennungs-Pfad).
	patterns := []struct {
		rx     *regexp.Regexp
		prefix string
	}{
		{regexp.MustCompile(`re[-\s]?(\d{2,4}[-/]?\d{2,6})`), "RE-"},
		{regexp.MustCompile(`(\d{4,10})`), ""},
	}
	for _, p := range patterns {
		m := p.rx.FindStringSubmatch(ref)
		if len(m) > 1 {
			num := p.prefix + m[1]
			var id uuid.UUID
			err := tx.QueryRow(ctx, `SELECT id FROM invoices_out WHERE LOWER(nummer)=LOWER($1) AND currency=$2 AND company_id=$3 LIMIT 1`, num, currency, companyID).Scan(&id)
			if err == nil {
				return id
			}
		}
	}
	return uuid.Nil
}
