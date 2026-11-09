package production

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Stammdaten der Grobplanung: Arbeitsplaetze und Arbeitsgangvorlagen je
// Mandant (ADR 0037, Backlog H.2.2). Deaktivieren statt loeschen.

var (
	ErrWorkCenterNotFound = errors.New("Arbeitsplatz nicht gefunden")
	ErrTemplateNotFound   = errors.New("Arbeitsgangvorlage nicht gefunden")
	errWorkCenterInactive = errors.New("Arbeitsplatz ist deaktiviert")
	errWorkCenterName     = errors.New("Arbeitsplatz mit diesem Namen bereits vorhanden")
	errTemplateName       = errors.New("Arbeitsgangvorlage mit diesem Namen bereits vorhanden")
)

type WorkCenter struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	StundenJeTag float64   `json:"stunden_je_tag"`
	Aktiv        bool      `json:"aktiv"`
	CreatedAt    time.Time `json:"created_at"`
}

type OperationTemplate struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	WorkCenterID    uuid.UUID `json:"work_center_id"`
	WorkCenterName  string    `json:"work_center_name"`
	Reihenfolge     int       `json:"reihenfolge"`
	RuestzeitMin    float64   `json:"ruestzeit_min"`
	MinutenJeStueck float64   `json:"minuten_je_stueck"`
	Aktiv           bool      `json:"aktiv"`
	CreatedAt       time.Time `json:"created_at"`
}

func isUniqueViolation(err error, index string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == index
}

const workCenterColumns = `id, name, stunden_je_tag::float8, aktiv, created_at`

func scanWorkCenter(row pgx.Row) (*WorkCenter, error) {
	var x WorkCenter
	if err := row.Scan(&x.ID, &x.Name, &x.StundenJeTag, &x.Aktiv, &x.CreatedAt); err != nil {
		return nil, err
	}
	return &x, nil
}

func (s *Service) CreateWorkCenter(ctx context.Context, in WorkCenterInput, companyID string) (*WorkCenter, error) {
	in = NormalizeWorkCenterInput(in)
	if err := ValidateWorkCenterInput(in); err != nil {
		return nil, err
	}
	x, err := scanWorkCenter(s.pg.QueryRow(ctx, `INSERT INTO production_work_centers (id, company_id, name, stunden_je_tag)
		VALUES ($1,$2,$3,$4) RETURNING `+workCenterColumns, uuid.New(), companyID, in.Name, in.StundenJeTag))
	if isUniqueViolation(err, "production_work_centers_company_name_key") {
		return nil, errWorkCenterName
	}
	return x, err
}

// UpdateWorkCenter setzt Name und Stunden; aktiv nil laesst den Zustand.
// Ein deaktivierter Arbeitsplatz behaelt seine Arbeitsgaenge.
func (s *Service) UpdateWorkCenter(ctx context.Context, id uuid.UUID, in WorkCenterInput, aktiv *bool, companyID string) (*WorkCenter, error) {
	in = NormalizeWorkCenterInput(in)
	if err := ValidateWorkCenterInput(in); err != nil {
		return nil, err
	}
	x, err := scanWorkCenter(s.pg.QueryRow(ctx, `UPDATE production_work_centers
		SET name=$3, stunden_je_tag=$4, aktiv=COALESCE($5, aktiv)
		WHERE id=$1 AND company_id=$2 RETURNING `+workCenterColumns, id, companyID, in.Name, in.StundenJeTag, aktiv))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrWorkCenterNotFound
	case isUniqueViolation(err, "production_work_centers_company_name_key"):
		return nil, errWorkCenterName
	}
	return x, err
}

