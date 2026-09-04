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
	Profilserie       string         `json:"profilserie"`
	RCKlasse          string         `json:"rc_klasse"`
	UWert             *float64       `json:"u_wert"`
	Brandschutzklasse string         `json:"brandschutzklasse"`
	MindestBestand    *float64       `json:"mindestbestand"`
	Aktiv             bool           `json:"aktiv"`
	Attribute         map[string]any `json:"attribute"`
	DurchschnittsEK   float64        `json:"durchschnitts_einkaufspreis"`
	Waehrung          string         `json:"waehrung"`
	EinkaufMengeSumme float64        `json:"einkauf_menge_summe"`
	EinkaufWertSumme  float64        `json:"einkauf_wert_summe"`
	AngelegtAm        time.Time      `json:"angelegt_am"`
}

type MaterialCreate struct {
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
	Profilserie       string         `json:"profilserie"`
	RCKlasse          string         `json:"rc_klasse"`
	UWert             *float64       `json:"u_wert"`
	Brandschutzklasse string         `json:"brandschutzklasse"`
	MindestBestand    *float64       `json:"mindestbestand"`
	Attribute         map[string]any `json:"attribute"`
}

type MaterialUpdate struct {
	Nummer            *string         `json:"nummer"`
	Bezeichnung       *string         `json:"bezeichnung"`
	Typ               *string         `json:"typ"`
	Norm              *string         `json:"norm"`
	Werkstoffnummer   *string         `json:"werkstoffnummer"`
	Einheit           *string         `json:"einheit"`
	Dichte            *float64        `json:"dichte"`
	LengthMM          *float64        `json:"length_mm"`
	WidthMM           *float64        `json:"width_mm"`
	HeightMM          *float64        `json:"height_mm"`
	Kategorie         *string         `json:"kategorie"`
	Profilserie       *string         `json:"profilserie"`
	RCKlasse          *string         `json:"rc_klasse"`
	UWert             *float64        `json:"u_wert"`
	Brandschutzklasse *string         `json:"brandschutzklasse"`
	MindestBestand    *float64        `json:"mindestbestand"`
	Attribute         *map[string]any `json:"attribute"`
	Aktiv             *bool           `json:"aktiv"`
}

type MaterialFilter struct {
	Q         string
	Typ       string
	Kategorie string
	Limit     int
	Offset    int
}

