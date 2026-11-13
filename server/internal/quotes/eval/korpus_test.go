package eval

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "X83-Dateien des Korpus neu erzeugen")

// xmlZaehlen zaehlt Elemente nach lokalem Namen und liefert den Namespace
// des Wurzelelements; die Datei muss wohlgeformt sein.
func xmlZaehlen(t *testing.T, raw []byte) (map[string]int, string) {
	t.Helper()
	n, ns := map[string]int{}, ""
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return n, ns
			}
			t.Fatalf("XML nicht wohlgeformt: %v", err)
		}
		if s, ok := tok.(xml.StartElement); ok {
			if ns == "" {
				ns = s.Name.Space
			}
			n[s.Name.Local]++
		}
	}
}

func TestKorpus(t *testing.T) {
	k := katalog(t)
	dateien, err := filepath.Glob(filepath.Join("testdata", "lv-*.erwartet.json"))
	if err != nil || len(dateien) < 2 {
		t.Fatalf("Korpus fehlt: %v %v", dateien, err)
	}
	var alle []ErwartetePosition
	stufen, formate := map[string]bool{}, map[string]bool{}
	for _, d := range dateien {
		lv, err := LoadLV(d, k)
		if err != nil {
			t.Fatal(err)
		}
		if want := strings.TrimSuffix(filepath.Base(d), ".erwartet.json") + ".x83"; lv.Datei != want {
			t.Fatalf("%s: Datei %q, erwartet %q", d, lv.Datei, want)
		}
		got, err := SchreibeX83(lv)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		x83 := filepath.Join("testdata", lv.Datei)
		if *update {
			if err := os.WriteFile(x83, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(x83)
		if err != nil {
			t.Fatalf("%v (neu erzeugen mit -update)", err)
		}
		if !bytes.Equal(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n")), got) {
			t.Fatalf("%s weicht von der Erwartung ab (neu erzeugen mit -update)", x83)
		}
		n, ns := xmlZaehlen(t, raw)
		if ns != x83Namespaces[lv.Format] || n["Item"] != len(lv.Positionen) || n["BoQCtgy"] != len(lv.Bereiche) ||
			n["Remark"] != len(lv.Vorbemerkungen) {
			t.Fatalf("%s: Namespace %q, %d Items, %d Bereiche, %d Hinweise", x83, ns, n["Item"], n["BoQCtgy"], n["Remark"])
		}
		if len(lv.Positionen) < 20 {
			t.Fatalf("%s: nur %d Positionen", d, len(lv.Positionen))
		}
		alle = append(alle, lv.Positionen...)
		formate[lv.Format] = true
		if len(lv.Bereiche) == 0 {
			stufen["flach"] = true
		}
		for _, b := range lv.Bereiche {
			stufen[b.Stufe] = true
		}
	}
	arten, ohneZiel, pauschal := map[string]bool{}, 0, false
	for _, p := range alle {
		arten[p.Art] = true
		pauschal = pauschal || p.Pauschal
		if p.Leistung == "" {
			ohneZiel++
		}
	}
	if len(arten) != len(validArten) || !pauschal || len(stufen) != len(validStufen)+1 || len(formate) != len(x83Namespaces) {
		t.Fatalf("Korpus deckt nicht alles ab: Arten %v, Pauschale %v, Stufen %v, Formate %v", arten, pauschal, stufen, formate)
	}
	// Umfang nach ADR 0039, Entscheidung 2 (a)
	if len(dateien) != 10 || len(alle) < 250 || len(alle) > 300 {
		t.Fatalf("Korpus: %d LVs mit %d Positionen, erwartet 10 LVs mit 250–300", len(dateien), len(alle))
	}
	if q := quote(ohneZiel, len(alle)); q < 0.15 || q > 0.30 {
		t.Fatalf("Anteil ohne Ziel %.2f außerhalb 15–30 %%", q)
	}
}

func TestSchreibeX83(t *testing.T) {
	l := lv()
	l.Positionen[3].Kurztext = "Rollladen <Kasten> & Gurt"
	l.Positionen[2].OZ = "01.01.00020" // längste Stelle bestimmt die Länge, nicht die letzte
	l.Positionen = append(l.Positionen, ErwartetePosition{OZ: "01.01.0040", Bereich: "01.01", Kurztext: "Fensterbank", Menge: 12.5,
		Einheit: "m", Art: ArtNormal, Pauschal: true, Langtext: "Zeile 1\nZeile 2"})
	raw, err := SchreibeX83(l)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	compact := strings.Join(strings.Fields(s), "") // ohne Einrückung
	for _, want := range []string{
		`<GAEB xmlns="http://www.gaeb.de/GAEB_DA_XML/DA83/3.3">`, "<Version>3.3</Version>", "<DP>83</DP>",
		"<BoQBkdn><Type>BoQLevel</Type><Length>2</Length>", "<BoQBkdn><Type>Item</Type><Length>5</Length>",
		`<BoQCtgy ID="C2" RNoPart="01">`, `<Item ID="I4" RNoPart="0010">`,
		"<Qty>4.000</Qty>", "<Qty>12.500</Qty>", "<ALNGroupNo>1</ALNGroupNo><ALNSerNo>0</ALNSerNo>",
		"<ALNGroupNo>1</ALNGroupNo><ALNSerNo>1</ALNSerNo>", "<Provis>WithoutTotal</Provis>",
		"<LumpSumItem>Yes</LumpSumItem>", "Rollladen &lt;Kasten&gt; &amp; Gurt",
		"<p><span>Zeile 1</span></p>\n", "<p><span>Zeile 2</span></p>\n",
	} {
		if !strings.Contains(s, want) && !strings.Contains(compact, want) {
			t.Fatalf("fehlt: %q\n%s", want, s)
		}
	}
	if strings.Count(s, "<DetailTxt>") != 1 || strings.Contains(s, "<Provis>WithTotal") {
		t.Fatalf("Langtext oder Bedarf falsch:\n%s", s)
	}
	n, _ := xmlZaehlen(t, raw)
	if n["Item"] != 5 || n["BoQCtgy"] != 2 || n["Remark"] != 0 {
		t.Fatalf("Zählung %v", n)
	}
	// Los oben, Vorbemerkung vor der ersten Position, 3.2 und Bedarf mit GB
	l = lv()
	l.Format = "da_xml_3.2"
	l.Bereiche[0].Stufe = StufeLos
	l.Vorbemerkungen = []string{"Maße am Bau nehmen."}
	l.Positionen[3].Art = ArtBedarfMitGB
	raw, err = SchreibeX83(l)
	if s = string(raw); err != nil || !strings.Contains(s, "DA83/3.2") || !strings.Contains(s, "<Version>3.2</Version>") || !strings.Contains(s, "<Type>Lot</Type>") ||
		!strings.Contains(s, "<Provis>WithTotal</Provis>") || strings.Index(s, "Maße am Bau") > strings.Index(s, "<Item ") {
		t.Fatalf("Variante: %v\n%s", err, s)
	}
	cases := []struct {
		name string
		edit func(*ErwartetesLV)
		want string
	}{
		{"Format", func(l *ErwartetesLV) { l.Format = "gaeb_2000" }, "Format"},
		{"Phase", func(l *ErwartetesLV) { l.Phase = "X81" }, "Phase"},
		{"Tiefe", func(l *ErwartetesLV) { l.Positionen[3].OZ = "01.01.0030.1" }, "Gliederungstiefe"},
		{"OZ nicht unter Bereich", func(l *ErwartetesLV) { l.Positionen[3].OZ = "01.02.0030" }, "liegt nicht unter 01.01"},
		{"Bereich nicht unter Eltern", func(l *ErwartetesLV) { l.Bereiche[1].OZ = "02.01" }, "liegt nicht unter 01"},
		{"Bereich ohne Trennpunkt", func(l *ErwartetesLV) { l.Bereiche[1].OZ = "010" }, "liegt nicht unter 01"},
		{"Bereich nicht direkt", func(l *ErwartetesLV) { l.Bereiche[1].OZ = "01.01.01" }, "nicht direkt unter"},
		{"Bereiche und Positionen", func(l *ErwartetesLV) {
			l.Positionen = []ErwartetePosition{{OZ: "01.0010", Bereich: "01", Kurztext: "x", Einheit: "m", Art: ArtNormal}}
		}, "enthaelt Bereiche und Positionen"},
		{"Grundposition fehlt", func(l *ErwartetesLV) { l.Positionen[1].Zu = "01.01.0020" }, "Grundposition 01.01.0020 fehlt"},
		{"zehn Alternativen", func(l *ErwartetesLV) {
			for i := 0; i < 9; i++ {
				l.Positionen = append(l.Positionen, l.Positionen[1])
			}
		}, "mehr als 9 Alternativen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := lv()
			c.edit(l)
			if _, err := SchreibeX83(l); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Fehler = %v, erwartet %q", err, c.want)
			}
		})
	}
}
