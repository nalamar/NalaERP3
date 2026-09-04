package purchasing

import (
	"context"
	"errors"
	"strings"
)

// Bedarfsermittlung aus Angebot/Mindestbestand (ADR 0016). Zwei getrennte,
// unabhängig lesbare Sichten statt einer erfundenen Verrechnungsformel
// zwischen den beiden Quellen (siehe ADR 0016, Optionen).

type MinStockShortfall struct {
	MaterialID     string  `json:"material_id"`
	MaterialNummer string  `json:"material_nummer"`
	MindestBestand float64 `json:"mindestbestand"`
	Verfuegbar     float64 `json:"verfuegbar"`
	FehlMenge      float64 `json:"fehlmenge"`
}

// MinStockShortfalls liefert je Material mit konfiguriertem
// mindestbestand, dessen verfügbarer Bestand (physischer Bestand minus
// aktive Reservierungen, materialweit über alle Lager des Mandanten
// aggregiert) das Soll unterschreitet, die Unterdeckung. Materialien ohne
// Unterdeckung oder ohne konfigurierten mindestbestand erscheinen nicht.
func (s *Service) MinStockShortfalls(ctx context.Context, companyID string) ([]MinStockShortfall, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT m.id, m.nummer, m.mindestbestand,
               COALESCE((SELECT SUM(sm.quantity) FROM stock_movements sm WHERE sm.material_id = m.id), 0)
               - COALESCE((SELECT SUM(sr.qty) FROM stock_reservations sr WHERE sr.material_id = m.id AND sr.status = 'aktiv'), 0) AS verfuegbar
        FROM materials m
        WHERE m.company_id = $1 AND m.mindestbestand IS NOT NULL
    `, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MinStockShortfall, 0)
	for rows.Next() {
		var row MinStockShortfall
		if err := rows.Scan(&row.MaterialID, &row.MaterialNummer, &row.MindestBestand, &row.Verfuegbar); err != nil {
			return nil, err
		}
		if row.Verfuegbar < row.MindestBestand {
			row.FehlMenge = row.MindestBestand - row.Verfuegbar
			out = append(out, row)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type QuoteDemandItem struct {
	MaterialID     string  `json:"material_id"`
	MaterialNummer string  `json:"material_nummer"`
	DemandQty      float64 `json:"demand_qty"`
}

// QuoteDemand liefert je Material die Summe der Mengen aus quote_items
// aller Angebote mit Status 'sent' oder 'accepted' (siehe ADR 0016 für
// die Begründung, warum akzeptierte Angebote mitzählen). Keine
// Verrechnung gegen vorhandenen Bestand.
func (s *Service) QuoteDemand(ctx context.Context, companyID string) ([]QuoteDemandItem, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT qi.material_id, m.nummer, SUM(qi.qty) AS demand_qty
        FROM quote_items qi
        JOIN quotes q ON q.id = qi.quote_id
        JOIN materials m ON m.id = qi.material_id
        WHERE q.company_id = $1 AND q.status IN ('sent', 'accepted') AND qi.material_id IS NOT NULL
        GROUP BY qi.material_id, m.nummer
        ORDER BY m.nummer ASC
    `, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QuoteDemandItem, 0)
	for rows.Next() {
		var row QuoteDemandItem
		if err := rows.Scan(&row.MaterialID, &row.MaterialNummer, &row.DemandQty); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
