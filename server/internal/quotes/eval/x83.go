package eval

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Die X83-Dateien des Korpus werden aus der Erwartung erzeugt (ADR 0039,
// I.1.2). Die Elementnamen stammen aus Sekundaerquellen: belegt sind
// Provis und LumpSumItem, die uebrigen (u. a. ALNGroupNo/ALNSerNo fuer
// Grund- und Alternativpositionen) prueft I.2 gegen das offizielle Schema
// oder einen echten Export. Sie stehen nur hier, damit das Korpus nach
// einer Korrektur neu erzeugt werden kann (go test -run TestKorpus -update).
var x83Namespaces = map[string]string{
	"da_xml_3.2": "http://www.gaeb.de/GAEB_DA_XML/DA83/3.2",
	"da_xml_3.3": "http://www.gaeb.de/GAEB_DA_XML/DA83/3.3",
}

type x83Writer struct {
	b      bytes.Buffer
	ind    int
	ids    int
	remark []string       // Vorbemerkungen, vor der ersten Position geschrieben
	gruppe map[string]int // Grundposition -> Nummer der Gruppe
	serie  map[string]int // Grundposition -> zuletzt vergebene Alternative
}

func (w *x83Writer) esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (w *x83Writer) line(s string) {
	w.b.WriteString(strings.Repeat("  ", w.ind) + s + "\n")
}

func (w *x83Writer) open(tag string) { w.line("<" + tag + ">"); w.ind++ }

func (w *x83Writer) close(tag string) { w.ind--; w.line("</" + tag + ">") }

func (w *x83Writer) leaf(tag, text string) { w.line("<" + tag + ">" + w.esc(text) + "</" + tag + ">") }

func (w *x83Writer) id(prefix string) string { w.ids++; return prefix + strconv.Itoa(w.ids) }

// absatz schreibt Text als Absaetze, je Zeile ein <p>.
func (w *x83Writer) absatz(tag, text string) {
	w.open(tag)
	for _, z := range strings.Split(text, "\n") {
		w.line("<p><span>" + w.esc(z) + "</span></p>")
	}
	w.close(tag)
}

// letzterTeil prueft, dass oz direkt unter eltern liegt, und liefert die
// letzte Stelle der OZ (RNoPart).
func letzterTeil(oz, eltern string) (string, error) {
	teil := oz
	if eltern != "" {
		if !strings.HasPrefix(oz, eltern+".") {
			return "", fmt.Errorf("OZ %s liegt nicht unter %s", oz, eltern)
		}
		teil = strings.TrimPrefix(oz, eltern+".")
	}
	if teil == "" || strings.Contains(teil, ".") {
		return "", fmt.Errorf("OZ %s liegt nicht direkt unter %q", oz, eltern)
	}
	return teil, nil
}

// SchreibeX83 erzeugt eine Angebotsaufforderung (X83) in DA XML 3.2 oder
// 3.3. Erwartet ein mit Validate geprueftes LV.
func SchreibeX83(lv *ErwartetesLV) ([]byte, error) {
	ns, ok := x83Namespaces[lv.Format]
	if !ok {
		return nil, fmt.Errorf("Format %q wird nicht erzeugt", lv.Format)
	}
	if lv.Phase != "X83" {
		return nil, fmt.Errorf("Phase %q wird nicht erzeugt", lv.Phase)
	}
	kinder := map[string][]Bereich{}
	for _, b := range lv.Bereiche {
		kinder[b.Eltern] = append(kinder[b.Eltern], b)
	}
	pos := map[string][]ErwartetePosition{}
	tiefe, laengen := 0, []int{}
	gruppe, anzahl := map[string]int{}, map[string]int{}
	for _, p := range lv.Positionen {
		pos[p.Bereich] = append(pos[p.Bereich], p)
		teile := strings.Split(p.OZ, ".")
		if tiefe == 0 {
			tiefe, laengen = len(teile), make([]int, len(teile))
		} else if len(teile) != tiefe {
			return nil, fmt.Errorf("Position %s: Gliederungstiefe weicht ab", p.OZ)
		}
		for i, t := range teile {
			laengen[i] = max(laengen[i], len(t))
		}
		switch p.Art {
		case ArtGrund:
			gruppe[p.OZ] = len(gruppe) + 1
		case ArtAlternativ:
			if gruppe[p.Zu] == 0 {
				return nil, fmt.Errorf("Position %s: Grundposition %s fehlt", p.OZ, p.Zu)
			}
			anzahl[p.Zu]++
			if anzahl[p.Zu] > 9 {
				return nil, fmt.Errorf("Position %s: mehr als 9 Alternativen", p.OZ)
			}
		}
	}
	w := &x83Writer{remark: lv.Vorbemerkungen, gruppe: gruppe, serie: map[string]int{}}
	w.line(`<?xml version="1.0" encoding="UTF-8"?>`)
	w.line(`<GAEB xmlns="` + ns + `">`)
	w.ind++
	w.open("GAEBInfo")
	w.leaf("Version", strings.TrimPrefix(lv.Format, "da_xml_"))
	w.leaf("ProgSystem", "NalaERP3 Evaluationsset (synthetisch)")
	w.close("GAEBInfo")
	w.open("PrjInfo")
	w.leaf("NamePrj", strings.TrimSuffix(lv.Datei, ".x83"))
	w.leaf("LblPrj", "Synthetisches Leistungsverzeichnis")
	w.leaf("Cur", "EUR")
	w.close("PrjInfo")
	w.open("Award")
	w.leaf("DP", "83")
	w.line(`<BoQ ID="` + w.id("Q") + `">`)
	w.ind++
	w.open("BoQInfo")
	w.leaf("Name", strings.TrimSuffix(lv.Datei, ".x83"))
	for i, l := range laengen {
		typ := "BoQLevel"
		if i == tiefe-1 {
			typ = "Item"
		} else if i == 0 && len(kinder[""]) > 0 && kinder[""][0].Stufe == StufeLos {
			typ = "Lot"
		}
		w.line("<BoQBkdn><Type>" + typ + "</Type><Length>" + strconv.Itoa(l) + "</Length><Num>Yes</Num></BoQBkdn>")
	}
	w.close("BoQInfo")
	var body func(eltern string) error
	body = func(eltern string) error {
		w.open("BoQBody")
		if len(kinder[eltern]) > 0 && len(pos[eltern]) > 0 {
			return fmt.Errorf("Bereich %q enthaelt Bereiche und Positionen", eltern)
		}
		for _, b := range kinder[eltern] {
			teil, err := letzterTeil(b.OZ, eltern)
			if err != nil {
				return err
			}
			w.line(`<BoQCtgy ID="` + w.id("C") + `" RNoPart="` + w.esc(teil) + `">`)
			w.ind++
			w.absatz("LblTx", b.Bezeichnung)
			if err := body(b.OZ); err != nil {
				return err
			}
			w.close("BoQCtgy")
		}
		if len(pos[eltern]) > 0 {
			w.open("Itemlist")
			for _, r := range w.remark {
				w.line(`<Remark ID="` + w.id("R") + `">`)
				w.ind++
				w.open("Description")
				w.open("CompleteText")
				w.open("DetailTxt")
				w.absatz("Text", r)
				w.close("DetailTxt")
				w.close("CompleteText")
				w.close("Description")
				w.close("Remark")
			}
			w.remark = nil
			for _, p := range pos[eltern] {
				if err := w.item(p, eltern); err != nil {
					return err
				}
			}
			w.close("Itemlist")
		}
		w.close("BoQBody")
		return nil
	}
	if err := body(""); err != nil {
		return nil, err
	}
	w.close("BoQ")
	w.close("Award")
	w.close("GAEB")
	return w.b.Bytes(), nil
}

func (w *x83Writer) item(p ErwartetePosition, eltern string) error {
	teil, err := letzterTeil(p.OZ, eltern)
	if err != nil {
		return err
	}
	w.line(`<Item ID="` + w.id("I") + `" RNoPart="` + w.esc(teil) + `">`)
	w.ind++
	w.leaf("Qty", strconv.FormatFloat(p.Menge, 'f', 3, 64))
	w.leaf("QU", p.Einheit)
	switch p.Art {
	case ArtBedarfMitGB:
		w.leaf("Provis", "WithTotal")
	case ArtBedarfOhneGB:
		w.leaf("Provis", "WithoutTotal")
	case ArtGrund:
		w.leaf("ALNGroupNo", strconv.Itoa(w.gruppe[p.OZ]))
		w.leaf("ALNSerNo", "0")
	case ArtAlternativ:
		w.serie[p.Zu]++
		w.leaf("ALNGroupNo", strconv.Itoa(w.gruppe[p.Zu]))
		w.leaf("ALNSerNo", strconv.Itoa(w.serie[p.Zu]))
	}
	if p.Pauschal {
		w.leaf("LumpSumItem", "Yes")
	}
	w.open("Description")
	w.open("CompleteText")
	if p.Langtext != "" {
		w.open("DetailTxt")
		w.absatz("Text", p.Langtext)
		w.close("DetailTxt")
	}
	w.open("OutlineText")
	w.open("OutlTxt")
	w.absatz("TextOutlTxt", p.Kurztext)
	w.close("OutlTxt")
	w.close("OutlineText")
	w.close("CompleteText")
	w.close("Description")
	w.close("Item")
	return nil
}
