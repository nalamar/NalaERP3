package hr

import (
	"reflect"
	"testing"
	"time"
)

// berlin baut einen Zeitpunkt in der Arbeitszeitzone.
func berlin(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.ParseInLocation("2006-01-02 15:04", s, workLocation)
	if err != nil {
		t.Fatalf("Zeitpunkt %q nicht lesbar: %v", s, err)
	}
	return ts
}

// closedEntry baut einen abgeschlossenen, aktiven Eintrag. Der Arbeitstag
// wird wie im Service aus dem Beginn abgeleitet.
func closedEntry(t *testing.T, start, end string, breakMinutes int) TimeEntry {
	t.Helper()
	s, e := berlin(t, start), berlin(t, end)
	return TimeEntry{WorkDate: WorkDateOf(s), StartedAt: s, EndedAt: &e, BreakMinutes: breakMinutes, Status: TimeEntryStatusActive}
}

func openEntry(t *testing.T, start string) TimeEntry {
	t.Helper()
	s := berlin(t, start)
	return TimeEntry{WorkDate: WorkDateOf(s), StartedAt: s, Status: TimeEntryStatusActive}
}

func TestWorkDateOfUsesBerlinCalendarDay(t *testing.T) {
	cases := []struct {
		name      string
		startedAt time.Time
		want      string
	}{
		// 23:30 UTC ist in Berlin (Sommerzeit, UTC+2) bereits 01:30 am
		// Folgetag - der UTC-Kalendertag waere hier falsch.
		{"kurz nach Mitternacht Berlin, in UTC noch Vortag (Sommer)", time.Date(2026, 7, 14, 23, 30, 0, 0, time.UTC), "2026-07-15"},
		// Im Winter (UTC+1) liegt 22:30 UTC noch am selben Berliner Tag.
		{"22:30 UTC im Winter ist 23:30 Berlin, selber Tag", time.Date(2026, 1, 14, 22, 30, 0, 0, time.UTC), "2026-01-14"},
		{"23:30 UTC im Winter ist 00:30 Berlin, Folgetag", time.Date(2026, 1, 14, 23, 30, 0, 0, time.UTC), "2026-01-15"},
		{"Nachtschicht gehoert zum Tag des Beginns", berlin(t, "2026-10-12 22:00"), "2026-10-12"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WorkDateOf(tc.startedAt)
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("erwartet %s, bekommen %s", tc.want, got.Format("2006-01-02"))
			}
			if got.Location() != time.UTC || got.Hour() != 0 || got.Minute() != 0 {
				t.Fatalf("erwartet Mitternacht UTC, bekommen %s", got)
			}
		})
	}
}

