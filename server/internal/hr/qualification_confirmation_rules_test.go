package hr

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Zwischenbestaetigung (Backlog F.10.3, DIN EN ISO 9606-1): reine Regeln.

func TestQualificationConfirmationDue(t *testing.T) {
	cases := []struct {
		name          string
		erworben      string
		confirmations []string
		want          string
	}{
		{"ohne Bestaetigung: sechs Monate nach Erwerb", "2026-01-15", nil, "2026-07-15"},
		{"rechtzeitige Bestaetigung verschiebt", "2026-01-15", []string{"2026-07-10"}, "2027-01-10"},
		{"Bestaetigung genau am letzten Tag zaehlt", "2026-01-15", []string{"2026-07-15"}, "2027-01-15"},
		{"Kette aus zwei Bestaetigungen", "2026-01-15", []string{"2026-07-10", "2027-01-05"}, "2027-07-05"},
		{"Reihenfolge egal", "2026-01-15", []string{"2027-01-05", "2026-07-10"}, "2027-07-05"},
		// Ohne rechtzeitige Bestaetigung verliert das Zertifikat seine
		// Gueltigkeit: eine spaete Bestaetigung rettet es nicht, und die
		// danach zaehlen auch nicht.
		{"verspaetete Bestaetigung zaehlt nicht", "2026-01-15", []string{"2026-07-16"}, "2026-07-15"},
		{"nach verspaeteter Bestaetigung endet die Kette", "2026-01-15", []string{"2026-07-10", "2027-01-11", "2027-03-01"}, "2027-01-10"},
		{"Bestaetigung vor dem Erwerb zaehlt nicht", "2026-01-15", []string{"2026-01-10"}, "2026-07-15"},
		{"Monatsende", "2026-08-31", nil, "2027-02-28"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := make([]time.Time, 0, len(tc.confirmations))
			for _, c := range tc.confirmations {
				conf = append(conf, qDay(t, c))
			}
			if got := QualificationConfirmationDue(qDay(t, tc.erworben), conf, 6); !got.Equal(qDay(t, tc.want)) {
				t.Fatalf("erwartet %s, bekommen %s", tc.want, got.Format("2006-01-02"))
			}
		})
	}
}

