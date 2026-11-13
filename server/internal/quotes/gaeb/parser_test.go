package gaeb

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const kopf = `<?xml version="1.0" encoding="UTF-8"?>
<GAEB xmlns="http://www.gaeb.de/GAEB_DA_XML/DA83/3.3"><GAEBInfo><Version>3.3</Version></GAEBInfo><Award><DP>83</DP>`

// dok baut eine X83 mit einem LV; bkdn ist der Typ der obersten Stufe.
func dok(bkdn, body string) string {
	return kopf + `<BoQ><BoQInfo><BoQBkdn><Type>` + bkdn + `</Type></BoQBkdn></BoQInfo><BoQBody>` + body + `</BoQBody></BoQ></Award></GAEB>`
}

// item baut eine Position; ohne eigene QU mit Einheit "Stk".
func item(rno, inner string) string {
	if !strings.Contains(inner, "<QU>") {
		inner = `<QU>Stk</QU>` + inner
	}
	return `<Item RNoPart="` + rno + `">` + inner + `</Item>`
}

func kurz(s string) string {
	return `<Description><CompleteText><OutlineText><OutlTxt><TextOutlTxt><p><span>` + s + `</span></p></TextOutlTxt></OutlTxt></OutlineText></CompleteText></Description>`
}

func ctgy(rno, name, body string) string {
	return `<BoQCtgy RNoPart="` + rno + `"><LblTx><p><span>` + name + `</span></p></LblTx><BoQBody>` + body + `</BoQBody></BoQCtgy>`
}

func parse(t *testing.T, s string) *LV {
	t.Helper()
	lv, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return lv
}

func TestParseGliederung(t *testing.T) {
	// Lose mit Titeln und Untertiteln, Positionen auf der untersten Stufe
	s := dok("Lot", ctgy("1", "Los A", ctgy("01", "Titel", ctgy("02", "Unter", ctgy("03", "Tiefer", `<Itemlist>`+item("0010", kurz("x"))+`</Itemlist>`)))))
	lv := parse(t, s)
	want := []Bereich{
		{OZ: "1", Stufe: StufeLos, Bezeichnung: "Los A"},
		{OZ: "1.01", Stufe: StufeTitel, Bezeichnung: "Titel", Eltern: "1"},
		{OZ: "1.01.02", Stufe: StufeUntertitel, Bezeichnung: "Unter", Eltern: "1.01"},
		{OZ: "1.01.02.03", Stufe: StufeUntertitel, Bezeichnung: "Tiefer", Eltern: "1.01.02"},
	}
	if !reflect.DeepEqual(lv.Bereiche, want) {
		t.Fatalf("Bereiche: %+v", lv.Bereiche)
	}
	if p := lv.Positionen[0]; p.OZ != "1.01.02.03.0010" || p.Bereich != "1.01.02.03" || p.Kurztext != "x" || p.Art != ArtNormal {
		t.Fatalf("Position: %+v", p)
	}
	// ohne Lose: oberste Stufe Titel, darunter Untertitel
	lv = parse(t, dok("BoQLevel", ctgy(" 01 ", "T", ctgy("02", "U", `<Itemlist>`+item("1", kurz("x"))+`</Itemlist>`))))
	if lv.Bereiche[0].Stufe != StufeTitel || lv.Bereiche[1].Stufe != StufeUntertitel || lv.Bereiche[1].OZ != "01.02" {
		t.Fatalf("ohne Lose: %+v", lv.Bereiche)
	}
}

