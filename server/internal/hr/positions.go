package hr

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Stellen als Vorlagen fuer erforderliche Qualifikationen (ADR 0031,
// Backlog F.11.2). Eine Stelle ("Monteur", "Schweisser") traegt die Arten,
// die ihre Inhaber vorweisen muessen. Ab F.11.3 gilt das dauerhaft: eine
// Aenderung an der Stelle wirkt sofort auf alle Inhaber.
//
// Wie der Qualifikationskatalog gehoert die Liste dem Mandanten; geloescht
// wird nichts, stillgelegt ueber aktiv=false.

// Namen aus Migration 097 (siehe isUniqueViolation).
const (
	positionCodeConstraint = "hr_positions_company_code_key"
	positionNameIndex      = "idx_hr_positions_company_name"
)

type Position struct {
	ID      uuid.UUID `json:"id"`
	Code    string    `json:"code"`
	Name    string    `json:"name"`
	Aktiv   bool      `json:"aktiv"`
	Hinweis string    `json:"hinweis"`
	// TypeIDs: die Arten, die die Stelle verlangt, sortiert. Auch
	// inzwischen stillgelegte Arten bleiben hier stehen.
	TypeIDs   []uuid.UUID `json:"type_ids"`
	CreatedAt time.Time   `json:"created_at"`
}

type PositionInput struct {
	Code    string      `json:"code"`
	Name    string      `json:"name"`
	Hinweis string      `json:"hinweis"`
	TypeIDs []uuid.UUID `json:"type_ids"`
}

// PositionUpdate ersetzt ALLE aenderbaren Angaben (wie
// QualificationTypeUpdate): TypeIDs ist die vollstaendige neue Menge. Der
// Code fehlt mit Absicht.
type PositionUpdate struct {
	Name    string      `json:"name"`
	Aktiv   bool        `json:"aktiv"`
	Hinweis string      `json:"hinweis"`
	TypeIDs []uuid.UUID `json:"type_ids"`
}

type PositionService struct{ pg *pgxpool.Pool }

func NewPositionService(pg *pgxpool.Pool) *PositionService { return &PositionService{pg: pg} }

const positionColumns = `p.id, p.code, p.name, p.aktiv, p.hinweis, p.created_at,
	COALESCE((SELECT array_agg(t.type_id ORDER BY t.type_id) FROM hr_position_qualification_types t WHERE t.position_id=p.id), '{}')`

func scanPosition(row pgx.Row) (*Position, error) {
	var p Position
	// COALESCE(..., '{}') in positionColumns: ohne Arten eine leere Liste,
	// nicht null.
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Aktiv, &p.Hinweis, &p.CreatedAt, &p.TypeIDs); err != nil {
		return nil, err
	}
	return &p, nil
}

// normalizePositionTypeIDs: ohne Doppelte, sortiert; uuid.Nil ist ein
// Fehler.
func normalizePositionTypeIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, errors.New("type_id erforderlich")
		}
	}
	out := uniqueTypeIDs(ids)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out, nil
}

func positionUniqueError(err error) error {
	switch {
	case isUniqueViolation(err, positionCodeConstraint):
		return errors.New("Stelle mit diesem Code existiert bereits")
	case isUniqueViolation(err, positionNameIndex):
		return errors.New("Stelle mit diesem Namen existiert bereits")
	}
	return err
}

// replacePositionTypesTx setzt die Arten einer Stelle auf genau wanted.
// Eine NEU hinzukommende Art muss zum Mandanten gehoeren und aktiv sein;
// eine bestehende Zuordnung zu einer inzwischen stillgelegten Art darf
// bleiben - sonst liesse sich die Stelle nicht mehr speichern (wie bei
// SetRequirements).
func replacePositionTypesTx(ctx context.Context, tx pgx.Tx, positionID uuid.UUID, wanted []uuid.UUID, companyID string) error {
	existing := make(map[uuid.UUID]bool)
	rows, err := tx.Query(ctx, `SELECT type_id FROM hr_position_qualification_types WHERE position_id=$1`, positionID)
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
		if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE aktiv) FROM hr_qualification_types WHERE id = ANY($1) AND company_id=$2`, added, companyID).Scan(&found, &active); err != nil {
			return err
		}
		if found != len(added) {
			return errors.New("Qualifikationsart nicht gefunden")
		}
		if active != len(added) {
			return errors.New("Qualifikationsart ist stillgelegt")
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM hr_position_qualification_types WHERE position_id=$1 AND NOT (type_id = ANY($2))`, positionID, wanted); err != nil {
		return err
	}
	for _, id := range added {
		if _, err := tx.Exec(ctx, `INSERT INTO hr_position_qualification_types (position_id, type_id) VALUES ($1,$2)`, positionID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *PositionService) Create(ctx context.Context, in PositionInput, companyID string) (*Position, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	code := normalizeQualificationCode(in.Code)
	name := strings.TrimSpace(in.Name)
	if code == "" {
		return nil, errors.New("Code erforderlich")
	}
	if !qualificationCodePattern.MatchString(code) {
		return nil, errors.New("Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)")
	}
	if name == "" {
		return nil, errors.New("Name erforderlich")
	}
	typeIDs, err := normalizePositionTypeIDs(in.TypeIDs)
	if err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO hr_positions (id, company_id, code, name, hinweis) VALUES ($1,$2,$3,$4,$5)`,
		id, companyID, code, name, strings.TrimSpace(in.Hinweis)); err != nil {
		return nil, positionUniqueError(err)
	}
	if err := replacePositionTypesTx(ctx, tx, id, typeIDs, companyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

func (s *PositionService) Get(ctx context.Context, id uuid.UUID, companyID string) (*Position, error) {
	p, err := scanPosition(s.pg.QueryRow(ctx, `SELECT `+positionColumns+` FROM hr_positions p WHERE p.id=$1 AND p.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Stelle nicht gefunden")
		}
		return nil, err
	}
	return p, nil
}

// List liefert die Stellen des Mandanten nach Name; ohne includeInactive
// nur die aktiven.
func (s *PositionService) List(ctx context.Context, companyID string, includeInactive bool) ([]Position, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+positionColumns+` FROM hr_positions p
		WHERE p.company_id=$1 AND (p.aktiv OR $2) ORDER BY lower(p.name), p.code`, companyID, includeInactive)
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

// Update ersetzt Name, Aktiv, Hinweis und die Arten einer Stelle in einer
// Transaktion.
func (s *PositionService) Update(ctx context.Context, id uuid.UUID, in PositionUpdate, companyID string) (*Position, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("Name erforderlich")
	}
	typeIDs, err := normalizePositionTypeIDs(in.TypeIDs)
	if err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Die Zeile wird gesperrt: zwei gleichzeitige Aenderungen der Arten
	// ueberschreiben einander nicht halb.
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `UPDATE hr_positions SET name=$3, aktiv=$4, hinweis=$5 WHERE id=$1 AND company_id=$2 RETURNING id`,
		id, companyID, name, in.Aktiv, strings.TrimSpace(in.Hinweis)).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Stelle nicht gefunden")
		}
		return nil, positionUniqueError(err)
	}
	if err := replacePositionTypesTx(ctx, tx, id, typeIDs, companyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}
