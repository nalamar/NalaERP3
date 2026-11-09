package production

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func dayPtr(s string) *time.Time { d := day(s); return &d }

func TestWeekStart(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-09": "2026-11-09", // Montag
		"2026-11-11": "2026-11-09",
		"2026-11-15": "2026-11-09", // Sonntag gehoert zur Woche davor
		"2026-11-16": "2026-11-16",
		"2027-01-01": "2026-12-28", // ueber den Jahreswechsel
	} {
		if got := WeekStart(day(in)).Format("2006-01-02"); got != want {
			t.Errorf("WeekStart(%s) = %s, erwartet %s", in, got, want)
		}
	}
	// Uhrzeit und Zeitzone fallen weg.
	late := time.Date(2026, 11, 15, 23, 30, 0, 0, time.FixedZone("x", 2*3600))
	if got := WeekStart(late); !got.Equal(day("2026-11-09")) {
		t.Errorf("mit Uhrzeit: %v", got)
	}
}

func TestDefaultWeeks(t *testing.T) {
	// Liefertermin Mittwoch 2026-12-02 (Woche ab 30.11.): letzter
	// Arbeitsgang in der Woche ab 23.11., davor je eine Woche frueher.
	got := DefaultWeeks(dayPtr("2026-12-02"), 3)
	want := []string{"2026-11-09", "2026-11-16", "2026-11-23"}
	if len(got) != 3 {
		t.Fatalf("Anzahl %d", len(got))
	}
	for i, w := range want {
		if got[i] == nil || got[i].Format("2006-01-02") != w {
			t.Errorf("Woche %d = %v, erwartet %s", i, got[i], w)
		}
	}
	for i, w := range DefaultWeeks(nil, 2) {
		if w != nil {
			t.Errorf("ohne Liefertermin Woche %d geplant: %v", i, w)
		}
	}
	if len(DefaultWeeks(dayPtr("2026-12-02"), 0)) != 0 {
		t.Error("ohne Arbeitsgaenge nicht leer")
	}
}

func TestAfterDelivery(t *testing.T) {
	lt := dayPtr("2026-12-02") // Mittwoch
	for _, tc := range []struct {
		woche *time.Time
		want  bool
	}{
		{dayPtr("2026-11-23"), false},
		{dayPtr("2026-11-30"), false}, // Woche des Liefertermins
		{dayPtr("2026-12-07"), true},
		{dayPtr("2026-12-03"), false}, // Tag in der Woche des Liefertermins
		{nil, false},
	} {
		if got := AfterDelivery(tc.woche, lt); got != tc.want {
			t.Errorf("AfterDelivery(%v) = %v, erwartet %v", tc.woche, got, tc.want)
		}
	}
	if AfterDelivery(dayPtr("2027-01-04"), nil) {
		t.Error("ohne Liefertermin Hinweis")
	}
}

func TestPlannedMinutesAndCapacity(t *testing.T) {
	if got := PlannedMinutes(30, 12.5, 8); got != 130 {
		t.Errorf("PlannedMinutes = %v, erwartet 130", got)
	}
	if got := PlannedMinutes(0, 0, 5); got != 0 {
		t.Errorf("ohne Zeiten = %v", got)
	}
	free := map[string]bool{"2026-12-25": true, "2026-12-24": true}
	isFree := func(d time.Time) bool { return free[d.Format("2006-01-02")] }
	if got := WeekWorkDays(day("2026-12-23"), isFree); got != 3 {
		t.Errorf("Weihnachtswoche: %d Arbeitstage, erwartet 3", got)
	}
	if got := WeekWorkDays(day("2026-11-12"), nil); got != 5 {
		t.Errorf("normale Woche: %d", got)
	}
	if got := AvailableMinutes(7.5, 3); got != 1350 {
		t.Errorf("AvailableMinutes = %v", got)
	}
	if Overloaded(1350, 1350) || Overloaded(1350.0000001, 1350) {
		t.Error("genau voll gilt nicht als Ueberlast")
	}
	if !Overloaded(1351, 1350) {
		t.Error("Ueberlast nicht erkannt")
	}
}

func TestValidateWorkCenterInput(t *testing.T) {
	in := NormalizeWorkCenterInput(WorkCenterInput{Name: "  Zuschnitt ", StundenJeTag: 8})
	if in.Name != "Zuschnitt" || ValidateWorkCenterInput(in) != nil {
		t.Fatalf("gueltig abgelehnt: %+v", in)
	}
	if ValidateWorkCenterInput(WorkCenterInput{Name: "x", StundenJeTag: 24}) != nil {
		t.Error("24 Stunden abgelehnt")
	}
	for name, bad := range map[string]WorkCenterInput{
		"ohne Name":     NormalizeWorkCenterInput(WorkCenterInput{Name: "  ", StundenJeTag: 8}),
		"0 Stunden":     {Name: "x", StundenJeTag: 0},
		"negativ":       {Name: "x", StundenJeTag: -1},
		"ueber 24 Std.": {Name: "x", StundenJeTag: 24.5},
	} {
		if ValidateWorkCenterInput(bad) == nil {
			t.Errorf("%s angenommen", name)
		}
	}
}

func TestValidateTemplateAndOperationInput(t *testing.T) {
	wc := uuid.New()
	tpl := NormalizeTemplateInput(TemplateInput{Name: " Verglasung ", WorkCenterID: wc, Reihenfolge: 1, RuestzeitMin: 0, MinutenJeStueck: MaxMinuten})
	if tpl.Name != "Verglasung" || ValidateTemplateInput(tpl) != nil {
		t.Fatalf("gueltige Vorlage abgelehnt: %+v", tpl)
	}
	for name, bad := range map[string]TemplateInput{
		"ohne Name":          {WorkCenterID: wc, Reihenfolge: 1},
		"ohne Arbeitsplatz":  {Name: "x", Reihenfolge: 1},
		"Reihenfolge 0":      {Name: "x", WorkCenterID: wc},
		"Ruestzeit negativ":  {Name: "x", WorkCenterID: wc, Reihenfolge: 1, RuestzeitMin: -1},
		"je Stueck zu gross": {Name: "x", WorkCenterID: wc, Reihenfolge: 1, MinutenJeStueck: MaxMinuten + 1},
	} {
		if ValidateTemplateInput(bad) == nil {
			t.Errorf("Vorlage %s angenommen", name)
		}
	}

	op := NormalizeOperationInput(OperationInput{Name: " Zuschnitt ", WorkCenterID: wc, RuestzeitMin: 15, GeplanteWoche: dayPtr("2026-11-13")})
	if op.Name != "Zuschnitt" || op.GeplanteWoche.Format("2006-01-02") != "2026-11-09" || ValidateOperationInput(op) != nil {
		t.Fatalf("Arbeitsgang falsch normalisiert oder abgelehnt: %+v", op)
	}
	if NormalizeOperationInput(OperationInput{Name: "x"}).GeplanteWoche != nil {
		t.Error("ungeplanter Arbeitsgang bekommt eine Woche")
	}
	for name, bad := range map[string]OperationInput{
		"ohne Name":          {WorkCenterID: wc},
		"ohne Arbeitsplatz":  {Name: "x"},
		"je Stueck negativ":  {Name: "x", WorkCenterID: wc, MinutenJeStueck: -0.5},
		"Ruestzeit zu gross": {Name: "x", WorkCenterID: wc, RuestzeitMin: MaxMinuten + 1},
	} {
		if ValidateOperationInput(bad) == nil {
			t.Errorf("Arbeitsgang %s angenommen", name)
		}
	}
}
