package settings

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MaterialGroup struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

type MaterialGroupService struct{ pg *pgxpool.Pool }

func NewMaterialGroupService(pg *pgxpool.Pool) *MaterialGroupService {
	return &MaterialGroupService{pg: pg}
}

func (s *MaterialGroupService) List(ctx context.Context) ([]MaterialGroup, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT code, name, COALESCE(description, ''), sort_order, is_active
        FROM material_groups
        ORDER BY sort_order ASC, name ASC, code ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MaterialGroup, 0)
	for rows.Next() {
		var group MaterialGroup
		if err := rows.Scan(&group.Code, &group.Name, &group.Description, &group.SortOrder, &group.IsActive); err != nil {
			return nil, err
		}
		out = append(out, group)
	}
	return out, nil
}

func (s *MaterialGroupService) Upsert(ctx context.Context, in MaterialGroup) error {
	if in.Code = trim(in.Code); in.Code == "" {
		return errors.New("code erforderlich")
	}
	if in.Name = trim(in.Name); in.Name == "" {
		return errors.New("name erforderlich")
	}
	_, err := s.pg.Exec(ctx, `
        INSERT INTO material_groups (code, name, description, sort_order, is_active, updated_at)
        VALUES ($1, $2, $3, $4, $5, now())
        ON CONFLICT (code) DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            sort_order = EXCLUDED.sort_order,
            is_active = EXCLUDED.is_active,
            updated_at = now()
    `, in.Code, in.Name, trim(in.Description), in.SortOrder, in.IsActive)
	return err
}

func (s *MaterialGroupService) Delete(ctx context.Context, code string) error {
	if code = trim(code); code == "" {
		return errors.New("code erforderlich")
	}
	var refs int
	if err := s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM materials WHERE TRIM(kategorie) = $1`, code).Scan(&refs); err != nil {
		return err
	}
	if refs > 0 {
		return errors.New("Materialgruppe wird noch von Materialien verwendet")
	}
	_, err := s.pg.Exec(ctx, `DELETE FROM material_groups WHERE code = $1`, code)
	return err
}
