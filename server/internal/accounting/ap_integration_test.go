package accounting

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// APService hat zum Zeitpunkt von D.3.3.2 noch KEIN HTTP-Wiring (das
// folgt in D.3.3.3) - diese Datei testet die Kernlogik direkt gegen
// echtes Postgres, analog zum bereits etablierten Muster in
// bank_integration_test.go (Backlog 0.37) und
// internal/hr/service_scoping_integration_test.go (Backlog 0.9).

// seedAPFixture legt Lieferant, Material, Lager und eine Bestellung mit
// einer Position direkt per SQL an (kein Import von purchasing/materials
// noetig) - der gemeinsame Ausgangszustand fuer die APService-Tests.
func seedAPFixture(t *testing.T, ctx context.Context, env *testutil.IntegrationEnv, companyID string, poQty, poPrice float64) (supplierID, poID, poItemID, materialID, warehouseID string) {
	t.Helper()
	nonce := uuid.NewString()[:8]

	supplierID = "sup-" + nonce
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier',$2,$3)
    `, supplierID, "AP-Testlieferant "+nonce, companyID); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}

	materialID = "mat-" + nonce
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$2,'AP-Testmaterial','profil','Stk',$3)
    `, materialID, "MAT-AP-"+nonce, companyID); err != nil {
		t.Fatalf("seed material: %v", err)
	}

	warehouseID = "wh-" + nonce
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$2,'AP-Testlager',$3)
    `, warehouseID, "WH-AP-"+nonce, companyID); err != nil {
		t.Fatalf("seed warehouse: %v", err)
	}

	poID = "po-" + nonce
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$3,'EUR','draft',$4)
    `, poID, supplierID, "PO-AP-"+nonce, companyID); err != nil {
		t.Fatalf("seed purchase order: %v", err)
	}

	poItemID = "poi-" + nonce
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,1,$3,$4,'Stk',$5,'EUR')
    `, poItemID, poID, materialID, poQty, poPrice); err != nil {
		t.Fatalf("seed purchase order item: %v", err)
	}

	return supplierID, poID, poItemID, materialID, warehouseID
}

func seedReceivedStock(t *testing.T, ctx context.Context, env *testutil.IntegrationEnv, materialID, warehouseID, poItemID string, qty float64) {
	t.Helper()
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO stock_movements (id, material_id, warehouse_id, quantity, uom, movement_type, purchase_order_item_id)
        VALUES ($1,$2,$3,$4,'Stk','purchase',$5)
    `, uuid.NewString(), materialID, warehouseID, qty, poItemID); err != nil {
		t.Fatalf("seed received stock movement: %v", err)
	}
}

func TestAPServiceCreateInvoiceInAndMatchWithFullReceipt(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)

	supplierID, poID, poItemID, materialID, warehouseID := seedAPFixture(t, ctx, env, "default", 10, 5.5)
	seedReceivedStock(t, ctx, env, materialID, warehouseID, poItemID, 10)

	poIDCopy := poID
	poItemIDCopy := poItemID
	invoice, items, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID:      supplierID,
		PurchaseOrderID: &poIDCopy,
		InvoiceNumber:   "RE-2026-001",
		Items: []InvoiceInItemInput{
			{PurchaseOrderItemID: &poItemIDCopy, Description: "Testposition", Qty: 10, UnitPrice: 5.5},
		},
	}, "default")
	if err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}
	if invoice.Status != "erfasst" {
		t.Fatalf("expected status=erfasst, got %+v", invoice)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %+v", items)
	}

	got, gotItems, err := svc.GetInvoiceIn(ctx, invoice.ID, "default")
	if err != nil {
		t.Fatalf("GetInvoiceIn: %v", err)
	}
	if got.InvoiceNumber != "RE-2026-001" || len(gotItems) != 1 {
		t.Fatalf("unexpected get result: %+v / %+v", got, gotItems)
	}

	match, err := svc.MatchInvoiceIn(ctx, invoice.ID, "default")
	if err != nil {
		t.Fatalf("MatchInvoiceIn: %v", err)
	}
	if len(match.Lines) != 1 {
		t.Fatalf("expected 1 match line, got %+v", match.Lines)
	}
	line := match.Lines[0]
	if !line.Matchable {
		t.Fatalf("expected line to be matchable, got %+v", line)
	}
	if line.BestelltQty == nil || *line.BestelltQty != 10 {
		t.Fatalf("expected bestellt_menge=10, got %+v", line.BestelltQty)
	}
	if line.ErhaltenQty == nil || *line.ErhaltenQty != 10 {
		t.Fatalf("expected erhalten_menge=10, got %+v", line.ErhaltenQty)
	}
	if line.MengeStimmt == nil || !*line.MengeStimmt {
		t.Fatalf("expected menge_stimmt=true (invoiced 10 == received 10), got %+v", line.MengeStimmt)
	}
	if line.PreisStimmt == nil || !*line.PreisStimmt {
		t.Fatalf("expected preis_stimmt=true (invoiced 5.5 == ordered 5.5), got %+v", line.PreisStimmt)
	}
}

