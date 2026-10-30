package fleet

import "testing"

// Faelligkeit nach km ohne Datenbank (Backlog G.3e.1).

func kmp(i int) *int { return &i }

func TestApplyKmDue(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		faelligKm  *int
		kmStand    *int
		warnKm     int
		wantStatus string
		wantRest   *int
	}{
		{"weit vor der Folge-km", TerminstandOK, kmp(60000), kmp(50000), 1000, TerminstandOK, kmp(10000)},
		{"genau Vorwarnung", TerminstandOK, kmp(60000), kmp(59000), 1000, TerminstandFaelligBald, kmp(1000)},
		{"knapp vor Vorwarnung", TerminstandOK, kmp(60000), kmp(58999), 1000, TerminstandOK, kmp(1001)},
		{"genau Folge-km", TerminstandOK, kmp(60000), kmp(60000), 1000, TerminstandFaelligBald, kmp(0)},
		{"ueberschritten", TerminstandOK, kmp(60000), kmp(60001), 1000, TerminstandUeberfaellig, kmp(-1)},
		{"nur km, ohne Datum", TerminstandOhneTermin, kmp(60000), kmp(10000), 1000, TerminstandOK, kmp(50000)},
		{"nur km, ohne Datum, faellig", TerminstandOhneTermin, kmp(60000), kmp(59500), 1000, TerminstandFaelligBald, kmp(500)},
		{"Datum dringender als km", TerminstandUeberfaellig, kmp(60000), kmp(10000), 1000, TerminstandUeberfaellig, kmp(50000)},
		{"Datum bald, km ueberschritten", TerminstandFaelligBald, kmp(60000), kmp(61000), 1000, TerminstandUeberfaellig, kmp(-1000)},
		{"Sperre bleibt", TerminstandGesperrt, kmp(60000), kmp(61000), 1000, TerminstandGesperrt, kmp(-1000)},
		{"negative Vorwarnung wie 0", TerminstandOK, kmp(60000), kmp(59999), -5, TerminstandOK, kmp(1)},
		{"negative Vorwarnung, genau Folge-km", TerminstandOK, kmp(60000), kmp(60000), -5, TerminstandFaelligBald, kmp(0)},
		{"Vorwarnung 0, genau Folge-km", TerminstandOK, kmp(60000), kmp(60000), 0, TerminstandFaelligBald, kmp(0)},
		{"ohne Folge-km", TerminstandOK, nil, kmp(60000), 1000, TerminstandOK, nil},
		{"ohne km-Stand", TerminstandOK, kmp(60000), nil, 1000, TerminstandOK, nil},
	}
	for _, c := range cases {
		st := ApplyKmDue(ScheduleStanding{Status: c.status, FaelligKm: c.faelligKm}, c.kmStand, c.warnKm)
		if st.Status != c.wantStatus {
			t.Errorf("%s: Status %q, erwartet %q", c.name, st.Status, c.wantStatus)
		}
		if (st.KmBisFaellig == nil) != (c.wantRest == nil) || (st.KmBisFaellig != nil && *st.KmBisFaellig != *c.wantRest) {
			t.Errorf("%s: KmBisFaellig %v, erwartet %v", c.name, st.KmBisFaellig, c.wantRest)
		}
		if (st.KmStand == nil) != (c.kmStand == nil) || (st.KmStand != nil && *st.KmStand != *c.kmStand) {
			t.Errorf("%s: KmStand %v, erwartet %v", c.name, st.KmStand, c.kmStand)
		}
	}
	// Der km-Stand im Ergebnis ist eine Kopie, kein Verweis auf die Eingabe.
	in := 100
	st := ApplyKmDue(ScheduleStanding{Status: TerminstandOK, FaelligKm: kmp(200)}, &in, 0)
	in = 999
	if *st.KmStand != 100 {
		t.Fatalf("KmStand folgt der Eingabe: %d", *st.KmStand)
	}
}
