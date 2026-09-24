package accounting

// Integrationstests fuer die E-Rechnungs-DB-Abfrage/Mapping (Backlog
// E.4.3.2). Laufen gegen echtes Postgres; der Serialisierer selbst
// (E.4.3.1) ist DB-los und in einvoice_cii_test.go abgedeckt.
//
// Die Rechnungen werden direkt ueber ARService angelegt und gebucht - der
// in bank_integration_test.go etablierte Weg, da es fuer das Buchen keinen
// eigenen Testpfad ueber HTTP in diesem Paket gibt.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/auditlog"
	"nalaerp3/internal/settings"
	"nalaerp3/internal/testutil"
)

const einvoiceTestCompany = "default"

func newEInvoiceTestServices(env *testutil.IntegrationEnv) (*EInvoiceService, *ARService) {
	journalSvc := NewJournalService(env.PG)
	numSvc := settings.NewNumberingService(env.PG)
	auditSvc := auditlog.NewService(env.PG)
	arSvc := NewARService(env.PG, numSvc, journalSvc, auditSvc)
	companySvc := settings.NewCompanyService(env.PG)
	return NewEInvoiceService(env.PG, companySvc), arSvc
}

// seedEInvoiceSeller pflegt die Verkaeuferstammdaten am Firmenprofil. Der
// Seed aus 025_company_profile.sql enthaelt nur Name und Land - ohne
// Anschrift und USt-IdNr. waere keine EN-16931-konforme Rechnung moeglich.
func seedEInvoiceSeller(t *testing.T, env *testutil.IntegrationEnv) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `
        UPDATE company_profiles
           SET street='Werkstrasse 4', postal_code='40213', city='Duesseldorf', country='DE',
               vat_id='DE123456789', tax_no='133/8150/1234', email='info@nala.example',
               invoice_email='rechnung@nala.example', phone='+49 211 1234567',
               iban='DE02120300000000202051', bic='BYLADEM1001', account_holder='Nala ERP'
         WHERE id=$1
    `, einvoiceTestCompany); err != nil {
		t.Fatalf("seed seller profile: %v", err)
	}
}

func seedEInvoiceContact(t *testing.T, env *testutil.IntegrationEnv, id, name string) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `
        INSERT INTO contacts (id, typ, rolle, name, email, vat_id)
        VALUES ($1,'org','customer',$2,'rechnung@kunde.example','DE987654321')
        ON CONFLICT (id) DO NOTHING
    `, id, name); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
}

func seedEInvoiceAddress(t *testing.T, env *testutil.IntegrationEnv, id, contactID, art, street, plz, ort string, isPrimary bool) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `
        INSERT INTO contact_addresses (id, contact_id, art, zeile1, plz, ort, land, is_primary)
        VALUES ($1,$2,$3,$4,$5,$6,'DE',$7)
        ON CONFLICT (id) DO NOTHING
    `, id, contactID, art, street, plz, ort, isPrimary); err != nil {
		t.Fatalf("seed address: %v", err)
	}
}

// createBookedInvoice legt eine Rechnung an, bucht sie und setzt die
// Kaeuferreferenz. buyer_reference hat bewusst noch keinen Schreibpfad im
// Anwendungscode (E.4.2 legte nur die Spalte an, der Endpunkt folgt in
// E.4.3.3) - daher hier direkt per SQL.
func createBookedInvoice(t *testing.T, env *testutil.IntegrationEnv, arSvc *ARService, contactID, buyerRef string, items []InvoiceItemInput) *InvoiceOut {
	t.Helper()
	ctx := context.Background()
	created, err := arSvc.Create(ctx, InvoiceOutInput{
		ContactID:   contactID,
		InvoiceDate: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC),
		Currency:    "EUR",
		Items:       items,
	}, einvoiceTestCompany)
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if buyerRef != "" {
		if _, err := env.PG.Exec(ctx, `UPDATE invoices_out SET buyer_reference=$1 WHERE id=$2`, buyerRef, created.ID); err != nil {
			t.Fatalf("set buyer_reference: %v", err)
		}
	}
	booked, err := arSvc.Book(ctx, created.ID, einvoiceTestCompany, "")
	if err != nil {
		t.Fatalf("book invoice: %v", err)
	}
	return booked
}

