package settings

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Unit struct {
    Code string `json:"code"`
    Name string `json:"name"`
}

type UnitService struct { pg *pgxpool.Pool }
func NewUnitService(pg *pgxpool.Pool) *UnitService { return &UnitService{pg: pg} }

func (s *UnitService) List(ctx context.Context) ([]Unit, error) {
    rows, err := s.pg.Query(ctx, `SELECT code, COALESCE(name,'') FROM units ORDER BY code ASC`)
    if err != nil { return nil, err }
    defer rows.Close()
    out := make([]Unit, 0)
    for rows.Next() {
        var u Unit
        if err := rows.Scan(&u.Code, &u.Name); err != nil { return nil, err }
        out = append(out, u)
    }
    return out, nil
}

func (s *UnitService) Upsert(ctx context.Context, code, name string) error {
    if code = trim(code); code == "" { return errors.New("code erforderlich") }
    _, err := s.pg.Exec(ctx, `INSERT INTO units (code, name) VALUES ($1,$2) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name`, code, name)
    return err
}

func (s *UnitService) Delete(ctx context.Context, code string) error {
    if code = trim(code); code == "" { return errors.New("code erforderlich") }
    _, err := s.pg.Exec(ctx, `DELETE FROM units WHERE code=$1`, code)
    return err
}

func trim(s string) string {
    for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') { s = s[1:] }
    for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') { s = s[:len(s)-1] }
    return s
}

