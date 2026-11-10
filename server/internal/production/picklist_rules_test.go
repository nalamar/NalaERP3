package production

import (
	"slices"
	"testing"
)

func fp(v float64) *float64 { return &v }

// Variable, damit 0.1+0.2 zur Laufzeit gerechnet wird (Konstanten rechnet
// Go exakt).
var pickTenth = 0.1

func TestPickAbmessung(t *testing.T) {
	for name, tc := range map[string]struct {
		it   Item
		want string
	}{
		"Profil":           {Item{Art: ArtProfil, LengthMM: fp(1230.4)}, "L 1230,4 mm"},
		"Profil ganz":      {Item{Art: ArtProfil, LengthMM: fp(6000)}, "L 6000 mm"},
		"Profil ohne Mass": {Item{Art: ArtProfil}, ""},
		"Glas":             {Item{Art: ArtGlas, WidthMM: fp(1100), HeightMM: fp(1350)}, "1100 × 1350 mm"},
		"Glas halb":        {Item{Art: ArtGlas, WidthMM: fp(1100)}, ""},
		"Artikel":          {Item{Art: ArtArtikel, LengthMM: fp(10)}, ""},
	} {
		if got := PickAbmessung(tc.it); got != tc.want {
			t.Errorf("%s: %q, erwartet %q", name, got, tc.want)
		}
	}
}

func TestPickQty(t *testing.T) {
	for name, tc := range map[string]struct {
		it      Item
		menge   float64
		einheit string
		gesamt  bool
		keep    bool
	}{
		"offen":            {Item{Offen: fp(2.5), MaterialEinheit: "m", Unit: "Stk", MengeGesamt: 4}, 2.5, "m", false, true},
		"ganz entnommen":   {Item{Offen: fp(0), MaterialEinheit: "m", MengeGesamt: 4}, 0, "m", false, false},
		"ohne Umrechnung":  {Item{MaterialEinheit: "m²", Unit: "Stk", MengeGesamt: 2}, 2, "Stk", true, true},
		"ohne Material":    {Item{Unit: "", MengeGesamt: pickTenth + 0.2}, 0.3, "", true, true},
		"Rest gerundet 0+": {Item{Offen: fp(0.000001), MaterialEinheit: "Stk"}, 0.000001, "Stk", false, true},
	} {
		m, e, g, k := PickQty(tc.it)
		if m != tc.menge || e != tc.einheit || g != tc.gesamt || k != tc.keep {
			t.Errorf("%s: %v %q %v %v", name, m, e, g, k)
		}
	}
}

func TestFormatPickPlaces(t *testing.T) {
	for name, tc := range map[string]struct {
		places []stockPlace
		want   []string
	}{
		"leer":               {nil, []string{"kein Bestand"}},
		"nur negativ":        {[]stockPlace{{"HL", "R1", -2}, {"HL", "", 0.0000001}}, []string{"kein Bestand"}},
		"ohne Platz":         {[]stockPlace{{"HL", "", 12.5}}, []string{"HL: 12,5 m"}},
		"sortiert":           {[]stockPlace{{"HL", "R2", 1}, {"HL", "R1", 3}, {"AL", "", 1}}, []string{"HL / R1: 3 m", "AL: 1 m", "HL / R2: 1 m"}},
		"mehr als max":       {[]stockPlace{{"A", "1", 4}, {"A", "2", 3}, {"A", "3", 2}, {"A", "4", 1}, {"A", "5", 0.5}}, []string{"A / 1: 4 m", "A / 2: 3 m", "A / 3: 2 m", "+ 2 weitere"}},
		"genau max":          {[]stockPlace{{"A", "1", 3}, {"A", "2", 2}, {"A", "3", 1}}, []string{"A / 1: 3 m", "A / 2: 2 m", "A / 3: 1 m"}},
		"negativ mittendrin": {[]stockPlace{{"A", "1", -1}, {"A", "2", 2}}, []string{"A / 2: 2 m"}},
	} {
		if got := FormatPickPlaces(tc.places, "m", MaxPickPlaces); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %q, erwartet %q", name, got, tc.want)
		}
	}
}
