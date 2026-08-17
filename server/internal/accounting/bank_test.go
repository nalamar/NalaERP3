package accounting

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// bank.go hat - anders als journal.go/ar.go/payments.go - praktisch keine
// Validierung vor dem ersten DB-Zugriff: Ingest() und Match() rufen sofort
// s.pg.Begin(ctx) auf. Die einzige ohne echte Postgres-Transaktion sicher
// testbare Logik ist der Nicht-Treffer-Zweig von findInvoiceIDInReference:
// bei leerer Referenz oder einer Referenz ohne passendes Zahlenmuster wird
// uuid.Nil zurueckgegeben, OHNE die uebergebene Transaktion anzufassen - das
// bleibt auch mit tx=nil sicher pruefbar. Alles danach (tatsaechlicher
// DB-Lookup, Ingest, Match, findInvoiceByAmount) ist ungetestet, siehe
// docs/backlog.md 0.9.

func TestFindInvoiceIDInReferenceReturnsNilForEmptyReference(t *testing.T) {
	s := &BankService{}

	got := s.findInvoiceIDInReference(context.Background(), nil, "", "EUR", "default")
	if got != uuid.Nil {
		t.Fatalf("expected uuid.Nil, got %v", got)
	}
}

func TestFindInvoiceIDInReferenceReturnsNilWhenNoPatternMatches(t *testing.T) {
	cases := []string{
		"Vielen Dank fuer Ihre Ueberweisung",
		"Dauerauftrag Miete",
		"abc",
	}
	for _, ref := range cases {
		s := &BankService{}

		got := s.findInvoiceIDInReference(context.Background(), nil, ref, "EUR", "default")
		if got != uuid.Nil {
			t.Errorf("reference=%q: expected uuid.Nil (kein Zahlenmuster), got %v", ref, got)
		}
	}
}
