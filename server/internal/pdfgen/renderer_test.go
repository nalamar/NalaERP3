package pdfgen

import (
	"context"
	"testing"
)

func TestParseHexColorParsesUpperAndLowercaseValues(t *testing.T) {
	fallback := rgbColor{R: 1, G: 2, B: 3}
	got := parseHexColor("#1f4B99", fallback)
	if got != (rgbColor{R: 31, G: 75, B: 153}) {
		t.Fatalf("expected parsed color {31 75 153}, got %#v", got)
	}
}

func TestParseHexColorFallsBackForInvalidValues(t *testing.T) {
	fallback := rgbColor{R: 107, G: 114, B: 128}
	got := parseHexColor("not-a-color", fallback)
	if got != fallback {
		t.Fatalf("expected fallback %#v, got %#v", fallback, got)
	}
}

func TestTemplateColorsUseDefaultsWhenUnset(t *testing.T) {
	primary, accent := templateColors(TemplateOptions{})
	if primary != (rgbColor{R: 31, G: 75, B: 153}) {
		t.Fatalf("unexpected default primary color %#v", primary)
	}
	if accent != (rgbColor{R: 107, G: 114, B: 128}) {
		t.Fatalf("unexpected default accent color %#v", accent)
	}
}

func TestQuoteItemKindNote(t *testing.T) {
	for _, c := range []struct{ kind, base, want string }{
		{"normal", "", ""},
		{"grundposition", "", ""},
		{"", "", ""},
		{"alternativposition", " 01.0010 ", "Alternative zu Pos. 01.0010"},
		{"alternativposition", "", "Alternativposition"},
		{"bedarfsposition_mit_gb", "", "Bedarfsposition"},
		{"bedarfsposition_ohne_gb", "", "Bedarfsposition"},
	} {
		if got := QuoteItemKindNote(c.kind, c.base); got != c.want {
			t.Fatalf("QuoteItemKindNote(%q, %q) = %q, want %q", c.kind, c.base, got, c.want)
		}
	}
}

func TestQuoteItemTexts(t *testing.T) {
	plain := QuoteItemData{Pos: 3, Description: "Fenster"}
	marked := QuoteItemData{Pos: 4, OutlineNo: " 01.0020 ", Description: "Fenster 2-flg.", KindNote: "Alternative zu Pos. 01.0010"}
	if quoteItemPosText(plain) != "3" || quoteItemPosText(marked) != "01.0020" {
		t.Fatalf("Pos: %q %q", quoteItemPosText(plain), quoteItemPosText(marked))
	}
	if quoteItemDescriptionText(plain) != "Fenster" ||
		quoteItemDescriptionText(marked) != "Fenster 2-flg. (Alternative zu Pos. 01.0010, nicht in der Summe)" {
		t.Fatalf("Text: %q %q", quoteItemDescriptionText(plain), quoteItemDescriptionText(marked))
	}
	if quoteExcludedItemsNote([]QuoteItemData{plain}) != "" ||
		quoteExcludedItemsNote([]QuoteItemData{plain, marked}) != "Alternativ- und Bedarfspositionen sind in den Summen nicht enthalten." {
		t.Fatal("Summenhinweis")
	}
}

func TestRenderQuoteWithGAEBItems(t *testing.T) {
	data := QuoteData{Number: "ANG-1", Currency: "EUR", NetAmount: 100, TaxAmount: 19, GrossAmount: 119, Items: []QuoteItemData{
		{Pos: 1, OutlineNo: "01.02.0010", Description: "Grundposition", Qty: 1, Unit: "Stk", UnitPrice: 100, LineTotal: 100},
		{Pos: 2, OutlineNo: "01.02.0020", Description: "Alternative", Qty: 1, Unit: "Stk", UnitPrice: 120, LineTotal: 120,
			KindNote: QuoteItemKindNote("alternativposition", "01.02.0010")},
	}}
	out, err := RenderQuote(context.Background(), nil, "", data, TemplateOptions{}, nil)
	if err != nil || len(out) < 4 || string(out[:4]) != "%PDF" {
		t.Fatalf("RenderQuote: %v (%d Bytes)", err, len(out))
	}
}
