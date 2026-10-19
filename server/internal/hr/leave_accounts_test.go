package hr

import "testing"

// Urlaubsvorgabe und Pruefungen (Backlog F.8.3.1). Reine Funktionen.

func TestDefaultLeaveDays(t *testing.T) {
	for n, want := range map[int]float64{5: 20, 3: 12, 6: 24, 1: 4} {
		if got := DefaultLeaveDays(n); got != want {
			t.Errorf("%d Arbeitstage: %v, erwartet %v", n, got, want)
		}
	}
}

func TestValidateEmployeeLeaveDays(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	fiveDays, threeDays := []int{1, 2, 3, 4, 5}, []int{1, 2, 3}
	ok := []struct {
		v    *float64
		days []int
	}{{nil, fiveDays}, {f(20), fiveDays}, {f(20.5), fiveDays}, {f(30), fiveDays}, {f(12), threeDays}, {f(366), fiveDays}}
	for _, c := range ok {
		if err := validateEmployeeLeaveDays(c.v, c.days); err != nil {
			t.Errorf("%v bei %v: %v", c.v, c.days, err)
		}
	}
	if err := validateEmployeeLeaveDays(f(19.5), fiveDays); err == nil ||
		err.Error() != "Ungültiger Urlaubsanspruch: mindestens 20 Tage bei 5 Arbeitstagen je Woche (gesetzlicher Mindesturlaub)" {
		t.Errorf("19,5 bei 5 Tagen: %v", err)
	}
	if err := validateEmployeeLeaveDays(f(11.5), threeDays); err == nil ||
		err.Error() != "Ungültiger Urlaubsanspruch: mindestens 12 Tage bei 3 Arbeitstagen je Woche (gesetzlicher Mindesturlaub)" {
		t.Errorf("11,5 bei 3 Tagen: %v", err)
	}
	for _, bad := range []float64{20.25, 367, -1} {
		if err := validateEmployeeLeaveDays(f(bad), fiveDays); err == nil {
			t.Errorf("%v angenommen", bad)
		}
	}
}

func TestValidateHalfDays(t *testing.T) {
	for _, v := range []float64{0, 0.5, 1, 13.5, 366} {
		if err := validateHalfDays(v, "x"); err != nil {
			t.Errorf("%v: %v", v, err)
		}
	}
	for _, v := range []float64{-0.5, 0.3, 366.5} {
		if err := validateHalfDays(v, "x"); err == nil || err.Error() != "Ungültiger Wert für x: ganze oder halbe Tage von 0 bis 366" {
			t.Errorf("%v: %v", v, err)
		}
	}
}
