package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"nalaerp3/internal/accounting"
	"nalaerp3/internal/projects"
	"nalaerp3/internal/settings"
)

type QuoteItemInput struct {
	ID                      string              `json:"id,omitempty"`
	Description             string              `json:"description"`
	Qty                     float64             `json:"qty"`
	Unit                    string              `json:"unit"`
	UnitPrice               float64             `json:"unit_price"`
	TaxCode                 string              `json:"tax_code"`
	MaterialID              string              `json:"material_id,omitempty"`
	PriceMappingStatus      string              `json:"price_mapping_status,omitempty"`
	MaterialCandidateStatus string              `json:"material_candidate_status,omitempty"`
	MaterialCandidates      []MaterialCandidate `json:"material_candidates,omitempty"`
}

type MaterialCandidate struct {
	MaterialID    string `json:"material_id"`
	MaterialNo    string `json:"material_no,omitempty"`
	MaterialLabel string `json:"material_label,omitempty"`
}

type QuoteInput struct {
	ProjectID  string           `json:"project_id"`
	ContactID  string           `json:"contact_id"`
	QuoteDate  time.Time        `json:"quote_date"`
	ValidUntil *time.Time       `json:"valid_until,omitempty"`
	Currency   string           `json:"currency"`
	Note       string           `json:"note"`
	Items      []QuoteItemInput `json:"items"`
}

type Quote struct {
	ID                  uuid.UUID        `json:"id"`
	Number              string           `json:"number"`
	RootQuoteID         string           `json:"root_quote_id"`
	RevisionNo          int              `json:"revision_no"`
	SupersededByQuoteID string           `json:"superseded_by_quote_id,omitempty"`
	ProjectID           string           `json:"project_id"`
	ProjectName         string           `json:"project_name"`
	ContactID           string           `json:"contact_id"`
	ContactName         string           `json:"contact_name"`
	Status              string           `json:"status"`
	AcceptedAt          *time.Time       `json:"accepted_at,omitempty"`
	LinkedInvoiceOutID  string           `json:"linked_invoice_out_id,omitempty"`
	LinkedSalesOrderID  string           `json:"linked_sales_order_id,omitempty"`
	QuoteDate           time.Time        `json:"quote_date"`
	ValidUntil          *time.Time       `json:"valid_until,omitempty"`
	Currency            string           `json:"currency"`
	Note                string           `json:"note"`
	NetAmount           float64          `json:"net_amount"`
	TaxAmount           float64          `json:"tax_amount"`
	GrossAmount         float64          `json:"gross_amount"`
	Items               []QuoteItemInput `json:"items"`
}

type QuoteListItem struct {
	ID                  uuid.UUID  `json:"id"`
	Number              string     `json:"number"`
	RootQuoteID         string     `json:"root_quote_id"`
	RevisionNo          int        `json:"revision_no"`
	SupersededByQuoteID string     `json:"superseded_by_quote_id,omitempty"`
	ProjectID           string     `json:"project_id"`
	ProjectName         string     `json:"project_name"`
	ContactID           string     `json:"contact_id"`
	ContactName         string     `json:"contact_name"`
	Status              string     `json:"status"`
	AcceptedAt          *time.Time `json:"accepted_at,omitempty"`
	LinkedInvoiceOutID  string     `json:"linked_invoice_out_id,omitempty"`
	LinkedSalesOrderID  string     `json:"linked_sales_order_id,omitempty"`
	QuoteDate           time.Time  `json:"quote_date"`
	ValidUntil          *time.Time `json:"valid_until,omitempty"`
	Currency            string     `json:"currency"`
	GrossAmount         float64    `json:"gross_amount"`
}

type ConvertToInvoiceInput struct {
	InvoiceDate    time.Time  `json:"invoice_date"`
	DueDate        *time.Time `json:"due_date,omitempty"`
	RevenueAccount string     `json:"revenue_account"`
}

type ConvertToInvoiceResult struct {
	Quote   *Quote                 `json:"quote"`
	Invoice *accounting.InvoiceOut `json:"invoice"`
}

type ReviseResult struct {
	SourceQuote  *Quote `json:"source_quote"`
	RevisedQuote *Quote `json:"revised_quote"`
}

type AcceptInput struct {
	ProjectStatus string `json:"project_status"`
}

