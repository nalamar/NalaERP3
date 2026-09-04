package settings

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type QuoteCalculationSettings struct {
	ID                                  string    `json:"id"`
	TargetMarginPercent                 float64   `json:"target_margin_percent"`
	DefaultMaterialZuschlagPercent      float64   `json:"default_material_zuschlag_percent"`
	DefaultLohnZuschlagPercent          float64   `json:"default_lohn_zuschlag_percent"`
	DefaultFremdleistungZuschlagPercent float64   `json:"default_fremdleistung_zuschlag_percent"`
	DefaultLohnStundensatz              float64   `json:"default_lohn_stundensatz"`
	UpdatedAt                           time.Time `json:"updated_at"`
}

type QuoteCalculationSettingsService struct{ pg *pgxpool.Pool }

func NewQuoteCalculationSettingsService(pg *pgxpool.Pool) *QuoteCalculationSettingsService {
	return &QuoteCalculationSettingsService{pg: pg}
}

func (s *QuoteCalculationSettingsService) Get(ctx context.Context) (*QuoteCalculationSettings, error) {
	var out QuoteCalculationSettings
	err := s.pg.QueryRow(ctx, `
		SELECT id, target_margin_percent,
		       default_material_zuschlag_percent, default_lohn_zuschlag_percent,
		       default_fremdleistung_zuschlag_percent, default_lohn_stundensatz,
		       updated_at
		FROM quote_calculation_settings
		WHERE id = 'default'
	`).Scan(&out.ID, &out.TargetMarginPercent,
		&out.DefaultMaterialZuschlagPercent, &out.DefaultLohnZuschlagPercent,
		&out.DefaultFremdleistungZuschlagPercent, &out.DefaultLohnStundensatz,
		&out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// validateQuoteCalculationDefaults validiert die Kalkulationseinstellungen
// und liefert die normalisierte Zielmarge zurueck. Anders als
// target_margin_percent (0 gilt historisch als "nicht gesetzt" -> Fallback
// 20) gelten die vier neuen Default-Zuschlag-/Stundensatz-Felder aus
// Backlog B.3 bewusst NICHT mit derselben "0 = ungesetzt"-Heuristik: laut
// ADR 0011 IST 0 ihr korrekter Ausgangswert ("noch kein Satz hinterlegt"),
// es gibt keinen sinnvollen Fallback-Wert wie bei der Zielmarge. Sie werden
// daher nur auf Bereichsgueltigkeit geprueft und unveraendert uebernommen.
// Eigene Funktion (statt inline in Upsert), damit die Validierung ohne
// Postgres unit-testbar ist.
func validateQuoteCalculationDefaults(in QuoteCalculationSettings) (float64, error) {
	targetMarginPercent := in.TargetMarginPercent
	if targetMarginPercent == 0 {
		targetMarginPercent = 20
	}
	if targetMarginPercent < 0 || targetMarginPercent > 1000 {
		return 0, errors.New("Zielmarge ist ungueltig")
	}
	if in.DefaultMaterialZuschlagPercent < 0 || in.DefaultMaterialZuschlagPercent > 1000 {
		return 0, errors.New("Material-Zuschlag ist ungueltig")
	}
	if in.DefaultLohnZuschlagPercent < 0 || in.DefaultLohnZuschlagPercent > 1000 {
		return 0, errors.New("Lohn-Zuschlag ist ungueltig")
	}
	if in.DefaultFremdleistungZuschlagPercent < 0 || in.DefaultFremdleistungZuschlagPercent > 1000 {
		return 0, errors.New("Fremdleistungs-Zuschlag ist ungueltig")
	}
	if in.DefaultLohnStundensatz < 0 {
		return 0, errors.New("Lohn-Stundensatz ist ungueltig")
	}
	return targetMarginPercent, nil
}

func (s *QuoteCalculationSettingsService) Upsert(ctx context.Context, in QuoteCalculationSettings) error {
	targetMarginPercent, err := validateQuoteCalculationDefaults(in)
	if err != nil {
		return err
	}

	_, err = s.pg.Exec(ctx, `
		INSERT INTO quote_calculation_settings (
			id,
			target_margin_percent,
			default_material_zuschlag_percent,
			default_lohn_zuschlag_percent,
			default_fremdleistung_zuschlag_percent,
			default_lohn_stundensatz,
			updated_at
		) VALUES (
			'default',
			$1, $2, $3, $4, $5,
			now()
		)
		ON CONFLICT (id) DO UPDATE SET
			target_margin_percent = EXCLUDED.target_margin_percent,
			default_material_zuschlag_percent = EXCLUDED.default_material_zuschlag_percent,
			default_lohn_zuschlag_percent = EXCLUDED.default_lohn_zuschlag_percent,
			default_fremdleistung_zuschlag_percent = EXCLUDED.default_fremdleistung_zuschlag_percent,
			default_lohn_stundensatz = EXCLUDED.default_lohn_stundensatz,
			updated_at = now()
	`, targetMarginPercent, in.DefaultMaterialZuschlagPercent, in.DefaultLohnZuschlagPercent,
		in.DefaultFremdleistungZuschlagPercent, in.DefaultLohnStundensatz)
	return err
}
