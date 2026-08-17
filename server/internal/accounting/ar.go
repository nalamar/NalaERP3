package accounting

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
	"nalaerp3/internal/auditlog"
	"nalaerp3/internal/settings"
)

type InvoiceItemInput struct {
	Description            string     `json:"description"`
	Qty                    float64    `json:"qty"`
	UnitPrice              float64    `json:"unit_price"`
	TaxCode                string     `json:"tax_code"`
	AccountCode            string     `json:"account_code"`
	SourceSalesOrderItemID *uuid.UUID `json:"source_sales_order_item_id,omitempty"`
}

type InvoiceOutInput struct {
	ContactID   string             `json:"contact_id"`
	InvoiceDate time.Time          `json:"invoice_date"`
	DueDate     *time.Time         `json:"due_date,omitempty"`
	Currency    string             `json:"currency"`
	Items       []InvoiceItemInput `json:"items"`
}

type InvoiceOut struct {
	ID                   uuid.UUID          `json:"id"`
	Number               *string            `json:"number,omitempty"`
	Status               string             `json:"status"`
	SourceQuoteID        *uuid.UUID         `json:"source_quote_id,omitempty"`
	SourceSalesOrderID   *uuid.UUID         `json:"source_sales_order_id,omitempty"`
	ContactID            string             `json:"contact_id"`
	ContactName          string             `json:"contact_name"`
	InvoiceDate          time.Time          `json:"invoice_date"`
	DueDate              *time.Time         `json:"due_date,omitempty"`
	Currency             string             `json:"currency"`
	NetAmount            float64            `json:"net_amount"`
	TaxAmount            float64            `json:"tax_amount"`
	GrossAmount          float64            `json:"gross_amount"`
	PaidAmount           float64            `json:"paid_amount"`
	StornoJournalEntryID *uuid.UUID         `json:"storno_journal_entry_id,omitempty"`
	StorniertAm          *time.Time         `json:"storniert_am,omitempty"`
	StornoGrund          string             `json:"storno_grund,omitempty"`
	Items                []InvoiceItemInput `json:"items"`
}

type InvoiceListItem struct {
	ID                 uuid.UUID  `json:"id"`
	Number             *string    `json:"number,omitempty"`
	Status             string     `json:"status"`
	SourceQuoteID      *uuid.UUID `json:"source_quote_id,omitempty"`
	SourceSalesOrderID *uuid.UUID `json:"source_sales_order_id,omitempty"`
	ContactID          string     `json:"contact_id"`
	ContactName        string     `json:"contact_name"`
	InvoiceDate        time.Time  `json:"invoice_date"`
	DueDate            *time.Time `json:"due_date,omitempty"`
	Currency           string     `json:"currency"`
	GrossAmount        float64    `json:"gross_amount"`
	PaidAmount         float64    `json:"paid_amount"`
	StorniertAm        *time.Time `json:"storniert_am,omitempty"`
	StornoGrund        string     `json:"storno_grund,omitempty"`
}

type InvoiceFilter struct {
	Status             string
	ContactID          string
	SourceSalesOrderID string
	Search             string
	Limit              int
	Offset             int
}

type ARService struct {
	pg      *pgxpool.Pool
	num     *settings.NumberingService
	journal *JournalService
	audit   *auditlog.Service
}

func NewARService(pg *pgxpool.Pool, num *settings.NumberingService, journal *JournalService, audit *auditlog.Service) *ARService {
	return &ARService{pg: pg, num: num, journal: journal, audit: audit}
}

