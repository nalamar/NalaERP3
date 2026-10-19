package hr

import (
	"errors"
	"testing"
	"time"
)

// Feiertage und Arbeitstage (Backlog F.8.2.1). Reine Funktionen, keine DB.

func ymd(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("Datum %q nicht lesbar: %v", s, err)
	}
	return d
}

func TestEasterSundayKnownDates(t *testing.T) {
	// Bekannte Ostersonntage inkl. fruehestem (22.03.) und spaetestem
	// (25.04.) moeglichen Datum.
	cases := map[int]string{
		1818: "1818-03-22",
		1943: "1943-04-25",
		2008: "2008-03-23",
		2019: "2019-04-21",
		2024: "2024-03-31",
		2025: "2025-04-20",
		2026: "2026-04-05",
		2027: "2027-03-28",
		2038: "2038-04-25",
		2285: "2285-03-22",
	}
	for year, want := range cases {
		if got := EasterSunday(year); !got.Equal(ymd(t, want)) {
			t.Errorf("Ostern %d: %s, erwartet %s", year, got.Format("2006-01-02"), want)
		}
	}
}

func TestPublicHolidaysCountPerLand2026(t *testing.T) {
	// Anzahl je Land 2026 (ohne Schalter). Abgleich mit absentify
	// (Bayern dort 13 inkl. Mariae Himmelfahrt; Brandenburg dort 10 ohne
	// die zwei Sonntage Ostern/Pfingsten).
	want := map[string]int{
		"BW": 12, "BY": 12, "BE": 10, "BB": 12, "HB": 10, "HH": 10, "HE": 10, "MV": 11,
		"NI": 10, "NW": 11, "RP": 11, "SL": 12, "SN": 11, "ST": 11, "SH": 10, "TH": 11,
	}
	if len(want) != len(Bundeslaender) {
		t.Fatalf("Test deckt %d Laender ab, bekannt sind %d", len(want), len(Bundeslaender))
	}
	for land, n := range want {
		hs, err := PublicHolidays(2026, land, HolidayOptions{})
		if err != nil {
			t.Fatalf("%s: %v", land, err)
		}
		if len(hs) != n {
			t.Errorf("%s 2026: %d Feiertage, erwartet %d: %v", land, len(hs), n, hs)
		}
		seen := map[time.Time]bool{}
		for i, h := range hs {
			if seen[h.Date] {
				t.Errorf("%s: Datum doppelt %s", land, h.Date)
			}
			seen[h.Date] = true
			if i > 0 && hs[i-1].Date.After(h.Date) {
				t.Errorf("%s: nicht sortiert bei %s", land, h.Name)
			}
			if h.Date.Year() != 2026 {
				t.Errorf("%s: %s liegt nicht in 2026", land, h.Name)
			}
		}
	}
}

func holidayNames(t *testing.T, year int, land string, opt HolidayOptions) map[string]string {
	t.Helper()
	hs, err := PublicHolidays(year, land, opt)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, h := range hs {
		out[h.Date.Format("2006-01-02")] = h.Name
	}
	return out
}

func TestPublicHolidaysDates2026(t *testing.T) {
	nw := holidayNames(t, 2026, "NW", HolidayOptions{})
	for date, name := range map[string]string{
		"2026-01-01": "Neujahr",
		"2026-04-03": "Karfreitag",
		"2026-04-06": "Ostermontag",
		"2026-05-01": "Tag der Arbeit",
		"2026-05-14": "Christi Himmelfahrt",
		"2026-05-25": "Pfingstmontag",
		"2026-06-04": "Fronleichnam",
		"2026-10-03": "Tag der Deutschen Einheit",
		"2026-11-01": "Allerheiligen",
		"2026-12-25": "1. Weihnachtsfeiertag",
		"2026-12-26": "2. Weihnachtsfeiertag",
	} {
		if nw[date] != name {
			t.Errorf("NW %s: %q, erwartet %q", date, nw[date], name)
		}
	}
	checks := []struct {
		land, date, name string
		present          bool
	}{
		{"BW", "2026-01-06", "Heilige Drei Könige", true},
		{"ST", "2026-01-06", "Heilige Drei Könige", true},
		{"NW", "2026-01-06", "", false},
		{"BE", "2026-03-08", "Internationaler Frauentag", true},
		{"MV", "2026-03-08", "Internationaler Frauentag", true},
		{"HH", "2026-03-08", "", false},
		{"BB", "2026-04-05", "Ostersonntag", true},
		{"BB", "2026-05-24", "Pfingstsonntag", true},
		{"BE", "2026-04-05", "", false},
		{"HE", "2026-06-04", "Fronleichnam", true},
		{"SN", "2026-06-04", "", false},
		{"SL", "2026-08-15", "Mariä Himmelfahrt", true},
		{"BY", "2026-08-15", "", false},
		{"TH", "2026-09-20", "Weltkindertag", true},
		{"SN", "2026-10-31", "Reformationstag", true},
		{"NI", "2026-10-31", "Reformationstag", true},
		{"BW", "2026-10-31", "", false},
		{"RP", "2026-11-01", "Allerheiligen", true},
		{"HE", "2026-11-01", "", false},
		{"SN", "2026-11-18", "Buß- und Bettag", true},
		{"ST", "2026-11-18", "", false},
	}
	for _, c := range checks {
		got := holidayNames(t, 2026, c.land, HolidayOptions{})[c.date]
		if c.present && got != c.name {
			t.Errorf("%s %s: %q, erwartet %q", c.land, c.date, got, c.name)
		}
		if !c.present && got != "" {
			t.Errorf("%s %s: unerwartet Feiertag %q", c.land, c.date, got)
		}
	}
}