// ListWorkCenters nach Name; deaktivierte nur auf Wunsch.
func (s *Service) ListWorkCenters(ctx context.Context, companyID string, includeInactive bool) ([]WorkCenter, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+workCenterColumns+` FROM production_work_centers
		WHERE company_id=$1 AND (aktiv OR $2) ORDER BY lower(name), id`, companyID, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkCenter{}
	for rows.Next() {
		x, err := scanWorkCenter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

// checkWorkCenter: Arbeitsplatz des Mandanten; ein deaktivierter nur,
// wenn er schon zugeordnet ist (keep).
func checkWorkCenter(ctx context.Context, q querier, id uuid.UUID, keep *uuid.UUID, companyID string) error {
	var aktiv bool
	err := q.QueryRow(ctx, `SELECT aktiv FROM production_work_centers WHERE id=$1 AND company_id=$2`, id, companyID).Scan(&aktiv)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWorkCenterNotFound
	}
	if err != nil {
		return err
	}
	if !aktiv && (keep == nil || *keep != id) {
		return errWorkCenterInactive
	}
	return nil
}

const templateColumns = `t.id, t.name, t.work_center_id, w.name, t.reihenfolge, t.ruestzeit_min::float8,
	t.minuten_je_stueck::float8, t.aktiv, t.created_at`

const templateFrom = `
	  FROM production_operation_templates t
	  JOIN production_work_centers w ON w.id = t.work_center_id`

func scanTemplate(row pgx.Row) (*OperationTemplate, error) {
	var x OperationTemplate
	if err := row.Scan(&x.ID, &x.Name, &x.WorkCenterID, &x.WorkCenterName, &x.Reihenfolge, &x.RuestzeitMin,
		&x.MinutenJeStueck, &x.Aktiv, &x.CreatedAt); err != nil {
		return nil, err
	}
	return &x, nil
}

func (s *Service) getTemplate(ctx context.Context, id uuid.UUID, companyID string) (*OperationTemplate, error) {
	x, err := scanTemplate(s.pg.QueryRow(ctx, `SELECT `+templateColumns+templateFrom+` WHERE t.id=$1 AND t.company_id=$2`, id, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	return x, err
}

func (s *Service) CreateTemplate(ctx context.Context, in TemplateInput, companyID string) (*OperationTemplate, error) {
	in = NormalizeTemplateInput(in)
	if err := ValidateTemplateInput(in); err != nil {
		return nil, err
	}
	if err := checkWorkCenter(ctx, s.pg, in.WorkCenterID, nil, companyID); err != nil {
		return nil, err
	}
	id := uuid.New()
	_, err := s.pg.Exec(ctx, `INSERT INTO production_operation_templates
		(id, company_id, name, work_center_id, reihenfolge, ruestzeit_min, minuten_je_stueck) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, companyID, in.Name, in.WorkCenterID, in.Reihenfolge, in.RuestzeitMin, in.MinutenJeStueck)
	if isUniqueViolation(err, "production_operation_templates_company_name_key") {
		return nil, errTemplateName
	}
	if err != nil {
		return nil, err
	}
	return s.getTemplate(ctx, id, companyID)
}

// UpdateTemplate setzt alle Angaben; aktiv nil laesst den Zustand. Ein
// deaktivierter Arbeitsplatz ist nur erlaubt, wenn er schon zugeordnet ist.
func (s *Service) UpdateTemplate(ctx context.Context, id uuid.UUID, in TemplateInput, aktiv *bool, companyID string) (*OperationTemplate, error) {
	in = NormalizeTemplateInput(in)
	if err := ValidateTemplateInput(in); err != nil {
		return nil, err
	}
	cur, err := s.getTemplate(ctx, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := checkWorkCenter(ctx, s.pg, in.WorkCenterID, &cur.WorkCenterID, companyID); err != nil {
		return nil, err
	}
	_, err = s.pg.Exec(ctx, `UPDATE production_operation_templates
		SET name=$3, work_center_id=$4, reihenfolge=$5, ruestzeit_min=$6, minuten_je_stueck=$7, aktiv=COALESCE($8, aktiv)
		WHERE id=$1 AND company_id=$2`, id, companyID, in.Name, in.WorkCenterID, in.Reihenfolge, in.RuestzeitMin, in.MinutenJeStueck, aktiv)
	if isUniqueViolation(err, "production_operation_templates_company_name_key") {
		return nil, errTemplateName
	}
	if err != nil {
		return nil, err
	}
	return s.getTemplate(ctx, id, companyID)
}

// ListTemplates in der Reihenfolge des Vorschlags; deaktivierte nur auf
// Wunsch.
func (s *Service) ListTemplates(ctx context.Context, companyID string, includeInactive bool) ([]OperationTemplate, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+templateColumns+templateFrom+`
		WHERE t.company_id=$1 AND (t.aktiv OR $2) ORDER BY t.reihenfolge, lower(t.name), t.id`, companyID, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OperationTemplate{}
	for rows.Next() {
		x, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}
