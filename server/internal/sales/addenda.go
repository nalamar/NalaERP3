package sales

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"nalaerp3/internal/auditlog"
)

// SalesOrderAddendum (Nachtrag) — Backlog B.2, siehe
// docs/adr/0010-nachtragsmanagement.md. Der Grundauftrag (sales_orders)
// bleibt beim Anlegen/Entscheiden eines Nachtrags unveraendert; die
// effektive Auftragssumme wird erst zur Lesezeit berechnet, siehe
// EffectiveOrderTotals.
type SalesOrderAddendum struct {
	ID              uuid.UUID                `json:"id"`
	SalesOrderID    uuid.UUID                `json:"sales_order_id"`
	NachtragNo      int                      `json:"nachtrag_no"`
	Status          string                   `json:"status"`
	Begruendung     string                   `json:"begruendung"`
	Currency        string                   `json:"currency"`
	NetAmount       float64                  `json:"net_amount"`
	TaxAmount       float64                  `json:"tax_amount"`
	GrossAmount     float64                  `json:"gross_amount"`
	BeantragtAm     *time.Time               `json:"beantragt_am,omitempty"`
	EntschiedenAm   *time.Time               `json:"entschieden_am,omitempty"`
	EntschiedenVon  string                   `json:"entschieden_von,omitempty"`
	Ablehnungsgrund string                   `json:"ablehnungsgrund,omitempty"`
	CreatedAt       time.Time                `json:"created_at"`
	Items           []SalesOrderAddendumItem `json:"items,omitempty"`
}

type SalesOrderAddendumItem struct {
	ID          string  `json:"id"`
	Position    int     `json:"position"`
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TaxCode     string  `json:"tax_code"`
}

type SalesOrderAddendumCreate struct {
	Begruendung string `json:"begruendung"`
}

type SalesOrderAddendumItemInput struct {
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TaxCode     string  `json:"tax_code"`
}

type SalesOrderAddendumItemUpdate struct {
	Description *string  `json:"description"`
	Qty         *float64 `json:"qty"`
	Unit        *string  `json:"unit"`
	UnitPrice   *float64 `json:"unit_price"`
	TaxCode     *string  `json:"tax_code"`
}

type SalesOrderAddendumRejection struct {
	Ablehnungsgrund string `json:"ablehnungsgrund"`
}

func AddendumStatuses() []string {
	return []string{"entwurf", "beantragt", "angenommen", "abgelehnt"}
}

func isAddendumEditableStatus(status string) bool {
	return status == "entwurf"
}

func validateAddendumStatusTransition(current, next string) error {
	switch current {
	case "entwurf":
		if next == "beantragt" {
			return nil
		}
	case "beantragt":
		if next == "angenommen" || next == "abgelehnt" {
			return nil
		}
	case "angenommen", "abgelehnt":
		return errors.New("entschiedene Nachträge können nicht erneut umgestellt werden")
	}
	return errors.New("Nachtragsstatus darf nicht in den gewünschten Status wechseln")
}

