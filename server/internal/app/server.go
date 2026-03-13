package app

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.mongodb.org/mongo-driver/mongo"

    "nalaerp3/internal/config"
    "nalaerp3/internal/db"
    "nalaerp3/internal/migrate"
    httpx "nalaerp3/internal/http"
)

type Server struct {
    Cfg       *config.Config
    HTTP      *http.Server
}

func New(ctx context.Context, cfg *config.Config) (*Server, error) {
    // Verbindungen herstellen (mit einfachen Retries – Dienste brauchen Zeit in Compose)
    var (
        pgConn *pgxpool.Pool
        mgConn *mongo.Client
        rdConn *redis.Client
        err    error
    )

    deadline := time.Now().Add(45 * time.Second)
    for {
        if time.Now().After(deadline) { break }
        if pgConn == nil {
            if p, e := db.ConnectPostgres(ctx, cfg.PostgresDSN); e == nil { pgConn = p } else { err = e }
        }
        if mgConn == nil {
            if m, e := db.ConnectMongo(ctx, cfg.MongoURI); e == nil { mgConn = m } else { err = e }
        }
        if rdConn == nil {
            if r, e := db.ConnectRedis(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB); e == nil { rdConn = r } else { err = e }
        }
        if pgConn != nil && mgConn != nil && rdConn != nil { break }
        time.Sleep(1500 * time.Millisecond)
    }
    if pgConn == nil || mgConn == nil || rdConn == nil {
        if pgConn != nil { pgConn.Close() }
        if mgConn != nil { _ = mgConn.Disconnect(context.Background()) }
        if rdConn != nil { _ = rdConn.Close() }
        return nil, err
    }

    // Migrations ausführen
    if err := migrate.Run(ctx, pgConn); err != nil {
        pgConn.Close(); _ = mgConn.Disconnect(context.Background())
        return nil, err
    }

    // Router mit Abhängigkeiten
    mux := httpx.NewRouterWithDeps(pgConn, mgConn, rdConn, cfg)

    srv := &http.Server{
        Addr:              cfg.APIAddr,
        Handler:           mux,
        // Für Datei-Uploads (z. B. Logikal) großzügigere Timeouts
        ReadTimeout:       300 * time.Second,
        ReadHeaderTimeout: 15 * time.Second,
        WriteTimeout:      300 * time.Second,
        IdleTimeout:       120 * time.Second,
    }

    return &Server{Cfg: cfg, HTTP: srv}, nil
}

func (s *Server) Start() error {
    log.Printf("API startet auf %s", s.Cfg.APIAddr)
    return s.HTTP.ListenAndServe()
}
