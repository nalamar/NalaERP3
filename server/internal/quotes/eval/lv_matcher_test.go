package eval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nalaerp3/internal/quotes/gaeb"
)

// lvFest liefert feste Vorschlaege oder einen Fehler und zaehlt die Aufrufe.
type lvFest struct {
	out   map[string]Vorschlag
	err   error
	calls *int
}

func (lvFest) Name() string { return "lv-fest" }

func (f lvFest) VorschlagenLV(_ context.Context, _ *gaeb.LV, _ *Katalog) (map[string]Vorschlag, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.out, f.err
}

func testLV() *gaeb.LV {
	return &gaeb.LV{Positionen: []gaeb.Position{
		{OZ: "01", Kurztext: "Haustür RC2", Langtext: "Ud ≤ 1,3 W/(m²K), T30", Einheit: "Stk"},
		{OZ: "02", Kurztext: "Fensterbank", Langtext: "Aluminium", Einheit: "m"},
		{OZ: "03", Kurztext: "Brandschutzverglasung F30", Einheit: "m2"},
	}}
}

func TestJePosition(t *testing.T) {
	k := katalog(t)
	var kurz []string
	m := JePosition(merkMatcher{&kurz})
	if m.Name() != "merk" {
		t.Fatal(m.Name())
	}
	out, err := m.VorschlagenLV(context.Background(), testLV(), k)
	if err != nil || len(out) != 3 || out["02"].Code != "FB-ALU" || strings.Join(kurz, "|") != "01 Haustür RC2|02 Fensterbank|03 Brandschutzverglasung F30" {
		t.Fatalf("JePosition: %v %v %v", out, err, kurz)
	}
	// abgebrochen: Fehler, keine weitere Position
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	kurz = nil
	if out, err := m.VorschlagenLV(ctx, testLV(), k); !errors.Is(err, context.Canceled) || out != nil || len(kurz) != 0 {
		t.Fatalf("abgebrochen: %v %v %v", out, err, kurz)
	}
}

func TestMitSystemvorgaben(t *testing.T) {
	k := katalog(t)
	falsch := 9.9
	inner := lvFest{out: map[string]Vorschlag{
		"01": {Code: "TU-HAUS-RC2", Score: 0.8, Begruendung: "Modell", Systemvorgaben: Systemvorgaben{
			Profilserie: "Serie T75", Verglasung: "2-fach", RCKlasse: "RC9", UWert: &falsch, Brandschutzklasse: "XX"}},
		"02": {Code: "FB-ALU", Score: 0.9, Begruendung: "Modell", Systemvorgaben: Systemvorgaben{RCKlasse: "RC4", Brandschutzklasse: "T90"}},
	}}
	m := MitSystemvorgaben(inner)
	if m.Name() != "lv-fest+systemvorgaben" {
		t.Fatal(m.Name())
	}
	out, err := m.VorschlagenLV(context.Background(), testLV(), k)
	if err != nil || len(out) != 3 {
		t.Fatalf("%v %v", out, err)
	}
	// deterministisch ersetzt die drei Merkmale, Profilserie/Verglasung bleiben, Code bleibt
	a := out["01"]
	if a.Code != "TU-HAUS-RC2" || a.Score != 0.8 || a.Begruendung != "Modell" || a.Systemvorgaben.Profilserie != "Serie T75" ||
		a.Systemvorgaben.Verglasung != "2-fach" || a.Systemvorgaben.RCKlasse != "RC2" || a.Systemvorgaben.UWert == nil ||
		*a.Systemvorgaben.UWert != 1.3 || a.Systemvorgaben.Brandschutzklasse != "T30" {
		t.Fatalf("01: %+v", a)
	}
	// nichts erkannt: Angaben des Matchers werden geleert
	if b := out["02"]; b.Code != "FB-ALU" || b.Systemvorgaben.RCKlasse != "" || b.Systemvorgaben.Brandschutzklasse != "" || b.Systemvorgaben.UWert != nil {
		t.Fatalf("02: %+v", b)
	}
	// ohne Vorschlag: einer ohne Code mit Begründung und den erkannten Merkmalen
	if c := out["03"]; c.Code != "" || c.Score != 0 || c.Begruendung != "kein Vorschlag des Matchers" || c.Systemvorgaben.Brandschutzklasse != "F30" {
		t.Fatalf("03: %+v", c)
	}
	// Matcher liefert nil: alle Positionen bekommen einen Vorschlag
	if out, err := MitSystemvorgaben(lvFest{}).VorschlagenLV(context.Background(), testLV(), k); err != nil || len(out) != 3 || out["01"].Systemvorgaben.RCKlasse != "RC2" {
		t.Fatalf("nil: %v %v", out, err)
	}
	// Fehler wird weitergereicht
	boom := errors.New("Tageslimit")
	if out, err := MitSystemvorgaben(lvFest{err: boom}).VorschlagenLV(context.Background(), testLV(), k); !errors.Is(err, boom) || out != nil {
		t.Fatalf("Fehler: %v %v", out, err)
	}
}