func TestValidateTimeEntryRejectsOnlyTheImpossible(t *testing.T) {
	now := berlin(t, "2026-10-13 18:00")

	withBreak := func(e TimeEntry, m int) TimeEntry { e.BreakMinutes = m; return e }

	cases := []struct {
		name    string
		entry   TimeEntry
		wantErr string
	}{
		{"normaler Tag", closedEntry(t, "2026-10-13 07:00", "2026-10-13 16:00", 45), ""},
		{"laufender Eintrag", openEntry(t, "2026-10-13 07:00"), ""},
		{"Beginn genau jetzt", openEntry(t, "2026-10-13 18:00"), ""},
		{"Ende genau jetzt", closedEntry(t, "2026-10-13 07:00", "2026-10-13 18:00", 60), ""},
		// Kernentscheidung der ADR: Verstoesse gegen das ArbZG sind
		// KEINE Validierungsfehler.
		{"11 Stunden ohne Pause wird angenommen", closedEntry(t, "2026-10-13 06:00", "2026-10-13 17:00", 0), ""},
		{"14 Stunden wird angenommen", closedEntry(t, "2026-10-13 03:00", "2026-10-13 17:00", 30), ""},
		{"Pause eine Minute kuerzer als Anwesenheit", closedEntry(t, "2026-10-13 08:00", "2026-10-13 09:00", 59), ""},

		{"Beginn fehlt", TimeEntry{}, "Beginn erforderlich"},
		{"Beginn in der Zukunft", openEntry(t, "2026-10-13 18:01"), "Beginn darf nicht in der Zukunft liegen"},
		{"Ende vor Beginn", closedEntry(t, "2026-10-13 09:00", "2026-10-13 08:00", 0), "Ende muss nach dem Beginn liegen"},
		{"Ende gleich Beginn", closedEntry(t, "2026-10-13 09:00", "2026-10-13 09:00", 0), "Ende muss nach dem Beginn liegen"},
		{"Ende in der Zukunft", closedEntry(t, "2026-10-13 07:00", "2026-10-13 18:01", 0), "Ende darf nicht in der Zukunft liegen"},
		{"negative Pause", closedEntry(t, "2026-10-13 08:00", "2026-10-13 16:00", -5), "Pause darf nicht negativ sein"},
		{"Pause gleich Anwesenheit", closedEntry(t, "2026-10-13 08:00", "2026-10-13 09:00", 60), "Pause muss kürzer sein als die Anwesenheit"},
		{"Pause laenger als Anwesenheit", closedEntry(t, "2026-10-13 08:00", "2026-10-13 09:00", 90), "Pause muss kürzer sein als die Anwesenheit"},
		{"Pause an laufendem Eintrag", withBreak(openEntry(t, "2026-10-13 07:00"), 30), "Pause kann erst beim Beenden angegeben werden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTimeEntry(tc.entry, now)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("erwartet kein Fehler, bekommen %q", err.Error())
				}
				return
			}
			if err == nil {
				t.Fatalf("erwartet Fehler %q, bekommen nil", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %q", tc.wantErr, err.Error())
			}
		})
	}
}

