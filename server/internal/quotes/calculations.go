package quotes

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// QuoteItemCalculation (Kalkulationsschema Material/Lohn/Fremdleistung,
// je mit eigenem Zuschlagssatz) — Backlog B.3, siehe
// docs/adr/0011-kalkulationsschema.md. Die *Total-/CalculatedUnitPrice-
// Felder sind reine Ableitungen (siehe computeCalculationTotals), NICHT
// in der DB gespeichert.
type QuoteItemCalculation struct {
	ID                           uuid.UUID `json:"id"`
	QuoteItemID                  uuid.UUID `json:"quote_item_id"`
	MaterialCost                 float64   `json:"material_cost"`
	MaterialZuschlagPercent      float64   `json:"material_zuschlag_percent"`
	LohnStunden                  float64   `json:"lohn_stunden"`
	LohnStundensatz              float64   `json:"lohn_stundensatz"`
	LohnZuschlagPercent          float64   `json:"lohn_zuschlag_percent"`
	FremdleistungCost            float64   `json:"fremdleistung_cost"`
	FremdleistungZuschlagPercent float64   `json:"fremdleistung_zuschlag_percent"`
	CreatedAt                    time.Time `json:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at"`
	MaterialTotal                float64   `json:"material_total"`
	LohnCost                     float64   `json:"lohn_cost"`
	LohnTotal                    float64   `json:"lohn_total"`
	FremdleistungTotal           float64   `json:"fremdleistung_total"`
	CalculatedUnitPrice          float64   `json:"calculated_unit_price"`
}

type QuoteItemCalculationInput struct {
	MaterialCost                 float64 `json:"material_cost"`
	MaterialZuschlagPercent      float64 `json:"material_zuschlag_percent"`
	LohnStunden                  float64 `json:"lohn_stunden"`
	LohnStundensatz              float64 `json:"lohn_stundensatz"`
	LohnZuschlagPercent          float64 `json:"lohn_zuschlag_percent"`
	FremdleistungCost            float64 `json:"fremdleistung_cost"`
	FremdleistungZuschlagPercent float64 `json:"fremdleistung_zuschlag_percent"`
}

func validateCalculationInput(in QuoteItemCalculationInput) error {
	if in.MaterialCost < 0 {
		return errors.New("Materialkosten dürfen nicht negativ sein")
	}
	if in.MaterialZuschlagPercent < 0 {
		return errors.New("Material-Zuschlag darf nicht negativ sein")
	}
	if in.LohnStunden < 0 {
		return errors.New("Arbeitsstunden dürfen nicht negativ sein")
	}
	if in.LohnStundensatz < 0 {
		return errors.New("Stundensatz darf nicht negativ sein")
	}
	if in.LohnZuschlagPercent < 0 {
		return errors.New("Lohn-Zuschlag darf nicht negativ sein")
	}
	if in.FremdleistungCost < 0 {
		return errors.New("Fremdleistungskosten dürfen nicht negativ sein")
	}
	if in.FremdleistungZuschlagPercent < 0 {
		return errors.New("Fremdleistungs-Zuschlag darf nicht negativ sein")
	}
	return nil
}

// computeCalculationTotals fuellt die abgeleiteten Felder (siehe ADR 0011:
// bewusst nicht in der DB gespeichert, reine Ableitung aus den Rohwerten,
// um Inkonsistenzen auszuschliessen). Klassische Zuschlagskalkulation mit
// Kostenartentrennung: jede Kostenart bekommt ihren EIGENEN Zuschlag statt
// einer gemeinsamen Marge auf die Gesamtsumme.
func computeCalculationTotals(c *QuoteItemCalculation) {
	c.MaterialTotal = roundCurrency(c.MaterialCost * (1 + c.MaterialZuschlagPercent/100))
	c.LohnCost = roundCurrency(c.LohnStunden * c.LohnStundensatz)
	c.LohnTotal = roundCurrency(c.LohnCost * (1 + c.LohnZuschlagPercent/100))
	c.FremdleistungTotal = roundCurrency(c.FremdleistungCost * (1 + c.FremdleistungZuschlagPercent/100))
	c.CalculatedUnitPrice = roundCurrency(c.MaterialTotal + c.LohnTotal + c.FremdleistungTotal)
}