// TestEInvoiceBuildMapsBookedInvoice deckt den vollstaendigen Happy Path
// ab: Verkaeufer aus dem Firmenprofil, Kaeufer inkl. aufgeloester
// Anschrift, Positionen, Steueraufschluesselung und Summen.
func TestEInvoiceBuildMapsBookedInvoice(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-happy", "Bauamt Beispielstadt")
	seedEInvoiceAddress(t, env, "a-einv-happy", "c-einv-happy", "billing", "Rathausplatz 1", "50667", "Koeln", false)

	inv := createBookedInvoice(t, env, arSvc, "c-einv-happy", "991-12345-67", []InvoiceItemInput{
		{Description: "Fensterelement RC2", Qty: 2, UnitPrice: 1250, TaxCode: "DE19", AccountCode: "8000"},
		{Description: "Montage", Qty: 1, UnitPrice: 500, TaxCode: "DE19", AccountCode: "8000"},
	})

	got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
	if err != nil {
		t.Fatalf("BuildCIIInvoice: %v", err)
	}

	if got.Number == "" || inv.Number == nil || got.Number != *inv.Number {
		t.Errorf("Rechnungsnummer nicht uebernommen: %q", got.Number)
	}
	if got.TypeCode != CIITypeCodeCommercialInvoice {
		t.Errorf("Typ-Code = %q, erwartet 380", got.TypeCode)
	}
	if got.BuyerReference != "991-12345-67" {
		t.Errorf("Kaeuferreferenz = %q", got.BuyerReference)
	}
	if got.Currency != "EUR" {
		t.Errorf("Waehrung = %q", got.Currency)
	}

	if got.Seller.Name == "" || got.Seller.Street != "Werkstrasse 4" || got.Seller.PostalCode != "40213" || got.Seller.City != "Duesseldorf" {
		t.Errorf("Verkaeuferanschrift nicht aus dem Firmenprofil uebernommen: %+v", got.Seller)
	}
	if got.Seller.VatID != "DE123456789" || got.Seller.TaxNo != "133/8150/1234" {
		t.Errorf("Steuerkennungen des Verkaeufers fehlen: %+v", got.Seller)
	}
	if got.Seller.Email != "rechnung@nala.example" {
		t.Errorf("Rechnungs-E-Mail muss Vorrang vor der allgemeinen haben, got %q", got.Seller.Email)
	}
	if got.IBAN != "DE02120300000000202051" || got.BIC != "BYLADEM1001" {
		t.Errorf("Bankverbindung nicht uebernommen: IBAN=%q BIC=%q", got.IBAN, got.BIC)
	}

	if got.Buyer.Name != "Bauamt Beispielstadt" || got.Buyer.Street != "Rathausplatz 1" ||
		got.Buyer.PostalCode != "50667" || got.Buyer.City != "Koeln" {
		t.Errorf("Kaeufer nicht korrekt aufgeloest: %+v", got.Buyer)
	}
	if got.Buyer.VatID != "DE987654321" {
		t.Errorf("USt-IdNr. des Kaeufers fehlt: %q", got.Buyer.VatID)
	}

	if len(got.Lines) != 2 {
		t.Fatalf("erwartet 2 Positionen, got %d", len(got.Lines))
	}
	if got.Lines[0].LineID != "1" || got.Lines[0].Name != "Fensterelement RC2" {
		t.Errorf("Position 1 falsch: %+v", got.Lines[0])
	}
	if got.Lines[0].LineTotalAmount != 2500 || got.Lines[0].NetUnitPrice != 1250 || got.Lines[0].Qty != 2 {
		t.Errorf("Betraege Position 1 falsch: %+v", got.Lines[0])
	}
	if got.Lines[0].UnitCode != CIIDefaultUnitCode {
		t.Errorf("Einheiten-Fallback erwartet C62, got %q", got.Lines[0].UnitCode)
	}
	if got.Lines[0].TaxCategoryCode != CIITaxCategoryStandard || got.Lines[0].TaxRatePercent != 19 {
		t.Errorf("Steuerkategorie/-satz Position 1 falsch: %+v", got.Lines[0])
	}

	if len(got.TaxBreakdown) != 1 {
		t.Fatalf("erwartet 1 Steuergruppe, got %d: %+v", len(got.TaxBreakdown), got.TaxBreakdown)
	}
	tb := got.TaxBreakdown[0]
	if tb.CategoryCode != CIITaxCategoryStandard || tb.RatePercent != 19 {
		t.Errorf("Steuergruppe falsch: %+v", tb)
	}
	if tb.BasisAmount != 3000 || tb.CalculatedAmount != 570 {
		t.Errorf("Steuergruppe Betraege falsch: Basis=%v Steuer=%v", tb.BasisAmount, tb.CalculatedAmount)
	}
	if tb.ExemptionReason != "" {
		t.Errorf("Kategorie S darf keine Befreiungsbegruendung tragen, got %q", tb.ExemptionReason)
	}

	if got.LineTotalAmount != 3000 || got.TaxBasisTotalAmount != 3000 ||
		got.TaxTotalAmount != 570 || got.GrandTotalAmount != 3570 || got.DuePayableAmount != 3570 {
		t.Errorf("Summen falsch: %+v", got)
	}

	// Gegenprobe: das Mapping muss den Serialisierer zufriedenstellen.
	if _, err := BuildCrossIndustryInvoice(got); err != nil {
		t.Fatalf("gemapptes Modell wird vom Serialisierer abgelehnt: %v", err)
	}
}

