package noticemail

import (
	"strings"
	"testing"

	"nalaerp3/internal/notices"
)

// Mailtext der Sammelmail ohne Datenbank (Backlog F.14.4a).

func TestBuildDigest(t *testing.T) {
	if _, _, ok := BuildDigest(nil); ok {
		t.Fatal("ohne Hinweise darf es keine Mail geben")
	}
	subject, text, ok := BuildDigest([]notices.Notice{
		{Kind: notices.KindFleetSchedulesDue, Title: "Fällige Fahrzeugtermine", Severity: notices.SeverityKritisch, Total: 3,
			Counts: map[string]int{"faellig_bald": 1, "ueberfaellig": 2}},
		{Kind: notices.KindOwnQualifications, Title: "Meine Qualifikationen", Severity: notices.SeverityHinweis, Total: 1,
			Counts: map[string]int{"laeuft_ab": 1}},
		{Kind: "neu", Title: "Neue Art", Severity: notices.SeverityHinweis, Total: 2,
			Counts: map[string]int{"zzz": 1, "aaa": 1}},
	})
	if !ok || subject != "Nala ERP: 6 fällige Hinweise" {
		t.Fatalf("Betreff: %q %v", subject, ok)
	}
	want := "Guten Morgen,\n\nim Nala ERP ist für Sie heute Folgendes fällig:\n\n" +
		"- Fällige Fahrzeugtermine: 3 (2 überfällig, 1 bald fällig) [dringend]\n" +
		"- Meine Qualifikationen: 1 (1 läuft ab)\n" +
		"- Neue Art: 2 (1 aaa, 1 zzz)\n" +
		"\nDie Einzelheiten stehen im System auf dem Dashboard.\n\n" +
		"Diese Mail kommt werktags nur, wenn etwas fällig ist. Abbestellen können Sie sie im System unter \"Hinweis-Mails\".\n"
	if text != want {
		t.Fatalf("Text:\n%s\nerwartet:\n%s", text, want)
	}
}

// Alle Staende der Quellen haben eine Beschriftung, und die Reihenfolge
// stellt das Dringende voran.
func TestStatusLabelsAndOrder(t *testing.T) {
	for _, s := range statusOrder {
		if statusLabels[s] == "" {
			t.Errorf("ohne Beschriftung: %s", s)
		}
	}
	if len(statusLabels) != len(statusOrder) {
		t.Fatalf("Beschriftungen %d, Reihenfolge %d", len(statusLabels), len(statusOrder))
	}
	_, text, _ := BuildDigest([]notices.Notice{{Title: "X", Total: 7, Counts: map[string]int{
		"faellig_bald": 1, "laeuft_ab": 1, "nie_geprueft": 1, "ueberfaellig": 1, "abgelaufen": 1, "fehlt": 1, "gesperrt": 1}}})
	// Unbekannte Staende hinter den bekannten.
	if _, mixed, _ := BuildDigest([]notices.Notice{{Title: "Y", Total: 2, Counts: map[string]int{"aaa": 1, "faellig_bald": 1}}}); !strings.Contains(mixed, "- Y: 2 (1 bald fällig, 1 aaa)\n") {
		t.Fatalf("unbekannter Stand: %s", mixed)
	}
	if !strings.Contains(text, "- X: 7 (1 gesperrt, 1 fehlt, 1 abgelaufen, 1 überfällig, 1 nie geprüft, 1 läuft ab, 1 bald fällig)\n") {
		t.Fatalf("Reihenfolge: %s", text)
	}
}
