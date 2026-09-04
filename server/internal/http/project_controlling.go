package apihttp

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/projects"
	"nalaerp3/internal/quotes"
)

// Projektcontrolling / Soll-Ist im Server (ADR 0020). Erlös-Seite
// wiederverwendet dieselben Bausteine wie buildProjectCommercialContext
// (quoteSvc.List/listProjectInvoices), Kosten-Seite ist neu: Soll aus
// quote_item_calculations (dieselbe, seit B.3.3 stabile Formel als
// SQL-Ausdruck statt Export einer privaten quotes-Funktion), Ist aus
// journal_lines gefiltert auf die Projekt-Kostenstelle und
// Aufwandskonten. ist_kosten/deckungsbeitrag_ist bleiben nil, wenn dem
// Projekt keine Kostenstelle zugeordnet ist (siehe ADR 0020 - "nicht
// messbar" statt fälschlich "0").

type projectControllingResponse struct {
	ProjectID           string   `json:"project_id"`
	SollErloes          float64  `json:"soll_erloes"`
	IstErloes           float64  `json:"ist_erloes"`
	SollKosten          float64  `json:"soll_kosten"`
	IstKosten           *float64 `json:"ist_kosten"`
	DeckungsbeitragSoll float64  `json:"deckungsbeitrag_soll"`
	DeckungsbeitragIst  *float64 `json:"deckungsbeitrag_ist"`
}

func buildProjectControlling(
	ctx context.Context,
	projectID string,
	pg *pgxpool.Pool,
	projSvc *projects.Service,
	quoteSvc *quotes.Service,
	companyID string,
) (*projectControllingResponse, error) {
	project, err := projSvc.Get(ctx, projectID, companyID)
	if err != nil {
		return nil, err
	}

	quotesList, err := quoteSvc.List(ctx, quotes.QuoteFilter{
		ProjectID: projectID,
		Limit:     200,
	}, companyID)
	if err != nil {
		return nil, err
	}
	var sollErloes float64
	for _, item := range quotesList {
		sollErloes += item.GrossAmount
	}

	invoices, err := listProjectInvoices(ctx, pg, projectID, 200)
	if err != nil {
		return nil, err
	}
	var istErloes float64
	for _, item := range invoices {
		istErloes += item.GrossAmount
	}

	var sollKosten float64
	if err := pg.QueryRow(ctx, `
        SELECT COALESCE(SUM(
            (qic.material_cost * (1 + qic.material_zuschlag_percent/100)
             + qic.lohn_stunden * qic.lohn_stundensatz * (1 + qic.lohn_zuschlag_percent/100)
             + qic.fremdleistung_cost * (1 + qic.fremdleistung_zuschlag_percent/100)
            ) * qi.qty
        ), 0)
        FROM quote_item_calculations qic
        JOIN quote_items qi ON qi.id = qic.quote_item_id
        JOIN quotes q ON q.id = qi.quote_id
        WHERE q.project_id = $1 AND q.company_id = $2 AND q.superseded_by_quote_id IS NULL
    `, projectID, companyID).Scan(&sollKosten); err != nil {
		return nil, err
	}

	var istKosten *float64
	if project.KostenstelleID != nil {
		var ist float64
		if err := pg.QueryRow(ctx, `
            SELECT COALESCE(SUM(jl.debit - jl.credit), 0)
            FROM journal_lines jl
            JOIN accounts a ON a.code = jl.account_code AND a.company_id = jl.company_id
            WHERE jl.kostenstelle_id = $1 AND jl.company_id = $2 AND a.type = 'expense'
        `, *project.KostenstelleID, companyID).Scan(&ist); err != nil {
			return nil, err
		}
		istKosten = &ist
	}

	deckungsbeitragSoll := sollErloes - sollKosten
	var deckungsbeitragIst *float64
	if istKosten != nil {
		d := istErloes - *istKosten
		deckungsbeitragIst = &d
	}

	return &projectControllingResponse{
		ProjectID:           projectID,
		SollErloes:          sollErloes,
		IstErloes:           istErloes,
		SollKosten:          sollKosten,
		IstKosten:           istKosten,
		DeckungsbeitragSoll: deckungsbeitragSoll,
		DeckungsbeitragIst:  deckungsbeitragIst,
	}, nil
}
