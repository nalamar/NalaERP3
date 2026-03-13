package settings

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Account struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	ParentCode *string `json:"parent_code,omitempty"`
	TaxCode    *string `json:"tax_code,omitempty"`
	Active     bool    `json:"active"`
}

type TaxCode struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Rate          float64 `json:"rate"`
	Country       string  `json:"country"`
	Kind          string  `json:"kind"`
	ReverseCharge bool    `json:"reverse_charge"`
	Active        bool    `json:"active"`
}

type AccountingService struct{ pg *pgxpool.Pool }

func NewAccountingService(pg *pgxpool.Pool) *AccountingService { return &AccountingService{pg: pg} }

func (s *AccountingService) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.pg.Query(ctx, `SELECT code, name, type, parent_code, tax_code, is_active FROM accounts ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Account, 0)
	for rows.Next() {
		var a Account
		var parent, tax sql.NullString
		if err := rows.Scan(&a.Code, &a.Name, &a.Type, &parent, &tax, &a.Active); err != nil {
			return nil, err
		}
		if parent.Valid {
			a.ParentCode = &parent.String
		}
		if tax.Valid {
			a.TaxCode = &tax.String
		}
		list = append(list, a)
	}
	return list, nil
}

func (s *AccountingService) ListTaxCodes(ctx context.Context) ([]TaxCode, error) {
	rows, err := s.pg.Query(ctx, `SELECT code, name, rate, country, kind, reverse_charge, is_active FROM tax_codes ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]TaxCode, 0)
	for rows.Next() {
		var t TaxCode
		if err := rows.Scan(&t.Code, &t.Name, &t.Rate, &t.Country, &t.Kind, &t.ReverseCharge, &t.Active); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}