type AcceptResult struct {
	Quote   *Quote            `json:"quote"`
	Project *projects.Project `json:"project,omitempty"`
}

type QuoteFilter struct {
	Status    string
	ContactID string
	ProjectID string
	Search    string
	Limit     int
	Offset    int
}

type Service struct {
	pg      *pgxpool.Pool
	num     *settings.NumberingService
	mg      *mongo.Client
	mongoDB string
}

func NewService(pg *pgxpool.Pool, num *settings.NumberingService) *Service {
	return &Service{pg: pg, num: num}
}

func (s *Service) Create(ctx context.Context, in QuoteInput) (*Quote, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	id, _, err := s.createQuoteTx(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) createQuoteTx(ctx context.Context, tx pgx.Tx, in QuoteInput) (uuid.UUID, []uuid.UUID, error) {
	if strings.TrimSpace(in.ContactID) == "" && strings.TrimSpace(in.ProjectID) == "" {
		return uuid.Nil, nil, errors.New("contact_id oder project_id erforderlich")
	}
	if len(in.Items) == 0 {
		return uuid.Nil, nil, errors.New("keine Positionen")
	}
	if strings.TrimSpace(in.ProjectID) != "" {
		if strings.TrimSpace(in.ContactID) == "" {
			if err := tx.QueryRow(ctx, `SELECT COALESCE(kunde_id,'') FROM projects WHERE id=$1`, in.ProjectID).Scan(&in.ContactID); err != nil {
				return uuid.Nil, nil, err
			}
			if strings.TrimSpace(in.ContactID) == "" {
				return uuid.Nil, nil, errors.New("Projekt hat keinen Kunden")
			}
		}
	}
	if strings.TrimSpace(in.ContactID) == "" {
		return uuid.Nil, nil, errors.New("contact_id fehlt")
	}
	if strings.TrimSpace(in.Currency) == "" {
		in.Currency = "EUR"
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.QuoteDate.IsZero() {
		in.QuoteDate = time.Now()
	}
	number, err := s.num.Next(ctx, "quote")
	if err != nil {
		return uuid.Nil, nil, err
	}
	net, tax := calcTotals(in.Items)
	gross := net + tax
	id := uuid.New()

	_, err = tx.Exec(ctx, `INSERT INTO quotes (id, nummer, root_quote_id, revision_no, project_id, contact_id, status, quote_date, valid_until, currency, note, net_amount, tax_amount, gross_amount)
		VALUES ($1,$2,$1,1,$3,$4,'draft',$5,$6,$7,$8,$9,$10,$11)`,
		id, number, nullIfEmpty(in.ProjectID), in.ContactID, in.QuoteDate, in.ValidUntil, in.Currency, in.Note, net, tax, gross)
	if err != nil {
		return uuid.Nil, nil, err
	}

	itemIDs := make([]uuid.UUID, 0, len(in.Items))
	for idx, item := range in.Items {
		item, err = s.normalizeQuoteItem(ctx, tx, item)
		if err != nil {
			return uuid.Nil, nil, err
		}
		lineID := uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code, material_id, price_mapping_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			lineID, id, idx+1, item.Description, item.Qty, item.Unit, item.UnitPrice, item.Qty*item.UnitPrice, item.Qty*item.UnitPrice*taxRate(item.TaxCode), nullIfEmpty(item.TaxCode), nullIfEmpty(item.MaterialID), item.PriceMappingStatus)
		if err != nil {
			return uuid.Nil, nil, err
		}
		itemIDs = append(itemIDs, lineID)
	}
	return id, itemIDs, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Quote, error) {
	var out Quote
	var projectID sql.NullString
	var rootQuoteID uuid.UUID
	var validUntil sql.NullTime
	var acceptedAt sql.NullTime
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	var supersededByQuoteID uuid.NullUUID
	err := s.pg.QueryRow(ctx, `SELECT q.id, q.nummer, q.root_quote_id, q.revision_no, q.superseded_by_quote_id, q.project_id::text, COALESCE(p.name,''), q.contact_id, COALESCE(c.name,''), q.status, q.accepted_at, q.linked_invoice_out_id, q.linked_sales_order_id, q.quote_date, q.valid_until, q.currency, COALESCE(q.note,''), q.net_amount, q.tax_amount, q.gross_amount
		FROM quotes q
		LEFT JOIN projects p ON p.id = q.project_id
		LEFT JOIN contacts c ON c.id = q.contact_id
		WHERE q.id=$1`, id).Scan(
		&out.ID, &out.Number, &rootQuoteID, &out.RevisionNo, &supersededByQuoteID, &projectID, &out.ProjectName, &out.ContactID, &out.ContactName, &out.Status, &acceptedAt, &linkedInvoiceOutID, &linkedSalesOrderID, &out.QuoteDate, &validUntil, &out.Currency, &out.Note, &out.NetAmount, &out.TaxAmount, &out.GrossAmount,
	)
	if err != nil {
		return nil, err
	}
	out.RootQuoteID = rootQuoteID.String()
	if projectID.Valid {
		out.ProjectID = projectID.String
	}
	if validUntil.Valid {
		t := validUntil.Time
		out.ValidUntil = &t
	}
	if acceptedAt.Valid {
		t := acceptedAt.Time
		out.AcceptedAt = &t
	}
	if linkedInvoiceOutID.Valid {
		out.LinkedInvoiceOutID = linkedInvoiceOutID.UUID.String()
	}
	if linkedSalesOrderID.Valid {
		out.LinkedSalesOrderID = linkedSalesOrderID.UUID.String()
	}
	if supersededByQuoteID.Valid {
		out.SupersededByQuoteID = supersededByQuoteID.UUID.String()
	}
	rows, err := s.pg.Query(ctx, `
		SELECT
			qi.id,
			qi.description,
			qi.qty,
			qi.unit,
			qi.unit_price,
			COALESCE(qi.tax_code,''),
			COALESCE(qi.material_id,''),
			COALESCE(qi.price_mapping_status,'open'),
			CASE
				WHEN COALESCE(qi.material_id,'') <> '' THEN 'none'
				WHEN EXISTS (
					SELECT 1
					FROM quote_import_item_links qil
					WHERE qil.quote_item_id = qi.id
				) THEN 'available'
				ELSE 'none'
			END AS material_candidate_status
		FROM quote_items qi
		WHERE qi.quote_id=$1
		ORDER BY qi.position
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var quoteItemID uuid.UUID
		var item QuoteItemInput
		if err := rows.Scan(&quoteItemID, &item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode, &item.MaterialID, &item.PriceMappingStatus, &item.MaterialCandidateStatus); err != nil {
			return nil, err
		}
		item.ID = quoteItemID.String()
		item.MaterialCandidates, err = s.listMaterialCandidatesForQuoteItem(ctx, quoteItemID, item.MaterialID, item.MaterialCandidateStatus)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	return &out, nil
}

func (s *Service) ApplyMaterialCandidate(ctx context.Context, quoteID, itemID uuid.UUID, materialID string) (*Quote, error) {
	materialID = strings.TrimSpace(materialID)
	if materialID == "" {
		return nil, errors.New("material_id fehlt")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	if err := tx.QueryRow(ctx, `SELECT status, superseded_by_quote_id FROM quotes WHERE id=$1 FOR UPDATE`, quoteID).Scan(&currentStatus, &supersededByQuoteID); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var currentMaterialID string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(material_id,'') FROM quote_items WHERE id=$1 AND quote_id=$2 FOR UPDATE`, itemID, quoteID).Scan(&currentMaterialID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if strings.TrimSpace(currentMaterialID) != "" {
		return nil, errors.New("Angebotsposition hat bereits ein Material")
	}

	var matchedMaterialID string
	err = tx.QueryRow(ctx, `
		SELECT m.id
		FROM quote_import_item_links qil
		JOIN quote_import_items qii ON qii.id = qil.quote_import_item_id
		JOIN materials m ON (
			LOWER(m.bezeichnung) = LOWER(BTRIM(qii.description))
			OR LOWER(m.nummer) = LOWER(BTRIM(qii.description))
		)
		WHERE qil.quote_item_id = $1
		  AND m.id = $2
		  AND BTRIM(COALESCE(qii.description, '')) <> ''
		LIMIT 1
	`, itemID, materialID).Scan(&matchedMaterialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("material_id ist kein sichtbarer Kandidat")
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `UPDATE quote_items SET material_id=$2, price_mapping_status='manual' WHERE id=$1`, itemID, matchedMaterialID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, quoteID)
}

func (s *Service) listMaterialCandidatesForQuoteItem(ctx context.Context, quoteItemID uuid.UUID, materialID, candidateStatus string) ([]MaterialCandidate, error) {
	if strings.TrimSpace(materialID) != "" || candidateStatus != "available" {
		return nil, nil
	}

	rows, err := s.pg.Query(ctx, `
		SELECT DISTINCT m.id, m.nummer, m.bezeichnung
		FROM quote_import_item_links qil
		JOIN quote_import_items qii ON qii.id = qil.quote_import_item_id
		JOIN materials m ON LOWER(m.bezeichnung) = LOWER(BTRIM(qii.description))
			OR LOWER(m.nummer) = LOWER(BTRIM(qii.description))
		WHERE qil.quote_item_id = $1
		  AND BTRIM(COALESCE(qii.description, '')) <> ''
		ORDER BY m.bezeichnung ASC, m.nummer ASC
		LIMIT 3
	`, quoteItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]MaterialCandidate, 0, 3)
	for rows.Next() {
		var candidate MaterialCandidate
		if err := rows.Scan(&candidate.MaterialID, &candidate.MaterialNo, &candidate.MaterialLabel); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (s *Service) List(ctx context.Context, f QuoteFilter) ([]QuoteListItem, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args := make([]any, 0)
	conds := make([]string, 0)
	if strings.TrimSpace(f.Status) != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("q.status=$%d", len(args)))
	}
	if strings.TrimSpace(f.ContactID) != "" {
		args = append(args, f.ContactID)
		conds = append(conds, fmt.Sprintf("q.contact_id=$%d", len(args)))
	}
	if strings.TrimSpace(f.ProjectID) != "" {
		args = append(args, f.ProjectID)
		conds = append(conds, fmt.Sprintf("q.project_id::text=$%d", len(args)))
	}
	if strings.TrimSpace(f.Search) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(f.Search))+"%")
		conds = append(conds, fmt.Sprintf("(LOWER(q.nummer) LIKE $%d OR LOWER(COALESCE(c.name,'')) LIKE $%d)", len(args), len(args)))
	}
	args = append(args, f.Limit, f.Offset)
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	query := `SELECT q.id, q.nummer, q.root_quote_id, q.revision_no, q.superseded_by_quote_id, COALESCE(q.project_id::text,''), COALESCE(p.name,''), q.contact_id, COALESCE(c.name,''), q.status, q.accepted_at, q.linked_invoice_out_id, q.linked_sales_order_id, q.quote_date, q.valid_until, q.currency, q.gross_amount
		FROM quotes q
		LEFT JOIN projects p ON p.id = q.project_id
		LEFT JOIN contacts c ON c.id = q.contact_id` + where + `
		ORDER BY q.quote_date DESC, q.created_at DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QuoteListItem, 0)
	for rows.Next() {
		var item QuoteListItem
		var rootQuoteID uuid.UUID
		var validUntil sql.NullTime
		var acceptedAt sql.NullTime
		var linkedInvoiceOutID uuid.NullUUID
		var linkedSalesOrderID uuid.NullUUID
		var supersededByQuoteID uuid.NullUUID
		if err := rows.Scan(&item.ID, &item.Number, &rootQuoteID, &item.RevisionNo, &supersededByQuoteID, &item.ProjectID, &item.ProjectName, &item.ContactID, &item.ContactName, &item.Status, &acceptedAt, &linkedInvoiceOutID, &linkedSalesOrderID, &item.QuoteDate, &validUntil, &item.Currency, &item.GrossAmount); err != nil {
			return nil, err
		}
		item.RootQuoteID = rootQuoteID.String()
		if validUntil.Valid {
			t := validUntil.Time
			item.ValidUntil = &t
		}
		if acceptedAt.Valid {
			t := acceptedAt.Time
			item.AcceptedAt = &t
		}
		if linkedInvoiceOutID.Valid {
			item.LinkedInvoiceOutID = linkedInvoiceOutID.UUID.String()
		}
		if linkedSalesOrderID.Valid {
			item.LinkedSalesOrderID = linkedSalesOrderID.UUID.String()
		}
		if supersededByQuoteID.Valid {
			item.SupersededByQuoteID = supersededByQuoteID.UUID.String()
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*Quote, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "draft", "sent", "accepted", "rejected":
	default:
		return nil, errors.New("ungültiger Status")
	}
	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	if err := s.pg.QueryRow(ctx, `SELECT status, superseded_by_quote_id, linked_invoice_out_id, linked_sales_order_id FROM quotes WHERE id=$1`, id).Scan(&currentStatus, &supersededByQuoteID, &linkedInvoiceOutID, &linkedSalesOrderID); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if (linkedInvoiceOutID.Valid || linkedSalesOrderID.Valid) && currentStatus != status {
		return nil, errors.New("Angebot mit Folgebeleg kann nicht manuell umgestellt werden")
	}
	if status == "accepted" {
		if _, err := s.pg.Exec(ctx, `UPDATE quotes SET status=$2, accepted_at=COALESCE(accepted_at, now()) WHERE id=$1`, id, status); err != nil {
			return nil, err
		}
	} else {
		if _, err := s.pg.Exec(ctx, `UPDATE quotes SET status=$2, accepted_at=NULL WHERE id=$1`, id, status); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Service) ConvertToInvoice(ctx context.Context, id uuid.UUID, arSvc *accounting.ARService, in ConvertToInvoiceInput) (*ConvertToInvoiceResult, error) {
	if arSvc == nil {
		return nil, errors.New("invoice service fehlt")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	var contactID string
	var currency string
	var supersededByQuoteID uuid.NullUUID
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	err = tx.QueryRow(ctx, `SELECT status, contact_id, currency, superseded_by_quote_id, linked_invoice_out_id, linked_sales_order_id FROM quotes WHERE id=$1 FOR UPDATE`, id).Scan(&status, &contactID, &currency, &supersededByQuoteID, &linkedInvoiceOutID, &linkedSalesOrderID)
	if err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen können nicht in Folgebelege überführt werden")
	}
	if linkedInvoiceOutID.Valid {
		return nil, errors.New("Angebot wurde bereits in eine Rechnung überführt")
	}
	if linkedSalesOrderID.Valid {
		return nil, errors.New("Angebot wurde bereits in einen Auftrag überführt")
	}
	switch status {
	case "sent", "accepted":
	default:
		return nil, errors.New("nur versendete oder angenommene Angebote können in Rechnungen überführt werden")
	}
	rows, err := tx.Query(ctx, `SELECT description, qty, unit_price, COALESCE(tax_code,'') FROM quote_items WHERE quote_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]accounting.InvoiceItemInput, 0)
	revenueAccount := strings.TrimSpace(in.RevenueAccount)
	if revenueAccount == "" {
		revenueAccount = "8000"
	}
	for rows.Next() {
		var description string
		var qty float64
		var unitPrice float64
		var taxCode string
		if err := rows.Scan(&description, &qty, &unitPrice, &taxCode); err != nil {
			return nil, err
		}
		items = append(items, accounting.InvoiceItemInput{
			Description: description,
			Qty:         qty,
			UnitPrice:   unitPrice,
			TaxCode:     taxCode,
			AccountCode: revenueAccount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("keine Positionen")
	}
	if in.InvoiceDate.IsZero() {
		in.InvoiceDate = time.Now()
	}
	invoice, err := arSvc.CreateFromQuoteTx(ctx, tx, id, accounting.InvoiceOutInput{
		ContactID:   contactID,
		InvoiceDate: in.InvoiceDate,
		DueDate:     in.DueDate,
		Currency:    currency,
		Items:       items,
	})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE quotes SET status='accepted', accepted_at=COALESCE(accepted_at, now()), linked_invoice_out_id=$2 WHERE id=$1`, id, invoice.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	quote, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ConvertToInvoiceResult{
		Quote:   quote,
		Invoice: invoice,
	}, nil
}

func (s *Service) Revise(ctx context.Context, id uuid.UUID) (*ReviseResult, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var source Quote
	var projectID sql.NullString
	var rootQuoteID uuid.UUID
	var supersededByQuoteID uuid.NullUUID
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	var validUntil sql.NullTime
	err = tx.QueryRow(ctx, `SELECT id, nummer, root_quote_id, revision_no, superseded_by_quote_id, project_id::text, contact_id, status, quote_date, valid_until, currency, COALESCE(note,''), net_amount, tax_amount, gross_amount, linked_invoice_out_id, linked_sales_order_id
		FROM quotes
		WHERE id=$1
		FOR UPDATE`, id).Scan(
		&source.ID,
		&source.Number,
		&rootQuoteID,
		&source.RevisionNo,
		&supersededByQuoteID,
		&projectID,
		&source.ContactID,
		&source.Status,
		&source.QuoteDate,
		&validUntil,
		&source.Currency,
		&source.Note,
		&source.NetAmount,
		&source.TaxAmount,
		&source.GrossAmount,
		&linkedInvoiceOutID,
		&linkedSalesOrderID,
	)
	if err != nil {
		return nil, err
	}

	source.RootQuoteID = rootQuoteID.String()
	if projectID.Valid {
		source.ProjectID = projectID.String
	}
	if validUntil.Valid {
		t := validUntil.Time
		source.ValidUntil = &t
	}
	if linkedInvoiceOutID.Valid {
		source.LinkedInvoiceOutID = linkedInvoiceOutID.UUID.String()
	}
	if linkedSalesOrderID.Valid {
		source.LinkedSalesOrderID = linkedSalesOrderID.UUID.String()
	}
	if supersededByQuoteID.Valid {
		source.SupersededByQuoteID = supersededByQuoteID.UUID.String()
	}

	if supersededByQuoteID.Valid {
		return nil, errors.New("Angebot darf nicht erneut revidiert werden")
	}
	if linkedInvoiceOutID.Valid || linkedSalesOrderID.Valid {
		return nil, errors.New("Angebot mit Folgebeleg darf nicht revidiert werden")
	}
	switch source.Status {
	case "draft", "sent", "rejected":
	default:
		if source.Status == "accepted" {
			return nil, errors.New("Angenommene Angebote dürfen nicht revidiert werden")
		}
		return nil, errors.New("Angebot ist nicht im Status Entwurf, versendet oder abgelehnt")
	}

	var nextRevisionNo int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM quotes WHERE root_quote_id=$1`, rootQuoteID).Scan(&nextRevisionNo); err != nil {
		return nil, err
	}

	revisedQuoteID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO quotes (id, nummer, root_quote_id, revision_no, project_id, contact_id, status, quote_date, valid_until, currency, note, net_amount, tax_amount, gross_amount)
		VALUES ($1,$2,$3,$4,$5,$6,'draft',$7,$8,$9,$10,$11,$12,$13)`,
		revisedQuoteID,
		source.Number,
		rootQuoteID,
		nextRevisionNo,
		nullIfEmpty(source.ProjectID),
		source.ContactID,
		source.QuoteDate,
		source.ValidUntil,
		source.Currency,
		source.Note,
		source.NetAmount,
		source.TaxAmount,
		source.GrossAmount,
	)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `SELECT position, description, qty, unit, unit_price, net_amount, tax_amount, COALESCE(tax_code,''), COALESCE(material_id,''), COALESCE(price_mapping_status,'open')
		FROM quote_items
		WHERE quote_id=$1
		ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var position int
		var description string
		var qty float64
		var unit string
		var unitPrice float64
		var netAmount float64
		var taxAmount float64
		var taxCode string
		var materialID string
		var priceMappingStatus string
		if err := rows.Scan(&position, &description, &qty, &unit, &unitPrice, &netAmount, &taxAmount, &taxCode, &materialID, &priceMappingStatus); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code, material_id, price_mapping_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			uuid.New(),
			revisedQuoteID,
			position,
			description,
			qty,
			unit,
			unitPrice,
			netAmount,
			taxAmount,
			nullIfEmpty(taxCode),
			nullIfEmpty(materialID),
			priceMappingStatus,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `UPDATE quotes SET superseded_by_quote_id=$2 WHERE id=$1`, id, revisedQuoteID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	sourceQuote, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	revisedQuote, err := s.Get(ctx, revisedQuoteID)
	if err != nil {
		return nil, err
	}
	return &ReviseResult{
		SourceQuote:  sourceQuote,
		RevisedQuote: revisedQuote,
	}, nil
}

