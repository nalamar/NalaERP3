package eval

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nalaerp3/internal/matching"
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
	nie, err := Ausfuehren(lvs, k, matching.NieZuordnen{}, 0.5)
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
	a, err := Ausfuehren(lvs, k, matching.Wortabgleich{}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Ausfuehren(lvs, k, matching.Wortabgleich{}, 0.5)
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
	for _, m := range []LVMatcher{matching.JePosition(matching.NieZuordnen{}), matching.JePosition(matching.Wortabgleich{}), matching.MitSystemvorgaben(matching.JePosition(matching.Wortabgleich{}))} {
		for _, s := range []float64{0.3, 0.5, 0.7, 0.9} {
			l, err := AusfuehrenLV(context.Background(), lvs, k, m, s)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("\n" + l.Bericht())
		}
	}
}

// merkMatcher merkt sich die Eingaben (OZ und Kurztext, in Reihenfolge).
type merkMatcher struct{ kurz *[]string }

func (merkMatcher) Name() string { return "merk" }
func (m merkMatcher) Vorschlagen(p Position, _ *Katalog) Vorschlag {
	*m.kurz = append(*m.kurz, p.OZ+" "+p.Kurztext)
	return Vorschlag{Code: "FB-ALU", Score: 1, Begruendung: "x"}
}

// kopie legt das Korpus in ein neues Verzeichnis; aendern veraendert die
// X83 von lv-02 (alt -> neu, genau einmal).
func kopie(t *testing.T, alt, neu string) string {
	t.Helper()
	dir := t.TempDir()
	dateien, _ := filepath.Glob(filepath.Join("testdata", "lv-*"))
	for _, d := range dateien {
		raw, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(d) == "lv-02.x83" {
			if strings.Count(string(raw), alt) < 1 {
				t.Fatalf("%q fehlt in lv-02.x83", alt)
			}
			raw = []byte(strings.Replace(string(raw), alt, neu, 1))
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(d)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAusfuehrenLiestX83(t *testing.T) {
	k := katalog(t)
	// Die Eingabe kommt aus der X83, nicht aus der Erwartung.
	dir := kopie(t, "Pfosten-Riegel-Fassade Aluminium</span>", "Geänderter Kurztext</span>")
	lvs, err := LadeKorpus(dir, k)
	if err != nil {
		t.Fatal(err)
	}
	var kurz []string
	l, err := Ausfuehren(lvs, k, merkMatcher{&kurz}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(kurz) != 250 || kurz[26] != "1.01.0010 Geänderter Kurztext" || lvs[1].Positionen[0].Kurztext != "Pfosten-Riegel-Fassade Aluminium" ||
		l.Gesamt.Automatisch != 250 || l.LVs[1].Parsefehler != "" || strings.Contains(l.Bericht(), "Parsefehler") {
		t.Fatalf("Eingabe: %d %q %+v", len(kurz), kurz[26], l.Gesamt)
	}
	// Parsefehler: die Positionen des LV landen in der Prüfliste, der Bericht nennt den Fehler.
	dir = kopie(t, "<QU>m2</QU>", "<QU> </QU>")
	lvs, err = LadeKorpus(dir, k)
	if err != nil {
		t.Fatal(err)
	}
	kurz = nil
	l, err = Ausfuehren(lvs, k, merkMatcher{&kurz}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	e := l.LVs[1]
	if !strings.Contains(e.Parsefehler, "Einheit fehlt") || e.Positionen != 26 || e.Pruefliste != 26 || e.Automatisch != 0 ||
		l.LVs[0].Parsefehler != "" || l.Gesamt.Automatisch != 224 || len(kurz) != 224 {
		t.Fatalf("Parsefehler: %+v / %+v", e, l.Gesamt)
	}
	if s := l.Bericht(); !strings.Contains(s, "Parsefehler lv-02.x83: Position 1.01.0010: Einheit fehlt") || strings.Count(s, "Parsefehler") != 1 {
		t.Fatalf("Bericht:\n%s", s)
	}
	// fehlende Datei und LV ohne LadeKorpus sind Fehler des Laufs
	if err := os.Remove(filepath.Join(dir, "lv-02.x83")); err != nil {
		t.Fatal(err)
	}
	if _, err := Ausfuehren(lvs, k, matching.NieZuordnen{}, 0.5); err == nil {
		t.Fatal("fehlende X83 angenommen")
	}
	if _, err := Ausfuehren([]*ErwartetesLV{lv()}, k, matching.NieZuordnen{}, 0.5); err == nil || !strings.Contains(err.Error(), "nicht mit LadeKorpus") {
		t.Fatalf("LV ohne X83: %v", err)
	}
}

// TestAusgangswert: mit dem Parser als Eingabe bleibt der Ausgangswert aus
// ADR 0039 (Wortabgleich, Schwelle 0,7) unverändert.
func TestAusgangswert(t *testing.T) {
	k, lvs := korpus(t)
	l, err := Ausfuehren(lvs, k, matching.Wortabgleich{}, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if g := l.Gesamt; g.Richtig != 47 || g.Falsch != 0 || g.Pruefliste != 203 || prozent(g.Trefferquote()) != "25,5 %" {
		t.Fatalf("Ausgangswert: %+v", g)
	}
}