// TestEInvoiceAddressPreference beweist die in ADR 0022 festgelegte
// Reihenfolge billing > is_primary > irgendeine.
func TestEInvoiceAddressPreference(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)

	t.Run("billing schlaegt is_primary", func(t *testing.T) {
		seedEInvoiceContact(t, env, "c-einv-addr1", "Kunde Adresse 1")
		seedEInvoiceAddress(t, env, "a-einv-addr1-p", "c-einv-addr1", "other", "Hauptweg 9", "10115", "Berlin", true)
		seedEInvoiceAddress(t, env, "a-einv-addr1-b", "c-einv-addr1", "billing", "Rechnungsweg 1", "20095", "Hamburg", false)

		inv := createBookedInvoice(t, env, arSvc, "c-einv-addr1", "REF-1", []InvoiceItemInput{
			{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
		})
		got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("BuildCIIInvoice: %v", err)
		}
		if got.Buyer.Street != "Rechnungsweg 1" || got.Buyer.City != "Hamburg" {
			t.Errorf("Rechnungsadresse muss Vorrang haben, got %+v", got.Buyer)
		}
	})

	t.Run("is_primary schlaegt beliebige", func(t *testing.T) {
		seedEInvoiceContact(t, env, "c-einv-addr2", "Kunde Adresse 2")
		seedEInvoiceAddress(t, env, "a-einv-addr2-x", "c-einv-addr2", "shipping", "Lieferweg 3", "80331", "Muenchen", false)
		seedEInvoiceAddress(t, env, "a-einv-addr2-p", "c-einv-addr2", "other", "Hauptweg 9", "10115", "Berlin", true)

		inv := createBookedInvoice(t, env, arSvc, "c-einv-addr2", "REF-2", []InvoiceItemInput{
			{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
		})
		got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("BuildCIIInvoice: %v", err)
		}
		if got.Buyer.Street != "Hauptweg 9" || got.Buyer.City != "Berlin" {
			t.Errorf("Hauptadresse muss Vorrang vor beliebiger haben, got %+v", got.Buyer)
		}
	})

	t.Run("beliebige wenn nichts markiert", func(t *testing.T) {
		seedEInvoiceContact(t, env, "c-einv-addr3", "Kunde Adresse 3")
		seedEInvoiceAddress(t, env, "a-einv-addr3-x", "c-einv-addr3", "shipping", "Lieferweg 3", "80331", "Muenchen", false)

		inv := createBookedInvoice(t, env, arSvc, "c-einv-addr3", "REF-3", []InvoiceItemInput{
			{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
		})
		got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("BuildCIIInvoice: %v", err)
		}
		if got.Buyer.Street != "Lieferweg 3" {
			t.Errorf("einzige vorhandene Adresse muss genutzt werden, got %+v", got.Buyer)
		}
	})
}