func (s *Service) Accept(ctx context.Context, id uuid.UUID, projectSvc *projects.Service, in AcceptInput) (*AcceptResult, error) {
	quote, err := s.UpdateStatus(ctx, id, "accepted")
	if err != nil {
		return nil, err
	}
	result := &AcceptResult{Quote: quote}
	if projectSvc != nil && strings.TrimSpace(in.ProjectStatus) != "" && strings.TrimSpace(quote.ProjectID) != "" {
		project, err := projectSvc.UpdateStatus(ctx, quote.ProjectID, strings.TrimSpace(in.ProjectStatus))
		if err != nil {
			return nil, err
		}
		result.Project = project
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in QuoteInput) (*Quote, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var currentProjectID sql.NullString
	var currentContactID string
	var supersededByQuoteID uuid.NullUUID
	err = tx.QueryRow(ctx, `SELECT status, superseded_by_quote_id, project_id::text, contact_id FROM quotes WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus, &supersededByQuoteID, &currentProjectID, &currentContactID)
	if err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	if strings.TrimSpace(in.ProjectID) == "" && currentProjectID.Valid {
		in.ProjectID = currentProjectID.String
	}
	if strings.TrimSpace(in.ContactID) == "" {
		in.ContactID = currentContactID
	}
	if strings.TrimSpace(in.ProjectID) != "" && strings.TrimSpace(in.ContactID) == "" {
		if err := tx.QueryRow(ctx, `SELECT COALESCE(kunde_id,'') FROM projects WHERE id=$1`, in.ProjectID).Scan(&in.ContactID); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(in.ContactID) == "" {
		return nil, errors.New("contact_id fehlt")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("keine Positionen")
	}
	if strings.TrimSpace(in.Currency) == "" {
		in.Currency = "EUR"
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.QuoteDate.IsZero() {
		in.QuoteDate = time.Now()
	}
	net, tax := calcTotals(in.Items)
	gross := net + tax

	_, err = tx.Exec(ctx, `UPDATE quotes
		SET project_id=$2, contact_id=$3, quote_date=$4, valid_until=$5, currency=$6, note=$7, net_amount=$8, tax_amount=$9, gross_amount=$10
		WHERE id=$1`,
		id, nullIfEmpty(in.ProjectID), in.ContactID, in.QuoteDate, in.ValidUntil, in.Currency, in.Note, net, tax, gross)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM quote_items WHERE quote_id=$1`, id); err != nil {
		return nil, err
	}
	for idx, item := range in.Items {
		item, err = s.normalizeQuoteItem(ctx, tx, item)
		if err != nil {
			return nil, err
		}
		lineID := uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code, material_id, price_mapping_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			lineID, id, idx+1, item.Description, item.Qty, item.Unit, item.UnitPrice, item.Qty*item.UnitPrice, item.Qty*item.UnitPrice*taxRate(item.TaxCode), nullIfEmpty(item.TaxCode), nullIfEmpty(item.MaterialID), item.PriceMappingStatus)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func calcTotals(items []QuoteItemInput) (net, tax float64) {
	for _, item := range items {
		n := item.Qty * item.UnitPrice
		net += n
		tax += n * taxRate(item.TaxCode)
	}
	return
}

func taxRate(code string) float64 {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "DE19":
		return 0.19
	case "DE7":
		return 0.07
	default:
		return 0
	}
}

func (s *Service) normalizeQuoteItem(ctx context.Context, tx pgx.Tx, item QuoteItemInput) (QuoteItemInput, error) {
	if strings.TrimSpace(item.Description) == "" {
		return item, errors.New("Beschreibung erforderlich")
	}
	if item.Qty == 0 {
		item.Qty = 1
	}
	if strings.TrimSpace(item.Unit) == "" {
		item.Unit = "Stk"
	}
	item.MaterialID = strings.TrimSpace(item.MaterialID)
	item.PriceMappingStatus = strings.ToLower(strings.TrimSpace(item.PriceMappingStatus))
	if item.PriceMappingStatus == "" {
		item.PriceMappingStatus = "open"
	}
	switch item.PriceMappingStatus {
	case "open", "manual":
	default:
		return item, errors.New("price_mapping_status ist ungültig")
	}
	if item.MaterialID != "" {
		var exists string
		if err := tx.QueryRow(ctx, `SELECT id FROM materials WHERE id=$1`, item.MaterialID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return item, errors.New("material_id ist ungültig")
			}
			return item, err
		}
	}
	return item, nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
