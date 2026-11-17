// Package matching ordnet LV-Positionen Leistungen eines Katalogs zu
// (ADR 0044, ADR 0045 Frage 7): deterministische Matcher, Erkennung der
// Systemvorgaben, Kandidaten-Vorauswahl und Matcher mit Sprachmodell. Jede
// Zuordnung ist ein Vorschlag mit Score und Begruendung; Preise gibt es
// hier nicht. Das Evaluationsset (internal/quotes/eval) misst diese Matcher.
package matching

import (
	"fmt"
	"strings"
)

// MaxSystemvorgabeText begrenzt jede Systemvorgabe als Text (Zeichen).
const MaxSystemvorgabeText = 200

// Systemvorgaben nach aufgabe.md §4; leer bzw. nil = nicht vorgegeben.
type Systemvorgaben struct {
	Profilserie       string   `json:"profilserie,omitempty"`
	Verglasung        string   `json:"verglasung,omitempty"`
	RCKlasse          string   `json:"rc_klasse,omitempty"`
	UWert             *float64 `json:"u_wert,omitempty"`
	Brandschutzklasse string   `json:"brandschutzklasse,omitempty"`
}

// Leistung im Katalog mit festem Code.
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

// Vorschlag eines Matchers je Position: Code leer = keine Zuordnung.
type Vorschlag struct {
	Code           string
	Score          float64
	Begruendung    string
	Systemvorgaben Systemvorgaben
}

// Validate prueft Laenge der Texte und Bereich des U-Werts.
func (s Systemvorgaben) Validate() error {
	for _, v := range []string{s.Profilserie, s.Verglasung, s.RCKlasse, s.Brandschutzklasse} {
		if len([]rune(v)) > MaxSystemvorgabeText {
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
		if err := l.Systemvorgaben.Validate(); err != nil {
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
