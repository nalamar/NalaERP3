package hr

import (
	"testing"
	"time"
)

// Teilurlaub nach §§ 4, 5 BUrlG (Backlog F.8.3.2). Reine Funktionen.

func TestMonthsEnd(t *testing.T) {
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-03-15", 1, "2026-04-14"},
		{"2026-01-01", 1, "2026-01-31"},
		{"2026-01-01", 12, "2026-12-31"},
		{"2026-01-31", 1, "2026-02-28"}, // 31.02. fehlt: letzter Tag des Monats
		{"2026-01-31", 2, "2026-03-30"},
		{"2028-01-31", 1, "2028-02-29"}, // Schaltjahr
		{"2026-07-01", 6, "2026-12-31"},
		{"2026-07-02", 6, "2027-01-01"},
		{"2026-11-15", 3, "2027-02-14"},
	}
	for _, c := range cases {
		if got := monthsEnd(ymd(t, c.from), c.n); !got.Equal(ymd(t, c.want)) {
			t.Errorf("%s + %d Monate: %s, erwartet %s", c.from, c.n, got.Format("2006-01-02"), c.want)
		}
	}
}

func TestRoundPartialLeave(t *testing.T) {
	for in, want := range map[float64]float64{0: 0, 5: 5, 20.0 * 5 / 12: 8.5, 20.0 * 4 / 12: 7, 30.0 * 3 / 12: 8, 20.0 * 2 / 12: 3.5, 0.1: 0.5, 6.5: 7} {
		if got := roundPartialLeave(in); got != want {
			t.Errorf("%v: %v, erwartet %v", in, got, want)
		}
	}
}

func TestPartialLeaveProposal(t *testing.T) {
	d := func(s string) *time.Time {
		v := ymd(t, s)
		return &v
	}
	long := d("2020-04-01")
	cases := []struct {
		name        string
		hire, term  *time.Time
		annual      float64
		wantNil     bool
		tage        float64
		monate      int
		grundPrefix string
	}{
		{"ganzes Jahr", long, nil, 20, true, 0, 0, ""},
		{"ohne Eintrittsdatum", nil, nil, 20, true, 0, 0, ""},
		{"Eintritt 15.03.: Wartezeit erfuellt", d("2026-03-15"), nil, 20, false, 20, 12, "Eintritt bis 1. Juli"},
		{"Eintritt 01.07.: Wartezeit endet 31.12.", d("2026-07-01"), nil, 28, false, 28, 12, "Eintritt bis 1. Juli"},
		{"Eintritt 02.07.: Teilurlaub 5 Monate", d("2026-07-02"), nil, 20, false, 8.5, 5, "Teilurlaub"},
		{"Eintritt 01.10. bei 30 Tagen", d("2026-10-01"), nil, 30, false, 8, 3, "Teilurlaub"},
		{"Austritt 31.03. erste Haelfte", long, d("2026-03-31"), 20, false, 5, 3, "Teilurlaub"},
		{"Austritt 30.06. erste Haelfte", long, d("2026-06-30"), 20, false, 10, 6, "Teilurlaub"},
		{"Austritt 01.07. zweite Haelfte", long, d("2026-07-01"), 20, false, 20, 12, "Austritt in der zweiten"},
		{"Ein- und Austritt im Jahr", d("2026-02-01"), d("2026-05-31"), 20, false, 7, 4, "Teilurlaub"},
		{"Ein-/Austritt, Wartezeit erfuellt, zweite Haelfte", d("2026-01-02"), d("2026-07-31"), 20, false, 20, 12, "Austritt in der zweiten"},
		{"Ein-/Austritt zweite Haelfte, Wartezeit nicht erfuellt", d("2026-03-01"), d("2026-08-15"), 24, false, 10, 5, "Teilurlaub"},
		{"Monatsende 31.01.-29.04.", d("2026-01-31"), d("2026-04-29"), 20, false, 3.5, 2, "Teilurlaub"},
		{"Eintritt 15.12.: kein voller Monat", d("2026-12-15"), nil, 20, false, 0, 0, "Teilurlaub"},
		{"Eintritt nach dem Jahr", d("2027-01-05"), nil, 20, false, 0, 0, "im Jahr nicht"},
		{"Austritt vor dem Jahr", long, d("2025-12-31"), 20, false, 0, 0, "im Jahr nicht"},
	}
	for _, c := range cases {
		got := PartialLeaveProposal(2026, c.hire, c.term, c.annual)
		if c.wantNil {
			if got != nil {
				t.Errorf("%s: %+v, erwartet nil", c.name, got)
			}
			continue
		}
		if got == nil || got.Tage != c.tage || got.Monate != c.monate || len(got.Grund) < len(c.grundPrefix) || got.Grund[:len(c.grundPrefix)] != c.grundPrefix {
			t.Errorf("%s: %+v, erwartet %v Tage, %d Monate, Grund %q…", c.name, got, c.tage, c.monate, c.grundPrefix)
		}
	}
}
