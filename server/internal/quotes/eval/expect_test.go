package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nalaerp3/internal/matching"
)

func f(v float64) *float64 { return &v }

func katalog(t *testing.T) *Katalog {
	t.Helper()
	k, err := LoadKatalog(filepath.Join("testdata", "katalog.json"))
	if err != nil {
		t.Fatalf("Katalog: %v", err)
	}
	return k
}

func lv() *ErwartetesLV {
	return &ErwartetesLV{
		Datei:  "lv-00.x83",
		Format: "da_xml_3.3",
		Phase:  "X83",
		Bereiche: []Bereich{
			{OZ: "01", Stufe: StufeTitel, Bezeichnung: "Fenster"},
			{OZ: "01.01", Stufe: StufeUntertitel, Bezeichnung: "Erdgeschoss", Eltern: "01"},
		},
		Positionen: []ErwartetePosition{
			{OZ: "01.01.0010", Bereich: "01.01", Kurztext: "Fenster 1-flg.", Menge: 4, Einheit: "Stk", Art: ArtGrund, Leistung: "FE-ALU-1F",
				Systemvorgaben: Systemvorgaben{Profilserie: "Serie A75", UWert: f(0.9)}},
			{OZ: "01.01.0011", Bereich: "01.01", Kurztext: "Fenster 2-flg. alternativ", Menge: 4, Einheit: "Stk", Art: ArtAlternativ, Zu: "01.01.0010", Leistung: "FE-ALU-2F"},
			{OZ: "01.01.0020", Bereich: "01.01", Kurztext: "Haustür RC2", Menge: 1, Einheit: "Stk", Art: ArtNormal, Leistung: "TU-HAUS-RC2",
				Systemvorgaben: Systemvorgaben{RCKlasse: "RC2"}},
			{OZ: "01.01.0030", Bereich: "01.01", Kurztext: "Rollladen", Menge: 4, Einheit: "Stk", Art: ArtBedarfOhneGB},
		},
	}
}

func TestKatalogUndLV(t *testing.T) {
	k := katalog(t)
	if len(k.Leistungen) < 10 || !k.Code("TU-BS-T30") || k.Code("GIBT-ES-NICHT") {
		t.Fatalf("Katalog unerwartet: %d Leistungen", len(k.Leistungen))
	}
	if err := lv().Validate(k); err != nil {
		t.Fatalf("gültiges LV abgelehnt: %v", err)
	}
	cases := []struct {
		name string
		edit func(*ErwartetesLV)
		want string
	}{
		{"ohne Phase", func(l *ErwartetesLV) { l.Phase = " " }, "Datei, Format und Phase"},
		{"Bereich doppelt", func(l *ErwartetesLV) { l.Bereiche[1].OZ = "01" }, "Bereich 01 ist doppelt"},
		{"Bereich ohne OZ", func(l *ErwartetesLV) { l.Bereiche[0].OZ = "" }, "Bereich ohne OZ"},
		{"Stufe unbekannt", func(l *ErwartetesLV) { l.Bereiche[0].Stufe = "kapitel" }, "unbekannte Stufe"},
		{"Eltern fehlt", func(l *ErwartetesLV) { l.Bereiche[1].Eltern = "02" }, "Eltern 02 fehlt"},
		{"Eltern später", func(l *ErwartetesLV) { l.Bereiche[0], l.Bereiche[1] = l.Bereiche[1], l.Bereiche[0] }, "Eltern 01 fehlt"},
		{"ohne Positionen", func(l *ErwartetesLV) { l.Positionen = nil }, "LV ohne Positionen"},
		{"OZ leer", func(l *ErwartetesLV) { l.Positionen[2].OZ = "" }, "Position ohne OZ"},
		{"OZ doppelt", func(l *ErwartetesLV) { l.Positionen[2].OZ = "01.01.0010" }, "ist doppelt"},
		{"Bereich fehlt", func(l *ErwartetesLV) { l.Positionen[2].Bereich = "09" }, "Bereich 09 fehlt"},
		{"ohne Einheit", func(l *ErwartetesLV) { l.Positionen[2].Einheit = "" }, "Kurztext und Einheit"},
		{"ohne Kurztext", func(l *ErwartetesLV) { l.Positionen[2].Kurztext = "  " }, "Kurztext und Einheit"},
		{"Langtext zu lang", func(l *ErwartetesLV) { l.Positionen[2].Langtext = strings.Repeat("ü", maxTextRunes+1) }, "Langtext ist zu lang"},
		{"Menge negativ", func(l *ErwartetesLV) { l.Positionen[2].Menge = -1 }, "Menge ist negativ"},
		{"Art unbekannt", func(l *ErwartetesLV) { l.Positionen[2].Art = "eventual" }, "unbekannte Art"},
		{"Alternative ohne Grund", func(l *ErwartetesLV) { l.Positionen[1].Zu = "01.01.0020" }, "ohne vorherige Grundposition"},
		{"Alternative zu Normalposition", func(l *ErwartetesLV) { l.Positionen[0].Art = ArtNormal }, "ohne vorherige Grundposition"},
		{"Alternative vor Grund", func(l *ErwartetesLV) { l.Positionen[0], l.Positionen[1] = l.Positionen[1], l.Positionen[0] }, "ohne vorherige Grundposition"},
		{"Verweis an Normalposition", func(l *ErwartetesLV) { l.Positionen[2].Zu = "01.01.0010" }, "nur Alternativpositionen"},
		{"Leistung unbekannt", func(l *ErwartetesLV) { l.Positionen[2].Leistung = "XX" }, "Leistung XX fehlt im Katalog"},
		{"U-Wert null", func(l *ErwartetesLV) { l.Positionen[0].Systemvorgaben.UWert = f(0) }, "U-Wert"},
		{"U-Wert zu groß", func(l *ErwartetesLV) { l.Positionen[0].Systemvorgaben.UWert = f(10.5) }, "U-Wert"},
		{"Systemvorgabe zu lang", func(l *ErwartetesLV) {
			l.Positionen[0].Systemvorgaben.Verglasung = strings.Repeat("é", matching.MaxSystemvorgabeText+1)
		}, "zu lang"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := lv()
			c.edit(l)
			err := l.Validate(k)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Fehler = %v, erwartet %q", err, c.want)
			}
		})
	}
	// Grenzwerte gelten noch
	l := lv()
	l.Positionen[0].Systemvorgaben.UWert = f(10)
	l.Positionen[0].Systemvorgaben.Verglasung = strings.Repeat("é", matching.MaxSystemvorgabeText)
	l.Positionen[2].Langtext = strings.Repeat("ü", maxTextRunes)
	l.Positionen[2].Menge = 0
	if err := l.Validate(k); err != nil {
		t.Fatalf("Grenzwerte abgelehnt: %v", err)
	}
}

