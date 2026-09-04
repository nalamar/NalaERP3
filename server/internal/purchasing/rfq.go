package purchasing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/settings"
)

// Anfrageprozess (RFQ) vor Bestellung (ADR 0017). Ein Gewinner-Lieferant
// je gesamter Anfrage, keine automatische Rollen-Prüfung (siehe ADR 0017
// für die Begründung, warum das konsistent zu Create ist).

type RFQ struct {
	ID       string     `json:"id"`
	Number   string     `json:"nummer"`
	Status   string     `json:"status"`
	Note     string     `json:"notiz"`
	Angelegt time.Time  `json:"angelegt_am"`
	Closed   *time.Time `json:"abgeschlossen_am"`
}

type RFQItem struct {
	ID          string  `json:"id"`
	RFQID       string  `json:"anfrage_id"`
	Position    int     `json:"position"`
	MaterialID  string  `json:"material_id"`
	Description string  `json:"bezeichnung"`
	Qty         float64 `json:"menge"`
	UOM         string  `json:"einheit"`
}

type RFQCreate struct {
	Number string         `json:"nummer"`
	Note   string         `json:"notiz"`
	Items  []RFQItemInput `json:"positionen"`
}

type RFQItemInput struct {
	MaterialID  string  `json:"material_id"`
	Description string  `json:"bezeichnung"`
	Qty         float64 `json:"menge"`
	UOM         string  `json:"einheit"`
}

// RFQStatuses spiegelt das Muster von purchasing.Statuses() - Validierung
// nur im Anwendungscode, kein DB-CHECK (ADR 0017, Konsistenz mit
// purchase_orders.status).
func RFQStatuses() []string { return []string{"offen", "abgeschlossen", "storniert"} }