func (s *Service) Create(ctx context.Context, in MaterialCreate, companyID string) (*Material, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.Nummer) == "" || strings.TrimSpace(in.Bezeichnung) == "" {
		return nil, errors.New("Nummer und Bezeichnung sind erforderlich")
	}
	var err error
	if in.Kategorie, err = s.normalizeAndValidateCategory(ctx, in.Kategorie, companyID); err != nil {
		return nil, err
	}
	if in.RCKlasse, err = normalizeAndValidateRCKlasse(in.RCKlasse); err != nil {
		return nil, err
	}
	if err := validateUWert(in.UWert); err != nil {
		return nil, err
	}
	if err := validateMindestbestand(in.MindestBestand); err != nil {
		return nil, err
	}
	in.Profilserie = strings.TrimSpace(in.Profilserie)
	in.Brandschutzklasse = strings.TrimSpace(in.Brandschutzklasse)
	id := uuid.NewString()
	if in.Attribute == nil {
		in.Attribute = map[string]any{}
	}
	var m Material
	err = s.pg.QueryRow(ctx, `
        INSERT INTO materials (
            id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, dichte, length_mm, width_mm, height_mm, kategorie,
            profilserie, rc_klasse, u_wert, brandschutzklasse, mindestbestand, attributes, company_id
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb,$19)
        RETURNING id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, COALESCE(dichte,0), length_mm, width_mm, height_mm, kategorie,
                  COALESCE(profilserie,''), COALESCE(rc_klasse,''), u_wert, COALESCE(brandschutzklasse,''), mindestbestand,
                  COALESCE(attributes,'{}'::jsonb), COALESCE(avg_purchase_price,0), COALESCE(currency,'EUR'),
                  COALESCE(purchase_total_qty,0), COALESCE(purchase_total_value,0), angelegt_am
    `, id, in.Nummer, in.Bezeichnung, in.Typ, in.Norm, in.Werkstoffnummer, in.Einheit, in.Dichte, in.LengthMM, in.WidthMM, in.HeightMM, in.Kategorie,
		in.Profilserie, in.RCKlasse, in.UWert, in.Brandschutzklasse, in.MindestBestand, toJSONB(in.Attribute), companyID).Scan(
		&m.ID, &m.Nummer, &m.Bezeichnung, &m.Typ, &m.Norm, &m.Werkstoffnummer, &m.Einheit, &m.Dichte, &m.LengthMM, &m.WidthMM, &m.HeightMM, &m.Kategorie,
		&m.Profilserie, &m.RCKlasse, &m.UWert, &m.Brandschutzklasse, &m.MindestBestand,
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

func (s *Service) Update(ctx context.Context, id string, u MaterialUpdate, companyID string) (*Material, error) {
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
		category, err := s.normalizeAndValidateCategory(ctx, *u.Kategorie, companyID)
		if err != nil {
			return nil, err
		}
		add("kategorie", category)
	}
	if u.Profilserie != nil {
		add("profilserie", strings.TrimSpace(*u.Profilserie))
	}
	if u.RCKlasse != nil {
		rcKlasse, err := normalizeAndValidateRCKlasse(*u.RCKlasse)
		if err != nil {
			return nil, err
		}
		add("rc_klasse", rcKlasse)
	}
	if u.UWert != nil {
		if err := validateUWert(u.UWert); err != nil {
			return nil, err
		}
		add("u_wert", *u.UWert)
	}
	if u.Brandschutzklasse != nil {
		add("brandschutzklasse", strings.TrimSpace(*u.Brandschutzklasse))
	}
	if u.MindestBestand != nil {
		if err := validateMindestbestand(u.MindestBestand); err != nil {
			return nil, err
		}
		add("mindestbestand", *u.MindestBestand)
	}
	if u.Attribute != nil {
		add("attributes", toJSONB(*u.Attribute))
	}
	if u.Aktiv != nil {
		add("aktiv", *u.Aktiv)
	}
	if len(sets) == 0 {
		return s.Get(ctx, id, companyID)
	}
	args = append(args, id, companyID)
	q := fmt.Sprintf("UPDATE materials SET %s WHERE id=$%d AND company_id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

func (s *Service) DeleteSoft(ctx context.Context, id string, companyID string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("ID erforderlich")
	}
	if _, err := s.pg.Exec(ctx, `UPDATE materials SET aktiv=false WHERE id=$1 AND company_id=$2`, id, companyID); err != nil {
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context, f MaterialFilter, companyID string) ([]Material, error) {
	// Defaults
	lim := f.Limit
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	off := f.Offset

	// Dynamische WHERE-Klausel
	sb := strings.Builder{}
	sb.WriteString(`SELECT id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, COALESCE(dichte,0), length_mm, width_mm, height_mm, kategorie,
               COALESCE(profilserie,''), COALESCE(rc_klasse,''), u_wert, COALESCE(brandschutzklasse,''), mindestbestand,
               COALESCE(attributes,'{}'::jsonb), aktiv, COALESCE(avg_purchase_price,0), COALESCE(currency,'EUR'),
               COALESCE(purchase_total_qty,0), COALESCE(purchase_total_value,0), angelegt_am
        FROM materials`)
	conds := []string{"company_id = $1"}
	args := []any{companyID}
	idx := 2
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
			&m.Profilserie, &m.RCKlasse, &m.UWert, &m.Brandschutzklasse, &m.MindestBestand,
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

func (s *Service) Get(ctx context.Context, id string, companyID string) (*Material, error) {
	var m Material
	var raw []byte
	err := s.pg.QueryRow(ctx, `
        SELECT id, nummer, bezeichnung, typ, norm, werkstoffnummer, einheit, COALESCE(dichte,0), length_mm, width_mm, height_mm, kategorie,
               COALESCE(profilserie,''), COALESCE(rc_klasse,''), u_wert, COALESCE(brandschutzklasse,''), mindestbestand,
               COALESCE(attributes,'{}'::jsonb), aktiv, COALESCE(avg_purchase_price,0), COALESCE(currency,'EUR'),
               COALESCE(purchase_total_qty,0), COALESCE(purchase_total_value,0), angelegt_am
        FROM materials WHERE id=$1 AND company_id=$2
    `, id, companyID).Scan(&m.ID, &m.Nummer, &m.Bezeichnung, &m.Typ, &m.Norm, &m.Werkstoffnummer, &m.Einheit, &m.Dichte, &m.LengthMM, &m.WidthMM, &m.HeightMM, &m.Kategorie,
		&m.Profilserie, &m.RCKlasse, &m.UWert, &m.Brandschutzklasse, &m.MindestBestand,
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

func (s *Service) CreateWarehouse(ctx context.Context, in WarehouseCreate, companyID string) (*Warehouse, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.Code) == "" {
		return nil, errors.New("Code erforderlich")
	}
	id := uuid.NewString()
	var w Warehouse
	if err := s.pg.QueryRow(ctx, `INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$2,$3,$4) RETURNING id, code, name`, id, in.Code, in.Name, companyID).Scan(&w.ID, &w.Code, &w.Name); err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Service) ListWarehouses(ctx context.Context, companyID string) ([]Warehouse, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, code, name FROM warehouses WHERE company_id=$1 ORDER BY code ASC`, companyID)
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

// warehouseOwnedByCompany prueft, ob das Lager warehouseID zum Mandanten
// companyID gehoert (locations haben keine eigene company_id, siehe ADR 0002).
func (s *Service) warehouseOwnedByCompany(ctx context.Context, warehouseID, companyID string) (bool, error) {
	var exists bool
	err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, warehouseID, companyID).Scan(&exists)
	return exists, err
}

func (s *Service) CreateLocation(ctx context.Context, warehouseID string, in LocationCreate, companyID string) (*Location, error) {
	if strings.TrimSpace(in.Code) == "" {
		return nil, errors.New("Code erforderlich")
	}
	owned, err := s.warehouseOwnedByCompany(ctx, warehouseID, companyID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Lager nicht gefunden")
	}
	id := uuid.NewString()
	var l Location
	if err := s.pg.QueryRow(ctx, `INSERT INTO locations (id, warehouse_id, code, name) VALUES ($1,$2,$3,$4) RETURNING id, warehouse_id, code, name`, id, warehouseID, in.Code, in.Name).Scan(&l.ID, &l.WarehouseID, &l.Code, &l.Name); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) ListLocations(ctx context.Context, warehouseID string, companyID string) ([]Location, error) {
	owned, err := s.warehouseOwnedByCompany(ctx, warehouseID, companyID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Lager nicht gefunden")
	}
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
	MaterialID          string   `json:"material_id"`
	WarehouseID         string   `json:"warehouse_id"`
	LocationID          *string  `json:"location_id"`
	BatchCode           *string  `json:"batch_code"`
	Menge               float64  `json:"menge"`
	Einheit             string   `json:"einheit"`
	Typ                 string   `json:"typ"` // purchase, in, out, transfer, adjust
	Grund               string   `json:"grund"`
	Referenz            string   `json:"referenz"`
	EKPreis             *float64 `json:"ek_preis"`
	Waehrung            *string  `json:"waehrung"`
	PurchaseOrderItemID *string  `json:"purchase_order_item_id"`
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

func (s *Service) CreateMovement(ctx context.Context, in StockMovementCreate, companyID string) (*StockMovement, error) {
	if in.Menge == 0 {
		return nil, errors.New("Menge darf nicht 0 sein")
	}
	if strings.TrimSpace(in.MaterialID) == "" || strings.TrimSpace(in.WarehouseID) == "" {
		return nil, errors.New("MaterialID und WarehouseID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	id := uuid.NewString()
	// Transaktion: ggf. Batch anlegen, Movement schreiben, Durchschnitts-EK aktualisieren
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var materialOwned, warehouseOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM materials WHERE id=$1 AND company_id=$2)`, in.MaterialID, companyID).Scan(&materialOwned); err != nil {
		return nil, err
	}
	if !materialOwned {
		return nil, errors.New("Material nicht gefunden")
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, in.WarehouseID, companyID).Scan(&warehouseOwned); err != nil {
		return nil, err
	}
	if !warehouseOwned {
		return nil, errors.New("Lager nicht gefunden")
	}

	if in.PurchaseOrderItemID != nil && strings.TrimSpace(*in.PurchaseOrderItemID) != "" {
		var poItemOwned bool
		if err := tx.QueryRow(ctx, `
            SELECT EXISTS(
                SELECT 1 FROM purchase_order_items poi
                JOIN purchase_orders po ON po.id = poi.order_id
                WHERE poi.id=$1 AND po.company_id=$2
            )
        `, *in.PurchaseOrderItemID, companyID).Scan(&poItemOwned); err != nil {
			return nil, err
		}
		if !poItemOwned {
			return nil, errors.New("Bestellposition nicht gefunden")
		}
	}

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
        INSERT INTO stock_movements (id, material_id, warehouse_id, location_id, batch_id, quantity, uom, movement_type, reason, reference, purchase_order_item_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `, id, in.MaterialID, in.WarehouseID, in.LocationID, batchID, in.Menge, in.Einheit, in.Typ, in.Grund, in.Referenz, in.PurchaseOrderItemID); err != nil {
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

func (s *Service) StockByMaterial(ctx context.Context, materialID string, companyID string) ([]StockRow, error) {
	if _, err := s.Get(ctx, materialID, companyID); err != nil {
		return nil, err
	}
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

// Stock Reservations (ADR 0013 - projektbezogene Lagerreservierung)
type StockReservation struct {
	ID          string     `json:"id"`
	MaterialID  string     `json:"material_id"`
	WarehouseID string     `json:"warehouse_id"`
	ProjectID   string     `json:"project_id"`
	Qty         float64    `json:"qty"`
	Status      string     `json:"status"`
	Grund       string     `json:"grund"`
	Referenz    string     `json:"referenz"`
	CreatedAt   time.Time  `json:"created_at"`
	ReleasedAt  *time.Time `json:"released_at"`
}

type StockReservationCreate struct {
	MaterialID  string  `json:"material_id"`
	WarehouseID string  `json:"warehouse_id"`
	ProjectID   string  `json:"project_id"`
	Qty         float64 `json:"qty"`
	Grund       string  `json:"grund"`
	Referenz    string  `json:"referenz"`
}

type StockReservationFilter struct {
	ProjectID   string
	MaterialID  string
	WarehouseID string
	Status      string
}

const stockReservationScanCols = `id, material_id, warehouse_id, project_id::text, qty, status, grund, referenz, created_at, released_at`

func scanStockReservation(row pgx.Row) (*StockReservation, error) {
	var r StockReservation
	if err := row.Scan(&r.ID, &r.MaterialID, &r.WarehouseID, &r.ProjectID, &r.Qty, &r.Status, &r.Grund, &r.Referenz, &r.CreatedAt, &r.ReleasedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// availableStockTx berechnet die verfügbare (unreservierte) Menge eines
// Materials in einem Lager: physischer Bestand (Summe stock_movements)
// minus Summe aktiver Reservierungen. Kernregel aus ADR 0013.
func availableStockTx(ctx context.Context, tx pgx.Tx, materialID, warehouseID string) (float64, error) {
	var physical float64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0) FROM stock_movements WHERE material_id=$1 AND warehouse_id=$2`, materialID, warehouseID).Scan(&physical); err != nil {
		return 0, err
	}
	var reserved float64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(qty),0) FROM stock_reservations WHERE material_id=$1 AND warehouse_id=$2 AND status='aktiv'`, materialID, warehouseID).Scan(&reserved); err != nil {
		return 0, err
	}
	return physical - reserved, nil
}

// AvailableStock ist der lesende Endpunkt-Pfad für availableStockTx (ohne
// offene Transaktion, für reine Abfragen außerhalb von CreateReservation).
func (s *Service) AvailableStock(ctx context.Context, materialID, warehouseID, companyID string) (float64, error) {
	if strings.TrimSpace(materialID) == "" || strings.TrimSpace(warehouseID) == "" {
		return 0, errors.New("MaterialID und WarehouseID erforderlich")
	}
	owned, err := s.warehouseOwnedByCompany(ctx, warehouseID, companyID)
	if err != nil {
		return 0, err
	}
	if !owned {
		return 0, errors.New("Lager nicht gefunden")
	}
	var physical float64
	if err := s.pg.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0) FROM stock_movements WHERE material_id=$1 AND warehouse_id=$2`, materialID, warehouseID).Scan(&physical); err != nil {
		return 0, err
	}
	var reserved float64
	if err := s.pg.QueryRow(ctx, `SELECT COALESCE(SUM(qty),0) FROM stock_reservations WHERE material_id=$1 AND warehouse_id=$2 AND status='aktiv'`, materialID, warehouseID).Scan(&reserved); err != nil {
		return 0, err
	}
	return physical - reserved, nil
}

// CreateReservation legt eine projektbezogene Lagerreservierung an.
// Lehnt ab (400), wenn die angeforderte Menge die verfügbare (physischer
// Bestand minus bereits aktiver Reservierungen) Menge übersteigt - das ist
// die eigentliche "Reservierungslogik" aus ADR 0013, ohne diese Prüfung
// wäre die Tabelle nur ein Label ohne fachlichen Wert. Ein
// Advisory-Lock auf das Material+Lager-Paar serialisiert konkurrierende
// Anlagen unabhängig davon, ob bereits sperrbare stock_movements-Zeilen
// existieren.
func (s *Service) CreateReservation(ctx context.Context, in StockReservationCreate, companyID string) (*StockReservation, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.MaterialID) == "" || strings.TrimSpace(in.WarehouseID) == "" || strings.TrimSpace(in.ProjectID) == "" {
		return nil, errors.New("MaterialID, WarehouseID und ProjectID erforderlich")
	}
	if in.Qty <= 0 {
		return nil, errors.New("Menge muss größer als 0 sein")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var materialOwned, warehouseOwned, projectOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM materials WHERE id=$1 AND company_id=$2)`, in.MaterialID, companyID).Scan(&materialOwned); err != nil {
		return nil, err
	}
	if !materialOwned {
		return nil, errors.New("Material nicht gefunden")
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, in.WarehouseID, companyID).Scan(&warehouseOwned); err != nil {
		return nil, err
	}
	if !warehouseOwned {
		return nil, errors.New("Lager nicht gefunden")
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND company_id=$2)`, in.ProjectID, companyID).Scan(&projectOwned); err != nil {
		return nil, err
	}
	if !projectOwned {
		return nil, errors.New("Projekt nicht gefunden")
	}

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, in.MaterialID, in.WarehouseID); err != nil {
		return nil, err
	}

	available, err := availableStockTx(ctx, tx, in.MaterialID, in.WarehouseID)
	if err != nil {
		return nil, err
	}
	if in.Qty > available {
		return nil, fmt.Errorf("angeforderte Menge übersteigt die verfügbare Menge im Lager (verfügbar: %g, angefordert: %g)", available, in.Qty)
	}

	out, err := scanStockReservation(tx.QueryRow(ctx, `
        INSERT INTO stock_reservations (id, material_id, warehouse_id, project_id, qty, grund, referenz)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING `+stockReservationScanCols, uuid.NewString(), in.MaterialID, in.WarehouseID, in.ProjectID, in.Qty, in.Grund, in.Referenz))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// ReleaseReservation gibt eine aktive Reservierung frei (status: aktiv ->
