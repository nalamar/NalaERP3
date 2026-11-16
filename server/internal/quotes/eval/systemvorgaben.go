package eval

import (
	"regexp"
	"strconv"
	"strings"
)

// Deterministische Erkennung der Systemvorgaben mit eindeutigem Muster
// (ADR 0044 Frage 5): RC-Klasse, U-Wert und Brandschutzklasse. Profilserie
// und Verglasung bleiben dem Modell. Abstaende in den Mustern nur Leerzeichen
// oder Tab: kein Treffer ueber eine Zeile hinweg.

var (
	// RC 1 bis 6, auch "RC-2", "RC 2" und die alte Bezeichnung "WK2".
	reRC = regexp.MustCompile(`\b(?:RC|WK)[ \t]*-?[ \t]*([1-6])\b`)
	// U-Wert des ganzen Elements (Fenster Uw, Tuer Ud, Fassade Ucw oder
	// "U-Wert"), nicht der des Glases (Ug) oder Rahmens (Uf).
	reUWert = regexp.MustCompile(`\b(?:Uw|Ud|Ucw|U)-?Wert\b|\b(?:Uw|Ud|Ucw)\b`)
	// nach dem Namen: "Fenster"/"Tür", Doppelpunkt, Vergleich, dann die Zahl
	reUZahl = regexp.MustCompile(`^[ \t]*(?:(?:Fenster|Tür)[ \t]*)?:?[ \t]*(?:max\.|≤|<=|<|=|kleiner gleich|kleiner als)?[ \t]*(\d+(?:[.,]\d+)?)`)
	// Tueren T30/T60/T90 (DIN 4102), Verglasungen F/G, europaeisch EI/E
	reTuer       = regexp.MustCompile(`\bT[ \t]?(30|60|90)\b`)
	reEuropa     = regexp.MustCompile(`\bE(I2?|W)?[ \t]?(15|20|30|45|60|90|120)\b`)
	reVerglasung = regexp.MustCompile(`\b([FG])[ \t]?(30|60|90|120)\b`)
	reRauch      = regexp.MustCompile(`\bRS\b`)
)

// ErkenneSystemvorgaben liest RC-Klasse, U-Wert und Brandschutzklasse aus
// Kurz- und Langtext. Nicht Erkanntes bleibt leer.
func ErkenneSystemvorgaben(kurztext, langtext string) Systemvorgaben {
	text := kurztext + "\n" + langtext
	var s Systemvorgaben
	if m := reRC.FindStringSubmatch(text); m != nil {
		s.RCKlasse = "RC" + m[1]
	}
	s.UWert = erkenneUWert(text)
	s.Brandschutzklasse = erkenneBrandschutz(text)
	return s
}

func erkenneUWert(text string) *float64 {
	for _, loc := range reUWert.FindAllStringIndex(text, -1) {
		m := reUZahl.FindStringSubmatch(text[loc[1]:])
		if m == nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
		if err == nil && v > 0 && v <= 10 {
			return &v
		}
	}
	return nil
}

// Reihenfolge: Tuerklasse, europaeische Klasse, Verglasung, Rauchschutz.
// "Serie F60" oder "Profilserie F60" ist eine Profilserie, keine
// Brandschutzklasse.
func erkenneBrandschutz(text string) string {
	if m := reTuer.FindStringSubmatch(text); m != nil {
		return "T" + m[1]
	}
	if m := reEuropa.FindStringSubmatch(text); m != nil {
		return "E" + m[1] + m[2]
	}
	for _, loc := range reVerglasung.FindAllStringSubmatchIndex(text, -1) {
		if strings.HasSuffix(strings.ToLower(strings.TrimRight(text[:loc[0]], " ")), "serie") {
			continue
		}
		return text[loc[2]:loc[3]] + text[loc[4]:loc[5]]
	}
	if reRauch.MatchString(text) {
		return "RS"
	}
	return ""
}
