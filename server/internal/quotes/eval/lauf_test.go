package eval

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// festerMatcher liefert immer denselben Vorschlag (fuer Fehlerfaelle).
type festerMatcher struct{ v Vorschlag }

func (festerMatcher) Name() string                               { return "fest" }
func (f festerMatcher) Vorschlagen(Position, *Katalog) Vorschlag { return f.v }

func korpus(t *testing.T) (*Katalog, []*ErwartetesLV) {
	t.Helper()
	k := katalog(t)
	lvs, err := LadeKorpus("testdata", k)
	if err != nil {
		t.Fatal(err)
	}
	return k, lvs
}

func TestLadeKorpus(t *testing.T) {
	k, lvs := korpus(t)
	if len(lvs) != 10 || lvs[0].Datei != "lv-01.x83" || lvs[9].Datei != "lv-10.x83" {
		t.Fatalf("Korpus: %d LVs", len(lvs))
	}
	if _, err := LadeKorpus(t.TempDir(), k); err == nil || !strings.Contains(err.Error(), "kein LV") {
		t.Fatalf("leeres Verzeichnis: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lv-01.erwartet.json"), []byte(`{"preis":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LadeKorpus(dir, k); err == nil {
		t.Fatal("ungültiges LV geladen")
	}
}

func TestAusfuehren(t *testing.T) {
	k, lvs := korpus(t)
	nie, err := Ausfuehren(lvs, k, NieZuordnen{}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	g := nie.Gesamt
	if nie.Matcher != "nie-zuordnen" || nie.Schwelle != 0.5 || len(nie.LVs) != 10 || g.Positionen != 250 || g.MitZiel != 184 ||
		g.Automatisch != 0 || g.Pruefliste != 250 || g.Trefferquote() != 0 || g.ZielErreicht() {
		t.Fatalf("nie-zuordnen: %+v", g)
	}
	if nie.LVs[0].Datei != "lv-01.x83" || nie.LVs[0].Positionen != 26 || nie.LVs[3].MitZiel != 16 {
		t.Fatalf("je LV: %+v %+v", nie.LVs[0], nie.LVs[3])
	}
	// Wortabgleich: deterministisch, jede Position genau einmal gezählt
	a, err := Ausfuehren(lvs, k, Wortabgleich{}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Ausfuehren(lvs, k, Wortabgleich{}, 0.5)
	if !reflect.DeepEqual(a, b) || a.Gesamt.Automatisch+a.Gesamt.Pruefliste != 250 || a.Gesamt.Richtig+a.Gesamt.Falsch != a.Gesamt.Automatisch ||
		a.Gesamt.Richtig == 0 {
		t.Fatalf("wortabgleich: %+v", a.Gesamt)
	}
	// Regeln aus aufgabe.md §4
	for name, v := range map[string]Vorschlag{
		"ohne Begründung": {Code: "FB-ALU", Score: 0.9, Begruendung: " "},
		"Score zu hoch":   {Code: "FB-ALU", Score: 1.01, Begruendung: "x"},
		"Score negativ":   {Score: -0.1},
	} {
		if _, err := Ausfuehren(lvs, k, festerMatcher{v}, 0.5); err == nil {
			t.Fatalf("%s angenommen", name)
		}
	}
	if l, err := Ausfuehren(lvs, k, festerMatcher{Vorschlag{Code: "FB-ALU", Score: 0.6, Begruendung: "x"}}, 0.7); err != nil || l.Gesamt.Pruefliste != 250 {
		t.Fatalf("Score unter der Schwelle: %v %+v", err, l.Gesamt)
	}
	if l, err := Ausfuehren(lvs, k, festerMatcher{Vorschlag{Score: 0}}, 0.5); err != nil || l.Gesamt.Pruefliste != 250 {
		t.Fatalf("leerer Vorschlag ohne Begründung: %v", err)
	}
	if l, err := Ausfuehren(lvs, k, festerMatcher{Vorschlag{Code: "FB-ALU", Score: 1, Begruendung: "x"}}, 1); err != nil || l.Gesamt.Automatisch != 250 {
		t.Fatalf("Score 1 an der Grenze: %v", err)
	}
}

func TestBerichtFormat(t *testing.T) {
	k, lvs := korpus(t)
	l, err := Ausfuehren(lvs[:2], k, festerMatcher{Vorschlag{Code: "FE-ALU-1F", Score: 1, Begruendung: "x"}}, 0.25)
	if err != nil {
		t.Fatal(err)
	}
	s := l.Bericht()
	for _, want := range []string{"Matcher fest, Schwelle 0,25", "Trefferquote", "lv-01.x83", "lv-02.x83", "Gesamt",
		"nein", "10,5 %", "92,3 %", "profilserie", "brandschutzklasse", "Vollständigkeit"} {
		if !strings.Contains(s, want) {
			t.Fatalf("fehlt %q:\n%s", want, s)
		}
	}
	// Spaltenfolge: Trefferquote vor Fehlzuordnung
	zeile := s[strings.Index(s, "lv-01.x83"):]
	zeile = zeile[:strings.Index(zeile, "\n")]
	if i, j := strings.Index(zeile, "10,5 %"), strings.Index(zeile, "92,3 %"); i < 0 || j < i {
		t.Fatalf("Spaltenfolge: %q", zeile)
	}
	if strings.Count(s, "\n") != 1+1+2+1+1+1+len(Merkmale) {
		t.Fatalf("Zeilen:\n%s", s)
	}
	if prozent(1.0/3) != "33,3 %" || jaNein(true) != "ja" {
		t.Fatal(prozent(1.0/3), jaNein(true))
	}
}

// TestBericht gibt die Kennzahlen der Basis-Matcher aus; nur mit NALA_EVAL=1.
func TestBericht(t *testing.T) {
	if os.Getenv("NALA_EVAL") == "" {
		t.Skip("NALA_EVAL nicht gesetzt")
	}
	k, lvs := korpus(t)
	for _, m := range []Matcher{NieZuordnen{}, Wortabgleich{}} {
		for _, s := range []float64{0.3, 0.5, 0.7, 0.9} {
			l, err := Ausfuehren(lvs, k, m, s)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("\n" + l.Bericht())
		}
	}
}
