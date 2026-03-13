package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type dependencyHealth struct {
	Status  string `json:"status"`
	Latency int64  `json:"latency_ms"`
	Error   string `json:"error,omitempty"`
}

type healthResponse struct {
	Status       string                      `json:"status"`
	Zeit         string                      `json:"zeit"`
	RequestID    string                      `json:"request_id,omitempty"`
	CorrelationID string                     `json:"correlation_id,omitempty"`
	Checks       map[string]dependencyHealth `json:"checks,omitempty"`
}

func writeSimpleHealth(w http.ResponseWriter, r *http.Request, status string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:        status,
		Zeit:          time.Now().Format(time.RFC3339),
		RequestID:     RequestIDFromContext(r.Context()),
		CorrelationID: CorrelationIDFromContext(r.Context()),
	})
}

func liveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSimpleHealth(w, r, "ok", http.StatusOK)
	}
}

func readyHandler(pg *pgxpool.Pool, mg *mongo.Client, rd *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]dependencyHealth{
			"postgres": pingPostgres(r.Context(), pg),
			"mongo":    pingMongo(r.Context(), mg),
			"redis":    pingRedis(r.Context(), rd),
		}

		overall := "ok"
		code := http.StatusOK
		for _, check := range checks {
			if check.Status != "ok" {
				overall = "degraded"
				code = http.StatusServiceUnavailable
				break
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:        overall,
			Zeit:          time.Now().Format(time.RFC3339),
			RequestID:     RequestIDFromContext(r.Context()),
			CorrelationID: CorrelationIDFromContext(r.Context()),
			Checks:        checks,
		})
	}
}

func pingPostgres(parent context.Context, pg *pgxpool.Pool) dependencyHealth {
	if pg == nil {
		return dependencyHealth{Status: "down", Error: "postgres pool is nil"}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	if err := pg.Ping(ctx); err != nil {
		return dependencyHealth{Status: "down", Latency: time.Since(start).Milliseconds(), Error: err.Error()}
	}
	return dependencyHealth{Status: "ok", Latency: time.Since(start).Milliseconds()}
}

func pingMongo(parent context.Context, mg *mongo.Client) dependencyHealth {
	if mg == nil {
		return dependencyHealth{Status: "down", Error: "mongo client is nil"}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	if err := mg.Ping(ctx, nil); err != nil {
		return dependencyHealth{Status: "down", Latency: time.Since(start).Milliseconds(), Error: err.Error()}
	}
	return dependencyHealth{Status: "ok", Latency: time.Since(start).Milliseconds()}
}

func pingRedis(parent context.Context, rd *redis.Client) dependencyHealth {
	if rd == nil {
		return dependencyHealth{Status: "down", Error: "redis client is nil"}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	if err := rd.Ping(ctx).Err(); err != nil {
		return dependencyHealth{Status: "down", Latency: time.Since(start).Milliseconds(), Error: err.Error()}
	}
	return dependencyHealth{Status: "ok", Latency: time.Since(start).Milliseconds()}
}
