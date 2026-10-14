package hr

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Anforderungen je Mitarbeiter und der Stand eines Mitarbeiters (ADR 0027,
// Backlog F.2.4.3).
//
// Eine Anforderung sagt, welche Art ein Mitarbeiter fuer seine Stelle
// braucht (Vorgabe des Nutzers vom 2026-10-13). Sie ist PLANUNG, kein
// Beleg: die Menge ist aenderbar, Eintraege werden eingefuegt und wieder
// entfernt. Das unterscheidet sie von den Nachweisen in qualifications.go,
// die unveraenderlich sind - deshalb steht dieser Code in einer eigenen
// Datei, und der Waechtertest fuer die Nachweise betrifft ihn nicht.

// QualificationRequirement ist eine Anforderung eines Mitarbeiters samt
// der Angaben zur Art.
type QualificationRequirement struct {
	TypeID    uuid.UUID `json:"type_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Kategorie string    `json:"kategorie"`
	// TypeAktiv ist false, wenn die Art inzwischen stillgelegt wurde. Die
	// Anforderung bleibt dann bestehen, zaehlt aber im Stand nicht mehr.
	TypeAktiv bool      `json:"type_aktiv"`
	CreatedAt time.Time `json:"created_at"`
}

func uniqueTypeIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// SetRequirements setzt die Anforderungen eines Mitarbeiters auf genau die
// genannten Arten: was fehlt, kommt hinzu, was nicht mehr genannt ist,
// wird entfernt, was bleibt, bleibt unberuehrt. Eine leere Liste entfernt
// alle. Alles in einer Transaktion.
//
// Eine NEU hinzukommende Art muss zum Mandanten gehoeren und aktiv sein.
// Eine bereits bestehende Anforderung auf eine inzwischen stillgelegte Art
// darf in der Liste stehen bleiben - sonst liesse sich die Liste eines
// Mitarbeiters nicht mehr speichern, sobald irgendeine seiner Arten
// stillgelegt wurde.
func (s *QualificationService) SetRequirements(ctx context.Context, employeeID uuid.UUID, typeIDs []uuid.UUID, actorUserID, companyID string) ([]QualificationRequirement, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if employeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	for _, id := range typeIDs {
		if id == uuid.Nil {
			return nil, errors.New("type_id erforderlich")
		}
	}
	wanted := uniqueTypeIDs(typeIDs)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Sperrt den Mitarbeiter: prueft den Mandanten und verhindert, dass
	// zwei gleichzeitige Aenderungen seiner Anforderungen einander
	// ueberschreiben.
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM hr_employees WHERE id=$1 AND company_id=$2 FOR UPDATE`, employeeID, companyID).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
		return nil, err
	}

	existing := make(map[uuid.UUID]bool)
	rows, err := tx.Query(ctx, `SELECT type_id FROM hr_employee_qualification_requirements WHERE employee_id=$1`, employeeID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		existing[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	added := make([]uuid.UUID, 0, len(wanted))
	for _, id := range wanted {
		if !existing[id] {
			added = append(added, id)
		}
	}
	if len(added) > 0 {
		var found, active int
		if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE aktiv) FROM hr_qualification_types WHERE id = ANY($1) AND company_id=$2`, added, companyID).Scan(&found, &active); err != nil {
			return nil, err
		}
		if found != len(added) {
			return nil, errors.New("Qualifikationsart nicht gefunden")
		}
		if active != len(added) {
			return nil, errors.New("Qualifikationsart ist stillgelegt")
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM hr_employee_qualification_requirements WHERE employee_id=$1 AND NOT (type_id = ANY($2))`, employeeID, wanted); err != nil {
		return nil, err
	}
	for _, id := range added {
		if _, err := tx.Exec(ctx, `INSERT INTO hr_employee_qualification_requirements (employee_id, type_id, created_by_user_id) VALUES ($1,$2,$3)`, employeeID, id, nullableUserID(actorUserID)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListRequirements(ctx, employeeID, companyID)
}

// ListRequirements liefert die Anforderungen eines Mitarbeiters, nach Name
// der Art sortiert.
func (s *QualificationService) ListRequirements(ctx context.Context, employeeID uuid.UUID, companyID string) ([]QualificationRequirement, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if employeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	var employeeOwned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, employeeID, companyID).Scan(&employeeOwned); err != nil {
		return nil, err
	}
	if !employeeOwned {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `
		SELECT r.type_id, t.code, t.name, t.kategorie, t.aktiv, r.created_at
		  FROM hr_employee_qualification_requirements r
		  JOIN hr_qualification_types t ON t.id = r.type_id
		 WHERE r.employee_id=$1
		 ORDER BY t.name, t.code
	`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QualificationRequirement, 0)
	for rows.Next() {
		var r QualificationRequirement
		if err := rows.Scan(&r.TypeID, &r.Code, &r.Name, &r.Kategorie, &r.TypeAktiv, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EmployeeQualificationStanding ist der Stand einer Art bei einem
// Mitarbeiter, ergaenzt um die Angaben zur Art.
type EmployeeQualificationStanding struct {
	QualificationStanding
	Code      string `json:"code"`
	Name      string `json:"name"`
	Kategorie string `json:"kategorie"`
}

// qualificationStandRank ordnet nach Dringlichkeit: was fehlt oder
// abgelaufen ist, steht oben.
func qualificationStandRank(status string) int {
	switch status {
	case QualificationStandFehlt:
		return 0
	case QualificationStandAbgelaufen:
		return 1
	case QualificationStandLaeuftAb:
		return 2
	default:
		return 3
	}
}

// Standing liefert den Stand eines Mitarbeiters am Tag today: je Art, fuer
// die er einen aktiven Nachweis hat oder die fuer ihn erforderlich ist,
// ein Eintrag - das Dringendste zuerst (fehlt, abgelaufen, laeuft ab,
// gueltig), darin nach Name.
//
// Die Regeln (was erforderlich ist, was nicht zaehlt) stehen in
// evaluateEmployeeStanding und gelten gleich fuer DueList.
func (s *QualificationService) Standing(ctx context.Context, employeeID uuid.UUID, today time.Time, warnDays int, companyID string) ([]EmployeeQualificationStanding, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if employeeID == uuid.Nil {
		return nil, errors.New("employee_id erforderlich")
	}
	var employeeActive bool
	if err := s.pg.QueryRow(ctx, `SELECT active FROM hr_employees WHERE id=$1 AND company_id=$2`, employeeID, companyID).Scan(&employeeActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
		return nil, err
	}

	catalog, err := s.loadActiveCatalog(ctx, companyID)
	if err != nil {
		return nil, err
	}

	ownRequirements := make([]uuid.UUID, 0)
	reqRows, err := s.pg.Query(ctx, `SELECT type_id FROM hr_employee_qualification_requirements WHERE employee_id=$1`, employeeID)
	if err != nil {
		return nil, err
	}
	for reqRows.Next() {
		var id uuid.UUID
		if err := reqRows.Scan(&id); err != nil {
			reqRows.Close()
			return nil, err
		}
		ownRequirements = append(ownRequirements, id)
	}
	reqRows.Close()
	if err := reqRows.Err(); err != nil {
		return nil, err
	}

	records, err := s.ListForEmployee(ctx, employeeID, false, companyID)
	if err != nil {
		return nil, err
	}
	return catalog.evaluateEmployeeStanding(employeeActive, ownRequirements, records, today, warnDays), nil
}
