package hr

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Stellen am Mitarbeiter (ADR 0031, Backlog F.11.3.1). Was eine aktive
// Stelle verlangt, ist fuer ihre Inhaber erforderlich - dauerhaft, nicht
// als Kopie: Standing und DueList lesen die Anforderungen ueber
// effectiveRequirementsSQL, das eigene Anforderungen und Stellen vereint.

// effectiveRequirementsSQL liefert je Mitarbeiter die Arten, die fuer ihn
// erforderlich sind, ausser pflicht_fuer_alle (das ergaenzt
// evaluateEmployeeStanding): eigene Anforderungen und die Arten seiner
// AKTIVEN Stellen. Eine stillgelegte Stelle zaehlt nicht mehr - wie eine
// stillgelegte Art. Spalten: employee_id, type_id; ohne Doppelte.
const effectiveRequirementsSQL = `
	SELECT r.employee_id, r.type_id FROM hr_employee_qualification_requirements r
	UNION
	SELECT ep.employee_id, pt.type_id
	  FROM hr_employee_positions ep
	  JOIN hr_positions p ON p.id = ep.position_id AND p.aktiv
	  JOIN hr_position_qualification_types pt ON pt.position_id = p.id`

// normalizePositionIDs: ohne Doppelte; uuid.Nil ist ein Fehler.
func normalizePositionIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, errors.New("position_id erforderlich")
		}
	}
	return uniqueTypeIDs(ids), nil
}

// setEmployeePositionsTx setzt die Stellen eines Mitarbeiters auf genau
// wanted (ohne Doppelte, ohne uuid.Nil). Eine NEU hinzukommende Stelle
// muss zum Mandanten gehoeren und aktiv sein; eine bestehende Zuordnung zu
// einer inzwischen stillgelegten Stelle darf bleiben (wie bei
// SetRequirements). Commit und Rollback liegen beim Aufrufer.
func setEmployeePositionsTx(ctx context.Context, tx pgx.Tx, employeeID uuid.UUID, wanted []uuid.UUID, actorUserID, companyID string) error {
	// Sperrt den Mitarbeiter: prueft den Mandanten und serialisiert
	// gleichzeitige Aenderungen seiner Stellen.
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM hr_employees WHERE id=$1 AND company_id=$2 FOR UPDATE`, employeeID, companyID).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("Mitarbeiter nicht gefunden")
		}
		return err
	}
	existing := make(map[uuid.UUID]bool)
	rows, err := tx.Query(ctx, `SELECT position_id FROM hr_employee_positions WHERE employee_id=$1`, employeeID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		existing[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	added := make([]uuid.UUID, 0, len(wanted))
	for _, id := range wanted {
		if !existing[id] {
			added = append(added, id)
		}
	}
	if len(added) > 0 {
		var found, active int
		if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE aktiv) FROM hr_positions WHERE id = ANY($1) AND company_id=$2`, added, companyID).Scan(&found, &active); err != nil {
			return err
		}
		if found != len(added) {
			return errors.New("Stelle nicht gefunden")
		}
		if active != len(added) {
			return errors.New("Stelle ist stillgelegt")
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM hr_employee_positions WHERE employee_id=$1 AND NOT (position_id = ANY($2))`, employeeID, wanted); err != nil {
		return err
	}
	for _, id := range added {
		if _, err := tx.Exec(ctx, `INSERT INTO hr_employee_positions (employee_id, position_id, created_by_user_id) VALUES ($1,$2,$3)`, employeeID, id, nullableUserID(actorUserID)); err != nil {
			return err
		}
	}
	return nil
}

// SetEmployeePositions setzt die Stellen eines Mitarbeiters auf genau die
// genannten (leere Liste = keine) und liefert sie danach.
func (s *QualificationService) SetEmployeePositions(ctx context.Context, employeeID uuid.UUID, positionIDs []uuid.UUID, actorUserID, companyID string) ([]Position, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if employeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	wanted, err := normalizePositionIDs(positionIDs)
	if err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := setEmployeePositionsTx(ctx, tx, employeeID, wanted, actorUserID, companyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListEmployeePositions(ctx, employeeID, companyID)
}

// ListEmployeePositions liefert die Stellen eines Mitarbeiters nach Name,
// auch stillgelegte (sie zaehlen im Stand nicht mehr, Aktiv=false).
func (s *QualificationService) ListEmployeePositions(ctx context.Context, employeeID uuid.UUID, companyID string) ([]Position, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if employeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	var owned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, employeeID, companyID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `SELECT `+positionColumns+`
		  FROM hr_employee_positions ep
		  JOIN hr_positions p ON p.id = ep.position_id
		 WHERE ep.employee_id=$1
		 ORDER BY lower(p.name), p.code`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Position, 0)
	for rows.Next() {
		p, err := scanPosition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}
