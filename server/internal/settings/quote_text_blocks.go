package settings

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuoteTextBlock struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Body      string `json:"body"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

type QuoteTextBlockService struct{ pg *pgxpool.Pool }

func NewQuoteTextBlockService(pg *pgxpool.Pool) *QuoteTextBlockService {
	return &QuoteTextBlockService{pg: pg}
}

func (s *QuoteTextBlockService) List(ctx context.Context) ([]QuoteTextBlock, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, code, name, category, body, sort_order, is_active
		FROM quote_text_blocks
		ORDER BY category ASC, sort_order ASC, name ASC, code ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]QuoteTextBlock, 0)
	for rows.Next() {
		var item QuoteTextBlock
		if err := rows.Scan(
			&item.ID,
			&item.Code,
			&item.Name,
			&item.Category,
			&item.Body,
			&item.SortOrder,
			&item.IsActive,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *QuoteTextBlockService) Upsert(ctx context.Context, in QuoteTextBlock) error {
	if in.Code = strings.ToLower(trim(in.Code)); in.Code == "" {
		return errors.New("code erforderlich")
	}
	if in.Name = trim(in.Name); in.Name == "" {
		return errors.New("name erforderlich")
	}
	if in.Category = normalizeQuoteTextBlockCategory(in.Category); in.Category == "" {
		return errors.New("category erforderlich")
	}
	if !isAllowedQuoteTextBlockCategory(in.Category) {
		return errors.New("ungültige category")
	}
	if in.Body = trim(in.Body); in.Body == "" {
		return errors.New("body erforderlich")
	}
	if in.ID = trim(in.ID); in.ID == "" {
		in.ID = uuid.NewString()
	} else if _, err := uuid.Parse(in.ID); err != nil {
		return errors.New("ungültige id")
	}
	_, err := s.pg.Exec(ctx, `
		INSERT INTO quote_text_blocks (id, code, name, category, body, sort_order, is_active, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (code) DO UPDATE
		SET name = EXCLUDED.name,
			category = EXCLUDED.category,
			body = EXCLUDED.body,
			sort_order = EXCLUDED.sort_order,
			is_active = EXCLUDED.is_active,
			updated_at = now()
	`, in.ID, in.Code, in.Name, in.Category, in.Body, in.SortOrder, in.IsActive)
	return err
}

func (s *QuoteTextBlockService) Delete(ctx context.Context, id string) error {
	if id = trim(id); id == "" {
		return errors.New("id erforderlich")
	}
	if _, err := uuid.Parse(id); err != nil {
		return errors.New("ungültige id")
	}
	_, err := s.pg.Exec(ctx, `DELETE FROM quote_text_blocks WHERE id = $1`, id)
	return err
}

func normalizeQuoteTextBlockCategory(v string) string {
	return strings.ToLower(trim(v))
}

func isAllowedQuoteTextBlockCategory(v string) bool {
	switch v {
	case "intro", "scope", "closing", "legal":
		return true
	default:
		return false
	}
}
