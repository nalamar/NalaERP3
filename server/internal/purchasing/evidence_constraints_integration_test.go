package purchasing

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

type seededEvidence struct {
	company, supplier, material, order, item string
	project                                  uuid.UUID
}

func seedEvidence(t *testing.T, env *testutil.IntegrationEnv) seededEvidence {
	t.Helper()
	ctx := context.Background()
	n := uuid.NewString()[:8]
	s := seededEvidence{company: "ev-" + n, supplier: "sup-ev-" + n, material: "mat-ev-" + n, order: "po-ev-" + n, item: "poi-ev-" + n, project: uuid.New()}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, []any{s.company, "Nachweistest " + s.company}},
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Testlieferant',$2)`, []any{s.supplier, s.company}},
		{`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$2,'Testprofil','profil','Stk',$3)`, []any{s.material, "MAT-EV-" + n, s.company}},
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, 'P-1', 'Testprojekt', $2)`, []any{s.project, s.company}},
		{`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$3,'EUR','draft',$4)`, []any{s.order, s.supplier, "PO-EV-" + n, s.company}},
		{`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,1,$3,5,'Stk',0,'EUR')`, []any{s.item, s.order, s.material}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("Testdaten: %v", err)
		}
	}
	return s
}

// Die Datenbank selbst schuetzt die Regeln der Nachweise der Zulieferer
// (Migration 110), auch gegen Direkt-SQL (Backlog H.4.1a).
func TestSupplierEvidenceTableConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	ok := func(name, sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: abgelehnt: %v", name, err)
		}
	}
	bad := func(name, sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err == nil {
			t.Fatalf("%s: von der Datenbank angenommen", name)
		}
	}

	cert := `INSERT INTO supplier_certificates (id, company_id, supplier_id, art, exc_bis, gueltig_von, gueltig_bis, storno_reason, storniert_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	c := func(art string, exc, von, bis any, grund string, storno any) []any {
		return []any{uuid.New(), s.company, s.supplier, art, exc, von, bis, grund, storno}
	}
	certID := uuid.New()
	args := c("en1090", "EXC2", "2026-01-01", "2028-12-31", "", nil)
	args[0] = certID
	ok("EN 1090 mit EXC und Gueltigkeit", cert, args...)
	ok("EN 1090 ohne Enddatum", cert, c("en1090", "EXC4", nil, nil, "", nil)...)
	ok("ISO 3834 ohne EXC", cert, c("iso3834", nil, "2026-01-01", "2026-01-01", "", nil)...)
	ok("storniert mit Grund", cert, c("sonstiges", nil, nil, nil, "falscher Lieferant", "2026-11-03T08:00:00Z")...)

	bad("Art unbekannt", cert, c("iso9001", nil, nil, nil, "", nil)...)
	bad("EN 1090 ohne EXC", cert, c("en1090", nil, nil, nil, "", nil)...)
	bad("EXC unbekannt", cert, c("en1090", "EXC5", nil, nil, "", nil)...)
	bad("EXC bei ISO 3834", cert, c("iso3834", "EXC2", nil, nil, "", nil)...)
	bad("gueltig bis vor gueltig von", cert, c("en1090", "EXC2", "2026-02-01", "2026-01-31", "", nil)...)
	bad("storniert ohne Grund", cert, c("en1090", "EXC2", nil, nil, " ", "2026-11-03T08:00:00Z")...)
	bad("Grund ohne Storno", cert, c("en1090", "EXC2", nil, nil, "Grund", nil)...)
	args = c("en1090", "EXC2", nil, nil, "", nil)
	args[2] = "kein-kontakt"
	bad("Lieferant unbekannt", cert, args...)

	certDoc := `INSERT INTO supplier_certificate_documents (id, certificate_id, document_id, filename, content_type, length) VALUES ($1,$2,$3,'z.pdf','application/pdf',$4)`
	ok("Datei zum Zertifikat", certDoc, uuid.New(), certID, "gfs-c1", 10)
	bad("Datei doppelt", certDoc, uuid.New(), certID, "gfs-c1", 10)
	bad("Laenge negativ", certDoc, uuid.New(), certID, "gfs-c2", -1)
	bad("Zertifikat mit Datei loeschen", `DELETE FROM supplier_certificates WHERE id=$1`, certID)
	// Zweiter Lieferant ohne Bestellung: nur das Zertifikat haelt ihn.
	other := s.supplier + "-2"
	ok("zweiter Lieferant", `INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Zweitlieferant',$2)`, other, s.company)
	args = c("iso3834", nil, nil, nil, "", nil)
	args[2] = other
	ok("Zertifikat am zweiten Lieferanten", cert, args...)
	bad("Lieferant mit Zertifikat loeschen", `DELETE FROM contacts WHERE id=$1`, other)

	doc := `INSERT INTO purchase_order_documents (id, purchase_order_id, purchase_order_item_id, art, document_id, filename, content_type, length, storno_reason, storniert_at)
		VALUES ($1,$2,$3,$4,$5,'l.pdf','application/pdf',1,$6,$7)`
	docID := uuid.New()
	ok("Leistungserklaerung zur Position", doc, docID, s.order, s.item, "leistungserklaerung", "gfs-d1", "", nil)
	ok("Zeugnis 3.1 ohne Position", doc, uuid.New(), s.order, nil, "zeugnis_3_1", "gfs-d2", "", nil)
	ok("Lieferschein storniert", doc, uuid.New(), s.order, nil, "lieferschein", "gfs-d3", "falsche Datei", "2026-11-03T08:00:00Z")
	bad("Art unbekannt", doc, uuid.New(), s.order, nil, "zeugnis_3_1_b", "gfs-d4", "", nil)
	bad("storniert ohne Grund", doc, uuid.New(), s.order, nil, "lieferschein", "gfs-d5", "", "2026-11-03T08:00:00Z")
	bad("Datei doppelt", doc, uuid.New(), s.order, nil, "lieferschein", "gfs-d1", "", nil)
	bad("Bestellung unbekannt", doc, uuid.New(), "kein-po", nil, "lieferschein", "gfs-d6", "", nil)
	bad("Bestellung mit Lieferpapier loeschen", `DELETE FROM purchase_orders WHERE id=$1`, s.order)

	// Position geloescht: das Lieferpapier bleibt an der Bestellung.
	ok("Position loeschen", `DELETE FROM purchase_order_items WHERE id=$1`, s.item)
	var item *string
	if err := env.PG.QueryRow(ctx, `SELECT purchase_order_item_id FROM purchase_order_documents WHERE id=$1`, docID).Scan(&item); err != nil {
		t.Fatalf("Lieferpapier nach Loeschen der Position: %v", err)
	}
	if item != nil {
		t.Fatalf("Position am Lieferpapier = %v, erwartet NULL", *item)
	}

	// Kennzeichen am Material: Vorgabe false.
	var flag bool
	if err := env.PG.QueryRow(ctx, `SELECT nachweis_en1090 FROM materials WHERE id=$1`, s.material).Scan(&flag); err != nil || flag {
		t.Fatalf("nachweis_en1090 = %v (%v), erwartet false", flag, err)
	}

	// Projekt an der Bestellung: optional, muss existieren, haelt das Projekt.
	bad("Projekt unbekannt", `UPDATE purchase_orders SET project_id=$2 WHERE id=$1`, s.order, uuid.New())
	ok("Projekt setzen", `UPDATE purchase_orders SET project_id=$2 WHERE id=$1`, s.order, s.project)
	bad("Projekt mit Bestellung loeschen", `DELETE FROM projects WHERE id=$1`, s.project)
}
