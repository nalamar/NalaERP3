package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Mail-Einstellungen ohne Datenbank (Backlog F.14.2a). Erfundene Werte,
// kein echter Zugang.

func TestNormalizeInput(t *testing.T) {
	got := NormalizeInput(Input{SMTPHost: " Mail.Example.TEST ", Sicherheit: " STARTTLS ", Benutzer: " erp@example.test ", Passwort: " geheim ", AbsenderAdresse: " erp@example.test ", AbsenderName: " ERP "})
	if got.SMTPHost != "mail.example.test" || got.Sicherheit != SicherheitStartTLS || got.SMTPPort != 587 || got.Benutzer != "erp@example.test" ||
		got.AbsenderAdresse != "erp@example.test" || got.AbsenderName != "ERP" {
		t.Fatalf("%+v", got)
	}
	if got.Passwort != " geheim " {
		t.Fatalf("Passwort veraendert: %q", got.Passwort)
	}
	if d := NormalizeInput(Input{}); d.Sicherheit != SicherheitSSL || d.SMTPPort != 465 {
		t.Fatalf("Standard: %+v", d)
	}
	if d := NormalizeInput(Input{Sicherheit: "ssl", SMTPPort: 2465}); d.SMTPPort != 2465 {
		t.Fatalf("eigener Port: %+v", d)
	}
}

func TestValidateInput(t *testing.T) {
	valid := NormalizeInput(Input{Aktiv: true, SMTPHost: "mail.example.test", Benutzer: "erp@example.test", AbsenderAdresse: "erp@example.test", AbsenderName: "Nala ERP"})
	if err := ValidateInput(valid); err != nil {
		t.Fatalf("gueltig: %v", err)
	}
	if err := ValidateInput(NormalizeInput(Input{})); err != nil {
		t.Fatalf("inaktiv leer: %v", err)
	}
	mod := func(f func(*Input)) Input {
		in := valid
		f(&in)
		return in
	}
	for name, tc := range map[string]struct {
		in   Input
		want string
	}{
		"Verschluesselung":   {mod(func(i *Input) { i.Sicherheit = "none" }), "Ungültige Verschlüsselung (ssl oder starttls)"},
		"Port 0":             {mod(func(i *Input) { i.SMTPPort = 0 }), "Port muss zwischen 1 und 65535 liegen"},
		"Port zu gross":      {mod(func(i *Input) { i.SMTPPort = 65536 }), "Port muss zwischen 1 und 65535 liegen"},
		"Server mit Schema":  {mod(func(i *Input) { i.SMTPHost = "smtp://mail.example.test" }), "Mail-Server ungültig (nur der Name, z. B. mail.example.de)"},
		"Server mit Port":    {mod(func(i *Input) { i.SMTPHost = "mail.example.test:465" }), "Mail-Server ungültig (nur der Name, z. B. mail.example.de)"},
		"Server Leerzeichen": {mod(func(i *Input) { i.SMTPHost = "mail example" }), "Mail-Server ungültig (nur der Name, z. B. mail.example.de)"},
		"Server zu lang":     {mod(func(i *Input) { i.SMTPHost = strings.Repeat("a", 254) }), "Mail-Server ungültig (nur der Name, z. B. mail.example.de)"},
		"Benutzer Umbruch":   {mod(func(i *Input) { i.Benutzer = "a\nb" }), "Benutzer ungültig"},
		"Passwort Umbruch":   {mod(func(i *Input) { i.Passwort = "a\r\nRCPT TO:<x@example.test>" }), "Passwort ungültig"},
		"Passwort zu lang":   {mod(func(i *Input) { i.Passwort = strings.Repeat("x", 513) }), "Passwort ungültig"},
		"setzen und loeschen": {mod(func(i *Input) {
			i.Passwort = "x"
			i.PasswortLoeschen = true
		}), "Passwort entweder neu setzen oder löschen"},
		"Absender ungueltig":   {mod(func(i *Input) { i.AbsenderAdresse = "kein-at" }), "Absenderadresse ungültig"},
		"Absender mit Namen":   {mod(func(i *Input) { i.AbsenderAdresse = "ERP <erp@example.test>" }), "Absenderadresse ungültig"},
		"Absendername Umbruch": {mod(func(i *Input) { i.AbsenderName = "ERP\r\nBcc: x@example.test" }), "Absendername ungültig"},
		"aktiv ohne alles":     {NormalizeInput(Input{Aktiv: true}), "Für den Versand fehlt: Mail-Server, Benutzer, Absenderadresse"},
		"aktiv ohne Absender":  {mod(func(i *Input) { i.AbsenderAdresse = "" }), "Für den Versand fehlt: Absenderadresse"},
	} {
		if err := ValidateInput(tc.in); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Grenzen sind zulaessig.
	for name, in := range map[string]Input{
		"Port 1":          mod(func(i *Input) { i.SMTPPort = 1 }),
		"Port 65535":      mod(func(i *Input) { i.SMTPPort = 65535 }),
		"Passwort 512":    mod(func(i *Input) { i.Passwort = strings.Repeat("x", 512) }),
		"Server 253":      mod(func(i *Input) { i.SMTPHost = strings.Repeat("a", 253) }),
		"loeschen allein": mod(func(i *Input) { i.PasswortLoeschen = true }),
	} {
		if err := ValidateInput(in); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// Das Passwort gelangt weder ueber JSON noch ueber fmt nach aussen.
func TestSMTPConfigHidesPassword(t *testing.T) {
	c := SMTPConfig{Host: "mail.example.test", Port: 465, Sicherheit: SicherheitSSL, Benutzer: "erp@example.test", Passwort: "streng-geheim-123"}
	b, _ := json.Marshal(c)
	for name, s := range map[string]string{
		"json": string(b),
		"%v":   fmt.Sprintf("%v", c),
		"%+v":  fmt.Sprintf("%+v", c),
		"%#v":  fmt.Sprintf("%#v", c),
		"%s":   fmt.Sprintf("%s", &c),
	} {
		if strings.Contains(s, "streng-geheim") {
			t.Fatalf("%s zeigt das Passwort: %s", name, s)
		}
	}
	if !strings.Contains(c.String(), "mail.example.test:465") {
		t.Fatalf("String: %s", c.String())
	}
}

// Mandant und Eingabe werden vor Postgres geprueft (nil-Pool).
func TestSettingsServiceValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewSettingsService(nil, nil)
	ctx := context.Background()
	if _, err := s.Get(ctx, " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Get: %v", err)
	}
	if _, err := s.Config(ctx, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Config: %v", err)
	}
	if _, err := s.Save(ctx, Input{}, "", ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Save ohne Mandant: %v", err)
	}
	if _, err := s.Save(ctx, Input{Sicherheit: "none"}, "", "c1"); err == nil || err.Error() != "Ungültige Verschlüsselung (ssl oder starttls)" {
		t.Fatalf("Save ungueltig: %v", err)
	}
}
