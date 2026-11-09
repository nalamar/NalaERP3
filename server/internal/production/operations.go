package production

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/auditlog"
)

// Arbeitsgaenge am Fertigungsauftrag (ADR 0037, Backlog H.2.3a): Planung,
// nicht Teil der eingefrorenen Kopie - aenderbar bis "fertig" oder
// "storniert", jede Aenderung im entity_change_log.

const HintNachLiefertermin = "nach_liefertermin"

var (
	ErrOperationNotFound = errors.New("Arbeitsgang nicht gefunden")
	errOperationsLocked  = errors.New("Auftrag ist abgeschlossen oder storniert, Arbeitsgänge nicht änderbar")
	errOperationsExist   = errors.New("Auftrag hat bereits Arbeitsgänge")
	errNoTemplates       = errors.New("Keine aktiven Arbeitsgangvorlagen vorhanden")
)

const auditEntityOrder = "production_order"

type Operation struct {
	ID              uuid.UUID  `json:"id"`
	Pos             int        `json:"pos"`
	Name            string     `json:"name"`
	WorkCenterID    uuid.UUID  `json:"work_center_id"`
	WorkCenterName  string     `json:"work_center_name"`
	TemplateID      *uuid.UUID `json:"template_id"`
	RuestzeitMin    float64    `json:"ruestzeit_min"`
	MinutenJeStueck float64    `json:"minuten_je_stueck"`
	GeplanteWoche   *time.Time `json:"geplante_woche"`
	// Planzeit fuer den ganzen Auftrag (PlannedMinutes).
	PlanMinuten float64  `json:"plan_minuten"`
	Hinweise    []string `json:"hinweise"`
}

const operationColumns = `op.id, op.pos, op.name, op.work_center_id, w.name, op.template_id,
	op.ruestzeit_min::float8, op.minuten_je_stueck::float8, op.geplante_woche`

const operationFrom = `
	  FROM production_order_operations op
	  JOIN production_work_centers w ON w.id = op.work_center_id`

// finish rechnet Planzeit und Hinweise aus den Angaben des Auftrags.
func (x *Operation) finish(stueckzahl int, liefertermin *time.Time) {
	x.PlanMinuten = PlannedMinutes(x.RuestzeitMin, x.MinutenJeStueck, stueckzahl)
	x.Hinweise = []string{}
	if AfterDelivery(x.GeplanteWoche, liefertermin) {
		x.Hinweise = append(x.Hinweise, HintNachLiefertermin)
	}
}

func scanOperation(row pgx.Row) (*Operation, error) {
	var x Operation
	if err := row.Scan(&x.ID, &x.Pos, &x.Name, &x.WorkCenterID, &x.WorkCenterName, &x.TemplateID,
		&x.RuestzeitMin, &x.MinutenJeStueck, &x.GeplanteWoche); err != nil {
		return nil, err
	}
	return &x, nil
}