// freigegeben). Keine automatische Verknüpfung mit CreateMovement (ADR
// 0013) - eine freigegebene Reservierung zählt ab sofort nicht mehr in
// availableStockTx.
func (s *Service) ReleaseReservation(ctx context.Context, id string, companyID string) (*StockReservation, error) {
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
	err = tx.QueryRow(ctx, `
        SELECT r.status
        FROM stock_reservations r
        JOIN warehouses w ON w.id = r.warehouse_id
        WHERE r.id=$1 AND w.company_id=$2
        FOR UPDATE OF r
    `, id, companyID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Reservierung nicht gefunden")
		}
		return nil, err
	}
	if status == "freigegeben" {
		return nil, errors.New("Reservierung ist bereits freigegeben")
	}

	out, err := scanStockReservation(tx.QueryRow(ctx, `
        UPDATE stock_reservations
        SET status='freigegeben', released_at=now()
        WHERE id=$1
        RETURNING `+stockReservationScanCols, id))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ListReservations(ctx context.Context, f StockReservationFilter, companyID string) ([]StockReservation, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	query := `
        SELECT r.id, r.material_id, r.warehouse_id, r.project_id::text, r.qty, r.status, r.grund, r.referenz, r.created_at, r.released_at
        FROM stock_reservations r
        JOIN warehouses w ON w.id = r.warehouse_id
        WHERE w.company_id=$1
    `
	args := []any{companyID}
	if strings.TrimSpace(f.ProjectID) != "" {
		args = append(args, f.ProjectID)
		query += fmt.Sprintf(" AND r.project_id=$%d", len(args))
	}
	if strings.TrimSpace(f.MaterialID) != "" {
		args = append(args, f.MaterialID)
		query += fmt.Sprintf(" AND r.material_id=$%d", len(args))
	}
	if strings.TrimSpace(f.WarehouseID) != "" {
		args = append(args, f.WarehouseID)
		query += fmt.Sprintf(" AND r.warehouse_id=$%d", len(args))
	}
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, f.Status)
		query += fmt.Sprintf(" AND r.status=$%d", len(args))
	}
	query += " ORDER BY r.created_at DESC"

	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StockReservation, 0)
	for rows.Next() {
		var r StockReservation
		if err := rows.Scan(&r.ID, &r.MaterialID, &r.WarehouseID, &r.ProjectID, &r.Qty, &r.Status, &r.Grund, &r.Referenz, &r.CreatedAt, &r.ReleasedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Inventuren (ADR 0014 - Inventurprozess)
type Inventory struct {
	ID          string     `json:"id"`
	WarehouseID string     `json:"warehouse_id"`
	Status      string     `json:"status"`
	Note        string     `json:"note"`
	StartedAt   time.Time  `json:"started_at"`
	ClosedAt    *time.Time `json:"closed_at"`
}

type InventoryCreate struct {
	WarehouseID string `json:"warehouse_id"`
	Note        string `json:"note"`
}

type InventoryFilter struct {
	WarehouseID string
	Status      string
}

type InventoryLine struct {
	ID          string    `json:"id"`
	InventoryID string    `json:"inventory_id"`
	MaterialID  string    `json:"material_id"`
	LocationID  *string   `json:"location_id"`
	SollQty     float64   `json:"soll_qty"`
	IstQty      float64   `json:"ist_qty"`
	CountedAt   time.Time `json:"counted_at"`
}

type InventoryLineCreate struct {
	MaterialID string  `json:"material_id"`
	LocationID *string `json:"location_id"`
	IstQty     float64 `json:"ist_qty"`
}

const inventoryScanCols = `id, warehouse_id, status, note, started_at, closed_at`

func scanInventory(row pgx.Row) (*Inventory, error) {
	var inv Inventory
	if err := row.Scan(&inv.ID, &inv.WarehouseID, &inv.Status, &inv.Note, &inv.StartedAt, &inv.ClosedAt); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Service) StartInventory(ctx context.Context, in InventoryCreate, companyID string) (*Inventory, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.WarehouseID) == "" {
		return nil, errors.New("WarehouseID erforderlich")
	}
	owned, err := s.warehouseOwnedByCompany(ctx, in.WarehouseID, companyID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Lager nicht gefunden")
	}
	out, err := scanInventory(s.pg.QueryRow(ctx, `
        INSERT INTO inventories (id, warehouse_id, note)
        VALUES ($1,$2,$3)
        RETURNING `+inventoryScanCols, uuid.NewString(), in.WarehouseID, in.Note))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) GetInventory(ctx context.Context, id string, companyID string) (*Inventory, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	out, err := scanInventory(s.pg.QueryRow(ctx, `
        SELECT i.id, i.warehouse_id, i.status, i.note, i.started_at, i.closed_at
        FROM inventories i
        JOIN warehouses w ON w.id = i.warehouse_id
        WHERE i.id=$1 AND w.company_id=$2
    `, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Inventur nicht gefunden")
		}
		return nil, err
	}
	return out, nil
}

func (s *Service) ListInventories(ctx context.Context, f InventoryFilter, companyID string) ([]Inventory, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	query := `
        SELECT i.id, i.warehouse_id, i.status, i.note, i.started_at, i.closed_at
        FROM inventories i
        JOIN warehouses w ON w.id = i.warehouse_id
        WHERE w.company_id=$1
    `
	args := []any{companyID}
	if strings.TrimSpace(f.WarehouseID) != "" {
		args = append(args, f.WarehouseID)
		query += fmt.Sprintf(" AND i.warehouse_id=$%d", len(args))
	}
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, f.Status)
		query += fmt.Sprintf(" AND i.status=$%d", len(args))
	}
	query += " ORDER BY i.started_at DESC"

	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Inventory, 0)
	for rows.Next() {
		var inv Inventory
		if err := rows.Scan(&inv.ID, &inv.WarehouseID, &inv.Status, &inv.Note, &inv.StartedAt, &inv.ClosedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, nil
}

// AddInventoryLine erfasst eine Zählposition. soll_qty wird aus dem
// aktuellen Buchbestand (stock_movements) fixiert - ein Snapshot je
// Position, kein warehouse-weiter Bewegungsstopp (ADR 0014).
func (s *Service) AddInventoryLine(ctx context.Context, inventoryID string, in InventoryLineCreate, companyID string) (*InventoryLine, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(inventoryID) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(in.MaterialID) == "" {
		return nil, errors.New("MaterialID erforderlich")
	}
	if in.IstQty < 0 {
		return nil, errors.New("Istmenge darf nicht negativ sein")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var warehouseID, status string
	err = tx.QueryRow(ctx, `
        SELECT i.warehouse_id, i.status
        FROM inventories i
        JOIN warehouses w ON w.id = i.warehouse_id
        WHERE i.id=$1 AND w.company_id=$2
        FOR UPDATE OF i
    `, inventoryID, companyID).Scan(&warehouseID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Inventur nicht gefunden")
		}
		return nil, err
	}
	if status != "laufend" {
		return nil, errors.New("Inventur ist bereits abgeschlossen")
	}

	var materialOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM materials WHERE id=$1 AND company_id=$2)`, in.MaterialID, companyID).Scan(&materialOwned); err != nil {
		return nil, err
	}
	if !materialOwned {
		return nil, errors.New("Material nicht gefunden")
	}

	hasLocation := in.LocationID != nil && strings.TrimSpace(*in.LocationID) != ""
	if hasLocation {
		var locationOwned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locations WHERE id=$1 AND warehouse_id=$2)`, *in.LocationID, warehouseID).Scan(&locationOwned); err != nil {
			return nil, err
		}
		if !locationOwned {
			return nil, errors.New("Lagerort nicht gefunden")
		}
	}

	var sollQty float64
	if hasLocation {
		if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0) FROM stock_movements WHERE material_id=$1 AND warehouse_id=$2 AND location_id=$3`, in.MaterialID, warehouseID, *in.LocationID).Scan(&sollQty); err != nil {
			return nil, err
		}
	} else {
		if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0) FROM stock_movements WHERE material_id=$1 AND warehouse_id=$2`, in.MaterialID, warehouseID).Scan(&sollQty); err != nil {
			return nil, err
		}
	}

	var out InventoryLine
	if err := tx.QueryRow(ctx, `
        INSERT INTO inventory_lines (id, inventory_id, material_id, location_id, soll_qty, ist_qty)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING id, inventory_id, material_id, location_id, soll_qty, ist_qty, counted_at
    `, uuid.NewString(), inventoryID, in.MaterialID, in.LocationID, sollQty, in.IstQty).Scan(
		&out.ID, &out.InventoryID, &out.MaterialID, &out.LocationID, &out.SollQty, &out.IstQty, &out.CountedAt,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) ListInventoryLines(ctx context.Context, inventoryID string, companyID string) ([]InventoryLine, error) {
	if strings.TrimSpace(inventoryID) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	var exists bool
	if err := s.pg.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM inventories i JOIN warehouses w ON w.id = i.warehouse_id
            WHERE i.id=$1 AND w.company_id=$2
        )
    `, inventoryID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Inventur nicht gefunden")
	}

	rows, err := s.pg.Query(ctx, `
        SELECT id, inventory_id, material_id, location_id, soll_qty, ist_qty, counted_at
        FROM inventory_lines
        WHERE inventory_id=$1
        ORDER BY counted_at ASC
    `, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InventoryLine, 0)
	for rows.Next() {
		var l InventoryLine
		if err := rows.Scan(&l.ID, &l.InventoryID, &l.MaterialID, &l.LocationID, &l.SollQty, &l.IstQty, &l.CountedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// CloseInventory schließt eine laufende Inventur ab: für jede
// Zählposition mit ist_qty != soll_qty wird genau eine adjust-Buchung in
// stock_movements erzeugt (ADR 0014 - Kernregel). Zeilen ohne Differenz
// erzeugen bewusst keine Leerbuchung. Danach ist die Inventur
// schreibgeschützt (Festschreibung) - AddInventoryLine lehnt jeden
// weiteren Versuch über die status-Prüfung ab.
func (s *Service) CloseInventory(ctx context.Context, id string, companyID string) (*Inventory, error) {
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

	var warehouseID, status string
	err = tx.QueryRow(ctx, `
        SELECT i.warehouse_id, i.status
        FROM inventories i
        JOIN warehouses w ON w.id = i.warehouse_id
        WHERE i.id=$1 AND w.company_id=$2
        FOR UPDATE OF i
    `, id, companyID).Scan(&warehouseID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Inventur nicht gefunden")
		}
		return nil, err
	}
	if status != "laufend" {
		return nil, errors.New("Inventur ist bereits abgeschlossen")
	}

	rows, err := tx.Query(ctx, `SELECT material_id, location_id, soll_qty, ist_qty FROM inventory_lines WHERE inventory_id=$1`, id)
	if err != nil {
		return nil, err
	}
	type diffLine struct {
		materialID string
		locationID *string
		diff       float64
	}
	var diffs []diffLine
	for rows.Next() {
		var materialID string
		var locationID *string
		var soll, ist float64
		if err := rows.Scan(&materialID, &locationID, &soll, &ist); err != nil {
			rows.Close()
			return nil, err
		}
		if ist != soll {
			diffs = append(diffs, diffLine{materialID: materialID, locationID: locationID, diff: ist - soll})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	reference := "inventur:" + id
	for _, d := range diffs {
		var einheit string
		if err := tx.QueryRow(ctx, `SELECT einheit FROM materials WHERE id=$1`, d.materialID).Scan(&einheit); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
            INSERT INTO stock_movements (id, material_id, warehouse_id, location_id, quantity, uom, movement_type, reason, reference)
            VALUES ($1,$2,$3,$4,$5,$6,'adjust','Inventurkorrektur',$7)
        `, uuid.NewString(), d.materialID, warehouseID, d.locationID, d.diff, einheit, reference); err != nil {
			return nil, err
		}
	}

	out, err := scanInventory(tx.QueryRow(ctx, `
        UPDATE inventories
        SET status='abgeschlossen', closed_at=now()
        WHERE id=$1
        RETURNING `+inventoryScanCols, id))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// Verschnitt-/Reststückverwaltung (ADR 0015)
