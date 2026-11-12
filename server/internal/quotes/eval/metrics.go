package eval

import (
	"math"
	"strings"
)

// Zielwerte als Bedingung fuer den Einsatz des Matchings (ADR 0039, 4 (a)).
const (
	MaxFehlzuordnungsrate = 0.02
	MinTrefferquote       = 0.70
	uWertToleranz         = 0.005
)

// Merkmale der Systemvorgaben in fester Reihenfolge fuer den Bericht.
var Merkmale = []string{"profilserie", "verglasung", "rc_klasse", "u_wert", "brandschutzklasse"}

// Vorschlag eines Matchers je Position: Code leer = keine Zuordnung.
type Vorschlag struct {
	Code           string
	Score          float64
	Begruendung    string
	Systemvorgaben Systemvorgaben
}

// MerkmalZahlen: Erwartet = Positionen mit Vorgabe, Erkannt = Positionen
// mit erkannter Angabe, Richtig = erkannte Angabe stimmt.
type MerkmalZahlen struct {
	Erwartet, Erkannt, Richtig int
}

type Ergebnis struct {
	Positionen  int
	MitZiel     int
	Automatisch int
	Richtig     int
	Falsch      int
	Pruefliste  int
	Merkmale    map[string]MerkmalZahlen
}

func quote(z, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(z) / float64(n)
}

// Trefferquote: richtig automatisch zugeordnet / Positionen mit Ziel.
func (e Ergebnis) Trefferquote() float64 { return quote(e.Richtig, e.MitZiel) }

// Fehlzuordnungsrate: falsch automatisch zugeordnet / automatisch zugeordnet.
func (e Ergebnis) Fehlzuordnungsrate() float64 { return quote(e.Falsch, e.Automatisch) }

// Prueflistenquote: nicht automatisch zugeordnet / alle Positionen.
func (e Ergebnis) Prueflistenquote() float64 { return quote(e.Pruefliste, e.Positionen) }

// Genauigkeit und Vollstaendigkeit je Merkmal.
func (m MerkmalZahlen) Genauigkeit() float64      { return quote(m.Richtig, m.Erkannt) }
func (m MerkmalZahlen) Vollstaendigkeit() float64 { return quote(m.Richtig, m.Erwartet) }

// ZielErreicht: beide Zielwerte; ohne Positionen mit Ziel ist die
// Trefferquote 0 und das Ziel damit verfehlt.
func (e Ergebnis) ZielErreicht() bool {
	return e.Fehlzuordnungsrate() <= MaxFehlzuordnungsrate && e.Trefferquote() >= MinTrefferquote
}

func norm(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// merkmal liefert je Merkmal, ob eine Angabe da ist und ob sie gleich ist.
func merkmal(name string, erwartet, erkannt Systemvorgaben) (hatErwartet, hatErkannt, gleich bool) {
	if name == "u_wert" {
		hatErwartet, hatErkannt = erwartet.UWert != nil, erkannt.UWert != nil
		return hatErwartet, hatErkannt, hatErwartet && hatErkannt && math.Abs(*erwartet.UWert-*erkannt.UWert) <= uWertToleranz
	}
	text := func(s Systemvorgaben) string {
		switch name {
		case "profilserie":
			return norm(s.Profilserie)
		case "verglasung":
			return norm(s.Verglasung)
		case "rc_klasse":
			return norm(s.RCKlasse)
		default:
			return norm(s.Brandschutzklasse)
		}
	}
	a, b := text(erwartet), text(erkannt)
	return a != "", b != "", a != "" && a == b
}

// Auswerten vergleicht die Vorschlaege (je OZ) mit der Erwartung. Eine
// Zuordnung ist automatisch bei Code mit Score >= schwelle; ohne Vorschlag
// landet die Position in der Pruefliste.
func Auswerten(lv *ErwartetesLV, vorschlaege map[string]Vorschlag, schwelle float64) Ergebnis {
	e := Ergebnis{Merkmale: map[string]MerkmalZahlen{}}
	for _, p := range lv.Positionen {
		e.Positionen++
		if p.Leistung != "" {
			e.MitZiel++
		}
		v, ok := vorschlaege[p.OZ]
		switch {
		case !ok || v.Code == "" || v.Score < schwelle:
			e.Pruefliste++
		case v.Code == p.Leistung:
			e.Automatisch++
			e.Richtig++
		default:
			e.Automatisch++
			e.Falsch++
		}
		for _, name := range Merkmale {
			hatErwartet, hatErkannt, gleich := merkmal(name, p.Systemvorgaben, v.Systemvorgaben)
			m := e.Merkmale[name]
			if hatErwartet {
				m.Erwartet++
			}
			if hatErkannt {
				m.Erkannt++
			}
			if gleich {
				m.Richtig++
			}
			e.Merkmale[name] = m
		}
	}
	return e
}

// Summe addiert Ergebnisse mehrerer LVs.
func Summe(list ...Ergebnis) Ergebnis {
	out := Ergebnis{Merkmale: map[string]MerkmalZahlen{}}
	for _, e := range list {
		out.Positionen += e.Positionen
		out.MitZiel += e.MitZiel
		out.Automatisch += e.Automatisch
		out.Richtig += e.Richtig
		out.Falsch += e.Falsch
		out.Pruefliste += e.Pruefliste
		for name, m := range e.Merkmale {
			o := out.Merkmale[name]
			o.Erwartet += m.Erwartet
			o.Erkannt += m.Erkannt
			o.Richtig += m.Richtig
			out.Merkmale[name] = o
		}
	}
	return out
}
