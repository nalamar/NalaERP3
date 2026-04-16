package apihttp

import (
	"context"
	"database/sql"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/accounting"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/sales"
)

type commercialContextStats struct {
	QuoteCount           int     `json:"quote_count"`
	SalesOrderCount      int     `json:"sales_order_count"`
	InvoiceCount         int     `json:"invoice_count"`
	OpenInvoiceCount     int     `json:"open_invoice_count"`
	QuoteGrossTotal      float64 `json:"quote_gross_total"`
	SalesOrderGrossTotal float64 `json:"sales_order_gross_total"`
	InvoiceGrossTotal    float64 `json:"invoice_gross_total"`
	InvoiceOpenTotal     float64 `json:"invoice_open_total"`
}

type contactCommercialContextResponse struct {
	ContactID   string                       `json:"contact_id"`
	Quotes      []quotes.QuoteListItem       `json:"quotes"`
	SalesOrders []sales.SalesOrderListItem   `json:"sales_orders"`
	InvoicesOut []accounting.InvoiceListItem `json:"invoices_out"`
	Stats       commercialContextStats       `json:"stats"`
}

type projectCommercialContextResponse struct {
	ProjectID   string                       `json:"project_id"`
	Quotes      []quotes.QuoteListItem       `json:"quotes"`
	SalesOrders []sales.SalesOrderListItem   `json:"sales_orders"`
	InvoicesOut []accounting.InvoiceListItem `json:"invoices_out"`
	Stats       commercialContextStats       `json:"stats"`
}

func buildContactCommercialContext(
	ctx context.Context,
	contactID string,
	quoteSvc *quotes.Service,
	salesSvc *sales.Service,
	arSvc *accounting.ARService,
) (*contactCommercialContextResponse, error) {
	quotesList, err := quoteSvc.List(ctx, quotes.QuoteFilter{
		ContactID: contactID,
		Limit:     200,
	})
	if err != nil {
		return nil, err
	}

	salesOrders, err := salesSvc.List(ctx, sales.SalesOrderFilter{
		ContactID: contactID,
		Limit:     200,
	})
	if err != nil {
		return nil, err
	}

	invoices, err := arSvc.List(ctx, accounting.InvoiceFilter{
		ContactID: contactID,
		Limit:     200,
	})
	if err != nil {
		return nil, err
	}

	stats := commercialContextStats{
		QuoteCount:      len(quotesList),
		SalesOrderCount: len(salesOrders),
		InvoiceCount:    len(invoices),
	}

	for _, item := range quotesList {
		stats.QuoteGrossTotal += item.GrossAmount
	}
	for _, item := range salesOrders {
		stats.SalesOrderGrossTotal += item.GrossAmount
	}
	for _, item := range invoices {
		stats.InvoiceGrossTotal += item.GrossAmount
		openAmount := item.GrossAmount - item.PaidAmount
		if openAmount > 0.0001 {
			stats.OpenInvoiceCount++
			stats.InvoiceOpenTotal += math.Max(openAmount, 0)
		}
	}

	return &contactCommercialContextResponse{
		ContactID:   contactID,
		Quotes:      quotesList,
		SalesOrders: salesOrders,
		InvoicesOut: invoices,
		Stats:       stats,
	}, nil
}

func buildProjectCommercialContext(
	ctx context.Context,
	projectID string,
	pg *pgxpool.Pool,
	quoteSvc *quotes.Service,
	salesSvc *sales.Service,
) (*projectCommercialContextResponse, error) {
	quotesList, err := quoteSvc.List(ctx, quotes.QuoteFilter{
		ProjectID: projectID,
		Limit:     200,
	})
	if err != nil {
		return nil, err
	}

	salesOrders, err := salesSvc.List(ctx, sales.SalesOrderFilter{
		ProjectID: projectID,
		Limit:     200,
	})
	if err != nil {
		return nil, err
	}

	invoices, err := listProjectInvoices(ctx, pg, projectID, 200)
	if err != nil {
		return nil, err
	}

	stats := commercialContextStats{
		QuoteCount:      len(quotesList),
		SalesOrderCount: len(salesOrders),
		InvoiceCount:    len(invoices),
	}

	for _, item := range quotesList {
		stats.QuoteGrossTotal += item.GrossAmount
	}
	for _, item := range salesOrders {
		stats.SalesOrderGrossTotal += item.GrossAmount
	}
	for _, item := range invoices {
		stats.InvoiceGrossTotal += item.GrossAmount
		openAmount := item.GrossAmount - item.PaidAmount
		if openAmount > 0.0001 {
			stats.OpenInvoiceCount++
			stats.InvoiceOpenTotal += math.Max(openAmount, 0)
		}
	}

	return &projectCommercialContextResponse{
		ProjectID:   projectID,
		Quotes:      quotesList,
		SalesOrders: salesOrders,
		InvoicesOut: invoices,
		Stats:       stats,
	}, nil
}

func listProjectInvoices(
	ctx context.Context,
	pg *pgxpool.Pool,
	projectID string,
	limit int,
) ([]accounting.InvoiceListItem, error) {
	rows, err := pg.Query(ctx, `
SELECT DISTINCT
	i.id,
	i.nummer,
	i.status,
	i.source_quote_id,
	i.source_sales_order_id,
	i.contact_id,
	COALESCE(c.name,''),
	i.invoice_date,
	i.due_date,
	i.currency,
	i.gross_amount,
	i.paid_amount
FROM invoices_out i
LEFT JOIN contacts c ON c.id = i.contact_id
LEFT JOIN quotes q ON q.id = i.source_quote_id
LEFT JOIN sales_orders so ON so.id = i.source_sales_order_id
WHERE q.project_id = $1 OR so.project_id = $1
ORDER BY i.invoice_date DESC, i.created_at DESC
LIMIT $2
`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]accounting.InvoiceListItem, 0)
	for rows.Next() {
		var item accounting.InvoiceListItem
		var number sql.NullString
		var due sql.NullTime
		var sourceQuoteID uuid.NullUUID
		var sourceSalesOrderID uuid.NullUUID
		if err := rows.Scan(
			&item.ID,
			&number,
			&item.Status,
			&sourceQuoteID,
			&sourceSalesOrderID,
			&item.ContactID,
			&item.ContactName,
			&item.InvoiceDate,
			&due,
			&item.Currency,
			&item.GrossAmount,
			&item.PaidAmount,
		); err != nil {
			return nil, err
		}
		if number.Valid {
			item.Number = &number.String
		}
		if sourceQuoteID.Valid {
			item.SourceQuoteID = &sourceQuoteID.UUID
		}
		if sourceSalesOrderID.Valid {
			item.SourceSalesOrderID = &sourceSalesOrderID.UUID
		}
		if due.Valid {
			t := due.Time
			item.DueDate = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
