package hr

import (
	"errors"
	"sort"
	"time"
)

// Gesetzliche Feiertage je Bundesland (ADR 0029, Backlog F.8.2.1).
//
// Feiertage werden berechnet statt gepflegt: neun gelten in allen Laendern,
// dazu je Land weitere. Stand der Regeln: geltendes Recht ab 2023 (seitdem
// ist der Frauentag auch in Mecklenburg-Vorpommern Feiertag). Fuer fruehere
// Jahre rechnet die Funktion mit denselben Regeln; einmalige Feiertage
// frueherer Jahre (z. B. Reformationstag 2017 bundesweit) fehlen dann.
//
// Nicht abgebildet, weil sie nur in einzelnen Gemeinden gelten:
// Fronleichnam in Teilen Sachsens und Thueringens. Mariae Himmelfahrt in
// Bayern (nur ueberwiegend katholische Gemeinden) und das Augsburger
// Friedensfest sind Schalter (HolidayOptions), die der Mandant setzt.

// Bundeslaender: Kuerzel nach ISO 3166-2:DE ohne "DE-".
var Bundeslaender = map[string]string{
	"BW": "Baden-Württemberg",
	"BY": "Bayern",
	"BE": "Berlin",
	"BB": "Brandenburg",
	"HB": "Bremen",
	"HH": "Hamburg",
	"HE": "Hessen",
	"MV": "Mecklenburg-Vorpommern",
	"NI": "Niedersachsen",
	"NW": "Nordrhein-Westfalen",
	"RP": "Rheinland-Pfalz",
	"SL": "Saarland",
	"SN": "Sachsen",
	"ST": "Sachsen-Anhalt",
	"SH": "Schleswig-Holstein",
	"TH": "Thüringen",
}

// ErrUnknownBundesland meldet ein unbekanntes Laenderkuerzel.
// Die Meldung enthaelt "Ungültig", damit sie ueber HTTP als 400 ankommt.
var ErrUnknownBundesland = errors.New("Ungültiges Bundesland")

// Holiday ist ein Feiertag; Date ist ein reines Datum (00:00 UTC wie
// dateOnly).
type Holiday struct {
	Date time.Time `json:"date"`
	Name string    `json:"name"`
}

// HolidayOptions sind die Sonderfaelle, die nicht fuer ein ganzes Land
// gelten. Beide wirken nur in Bayern.
type HolidayOptions struct {
	// MariaeHimmelfahrt: 15.08., gilt in Bayern nur in ueberwiegend
	// katholischen Gemeinden (im Saarland immer).
	MariaeHimmelfahrt bool
	// AugsburgerFriedensfest: 08.08., gilt nur in der Stadt Augsburg.
	AugsburgerFriedensfest bool
}

func calendarDay(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

// EasterSunday berechnet den Ostersonntag des gregorianischen Kalenders
// (anonymer gregorianischer Algorithmus nach Meeus/Jones/Butcher).
func EasterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	dd := (h+l-7*m+114)%31 + 1
	return calendarDay(year, time.Month(month), dd)
}

