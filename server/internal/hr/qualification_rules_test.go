package hr

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func qDay(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("Datum %q nicht lesbar: %v", s, err)
	}
	return d
}

func qDayPtr(t *testing.T, s string) *time.Time {
	t.Helper()
	d := qDay(t, s)
	return &d
}

func TestQualificationValidUntil(t *testing.T) {
	cases := []struct {
		name     string
		erworben string
		months   int
		want     string
	}{
		{"jaehrlich: gleicher Tag im Folgejahr", "2026-03-15", 12, "2027-03-15"},
		{"zwei Jahre (Ersthelfer)", "2026-03-15", 24, "2028-03-15"},
		{"drei Jahre (Schweisserpruefung)", "2026-03-15", 36, "2029-03-15"},
		{"ein Monat", "2026-03-15", 1, "2026-04-15"},
		{"ueber den Jahreswechsel", "2026-11-30", 3, "2027-02-28"},
		// Den Tag gibt es im Zielmonat nicht: letzter Tag des Monats.
		// time.AddDate liefe hier in den Maerz ueber.
		{"31.01. plus ein Monat", "2027-01-31", 1, "2027-02-28"},
		{"31.01. plus ein Monat im Schaltjahr", "2028-01-31", 1, "2028-02-29"},
		{"31.08. plus sechs Monate", "2026-08-31", 6, "2027-02-28"},
		{"31.03. plus ein Monat", "2026-03-31", 1, "2026-04-30"},
		{"29.02. plus ein Jahr", "2028-02-29", 12, "2029-02-28"},
		{"29.02. plus vier Jahre", "2028-02-29", 48, "2032-02-29"},
		// Kein Ueberlauf noetig: der Tag existiert.
		{"28.02. plus ein Jahr", "2027-02-28", 12, "2028-02-28"},
		{"31.12. plus ein Jahr", "2026-12-31", 12, "2027-12-31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.months
			got := QualificationValidUntil(qDay(t, tc.erworben), &m)
			if got == nil {
				t.Fatal("erwartet ein Datum, bekommen nil")
			}
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("erwartet %s, bekommen %s", tc.want, got.Format("2006-01-02"))
			}
			if got.Location() != time.UTC || got.Hour() != 0 {
				t.Fatalf("erwartet Mitternacht UTC, bekommen %s", got)
			}
		})
	}

	t.Run("ohne Frist unbefristet", func(t *testing.T) {
		if got := QualificationValidUntil(qDay(t, "2026-03-15"), nil); got != nil {
			t.Fatalf("erwartet nil, bekommen %s", got)
		}
	})
}

