package accounting

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestApplyRejectsNonPositiveAmount(t *testing.T) {
	cases := []float64{0, -0.01, -100}
	for _, amount := range cases {
		s := NewPaymentService(nil, nil)

		pay, err := s.Apply(context.Background(), PaymentInput{
			InvoiceID: uuid.New(),
			Amount:    amount,
		})
		if err == nil {
			t.Fatalf("amount=%v: expected validation error, got nil", amount)
		}
		if pay != nil {
			t.Fatalf("amount=%v: expected nil payment, got %#v", amount, pay)
		}
		if err.Error() != "Betrag muss > 0 sein" {
			t.Fatalf("amount=%v: expected 'Betrag muss > 0 sein', got %q", amount, err.Error())
		}
	}
}

func TestPaymentJournalSelectsBankAccountByMethod(t *testing.T) {
	cases := map[string]string{
		"cash":  "1000",
		"bank":  "1200",
		"":      "1200", // apply() defaultet leeres Method auf "bank", paymentJournal selbst faellt ebenfalls auf 1200 zurueck
		"other": "1200",
	}
	invoiceID := uuid.New()
	for method, want := range cases {
		entry := paymentJournal(PaymentInput{Amount: 100, Currency: "EUR", Method: method, Date: time.Now()}, invoiceID)
		if len(entry.Lines) == 0 || entry.Lines[0].AccountCode != want {
			t.Errorf("method=%q: bank account = %v, want %v", method, entry.Lines[0].AccountCode, want)
		}
	}
}

// TestPaymentJournalProducesBalancedEntry prueft dieselbe GoBD-relevante
// Invariante wie TestBuildJournalProducesBalancedEntry in ar_test.go: jede
// Zahlungsbuchung muss Soll=Haben ergeben, da sie ueber JournalService.CreateTx
// gegen dieselbe Ausgleichspruefung aus journal.go laeuft.
func TestPaymentJournalProducesBalancedEntry(t *testing.T) {
	entry := paymentJournal(PaymentInput{
		Amount:    250.5,
		Currency:  "EUR",
		Method:    "bank",
		Reference: "Ueberweisung 2026-08-11",
		Date:      time.Now(),
	}, uuid.New())

	var debit, credit float64
	for _, l := range entry.Lines {
		debit += l.Debit
		credit += l.Credit
	}
	if math.Abs(debit-credit) > 0.0001 {
		t.Fatalf("Journal nicht ausgeglichen: Soll=%v Haben=%v", debit, credit)
	}
	if len(entry.Lines) != 2 {
		t.Fatalf("len(entry.Lines) = %d, want 2 (Bank/Kasse + AR-Ausgleich)", len(entry.Lines))
	}
	if entry.Lines[1].AccountCode != "1400" || entry.Lines[1].Credit != 250.5 {
		t.Fatalf("erwartete AR-Ausgleichszeile 1400 ueber 250.5, got %#v", entry.Lines[1])
	}
}
