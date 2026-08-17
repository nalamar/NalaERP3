package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/auditlog"
	"nalaerp3/internal/settings"
	"nalaerp3/internal/testutil"
)

// BankService ist aktuell an KEINEN HTTP-Handler angebunden (per
// `grep -rln NewBankService` ueber server/internal/http bestaetigt: die
// Funktion wird nirgends aufgerufen, siehe docs/backlog.md 0.37) - daher
// testet diese Datei die Bank-Domaene direkt gegen echtes Postgres statt
// ueber HTTP, analog zum bereits etablierten Muster in
// internal/hr/service_scoping_integration_test.go (Backlog 0.9).

func newBankTestServices(env *testutil.IntegrationEnv) (*BankService, *ARService) {
	journalSvc := NewJournalService(env.PG)
	paymentSvc := NewPaymentService(env.PG, journalSvc)
	numSvc := settings.NewNumberingService(env.PG)
	auditSvc := auditlog.NewService(env.PG)
	arSvc := NewARService(env.PG, numSvc, journalSvc, auditSvc)
	bankSvc := NewBankService(env.PG, paymentSvc)
	return bankSvc, arSvc
}

func seedBankTestContact(t *testing.T, env *testutil.IntegrationEnv, id, name string) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `
        INSERT INTO contacts (id, typ, rolle, name) VALUES ($1,'org','customer',$2)
        ON CONFLICT (id) DO NOTHING
    `, id, name); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
}

// createAndBookInvoice legt eine Rechnung mit einer Position (Nettopreis
// netAmount, Steuerkennzeichen DE19) an und bucht sie sofort, sodass sie ein
// offener Posten fuer den Bankabgleich ist. Aufrufer verwenden den
// zurueckgegebenen GrossAmount als Zahlungsbetrag statt netAmount direkt,
// da die Rechnung inkl. 19% USt gebucht wird.
//
// Bewusst DE19 statt eines leeren Steuerkennzeichens: server/internal/accounting/ar.go
// createTx() setzt bei leerem TaxCode aktuell keine SQL-NULL, sondern die
// leere Zeichenkette in invoice_out_items.tax_code - das verletzt den
// Fremdschluessel auf tax_codes(code) (kein Eintrag mit code=”). Ein
// vorbestehender, von dieser Subtask unabhaengiger Bug (siehe Backlog 0.38),
// hier bewusst umgangen statt mitgefixt, um den Scope auf bank.go zu
// begrenzen.
func createAndBookInvoice(t *testing.T, ctx context.Context, arSvc *ARService, contactID, companyID string, netAmount float64) *InvoiceOut {
	t.Helper()
	created, err := arSvc.Create(ctx, InvoiceOutInput{
		ContactID:   contactID,
		InvoiceDate: time.Now(),
		Currency:    "EUR",
		Items: []InvoiceItemInput{
			{Description: "Testposition", Qty: 1, UnitPrice: netAmount, TaxCode: "DE19", AccountCode: "8000"},
		},
	}, companyID)
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	// Leerer actorUserID statt eines erfundenen Strings: entity_change_log.actor_user_id
	// hat eine Fremdschluessel-Referenz auf users(id); auditlog.Service.Record
	// setzt bei leerem ActorUserID bewusst NULL statt eines ungueltigen Werts.
	booked, err := arSvc.Book(ctx, created.ID, companyID, "")
	if err != nil {
		t.Fatalf("book invoice: %v", err)
	}
	return booked
}

func TestBankIngestPersistsStatementAndAppliesPaymentWhenInvoiceIDProvided(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	bankSvc, arSvc := newBankTestServices(env)
	seedBankTestContact(t, env, "itest-bank-contact-1", "Bank Ingest Kunde")

	inv := createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-1", "default", 250)

	stmtID, err := bankSvc.Ingest(ctx, BankStatementInput{
		BookingDate:  time.Now(),
		Amount:       inv.GrossAmount,
		Currency:     "EUR",
		Counterparty: "Bank Ingest Kunde",
		Reference:    "Zahlung " + *inv.Number,
		InvoiceID:    &inv.ID,
	}, "default")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if stmtID == uuid.Nil {
		t.Fatal("expected non-nil statement id")
	}

	updated, err := arSvc.Get(ctx, inv.ID, "default")
	if err != nil {
		t.Fatalf("get invoice: %v", err)
	}
	if updated.Status != "paid" {
		t.Fatalf("expected invoice status paid after ingest with invoice_id, got %q", updated.Status)
	}

	statements, err := bankSvc.List(ctx, 10, "default")
	if err != nil {
		t.Fatalf("list statements: %v", err)
	}
	found := false
	for _, s := range statements {
		if s["id"] == stmtID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected ingested statement to appear in list")
	}
}