// Jede Schwelle wird an drei Stellen geprueft: knapp darunter, GENAU
// darauf und knapp darueber. Das Gesetz sagt durchgehend "mehr als" bzw.
// "mindestens" - der Wert genau auf der Schwelle ist jeweils der Fall, in
// dem ein falscher Vergleichsoperator unbemerkt bliebe.
func TestEvaluateWorkDaysSingleDayThresholds(t *testing.T) {
	cases := []struct {
		name      string
		entries   []TimeEntry
		wantNet   int
		wantBreak int
		wantHints []string
	}{
		{"normaler Tag: 8 h netto, 45 min Pause", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 15:45", 45)}, 480, 45, []string{}},

		// §4: Pause erst bei MEHR als 6 Stunden.
		{"genau 6 h ohne Pause: kein Hinweis", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 13:00", 0)}, 360, 0, []string{}},
		{"6 h 1 min ohne Pause", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 13:01", 0)}, 361, 0, []string{HinweisPauseZuKurz}},
		{"6 h 1 min mit 29 min Pause", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 13:30", 29)}, 361, 29, []string{HinweisPauseZuKurz}},
		{"6 h 1 min mit genau 30 min Pause: kein Hinweis", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 13:31", 30)}, 361, 30, []string{}},

		// §4: 45 Minuten erst bei MEHR als 9 Stunden.
		{"genau 9 h mit 30 min Pause: nur ueber 8 h", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 16:30", 30)}, 540, 30, []string{HinweisUeber8h}},
		{"9 h 1 min mit 30 min Pause", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 16:31", 30)}, 541, 30, []string{HinweisUeber8h, HinweisPauseZuKurz}},
		{"9 h 1 min mit 44 min Pause", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 16:45", 44)}, 541, 44, []string{HinweisUeber8h, HinweisPauseZuKurz}},
		{"9 h 1 min mit genau 45 min Pause", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 16:46", 45)}, 541, 45, []string{HinweisUeber8h}},

		// §3: 8 und 10 Stunden.
		{"genau 8 h: kein Hinweis", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 15:30", 30)}, 480, 30, []string{}},
		{"8 h 1 min", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 15:31", 30)}, 481, 30, []string{HinweisUeber8h}},
		{"genau 10 h: nur ueber 8 h", []TimeEntry{closedEntry(t, "2026-10-13 06:00", "2026-10-13 16:45", 45)}, 600, 45, []string{HinweisUeber8h}},
		{"10 h 1 min", []TimeEntry{closedEntry(t, "2026-10-13 06:00", "2026-10-13 16:46", 45)}, 601, 45, []string{HinweisUeber8h, HinweisUeber10h}},

		{"11 h ohne Pause: alle drei Tageshinweise", []TimeEntry{closedEntry(t, "2026-10-13 06:00", "2026-10-13 17:00", 0)}, 660, 0, []string{HinweisUeber8h, HinweisUeber10h, HinweisPauseZuKurz}},

		// Die Pause zaehlt nicht zur Arbeitszeit: 9 h Anwesenheit mit 60
		// min Pause sind 8 h Arbeit, nicht 9.
		{"Pause wird von der Anwesenheit abgezogen", []TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 16:00", 60)}, 480, 60, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateWorkDays(tc.entries)
			if len(got) != 1 {
				t.Fatalf("erwartet genau einen Tag, bekommen %d", len(got))
			}
			if got[0].NetMinutes != tc.wantNet || got[0].BreakMinutes != tc.wantBreak {
				t.Fatalf("erwartet netto=%d pause=%d, bekommen netto=%d pause=%d", tc.wantNet, tc.wantBreak, got[0].NetMinutes, got[0].BreakMinutes)
			}
			if !reflect.DeepEqual(got[0].Hinweise, tc.wantHints) {
				t.Fatalf("erwartet Hinweise %v, bekommen %v", tc.wantHints, got[0].Hinweise)
			}
		})
	}
}

func TestEvaluateWorkDaysCountsGapsBetweenEntriesAsBreak(t *testing.T) {
	t.Run("ausgestempelte Mittagspause zaehlt als Pause", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 07:00", "2026-10-13 12:00", 0),
			closedEntry(t, "2026-10-13 12:30", "2026-10-13 16:00", 0),
		})
		if len(got) != 1 {
			t.Fatalf("erwartet einen Tag, bekommen %d", len(got))
		}
		// 5 h + 3,5 h = 8,5 h Arbeit, 30 min Pause: reicht (unter 9 h).
		if got[0].NetMinutes != 510 || got[0].BreakMinutes != 30 {
			t.Fatalf("erwartet netto=510 pause=30, bekommen netto=%d pause=%d", got[0].NetMinutes, got[0].BreakMinutes)
		}
		if !reflect.DeepEqual(got[0].Hinweise, []string{HinweisUeber8h}) {
			t.Fatalf("erwartet nur ueber_8h, bekommen %v", got[0].Hinweise)
		}
	})

	t.Run("Reihenfolge der Eingabe ist egal", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 12:30", "2026-10-13 16:00", 0),
			closedEntry(t, "2026-10-13 07:00", "2026-10-13 12:00", 0),
		})
		if got[0].NetMinutes != 510 || got[0].BreakMinutes != 30 {
			t.Fatalf("erwartet netto=510 pause=30, bekommen netto=%d pause=%d", got[0].NetMinutes, got[0].BreakMinutes)
		}
	})

	// §4 Satz 2: Ruhepausen bestehen aus Abschnitten von mindestens 15
	// Minuten. Drei Unterbrechungen von je 10 Minuten sind 30 Minuten
	// Abwesenheit, aber KEINE Ruhepause.
	t.Run("Luecken unter 15 Minuten sind keine Ruhepause", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 07:00", "2026-10-13 09:00", 0),
			closedEntry(t, "2026-10-13 09:10", "2026-10-13 11:00", 0),
			closedEntry(t, "2026-10-13 11:10", "2026-10-13 13:00", 0),
			closedEntry(t, "2026-10-13 13:10", "2026-10-13 14:30", 0),
		})
		// 2 h + 1 h 50 + 1 h 50 + 1 h 20 = 7 h.
		if got[0].NetMinutes != 420 || got[0].BreakMinutes != 0 {
			t.Fatalf("erwartet netto=420 pause=0, bekommen netto=%d pause=%d", got[0].NetMinutes, got[0].BreakMinutes)
		}
		if !reflect.DeepEqual(got[0].Hinweise, []string{HinweisPauseZuKurz}) {
			t.Fatalf("erwartet pause_zu_kurz, bekommen %v", got[0].Hinweise)
		}
	})

	t.Run("Luecke von genau 15 Minuten zaehlt", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 07:00", "2026-10-13 10:00", 0),
			closedEntry(t, "2026-10-13 10:15", "2026-10-13 13:00", 0),
			closedEntry(t, "2026-10-13 13:15", "2026-10-13 14:00", 0),
		})
		// 3 h + 2 h 45 + 45 min = 6,5 h Arbeit, 2 x 15 min Pause.
		if got[0].NetMinutes != 390 || got[0].BreakMinutes != 30 {
			t.Fatalf("erwartet netto=390 pause=30, bekommen netto=%d pause=%d", got[0].NetMinutes, got[0].BreakMinutes)
		}
		if len(got[0].Hinweise) != 0 {
			t.Fatalf("erwartet keine Hinweise, bekommen %v", got[0].Hinweise)
		}
	})

	t.Run("Luecke und Pausenminuten addieren sich", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 06:00", "2026-10-13 11:00", 15),
			closedEntry(t, "2026-10-13 11:30", "2026-10-13 16:30", 0),
		})
		// 4 h 45 + 5 h = 9 h 45 Arbeit, 15 + 30 = 45 min Pause: reicht.
		if got[0].NetMinutes != 585 || got[0].BreakMinutes != 45 {
			t.Fatalf("erwartet netto=585 pause=45, bekommen netto=%d pause=%d", got[0].NetMinutes, got[0].BreakMinutes)
		}
		if !reflect.DeepEqual(got[0].Hinweise, []string{HinweisUeber8h}) {
			t.Fatalf("erwartet nur ueber_8h, bekommen %v", got[0].Hinweise)
		}
	})
}