// ensureQuoteItemEditableTx prueft Ownership (ueber quote_id UND
// company_id), Schreibschutz (nur draft, keine historischen Revisionen —
// identisches Muster wie ApplyPrimaryPriceSourceForQuoteItem u. a.) UND
// dass die Position zum Angebot gehoert.
func ensureQuoteItemEditableTx(ctx context.Context, tx pgx.Tx, quoteID, itemID uuid.UUID, companyID string) error {
	var status string
	var supersededByQuoteID uuid.NullUUID
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id
		FROM quotes
		WHERE id=$1 AND company_id=$2
		FOR UPDATE
	`, quoteID, companyID).Scan(&status, &supersededByQuoteID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("Angebot nicht gefunden")
		}
		return err
	}
	if supersededByQuoteID.Valid {
		return errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if status != "draft" {
		return errors.New("nur Entwürfe sind bearbeitbar")
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM quote_items WHERE id=$1 AND quote_id=$2)`, itemID, quoteID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("Angebotsposition nicht gefunden")
	}
	return nil
}

// UpsertQuoteItemCalculation legt die Kalkulationsdetails einer Position
// an oder aktualisiert sie (1:1-Beziehung, PUT-Semantik).
func (s *Service) UpsertQuoteItemCalculation(ctx context.Context, quoteID, itemID uuid.UUID, in QuoteItemCalculationInput, companyID string) (*QuoteItemCalculation, error) {
	if err := validateCalculationInput(in); err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureQuoteItemEditableTx(ctx, tx, quoteID, itemID, companyID); err != nil {
		return nil, err
	}

	var c QuoteItemCalculation
	err = tx.QueryRow(ctx, `
        INSERT INTO quote_item_calculations (
            id, quote_item_id, material_cost, material_zuschlag_percent,
            lohn_stunden, lohn_stundensatz, lohn_zuschlag_percent,
            fremdleistung_cost, fremdleistung_zuschlag_percent
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        ON CONFLICT (quote_item_id) DO UPDATE SET
            material_cost = EXCLUDED.material_cost,
            material_zuschlag_percent = EXCLUDED.material_zuschlag_percent,
            lohn_stunden = EXCLUDED.lohn_stunden,
            lohn_stundensatz = EXCLUDED.lohn_stundensatz,
            lohn_zuschlag_percent = EXCLUDED.lohn_zuschlag_percent,
            fremdleistung_cost = EXCLUDED.fremdleistung_cost,
            fremdleistung_zuschlag_percent = EXCLUDED.fremdleistung_zuschlag_percent,
            updated_at = now()
        RETURNING id, quote_item_id, material_cost, material_zuschlag_percent,
                  lohn_stunden, lohn_stundensatz, lohn_zuschlag_percent,
                  fremdleistung_cost, fremdleistung_zuschlag_percent, created_at, updated_at
    `, uuid.New(), itemID, in.MaterialCost, in.MaterialZuschlagPercent,
		in.LohnStunden, in.LohnStundensatz, in.LohnZuschlagPercent,
		in.FremdleistungCost, in.FremdleistungZuschlagPercent).Scan(
		&c.ID, &c.QuoteItemID, &c.MaterialCost, &c.MaterialZuschlagPercent,
		&c.LohnStunden, &c.LohnStundensatz, &c.LohnZuschlagPercent,
		&c.FremdleistungCost, &c.FremdleistungZuschlagPercent, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	computeCalculationTotals(&c)
	return &c, nil
}

func (s *Service) GetQuoteItemCalculation(ctx context.Context, quoteID, itemID uuid.UUID, companyID string) (*QuoteItemCalculation, error) {
	var c QuoteItemCalculation
	err := s.pg.QueryRow(ctx, `
        SELECT qic.id, qic.quote_item_id, qic.material_cost, qic.material_zuschlag_percent,
               qic.lohn_stunden, qic.lohn_stundensatz, qic.lohn_zuschlag_percent,
               qic.fremdleistung_cost, qic.fremdleistung_zuschlag_percent, qic.created_at, qic.updated_at
        FROM quote_item_calculations qic
        JOIN quote_items qi ON qi.id = qic.quote_item_id
        JOIN quotes q ON q.id = qi.quote_id
        WHERE qic.quote_item_id=$1 AND qi.quote_id=$2 AND q.company_id=$3
    `, itemID, quoteID, companyID).Scan(
		&c.ID, &c.QuoteItemID, &c.MaterialCost, &c.MaterialZuschlagPercent,
		&c.LohnStunden, &c.LohnStundensatz, &c.LohnZuschlagPercent,
		&c.FremdleistungCost, &c.FremdleistungZuschlagPercent, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("keine Kalkulation für diese Position vorhanden")
		}
		return nil, err
	}
	computeCalculationTotals(&c)
	return &c, nil
}