func TestBankMatchRejectsAlreadyMatchedStatement(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	bankSvc, arSvc := newBankTestServices(env)
	seedBankTestContact(t, env, "itest-bank-contact-2", "Bank Match Kunde")

	inv := createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-2", "default", 300)

	stmtID, err := bankSvc.Ingest(ctx, BankStatementInput{
		BookingDate: time.Now(),
		Amount:      inv.GrossAmount,
		Currency:    "EUR",
	}, "default")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if _, err := bankSvc.Match(ctx, stmtID, &inv.ID, "default"); err != nil {
		t.Fatalf("first match: %v", err)
	}

	if _, err := bankSvc.Match(ctx, stmtID, &inv.ID, "default"); err == nil {
		t.Fatal("expected error for already matched statement, got nil")
	} else if err.Error() != "Statement bereits gematcht" {
		t.Fatalf("expected 'Statement bereits gematcht', got %q", err.Error())
	}
}

// TestBankMatchFindsInvoiceByReferenceNumber belegt, dass der
// Referenz-Erkennungs-Pfad (findInvoiceIDInReference) VOR der
// Betrags-Heuristik greift: zwei Rechnungen mit identischem offenem Betrag
// wuerden die Betrags-Heuristik zwingend zur Mehrdeutigkeit fuehren - der
// Match gelingt hier trotzdem eindeutig, weil die Referenz die Rechnungs-
// nummer enthaelt.
func TestBankMatchFindsInvoiceByReferenceNumber(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	bankSvc, arSvc := newBankTestServices(env)
	seedBankTestContact(t, env, "itest-bank-contact-3", "Bank Reference Kunde")

	invA := createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-3", "default", 400)
	_ = createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-3", "default", 400)

	stmtID, err := bankSvc.Ingest(ctx, BankStatementInput{
		BookingDate: time.Now(),
		Amount:      invA.GrossAmount,
		Currency:    "EUR",
		Reference:   "Zahlung " + *invA.Number,
	}, "default")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	payID, err := bankSvc.Match(ctx, stmtID, nil, "default")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if payID == uuid.Nil {
		t.Fatal("expected non-nil payment id")
	}

	updated, err := arSvc.Get(ctx, invA.ID, "default")
	if err != nil {
		t.Fatalf("get invoice: %v", err)
	}
	if updated.Status != "paid" {
		t.Fatalf("expected referenced invoice to be paid, got %q", updated.Status)
	}
}

func TestBankMatchFindsInvoiceByAmountWhenReferenceHasNoMatch(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	bankSvc, arSvc := newBankTestServices(env)
	seedBankTestContact(t, env, "itest-bank-contact-4", "Bank Amount Kunde")

	inv := createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-4", "default", 175)

	stmtID, err := bankSvc.Ingest(ctx, BankStatementInput{
		BookingDate: time.Now(),
		Amount:      inv.GrossAmount,
		Currency:    "EUR",
		Reference:   "Ueberweisung ohne erkennbare Rechnungsnummer",
	}, "default")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	payID, err := bankSvc.Match(ctx, stmtID, nil, "default")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if payID == uuid.Nil {
		t.Fatal("expected non-nil payment id")
	}

	updated, err := arSvc.Get(ctx, inv.ID, "default")
	if err != nil {
		t.Fatalf("get invoice: %v", err)
	}
	if updated.Status != "paid" {
		t.Fatalf("expected invoice matched by amount to be paid, got %q", updated.Status)
	}
}

func TestBankMatchReturnsErrorForAmbiguousAmount(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	bankSvc, arSvc := newBankTestServices(env)
	seedBankTestContact(t, env, "itest-bank-contact-5", "Bank Ambiguous Kunde")

	ambiguousInv := createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-5", "default", 220)
	createAndBookInvoice(t, ctx, arSvc, "itest-bank-contact-5", "default", 220)

	stmtID, err := bankSvc.Ingest(ctx, BankStatementInput{
		BookingDate: time.Now(),
		Amount:      ambiguousInv.GrossAmount,
		Currency:    "EUR",
		Reference:   "Unklare Ueberweisung",
	}, "default")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if _, err := bankSvc.Match(ctx, stmtID, nil, "default"); err == nil {
		t.Fatal("expected ambiguity error, got nil")
	} else if err.Error() != "Mehrere mögliche offene Posten gefunden, bitte manuell zuordnen" {
		t.Fatalf("expected ambiguity error message, got %q", err.Error())
	}
}

func TestBankMatchReturnsErrorWhenNoInvoiceMatches(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	bankSvc, _ := newBankTestServices(env)

	stmtID, err := bankSvc.Ingest(ctx, BankStatementInput{
		BookingDate: time.Now(),
		Amount:      999999.99,
		Currency:    "EUR",
		Reference:   "Kein passender Betrag",
	}, "default")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if _, err := bankSvc.Match(ctx, stmtID, nil, "default"); err == nil {
		t.Fatal("expected no-match error, got nil")
	} else if err.Error() != "Kein passender offener Posten gefunden" {
		t.Fatalf("expected no-match error message, got %q", err.Error())
	}
}
