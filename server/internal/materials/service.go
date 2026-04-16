package materials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	pg      *pgxpool.Pool
	mg      *mongo.Client
	mongoDB string
}

func NewService(pg *pgxpool.Pool, mg *mongo.Client, mongoDB string) *Service {
	return &Service{pg: pg, mg: mg, mongoDB: mongoDB}
}

// Material
type Material struct {
	ID                string         `json:"id"`
	Nummer            string         `json:"nummer"`
	Bezeichnung       string         `json:"bezeichnung"`
	Typ               string         `json:"typ"`
	Norm              string         `json:"norm"`
	Werkstoffnummer   string         `json:"werkstoffnummer"`
	Einheit           string         `json:"einheit"`
	Dichte            float64        `json:"dichte"`
	LengthMM          *float64       `json:"length_mm"`
	WidthMM           *float64       `json:"width_mm"`
	HeightMM          *float64       `json:"height_mm"`
	Kategorie         string         `json:"kategorie"`
	Aktiv             bool           `json:"aktiv"`
	Attribute         map[string]any `json:"attribute"`
	DurchschnittsEK   float64        `json:"durchschnitts_einkaufspreis"`
	Waehrung          string         `json:"waehrung"`
	EinkaufMengeSumme float64        `json:"einkauf_menge_summe"`
	EinkaufWertSumme  float64        `json:"einkauf_wert_summe"`
	AngelegtAm        time.Time      `json:"angelegt_am"`
}

type MaterialCreate struct {
	Nummer          string         `json:"nummer"`
	Bezeichnung     string         `json:"bezeichnung"`
	Typ             string         `json:"typ"`
	Norm            string         `json:"norm"`
	Werkstoffnummer string         `json:"werkstoffnummer"`
	Einheit         string         `json:"einheit"`
	Dichte          float64        `json:"dichte"`
	LengthMM        *float64       `json:"length_mm"`
	WidthMM         *float64       `json:"width_mm"`
	HeightMM        *float64       `json:"height_mm"`
	Kategorie       string         `json:"kategorie"`
	Attribute       map[string]any `json:"attribute"`
}

type MaterialUpdate struct {
	Nummer          *string         `json:"nummer"`
	Bezeichnung     *string         `json:"bezeichnung"`
	Typ             *string         `json:"typ"`
	Norm            *string         `json:"norm"`
	Werkstoffnummer *string         `json:"werkstoffnummer"`
	Einheit         *string         `json:"einheit"`
	Dichte          *float64        `json:"dichte"`
	LengthMM        *float64        `json:"length_mm"`
	WidthMM         *float64        `json:"width_mm"`
	HeightMM        *float64        `json:"height_mm"`
	Kategorie       *string         `json:"kategorie"`
	Attribute       *map[string]any `json:"attribute"`
	Aktiv           *bool           `json:"aktiv"`
}

type MaterialFilter struct {
	Q         string
	Typ       string
	Kategorie string
	Limit     int
	Offset    int
}

func (s *Service) Create(ctx context.Context, in MaterialCreate) (*Material, error) {
	if strings.TrimSpace(in.Nummer) == "" || strings.TrimSpace(in.Bezeichnung) == "" {
		return nil, errors.New("Nummer und Bezeichnung sind erforderlich")
	}
	var err error
	if in.Kategorie, err = s.normalizeAndValidateCategory(ctx, in.Kategorie); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	if in.Attribute == nil {
		in.Attribute = map[string]any{}
	}
	var m Material
	err = s.pg.QueryRow(ctx, `
        INSERT INTO materials (
            id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, dichte, length_mm, width_mm, height_mm, kategorie, attributes
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)
        RETURNING id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, COALESCE(dichte,0), length_mm, width_mm, height_mm, kategorie,
                  COALESCE(attributes,'{}'::jsonb), COALESCE(avg_purchase_price,0), COALESCE(currency,'EUR'),
                  COALESCE(purchase_total_qty,0), COALESCE(purchase_total_value,0), angelegt_am
    `, id, in.Nummer, in.Bezeichnung, in.Typ, in.Norm, in.Werkstoffnummer, in.Einheit, in.Dichte, in.LengthMM, in.WidthMM, in.HeightMM, in.Kategorie, toJSONB(in.Attribute)).Scan(
		&m.ID, &m.Nummer, &m.Bezeichnung, &m.Typ, &m.Norm, &m.Werkstoffnummer, &m.Einheit, &m.Dichte, &m.LengthMM, &m.WidthMM, &m.HeightMM, &m.Kategorie,
		new([]byte), &m.DurchschnittsEK, &m.Waehrung, &m.EinkaufMengeSumme, &m.EinkaufWertSumme, &m.AngelegtAm,
	)
	if err != nil {
		return nil, err
	}
	m.Aktiv = true
	// Attribut-Rohdaten erneut laden, um sicher zu gehen
	var raw []byte
	if err := s.pg.QueryRow(ctx, `SELECT COALESCE(attributes,'{}'::jsonb) FROM materials WHERE id=$1`, m.ID).Scan(&raw); err == nil {
		var attrs map[string]any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &attrs)
		}
		if attrs == nil {
			attrs = map[string]any{}
		}
		m.Attribute = attrs
	} else {
		m.Attribute = map[string]any{}
	}
	return &m, nil
}

