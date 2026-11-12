// Package eval enthaelt das Evaluationsset fuer das GAEB-Matching
// (Backlog I.1, ADR 0039): synthetischer Leistungskatalog, erwartete
// Ergebnisse je LV und die Kennzahlen. Kein Produktivcode, kein
// Modellzugriff; Testdaten nur synthetisch unter testdata/.
package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Positionsarten der Erwartung; Eventualposition = Bedarfsposition.
const (
	ArtNormal            = "normal"
	ArtGrund             = "grundposition"
	ArtAlternativ        = "alternativposition"
	ArtBedarfMitGB       = "bedarfsposition_mit_gb"
	ArtBedarfOhneGB      = "bedarfsposition_ohne_gb"
	StufeLos             = "los"
	StufeTitel           = "titel"
	StufeUntertitel      = "untertitel"
	maxTextRunes         = 20000
	maxSystemvorgabeText = 200
)

var validArten = map[string]bool{ArtNormal: true, ArtGrund: true, ArtAlternativ: true, ArtBedarfMitGB: true, ArtBedarfOhneGB: true}
var validStufen = map[string]bool{StufeLos: true, StufeTitel: true, StufeUntertitel: true}

// Systemvorgaben nach aufgabe.md §4; leer bzw. nil = nicht vorgegeben.
type Systemvorgaben struct {
	Profilserie       string   `json:"profilserie,omitempty"`
	Verglasung        string   `json:"verglasung,omitempty"`
	RCKlasse          string   `json:"rc_klasse,omitempty"`
	UWert             *float64 `json:"u_wert,omitempty"`
	Brandschutzklasse string   `json:"brandschutzklasse,omitempty"`
}

// Leistung im synthetischen Katalog mit festem Code.
type Leistung struct {
	Code           string         `json:"code"`
	Kurztext       string         `json:"kurztext"`
	Einheit        string         `json:"einheit"`
	Gewerk         string         `json:"gewerk"`
	Systemvorgaben Systemvorgaben `json:"systemvorgaben"`
}

type Katalog struct {
	Leistungen []Leistung `json:"leistungen"`
}

// Bereich: Los, Titel oder Untertitel; Eltern = OZ des uebergeordneten
// Bereichs (leer auf oberster Ebene).
type Bereich struct {
	OZ          string `json:"oz"`
	Stufe       string `json:"stufe"`
	Bezeichnung string `json:"bezeichnung"`
	Eltern      string `json:"eltern,omitempty"`
}

// ErwartetePosition: erwartetes Parse-Ergebnis (Prueforakel fuer I.2) und
// erwartete Zuordnung. Leistung leer = kein passendes Ziel im Katalog.
type ErwartetePosition struct {
	OZ             string         `json:"oz"`
	Bereich        string         `json:"bereich"`
	Kurztext       string         `json:"kurztext"`
	Langtext       string         `json:"langtext"`
	Menge          float64        `json:"menge"`
	Einheit        string         `json:"einheit"`
	Art            string         `json:"art"`
	Pauschal       bool           `json:"pauschal,omitempty"`
	Zu             string         `json:"zu,omitempty"` // Alternativposition: OZ der Grundposition
	Leistung       string         `json:"leistung"`
	Systemvorgaben Systemvorgaben `json:"systemvorgaben"`
}

type ErwartetesLV struct {
	Datei          string              `json:"datei"`
	Format         string              `json:"format"`
	Phase          string              `json:"phase"`
	Vorbemerkungen []string            `json:"vorbemerkungen"`
	Bereiche       []Bereich           `json:"bereiche"`
	Positionen     []ErwartetePosition `json:"positionen"`
}

