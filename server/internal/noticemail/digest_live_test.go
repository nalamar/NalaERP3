package noticemail

import (
	"context"
	"os"
	"strconv"
	"testing"

	"nalaerp3/internal/mail"
	"nalaerp3/internal/notices"
)

// Echter Versand einer Sammelmail an das Entwicklungskonto (Backlog
// F.14.4d): prueft Betreff, Umlaute und Umbrueche im Postfach. Nur mit
// NALA_SMTP_LIVE=1; Zugangsdaten und Empfaenger kommen aus der Umgebung
// (lokale .env, DEV_SMTP_*), nie aus dem Repo. Erfundene Anzahlen.
func TestDigestLive(t *testing.T) {
	if os.Getenv("NALA_SMTP_LIVE") != "1" {
		t.Skip("NALA_SMTP_LIVE=1 fuer echten Versand")
	}
	to := os.Getenv("DEV_SMTP_TEST_TO")
	if to == "" {
		t.Fatal("DEV_SMTP_TEST_TO fehlt - Testmails nur an die vorgegebene Testadresse")
	}
	subject, text, ok := BuildDigest([]notices.Notice{
		{Kind: notices.KindFleetSchedulesDue, Title: "Fällige Fahrzeugtermine", Severity: notices.SeverityKritisch, Total: 3,
			Counts: map[string]int{"faellig_bald": 1, "ueberfaellig": 2}},
		{Kind: notices.KindOwnQualifications, Title: "Meine Qualifikationen", Severity: notices.SeverityHinweis, Total: 1,
			Counts: map[string]int{"laeuft_ab": 1}},
	})
	if !ok {
		t.Fatal("keine Sammelmail gebaut")
	}
	port, _ := strconv.Atoi(os.Getenv("DEV_SMTP_PORT"))
	cfg := mail.SMTPConfig{Host: os.Getenv("DEV_SMTP_HOST"), Port: port, Sicherheit: os.Getenv("DEV_SMTP_SICHERHEIT"),
		Benutzer: os.Getenv("DEV_SMTP_USER"), Passwort: os.Getenv("DEV_SMTP_PASSWORD"), AbsenderAdresse: os.Getenv("DEV_SMTP_FROM"), AbsenderName: "Nala ERP Test"}
	if err := (&mail.Sender{}).Send(context.Background(), cfg, mail.Message{To: []string{to}, Subject: subject, Text: text}); err != nil {
		t.Fatalf("Versand: %v", err)
	}
}
