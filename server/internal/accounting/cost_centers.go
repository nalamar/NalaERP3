package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Kostenstellenmodell (ADR 0019). Mandantenweite Stammdaten, analog zu
// materials/warehouses. Kein echtes Löschen - Kostenstellen dürfen nach
// Verwendung in Buchungen nicht spurlos verschwinden (Soft-Delete über
// aktiv=false, analog zu materials.DeleteSoft).

type CostCenter struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Aktiv     bool      `json:"aktiv"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type CostCenterCreate struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Note string `json:"note"`
}

type CostCenterUpdate struct {
	Code  *string `json:"code"`
	Name  *string `json:"name"`
	Aktiv *bool   `json:"aktiv"`
	Note  *string `json:"note"`
}

type CostCenterService struct {
	pg *pgxpool.Pool
}

func NewCostCenterService(pg *pgxpool.Pool) *CostCenterService {
	return &CostCenterService{pg: pg}
}

func (s *CostCenterService) Create(ctx context.Context, in CostCenterCreate, companyID string) (*CostCenter, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Code und Name sind erforderlich")
	}
	var codeExists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cost_centers WHERE company_id=$1 AND code=$2)`, companyID, in.Code).Scan(&codeExists); err != nil {
		return nil, err
	}
	if codeExists {
		return nil, errors.New("Kostenstelle mit diesem Code existiert bereits")
	}
	id := uuid.NewString()
	var cc CostCenter
	err := s.pg.QueryRow(ctx, `
        INSERT INTO cost_centers (id, company_id, code, name, note)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING id, code, name, aktiv, COALESCE(note,''), created_at
    `, id, companyID, in.Code, in.Name, in.Note).Scan(&cc.ID, &cc.Code, &cc.Name, &cc.Aktiv, &cc.Note, &cc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &cc, nil
}

func (s *CostCenterService) Get(ctx context.Context, id string, companyID string) (*CostCenter, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	var cc CostCenter
	err := s.pg.QueryRow(ctx, `
        SELECT id, code, name, aktiv, COALESCE(note,''), created_at
        FROM cost_centers WHERE id=$1 AND company_id=$2
    `, id, companyID).Scan(&cc.ID, &cc.Code, &cc.Name, &cc.Aktiv, &cc.Note, &cc.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Kostenstelle nicht gefunden")
		}
		return nil, err
	}
	return &cc, nil
}

func (s *CostCenterService) List(ctx context.Context, companyID string, includeInactive bool) ([]CostCenter, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	query := `SELECT id, code, name, aktiv, COALESCE(note,''), created_at FROM cost_centers WHERE company_id=$1`
	if !includeInactive {
		query += ` AND aktiv = true`
	}
	query += ` ORDER BY code ASC`

	rows, err := s.pg.Query(ctx, query, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CostCenter, 0)
	for rows.Next() {
		var cc CostCenter
		if err := rows.Scan(&cc.ID, &cc.Code, &cc.Name, &cc.Aktiv, &cc.Note, &cc.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, cc)
	}
	return out, nil
}

func (s *CostCenterService) Update(ctx context.Context, id string, u CostCenterUpdate, companyID string) (*CostCenter, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if u.Code != nil {
		if strings.TrimSpace(*u.Code) == "" {
			return nil, errors.New("Code erforderlich")
		}
		var codeExists bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cost_centers WHERE company_id=$1 AND code=$2 AND id<>$3)`, companyID, *u.Code, id).Scan(&codeExists); err != nil {
			return nil, err
		}
		if codeExists {
			return nil, errors.New("Kostenstelle mit diesem Code existiert bereits")
		}
		add("code", *u.Code)
	}
	if u.Name != nil {
		if strings.TrimSpace(*u.Name) == "" {
			return nil, errors.New("Name erforderlich")
		}
		add("name", *u.Name)
	}
	if u.Aktiv != nil {
		add("aktiv", *u.Aktiv)
	}
	if u.Note != nil {
		add("note", *u.Note)
	}
	if len(sets) == 0 {
		return s.Get(ctx, id, companyID)
	}
	args = append(args, id, companyID)
	q := fmt.Sprintf("UPDATE cost_centers SET %s WHERE id=$%d AND company_id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}
