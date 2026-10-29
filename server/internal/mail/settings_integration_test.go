package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/secretbox"
	"nalaerp3/internal/testutil"
)

// Mail-Einstellungen gegen echtes Postgres (Backlog F.14.2a). Erfundene
// Werte und Schluessel, kein echter Zugang.

func seedMailCompany(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	id := "ml-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Mailtest "+id); err != nil {
		t.Fatalf("Mandant: %v", err)
	}
	return id
}

func testBox(t *testing.T, fill byte) *secretbox.Box {
	t.Helper()
	b, err := secretbox.New(bytes.Repeat([]byte{fill}, 32))
	if err != nil {
		t.Fatalf("Box: %v", err)
	}
	return b
}

func storedPassword(t *testing.T, env *testutil.IntegrationEnv, company string) []byte {
	t.Helper()
	var enc []byte
	if err := env.PG.QueryRow(context.Background(), `SELECT password_enc FROM company_mail_settings WHERE company_id=$1`, company).Scan(&enc); err != nil {
		t.Fatalf("lesen: %v", err)
	}
	return enc
}

func TestMailSettingsSaveGetConfig(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedMailCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "mail-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "admin")
	s := NewSettingsService(env.PG, testBox(t, 1))
	const pw = "erfundenes-Passwort-42"

	empty, err := s.Get(ctx, company)
	if err != nil || empty.Eingerichtet || empty.Aktiv || empty.SMTPPort != 465 || empty.Sicherheit != SicherheitSSL || empty.PasswortGesetzt || !empty.SchluesselEingerichtet {
		t.Fatalf("leer: %+v %v", empty, err)
	}
	if _, err := s.Config(ctx, company); err == nil || err.Error() != "E-Mail-Versand ist nicht vollständig eingerichtet" {
		t.Fatalf("Config leer: %v", err)
	}

	full := Input{Aktiv: true, SMTPHost: "Mail.Example.test", Sicherheit: "ssl", Benutzer: "erp@example.test", Passwort: pw, AbsenderAdresse: "erp@example.test", AbsenderName: "Nala ERP"}
	got, err := s.Save(ctx, full, actor, company)
	if err != nil {
		t.Fatalf("speichern: %v", err)
	}
	if !got.Eingerichtet || !got.Aktiv || got.SMTPHost != "mail.example.test" || got.SMTPPort != 465 || !got.PasswortGesetzt || !got.PasswortLesbar ||
		got.UpdatedByUserID == nil || *got.UpdatedByUserID != actor || got.UpdatedAt == nil {
		t.Fatalf("gespeichert: %+v", got)
	}
	// Weder JSON noch Datenbank zeigen das Passwort im Klartext.
	if b, _ := json.Marshal(got); strings.Contains(string(b), pw) {
		t.Fatalf("Passwort im JSON: %s", b)
	}
	enc := storedPassword(t, env, company)
	if bytes.Contains(enc, []byte(pw)) || enc[0] != 1 {
		t.Fatal("Passwort nicht verschluesselt gespeichert")
	}
	cfg, err := s.Config(ctx, company)
	if err != nil || cfg.Passwort != pw || cfg.Host != "mail.example.test" || cfg.Port != 465 || cfg.Benutzer != "erp@example.test" || !cfg.Aktiv || cfg.AbsenderName != "Nala ERP" {
		t.Fatalf("Config: %v %v", cfg, err)
	}

	// Leeres Passwort laesst das gespeicherte stehen (auch das Chiffrat).
	keep := full
	keep.Passwort = ""
	keep.Sicherheit = "starttls"
	keep.SMTPPort = 0
	if got, err := s.Save(ctx, keep, "", company); err != nil || !got.PasswortGesetzt || got.SMTPPort != 587 || got.Sicherheit != SicherheitStartTLS || got.UpdatedByUserID != nil {
		t.Fatalf("ohne Passwort: %+v %v", got, err)
	}
	if !bytes.Equal(storedPassword(t, env, company), enc) {
		t.Fatal("Passwort beim Speichern ohne Passwort veraendert")
	}
	if cfg, _ := s.Config(ctx, company); cfg.Passwort != pw {
		t.Fatal("Passwort nach Speichern verloren")
	}
	// Neues Passwort: neues Chiffrat.
	changed := full
	changed.Passwort = "anderes-Passwort"
	if _, err := s.Save(ctx, changed, "", company); err != nil {
		t.Fatalf("aendern: %v", err)
	}
	if cfg, _ := s.Config(ctx, company); cfg.Passwort != "anderes-Passwort" {
		t.Fatalf("geaendert: %v", cfg)
	}
	// Inaktiv bleibt der Zugang fuer einen Testversand lesbar.
	changed.Aktiv = false
	if _, err := s.Save(ctx, changed, "", company); err != nil {
		t.Fatalf("inaktiv: %v", err)
	}
	if cfg, err := s.Config(ctx, company); err != nil || cfg.Aktiv || cfg.Passwort != "anderes-Passwort" {
		t.Fatalf("Config inaktiv: %v %v", cfg, err)
	}

	// Aktiv ohne Passwort abgelehnt - nichts geaendert.
	del := full
	del.Passwort = ""
	del.PasswortLoeschen = true
	if _, err := s.Save(ctx, del, "", company); err == nil || err.Error() != "Für den Versand fehlt: Passwort" {
		t.Fatalf("aktiv geloescht: %v", err)
	}
	if storedPassword(t, env, company) == nil {
		t.Fatal("abgelehnte Aenderung hat das Passwort geloescht")
	}
	del.Aktiv = false
	got, err = s.Save(ctx, del, "", company)
	if err != nil || got.PasswortGesetzt || got.PasswortLesbar || got.Aktiv {
		t.Fatalf("loeschen: %+v %v", got, err)
	}
	if _, err := s.Config(ctx, company); err == nil || err.Error() != "E-Mail-Versand ist nicht vollständig eingerichtet" {
		t.Fatalf("Config ohne Passwort: %v", err)
	}
	// Inaktiv geht auch ohne Passwort neu anlegen.
	other := seedMailCompany(t, env)
	if _, err := s.Save(ctx, Input{Aktiv: true, SMTPHost: "mail.example.test", Benutzer: "a@example.test", AbsenderAdresse: "a@example.test"}, "", other); err == nil || err.Error() != "Für den Versand fehlt: Passwort" {
		t.Fatalf("neu aktiv ohne Passwort: %v", err)
	}
	if got, _ := s.Get(ctx, other); got.Eingerichtet {
		t.Fatal("abgelehnte Einstellung gespeichert")
	}
}

