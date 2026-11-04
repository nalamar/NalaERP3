package purchasing

import (
	"context"
	"time"
)

// Hinweise der Nachweise an der Bestellung (ADR 0035 §3/§4, Backlog
// H.4.3d). Regeln in evidence.go (OrderEvidenceHints). Fuer beliebig viele
// Bestellungen genau drei Abfragen: Positionen mit Kennzeichen am
// Material, Lieferpapiere, EN-1090-Zertifikate der Lieferanten.

func (s *Service) withHints(ctx context.Context, orders []PurchaseOrder, companyID string) error {
	if len(orders) == 0 {
		return nil
	}
	orderIDs := make([]string, len(orders))
	supplierIDs := make([]string, 0, len(orders))
	seen := map[string]bool{}
	for i, o := range orders {
		orderIDs[i] = o.ID
		if !seen[o.SupplierID] {
			seen[o.SupplierID] = true
			supplierIDs = append(supplierIDs, o.SupplierID)
		}
	}

	items := map[string][]EvidenceItem{}
	dates := map[string][]*time.Time{}
	rows, err := s.pg.Query(ctx, `
        SELECT i.order_id, i.id, COALESCE(m.nachweis_en1090, false), i.delivery_date
          FROM purchase_order_items i
          LEFT JOIN materials m ON m.id = i.material_id
         WHERE i.order_id = ANY($1)
    `, orderIDs)
	if err != nil {
		return err
	}
	for rows.Next() {
		var orderID string
		var it EvidenceItem
		var d *time.Time
		if err := rows.Scan(&orderID, &it.ID, &it.Nachweis, &d); err != nil {
			rows.Close()
			return err
		}
		items[orderID] = append(items[orderID], it)
		dates[orderID] = append(dates[orderID], d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	papers := map[string][]DeliveryPaper{}
	rows, err = s.pg.Query(ctx, `
        SELECT purchase_order_id, purchase_order_item_id, art, storniert_at IS NOT NULL
          FROM purchase_order_documents WHERE purchase_order_id = ANY($1)
    `, orderIDs)
	if err != nil {
		return err
	}
	for rows.Next() {
		var orderID string
		var p DeliveryPaper
		if err := rows.Scan(&orderID, &p.ItemID, &p.Art, &p.Storniert); err != nil {
			rows.Close()
			return err
		}
		papers[orderID] = append(papers[orderID], p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	certs := map[string][]Certificate{}
	rows, err = s.pg.Query(ctx, `
        SELECT supplier_id, art, exc_bis, gueltig_von, gueltig_bis, storniert_at IS NOT NULL
          FROM supplier_certificates WHERE company_id=$1 AND supplier_id = ANY($2) AND art=$3
    `, companyID, supplierIDs, ZertifikatEN1090)
	if err != nil {
		return err
	}
	for rows.Next() {
		var supplierID string
		var c Certificate
		if err := rows.Scan(&supplierID, &c.Art, &c.ExcBis, &c.GueltigVon, &c.GueltigBis, &c.Storniert); err != nil {
			rows.Close()
			return err
		}
		certs[supplierID] = append(certs[supplierID], c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range orders {
		o := &orders[i]
		o.Hinweise = []string{}
		o.Hinweise = append(o.Hinweise, OrderEvidenceHints(o.Status, items[o.ID], papers[o.ID], certs[o.SupplierID], o.OrderDate, dates[o.ID])...)
	}
	return nil
}

// hintOne: Hinweise fuer eine einzelne Bestellung.
func (s *Service) hintOne(ctx context.Context, po *PurchaseOrder, companyID string) error {
	one := []PurchaseOrder{*po}
	if err := s.withHints(ctx, one, companyID); err != nil {
		return err
	}
	po.Hinweise = one[0].Hinweise
	return nil
}