// TestEInvoiceRejectsMissingBuyerAddress beweist, dass ohne Anschrift keine
// Rechnung erzeugt wird statt einer mit erfundener Adresse.
func TestEInvoiceRejectsMissingBuyerAddress(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-noaddr", "Kunde ohne Anschrift")

	inv := createBookedInvoice(t, env, arSvc, "c-einv-noaddr", "REF-NOADDR", []InvoiceItemInput{
		{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
	})

	_, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
	if err == nil {
		t.Fatal("erwartet Fehler wegen fehlender Kaeuferanschrift")
	}
	if !strings.Contains(err.Error(), "Anschrift") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
	}
}

// TestEInvoiceRejectsMissingBuyerReference deckt das EN-16931-Pflichtfeld
// BT-10 ab, dessen Spalte E.4.2 angelegt hat.
func TestEInvoiceRejectsMissingBuyerReference(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-noref", "Kunde ohne Referenz")
	seedEInvoiceAddress(t, env, "a-einv-noref", "c-einv-noref", "billing", "Weg 1", "50667", "Koeln", false)

	inv := createBookedInvoice(t, env, arSvc, "c-einv-noref", "", []InvoiceItemInput{
		{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
	})

	_, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
	if err == nil {
		t.Fatal("erwartet Fehler wegen fehlender Kaeuferreferenz")
	}
	if !strings.Contains(err.Error(), "Kaeuferreferenz") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
	}
}