func TestParsePosition(t *testing.T) {
	lang := `<DetailTxt><Text><p><span>Zeile 1</span> <span>&amp; mehr</span></p><p><span></span></p><p>Zeile 3</p></Text></DetailTxt>`
	body := `<Itemlist>` +
		`<Remark><Description><CompleteText><DetailTxt><Text><p><span>Hinweis A</span></p><p><span>zweite Zeile</span></p></Text></DetailTxt></CompleteText></Description></Remark>` +
		item(" 0010 ", `<Qty> 12.500 </Qty><QU> m2 </QU><Unbekannt>x</Unbekannt><LumpSumItem>Yes</LumpSumItem><Description><CompleteText>`+lang+
			`<OutlineText><OutlTxt><TextOutlTxt><p><span>Kurz</span></p></TextOutlTxt></OutlTxt></OutlineText></CompleteText></Description>`) +
		item("0020", `<QU>psch</QU><LumpSumItem>No</LumpSumItem>`+kurz("ohne Menge")) +
		item("0030", `<Provis> WithTotal </Provis>`+kurz("b1")) +
		item("0040", `<Provis>WithoutTotal</Provis>`+kurz("b2")) +
		item("0050", `<ALNGroupNo>7</ALNGroupNo><ALNSerNo>0</ALNSerNo>`+kurz("g7")) +
		item("0060", `<ALNGroupNo>8</ALNGroupNo><ALNSerNo>0</ALNSerNo>`+kurz("g8")) +
		item("0070", `<ALNGroupNo> 7 </ALNGroupNo><ALNSerNo> 1 </ALNSerNo>`+kurz("a7")) +
		item("0080", `<ALNGroupNo>8</ALNGroupNo><ALNSerNo>2</ALNSerNo>`+kurz("a8")) +
		`<Remark><Description><CompleteText><DetailTxt><Text>ohne Absatz</Text></DetailTxt></CompleteText></Description></Remark>` +
		`</Itemlist>`
	lv := parse(t, dok("Item", body))
	if !reflect.DeepEqual(lv.Vorbemerkungen, []string{"Hinweis A\nzweite Zeile", "ohne Absatz"}) || len(lv.Bereiche) != 0 || lv.Bereiche == nil {
		t.Fatalf("Vorbemerkungen/Bereiche: %q %v", lv.Vorbemerkungen, lv.Bereiche)
	}
	want := []Position{
		{OZ: "0010", Kurztext: "Kurz", Langtext: "Zeile 1 & mehr\n\nZeile 3", Menge: 12.5, Einheit: "m2", Art: ArtNormal, Pauschal: true},
		{OZ: "0020", Kurztext: "ohne Menge", Einheit: "psch", Art: ArtNormal},
		{OZ: "0030", Kurztext: "b1", Art: ArtBedarfMitGB, Einheit: "Stk"},
		{OZ: "0040", Kurztext: "b2", Art: ArtBedarfOhneGB, Einheit: "Stk"},
		{OZ: "0050", Kurztext: "g7", Art: ArtGrund, Einheit: "Stk"},
		{OZ: "0060", Kurztext: "g8", Art: ArtGrund, Einheit: "Stk"},
		{OZ: "0070", Kurztext: "a7", Art: ArtAlternativ, Zu: "0050", Einheit: "Stk"},
		{OZ: "0080", Kurztext: "a8", Art: ArtAlternativ, Zu: "0060", Einheit: "Stk"},
	}
	if !reflect.DeepEqual(lv.Positionen, want) {
		t.Fatalf("Positionen:\n got %+v\nwant %+v", lv.Positionen, want)
	}
}

func TestParseVersion(t *testing.T) {
	ohne := func(ns, version string) string {
		return `<GAEB xmlns="` + ns + `"><GAEBInfo>` + version + `</GAEBInfo><Award><DP>83</DP><BoQ><BoQBody><Itemlist>` +
			item("1", kurz("x")) + `</Itemlist></BoQBody></BoQ></Award></GAEB>`
	}
	for _, c := range []struct{ doc, format string }{
		{ohne("http://www.gaeb.de/GAEB_DA_XML/DA83/3.2", ""), "da_xml_3.2"},
		{ohne("http://www.gaeb.de/GAEB_DA_XML/DA83/3.3", "<Version> 3.3 </Version>"), "da_xml_3.3"},
		{ohne("urn:anders", "<Version>3.2</Version>"), "da_xml_3.2"},
		{strings.Replace(ohne("", "<Version>3.3</Version>"), ` xmlns=""`, ``, 1), "da_xml_3.3"},
	} {
		if lv := parse(t, c.doc); lv.Format != c.format || lv.Phase != "X83" || len(lv.Positionen) != 1 {
			t.Fatalf("%s: %+v", c.format, lv)
		}
	}
}

