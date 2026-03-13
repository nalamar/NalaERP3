package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"nalaerp3/internal/auth"
	"nalaerp3/internal/config"
	"nalaerp3/internal/db"
	"nalaerp3/internal/migrate"
)

type IntegrationEnv struct {
	Cfg   *config.Config
	PG    *pgxpool.Pool
	Mongo *mongo.Client
	Redis *redis.Client
}

func SetupIntegrationEnv(t *testing.T) *IntegrationEnv {
	t.Helper()

	if os.Getenv("NALA_INTEGRATION") != "1" {
		t.Skip("integration tests disabled; set NALA_INTEGRATION=1")
	}

	cfg := &config.Config{
		APIAddr:                 ":0",
		PostgresDSN:             getenv("TEST_POSTGRES_DSN", "postgres://nala:secret@localhost:55432/nalaerp_test?sslmode=disable"),
		MongoURI:                getenv("TEST_MONGO_URI", "mongodb://localhost:57017"),
		MongoDB:                 getenv("TEST_MONGO_DB", "nalaerp_test"),
		RedisAddr:               getenv("TEST_REDIS_ADDR", "localhost:56379"),
		RedisPass:               getenv("TEST_REDIS_PASSWORD", ""),
		JWTSecret:               getenv("TEST_JWT_SECRET", "integration-secret"),
		AccessTokenTTLMinutes:   15,
		SessionTTLHours:         12,
		EmfPngDPI:               300,
		EmfPngMinWidth:          600,
		EmfPngTargetWidth:       1200,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	pg, err := db.ConnectPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(pg.Close)

	mg, err := db.ConnectMongo(ctx, cfg.MongoURI)
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	t.Cleanup(func() {
		_ = mg.Disconnect(context.Background())
	})

	rd, err := db.ConnectRedis(ctx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() {
		_ = rd.Close()
	})

	if err := migrate.Run(ctx, pg); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return &IntegrationEnv{
		Cfg:   cfg,
		PG:    pg,
		Mongo: mg,
		Redis: rd,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func SeedAuthUser(t *testing.T, env *IntegrationEnv, email, password string, roleCode string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := "itest-" + roleCode + "-" + email
	_, err = env.PG.Exec(ctx, `
		INSERT INTO users (
			id, email, username, password_hash, first_name, last_name, display_name, locale, timezone, is_active, is_locked
		) VALUES ($1,$2,$3,$4,'Integration','Test','Integration Test','de-DE','Europe/Berlin',true,false)
		ON CONFLICT (email) DO UPDATE SET
			username=EXCLUDED.username,
			password_hash=EXCLUDED.password_hash,
			first_name=EXCLUDED.first_name,
			last_name=EXCLUDED.last_name,
			display_name=EXCLUDED.display_name,
			locale=EXCLUDED.locale,
			timezone=EXCLUDED.timezone,
			is_active=true,
			is_locked=false,
			updated_at=now()
	`, userID, email, email, passwordHash)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	_, err = env.PG.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, r.id
		FROM roles r
		WHERE r.code = $2
		ON CONFLICT DO NOTHING
	`, userID, roleCode)
	if err != nil {
		t.Fatalf("seed user role: %v", err)
	}

	return userID
}