// TestEInvoiceStatusGuard beweist den Status-Guard aus ADR 0022: weder ein
// draft noch eine stornierte Rechnung darf das Haus verlassen.
func TestEInvoiceStatusGuard(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-status", "Kunde Status")
	seedEInvoiceAddress(t, env, "a-einv-status", "c-einv-status", "billing", "Weg 2", "50667", "Koeln", false)

	t.Run("draft wird abgelehnt", func(t *testing.T) {
		created, err := arSvc.Create(ctx, InvoiceOutInput{
			ContactID:   "c-einv-status",
			InvoiceDate: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC),
			Currency:    "EUR",
			Items: []InvoiceItemInput{
				{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
			},
		}, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("create invoice: %v", err)
		}
		if _, err := env.PG.Exec(ctx, `UPDATE invoices_out SET buyer_reference='REF-DRAFT' WHERE id=$1`, created.ID); err != nil {
			t.Fatalf("set buyer_reference: %v", err)
		}

		_, err = svc.BuildCIIInvoice(ctx, created.ID, einvoiceTestCompany)
		if err == nil {
			t.Fatal("erwartet Fehler fuer draft")
		}
		if !strings.Contains(err.Error(), "gebuchte") {
			t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
		}
	})

	t.Run("storniert wird abgelehnt", func(t *testing.T) {
		inv := createBookedInvoice(t, env, arSvc, "c-einv-status", "REF-STORNO", []InvoiceItemInput{
			{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
		})
		if _, err := arSvc.Storno(ctx, inv.ID, "Testfall", einvoiceTestCompany, ""); err != nil {
			t.Fatalf("storno: %v", err)
		}

		_, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err == nil {
			t.Fatal("erwartet Fehler fuer stornierte Rechnung")
		}
		if !strings.Contains(err.Error(), "torniert") {
			t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
		}
	})
}

// TestEInvoiceTaxCategoryMapping beweist die Ableitung der
// UNTDID-5305-Kategorie aus den vorhandenen Steuerstammdaten, inkl. der in
// E.4.3.2 gesetzten Befreiungsbegruendungen.
func TestEInvoiceTaxCategoryMapping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-tax", "Kunde Steuer")
	seedEInvoiceAddress(t, env, "a-einv-tax", "c-einv-tax", "billing", "Weg 3", "50667", "Koeln", false)

	t.Run("DE0 wird zu E mit Begruendung", func(t *testing.T) {
		inv := createBookedInvoice(t, env, arSvc, "c-einv-tax", "REF-E", []InvoiceItemInput{
			{Description: "Steuerfreie Leistung", Qty: 1, UnitPrice: 300, TaxCode: "DE0", AccountCode: "8000"},
		})
		got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("BuildCIIInvoice: %v", err)
		}
		if len(got.TaxBreakdown) != 1 || got.TaxBreakdown[0].CategoryCode != CIITaxCategoryExempt {
			t.Fatalf("erwartet Kategorie E, got %+v", got.TaxBreakdown)
		}
		if got.TaxBreakdown[0].ExemptionReason != ciiExemptionReasonExempt {
			t.Errorf("Befreiungsbegruendung fehlt: %+v", got.TaxBreakdown[0])
		}
		if _, err := BuildCrossIndustryInvoice(got); err != nil {
			t.Fatalf("Serialisierer lehnt E-Fall ab: %v", err)
		}
	})

	t.Run("RC wird zu AE mit Begruendung", func(t *testing.T) {
		inv := createBookedInvoice(t, env, arSvc, "c-einv-tax", "REF-AE", []InvoiceItemInput{
			{Description: "Reverse-Charge-Leistung", Qty: 1, UnitPrice: 400, TaxCode: "RC", AccountCode: "8000"},
		})
		got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("BuildCIIInvoice: %v", err)
		}
		if len(got.TaxBreakdown) != 1 || got.TaxBreakdown[0].CategoryCode != CIITaxCategoryReverseCharge {
			t.Fatalf("erwartet Kategorie AE, got %+v", got.TaxBreakdown)
		}
		if got.TaxBreakdown[0].ExemptionReason != ciiExemptionReasonReverseCharge {
			t.Errorf("Befreiungsbegruendung fehlt: %+v", got.TaxBreakdown[0])
		}
		if _, err := BuildCrossIndustryInvoice(got); err != nil {
			t.Fatalf("Serialisierer lehnt AE-Fall ab: %v", err)
		}
	})

	t.Run("zwei Steuersaetze ergeben zwei Gruppen", func(t *testing.T) {
		inv := createBookedInvoice(t, env, arSvc, "c-einv-tax", "REF-MIX", []InvoiceItemInput{
			{Description: "Regelsatz", Qty: 1, UnitPrice: 1000, TaxCode: "DE19", AccountCode: "8000"},
			{Description: "Ermaessigt", Qty: 1, UnitPrice: 200, TaxCode: "DE7", AccountCode: "8100"},
		})
		got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
		if err != nil {
			t.Fatalf("BuildCIIInvoice: %v", err)
		}
		if len(got.TaxBreakdown) != 2 {
			t.Fatalf("erwartet 2 Steuergruppen, got %+v", got.TaxBreakdown)
		}
		byRate := map[float64]CIITaxBreakdown{}
		for _, b := range got.TaxBreakdown {
			byRate[b.RatePercent] = b
		}
		if b := byRate[19]; b.BasisAmount != 1000 || b.CalculatedAmount != 190 {
			t.Errorf("19%%-Gruppe falsch: %+v", b)
		}
		if b := byRate[7]; b.BasisAmount != 200 || b.CalculatedAmount != 14 {
			t.Errorf("7%%-Gruppe falsch: %+v", b)
		}
		if got.TaxTotalAmount != 204 || got.GrandTotalAmount != 1404 {
			t.Errorf("Summen falsch: Steuer=%v Brutto=%v", got.TaxTotalAmount, got.GrandTotalAmount)
		}
	})
}

