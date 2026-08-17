package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectPostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	// pgx dekodiert timestamptz standardmaessig mit ScanLocation=nil, was
	// intern auf time.Unix(...) (= time.Local) zurueckfaellt - der Wert
	// landet dann in der Zeitzone des Go-PROZESSES (nicht der Postgres-
	// Sitzung), z.B. Europe/Berlin auf einer entsprechend konfigurierten
	// Maschine. Dieselbe Zeit wird dadurch inkonsistent serialisiert
	// (z.B. "+02:00" statt "Z"), obwohl der Instant korrekt bleibt
	// (Backlog 0.16: "expected due date to roundtrip"). Erzwingt UTC als
	// ScanLocation fuer jede neue Verbindung im Pool.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "timestamptz",
			OID:   pgtype.TimestamptzOID,
			Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
		})
		return nil
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
