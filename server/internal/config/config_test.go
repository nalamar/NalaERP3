package config

import (
	"encoding/base64"
	"testing"
)

func TestValidateForStartupRejectsShortJWTSecretInProduction(t *testing.T) {
	cases := []struct {
		name      string
		appEnv    string
		jwtSecret string
		wantErr   bool
	}{
		{"production mit Code-Default", "production", "dev-secret-change-me", true},
		{"production mit .env.example-Platzhalter", "production", "change-me", true},
		{"production mit leerem Secret", "production", "", true},
		{"Production Grossschreibung mit kurzem Secret", "Production", "kurz", true},
		{"production mit ausreichend langem Secret", "production", "a-sufficiently-long-random-secret-value-1234567890", false},
		{"development mit Code-Default bleibt erlaubt", "development", "dev-secret-change-me", false},
		{"leeres AppEnv (Default) mit Code-Default bleibt erlaubt", "", "dev-secret-change-me", false},
		{"staging mit kurzem Secret bleibt erlaubt (kein production)", "staging", "kurz", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{AppEnv: tc.appEnv, JWTSecret: tc.jwtSecret}
			err := ValidateForStartup(cfg)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for appEnv=%q secret=%q, got nil", tc.appEnv, tc.jwtSecret)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for appEnv=%q secret=%q, got %v", tc.appEnv, tc.jwtSecret, err)
			}
		})
	}
}

func TestLoadDefaultsAppEnvToDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "")

	cfg := Load()
	if cfg.AppEnv != "development" {
		t.Fatalf("expected default AppEnv 'development', got %q", cfg.AppEnv)
	}
}

// SETTINGS_ENCRYPTION_KEY (ADR 0033): leer erlaubt, gesetzt nur gueltig -
// in jeder Umgebung.
func TestValidateForStartupSettingsEncryptionKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(make([]byte, 32))
	for name, tc := range map[string]struct {
		key     string
		wantErr bool
	}{
		"leer":        {"", false},
		"gueltig":     {good, false},
		"zu kurz":     {base64.StdEncoding.EncodeToString(make([]byte, 16)), true},
		"kein Base64": {"nicht-base64!", true},
	} {
		cfg := &Config{AppEnv: "development", SettingsEncryptionKey: tc.key}
		if err := ValidateForStartup(cfg); (err != nil) != tc.wantErr {
			t.Fatalf("%s: %v", name, err)
		}
	}
	t.Setenv("SETTINGS_ENCRYPTION_KEY", good)
	if cfg := Load(); cfg.SettingsEncryptionKey != good {
		t.Fatal("SETTINGS_ENCRYPTION_KEY nicht gelesen")
	}
}

func TestValidateForStartupMailRedirect(t *testing.T) {
	for name, tc := range map[string]struct {
		env, to string
		wantErr bool
	}{
		"leer":               {"development", "", false},
		"gueltig":            {"development", "test@example.test", false},
		"leer in Produktion": {"production", "", false},
		"in Produktion":      {"production", "test@example.test", true},
		"Produktion gross":   {"Production", "test@example.test", true},
		"mit Namen":          {"development", "Test <test@example.test>", true},
		"Liste":              {"development", "a@example.test, b@example.test", true},
		"keine Adresse":      {"development", "kein-at", true},
	} {
		cfg := &Config{AppEnv: tc.env, JWTSecret: "a-sufficiently-long-random-secret-value-1234567890", MailRedirectTo: tc.to}
		if err := ValidateForStartup(cfg); (err != nil) != tc.wantErr {
			t.Fatalf("%s: %v", name, err)
		}
	}
	t.Setenv("MAIL_REDIRECT_TO", " test@example.test ")
	if cfg := Load(); cfg.MailRedirectTo != "test@example.test" {
		t.Fatalf("MAIL_REDIRECT_TO: %q", cfg.MailRedirectTo)
	}
}

// Hinweis-Mails laufen, solange sie nicht ausdruecklich abgeschaltet sind.
func TestLoadNoticeMailsDisabled(t *testing.T) {
	for value, want := range map[string]bool{"": false, "false": false, "1": false, "true": true, " TRUE ": true} {
		t.Setenv("NOTICE_MAILS_DISABLED", value)
		if got := Load().NoticeMailsDisabled; got != want {
			t.Errorf("%q: %v", value, got)
		}
	}
}
