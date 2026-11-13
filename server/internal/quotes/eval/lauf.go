package eval

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

// LVErgebnis ist das Ergebnis eines Matchers fuer ein LV.
type LVErgebnis struct {
	Datei string
	Ergebnis
}

// Lauf ist das Ergebnis eines Matchers ueber das Korpus.
type Lauf struct {
	Matcher  string
	Schwelle float64
	LVs      []LVErgebnis
	Gesamt   Ergebnis
}

// LadeKorpus liest alle lv-*.erwartet.json eines Verzeichnisses in
// Dateinamen-Reihenfolge und prueft sie gegen den Katalog.
func LadeKorpus(dir string, k *Katalog) ([]*ErwartetesLV, error) {
	dateien, err := filepath.Glob(filepath.Join(dir, "lv-*.erwartet.json"))
	if err != nil {
		return nil, err
	}
	if len(dateien) == 0 {
		return nil, fmt.Errorf("kein LV in %s", dir)
	}
	sort.Strings(dateien)
	out := make([]*ErwartetesLV, 0, len(dateien))
	for _, d := range dateien {
		lv, err := LoadLV(d, k)
		if err != nil {
			return nil, err
		}
		out = append(out, lv)
	}
	return out, nil
}

// Ausfuehren laesst den Matcher jede Position vorschlagen und wertet aus.
// Ein Vorschlag mit Code braucht eine Begruendung, der Score liegt in
// [0, 1] (aufgabe.md §4); sonst bricht der Lauf ab.
func Ausfuehren(lvs []*ErwartetesLV, k *Katalog, m Matcher, schwelle float64) (Lauf, error) {
	lauf := Lauf{Matcher: m.Name(), Schwelle: schwelle}
	var alle []Ergebnis
	for _, lv := range lvs {
		vorschlaege := map[string]Vorschlag{}
		for _, p := range lv.Positionen {
			v := m.Vorschlagen(Eingabe(p), k)
			if v.Score < 0 || v.Score > 1 {
				return Lauf{}, fmt.Errorf("%s %s: Score %v außerhalb [0, 1]", lv.Datei, p.OZ, v.Score)
			}
			if v.Code != "" && strings.TrimSpace(v.Begruendung) == "" {
				return Lauf{}, fmt.Errorf("%s %s: Zuordnung ohne Begründung", lv.Datei, p.OZ)
			}
			vorschlaege[p.OZ] = v
		}
		e := Auswerten(lv, vorschlaege, schwelle)
		lauf.LVs = append(lauf.LVs, LVErgebnis{Datei: lv.Datei, Ergebnis: e})
		alle = append(alle, e)
	}
	lauf.Gesamt = Summe(alle...)
	return lauf, nil
}

func prozent(q float64) string {
	return strings.Replace(fmt.Sprintf("%.1f %%", 100*q), ".", ",", 1)
}

func jaNein(b bool) string {
	if b {
		return "ja"
	}
	return "nein"
}

// Bericht gibt den Lauf als Texttabelle je LV und gesamt aus, dazu die
// Systemvorgaben je Merkmal.
func (l Lauf) Bericht() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Matcher %s, Schwelle %s\n", l.Matcher, strings.Replace(fmt.Sprintf("%.2f", l.Schwelle), ".", ",", 1))
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "LV\tPositionen\tmit Ziel\tautomatisch\trichtig\tfalsch\tPrüfliste\tTrefferquote\tFehlzuordnung\tPrüflistenquote\tZiel\t")
	zeile := func(name string, e Ergebnis) {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t\n", name, e.Positionen, e.MitZiel, e.Automatisch,
			e.Richtig, e.Falsch, e.Pruefliste, prozent(e.Trefferquote()), prozent(e.Fehlzuordnungsrate()),
			prozent(e.Prueflistenquote()), jaNein(e.ZielErreicht()))
	}
	for _, lv := range l.LVs {
		zeile(lv.Datei, lv.Ergebnis)
	}
	zeile("Gesamt", l.Gesamt)
	w.Flush()
	b.WriteString("\n")
	w = tabwriter.NewWriter(&b, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "Merkmal\terwartet\terkannt\trichtig\tGenauigkeit\tVollständigkeit\t")
	for _, name := range Merkmale {
		m := l.Gesamt.Merkmale[name]
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%s\t\n", name, m.Erwartet, m.Erkannt, m.Richtig, prozent(m.Genauigkeit()), prozent(m.Vollstaendigkeit()))
	}
	w.Flush()
	return b.String()
}