type ProfileOffcut struct {
	ID             string     `json:"id"`
	MaterialID     string     `json:"material_id"`
	WarehouseID    string     `json:"warehouse_id"`
	LocationID     *string    `json:"location_id"`
	LengthMM       float64    `json:"length_mm"`
	Status         string     `json:"status"`
	SourceOffcutID *string    `json:"source_offcut_id"`
	Note           string     `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	ConsumedAt     *time.Time `json:"consumed_at"`
	UsedLengthMM   *float64   `json:"used_length_mm"`
}

type ProfileOffcutCreate struct {
	MaterialID  string  `json:"material_id"`
	WarehouseID string  `json:"warehouse_id"`
	LocationID  *string `json:"location_id"`
	LengthMM    float64 `json:"length_mm"`
	Note        string  `json:"note"`
}

type ProfileOffcutFilter struct {
	MaterialID  string
	WarehouseID string
	Status      string
	MinLengthMM *float64
}

type ProfileOffcutConsume struct {
	UsedLengthMM float64 `json:"used_length_mm"`
}

type ConsumeOffcutResult struct {
	Consumed  ProfileOffcut  `json:"consumed"`
	Remainder *ProfileOffcut `json:"remainder"`
}

const profileOffcutScanCols = `id, material_id, warehouse_id, location_id, length_mm, status, source_offcut_id, note, created_at, consumed_at, used_length_mm`

func scanProfileOffcut(row pgx.Row) (*ProfileOffcut, error) {
	var o ProfileOffcut
	if err := row.Scan(&o.ID, &o.MaterialID, &o.WarehouseID, &o.LocationID, &o.LengthMM, &o.Status, &o.SourceOffcutID, &o.Note, &o.CreatedAt, &o.ConsumedAt, &o.UsedLengthMM); err != nil {
		return nil, err
	}
	return &o, nil
}

// RegisterOffcut erfasst ein neues, wiederverwendbares Reststück. Die
// Registrierung selbst ist die fachliche Entscheidung "dieser Rest ist
// brauchbar" - es gibt keine Mindestlänge, die das System durchsetzt
// (ADR 0015). Nur für Materialien mit gesetztem profilserie (A.1).
func (s *Service) RegisterOffcut(ctx context.Context, in ProfileOffcutCreate, companyID string) (*ProfileOffcut, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.MaterialID) == "" || strings.TrimSpace(in.WarehouseID) == "" {
		return nil, errors.New("MaterialID und WarehouseID erforderlich")
	}
	if in.LengthMM <= 0 {
		return nil, errors.New("Länge muss größer als 0 sein")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var profilserie string
	err = tx.QueryRow(ctx, `SELECT COALESCE(profilserie,'') FROM materials WHERE id=$1 AND company_id=$2`, in.MaterialID, companyID).Scan(&profilserie)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Material nicht gefunden")
		}
		return nil, err
	}
	if strings.TrimSpace(profilserie) == "" {
		return nil, errors.New("Material ist kein Profil (profilserie erforderlich)")
	}

	var warehouseOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, in.WarehouseID, companyID).Scan(&warehouseOwned); err != nil {
		return nil, err
	}
	if !warehouseOwned {
		return nil, errors.New("Lager nicht gefunden")
	}

	if in.LocationID != nil && strings.TrimSpace(*in.LocationID) != "" {
		var locationOwned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locations WHERE id=$1 AND warehouse_id=$2)`, *in.LocationID, in.WarehouseID).Scan(&locationOwned); err != nil {
			return nil, err
		}
		if !locationOwned {
			return nil, errors.New("Lagerort nicht gefunden")
		}
	}

	out, err := scanProfileOffcut(tx.QueryRow(ctx, `
        INSERT INTO profile_offcuts (id, material_id, warehouse_id, location_id, length_mm, note)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING `+profileOffcutScanCols, uuid.NewString(), in.MaterialID, in.WarehouseID, in.LocationID, in.LengthMM, in.Note))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ListOffcuts(ctx context.Context, f ProfileOffcutFilter, companyID string) ([]ProfileOffcut, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	query := `
        SELECT o.id, o.material_id, o.warehouse_id, o.location_id, o.length_mm, o.status, o.source_offcut_id, o.note, o.created_at, o.consumed_at, o.used_length_mm
        FROM profile_offcuts o
        JOIN warehouses w ON w.id = o.warehouse_id
        WHERE w.company_id=$1
    `
	args := []any{companyID}
	if strings.TrimSpace(f.MaterialID) != "" {
		args = append(args, f.MaterialID)
		query += fmt.Sprintf(" AND o.material_id=$%d", len(args))
	}
	if strings.TrimSpace(f.WarehouseID) != "" {
		args = append(args, f.WarehouseID)
		query += fmt.Sprintf(" AND o.warehouse_id=$%d", len(args))
	}
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, f.Status)
		query += fmt.Sprintf(" AND o.status=$%d", len(args))
	}
	if f.MinLengthMM != nil {
		args = append(args, *f.MinLengthMM)
		query += fmt.Sprintf(" AND o.length_mm>=$%d", len(args))
	}
	query += " ORDER BY o.length_mm ASC"

	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProfileOffcut, 0)
	for rows.Next() {
		var o ProfileOffcut
		if err := rows.Scan(&o.ID, &o.MaterialID, &o.WarehouseID, &o.LocationID, &o.LengthMM, &o.Status, &o.SourceOffcutID, &o.Note, &o.CreatedAt, &o.ConsumedAt, &o.UsedLengthMM); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

// ConsumeOffcut verbraucht ein Reststück (oder einen Teil davon). Die
// verbrauchte Zeile wird nie in der Länge mutiert, sondern abgeschlossen
// (status='verbraucht'); bleibt eine Restlänge > 0, entsteht transaktional
// EIN neues, über source_offcut_id verkettetes Stück (ADR 0015 -
// Kernregel, Storno-statt-Mutation-Muster).
func (s *Service) ConsumeOffcut(ctx context.Context, id string, in ProfileOffcutConsume, companyID string) (*ConsumeOffcutResult, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.UsedLengthMM <= 0 {
		return nil, errors.New("Verbrauchte Länge muss größer als 0 sein")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var materialID, warehouseID, status string
	var locationID *string
	var lengthMM float64
	err = tx.QueryRow(ctx, `
        SELECT o.material_id, o.warehouse_id, o.location_id, o.length_mm, o.status
        FROM profile_offcuts o
        JOIN warehouses w ON w.id = o.warehouse_id
        WHERE o.id=$1 AND w.company_id=$2
        FOR UPDATE OF o
    `, id, companyID).Scan(&materialID, &warehouseID, &locationID, &lengthMM, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Reststück nicht gefunden")
		}
		return nil, err
	}
	if status != "verfügbar" {
		return nil, errors.New("Reststück ist bereits verbraucht")
	}
	if in.UsedLengthMM > lengthMM {
		return nil, errors.New("verbrauchte Länge übersteigt die Stücklänge")
	}

	consumed, err := scanProfileOffcut(tx.QueryRow(ctx, `
        UPDATE profile_offcuts
        SET status='verbraucht', consumed_at=now(), used_length_mm=$2
        WHERE id=$1
        RETURNING `+profileOffcutScanCols, id, in.UsedLengthMM))
	if err != nil {
		return nil, err
	}

	result := &ConsumeOffcutResult{Consumed: *consumed}

	remainderMM := lengthMM - in.UsedLengthMM
	if remainderMM > 0 {
		remainder, err := scanProfileOffcut(tx.QueryRow(ctx, `
            INSERT INTO profile_offcuts (id, material_id, warehouse_id, location_id, length_mm, source_offcut_id)
            VALUES ($1,$2,$3,$4,$5,$6)
            RETURNING `+profileOffcutScanCols, uuid.NewString(), materialID, warehouseID, locationID, remainderMM, id))
		if err != nil {
			return nil, err
		}
		result.Remainder = remainder
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// Facetten: Typen und Kategorien
func (s *Service) ListTypes(ctx context.Context, companyID string) ([]string, error) {
	rows, err := s.pg.Query(ctx, `SELECT DISTINCT typ FROM materials WHERE TRIM(typ) <> '' AND company_id=$1 ORDER BY typ ASC`, companyID)
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

func (s *Service) ListCategories(ctx context.Context, companyID string) ([]string, error) {
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
              AND m.company_id = $1
              AND NOT EXISTS (
                  SELECT 1
                  FROM material_groups mg
                  WHERE mg.code = TRIM(m.kategorie)
                    AND mg.is_active = TRUE
              )
        ) categories
        ORDER BY source_order ASC, sort_order ASC, category ASC`, companyID)
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

// validRCKlassen sind die nach DIN EN 1627 definierten Widerstandsklassen
// gegen Einbruch (siehe docs/adr/0006-metallbau-artikel-profilattribute.md).
var validRCKlassen = map[string]bool{
	"RC1": true, "RC1N": true,
	"RC2": true, "RC2N": true,
	"RC3": true,
	"RC4": true,
	"RC5": true,
	"RC6": true,
}

func normalizeAndValidateRCKlasse(rcKlasse string) (string, error) {
	rcKlasse = strings.ToUpper(strings.TrimSpace(rcKlasse))
	if rcKlasse == "" {
		return "", nil
	}
	if !validRCKlassen[rcKlasse] {
		return "", errors.New("Ungültige RC-Klasse (gültig: RC1, RC1N, RC2, RC2N, RC3, RC4, RC5, RC6)")
	}
	return rcKlasse, nil
}

// validateUWert prueft nur auf physikalische Plausibilitaet (>0), siehe
// ADR 0006: kein Enum, da physikalischer Messwert ohne festen Katalog.
func validateUWert(uWert *float64) error {
	if uWert != nil && *uWert <= 0 {
		return errors.New("U-Wert muss größer als 0 sein")
	}
	return nil
}

func validateMindestbestand(mindestbestand *float64) error {
	if mindestbestand != nil && *mindestbestand < 0 {
		return errors.New("Mindestbestand darf nicht negativ sein")
	}
	return nil
}

func (s *Service) normalizeAndValidateCategory(ctx context.Context, category string, companyID string) (string, error) {
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
                  AND company_id = $2
            )
    `, category, companyID).Scan(&allowed); err != nil {
		return "", err
	}
	if !allowed {
		return "", errors.New("Ungültige Materialkategorie")
	}
	return category, nil
}