func TestParseFehler(t *testing.T) {
	il := func(items string) string { return dok("Item", `<Itemlist>`+items+`</Itemlist>`) }
	cases := []struct{ name, doc, want string }{
		{"kein XML", "kein xml", "kein lesbares XML"},
		{"abgebrochen", kopf, "kein lesbares XML"},
		{"falsche Wurzel", `<LV><Award><DP>83</DP></Award></LV>`, "nicht GAEB"},
		{"Version", strings.Replace(il(item("1", kurz("x"))), "<Version>3.3</Version>", "<Version>3.1</Version>", 1), `DA XML "3.1"`},
		{"ohne Version", `<GAEB><Award><DP>83</DP></Award></GAEB>`, `DA XML ""`},
		{"Phase", strings.Replace(il(item("1", kurz("x"))), "<DP>83</DP>", "<DP>81</DP>", 1), `Phase "81"`},
		{"Phase 84", strings.Replace(il(item("1", kurz("x"))), "<DP>83</DP>", "<DP>84</DP>", 1), `Phase "84"`},
		{"ohne Phase", strings.Replace(il(item("1", kurz("x"))), "<DP>83</DP>", "", 1), `Phase ""`},
		{"zwei LVs", kopf + `<BoQ/><BoQ/></Award></GAEB>`, "2 Leistungsverzeichnisse"},
		{"kein LV", kopf + `</Award></GAEB>`, "0 Leistungsverzeichnisse"},
		{"gemischt", dok("BoQLevel", ctgy("01", "T", `<Itemlist/>`)+`<Itemlist/>`), `Bereich "" enthält`},
		{"gemischt tief", dok("BoQLevel", ctgy("01", "T", ctgy("02", "U", "")+`<Itemlist/>`)), `Bereich "01" enthält`},
		{"Menge", il(item("1", `<Qty>1,5</Qty>`)), `Position 1: Menge "1,5"`},
		{"Bedarf", il(item("1", `<Provis>Yes</Provis>`)), "unbekannte Bedarfsposition"},
		{"Bedarf und Wahl", il(item("1", `<Provis>WithTotal</Provis><ALNGroupNo>1</ALNGroupNo><ALNSerNo>0</ALNSerNo>`)), "zugleich"},
		{"Gruppe ohne Serie", il(item("1", `<ALNGroupNo>1</ALNGroupNo>`)), "unvollständig"},
		{"Serie ohne Gruppe", il(item("1", `<ALNSerNo>0</ALNSerNo>`)), "unvollständig"},
		{"Serie keine Zahl", il(item("1", `<ALNGroupNo>1</ALNGroupNo><ALNSerNo>a</ALNSerNo>`)), "unvollständig"},
		{"Serie negativ", il(item("1", `<ALNGroupNo>1</ALNGroupNo><ALNSerNo>-1</ALNSerNo>`)), "unvollständig"},
		{"Alternative zuerst", il(item("1", `<ALNGroupNo>1</ALNGroupNo><ALNSerNo>1</ALNSerNo>`)), "Grundposition der Gruppe 1 fehlt"},
		{"zwei Grundpositionen", il(item("1", `<ALNGroupNo>1</ALNGroupNo><ALNSerNo>0</ALNSerNo>`+kurz("x")) +
			item("2", `<ALNGroupNo>1</ALNGroupNo><ALNSerNo>0</ALNSerNo>`)), "Position 2: Gruppe 1 hat schon die Grundposition 1"},
		{"Pauschale", il(item("1", `<LumpSumItem>true</LumpSumItem>`)), "unbekannte Pauschale"},
	}
	for _, c := range cases {
		lv, err := Parse(strings.NewReader(c.doc))
		if err == nil || !strings.Contains(err.Error(), c.want) || lv != nil {
			t.Fatalf("%s: %v, erwartet %q", c.name, err, c.want)
		}
	}
}

