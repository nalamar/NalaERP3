package materials

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PriceList (Preisliste, Header) — Backlog A.2, siehe
// docs/adr/0007-preisliste-entitaet.md.
type PriceList struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Lieferant  string     `json:"lieferant"`
	Currency   string     `json:"currency"`
	GueltigVon time.Time  `json:"gueltig_von"`
	GueltigBis *time.Time `json:"gueltig_bis"`
	Aktiv      bool       `json:"aktiv"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type PriceListCreate struct {
	Name       string     `json:"name"`
	Lieferant  string     `json:"lieferant"`
	Currency   string     `json:"currency"`
	GueltigVon time.Time  `json:"gueltig_von"`
	GueltigBis *time.Time `json:"gueltig_bis"`
}

type PriceListUpdate struct {
	Name       *string    `json:"name"`
	Lieferant  *string    `json:"lieferant"`
	Currency   *string    `json:"currency"`
	GueltigVon *time.Time `json:"gueltig_von"`
	GueltigBis *time.Time `json:"gueltig_bis"`
	Aktiv      *bool      `json:"aktiv"`
}

type PriceListFilter struct {
	Q      string
	Limit  int
	Offset int
}

func validatePriceListDates(gueltigVon time.Time, gueltigBis *time.Time) error {
	if gueltigVon.IsZero() {
		return errors.New("Gültig-von erforderlich")
	}
	if gueltigBis != nil && gueltigBis.Before(gueltigVon) {
		return errors.New("Gültig-bis darf nicht vor Gültig-von liegen")
	}
	return nil
}

func (s *Service) CreatePriceList(ctx context.Context, in PriceListCreate, companyID string) (*PriceList, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Name erforderlich")
	}
	if err := validatePriceListDates(in.GueltigVon, in.GueltigBis); err != nil {
		return nil, err
	}
	currency := strings.TrimSpace(in.Currency)
	if currency == "" {
		currency = "EUR"
	}
	id := uuid.NewString()
	var pl PriceList
	err := s.pg.QueryRow(ctx, `
        INSERT INTO price_lists (id, company_id, name, lieferant, currency, gueltig_von, gueltig_bis)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id, name, lieferant, currency, gueltig_von, gueltig_bis, aktiv, created_at, updated_at
    `, id, companyID, in.Name, strings.TrimSpace(in.Lieferant), currency, in.GueltigVon, in.GueltigBis).Scan(
		&pl.ID, &pl.Name, &pl.Lieferant, &pl.Currency, &pl.GueltigVon, &pl.GueltigBis, &pl.Aktiv, &pl.CreatedAt, &pl.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &pl, nil
}

func (s *Service) UpdatePriceList(ctx context.Context, id string, u PriceListUpdate, companyID string) (*PriceList, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("ID erforderlich")
	}
	existing, err := s.GetPriceList(ctx, id, companyID)
	if err != nil {
		return nil, err
	}

	gueltigVon := existing.GueltigVon
	gueltigBis := existing.GueltigBis
	if u.GueltigVon != nil {
		gueltigVon = *u.GueltigVon
	}
	if u.GueltigBis != nil {
		gueltigBis = u.GueltigBis
	}
	if u.GueltigVon != nil || u.GueltigBis != nil {
		if err := validatePriceListDates(gueltigVon, gueltigBis); err != nil {
			return nil, err
		}
	}

	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if u.Name != nil {
		if strings.TrimSpace(*u.Name) == "" {
			return nil, errors.New("Name erforderlich")
		}
		add("name", *u.Name)
	}
	if u.Lieferant != nil {
		add("lieferant", strings.TrimSpace(*u.Lieferant))
	}
	if u.Currency != nil {
		currency := strings.TrimSpace(*u.Currency)
		if currency == "" {
			return nil, errors.New("Währung erforderlich")
		}
		add("currency", currency)
	}
	if u.GueltigVon != nil {
		add("gueltig_von", gueltigVon)
	}
	if u.GueltigBis != nil {
		add("gueltig_bis", gueltigBis)
	}
	if u.Aktiv != nil {
		add("aktiv", *u.Aktiv)
	}
	if len(sets) == 0 {
		return existing, nil
	}
	add("updated_at", time.Now())
	args = append(args, id, companyID)
	q := fmt.Sprintf("UPDATE price_lists SET %s WHERE id=$%d AND company_id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.GetPriceList(ctx, id, companyID)
}

func (s *Service) DeleteSoftPriceList(ctx context.Context, id string, companyID string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("ID erforderlich")
	}
	if _, err := s.pg.Exec(ctx, `UPDATE price_lists SET aktiv=false, updated_at=now() WHERE id=$1 AND company_id=$2`, id, companyID); err != nil {
		return err
	}
	return nil
}

func (s *Service) ListPriceLists(ctx context.Context, f PriceListFilter, companyID string) ([]PriceList, error) {
	lim := f.Limit
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	off := f.Offset

	sb := strings.Builder{}
	sb.WriteString(`SELECT id, name, lieferant, currency, gueltig_von, gueltig_bis, aktiv, created_at, updated_at FROM price_lists`)
	conds := []string{"company_id = $1"}
	args := []any{companyID}
	idx := 2
	if strings.TrimSpace(f.Q) != "" {
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR lieferant ILIKE $%d)", idx, idx+1))
		q := "%" + f.Q + "%"
		args = append(args, q, q)
		idx += 2
	}
	sb.WriteString(" WHERE " + strings.Join(conds, " AND "))
	sb.WriteString(" ORDER BY gueltig_von DESC")
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", lim, off))

	rows, err := s.pg.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PriceList, 0, lim)
	for rows.Next() {
		var pl PriceList
		if err := rows.Scan(&pl.ID, &pl.Name, &pl.Lieferant, &pl.Currency, &pl.GueltigVon, &pl.GueltigBis, &pl.Aktiv, &pl.CreatedAt, &pl.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, pl)
	}
	return out, nil
}

func (s *Service) GetPriceList(ctx context.Context, id string, companyID string) (*PriceList, error) {
	var pl PriceList
	err := s.pg.QueryRow(ctx, `
        SELECT id, name, lieferant, currency, gueltig_von, gueltig_bis, aktiv, created_at, updated_at
        FROM price_lists WHERE id=$1 AND company_id=$2
    `, id, companyID).Scan(&pl.ID, &pl.Name, &pl.Lieferant, &pl.Currency, &pl.GueltigVon, &pl.GueltigBis, &pl.Aktiv, &pl.CreatedAt, &pl.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Preisliste nicht gefunden")
		}
		return nil, err
	}
	return &pl, nil
}

// PriceListItem (Staffelpreis-Zeile einer Preisliste).
type PriceListItem struct {
	ID          string    `json:"id"`
	PriceListID string    `json:"price_list_id"`
	MaterialID  string    `json:"material_id"`
	MinMenge    float64   `json:"min_menge"`
	UnitPrice   float64   `json:"unit_price"`
	CreatedAt   time.Time `json:"created_at"`
}

type PriceListItemCreate struct {
	MaterialID string  `json:"material_id"`
	MinMenge   float64 `json:"min_menge"`
	UnitPrice  float64 `json:"unit_price"`
}

// priceListOwnedByCompany prueft, ob die Preisliste priceListID zum
// Mandanten companyID gehoert (price_list_items haben kein eigenes
// company_id, siehe ADR 0007 / ADR 0002-Muster).
func (s *Service) priceListOwnedByCompany(ctx context.Context, priceListID, companyID string) (bool, error) {
	var exists bool
	err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM price_lists WHERE id=$1 AND company_id=$2)`, priceListID, companyID).Scan(&exists)
	return exists, err
}

