package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
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
	ID                      string                          `json:"id,omitempty"`
	Description             string                          `json:"description"`
	Qty                     float64                         `json:"qty"`
	Unit                    string                          `json:"unit"`
	UnitPrice               float64                         `json:"unit_price"`
	TaxCode                 string                          `json:"tax_code"`
	MaterialID              string                          `json:"material_id,omitempty"`
	PriceMappingStatus      string                          `json:"price_mapping_status,omitempty"`
	MaterialCandidateStatus string                          `json:"material_candidate_status,omitempty"`
	MaterialCandidates      []MaterialCandidate             `json:"material_candidates,omitempty"`
	ActiveApprovalRequest   *QuoteItemApprovalRequest       `json:"active_approval_request,omitempty"`
	LatestApprovalDecision  *QuoteItemApprovalDecisionBadge `json:"latest_approval_decision,omitempty"`
}

type MaterialCandidate struct {
	MaterialID    string `json:"material_id"`
	MaterialNo    string `json:"material_no,omitempty"`
	MaterialLabel string `json:"material_label,omitempty"`
}

type PriceSuggestion struct {
	MaterialID         string  `json:"material_id"`
	SuggestedUnitPrice float64 `json:"suggested_unit_price"`
	Currency           string  `json:"currency,omitempty"`
	SourceLabel        string  `json:"source_label,omitempty"`
}

type PriceHistoryEntry struct {
	SourceLabel string     `json:"source_label"`
	UnitPrice   float64    `json:"unit_price"`
	Currency    string     `json:"currency,omitempty"`
	Reference   string     `json:"reference,omitempty"`
	Date        *time.Time `json:"date,omitempty"`
}

type PriceSourcePriorityEntry struct {
	SourceLabel    string     `json:"source_label"`
	UnitPrice      float64    `json:"unit_price"`
	Currency       string     `json:"currency,omitempty"`
	Reference      string     `json:"reference,omitempty"`
	Date           *time.Time `json:"date,omitempty"`
	PriorityRank   int        `json:"priority_rank"`
	PriorityReason string     `json:"priority_reason,omitempty"`
}

type PriceEvaluation struct {
	CurrentUnitPrice       float64    `json:"current_unit_price"`
	Currency               string     `json:"currency,omitempty"`
	PrimarySourceLabel     string     `json:"primary_source_label"`
	PrimarySourceUnitPrice float64    `json:"primary_source_unit_price"`
	PrimarySourceReference string     `json:"primary_source_reference,omitempty"`
	PrimarySourceDate      *time.Time `json:"primary_source_date,omitempty"`
	AbsoluteDelta          float64    `json:"absolute_delta"`
	RelativeDeltaPercent   *float64   `json:"relative_delta_percent"`
	EvaluationStatus       string     `json:"evaluation_status"`
	EvaluationReason       string     `json:"evaluation_reason,omitempty"`
}

type PriceDecisionTransparency struct {
	CurrentUnitPrice       float64    `json:"current_unit_price"`
	Currency               string     `json:"currency,omitempty"`
	PrimarySourceLabel     string     `json:"primary_source_label"`
	PrimarySourceUnitPrice float64    `json:"primary_source_unit_price"`
	PrimarySourceReference string     `json:"primary_source_reference,omitempty"`
	PrimarySourceDate      *time.Time `json:"primary_source_date,omitempty"`
	AbsoluteDelta          float64    `json:"absolute_delta"`
	RelativeDeltaPercent   *float64   `json:"relative_delta_percent"`
	DecisionStatus         string     `json:"decision_status"`
	DecisionReason         string     `json:"decision_reason,omitempty"`
}

