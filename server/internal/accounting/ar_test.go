package accounting

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateRejectsMissingCompanyID(t *testing.T) {
	s := &ARService{}

	out, err := s.Create(context.Background(), InvoiceOutInput{
		ContactID: "contact-1",
		Items:     []InvoiceItemInput{{Description: "Pos", Qty: 1, UnitPrice: 10}},
	}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil invoice, got %#v", out)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestStornoRejectsMissingCompanyID(t *testing.T) {
	s := &ARService{}

	out, err := s.Storno(context.Background(), uuid.New(), "Kunde storniert", "", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil invoice, got %#v", out)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestStornoRejectsMissingReason(t *testing.T) {
	s := &ARService{}

	out, err := s.Storno(context.Background(), uuid.New(), "   ", "company-1", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil invoice, got %#v", out)
	}
	if err.Error() != "Stornogrund erforderlich" {
		t.Fatalf("expected Stornogrund erforderlich, got %q", err.Error())
	}
}

// TestBuildStornoJournalReversesDebitAndCreditButStaysBalanced belegt den
// GoBD-Kern des Storno-Konzepts: die Umkehrbuchung nutzt dieselben Konten
// und Betraege wie die Originalbuchung, aber mit vertauschtem Soll/Haben -
// sie bleibt dabei selbst wieder ausgeglichen (Soll=Haben) und veraendert
// nirgends die Originalbuchung.
func TestBuildStornoJournalReversesDebitAndCreditButStaysBalanced(t *testing.T) {
	codes := testTaxCodes()
	items := []InvoiceItemInput{
		{Description: "Profile", Qty: 2, UnitPrice: 50, TaxCode: "DE19", AccountCode: "8400"},
	}
	net, tax, err := calcTotals(codes, items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inv := InvoiceOut{
		ID:          uuid.New(),
		InvoiceDate: time.Now(),
		Currency:    "EUR",
		GrossAmount: net + tax,
	}

	original, err := buildJournal(codes, inv, "RE-0001", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	storno, err := buildStornoJournal(codes, inv, "RE-0001", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(storno.Lines) != len(original.Lines) {
		t.Fatalf("expected %d storno lines, got %d", len(original.Lines), len(storno.Lines))
	}
	var debit, credit float64
	for i, l := range storno.Lines {
		if l.AccountCode != original.Lines[i].AccountCode {
			t.Fatalf("line %d: expected same account code %q, got %q", i, original.Lines[i].AccountCode, l.AccountCode)
		}
		if l.Debit != original.Lines[i].Credit || l.Credit != original.Lines[i].Debit {
			t.Fatalf("line %d: expected debit/credit swapped vs original (orig debit=%v credit=%v), got debit=%v credit=%v", i, original.Lines[i].Debit, original.Lines[i].Credit, l.Debit, l.Credit)
		}
		debit += l.Debit
		credit += l.Credit
	}
	if math.Abs(debit-credit) > 0.0001 {
		t.Fatalf("Storno-Journal nicht ausgeglichen: Soll=%v Haben=%v", debit, credit)
	}
	if storno.Source != "invoice_out_storno" {
		t.Fatalf("expected source invoice_out_storno, got %q", storno.Source)
	}
}

func TestCreateTxRejectsMissingContact(t *testing.T) {
	s := &ARService{}

	out, err := s.createTx(context.Background(), nil, InvoiceOutInput{
		Items: []InvoiceItemInput{{Description: "Pos", Qty: 1, UnitPrice: 10}},
	}, nil, nil, "company-1")
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
	}, nil, nil, "company-1")
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

// testTaxCodes liefert eine handgebaute Stammdaten-Fixtur, die exakt dem
// entspricht, was loadTaxCodes(ctx, tx) fuer die per 017_accounting_basics.sql
// geseedeten Steuerkennzeichen liefern wuerde - ohne echte DB, fuer die
// DB-losen Tests in dieser Datei (Backlog 0.7: taxRate/taxAccountFor lesen
// jetzt aus den Stammdaten statt einen Sonderfall hartzucodieren).
func testTaxCodes() map[string]taxCodeInfo {
	return map[string]taxCodeInfo{
		"DE19": {Rate: 0.19, LiabilityAccount: "1776"},
		"DE7":  {Rate: 0.07, LiabilityAccount: "1771"},
		"DE0":  {Rate: 0, LiabilityAccount: ""}, // bekannter Code ohne konfiguriertes USt-Konto
	}
}

func TestTaxRateKnownAndUnknownCodes(t *testing.T) {
	codes := testTaxCodes()
	cases := []struct {
		code    string
		want    float64
		wantErr bool
	}{
		{"DE19", 0.19, false},
		{"DE7", 0.07, false},
		{"", 0, false}, // kein Steuerkennzeichen = steuerfrei, kein Fehler
		// unbekannter/inaktiver Code liefert jetzt einen Fehler statt still
		// auf 0% zurueckzufallen (Kern der Behebung von Backlog 0.7).
		{"XX", 0, true},
	}
	for _, tc := range cases {
		got, err := taxRate(codes, tc.code)
		if tc.wantErr {
			if err == nil {
				t.Errorf("taxRate(%q): expected error, got nil", tc.code)
			}
			continue
		}
		if err != nil {
			t.Errorf("taxRate(%q): unexpected error %v", tc.code, err)
			continue
		}
		if got != tc.want {
			t.Errorf("taxRate(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestTaxAccountForKnownAndUnknownCodes(t *testing.T) {
	codes := testTaxCodes()
	cases := []struct {
		code    string
		want    string
		wantErr bool
	}{
		{"DE19", "1776", false},
		{"DE7", "1771", false},
		// unbekannter Code liefert jetzt einen Fehler statt auf das
		// DE19-Steuerkonto zurueckzufallen (Kern der Behebung von Backlog 0.7).
		{"XX", "", true},
		// bekannter Code, aber ohne konfiguriertes Konto -> ebenfalls Fehler,
		// nicht stiller Rueckfall.
		{"DE0", "", true},
	}
	for _, tc := range cases {
		got, err := taxAccountFor(codes, tc.code)
		if tc.wantErr {
			if err == nil {
				t.Errorf("taxAccountFor(%q): expected error, got nil", tc.code)
			}
			continue
		}
		if err != nil {
			t.Errorf("taxAccountFor(%q): unexpected error %v", tc.code, err)
			continue
		}
		if got != tc.want {
			t.Errorf("taxAccountFor(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestCalcTotalsSumsNetAndTax(t *testing.T) {
	codes := testTaxCodes()
	items := []InvoiceItemInput{
		{Qty: 2, UnitPrice: 50, TaxCode: "DE19"}, // net 100, tax 19
		{Qty: 1, UnitPrice: 200, TaxCode: "DE7"}, // net 200, tax 14
		{Qty: 3, UnitPrice: 10, TaxCode: ""},     // net 30, tax 0 (steuerfrei)
	}
	net, tax, err := calcTotals(codes, items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(net-330) > 0.0001 {
		t.Errorf("net = %v, want 330", net)
	}
	if math.Abs(tax-33) > 0.0001 {
		t.Errorf("tax = %v, want 33", tax)
	}
}

func TestCalcTotalsRejectsUnknownTaxCode(t *testing.T) {
	codes := testTaxCodes()
	items := []InvoiceItemInput{{Qty: 1, UnitPrice: 100, TaxCode: "XX"}}
	if _, _, err := calcTotals(codes, items); err == nil {
		t.Fatal("expected error for unknown tax code, got nil")
	}
}

// TestBuildJournalProducesBalancedEntry prueft die fuer die Buchhaltung
// zentrale Regel: jede aus einer Ausgangsrechnung erzeugte Buchung muss
// Soll=Haben ergeben (dieselbe Invariante, die JournalService.Create in
// journal.go durchsetzt). Ein hier erzeugtes unausgeglichenes Journal wuerde
// beim tatsaechlichen Buchen (ARService.Book -> JournalService.CreateTx)
// mit "Soll/Haben nicht ausgeglichen" abgelehnt.
func TestBuildJournalProducesBalancedEntry(t *testing.T) {
	codes := testTaxCodes()
	items := []InvoiceItemInput{
		{Description: "Profile", Qty: 2, UnitPrice: 50, TaxCode: "DE19", AccountCode: "8400"},
		{Description: "Montage", Qty: 1, UnitPrice: 200, TaxCode: "DE7", AccountCode: "8300"},
		{Description: "Skonto (steuerfrei)", Qty: 3, UnitPrice: 10, TaxCode: "", AccountCode: "8200"},
	}
	net, tax, err := calcTotals(codes, items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inv := InvoiceOut{
		ID:          uuid.New(),
		InvoiceDate: time.Now(),
		Currency:    "EUR",
		GrossAmount: net + tax,
	}

	entry, err := buildJournal(codes, inv, "RE-0001", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
