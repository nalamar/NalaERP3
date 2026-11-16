package leistungen

import (
	"strings"
	"testing"
)

func TestNormalisieren(t *testing.T) {
	got := Normalisieren(Eingabe{Code: " FE-1 ", Kurztext: " Fenster ", Langtext: " lang ", Einheit: " Stk ", Gewerk: " fenster ",
		Profilserie: " A75 ", Verglasung: " 3-fach ", RCKlasse: " rc 2 ", Brandschutzklasse: " T30 "})
	want := Eingabe{Code: "FE-1", Kurztext: "Fenster", Langtext: "lang", Einheit: "Stk", Gewerk: "fenster", Profilserie: "A75",
		Verglasung: "3-fach", RCKlasse: "RC2", Brandschutzklasse: "T30"}
	if got.Code != want.Code || got.Kurztext != want.Kurztext || got.Langtext != want.Langtext || got.Einheit != want.Einheit ||
		got.Gewerk != want.Gewerk || got.Profilserie != want.Profilserie || got.Verglasung != want.Verglasung ||
		got.RCKlasse != want.RCKlasse || got.Brandschutzklasse != want.Brandschutzklasse {
		t.Fatalf("Normalisieren = %+v", got)
	}
	if Normalisieren(Eingabe{RCKlasse: "Rc-3"}).RCKlasse != "RC3" {
		t.Fatal("Bindestrich")
	}
}

func TestPruefen(t *testing.T) {
	u := func(v float64) *float64 { return &v }
	gut := Eingabe{Code: "FE-1", Kurztext: "Fenster", Einheit: "Stk"}
	if err := Pruefen(gut); err != nil {
		t.Fatal(err)
	}
	grenzen := gut
	grenzen.Code, grenzen.Kurztext, grenzen.Einheit = strings.Repeat("ä", 50), strings.Repeat("k", 500), strings.Repeat("e", 20)
	grenzen.Langtext, grenzen.Gewerk, grenzen.Profilserie = strings.Repeat("l", 20000), strings.Repeat("g", 100), strings.Repeat("p", 200)
	grenzen.Verglasung, grenzen.Brandschutzklasse, grenzen.RCKlasse, grenzen.UWert = strings.Repeat("v", 200), strings.Repeat("b", 200), "RC6", u(10)
	if err := Pruefen(grenzen); err != nil {
		t.Fatalf("Grenzen: %v", err)
	}
	if err := Pruefen(Eingabe{Code: "x", Kurztext: "y", Einheit: "z", RCKlasse: "RC1", UWert: u(0.001)}); err != nil {
		t.Fatalf("untere Grenzen: %v", err)
	}
	for name, c := range map[string]struct {
		mod  func(*Eingabe)
		want string
	}{
		"Code leer":        {func(e *Eingabe) { e.Code = "" }, "Code erforderlich"},
		"Code lang":        {func(e *Eingabe) { e.Code = strings.Repeat("c", 51) }, "Code ungültig (ohne Leerzeichen, höchstens 50 Zeichen)"},
		"Code Leerzeichen": {func(e *Eingabe) { e.Code = "FE 1" }, "Code ungültig (ohne Leerzeichen, höchstens 50 Zeichen)"},
		"Code Tab":         {func(e *Eingabe) { e.Code = "FE\t1" }, "Code ungültig (ohne Leerzeichen, höchstens 50 Zeichen)"},
		"Kurztext leer":    {func(e *Eingabe) { e.Kurztext = "" }, "Kurztext erforderlich"},
		"Kurztext lang":    {func(e *Eingabe) { e.Kurztext = strings.Repeat("k", 501) }, "Kurztext ungültig (höchstens 500 Zeichen)"},
		"Langtext lang":    {func(e *Eingabe) { e.Langtext = strings.Repeat("l", 20001) }, "Langtext ungültig (höchstens 20000 Zeichen)"},
		"Einheit leer":     {func(e *Eingabe) { e.Einheit = "" }, "Einheit erforderlich"},
		"Einheit lang":     {func(e *Eingabe) { e.Einheit = strings.Repeat("e", 21) }, "Einheit ungültig (höchstens 20 Zeichen)"},
		"Gewerk lang":      {func(e *Eingabe) { e.Gewerk = strings.Repeat("g", 101) }, "Gewerk ungültig (höchstens 100 Zeichen)"},
		"Profilserie":      {func(e *Eingabe) { e.Profilserie = strings.Repeat("p", 201) }, "Systemvorgabe ungültig (höchstens 200 Zeichen)"},
		"Verglasung":       {func(e *Eingabe) { e.Verglasung = strings.Repeat("v", 201) }, "Systemvorgabe ungültig (höchstens 200 Zeichen)"},
		"Brandschutz":      {func(e *Eingabe) { e.Brandschutzklasse = strings.Repeat("b", 201) }, "Systemvorgabe ungültig (höchstens 200 Zeichen)"},
		"RC7":              {func(e *Eingabe) { e.RCKlasse = "RC7" }, "RC-Klasse ungültig (RC1 bis RC6)"},
		"RC0":              {func(e *Eingabe) { e.RCKlasse = "RC0" }, "RC-Klasse ungültig (RC1 bis RC6)"},
		"RC2x":             {func(e *Eingabe) { e.RCKlasse = "RC2x" }, "RC-Klasse ungültig (RC1 bis RC6)"},
		"WK2":              {func(e *Eingabe) { e.RCKlasse = "WK2" }, "RC-Klasse ungültig (RC1 bis RC6)"},
		"U 0":              {func(e *Eingabe) { e.UWert = u(0) }, "U-Wert ungültig (über 0 bis 10)"},
		"U negativ":        {func(e *Eingabe) { e.UWert = u(-1) }, "U-Wert ungültig (über 0 bis 10)"},
		"U 10,01":          {func(e *Eingabe) { e.UWert = u(10.01) }, "U-Wert ungültig (über 0 bis 10)"},
	} {
		e := gut
		c.mod(&e)
		if err := Pruefen(e); err == nil || err.Error() != c.want {
			t.Errorf("%s: %v", name, err)
		}
	}
}
