package config

import "testing"

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