type PriceDecisionHistoryEntry struct {
	ID               uuid.UUID  `json:"id"`
	DecisionType     string     `json:"decision_type"`
	MaterialID       string     `json:"material_id,omitempty"`
	SourceLabel      string     `json:"source_label"`
	SourceUnitPrice  float64    `json:"source_unit_price"`
	AppliedUnitPrice float64    `json:"applied_unit_price"`
	Currency         string     `json:"currency,omitempty"`
	SourceReference  string     `json:"source_reference,omitempty"`
	SourceDate       *time.Time `json:"source_date,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type QuoteItemMarginAnchor struct {
	CurrentUnitPrice   float64    `json:"current_unit_price"`
	CostBasisUnitPrice float64    `json:"cost_basis_unit_price"`
	Currency           string     `json:"currency,omitempty"`
	AbsoluteMargin     float64    `json:"absolute_margin"`
	MarginPercent      *float64   `json:"margin_percent"`
	MarginStatus       string     `json:"margin_status"`
	DecisionID         uuid.UUID  `json:"decision_id"`
	DecisionType       string     `json:"decision_type"`
	SourceLabel        string     `json:"source_label"`
	SourceReference    string     `json:"source_reference,omitempty"`
	SourceDate         *time.Time `json:"source_date,omitempty"`
	DecisionCreatedAt  time.Time  `json:"decision_created_at"`
}

type QuoteItemApprovalHint struct {
	ApprovalStatus     string     `json:"approval_status"`
	ApprovalReason     string     `json:"approval_reason"`
	MarginStatus       string     `json:"margin_status,omitempty"`
	CurrentUnitPrice   *float64   `json:"current_unit_price,omitempty"`
	CostBasisUnitPrice *float64   `json:"cost_basis_unit_price,omitempty"`
	Currency           string     `json:"currency,omitempty"`
	AbsoluteMargin     *float64   `json:"absolute_margin,omitempty"`
	MarginPercent      *float64   `json:"margin_percent,omitempty"`
	DecisionID         *uuid.UUID `json:"decision_id,omitempty"`
	DecisionType       string     `json:"decision_type,omitempty"`
	SourceLabel        string     `json:"source_label,omitempty"`
	DecisionCreatedAt  *time.Time `json:"decision_created_at,omitempty"`
}

type QuoteItemTargetMarginAnchor struct {
	TargetStatus            string     `json:"target_status"`
	TargetReason            string     `json:"target_reason"`
	TargetMarginPercent     float64    `json:"target_margin_percent"`
	CurrentUnitPrice        *float64   `json:"current_unit_price,omitempty"`
	CostBasisUnitPrice      *float64   `json:"cost_basis_unit_price,omitempty"`
	TargetUnitPrice         *float64   `json:"target_unit_price,omitempty"`
	Currency                string     `json:"currency,omitempty"`
	AbsoluteMargin          *float64   `json:"absolute_margin,omitempty"`
	MarginPercent           *float64   `json:"margin_percent,omitempty"`
	TargetDifference        *float64   `json:"target_difference,omitempty"`
	TargetDifferencePercent *float64   `json:"target_difference_percent,omitempty"`
	MarginStatus            string     `json:"margin_status,omitempty"`
	DecisionID              *uuid.UUID `json:"decision_id,omitempty"`
	DecisionType            string     `json:"decision_type,omitempty"`
	SourceLabel             string     `json:"source_label,omitempty"`
	DecisionCreatedAt       *time.Time `json:"decision_created_at,omitempty"`
}

type QuoteItemApprovalRequest struct {
	ID                          uuid.UUID  `json:"id"`
	QuoteID                     uuid.UUID  `json:"quote_id"`
	QuoteItemID                 uuid.UUID  `json:"quote_item_id"`
	Status                      string     `json:"status"`
	ReasonCode                  string     `json:"reason_code"`
	ReasonText                  string     `json:"reason_text,omitempty"`
	CurrentUnitPriceSnapshot    float64    `json:"current_unit_price_snapshot"`
	CostBasisUnitPriceSnapshot  float64    `json:"cost_basis_unit_price_snapshot"`
	TargetUnitPriceSnapshot     float64    `json:"target_unit_price_snapshot"`
	TargetMarginPercentSnapshot float64    `json:"target_margin_percent_snapshot"`
	TargetDifferenceSnapshot    float64    `json:"target_difference_snapshot"`
	MarginPercentSnapshot       *float64   `json:"margin_percent_snapshot,omitempty"`
	PriceDecisionID             *uuid.UUID `json:"price_decision_id,omitempty"`
	RequestedBy                 string     `json:"requested_by,omitempty"`
	RequestedByName             string     `json:"requested_by_name,omitempty"`
	RequestedAt                 time.Time  `json:"requested_at"`
	CancelledBy                 string     `json:"cancelled_by,omitempty"`
	CancelledByName             string     `json:"cancelled_by_name,omitempty"`
	CancelledAt                 *time.Time `json:"cancelled_at,omitempty"`
	DecidedBy                   string     `json:"decided_by,omitempty"`
	DecidedByName               string     `json:"decided_by_name,omitempty"`
	DecidedAt                   *time.Time `json:"decided_at,omitempty"`
	DecisionComment             string     `json:"decision_comment,omitempty"`
	ApprovedUnitPriceSnapshot   *float64   `json:"approved_unit_price_snapshot,omitempty"`
	ApprovedTargetMarginPercent *float64   `json:"approved_target_margin_percent_snapshot,omitempty"`
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}

type QuoteItemApprovalDecisionBadge struct {
	ID                          uuid.UUID `json:"id"`
	Status                      string    `json:"status"`
	ReasonCode                  string    `json:"reason_code,omitempty"`
	ReasonText                  string    `json:"reason_text,omitempty"`
	DecidedBy                   string    `json:"decided_by,omitempty"`
	DecidedByName               string    `json:"decided_by_name,omitempty"`
	DecidedAt                   time.Time `json:"decided_at"`
	DecisionComment             string    `json:"decision_comment,omitempty"`
	ApprovedUnitPriceSnapshot   *float64  `json:"approved_unit_price_snapshot,omitempty"`
	ApprovedTargetMarginPercent *float64  `json:"approved_target_margin_percent_snapshot,omitempty"`
}

type QuoteApprovalReworkQueueFilter struct {
	ProjectID uuid.UUID
	ContactID string
	QuoteID   uuid.UUID
}

type QuoteApprovalRequestQueueFilter struct {
	ProjectID uuid.UUID
	ContactID string
	QuoteID   uuid.UUID
}

type QuoteApprovalReworkQueueItem struct {
	QuoteID                     uuid.UUID  `json:"quote_id"`
	QuoteNumber                 string     `json:"quote_number"`
	QuoteStatus                 string     `json:"quote_status"`
	QuoteDate                   time.Time  `json:"quote_date"`
	ProjectID                   string     `json:"project_id,omitempty"`
	ProjectName                 string     `json:"project_name,omitempty"`
	ContactID                   string     `json:"contact_id,omitempty"`
	ContactName                 string     `json:"contact_name,omitempty"`
	QuoteItemID                 uuid.UUID  `json:"quote_item_id"`
	Position                    int        `json:"position"`
	Description                 string     `json:"description"`
	CurrentUnitPrice            float64    `json:"current_unit_price"`
	Currency                    string     `json:"currency,omitempty"`
	ApprovalRequestID           uuid.UUID  `json:"approval_request_id"`
	ReasonCode                  string     `json:"reason_code"`
	ReasonText                  string     `json:"reason_text,omitempty"`
	DecisionComment             string     `json:"decision_comment,omitempty"`
	DecidedBy                   string     `json:"decided_by,omitempty"`
	DecidedByName               string     `json:"decided_by_name,omitempty"`
	DecidedAt                   time.Time  `json:"decided_at"`
	CurrentUnitPriceSnapshot    float64    `json:"current_unit_price_snapshot"`
	CostBasisUnitPriceSnapshot  float64    `json:"cost_basis_unit_price_snapshot"`
	TargetUnitPriceSnapshot     float64    `json:"target_unit_price_snapshot"`
	TargetMarginPercentSnapshot float64    `json:"target_margin_percent_snapshot"`
	TargetDifferenceSnapshot    float64    `json:"target_difference_snapshot"`
	MarginPercentSnapshot       *float64   `json:"margin_percent_snapshot,omitempty"`
	PriceDecisionID             *uuid.UUID `json:"price_decision_id,omitempty"`
	CurrentTargetStatus         string     `json:"current_target_status,omitempty"`
	CurrentTargetDifference     *float64   `json:"current_target_difference,omitempty"`
	CurrentTargetUnitPrice      *float64   `json:"current_target_unit_price,omitempty"`
	CurrentMarginPercent        *float64   `json:"current_margin_percent,omitempty"`
	CurrentPriceDecisionID      *uuid.UUID `json:"current_price_decision_id,omitempty"`
}

type QuoteApprovalRequestQueueItem struct {
	QuoteID                     uuid.UUID  `json:"quote_id"`
	QuoteNumber                 string     `json:"quote_number"`
	QuoteStatus                 string     `json:"quote_status"`
	QuoteDate                   time.Time  `json:"quote_date"`
	ProjectID                   string     `json:"project_id,omitempty"`
	ProjectName                 string     `json:"project_name,omitempty"`
	ContactID                   string     `json:"contact_id,omitempty"`
	ContactName                 string     `json:"contact_name,omitempty"`
	QuoteItemID                 uuid.UUID  `json:"quote_item_id"`
	Position                    int        `json:"position"`
	Description                 string     `json:"description"`
	CurrentUnitPrice            float64    `json:"current_unit_price"`
	Currency                    string     `json:"currency,omitempty"`
	ApprovalRequestID           uuid.UUID  `json:"approval_request_id"`
	ReasonCode                  string     `json:"reason_code"`
	ReasonText                  string     `json:"reason_text,omitempty"`
	RequestedBy                 string     `json:"requested_by,omitempty"`
	RequestedByName             string     `json:"requested_by_name,omitempty"`
	RequestedAt                 time.Time  `json:"requested_at"`
	CurrentUnitPriceSnapshot    float64    `json:"current_unit_price_snapshot"`
	CostBasisUnitPriceSnapshot  float64    `json:"cost_basis_unit_price_snapshot"`
	TargetUnitPriceSnapshot     float64    `json:"target_unit_price_snapshot"`
	TargetMarginPercentSnapshot float64    `json:"target_margin_percent_snapshot"`
	TargetDifferenceSnapshot    float64    `json:"target_difference_snapshot"`
	MarginPercentSnapshot       *float64   `json:"margin_percent_snapshot,omitempty"`
	PriceDecisionID             *uuid.UUID `json:"price_decision_id,omitempty"`
	CurrentTargetStatus         string     `json:"current_target_status,omitempty"`
	CurrentTargetDifference     *float64   `json:"current_target_difference,omitempty"`
	CurrentTargetUnitPrice      *float64   `json:"current_target_unit_price,omitempty"`
	CurrentMarginPercent        *float64   `json:"current_margin_percent,omitempty"`
	CurrentPriceDecisionID      *uuid.UUID `json:"current_price_decision_id,omitempty"`
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

type quoteApprovalReworkQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
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
			END AS material_candidate_status,
			active_approval.id,
			active_approval.quote_id,
			active_approval.quote_item_id,
			active_approval.status,
			active_approval.reason_code,
			active_approval.reason_text,
			active_approval.current_unit_price_snapshot,
			active_approval.cost_basis_unit_price_snapshot,
			active_approval.target_unit_price_snapshot,
			active_approval.target_margin_percent_snapshot,
			active_approval.target_difference_snapshot,
			active_approval.margin_percent_snapshot,
			active_approval.price_decision_id,
			active_approval.requested_by,
			active_approval.requested_at,
			active_approval.cancelled_by,
			active_approval.cancelled_at,
			active_approval.decided_by,
			active_approval.decided_at,
			active_approval.decision_comment,
			active_approval.approved_unit_price_snapshot,
			active_approval.approved_target_margin_percent_snapshot,
			active_approval.created_at,
			active_approval.updated_at,
			latest_approval_decision.id,
			latest_approval_decision.status,
			latest_approval_decision.reason_code,
			latest_approval_decision.reason_text,
			latest_approval_decision.decided_by,
			latest_approval_decision.decided_by_name,
			latest_approval_decision.decided_at,
			latest_approval_decision.decision_comment,
			latest_approval_decision.approved_unit_price_snapshot,
			latest_approval_decision.approved_target_margin_percent_snapshot
		FROM quote_items qi
		LEFT JOIN LATERAL (
			SELECT
				qar.id,
				qar.quote_id,
				qar.quote_item_id,
				qar.status,
				qar.reason_code,
				qar.reason_text,
				qar.current_unit_price_snapshot,
				qar.cost_basis_unit_price_snapshot,
				qar.target_unit_price_snapshot,
				qar.target_margin_percent_snapshot,
				qar.target_difference_snapshot,
				qar.margin_percent_snapshot,
				qar.price_decision_id,
				qar.requested_by,
				qar.requested_at,
				qar.cancelled_by,
				qar.cancelled_at,
				qar.decided_by,
				qar.decided_at,
				qar.decision_comment,
				qar.approved_unit_price_snapshot,
				qar.approved_target_margin_percent_snapshot,
				qar.created_at,
				qar.updated_at
			FROM quote_item_approval_requests qar
			WHERE qar.quote_item_id = qi.id
			  AND qar.status = 'requested'
			ORDER BY qar.requested_at DESC
			LIMIT 1
		) active_approval ON true
		LEFT JOIN LATERAL (
			SELECT
				qar.id,
				qar.status,
				qar.reason_code,
				qar.reason_text,
				qar.decided_by,
				CASE
					WHEN COALESCE(qar.decided_by, '') = '' THEN ''
					ELSE COALESCE(NULLIF(decided_user.display_name, ''), NULLIF(decided_user.email, ''), qar.decided_by)
				END AS decided_by_name,
				qar.decided_at,
				qar.decision_comment,
				qar.approved_unit_price_snapshot,
				qar.approved_target_margin_percent_snapshot
			FROM quote_item_approval_requests qar
			LEFT JOIN users decided_user ON decided_user.id = qar.decided_by
			WHERE qar.quote_item_id = qi.id
			  AND qar.status IN ('approved', 'rejected', 'rework_resolved')
			ORDER BY qar.decided_at DESC NULLS LAST, qar.updated_at DESC
			LIMIT 1
		) latest_approval_decision ON true
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
		var approvalID uuid.NullUUID
		var approvalQuoteID uuid.NullUUID
		var approvalQuoteItemID uuid.NullUUID
		var approvalStatus sql.NullString
		var approvalReasonCode sql.NullString
		var approvalReasonText sql.NullString
		var approvalCurrentUnitPrice sql.NullFloat64
		var approvalCostBasisUnitPrice sql.NullFloat64
		var approvalTargetUnitPrice sql.NullFloat64
		var approvalTargetMarginPercent sql.NullFloat64
		var approvalTargetDifference sql.NullFloat64
		var approvalMarginPercent sql.NullFloat64
		var approvalPriceDecisionID uuid.NullUUID
		var approvalRequestedBy sql.NullString
		var approvalRequestedAt sql.NullTime
		var approvalCancelledBy sql.NullString
		var approvalCancelledAt sql.NullTime
		var approvalDecidedBy sql.NullString
		var approvalDecidedAt sql.NullTime
		var approvalDecisionComment sql.NullString
		var approvalApprovedUnitPrice sql.NullFloat64
		var approvalApprovedTargetMarginPercent sql.NullFloat64
		var approvalCreatedAt sql.NullTime
		var approvalUpdatedAt sql.NullTime
		var latestApprovalID uuid.NullUUID
		var latestApprovalStatus sql.NullString
		var latestApprovalReasonCode sql.NullString
		var latestApprovalReasonText sql.NullString
		var latestApprovalDecidedBy sql.NullString
		var latestApprovalDecidedByName sql.NullString
		var latestApprovalDecidedAt sql.NullTime
		var latestApprovalDecisionComment sql.NullString
		var latestApprovalApprovedUnitPrice sql.NullFloat64
		var latestApprovalApprovedTargetMarginPercent sql.NullFloat64
		if err := rows.Scan(
			&quoteItemID,
			&item.Description,
			&item.Qty,
			&item.Unit,
			&item.UnitPrice,
			&item.TaxCode,
			&item.MaterialID,
			&item.PriceMappingStatus,
			&item.MaterialCandidateStatus,
			&approvalID,
			&approvalQuoteID,
			&approvalQuoteItemID,
			&approvalStatus,
			&approvalReasonCode,
			&approvalReasonText,
			&approvalCurrentUnitPrice,
			&approvalCostBasisUnitPrice,
			&approvalTargetUnitPrice,
			&approvalTargetMarginPercent,
			&approvalTargetDifference,
			&approvalMarginPercent,
			&approvalPriceDecisionID,
			&approvalRequestedBy,
			&approvalRequestedAt,
			&approvalCancelledBy,
			&approvalCancelledAt,
			&approvalDecidedBy,
			&approvalDecidedAt,
			&approvalDecisionComment,
			&approvalApprovedUnitPrice,
			&approvalApprovedTargetMarginPercent,
			&approvalCreatedAt,
			&approvalUpdatedAt,
			&latestApprovalID,
			&latestApprovalStatus,
			&latestApprovalReasonCode,
			&latestApprovalReasonText,
			&latestApprovalDecidedBy,
			&latestApprovalDecidedByName,
			&latestApprovalDecidedAt,
			&latestApprovalDecisionComment,
			&latestApprovalApprovedUnitPrice,
			&latestApprovalApprovedTargetMarginPercent,
		); err != nil {
			return nil, err
		}
		item.ID = quoteItemID.String()
		if approvalID.Valid && approvalQuoteID.Valid && approvalQuoteItemID.Valid && approvalRequestedAt.Valid && approvalCreatedAt.Valid && approvalUpdatedAt.Valid {
			item.ActiveApprovalRequest = &QuoteItemApprovalRequest{
				ID:                          approvalID.UUID,
				QuoteID:                     approvalQuoteID.UUID,
				QuoteItemID:                 approvalQuoteItemID.UUID,
				Status:                      approvalStatus.String,
				ReasonCode:                  approvalReasonCode.String,
				ReasonText:                  approvalReasonText.String,
				CurrentUnitPriceSnapshot:    approvalCurrentUnitPrice.Float64,
				CostBasisUnitPriceSnapshot:  approvalCostBasisUnitPrice.Float64,
				TargetUnitPriceSnapshot:     approvalTargetUnitPrice.Float64,
				TargetMarginPercentSnapshot: approvalTargetMarginPercent.Float64,
				TargetDifferenceSnapshot:    approvalTargetDifference.Float64,
				RequestedAt:                 approvalRequestedAt.Time,
				CreatedAt:                   approvalCreatedAt.Time,
				UpdatedAt:                   approvalUpdatedAt.Time,
			}
			if approvalMarginPercent.Valid {
				item.ActiveApprovalRequest.MarginPercentSnapshot = &approvalMarginPercent.Float64
			}
			if approvalPriceDecisionID.Valid {
				item.ActiveApprovalRequest.PriceDecisionID = &approvalPriceDecisionID.UUID
			}
			if approvalRequestedBy.Valid {
				item.ActiveApprovalRequest.RequestedBy = approvalRequestedBy.String
			}
			if approvalCancelledBy.Valid {
				item.ActiveApprovalRequest.CancelledBy = approvalCancelledBy.String
			}
			if approvalCancelledAt.Valid {
				item.ActiveApprovalRequest.CancelledAt = &approvalCancelledAt.Time
			}
			if approvalDecidedBy.Valid {
				item.ActiveApprovalRequest.DecidedBy = approvalDecidedBy.String
			}
			if approvalDecidedAt.Valid {
				item.ActiveApprovalRequest.DecidedAt = &approvalDecidedAt.Time
			}
			if approvalDecisionComment.Valid {
				item.ActiveApprovalRequest.DecisionComment = approvalDecisionComment.String
			}
			if approvalApprovedUnitPrice.Valid {
				item.ActiveApprovalRequest.ApprovedUnitPriceSnapshot = &approvalApprovedUnitPrice.Float64
			}
			if approvalApprovedTargetMarginPercent.Valid {
				item.ActiveApprovalRequest.ApprovedTargetMarginPercent = &approvalApprovedTargetMarginPercent.Float64
			}
		}
		if latestApprovalID.Valid && latestApprovalDecidedAt.Valid {
			item.LatestApprovalDecision = &QuoteItemApprovalDecisionBadge{
				ID:         latestApprovalID.UUID,
				Status:     latestApprovalStatus.String,
				ReasonCode: latestApprovalReasonCode.String,
				ReasonText: latestApprovalReasonText.String,
				DecidedAt:  latestApprovalDecidedAt.Time,
			}
			if latestApprovalDecidedBy.Valid {
				item.LatestApprovalDecision.DecidedBy = latestApprovalDecidedBy.String
			}
			if latestApprovalDecidedByName.Valid {
				item.LatestApprovalDecision.DecidedByName = latestApprovalDecidedByName.String
			}
			if latestApprovalDecisionComment.Valid {
				item.LatestApprovalDecision.DecisionComment = latestApprovalDecisionComment.String
			}
			if latestApprovalApprovedUnitPrice.Valid {
				item.LatestApprovalDecision.ApprovedUnitPriceSnapshot = &latestApprovalApprovedUnitPrice.Float64
			}
			if latestApprovalApprovedTargetMarginPercent.Valid {
				item.LatestApprovalDecision.ApprovedTargetMarginPercent = &latestApprovalApprovedTargetMarginPercent.Float64
			}
		}
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

func (s *Service) SearchMaterialsForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, query string) ([]MaterialCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("Suchbegriff fehlt")
	}

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var currentMaterialID string
	err := s.pg.QueryRow(ctx, `
		SELECT
			q.status,
			q.superseded_by_quote_id,
			COALESCE(qi.material_id,'')
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		WHERE q.id = $1
		  AND qi.id = $2
	`, quoteID, itemID).Scan(&currentStatus, &supersededByQuoteID, &currentMaterialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}
	if strings.TrimSpace(currentMaterialID) != "" {
		return nil, errors.New("Angebotsposition hat bereits ein Material")
	}

	searchPattern := "%" + query + "%"
	rows, err := s.pg.Query(ctx, `
		SELECT m.id, m.nummer, m.bezeichnung
		FROM materials m
		WHERE m.aktiv = TRUE
		  AND (
			m.nummer ILIKE $1
			OR m.bezeichnung ILIKE $1
		  )
		ORDER BY m.bezeichnung ASC, m.nummer ASC
		LIMIT 5
	`, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]MaterialCandidate, 0, 5)
	for rows.Next() {
		var candidate MaterialCandidate
		if err := rows.Scan(&candidate.MaterialID, &candidate.MaterialNo, &candidate.MaterialLabel); err != nil {
			return nil, err
		}
		results = append(results, candidate)
	}
	return results, rows.Err()
}

func (s *Service) ApplySearchedMaterial(ctx context.Context, quoteID, itemID uuid.UUID, query, materialID string) (*Quote, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("Suchbegriff fehlt")
	}
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

	searchPattern := "%" + query + "%"
	var matchedMaterialID string
	err = tx.QueryRow(ctx, `
		SELECT m.id
		FROM materials m
		WHERE m.aktiv = TRUE
		  AND m.id = $1
		  AND (
			m.nummer ILIKE $2
			OR m.bezeichnung ILIKE $2
		  )
		LIMIT 1
	`, materialID, searchPattern).Scan(&matchedMaterialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("material_id ist kein sichtbarer Suchtreffer")
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

func (s *Service) SuggestPriceForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*PriceSuggestion, error) {
	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var currentMaterialID string
	err := s.pg.QueryRow(ctx, `
		SELECT
			q.status,
			q.superseded_by_quote_id,
			COALESCE(qi.material_id,'')
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		WHERE q.id = $1
		  AND qi.id = $2
	`, quoteID, itemID).Scan(&currentStatus, &supersededByQuoteID, &currentMaterialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}
	if strings.TrimSpace(currentMaterialID) == "" {
		return nil, errors.New("Angebotsposition hat kein Material")
	}

	var suggestion PriceSuggestion
	if err := s.pg.QueryRow(ctx, `
		SELECT
			m.id,
			COALESCE(m.avg_purchase_price, 0),
			COALESCE(NULLIF(BTRIM(m.currency), ''), 'EUR')
		FROM materials m
		WHERE m.id = $1
	`, currentMaterialID).Scan(&suggestion.MaterialID, &suggestion.SuggestedUnitPrice, &suggestion.Currency); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Material nicht gefunden")
		}
		return nil, err
	}
	suggestion.SourceLabel = "Durchschnittlicher Einkaufspreis"
	return &suggestion, nil
}

func (s *Service) ApplyPriceSuggestionForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*Quote, error) {
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
	if strings.TrimSpace(currentMaterialID) == "" {
		return nil, errors.New("Angebotsposition hat kein Material")
	}

	var suggestedUnitPrice float64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(m.avg_purchase_price, 0)
		FROM materials m
		WHERE m.id = $1
	`, currentMaterialID).Scan(&suggestedUnitPrice); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Material nicht gefunden")
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `UPDATE quote_items SET unit_price=$2, price_mapping_status='manual' WHERE id=$1`, itemID, suggestedUnitPrice); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, quoteID)
}

func (s *Service) ApplyPrimaryPriceSourceForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*Quote, error) {
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
	var qty float64
	var taxCode string
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(material_id, ''),
			qty,
			COALESCE(tax_code, '')
		FROM quote_items
		WHERE id = $1
		  AND quote_id = $2
		FOR UPDATE
	`, itemID, quoteID).Scan(&currentMaterialID, &qty, &taxCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if strings.TrimSpace(currentMaterialID) == "" {
		return nil, errors.New("Angebotsposition hat kein Material")
	}

	primary, err := primaryPriceSourceForMaterialTx(ctx, tx, currentMaterialID)
	if err != nil {
		return nil, err
	}

	netAmount := qty * primary.UnitPrice
	taxAmount := netAmount * taxRate(taxCode)
	if _, err := tx.Exec(ctx, `
		UPDATE quote_items
		SET unit_price = $2,
			net_amount = $3,
			tax_amount = $4,
			price_mapping_status = 'manual'
		WHERE id = $1
	`, itemID, primary.UnitPrice, netAmount, taxAmount); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE quotes
		SET net_amount = totals.net_amount,
			tax_amount = totals.tax_amount,
			gross_amount = totals.net_amount + totals.tax_amount
		FROM (
			SELECT
				COALESCE(SUM(net_amount), 0) AS net_amount,
				COALESCE(SUM(tax_amount), 0) AS tax_amount
			FROM quote_items
			WHERE quote_id = $1
		) totals
		WHERE quotes.id = $1
	`, quoteID); err != nil {
		return nil, err
	}

	if err := insertPrimarySourceAppliedDecisionTx(ctx, tx, quoteID, itemID, currentMaterialID, primary); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, quoteID)
}

func (s *Service) ApplyTargetUnitPriceForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*Quote, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var quoteCurrency string
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id, COALESCE(NULLIF(BTRIM(currency), ''), 'EUR')
		FROM quotes
		WHERE id = $1
		FOR UPDATE
	`, quoteID).Scan(&currentStatus, &supersededByQuoteID, &quoteCurrency); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var currentMaterialID string
	var qty float64
	var taxCode string
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(material_id, ''),
			qty,
			COALESCE(tax_code, '')
		FROM quote_items
		WHERE id = $1
		  AND quote_id = $2
		FOR UPDATE
	`, itemID, quoteID).Scan(&currentMaterialID, &qty, &taxCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}

	var costBasisUnitPrice float64
	var decisionCurrency string
	err = tx.QueryRow(ctx, `
		SELECT
			source_unit_price,
			BTRIM(currency)
		FROM quote_item_price_decisions
		WHERE quote_id = $1
		  AND quote_item_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, quoteID, itemID).Scan(&costBasisUnitPrice, &decisionCurrency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("keine Preisentscheidung fuer diese Position vorhanden")
		}
		return nil, err
	}

	targetMarginPercent := s.quoteTargetMarginPercent(ctx)
	targetUnitPrice := roundCurrency(costBasisUnitPrice * (1 + targetMarginPercent/100))
	netAmount := qty * targetUnitPrice
	taxAmount := netAmount * taxRate(taxCode)
	if _, err := tx.Exec(ctx, `
		UPDATE quote_items
		SET unit_price = $2,
			net_amount = $3,
			tax_amount = $4,
			price_mapping_status = 'manual'
		WHERE id = $1
	`, itemID, targetUnitPrice, netAmount, taxAmount); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE quotes
		SET net_amount = totals.net_amount,
			tax_amount = totals.tax_amount,
			gross_amount = totals.net_amount + totals.tax_amount
		FROM (
			SELECT
				COALESCE(SUM(net_amount), 0) AS net_amount,
				COALESCE(SUM(tax_amount), 0) AS tax_amount
			FROM quote_items
			WHERE quote_id = $1
		) totals
		WHERE quotes.id = $1
	`, quoteID); err != nil {
		return nil, err
	}

	currency := strings.TrimSpace(decisionCurrency)
	if currency == "" {
		currency = quoteCurrency
	}
	if err := insertTargetPriceAppliedDecisionTx(ctx, tx, quoteID, itemID, currentMaterialID, costBasisUnitPrice, targetUnitPrice, currency, targetMarginPercent); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, quoteID)
}

