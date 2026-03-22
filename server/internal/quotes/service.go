package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/accounting"
	"nalaerp3/internal/projects"
	"nalaerp3/internal/settings"
)

type QuoteItemInput struct {
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TaxCode     string  `json:"tax_code"`
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
	ID                 uuid.UUID        `json:"id"`
	Number             string           `json:"number"`
	ProjectID          string           `json:"project_id"`
	ProjectName        string           `json:"project_name"`
	ContactID          string           `json:"contact_id"`
	ContactName        string           `json:"contact_name"`
	Status             string           `json:"status"`
	AcceptedAt         *time.Time       `json:"accepted_at,omitempty"`
	LinkedInvoiceOutID string           `json:"linked_invoice_out_id,omitempty"`
	LinkedSalesOrderID string           `json:"linked_sales_order_id,omitempty"`
	QuoteDate          time.Time        `json:"quote_date"`
	ValidUntil         *time.Time       `json:"valid_until,omitempty"`
	Currency           string           `json:"currency"`
	Note               string           `json:"note"`
	NetAmount          float64          `json:"net_amount"`
	TaxAmount          float64          `json:"tax_amount"`
	GrossAmount        float64          `json:"gross_amount"`
	Items              []QuoteItemInput `json:"items"`
}

type QuoteListItem struct {
	ID                 uuid.UUID  `json:"id"`
	Number             string     `json:"number"`
	ProjectID          string     `json:"project_id"`
	ProjectName        string     `json:"project_name"`
	ContactID          string     `json:"contact_id"`
	ContactName        string     `json:"contact_name"`
	Status             string     `json:"status"`
	AcceptedAt         *time.Time `json:"accepted_at,omitempty"`
	LinkedInvoiceOutID string     `json:"linked_invoice_out_id,omitempty"`
	LinkedSalesOrderID string     `json:"linked_sales_order_id,omitempty"`
	QuoteDate          time.Time  `json:"quote_date"`
	ValidUntil         *time.Time `json:"valid_until,omitempty"`
	Currency           string     `json:"currency"`
	GrossAmount        float64    `json:"gross_amount"`
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
	pg  *pgxpool.Pool
	num *settings.NumberingService
}

func NewService(pg *pgxpool.Pool, num *settings.NumberingService) *Service {
	return &Service{pg: pg, num: num}
}

