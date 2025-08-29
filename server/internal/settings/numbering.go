package settings

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type NumberingService struct { pg *pgxpool.Pool }
func NewNumberingService(pg *pgxpool.Pool) *NumberingService { return &NumberingService{pg: pg} }

type NumberingConfig struct {
    Entity    string `json:"entity"`
    Pattern   string `json:"pattern"`
    NextValue int    `json:"next_value"`
    LastYear  *int   `json:"last_year"`
}

func pad(n int, width int) string {
    s := fmt.Sprintf("%d", n)
    if len(s) >= width { return s }
    return strings.Repeat("0", width-len(s)) + s
}

func formatWithPattern(pattern string, n int, t time.Time) string {
    y := t.Year()
    rep := map[string]string{
        "{YYYY}": fmt.Sprintf("%04d", y),
        "{YY}": fmt.Sprintf("%02d", y%100),
        "{MM}": fmt.Sprintf("%02d", int(t.Month())),
        "{DD}": fmt.Sprintf("%02d", t.Day()),
    }
    out := pattern
    for k, v := range rep { out = strings.ReplaceAll(out, k, v) }
    // find N-sequence
    // support {NN}, {NNN}, {NNNN}
    if strings.Contains(out, "{NNNN}") { out = strings.ReplaceAll(out, "{NNNN}", pad(n,4)) } else
    if strings.Contains(out, "{NNN}") { out = strings.ReplaceAll(out, "{NNN}", pad(n,3)) } else
    if strings.Contains(out, "{NN}") { out = strings.ReplaceAll(out, "{NN}", pad(n,2)) }
    return out
}

func (s *NumberingService) Get(ctx context.Context, entity string) (*NumberingConfig, error) {
    var cfg NumberingConfig
    var lastYear *int
    err := s.pg.QueryRow(ctx, `SELECT entity, pattern, next_value, last_year FROM number_sequences WHERE entity=$1`, entity).Scan(
        &cfg.Entity, &cfg.Pattern, &cfg.NextValue, &lastYear,
    )
    if err != nil { return nil, err }
    cfg.LastYear = lastYear
    return &cfg, nil
}

func (s *NumberingService) UpdatePattern(ctx context.Context, entity, pattern string) error {
    ct, err := s.pg.Exec(ctx, `UPDATE number_sequences SET pattern=$2, updated_at=now() WHERE entity=$1`, entity, pattern)
    if err != nil { return err }
    if ct.RowsAffected() == 0 {
        // create if missing
        _, err = s.pg.Exec(ctx, `INSERT INTO number_sequences (entity, pattern, next_value) VALUES ($1,$2,1)`, entity, pattern)
    }
    return err
}

func (s *NumberingService) Preview(ctx context.Context, entity string) (string, error) {
    cfg, err := s.Get(ctx, entity)
    if err != nil { return "", err }
    return formatWithPattern(cfg.Pattern, cfg.NextValue, time.Now()), nil
}

// Next returns next formatted number and increments the sequence atomically
func (s *NumberingService) Next(ctx context.Context, entity string) (string, error) {
    tx, err := s.pg.BeginTx(ctx, pgx.TxOptions{})
    if err != nil { return "", err }
    defer func(){ _ = tx.Rollback(ctx) }()
    var pattern string
    var next int
    var lastYear *int
    err = tx.QueryRow(ctx, `SELECT pattern, next_value, last_year FROM number_sequences WHERE entity=$1 FOR UPDATE`, entity).Scan(&pattern, &next, &lastYear)
    if err != nil {
        if err == pgx.ErrNoRows {
            pattern = "PO-{YYYY}-{NNNN}"; next = 1
            if _, e := tx.Exec(ctx, `INSERT INTO number_sequences (entity, pattern, next_value) VALUES ($1,$2,$3)`, entity, pattern, next); e != nil { return "", e }
        } else { return "", err }
    }
    now := time.Now()
    // reset per year if pattern uses {YYYY} or {YY}
    if strings.Contains(pattern, "{YYYY}") || strings.Contains(pattern, "{YY}") {
        ly := 0; if lastYear != nil { ly = *lastYear }
        if ly != now.Year() { next = 1 }
    }
    formatted := formatWithPattern(pattern, next, now)
    // increment and persist
    ny := now.Year()
    if _, err := tx.Exec(ctx, `UPDATE number_sequences SET next_value=$2, last_year=$3, updated_at=now() WHERE entity=$1`, entity, next+1, ny); err != nil { return "", err }
    if err := tx.Commit(ctx); err != nil { return "", err }
    return formatted, nil
}

