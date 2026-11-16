package leistungen

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestLeseCSV(t *testing.T) {
	datei := "\xef\xbb\xbf Code ;KURZTEXT;einheit;langtext;u_wert;aktiv;rc_klasse;gewerk;profilserie;verglasung;brandschutzklasse\n" +
		"FE-1;\"Fenster; einflügelig\";Stk;\"Zeile 1\nZeile 2\";0,9;ja;RC2;fenster;A75;3-fach;T30\n" +
		"\n" +
		"FE-2;Fenster 2;Stk;;1.1;NEIN;;;;;\n" +
		"FE-3;Fenster 3;Stk;;;;;;;;\n" +
		"FE-4;Fenster 4;Stk;;;x;;;;;\n" +
		"FE-5;Fenster 5;Stk;;;0;;;;;\n" +
		"   \n" // nur Leerzeichen: übersprungen
	got, nummern, err := LeseCSV(strings.NewReader(datei))
	if err != nil || len(got) != 5 {
		t.Fatalf("%v %d", err, len(got))
	}
	a := got[0]
	if a.Code != "FE-1" || a.Kurztext != "Fenster; einflügelig" || a.Langtext != "Zeile 1\nZeile 2" || a.Einheit != "Stk" || a.UWert == nil ||
		*a.UWert != 0.9 || a.Aktiv == nil || !*a.Aktiv || a.RCKlasse != "RC2" || a.Gewerk != "fenster" || a.Profilserie != "A75" ||
		a.Verglasung != "3-fach" || a.Brandschutzklasse != "T30" {
		t.Fatalf("Zeile 2: %+v", a)
	}
	if b := got[1]; b.UWert == nil || *b.UWert != 1.1 || b.Aktiv == nil || *b.Aktiv {
		t.Fatalf("FE-2: %+v", b)
	}
	if c := got[2]; c.UWert != nil || c.Aktiv != nil {
		t.Fatalf("FE-3: %+v", c)
	}
	if !*got[3].Aktiv || *got[4].Aktiv {
		t.Fatalf("x/0: %v %v", *got[3].Aktiv, *got[4].Aktiv)
	}
	// Dateizeilen: Kopf 1, FE-1 ab 2 (zweizeiliger Langtext), Leerzeile 4
	if fmt.Sprint(nummern) != "[2 5 6 7 8]" {
		t.Fatalf("Zeilen: %v", nummern)
	}
	// Komma als Trennzeichen, nur Pflichtspalten
	got, nummern, err = LeseCSV(strings.NewReader("code,kurztext,einheit\r\nX,\"a, b\",m\r\n"))
	if err != nil || len(got) != 1 || got[0].Kurztext != "a, b" || got[0].Einheit != "m" || nummern[0] != 2 {
		t.Fatalf("Komma: %+v %v %v", got, nummern, err)
	}
	// Semikolon gewinnt, wenn beide vorkommen
	if got, _, err := LeseCSV(strings.NewReader("code;kurztext;einheit\nX;a,b;m\n")); err != nil || got[0].Kurztext != "a,b" {
		t.Fatalf("Semikolon: %+v %v", got, err)
	}
	// nur Kopfzeile: keine Leistungen, kein Fehler (Importieren meldet leer)
	if got, _, err := LeseCSV(strings.NewReader("code;kurztext;einheit\n")); err != nil || len(got) != 0 {
		t.Fatalf("nur Kopf: %+v %v", got, err)
	}
	for datei, want := range map[string]string{
		"":                                        "CSV ungültig: Datei ist leer",
		"code;kurztext;preis\n":                   `CSV ungültig: Zeile 1: unbekannte Spalte "preis"`,
		"code;kurztext;einheit;Code\n":            `CSV ungültig: Zeile 1: Spalte "code" doppelt`,
		"code;einheit\n":                          `CSV ungültig: Zeile 1: Spalte "kurztext" fehlt`,
		"kurztext;einheit\n":                      `CSV ungültig: Zeile 1: Spalte "code" fehlt`,
		"code;kurztext\n":                         `CSV ungültig: Zeile 1: Spalte "einheit" fehlt`,
		"code;kurztext;einheit,langtext\n":        `CSV ungültig: Zeile 1: unbekannte Spalte "einheit,langtext"`,
		"code;kurztext;einheit\nA;b\n":            "CSV ungültig: Zeile 2: 2 statt 3 Spalten",
		"code;kurztext;einheit\nA;b;c;d\n":        "CSV ungültig: Zeile 2: 4 statt 3 Spalten",
		"code;kurztext;einheit;u_wert\nA;b;c;x\n": `CSV ungültig: Zeile 2: U-Wert "x" ist keine Zahl`,
		"code;kurztext;einheit;aktiv\n\nA;b;c;vielleicht\n": `CSV ungültig: Zeile 3: aktiv "vielleicht" (ja oder nein)`,
		"code;kurztext;einheit\nA;\"b;c\n":                  "CSV ungültig: Zeile 2: extraneous or missing \" in quoted-field",
	} {
		if _, _, err := LeseCSV(strings.NewReader(datei)); err == nil || err.Error() != want {
			t.Errorf("%q: %v", datei, err)
		}
	}
	if _, _, err := LeseCSV(fehlerReader{}); err == nil || err.Error() != "lesen gescheitert" {
		t.Fatalf("Lesefehler: %v", err)
	}
	if err := csvFehler(errors.New("x")); err.Error() != "CSV ungültig: x" {
		t.Fatal(err)
	}
}

type fehlerReader struct{}

func (fehlerReader) Read([]byte) (int, error) { return 0, errors.New("lesen gescheitert") }
