// Package noticemail verschickt die Hinweise per E-Mail (ADR 0033,
// Backlog F.14.4): werktags eine Sammelmail je Benutzer, nur wenn etwas
// faellig ist, mit Versandprotokoll ohne Inhalt und Abbestellung je
// Benutzer.
//
// DSGVO: der Mailtext enthaelt nur Arten und Anzahlen wie das Dashboard
// (notices) - nie Namen, nie Einzelheiten. Die stehen im System hinter
// der Anmeldung.
package noticemail

import (
	"fmt"
	"sort"
	"strings"

	"nalaerp3/internal/notices"
)

// Beschriftung der Staende wie im Client (dashboard_notices.dart).
var statusLabels = map[string]string{
	"gesperrt":     "gesperrt",
	"fehlt":        "fehlt",
	"abgelaufen":   "abgelaufen",
	"ueberfaellig": "überfällig",
	"nie_geprueft": "nie geprüft",
	"laeuft_ab":    "läuft ab",
	"faellig_bald": "bald fällig",
}

// Reihenfolge der Staende im Text: das Dringendste zuerst.
var statusOrder = []string{"gesperrt", "fehlt", "abgelaufen", "ueberfaellig", "nie_geprueft", "laeuft_ab", "faellig_bald"}

func statusRank(s string) int {
	for i, x := range statusOrder {
		if x == s {
			return i
		}
	}
	return len(statusOrder)
}

// BuildDigest baut Betreff und Text der Sammelmail. Ohne Hinweise gibt es
// keine Mail (ok=false).
func BuildDigest(ns []notices.Notice) (subject, text string, ok bool) {
	if len(ns) == 0 {
		return "", "", false
	}
	total := 0
	var b strings.Builder
	b.WriteString("Guten Morgen,\n\nim Nala ERP ist für Sie heute Folgendes fällig:\n\n")
	for _, n := range ns {
		total += n.Total
		keys := make([]string, 0, len(n.Counts))
		for k := range n.Counts {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if ri, rj := statusRank(keys[i]), statusRank(keys[j]); ri != rj {
				return ri < rj
			}
			return keys[i] < keys[j]
		})
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			label := statusLabels[k]
			if label == "" {
				label = k
			}
			parts = append(parts, fmt.Sprintf("%d %s", n.Counts[k], label))
		}
		mark := ""
		if n.Severity == notices.SeverityKritisch {
			mark = " [dringend]"
		}
		fmt.Fprintf(&b, "- %s: %d (%s)%s\n", n.Title, n.Total, strings.Join(parts, ", "), mark)
	}
	b.WriteString("\nDie Einzelheiten stehen im System auf dem Dashboard.\n\n")
	b.WriteString("Diese Mail kommt werktags nur, wenn etwas fällig ist. Abbestellen können Sie sie im System unter \"Hinweis-Mails\".\n")
	return fmt.Sprintf("Nala ERP: %d fällige Hinweise", total), b.String(), true
}
