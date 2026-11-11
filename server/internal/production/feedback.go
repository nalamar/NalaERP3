package production

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Rueckmeldung je Arbeitsgang (ADR 0038 Frage 5 (a), Backlog H.3.5b):
// "erledigt" mit optionaler Ist-Zeit ohne Personenbezug, zuruecknehmen
// mit Begruendung. Beides nur, solange der Auftrag in Arbeit ist; jede
// Aenderung im entity_change_log des Auftrags.

var (
	errOperationAlreadyDone = errors.New("Arbeitsgang ist bereits erledigt gemeldet")
	errOperationNotDone     = errors.New("Arbeitsgang ist nicht erledigt gemeldet")
)

// lockOrderForFeedback sperrt den Auftrag des Mandanten und prueft, dass
// gemeldet werden darf.
func lockOrderForFeedback(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, companyID string) (*planningOrder, error) {
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
	if err := FeedbackAllowed(status); err != nil {
		return nil, err
	}
	return &o, nil
}

// ReportOperationDone meldet einen Arbeitsgang als erledigt.
func (s *Service) ReportOperationDone(ctx context.Context, orderID, opID uuid.UUID, in FeedbackInput, actorUserID, companyID string) (*Operation, error) {
	in = in.Normalize()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	o, err := lockOrderForFeedback(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	before, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return nil, err
	}
	if before.Erledigt {
		return nil, errOperationAlreadyDone
	}
	if _, err := tx.Exec(ctx, `INSERT INTO production_operation_feedback (id, company_id, operation_id, ist_minuten, notiz, gemeldet_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6)`, uuid.New(), companyID, opID, in.IstMinuten, in.Notiz, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	after, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "operation_done", actorUserID, before, after); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return after, nil
}

// OrderSollIst: Soll/Ist-Uebersicht der Arbeitsgaenge des Auftrags
// (H.3.5c), in jedem Status.
func (s *Service) OrderSollIst(ctx context.Context, orderID uuid.UUID, companyID string) (*SollIst, error) {
	ops, err := s.ListOperations(ctx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	x := SummarizeSollIst(ops)
	return &x, nil
}

// RevertOperationDone nimmt die gueltige Rueckmeldung mit Begruendung
// zurueck; danach kann neu gemeldet werden.
func (s *Service) RevertOperationDone(ctx context.Context, orderID, opID uuid.UUID, reason, actorUserID, companyID string) (*Operation, error) {
	reason, err := ValidateFeedbackRevertReason(reason)
	if err != nil {
		return nil, err
	}
	tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	o, err := lockOrderForFeedback(ctx, tx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	before, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `UPDATE production_operation_feedback
		SET zurueckgenommen_at=now(), ruecknahme_reason=$2, zurueckgenommen_by_user_id=$3
		WHERE operation_id=$1 AND zurueckgenommen_at IS NULL`, opID, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, errOperationNotDone
	}
	after, err := getOperation(ctx, tx, orderID, opID, o)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, tx, companyID, orderID, "operation_done_reverted", actorUserID, before,
		map[string]any{"operation": after, "reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return after, nil
}
