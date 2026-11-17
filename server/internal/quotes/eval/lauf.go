package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"nalaerp3/internal/matching"
	"nalaerp3/internal/quotes/gaeb"
)

// LVErgebnis ist das Ergebnis eines Matchers fuer ein LV. Parsefehler ist
// gesetzt, wenn der Parser die X83 ablehnt; dann landen alle Positionen
// in der Pruefliste.
type LVErgebnis struct {
	Datei       string
	Parsefehler string
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
		lv.x83 = filepath.Join(dir, lv.Datei)
		out = append(out, lv)
	}
	return out, nil
}

// lese liest die X83 eines mit LadeKorpus geladenen LV mit dem Parser.
// Fehlt die Datei, ist das ein Fehler des Laufs; lehnt der Parser sie ab,
// liefert lese den Parsefehler.
func lese(lv *ErwartetesLV) (*gaeb.LV, string, error) {
	if lv.x83 == "" {
		return nil, "", fmt.Errorf("%s: LV nicht mit LadeKorpus geladen", lv.Datei)
	}
	f, err := os.Open(lv.x83)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	gelesen, err := gaeb.Parse(f)
	if err != nil {
		return nil, err.Error(), nil
	}
	return gelesen, "", nil
}

// Ausfuehren laesst einen Matcher je Position ueber das Korpus laufen
// (siehe AusfuehrenLV).
func Ausfuehren(lvs []*ErwartetesLV, k *Katalog, m Matcher, schwelle float64) (Lauf, error) {
	return AusfuehrenLV(context.Background(), lvs, k, matching.JePosition(m), schwelle)
}

// AusfuehrenLV liest jede X83 mit dem Parser, laesst den Matcher fuer das
// gelesene LV vorschlagen und wertet gegen die Erwartung aus. Lehnt der
// Parser die X83 ab, wird der Matcher nicht gefragt. Ein Vorschlag mit Code
// braucht eine Begruendung, der Score liegt in [0, 1] (aufgabe.md §4), und
// jede OZ muss im LV stehen; sonst bricht der Lauf ab, ebenso bei einem
// Fehler des Matchers.
func AusfuehrenLV(ctx context.Context, lvs []*ErwartetesLV, k *Katalog, m LVMatcher, schwelle float64) (Lauf, error) {
	lauf := Lauf{Matcher: m.Name(), Schwelle: schwelle}
	var alle []Ergebnis
	for _, lv := range lvs {
		gelesen, parsefehler, err := lese(lv)
		if err != nil {
			return Lauf{}, err
		}
		vorschlaege := map[string]Vorschlag{}
		if gelesen != nil {
			if vorschlaege, err = m.VorschlagenLV(ctx, gelesen, k); err != nil {
				return Lauf{}, fmt.Errorf("%s: %w", lv.Datei, err)
			}
			if err := pruefeVorschlaege(lv.Datei, gelesen.Positionen, vorschlaege); err != nil {
				return Lauf{}, err
			}
		}
		e := Auswerten(lv, vorschlaege, schwelle)
		lauf.LVs = append(lauf.LVs, LVErgebnis{Datei: lv.Datei, Parsefehler: parsefehler, Ergebnis: e})
		alle = append(alle, e)
	}
	lauf.Gesamt = Summe(alle...)
	return lauf, nil
}

func pruefeVorschlaege(datei string, positionen []gaeb.Position, vorschlaege map[string]Vorschlag) error {
	bekannt := make(map[string]bool, len(positionen))
	for _, p := range positionen {
		bekannt[p.OZ] = true
	}
	for oz := range vorschlaege {
		if !bekannt[oz] {
			return fmt.Errorf("%s: Vorschlag für unbekannte OZ %q", datei, oz)
		}
	}
	for _, p := range positionen {
		v, ok := vorschlaege[p.OZ]
		if !ok {
			continue
		}
		if v.Score < 0 || v.Score > 1 {
			return fmt.Errorf("%s %s: Score %v außerhalb [0, 1]", datei, p.OZ, v.Score)
		}
		if v.Code != "" && strings.TrimSpace(v.Begruendung) == "" {
			return fmt.Errorf("%s %s: Zuordnung ohne Begründung", datei, p.OZ)
		}
	}
	return nil
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
	for _, lv := range l.LVs {
		if lv.Parsefehler != "" {
			fmt.Fprintf(&b, "Parsefehler %s: %s\n", lv.Datei, lv.Parsefehler)
		}
	}
	return b.String()
}