func (s *Service) CreateRFQ(ctx context.Context, in RFQCreate, companyID string) (*RFQ, []RFQItem, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	if len(in.Items) == 0 {
		return nil, nil, errors.New("mindestens eine Position erforderlich")
	}
	for _, it := range in.Items {
		if strings.TrimSpace(it.MaterialID) == "" || it.Qty == 0 || strings.TrimSpace(it.UOM) == "" {
			return nil, nil, errors.New("Ungültige Position")
		}
	}

	if strings.TrimSpace(in.Number) == "" {
		numSvc := settings.NewNumberingService(s.pg)
		if n, err := numSvc.Next(ctx, "rfq", companyID); err == nil {
			in.Number = n
		}
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	id := uuid.NewString()
	var rfq RFQ
	if err := tx.QueryRow(ctx, `
        INSERT INTO rfqs (id, company_id, nummer, note)
        VALUES ($1,$2,$3,$4)
        RETURNING id, nummer, status, COALESCE(note,''), created_at, closed_at
    `, id, companyID, in.Number, in.Note).Scan(&rfq.ID, &rfq.Number, &rfq.Status, &rfq.Note, &rfq.Angelegt, &rfq.Closed); err != nil {
		return nil, nil, err
	}

	items := make([]RFQItem, 0, len(in.Items))
	pos := 1
	for _, it := range in.Items {
		iid := uuid.NewString()
		var out RFQItem
		if err := tx.QueryRow(ctx, `
            INSERT INTO rfq_items (id, rfq_id, position, material_id, description, qty, uom)
            VALUES ($1,$2,$3,$4,$5,$6,$7)
            RETURNING id, rfq_id, position, material_id, COALESCE(description,''), qty, uom
        `, iid, rfq.ID, pos, it.MaterialID, it.Description, it.Qty, it.UOM).Scan(
			&out.ID, &out.RFQID, &out.Position, &out.MaterialID, &out.Description, &out.Qty, &out.UOM,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, out)
		pos++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &rfq, items, nil
}

func (s *Service) GetRFQ(ctx context.Context, id string, companyID string) (*RFQ, []RFQItem, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	var rfq RFQ
	err := s.pg.QueryRow(ctx, `
        SELECT id, nummer, status, COALESCE(note,''), created_at, closed_at
        FROM rfqs WHERE id=$1 AND company_id=$2
    `, id, companyID).Scan(&rfq.ID, &rfq.Number, &rfq.Status, &rfq.Note, &rfq.Angelegt, &rfq.Closed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, errors.New("Anfrage nicht gefunden")
		}
		return nil, nil, err
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, rfq_id, position, material_id, COALESCE(description,''), qty, uom
        FROM rfq_items WHERE rfq_id=$1 ORDER BY position ASC
    `, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]RFQItem, 0)
	for rows.Next() {
		var it RFQItem
		if err := rows.Scan(&it.ID, &it.RFQID, &it.Position, &it.MaterialID, &it.Description, &it.Qty, &it.UOM); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	return &rfq, items, nil
}

func (s *Service) ListRFQs(ctx context.Context, companyID string) ([]RFQ, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, nummer, status, COALESCE(note,''), created_at, closed_at
        FROM rfqs WHERE company_id=$1 ORDER BY created_at DESC
    `, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RFQ, 0)
	for rows.Next() {
		var rfq RFQ
		if err := rows.Scan(&rfq.ID, &rfq.Number, &rfq.Status, &rfq.Note, &rfq.Angelegt, &rfq.Closed); err != nil {
			return nil, err
		}
		out = append(out, rfq)
	}
	return out, nil
}

type SupplierQuote struct {
	ID           string     `json:"id"`
	RFQItemID    string     `json:"anfrage_position_id"`
	SupplierID   string     `json:"lieferant_id"`
	UnitPrice    float64    `json:"preis"`
	Currency     string     `json:"waehrung"`
	DeliveryDate *time.Time `json:"liefertermin"`
	Note         string     `json:"notiz"`
	QuotedAt     time.Time  `json:"angebot_am"`
}

type SupplierQuoteInput struct {
	SupplierID   string     `json:"lieferant_id"`
	UnitPrice    float64    `json:"preis"`
	Currency     string     `json:"waehrung"`
	DeliveryDate *time.Time `json:"liefertermin"`
	Note         string     `json:"notiz"`
}

// RegisterSupplierQuote legt eine Lieferanten-Offerte fuer eine
// Anfrage-Position an oder aktualisiert sie (Upsert, ADR 0017 - vor der
// Bestellung ist eine Offerte keine GoBD-relevante Historie). Lehnt ab,
// wenn die zugehoerige Anfrage nicht mehr 'offen' ist.
func (s *Service) RegisterSupplierQuote(ctx context.Context, rfqItemID string, in SupplierQuoteInput, companyID string) (*SupplierQuote, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(rfqItemID) == "" || strings.TrimSpace(in.SupplierID) == "" {
		return nil, errors.New("Anfrage-Position und Lieferant erforderlich")
	}
	if in.UnitPrice <= 0 {
		return nil, errors.New("Preis muss größer als 0 sein")
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var rfqStatus string
	err = tx.QueryRow(ctx, `
        SELECT r.status
        FROM rfq_items ri
        JOIN rfqs r ON r.id = ri.rfq_id
        WHERE ri.id=$1 AND r.company_id=$2
    `, rfqItemID, companyID).Scan(&rfqStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Anfrage-Position nicht gefunden")
		}
		return nil, err
	}
	if rfqStatus != "offen" {
		return nil, errors.New("Anfrage ist nicht mehr offen")
	}

	var supplierOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contacts WHERE id=$1 AND company_id=$2)`, in.SupplierID, companyID).Scan(&supplierOwned); err != nil {
		return nil, err
	}
	if !supplierOwned {
		return nil, errors.New("Lieferant nicht gefunden")
	}

	var out SupplierQuote
	if err := tx.QueryRow(ctx, `
        INSERT INTO rfq_supplier_quotes (id, rfq_item_id, supplier_id, unit_price, currency, delivery_date, note)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        ON CONFLICT (rfq_item_id, supplier_id) DO UPDATE SET
            unit_price = EXCLUDED.unit_price,
            currency = EXCLUDED.currency,
            delivery_date = EXCLUDED.delivery_date,
            note = EXCLUDED.note,
            quoted_at = now()
        RETURNING id, rfq_item_id, supplier_id, unit_price, currency, delivery_date, COALESCE(note,''), quoted_at
    `, uuid.NewString(), rfqItemID, in.SupplierID, in.UnitPrice, in.Currency, in.DeliveryDate, in.Note).Scan(
		&out.ID, &out.RFQItemID, &out.SupplierID, &out.UnitPrice, &out.Currency, &out.DeliveryDate, &out.Note, &out.QuotedAt,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) ListSupplierQuotes(ctx context.Context, rfqID string, companyID string) ([]SupplierQuote, error) {
	if strings.TrimSpace(rfqID) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rfqs WHERE id=$1 AND company_id=$2)`, rfqID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Anfrage nicht gefunden")
	}

	rows, err := s.pg.Query(ctx, `
        SELECT q.id, q.rfq_item_id, q.supplier_id, q.unit_price, q.currency, q.delivery_date, COALESCE(q.note,''), q.quoted_at
        FROM rfq_supplier_quotes q
        JOIN rfq_items ri ON ri.id = q.rfq_item_id
        WHERE ri.rfq_id=$1
        ORDER BY ri.position ASC, q.unit_price ASC
    `, rfqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SupplierQuote, 0)
	for rows.Next() {
		var q SupplierQuote
		if err := rows.Scan(&q.ID, &q.RFQItemID, &q.SupplierID, &q.UnitPrice, &q.Currency, &q.DeliveryDate, &q.Note, &q.QuotedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// CancelRFQ storniert eine offene Anfrage (Festschreibung - keine
// weiteren Offerten danach moeglich).
func (s *Service) CancelRFQ(ctx context.Context, id string, companyID string) (*RFQ, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM rfqs WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Anfrage nicht gefunden")
		}
		return nil, err
	}
	if status != "offen" {
		return nil, errors.New("Anfrage ist nicht mehr offen")
	}

	var rfq RFQ
	if err := tx.QueryRow(ctx, `
        UPDATE rfqs SET status='storniert', closed_at=now()
        WHERE id=$1
        RETURNING id, nummer, status, COALESCE(note,''), created_at, closed_at
    `, id).Scan(&rfq.ID, &rfq.Number, &rfq.Status, &rfq.Note, &rfq.Angelegt, &rfq.Closed); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &rfq, nil
}

