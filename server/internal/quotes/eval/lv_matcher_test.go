package eval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nalaerp3/internal/matching"
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
	l, err := AusfuehrenLV(context.Background(), lvs, k, matching.MitSystemvorgaben(matching.JePosition(matching.Wortabgleich{})), 0.7)
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
