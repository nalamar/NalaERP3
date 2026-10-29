package fleet

import (
	"math"
	"strings"
	"testing"
	"time"
)

// Fachregeln der Fahrzeugkosten ohne Datenbank (Backlog G.3d.1).

func floatPtr(v float64) *float64 { return &v }

func TestNormalizeCostInput(t *testing.T) {
	in := NormalizeCostInput(CostInput{
		Datum: time.Date(2026, 3, 2, 15, 4, 0, 0, time.UTC), Kostenart: " Kraftstoff ", Bezeichnung: " Diesel ",
		Lieferant: " Tanke ", Belegnummer: " B-1 ", Notiz: " n ", KostenstelleID: strPtr("  "),
	})
	if in.Kostenart != KostenartKraftstoff || in.Bezeichnung != "Diesel" || in.Lieferant != "Tanke" || in.Belegnummer != "B-1" ||
		in.Notiz != "n" || in.KostenstelleID != nil || !in.Datum.Equal(day(2026, 3, 2)) {
		t.Fatalf("normalisiert: %+v", in)
	}
	if k := NormalizeCostInput(CostInput{KostenstelleID: strPtr(" cc-1 ")}).KostenstelleID; k == nil || *k != "cc-1" {
		t.Fatalf("Kostenstelle: %v", k)
	}
	if d := NormalizeCostInput(CostInput{}).Datum; !d.IsZero() {
		t.Fatalf("leeres Datum bleibt leer: %v", d)
	}
}

func TestValidateCostInput(t *testing.T) {
	now := day(2026, 3, 10)
	ok := CostInput{Datum: day(2026, 3, 10), Kostenart: KostenartKraftstoff, BetragNetto: 80.12, KmStand: intPtr(0), Liter: floatPtr(45.5)}
	with := func(f func(*CostInput)) CostInput { c := ok; f(&c); return c }
	for name, in := range map[string]CostInput{
		"Kraftstoff mit Litern": ok,
		"Sonstiges mit Name":    with(func(c *CostInput) { c.Kostenart = KostenartSonstiges; c.Bezeichnung = "Vignette"; c.Liter = nil }),
		"ein Cent":              with(func(c *CostInput) { c.BetragNetto = 0.01 }),
		"alter Beleg":           with(func(c *CostInput) { c.Datum = day(2020, 1, 1) }),
	} {
		if err := ValidateCostInput(in, now); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, art := range Kostenarten {
		c := with(func(c *CostInput) { c.Kostenart = art; c.Bezeichnung = "x"; c.Liter = nil })
		if err := ValidateCostInput(c, now); err != nil {
			t.Errorf("Kostenart %s: %v", art, err)
		}
	}
	for want, in := range map[string]CostInput{
		"Datum erforderlich":                                with(func(c *CostInput) { c.Datum = time.Time{} }),
		"Datum darf nicht in der Zukunft liegen":            with(func(c *CostInput) { c.Datum = day(2026, 3, 11) }),
		"Ungültige Kostenart":                               with(func(c *CostInput) { c.Kostenart = "bussgeld" }),
		"Bezeichnung ist bei sonstigen Kosten erforderlich": with(func(c *CostInput) { c.Kostenart = KostenartSonstiges; c.Liter = nil }),
		"Betrag muss größer als 0 sein":                     with(func(c *CostInput) { c.BetragNetto = 0 }),
		"Betrag darf höchstens zwei Nachkommastellen haben": with(func(c *CostInput) { c.BetragNetto = 12.345 }),
		// knapp neben dem Cent: die Toleranz faengt nur Rechenungenauigkeit ab
		"Betrag darf höchstens zwei Nachkommastellen haben ": with(func(c *CostInput) { c.BetragNetto = 10.00001 }),
		"Kilometerstand darf nicht negativ sein":             with(func(c *CostInput) { c.KmStand = intPtr(-1) }),
		"Liter nur bei Kraftstoff":                           with(func(c *CostInput) { c.Kostenart = KostenartWartung }),
		"Liter muss größer als 0 sein":                       with(func(c *CostInput) { c.Liter = floatPtr(0) }),
		"Liter darf höchstens zwei Nachkommastellen haben":   with(func(c *CostInput) { c.Liter = floatPtr(1.005) }),
	} {
		if err := ValidateCostInput(in, now); err == nil || err.Error() != strings.TrimSpace(want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		if err := ValidateCostInput(with(func(c *CostInput) { c.BetragNetto = bad }), now); err == nil || err.Error() != "Betrag muss größer als 0 sein" {
			t.Errorf("Betrag %v: %v", bad, err)
		}
		if err := ValidateCostInput(with(func(c *CostInput) { c.Liter = floatPtr(bad) }), now); err == nil || err.Error() != "Liter muss größer als 0 sein" {
			t.Errorf("Liter %v: %v", bad, err)
		}
	}
}

func TestRoundCents(t *testing.T) {
	for in, want := range map[float64]int64{0.1 + 0.2: 30, 19.99: 1999, 0.005: 1, -0.004: 0, -1.25: -125} {
		if got := roundCents(in); got != want {
			t.Errorf("%v: %d", in, got)
		}
	}
}
