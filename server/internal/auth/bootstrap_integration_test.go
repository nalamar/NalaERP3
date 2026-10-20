package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"nalaerp3/internal/auth"
	"nalaerp3/internal/db"
	"nalaerp3/internal/migrate"
	"nalaerp3/internal/testutil"
)

// Erster Admin bei Neuinstallation (Backlog 0.49). Die gemeinsame
// Test-DB hat schon Benutzer - deshalb eine eigene, frisch migrierte
// Datenbank, die danach wieder geloescht wird.

func TestBootstrapAdminOnEmptyDatabaseOnly(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()

	// Vorhandene Benutzer: nichts passiert.
	created, err := auth.EnsureBootstrapAdmin(ctx, env.PG)
	if err != nil || created {
		t.Fatalf("Test-DB mit Benutzern: created=%v, err=%v", created, err)
	}
	var admins int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM users WHERE username=$1`, auth.BootstrapAdminUsername).Scan(&admins); err != nil || admins != 0 {
		t.Fatalf("Bootstrap-Admin trotz vorhandener Benutzer: %d, %v", admins, err)
	}

	name := "nala_bootstrap_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := env.PG.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatalf("Datenbank anlegen: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.PG.Exec(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})
	dsn := strings.Replace(env.Cfg.PostgresDSN, "/nalaerp_test?", "/"+name+"?", 1)
	if dsn == env.Cfg.PostgresDSN {
		t.Fatalf("DSN ohne erwarteten Datenbanknamen: Test passt nicht zur Umgebung")
	}
	pg, err := db.ConnectPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("Verbinden: %v", err)
	}
	defer pg.Close()
	if err := migrate.Run(ctx, pg); err != nil {
		t.Fatalf("Migrationen: %v", err)
	}

	created, err = auth.EnsureBootstrapAdmin(ctx, pg)
	if err != nil || !created {
		t.Fatalf("leere DB: created=%v, err=%v", created, err)
	}
	// Zweiter Start: kein zweiter Admin, kein neues Passwort.
	var hashBefore string
	if err := pg.QueryRow(ctx, `SELECT password_hash FROM users WHERE username=$1`, auth.BootstrapAdminUsername).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	created, err = auth.EnsureBootstrapAdmin(ctx, pg)
	if err != nil || created {
		t.Fatalf("zweiter Start: created=%v, err=%v", created, err)
	}

	var n int
	var email, company, hash string
	var mustChange, active bool
	if err := pg.QueryRow(ctx, `SELECT count(*) OVER (), email, company_id, password_hash, must_change_password, is_active FROM users`).
		Scan(&n, &email, &company, &hash, &mustChange, &active); err != nil {
		t.Fatal(err)
	}
	if n != 1 || email != auth.BootstrapAdminEmail || company != "default" || !mustChange || !active || hash != hashBefore {
		t.Fatalf("Bootstrap-Admin: n=%d email=%s company=%s must=%v active=%v gleicherHash=%v", n, email, company, mustChange, active, hash == hashBefore)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("Admin")) != nil {
		t.Fatal("Passwort ist nicht Admin")
	}
	var roles string
	if err := pg.QueryRow(ctx, `SELECT string_agg(r.code, ',') FROM user_roles ur JOIN roles r ON r.id=ur.role_id`).Scan(&roles); err != nil || roles != "admin" {
		t.Fatalf("Rollen: %q, %v", roles, err)
	}
	// Login ueber den Benutzernamen, ohne Ruecksicht auf Gross/klein.
	u, err := auth.NewRepository(pg).FindUserByLogin(ctx, "admin")
	if err != nil || !u.MustChangePassword {
		t.Fatalf("Login per Benutzername: %+v, %v", u, err)
	}
}