func TestEvaluateWorkDaysRestPeriod(t *testing.T) {
	cases := []struct {
		name        string
		firstEnd    string
		secondStart string
		wantHint    bool
	}{
		{"genau 11 h Ruhezeit: kein Hinweis", "2026-10-12 20:00", "2026-10-13 07:00", false},
		{"10 h 59 min Ruhezeit", "2026-10-12 20:01", "2026-10-13 07:00", true},
		{"11 h 1 min Ruhezeit", "2026-10-12 19:59", "2026-10-13 07:00", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateWorkDays([]TimeEntry{
				closedEntry(t, "2026-10-13 07:00", "2026-10-13 12:00", 0),
				// Beginn 15:00, damit der erste Tag in jedem Fall unter
				// 6 h bleibt und selbst keinen Pausenhinweis ausloest.
				closedEntry(t, "2026-10-12 15:00", tc.firstEnd, 0),
			})
			if len(got) != 2 {
				t.Fatalf("erwartet zwei Tage, bekommen %d", len(got))
			}
			if got[0].WorkDate != "2026-10-12" || got[1].WorkDate != "2026-10-13" {
				t.Fatalf("erwartet aufsteigende Tage, bekommen %s, %s", got[0].WorkDate, got[1].WorkDate)
			}
			if len(got[0].Hinweise) != 0 {
				t.Fatalf("erster Tag darf keinen Hinweis tragen, bekommen %v", got[0].Hinweise)
			}
			has := reflect.DeepEqual(got[1].Hinweise, []string{HinweisRuhezeitUnterschritten})
			if tc.wantHint && !has {
				t.Fatalf("erwartet ruhezeit_unterschritten am zweiten Tag, bekommen %v", got[1].Hinweise)
			}
			if !tc.wantHint && len(got[1].Hinweise) != 0 {
				t.Fatalf("erwartet keinen Hinweis, bekommen %v", got[1].Hinweise)
			}
		})
	}

	// Eine Nachtschicht endet am Folgetag, gehoert aber zum Tag ihres
	// Beginns. Die Ruhezeit laeuft ab ihrem tatsaechlichen Ende.
	t.Run("Ruhezeit nach Nachtschicht zaehlt ab dem tatsaechlichen Ende", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-12 22:00", "2026-10-13 06:00", 30),
			closedEntry(t, "2026-10-13 14:00", "2026-10-13 18:00", 0),
		})
		if len(got) != 2 {
			t.Fatalf("erwartet zwei Tage, bekommen %d", len(got))
		}
		if got[0].WorkDate != "2026-10-12" || got[0].NetMinutes != 450 {
			t.Fatalf("Nachtschicht: erwartet 2026-10-12 mit 450 min, bekommen %s mit %d", got[0].WorkDate, got[0].NetMinutes)
		}
		if !reflect.DeepEqual(got[1].Hinweise, []string{HinweisRuhezeitUnterschritten}) {
			t.Fatalf("erwartet ruhezeit_unterschritten (nur 8 h), bekommen %v", got[1].Hinweise)
		}
	})

	// Eine Luecke am selben Tag ist eine Pause, keine Ruhezeit.
	t.Run("Luecke innerhalb eines Tages loest keinen Ruhezeit-Hinweis aus", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 06:00", "2026-10-13 10:00", 0),
			closedEntry(t, "2026-10-13 15:00", "2026-10-13 18:00", 0),
		})
		if len(got) != 1 || len(got[0].Hinweise) != 0 {
			t.Fatalf("erwartet einen Tag ohne Hinweise, bekommen %+v", got)
		}
	})
}