// ensureAddendumEditableTx prueft Ownership (ueber sales_order_id UND
// company_id) UND dass der Nachtrag im Entwurfsstatus ist - nur dann
// duerfen seine Positionen veraendert werden (ADR 0010: "nur entwurf ist
// editierbar"). FOR UPDATE OF soa sperrt bewusst nur die Nachtragszeile,
// nicht die per JOIN gelesene sales_orders-Zeile.
func ensureAddendumEditableTx(ctx context.Context, tx pgx.Tx, orderID, addendumID uuid.UUID, companyID string) error {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT soa.status
		FROM sales_order_addenda soa
		JOIN sales_orders so ON so.id = soa.sales_order_id
		WHERE soa.id=$1 AND soa.sales_order_id=$2 AND so.company_id=$3
		FOR UPDATE OF soa
	`, addendumID, orderID, companyID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("Nachtrag nicht gefunden")
		}
		return err
	}
	if !isAddendumEditableStatus(status) {
		return errors.New("nur Nachträge im Entwurfsstatus können bearbeitet werden")
	}
	return nil
}

func (s *Service) CreateAddendum(ctx context.Context, orderID uuid.UUID, in SalesOrderAddendumCreate, companyID string) (*SalesOrderAddendum, error) {
	begruendung := strings.TrimSpace(in.Begruendung)
	if begruendung == "" {
		return nil, errors.New("Begründung erforderlich")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// ensureOrderEditableTx sperrt die sales_orders-Zeile (FOR UPDATE) und
	// serialisiert dadurch gleichzeitige Nachtrag-Anlagen fuer denselben
	// Auftrag - kein separater Advisory-Lock fuer die nachtrag_no-Vergabe
	// noetig.
	if err := ensureOrderEditableTx(ctx, tx, orderID, companyID); err != nil {
		return nil, err
	}

	var currency string
	if err := tx.QueryRow(ctx, `SELECT currency FROM sales_orders WHERE id=$1`, orderID).Scan(&currency); err != nil {
		return nil, err
	}

	id := uuid.New()
	var a SalesOrderAddendum
	err = tx.QueryRow(ctx, `
        INSERT INTO sales_order_addenda (id, sales_order_id, nachtrag_no, status, begruendung, currency)
        VALUES ($1,$2,(SELECT COALESCE(MAX(nachtrag_no),0)+1 FROM sales_order_addenda WHERE sales_order_id=$2),'entwurf',$3,$4)
        RETURNING id, sales_order_id, nachtrag_no, status, begruendung, currency, net_amount, tax_amount, gross_amount, created_at
    `, id, orderID, begruendung, currency).Scan(
		&a.ID, &a.SalesOrderID, &a.NachtragNo, &a.Status, &a.Begruendung, &a.Currency, &a.NetAmount, &a.TaxAmount, &a.GrossAmount, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) ListAddenda(ctx context.Context, orderID uuid.UUID, companyID string) ([]SalesOrderAddendum, error) {
	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sales_orders WHERE id=$1 AND company_id=$2)`, orderID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Auftrag nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, sales_order_id, nachtrag_no, status, begruendung, currency, net_amount, tax_amount, gross_amount, beantragt_am, entschieden_am, COALESCE(entschieden_von,''), ablehnungsgrund, created_at
        FROM sales_order_addenda WHERE sales_order_id=$1
        ORDER BY nachtrag_no ASC
    `, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SalesOrderAddendum, 0)
	for rows.Next() {
		var a SalesOrderAddendum
		if err := rows.Scan(&a.ID, &a.SalesOrderID, &a.NachtragNo, &a.Status, &a.Begruendung, &a.Currency, &a.NetAmount, &a.TaxAmount, &a.GrossAmount, &a.BeantragtAm, &a.EntschiedenAm, &a.EntschiedenVon, &a.Ablehnungsgrund, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Service) GetAddendum(ctx context.Context, orderID, addendumID uuid.UUID, companyID string) (*SalesOrderAddendum, error) {
	var a SalesOrderAddendum
	err := s.pg.QueryRow(ctx, `
        SELECT soa.id, soa.sales_order_id, soa.nachtrag_no, soa.status, soa.begruendung, soa.currency, soa.net_amount, soa.tax_amount, soa.gross_amount, soa.beantragt_am, soa.entschieden_am, COALESCE(soa.entschieden_von,''), soa.ablehnungsgrund, soa.created_at
        FROM sales_order_addenda soa
        JOIN sales_orders so ON so.id = soa.sales_order_id
        WHERE soa.id=$1 AND soa.sales_order_id=$2 AND so.company_id=$3
    `, addendumID, orderID, companyID).Scan(
		&a.ID, &a.SalesOrderID, &a.NachtragNo, &a.Status, &a.Begruendung, &a.Currency, &a.NetAmount, &a.TaxAmount, &a.GrossAmount, &a.BeantragtAm, &a.EntschiedenAm, &a.EntschiedenVon, &a.Ablehnungsgrund, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachtrag nicht gefunden")
		}
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `SELECT id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'') FROM sales_order_addendum_items WHERE addendum_id=$1 ORDER BY position`, addendumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item SalesOrderAddendumItem
		if err := rows.Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode); err != nil {
			return nil, err
		}
		a.Items = append(a.Items, item)
	}
	return &a, nil
}

func refreshAddendumTotalsTx(ctx context.Context, tx pgx.Tx, addendumID uuid.UUID) error {
	codes, err := loadTaxCodesTx(ctx, tx)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT qty, unit_price, COALESCE(tax_code,'') FROM sales_order_addendum_items WHERE addendum_id=$1`, addendumID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var netAmount, taxAmount float64
	for rows.Next() {
		var qty, unitPrice float64
		var taxCode string
		if err := rows.Scan(&qty, &unitPrice, &taxCode); err != nil {
			return err
		}
		lineNet := qty * unitPrice
		netAmount += lineNet
		rate, err := taxRate(codes, taxCode)
		if err != nil {
			return err
		}
		taxAmount += lineNet * rate
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE sales_order_addenda SET net_amount=$2, tax_amount=$3, gross_amount=$4 WHERE id=$1`, addendumID, netAmount, taxAmount, netAmount+taxAmount)
	return err
}

func (s *Service) CreateAddendumItem(ctx context.Context, orderID, addendumID uuid.UUID, in SalesOrderAddendumItemInput, companyID string) (*SalesOrderAddendumItem, error) {
	taxCode := normalizeTaxCode(in.TaxCode)
	if err := validateItemInput(in.Description, in.Qty, in.Unit, in.UnitPrice, taxCode); err != nil {
		return nil, err
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureAddendumEditableTx(ctx, tx, orderID, addendumID, companyID); err != nil {
		return nil, err
	}
	codes, err := loadTaxCodesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	rate, err := taxRate(codes, taxCode)
	if err != nil {
		return nil, err
	}
	var item SalesOrderAddendumItem
	err = tx.QueryRow(ctx, `
        INSERT INTO sales_order_addendum_items (id, addendum_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code)
        VALUES (
            $1, $2,
            (SELECT COALESCE(MAX(position), 0) + 1 FROM sales_order_addendum_items WHERE addendum_id=$2),
            $3, $4, $5, $6, $7, $8, $9
        )
        RETURNING id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'')
    `,
		uuid.New(),
		addendumID,
		strings.TrimSpace(in.Description),
		in.Qty,
		strings.TrimSpace(in.Unit),
		in.UnitPrice,
		in.Qty*in.UnitPrice,
		in.Qty*in.UnitPrice*rate,
		nullIfEmpty(taxCode),
	).Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode)
	if err != nil {
		return nil, err
	}
	if err := refreshAddendumTotalsTx(ctx, tx, addendumID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdateAddendumItem(ctx context.Context, orderID, addendumID, itemID uuid.UUID, in SalesOrderAddendumItemUpdate, companyID string) (*SalesOrderAddendumItem, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := ensureAddendumEditableTx(ctx, tx, orderID, addendumID, companyID); err != nil {
		return nil, err
	}
	var current SalesOrderAddendumItem
	if err := tx.QueryRow(ctx, `SELECT id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'') FROM sales_order_addendum_items WHERE addendum_id=$1 AND id=$2`, addendumID, itemID).
		Scan(&current.ID, &current.Position, &current.Description, &current.Qty, &current.Unit, &current.UnitPrice, &current.TaxCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachtragsposition nicht gefunden")
		}
		return nil, err
	}

	description := current.Description
	qty := current.Qty
	unit := current.Unit
	unitPrice := current.UnitPrice
	taxCode := current.TaxCode
	if in.Description != nil {
		description = strings.TrimSpace(*in.Description)
	}
	if in.Qty != nil {
		qty = *in.Qty
	}
	if in.Unit != nil {
		unit = strings.TrimSpace(*in.Unit)
	}
	if in.UnitPrice != nil {
		unitPrice = *in.UnitPrice
	}
	if in.TaxCode != nil {
		taxCode = normalizeTaxCode(*in.TaxCode)
	}
	if err := validateItemInput(description, qty, unit, unitPrice, taxCode); err != nil {
		return nil, err
	}
	codes, err := loadTaxCodesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	rate, err := taxRate(codes, taxCode)
	if err != nil {
		return nil, err
	}
	var item SalesOrderAddendumItem
	err = tx.QueryRow(ctx, `
        UPDATE sales_order_addendum_items
        SET description=$3, qty=$4, unit=$5, unit_price=$6, net_amount=$7, tax_amount=$8, tax_code=$9
        WHERE addendum_id=$1 AND id=$2
        RETURNING id::text, position, description, qty, unit, unit_price, COALESCE(tax_code,'')
    `,
		addendumID, itemID, description, qty, unit, unitPrice, qty*unitPrice, qty*unitPrice*rate, nullIfEmpty(taxCode),
	).Scan(&item.ID, &item.Position, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode)
	if err != nil {
		return nil, err
	}
	if err := refreshAddendumTotalsTx(ctx, tx, addendumID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeleteAddendumItem(ctx context.Context, orderID, addendumID, itemID uuid.UUID, companyID string) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureAddendumEditableTx(ctx, tx, orderID, addendumID, companyID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM sales_order_addendum_items WHERE addendum_id=$1 AND id=$2`, addendumID, itemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("Nachtragsposition nicht gefunden")
	}
	if err := refreshAddendumTotalsTx(ctx, tx, addendumID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// SubmitAddendum stellt einen Nachtrag von entwurf auf beantragt um -
// friert damit seine Positionen ein (Festschreibung ab Antragstellung,
// GoBD-Geist, analog zur Auftrags-Festschreibung aus Backlog 0.3.2).
func (s *Service) SubmitAddendum(ctx context.Context, orderID, addendumID uuid.UUID, companyID string, actorUserID string) (*SalesOrderAddendum, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx, `
		SELECT soa.status
		FROM sales_order_addenda soa
		JOIN sales_orders so ON so.id = soa.sales_order_id
		WHERE soa.id=$1 AND soa.sales_order_id=$2 AND so.company_id=$3
		FOR UPDATE OF soa
	`, addendumID, orderID, companyID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachtrag nicht gefunden")
		}
		return nil, err
	}
	if err := validateAddendumStatusTransition(status, "beantragt"); err != nil {
		return nil, err
	}
	var itemCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM sales_order_addendum_items WHERE addendum_id=$1`, addendumID).Scan(&itemCount); err != nil {
		return nil, err
	}
	if itemCount == 0 {
		return nil, errors.New("keine Positionen")
	}
	if _, err := tx.Exec(ctx, `UPDATE sales_order_addenda SET status='beantragt', beantragt_am=now() WHERE id=$1`, addendumID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAddendum(ctx, orderID, addendumID, companyID)
}

// DecideAddendum entscheidet einen beantragten Nachtrag (angenommen oder
// abgelehnt) und protokolliert die Entscheidung ueber das generische
// Aenderungsprotokoll - das fachlich zentrale, GoBD-relevante Ereignis
// eines Nachtragsmanagements (ADR 0010). accepted=true -> angenommen,
// sonst abgelehnt (erfordert dann eine Ablehnungsbegruendung).
func (s *Service) DecideAddendum(ctx context.Context, orderID, addendumID uuid.UUID, accepted bool, rejection SalesOrderAddendumRejection, companyID string, actorUserID string) (*SalesOrderAddendum, error) {
	nextStatus := "angenommen"
	ablehnungsgrund := ""
	if !accepted {
		nextStatus = "abgelehnt"
		ablehnungsgrund = strings.TrimSpace(rejection.Ablehnungsgrund)
		if ablehnungsgrund == "" {
			return nil, errors.New("Ablehnungsgrund erforderlich")
		}
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT soa.status
		FROM sales_order_addenda soa
		JOIN sales_orders so ON so.id = soa.sales_order_id
		WHERE soa.id=$1 AND soa.sales_order_id=$2 AND so.company_id=$3
		FOR UPDATE OF soa
	`, addendumID, orderID, companyID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Nachtrag nicht gefunden")
		}
		return nil, err
	}
	if err := validateAddendumStatusTransition(currentStatus, nextStatus); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
        UPDATE sales_order_addenda
        SET status=$2, entschieden_am=now(), entschieden_von=$3, ablehnungsgrund=$4
        WHERE id=$1
    `, addendumID, nextStatus, nullIfEmpty(actorUserID), ablehnungsgrund); err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, tx, companyID, auditlog.RecordInput{
			EntityType:  "sales_order_addendum",
			EntityID:    addendumID.String(),
			Action:      "entschieden",
			ActorUserID: actorUserID,
			Before:      map[string]any{"status": currentStatus},
			After:       map[string]any{"status": nextStatus},
			Note:        ablehnungsgrund,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAddendum(ctx, orderID, addendumID, companyID)
}

// EffectiveOrderTotals berechnet die effektive Auftragssumme (Grundauftrag
// + Summe aller angenommenen Nachtraege) rein zur Lesezeit - sales_orders
// selbst wird dabei NIE geschrieben (ADR 0010: keine stille Mutation
// bereits festgeschriebener Summen).
type EffectiveOrderTotals struct {
	BaseNetAmount            float64 `json:"base_net_amount"`
	BaseTaxAmount            float64 `json:"base_tax_amount"`
	BaseGrossAmount          float64 `json:"base_gross_amount"`
	ApprovedAddendaNetAmount float64 `json:"approved_addenda_net_amount"`
	ApprovedAddendaTaxAmount float64 `json:"approved_addenda_tax_amount"`
	ApprovedAddendaGross     float64 `json:"approved_addenda_gross_amount"`
	EffectiveNetAmount       float64 `json:"effective_net_amount"`
	EffectiveTaxAmount       float64 `json:"effective_tax_amount"`
	EffectiveGrossAmount     float64 `json:"effective_gross_amount"`
}

func (s *Service) EffectiveOrderTotals(ctx context.Context, orderID uuid.UUID, companyID string) (*EffectiveOrderTotals, error) {
	order, err := s.Get(ctx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	var approvedNet, approvedTax, approvedGross float64
	if err := s.pg.QueryRow(ctx, `
        SELECT COALESCE(SUM(net_amount),0), COALESCE(SUM(tax_amount),0), COALESCE(SUM(gross_amount),0)
        FROM sales_order_addenda
        WHERE sales_order_id=$1 AND status='angenommen'
    `, orderID).Scan(&approvedNet, &approvedTax, &approvedGross); err != nil {
		return nil, err
	}
	return &EffectiveOrderTotals{
		BaseNetAmount:            order.NetAmount,
		BaseTaxAmount:            order.TaxAmount,
		BaseGrossAmount:          order.GrossAmount,
		ApprovedAddendaNetAmount: approvedNet,
		ApprovedAddendaTaxAmount: approvedTax,
		ApprovedAddendaGross:     approvedGross,
		EffectiveNetAmount:       order.NetAmount + approvedNet,
		EffectiveTaxAmount:       order.TaxAmount + approvedTax,
		EffectiveGrossAmount:     order.GrossAmount + approvedGross,
	}, nil
}