// TestEInvoiceRejectsItemWithoutTaxCode beweist, dass ein leeres
// Steuerkennzeichen abgelehnt statt auf eine Kategorie geraten wird. Die
// Position wird direkt per SQL eingefuegt, da der regulaere Anlagepfad
// wegen des Fremdschluessels auf tax_codes gar nicht erst bis hierher
// kaeme (vorbestehender Befund, siehe Backlog 0.38).
func TestEInvoiceRejectsItemWithoutTaxCode(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-notax", "Kunde ohne Steuerkennzeichen")
	seedEInvoiceAddress(t, env, "a-einv-notax", "c-einv-notax", "billing", "Weg 4", "50667", "Koeln", false)

	inv := createBookedInvoice(t, env, arSvc, "c-einv-notax", "REF-NOTAX", []InvoiceItemInput{
		{Description: "Pos", Qty: 1, UnitPrice: 100, TaxCode: "DE19", AccountCode: "8000"},
	})
	if _, err := env.PG.Exec(ctx, `UPDATE invoice_out_items SET tax_code=NULL WHERE invoice_id=$1`, inv.ID); err != nil {
		t.Fatalf("tax_code auf NULL setzen: %v", err)
	}

	_, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
	if err == nil {
		t.Fatal("erwartet Fehler wegen fehlenden Steuerkennzeichens")
	}
	if !strings.Contains(err.Error(), "Steuerkennzeichen") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %v", err)
	}
}

// TestEInvoicePrepaymentTypeCode beweist die Typ-Code-Zuordnung aus
// ADR 0022 fuer Abschlagsrechnungen.
func TestEInvoicePrepaymentTypeCode(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, arSvc := newEInvoiceTestServices(env)
	seedEInvoiceSeller(t, env)
	seedEInvoiceContact(t, env, "c-einv-abschlag", "Kunde Abschlag")
	seedEInvoiceAddress(t, env, "a-einv-abschlag", "c-einv-abschlag", "billing", "Weg 5", "50667", "Koeln", false)

	inv := createBookedInvoice(t, env, arSvc, "c-einv-abschlag", "REF-ABS", []InvoiceItemInput{
		{Description: "Abschlag", Qty: 1, UnitPrice: 1000, TaxCode: "DE19", AccountCode: "8000"},
	})
	if _, err := env.PG.Exec(ctx, `UPDATE invoices_out SET invoice_type='abschlagsrechnung' WHERE id=$1`, inv.ID); err != nil {
		t.Fatalf("invoice_type setzen: %v", err)
	}

	got, err := svc.BuildCIIInvoice(ctx, inv.ID, einvoiceTestCompany)
	if err != nil {
		t.Fatalf("BuildCIIInvoice: %v", err)
	}
	if got.TypeCode != CIITypeCodePrepaymentInvoice {
		t.Errorf("Abschlagsrechnung muss Typ-Code 386 tragen, got %q", got.TypeCode)
	}
}

// TestEInvoiceRejectsUnknownInvoice deckt den Fall einer unbekannten
// Rechnungs-ID ab (fuer die spaetere 404-Zuordnung in E.4.3.3 relevant).
func TestEInvoiceRejectsUnknownInvoice(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc, _ := newEInvoiceTestServices(env)

	_, err := svc.BuildCIIInvoice(ctx, uuid.MustParse("00000000-0000-0000-0000-00000000dead"), einvoiceTestCompany)
	if err == nil {
		t.Fatal("erwartet Fehler fuer unbekannte Rechnung")
	}
	if !strings.Contains(err.Error(), "nicht gefunden") {
		t.Errorf("Fehlermeldung muss den etablierten Substring 'nicht gefunden' enthalten (404-Zuordnung), got: %v", err)
	}
}
