package apihttp

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.mongodb.org/mongo-driver/mongo"
    "nalaerp3/internal/version"
    "nalaerp3/internal/config"
)

// corsMiddleware allows cross-origin requests (dev: client on :3000)
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Accept-Language")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// NewRouter stellt den HTTP-Router bereit.
func NewRouter() http.Handler {
    r := chi.NewRouter()

    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    r.Use(middleware.AllowContentType("application/json", "multipart/form-data"))
    r.Use(corsMiddleware)
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            w.Header().Set("Content-Language", "de-DE")
            next.ServeHTTP(w, req)
        })
    })

    r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "status": "ok",
            "zeit":   time.Now().Format(time.RFC3339),
        })
    })

    r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "version":   version.Version,
            "commit":    version.Commit,
            "build_time": version.BuildTime,
        })
    })

    // API v1 Namespace
    r.Route("/api/v1", func(r chi.Router) {
        // Material-Routen folgen hier (MVP)
        r.Get("/materials", func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.WriteHeader(http.StatusOK)
            _ = json.NewEncoder(w).Encode([]any{})
        })
    })

    return r
}

// NewRouterWithDeps stellt den Router mit DB-Abhängigkeiten bereit.
func NewRouterWithDeps(pg *pgxpool.Pool, mg *mongo.Client, rd *redis.Client, cfg *config.Config) http.Handler {
    r := chi.NewRouter()

    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    r.Use(middleware.AllowContentType("application/json", "multipart/form-data"))
    r.Use(corsMiddleware)
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            w.Header().Set("Content-Language", "de-DE")
            next.ServeHTTP(w, req)
        })
    })

    // Basis
    r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        _ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "zeit": time.Now().Format(time.RFC3339)})
    })
    r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        _ = json.NewEncoder(w).Encode(map[string]any{"version": version.Version, "commit": version.Commit, "build_time": version.BuildTime})
    })

    // API v1
    r.Mount("/api/v1", NewV1Router(pg, mg, rd, cfg))
    return r
}