func decodeStrict(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// LoadKatalog liest und prueft den Katalog; unbekannte Felder sind Fehler.
func LoadKatalog(path string) (*Katalog, error) {
	var k Katalog
	if err := decodeStrict(path, &k); err != nil {
		return nil, err
	}
	if err := k.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &k, nil
}

// LoadLV liest und prueft eine Erwartung gegen den Katalog.
func LoadLV(path string, k *Katalog) (*ErwartetesLV, error) {
	var lv ErwartetesLV
	if err := decodeStrict(path, &lv); err != nil {
		return nil, err
	}
	if err := lv.Validate(k); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &lv, nil
}

func (s Systemvorgaben) validate() error {
	for _, v := range []string{s.Profilserie, s.Verglasung, s.RCKlasse, s.Brandschutzklasse} {
		if len([]rune(v)) > maxSystemvorgabeText {
			return fmt.Errorf("Systemvorgabe ist zu lang")
		}
	}
	if s.UWert != nil && (*s.UWert <= 0 || *s.UWert > 10) {
		return fmt.Errorf("U-Wert muss zwischen 0 und 10 liegen")
	}
	return nil
}

func (k *Katalog) Validate() error {
	if len(k.Leistungen) == 0 {
		return fmt.Errorf("Katalog ist leer")
	}
	seen := map[string]bool{}
	for i, l := range k.Leistungen {
		code := strings.TrimSpace(l.Code)
		switch {
		case code == "" || code != l.Code:
			return fmt.Errorf("Leistung %d: Code fehlt oder hat Leerraum", i+1)
		case seen[code]:
			return fmt.Errorf("Leistung %s ist doppelt", code)
		case strings.TrimSpace(l.Kurztext) == "" || strings.TrimSpace(l.Einheit) == "":
			return fmt.Errorf("Leistung %s: Kurztext und Einheit sind Pflicht", code)
		}
		if err := l.Systemvorgaben.validate(); err != nil {
			return fmt.Errorf("Leistung %s: %w", code, err)
		}
		seen[code] = true
	}
	return nil
}

// Code liefert, ob der Katalog den Code kennt.
func (k *Katalog) Code(code string) bool {
	for _, l := range k.Leistungen {
		if l.Code == code {
			return true
		}
	}
	return false
}

func (lv *ErwartetesLV) Validate(k *Katalog) error {
	if strings.TrimSpace(lv.Datei) == "" || strings.TrimSpace(lv.Format) == "" || strings.TrimSpace(lv.Phase) == "" {
		return fmt.Errorf("Datei, Format und Phase sind Pflicht")
	}
	bereiche := map[string]bool{}
	for _, b := range lv.Bereiche {
		switch {
		case strings.TrimSpace(b.OZ) == "":
			return fmt.Errorf("Bereich ohne OZ")
		case bereiche[b.OZ]:
			return fmt.Errorf("Bereich %s ist doppelt", b.OZ)
		case !validStufen[b.Stufe]:
			return fmt.Errorf("Bereich %s: unbekannte Stufe %q", b.OZ, b.Stufe)
		case b.Eltern != "" && !bereiche[b.Eltern]:
			return fmt.Errorf("Bereich %s: Eltern %s fehlt oder steht später", b.OZ, b.Eltern)
		}
		bereiche[b.OZ] = true
	}
	if len(lv.Positionen) == 0 {
		return fmt.Errorf("LV ohne Positionen")
	}
	oz := map[string]string{}
	for _, p := range lv.Positionen {
		switch {
		case strings.TrimSpace(p.OZ) == "":
			return fmt.Errorf("Position ohne OZ")
		case oz[p.OZ] != "":
			return fmt.Errorf("Position %s ist doppelt", p.OZ)
		case p.Bereich != "" && !bereiche[p.Bereich]:
			return fmt.Errorf("Position %s: Bereich %s fehlt", p.OZ, p.Bereich)
		case strings.TrimSpace(p.Kurztext) == "" || strings.TrimSpace(p.Einheit) == "":
			return fmt.Errorf("Position %s: Kurztext und Einheit sind Pflicht", p.OZ)
		case len([]rune(p.Langtext)) > maxTextRunes:
			return fmt.Errorf("Position %s: Langtext ist zu lang", p.OZ)
		case p.Menge < 0:
			return fmt.Errorf("Position %s: Menge ist negativ", p.OZ)
		case !validArten[p.Art]:
			return fmt.Errorf("Position %s: unbekannte Art %q", p.OZ, p.Art)
		case p.Art == ArtAlternativ && oz[p.Zu] != ArtGrund:
			return fmt.Errorf("Position %s: Alternativposition ohne vorherige Grundposition", p.OZ)
		case p.Art != ArtAlternativ && p.Zu != "":
			return fmt.Errorf("Position %s: nur Alternativpositionen verweisen auf eine Grundposition", p.OZ)
		case p.Leistung != "" && !k.Code(p.Leistung):
			return fmt.Errorf("Position %s: Leistung %s fehlt im Katalog", p.OZ, p.Leistung)
		}
		if err := p.Systemvorgaben.validate(); err != nil {
			return fmt.Errorf("Position %s: %w", p.OZ, err)
		}
		oz[p.OZ] = p.Art
	}
	return nil
}
