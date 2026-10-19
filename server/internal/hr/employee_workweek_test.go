package hr

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// Arbeitstage und Bundesland am Mitarbeiter (Backlog F.8.2.3). Reine
// Funktionen.

func TestNormalizeArbeitstage(t *testing.T) {
	got, err := normalizeArbeitstage(nil)
	if err != nil || !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("nil: %v, %v", got, err)
	}
	// Die Vorgabe wird kopiert, nicht geteilt.
	got[0] = 7
	if DefaultArbeitstage[0] != 1 {
		t.Fatal("Vorgabe wurde veraendert")
	}
	got, err = normalizeArbeitstage([]int{5, 3, 1, 3})
	if err != nil || !reflect.DeepEqual(got, []int{1, 3, 5}) {
		t.Fatalf("sortieren/Doppelte: %v, %v", got, err)
	}
	if _, err := normalizeArbeitstage([]int{}); err == nil || err.Error() != "Mindestens ein Arbeitstag erforderlich" {
		t.Fatalf("leer: %v", err)
	}
	for _, bad := range []int{0, 8, -1} {
		if _, err := normalizeArbeitstage([]int{1, bad}); err == nil {
			t.Fatalf("%d angenommen", bad)
		}
	}
	if got, _ := normalizeArbeitstage([]int{7, 6}); !reflect.DeepEqual(got, []int{6, 7}) {
		t.Fatalf("Wochenende: %v", got)
	}
}

func TestNormalizeEmployeeBundesland(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, c := range []struct {
		in   *string
		want string
	}{{nil, ""}, {s(""), ""}, {s("  "), ""}, {s(" nw "), "NW"}, {s("BY"), "BY"}} {
		got, err := normalizeEmployeeBundesland(c.in)
		if err != nil {
			t.Fatalf("%v: %v", c.in, err)
		}
		if (got == nil) != (c.want == "") || (got != nil && *got != c.want) {
			t.Fatalf("%v: %v, erwartet %q", c.in, got, c.want)
		}
	}
	if _, err := normalizeEmployeeBundesland(s("XX")); !errors.Is(err, ErrUnknownBundesland) {
		t.Fatalf("XX: %v", err)
	}
}

func TestPatchArbeitstageAndBundesland(t *testing.T) {
	got, err := patchArbeitstage([]any{float64(3), float64(1)})
	if err != nil || !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("JSON-Liste: %v, %v", got, err)
	}
	if _, err := patchArbeitstage([]any{}); err == nil || err.Error() != "Mindestens ein Arbeitstag erforderlich" {
		t.Fatalf("leere Liste: %v", err)
	}
	for _, bad := range []any{nil, "1,2", float64(1), []any{"1"}, []any{1.5}, []any{float64(9)}} {
		if _, err := patchArbeitstage(bad); err == nil {
			t.Fatalf("%#v angenommen", bad)
		}
	}
	if land, err := patchBundesland(nil); err != nil || land != nil {
		t.Fatalf("null: %v, %v", land, err)
	}
	if land, err := patchBundesland("sn"); err != nil || land == nil || *land != "SN" {
		t.Fatalf("sn: %v, %v", land, err)
	}
	if _, err := patchBundesland(float64(1)); err == nil || err.Error() != "Ungültiger Wert für Feld: bundesland" {
		t.Fatalf("Zahl: %v", err)
	}
	if _, err := patchBundesland("XX"); !errors.Is(err, ErrUnknownBundesland) {
		t.Fatalf("XX: %v", err)
	}
}

func TestWorkWeekFromISO(t *testing.T) {
	if got := WorkWeekFromISO([]int{1, 2, 3, 4, 5}); got != StandardWorkWeek {
		t.Fatalf("Mo-Fr: %v", got)
	}
	got := WorkWeekFromISO([]int{6, 7})
	if !got[time.Saturday] || !got[time.Sunday] || got[time.Monday] {
		t.Fatalf("Wochenende: %v", got)
	}
}