func listOperations(ctx context.Context, q querier, orderID uuid.UUID, stueckzahl int, liefertermin *time.Time) ([]Operation, error) {
	rows, err := q.Query(ctx, `SELECT `+operationColumns+operationFrom+` WHERE op.order_id=$1 ORDER BY op.pos`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Operation{}
	for rows.Next() {
		x, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		x.finish(stueckzahl, liefertermin)
		out = append(out, *x)
	}
	return out, rows.Err()
}

// ListOperations: Arbeitsgaenge eines Auftrags des Mandanten nach Pos.
func (s *Service) ListOperations(ctx context.Context, orderID uuid.UUID, companyID string) ([]Operation, error) {
	var stueckzahl int
	var liefertermin *time.Time
	err := s.pg.QueryRow(ctx, `SELECT stueckzahl, liefertermin FROM production_orders WHERE id=$1 AND company_id=$2`,
		orderID, companyID).Scan(&stueckzahl, &liefertermin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return listOperations(ctx, s.pg, orderID, stueckzahl, liefertermin)
}

type planningOrder struct {
	stueckzahl   int
	liefertermin *time.Time
}

// lockOrderForOperations sperrt den Auftrag (Reihenfolge der Pos, Status)
// und prueft, dass er noch Arbeitsgaenge aendern darf.
func lockOrderForOperations(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, companyID string) (*planningOrder, error) {
	var o planningOrder
	var status string
	err := tx.QueryRow(ctx, `SELECT status, stueckzahl, liefertermin FROM production_orders
		WHERE id=$1 AND company_id=$2 FOR UPDATE`, orderID, companyID).Scan(&status, &o.stueckzahl, &o.liefertermin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == StatusFertig || status == StatusStorniert {
		return nil, errOperationsLocked
	}
	return &o, nil
}

func getOperation(ctx context.Context, q querier, orderID, opID uuid.UUID, o *planningOrder) (*Operation, error) {
	x, err := scanOperation(q.QueryRow(ctx, `SELECT `+operationColumns+operationFrom+` WHERE op.id=$1 AND op.order_id=$2`, opID, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOperationNotFound
	}
	if err != nil {
		return nil, err
	}
	x.finish(o.stueckzahl, o.liefertermin)
	return x, nil
}

func (s *Service) record(ctx context.Context, tx pgx.Tx, companyID string, orderID uuid.UUID, action, actor string, before, after any) error {
	return s.audit.Record(ctx, tx, companyID, auditlog.RecordInput{
		EntityType: auditEntityOrder, EntityID: orderID.String(), Action: action, ActorUserID: actor, Before: before, After: after,
	})
}

// SuggestOperations legt aus den aktiven Vorlagen (mit aktivem
// Arbeitsplatz, in ihrer Reihenfolge) die Arbeitsgaenge an, mit Wochen
// rueckwaerts vom Liefertermin (DefaultWeeks). Nur fuer einen Auftrag ohne
// Arbeitsgaenge - danach von Hand aendern.
func (s *Service) SuggestOperations(ctx context.Context, orderID uuid.UUID, actorUserID, companyID string) ([]Operation, error) {
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	o, err := lockOrderForOperations(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM production_order_operations WHERE order_id=$1)`, orderID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, errOperationsExist
	}
	rows, err := tx.Query(ctx, `SELECT t.id, t.name, t.work_center_id, t.ruestzeit_min::float8, t.minuten_je_stueck::float8`+templateFrom+`
		WHERE t.company_id=$1 AND t.aktiv AND w.aktiv ORDER BY t.reihenfolge, lower(t.name), t.id`, companyID)
	if err != nil {
		return nil, err
	}
	type tpl struct {
		id, wc        uuid.UUID
		name          string
		ruest, stueck float64
	}
	var tpls []tpl
	for rows.Next() {
		var t tpl
		if err := rows.Scan(&t.id, &t.name, &t.wc, &t.ruest, &t.stueck); err != nil {
			rows.Close()
			return nil, err
		}
		tpls = append(tpls, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tpls) == 0 {
		return nil, errNoTemplates
	}
	weeks := DefaultWeeks(o.liefertermin, len(tpls))
	for i, t := range tpls {
		if _, err := tx.Exec(ctx, `INSERT INTO production_order_operations
			(id, order_id, pos, name, work_center_id, template_id, ruestzeit_min, minuten_je_stueck, geplante_woche, created_by_user_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			uuid.New(), orderID, i+1, t.name, t.wc, t.id, t.ruest, t.stueck, weeks[i], nullableUserID(actorUserID)); err != nil {
			return nil, err
		}
	}
	out, err := listOperations(ctx, tx, orderID, o.stueckzahl, o.liefertermin)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "operations_suggested", actorUserID, nil, out); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// AddOperation haengt einen Arbeitsgang hinten an.
func (s *Service) AddOperation(ctx context.Context, orderID uuid.UUID, in OperationInput, actorUserID, companyID string) (*Operation, error) {
	in = NormalizeOperationInput(in)
	if err := ValidateOperationInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	o, err := lockOrderForOperations(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	if err := checkWorkCenter(ctx, tx, in.WorkCenterID, nil, companyID); err != nil {
		return nil, err
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO production_order_operations
		(id, order_id, pos, name, work_center_id, ruestzeit_min, minuten_je_stueck, geplante_woche, created_by_user_id)
		VALUES ($1,$2,(SELECT COALESCE(max(pos),0)+1 FROM production_order_operations WHERE order_id=$2),$3,$4,$5,$6,$7,$8)`,
		id, orderID, in.Name, in.WorkCenterID, in.RuestzeitMin, in.MinutenJeStueck, in.GeplanteWoche, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	x, err := getOperation(ctx, tx, orderID, id, o)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "operation_added", actorUserID, nil, x); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return x, nil
}

// UpdateOperation setzt alle Angaben. Ein deaktivierter Arbeitsplatz ist
// nur erlaubt, wenn er schon zugeordnet ist. Die Herkunft (Vorlage) bleibt.
func (s *Service) UpdateOperation(ctx context.Context, orderID, opID uuid.UUID, in OperationInput, actorUserID, companyID string) (*Operation, error) {
	in = NormalizeOperationInput(in)
	if err := ValidateOperationInput(in); err != nil {
		return nil, err
	}
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	o, err := lockOrderForOperations(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	before, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return nil, err
	}
	if err := checkWorkCenter(ctx, tx, in.WorkCenterID, &before.WorkCenterID, companyID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE production_order_operations
		SET name=$3, work_center_id=$4, ruestzeit_min=$5, minuten_je_stueck=$6, geplante_woche=$7, updated_at=now()
		WHERE id=$1 AND order_id=$2`, opID, orderID, in.Name, in.WorkCenterID, in.RuestzeitMin, in.MinutenJeStueck, in.GeplanteWoche); err != nil {
		return nil, err
	}
	after, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "operation_changed", actorUserID, before, after); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return after, nil
}

// RemoveOperation entfernt einen Arbeitsgang; die Pos der uebrigen
// bleiben (Luecken sind erlaubt).
func (s *Service) RemoveOperation(ctx context.Context, orderID, opID uuid.UUID, actorUserID, companyID string) error {
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	o, err := lockOrderForOperations(ctx, tx, orderID, companyID)
	if err != nil {
		return err
	}
	before, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM production_order_operations WHERE id=$1 AND order_id=$2`, opID, orderID); err != nil {
		return err
	}
	if err := s.record(ctx, tx, companyID, orderID, "operation_removed", actorUserID, before, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