func (s *ARService) Get(ctx context.Context, id uuid.UUID, companyID string) (*InvoiceOut, error) {
	var inv InvoiceOut
	var number sql.NullString
	var due sql.NullTime
	var sourceQuoteID uuid.NullUUID
	var sourceSalesOrderID uuid.NullUUID
	var stornoJournalEntryID uuid.NullUUID
	var storniertAm sql.NullTime
	err := s.pg.QueryRow(ctx, `SELECT i.id, i.nummer, i.status, i.source_quote_id, i.source_sales_order_id, i.contact_id, COALESCE(c.name,''), i.invoice_date, i.due_date, i.currency, i.net_amount, i.tax_amount, i.gross_amount, i.paid_amount, i.storno_journal_entry_id, i.storniert_am, i.storno_grund
		FROM invoices_out i
		LEFT JOIN contacts c ON c.id = i.contact_id
		WHERE i.id=$1 AND i.company_id=$2`, id, companyID).Scan(
		&inv.ID, &number, &inv.Status, &sourceQuoteID, &sourceSalesOrderID, &inv.ContactID, &inv.ContactName, &inv.InvoiceDate, &due, &inv.Currency, &inv.NetAmount, &inv.TaxAmount, &inv.GrossAmount, &inv.PaidAmount, &stornoJournalEntryID, &storniertAm, &inv.StornoGrund,
	)
	if err != nil {
		return nil, err
	}
	if number.Valid {
		inv.Number = &number.String
	}
	if sourceQuoteID.Valid {
		inv.SourceQuoteID = &sourceQuoteID.UUID
	}
	if sourceSalesOrderID.Valid {
		inv.SourceSalesOrderID = &sourceSalesOrderID.UUID
	}
	if due.Valid {
		t := due.Time
		inv.DueDate = &t
	}
	if stornoJournalEntryID.Valid {
		inv.StornoJournalEntryID = &stornoJournalEntryID.UUID
	}
	if storniertAm.Valid {
		t := storniertAm.Time
		inv.StorniertAm = &t
	}
	rows, err := s.pg.Query(ctx, `SELECT description, qty, unit_price, tax_code, account_code, source_sales_order_item_id FROM invoice_out_items WHERE invoice_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it InvoiceItemInput
		var sourceSalesOrderItemID uuid.NullUUID
		if err := rows.Scan(&it.Description, &it.Qty, &it.UnitPrice, &it.TaxCode, &it.AccountCode, &sourceSalesOrderItemID); err != nil {
			return nil, err
		}
		if sourceSalesOrderItemID.Valid {
			it.SourceSalesOrderItemID = &sourceSalesOrderItemID.UUID
		}
		inv.Items = append(inv.Items, it)
	}
	return &inv, nil
}

func (s *ARService) List(ctx context.Context, f InvoiceFilter, companyID string) ([]InvoiceListItem, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args := []any{companyID}
	conds := []string{fmt.Sprintf("i.company_id=$%d", len(args))}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("i.status=$%d", len(args)))
	}
	if f.ContactID != "" {
		args = append(args, f.ContactID)
		conds = append(conds, fmt.Sprintf("i.contact_id=$%d", len(args)))
	}
	if f.SourceSalesOrderID != "" {
		args = append(args, f.SourceSalesOrderID)
		conds = append(conds, fmt.Sprintf("i.source_sales_order_id=$%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
		conds = append(conds, fmt.Sprintf("(LOWER(i.nummer) LIKE $%d OR LOWER(i.id::text) LIKE $%d)", len(args), len(args)))
	}
	args = append(args, f.Limit)
	args = append(args, f.Offset)
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	query := `
SELECT i.id, i.nummer, i.status, i.source_quote_id, i.source_sales_order_id, i.contact_id, COALESCE(c.name,''), i.invoice_date, i.due_date, i.currency, i.gross_amount, i.paid_amount, i.storniert_am, i.storno_grund
FROM invoices_out i
LEFT JOIN contacts c ON c.id = i.contact_id
` + where + `
ORDER BY i.invoice_date DESC, i.created_at DESC
LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InvoiceListItem, 0)
	for rows.Next() {
		var it InvoiceListItem
		var num sql.NullString
		var due sql.NullTime
		var sourceQuoteID uuid.NullUUID
		var sourceSalesOrderID uuid.NullUUID
		var storniertAm sql.NullTime
		if err := rows.Scan(&it.ID, &num, &it.Status, &sourceQuoteID, &sourceSalesOrderID, &it.ContactID, &it.ContactName, &it.InvoiceDate, &due, &it.Currency, &it.GrossAmount, &it.PaidAmount, &storniertAm, &it.StornoGrund); err != nil {
			return nil, err
		}
		if num.Valid {
			it.Number = &num.String
		}
		if sourceQuoteID.Valid {
			it.SourceQuoteID = &sourceQuoteID.UUID
		}
		if sourceSalesOrderID.Valid {
			it.SourceSalesOrderID = &sourceSalesOrderID.UUID
		}
		if due.Valid {
			t := due.Time
			it.DueDate = &t
		}
		if storniertAm.Valid {
			t := storniertAm.Time
			it.StorniertAm = &t
		}
		out = append(out, it)
	}
	return out, nil
}

