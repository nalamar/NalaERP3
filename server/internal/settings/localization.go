package settings

import (
    "context"
    "errors"
    "strings"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type LocalizationSettings struct {
    ID              string    `json:"id"`
    DefaultCurrency string    `json:"default_currency"`
    TaxCountry      string    `json:"tax_country"`
    StandardVATRate float64   `json:"standard_vat_rate"`
    Locale          string    `json:"locale"`
    Timezone        string    `json:"timezone"`
    DateFormat      string    `json:"date_format"`
    NumberFormat    string    `json:"number_format"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type LocalizationService struct{ pg *pgxpool.Pool }

func NewLocalizationService(pg *pgxpool.Pool) *LocalizationService {
    return &LocalizationService{pg: pg}
}

func (s *LocalizationService) Get(ctx context.Context) (*LocalizationSettings, error) {
    var out LocalizationSettings
    err := s.pg.QueryRow(ctx, `
        SELECT id, default_currency, tax_country, standard_vat_rate, locale, timezone, date_format, number_format, updated_at
        FROM company_localization_settings
        WHERE id='default'
    `).Scan(
        &out.ID,
        &out.DefaultCurrency,
        &out.TaxCountry,
        &out.StandardVATRate,
        &out.Locale,
        &out.Timezone,
        &out.DateFormat,
        &out.NumberFormat,
        &out.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    return &out, nil
}

func (s *LocalizationService) Upsert(ctx context.Context, in LocalizationSettings) error {
    in.DefaultCurrency = normalizeCurrency(in.DefaultCurrency)
    in.TaxCountry = normalizeCountry(in.TaxCountry)
    in.Locale = normalizeLocale(in.Locale)
    in.Timezone = normalizeTimezone(in.Timezone)
    in.DateFormat = normalizeDateFormat(in.DateFormat)
    in.NumberFormat = normalizeLocale(in.NumberFormat)
    if in.StandardVATRate < 0 {
        return errors.New("Steuersatz ungültig")
    }

    _, err := s.pg.Exec(ctx, `
        INSERT INTO company_localization_settings (
            id, default_currency, tax_country, standard_vat_rate, locale, timezone, date_format, number_format, updated_at
        ) VALUES (
            'default', $1, $2, $3, $4, $5, $6, $7, now()
        )
        ON CONFLICT (id) DO UPDATE SET
            default_currency=EXCLUDED.default_currency,
            tax_country=EXCLUDED.tax_country,
            standard_vat_rate=EXCLUDED.standard_vat_rate,
            locale=EXCLUDED.locale,
            timezone=EXCLUDED.timezone,
            date_format=EXCLUDED.date_format,
            number_format=EXCLUDED.number_format,
            updated_at=now()
    `, in.DefaultCurrency, in.TaxCountry, in.StandardVATRate, in.Locale, in.Timezone, in.DateFormat, in.NumberFormat)
    return err
}

func normalizeCurrency(s string) string {
    s = strings.ToUpper(trim(s))
    if s == "" {
        return "EUR"
    }
    return s
}

func normalizeLocale(s string) string {
    s = trim(s)
    if s == "" {
        return "de-DE"
    }
    return s
}

func normalizeTimezone(s string) string {
    s = trim(s)
    if s == "" {
        return "Europe/Berlin"
    }
    return s
}

func normalizeDateFormat(s string) string {
    s = trim(s)
    if s == "" {
        return "dd.MM.yyyy"
    }
    return s
}