func TestAusfuehrenLV(t *testing.T) {
	k, lvs := korpus(t)
	ctx := context.Background()
	// unbekannte OZ bricht ab
	if _, err := AusfuehrenLV(ctx, lvs, k, lvFest{out: map[string]Vorschlag{"99.9999": {}}}, 0.5); err == nil || !strings.Contains(err.Error(), `lv-01.x83: Vorschlag für unbekannte OZ "99.9999"`) {
		t.Fatalf("unbekannte OZ: %v", err)
	}
	// Score und Begründung werden auch hier geprüft
	oz := lvs[0].Positionen[0].OZ
	for _, v := range []Vorschlag{{Code: "FB-ALU", Score: 1.5, Begruendung: "x"}, {Code: "FB-ALU", Score: 0.5, Begruendung: " "}} {
		if _, err := AusfuehrenLV(ctx, lvs, k, lvFest{out: map[string]Vorschlag{oz: v}}, 0.5); err == nil || !strings.Contains(err.Error(), "lv-01.x83 "+oz) {
			t.Fatalf("%+v: %v", v, err)
		}
	}
	// Fehler des Matchers: Lauf bricht mit Datei ab
	boom := errors.New("Tageslimit erreicht")
	calls := 0
	if _, err := AusfuehrenLV(ctx, lvs, k, lvFest{err: boom, calls: &calls}, 0.5); !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "lv-01.x83: ") || calls != 1 {
		t.Fatalf("Fehler: %v %d", err, calls)
	}
	// fehlende OZ landen in der Prüfliste; ein Aufruf je LV
	calls = 0
	l, err := AusfuehrenLV(ctx, lvs, k, lvFest{out: nil, calls: &calls}, 0.5)
	if err != nil || calls != 10 || l.Gesamt.Pruefliste != 250 || l.Matcher != "lv-fest" {
		t.Fatalf("ohne Vorschläge: %v %d %+v", err, calls, l.Gesamt)
	}
	// Parsefehler: der Matcher wird für dieses LV nicht gefragt
	dir := kopie(t, "<QU>m2</QU>", "<QU> </QU>")
	broken, err := LadeKorpus(dir, k)
	if err != nil {
		t.Fatal(err)
	}
	calls = 0
	if l, err := AusfuehrenLV(ctx, broken, k, lvFest{calls: &calls}, 0.5); err != nil || calls != 9 || l.LVs[1].Parsefehler == "" {
		t.Fatalf("Parsefehler: %v %d", err, calls)
	}
}

// TestAusgangswertSystemvorgaben: die deterministische Erkennung (ADR 0044
// Frage 5) am Korpus; die Zuordnung bleibt beim Ausgangswert.
func TestAusgangswertSystemvorgaben(t *testing.T) {
	k, lvs := korpus(t)
	l, err := AusfuehrenLV(context.Background(), lvs, k, MitSystemvorgaben(JePosition(Wortabgleich{})), 0.7)
	if err != nil {
		t.Fatal(err)
	}
	g := l.Gesamt
	if g.Richtig != 47 || g.Falsch != 0 || l.Matcher != "wortabgleich+systemvorgaben" {
		t.Fatalf("Zuordnung: %+v", g)
	}
	want := map[string][3]int{"rc_klasse": {12, 12, 12}, "u_wert": {42, 42, 42}, "brandschutzklasse": {27, 27, 27},
		"profilserie": {51, 0, 0}, "verglasung": {39, 0, 0}}
	for name, w := range want {
		m := g.Merkmale[name]
		if [3]int{m.Erwartet, m.Erkannt, m.Richtig} != w {
			t.Errorf("%s: %+v, erwartet %v", name, m, w)
		}
	}
}
