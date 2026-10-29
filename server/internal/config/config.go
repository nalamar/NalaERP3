package config

import (
	"errors"
	netmail "net/mail"
	"os"
	"strconv"
	"strings"

	"nalaerp3/internal/secretbox"
)

type Config struct {
	APIAddr string
	AppEnv  string

	PostgresDSN           string
	MongoURI              string
	MongoDB               string
	RedisAddr             string
	RedisDB               int
	RedisPass             string
	JWTSecret             string
	AccessTokenTTLMinutes int
	SessionTTLHours       int

	// Schluessel fuer verschluesselte Einstellungen, z. B. das Passwort des
	// Mail-Zugangs (ADR 0033): Base64, 32 Byte. Leer = nicht eingerichtet.
	SettingsEncryptionKey string

	// Leitet jede ausgehende Mail an diese eine Adresse um (Entwicklung,
	// Backlog F.14.2c). Leer = keine Umleitung; in Produktion verboten.
	MailRedirectTo string

	// Rate-Limiting fuer /auth/login (Subtask 0.5.2): Anzahl fehlgeschlagener
	// Anmeldeversuche pro IP-Adresse, bevor sie fuer die Fensterdauer gesperrt wird.
	LoginRateLimitMaxAttempts   int
	LoginRateLimitWindowSeconds int

	// Image conversion settings
	EmfPngDPI         int
	EmfPngMinWidth    int
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
	cfg.AppEnv = getenv("APP_ENV", "development")
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
	cfg.SettingsEncryptionKey = os.Getenv("SETTINGS_ENCRYPTION_KEY")
	cfg.MailRedirectTo = strings.TrimSpace(os.Getenv("MAIL_REDIRECT_TO"))
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
	cfg.LoginRateLimitMaxAttempts = 10
	if v := os.Getenv("LOGIN_RATE_LIMIT_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LoginRateLimitMaxAttempts = n
		}
	}
	cfg.LoginRateLimitWindowSeconds = 60
	if v := os.Getenv("LOGIN_RATE_LIMIT_WINDOW_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LoginRateLimitWindowSeconds = n
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

// minProductionJWTSecretLength ist bewusst als Laengenschwelle statt als
// Liste bekannter unsicherer Werte definiert (z.B. "dev-secret-change-me",
// das Code-Default, oder "change-me", der Platzhalter aus .env.example) -
// eine Laengenpruefung faengt jeden schwachen Wert ab, nicht nur die
// aktuell bekannten, und bleibt robust, falls der Default-String sich
// spaeter aendert.
const minProductionJWTSecretLength = 32

// ValidateForStartup prueft sicherheitsrelevante Konfiguration, BEVOR der
// Server startet (Subtask 0.5.1). Gibt bewusst nur einen Fehler zurueck,
// statt selbst log.Fatal/os.Exit aufzurufen - das haelt die Funktion pur
// testbar; der Aufrufer (cmd/api/main.go) entscheidet ueber das
// tatsaechliche Beenden.
func ValidateForStartup(cfg *Config) error {
	if strings.EqualFold(cfg.AppEnv, "production") && len(cfg.JWTSecret) < minProductionJWTSecretLength {
		return errors.New("JWT_SECRET ist in Produktivumgebung (APP_ENV=production) nicht gesetzt oder zu kurz (mindestens 32 Zeichen erforderlich) - bitte einen sicheren, zufälligen Wert setzen, z.B. via 'openssl rand -base64 32'")
	}
	// Ein gesetzter, aber unbrauchbarer Schluessel ist ein Konfigurationsfehler;
	// fehlt er, laesst sich nur kein Passwort speichern (ADR 0033).
	if _, err := secretbox.FromEnvValue(cfg.SettingsEncryptionKey); err != nil {
		return err
	}
	// Eine Umleitung in Produktion wuerde jede Mail still verschlucken.
	if cfg.MailRedirectTo != "" {
		if strings.EqualFold(cfg.AppEnv, "production") {
			return errors.New("MAIL_REDIRECT_TO ist in Produktivumgebung (APP_ENV=production) nicht erlaubt")
		}
		if a, err := netmail.ParseAddress(cfg.MailRedirectTo); err != nil || a.Address != cfg.MailRedirectTo {
			return errors.New("MAIL_REDIRECT_TO ist keine gültige E-Mail-Adresse")
		}
	}
	return nil
}
