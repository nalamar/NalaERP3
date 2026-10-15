package assets

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Pruefungen ohne Datenbank (Backlog F.3.5.1).

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func TestNextInspectionDue(t *testing.T) {
	cases := []struct {
		name      string
		geprueft  time.Time
		intervall *int
		want      *time.Time
	}{
		{"ohne Intervall", d(2026, 3, 15), nil, nil},
		{"6 Monate", d(2026, 3, 15), intPtr(6), datePtr(2026, 9, 15)},
		{"12 Monate", d(2026, 3, 15), intPtr(12), datePtr(2027, 3, 15)},
		{"Monatsende begrenzt", d(2026, 8, 31), intPtr(6), datePtr(2027, 2, 28)},
		{"Schaltjahr", d(2027, 8, 31), intPtr(6), datePtr(2028, 2, 29)},
		// 00:30 Uhr am 16. bei +02:00 ist in UTC noch der 15.: gemeint ist
		// der Kalendertag des Werts.
		{"Uhrzeit in anderer Zone zaehlt als Kalendertag", time.Date(2026, 3, 16, 0, 30, 0, 0, time.FixedZone("x", 7200)), intPtr(3), datePtr(2026, 6, 16)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NextInspectionDue(tc.geprueft, tc.intervall)
			if (got == nil) != (tc.want == nil) || (got != nil && !got.Equal(*tc.want)) {
				t.Fatalf("erwartet %v, bekommen %v", tc.want, got)
			}
		})
	}
}