// Create draft invoice
func (s *ARService) Create(ctx context.Context, in InvoiceOutInput, companyID string) (*InvoiceOut, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	out, err := s.createTx(ctx, tx, in, nil, nil, companyID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ARService) CreateFromQuoteTx(ctx context.Context, tx pgx.Tx, quoteID uuid.UUID, in InvoiceOutInput, companyID string) (*InvoiceOut, error) {
	return s.createTx(ctx, tx, in, &quoteID, nil, companyID)
}

func (s *ARService) CreateFromSalesOrderTx(ctx context.Context, tx pgx.Tx, salesOrderID uuid.UUID, sourceQuoteID *uuid.UUID, in InvoiceOutInput, companyID string) (*InvoiceOut, error) {
	return s.createTx(ctx, tx, in, sourceQuoteID, &salesOrderID, companyID)
}

func (s *ARService) createTx(ctx context.Context, tx pgx.Tx, in InvoiceOutInput, sourceQuoteID, sourceSalesOrderID *uuid.UUID, companyID string) (*InvoiceOut, error) {
	if in.ContactID == "" {
		return nil, errors.New("contact_id fehlt")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("keine Positionen")
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
	if in.InvoiceDate.IsZero() {
		in.InvoiceDate = time.Now()
	}
	id := uuid.New()
	codes, err := loadTaxCodes(ctx, tx)
	if err != nil {
		return nil, err
	}
	netSum, taxSum, err := calcTotals(codes, in.Items)
	if err != nil {
		return nil, err
	}
	gross := netSum + taxSum
	_, err = tx.Exec(ctx, `INSERT INTO invoices_out (id, contact_id, status, invoice_date, due_date, currency, net_amount, tax_amount, gross_amount, source_quote_id, source_sales_order_id, company_id)
	VALUES ($1,$2,'draft',$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		id, in.ContactID, in.InvoiceDate, in.DueDate, in.Currency, netSum, taxSum, gross, sourceQuoteID, sourceSalesOrderID, companyID)
	if err != nil {
		return nil, err
	}
	for idx, it := range in.Items {
		lineID := uuid.New()
		rate, err := taxRate(codes, it.TaxCode)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO invoice_out_items (id, invoice_id, position, description, qty, unit_price, net_amount, tax_amount, tax_code, account_code, source_sales_order_item_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			lineID, id, idx+1, it.Description, it.Qty, it.UnitPrice, it.Qty*it.UnitPrice, it.UnitPrice*it.Qty*rate, it.TaxCode, it.AccountCode, it.SourceSalesOrderItemID)
		if err != nil {
			return nil, err
		}
	}
	return &InvoiceOut{
		ID:                 id,
		Status:             "draft",
		SourceQuoteID:      sourceQuoteID,
		SourceSalesOrderID: sourceSalesOrderID,
		ContactID:          in.ContactID,
		InvoiceDate:        in.InvoiceDate,
		DueDate:            in.DueDate,
		Currency:           in.Currency,
		NetAmount:          netSum,
		TaxAmount:          taxSum,
		GrossAmount:        gross,
		Items:              in.Items,
	}, nil
}

// Book moves draft to booked, assigns number and creates journal entry (AR 1400 / revenue + USt)
func (s *ARService) Book(ctx context.Context, id uuid.UUID, companyID string, actorUserID string) (*InvoiceOut, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var inv InvoiceOut
	var journalID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id, nummer, status, contact_id, invoice_date, due_date, currency, net_amount, tax_amount, gross_amount, journal_entry_id, paid_amount
		FROM invoices_out WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(
		&inv.ID, &inv.Number, &inv.Status, &inv.ContactID, &inv.InvoiceDate, &inv.DueDate, &inv.Currency, &inv.NetAmount, &inv.TaxAmount, &inv.GrossAmount, &journalID, &inv.PaidAmount,
	)
	if err != nil {
		return nil, err
	}
	if inv.Status != "draft" {
		return nil, errors.New("Rechnung ist nicht im Status draft")
	}
	// load items
	rows, err := tx.Query(ctx, `SELECT description, qty, unit_price, tax_code, account_code, source_sales_order_item_id FROM invoice_out_items WHERE invoice_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]InvoiceItemInput, 0)
	for rows.Next() {
		var it InvoiceItemInput
		var sourceSalesOrderItemID uuid.NullUUID
		if err := rows.Scan(&it.Description, &it.Qty, &it.UnitPrice, &it.TaxCode, &it.AccountCode, &sourceSalesOrderItemID); err != nil {
			return nil, err
		}
		if sourceSalesOrderItemID.Valid {
			it.SourceSalesOrderItemID = &sourceSalesOrderItemID.UUID
		}
		items = append(items, it)
	}
	inv.Items = items

	nummer, err := s.num.Next(ctx, "invoice_out", companyID)
	if err != nil {
		return nil, err
	}
	codes, err := loadTaxCodes(ctx, tx)
	if err != nil {
		return nil, err
	}
	journalInput, err := buildJournal(codes, inv, nummer, items)
	if err != nil {
		return nil, err
	}
	entry, err := s.journal.CreateTx(ctx, tx, journalInput, companyID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE invoices_out SET status='booked', nummer=$2, journal_entry_id=$3 WHERE id=$1`, id, nummer, entry.ID)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, tx, companyID, auditlog.RecordInput{
			EntityType:  "invoice_out",
			EntityID:    id.String(),
			Action:      "gebucht",
			ActorUserID: actorUserID,
			Before:      map[string]any{"status": "draft"},
			After:       map[string]any{"status": "booked", "nummer": nummer, "journal_entry_id": entry.ID},
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	inv.Status = "booked"
	inv.Number = &nummer
	return &inv, nil
}

// Storno storniert eine gebuchte Rechnung GoBD-konform: die urspruengliche
// Buchung (journal_entry_id) bleibt unveraendert bestehen, stattdessen wird
// eine neue, vollstaendige Umkehrbuchung (Soll/Haben vertauscht) erzeugt.
// Nur aus Status "booked" ohne bereits erhaltene Zahlungen moeglich -
// Rueckabwicklung bereits erhaltener Zahlungen ist bewusst nicht Teil
// dieses Storno-Konzepts (siehe docs/backlog.md).
func (s *ARService) Storno(ctx context.Context, id uuid.UUID, reason string, companyID string, actorUserID string) (*InvoiceOut, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Stornogrund erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var inv InvoiceOut
	var journalID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id, nummer, status, contact_id, invoice_date, due_date, currency, net_amount, tax_amount, gross_amount, journal_entry_id, paid_amount
		FROM invoices_out WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(
		&inv.ID, &inv.Number, &inv.Status, &inv.ContactID, &inv.InvoiceDate, &inv.DueDate, &inv.Currency, &inv.NetAmount, &inv.TaxAmount, &inv.GrossAmount, &journalID, &inv.PaidAmount,
	)
	if err != nil {
		return nil, err
	}
	if inv.Status != "booked" {
		return nil, fmt.Errorf("Rechnung ist nicht im Status booked (aktuell: %s)", inv.Status)
	}
	if inv.PaidAmount != 0 {
		return nil, errors.New("Rechnung hat bereits Zahlungen erhalten, Storno derzeit nicht unterstützt")
	}
	// load items (fuer die Umkehrbuchung werden dieselben Konten/Betraege
	// wie bei der urspruenglichen Buchung benoetigt)
	rows, err := tx.Query(ctx, `SELECT description, qty, unit_price, tax_code, account_code, source_sales_order_item_id FROM invoice_out_items WHERE invoice_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]InvoiceItemInput, 0)
	for rows.Next() {
		var it InvoiceItemInput
		var sourceSalesOrderItemID uuid.NullUUID
		if err := rows.Scan(&it.Description, &it.Qty, &it.UnitPrice, &it.TaxCode, &it.AccountCode, &sourceSalesOrderItemID); err != nil {
			return nil, err
		}
		if sourceSalesOrderItemID.Valid {
			it.SourceSalesOrderItemID = &sourceSalesOrderItemID.UUID
		}
		items = append(items, it)
	}
	inv.Items = items

	nummer := ""
	if inv.Number != nil {
		nummer = *inv.Number
	}
	codes, err := loadTaxCodes(ctx, tx)
	if err != nil {
		return nil, err
	}
	stornoJournalInput, err := buildStornoJournal(codes, inv, nummer, items)
	if err != nil {
		return nil, err
	}
	entry, err := s.journal.CreateTx(ctx, tx, stornoJournalInput, companyID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, err = tx.Exec(ctx, `UPDATE invoices_out SET status='storniert', storno_journal_entry_id=$2, storniert_am=$3, storno_grund=$4 WHERE id=$1`, id, entry.ID, now, reason)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, tx, companyID, auditlog.RecordInput{
			EntityType:  "invoice_out",
			EntityID:    id.String(),
			Action:      "storniert",
			ActorUserID: actorUserID,
			Before:      map[string]any{"status": "booked"},
			After:       map[string]any{"status": "storniert", "storno_journal_entry_id": entry.ID},
			Note:        reason,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	inv.Status = "storniert"
	inv.StornoJournalEntryID = &entry.ID
	inv.StorniertAm = &now
	inv.StornoGrund = reason
	return &inv, nil
}

// taxCodeInfo buendelt die aus den Stammdaten (tax_codes/accounts) gelesenen
// Angaben zu einem Steuerkennzeichen (Subtask/Backlog 0.7).
type taxCodeInfo struct {
	Rate             float64
	LiabilityAccount string
}

// loadTaxCodes liest alle aktiven Steuerkennzeichen samt zugehoerigem
// USt-Verbindlichkeitskonto (accounts.type='liability') aus den Stammdaten.
// Einzige Quelle der Wahrheit statt der zuvor hartcodierten DE19/DE7-
// Sonderfaelle in taxRate/taxAccountFor (Backlog 0.7: unbekannte/inaktive
// Codes wurden bisher STILLSCHWEIGEND als 0% behandelt bzw. auf das
// DE19-Konto 1776 zurueckgefallen, statt einen Fehler zu liefern).
func loadTaxCodes(ctx context.Context, tx pgx.Tx) (map[string]taxCodeInfo, error) {
	rows, err := tx.Query(ctx, `
        SELECT tc.code, tc.rate, COALESCE(a.code, '')
          FROM tax_codes tc
          LEFT JOIN accounts a ON a.tax_code = tc.code AND a.type = 'liability' AND a.is_active
         WHERE tc.is_active
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]taxCodeInfo)
	for rows.Next() {
		var code, account string
		var rate float64
		if err := rows.Scan(&code, &rate, &account); err != nil {
			return nil, err
		}
		out[code] = taxCodeInfo{Rate: rate, LiabilityAccount: account}
	}
	return out, rows.Err()
}

// taxRate liefert den Steuersatz fuer ein Steuerkennzeichen aus den
// Stammdaten. Ein leerer Code gilt als "kein Steuerkennzeichen" (0%, z.B.
// Skonto-/Durchlaufposten) und ist KEIN Fehler; ein nicht-leerer, aber
// unbekannter oder inaktiver Code liefert einen Fehler statt still auf 0%
// zurueckzufallen.
func taxRate(codes map[string]taxCodeInfo, code string) (float64, error) {
	if code == "" {
		return 0, nil
	}
	info, ok := codes[code]
	if !ok {
		return 0, fmt.Errorf("unbekanntes oder inaktives Steuerkennzeichen: %s", code)
	}
	return info.Rate, nil
}

// taxAccountFor liefert das USt-Verbindlichkeitskonto fuer ein
// Steuerkennzeichen aus den Stammdaten. Liefert einen Fehler statt wie
// zuvor still auf das DE19-Konto 1776 zurueckzufallen, wenn der Code
// unbekannt/inaktiv ist oder kein Konto dafuer konfiguriert wurde.
func taxAccountFor(codes map[string]taxCodeInfo, code string) (string, error) {
	info, ok := codes[code]
	if !ok {
		return "", fmt.Errorf("unbekanntes oder inaktives Steuerkennzeichen: %s", code)
	}
	if info.LiabilityAccount == "" {
		return "", fmt.Errorf("kein Umsatzsteuer-Konto fuer Steuerkennzeichen %s konfiguriert", code)
	}
	return info.LiabilityAccount, nil
}

func buildJournal(codes map[string]taxCodeInfo, inv InvoiceOut, nummer string, items []InvoiceItemInput) (JournalEntryInput, error) {
	desc := "AR " + nummer
	debit := JournalLineInput{AccountCode: "1400", Debit: inv.GrossAmount, Credit: 0, Memo: "Forderung"}
	lines := []JournalLineInput{debit}
	for _, it := range items {
		net := it.Qty * it.UnitPrice
		rate, err := taxRate(codes, it.TaxCode)
		if err != nil {
			return JournalEntryInput{}, err
		}
		tax := net * rate
		lines = append(lines, JournalLineInput{
			AccountCode: it.AccountCode,
			Debit:       0,
			Credit:      net,
			Memo:        it.Description,
		})
		if tax > 0.0001 && it.TaxCode != "" {
			account, err := taxAccountFor(codes, it.TaxCode)
			if err != nil {
				return JournalEntryInput{}, err
			}
			lines = append(lines, JournalLineInput{
				AccountCode: account,
				Debit:       0,
				Credit:      tax,
				Memo:        "USt",
			})
		}
	}
	return JournalEntryInput{
		Date:        inv.InvoiceDate,
		Description: desc,
		Currency:    inv.Currency,
		Source:      "invoice_out",
		SourceID:    inv.ID.String(),
		Lines:       lines,
	}, nil
}

// buildStornoJournal erzeugt die Umkehrbuchung zu buildJournal: identische
// Konten/Betraege, aber Soll und Haben vertauscht - die urspruengliche
// Buchung wird dadurch NICHT veraendert, sondern durch eine zweite,
// gegenlaeufige Buchung neutralisiert (GoBD-Grundprinzip).
func buildStornoJournal(codes map[string]taxCodeInfo, inv InvoiceOut, nummer string, items []InvoiceItemInput) (JournalEntryInput, error) {
	original, err := buildJournal(codes, inv, nummer, items)
	if err != nil {
		return JournalEntryInput{}, err
	}
	lines := make([]JournalLineInput, len(original.Lines))
	for i, l := range original.Lines {
		lines[i] = JournalLineInput{
			AccountCode: l.AccountCode,
			Debit:       l.Credit,
			Credit:      l.Debit,
			Memo:        "Storno: " + l.Memo,
		}
	}
	return JournalEntryInput{
		Date:        time.Now(),
		Description: "Storno AR " + nummer,
		Currency:    inv.Currency,
		Source:      "invoice_out_storno",
		SourceID:    inv.ID.String(),
		Lines:       lines,
	}, nil
}

func calcTotals(codes map[string]taxCodeInfo, items []InvoiceItemInput) (net, tax float64, err error) {
	for _, it := range items {
		n := it.Qty * it.UnitPrice
		net += n
		rate, err := taxRate(codes, it.TaxCode)
		if err != nil {
			return 0, 0, err
		}
		tax += n * rate
	}
	return net, tax, nil
}
