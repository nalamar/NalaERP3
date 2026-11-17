package matching

import "testing"

func TestErkenneSystemvorgaben(t *testing.T) {
	u := func(v float64) *float64 { return &v }
	cases := []struct {
		text  string
		rc    string
		uwert *float64
		brand string
	}{
		// RC-Klasse
		{"Haustür RC2", "RC2", nil, ""},
		{"einbruchhemmend RC 3 nach DIN EN 1627", "RC3", nil, ""},
		{"Serie T75, RC-2", "RC2", nil, ""},
		{"Widerstandsklasse WK2", "RC2", nil, ""},
		{"Widerstandsklasse WK 6", "RC6", nil, ""},
		{"RC7 gibt es nicht", "", nil, ""},
		{"RC0", "", nil, ""},
		{"SRC2 ist kein Treffer", "", nil, ""},
		// U-Wert
		{"Uw ≤ 0,9 W/(m²K)", "", u(0.9), ""},
		{"Uw max. 1,0 W/(m²K).", "", u(1.0), ""},
		{"Uw-Wert: ≤ 0,90 W/(m²K)", "", u(0.9), ""},
		{"Uw-Wert: 0.90", "", u(0.9), ""},
		{"Uw kleiner gleich 0,90 W/m²K", "", u(0.9), ""},
		{"Ud ≤ 1,3 W/(m²K)", "", u(1.3), ""},
		{"Ud-Wert 1,3", "", u(1.3), ""},
		{"Ucw ≤ 1,1 W/(m²K); Ug = 1,0 W/(m²K)", "", u(1.1), ""},
		{"Ug = 1,0 W/(m²K); Ucw-Wert 1,1", "", u(1.1), ""},
		{"U-Wert Fenster 0,9.", "", u(0.9), ""},
		{"U-Wert Tür 1,3", "", u(1.3), ""},
		{"Uw = 1", "", u(1), ""},
		{"Uw < 0,8", "", u(0.8), ""},
		{"Uw siehe Plan, Ud 1,3", "", u(1.3), ""},
		{"Ug 0,6 und Uf 1,2", "", nil, ""},
		{"Ug-Wert 0,5; Uw-Wert 0,9", "", u(0.9), ""},
		{"Ug 0,5 / Uw 0,9", "", u(0.9), ""},
		{"Uw 12,0", "", nil, ""},
		{"Uw ≤ 0", "", nil, ""},
		{"Uw nach Nachweis", "", nil, ""},
		{"Uwe Wert 1,0", "", nil, ""},
		// Brandschutz
		{"Feuerschutztür T30", "", nil, "T30"},
		{"Feuerschutztür T 90, Serie F60", "", nil, "T90"},
		{"Serie F60, T30-RS", "", nil, "T30"},
		{"Brandschutzverglasung F30", "", nil, "F30"},
		{"Brandschutzverglasung G 30", "", nil, "G30"},
		{"Profilserie F60 mit F90-Verglasung", "", nil, "F90"},
		{"Profil Serie F60", "", nil, ""},
		{"Abschluss EI2 30-Sm", "", nil, "EI230"},
		{"Tür EI 60", "", nil, "EI60"},
		{"Verglasung EW 30", "", nil, "EW30"},
		{"E 90 und F30", "", nil, "E90"},
		{"Rauchschutztür RS, Serie F60", "", nil, "RS"},
		{"DIN EN 1627, RS485-Schnittstelle, T300", "", nil, ""},
		// zusammen, Kurztext und Langtext
		{"Haustür RC2, Ud ≤ 1,3 W/(m²K), T30", "RC2", u(1.3), "T30"},
	}
	for _, c := range cases {
		got := ErkenneSystemvorgaben(c.text, "")
		if got.RCKlasse != c.rc || got.Brandschutzklasse != c.brand || (got.UWert == nil) != (c.uwert == nil) ||
			(got.UWert != nil && *got.UWert != *c.uwert) || got.Profilserie != "" || got.Verglasung != "" {
			t.Errorf("%q: %+v (U %v)", c.text, got, got.UWert)
		}
	}
	// Kurz- und Langtext werden getrennt gelesen, nicht zusammengeklebt
	if got := ErkenneSystemvorgaben("Haustür RC", "2 Flügel"); got.RCKlasse != "" {
		t.Errorf("über die Grenze: %+v", got)
	}
	if got := ErkenneSystemvorgaben("Haustür", "Ud 1,3, RC2"); got.RCKlasse != "RC2" || got.UWert == nil {
		t.Errorf("Langtext: %+v", got)
	}
}