func (s *Service) Update(ctx context.Context, id string, u MaterialUpdate) (*Material, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if u.Nummer != nil {
		if strings.TrimSpace(*u.Nummer) == "" {
			return nil, errors.New("Nummer erforderlich")
		}
		add("nummer", *u.Nummer)
	}
	if u.Bezeichnung != nil {
		if strings.TrimSpace(*u.Bezeichnung) == "" {
			return nil, errors.New("Bezeichnung erforderlich")
		}
		add("bezeichnung", *u.Bezeichnung)
	}
	if u.Typ != nil {
		add("typ", *u.Typ)
	}
	if u.Norm != nil {
		add("norm", *u.Norm)
	}
	if u.Werkstoffnummer != nil {
		add("werkstoffnummer", *u.Werkstoffnummer)
	}
	if u.Einheit != nil {
		if strings.TrimSpace(*u.Einheit) == "" {
			return nil, errors.New("Einheit erforderlich")
		}
		add("einheit", *u.Einheit)
	}
	if u.Dichte != nil {
		add("dichte", *u.Dichte)
	}
	if u.LengthMM != nil {
		add("length_mm", *u.LengthMM)
	}
	if u.WidthMM != nil {
		add("width_mm", *u.WidthMM)
	}
	if u.HeightMM != nil {
		add("height_mm", *u.HeightMM)
	}
	if u.Kategorie != nil {
		category, err := s.normalizeAndValidateCategory(ctx, *u.Kategorie)
		if err != nil {
			return nil, err
		}
		add("kategorie", category)
	}
	if u.Attribute != nil {
		add("attributes", toJSONB(*u.Attribute))
	}
	if u.Aktiv != nil {
		add("aktiv", *u.Aktiv)
	}
	if len(sets) == 0 {
		return s.Get(ctx, id)
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE materials SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) DeleteSoft(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("ID erforderlich")
	}
	if _, err := s.pg.Exec(ctx, `UPDATE materials SET aktiv=false WHERE id=$1`, id); err != nil {
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context, f MaterialFilter) ([]Material, error) {
	// Defaults
	lim := f.Limit
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	off := f.Offset

	// Dynamische WHERE-Klausel
	sb := strings.Builder{}
	sb.WriteString(`SELECT id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, COALESCE(dichte,0), length_mm, width_mm, height_mm, kategorie,
               COALESCE(attributes,'{}'::jsonb), aktiv, COALESCE(avg_purchase_price,0), COALESCE(currency,'EUR'),
               COALESCE(purchase_total_qty,0), COALESCE(purchase_total_value,0), angelegt_am
        FROM materials`)
	var conds []string
	var args []any
	idx := 1
	if strings.TrimSpace(f.Q) != "" {
		conds = append(conds, fmt.Sprintf("(nummer ILIKE $%d OR bezeichnung ILIKE $%d)", idx, idx+1))
		q := "%" + f.Q + "%"
		args = append(args, q, q)
		idx += 2
	}
	if strings.TrimSpace(f.Typ) != "" {
		conds = append(conds, fmt.Sprintf("typ = $%d", idx))
		args = append(args, f.Typ)
		idx++
	}
	if strings.TrimSpace(f.Kategorie) != "" {
		conds = append(conds, fmt.Sprintf("kategorie = $%d", idx))
		args = append(args, f.Kategorie)
		idx++
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	sb.WriteString(" ORDER BY bezeichnung ASC")
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", lim, off))

	rows, err := s.pg.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Material, 0, lim)
	for rows.Next() {
		var m Material
		var raw []byte
		if err := rows.Scan(&m.ID, &m.Nummer, &m.Bezeichnung, &m.Typ, &m.Norm, &m.Werkstoffnummer, &m.Einheit, &m.Dichte, &m.LengthMM, &m.WidthMM, &m.HeightMM, &m.Kategorie,
			&raw, &m.Aktiv, &m.DurchschnittsEK, &m.Waehrung, &m.EinkaufMengeSumme, &m.EinkaufWertSumme, &m.AngelegtAm); err != nil {
			return nil, err
		}
		var attrs map[string]any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &attrs)
		}
		if attrs == nil {
			attrs = map[string]any{}
		}
		m.Attribute = attrs
		out = append(out, m)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Material, error) {
	var m Material
	var raw []byte
	err := s.pg.QueryRow(ctx, `
        SELECT id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, COALESCE(dichte,0), length_mm, width_mm, height_mm, kategorie,
               COALESCE(attributes,'{}'::jsonb), aktiv, COALESCE(avg_purchase_price,0), COALESCE(currency,'EUR'),
               COALESCE(purchase_total_qty,0), COALESCE(purchase_total_value,0), angelegt_am
        FROM materials WHERE id=$1
    `, id).Scan(&m.ID, &m.Nummer, &m.Bezeichnung, &m.Typ, &m.Norm, &m.Werkstoffnummer, &m.Einheit, &m.Dichte, &m.LengthMM, &m.WidthMM, &m.HeightMM, &m.Kategorie,
		&raw, &m.Aktiv, &m.DurchschnittsEK, &m.Waehrung, &m.EinkaufMengeSumme, &m.EinkaufWertSumme, &m.AngelegtAm)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Material nicht gefunden")
		}
		return nil, err
	}
	var attrs map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &attrs)
	}
	if attrs == nil {
		attrs = map[string]any{}
	}
	m.Attribute = attrs
	return &m, nil
}

