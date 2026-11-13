// Package gaeb liest Leistungsverzeichnisse im Format GAEB DA XML 3.x
// (Backlog I.2, ADR 0040). Der Parser arbeitet deterministisch, ohne
// Datenbank und ohne Sprachmodell (aufgabe.md §4). Unterstuetzt ist die
// Angebotsaufforderung (DP 83) in DA XML 3.2 und 3.3.
//
// Die Elementnamen folgen dem Korpus aus I.1 (eval/x83.go): belegt sind
// Provis und LumpSumItem, die uebrigen werden gegen eine echte X83 oder
// das Schema geprueft. Gelesen werden nur lokale Namen; der Namespace
// liefert hoechstens die Version.
package gaeb

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Positionsarten und Stufen; gleiche Werte wie im Evaluationsset.
const (
	ArtNormal       = "normal"
	ArtGrund        = "grundposition"
	ArtAlternativ   = "alternativposition"
	ArtBedarfMitGB  = "bedarfsposition_mit_gb"
	ArtBedarfOhneGB = "bedarfsposition_ohne_gb"
	StufeLos        = "los"
	StufeTitel      = "titel"
	StufeUntertitel = "untertitel"
)

// Grenzen (ADR 0040, Entscheidung 6): groessere Dateien sind ein Fehler,
// nichts wird gekuerzt.
const (
	MaxPositionen  = 20000
	MaxTextZeichen = 20000
	MaxTiefe       = 5 // Stufen von Bereichen ueber den Positionen
)

var versionen = map[string]bool{"3.2": true, "3.3": true}

// Bereich ist ein Los, Titel oder Untertitel; Eltern ist die OZ des
// uebergeordneten Bereichs (leer auf oberster Ebene).
type Bereich struct {
	OZ          string
	Stufe       string
	Bezeichnung string
	Eltern      string
}

// Position ist eine Position des LV; Zu ist bei Alternativpositionen die
// OZ der Grundposition.
type Position struct {
	OZ       string
	Bereich  string
	Kurztext string
	Langtext string
	Menge    float64
	Einheit  string
	Art      string
	Pauschal bool
	Zu       string
}

// LV ist das Ergebnis des Parsers.
type LV struct {
	Format         string // z. B. "da_xml_3.3"
	Phase          string // z. B. "X83"
	Vorbemerkungen []string
	Bereiche       []Bereich
	Positionen     []Position
}

type xText struct {
	Inner []byte `xml:",innerxml"`
}

type xItem struct {
	RNoPart     string `xml:"RNoPart,attr"`
	Qty         string `xml:"Qty"`
	QU          string `xml:"QU"`
	Provis      string `xml:"Provis"`
	ALNGroupNo  string `xml:"ALNGroupNo"`
	ALNSerNo    string `xml:"ALNSerNo"`
	LumpSumItem string `xml:"LumpSumItem"`
	Langtext    xText  `xml:"Description>CompleteText>DetailTxt>Text"`
	Kurztext    xText  `xml:"Description>CompleteText>OutlineText>OutlTxt>TextOutlTxt"`
}

type xItemlist struct {
	Remarks []xText `xml:"Remark>Description>CompleteText>DetailTxt>Text"`
	Items   []xItem `xml:"Item"`
}

type xBody struct {
	Ctgys     []xCtgy     `xml:"BoQCtgy"`
	Itemlists []xItemlist `xml:"Itemlist"`
}

type xCtgy struct {
	RNoPart string `xml:"RNoPart,attr"`
	LblTx   xText  `xml:"LblTx"`
	Body    xBody  `xml:"BoQBody"`
}

type xBoQ struct {
	Bkdn []string `xml:"BoQInfo>BoQBkdn>Type"`
	Body xBody    `xml:"BoQBody"`
}

type xGAEB struct {
	XMLName xml.Name
	Version string `xml:"GAEBInfo>Version"`
	DP      string `xml:"Award>DP"`
	BoQs    []xBoQ `xml:"Award>BoQ"`
}