func TestEvaluateQualificationsWithConfirmations(t *testing.T) {
	weld, other := uuid.New(), uuid.New()
	rec := EmployeeQualification{ID: uuid.New(), TypeID: weld, ErworbenAm: qDay(t, "2026-01-15"), GueltigBis: qDayPtr(t, "2029-01-15"), Status: QualificationRecordActive}
	conf := func(dates ...string) QualificationConfirmations {
		c := QualificationConfirmations{Dates: map[uuid.UUID][]time.Time{}, Months: map[uuid.UUID]int{weld: 6}}
		for _, d := range dates {
			c.Dates[rec.ID] = append(c.Dates[rec.ID], qDay(t, d))
		}
		return c
	}
	one := func(t *testing.T, got []QualificationStanding) QualificationStanding {
		t.Helper()
		if len(got) != 1 {
			t.Fatalf("erwartet einen Eintrag, bekommen %+v", got)
		}
		return got[0]
	}

	t.Run("ueberfaellige Bestaetigung wirkt wie Ablauf", func(t *testing.T) {
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{rec}, conf("2026-07-10"), nil, qDay(t, "2027-02-01"), 60))
		if st.Status != QualificationStandAbgelaufen || st.AblaufGrund != QualificationAblaufBestaetigung ||
			st.BestaetigungFaelligAm == nil || !st.BestaetigungFaelligAm.Equal(qDay(t, "2027-01-10")) || *st.TageBisAblauf != -22 {
			t.Fatalf("%+v", st)
		}
		// Das Gueltig-bis des Nachweises bleibt, wie es ist.
		if !st.GueltigBis.Equal(qDay(t, "2029-01-15")) {
			t.Fatalf("GueltigBis veraendert: %v", st.GueltigBis)
		}
	})
	t.Run("Bestaetigung bald faellig", func(t *testing.T) {
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{rec}, conf("2026-07-10"), nil, qDay(t, "2026-12-01"), 60))
		if st.Status != QualificationStandLaeuftAb || *st.TageBisAblauf != 40 || st.AblaufGrund != QualificationAblaufBestaetigung {
			t.Fatalf("%+v", st)
		}
	})
	t.Run("bestaetigt und gueltig", func(t *testing.T) {
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{rec}, conf("2026-07-10"), nil, qDay(t, "2026-09-01"), 60))
		if st.Status != QualificationStandGueltig || *st.TageBisAblauf != 131 || st.AblaufGrund != QualificationAblaufBestaetigung {
			t.Fatalf("%+v", st)
		}
	})
	t.Run("ohne Bestaetigung: sechs Monate nach Erwerb", func(t *testing.T) {
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{rec}, conf(), nil, qDay(t, "2026-07-16"), 60))
		if st.Status != QualificationStandAbgelaufen || !st.BestaetigungFaelligAm.Equal(qDay(t, "2026-07-15")) {
			t.Fatalf("%+v", st)
		}
	})
	t.Run("faellige Bestaetigung nach dem Ablauf zaehlt nicht", func(t *testing.T) {
		// Bestaetigt am 2028-12-01 -> faellig 2029-06-01, Ablauf 2029-01-15.
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{rec},
			conf("2026-07-10", "2027-01-05", "2027-07-01", "2028-01-01", "2028-06-15", "2028-12-01"), nil, qDay(t, "2029-01-10"), 60))
		if st.BestaetigungFaelligAm != nil || st.AblaufGrund != "" || st.Status != QualificationStandLaeuftAb || *st.TageBisAblauf != 5 {
			t.Fatalf("%+v", st)
		}
	})
	t.Run("Bestaetigung genau am Ablauftag: Ablauf durch Gueltig-bis", func(t *testing.T) {
		short := rec
		short.GueltigBis = qDayPtr(t, "2026-07-15")
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{short}, conf(), nil, qDay(t, "2026-07-01"), 60))
		if st.BestaetigungFaelligAm == nil || st.AblaufGrund != "" || *st.TageBisAblauf != 14 {
			t.Fatalf("%+v", st)
		}
	})
	t.Run("Art ohne Intervall bleibt unberuehrt", func(t *testing.T) {
		r := rec
		r.TypeID = other
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{r}, conf(), nil, qDay(t, "2027-02-01"), 60))
		if st.Status != QualificationStandGueltig || st.BestaetigungFaelligAm != nil || st.AblaufGrund != "" {
			t.Fatalf("%+v", st)
		}
		// Ohne Angaben (EvaluateQualifications) ebenso fuer die Schweisserart.
		st = one(t, EvaluateQualifications([]EmployeeQualification{rec}, nil, qDay(t, "2027-02-01"), 60))
		if st.Status != QualificationStandGueltig || st.BestaetigungFaelligAm != nil {
			t.Fatalf("%+v", st)
		}
	})
	t.Run("massgeblich ist der Nachweis mit der laengeren wirksamen Gueltigkeit", func(t *testing.T) {
		// Der alte Nachweis gilt laut Zertifikat laenger, ist aber nicht
		// bestaetigt; der neue ist frisch erworben.
		newer := EmployeeQualification{ID: uuid.New(), TypeID: weld, ErworbenAm: qDay(t, "2027-01-20"), GueltigBis: qDayPtr(t, "2028-06-30"), Status: QualificationRecordActive}
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{rec, newer}, conf(), nil, qDay(t, "2027-02-01"), 60))
		if *st.QualificationID != newer.ID || st.Status != QualificationStandGueltig || !st.BestaetigungFaelligAm.Equal(qDay(t, "2027-07-20")) {
			t.Fatalf("%+v", st)
		}
		st = one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{newer, rec}, conf(), nil, qDay(t, "2027-02-01"), 60))
		if *st.QualificationID != newer.ID {
			t.Fatalf("Reihenfolge der Eingabe entscheidet: %+v", st)
		}
	})
	t.Run("stornierter Nachweis zaehlt nicht", func(t *testing.T) {
		s := rec
		s.Status = QualificationRecordStorniert
		st := one(t, EvaluateQualificationsWithConfirmations([]EmployeeQualification{s}, conf("2026-07-10"), []uuid.UUID{weld}, qDay(t, "2026-09-01"), 60))
		if st.Status != QualificationStandFehlt {
			t.Fatalf("%+v", st)
		}
	})
}
