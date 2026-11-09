package production

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/materials"
)

// Entnahme fuer Fertigungsauftraege (ADR 0038 Frage 2, Backlog H.3.2a):
// entnehmen gegen eine Zeile der Stueckliste oder Material ohne Zeile,
// stornieren mit Gegenbuchung, lesen. Jede Entnahme bucht einen
// Lagerabgang und verbraucht - mit Zeile - deren Reservierung; das Storno
// stellt sie wieder her, solange der Auftrag offen ist. Regeln in
// withdrawal_rules.go, Tabelle in Migration 113.

type Withdrawal struct {
	ID                  uuid.UUID  `json:"id"`
	ItemID              *uuid.UUID `json:"item_id"`
	ItemPos             *int       `json:"item_pos"`
	MaterialID          string     `json:"material_id"`
	MaterialNummer      string     `json:"material_nummer"`
	MaterialBezeichnung string     `json:"material_bezeichnung"`
	WarehouseID         string     `json:"warehouse_id"`
	LocationID          *string    `json:"location_id"`
	Menge               float64    `json:"menge"`
	Einheit             string     `json:"einheit"`
	ReservierungMenge   float64    `json:"reservierung_menge"`
	Notiz               string     `json:"notiz"`
	CreatedByUserID     *string    `json:"created_by_user_id"`
	CreatedAt           time.Time  `json:"created_at"`
	StornoReason        string     `json:"storno_reason,omitempty"`
	StorniertAt         *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID   *string    `json:"storniert_by_user_id,omitempty"`
	// Hinweise der Buchung (HintMehrentnahme), nur in der Antwort.
	Hinweise []string `json:"hinweise"`

	stockMovementID    string
	stockReservationID *string
}

var (
	ErrWithdrawalNotFound = errors.New("Entnahme nicht gefunden")
	errItemNotInOrder     = errors.New("Zeile nicht im Fertigungsauftrag")
	errItemNoMaterial     = errors.New("Zeile hat kein Lagermaterial")
	errItemOtherMaterial  = errors.New("Material passt nicht zur Zeile")
)

const withdrawalColumns = `w.id, w.item_id, i.pos, w.material_id, m.nummer, m.bezeichnung, w.warehouse_id, w.location_id,
	w.menge::float8, w.einheit, w.reservierung_menge::float8, w.notiz, w.created_by_user_id, w.created_at,
	w.storno_reason, w.storniert_at, w.storniert_by_user_id, w.stock_movement_id, w.stock_reservation_id`

const withdrawalFrom = `
	  FROM production_order_withdrawals w
	  JOIN materials m ON m.id = w.material_id
	  LEFT JOIN production_order_items i ON i.id = w.item_id`

func scanWithdrawal(row pgx.Row) (*Withdrawal, error) {
	w := Withdrawal{Hinweise: []string{}}
	if err := row.Scan(&w.ID, &w.ItemID, &w.ItemPos, &w.MaterialID, &w.MaterialNummer, &w.MaterialBezeichnung, &w.WarehouseID,
		&w.LocationID, &w.Menge, &w.Einheit, &w.ReservierungMenge, &w.Notiz, &w.CreatedByUserID, &w.CreatedAt,
		&w.StornoReason, &w.StorniertAt, &w.StorniertByUserID, &w.stockMovementID, &w.stockReservationID); err != nil {
		return nil, err
	}
	return &w, nil
}