func TestBussUndBettag(t *testing.T) {
	// Mittwoch vor dem 23.11.: auch wenn der 22.11. selbst ein Mittwoch ist.
	for year, want := range map[int]string{2022: "2022-11-16", 2023: "2023-11-22", 2024: "2024-11-20", 2025: "2025-11-19", 2026: "2026-11-18", 2028: "2028-11-22"} {
		if got := bussUndBettag(year); !got.Equal(ymd(t, want)) {
			t.Errorf("Buß- und Bettag %d: %s, erwartet %s", year, got.Format("2006-01-02"), want)
		}
	}
}

func TestPublicHolidaysBavarianOptionsAndBerlin2025(t *testing.T) {
	by := holidayNames(t, 2026, "BY", HolidayOptions{MariaeHimmelfahrt: true, AugsburgerFriedensfest: true})
	if by["2026-08-15"] != "Mariä Himmelfahrt" || by["2026-08-08"] != "Augsburger Friedensfest" || len(by) != 14 {
		t.Errorf("BY mit Schaltern: %v", by)
	}
	// Schalter wirken nur in Bayern.
	he := holidayNames(t, 2026, "HE", HolidayOptions{MariaeHimmelfahrt: true, AugsburgerFriedensfest: true})
	if he["2026-08-15"] != "" || he["2026-08-08"] != "" {
		t.Errorf("Schalter wirken ausserhalb Bayerns: %v", he)
	}
	if got := holidayNames(t, 2025, "BE", HolidayOptions{})["2025-05-08"]; got != "Tag der Befreiung" {
		t.Errorf("Berlin 08.05.2025: %q", got)
	}
	if got := holidayNames(t, 2026, "BE", HolidayOptions{})["2026-05-08"]; got != "" {
		t.Errorf("Berlin 08.05.2026 ist kein Feiertag: %q", got)
	}
}

func TestPublicHolidaysUnknownLand(t *testing.T) {
	for _, land := range []string{"", "XX", "nw", "DE-NW"} {
		if _, err := PublicHolidays(2026, land, HolidayOptions{}); !errors.Is(err, ErrUnknownBundesland) {
			t.Errorf("%q: Fehler %v, erwartet ErrUnknownBundesland", land, err)
		}
	}
}

func TestCountWorkDays(t *testing.T) {
	hs, err := PublicHolidays(2026, "NW", HolidayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	isHoliday := HolidaySet(hs)
	cases := []struct {
		name     string
		from, to string
		week     [7]bool
		holidays func(time.Time) bool
		want     int
	}{
		{"Ostern Fr-Mo nur Feiertage und Wochenende", "2026-04-03", "2026-04-06", StandardWorkWeek, isHoliday, 0},
		{"Woche mit Himmelfahrt", "2026-05-11", "2026-05-15", StandardWorkWeek, isHoliday, 4},
		{"dieselbe Woche ohne Feiertage", "2026-05-11", "2026-05-15", StandardWorkWeek, nil, 5},
		{"ein Tag", "2026-05-12", "2026-05-12", StandardWorkWeek, isHoliday, 1},
		{"Samstag allein", "2026-05-16", "2026-05-16", StandardWorkWeek, isHoliday, 0},
		{"Ende vor Beginn", "2026-05-15", "2026-05-11", StandardWorkWeek, isHoliday, 0},
		{"Teilzeit Mo-Mi, zwei Wochen", "2026-05-04", "2026-05-17", [7]bool{false, true, true, true, false, false, false}, isHoliday, 6},
		{"ganzes Jahr NW", "2026-01-01", "2026-12-31", StandardWorkWeek, isHoliday, 253},
		{"Jahreswechsel", "2026-12-24", "2027-01-04", StandardWorkWeek, HolidaySet(append(hs, mustHolidays(t, 2027, "NW")...)), 6},
	}
	for _, c := range cases {
		if got := CountWorkDays(ymd(t, c.from), ymd(t, c.to), c.week, c.holidays); got != c.want {
			t.Errorf("%s: %d, erwartet %d", c.name, got, c.want)
		}
	}
	// HolidaySet direkt mit Uhrzeit: der Feiertag gilt den ganzen Tag.
	if !isHoliday(time.Date(2026, 5, 14, 15, 30, 0, 0, time.UTC)) {
		t.Error("Himmelfahrt 15:30 nicht als Feiertag erkannt")
	}
	// Uhrzeit und Zeitzone der Eingabe spielen keine Rolle.
	from := time.Date(2026, 5, 11, 23, 30, 0, 0, time.FixedZone("x", 2*3600))
	to := time.Date(2026, 5, 15, 0, 1, 0, 0, time.UTC)
	if got := CountWorkDays(from, to, StandardWorkWeek, isHoliday); got != 4 {
		t.Errorf("mit Uhrzeiten: %d, erwartet 4", got)
	}
}

func mustHolidays(t *testing.T, year int, land string) []Holiday {
	t.Helper()
	hs, err := PublicHolidays(year, land, HolidayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return hs
}
