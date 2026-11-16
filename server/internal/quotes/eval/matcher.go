package eval

import (
	"sort"
	"strings"
	"unicode"

	"nalaerp3/internal/quotes/gaeb"
)

// Position ist das, was ein Matcher von einer LV-Position sieht: nur
// Angaben aus dem LV, nie die erwartete Leistung oder Systemvorgaben.
type Position struct {
	OZ       string
	Kurztext string
	Langtext string
	Menge    float64
	Einheit  string
	Art      string
	Pauschal bool
}

// Eingabe macht aus einer Position des Parsers die Eingabe fuer einen
// Matcher (I.2.1c: die Positionen kommen aus der X83, nicht aus der
// Erwartung).
func Eingabe(p gaeb.Position) Position {
	return Position{OZ: p.OZ, Kurztext: p.Kurztext, Langtext: p.Langtext, Menge: p.Menge,
		Einheit: p.Einheit, Art: p.Art, Pauschal: p.Pauschal}
}

// Matcher schlaegt je Position eine Leistung des Katalogs vor (ADR 0039).
// Ein leerer Code bedeutet: keine Zuordnung.
type Matcher interface {
	Name() string
	Vorschlagen(p Position, k *Katalog) Vorschlag
}

// NieZuordnen ist die untere Referenz: alles landet in der Pruefliste.
type NieZuordnen struct{}

func (NieZuordnen) Name() string { return "nie-zuordnen" }

func (NieZuordnen) Vorschlagen(Position, *Katalog) Vorschlag {
	return Vorschlag{Begruendung: "ordnet nie zu"}
}

// Fuellwoerter ohne Aussage fuer die Zuordnung.
var stoppwoerter = map[string]bool{
	"und": true, "oder": true, "mit": true, "ohne": true, "aus": true, "für": true, "fuer": true,
	"der": true, "die": true, "das": true, "den": true, "dem": true, "des": true, "ein": true, "eine": true,
	"wie": true, "vor": true, "jedoch": true, "nur": true, "auf": true, "über": true, "von": true, "zum": true, "zur": true,
	"liefern": true, "montieren": true, "pos": true,
}

// woerter zerlegt Text in kleingeschriebene Woerter aus Buchstaben und
// Ziffern; kuerzer als 3 Zeichen und Fuellwoerter fallen weg.
func woerter(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(w)) >= 3 && !stoppwoerter[w] {
			out[w] = true
		}
	}
	return out
}

// Wortabgleich ist die einfache deterministische Referenz: Kurztext der
// Position gegen Kurztext der Leistung, Score = Dice-Koeffizient der
// Wortmengen. Bei Gleichstand gewinnt die fruehere Leistung im Katalog.
type Wortabgleich struct{}

func (Wortabgleich) Name() string { return "wortabgleich" }

func (Wortabgleich) Vorschlagen(p Position, k *Katalog) Vorschlag {
	pw := woerter(p.Kurztext)
	best, bestScore, bestGemeinsam := "", 0.0, []string(nil)
	for _, l := range k.Leistungen {
		lw := woerter(l.Kurztext)
		var gemeinsam []string
		for w := range pw {
			if lw[w] {
				gemeinsam = append(gemeinsam, w)
			}
		}
		score := 2 * float64(len(gemeinsam)) / float64(len(pw)+len(lw))
		if score > bestScore {
			best, bestScore, bestGemeinsam = l.Code, score, gemeinsam
		}
	}
	if best == "" {
		return Vorschlag{Begruendung: "kein gemeinsames Wort mit dem Katalog"}
	}
	sort.Strings(bestGemeinsam)
	return Vorschlag{Code: best, Score: bestScore, Begruendung: "gemeinsame Wörter: " + strings.Join(bestGemeinsam, ", ")}
}
