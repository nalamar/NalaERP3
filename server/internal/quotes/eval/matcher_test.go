package eval

import (
	"reflect"
	"testing"
)

func TestEingabeOhneErwartung(t *testing.T) {
	p := lv().Positionen[0]
	got := Eingabe(p)
	want := Position{OZ: p.OZ, Kurztext: p.Kurztext, Langtext: p.Langtext, Menge: p.Menge, Einheit: p.Einheit, Art: p.Art, Pauschal: p.Pauschal}
	if got != want {
		t.Fatalf("Eingabe = %+v", got)
	}
	// Die Eingabe darf keine Felder der Erwartung tragen.
	for i := 0; i < reflect.TypeOf(got).NumField(); i++ {
		switch reflect.TypeOf(got).Field(i).Name {
		case "Leistung", "Systemvorgaben", "Zu", "Bereich":
			t.Fatalf("Feld %s in der Eingabe", reflect.TypeOf(got).Field(i).Name)
		}
	}
	p.Pauschal, p.Einheit = true, "psch"
	if g := Eingabe(p); !g.Pauschal || g.Einheit != "psch" {
		t.Fatalf("Pauschale: %+v", g)
	}
}

func TestNieZuordnen(t *testing.T) {
	k := katalog(t)
	v := NieZuordnen{}.Vorschlagen(Eingabe(lv().Positionen[0]), k)
	if v.Code != "" || v.Score != 0 || v.Begruendung == "" || (NieZuordnen{}).Name() != "nie-zuordnen" {
		t.Fatalf("NieZuordnen = %+v", v)
	}
}

func TestWoerter(t *testing.T) {
	got := woerter("Fenster 1-flg., Alu-Fenster liefern und montieren; Ü-Wert ÄÖ 4/16")
	want := map[string]bool{"fenster": true, "flg": true, "alu": true, "wert": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("woerter = %v", got)
	}
	if got := woerter("Außenfensterbank T30"); !got["außenfensterbank"] || !got["t30"] {
		t.Fatalf("Umlaute/Ziffern: %v", got)
	}
}

func TestWortabgleich(t *testing.T) {
	k := katalog(t)
	m := Wortabgleich{}
	if m.Name() != "wortabgleich" {
		t.Fatal(m.Name())
	}
	cases := []struct {
		kurztext, code string
	}{
		{"Feuerschutztür einflügelig", "TU-BS-T30"},
		{"Stahltreppe gerade, Gitterroststufen", "ST-TREPPE"},
		{"Altfenster ausbauen und entsorgen", "DEM-FE"},
		{"Aluminiumfenster zweiflügelig", "FE-ALU-2F"},
		{"Sektionaltor mit Antrieb", ""},
		{"und mit liefern", ""},
	}
	for _, c := range cases {
		v := m.Vorschlagen(Position{Kurztext: c.kurztext, Langtext: "Feuerschutztür Stahltreppe"}, k)
		if v.Code != c.code || (v.Code == "") != (v.Score == 0) || v.Score > 1 || v.Begruendung == "" {
			t.Fatalf("%q: %+v, erwartet %q", c.kurztext, v, c.code)
		}
		if c.code == "" && v.Begruendung != "kein gemeinsames Wort mit dem Katalog" {
			t.Fatalf("%q: Begründung %q", c.kurztext, v.Begruendung)
		}
	}
	// Score und Begründung: 2 gemeinsame Wörter bei 3 (Position) und 2 (Leistung, ohne Füllwörter)
	v := m.Vorschlagen(Position{Kurztext: "Aluminiumfenster einflügelig mit Oberlicht"}, k)
	if v.Code != "FE-ALU-1F" || v.Score != 2*2.0/5 || v.Begruendung != "gemeinsame Wörter: aluminiumfenster, einflügelig" {
		t.Fatalf("Score/Begründung: %+v", v)
	}
	// Gleichstand: die frühere Leistung gewinnt
	tie := &Katalog{Leistungen: []Leistung{{Code: "B", Kurztext: "Blech rund"}, {Code: "A", Kurztext: "Blech eckig"}}}
	if v := m.Vorschlagen(Position{Kurztext: "Blech"}, tie); v.Code != "B" {
		t.Fatalf("Gleichstand: %+v", v)
	}
	// Begründung sortiert, unabhängig von der Reihenfolge im Text
	vier := &Katalog{Leistungen: []Leistung{{Code: "V", Kurztext: "Rund Glatt Eckig Blech"}}}
	for i := 0; i < 5; i++ {
		if v := m.Vorschlagen(Position{Kurztext: "glatt eckig rund blech"}, vier); v.Begruendung != "gemeinsame Wörter: blech, eckig, glatt, rund" {
			t.Fatalf("Begründung: %q", v.Begruendung)
		}
	}
	// höherer Score schlägt frühere Leistung
	if v := m.Vorschlagen(Position{Kurztext: "Blech eckig"}, tie); v.Code != "A" || v.Score != 1 {
		t.Fatalf("besserer Treffer: %+v", v)
	}
}