// absaetze liefert den Text eines Textelements, je Absatz (<p>) eine Zeile.
func absaetze(t xText) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(t.Inner))
	var zeilen []string
	var zeile strings.Builder
	inP := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "p" {
				inP = true
			}
		case xml.EndElement:
			if tok.Name.Local == "p" {
				inP = false
				zeilen = append(zeilen, zeile.String())
				zeile.Reset()
			}
		case xml.CharData:
			if inP {
				zeile.Write(tok)
			} else if s := strings.TrimSpace(string(tok)); s != "" {
				zeilen = append(zeilen, s)
			}
		}
	}
	return strings.Join(zeilen, "\n"), nil
}

type parser struct {
	lv     *LV
	lose   bool              // oberste Stufe sind Lose
	gruppe map[string]string // ALNGroupNo -> OZ der Grundposition
	oz     map[string]bool   // vergebene OZ von Bereichen und Positionen
}

// text liest ein Textelement und prueft die Laenge.
func (p *parser) text(t xText, was string) (string, error) {
	s, err := absaetze(t)
	if err != nil {
		return "", fmt.Errorf("%s: %w", was, err)
	}
	if utf8.RuneCountInString(s) > MaxTextZeichen {
		return "", fmt.Errorf("%s ist länger als %d Zeichen", was, MaxTextZeichen)
	}
	return s, nil
}

// neueOZ bildet die OZ aus Eltern und RNoPart und prueft, dass sie
// vollstaendig und eindeutig ist.
func (p *parser) neueOZ(eltern, rno string) (string, error) {
	rno = strings.TrimSpace(rno)
	oz := verbinde(eltern, rno)
	if rno == "" {
		return "", fmt.Errorf("Ordnungszahl unter %q fehlt", eltern)
	}
	if p.oz[oz] {
		return "", fmt.Errorf("Ordnungszahl %s ist doppelt", oz)
	}
	p.oz[oz] = true
	return oz, nil
}

func (p *parser) stufe(tiefe int) string {
	if p.lose {
		tiefe--
	}
	switch {
	case tiefe < 0:
		return StufeLos
	case tiefe == 0:
		return StufeTitel
	default:
		return StufeUntertitel
	}
}

func verbinde(eltern, teil string) string {
	if eltern == "" {
		return teil
	}
	return eltern + "." + teil
}

