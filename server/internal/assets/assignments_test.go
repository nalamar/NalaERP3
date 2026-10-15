package assets

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Fachregeln der Ausgaben ohne Datenbank (Backlog F.3.4).

func TestValidateIssueDate(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		datum       time.Time
		anschaffung *time.Time
		wantErr     string
	}{
		{"heute", today, nil, ""},
		{"heute spaet in anderer Zone", time.Date(2026, 10, 15, 23, 30, 0, 0, time.FixedZone("x", 7200)), nil, ""},
		{"morgen", today.AddDate(0, 0, 1), nil, "Zukunft"},
		{"ohne Datum", time.Time{}, nil, "Ausgabedatum erforderlich"},
		{"am Anschaffungstag", today, datePtr(2026, 10, 15), ""},
		{"vor Anschaffung", today.AddDate(0, 0, -1), datePtr(2026, 10, 15), "vor dem Anschaffungsdatum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIssueDate(tc.datum, tc.anschaffung, today)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unerwarteter Fehler: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("erwartet Fehler mit %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateReturnDate(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	issued := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		datum   time.Time
		wantErr string
	}{
		{"heute", today, ""},
		{"am Ausgabetag", issued, ""},
		{"vor der Ausgabe", issued.AddDate(0, 0, -1), "vor dem Ausgabedatum"},
		{"morgen", today.AddDate(0, 0, 1), "Zukunft"},
		{"ohne Datum", time.Time{}, "Rückgabedatum erforderlich"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReturnDate(tc.datum, issued, today)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unerwarteter Fehler: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("erwartet Fehler mit %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

// Waechter fuer die Unveraenderbarkeit der Ausgaben (ADR 0028). Nicht
// entfernen.
//
// Der Service darf genau ZWEI UPDATEs auf asset_assignments enthalten -
// die einmalige Rueckgabe und das einmalige Storno, jeweils auf die
// zugehoerigen Spalten beschraenkt - und keinen DELETE. Wer einen weiteren
// Schreibpfad ergaenzt, soll hier anhalten und begruenden, warum er die
// Historie "wer hatte was wann" nicht beruehrt.
func TestAssignmentServiceHasNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("assignments.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)
	if got := strings.Count(code, "UPDATE asset_assignments"); got != 2 {
		t.Fatalf("erwartet genau 2 UPDATEs auf asset_assignments (Rueckgabe, Storno), gefunden %d", got)
	}
	for _, want := range []string{
		"UPDATE asset_assignments SET zurueckgegeben_am=$2, rueckgabe_zustand=$3, zurueckgenommen_by_user_id=$4",
		"WHERE id=$1 AND status='active' AND zurueckgegeben_am IS NULL",
		"UPDATE asset_assignments SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3",
		"WHERE id=$1 AND status='active'\n",
	} {
		if !strings.Contains(strings.ReplaceAll(code, "\r\n", "\n"), want) {
			t.Fatalf("erwartetes Fragment fehlt im Service: %s", want)
		}
	}
	if strings.Contains(code, "DELETE FROM") {
		t.Fatal("Ausgaben duerfen nicht geloescht werden")
	}
	// Auch der Asset-Stamm schreibt keine Ausgaben.
	stamm, err := os.ReadFile("assets.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	if strings.Contains(string(stamm), "asset_assignments SET") || strings.Contains(string(stamm), "DELETE FROM") {
		t.Fatal("assets.go darf Ausgaben weder aendern noch etwas loeschen")
	}
}
