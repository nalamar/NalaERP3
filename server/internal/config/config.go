package config

import (
    "os"
    "strconv"
)

type Config struct {
    APIAddr string

    PostgresDSN  string
    MongoURI     string
    MongoDB      string
    RedisAddr    string
    RedisDB      int
    RedisPass    string
    JWTSecret    string
    AccessTokenTTLMinutes int
    SessionTTLHours int

    // Image conversion settings
    EmfPngDPI    int
    EmfPngMinWidth int
    EmfPngTargetWidth int
}

func getenv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func Load() *Config {
    cfg := &Config{}
    cfg.APIAddr = getenv("API_ADDR", ":8080")
    cfg.PostgresDSN = getenv("POSTGRES_DSN", "")
    if cfg.PostgresDSN == "" {
        host := getenv("POSTGRES_HOST", "postgres")
        port := getenv("POSTGRES_PORT", "5432")
        db := getenv("POSTGRES_DB", "nalaerp")
        user := getenv("POSTGRES_USER", "nala")
        pass := getenv("POSTGRES_PASSWORD", "secret")
        cfg.PostgresDSN = "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=disable"
    }
    cfg.MongoURI = getenv("MONGO_URI", "mongodb://mongo:27017")
    cfg.MongoDB = getenv("MONGO_DB", "nalaerp")
    cfg.RedisAddr = getenv("REDIS_ADDR", "redis:6379")
    cfg.RedisPass = getenv("REDIS_PASSWORD", "")
    if v := os.Getenv("REDIS_DB"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n >= 0 {
            cfg.RedisDB = n
        }
    }
    cfg.JWTSecret = getenv("JWT_SECRET", "dev-secret-change-me")
    cfg.AccessTokenTTLMinutes = 15
    if v := os.Getenv("ACCESS_TOKEN_TTL_MINUTES"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.AccessTokenTTLMinutes = n
        }
    }
    cfg.SessionTTLHours = 12
    if v := os.Getenv("SESSION_TTL_HOURS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.SessionTTLHours = n
        }
    }
    // EMF->PNG DPI (Standard 300)
    if v := os.Getenv("EMF_PNG_DPI"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.EmfPngDPI = n
        } else {
            cfg.EmfPngDPI = 300
        }
    } else {
        cfg.EmfPngDPI = 300
    }
    // EMF->PNG minimale Breite (Standard 600)
    if v := os.Getenv("EMF_PNG_MIN_WIDTH"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.EmfPngMinWidth = n
        } else {
            cfg.EmfPngMinWidth = 600
        }
    } else {
        cfg.EmfPngMinWidth = 600
    }
    // EMF->PNG Zielbreite (Standard 1200)
    if v := os.Getenv("EMF_PNG_TARGET_WIDTH"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.EmfPngTargetWidth = n
        } else {
            cfg.EmfPngTargetWidth = 1200
        }
    } else {
        cfg.EmfPngTargetWidth = 1200
    }
    return cfg
}