func TestValidateQualificationDates(t *testing.T) {
	today := qDay(t, "2026-10-13")
	cases := []struct {
		name     string
		erworben time.Time
		bis      *time.Time
		wantErr  string
	}{
		{"normal", qDay(t, "2026-03-15"), qDayPtr(t, "2027-03-15"), ""},
		{"unbefristet", qDay(t, "2020-01-01"), nil, ""},
		{"heute erworben", qDay(t, "2026-10-13"), nil, ""},
		{"gueltig bis am Erwerbstag", qDay(t, "2026-03-15"), qDayPtr(t, "2026-03-15"), ""},
		// Ein bereits abgelaufener Nachweis darf erfasst werden: er ist
		// eine Tatsache, und die Historie soll vollstaendig sein koennen.
		{"bereits abgelaufen", qDay(t, "2020-03-15"), qDayPtr(t, "2022-03-15"), ""},
		// Uhrzeit und Zeitzone spielen keine Rolle: verglichen werden Tage.
		{"heute mit Uhrzeit am Abend", time.Date(2026, 10, 13, 23, 59, 0, 0, time.UTC), nil, ""},

		{"Erwerbsdatum fehlt", time.Time{}, nil, "Erwerbsdatum erforderlich"},
		{"morgen erworben", qDay(t, "2026-10-14"), nil, "Erwerbsdatum darf nicht in der Zukunft liegen"},
		{"gueltig bis vor Erwerb", qDay(t, "2026-03-15"), qDayPtr(t, "2026-03-14"), "Gültig-bis darf nicht vor dem Erwerbsdatum liegen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateQualificationDates(tc.erworben, tc.bis, today)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("erwartet kein Fehler, bekommen %q", err.Error())
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

// Jede Schwelle der Statusberechnung an drei Stellen: knapp davor, genau
// darauf, knapp danach.
func TestEvaluateQualificationsStatusThresholds(t *testing.T) {
	today := qDay(t, "2026-10-13")
	typeID := uuid.New()

	cases := []struct {
		name       string
		gueltigBis *time.Time
		warnDays   int
		wantStatus string
		wantDays   *int
	}{
		{"unbefristet", nil, 60, QualificationStandGueltig, nil},

		// gueltig_bis ist der LETZTE gueltige Tag.
		{"laeuft heute ab: gilt noch", qDayPtr(t, "2026-10-13"), 60, QualificationStandLaeuftAb, intPtr(0)},
		{"gestern abgelaufen", qDayPtr(t, "2026-10-12"), 60, QualificationStandAbgelaufen, intPtr(-1)},
		{"morgen letzter Tag", qDayPtr(t, "2026-10-14"), 60, QualificationStandLaeuftAb, intPtr(1)},
		{"seit einem Jahr abgelaufen", qDayPtr(t, "2025-10-13"), 60, QualificationStandAbgelaufen, intPtr(-365)},

		// Vorwarnzeit 60 Tage: 12.12.2026 ist Tag 60.
		{"59 Tage: Vorwarnung", qDayPtr(t, "2026-12-11"), 60, QualificationStandLaeuftAb, intPtr(59)},
		{"genau 60 Tage: Vorwarnung", qDayPtr(t, "2026-12-12"), 60, QualificationStandLaeuftAb, intPtr(60)},
		{"61 Tage: noch gueltig", qDayPtr(t, "2026-12-13"), 60, QualificationStandGueltig, intPtr(61)},

		// Die Vorwarnzeit ist ein Parameter.
		{"Vorwarnzeit 0: nur der letzte Tag warnt", qDayPtr(t, "2026-10-13"), 0, QualificationStandLaeuftAb, intPtr(0)},
		{"Vorwarnzeit 0: morgen ist noch gueltig", qDayPtr(t, "2026-10-14"), 0, QualificationStandGueltig, intPtr(1)},
		{"Vorwarnzeit 90", qDayPtr(t, "2027-01-11"), 90, QualificationStandLaeuftAb, intPtr(90)},
		{"negative Vorwarnzeit gilt als 0", qDayPtr(t, "2026-10-14"), -5, QualificationStandGueltig, intPtr(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := EmployeeQualification{ID: uuid.New(), TypeID: typeID, ErworbenAm: qDay(t, "2024-01-01"), GueltigBis: tc.gueltigBis, Status: QualificationRecordActive}
			got := EvaluateQualifications([]EmployeeQualification{rec}, nil, today, tc.warnDays)
			if len(got) != 1 {
				t.Fatalf("erwartet genau einen Eintrag, bekommen %d", len(got))
			}
			if got[0].Status != tc.wantStatus {
				t.Fatalf("erwartet Status %s, bekommen %s", tc.wantStatus, got[0].Status)
			}
			switch {
			case tc.wantDays == nil && got[0].TageBisAblauf != nil:
				t.Fatalf("erwartet keine Tagesangabe, bekommen %d", *got[0].TageBisAblauf)
			case tc.wantDays != nil && (got[0].TageBisAblauf == nil || *got[0].TageBisAblauf != *tc.wantDays):
				t.Fatalf("erwartet %d Tage, bekommen %v", *tc.wantDays, got[0].TageBisAblauf)
			}
			if got[0].QualificationID == nil || *got[0].QualificationID != rec.ID || got[0].Erforderlich {
				t.Fatalf("erwartet Bezug auf den Nachweis und nicht erforderlich: %+v", got[0])
			}
		})
	}
}

// Uhrzeit und Zeitzone von "heute" duerfen den Stand nicht verschieben.
func TestEvaluateQualificationsComparesCalendarDays(t *testing.T) {
	rec := EmployeeQualification{ID: uuid.New(), TypeID: uuid.New(), ErworbenAm: qDay(t, "2025-10-13"), GueltigBis: qDayPtr(t, "2026-10-13"), Status: QualificationRecordActive}
	for _, now := range []time.Time{
		time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 13, 23, 59, 59, 0, time.UTC),
	} {
		got := EvaluateQualifications([]EmployeeQualification{rec}, nil, now, 60)
		if got[0].Status != QualificationStandLaeuftAb || *got[0].TageBisAblauf != 0 {
			t.Fatalf("am letzten gueltigen Tag um %s: erwartet laeuft_ab/0, bekommen %s/%d", now.Format("15:04"), got[0].Status, *got[0].TageBisAblauf)
		}
	}

	// Faelle, in denen eine nicht auf den Tag gekuerzte Uhrzeit das
	// Ergebnis tatsaechlich verschiebt: ein angebrochener Tag darf nicht
	// als ganzer fehlen.
	evening := time.Date(2026, 10, 13, 23, 59, 59, 0, time.UTC)
	tomorrow := EmployeeQualification{ID: uuid.New(), TypeID: uuid.New(), ErworbenAm: qDay(t, "2025-10-14"), GueltigBis: qDayPtr(t, "2026-10-14"), Status: QualificationRecordActive}
	if got := EvaluateQualifications([]EmployeeQualification{tomorrow}, nil, evening, 60); *got[0].TageBisAblauf != 1 {
		t.Fatalf("am Abend vor dem letzten Tag: erwartet 1 Tag, bekommen %d", *got[0].TageBisAblauf)
	}
	// 61 Tage sind es am Morgen wie am Abend - und damit noch keine
	// Vorwarnung bei 60 Tagen Vorwarnzeit.
	day61 := EmployeeQualification{ID: uuid.New(), TypeID: uuid.New(), ErworbenAm: qDay(t, "2025-12-13"), GueltigBis: qDayPtr(t, "2026-12-13"), Status: QualificationRecordActive}
	if got := EvaluateQualifications([]EmployeeQualification{day61}, nil, evening, 60); got[0].Status != QualificationStandGueltig || *got[0].TageBisAblauf != 61 {
		t.Fatalf("61 Tage vor Ablauf am Abend: erwartet gueltig/61, bekommen %s/%d", got[0].Status, *got[0].TageBisAblauf)
	}
}

func TestEvaluateQualificationsPicksTheRecordThatLastsLongest(t *testing.T) {
	today := qDay(t, "2026-10-13")
	typeID := uuid.New()
	rec := func(erworben string, bis *time.Time) EmployeeQualification {
		return EmployeeQualification{ID: uuid.New(), TypeID: typeID, ErworbenAm: qDay(t, erworben), GueltigBis: bis, Status: QualificationRecordActive}
	}
	evaluate := func(t *testing.T, records ...EmployeeQualification) QualificationStanding {
		t.Helper()
		got := EvaluateQualifications(records, nil, today, 60)
		if len(got) != 1 {
			t.Fatalf("erwartet einen Eintrag je Art, bekommen %d", len(got))
		}
		return got[0]
	}

	t.Run("Wiederholung loest den alten Nachweis ab", func(t *testing.T) {
		old, renewed := rec("2025-09-01", qDayPtr(t, "2026-09-01")), rec("2026-08-20", qDayPtr(t, "2027-08-20"))
		got := evaluate(t, old, renewed)
		if *got.QualificationID != renewed.ID || got.Status != QualificationStandGueltig {
			t.Fatalf("erwartet den erneuerten Nachweis und gueltig, bekommen %+v", got)
		}
	})

	t.Run("Reihenfolge der Eingabe ist egal", func(t *testing.T) {
		old, renewed := rec("2025-09-01", qDayPtr(t, "2026-09-01")), rec("2026-08-20", qDayPtr(t, "2027-08-20"))
		if got := evaluate(t, renewed, old); *got.QualificationID != renewed.ID {
			t.Fatalf("erwartet den erneuerten Nachweis, bekommen %+v", got)
		}
	})

	// Eine lang gueltige Ausbildung und eine spaeter erfasste, kuerzer
	// laufende Auffrischung: gedeckt ist der Mitarbeiter, solange die
	// Ausbildung gilt.
	t.Run("laenger gueltiger aelterer Nachweis bleibt massgeblich", func(t *testing.T) {
		long, short := rec("2024-01-10", qDayPtr(t, "2029-01-10")), rec("2025-06-01", qDayPtr(t, "2026-06-01"))
		got := evaluate(t, long, short)
		if *got.QualificationID != long.ID || got.Status != QualificationStandGueltig {
			t.Fatalf("erwartet den laenger gueltigen Nachweis, bekommen %+v", got)
		}
	})

	t.Run("unbefristet schlaegt befristet", func(t *testing.T) {
		unlimited, limited := rec("2020-05-05", nil), rec("2026-01-01", qDayPtr(t, "2030-01-01"))
		got := evaluate(t, limited, unlimited)
		if *got.QualificationID != unlimited.ID || got.GueltigBis != nil || got.TageBisAblauf != nil {
			t.Fatalf("erwartet den unbefristeten Nachweis, bekommen %+v", got)
		}
	})

	t.Run("bei gleichem Ablauf der juengere", func(t *testing.T) {
		a, b := rec("2025-01-01", qDayPtr(t, "2027-01-01")), rec("2025-06-01", qDayPtr(t, "2027-01-01"))
		if got := evaluate(t, a, b); *got.QualificationID != b.ID {
			t.Fatalf("erwartet den juengeren Nachweis, bekommen %+v", got)
		}
	})

	t.Run("stornierter Nachweis zaehlt nicht", func(t *testing.T) {
		good := rec("2025-01-01", qDayPtr(t, "2026-01-01"))
		wrong := rec("2026-09-01", qDayPtr(t, "2027-09-01"))
		wrong.Status = QualificationRecordStorniert
		got := evaluate(t, good, wrong)
		if *got.QualificationID != good.ID || got.Status != QualificationStandAbgelaufen {
			t.Fatalf("erwartet den nicht stornierten, abgelaufenen Nachweis, bekommen %+v", got)
		}
	})
}

func TestEvaluateQualificationsRequiredAndMissing(t *testing.T) {
	today := qDay(t, "2026-10-13")
	have, missing, expired, extra := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	records := []EmployeeQualification{
		{ID: uuid.New(), TypeID: have, ErworbenAm: qDay(t, "2026-06-01"), GueltigBis: qDayPtr(t, "2027-06-01"), Status: QualificationRecordActive},
		{ID: uuid.New(), TypeID: expired, ErworbenAm: qDay(t, "2024-06-01"), GueltigBis: qDayPtr(t, "2025-06-01"), Status: QualificationRecordActive},
		{ID: uuid.New(), TypeID: extra, ErworbenAm: qDay(t, "2020-06-01"), Status: QualificationRecordActive},
	}
	byType := func(got []QualificationStanding) map[uuid.UUID]QualificationStanding {
		m := make(map[uuid.UUID]QualificationStanding, len(got))
		for _, s := range got {
			m[s.TypeID] = s
		}
		return m
	}

	got := EvaluateQualifications(records, []uuid.UUID{have, missing, expired, missing}, today, 60)
	if len(got) != 4 {
		t.Fatalf("erwartet vier Eintraege (drei mit Nachweis, einer fehlend), bekommen %d: %+v", len(got), got)
	}
	m := byType(got)
	if s := m[have]; s.Status != QualificationStandGueltig || !s.Erforderlich {
		t.Fatalf("vorhanden und erforderlich: %+v", s)
	}
	// Ein abgelaufener Nachweis ist "abgelaufen", nicht "fehlt": er war da.
	if s := m[expired]; s.Status != QualificationStandAbgelaufen || !s.Erforderlich || s.QualificationID == nil {
		t.Fatalf("abgelaufen und erforderlich: %+v", s)
	}
	if s := m[missing]; s.Status != QualificationStandFehlt || !s.Erforderlich || s.QualificationID != nil || s.ErworbenAm != nil || s.GueltigBis != nil || s.TageBisAblauf != nil {
		t.Fatalf("fehlend: erwartet Status fehlt ohne Nachweisangaben, bekommen %+v", s)
	}
	// Was jemand hat, ohne es zu brauchen, erscheint - als nicht erforderlich.
	if s := m[extra]; s.Status != QualificationStandGueltig || s.Erforderlich {
		t.Fatalf("vorhanden, nicht erforderlich: %+v", s)
	}

	t.Run("ein NUR stornierter Nachweis laesst die Anforderung fehlen", func(t *testing.T) {
		storniert := EmployeeQualification{ID: uuid.New(), TypeID: missing, ErworbenAm: qDay(t, "2026-06-01"), Status: QualificationRecordStorniert}
		m := byType(EvaluateQualifications([]EmployeeQualification{storniert}, []uuid.UUID{missing}, today, 60))
		if s := m[missing]; s.Status != QualificationStandFehlt {
			t.Fatalf("erwartet fehlt, bekommen %+v", s)
		}
	})

	t.Run("ohne Anforderungen fehlt nichts", func(t *testing.T) {
		for _, s := range EvaluateQualifications(records, nil, today, 60) {
			if s.Status == QualificationStandFehlt || s.Erforderlich {
				t.Fatalf("ohne Anforderungen darf nichts fehlen oder erforderlich sein: %+v", s)
			}
		}
	})

	t.Run("nichts vorhanden, nichts erforderlich: leere Liste, nicht nil", func(t *testing.T) {
		got := EvaluateQualifications(nil, nil, today, 60)
		if got == nil || len(got) != 0 {
			t.Fatalf("erwartet leere Liste, bekommen %#v", got)
		}
	})

	t.Run("Ergebnis ist stabil sortiert", func(t *testing.T) {
		a := EvaluateQualifications(records, []uuid.UUID{missing, have}, today, 60)
		b := EvaluateQualifications([]EmployeeQualification{records[2], records[0], records[1]}, []uuid.UUID{have, missing}, today, 60)
		if len(a) != len(b) {
			t.Fatalf("unterschiedliche Laenge: %d und %d", len(a), len(b))
		}
		for i := range a {
			if a[i].TypeID != b[i].TypeID {
				t.Fatalf("Reihenfolge haengt von der Eingabe ab (Position %d)", i)
			}
			if i > 0 && a[i-1].TypeID.String() >= a[i].TypeID.String() {
				t.Fatalf("nicht nach TypeID sortiert (Position %d)", i)
			}
		}
	})
}
