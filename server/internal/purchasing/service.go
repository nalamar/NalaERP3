package purchasing

import (
    "context"
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "nalaerp3/internal/settings"
)

type Service struct { pg *pgxpool.Pool }
func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

func Statuses() []string { return []string{"draft","ordered","received","canceled"} }

func isIn(v string, list []string) bool {
    v = strings.ToLower(strings.TrimSpace(v))
    for _, x := range list { if v == x { return true } }
    return false
}

type PurchaseOrder struct {
    ID        string    `json:"id"`
    SupplierID string   `json:"lieferant_id"`
    Number    string    `json:"nummer"`
    OrderDate time.Time `json:"datum"`
    Currency  string    `json:"waehrung"`
    Status    string    `json:"status"`
    Note      string    `json:"notiz"`
    Angelegt  time.Time `json:"angelegt_am"`
}

type PurchaseOrderItem struct {
    ID         string     `json:"id"`
    OrderID    string     `json:"bestellung_id"`
    Position   int        `json:"position"`
    MaterialID string     `json:"material_id"`
    Description string    `json:"bezeichnung"`
    Qty        float64    `json:"menge"`
    UOM        string     `json:"einheit"`
    UnitPrice  float64    `json:"preis"`
    Currency   string     `json:"waehrung"`
    DeliveryDate *time.Time `json:"liefertermin"`
}

type PurchaseOrderCreate struct {
    SupplierID string                   `json:"lieferant_id"`
    Number     string                   `json:"nummer"`
    OrderDate  *time.Time               `json:"datum"`
    Currency   string                   `json:"waehrung"`
    Status     string                   `json:"status"`
    Note       string                   `json:"notiz"`
    Items      []PurchaseOrderItemInput `json:"positionen"`
}

type PurchaseOrderItemInput struct {
    MaterialID  string     `json:"material_id"`
    Description string     `json:"bezeichnung"`
    Qty         float64    `json:"menge"`
    UOM         string     `json:"einheit"`
    UnitPrice   float64    `json:"preis"`
    Currency    string     `json:"waehrung"`
    DeliveryDate *time.Time `json:"liefertermin"`
}

type PurchaseOrderFilter struct {
    Q       string
    SupplierID string
    Status  string
    Limit   int
    Offset  int
}

func (s *Service) Create(ctx context.Context, in PurchaseOrderCreate) (*PurchaseOrder, []PurchaseOrderItem, error) {
    if strings.TrimSpace(in.SupplierID) == "" { return nil, nil, errors.New("Lieferant erforderlich") }
    if in.Currency == "" { in.Currency = "EUR" }
    if in.Status == "" { in.Status = "draft" }
    if !isIn(in.Status, Statuses()) { return nil, nil, errors.New("Ungültiger Status") }
    id := uuid.NewString()
    date := time.Now()
    if in.OrderDate != nil { date = *in.OrderDate }
    tx, err := s.pg.Begin(ctx)
    if err != nil { return nil, nil, err }
    defer func(){ _ = tx.Rollback(ctx) }()
    var po PurchaseOrder
    // Autonummer, falls leer
    if strings.TrimSpace(in.Number) == "" {
        numSvc := settings.NewNumberingService(s.pg)
        if n, err := numSvc.Next(ctx, "purchase_order"); err == nil { in.Number = n }
    }
    if err := tx.QueryRow(ctx, `
        INSERT INTO purchase_orders (id, supplier_id, number, order_date, currency, status, note)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id, supplier_id, number, order_date, currency, status, COALESCE(note,''), angelegt_am
    `, id, in.SupplierID, in.Number, date, in.Currency, in.Status, in.Note).Scan(
        &po.ID, &po.SupplierID, &po.Number, &po.OrderDate, &po.Currency, &po.Status, &po.Note, &po.Angelegt,
    ); err != nil { return nil, nil, err }
    items := make([]PurchaseOrderItem, 0, len(in.Items))
    pos := 1
    for _, it := range in.Items {
        if strings.TrimSpace(it.MaterialID) == "" || it.Qty == 0 || strings.TrimSpace(it.UOM) == "" {
            return nil, nil, errors.New("Ungültige Position")
        }
        if it.Currency == "" { it.Currency = po.Currency }
        iid := uuid.NewString()
        var out PurchaseOrderItem
        if err := tx.QueryRow(ctx, `
            INSERT INTO purchase_order_items (id, order_id, position, material_id, description, qty, uom, unit_price, currency, delivery_date)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
            RETURNING id, order_id, position, material_id, COALESCE(description,''), qty, uom, unit_price, currency, delivery_date
        `, iid, po.ID, pos, it.MaterialID, it.Description, it.Qty, it.UOM, it.UnitPrice, it.Currency, it.DeliveryDate).Scan(
            &out.ID, &out.OrderID, &out.Position, &out.MaterialID, &out.Description, &out.Qty, &out.UOM, &out.UnitPrice, &out.Currency, &out.DeliveryDate,
        ); err != nil { return nil, nil, err }
        items = append(items, out)
        pos++
    }
    if err := tx.Commit(ctx); err != nil { return nil, nil, err }
    return &po, items, nil
}

