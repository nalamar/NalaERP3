package eval

import (
	"math"
	"testing"
)

func TestAuswerten(t *testing.T) {
	l := lv()
	v := map[string]Vorschlag{
		// richtig, Merkmale teils erkannt (Groß-/Kleinschreibung, U-Wert in Toleranz)
		"01.01.0010": {Code: "FE-ALU-1F", Score: 0.9, Systemvorgaben: Systemvorgaben{Profilserie: " serie  a75 ", UWert: f(0.904)}},
		// falsch
		"01.01.0011": {Code: "FE-ALU-1F", Score: 0.8},
		// unter der Schwelle: Prüfliste, auch wenn richtig
		"01.01.0020": {Code: "TU-HAUS-RC2", Score: 0.69, Systemvorgaben: Systemvorgaben{RCKlasse: "RC3", Verglasung: "VSG", UWert: f(1.1)}},
		// kein Ziel erwartet, trotzdem zugeordnet: falsch
		"01.01.0030": {Code: "FB-ALU", Score: 0.95},
	}
	e := Auswerten(l, v, 0.7)
	want := Ergebnis{Positionen: 4, MitZiel: 3, Automatisch: 3, Richtig: 1, Falsch: 2, Pruefliste: 1}
	if e.Positionen != want.Positionen || e.MitZiel != want.MitZiel || e.Automatisch != want.Automatisch ||
		e.Richtig != want.Richtig || e.Falsch != want.Falsch || e.Pruefliste != want.Pruefliste {
		t.Fatalf("Ergebnis = %+v", e)
	}
	if math.Abs(e.Trefferquote()-1.0/3) > 1e-9 || math.Abs(e.Fehlzuordnungsrate()-2.0/3) > 1e-9 || e.Prueflistenquote() != 0.25 {
		t.Fatalf("Quoten %v %v %v", e.Trefferquote(), e.Fehlzuordnungsrate(), e.Prueflistenquote())
	}
	if e.ZielErreicht() {
		t.Fatal("Ziel trotz Fehlzuordnungen erreicht")
	}
	m := e.Merkmale
	if m["profilserie"] != (MerkmalZahlen{Erwartet: 1, Erkannt: 1, Richtig: 1}) ||
		m["u_wert"] != (MerkmalZahlen{Erwartet: 1, Erkannt: 2, Richtig: 1}) ||
		m["rc_klasse"] != (MerkmalZahlen{Erwartet: 1, Erkannt: 1, Richtig: 0}) ||
		m["verglasung"] != (MerkmalZahlen{Erwartet: 0, Erkannt: 1, Richtig: 0}) ||
		m["brandschutzklasse"] != (MerkmalZahlen{}) {
		t.Fatalf("Merkmale = %+v", m)
	}
	if m["rc_klasse"].Genauigkeit() != 0 || m["profilserie"].Vollstaendigkeit() != 1 || m["verglasung"].Vollstaendigkeit() != 0 {
		t.Fatal("Merkmalquoten falsch")
	}
	if z := (MerkmalZahlen{Erwartet: 4, Erkannt: 2, Richtig: 1}); z.Genauigkeit() != 0.5 || z.Vollstaendigkeit() != 0.25 {
		t.Fatalf("Genauigkeit %v, Vollständigkeit %v", z.Genauigkeit(), z.Vollstaendigkeit())
	}
	// U-Wert knapp außerhalb der Toleranz
	v["01.01.0010"] = Vorschlag{Code: "FE-ALU-1F", Score: 0.9, Systemvorgaben: Systemvorgaben{UWert: f(0.906)}}
	if got := Auswerten(l, v, 0.7).Merkmale["u_wert"]; got != (MerkmalZahlen{Erwartet: 1, Erkannt: 2}) {
		t.Fatalf("U-Wert außerhalb der Toleranz: %+v", got)
	}
	// Schwelle genau erreicht zählt als automatisch
	if got := Auswerten(l, v, 0.69); got.Automatisch != 4 || got.Richtig != 2 {
		t.Fatalf("Schwelle genau: %+v", got)
	}
}

