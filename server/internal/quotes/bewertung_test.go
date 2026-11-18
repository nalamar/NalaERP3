package quotes

import (
	"strings"
	"testing"
)

// Grenzen der Gesamtbewertung bei der Freigabe (Backlog I.8.3b).
func TestPruefeBewertung(t *testing.T) {
	n := func(v int) *int { return &v }
	for _, c := range []struct {
		name string
		in   ImportBewertung
		want string
	}{
		{"leer", ImportBewertung{}, ""},
		{"Note 1", ImportBewertung{Note: n(1)}, ""},
		{"Note 5", ImportBewertung{Note: n(5)}, ""},
		{"Note 0", ImportBewertung{Note: n(0)}, "Bewertung ungültig"},
		{"Note 6", ImportBewertung{Note: n(6)}, "Bewertung ungültig"},
		{"Nacharbeit 0", ImportBewertung{NacharbeitMin: n(0)}, ""},
		{"Nacharbeit 100000", ImportBewertung{NacharbeitMin: n(100000)}, ""},
		{"Nacharbeit -1", ImportBewertung{NacharbeitMin: n(-1)}, "Nacharbeit ungültig"},
		{"Nacharbeit 100001", ImportBewertung{NacharbeitMin: n(100001)}, "Nacharbeit ungültig"},
		{"Text 2000 Umlaute", ImportBewertung{Text: strings.Repeat("ä", 2000)}, ""},
		{"Text 2001 Umlaute", ImportBewertung{Text: strings.Repeat("ä", 2001)}, "höchstens 2000 Zeichen"},
		{"Text 2000 mit Leerraum", ImportBewertung{Text: "  " + strings.Repeat("x", 2000) + "  "}, ""},
	} {
		err := pruefeBewertung(&c.in)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	b := ImportBewertung{Text: "  passt \n"}
	if err := pruefeBewertung(&b); err != nil || b.Text != "passt" {
		t.Fatalf("getrimmt: %q %v", b.Text, err)
	}
	if !(ImportBewertung{Text: ""}).leer() || (ImportBewertung{NacharbeitMin: n(0)}).leer() || (ImportBewertung{Note: n(3)}).leer() || (ImportBewertung{Text: "x"}).leer() {
		t.Fatal("leer() falsch")
	}
}
