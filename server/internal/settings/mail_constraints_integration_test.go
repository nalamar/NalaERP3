package settings

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Die Datenbank schuetzt den Mail-Zugang (Migration 104, ADR 0033) auch
// gegen Direkt-SQL. Erfundene Werte, kein echter Zugang.
func TestMailSettingsTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	newCompany := func() string {
		t.Helper()
		id := "ms-" + uuid.NewString()[:12]
		if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Mailtest "+id); err != nil {
			t.Fatalf("Mandant: %v", err)
		}
		return id
	}
	// Version 1, 12 Byte Nonce, 16 Byte Tag: 29 Byte.
	sealed := append([]byte{1}, make([]byte, 28)...)
	ins := `INSERT INTO company_mail_settings (company_id, aktiv, smtp_host, smtp_port, sicherheit, benutzer, password_enc, absender_adresse)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	row := func(aktiv bool, host string, port int, sicherheit, benutzer string, pw []byte, absender string) error {
		_, err := env.PG.Exec(ctx, ins, newCompany(), aktiv, host, port, sicherheit, benutzer, pw, absender)
		return err
	}
	for name, err := range map[string]error{
		"aktiv vollstaendig ssl":      row(true, "smtp.example.test", 465, "ssl", "erp@example.test", sealed, "erp@example.test"),
		"aktiv vollstaendig starttls": row(true, "smtp.example.test", 587, "starttls", "erp@example.test", sealed, "erp@example.test"),
		"inaktiv leer":                row(false, "", 465, "ssl", "", nil, ""),
	} {
		if err != nil {
			t.Fatalf("%s: abgelehnt: %v", name, err)
		}
	}
	for name, err := range map[string]error{
		"Port 0":              row(false, "", 0, "ssl", "", nil, ""),
		"Port zu gross":       row(false, "", 65536, "ssl", "", nil, ""),
		"unverschluesselt":    row(false, "", 25, "none", "", nil, ""),
		"Passwort Klartext":   row(false, "", 465, "ssl", "", []byte("klartext-passwort-ohne-version"), ""),
		"Passwort zu kurz":    row(false, "", 465, "ssl", "", sealed[:28], ""),
		"aktiv ohne Server":   row(true, " ", 465, "ssl", "erp@example.test", sealed, "erp@example.test"),
		"aktiv ohne Benutzer": row(true, "smtp.example.test", 465, "ssl", "", sealed, "erp@example.test"),
		"aktiv ohne Passwort": row(true, "smtp.example.test", 465, "ssl", "erp@example.test", nil, "erp@example.test"),
		"aktiv ohne Absender": row(true, "smtp.example.test", 465, "ssl", "erp@example.test", sealed, ""),
	} {
		if err == nil {
			t.Fatalf("%s: von der Datenbank angenommen", name)
		}
	}
	// Eine Zeile je Mandant; unbekannter Mandant abgelehnt.
	c := newCompany()
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_mail_settings (company_id) VALUES ($1)`, c); err != nil {
		t.Fatalf("Standardwerte: %v", err)
	}
	var port int
	var sicherheit string
	var aktiv bool
	if err := env.PG.QueryRow(ctx, `SELECT smtp_port, sicherheit, aktiv FROM company_mail_settings WHERE company_id=$1`, c).Scan(&port, &sicherheit, &aktiv); err != nil || port != 465 || sicherheit != "ssl" || aktiv {
		t.Fatalf("Standardwerte: %d %s %v %v", port, sicherheit, aktiv, err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_mail_settings (company_id) VALUES ($1)`, c); err == nil {
		t.Fatal("zweite Zeile je Mandant angenommen")
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_mail_settings (company_id) VALUES ('gibt-es-nicht')`); err == nil {
		t.Fatal("unbekannter Mandant angenommen")
	}
}
