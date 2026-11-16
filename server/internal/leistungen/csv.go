package leistungen

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSV-Import (Backlog I.4.4b): erste Zeile = Spaltennamen wie im JSON
// (code, kurztext, langtext, einheit, gewerk, profilserie, verglasung,
// rc_klasse, u_wert, brandschutzklasse, aktiv), Pflicht sind code, kurztext
// und einheit. Trennzeichen Semikolon (Excel de-DE), Komma nur, wenn die
// Kopfzeile kein Semikolon enthaelt. U-Wert mit Komma oder Punkt; aktiv
// ja/nein, true/false, 1/0 oder leer (= unveraendert bzw. aktiv).

var csvSpalten = map[string]bool{
	"code": true, "kurztext": true, "langtext": true, "einheit": true, "gewerk": true, "profilserie": true,
	"verglasung": true, "rc_klasse": true, "u_wert": true, "brandschutzklasse": true, "aktiv": true,
}

// LeseCSV liest Leistungen aus einer CSV-Datei und liefert je Leistung die
// Zeile der Datei (fuer die Meldungen von Importieren). Leerzeilen werden
// uebersprungen. Fehler nennen die Zeile der Datei.
func LeseCSV(r io.Reader) ([]Eingabe, []int, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	kopf, _, _ := bytes.Cut(raw, []byte("\n"))
	cr := csv.NewReader(bytes.NewReader(raw))
	cr.Comma = ';'
	if !bytes.ContainsRune(kopf, ';') && bytes.ContainsRune(kopf, ',') {
		cr.Comma = ','
	}
	cr.FieldsPerRecord = -1
	namen, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, errors.New("CSV ungültig: Datei ist leer")
	}
	if err != nil {
		return nil, nil, csvFehler(err)
	}
	spalte := map[string]int{}
	for i, n := range namen {
		n = strings.ToLower(strings.TrimSpace(n))
		if !csvSpalten[n] {
			return nil, nil, fmt.Errorf("CSV ungültig: Zeile 1: unbekannte Spalte %q", n)
		}
		if _, doppelt := spalte[n]; doppelt {
			return nil, nil, fmt.Errorf("CSV ungültig: Zeile 1: Spalte %q doppelt", n)
		}
		spalte[n] = i
	}
	for _, pflicht := range []string{"code", "kurztext", "einheit"} {
		if _, ok := spalte[pflicht]; !ok {
			return nil, nil, fmt.Errorf("CSV ungültig: Zeile 1: Spalte %q fehlt", pflicht)
		}
	}
	var out []Eingabe
	var nummern []int
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, csvFehler(err)
		}
		zeile, _ := cr.FieldPos(0)
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue // Leerzeile
		}
		if len(rec) != len(namen) {
			return nil, nil, fmt.Errorf("CSV ungültig: Zeile %d: %d statt %d Spalten", zeile, len(rec), len(namen))
		}
		feld := func(n string) string {
			if i, ok := spalte[n]; ok {
				return rec[i]
			}
			return ""
		}
		in := Eingabe{Code: feld("code"), Kurztext: feld("kurztext"), Langtext: feld("langtext"), Einheit: feld("einheit"),
			Gewerk: feld("gewerk"), Profilserie: feld("profilserie"), Verglasung: feld("verglasung"), RCKlasse: feld("rc_klasse"),
			Brandschutzklasse: feld("brandschutzklasse")}
		if s := strings.TrimSpace(feld("u_wert")); s != "" {
			v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
			if err != nil {
				return nil, nil, fmt.Errorf("CSV ungültig: Zeile %d: U-Wert %q ist keine Zahl", zeile, s)
			}
			in.UWert = &v
		}
		switch strings.ToLower(strings.TrimSpace(feld("aktiv"))) {
		case "":
		case "ja", "true", "1", "x":
			an := true
			in.Aktiv = &an
		case "nein", "false", "0":
			aus := false
			in.Aktiv = &aus
		default:
			return nil, nil, fmt.Errorf("CSV ungültig: Zeile %d: aktiv %q (ja oder nein)", zeile, feld("aktiv"))
		}
		out = append(out, in)
		nummern = append(nummern, zeile)
	}
	return out, nummern, nil
}

func csvFehler(err error) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("CSV ungültig: Zeile %d: %v", pe.Line, pe.Err)
	}
	return fmt.Errorf("CSV ungültig: %v", err)
}
