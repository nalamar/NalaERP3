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