func TestEvaluateWorkDaysIgnoresStornierteAndFlagsOpenEntries(t *testing.T) {
	t.Run("stornierter Eintrag zaehlt nicht", func(t *testing.T) {
		storniert := closedEntry(t, "2026-10-13 06:00", "2026-10-13 17:00", 0)
		storniert.Status = TimeEntryStatusStorniert
		got := EvaluateWorkDays([]TimeEntry{
			storniert,
			closedEntry(t, "2026-10-13 07:00", "2026-10-13 15:30", 30),
		})
		if len(got) != 1 {
			t.Fatalf("erwartet einen Tag, bekommen %d", len(got))
		}
		if got[0].NetMinutes != 480 || len(got[0].Hinweise) != 0 {
			t.Fatalf("erwartet 480 min ohne Hinweise, bekommen %d mit %v", got[0].NetMinutes, got[0].Hinweise)
		}
	})

	t.Run("Tag nur mit storniertem Eintrag erscheint nicht", func(t *testing.T) {
		storniert := closedEntry(t, "2026-10-13 06:00", "2026-10-13 17:00", 0)
		storniert.Status = TimeEntryStatusStorniert
		if got := EvaluateWorkDays([]TimeEntry{storniert}); len(got) != 0 {
			t.Fatalf("erwartet keine Tage, bekommen %+v", got)
		}
	})

	t.Run("laufender Eintrag markiert den Tag als offen und zaehlt nicht mit", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{
			closedEntry(t, "2026-10-13 07:00", "2026-10-13 12:00", 0),
			openEntry(t, "2026-10-13 12:30"),
		})
		if len(got) != 1 {
			t.Fatalf("erwartet einen Tag, bekommen %d", len(got))
		}
		if !got[0].Open {
			t.Fatal("erwartet Open=true")
		}
		if got[0].NetMinutes != 300 || got[0].BreakMinutes != 0 {
			t.Fatalf("erwartet netto=300 pause=0, bekommen netto=%d pause=%d", got[0].NetMinutes, got[0].BreakMinutes)
		}
	})

	t.Run("abgeschlossener Tag ist nicht offen", func(t *testing.T) {
		got := EvaluateWorkDays([]TimeEntry{closedEntry(t, "2026-10-13 07:00", "2026-10-13 12:00", 0)})
		if got[0].Open {
			t.Fatal("erwartet Open=false")
		}
	})

	t.Run("leere Eingabe liefert leere Liste, nicht nil", func(t *testing.T) {
		got := EvaluateWorkDays(nil)
		if got == nil || len(got) != 0 {
			t.Fatalf("erwartet leere Liste, bekommen %#v", got)
		}
	})
}