// ConvertToPurchaseOrder waehlt EINEN Lieferanten als Gewinner der
// gesamten Anfrage (ADR 0017 - kein Splitting ueber mehrere Lieferanten).
// Lehnt ab, wenn der Lieferant nicht fuer JEDE Position eine Offerte
// abgegeben hat. Erzeugt eine neue PurchaseOrder (gleiches Insert-Muster
// wie Create, hier dupliziert statt Create aufgerufen, um Anfrage- und
// Bestellungs-Anlage in EINER Transaktion atomar zu halten).
func (s *Service) ConvertToPurchaseOrder(ctx context.Context, rfqID string, supplierID string, companyID string) (*PurchaseOrder, []PurchaseOrderItem, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(rfqID) == "" || strings.TrimSpace(supplierID) == "" {
		return nil, nil, errors.New("Anfrage und Lieferant erforderlich")
	}

	numSvc := settings.NewNumberingService(s.pg)
	number, numErr := numSvc.Next(ctx, "purchase_order", companyID)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var rfqStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM rfqs WHERE id=$1 AND company_id=$2 FOR UPDATE`, rfqID, companyID).Scan(&rfqStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, errors.New("Anfrage nicht gefunden")
		}
		return nil, nil, err
	}
	if rfqStatus != "offen" {
		return nil, nil, errors.New("Anfrage ist nicht mehr offen")
	}

	rows, err := tx.Query(ctx, `
        SELECT ri.material_id, ri.description, ri.qty, ri.uom,
               q.unit_price, q.currency, q.delivery_date
        FROM rfq_items ri
        LEFT JOIN rfq_supplier_quotes q ON q.rfq_item_id = ri.id AND q.supplier_id = $2
        WHERE ri.rfq_id = $1
        ORDER BY ri.position ASC
    `, rfqID, supplierID)
	if err != nil {
		return nil, nil, err
	}
	type rfqItemWithQuote struct {
		materialID   string
		description  string
		qty          float64
		uom          string
		unitPrice    *float64
		currency     *string
		deliveryDate *time.Time
	}
	var itemsWithQuotes []rfqItemWithQuote
	for rows.Next() {
		var r rfqItemWithQuote
		if err := rows.Scan(&r.materialID, &r.description, &r.qty, &r.uom, &r.unitPrice, &r.currency, &r.deliveryDate); err != nil {
			rows.Close()
			return nil, nil, err
		}
		itemsWithQuotes = append(itemsWithQuotes, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(itemsWithQuotes) == 0 {
		return nil, nil, errors.New("Anfrage hat keine Positionen")
	}
	for _, r := range itemsWithQuotes {
		if r.unitPrice == nil {
			return nil, nil, errors.New("Lieferant hat nicht für alle Positionen ein Angebot abgegeben")
		}
	}

	if numErr != nil || strings.TrimSpace(number) == "" {
		number = uuid.NewString()
	}

	id := uuid.NewString()
	var po PurchaseOrder
	if err := tx.QueryRow(ctx, `
        INSERT INTO purchase_orders (id, supplier_id, number, order_date, currency, status, note, company_id)
        VALUES ($1,$2,$3,CURRENT_DATE,$4,'draft','',$5)
        RETURNING id, supplier_id, number, order_date, currency, status, COALESCE(note,''), angelegt_am
    `, id, supplierID, number, *itemsWithQuotes[0].currency, companyID).Scan(
		&po.ID, &po.SupplierID, &po.Number, &po.OrderDate, &po.Currency, &po.Status, &po.Note, &po.Angelegt,
	); err != nil {
		return nil, nil, err
	}

	items := make([]PurchaseOrderItem, 0, len(itemsWithQuotes))
	pos := 1
	for _, r := range itemsWithQuotes {
		iid := uuid.NewString()
		var out PurchaseOrderItem
		if err := tx.QueryRow(ctx, `
            INSERT INTO purchase_order_items (id, order_id, position, material_id, description, qty, uom, unit_price, currency, delivery_date)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
            RETURNING id, order_id, position, material_id, COALESCE(description,''), qty, uom, unit_price, currency, delivery_date
        `, iid, po.ID, pos, r.materialID, r.description, r.qty, r.uom, *r.unitPrice, *r.currency, r.deliveryDate).Scan(
			&out.ID, &out.OrderID, &out.Position, &out.MaterialID, &out.Description, &out.Qty, &out.UOM, &out.UnitPrice, &out.Currency, &out.DeliveryDate,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, out)
		pos++
	}

	if _, err := tx.Exec(ctx, `UPDATE rfqs SET status='abgeschlossen', closed_at=now() WHERE id=$1`, rfqID); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &po, items, nil
}
