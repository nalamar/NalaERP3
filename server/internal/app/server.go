package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"nalaerp3/internal/auth"
	"nalaerp3/internal/config"
	"nalaerp3/internal/db"
	httpx "nalaerp3/internal/http"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/migrate"
)

type Server struct {
	Cfg  *config.Config
	HTTP *http.Server
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
		if time.Now().After(deadline) {
			break
		}
		if pgConn == nil {
			if p, e := db.ConnectPostgres(ctx, cfg.PostgresDSN); e == nil {
				pgConn = p
			} else {
				err = e
			}
		}
		if mgConn == nil {
			if m, e := db.ConnectMongo(ctx, cfg.MongoURI); e == nil {
				mgConn = m
			} else {
				err = e
			}
		}
		if rdConn == nil {
			if r, e := db.ConnectRedis(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB); e == nil {
				rdConn = r
			} else {
				err = e
			}
		}
		if pgConn != nil && mgConn != nil && rdConn != nil {
			break
		}
		time.Sleep(1500 * time.Millisecond)
	}
	if pgConn == nil || mgConn == nil || rdConn == nil {
		if pgConn != nil {
			pgConn.Close()
		}
		if mgConn != nil {
			_ = mgConn.Disconnect(context.Background())
		}
		if rdConn != nil {
			_ = rdConn.Close()
		}
		return nil, err
	}

	// Migrations ausführen
	if err := migrate.Run(ctx, pgConn); err != nil {
		pgConn.Close()
		_ = mgConn.Disconnect(context.Background())
		return nil, err
	}

	// Erster Admin bei Neuinstallation (Backlog 0.49): nur wenn es noch
	// keinen einzigen Benutzer gibt. Das Passwort steht bewusst nicht im Log.
	if created, err := auth.EnsureBootstrapAdmin(ctx, pgConn); err != nil {
		pgConn.Close()
		_ = mgConn.Disconnect(context.Background())
		return nil, err
	} else if created {
		log.Printf("Erster Administrator angelegt: Benutzer %q - das Passwort muss bei der ersten Anmeldung geändert werden", auth.BootstrapAdminUsername)
	}

	// Laeufe der Zuordnung, die beim Ende des letzten Prozesses noch liefen,
	// koennen nicht weiterlaufen (ADR 0045 Frage 2): als abgebrochen markieren.
	if n, err := matching.NewLaufService(matching.NewStore(pgConn), nil).MarkiereAbgebrochen(ctx); err != nil {
		log.Printf("Zuordnung: abgebrochene Läufe nicht markiert: %v", err)
	} else if n > 0 {
		log.Printf("Zuordnung: %d abgebrochene Läufe markiert", n)
	}

	// Hinweis-Mails werktags 7:00 (F.14.4b) - nur im API-Prozess, die
	// Tests bauen den Router ohne app.New.
	if sch := newNoticeMailScheduler(pgConn, cfg); sch != nil {
		sch.Start(context.Background())
	}

	// Router mit Abhängigkeiten
	mux := httpx.NewRouterWithDeps(pgConn, mgConn, rdConn, cfg)

	srv := &http.Server{
		Addr:    cfg.APIAddr,
		Handler: mux,
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
