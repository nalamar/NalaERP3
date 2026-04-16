package apihttp

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/sales"
)

type commercialWorkflowFilter struct {
	ProjectID string
	ContactID string
	Kind      string
}

type commercialWorkflowResponse struct {
	Items []commercialWorkflowItem `json:"items"`
}

type commercialWorkflowItem struct {
	Kind             string    `json:"kind"`
	Stage            string    `json:"stage"`
	Priority         string    `json:"priority"`
	ProjectID        string    `json:"project_id,omitempty"`
	ProjectName      string    `json:"project_name,omitempty"`
	ContactID        string    `json:"contact_id,omitempty"`
	ContactName      string    `json:"contact_name,omitempty"`
	QuoteID          string    `json:"quote_id,omitempty"`
	QuoteNumber      string    `json:"quote_number,omitempty"`
	SalesOrderID     string    `json:"sales_order_id,omitempty"`
	SalesOrderNumber string    `json:"sales_order_number,omitempty"`
	InvoiceID        string    `json:"invoice_id,omitempty"`
	InvoiceNumber    string    `json:"invoice_number,omitempty"`
	DocumentDate     time.Time `json:"document_date"`
	Status           string    `json:"status"`
	GrossTotal       float64   `json:"gross_total"`
	OpenGrossTotal   float64   `json:"open_gross_total"`
	NextActionLabel  string    `json:"next_action_label"`
}

func buildCommercialWorkflow(
	ctx context.Context,
	_ *pgxpool.Pool,
	quoteSvc *quotes.Service,
	salesSvc *sales.Service,
	filter commercialWorkflowFilter,
) (*commercialWorkflowResponse, error) {
	items := make([]commercialWorkflowItem, 0)
	kind := strings.TrimSpace(filter.Kind)

	if kind == "" || kind == "quote_sent_pending" || kind == "quote_accepted_pending_followup" {
		sentQuotes, err := quoteSvc.List(ctx, quotes.QuoteFilter{
			Status:    "sent",
			ProjectID: filter.ProjectID,
			ContactID: filter.ContactID,
			Limit:     200,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range sentQuotes {
			if item.SupersededByQuoteID != "" || item.LinkedSalesOrderID != "" || item.LinkedInvoiceOutID != "" {
				continue
			}
			items = append(items, commercialWorkflowItem{
				Kind:            "quote_sent_pending",
				Stage:           "quote",
				Priority:        "normal",
				ProjectID:       item.ProjectID,
				ProjectName:     item.ProjectName,
				ContactID:       item.ContactID,
				ContactName:     item.ContactName,
				QuoteID:         item.ID.String(),
				QuoteNumber:     item.Number,
				DocumentDate:    item.QuoteDate,
				Status:          item.Status,
				GrossTotal:      item.GrossAmount,
				OpenGrossTotal:  item.GrossAmount,
				NextActionLabel: "Entscheidung ausstehend",
			})
		}

		acceptedQuotes, err := quoteSvc.List(ctx, quotes.QuoteFilter{
			Status:    "accepted",
			ProjectID: filter.ProjectID,
			ContactID: filter.ContactID,
			Limit:     200,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range acceptedQuotes {
			if item.SupersededByQuoteID != "" || item.LinkedSalesOrderID != "" || item.LinkedInvoiceOutID != "" {
				continue
			}
			items = append(items, commercialWorkflowItem{
				Kind:            "quote_accepted_pending_followup",
				Stage:           "quote",
				Priority:        "high",
				ProjectID:       item.ProjectID,
				ProjectName:     item.ProjectName,
				ContactID:       item.ContactID,
				ContactName:     item.ContactName,
				QuoteID:         item.ID.String(),
				QuoteNumber:     item.Number,
				DocumentDate:    item.QuoteDate,
				Status:          item.Status,
				GrossTotal:      item.GrossAmount,
				OpenGrossTotal:  item.GrossAmount,
				NextActionLabel: "Folgebeleg erzeugen",
			})
		}
	}

	if kind == "" || kind == "sales_order_pending_invoice" || kind == "sales_order_partially_invoiced" {
		salesOrders, err := salesSvc.List(ctx, sales.SalesOrderFilter{
			ProjectID: filter.ProjectID,
			ContactID: filter.ContactID,
			Limit:     200,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range salesOrders {
			if item.Status == "completed" || item.Status == "canceled" {
				continue
			}
			if item.RelatedInvoiceCount == 0 && item.RemainingGrossAmount > 0.0001 {
				items = append(items, commercialWorkflowItem{
					Kind:             "sales_order_pending_invoice",
					Stage:            "sales_order",
					Priority:         "high",
					ProjectID:        item.ProjectID,
					ProjectName:      item.ProjectName,
					ContactID:        item.ContactID,
					ContactName:      item.ContactName,
					QuoteID:          item.SourceQuoteID.String(),
					SalesOrderID:     item.ID.String(),
					SalesOrderNumber: item.Number,
					DocumentDate:     item.OrderDate,
					Status:           item.Status,
					GrossTotal:       item.GrossAmount,
					OpenGrossTotal:   item.RemainingGrossAmount,
					NextActionLabel:  "Rechnung erzeugen",
				})
				continue
			}
			if item.RelatedInvoiceCount > 0 && item.RemainingGrossAmount > 0.0001 {
				items = append(items, commercialWorkflowItem{
					Kind:             "sales_order_partially_invoiced",
					Stage:            "sales_order",
					Priority:         "normal",
					ProjectID:        item.ProjectID,
					ProjectName:      item.ProjectName,
					ContactID:        item.ContactID,
					ContactName:      item.ContactName,
					QuoteID:          item.SourceQuoteID.String(),
					SalesOrderID:     item.ID.String(),
					SalesOrderNumber: item.Number,
					DocumentDate:     item.OrderDate,
					Status:           item.Status,
					GrossTotal:       item.GrossAmount,
					OpenGrossTotal:   item.RemainingGrossAmount,
					NextActionLabel:  "Restbetrag fakturieren",
				})
			}
		}
	}

	if kind != "" {
		filtered := make([]commercialWorkflowItem, 0, len(items))
		for _, item := range items {
			if item.Kind == kind {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	sort.SliceStable(items, func(i, j int) bool {
		if workflowPriorityRank(items[i].Priority) != workflowPriorityRank(items[j].Priority) {
			return workflowPriorityRank(items[i].Priority) < workflowPriorityRank(items[j].Priority)
		}
		return items[i].DocumentDate.After(items[j].DocumentDate)
	})

	return &commercialWorkflowResponse{Items: items}, nil
}

func workflowPriorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 0
	default:
		return 1
	}
}
