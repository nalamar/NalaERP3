package quotes

import (
	"context"
	"testing"
)

// Create() validiert companyID VOR s.pg.Begin(ctx) (server/internal/quotes/service.go),
// analog zum bereits etablierten Muster in contacts/projects/purchasing/sales/accounting.
// Alle weiteren Validierungen (contact_id/project_id, Positionen) liegen in
// createQuoteTx hinter s.pg.Begin(ctx) und sind daher ohne Postgres nicht
// unit-testbar (kein DB-Mock im Repo, siehe docs/adr/0001-baseline.md).
func TestCreateRejectsMissingCompanyID(t *testing.T) {
	s := &Service{}

	out, err := s.Create(context.Background(), QuoteInput{}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil quote, got %#v", out)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

// taxRate liest den Steuersatz aus den per loadTaxCodesTx geladenen
// Stammdaten statt (wie vor Backlog 0.36) einen DE19/DE7-Sonderfall
// hartzucodieren; ein unbekannter/inaktiver Code liefert jetzt einen
// Fehler statt still auf 0% zurueckzufallen (gleiches Muster wie
// accounting/ar.go aus Backlog 0.7 und sales/service.go aus Backlog 0.36).
func TestQuoteTaxRateKnownAndUnknownCodes(t *testing.T) {
	codes := map[string]taxCodeInfo{
		"DE19": {Rate: 0.19},
		"DE7":  {Rate: 0.07},
	}
	cases := []struct {
		code    string
		want    float64
		wantErr bool
	}{
		{"DE19", 0.19, false},
		{"DE7", 0.07, false},
		{"", 0, false}, // kein Steuerkennzeichen = steuerfrei, kein Fehler
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

func TestQuoteCalcTotalsSumsNetAndTax(t *testing.T) {
	codes := map[string]taxCodeInfo{"DE19": {Rate: 0.19}}
	items := []QuoteItemInput{
		{Qty: 2, UnitPrice: 100, TaxCode: "DE19"},
		{Qty: 1, UnitPrice: 50, TaxCode: ""},
	}
	net, tax, err := calcTotals(codes, items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if net != 250 {
		t.Fatalf("expected net 250, got %v", net)
	}
	if tax != 38 {
		t.Fatalf("expected tax 38 (19%% of 200), got %v", tax)
	}
}

func TestQuoteCalcTotalsRejectsUnknownTaxCode(t *testing.T) {
	codes := map[string]taxCodeInfo{"DE19": {Rate: 0.19}}
	items := []QuoteItemInput{{Qty: 1, UnitPrice: 100, TaxCode: "XX"}}
	if _, _, err := calcTotals(codes, items); err == nil {
		t.Fatal("expected error for unknown tax code, got nil")
	}
}
