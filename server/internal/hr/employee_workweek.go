package hr

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Bundesland und Arbeitstage am Mitarbeiter (ADR 0029, Backlog F.8.2.3).

// DefaultArbeitstage ist die 5-Tage-Woche Montag bis Freitag (ISO 8601:
// 1 = Montag ... 7 = Sonntag).
var DefaultArbeitstage = []int{1, 2, 3, 4, 5}

// normalizeArbeitstage prueft die Arbeitstage und liefert sie sortiert und
// ohne Doppelte. nil heisst "nicht angegeben" und ergibt die Vorgabe Mo-Fr;
// eine leere Liste ist dagegen ein Fehler - ein Mitarbeiter ohne
// Arbeitstag haette nie einen Urlaubstag.
func normalizeArbeitstage(days []int) ([]int, error) {
	if days == nil {
		return append([]int(nil), DefaultArbeitstage...), nil
	}
	if len(days) == 0 {
		return nil, errors.New("Mindestens ein Arbeitstag erforderlich")
	}
	seen := map[int]bool{}
	out := make([]int, 0, len(days))
	for _, d := range days {
		if d < 1 || d > 7 {
			return nil, fmt.Errorf("Ungültiger Arbeitstag: %d (1 = Montag bis 7 = Sonntag)", d)
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Ints(out)
	return out, nil
}

// normalizeEmployeeBundesland: nil oder leer = es gilt der Mandant.
func normalizeEmployeeBundesland(v *string) (*string, error) {
	if v == nil {
		return nil, nil
	}
	land := strings.ToUpper(strings.TrimSpace(*v))
	if land == "" {
		return nil, nil
	}
	if _, ok := Bundeslaender[land]; !ok {
		return nil, ErrUnknownBundesland
	}
	return &land, nil
}

// patchArbeitstage liest arbeitstage aus einem JSON-Patch: eine Liste
// ganzer Zahlen (JSON liefert float64). null ist nicht zulaessig - die
// Spalte hat immer einen Wert.
func patchArbeitstage(v any) ([]int, error) {
	invalid := errors.New("Ungültiger Wert für Feld: arbeitstage")
	var days []int
	switch t := v.(type) {
	case []int:
		days = t
	case []any:
		days = make([]int, 0, len(t))
		for _, x := range t {
			f, ok := x.(float64)
			if !ok || f != math.Trunc(f) {
				return nil, invalid
			}
			days = append(days, int(f))
		}
	default:
		return nil, invalid
	}
	if days == nil {
		days = []int{}
	}
	return normalizeArbeitstage(days)
}

// patchBundesland liest bundesland aus einem JSON-Patch: null oder "" =
// wieder das Bundesland des Mandanten.
func patchBundesland(v any) (*string, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		return normalizeEmployeeBundesland(&t)
	default:
		return nil, errors.New("Ungültiger Wert für Feld: bundesland")
	}
}

// WorkWeekFromISO macht aus ISO-Wochentagen das Muster fuer CountWorkDays
// (indiziert mit time.Weekday, 0 = Sonntag).
func WorkWeekFromISO(days []int) [7]bool {
	var w [7]bool
	for _, d := range days {
		if d >= 1 && d <= 7 {
			w[d%7] = true
		}
	}
	return w
}

func int16sToInts(in []int16) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func intsToInt16s(in []int) []int16 {
	out := make([]int16, len(in))
	for i, v := range in {
		out[i] = int16(v)
	}
	return out
}