// Schluessel: ohne keiner speicherbar; mit anderem Schluessel oder in der
// Zeile eines anderen Mandanten nicht lesbar.
func TestMailSettingsKeyAndTenantBinding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	a, b := seedMailCompany(t, env), seedMailCompany(t, env)
	in := Input{Aktiv: true, SMTPHost: "mail.example.test", Benutzer: "erp@example.test", Passwort: "erfunden-1", AbsenderAdresse: "erp@example.test"}

	noKey := NewSettingsService(env.PG, nil)
	if _, err := noKey.Save(ctx, in, "", a); !errors.Is(err, secretbox.ErrNoKey) {
		t.Fatalf("ohne Schluessel: %v", err)
	}
	if got, _ := noKey.Get(ctx, a); got.Eingerichtet || got.SchluesselEingerichtet {
		t.Fatalf("ohne Schluessel gespeichert: %+v", got)
	}

	s := NewSettingsService(env.PG, testBox(t, 1))
	for _, c := range []string{a, b} {
		if _, err := s.Save(ctx, in, "", c); err != nil {
			t.Fatalf("speichern %s: %v", c, err)
		}
	}
	// Anderer Schluessel: Passwort gesetzt, aber nicht lesbar.
	rotated := NewSettingsService(env.PG, testBox(t, 2))
	if got, err := rotated.Get(ctx, a); err != nil || !got.PasswortGesetzt || got.PasswortLesbar {
		t.Fatalf("anderer Schluessel: %+v %v", got, err)
	}
	if _, err := rotated.Config(ctx, a); !errors.Is(err, secretbox.ErrOpen) {
		t.Fatalf("Config anderer Schluessel: %v", err)
	}
	if got, _ := noKey.Get(ctx, a); !got.PasswortGesetzt || got.PasswortLesbar {
		t.Fatalf("ohne Schluessel lesen: %+v", got)
	}
	// Neu eingetragen ist es mit dem neuen Schluessel wieder lesbar.
	if got, err := rotated.Save(ctx, in, "", a); err != nil || !got.PasswortLesbar {
		t.Fatalf("neu eingetragen: %+v %v", got, err)
	}

	// Chiffrat in die Zeile eines anderen Mandanten kopiert: nicht lesbar.
	if _, err := env.PG.Exec(ctx, `UPDATE company_mail_settings SET password_enc=(SELECT password_enc FROM company_mail_settings WHERE company_id=$1) WHERE company_id=$2`, a, b); err != nil {
		t.Fatalf("kopieren: %v", err)
	}
	if got, _ := rotated.Get(ctx, b); got.PasswortLesbar {
		t.Fatal("kopiertes Chiffrat beim anderen Mandanten lesbar")
	}
	if _, err := rotated.Config(ctx, b); !errors.Is(err, secretbox.ErrOpen) {
		t.Fatalf("Config kopiert: %v", err)
	}
	if cfg, err := rotated.Config(ctx, a); err != nil || cfg.Passwort != "erfunden-1" {
		t.Fatalf("eigener Mandant: %v", err)
	}
}
