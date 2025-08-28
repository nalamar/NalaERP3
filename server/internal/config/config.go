package config

import (
    "os"
)

type Config struct {
    APIAddr string

    PostgresDSN  string
    MongoURI     string
    MongoDB      string
    RedisAddr    string
    RedisDB      int
    RedisPass    string
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
    // Redis DB Index optional; Standard 0 – einfach über Env weglasssen
    return cfg
}

