package apihttp

import (
	"testing"

	"nalaerp3/internal/pdfgen"
	"nalaerp3/internal/quotes"
)

func TestQuotePDFItemsMarksAlternativeAndProvisionalItems(t *testing.T) {
	got := quotePDFItems([]quotes.QuoteItemInput{
		{Description: "Von Hand", Qty: 2, Unit: "Stk", UnitPrice: 10, TaxCode: "DE19"},
		{Description: "Grund", Qty: 1, Unit: "Stk", UnitPrice: 50, OutlineNo: "01.0010", ItemKind: "grundposition"},
		{Description: "Alternative", Qty: 1, Unit: "Stk", UnitPrice: 60, OutlineNo: "01.0020", ItemKind: "alternativposition", BaseOutlineNo: "01.0010"},
		{Description: "Bedarf", Qty: 3, Unit: "h", UnitPrice: 40, OutlineNo: "01.0030", ItemKind: "bedarfsposition_ohne_gb"},
	})
	want := []pdfgen.QuoteItemData{
		{Pos: 1, Description: "Von Hand", Qty: 2, Unit: "Stk", UnitPrice: 10, TaxCode: "DE19", LineTotal: 20},
		{Pos: 2, Description: "Grund", Qty: 1, Unit: "Stk", UnitPrice: 50, LineTotal: 50, OutlineNo: "01.0010"},
		{Pos: 3, Description: "Alternative", Qty: 1, Unit: "Stk", UnitPrice: 60, LineTotal: 60, OutlineNo: "01.0020", KindNote: "Alternative zu Pos. 01.0010"},
		{Pos: 4, Description: "Bedarf", Qty: 3, Unit: "h", UnitPrice: 40, LineTotal: 120, OutlineNo: "01.0030", KindNote: "Bedarfsposition"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d Zeilen", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Zeile %d: %+v, erwartet %+v", i, got[i], want[i])
		}
	}
}