func (p *parser) body(b xBody, eltern string, tiefe int) error {
	if len(b.Ctgys) > 0 && len(b.Itemlists) > 0 {
		return fmt.Errorf("Bereich %q enthält Bereiche und Positionen", eltern)
	}
	if len(b.Ctgys) > 0 && tiefe >= MaxTiefe {
		return fmt.Errorf("Gliederung unter %s ist tiefer als %d Stufen", eltern, MaxTiefe)
	}
	for _, c := range b.Ctgys {
		oz, err := p.neueOZ(eltern, c.RNoPart)
		if err != nil {
			return err
		}
		bez, err := p.text(c.LblTx, "Bezeichnung von Bereich "+oz)
		if err != nil {
			return err
		}
		p.lv.Bereiche = append(p.lv.Bereiche, Bereich{OZ: oz, Stufe: p.stufe(tiefe), Bezeichnung: bez, Eltern: eltern})
		if err := p.body(c.Body, oz, tiefe+1); err != nil {
			return err
		}
	}
	for _, il := range b.Itemlists {
		for _, r := range il.Remarks {
			s, err := p.text(r, "Vorbemerkung")
			if err != nil {
				return err
			}
			p.lv.Vorbemerkungen = append(p.lv.Vorbemerkungen, s)
		}
		for _, it := range il.Items {
			if err := p.item(it, eltern); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *parser) item(it xItem, eltern string) error {
	if len(p.lv.Positionen) >= MaxPositionen {
		return fmt.Errorf("mehr als %d Positionen", MaxPositionen)
	}
	oz, err := p.neueOZ(eltern, it.RNoPart)
	if err != nil {
		return err
	}
	pos := Position{OZ: oz, Bereich: eltern, Einheit: strings.TrimSpace(it.QU), Art: ArtNormal}
	fehler := func(format string, a ...any) error {
		return fmt.Errorf("Position %s: %s", pos.OZ, fmt.Sprintf(format, a...))
	}
	if pos.Einheit == "" {
		return fehler("Einheit fehlt")
	}
	if q := strings.TrimSpace(it.Qty); q != "" {
		m, err := strconv.ParseFloat(q, 64)
		if err != nil || math.IsNaN(m) || math.IsInf(m, 0) || m < 0 {
			return fehler("Menge %q ist keine Zahl ab 0", q)
		}
		pos.Menge = m
	}
	switch strings.TrimSpace(it.Provis) {
	case "":
	case "WithTotal":
		pos.Art = ArtBedarfMitGB
	case "WithoutTotal":
		pos.Art = ArtBedarfOhneGB
	default:
		return fehler("unbekannte Bedarfsposition %q", it.Provis)
	}
	gruppe, serie := strings.TrimSpace(it.ALNGroupNo), strings.TrimSpace(it.ALNSerNo)
	if gruppe != "" || serie != "" {
		if pos.Art != ArtNormal {
			return fehler("Bedarfs- und Wahlposition zugleich")
		}
		n, err := strconv.Atoi(serie)
		if gruppe == "" || err != nil || n < 0 {
			return fehler("Zuordnung %q/%q ist unvollständig", gruppe, serie)
		}
		if n == 0 {
			if p.gruppe[gruppe] != "" {
				return fehler("Gruppe %s hat schon die Grundposition %s", gruppe, p.gruppe[gruppe])
			}
			pos.Art = ArtGrund
			p.gruppe[gruppe] = pos.OZ
		} else {
			if p.gruppe[gruppe] == "" {
				return fehler("Grundposition der Gruppe %s fehlt", gruppe)
			}
			pos.Art, pos.Zu = ArtAlternativ, p.gruppe[gruppe]
		}
	}
	switch strings.TrimSpace(it.LumpSumItem) {
	case "", "No":
	case "Yes":
		pos.Pauschal = true
	default:
		return fehler("unbekannte Pauschale %q", it.LumpSumItem)
	}
	if pos.Kurztext, err = p.text(it.Kurztext, "Position "+oz+": Kurztext"); err != nil {
		return err
	}
	if strings.TrimSpace(pos.Kurztext) == "" {
		return fehler("Kurztext fehlt")
	}
	if pos.Langtext, err = p.text(it.Langtext, "Position "+oz+": Langtext"); err != nil {
		return err
	}
	p.lv.Positionen = append(p.lv.Positionen, pos)
	return nil
}

// Parse liest ein LV in GAEB DA XML. Unbekannte Elemente werden
// uebergangen; ein unbekannter Wert in einem gelesenen Element ist ein
// Fehler.
func Parse(r io.Reader) (*LV, error) {
	var g xGAEB
	if err := xml.NewDecoder(r).Decode(&g); err != nil {
		return nil, fmt.Errorf("kein lesbares XML: %w", err)
	}
	if g.XMLName.Local != "GAEB" {
		return nil, fmt.Errorf("Wurzelelement %q ist nicht GAEB", g.XMLName.Local)
	}
	version := strings.TrimSpace(g.Version)
	if version == "" {
		version = g.XMLName.Space[strings.LastIndex(g.XMLName.Space, "/")+1:]
	}
	if !versionen[version] {
		return nil, fmt.Errorf("DA XML %q wird nicht unterstützt", version)
	}
	if dp := strings.TrimSpace(g.DP); dp != "83" {
		return nil, fmt.Errorf("Phase %q wird nicht unterstützt", dp)
	}
	if len(g.BoQs) != 1 {
		return nil, fmt.Errorf("%d Leistungsverzeichnisse statt einem", len(g.BoQs))
	}
	boq := g.BoQs[0]
	p := &parser{
		lv:     &LV{Format: "da_xml_" + version, Phase: "X83", Vorbemerkungen: []string{}, Bereiche: []Bereich{}, Positionen: []Position{}},
		lose:   len(boq.Bkdn) > 0 && strings.TrimSpace(boq.Bkdn[0]) == "Lot",
		gruppe: map[string]string{},
		oz:     map[string]bool{},
	}
	if err := p.body(boq.Body, "", 0); err != nil {
		return nil, err
	}
	if len(p.lv.Positionen) == 0 {
		return nil, fmt.Errorf("LV ohne Positionen")
	}
	return p.lv, nil
}