func TestAPServiceMatchInvoiceInDetectsPartialReceiptAndPriceMismatch(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)

	supplierID, _, poItemID, materialID, warehouseID := seedAPFixture(t, ctx, env, "default", 10, 5.5)

	// Nur 6 von 10 bestellten Stueck tatsaechlich eingegangen.
	seedReceivedStock(t, ctx, env, materialID, warehouseID, poItemID, 6)

	poItemIDCopy := poItemID
	invoice, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID: supplierID,
		Items: []InvoiceInItemInput{
			// Rechnung ueber alle 10 bestellten Stueck, obwohl nur 6 eingegangen sind,
			// UND zu einem abweichenden Preis (6.0 statt bestellter 5.5).
			{PurchaseOrderItemID: &poItemIDCopy, Description: "Testposition", Qty: 10, UnitPrice: 6.0},
		},
	}, "default")
	if err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}

	match, err := svc.MatchInvoiceIn(ctx, invoice.ID, "default")
	if err != nil {
		t.Fatalf("MatchInvoiceIn: %v", err)
	}
	if len(match.Lines) != 1 {
		t.Fatalf("expected 1 match line, got %+v", match.Lines)
	}
	line := match.Lines[0]
	if line.ErhaltenQty == nil || *line.ErhaltenQty != 6 {
		t.Fatalf("expected erhalten_menge=6, got %+v", line.ErhaltenQty)
	}
	if line.MengeStimmt == nil || *line.MengeStimmt {
		t.Fatalf("expected menge_stimmt=false (invoiced 10 != received 6), got %+v", line.MengeStimmt)
	}
	if line.PreisStimmt == nil || *line.PreisStimmt {
		t.Fatalf("expected preis_stimmt=false (invoiced 6.0 != ordered 5.5), got %+v", line.PreisStimmt)
	}
}

func TestAPServiceMatchInvoiceInMarksUnlinkedItemAsNotMatchable(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)

	supplierID, _, _, _, _ := seedAPFixture(t, ctx, env, "default", 10, 5.5)

	invoice, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID: supplierID,
		Items: []InvoiceInItemInput{
			{Description: "Nebenkosten ohne Bestellbezug", Qty: 1, UnitPrice: 42},
		},
	}, "default")
	if err != nil {
		t.Fatalf("CreateInvoiceIn: %v", err)
	}

	match, err := svc.MatchInvoiceIn(ctx, invoice.ID, "default")
	if err != nil {
		t.Fatalf("MatchInvoiceIn: %v", err)
	}
	if len(match.Lines) != 1 {
		t.Fatalf("expected 1 match line, got %+v", match.Lines)
	}
	line := match.Lines[0]
	if line.Matchable {
		t.Fatalf("expected line without purchase_order_item_id to be not matchable, got %+v", line)
	}
	if line.BestelltQty != nil || line.ErhaltenQty != nil || line.MengeStimmt != nil || line.PreisStimmt != nil {
		t.Fatalf("expected all comparison fields to be nil for an unlinked item, got %+v", line)
	}
}

func TestAPServiceCreateInvoiceInRejectsUnknownSupplier(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)

	_, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID: "00000000-0000-0000-0000-000000000000",
		Items:      []InvoiceInItemInput{{Qty: 1, UnitPrice: 10}},
	}, "default")
	if err == nil {
		t.Fatal("expected error for unknown supplier, got nil")
	}
	if err.Error() != "Lieferant nicht gefunden" {
		t.Fatalf("expected Lieferant nicht gefunden, got %q", err.Error())
	}
}

func TestAPServiceListInvoicesInFiltersBySupplier(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewAPService(env.PG)

	supplierA, _, _, _, _ := seedAPFixture(t, ctx, env, "default", 10, 5.5)
	supplierB, _, _, _, _ := seedAPFixture(t, ctx, env, "default", 10, 5.5)

	if _, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID: supplierA,
		Items:      []InvoiceInItemInput{{Qty: 1, UnitPrice: 10}},
	}, "default"); err != nil {
		t.Fatalf("CreateInvoiceIn (A): %v", err)
	}
	if _, _, err := svc.CreateInvoiceIn(ctx, InvoiceInCreate{
		SupplierID: supplierB,
		Items:      []InvoiceInItemInput{{Qty: 1, UnitPrice: 10}},
	}, "default"); err != nil {
		t.Fatalf("CreateInvoiceIn (B): %v", err)
	}

	list, err := svc.ListInvoicesIn(ctx, InvoiceInFilter{SupplierID: supplierA}, "default")
	if err != nil {
		t.Fatalf("ListInvoicesIn: %v", err)
	}
	if len(list) != 1 || list[0].SupplierID != supplierA {
		t.Fatalf("expected exactly 1 invoice for supplier A, got %+v", list)
	}
}