func (s *Service) Create(ctx context.Context, in QuoteInput) (*Quote, error) {
	if strings.TrimSpace(in.ContactID) == "" && strings.TrimSpace(in.ProjectID) == "" {
		return nil, errors.New("contact_id oder project_id erforderlich")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("keine Positionen")
	}
	if strings.TrimSpace(in.ProjectID) != "" {
		if strings.TrimSpace(in.ContactID) == "" {
			if err := s.pg.QueryRow(ctx, `SELECT COALESCE(kunde_id,'') FROM projects WHERE id=$1`, in.ProjectID).Scan(&in.ContactID); err != nil {
				return nil, err
			}
			if strings.TrimSpace(in.ContactID) == "" {
				return nil, errors.New("Projekt hat keinen Kunden")
			}
		}
	}
	if strings.TrimSpace(in.ContactID) == "" {
		return nil, errors.New("contact_id fehlt")
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
		return nil, err
	}
	net, tax := calcTotals(in.Items)
	gross := net + tax
	id := uuid.New()

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `INSERT INTO quotes (id, nummer, project_id, contact_id, status, quote_date, valid_until, currency, note, net_amount, tax_amount, gross_amount)
		VALUES ($1,$2,$3,$4,'draft',$5,$6,$7,$8,$9,$10,$11)`,
		id, number, nullIfEmpty(in.ProjectID), in.ContactID, in.QuoteDate, in.ValidUntil, in.Currency, in.Note, net, tax, gross)
	if err != nil {
		return nil, err
	}

	for idx, item := range in.Items {
		if strings.TrimSpace(item.Description) == "" {
			return nil, errors.New("Beschreibung erforderlich")
		}
		if item.Qty == 0 {
			item.Qty = 1
		}
		if strings.TrimSpace(item.Unit) == "" {
			item.Unit = "Stk"
		}
		lineID := uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			lineID, id, idx+1, item.Description, item.Qty, item.Unit, item.UnitPrice, item.Qty*item.UnitPrice, item.Qty*item.UnitPrice*taxRate(item.TaxCode), nullIfEmpty(item.TaxCode))
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Quote, error) {
	var out Quote
	var projectID sql.NullString
	var validUntil sql.NullTime
	var acceptedAt sql.NullTime
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	err := s.pg.QueryRow(ctx, `SELECT q.id, q.nummer, q.project_id::text, COALESCE(p.name,''), q.contact_id, COALESCE(c.name,''), q.status, q.accepted_at, q.linked_invoice_out_id, q.linked_sales_order_id, q.quote_date, q.valid_until, q.currency, COALESCE(q.note,''), q.net_amount, q.tax_amount, q.gross_amount
		FROM quotes q
		LEFT JOIN projects p ON p.id = q.project_id
		LEFT JOIN contacts c ON c.id = q.contact_id
		WHERE q.id=$1`, id).Scan(
		&out.ID, &out.Number, &projectID, &out.ProjectName, &out.ContactID, &out.ContactName, &out.Status, &acceptedAt, &linkedInvoiceOutID, &linkedSalesOrderID, &out.QuoteDate, &validUntil, &out.Currency, &out.Note, &out.NetAmount, &out.TaxAmount, &out.GrossAmount,
	)
	if err != nil {
		return nil, err
	}
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
	rows, err := s.pg.Query(ctx, `SELECT description, qty, unit, unit_price, COALESCE(tax_code,'') FROM quote_items WHERE quote_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item QuoteItemInput
		if err := rows.Scan(&item.Description, &item.Qty, &item.Unit, &item.UnitPrice, &item.TaxCode); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	return &out, nil
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
	query := `SELECT q.id, q.nummer, COALESCE(q.project_id::text,''), COALESCE(p.name,''), q.contact_id, COALESCE(c.name,''), q.status, q.accepted_at, q.linked_invoice_out_id, q.linked_sales_order_id, q.quote_date, q.valid_until, q.currency, q.gross_amount
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
		var validUntil sql.NullTime
		var acceptedAt sql.NullTime
		var linkedInvoiceOutID uuid.NullUUID
		var linkedSalesOrderID uuid.NullUUID
		if err := rows.Scan(&item.ID, &item.Number, &item.ProjectID, &item.ProjectName, &item.ContactID, &item.ContactName, &item.Status, &acceptedAt, &linkedInvoiceOutID, &linkedSalesOrderID, &item.QuoteDate, &validUntil, &item.Currency, &item.GrossAmount); err != nil {
			return nil, err
		}
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
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	if err := s.pg.QueryRow(ctx, `SELECT status, linked_invoice_out_id, linked_sales_order_id FROM quotes WHERE id=$1`, id).Scan(&currentStatus, &linkedInvoiceOutID, &linkedSalesOrderID); err != nil {
		return nil, err
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
	var linkedInvoiceOutID uuid.NullUUID
	var linkedSalesOrderID uuid.NullUUID
	err = tx.QueryRow(ctx, `SELECT status, contact_id, currency, linked_invoice_out_id, linked_sales_order_id FROM quotes WHERE id=$1 FOR UPDATE`, id).Scan(&status, &contactID, &currency, &linkedInvoiceOutID, &linkedSalesOrderID)
	if err != nil {
		return nil, err
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
	err = tx.QueryRow(ctx, `SELECT status, project_id::text, contact_id FROM quotes WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus, &currentProjectID, &currentContactID)
	if err != nil {
		return nil, err
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
		if strings.TrimSpace(item.Description) == "" {
			return nil, errors.New("Beschreibung erforderlich")
		}
		if item.Qty == 0 {
			item.Qty = 1
		}
		if strings.TrimSpace(item.Unit) == "" {
			item.Unit = "Stk"
		}
		lineID := uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO quote_items (id, quote_id, position, description, qty, unit, unit_price, net_amount, tax_amount, tax_code)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			lineID, id, idx+1, item.Description, item.Qty, item.Unit, item.UnitPrice, item.Qty*item.UnitPrice, item.Qty*item.UnitPrice*taxRate(item.TaxCode), nullIfEmpty(item.TaxCode))
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

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