func (s *Service) Get(ctx context.Context, id string) (*PurchaseOrder, []PurchaseOrderItem, error) {
    var po PurchaseOrder
    if err := s.pg.QueryRow(ctx, `SELECT id, supplier_id, number, order_date, currency, status, COALESCE(note,''), angelegt_am FROM purchase_orders WHERE id=$1`, id).Scan(
        &po.ID, &po.SupplierID, &po.Number, &po.OrderDate, &po.Currency, &po.Status, &po.Note, &po.Angelegt,
    ); err != nil { return nil, nil, err }
    rows, err := s.pg.Query(ctx, `SELECT id, order_id, position, material_id, COALESCE(description,''), qty, uom, unit_price, currency, delivery_date FROM purchase_order_items WHERE order_id=$1 ORDER BY position ASC`, id)
    if err != nil { return &po, nil, err }
    defer rows.Close()
    var items []PurchaseOrderItem
    for rows.Next() {
        var it PurchaseOrderItem
        if err := rows.Scan(&it.ID, &it.OrderID, &it.Position, &it.MaterialID, &it.Description, &it.Qty, &it.UOM, &it.UnitPrice, &it.Currency, &it.DeliveryDate); err != nil { return &po, nil, err }
        items = append(items, it)
    }
    if items == nil { items = make([]PurchaseOrderItem, 0) }
    return &po, items, nil
}

func (s *Service) List(ctx context.Context, f PurchaseOrderFilter) ([]PurchaseOrder, error) {
    lim := f.Limit; if lim <= 0 || lim > 200 { lim = 50 }
    off := f.Offset
    sb := strings.Builder{}
    sb.WriteString(`SELECT id, supplier_id, number, order_date, currency, status, COALESCE(note,''), angelegt_am FROM purchase_orders`)
    var conds []string
    var args []any
    idx := 1
    if strings.TrimSpace(f.Q) != "" { conds = append(conds, fmt.Sprintf("(number ILIKE $%d)", idx)); args = append(args, "%"+f.Q+"%"); idx++ }
    if strings.TrimSpace(f.SupplierID) != "" { conds = append(conds, fmt.Sprintf("supplier_id=$%d", idx)); args = append(args, f.SupplierID); idx++ }
    if strings.TrimSpace(f.Status) != "" { conds = append(conds, fmt.Sprintf("status=$%d", idx)); args = append(args, f.Status); idx++ }
    if len(conds) > 0 { sb.WriteString(" WHERE "); sb.WriteString(strings.Join(conds, " AND ")) }
    sb.WriteString(" ORDER BY order_date DESC, number DESC")
    sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", lim, off))
    rows, err := s.pg.Query(ctx, sb.String(), args...)
    if err != nil { return nil, err }
    defer rows.Close()
    out := make([]PurchaseOrder, 0, lim)
    for rows.Next() {
        var po PurchaseOrder
        if err := rows.Scan(&po.ID, &po.SupplierID, &po.Number, &po.OrderDate, &po.Currency, &po.Status, &po.Note, &po.Angelegt); err != nil { return nil, err }
        out = append(out, po)
    }
    return out, nil
}

// --- Updates & Items ---

type PurchaseOrderUpdate struct {
    Number    *string    `json:"nummer"`
    OrderDate *time.Time `json:"datum"`
    Currency  *string    `json:"waehrung"`
    Status    *string    `json:"status"`
    Note      *string    `json:"notiz"`
}

type PurchaseOrderItemUpdate struct {
    Description *string    `json:"bezeichnung"`
    Qty         *float64   `json:"menge"`
    UOM         *string    `json:"einheit"`
    UnitPrice   *float64   `json:"preis"`
    Currency    *string    `json:"waehrung"`
    DeliveryDate *time.Time `json:"liefertermin"`
}

