package settings

import (
    "context"
    "errors"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type PDFTemplate struct {
    Entity       string   `json:"entity"`
    HeaderText   string   `json:"header_text"`
    FooterText   string   `json:"footer_text"`
    LogoDocID    *string  `json:"logo_doc_id"`
    BgFirstDocID *string  `json:"bg_first_doc_id"`
    BgOtherDocID *string  `json:"bg_other_doc_id"`
    TopFirstMM   float64  `json:"top_first_mm"`
    TopOtherMM   float64  `json:"top_other_mm"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type PDFService struct { pg *pgxpool.Pool }
func NewPDFService(pg *pgxpool.Pool) *PDFService { return &PDFService{pg: pg} }

func (s *PDFService) Get(ctx context.Context, entity string) (*PDFTemplate, error) {
    var t PDFTemplate
    var logo, bg1, bg2 *string
    err := s.pg.QueryRow(ctx, `
        SELECT entity, header_text, footer_text, logo_doc_id, bg_first_doc_id, bg_other_doc_id, COALESCE(top_first_mm,30), COALESCE(top_other_mm,20), updated_at
        FROM pdf_templates WHERE entity=$1
    `, entity).Scan(&t.Entity, &t.HeaderText, &t.FooterText, &logo, &bg1, &bg2, &t.TopFirstMM, &t.TopOtherMM, &t.UpdatedAt)
    if err != nil { return nil, err }
    t.LogoDocID, t.BgFirstDocID, t.BgOtherDocID = logo, bg1, bg2
    return &t, nil
}

func (s *PDFService) Upsert(ctx context.Context, in PDFTemplate) error {
    if in.Entity == "" { return errors.New("entity erforderlich") }
    _, err := s.pg.Exec(ctx, `
        INSERT INTO pdf_templates (entity, header_text, footer_text, top_first_mm, top_other_mm, updated_at)
        VALUES ($1,$2,$3,$4,$5,now())
        ON CONFLICT (entity) DO UPDATE SET header_text=EXCLUDED.header_text, footer_text=EXCLUDED.footer_text, top_first_mm=EXCLUDED.top_first_mm, top_other_mm=EXCLUDED.top_other_mm, updated_at=now()
    `, in.Entity, in.HeaderText, in.FooterText, in.TopFirstMM, in.TopOtherMM)
    return err
}

func (s *PDFService) SetImage(ctx context.Context, entity string, kind string, docID *string) error {
    col := ""
    switch kind {
    case "logo": col = "logo_doc_id"
    case "bg_first": col = "bg_first_doc_id"
    case "bg_other": col = "bg_other_doc_id"
    default: return errors.New("ungültiger Typ")
    }
    _, err := s.pg.Exec(ctx, `UPDATE pdf_templates SET `+col+`=$2, updated_at=now() WHERE entity=$1`, entity, docID)
    return err
}