// bussUndBettag ist der Mittwoch vor dem 23. November.
func bussUndBettag(year int) time.Time {
	d := calendarDay(year, time.November, 22)
	for d.Weekday() != time.Wednesday {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

func inLand(land string, lands ...string) bool {
	for _, l := range lands {
		if l == land {
			return true
		}
	}
	return false
}

// NationwideHolidays liefert die neun Feiertage, die in allen Laendern
// gelten - nach Datum sortiert. Sie gelten auch, solange fuer einen
// Mandanten noch kein Bundesland eingestellt ist.
func NationwideHolidays(year int) []Holiday {
	easter := EasterSunday(year)
	rel := func(days int) time.Time { return easter.AddDate(0, 0, days) }
	return []Holiday{
		{calendarDay(year, time.January, 1), "Neujahr"},
		{rel(-2), "Karfreitag"},
		{rel(1), "Ostermontag"},
		{calendarDay(year, time.May, 1), "Tag der Arbeit"},
		{rel(39), "Christi Himmelfahrt"},
		{rel(50), "Pfingstmontag"},
		{calendarDay(year, time.October, 3), "Tag der Deutschen Einheit"},
		{calendarDay(year, time.December, 25), "1. Weihnachtsfeiertag"},
		{calendarDay(year, time.December, 26), "2. Weihnachtsfeiertag"},
	}
}

// PublicHolidays liefert die gesetzlichen Feiertage eines Jahres in einem
// Bundesland, nach Datum sortiert. Ostersonntag und Pfingstsonntag sind
// nur in Brandenburg ausdruecklich gesetzliche Feiertage und erscheinen
// nur dort.
func PublicHolidays(year int, land string, opt HolidayOptions) ([]Holiday, error) {
	if _, ok := Bundeslaender[land]; !ok {
		return nil, ErrUnknownBundesland
	}
	easter := EasterSunday(year)
	rel := func(days int) time.Time { return easter.AddDate(0, 0, days) }
	out := NationwideHolidays(year)
	add := func(d time.Time, name string) { out = append(out, Holiday{d, name}) }
	if inLand(land, "BW", "BY", "ST") {
		add(calendarDay(year, time.January, 6), "Heilige Drei Könige")
	}
	if inLand(land, "BE", "MV") {
		add(calendarDay(year, time.March, 8), "Internationaler Frauentag")
	}
	if land == "BB" {
		add(easter, "Ostersonntag")
		add(rel(49), "Pfingstsonntag")
	}
	if inLand(land, "BW", "BY", "HE", "NW", "RP", "SL") {
		add(rel(60), "Fronleichnam")
	}
	if land == "BY" && opt.AugsburgerFriedensfest {
		add(calendarDay(year, time.August, 8), "Augsburger Friedensfest")
	}
	if land == "SL" || (land == "BY" && opt.MariaeHimmelfahrt) {
		add(calendarDay(year, time.August, 15), "Mariä Himmelfahrt")
	}
	if land == "TH" {
		add(calendarDay(year, time.September, 20), "Weltkindertag")
	}
	if inLand(land, "BB", "HB", "HH", "MV", "NI", "SN", "ST", "SH", "TH") {
		add(calendarDay(year, time.October, 31), "Reformationstag")
	}
	if inLand(land, "BW", "BY", "NW", "RP", "SL") {
		add(calendarDay(year, time.November, 1), "Allerheiligen")
	}
	if land == "SN" {
		add(bussUndBettag(year), "Buß- und Bettag")
	}
	// Einmaliger Feiertag: 80. Jahrestag des Kriegsendes in Berlin.
	if land == "BE" && year == 2025 {
		add(calendarDay(2025, time.May, 8), "Tag der Befreiung")
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

// StandardWorkWeek ist die 5-Tage-Woche Montag bis Freitag, indiziert mit
// time.Weekday (0 = Sonntag).
var StandardWorkWeek = [7]bool{false, true, true, true, true, true, false}

// CountWorkDays zaehlt die Arbeitstage im Zeitraum from..to (beide
// einschliesslich): Tage, die laut workWeek gearbeitet werden und kein
// Feiertag sind. isHoliday darf nil sein. Uhrzeiten werden ignoriert; liegt
// to vor from, ist das Ergebnis 0.
func CountWorkDays(from, to time.Time, workWeek [7]bool, isHoliday func(time.Time) bool) int {
	from, to = dateOnly(from), dateOnly(to)
	n := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if !workWeek[d.Weekday()] {
			continue
		}
		if isHoliday != nil && isHoliday(d) {
			continue
		}
		n++
	}
	return n
}

// HolidaySet macht aus Feiertagen eine Nachschlagefunktion fuer
// CountWorkDays.
func HolidaySet(holidays []Holiday) func(time.Time) bool {
	set := make(map[time.Time]bool, len(holidays))
	for _, h := range holidays {
		set[dateOnly(h.Date)] = true
	}
	return func(d time.Time) bool { return set[dateOnly(d)] }
}
