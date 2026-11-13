package quotes

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestGAEBDAXMLParser(t *testing.T) {
	var _ GAEBImportParser = GAEBDAXMLParser{}
	f, err := os.Open("eval/testdata/lv-01.x83")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	result, err := (GAEBDAXMLParser{}).ParseGAEB(context.Background(), f, "lv-01.x83")
	if err != nil {
		t.Fatal(err)
	}
	if result.ParserVersion != "gaeb-da-xml-v1" || result.DetectedFormat != "da_xml_3.3_x83" || len(result.Items) != 26 {
		t.Fatalf("Kopf: %s %s %d", result.ParserVersion, result.DetectedFormat, len(result.Items))
	}
	byOZ := map[string]QuoteImportItemInput{}
	for i, item := range result.Items {
		if item.SortOrder != i+1 {
			t.Fatalf("SortOrder %d an Stelle %d", item.SortOrder, i+1)
		}
		byOZ[item.PositionNo] = item
	}
	first := result.Items[0]
	if first.PositionNo != "01.0010" || first.OutlineNo != "01" || first.Qty != 1 || first.Unit != "psch" || first.IsOptional ||
		first.ParserHint != "normal" ||
		first.Description != "Baustelleneinrichtung einrichten und räumen\n\nEinrichten, Vorhalten und Räumen der Baustelleneinrichtung für das Gewerk Fenster und Türen." {
		t.Fatalf("erste Position: %+v", first)
	}
	for oz, optional := range map[string]bool{"02.0080": false, "02.0090": true, "03.0050": true, "03.0060": true} {
		if item := byOZ[oz]; item.IsOptional != optional || item.ParserHint == "" || item.ParserHint == "normal" {
			t.Fatalf("%s: %+v", oz, item)
		}
	}
	if byOZ["02.0090"].ParserHint != "alternativposition" || byOZ["06.0010"].Qty != 20 || byOZ["06.0010"].Unit != "h" {
		t.Fatalf("Art: %+v", byOZ["02.0090"])
	}
	// ohne Langtext nur der Kurztext
	ohneLang := `<GAEB><GAEBInfo><Version>3.2</Version></GAEBInfo><Award><DP>83</DP><BoQ><BoQBody><Itemlist><Item RNoPart="1"><QU>m</QU>` +
		`<Description><CompleteText><OutlineText><OutlTxt><TextOutlTxt><p>Kurz</p></TextOutlTxt></OutlTxt></OutlineText></CompleteText></Description>` +
		`</Item></Itemlist></BoQBody></BoQ></Award></GAEB>`
	result, err = (GAEBDAXMLParser{}).ParseGAEB(context.Background(), strings.NewReader(ohneLang), "x.x83")
	if err != nil || result.DetectedFormat != "da_xml_3.2_x83" || result.Items[0].Description != "Kurz" || result.Items[0].OutlineNo != "" {
		t.Fatalf("ohne Langtext: %+v %v", result, err)
	}
	// Fehler des Parsers werden weitergereicht
	if _, err := (GAEBDAXMLParser{}).ParseGAEB(context.Background(), strings.NewReader(`<gaeb><item/></gaeb>`), "x.xml"); err == nil ||
		!strings.Contains(err.Error(), "GAEB-Datei konnte nicht gelesen werden") || !strings.Contains(err.Error(), "nicht GAEB") {
		t.Fatalf("Subset-Format angenommen: %v", err)
	}
}