func (s *Service) RequestApprovalForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, requestedBy, comment string) (*QuoteItemApprovalRequest, error) {
	reasonText := strings.TrimSpace(comment)
	if len([]rune(reasonText)) > 500 {
		return nil, errors.New("Kommentar darf nicht laenger als 500 Zeichen sein")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id
		FROM quotes
		WHERE id = $1
		FOR UPDATE
	`, quoteID).Scan(&currentStatus, &supersededByQuoteID); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var currentUnitPrice float64
	if err := tx.QueryRow(ctx, `
		SELECT unit_price
		FROM quote_items
		WHERE id = $1
		  AND quote_id = $2
		FOR UPDATE
	`, itemID, quoteID).Scan(&currentUnitPrice); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}

	var decisionID uuid.UUID
	var costBasisUnitPrice float64
	err = tx.QueryRow(ctx, `
		SELECT id, source_unit_price
		FROM quote_item_price_decisions
		WHERE quote_id = $1
		  AND quote_item_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, quoteID, itemID).Scan(&decisionID, &costBasisUnitPrice)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("keine Preisentscheidung fuer diese Position vorhanden")
		}
		return nil, err
	}

	targetMarginPercent := quoteTargetMarginPercentTx(ctx, tx)
	targetUnitPrice := costBasisUnitPrice * (1 + targetMarginPercent/100)
	targetDifference := currentUnitPrice - targetUnitPrice
	absoluteMargin := currentUnitPrice - costBasisUnitPrice
	var marginPercent *float64
	if costBasisUnitPrice != 0 {
		percent := absoluteMargin / costBasisUnitPrice * 100
		marginPercent = &percent
	}

	reasonCode := ""
	const centTolerance = 0.005
	if absoluteMargin < -centTolerance {
		reasonCode = "negative_margin"
	} else if targetDifference < -centTolerance {
		reasonCode = "below_target_margin"
	} else {
		return nil, errors.New("Position erreicht den Zielpreis; keine Freigabeanforderung erforderlich")
	}

	var activeRequestID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM quote_item_approval_requests
		WHERE quote_item_id = $1
		  AND status = 'requested'
		LIMIT 1
	`, itemID).Scan(&activeRequestID)
	if err == nil {
		return nil, errors.New("Freigabeanforderung ist bereits aktiv")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	requestID := uuid.New()
	out, err := scanApprovalRequest(tx.QueryRow(ctx, `
		INSERT INTO quote_item_approval_requests (
			id,
			quote_id,
			quote_item_id,
			status,
			reason_code,
			reason_text,
			current_unit_price_snapshot,
			cost_basis_unit_price_snapshot,
			target_unit_price_snapshot,
			target_margin_percent_snapshot,
			target_difference_snapshot,
			margin_percent_snapshot,
			price_decision_id,
			requested_by
		)
		VALUES ($1, $2, $3, 'requested', $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''))
		RETURNING
			id,
			quote_id,
			quote_item_id,
			status,
			reason_code,
			reason_text,
			current_unit_price_snapshot,
			cost_basis_unit_price_snapshot,
			target_unit_price_snapshot,
			target_margin_percent_snapshot,
			target_difference_snapshot,
			margin_percent_snapshot,
			price_decision_id,
			requested_by,
			'' AS requested_by_name,
			requested_at,
			cancelled_by,
			'' AS cancelled_by_name,
			cancelled_at,
			decided_by,
			'' AS decided_by_name,
			decided_at,
			decision_comment,
			approved_unit_price_snapshot,
			approved_target_margin_percent_snapshot,
			created_at,
			updated_at
	`,
		requestID,
		quoteID,
		itemID,
		reasonCode,
		reasonText,
		currentUnitPrice,
		costBasisUnitPrice,
		targetUnitPrice,
		targetMarginPercent,
		targetDifference,
		marginPercent,
		decisionID,
		strings.TrimSpace(requestedBy),
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) CancelApprovalRequestForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, cancelledBy string) (*QuoteItemApprovalRequest, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id
		FROM quotes
		WHERE id = $1
		FOR UPDATE
	`, quoteID).Scan(&currentStatus, &supersededByQuoteID); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var itemExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM quote_items
			WHERE id = $1
			  AND quote_id = $2
		)
	`, itemID, quoteID).Scan(&itemExists); err != nil {
		return nil, err
	}
	if !itemExists {
		return nil, errors.New("Angebotsposition nicht gefunden")
	}

	out, err := scanApprovalRequest(tx.QueryRow(ctx, `
		UPDATE quote_item_approval_requests
		SET
			status = 'cancelled',
			cancelled_by = NULLIF($3, ''),
			cancelled_at = now(),
			updated_at = now()
		WHERE quote_id = $1
		  AND quote_item_id = $2
		  AND status = 'requested'
		RETURNING
			id,
			quote_id,
			quote_item_id,
			status,
			reason_code,
			reason_text,
			current_unit_price_snapshot,
			cost_basis_unit_price_snapshot,
			target_unit_price_snapshot,
			target_margin_percent_snapshot,
			target_difference_snapshot,
			margin_percent_snapshot,
			price_decision_id,
			requested_by,
			'' AS requested_by_name,
			requested_at,
			cancelled_by,
			'' AS cancelled_by_name,
			cancelled_at,
			decided_by,
			'' AS decided_by_name,
			decided_at,
			decision_comment,
			approved_unit_price_snapshot,
			approved_target_margin_percent_snapshot,
			created_at,
			updated_at
	`, quoteID, itemID, strings.TrimSpace(cancelledBy)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Keine aktive Freigabeanforderung vorhanden")
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ApproveApprovalRequestForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, decidedBy, comment string) (*QuoteItemApprovalRequest, error) {
	return s.decideApprovalRequestForQuoteItem(ctx, quoteID, itemID, decidedBy, comment, "approved")
}

func (s *Service) RejectApprovalRequestForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, decidedBy, comment string) (*QuoteItemApprovalRequest, error) {
	return s.decideApprovalRequestForQuoteItem(ctx, quoteID, itemID, decidedBy, comment, "rejected")
}

func (s *Service) ResolveApprovalReworkForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, resolvedBy, comment string) (*QuoteItemApprovalRequest, error) {
	comment = strings.TrimSpace(comment)
	if len([]rune(comment)) > 500 {
		return nil, errors.New("Kommentar darf nicht laenger als 500 Zeichen sein")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id
		FROM quotes
		WHERE id = $1
		FOR UPDATE
	`, quoteID).Scan(&currentStatus, &supersededByQuoteID); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var currentUnitPrice float64
	if err := tx.QueryRow(ctx, `
		SELECT unit_price
		FROM quote_items
		WHERE id = $1
		  AND quote_id = $2
		FOR UPDATE
	`, itemID, quoteID).Scan(&currentUnitPrice); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}

	var activeRequestID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM quote_item_approval_requests
		WHERE quote_item_id = $1
		  AND status = 'requested'
		LIMIT 1
	`, itemID).Scan(&activeRequestID)
	if err == nil {
		return nil, errors.New("Freigabeanforderung ist bereits aktiv")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var latestStatus string
	var latestReasonCode string
	var latestReasonText string
	err = tx.QueryRow(ctx, `
		SELECT status, reason_code, reason_text
		FROM quote_item_approval_requests
		WHERE quote_item_id = $1
		  AND status IN ('approved', 'rejected', 'rework_resolved')
		ORDER BY decided_at DESC NULLS LAST, updated_at DESC
		LIMIT 1
	`, itemID).Scan(&latestStatus, &latestReasonCode, &latestReasonText)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Keine offene Nacharbeit fuer diese Position vorhanden")
		}
		return nil, err
	}
	if latestStatus != "rejected" {
		return nil, errors.New("Keine offene Nacharbeit fuer diese Position vorhanden")
	}

	var decisionID uuid.UUID
	var costBasisUnitPrice float64
	err = tx.QueryRow(ctx, `
		SELECT id, source_unit_price
		FROM quote_item_price_decisions
		WHERE quote_id = $1
		  AND quote_item_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, quoteID, itemID).Scan(&decisionID, &costBasisUnitPrice)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("keine Preisentscheidung fuer diese Position vorhanden")
		}
		return nil, err
	}

	targetMarginPercent := quoteTargetMarginPercentTx(ctx, tx)
	targetUnitPrice := costBasisUnitPrice * (1 + targetMarginPercent/100)
	targetDifference := currentUnitPrice - targetUnitPrice
	const centTolerance = 0.005
	if targetDifference < -centTolerance {
		return nil, errors.New("Nacharbeit erreicht die Zielmarge noch nicht")
	}

	absoluteMargin := currentUnitPrice - costBasisUnitPrice
	var marginPercent *float64
	if costBasisUnitPrice != 0 {
		percent := absoluteMargin / costBasisUnitPrice * 100
		marginPercent = &percent
	}

	reasonCode := strings.TrimSpace(latestReasonCode)
	if reasonCode == "" {
		reasonCode = "below_target_margin"
	}
	out, err := scanApprovalRequest(tx.QueryRow(ctx, `
		INSERT INTO quote_item_approval_requests (
			id,
			quote_id,
			quote_item_id,
			status,
			reason_code,
			reason_text,
			current_unit_price_snapshot,
			cost_basis_unit_price_snapshot,
			target_unit_price_snapshot,
			target_margin_percent_snapshot,
			target_difference_snapshot,
			margin_percent_snapshot,
			price_decision_id,
			decided_by,
			decided_at,
			decision_comment,
			approved_unit_price_snapshot,
			approved_target_margin_percent_snapshot
		)
		VALUES ($1, $2, $3, 'rework_resolved', $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''), now(), $14, $6, $9)
		RETURNING
			id,
			quote_id,
			quote_item_id,
			status,
			reason_code,
			reason_text,
			current_unit_price_snapshot,
			cost_basis_unit_price_snapshot,
			target_unit_price_snapshot,
			target_margin_percent_snapshot,
			target_difference_snapshot,
			margin_percent_snapshot,
			price_decision_id,
			requested_by,
			'' AS requested_by_name,
			requested_at,
			cancelled_by,
			'' AS cancelled_by_name,
			cancelled_at,
			decided_by,
			'' AS decided_by_name,
			decided_at,
			decision_comment,
			approved_unit_price_snapshot,
			approved_target_margin_percent_snapshot,
			created_at,
			updated_at
	`,
		uuid.New(),
		quoteID,
		itemID,
		reasonCode,
		strings.TrimSpace(latestReasonText),
		currentUnitPrice,
		costBasisUnitPrice,
		targetUnitPrice,
		targetMarginPercent,
		targetDifference,
		marginPercent,
		decisionID,
		strings.TrimSpace(resolvedBy),
		comment,
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ListApprovalRequestsForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) ([]QuoteItemApprovalRequest, error) {
	var itemExists bool
	if err := s.pg.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM quotes q
			JOIN quote_items qi ON qi.quote_id = q.id
			WHERE q.id = $1
			  AND qi.id = $2
		)
	`, quoteID, itemID).Scan(&itemExists); err != nil {
		return nil, err
	}
	if !itemExists {
		return nil, errors.New("Angebotsposition nicht gefunden")
	}

	rows, err := s.pg.Query(ctx, `
		SELECT
			qar.id,
			qar.quote_id,
			qar.quote_item_id,
			qar.status,
			qar.reason_code,
			qar.reason_text,
			qar.current_unit_price_snapshot,
			qar.cost_basis_unit_price_snapshot,
			qar.target_unit_price_snapshot,
			qar.target_margin_percent_snapshot,
			qar.target_difference_snapshot,
			qar.margin_percent_snapshot,
			qar.price_decision_id,
			qar.requested_by,
			CASE
				WHEN COALESCE(qar.requested_by, '') = '' THEN ''
				ELSE COALESCE(NULLIF(requested_user.display_name, ''), NULLIF(requested_user.email, ''), qar.requested_by)
			END AS requested_by_name,
			qar.requested_at,
			qar.cancelled_by,
			CASE
				WHEN COALESCE(qar.cancelled_by, '') = '' THEN ''
				ELSE COALESCE(NULLIF(cancelled_user.display_name, ''), NULLIF(cancelled_user.email, ''), qar.cancelled_by)
			END AS cancelled_by_name,
			qar.cancelled_at,
			qar.decided_by,
			CASE
				WHEN COALESCE(qar.decided_by, '') = '' THEN ''
				ELSE COALESCE(NULLIF(decided_user.display_name, ''), NULLIF(decided_user.email, ''), qar.decided_by)
			END AS decided_by_name,
			qar.decided_at,
			qar.decision_comment,
			qar.approved_unit_price_snapshot,
			qar.approved_target_margin_percent_snapshot,
			qar.created_at,
			qar.updated_at
		FROM quote_item_approval_requests qar
		LEFT JOIN users requested_user ON requested_user.id = qar.requested_by
		LEFT JOIN users cancelled_user ON cancelled_user.id = qar.cancelled_by
		LEFT JOIN users decided_user ON decided_user.id = qar.decided_by
		WHERE qar.quote_id = $1
		  AND qar.quote_item_id = $2
		ORDER BY qar.created_at DESC, qar.requested_at DESC
	`, quoteID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]QuoteItemApprovalRequest, 0)
	for rows.Next() {
		entry, err := scanApprovalRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *entry)
	}
	return out, rows.Err()
}

func (s *Service) ListApprovalReworkQueue(ctx context.Context, filter QuoteApprovalReworkQueueFilter) ([]QuoteApprovalReworkQueueItem, error) {
	args := make([]any, 0)
	conds := []string{
		"q.superseded_by_quote_id IS NULL",
		"latest_approval.status = 'rejected'",
	}
	if filter.ProjectID != uuid.Nil {
		args = append(args, filter.ProjectID)
		conds = append(conds, fmt.Sprintf("q.project_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		args = append(args, strings.TrimSpace(filter.ContactID))
		conds = append(conds, fmt.Sprintf("q.contact_id = $%d", len(args)))
	}
	if filter.QuoteID != uuid.Nil {
		args = append(args, filter.QuoteID)
		conds = append(conds, fmt.Sprintf("q.id = $%d", len(args)))
	}

	rows, err := s.pg.Query(ctx, `
		SELECT
			q.id,
			q.nummer,
			q.status,
			q.quote_date,
			COALESCE(q.project_id::text, ''),
			COALESCE(p.name, ''),
			COALESCE(q.contact_id, ''),
			COALESCE(c.name, ''),
			qi.id,
			qi.position,
			qi.description,
			qi.unit_price,
			COALESCE(NULLIF(BTRIM(q.currency), ''), 'EUR'),
			latest_approval.id,
			latest_approval.reason_code,
			latest_approval.reason_text,
			latest_approval.decision_comment,
			latest_approval.decided_by,
			latest_approval.decided_by_name,
			latest_approval.decided_at,
			latest_approval.current_unit_price_snapshot,
			latest_approval.cost_basis_unit_price_snapshot,
			latest_approval.target_unit_price_snapshot,
			latest_approval.target_margin_percent_snapshot,
			latest_approval.target_difference_snapshot,
			latest_approval.margin_percent_snapshot,
			latest_approval.price_decision_id,
			current_price_decision.id,
			current_price_decision.source_unit_price
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		LEFT JOIN projects p ON p.id = q.project_id
		LEFT JOIN contacts c ON c.id = q.contact_id
		JOIN LATERAL (
			SELECT
				qar.id,
				qar.status,
				qar.reason_code,
				qar.reason_text,
				qar.decision_comment,
				qar.decided_by,
				CASE
					WHEN COALESCE(qar.decided_by, '') = '' THEN ''
					ELSE COALESCE(NULLIF(decided_user.display_name, ''), NULLIF(decided_user.email, ''), qar.decided_by)
				END AS decided_by_name,
				qar.decided_at,
				qar.current_unit_price_snapshot,
				qar.cost_basis_unit_price_snapshot,
				qar.target_unit_price_snapshot,
				qar.target_margin_percent_snapshot,
				qar.target_difference_snapshot,
				qar.margin_percent_snapshot,
				qar.price_decision_id
			FROM quote_item_approval_requests qar
			LEFT JOIN users decided_user ON decided_user.id = qar.decided_by
			WHERE qar.quote_item_id = qi.id
			  AND qar.status IN ('approved', 'rejected', 'rework_resolved')
			ORDER BY qar.decided_at DESC NULLS LAST, qar.updated_at DESC
			LIMIT 1
		) latest_approval ON true
		LEFT JOIN LATERAL (
			SELECT qipd.id, qipd.source_unit_price
			FROM quote_item_price_decisions qipd
			WHERE qipd.quote_item_id = qi.id
			ORDER BY qipd.created_at DESC
			LIMIT 1
		) current_price_decision ON true
		WHERE `+strings.Join(conds, " AND ")+`
		ORDER BY latest_approval.decided_at ASC NULLS LAST, q.nummer ASC, qi.position ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targetMarginPercent := s.quoteTargetMarginPercent(ctx)
	out := make([]QuoteApprovalReworkQueueItem, 0)
	for rows.Next() {
		var item QuoteApprovalReworkQueueItem
		var marginPercentSnapshot sql.NullFloat64
		var priceDecisionID uuid.NullUUID
		var currentPriceDecisionID uuid.NullUUID
		var currentCostBasis sql.NullFloat64
		if err := rows.Scan(
			&item.QuoteID,
			&item.QuoteNumber,
			&item.QuoteStatus,
			&item.QuoteDate,
			&item.ProjectID,
			&item.ProjectName,
			&item.ContactID,
			&item.ContactName,
			&item.QuoteItemID,
			&item.Position,
			&item.Description,
			&item.CurrentUnitPrice,
			&item.Currency,
			&item.ApprovalRequestID,
			&item.ReasonCode,
			&item.ReasonText,
			&item.DecisionComment,
			&item.DecidedBy,
			&item.DecidedByName,
			&item.DecidedAt,
			&item.CurrentUnitPriceSnapshot,
			&item.CostBasisUnitPriceSnapshot,
			&item.TargetUnitPriceSnapshot,
			&item.TargetMarginPercentSnapshot,
			&item.TargetDifferenceSnapshot,
			&marginPercentSnapshot,
			&priceDecisionID,
			&currentPriceDecisionID,
			&currentCostBasis,
		); err != nil {
			return nil, err
		}
		if marginPercentSnapshot.Valid {
			item.MarginPercentSnapshot = &marginPercentSnapshot.Float64
		}
		if priceDecisionID.Valid {
			id := priceDecisionID.UUID
			item.PriceDecisionID = &id
		}
		if currentPriceDecisionID.Valid {
			id := currentPriceDecisionID.UUID
			item.CurrentPriceDecisionID = &id
		}
		if currentCostBasis.Valid {
			targetUnitPrice := currentCostBasis.Float64 * (1 + targetMarginPercent/100)
			targetDifference := item.CurrentUnitPrice - targetUnitPrice
			item.CurrentTargetUnitPrice = &targetUnitPrice
			item.CurrentTargetDifference = &targetDifference
			if currentCostBasis.Float64 != 0 {
				percent := (item.CurrentUnitPrice - currentCostBasis.Float64) / currentCostBasis.Float64 * 100
				item.CurrentMarginPercent = &percent
			}

			item.CurrentTargetStatus = "on_target"
			const centTolerance = 0.005
			if item.CurrentUnitPrice-currentCostBasis.Float64 < -centTolerance {
				item.CurrentTargetStatus = "below_cost"
			} else if targetDifference < -centTolerance {
				item.CurrentTargetStatus = "below_target"
			} else if math.Abs(targetDifference) <= centTolerance {
				item.CurrentTargetStatus = "on_target"
			} else {
				item.CurrentTargetStatus = "above_target"
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ListApprovalRequestQueue(ctx context.Context, filter QuoteApprovalRequestQueueFilter) ([]QuoteApprovalRequestQueueItem, error) {
	args := make([]any, 0)
	conds := []string{
		"q.superseded_by_quote_id IS NULL",
		"qar.status = 'requested'",
	}
	if filter.ProjectID != uuid.Nil {
		args = append(args, filter.ProjectID)
		conds = append(conds, fmt.Sprintf("q.project_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		args = append(args, strings.TrimSpace(filter.ContactID))
		conds = append(conds, fmt.Sprintf("q.contact_id = $%d", len(args)))
	}
	if filter.QuoteID != uuid.Nil {
		args = append(args, filter.QuoteID)
		conds = append(conds, fmt.Sprintf("q.id = $%d", len(args)))
	}

	rows, err := s.pg.Query(ctx, `
		SELECT
			q.id,
			q.nummer,
			q.status,
			q.quote_date,
			COALESCE(q.project_id::text, ''),
			COALESCE(p.name, ''),
			COALESCE(q.contact_id, ''),
			COALESCE(c.name, ''),
			qi.id,
			qi.position,
			qi.description,
			qi.unit_price,
			COALESCE(NULLIF(BTRIM(q.currency), ''), 'EUR'),
			qar.id,
			qar.reason_code,
			qar.reason_text,
			qar.requested_by,
			CASE
				WHEN COALESCE(qar.requested_by, '') = '' THEN ''
				ELSE COALESCE(NULLIF(requested_user.display_name, ''), NULLIF(requested_user.email, ''), qar.requested_by)
			END AS requested_by_name,
			qar.requested_at,
			qar.current_unit_price_snapshot,
			qar.cost_basis_unit_price_snapshot,
			qar.target_unit_price_snapshot,
			qar.target_margin_percent_snapshot,
			qar.target_difference_snapshot,
			qar.margin_percent_snapshot,
			qar.price_decision_id,
			current_price_decision.id,
			current_price_decision.source_unit_price
		FROM quote_item_approval_requests qar
		JOIN quotes q ON q.id = qar.quote_id
		JOIN quote_items qi ON qi.id = qar.quote_item_id AND qi.quote_id = q.id
		LEFT JOIN users requested_user ON requested_user.id = qar.requested_by
		LEFT JOIN projects p ON p.id = q.project_id
		LEFT JOIN contacts c ON c.id = q.contact_id
		LEFT JOIN LATERAL (
			SELECT qipd.id, qipd.source_unit_price
			FROM quote_item_price_decisions qipd
			WHERE qipd.quote_item_id = qi.id
			ORDER BY qipd.created_at DESC
			LIMIT 1
		) current_price_decision ON true
		WHERE `+strings.Join(conds, " AND ")+`
		ORDER BY qar.requested_at ASC, q.nummer ASC, qi.position ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targetMarginPercent := s.quoteTargetMarginPercent(ctx)
	out := make([]QuoteApprovalRequestQueueItem, 0)
	for rows.Next() {
		var item QuoteApprovalRequestQueueItem
		var marginPercentSnapshot sql.NullFloat64
		var priceDecisionID uuid.NullUUID
		var currentPriceDecisionID uuid.NullUUID
		var currentCostBasis sql.NullFloat64
		if err := rows.Scan(
			&item.QuoteID,
			&item.QuoteNumber,
			&item.QuoteStatus,
			&item.QuoteDate,
			&item.ProjectID,
			&item.ProjectName,
			&item.ContactID,
			&item.ContactName,
			&item.QuoteItemID,
			&item.Position,
			&item.Description,
			&item.CurrentUnitPrice,
			&item.Currency,
			&item.ApprovalRequestID,
			&item.ReasonCode,
			&item.ReasonText,
			&item.RequestedBy,
			&item.RequestedByName,
			&item.RequestedAt,
			&item.CurrentUnitPriceSnapshot,
			&item.CostBasisUnitPriceSnapshot,
			&item.TargetUnitPriceSnapshot,
			&item.TargetMarginPercentSnapshot,
			&item.TargetDifferenceSnapshot,
			&marginPercentSnapshot,
			&priceDecisionID,
			&currentPriceDecisionID,
			&currentCostBasis,
		); err != nil {
			return nil, err
		}
		if marginPercentSnapshot.Valid {
			item.MarginPercentSnapshot = &marginPercentSnapshot.Float64
		}
		if priceDecisionID.Valid {
			id := priceDecisionID.UUID
			item.PriceDecisionID = &id
		}
		if currentPriceDecisionID.Valid {
			id := currentPriceDecisionID.UUID
			item.CurrentPriceDecisionID = &id
		}
		if currentCostBasis.Valid {
			targetUnitPrice := currentCostBasis.Float64 * (1 + targetMarginPercent/100)
			targetDifference := item.CurrentUnitPrice - targetUnitPrice
			item.CurrentTargetUnitPrice = &targetUnitPrice
			item.CurrentTargetDifference = &targetDifference
			if currentCostBasis.Float64 != 0 {
				percent := (item.CurrentUnitPrice - currentCostBasis.Float64) / currentCostBasis.Float64 * 100
				item.CurrentMarginPercent = &percent
			}

			item.CurrentTargetStatus = "on_target"
			const centTolerance = 0.005
			if item.CurrentUnitPrice-currentCostBasis.Float64 < -centTolerance {
				item.CurrentTargetStatus = "below_cost"
			} else if targetDifference < -centTolerance {
				item.CurrentTargetStatus = "below_target"
			} else if math.Abs(targetDifference) <= centTolerance {
				item.CurrentTargetStatus = "on_target"
			} else {
				item.CurrentTargetStatus = "above_target"
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) decideApprovalRequestForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID, decidedBy, comment, decisionStatus string) (*QuoteItemApprovalRequest, error) {
	comment = strings.TrimSpace(comment)
	if len([]rune(comment)) > 500 {
		return nil, errors.New("Kommentar darf nicht laenger als 500 Zeichen sein")
	}
	if decisionStatus != "approved" && decisionStatus != "rejected" {
		return nil, errors.New("ungueltiger Freigabeentscheid")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	if err := tx.QueryRow(ctx, `
		SELECT status, superseded_by_quote_id
		FROM quotes
		WHERE id = $1
		FOR UPDATE
	`, quoteID).Scan(&currentStatus, &supersededByQuoteID); err != nil {
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var itemExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM quote_items
			WHERE id = $1
			  AND quote_id = $2
		)
	`, itemID, quoteID).Scan(&itemExists); err != nil {
		return nil, err
	}
	if !itemExists {
		return nil, errors.New("Angebotsposition nicht gefunden")
	}

	out, err := scanApprovalRequest(tx.QueryRow(ctx, `
		UPDATE quote_item_approval_requests
		SET
			status = $3,
			decided_by = NULLIF($4, ''),
			decided_at = now(),
			decision_comment = $5,
			approved_unit_price_snapshot = CASE WHEN $3 = 'approved' THEN current_unit_price_snapshot ELSE NULL END,
			approved_target_margin_percent_snapshot = CASE WHEN $3 = 'approved' THEN target_margin_percent_snapshot ELSE NULL END,
			updated_at = now()
		WHERE quote_id = $1
		  AND quote_item_id = $2
		  AND status = 'requested'
		RETURNING
			id,
			quote_id,
			quote_item_id,
			status,
			reason_code,
			reason_text,
			current_unit_price_snapshot,
			cost_basis_unit_price_snapshot,
			target_unit_price_snapshot,
			target_margin_percent_snapshot,
			target_difference_snapshot,
			margin_percent_snapshot,
			price_decision_id,
			requested_by,
			'' AS requested_by_name,
			requested_at,
			cancelled_by,
			'' AS cancelled_by_name,
			cancelled_at,
			decided_by,
			'' AS decided_by_name,
			decided_at,
			decision_comment,
			approved_unit_price_snapshot,
			approved_target_margin_percent_snapshot,
			created_at,
			updated_at
	`, quoteID, itemID, decisionStatus, strings.TrimSpace(decidedBy), comment))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Keine aktive Freigabeanforderung vorhanden")
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func insertPrimarySourceAppliedDecisionTx(ctx context.Context, tx pgx.Tx, quoteID, itemID uuid.UUID, materialID string, primary PriceSourcePriorityEntry) error {
	currency := strings.TrimSpace(primary.Currency)
	if currency == "" {
		currency = "EUR"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO quote_item_price_decisions (
			id,
			quote_id,
			quote_item_id,
			material_id,
			decision_type,
			source_label,
			source_unit_price,
			applied_unit_price,
			currency,
			source_reference,
			source_date
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), 'primary_source_applied', $5, $6, $6, $7, $8, $9)
	`,
		uuid.New(),
		quoteID,
		itemID,
		strings.TrimSpace(materialID),
		primary.SourceLabel,
		primary.UnitPrice,
		currency,
		primary.Reference,
		primary.Date,
	)
	return err
}

func insertTargetPriceAppliedDecisionTx(ctx context.Context, tx pgx.Tx, quoteID, itemID uuid.UUID, materialID string, costBasisUnitPrice, targetUnitPrice float64, currency string, targetMarginPercent float64) error {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "EUR"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO quote_item_price_decisions (
			id,
			quote_id,
			quote_item_id,
			material_id,
			decision_type,
			source_label,
			source_unit_price,
			applied_unit_price,
			currency,
			source_reference
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), 'target_price_applied', 'Zielpreis aus Zielmarge', $5, $6, $7, $8)
	`,
		uuid.New(),
		quoteID,
		itemID,
		strings.TrimSpace(materialID),
		costBasisUnitPrice,
		targetUnitPrice,
		currency,
		fmt.Sprintf("target_margin_percent=%.2f", targetMarginPercent),
	)
	return err
}

func roundCurrency(value float64) float64 {
	return math.Round(value*100) / 100
}

func scanApprovalRequest(row pgx.Row) (*QuoteItemApprovalRequest, error) {
	var out QuoteItemApprovalRequest
	var marginPercent sql.NullFloat64
	var priceDecisionID uuid.NullUUID
	var requestedBy sql.NullString
	var requestedByName sql.NullString
	var cancelledBy sql.NullString
	var cancelledByName sql.NullString
	var cancelledAt sql.NullTime
	var decidedBy sql.NullString
	var decidedByName sql.NullString
	var decidedAt sql.NullTime
	var decisionComment sql.NullString
	var approvedUnitPrice sql.NullFloat64
	var approvedTargetMarginPercent sql.NullFloat64
	if err := row.Scan(
		&out.ID,
		&out.QuoteID,
		&out.QuoteItemID,
		&out.Status,
		&out.ReasonCode,
		&out.ReasonText,
		&out.CurrentUnitPriceSnapshot,
		&out.CostBasisUnitPriceSnapshot,
		&out.TargetUnitPriceSnapshot,
		&out.TargetMarginPercentSnapshot,
		&out.TargetDifferenceSnapshot,
		&marginPercent,
		&priceDecisionID,
		&requestedBy,
		&requestedByName,
		&out.RequestedAt,
		&cancelledBy,
		&cancelledByName,
		&cancelledAt,
		&decidedBy,
		&decidedByName,
		&decidedAt,
		&decisionComment,
		&approvedUnitPrice,
		&approvedTargetMarginPercent,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if marginPercent.Valid {
		out.MarginPercentSnapshot = &marginPercent.Float64
	}
	if priceDecisionID.Valid {
		out.PriceDecisionID = &priceDecisionID.UUID
	}
	if requestedBy.Valid {
		out.RequestedBy = requestedBy.String
	}
	if requestedByName.Valid {
		out.RequestedByName = requestedByName.String
	}
	if cancelledBy.Valid {
		out.CancelledBy = cancelledBy.String
	}
	if cancelledByName.Valid {
		out.CancelledByName = cancelledByName.String
	}
	if cancelledAt.Valid {
		out.CancelledAt = &cancelledAt.Time
	}
	if decidedBy.Valid {
		out.DecidedBy = decidedBy.String
	}
	if decidedByName.Valid {
		out.DecidedByName = decidedByName.String
	}
	if decidedAt.Valid {
		out.DecidedAt = &decidedAt.Time
	}
	if decisionComment.Valid {
		out.DecisionComment = decisionComment.String
	}
	if approvedUnitPrice.Valid {
		out.ApprovedUnitPriceSnapshot = &approvedUnitPrice.Float64
	}
	if approvedTargetMarginPercent.Valid {
		out.ApprovedTargetMarginPercent = &approvedTargetMarginPercent.Float64
	}
	return &out, nil
}

func (s *Service) PriceHistoryForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) ([]PriceHistoryEntry, error) {
	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var currentMaterialID string
	err := s.pg.QueryRow(ctx, `
		SELECT
			q.status,
			q.superseded_by_quote_id,
			COALESCE(qi.material_id,'')
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		WHERE q.id = $1
		  AND qi.id = $2
	`, quoteID, itemID).Scan(&currentStatus, &supersededByQuoteID, &currentMaterialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}
	if strings.TrimSpace(currentMaterialID) == "" {
		return nil, errors.New("Angebotsposition hat kein Material")
	}

	entries := make([]PriceHistoryEntry, 0, 2)

	var avgEntry PriceHistoryEntry
	if err := s.pg.QueryRow(ctx, `
		SELECT
			COALESCE(m.avg_purchase_price, 0),
			COALESCE(NULLIF(BTRIM(m.currency), ''), 'EUR')
		FROM materials m
		WHERE m.id = $1
	`, currentMaterialID).Scan(&avgEntry.UnitPrice, &avgEntry.Currency); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Material nicht gefunden")
		}
		return nil, err
	}
	avgEntry.SourceLabel = "Durchschnittlicher Einkaufspreis"
	entries = append(entries, avgEntry)

	var latestEntry PriceHistoryEntry
	var orderDate time.Time
	err = s.pg.QueryRow(ctx, `
		SELECT
			poi.unit_price,
			COALESCE(NULLIF(BTRIM(poi.currency), ''), COALESCE(NULLIF(BTRIM(po.currency), ''), 'EUR')),
			COALESCE(NULLIF(BTRIM(po.number), ''), po.id),
			po.order_date
		FROM purchase_order_items poi
		JOIN purchase_orders po ON po.id = poi.order_id
		WHERE poi.material_id = $1
		  AND po.status <> 'canceled'
		ORDER BY po.order_date DESC, poi.position DESC
		LIMIT 1
	`, currentMaterialID).Scan(&latestEntry.UnitPrice, &latestEntry.Currency, &latestEntry.Reference, &orderDate)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		latestEntry.SourceLabel = "Letzter Bestellpreis"
		latestEntry.Reference = "Bestellung " + latestEntry.Reference
		latestEntry.Date = &orderDate
		entries = append(entries, latestEntry)
	}

	return entries, nil
}

func (s *Service) PriceDecisionHistoryForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) ([]PriceDecisionHistoryEntry, error) {
	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	err := s.pg.QueryRow(ctx, `
		SELECT
			q.status,
			q.superseded_by_quote_id
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		WHERE q.id = $1
		  AND qi.id = $2
	`, quoteID, itemID).Scan(&currentStatus, &supersededByQuoteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	rows, err := s.pg.Query(ctx, `
		SELECT
			id,
			decision_type,
			COALESCE(material_id, ''),
			source_label,
			source_unit_price,
			applied_unit_price,
			BTRIM(currency),
			source_reference,
			source_date,
			created_at
		FROM quote_item_price_decisions
		WHERE quote_id = $1
		  AND quote_item_id = $2
		ORDER BY created_at DESC
		LIMIT 10
	`, quoteID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]PriceDecisionHistoryEntry, 0, 10)
	for rows.Next() {
		var entry PriceDecisionHistoryEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.DecisionType,
			&entry.MaterialID,
			&entry.SourceLabel,
			&entry.SourceUnitPrice,
			&entry.AppliedUnitPrice,
			&entry.Currency,
			&entry.SourceReference,
			&entry.SourceDate,
			&entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Service) MarginAnchorForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*QuoteItemMarginAnchor, error) {
	var currentStatus string
	var supersededByQuoteID uuid.NullUUID
	var currentUnitPrice float64
	var quoteCurrency string
	err := s.pg.QueryRow(ctx, `
		SELECT
			q.status,
			q.superseded_by_quote_id,
			qi.unit_price,
			COALESCE(NULLIF(BTRIM(q.currency), ''), 'EUR')
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		WHERE q.id = $1
		  AND qi.id = $2
	`, quoteID, itemID).Scan(&currentStatus, &supersededByQuoteID, &currentUnitPrice, &quoteCurrency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}
	if supersededByQuoteID.Valid {
		return nil, errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if currentStatus != "draft" {
		return nil, errors.New("nur Entwürfe sind bearbeitbar")
	}

	var anchor QuoteItemMarginAnchor
	err = s.pg.QueryRow(ctx, `
		SELECT
			id,
			decision_type,
			source_label,
			source_unit_price,
			BTRIM(currency),
			source_reference,
			source_date,
			created_at
		FROM quote_item_price_decisions
		WHERE quote_id = $1
		  AND quote_item_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, quoteID, itemID).Scan(
		&anchor.DecisionID,
		&anchor.DecisionType,
		&anchor.SourceLabel,
		&anchor.CostBasisUnitPrice,
		&anchor.Currency,
		&anchor.SourceReference,
		&anchor.SourceDate,
		&anchor.DecisionCreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("keine Preisentscheidung fuer diese Position vorhanden")
		}
		return nil, err
	}

	anchor.CurrentUnitPrice = currentUnitPrice
	if strings.TrimSpace(anchor.Currency) == "" {
		anchor.Currency = quoteCurrency
	}
	anchor.AbsoluteMargin = currentUnitPrice - anchor.CostBasisUnitPrice
	if anchor.CostBasisUnitPrice != 0 {
		percent := anchor.AbsoluteMargin / anchor.CostBasisUnitPrice * 100
		anchor.MarginPercent = &percent
	}

	anchor.MarginStatus = "zero_margin"
	const centTolerance = 0.005
	if anchor.AbsoluteMargin < -centTolerance {
		anchor.MarginStatus = "negative_margin"
	} else if anchor.AbsoluteMargin > centTolerance {
		anchor.MarginStatus = "positive_margin"
	}

	return &anchor, nil
}

func (s *Service) ApprovalHintForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*QuoteItemApprovalHint, error) {
	marginAnchor, err := s.MarginAnchorForQuoteItem(ctx, quoteID, itemID)
	if err != nil {
		if err.Error() == "keine Preisentscheidung fuer diese Position vorhanden" {
			return &QuoteItemApprovalHint{
				ApprovalStatus: "approval_blocked_until_margin_available",
				ApprovalReason: "Keine gespeicherte Preisentscheidung als Kostenbasis vorhanden",
			}, nil
		}
		return nil, err
	}

	status := "approval_not_required"
	reason := "Marge ist nicht negativ; keine Freigabeempfehlung"
	if marginAnchor.MarginStatus == "negative_margin" {
		status = "approval_recommended"
		reason = "Negative Marge sichtbar; spaetere Freigabe empfohlen"
	}

	return &QuoteItemApprovalHint{
		ApprovalStatus:     status,
		ApprovalReason:     reason,
		MarginStatus:       marginAnchor.MarginStatus,
		CurrentUnitPrice:   &marginAnchor.CurrentUnitPrice,
		CostBasisUnitPrice: &marginAnchor.CostBasisUnitPrice,
		Currency:           marginAnchor.Currency,
		AbsoluteMargin:     &marginAnchor.AbsoluteMargin,
		MarginPercent:      marginAnchor.MarginPercent,
		DecisionID:         &marginAnchor.DecisionID,
		DecisionType:       marginAnchor.DecisionType,
		SourceLabel:        marginAnchor.SourceLabel,
		DecisionCreatedAt:  &marginAnchor.DecisionCreatedAt,
	}, nil
}

func (s *Service) TargetMarginAnchorForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*QuoteItemTargetMarginAnchor, error) {
	targetMarginPercent := s.quoteTargetMarginPercent(ctx)

	marginAnchor, err := s.MarginAnchorForQuoteItem(ctx, quoteID, itemID)
	if err != nil {
		if err.Error() == "keine Preisentscheidung fuer diese Position vorhanden" {
			return &QuoteItemTargetMarginAnchor{
				TargetStatus:        "target_blocked_until_margin_available",
				TargetReason:        "Keine gespeicherte Preisentscheidung als Kostenbasis vorhanden",
				TargetMarginPercent: targetMarginPercent,
			}, nil
		}
		return nil, err
	}

	targetUnitPrice := marginAnchor.CostBasisUnitPrice * (1 + targetMarginPercent/100)
	targetDifference := marginAnchor.CurrentUnitPrice - targetUnitPrice
	var targetDifferencePercent *float64
	if targetUnitPrice != 0 {
		percent := targetDifference / targetUnitPrice * 100
		targetDifferencePercent = &percent
	}

	status := "on_target"
	reason := "Aktueller Preis erreicht den Zielaufschlag"
	const centTolerance = 0.005
	if marginAnchor.MarginStatus == "negative_margin" {
		status = "below_cost"
		reason = "Aktueller Preis liegt unter der Kostenbasis"
	} else if targetDifference < -centTolerance {
		status = "below_target"
		reason = "Aktueller Preis erreicht den Zielaufschlag noch nicht"
	} else if targetDifference > centTolerance {
		status = "above_target"
		reason = "Aktueller Preis liegt ueber dem Zielaufschlag"
	}

	return &QuoteItemTargetMarginAnchor{
		TargetStatus:            status,
		TargetReason:            reason,
		TargetMarginPercent:     targetMarginPercent,
		CurrentUnitPrice:        &marginAnchor.CurrentUnitPrice,
		CostBasisUnitPrice:      &marginAnchor.CostBasisUnitPrice,
		TargetUnitPrice:         &targetUnitPrice,
		Currency:                marginAnchor.Currency,
		AbsoluteMargin:          &marginAnchor.AbsoluteMargin,
		MarginPercent:           marginAnchor.MarginPercent,
		TargetDifference:        &targetDifference,
		TargetDifferencePercent: targetDifferencePercent,
		MarginStatus:            marginAnchor.MarginStatus,
		DecisionID:              &marginAnchor.DecisionID,
		DecisionType:            marginAnchor.DecisionType,
		SourceLabel:             marginAnchor.SourceLabel,
		DecisionCreatedAt:       &marginAnchor.DecisionCreatedAt,
	}, nil
}

func (s *Service) quoteTargetMarginPercent(ctx context.Context) float64 {
	const defaultTargetMarginPercent = 20.0
	var targetMarginPercent float64
	err := s.pg.QueryRow(ctx, `
		SELECT target_margin_percent
		FROM quote_calculation_settings
		WHERE id = 'default'
	`).Scan(&targetMarginPercent)
	if err != nil || targetMarginPercent < 0 || targetMarginPercent > 1000 {
		return defaultTargetMarginPercent
	}
	return targetMarginPercent
}

func quoteTargetMarginPercentTx(ctx context.Context, tx pgx.Tx) float64 {
	const defaultTargetMarginPercent = 20.0
	var targetMarginPercent float64
	err := tx.QueryRow(ctx, `
		SELECT target_margin_percent
		FROM quote_calculation_settings
		WHERE id = 'default'
	`).Scan(&targetMarginPercent)
	if err != nil || targetMarginPercent < 0 || targetMarginPercent > 1000 {
		return defaultTargetMarginPercent
	}
	return targetMarginPercent
}

func primaryPriceSourceForMaterialTx(ctx context.Context, tx pgx.Tx, materialID string) (PriceSourcePriorityEntry, error) {
	var latestEntry PriceSourcePriorityEntry
	var orderDate time.Time
	err := tx.QueryRow(ctx, `
		SELECT
			poi.unit_price,
			COALESCE(NULLIF(BTRIM(poi.currency), ''), COALESCE(NULLIF(BTRIM(po.currency), ''), 'EUR')),
			COALESCE(NULLIF(BTRIM(po.number), ''), po.id),
			po.order_date
		FROM purchase_order_items poi
		JOIN purchase_orders po ON po.id = poi.order_id
		WHERE poi.material_id = $1
		  AND po.status <> 'canceled'
		ORDER BY po.order_date DESC, poi.position DESC
		LIMIT 1
	`, materialID).Scan(&latestEntry.UnitPrice, &latestEntry.Currency, &latestEntry.Reference, &orderDate)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return PriceSourcePriorityEntry{}, err
	}
	if err == nil {
		latestEntry.SourceLabel = "Letzter Bestellpreis"
		latestEntry.Reference = "Bestellung " + latestEntry.Reference
		latestEntry.Date = &orderDate
		latestEntry.PriorityRank = 1
		latestEntry.PriorityReason = "Juengste konkrete Einkaufsquelle"
		return latestEntry, nil
	}

	var avgEntry PriceSourcePriorityEntry
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(m.avg_purchase_price, 0),
			COALESCE(NULLIF(BTRIM(m.currency), ''), 'EUR')
		FROM materials m
		WHERE m.id = $1
	`, materialID).Scan(&avgEntry.UnitPrice, &avgEntry.Currency); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PriceSourcePriorityEntry{}, errors.New("Material nicht gefunden")
		}
		return PriceSourcePriorityEntry{}, err
	}
	avgEntry.SourceLabel = "Durchschnittlicher Einkaufspreis"
	avgEntry.PriorityRank = 1
	avgEntry.PriorityReason = "Fallback auf Materialdurchschnitt"
	return avgEntry, nil
}

func (s *Service) PriceSourcePriorityForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) ([]PriceSourcePriorityEntry, error) {
	history, err := s.PriceHistoryForQuoteItem(ctx, quoteID, itemID)
	if err != nil {
		return nil, err
	}

	prioritized := make([]PriceSourcePriorityEntry, 0, len(history))
	appendMatching := func(sourceLabel, reason string) {
		for _, entry := range history {
			if entry.SourceLabel != sourceLabel {
				continue
			}
			prioritized = append(prioritized, PriceSourcePriorityEntry{
				SourceLabel:    entry.SourceLabel,
				UnitPrice:      entry.UnitPrice,
				Currency:       entry.Currency,
				Reference:      entry.Reference,
				Date:           entry.Date,
				PriorityRank:   len(prioritized) + 1,
				PriorityReason: reason,
			})
		}
	}

	appendMatching("Letzter Bestellpreis", "Juengste konkrete Einkaufsquelle")
	appendMatching("Durchschnittlicher Einkaufspreis", "Fallback auf Materialdurchschnitt")
	for _, entry := range history {
		if entry.SourceLabel == "Letzter Bestellpreis" || entry.SourceLabel == "Durchschnittlicher Einkaufspreis" {
			continue
		}
		prioritized = append(prioritized, PriceSourcePriorityEntry{
			SourceLabel:    entry.SourceLabel,
			UnitPrice:      entry.UnitPrice,
			Currency:       entry.Currency,
			Reference:      entry.Reference,
			Date:           entry.Date,
			PriorityRank:   len(prioritized) + 1,
			PriorityReason: "Weitere sichtbare Preisquelle",
		})
	}

	return prioritized, nil
}