func (s *Service) Update(ctx context.Context, id string, u PurchaseOrderUpdate) (*PurchaseOrder, []PurchaseOrderItem, error) {
    sets := make([]string, 0)
    args := make([]any, 0)
    idx := 1
    add := func(field string, v any) { sets = append(sets, fmt.Sprintf("%s=$%d", field, idx)); args = append(args, v); idx++ }
    if u.Number != nil { add("number", *u.Number) }
    if u.OrderDate != nil { add("order_date", *u.OrderDate) }
    if u.Currency != nil { add("currency", strings.ToUpper(strings.TrimSpace(*u.Currency))) }
    if u.Status != nil { if !isIn(*u.Status, Statuses()) { return nil, nil, errors.New("Ungültiger Status") }; add("status", *u.Status) }
    if u.Note != nil { add("note", *u.Note) }
    if len(sets) == 0 { return s.Get(ctx, id) }
    args = append(args, id)
    q := fmt.Sprintf("UPDATE purchase_orders SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
    if _, err := s.pg.Exec(ctx, q, args...); err != nil { return nil, nil, err }
    return s.Get(ctx, id)
}

func (s *Service) CreateItem(ctx context.Context, orderID string, in PurchaseOrderItemInput) (*PurchaseOrderItem, error) {
    if strings.TrimSpace(in.MaterialID) == "" || in.Qty == 0 || strings.TrimSpace(in.UOM) == "" { return nil, errors.New("Ungültige Position") }
    if in.Currency == "" { in.Currency = "EUR" }
    id := uuid.NewString()
    var it PurchaseOrderItem
    if err := s.pg.QueryRow(ctx, `
        INSERT INTO purchase_order_items (id, order_id, position, material_id, description, qty, uom, unit_price, currency, delivery_date)
        VALUES ($1,$2,(SELECT COALESCE(MAX(position),0)+1 FROM purchase_order_items WHERE order_id=$2),$3,$4,$5,$6,$7,$8,$9)
        RETURNING id, order_id, position, material_id, COALESCE(description,''), qty, uom, unit_price, currency, delivery_date
    `, id, orderID, in.MaterialID, in.Description, in.Qty, in.UOM, in.UnitPrice, in.Currency, in.DeliveryDate).Scan(
        &it.ID, &it.OrderID, &it.Position, &it.MaterialID, &it.Description, &it.Qty, &it.UOM, &it.UnitPrice, &it.Currency, &it.DeliveryDate,
    ); err != nil { return nil, err }
    return &it, nil
}

func (s *Service) UpdateItem(ctx context.Context, orderID, itemID string, u PurchaseOrderItemUpdate) (*PurchaseOrderItem, error) {
    sets := make([]string, 0)
    args := make([]any, 0)
    idx := 1
    add := func(field string, v any) { sets = append(sets, fmt.Sprintf("%s=$%d", field, idx)); args = append(args, v); idx++ }
    if u.Description != nil { add("description", *u.Description) }
    if u.Qty != nil { add("qty", *u.Qty) }
    if u.UOM != nil { add("uom", *u.UOM) }
    if u.UnitPrice != nil { add("unit_price", *u.UnitPrice) }
    if u.Currency != nil { add("currency", strings.ToUpper(strings.TrimSpace(*u.Currency))) }
    if u.DeliveryDate != nil { add("delivery_date", *u.DeliveryDate) }
    if len(sets) == 0 { return s.getItem(ctx, itemID) }
    args = append(args, orderID, itemID)
    q := fmt.Sprintf("UPDATE purchase_order_items SET %s WHERE order_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
    if _, err := s.pg.Exec(ctx, q, args...); err != nil { return nil, err }
    return s.getItem(ctx, itemID)
}

func (s *Service) getItem(ctx context.Context, itemID string) (*PurchaseOrderItem, error) {
    var it PurchaseOrderItem
    if err := s.pg.QueryRow(ctx, `SELECT id, order_id, position, material_id, COALESCE(description,''), qty, uom, unit_price, currency, delivery_date FROM purchase_order_items WHERE id=$1`, itemID).Scan(
        &it.ID, &it.OrderID, &it.Position, &it.MaterialID, &it.Description, &it.Qty, &it.UOM, &it.UnitPrice, &it.Currency, &it.DeliveryDate,
    ); err != nil { return nil, err }
    return &it, nil
}

func (s *Service) DeleteItem(ctx context.Context, orderID, itemID string) error {
    _, err := s.pg.Exec(ctx, `DELETE FROM purchase_order_items WHERE order_id=$1 AND id=$2`, orderID, itemID)
    return err
}
