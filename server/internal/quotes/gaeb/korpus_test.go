package gaeb_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"nalaerp3/internal/quotes/eval"
	"nalaerp3/internal/quotes/gaeb"
)

// erwartet bildet die Erwartung des Evaluationssets auf das Ergebnis des
// Parsers ab (nur Parse-Felder, ohne Leistung und Systemvorgaben).
func erwartet(lv *eval.ErwartetesLV) *gaeb.LV {
	out := &gaeb.LV{Format: lv.Format, Phase: lv.Phase, Vorbemerkungen: lv.Vorbemerkungen, Bereiche: []gaeb.Bereich{}}
	for _, b := range lv.Bereiche {
		out.Bereiche = append(out.Bereiche, gaeb.Bereich{OZ: b.OZ, Stufe: b.Stufe, Bezeichnung: b.Bezeichnung, Eltern: b.Eltern})
	}
	for _, p := range lv.Positionen {
		out.Positionen = append(out.Positionen, gaeb.Position{OZ: p.OZ, Bereich: p.Bereich, Kurztext: p.Kurztext, Langtext: p.Langtext,
			Menge: p.Menge, Einheit: p.Einheit, Art: p.Art, Pauschal: p.Pauschal, Zu: p.Zu})
	}
	return out
}

// TestKorpus misst den Parser an allen X83 des Evaluationssets (ADR 0040):
// jedes Feld muss der Erwartung entsprechen.
func TestKorpus(t *testing.T) {
	dir := filepath.Join("..", "eval", "testdata")
	k, err := eval.LoadKatalog(filepath.Join(dir, "katalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	lvs, err := eval.LadeKorpus(dir, k)
	if err != nil {
		t.Fatal(err)
	}
	positionen := 0
	for _, lv := range lvs {
		f, err := os.Open(filepath.Join(dir, lv.Datei))
		if err != nil {
			t.Fatal(err)
		}
		got, err := gaeb.Parse(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", lv.Datei, err)
		}
		want := erwartet(lv)
		if got.Format != want.Format || got.Phase != want.Phase || !reflect.DeepEqual(got.Vorbemerkungen, want.Vorbemerkungen) ||
			!reflect.DeepEqual(got.Bereiche, want.Bereiche) {
			t.Fatalf("%s: Kopf, Vorbemerkungen oder Bereiche\n got %+v\nwant %+v", lv.Datei, *got, *want)
		}
		if len(got.Positionen) != len(want.Positionen) {
			t.Fatalf("%s: %d Positionen, erwartet %d", lv.Datei, len(got.Positionen), len(want.Positionen))
		}
		for i := range want.Positionen {
			if got.Positionen[i] != want.Positionen[i] {
				t.Fatalf("%s:\n got %+v\nwant %+v", lv.Datei, got.Positionen[i], want.Positionen[i])
			}
		}
		positionen += len(got.Positionen)
	}
	if len(lvs) != 10 || positionen != 250 {
		t.Fatalf("%d LVs, %d Positionen", len(lvs), positionen)
	}
}