func (s *Service) PriceEvaluationForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*PriceEvaluation, error) {
	priority, err := s.PriceSourcePriorityForQuoteItem(ctx, quoteID, itemID)
	if err != nil {
		return nil, err
	}
	if len(priority) == 0 {
		return nil, errors.New("keine priorisierte Preisquelle gefunden")
	}

	primary := priority[0]
	for _, entry := range priority {
		if entry.PriorityRank == 1 {
			primary = entry
			break
		}
	}

	var currentUnitPrice float64
	var quoteCurrency string
	if err := s.pg.QueryRow(ctx, `
		SELECT
			qi.unit_price,
			COALESCE(NULLIF(BTRIM(q.currency), ''), 'EUR')
		FROM quotes q
		JOIN quote_items qi ON qi.quote_id = q.id
		WHERE q.id = $1
		  AND qi.id = $2
	`, quoteID, itemID).Scan(&currentUnitPrice, &quoteCurrency); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Angebotsposition nicht gefunden")
		}
		return nil, err
	}

	absoluteDelta := currentUnitPrice - primary.UnitPrice
	var relativeDeltaPercent *float64
	if primary.UnitPrice != 0 {
		relative := absoluteDelta / primary.UnitPrice * 100
		relativeDeltaPercent = &relative
	}

	status := "at_cost_basis"
	reason := "Aktueller Preis liegt auf der primaeren Kostenbasis"
	const centTolerance = 0.005
	if absoluteDelta < -centTolerance {
		status = "below_cost_basis"
		reason = "Aktueller Preis liegt unter der primaeren Kostenbasis"
	} else if absoluteDelta > centTolerance {
		status = "above_cost_basis"
		reason = "Aktueller Preis liegt ueber der primaeren Kostenbasis"
	}

	currency := strings.TrimSpace(primary.Currency)
	if currency == "" {
		currency = quoteCurrency
	}

	return &PriceEvaluation{
		CurrentUnitPrice:       currentUnitPrice,
		Currency:               currency,
		PrimarySourceLabel:     primary.SourceLabel,
		PrimarySourceUnitPrice: primary.UnitPrice,
		PrimarySourceReference: primary.Reference,
		PrimarySourceDate:      primary.Date,
		AbsoluteDelta:          absoluteDelta,
		RelativeDeltaPercent:   relativeDeltaPercent,
		EvaluationStatus:       status,
		EvaluationReason:       reason,
	}, nil
}