func TestValidateInspection(t *testing.T) {
	today := d(2026, 10, 15)
	cases := []struct {
		name        string
		geprueft    time.Time
		ergebnis    string
		naechste    *time.Time
		pruefer     string
		anschaffung *time.Time
		wantErr     string
	}{
		{"gueltig", today, ErgebnisBestanden, nil, "Elektro Meier", nil, ""},
		{"mit Maengeln und Folgetermin", today, ErgebnisMaengel, datePtr(2026, 10, 16), "X", nil, ""},
		{"nicht bestanden ohne Folgetermin", today, ErgebnisNichtBestanden, nil, "X", nil, ""},
		{"ohne Datum", time.Time{}, ErgebnisBestanden, nil, "X", nil, "Prüfdatum erforderlich"},
		{"morgen", today.AddDate(0, 0, 1), ErgebnisBestanden, nil, "X", nil, "Zukunft"},
		{"am Anschaffungstag", today, ErgebnisBestanden, nil, "X", datePtr(2026, 10, 15), ""},
		{"vor Anschaffung", today.AddDate(0, 0, -1), ErgebnisBestanden, nil, "X", datePtr(2026, 10, 15), "Anschaffungsdatum"},
		{"unbekanntes Ergebnis", today, "ok", nil, "X", nil, "Ungültiges Prüfergebnis"},
		{"ohne Pruefer", today, ErgebnisBestanden, nil, "  ", nil, "Prüfer erforderlich"},
		{"Folgetermin am Pruefdatum", today, ErgebnisBestanden, datePtr(2026, 10, 15), "X", nil, "nach dem Prüfdatum"},
		{"Folgetermin davor", today, ErgebnisBestanden, datePtr(2026, 10, 1), "X", nil, "nach dem Prüfdatum"},
		{"nicht bestanden mit Folgetermin", today, ErgebnisNichtBestanden, datePtr(2027, 1, 1), "X", nil, "keine nächste Prüfung"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInspection(tc.geprueft, tc.ergebnis, tc.naechste, tc.pruefer, tc.anschaffung, today)
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

func insp(geprueft time.Time, ergebnis string, naechste *time.Time, created time.Time) Inspection {
	return Inspection{ID: uuid.New(), GeprueftAm: geprueft, Ergebnis: ergebnis, NaechstePruefungAm: naechste, Status: InspectionActive, CreatedAt: created}
}

func TestEvaluateInspectionsStatus(t *testing.T) {
	today := d(2026, 10, 15)
	t0 := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		records   []Inspection
		intervall *int
		warn      int
		want      string
		wantTage  *int
		maengel   bool
	}{
		{"keine Pruefung, kein Intervall", nil, nil, 30, PruefstandNichtPruefpflichtig, nil, false},
		{"keine Pruefung, mit Intervall", nil, intPtr(12), 30, PruefstandNieGeprueft, nil, false},
		{"bestanden ohne Folgetermin", []Inspection{insp(d(2026, 1, 1), ErgebnisBestanden, nil, t0)}, nil, 30, PruefstandGeprueft, nil, false},
		{"weit vor Faelligkeit", []Inspection{insp(d(2026, 6, 1), ErgebnisBestanden, datePtr(2026, 11, 15), t0)}, intPtr(6), 30, PruefstandGeprueft, intPtr(31), false},
		{"genau Vorwarnzeit", []Inspection{insp(d(2026, 6, 1), ErgebnisBestanden, datePtr(2026, 11, 14), t0)}, intPtr(6), 30, PruefstandFaelligBald, intPtr(30), false},
		{"am Faelligkeitstag", []Inspection{insp(d(2026, 6, 1), ErgebnisBestanden, datePtr(2026, 10, 15), t0)}, intPtr(6), 30, PruefstandFaelligBald, intPtr(0), false},
		{"einen Tag ueberfaellig", []Inspection{insp(d(2026, 4, 1), ErgebnisBestanden, datePtr(2026, 10, 14), t0)}, intPtr(6), 30, PruefstandUeberfaellig, intPtr(-1), false},
		{"Vorwarnzeit 0, morgen faellig", []Inspection{insp(d(2026, 6, 1), ErgebnisBestanden, datePtr(2026, 10, 16), t0)}, intPtr(6), 0, PruefstandGeprueft, intPtr(1), false},
		{"negative Vorwarnzeit wie 0", []Inspection{insp(d(2026, 6, 1), ErgebnisBestanden, datePtr(2026, 10, 15), t0)}, intPtr(6), -5, PruefstandFaelligBald, intPtr(0), false},
		{"Maengel bleibt geprueft, mit Hinweis", []Inspection{insp(d(2026, 6, 1), ErgebnisMaengel, datePtr(2027, 6, 1), t0)}, intPtr(12), 30, PruefstandGeprueft, intPtr(229), true},
		{"nicht bestanden sperrt", []Inspection{insp(d(2026, 6, 1), ErgebnisNichtBestanden, nil, t0)}, intPtr(12), 30, PruefstandGesperrt, nil, false},
	}
	// "Heute" mit Uhrzeit zaehlt als Kalendertag: morgen faellig ist
	// genau 1 Tag, nicht ein Bruchteil.
	evening := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	if got := EvaluateInspections([]Inspection{insp(d(2026, 6, 1), ErgebnisBestanden, datePtr(2026, 10, 16), t0)}, intPtr(6), evening, 0); got.Status != PruefstandGeprueft || *got.TageBisFaellig != 1 {
		t.Fatalf("heute mit Uhrzeit: erwartet geprueft mit 1 Tag, bekommen %+v", got)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateInspections(tc.records, tc.intervall, today, tc.warn)
			if got.Status != tc.want {
				t.Fatalf("Status: erwartet %s, bekommen %s", tc.want, got.Status)
			}
			if (got.TageBisFaellig == nil) != (tc.wantTage == nil) || (got.TageBisFaellig != nil && *got.TageBisFaellig != *tc.wantTage) {
				t.Fatalf("Tage: erwartet %v, bekommen %v", tc.wantTage, got.TageBisFaellig)
			}
			if got.Maengel != tc.maengel {
				t.Fatalf("Maengel: erwartet %v, bekommen %v", tc.maengel, got.Maengel)
			}
			if len(tc.records) == 0 && (got.InspectionID != nil || got.GeprueftAm != nil || got.Ergebnis != "") {
				t.Fatalf("ohne Pruefung keine Angaben erwartet: %+v", got)
			}
		})
	}
}