func TestParsePflicht(t *testing.T) {
	il := func(items string) string { return dok("Item", `<Itemlist>`+items+`</Itemlist>`) }
	cases := []struct{ name, doc, want string }{
		{"ohne OZ", il(item(" ", kurz("x"))), `Ordnungszahl unter "" fehlt`},
		{"Bereich ohne OZ", dok("BoQLevel", ctgy("", "T", `<Itemlist>`+item("1", kurz("x"))+`</Itemlist>`)), `Ordnungszahl unter "" fehlt`},
		{"OZ doppelt", il(item("1", kurz("x")) + item(" 1", kurz("y"))), "Ordnungszahl 1 ist doppelt"},
		{"Bereich doppelt", dok("BoQLevel", ctgy("01", "A", `<Itemlist>`+item("1", kurz("x"))+`</Itemlist>`)+
			ctgy("01", "B", `<Itemlist>`+item("2", kurz("x"))+`</Itemlist>`)), "Ordnungszahl 01 ist doppelt"},
		{"ohne Einheit", il(item("1", `<QU> </QU>`+kurz("x"))), "Position 1: Einheit fehlt"},
		{"ohne Kurztext", il(item("1", "")), "Position 1: Kurztext fehlt"},
		{"leerer Kurztext", il(item("1", kurz("  "))), "Position 1: Kurztext fehlt"},
		{"Menge negativ", il(item("1", `<Qty>-1</Qty>`+kurz("x"))), `Menge "-1"`},
		{"Menge NaN", il(item("1", `<Qty>NaN</Qty>`+kurz("x"))), `Menge "NaN"`},
		{"Menge unendlich", il(item("1", `<Qty>+Inf</Qty>`+kurz("x"))), `Menge "+Inf"`},
		{"ohne Positionen", dok("BoQLevel", ctgy("01", "T", `<Itemlist/>`)), "LV ohne Positionen"},
	}
	for _, c := range cases {
		lv, err := Parse(strings.NewReader(c.doc))
		if err == nil || !strings.Contains(err.Error(), c.want) || lv != nil {
			t.Fatalf("%s: %v, erwartet %q", c.name, err, c.want)
		}
	}
	if lv := parse(t, il(item("1", `<Qty>0</Qty>`+kurz("x")))); lv.Positionen[0].Menge != 0 {
		t.Fatalf("Menge 0: %+v", lv.Positionen[0])
	}
}

func TestParseGrenzen(t *testing.T) {
	ok := func(name, doc string) {
		t.Helper()
		if _, err := Parse(strings.NewReader(doc)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	zuViel := func(name, doc, want string) {
		t.Helper()
		if lv, err := Parse(strings.NewReader(doc)); err == nil || !strings.Contains(err.Error(), want) || lv != nil {
			t.Fatalf("%s: %v, erwartet %q", name, err, want)
		}
	}
	// Gliederungstiefe: 5 Stufen von Bereichen erlaubt, 6 nicht
	tief := func(n int) string {
		s := `<Itemlist>` + item("1", kurz("x")) + `</Itemlist>`
		for i := n; i > 0; i-- {
			s = ctgy(strconv.Itoa(i), "B", s)
		}
		return dok("BoQLevel", s)
	}
	ok("Tiefe 5", tief(MaxTiefe))
	zuViel("Tiefe 6", tief(MaxTiefe+1), "Gliederung unter 1.2.3.4.5 ist tiefer als 5 Stufen")
	// Texte: Zeichen, nicht Bytes
	lang := func(n int) string { return strings.Repeat("ä", n) }
	langtext := func(s string) string {
		return `<Description><CompleteText><DetailTxt><Text><p>` + s + `</p></Text></DetailTxt>` +
			`<OutlineText><OutlTxt><TextOutlTxt><p>x</p></TextOutlTxt></OutlTxt></OutlineText></CompleteText></Description>`
	}
	il := func(items string) string { return dok("Item", `<Itemlist>`+items+`</Itemlist>`) }
	ok("Langtext an der Grenze", il(item("1", langtext(lang(MaxTextZeichen)))))
	zuViel("Langtext", il(item("1", langtext(lang(MaxTextZeichen+1)))), "Position 1: Langtext ist länger als 20000 Zeichen")
	zuViel("Kurztext", il(item("1", kurz(lang(MaxTextZeichen+1)))), "Position 1: Kurztext ist länger")
	zuViel("Bezeichnung", dok("BoQLevel", ctgy("01", lang(MaxTextZeichen+1), `<Itemlist>`+item("1", kurz("x"))+`</Itemlist>`)),
		"Bezeichnung von Bereich 01 ist länger")
	zuViel("Vorbemerkung", il(`<Remark><Description><CompleteText><DetailTxt><Text><p>`+lang(MaxTextZeichen+1)+
		`</p></Text></DetailTxt></CompleteText></Description></Remark>`+item("1", kurz("x"))), "Vorbemerkung ist länger")
	// Positionen: 20.000 erlaubt, eine mehr nicht
	viele := func(n int) string {
		var b strings.Builder
		for i := 1; i <= n; i++ {
			b.WriteString(item(strconv.Itoa(i), kurz("x")))
		}
		return il(b.String())
	}
	ok("20000 Positionen", viele(MaxPositionen))
	zuViel("20001 Positionen", viele(MaxPositionen+1), "mehr als 20000 Positionen")
}