func (s *Service) DeleteQuoteItemCalculation(ctx context.Context, quoteID, itemID uuid.UUID, companyID string) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureQuoteItemEditableTx(ctx, tx, quoteID, itemID, companyID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM quote_item_calculations WHERE quote_item_id=$1`, itemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("keine Kalkulation für diese Position vorhanden")
	}
	return tx.Commit(ctx)
}

// ApplyCalculationForQuoteItem schreibt calculated_unit_price in
// quote_items.unit_price (inkl. Neuberechnung von net_amount/tax_amount
// und der Angebotssumme) und protokolliert die Entscheidung in der
// bestehenden quote_item_price_decisions (decision_type
// 'calculation_scheme_applied') — identisches Muster wie
// ApplyPrimaryPriceSourceForQuoteItem/ApplyTargetUnitPriceForQuoteItem,
// bewusst OHNE material_id-Pflicht (anders als ApplyPrimaryPriceSource...:
// eine Kalkulation kann rein lohn-/fremdleistungsbasiert sein).
func (s *Service) ApplyCalculationForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, companyID string) (*Quote, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var quoteCurrency string
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id, COALESCE(NULLIF(BTRIM(currency), ''), 'EUR')
		FROM quotes
		WHERE id=$1 AND company_id=$2
		FOR UPDATE
	`, quoteID, companyID).Scan(&currentStatus, &supersededByQuoteID, &quoteCurrency); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebot nicht gefunden")
		}
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var currentMaterialID string
	var qty float64
	var taxCode string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(material_id, ''), qty, COALESCE(tax_code, '')
		FROM quote_items
		WHERE id=$1 AND quote_id=$2
		FOR UPDATE
	`, itemID, quoteID).Scan(&currentMaterialID, &qty, &taxCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}

	var c QuoteItemCalculation
	err = tx.QueryRow(ctx, `
        SELECT material_cost, material_zuschlag_percent, lohn_stunden, lohn_stundensatz,
               lohn_zuschlag_percent, fremdleistung_cost, fremdleistung_zuschlag_percent
        FROM quote_item_calculations WHERE quote_item_id=$1
    `, itemID).Scan(&c.MaterialCost, &c.MaterialZuschlagPercent, &c.LohnStunden, &c.LohnStundensatz,
		&c.LohnZuschlagPercent, &c.FremdleistungCost, &c.FremdleistungZuschlagPercent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("keine Kalkulation für diese Position vorhanden")
		}
		return nil, err
	}
	computeCalculationTotals(&c)

	codes, err := loadTaxCodesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	rate, err := taxRate(codes, taxCode)
	if err != nil {
		return nil, err
	}
	netAmount := qty * c.CalculatedUnitPrice
	taxAmount := netAmount * rate
	if _, err := tx.Exec(ctx, `
		UPDATE quote_items
		SET unit_price = $2,
			net_amount = $3,
			tax_amount = $4,
			price_mapping_status = 'manual'
		WHERE id = $1
	`, itemID, c.CalculatedUnitPrice, netAmount, taxAmount); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE quotes
		SET net_amount = totals.net_amount,
			tax_amount = totals.tax_amount,
			gross_amount = totals.net_amount + totals.tax_amount
		FROM (
			SELECT COALESCE(SUM(net_amount), 0) AS net_amount, COALESCE(SUM(tax_amount), 0) AS tax_amount
			FROM quote_items
			WHERE quote_id = $1
		) totals
		WHERE quotes.id = $1
	`, quoteID); err != nil {
		return nil, err
	}

	if err := insertCalculationSchemeAppliedDecisionTx(ctx, tx, quoteID, itemID, currentMaterialID, c.CalculatedUnitPrice, quoteCurrency); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, quoteID, companyID)
}

func insertCalculationSchemeAppliedDecisionTx(ctx context.Context, tx pgx.Tx, quoteID, itemID uuid.UUID, materialID string, appliedUnitPrice float64, currency string) error {
	if currency == "" {
		currency = "EUR"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO quote_item_price_decisions (
			id, quote_id, quote_item_id, material_id, decision_type,
			source_label, source_unit_price, applied_unit_price, currency
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), 'calculation_scheme_applied', 'Kalkulationsschema (Material/Lohn/Fremdleistung)', $5, $5, $6)
	`, uuid.New(), quoteID, itemID, materialID, appliedUnitPrice, currency)
	return err
}