func TestOhneVorschlagUndZiel(t *testing.T) {
	l := lv()
	// ohne Vorschläge: alles in der Prüfliste, keine Fehlzuordnung, Ziel verfehlt
	e := Auswerten(l, nil, 0.5)
	if e.Pruefliste != 4 || e.Automatisch != 0 || e.Fehlzuordnungsrate() != 0 || e.Trefferquote() != 0 || e.ZielErreicht() ||
		e.Merkmale["profilserie"] != (MerkmalZahlen{Erwartet: 1}) || e.Merkmale["rc_klasse"] != (MerkmalZahlen{Erwartet: 1}) {
		t.Fatalf("ohne Vorschläge: %+v", e)
	}
	// leerer Code mit hohem Score ist keine Zuordnung
	e = Auswerten(l, map[string]Vorschlag{"01.01.0010": {Score: 1}}, 0.5)
	if e.Automatisch != 0 || e.Pruefliste != 4 {
		t.Fatalf("leerer Code: %+v", e)
	}
	// alle richtig, Rollladen in der Prüfliste: Ziel erreicht
	alle := map[string]Vorschlag{
		"01.01.0010": {Code: "FE-ALU-1F", Score: 1},
		"01.01.0011": {Code: "FE-ALU-2F", Score: 1},
		"01.01.0020": {Code: "TU-HAUS-RC2", Score: 1},
	}
	e = Auswerten(l, alle, 0.5)
	if !e.ZielErreicht() || e.Trefferquote() != 1 || e.Prueflistenquote() != 0.25 {
		t.Fatalf("alle richtig: %+v", e)
	}
	// Grenzen der Zielwerte: genau 70 % Treffer und genau 2 % Fehlzuordnung erreichen das Ziel
	grenze := Ergebnis{MitZiel: 100, Richtig: 70, Automatisch: 100, Falsch: 2}
	if !grenze.ZielErreicht() {
		t.Fatal("Grenzwerte verfehlt")
	}
	if (Ergebnis{MitZiel: 100, Richtig: 69, Automatisch: 100, Falsch: 2}).ZielErreicht() ||
		(Ergebnis{MitZiel: 100, Richtig: 70, Automatisch: 100, Falsch: 3}).ZielErreicht() ||
		(Ergebnis{MitZiel: 100, Richtig: 69, Automatisch: 70, Falsch: 1}).ZielErreicht() ||
		(Ergebnis{}).ZielErreicht() {
		t.Fatal("Ziel trotz Unterschreitung erreicht")
	}
}

func TestSumme(t *testing.T) {
	a := Ergebnis{Positionen: 4, MitZiel: 3, Automatisch: 2, Richtig: 1, Falsch: 1, Pruefliste: 2,
		Merkmale: map[string]MerkmalZahlen{"u_wert": {Erwartet: 1, Erkannt: 1, Richtig: 1}}}
	b := Ergebnis{Positionen: 6, MitZiel: 5, Automatisch: 5, Richtig: 5, Pruefliste: 1,
		Merkmale: map[string]MerkmalZahlen{"u_wert": {Erwartet: 2, Erkannt: 1}, "rc_klasse": {Erwartet: 1}}}
	s := Summe(a, b)
	if s.Positionen != 10 || s.MitZiel != 8 || s.Automatisch != 7 || s.Richtig != 6 || s.Falsch != 1 || s.Pruefliste != 3 {
		t.Fatalf("Summe = %+v", s)
	}
	if s.Merkmale["u_wert"] != (MerkmalZahlen{Erwartet: 3, Erkannt: 2, Richtig: 1}) || s.Merkmale["rc_klasse"] != (MerkmalZahlen{Erwartet: 1}) {
		t.Fatalf("Summe Merkmale = %+v", s.Merkmale)
	}
	if len(Summe().Merkmale) != 0 || Summe().Positionen != 0 {
		t.Fatal("leere Summe")
	}
}
