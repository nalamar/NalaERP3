package accounting

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateTxRejectsMissingContact(t *testing.T) {
	s := &ARService{}

	out, err := s.createTx(context.Background(), nil, InvoiceOutInput{
		Items: []InvoiceItemInput{{Description: "Pos", Qty: 1, UnitPrice: 10}},
	}, nil, nil)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil invoice, got %#v", out)
	}
	if err.Error() != "contact_id fehlt" {
		t.Fatalf("expected 'contact_id fehlt', got %q", err.Error())
	}
}

func TestCreateTxRejectsEmptyItems(t *testing.T) {
	s := &ARService{}

	out, err := s.createTx(context.Background(), nil, InvoiceOutInput{
		ContactID: "contact-1",
	}, nil, nil)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil invoice, got %#v", out)
	}
	if err.Error() != "keine Positionen" {
		t.Fatalf("expected 'keine Positionen', got %q", err.Error())
	}
}

func TestTaxRateKnownAndUnknownCodes(t *testing.T) {
	cases := map[string]float64{
		"DE19": 0.19,
		"DE7":  0.07,
		"":     0,
		"XX":   0, // unbekannter Code faellt auf 0% zurueck, siehe docs/backlog.md 0.7
	}
	for code, want := range cases {
		if got := taxRate(code); got != want {
			t.Errorf("taxRate(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestTaxAccountForKnownAndUnknownCodes(t *testing.T) {
	cases := map[string]string{
		"DE19": "1776",
		"DE7":  "1771",
		// Unbekannte Codes fallen auf das DE19-Steuerkonto zurueck statt
		// einen Fehler zu liefern - dokumentiertes, nicht in dieser Subtask
		// behobenes Verhalten, siehe docs/backlog.md 0.7.
		"XX": "1776",
	}
	for code, want := range cases {
		if got := taxAccountFor(code); got != want {
			t.Errorf("taxAccountFor(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestCalcTotalsSumsNetAndTax(t *testing.T) {
	items := []InvoiceItemInput{
		{Qty: 2, UnitPrice: 50, TaxCode: "DE19"}, // net 100, tax 19
		{Qty: 1, UnitPrice: 200, TaxCode: "DE7"}, // net 200, tax 14
		{Qty: 3, UnitPrice: 10, TaxCode: ""},     // net 30, tax 0 (steuerfrei)
	}
	net, tax := calcTotals(items)
	if math.Abs(net-330) > 0.0001 {
		t.Errorf("net = %v, want 330", net)
	}
	if math.Abs(tax-33) > 0.0001 {
		t.Errorf("tax = %v, want 33", tax)
	}
}

// TestBuildJournalProducesBalancedEntry prueft die fuer die Buchhaltung
// zentrale Regel: jede aus einer Ausgangsrechnung erzeugte Buchung muss
// Soll=Haben ergeben (dieselbe Invariante, die JournalService.Create in
// journal.go durchsetzt). Ein hier erzeugtes unausgeglichenes Journal wuerde
// beim tatsaechlichen Buchen (ARService.Book -> JournalService.CreateTx)
// mit "Soll/Haben nicht ausgeglichen" abgelehnt.
func TestBuildJournalProducesBalancedEntry(t *testing.T) {
	items := []InvoiceItemInput{
		{Description: "Profile", Qty: 2, UnitPrice: 50, TaxCode: "DE19", AccountCode: "8400"},
		{Description: "Montage", Qty: 1, UnitPrice: 200, TaxCode: "DE7", AccountCode: "8300"},
		{Description: "Skonto (steuerfrei)", Qty: 3, UnitPrice: 10, TaxCode: "", AccountCode: "8200"},
	}
	net, tax := calcTotals(items)
	inv := InvoiceOut{
		ID:          uuid.New(),
		InvoiceDate: time.Now(),
		Currency:    "EUR",
		GrossAmount: net + tax,
	}

	entry := buildJournal(inv, "RE-0001", items)

	var debit, credit float64
	for _, l := range entry.Lines {
		debit += l.Debit
		credit += l.Credit
	}
	if math.Abs(debit-credit) > 0.0001 {
		t.Fatalf("Journal nicht ausgeglichen: Soll=%v Haben=%v", debit, credit)
	}

	// 1 Debitorenzeile (AR) + 3 Erloeszeilen + 2 USt-Zeilen (nur fuer die
	// beiden steuerpflichtigen Positionen, die steuerfreie Position erzeugt
	// keine eigene USt-Zeile).
	wantLines := 1 + len(items) + 2
	if len(entry.Lines) != wantLines {
		t.Fatalf("len(entry.Lines) = %d, want %d", len(entry.Lines), wantLines)
	}
	if entry.Lines[0].AccountCode != "1400" || entry.Lines[0].Debit != inv.GrossAmount {
		t.Fatalf("erwartete Debitorenzeile 1400 ueber %v, got %#v", inv.GrossAmount, entry.Lines[0])
	}
}
