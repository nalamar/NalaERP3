package quotes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func TestApprovalRequestDecisionsMutateOnlyRequest(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	deciderID := "approval-decider"
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, display_name)
		VALUES ($1, $2, 'test', 'Approval Decider')
		ON CONFLICT (id) DO NOTHING
	`, deciderID, "approval-decider@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	contactID := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO contacts (id, typ, rolle, status, name, email, phone, waehrung)
		VALUES ($1, 'org', 'customer', 'active', 'Approval Kunde GmbH', 'approval@example.com', '+49 211 1000', 'EUR')
	`, contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	svc := NewService(env.PG, nil)

	const companyID = "default"

	approvedQuoteID, approvedItemID := seedApprovalQuote(t, ctx, env, contactID, "ANG-APPROVE-TEST", 50, 60)
	requested, err := svc.RequestApprovalForQuoteItem(ctx, approvedQuoteID, approvedItemID, deciderID, "Zielmarge pruefen", companyID)
	if err != nil {
		t.Fatalf("request approval: %v", err)
	}
	if requested.Status != "requested" {
		t.Fatalf("expected requested status, got %+v", requested)
	}

	approved, err := svc.ApproveApprovalRequestForQuoteItem(ctx, approvedQuoteID, approvedItemID, deciderID, "wirtschaftlich freigegeben", companyID)
	if err != nil {
		t.Fatalf("approve approval request: %v", err)
	}
	if approved.ID != requested.ID || approved.Status != "approved" {
		t.Fatalf("unexpected approved request: %+v", approved)
	}
	if approved.DecidedBy != deciderID || approved.DecidedAt == nil || approved.DecisionComment != "wirtschaftlich freigegeben" {
		t.Fatalf("missing approval decision metadata: %+v", approved)
	}
	if approved.ApprovedUnitPriceSnapshot == nil || *approved.ApprovedUnitPriceSnapshot != approved.CurrentUnitPriceSnapshot {
		t.Fatalf("expected approved unit price snapshot from current price, got %+v", approved)
	}
	if approved.ApprovedTargetMarginPercent == nil || *approved.ApprovedTargetMarginPercent != approved.TargetMarginPercentSnapshot {
		t.Fatalf("expected approved target margin snapshot, got %+v", approved)
	}

	if _, err := svc.RejectApprovalRequestForQuoteItem(ctx, approvedQuoteID, approvedItemID, deciderID, "zu spaet", companyID); err == nil || !strings.Contains(err.Error(), "Keine aktive Freigabeanforderung vorhanden") {
		t.Fatalf("expected duplicate decision to fail with no active request, got %v", err)
	}

	assertQuoteCommercialsUnchanged(t, ctx, env, approvedQuoteID, approvedItemID, 50, 50)

	rejectedQuoteID, rejectedItemID := seedApprovalQuote(t, ctx, env, contactID, "ANG-REJECT-TEST", 50, 60)
	if _, err := svc.RequestApprovalForQuoteItem(ctx, rejectedQuoteID, rejectedItemID, deciderID, "Zielmarge pruefen", companyID); err != nil {
		t.Fatalf("request approval for rejection: %v", err)
	}
	rejected, err := svc.RejectApprovalRequestForQuoteItem(ctx, rejectedQuoteID, rejectedItemID, deciderID, "Preis nacharbeiten", companyID)
	if err != nil {
		t.Fatalf("reject approval request: %v", err)
	}
	if rejected.Status != "rejected" || rejected.DecidedBy != deciderID || rejected.DecidedAt == nil || rejected.DecisionComment != "Preis nacharbeiten" {
		t.Fatalf("unexpected rejected request: %+v", rejected)
	}
	if rejected.ApprovedUnitPriceSnapshot != nil || rejected.ApprovedTargetMarginPercent != nil {
		t.Fatalf("rejection must not carry approved snapshots: %+v", rejected)
	}

	if _, err := svc.ApproveApprovalRequestForQuoteItem(ctx, rejectedQuoteID, rejectedItemID, deciderID, strings.Repeat("x", 501), companyID); err == nil || !strings.Contains(err.Error(), "Kommentar darf nicht laenger als 500 Zeichen sein") {
		t.Fatalf("expected long decision comment to fail, got %v", err)
	}

	assertQuoteCommercialsUnchanged(t, ctx, env, rejectedQuoteID, rejectedItemID, 50, 50)
}

func seedApprovalQuote(t *testing.T, ctx context.Context, env *testutil.IntegrationEnv, contactID, number string, unitPrice, costBasis float64) (uuid.UUID, uuid.UUID) {
	t.Helper()

	quoteID := uuid.New()
	itemID := uuid.New()
	decisionID := uuid.New()
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO quotes (id, nummer, root_quote_id, revision_no, contact_id, status, quote_date, currency, net_amount, tax_amount, gross_amount, company_id)
		VALUES ($1, $2, $1, 1, $3, 'draft', CURRENT_DATE, 'EUR', $4, 0, $4, 'default')
	`, quoteID, number, contactID, unitPrice); err != nil {
		t.Fatalf("seed quote: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount)
		VALUES ($1, $2, 1, 'Freigabeposition', 1, 'Stk', $3, $3, 0)
	`, itemID, quoteID, unitPrice); err != nil {
		t.Fatalf("seed quote item: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
		INSERT INTO quote_item_price_decisions (
			id,
			quote_id,
			quote_item_id,
			decision_type,
			source_label,
			source_unit_price,
			applied_unit_price,
			currency
		)
		VALUES ($1, $2, $3, 'primary_source_applied', 'Test-Kostenbasis', $4, $4, 'EUR')
	`, decisionID, quoteID, itemID, costBasis); err != nil {
		t.Fatalf("seed price decision: %v", err)
	}
	return quoteID, itemID
}

func assertQuoteCommercialsUnchanged(t *testing.T, ctx context.Context, env *testutil.IntegrationEnv, quoteID, itemID uuid.UUID, expectedUnitPrice, expectedNet float64) {
	t.Helper()

	var unitPrice float64
	if err := env.PG.QueryRow(ctx, `
		SELECT unit_price
		FROM quote_items
		WHERE id = $1
	`, itemID).Scan(&unitPrice); err != nil {
		t.Fatalf("read quote item price: %v", err)
	}
	if unitPrice != expectedUnitPrice {
		t.Fatalf("expected unchanged unit price %.2f, got %.2f", expectedUnitPrice, unitPrice)
	}

	var netAmount float64
	if err := env.PG.QueryRow(ctx, `
		SELECT net_amount
		FROM quotes
		WHERE id = $1
	`, quoteID).Scan(&netAmount); err != nil {
		t.Fatalf("read quote net amount: %v", err)
	}
	if netAmount != expectedNet {
		t.Fatalf("expected unchanged quote net %.2f, got %.2f", expectedNet, netAmount)
	}
}
