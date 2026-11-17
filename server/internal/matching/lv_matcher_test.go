package matching

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
