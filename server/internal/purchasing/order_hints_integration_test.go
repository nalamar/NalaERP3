package purchasing

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Hinweise der Nachweise an der Bestellung (Backlog H.4.3d). Daten in 2031,
// damit "heute" keine Rolle spielt.
func TestPurchaseOrderEvidenceHints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	other := seedEvidence(t, env)
	svc := NewService(env.PG)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	hints := func(id string) []string {
		t.Helper()
		po, _, err := svc.Get(ctx, id, s.company)
		if err != nil {
			t.Fatal(err)
		}
		if po.Hinweise == nil {
			t.Fatal("Hinweise nil statt leer")
		}
		return po.Hinweise
	}
	want := func(name, id string, exp ...string) {
		t.Helper()
		if got := hints(id); !slices.Equal(got, append([]string{}, exp...)) {
			t.Fatalf("%s: %v, erwartet %v", name, got, exp)
		}
	}
	cert := func(supplier, company, art string, exc any, bis any) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`INSERT INTO supplier_certificates (id, company_id, supplier_id, art, exc_bis, gueltig_bis) VALUES ($1,$2,$3,$4,$5,$6)`, id, company, supplier, art, exc, bis)
		return id
	}
	exec(`UPDATE purchase_orders SET order_date='2031-01-10' WHERE id=$1`, s.order)
	exec(`UPDATE purchase_order_items SET delivery_date='2031-02-15' WHERE id=$1`, s.item)

	want("ohne Kennzeichen", s.order)
	exec(`UPDATE materials SET nachweis_en1090=true WHERE id=$1`, s.material)
	want("ohne Zertifikat", s.order, HintZertifikatFehlt)

	// Zaehlen nicht: ISO 3834, EN 1090 eines anderen Lieferanten.
	cert(s.supplier, s.company, "iso3834", nil, nil)
	cert(other.supplier, other.company, "en1090", "EXC2", nil)
	want("nur fremde/andere", s.order, HintZertifikatFehlt)
	// Endet vor dem Liefertermin.
	cert(s.supplier, s.company, "en1090", "EXC2", "2031-02-14")
	want("endet vor Liefertermin", s.order, HintZertifikatUngueltig)
	// Storniert zaehlt nicht.
	exec(`INSERT INTO supplier_certificates (id, company_id, supplier_id, art, exc_bis, storno_reason, storniert_at) VALUES ($1,$2,$3,'en1090','EXC2','ersetzt',now())`, uuid.New(), s.company, s.supplier)
	want("storniert", s.order, HintZertifikatUngueltig)
	// Gilt genau bis zum Liefertermin.
	cert(s.supplier, s.company, "en1090", "EXC3", "2031-02-15")
	want("gilt bis Liefertermin", s.order)

	// Wareneingang: Leistungserklaerung fehlt; Zeugnis ersetzt sie nicht.
	exec(`UPDATE purchase_orders SET status='received' WHERE id=$1`, s.order)
	want("received", s.order, HintLeistungserklaerungFehlt)
	paper := func(art string, item any) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`INSERT INTO purchase_order_documents (id, purchase_order_id, purchase_order_item_id, art, document_id, filename, content_type, length)
              VALUES ($1,$2,$3,$4,$5,'x.pdf','application/pdf',1)`, id, s.order, item, art, uuid.NewString())
		return id
	}
	paper("zeugnis_3_1", s.item)
	want("nur Zeugnis", s.order, HintLeistungserklaerungFehlt)
	dop := paper("leistungserklaerung", s.item)
	want("mit Leistungserklaerung", s.order)
	exec(`UPDATE purchase_order_documents SET storniert_at=now(), storno_reason='falsch' WHERE id=$1`, dop)
	want("Leistungserklaerung storniert", s.order, HintLeistungserklaerungFehlt)
	paper("leistungserklaerung", nil)
	want("fuer die ganze Bestellung", s.order)

	// Liste: jede Bestellung mit ihren eigenen Hinweisen. Zweite Bestellung
	// desselben Lieferanten ohne Zertifikat-Deckung (Liefertermin spaeter),
	// dritte ohne nachweispflichtiges Material.
	o2, o3 := s.order+"-2", s.order+"-3"
	exec(`INSERT INTO purchase_orders (id, supplier_id, number, order_date, currency, status, company_id) VALUES ($1,$2,$1,'2031-01-10','EUR','ordered',$3)`, o2, s.supplier, s.company)
	exec(`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency, delivery_date) VALUES ($1,$2,1,$3,1,'Stk',0,'EUR','2031-03-01')`, o2+"-i", o2, s.material)
	exec(`INSERT INTO purchase_orders (id, supplier_id, number, order_date, currency, status, company_id) VALUES ($1,$2,$1,'2031-01-10','EUR','received',$3)`, o3, s.supplier, s.company)
	list, err := svc.List(ctx, PurchaseOrderFilter{SupplierID: s.supplier}, s.company)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string][]string{}
	for _, o := range list {
		if o.Hinweise == nil {
			t.Fatalf("Liste: Hinweise nil bei %s", o.ID)
		}
		byID[o.ID] = o.Hinweise
	}
	if len(byID) != 3 || len(byID[s.order]) != 0 || !slices.Equal(byID[o2], []string{HintZertifikatUngueltig}) || len(byID[o3]) != 0 {
		t.Fatalf("Liste: %v", byID)
	}

	// Stornierte Bestellung: keine Hinweise.
	exec(`UPDATE purchase_orders SET status='canceled' WHERE id=$1`, o2)
	want("canceled", o2)

	// Neu angelegte Bestellung bringt ihre Hinweise gleich mit.
	po, _, err := svc.Create(ctx, PurchaseOrderCreate{Number: "PO-HINT-" + uuid.NewString()[:13], SupplierID: other.supplier,
		Items: []PurchaseOrderItemInput{{MaterialID: other.material, Qty: 1, UOM: "Stk"}}}, other.company)
	if err != nil || po.Hinweise == nil || len(po.Hinweise) != 0 {
		t.Fatalf("angelegt ohne Kennzeichen: %+v, %v", po, err)
	}
	exec(`UPDATE materials SET nachweis_en1090=true WHERE id=$1`, other.material)
	exec(`UPDATE supplier_certificates SET storniert_at=now(), storno_reason='x' WHERE supplier_id=$1`, other.supplier)
	po, _, err = svc.Create(ctx, PurchaseOrderCreate{Number: "PO-HINT-" + uuid.NewString()[:13], SupplierID: other.supplier,
		Items: []PurchaseOrderItemInput{{MaterialID: other.material, Qty: 1, UOM: "Stk"}}}, other.company)
	if err != nil || !slices.Equal(po.Hinweise, []string{HintZertifikatFehlt}) {
		t.Fatalf("angelegt mit Kennzeichen: %+v, %v", po, err)
	}
}
