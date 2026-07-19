package settings

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type QuoteCalculationSettings struct {
	ID                  string    `json:"id"`
	TargetMarginPercent float64   `json:"target_margin_percent"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type QuoteCalculationSettingsService struct{ pg *pgxpool.Pool }

func NewQuoteCalculationSettingsService(pg *pgxpool.Pool) *QuoteCalculationSettingsService {
	return &QuoteCalculationSettingsService{pg: pg}
}

func (s *QuoteCalculationSettingsService) Get(ctx context.Context) (*QuoteCalculationSettings, error) {
	var out QuoteCalculationSettings
	err := s.pg.QueryRow(ctx, `
		SELECT id, target_margin_percent, updated_at
		FROM quote_calculation_settings
		WHERE id = 'default'
	`).Scan(&out.ID, &out.TargetMarginPercent, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *QuoteCalculationSettingsService) Upsert(ctx context.Context, in QuoteCalculationSettings) error {
	targetMarginPercent := in.TargetMarginPercent
	if targetMarginPercent == 0 {
		targetMarginPercent = 20
	}
	if targetMarginPercent < 0 || targetMarginPercent > 1000 {
		return errors.New("Zielmarge ist ungueltig")
	}

	_, err := s.pg.Exec(ctx, `
		INSERT INTO quote_calculation_settings (
			id,
			target_margin_percent,
			updated_at
		) VALUES (
			'default',
			$1,
			now()
		)
		ON CONFLICT (id) DO UPDATE SET
			target_margin_percent = EXCLUDED.target_margin_percent,
			updated_at = now()
	`, targetMarginPercent)
	return err
}