func TestKatalogFehler(t *testing.T) {
	cases := []struct {
		name string
		k    Katalog
		want string
	}{
		{"leer", Katalog{}, "Katalog ist leer"},
		{"Code fehlt", Katalog{Leistungen: []Leistung{{Kurztext: "x", Einheit: "Stk"}}}, "Code fehlt"},
		{"Code mit Leerraum", Katalog{Leistungen: []Leistung{{Code: "A ", Kurztext: "x", Einheit: "Stk"}}}, "Leerraum"},
		{"doppelt", Katalog{Leistungen: []Leistung{{Code: "A", Kurztext: "x", Einheit: "Stk"}, {Code: "A", Kurztext: "y", Einheit: "m"}}}, "A ist doppelt"},
		{"ohne Einheit", Katalog{Leistungen: []Leistung{{Code: "A", Kurztext: "x"}}}, "Kurztext und Einheit"},
		{"ohne Kurztext", Katalog{Leistungen: []Leistung{{Code: "A", Einheit: "m"}}}, "Kurztext und Einheit"},
		{"U-Wert negativ", Katalog{Leistungen: []Leistung{{Code: "A", Kurztext: "x", Einheit: "m", Systemvorgaben: Systemvorgaben{UWert: f(-1)}}}}, "U-Wert"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.k.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Fehler = %v, erwartet %q", err, c.want)
			}
		})
	}
}

func TestLadenStreng(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	k := katalog(t)
	ok := `{"datei":"lv.x83","format":"da_xml_3.3","phase":"X83","bereiche":[],"positionen":[{"oz":"0010","bereich":"","kurztext":"Treppe","langtext":"","menge":1,"einheit":"Stk","art":"normal","leistung":"ST-TREPPE","systemvorgaben":{}}]}`
	got, err := LoadLV(write("ok.json", ok), k)
	if err != nil || got.Positionen[0].Leistung != "ST-TREPPE" {
		t.Fatalf("LoadLV: %v %+v", err, got)
	}
	if _, err := LoadLV(write("feld.json", strings.Replace(ok, `"art"`, `"preis":5,"art"`, 1)), k); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unbekanntes Feld nicht abgelehnt: %v", err)
	}
	if _, err := LoadLV(write("leistung.json", strings.Replace(ok, "ST-TREPPE", "XX", 1)), k); err == nil || !strings.Contains(err.Error(), "fehlt im Katalog") {
		t.Fatalf("unbekannte Leistung nicht abgelehnt: %v", err)
	}
	if _, err := LoadLV(filepath.Join(dir, "fehlt.json"), k); err == nil {
		t.Fatal("fehlende Datei nicht gemeldet")
	}
	if _, err := LoadKatalog(write("k.json", `{"leistungen":[{"code":"A","kurztext":"x","einheit":"m","preis":3}]}`)); err == nil {
		t.Fatal("Katalog mit Preis angenommen")
	}
	if _, err := LoadKatalog(write("k2.json", `{"leistungen":[]}`)); err == nil || !strings.Contains(err.Error(), "leer") {
		t.Fatalf("leerer Katalog: %v", err)
	}
}