// Warehouses & Locations
type Warehouse struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type WarehouseCreate struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (s *Service) CreateWarehouse(ctx context.Context, in WarehouseCreate) (*Warehouse, error) {
	if strings.TrimSpace(in.Code) == "" {
		return nil, errors.New("Code erforderlich")
	}
	id := uuid.NewString()
	var w Warehouse
	if err := s.pg.QueryRow(ctx, `INSERT INTO warehouses (id, code, name) VALUES ($1,$2,$3) RETURNING id, code, name`, id, in.Code, in.Name).Scan(&w.ID, &w.Code, &w.Name); err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Service) ListWarehouses(ctx context.Context) ([]Warehouse, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, code, name FROM warehouses ORDER BY code ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Warehouse, 0)
	for rows.Next() {
		var w Warehouse
		if err := rows.Scan(&w.ID, &w.Code, &w.Name); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

type Location struct {
	ID          string `json:"id"`
	WarehouseID string `json:"warehouse_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
}
type LocationCreate struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (s *Service) CreateLocation(ctx context.Context, warehouseID string, in LocationCreate) (*Location, error) {
	if strings.TrimSpace(in.Code) == "" {
		return nil, errors.New("Code erforderlich")
	}
	id := uuid.NewString()
	var l Location
	if err := s.pg.QueryRow(ctx, `INSERT INTO locations (id, warehouse_id, code, name) VALUES ($1,$2,$3,$4) RETURNING id, warehouse_id, code, name`, id, warehouseID, in.Code, in.Name).Scan(&l.ID, &l.WarehouseID, &l.Code, &l.Name); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) ListLocations(ctx context.Context, warehouseID string) ([]Location, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, warehouse_id, code, name FROM locations WHERE warehouse_id=$1 ORDER BY code ASC`, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Location, 0)
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.WarehouseID, &l.Code, &l.Name); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// Stock Movements
type StockMovementCreate struct {
	MaterialID  string   `json:"material_id"`
	WarehouseID string   `json:"warehouse_id"`
	LocationID  *string  `json:"location_id"`
	BatchCode   *string  `json:"batch_code"`
	Menge       float64  `json:"menge"`
	Einheit     string   `json:"einheit"`
	Typ         string   `json:"typ"` // purchase, in, out, transfer, adjust
	Grund       string   `json:"grund"`
	Referenz    string   `json:"referenz"`
	EKPreis     *float64 `json:"ek_preis"`
	Waehrung    *string  `json:"waehrung"`
}

type StockMovement struct {
	ID string `json:"id"`
}

type StockRow struct {
	WarehouseID string  `json:"warehouse_id"`
	LocationID  *string `json:"location_id"`
	BatchCode   *string `json:"batch_code"`
	Menge       float64 `json:"menge"`
	Einheit     string  `json:"einheit"`
}

