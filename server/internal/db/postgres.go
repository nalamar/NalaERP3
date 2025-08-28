package db

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

func ConnectPostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    cfg.MinConns = 1
    cfg.MaxConns = 10
    cfg.MaxConnLifetime = time.Hour
    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, err
    }
    ctxPing, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    if err := pool.Ping(ctxPing); err != nil {
        pool.Close()
        return nil, err
    }
    return pool, nil
}

