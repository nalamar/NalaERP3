package accounting

// Integrationstests fuer die in E.8.1 ergaenzten Steuer- und
// Summenfelder auf Service-Ebene (Backlog E.8.2).

import (
	"context"
	"testing"
	"time"

	"nalaerp3/internal/testutil"
)

func seedTaxTotalsSupplier(t *testing.T, env *testutil.IntegrationEnv, id, name string) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `
        INSERT INTO contacts (id, typ, rolle, name, company_id)
        VALUES ($1,'org','supplier',$2,'default')
        ON CONFLICT (id) DO NOTHING
    `, id, name); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}
}

// TestInvoiceInPersistsTaxAndTotals deckt den vollen Weg ab: schreiben,
// zurueckgeben, wieder laden.
func TestInvoiceInPersistsTaxAndTotals(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)
	seedTaxTotalsSupplier(t, env, "c-e82", "E.8.2 Lieferant")

	due := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	created, items, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID:    "c-e82",
		InvoiceNumber: "E82-1",
		InvoiceDate:   &[]time.Time{time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)}[0],
		DueDate:       &due,
		Currency:      "EUR",
		NetAmount:     1200,
		TaxAmount:     214,
		GrossAmount:   1414,
		Items: []InvoiceInItemInput{
			{Description: "Profil", Qty: 10, UnitPrice: 100, TaxCategory: "S", TaxRate: 19, UnitCode: "MTR"},
			{Description: "Broschüre", Qty: 20, UnitPrice: 10, TaxCategory: "S", TaxRate: 7, UnitCode: "C62"},
		},
		Taxes: []InvoiceInTaxInput{
			{TaxCategory: "S", TaxRate: 19, BasisAmount: 1000, CalculatedAmount: 190},
			{TaxCategory: "S", TaxRate: 7, BasisAmount: 200, CalculatedAmount: 14},
		},
	}, "default")
	if err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}

	// Rueckgabe von Create
	if created.NetAmount != 1200 || created.TaxAmount != 214 || created.GrossAmount != 1414 {
		t.Errorf("Summen in der Create-Rückgabe falsch: %+v", created)
	}
	if created.DueDate == nil || !created.DueDate.Equal(due) {
		t.Errorf("Fälligkeit in der Create-Rückgabe falsch: %v", created.DueDate)
	}
	if len(created.Taxes) != 2 {
		t.Fatalf("erwartet 2 Steuergruppen in der Create-Rückgabe, got %d", len(created.Taxes))
	}
	if len(items) != 2 || items[0].TaxCategory != "S" || items[0].TaxRate != 19 || items[0].UnitCode != "MTR" {
		t.Errorf("Positionsangaben in der Create-Rückgabe falsch: %+v", items)
	}

	// Erneut laden - erst das beweist, dass wirklich gespeichert wurde.
	got, gotItems, err := svc.GetInvoiceIn(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("GetInvoiceIn: %v", err)
	}
	if got.NetAmount != 1200 || got.TaxAmount != 214 || got.GrossAmount != 1414 {
		t.Errorf("Summen nach dem Laden falsch: %+v", got)
	}
	if got.DueDate == nil || !got.DueDate.Equal(due) {
		t.Errorf("Fälligkeit nach dem Laden falsch: %v", got.DueDate)
	}

	if len(gotItems) != 2 {
		t.Fatalf("erwartet 2 Positionen, got %d", len(gotItems))
	}
	byName := map[string]InvoiceInItem{}
	for _, it := range gotItems {
		byName[it.Description] = it
	}
	if p := byName["Profil"]; p.TaxCategory != "S" || p.TaxRate != 19 || p.UnitCode != "MTR" {
		t.Errorf("Position 'Profil' falsch geladen: %+v", p)
	}
	if b := byName["Broschüre"]; b.TaxRate != 7 || b.UnitCode != "C62" {
		t.Errorf("Position 'Broschüre' falsch geladen: %+v", b)
	}

	// Steueraufschluesselung, absteigend nach Satz sortiert.
	if len(got.Taxes) != 2 {
		t.Fatalf("erwartet 2 Steuergruppen, got %d", len(got.Taxes))
	}
	if got.Taxes[0].TaxRate != 19 || got.Taxes[0].BasisAmount != 1000 || got.Taxes[0].CalculatedAmount != 190 {
		t.Errorf("erste Steuergruppe falsch: %+v", got.Taxes[0])
	}
	if got.Taxes[1].TaxRate != 7 || got.Taxes[1].BasisAmount != 200 || got.Taxes[1].CalculatedAmount != 14 {
		t.Errorf("zweite Steuergruppe falsch: %+v", got.Taxes[1])
	}
	if got.Taxes[0].InvoiceInID != created.ID {
		t.Errorf("Steuergruppe nicht der Rechnung zugeordnet: %+v", got.Taxes[0])
	}
}

// TestInvoiceInKeepsSupplierStatedTotals ist der fachliche Kerntest: die
// vom Lieferanten ausgewiesenen Summen werden UNVERAENDERT uebernommen,
// auch wenn sie nicht zu den Positionen passen. Es wird nichts
// nachgerechnet und nichts korrigiert (ADR 0023) - eine abweichende
// Summe ist eine Tatsache der Rechnung.
func TestInvoiceInKeepsSupplierStatedTotals(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)
	seedTaxTotalsSupplier(t, env, "c-e82-abw", "E.8.2 Abweichung")

	created, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID:    "c-e82-abw",
		InvoiceNumber: "E82-ABWEICHEND",
		Currency:      "EUR",
		// Positionen ergeben 100, ausgewiesen ist etwas voellig anderes.
		NetAmount:   100,
		TaxAmount:   19,
		GrossAmount: 999.99,
		Items: []InvoiceInItemInput{
			{Description: "Eine Position", Qty: 1, UnitPrice: 100},
		},
	}, "default")
	if err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}

	got, _, err := svc.GetInvoiceIn(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("GetInvoiceIn: %v", err)
	}
	if got.GrossAmount != 999.99 {
		t.Errorf("der ausgewiesene Bruttobetrag muss unverändert bleiben, got %v", got.GrossAmount)
	}
	if got.NetAmount != 100 || got.TaxAmount != 19 {
		t.Errorf("Netto/Steuer wurden verändert: %+v", got)
	}
}

