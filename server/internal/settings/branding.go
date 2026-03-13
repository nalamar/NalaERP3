package settings

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BrandingSettings struct {
	ID                 string    `json:"id"`
	DisplayName        string    `json:"display_name"`
	Claim              string    `json:"claim"`
	PrimaryColor       string    `json:"primary_color"`
	AccentColor        string    `json:"accent_color"`
	DocumentHeaderText string    `json:"document_header_text"`
	DocumentFooterText string    `json:"document_footer_text"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type BrandingService struct{ pg *pgxpool.Pool }

func NewBrandingService(pg *pgxpool.Pool) *BrandingService {
	return &BrandingService{pg: pg}
}

func (s *BrandingService) Get(ctx context.Context) (*BrandingSettings, error) {
	var out BrandingSettings
	err := s.pg.QueryRow(ctx, `
        SELECT id, display_name, claim, primary_color, accent_color, document_header_text, document_footer_text, updated_at
        FROM company_branding_settings
        WHERE id='default'
    `).Scan(
		&out.ID,
		&out.DisplayName,
		&out.Claim,
		&out.PrimaryColor,
		&out.AccentColor,
		&out.DocumentHeaderText,
		&out.DocumentFooterText,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *BrandingService) Upsert(ctx context.Context, in BrandingSettings) error {
	in.DisplayName = trim(in.DisplayName)
	in.Claim = trim(in.Claim)
	in.DocumentHeaderText = trim(in.DocumentHeaderText)
	in.DocumentFooterText = trim(in.DocumentFooterText)
	in.PrimaryColor = normalizeHexColor(in.PrimaryColor, "#1F4B99")
	in.AccentColor = normalizeHexColor(in.AccentColor, "#6B7280")
	if in.DisplayName == "" {
		return errors.New("Brand-Name erforderlich")
	}

	_, err := s.pg.Exec(ctx, `
        INSERT INTO company_branding_settings (
            id, display_name, claim, primary_color, accent_color, document_header_text, document_footer_text, updated_at
        ) VALUES (
            'default', $1, $2, $3, $4, $5, $6, now()
        )
        ON CONFLICT (id) DO UPDATE SET
            display_name=EXCLUDED.display_name,
            claim=EXCLUDED.claim,
            primary_color=EXCLUDED.primary_color,
            accent_color=EXCLUDED.accent_color,
            document_header_text=EXCLUDED.document_header_text,
            document_footer_text=EXCLUDED.document_footer_text,
            updated_at=now()
    `, in.DisplayName, in.Claim, in.PrimaryColor, in.AccentColor, in.DocumentHeaderText, in.DocumentFooterText)
	return err
}

func normalizeHexColor(s string, fallback string) string {
	s = strings.ToUpper(trim(s))
	if s == "" {
		return fallback
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	if len(s) != 7 {
		return fallback
	}
	for _, r := range s[1:] {
		if !strings.ContainsRune("0123456789ABCDEF", r) {
			return fallback
		}
	}
	return s
}

func ApplyBrandingDefaults(t PDFTemplate, b *BrandingSettings) PDFTemplate {
	if b == nil {
		return t
	}

	if trim(t.HeaderText) == "" {
		if trim(b.DocumentHeaderText) != "" {
			t.HeaderText = trim(b.DocumentHeaderText)
		} else {
			headerParts := make([]string, 0, 2)
			if trim(b.DisplayName) != "" {
				headerParts = append(headerParts, trim(b.DisplayName))
			}
			if trim(b.Claim) != "" {
				headerParts = append(headerParts, trim(b.Claim))
			}
			t.HeaderText = strings.Join(headerParts, "\n")
		}
	}
	if trim(t.FooterText) == "" && trim(b.DocumentFooterText) != "" {
		t.FooterText = trim(b.DocumentFooterText)
	}
	return t
}