// ListWithdrawals: Entnahmen eines Auftrags, neueste zuerst, stornierte
// eingeschlossen.
func (s *Service) ListWithdrawals(ctx context.Context, orderID uuid.UUID, companyID string) ([]Withdrawal, error) {
	var ok bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM production_orders WHERE id=$1 AND company_id=$2)`, orderID, companyID).Scan(&ok); err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	rows, err := s.pg.Query(ctx, `SELECT `+withdrawalColumns+withdrawalFrom+`
		 WHERE w.order_id=$1 AND w.company_id=$2 ORDER BY w.created_at DESC, w.id`, orderID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Withdrawal, 0)
	for rows.Next() {
		w, err := scanWithdrawal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

// withdrawalItem: die Zeile mit Material, Bedarf in der Einheit des
// Materials und bisher Entnommenem (ohne Stornos).
type withdrawalItem struct {
	materialID    *string
	reservationID *string
	bedarf        float64
	bedarfKnown   bool
	entnommen     float64
}

func loadWithdrawalItem(ctx context.Context, tx pgx.Tx, orderID, itemID uuid.UUID, stueckzahl int) (*withdrawalItem, error) {
	var it Item
	var x withdrawalItem
	err := tx.QueryRow(ctx, `SELECT i.art, i.unit, i.length_mm::float8, i.menge_je_stueck::float8, i.material_id,
		COALESCE(m.einheit,''), i.stock_reservation_id,
		COALESCE((SELECT SUM(w.menge) FROM production_order_withdrawals w WHERE w.item_id=i.id AND w.storniert_at IS NULL), 0)::float8
		  FROM production_order_items i
		  LEFT JOIN materials m ON m.id = i.material_id
		 WHERE i.id=$1 AND i.order_id=$2`, itemID, orderID).Scan(&it.Art, &it.Unit, &it.LengthMM, &it.MengeJeStueck,
		&x.materialID, &it.MaterialEinheit, &x.reservationID, &x.entnommen)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errItemNotInOrder
	}
	if err != nil {
		return nil, err
	}
	x.bedarf, x.bedarfKnown = ReservationQty(it.Art, it.Unit, it.MaterialEinheit, it.LengthMM, it.MengeJeStueck*float64(stueckzahl))
	return &x, nil
}

// Withdraw bucht eine Entnahme. Kein Bestand im System verhindert sie
// nicht: das Material hat das Lager verlassen (Korrektur ueber Inventur).
func (s *Service) Withdraw(ctx context.Context, orderID uuid.UUID, in WithdrawalInput, actorUserID, companyID string) (*Withdrawal, error) {
	in = in.Normalize()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := lockOrder(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	if err := WithdrawalAllowed(cur.status); err != nil {
		return nil, err
	}

	hinweise := []string{}
	var itemID *uuid.UUID
	var reservationID *string
	materialID := in.MaterialID
	if in.ItemID != "" {
		id, err := uuid.Parse(in.ItemID)
		if err != nil {
			return nil, errItemNotInOrder
		}
		it, err := loadWithdrawalItem(ctx, tx, orderID, id, cur.stueckzahl)
		if err != nil {
			return nil, err
		}
		if it.materialID == nil {
			return nil, errItemNoMaterial
		}
		if materialID != "" && materialID != *it.materialID {
			return nil, errItemOtherMaterial
		}
		materialID, itemID, reservationID = *it.materialID, &id, it.reservationID
		if Overdrawn(it.bedarf, it.bedarfKnown, it.entnommen, in.Menge) {
			hinweise = append(hinweise, HintMehrentnahme)
		}
	}

	var einheit string
	if err := tx.QueryRow(ctx, `SELECT einheit FROM materials WHERE id=$1 AND company_id=$2`, materialID, companyID).Scan(&einheit); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Material nicht gefunden")
		}
		return nil, err
	}
	var location *string
	if in.LocationID != "" {
		location = &in.LocationID
	}
	mv, err := materials.CreateMovementTx(ctx, tx, materials.StockMovementCreate{
		MaterialID: materialID, WarehouseID: in.WarehouseID, LocationID: location, Menge: -in.Menge, Einheit: einheit,
		Typ: EntnahmeBewegung, Grund: EntnahmeGrund, Referenz: cur.nummer,
	}, companyID)
	if err != nil {
		return nil, err
	}

	verbraucht := 0.0
	if reservationID != nil {
		var status string
		var qty float64
		err := tx.QueryRow(ctx, `SELECT status, qty::float8 FROM stock_reservations WHERE id=$1 FOR UPDATE`, *reservationID).Scan(&status, &qty)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			var rest float64
			var next string
			verbraucht, rest, next = ReservationConsumption(status, qty, in.Menge)
			if verbraucht > 0 {
				if _, err := tx.Exec(ctx, `UPDATE stock_reservations SET qty=$2, status=$3,
					released_at = CASE WHEN $3 = 'verbraucht' THEN now() ELSE released_at END WHERE id=$1`,
					*reservationID, rest, next); err != nil {
					return nil, err
				}
			}
		}
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO production_order_withdrawals (id, company_id, order_id, item_id, material_id, warehouse_id,
		location_id, menge, einheit, stock_movement_id, stock_reservation_id, reservierung_menge, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		id, companyID, orderID, itemID, materialID, in.WarehouseID, location, in.Menge, einheit, mv.ID,
		reservationID, verbraucht, in.Notiz, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	out, err := scanWithdrawal(tx.QueryRow(ctx, `SELECT `+withdrawalColumns+withdrawalFrom+` WHERE w.id=$1`, id))
	if err != nil {
		return nil, err
	}
	out.Hinweise = hinweise
	if err := s.record(ctx, tx, companyID, orderID, "withdrawal_booked", actorUserID, nil, out); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// CancelWithdrawal storniert eine Entnahme mit Begruendung: Gegenbuchung
// ins selbe Lager und auf denselben Platz; die verbrauchte Reservierung
// kommt zurueck, solange der Auftrag offen ist (ReservationAfterStorno).
// In jedem Status des Auftrags moeglich.
func (s *Service) CancelWithdrawal(ctx context.Context, orderID, withdrawalID uuid.UUID, reason, actorUserID, companyID string) (*Withdrawal, error) {
	r, err := ValidateWithdrawalStornoReason(reason)
	if err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := lockOrder(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	before, err := scanWithdrawal(tx.QueryRow(ctx, `SELECT `+withdrawalColumns+withdrawalFrom+`
		 WHERE w.id=$1 AND w.order_id=$2 AND w.company_id=$3 FOR UPDATE OF w`, withdrawalID, orderID, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWithdrawalNotFound
	}
	if err != nil {
		return nil, err
	}
	if before.StorniertAt != nil {
		return nil, errors.New("Entnahme ist bereits storniert")
	}
	mv, err := materials.CreateMovementTx(ctx, tx, materials.StockMovementCreate{
		MaterialID: before.MaterialID, WarehouseID: before.WarehouseID, LocationID: before.LocationID, Menge: before.Menge,
		Einheit: before.Einheit, Typ: EntnahmeStornoBewegung, Grund: EntnahmeStornoGrund, Referenz: cur.nummer,
	}, companyID)
	if err != nil {
		return nil, err
	}
	if before.stockReservationID != nil && before.ReservierungMenge > 0 {
		var status string
		var qty float64
		err := tx.QueryRow(ctx, `SELECT status, qty::float8 FROM stock_reservations WHERE id=$1 FOR UPDATE`, *before.stockReservationID).Scan(&status, &qty)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			if q, st, restore := ReservationAfterStorno(cur.status, status, qty, before.ReservierungMenge); restore {
				if _, err := tx.Exec(ctx, `UPDATE stock_reservations SET qty=$2, status=$3, released_at=NULL WHERE id=$1`,
					*before.stockReservationID, q, st); err != nil {
					return nil, err
				}
			}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE production_order_withdrawals SET storniert_at=now(), storno_reason=$2,
		storniert_by_user_id=$3, storno_movement_id=$4 WHERE id=$1`, withdrawalID, r, nullableUserID(actorUserID), mv.ID); err != nil {
		return nil, err
	}
	out, err := scanWithdrawal(tx.QueryRow(ctx, `SELECT `+withdrawalColumns+withdrawalFrom+` WHERE w.id=$1`, withdrawalID))
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "withdrawal_cancelled", actorUserID, before, out); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