// TestInvoiceInAcceptsUnknownTaxCategory belegt die Entscheidung aus
// E.8.1: die Steuerkategorie ist eine Angabe des Absenders und wird
// weder gegen tax_codes geprueft noch auf eine Werteliste eingeschraenkt.
func TestInvoiceInAcceptsUnknownTaxCategory(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)
	seedTaxTotalsSupplier(t, env, "c-e82-kat", "E.8.2 Kategorie")

	created, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID:    "c-e82-kat",
		InvoiceNumber: "E82-KAT",
		Currency:      "EUR",
		Items: []InvoiceInItemInput{
			// "Z" ist in unseren tax_codes nicht vorhanden und auch von
			// unserem eigenen Ausgang nie geschrieben.
			{Description: "Sonderfall", Qty: 1, UnitPrice: 50, TaxCategory: "Z", TaxRate: 0},
		},
		Taxes: []InvoiceInTaxInput{
			{TaxCategory: "Z", TaxRate: 0, BasisAmount: 50, ExemptionReason: "Sonderregelung des Lieferanten"},
		},
	}, "default")
	if err != nil {
		t.Fatalf("eine unbekannte Steuerkategorie darf nicht abgelehnt werden: %v", err)
	}

	got, gotItems, err := svc.GetInvoiceIn(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("GetInvoiceIn: %v", err)
	}
	if gotItems[0].TaxCategory != "Z" {
		t.Errorf("Steuerkategorie = %q", gotItems[0].TaxCategory)
	}
	if len(got.Taxes) != 1 || got.Taxes[0].ExemptionReason != "Sonderregelung des Lieferanten" {
		t.Errorf("Befreiungsgrund nicht übernommen: %+v", got.Taxes)
	}
}

// TestInvoiceInWithoutTaxDataStaysCreatable beweist die
// Rueckwaertskompatibilitaet: eine manuell erfasste Rechnung ohne die
// neuen Angaben bleibt anlegbar, die Defaults greifen, und es entsteht
// keine leere Steuergruppe.
func TestInvoiceInWithoutTaxDataStaysCreatable(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)
	seedTaxTotalsSupplier(t, env, "c-e82-leer", "E.8.2 Ohne Angaben")

	created, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID:    "c-e82-leer",
		InvoiceNumber: "E82-LEER",
		Currency:      "EUR",
		Items: []InvoiceInItemInput{
			{Description: "Position ohne Steuerangaben", Qty: 2, UnitPrice: 25},
		},
	}, "default")
	if err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}

	got, gotItems, err := svc.GetInvoiceIn(ctx, created.ID, "default")
	if err != nil {
		t.Fatalf("GetInvoiceIn: %v", err)
	}
	if got.NetAmount != 0 || got.TaxAmount != 0 || got.GrossAmount != 0 {
		t.Errorf("ohne Angaben erwartet 0 als Summen, got %+v", got)
	}
	if got.DueDate != nil {
		t.Errorf("ohne Angabe darf keine Fälligkeit entstehen: %v", got.DueDate)
	}
	if len(got.Taxes) != 0 {
		t.Errorf("ohne Angaben darf keine Steuergruppe entstehen: %+v", got.Taxes)
	}
	if gotItems[0].TaxCategory != "" || gotItems[0].TaxRate != 0 || gotItems[0].UnitCode != "" {
		t.Errorf("ohne Angaben dürfen keine Steuerwerte erfunden werden: %+v", gotItems[0])
	}
}

// TestListInvoicesInReturnsTotalsButNoTaxBreakdown haelt die bewusste
// Design-Entscheidung fest: die Liste liefert die Summen mit, aber nicht
// die Aufschluesselung (die wuerde je Zeile eine zweite Abfrage kosten).
func TestListInvoicesInReturnsTotalsButNoTaxBreakdown(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)
	seedTaxTotalsSupplier(t, env, "c-e82-liste", "E.8.2 Liste")

	if _, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID:    "c-e82-liste",
		InvoiceNumber: "E82-LISTE",
		Currency:      "EUR",
		NetAmount:     500,
		TaxAmount:     95,
		GrossAmount:   595,
		Items:         []InvoiceInItemInput{{Description: "Pos", Qty: 1, UnitPrice: 500}},
		Taxes:         []InvoiceInTaxInput{{TaxCategory: "S", TaxRate: 19, BasisAmount: 500, CalculatedAmount: 95}},
	}, "default"); err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}

	list, err := svc.ListInvoicesIn(ctx, InvoiceInFilter{SupplierID: "c-e82-liste"}, "default")
	if err != nil {
		t.Fatalf("ListInvoicesIn: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("erwartet 1 Rechnung, got %d", len(list))
	}
	if list[0].GrossAmount != 595 || list[0].NetAmount != 500 {
		t.Errorf("Summen fehlen in der Liste: %+v", list[0])
	}
	if len(list[0].Taxes) != 0 {
		t.Errorf("die Liste soll die Aufschlüsselung bewusst NICHT laden, got %+v", list[0].Taxes)
	}
}