func TestEvaluateInspectionsPicksLatest(t *testing.T) {
	today := d(2026, 10, 15)
	t0 := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)

	// Eine spaeter durchgefallene Pruefung loest die fruehere bestandene
	// ab - auch wenn diese laenger gelten wuerde.
	ok := insp(d(2026, 5, 1), ErgebnisBestanden, datePtr(2027, 5, 1), t0)
	failed := insp(d(2026, 9, 1), ErgebnisNichtBestanden, nil, t0)
	got := EvaluateInspections([]Inspection{ok, failed}, intPtr(12), today, 30)
	if got.Status != PruefstandGesperrt || *got.InspectionID != failed.ID {
		t.Fatalf("juengste (durchgefallene) erwartet: %+v", got)
	}
	// Reihenfolge der Eingabe egal.
	got = EvaluateInspections([]Inspection{failed, ok}, intPtr(12), today, 30)
	if *got.InspectionID != failed.ID {
		t.Fatalf("Reihenfolge der Eingabe darf nicht zaehlen: %+v", got)
	}
	// Nach erfolgreicher Wiederholung ist das Stueck wieder frei.
	repaired := insp(d(2026, 9, 20), ErgebnisBestanden, datePtr(2027, 9, 20), t0)
	got = EvaluateInspections([]Inspection{ok, failed, repaired}, intPtr(12), today, 30)
	if got.Status != PruefstandGeprueft || *got.InspectionID != repaired.ID || !got.GeprueftAm.Equal(d(2026, 9, 20)) ||
		got.Ergebnis != ErgebnisBestanden || !got.NaechstePruefungAm.Equal(d(2027, 9, 20)) {
		t.Fatalf("Wiederholung erwartet: %+v", got)
	}
	// Storniert zaehlt nicht: storniert man die Wiederholung, gilt wieder
	// die Sperre; storniert man auch die, wieder die erste.
	repaired.Status = InspectionStorniert
	got = EvaluateInspections([]Inspection{ok, failed, repaired}, intPtr(12), today, 30)
	if *got.InspectionID != failed.ID {
		t.Fatalf("stornierte Wiederholung darf nicht zaehlen: %+v", got)
	}
	failed.Status = InspectionStorniert
	got = EvaluateInspections([]Inspection{ok, failed, repaired}, intPtr(12), today, 30)
	if *got.InspectionID != ok.ID || got.Status != PruefstandGeprueft {
		t.Fatalf("erwartet die erste Pruefung: %+v", got)
	}
	ok.Status = InspectionStorniert
	got = EvaluateInspections([]Inspection{ok, failed, repaired}, intPtr(12), today, 30)
	if got.Status != PruefstandNieGeprueft || got.InspectionID != nil {
		t.Fatalf("alles storniert = nie geprueft: %+v", got)
	}

	// Gleicher Pruefungstag: die zuletzt erfasste zaehlt.
	morning := insp(d(2026, 9, 1), ErgebnisNichtBestanden, nil, time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC))
	afternoon := insp(d(2026, 9, 1), ErgebnisBestanden, datePtr(2027, 9, 1), time.Date(2026, 9, 1, 15, 0, 0, 0, time.UTC))
	for _, order := range [][]Inspection{{morning, afternoon}, {afternoon, morning}} {
		got = EvaluateInspections(order, intPtr(12), today, 30)
		if *got.InspectionID != afternoon.ID {
			t.Fatalf("gleicher Tag: die zuletzt erfasste erwartet: %+v", got)
		}
	}
}

// Waechter fuer die Unveraenderbarkeit der Pruefungen (ADR 0028). Nicht
// entfernen.
//
// Der Service darf genau EINEN UPDATE auf asset_inspections enthalten -
// das einmalige Stornieren, beschraenkt auf aktive Pruefungen und die vier
// Storno-Spalten - und keinen DELETE.
func TestInspectionServiceHasNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("inspections.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := strings.ReplaceAll(string(src), "\r\n", "\n")
	if got := strings.Count(code, "UPDATE asset_inspections"); got != 1 {
		t.Fatalf("erwartet genau 1 UPDATE auf asset_inspections (Stornieren), gefunden %d", got)
	}
	for _, want := range []string{
		"UPDATE asset_inspections SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3",
		"WHERE id=$1 AND status='active'\n",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("erwartetes Fragment fehlt im Service: %s", want)
		}
	}
	if strings.Contains(code, "DELETE FROM") {
		t.Fatal("Pruefungen duerfen nicht geloescht werden")
	}
	for _, f := range []string{"assets.go", "assignments.go"} {
		other, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("Quelltext lesen: %v", err)
		}
		if strings.Contains(string(other), "UPDATE asset_inspections") {
			t.Fatalf("%s darf Pruefungen nicht aendern", f)
		}
	}
}