func (s *Service) CreatePriceListItem(ctx context.Context, priceListID string, in PriceListItemCreate, companyID string) (*PriceListItem, error) {
	if strings.TrimSpace(in.MaterialID) == "" {
		return nil, errors.New("material_id erforderlich")
	}
	if in.MinMenge < 0 {
		return nil, errors.New("Mindestmenge darf nicht negativ sein")
	}
	if in.UnitPrice < 0 {
		return nil, errors.New("Preis darf nicht negativ sein")
	}
	owned, err := s.priceListOwnedByCompany(ctx, priceListID, companyID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Preisliste nicht gefunden")
	}
	// s.Get dient hier zugleich als Existenz- UND Mandanten-Ownership-Pruefung
	// fuer das Material, analog zu StockByMaterial (siehe service.go).
	if _, err := s.Get(ctx, in.MaterialID, companyID); err != nil {
		return nil, err
	}
	var duplicate bool
	if err := s.pg.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM price_list_items WHERE price_list_id=$1 AND material_id=$2 AND min_menge=$3)
    `, priceListID, in.MaterialID, in.MinMenge).Scan(&duplicate); err != nil {
		return nil, err
	}
	if duplicate {
		return nil, errors.New("Für diese Mindestmenge existiert bereits eine Preisstaffel für dieses Material")
	}
	id := uuid.NewString()
	var item PriceListItem
	err = s.pg.QueryRow(ctx, `
        INSERT INTO price_list_items (id, price_list_id, material_id, min_menge, unit_price)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING id, price_list_id, material_id, min_menge, unit_price, created_at
    `, id, priceListID, in.MaterialID, in.MinMenge, in.UnitPrice).Scan(
		&item.ID, &item.PriceListID, &item.MaterialID, &item.MinMenge, &item.UnitPrice, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListPriceListItems(ctx context.Context, priceListID string, companyID string) ([]PriceListItem, error) {
	owned, err := s.priceListOwnedByCompany(ctx, priceListID, companyID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Preisliste nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, price_list_id, material_id, min_menge, unit_price, created_at
        FROM price_list_items WHERE price_list_id=$1
        ORDER BY material_id ASC, min_menge ASC
    `, priceListID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PriceListItem, 0)
	for rows.Next() {
		var item PriceListItem
		if err := rows.Scan(&item.ID, &item.PriceListID, &item.MaterialID, &item.MinMenge, &item.UnitPrice, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) DeletePriceListItem(ctx context.Context, priceListID, itemID string, companyID string) error {
	owned, err := s.priceListOwnedByCompany(ctx, priceListID, companyID)
	if err != nil {
		return err
	}
	if !owned {
		return errors.New("Preisliste nicht gefunden")
	}
	if _, err := s.pg.Exec(ctx, `DELETE FROM price_list_items WHERE id=$1 AND price_list_id=$2`, itemID, priceListID); err != nil {
		return err
	}
	return nil
}

// EffectivePrice ist das Ergebnis von EffectivePriceForMaterial.
type EffectivePrice struct {
	PriceListID   string  `json:"price_list_id"`
	PriceListName string  `json:"price_list_name"`
	MinMenge      float64 `json:"min_menge"`
	UnitPrice     float64 `json:"unit_price"`
	Currency      string  `json:"currency"`
}

// EffectivePriceForMaterial ermittelt den geltenden Preis fuer ein Material
// bei gegebener Menge zu einem Stichtag (Backlog A.2.3). Staffelpreis-
// Semantik: die Zeile mit der groessten min_menge <= menge gewinnt. Sind
// mehrere Preislisten gleichzeitig gueltig, gewinnt die mit dem spaetesten
// gueltig_von (die "neueste" Liste) - diese Prioritaet wurde bewusst erst
// hier im Anwendungscode entschieden, siehe ADR 0007 ("Lookup-Logik ...
// wird bewusst nicht in dieser ADR festgelegt"). Liefert (nil, nil), wenn
// keine Preisliste einen Treffer liefert - das ist kein Fehler, sondern
// bedeutet "keine Preisliste zustaendig" (Aufrufer entscheidet ueber
// Fallback, z. B. auf materials.avg_purchase_price).
func (s *Service) EffectivePriceForMaterial(ctx context.Context, materialID string, menge float64, at time.Time, companyID string) (*EffectivePrice, error) {
	if strings.TrimSpace(materialID) == "" {
		return nil, errors.New("material_id erforderlich")
	}
	if menge < 0 {
		return nil, errors.New("Menge darf nicht negativ sein")
	}
	if at.IsZero() {
		at = time.Now()
	}
	var ep EffectivePrice
	err := s.pg.QueryRow(ctx, `
        SELECT pl.id, pl.name, pli.min_menge, pli.unit_price, pl.currency
        FROM price_list_items pli
        JOIN price_lists pl ON pl.id = pli.price_list_id
        WHERE pli.material_id = $1
          AND pl.company_id = $2
          AND pl.aktiv = true
          AND pli.min_menge <= $3
          AND pl.gueltig_von <= $4
          AND (pl.gueltig_bis IS NULL OR pl.gueltig_bis >= $4)
        ORDER BY pl.gueltig_von DESC, pli.min_menge DESC
        LIMIT 1
    `, materialID, companyID, menge, at).Scan(&ep.PriceListID, &ep.PriceListName, &ep.MinMenge, &ep.UnitPrice, &ep.Currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &ep, nil
}
