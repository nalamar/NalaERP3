package accounting

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JournalLineInput struct {
	AccountCode string  `json:"account_code"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	Memo        string  `json:"memo"`
}

type JournalEntryInput struct {
	Date        time.Time          `json:"date"`
	Description string             `json:"description"`
	Currency    string             `json:"currency"`
	Source      string             `json:"source"`
	SourceID    string             `json:"source_id"`
	Lines       []JournalLineInput `json:"lines"`
}

type JournalEntry struct {
	ID          uuid.UUID          `json:"id"`
	Date        time.Time          `json:"date"`
	Description string             `json:"description"`
	Currency    string             `json:"currency"`
	Source      string             `json:"source"`
	SourceID    string             `json:"source_id"`
	Lines       []JournalLineInput `json:"lines"`
}

type JournalService struct{ pg *pgxpool.Pool }

func NewJournalService(pg *pgxpool.Pool) *JournalService { return &JournalService{pg: pg} }

func (s *JournalService) Create(ctx context.Context, in JournalEntryInput) (*JournalEntry, error) {
	return s.create(ctx, nil, in)
}

func (s *JournalService) CreateTx(ctx context.Context, tx pgx.Tx, in JournalEntryInput) (*JournalEntry, error) {
	return s.create(ctx, tx, in)
}

// Create within optional transaction
func (s *JournalService) create(ctx context.Context, tx pgx.Tx, in JournalEntryInput) (*JournalEntry, error) {
	if len(in.Lines) == 0 {
		return nil, errors.New("keine Buchungszeilen")
	}
	var debit, credit float64
	for _, l := range in.Lines {
		debit += l.Debit
		credit += l.Credit
		if l.AccountCode == "" {
			return nil, errors.New("account_code fehlt")
		}
	}
	if abs(debit-credit) > 0.0001 {
		return nil, errors.New("Soll/Haben nicht ausgeglichen")
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
	if in.Description == "" {
		in.Description = "Buchung"
	}
	id := uuid.New()
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
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries (id, entry_date, description, currency, source, source_id) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, in.Date, in.Description, in.Currency, in.Source, in.SourceID)
	if err != nil {
		return nil, err
	}
	for i, l := range in.Lines {
		lineID := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO journal_lines (id, entry_id, account_code, debit, credit, memo) VALUES ($1,$2,$3,$4,$5,$6)`,
			lineID, id, l.AccountCode, l.Debit, l.Credit, l.Memo); err != nil {
			return nil, err
		}
		_ = i
	}
	if commit {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return &JournalEntry{
		ID:          id,
		Date:        in.Date,
		Description: in.Description,
		Currency:    in.Currency,
		Source:      in.Source,
		SourceID:    in.SourceID,
		Lines:       in.Lines,
	}, nil
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
