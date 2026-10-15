package assets

import (
	"reflect"
	"testing"
	"time"
)

// Stand am Stichtag und Hinweise ohne Datenbank (Backlog F.3.5.2).

func TestStandingAtIgnoresLaterInspections(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	ok := insp(d(2026, 3, 1), ErgebnisBestanden, datePtr(2027, 3, 1), t0)
	failed := insp(d(2026, 5, 10), ErgebnisNichtBestanden, nil, t0)
	records := []Inspection{ok, failed}

	// Vor der durchgefallenen Pruefung: geprueft.
	if st := standingAt(records, intPtr(12), d(2026, 5, 9), 30); st.Status != PruefstandGeprueft || *st.InspectionID != ok.ID {
		t.Fatalf("am Vortag: %+v", st)
	}
	// Am Tag der durchgefallenen Pruefung zaehlt sie schon.
	if st := standingAt(records, intPtr(12), d(2026, 5, 10), 30); st.Status != PruefstandGesperrt {
		t.Fatalf("am Pruefungstag: %+v", st)
	}
	// Uhrzeit des Stichtags spielt keine Rolle.
	if st := standingAt(records, intPtr(12), time.Date(2026, 5, 10, 0, 30, 0, 0, time.FixedZone("x", 7200)), 30); st.Status != PruefstandGesperrt {
		t.Fatalf("am Pruefungstag mit Uhrzeit: %+v", st)
	}
	// Vor jeder Pruefung: nie geprueft.
	if st := standingAt(records, intPtr(12), d(2026, 2, 1), 30); st.Status != PruefstandNieGeprueft {
		t.Fatalf("vor jeder Pruefung: %+v", st)
	}
}

func TestIssueHints(t *testing.T) {
	cases := []struct {
		name string
		st   InspectionStanding
		want []string
	}{
		{"geprueft", InspectionStanding{Status: PruefstandGeprueft}, []string{}},
		{"bald faellig", InspectionStanding{Status: PruefstandFaelligBald, NaechstePruefungAm: datePtr(2026, 5, 1)}, []string{}},
		{"nicht pruefpflichtig", InspectionStanding{Status: PruefstandNichtPruefpflichtig}, []string{}},
		{"ueberfaellig", InspectionStanding{Status: PruefstandUeberfaellig, NaechstePruefungAm: datePtr(2026, 3, 7)}, []string{"Prüfung überfällig seit 07.03.2026"}},
		{"nie geprueft", InspectionStanding{Status: PruefstandNieGeprueft}, []string{"Asset wurde noch nie geprüft"}},
		{"Maengel", InspectionStanding{Status: PruefstandGeprueft, Maengel: true}, []string{"Letzte Prüfung mit Mängeln"}},
		{"ueberfaellig mit Maengeln", InspectionStanding{Status: PruefstandUeberfaellig, NaechstePruefungAm: datePtr(2026, 3, 7), Maengel: true}, []string{"Prüfung überfällig seit 07.03.2026", "Letzte Prüfung mit Mängeln"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := issueHints(tc.st); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("erwartet %q, bekommen %q", tc.want, got)
			}
		})
	}
}

func TestInspectionStandRank(t *testing.T) {
	order := []string{PruefstandGesperrt, PruefstandUeberfaellig, PruefstandNieGeprueft, PruefstandFaelligBald}
	for i := 1; i < len(order); i++ {
		if inspectionStandRank(order[i-1]) >= inspectionStandRank(order[i]) {
			t.Fatalf("%s muss vor %s stehen", order[i-1], order[i])
		}
	}
	for _, s := range order {
		if !inspectionNeedsAttention(s) {
			t.Fatalf("%s gehoert in die Liste", s)
		}
	}
	for _, s := range []string{PruefstandGeprueft, PruefstandNichtPruefpflichtig} {
		if inspectionNeedsAttention(s) {
			t.Fatalf("%s gehoert nicht in die Liste", s)
		}
	}
}