func (s *Service) CreateMovement(ctx context.Context, in StockMovementCreate) (*StockMovement, error) {
	if in.Menge == 0 {
		return nil, errors.New("Menge darf nicht 0 sein")
	}
	if strings.TrimSpace(in.MaterialID) == "" || strings.TrimSpace(in.WarehouseID) == "" {
		return nil, errors.New("MaterialID und WarehouseID erforderlich")
	}
	id := uuid.NewString()
	// Transaktion: ggf. Batch anlegen, Movement schreiben, Durchschnitts-EK aktualisieren
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var batchID *string
	if in.BatchCode != nil && strings.TrimSpace(*in.BatchCode) != "" {
		var bid string
		// Upsert Batch
		err := tx.QueryRow(ctx, `
            INSERT INTO batches (id, material_id, code)
            VALUES ($1,$2,$3)
            ON CONFLICT (material_id, code) DO UPDATE SET code = EXCLUDED.code
            RETURNING id
        `, uuid.NewString(), in.MaterialID, *in.BatchCode).Scan(&bid)
		if err != nil {
			return nil, err
		}
		batchID = &bid
	}

	// Insert Movement
	if _, err := tx.Exec(ctx, `
        INSERT INTO stock_movements (id, material_id, warehouse_id, location_id, batch_id, quantity, uom, movement_type, reason, reference)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    `, id, in.MaterialID, in.WarehouseID, in.LocationID, batchID, in.Menge, in.Einheit, in.Typ, in.Grund, in.Referenz); err != nil {
		return nil, err
	}

	// Einkauf: Durchschnittspreis fortschreiben
	if strings.ToLower(in.Typ) == "purchase" && in.EKPreis != nil {
		waehrung := "EUR"
		if in.Waehrung != nil && *in.Waehrung != "" {
			waehrung = *in.Waehrung
		}
		if _, err := tx.Exec(ctx, `
            UPDATE materials
            SET purchase_total_qty = purchase_total_qty + $2,
                purchase_total_value = purchase_total_value + ($2 * $3),
                avg_purchase_price = CASE WHEN (purchase_total_qty + $2) > 0 THEN (purchase_total_value + ($2 * $3)) / (purchase_total_qty + $2) ELSE 0 END,
                currency = $4
            WHERE id=$1
        `, in.MaterialID, in.Menge, *in.EKPreis, waehrung); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &StockMovement{ID: id}, nil
}

func (s *Service) StockByMaterial(ctx context.Context, materialID string) ([]StockRow, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT warehouse_id,
               location_id,
               (SELECT code FROM batches b WHERE b.id = sm.batch_id) AS batch_code,
               SUM(quantity) AS menge,
               MAX(uom) as einheit
        FROM stock_movements sm
        WHERE material_id=$1
        GROUP BY warehouse_id, location_id, batch_id
        HAVING SUM(quantity) <> 0
        ORDER BY warehouse_id, location_id NULLS FIRST
    `, materialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StockRow, 0)
	for rows.Next() {
		var r StockRow
		var batchCode *string
		if err := rows.Scan(&r.WarehouseID, &r.LocationID, &batchCode, &r.Menge, &r.Einheit); err != nil {
			return nil, err
		}
		r.BatchCode = batchCode
		out = append(out, r)
	}
	return out, nil
}

// Facetten: Typen und Kategorien
func (s *Service) ListTypes(ctx context.Context) ([]string, error) {
	rows, err := s.pg.Query(ctx, `SELECT DISTINCT typ FROM materials WHERE TRIM(typ) <> '' ORDER BY typ ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]string, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT category
        FROM (
            SELECT mg.code AS category, mg.sort_order, 0 AS source_order
            FROM material_groups mg
            WHERE mg.is_active = TRUE

            UNION ALL

            SELECT DISTINCT TRIM(m.kategorie) AS category, 999999 AS sort_order, 1 AS source_order
            FROM materials m
            WHERE TRIM(m.kategorie) <> ''
              AND NOT EXISTS (
                  SELECT 1
                  FROM material_groups mg
                  WHERE mg.code = TRIM(m.kategorie)
                    AND mg.is_active = TRUE
              )
        ) categories
        ORDER BY source_order ASC, sort_order ASC, category ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) normalizeAndValidateCategory(ctx context.Context, category string) (string, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return "", nil
	}

	var allowed bool
	if err := s.pg.QueryRow(ctx, `
        SELECT
            EXISTS (
                SELECT 1
                FROM material_groups
                WHERE code = $1
                  AND is_active = TRUE
            )
            OR EXISTS (
                SELECT 1
                FROM materials
                WHERE TRIM(kategorie) = $1
            )
    `, category).Scan(&allowed); err != nil {
		return "", err
	}
	if !allowed {
		return "", errors.New("Ungültige Materialkategorie")
	}
	return category, nil
}
