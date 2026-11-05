package purchasing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"nalaerp3/internal/auditlog"
	"nalaerp3/internal/materials"
)

// ReceiveInput: Wareneingang einer Bestellung am Stueck (Backlog H.4.z1).
type ReceiveInput struct {
	WarehouseID string  `json:"warehouse_id"`
	LocationID  *string `json:"location_id"`
	SetReceived bool    `json:"status_received"`
}

// Receive bucht alle Positionen der Bestellung mit voller Menge und Bezug
// zur Position als Lagerbewegung und setzt auf Wunsch den Status
// `received` - alles in einer Transaktion. Scheitert eine Position, ist
// nichts gebucht. Die Sperre auf die Bestellung und die Pruefung auf
// vorhandene Bewegungen verhindern doppeltes Buchen (ANNAHME, ADR 0035
// Praezisierung H.4.z1).
func (s *Service) Receive(ctx context.Context, id string, in ReceiveInput, companyID string, actorUserID string) (*PurchaseOrder, []PurchaseOrderItem, int, error) {
	warehouseID := strings.TrimSpace(in.WarehouseID)
	if warehouseID == "" {
		return nil, nil, 0, errors.New("Lager erforderlich")
	}
	var locationID *string
	if in.LocationID != nil && strings.TrimSpace(*in.LocationID) != "" {
		loc := strings.TrimSpace(*in.LocationID)
		locationID = &loc
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	defer tx.Rollback(ctx)

	var number, status, currency string
	err = tx.QueryRow(ctx, `SELECT number, status, currency FROM purchase_orders WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&number, &status, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, 0, errors.New("Bestellung nicht gefunden")
	}
	if err != nil {
		return nil, nil, 0, err
	}
	if status != "ordered" {
		return nil, nil, 0, fmt.Errorf("Wareneingang ist im aktuellen Status nicht erlaubt (aktuell: %s)", status)
	}
	var booked bool
	if err := tx.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM stock_movements sm
            JOIN purchase_order_items poi ON poi.id = sm.purchase_order_item_id
            WHERE poi.order_id=$1
        )`, id).Scan(&booked); err != nil {
		return nil, nil, 0, err
	}
	if booked {
		return nil, nil, 0, errors.New("Wareneingang zu dieser Bestellung wurde bereits erfasst")
	}

	rows, err := tx.Query(ctx, `SELECT id, material_id, qty, uom, unit_price, currency FROM purchase_order_items WHERE order_id=$1 ORDER BY position ASC`, id)
	if err != nil {
		return nil, nil, 0, err
	}
	var moves []materials.StockMovementCreate
	for rows.Next() {
		var itemID string
		var m materials.StockMovementCreate
		var price float64
		var cur string
		if err := rows.Scan(&itemID, &m.MaterialID, &m.Menge, &m.Einheit, &price, &cur); err != nil {
			rows.Close()
			return nil, nil, 0, err
		}
		m.WarehouseID = warehouseID
		m.LocationID = locationID
		m.Typ = "purchase"
		m.Grund = "Wareneingang"
		m.Referenz = number
		m.EKPreis = &price
		cur = strings.ToUpper(strings.TrimSpace(cur))
		if cur == "" {
			cur = strings.ToUpper(strings.TrimSpace(currency))
		}
		m.Waehrung = &cur
		m.PurchaseOrderItemID = &itemID
		moves = append(moves, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, 0, err
	}
	if len(moves) == 0 {
		return nil, nil, 0, errors.New("Bestellung hat keine Positionen")
	}
	for _, m := range moves {
		if _, err := materials.CreateMovementTx(ctx, tx, m, companyID); err != nil {
			return nil, nil, 0, err
		}
	}

	if in.SetReceived {
		if _, err := tx.Exec(ctx, `UPDATE purchase_orders SET status='received' WHERE id=$1 AND company_id=$2`, id, companyID); err != nil {
			return nil, nil, 0, err
		}
		if s.audit != nil {
			if err := s.audit.Record(ctx, tx, companyID, auditlog.RecordInput{
				EntityType:  "purchase_order",
				EntityID:    id,
				Action:      "status_geaendert",
				ActorUserID: actorUserID,
				Before:      map[string]any{"status": status},
				After:       map[string]any{"status": "received"},
			}); err != nil {
				return nil, nil, 0, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, 0, err
	}
	po, items, err := s.Get(ctx, id, companyID)
	return po, items, len(moves), err
}