func (s *Service) PriceDecisionTransparencyForQuoteItem(ctx context.Context, quoteID, itemID uuid.UUID) (*PriceDecisionTransparency, error) {
	evaluation, err := s.PriceEvaluationForQuoteItem(ctx, quoteID, itemID)
	if err != nil {
		return nil, err
	}

	status := "matches_primary_source"
	reason := "Aktueller Preis entspricht der primaeren Preisquelle"
	if evaluation.EvaluationStatus != "at_cost_basis" {
		status = "differs_from_primary_source"
		reason = "Aktueller Preis weicht von der primaeren Preisquelle ab"
	}

	return &PriceDecisionTransparency{
		CurrentUnitPrice:       evaluation.CurrentUnitPrice,
		Currency:               evaluation.Currency,
		PrimarySourceLabel:     evaluation.PrimarySourceLabel,
		PrimarySourceUnitPrice: evaluation.PrimarySourceUnitPrice,
		PrimarySourceReference: evaluation.PrimarySourceReference,
		PrimarySourceDate:      evaluation.PrimarySourceDate,
		AbsoluteDelta:          evaluation.AbsoluteDelta,
		RelativeDeltaPercent:   evaluation.RelativeDeltaPercent,
		DecisionStatus:         status,
		DecisionReason:         reason,
	}, nil
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
	if status == "sent" || status == "accepted" {
		hasOpenApprovalRework, err := quoteHasOpenApprovalRework(ctx, s.pg, id)
		if err != nil {
			return nil, err
		}
		if hasOpenApprovalRework {
			return nil, errors.New("Angebot enthaelt abgelehnte Freigabeentscheidungen; Nacharbeit vor Versand, Annahme oder Folgebeleg erforderlich")
		}
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

func quoteHasOpenApprovalRework(ctx context.Context, q quoteApprovalReworkQuerier, quoteID uuid.UUID) (bool, error) {
	var hasOpenRework bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM quote_items qi
			JOIN LATERAL (
				SELECT qar.status
				FROM quote_item_approval_requests qar
				WHERE qar.quote_item_id = qi.id
				  AND qar.status IN ('approved', 'rejected', 'rework_resolved')
				ORDER BY qar.decided_at DESC NULLS LAST, qar.updated_at DESC
				LIMIT 1
			) latest ON true
			WHERE qi.quote_id = $1
			  AND latest.status = 'rejected'
		)
	`, quoteID).Scan(&hasOpenRework); err != nil {
		return false, err
	}
	return hasOpenRework, nil
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
	hasOpenApprovalRework, err := quoteHasOpenApprovalRework(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if hasOpenApprovalRework {
		return nil, errors.New("Angebot enthaelt abgelehnte Freigabeentscheidungen; Nacharbeit vor Versand, Annahme oder Folgebeleg erforderlich")
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

	var activeApprovalRequestCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM quote_item_approval_requests
		WHERE quote_id = $1
		  AND status = 'requested'
	`, id).Scan(&activeApprovalRequestCount); err != nil {
		return nil, err
	}
	if activeApprovalRequestCount > 0 {
		return nil, errors.New("Aktive Freigabeanforderungen muessen vor dem Speichern storniert werden")
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
