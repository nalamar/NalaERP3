package quotes

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"nalaerp3/internal/auditlog"
)

type QuoteImport struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	ContactID        string    `json:"contact_id,omitempty"`
	SourceKind       string    `json:"source_kind"`
	SourceFilename   string    `json:"source_filename"`
	SourceDocumentID string    `json:"source_document_id"`
	Status           string    `json:"status"`
	ParserVersion    string    `json:"parser_version"`
	DetectedFormat   string    `json:"detected_format"`
	ErrorMessage     string    `json:"error_message"`
	CreatedQuoteID   string    `json:"created_quote_id,omitempty"`
	ItemCount        int       `json:"item_count"`
	AcceptedCount    int       `json:"accepted_count"`
	RejectedCount    int       `json:"rejected_count"`
	PendingCount     int       `json:"pending_count"`
	UploadedAt       time.Time `json:"uploaded_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type QuoteImportItem struct {
	ID                string  `json:"id"`
	ImportID          string  `json:"import_id"`
	PositionNo        string  `json:"position_no"`
	OutlineNo         string  `json:"outline_no"`
	Description       string  `json:"description"`
	Qty               float64 `json:"qty"`
	Unit              string  `json:"unit"`
	IsOptional        bool    `json:"is_optional"`
	ParserHint        string  `json:"parser_hint"`
	ReviewStatus      string  `json:"review_status"`
	ReviewNote        string  `json:"review_note"`
	SortOrder         int     `json:"sort_order"`
	LinkedQuoteID     string  `json:"linked_quote_id,omitempty"`
	LinkedQuoteItemID string  `json:"linked_quote_item_id,omitempty"`
	LinkedQuotePos    int     `json:"linked_quote_position,omitempty"`
	ShortText         string  `json:"short_text"`
	LongText          string  `json:"long_text"`
	ItemKind          string  `json:"item_kind"`
	IsLumpSum         bool    `json:"is_lump_sum"`
	BasePositionNo    string  `json:"base_position_no"`
	GroupID           string  `json:"group_id,omitempty"`
}

// QuoteImportGroup ist ein Los, Titel oder Untertitel eines Importlaufs.
type QuoteImportGroup struct {
	ID            string `json:"id"`
	OutlineNo     string `json:"outline_no"`
	ParentGroupID string `json:"parent_group_id,omitempty"`
	Kind          string `json:"kind"`
	Bezeichnung   string `json:"bezeichnung"`
	SortOrder     int    `json:"sort_order"`
}

// QuoteImportStructure: Gliederung und Vorbemerkungen eines Importlaufs.
type QuoteImportStructure struct {
	ImportID       string             `json:"import_id"`
	Groups         []QuoteImportGroup `json:"groups"`
	Vorbemerkungen []string           `json:"vorbemerkungen"`
}

// QuoteImportItemInput ist eine gelesene Position. OutlineNo verweist auf
// den Bereich (QuoteImportGroupInput.OutlineNo), falls es ihn gibt.
// ItemKind leer = normal; BasePositionNo nur bei Alternativpositionen.
type QuoteImportItemInput struct {
	PositionNo     string
	OutlineNo      string
	Description    string
	Qty            float64
	Unit           string
	IsOptional     bool
	ParserHint     string
	SortOrder      int
	ShortText      string
	LongText       string
	ItemKind       string
	IsLumpSum      bool
	BasePositionNo string
}

// QuoteImportGroupInput ist ein Los, Titel oder Untertitel; Eltern vor
// Kindern, ParentOutlineNo leer auf oberster Ebene.
type QuoteImportGroupInput struct {
	OutlineNo       string
	ParentOutlineNo string
	Kind            string
	Bezeichnung     string
}

var quoteImportItemKinds = map[string]bool{"normal": true, "grundposition": true, "alternativposition": true,
	"bedarfsposition_mit_gb": true, "bedarfsposition_ohne_gb": true}

var quoteImportGroupKinds = map[string]bool{"los": true, "titel": true, "untertitel": true}

type acceptedQuoteImportApplyItem struct {
	ImportItemID  string
	Description   string
	Qty           float64
	Unit          string
	ImportGroupID string
}

// copyImportGroupsTx legt die Gliederung eines Importlaufs als
// quote_item_groups an (ADR 0041): in Reihenfolge, Eltern vor Kindern, mit
// OZ; eine leere Bezeichnung wird durch die OZ ersetzt. Auch Gruppen ohne
// uebernommene Positionen entstehen, damit die Gliederung vollstaendig
// bleibt. Liefert Import-Gruppe -> Angebotsgruppe.
func copyImportGroupsTx(ctx context.Context, tx pgx.Tx, importID string, quoteID uuid.UUID) (map[string]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, COALESCE(parent_group_id::text,''), outline_no, kind, bezeichnung, sort_order
		FROM quote_import_groups
		WHERE import_id=$1
		ORDER BY sort_order ASC, id ASC
	`, importID)
	if err != nil {
		return nil, err
	}
	type importGroup struct {
		id, parentID, outline, kind, bezeichnung string
		sortOrder                                int
	}
	var groups []importGroup
	for rows.Next() {
		var g importGroup
		if err := rows.Scan(&g.id, &g.parentID, &g.outline, &g.kind, &g.bezeichnung, &g.sortOrder); err != nil {
			rows.Close()
			return nil, err
		}
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make(map[string]uuid.UUID, len(groups))
	for _, g := range groups {
		var parentID *uuid.UUID
		if g.parentID != "" {
			id, ok := out[g.parentID]
			if !ok {
				return nil, fmt.Errorf("Gliederung %s: übergeordneter Bereich fehlt", g.outline)
			}
			parentID = &id
		}
		bezeichnung := strings.TrimSpace(g.bezeichnung)
		if bezeichnung == "" {
			bezeichnung = g.outline
		}
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO quote_item_groups (id, quote_id, parent_group_id, kind, bezeichnung, sort_order, outline_no)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, id, quoteID, parentID, g.kind, bezeichnung, g.sortOrder, g.outline); err != nil {
			return nil, err
		}
		out[g.id] = id
	}
	return out, nil
}

type QuoteImportApplyResult struct {
	Import *QuoteImport `json:"import"`
	Quote  *Quote       `json:"quote"`
}

type QuoteImportCreateInput struct {
	ProjectID string
	ContactID string
}

type GAEBImportParseResult struct {
	ParserVersion  string
	DetectedFormat string
	Items          []QuoteImportItemInput
	Groups         []QuoteImportGroupInput
	Vorbemerkungen []string
}

type GAEBImportParser interface {
	ParseGAEB(ctx context.Context, source io.Reader, filename string) (GAEBImportParseResult, error)
}

type QuoteImportFilter struct {
	ProjectID string
	ContactID string
	Limit     int
	Offset    int
}

var allowedGAEBImportExtensions = map[string]struct{}{
	".x83":  {},
	".x84":  {},
	".gaeb": {},
	".xml":  {},
}

// GAEB 90/2000 ist kein XML und wird beim Upload abgelehnt, bis es ein
// eigener Punkt ist (ADR 0040, Entscheidung 5 (a)).
var gaeb90ImportExtensions = map[string]struct{}{
	".d83": {},
	".p83": {},
}

// MaxGAEBImportBytes begrenzt eine GAEB-Datei (ADR 0040, Entscheidung 6 (a)).
const MaxGAEBImportBytes = 20 << 20

var allowedQuoteImportReviewStatuses = map[string]struct{}{
	"pending":  {},
	"accepted": {},
	"rejected": {},
}

func (s *Service) WithMongo(mg *mongo.Client, mongoDB string) *Service {
	s.mg = mg
	s.mongoDB = mongoDB
	return s
}

func (s *Service) WithGAEBImportParser(parser GAEBImportParser) *Service {
	s.gaebImportParser = parser
	return s
}

// WithAudit aktiviert das generische Aenderungsprotokoll (Subtask 0.3.3.2)
// fuer Accept/Revise/Freigabe-Entscheidungen. Ohne Aufruf bleibt s.audit
// nil und die entsprechenden Protokollaufrufe werden uebersprungen -
// wichtig fuer die vielen bestehenden Tests, die NewService(...) ohne
// WithAudit(...) konstruieren.
func (s *Service) WithAudit(audit *auditlog.Service) *Service {
	s.audit = audit
	return s
}

func (s *Service) CreateGAEBImport(ctx context.Context, in QuoteImportCreateInput, r io.Reader, filename string, companyID string) (*QuoteImport, error) {
	if s.pg == nil {
		return nil, errors.New("Postgres nicht konfiguriert")
	}
	if s.mg == nil || strings.TrimSpace(s.mongoDB) == "" {
		return nil, errors.New("MongoDB nicht konfiguriert")
	}
	if strings.TrimSpace(in.ProjectID) == "" {
		return nil, errors.New("project_id erforderlich")
	}
	if r == nil {
		return nil, errors.New("Datei erforderlich")
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, errors.New("Dateiname erforderlich")
	}
	if _, ok := gaeb90ImportExtensions[strings.ToLower(filepath.Ext(filename))]; ok {
		return nil, errors.New("GAEB 90/2000 (.d83, .p83) wird nicht unterstützt; bitte die Datei als GAEB DA XML (.x83) exportieren")
	}
	if !isAllowedGAEBImportFilename(filename) {
		return nil, errors.New("Nur GAEB-Dateien mit den Endungen .x83, .x84, .gaeb oder .xml sind zulässig")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxGAEBImportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxGAEBImportBytes {
		return nil, fmt.Errorf("GAEB-Datei ist größer als %d MB", MaxGAEBImportBytes>>20)
	}
	if len(data) == 0 {
		return nil, errors.New("GAEB-Datei ist leer")
	}

	var projectExists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND company_id=$2)`, in.ProjectID, companyID).Scan(&projectExists); err != nil {
		return nil, err
	}
	if !projectExists {
		return nil, errors.New("Projekt nicht gefunden")
	}
	in.ContactID = strings.TrimSpace(in.ContactID)
	if in.ContactID != "" {
		var contactExists bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contacts WHERE id=$1)`, in.ContactID).Scan(&contactExists); err != nil {
			return nil, err
		}
		if !contactExists {
			return nil, errors.New("Kontakt nicht gefunden")
		}
	}

	db := s.mg.Database(s.mongoDB)
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		return nil, err
	}
	oid, err := bucket.UploadFromStream(filename, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var storedFilename string
	var uploadedAt time.Time
	var fileDoc struct {
		ID         primitive.ObjectID `bson:"_id"`
		Filename   string             `bson:"filename"`
		UploadDate time.Time          `bson:"uploadDate"`
	}
	if err := db.Collection("fs.files").FindOne(ctx, bson.M{"_id": oid}).Decode(&fileDoc); err == nil {
		storedFilename = strings.TrimSpace(fileDoc.Filename)
		uploadedAt = fileDoc.UploadDate
	}
	if storedFilename == "" {
		storedFilename = filename
	}
	if uploadedAt.IsZero() {
		uploadedAt = time.Now()
	}

	id := uuid.NewString()
	_, err = s.pg.Exec(ctx, `
		INSERT INTO quote_imports (
			id, project_id, contact_id, source_kind, source_filename, source_document_id,
			status, parser_version, detected_format, error_message, created_quote_id, uploaded_at, updated_at
		) VALUES ($1,$2,$3,'gaeb',$4,$5,'uploaded','','','',NULL,$6,$6)
	`, id, in.ProjectID, nullIfEmpty(in.ContactID), storedFilename, oid.Hex(), uploadedAt)
	if err != nil {
		return nil, err
	}
	return s.GetImport(ctx, id, companyID)
}

func (s *Service) ListImports(ctx context.Context, f QuoteImportFilter, companyID string) ([]QuoteImport, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args := []any{companyID}
	conds := []string{"p.company_id=$1"}
	if strings.TrimSpace(f.ProjectID) != "" {
		args = append(args, strings.TrimSpace(f.ProjectID))
		conds = append(conds, fmt.Sprintf("quote_imports.project_id::text=$%d", len(args)))
	}
	if strings.TrimSpace(f.ContactID) != "" {
		args = append(args, strings.TrimSpace(f.ContactID))
		conds = append(conds, fmt.Sprintf("quote_imports.contact_id::text=$%d", len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pg.Query(ctx, `
		SELECT quote_imports.id::text, quote_imports.project_id::text, COALESCE(quote_imports.contact_id::text,''), quote_imports.source_kind, quote_imports.source_filename,
		       quote_imports.source_document_id, quote_imports.status, quote_imports.parser_version, quote_imports.detected_format, quote_imports.error_message,
		       COALESCE(quote_imports.created_quote_id::text,''),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='accepted'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='rejected'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='pending'), 0),
		       quote_imports.uploaded_at, quote_imports.updated_at
		FROM quote_imports
		JOIN projects p ON p.id = quote_imports.project_id`+where+`
		ORDER BY uploaded_at DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]QuoteImport, 0)
	for rows.Next() {
		var item QuoteImport
		if err := rows.Scan(
			&item.ID,
			&item.ProjectID,
			&item.ContactID,
			&item.SourceKind,
			&item.SourceFilename,
			&item.SourceDocumentID,
			&item.Status,
			&item.ParserVersion,
			&item.DetectedFormat,
			&item.ErrorMessage,
			&item.CreatedQuoteID,
			&item.ItemCount,
			&item.AcceptedCount,
			&item.RejectedCount,
			&item.PendingCount,
			&item.UploadedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) GetImport(ctx context.Context, id string, companyID string) (*QuoteImport, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("id erforderlich")
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, errors.New("ungültige id")
	}

	var item QuoteImport
	err := s.pg.QueryRow(ctx, `
		SELECT quote_imports.id::text, quote_imports.project_id::text, COALESCE(quote_imports.contact_id::text,''), quote_imports.source_kind, quote_imports.source_filename,
		       quote_imports.source_document_id, quote_imports.status, quote_imports.parser_version, quote_imports.detected_format, quote_imports.error_message,
		       COALESCE(quote_imports.created_quote_id::text,''),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='accepted'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='rejected'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='pending'), 0),
		       quote_imports.uploaded_at, quote_imports.updated_at
		FROM quote_imports
		JOIN projects p ON p.id = quote_imports.project_id
		WHERE quote_imports.id=$1 AND p.company_id=$2
	`, id, companyID).Scan(
		&item.ID,
		&item.ProjectID,
		&item.ContactID,
		&item.SourceKind,
		&item.SourceFilename,
		&item.SourceDocumentID,
		&item.Status,
		&item.ParserVersion,
		&item.DetectedFormat,
		&item.ErrorMessage,
		&item.CreatedQuoteID,
		&item.ItemCount,
		&item.AcceptedCount,
		&item.RejectedCount,
		&item.PendingCount,
		&item.UploadedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListImportItems(ctx context.Context, importID string, companyID string) ([]QuoteImportItem, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}
	if _, err := s.GetImport(ctx, importID, companyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}

	rows, err := s.pg.Query(ctx, `
		SELECT ii.id::text, ii.import_id::text, ii.position_no, ii.outline_no, ii.description, ii.qty, ii.unit,
		       ii.is_optional, ii.parser_hint, ii.review_status, ii.review_note, ii.sort_order,
		       COALESCE(link.quote_id::text,''), COALESCE(link.quote_item_id::text,''), COALESCE(qi.position, 0),
		       ii.short_text, ii.long_text, ii.item_kind, ii.is_lump_sum, ii.base_position_no, COALESCE(ii.group_id::text,'')
		FROM quote_import_items ii
		LEFT JOIN quote_import_item_links link ON link.quote_import_item_id = ii.id
		LEFT JOIN quote_items qi ON qi.id = link.quote_item_id
		WHERE ii.import_id=$1
		ORDER BY ii.sort_order ASC, ii.id ASC
	`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]QuoteImportItem, 0)
	for rows.Next() {
		var item QuoteImportItem
		if err := rows.Scan(
			&item.ID,
			&item.ImportID,
			&item.PositionNo,
			&item.OutlineNo,
			&item.Description,
			&item.Qty,
			&item.Unit,
			&item.IsOptional,
			&item.ParserHint,
			&item.ReviewStatus,
			&item.ReviewNote,
			&item.SortOrder,
			&item.LinkedQuoteID,
			&item.LinkedQuoteItemID,
			&item.LinkedQuotePos,
			&item.ShortText,
			&item.LongText,
			&item.ItemKind,
			&item.IsLumpSum,
			&item.BasePositionNo,
			&item.GroupID,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// GetImportStructure liefert Gliederung (in Reihenfolge, Eltern vor
// Kindern) und Vorbemerkungen eines Importlaufs.
func (s *Service) GetImportStructure(ctx context.Context, importID string, companyID string) (*QuoteImportStructure, error) {
	imp, err := s.GetImport(ctx, importID, companyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}
	out := &QuoteImportStructure{ImportID: imp.ID, Groups: []QuoteImportGroup{}}
	if err := s.pg.QueryRow(ctx, `SELECT vorbemerkungen FROM quote_imports WHERE id=$1`, imp.ID).Scan(&out.Vorbemerkungen); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, outline_no, COALESCE(parent_group_id::text,''), kind, bezeichnung, sort_order
		FROM quote_import_groups
		WHERE import_id=$1
		ORDER BY sort_order ASC, id ASC
	`, imp.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g QuoteImportGroup
		if err := rows.Scan(&g.ID, &g.OutlineNo, &g.ParentGroupID, &g.Kind, &g.Bezeichnung, &g.SortOrder); err != nil {
			return nil, err
		}
		out.Groups = append(out.Groups, g)
	}
	return out, rows.Err()
}

func (s *Service) GetImportItem(ctx context.Context, importID, itemID string, companyID string) (*QuoteImportItem, error) {
	importID = strings.TrimSpace(importID)
	itemID = strings.TrimSpace(itemID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if itemID == "" {
		return nil, errors.New("item_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}
	if _, err := uuid.Parse(itemID); err != nil {
		return nil, errors.New("ungültige item_id")
	}
	if _, err := s.GetImport(ctx, importID, companyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}

	var item QuoteImportItem
	err := s.pg.QueryRow(ctx, `
		SELECT ii.id::text, ii.import_id::text, ii.position_no, ii.outline_no, ii.description, ii.qty, ii.unit,
		       ii.is_optional, ii.parser_hint, ii.review_status, ii.review_note, ii.sort_order,
		       COALESCE(link.quote_id::text,''), COALESCE(link.quote_item_id::text,''), COALESCE(qi.position, 0),
		       ii.short_text, ii.long_text, ii.item_kind, ii.is_lump_sum, ii.base_position_no, COALESCE(ii.group_id::text,'')
		FROM quote_import_items ii
		LEFT JOIN quote_import_item_links link ON link.quote_import_item_id = ii.id
		LEFT JOIN quote_items qi ON qi.id = link.quote_item_id
		WHERE ii.import_id=$1 AND ii.id=$2
	`, importID, itemID).Scan(
		&item.ID,
		&item.ImportID,
		&item.PositionNo,
		&item.OutlineNo,
		&item.Description,
		&item.Qty,
		&item.Unit,
		&item.IsOptional,
		&item.ParserHint,
		&item.ReviewStatus,
		&item.ReviewNote,
		&item.SortOrder,
		&item.LinkedQuoteID,
		&item.LinkedQuoteItemID,
		&item.LinkedQuotePos,
		&item.ShortText,
		&item.LongText,
		&item.ItemKind,
		&item.IsLumpSum,
		&item.BasePositionNo,
		&item.GroupID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importposition nicht gefunden")
		}
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdateImportItemReview(ctx context.Context, importID, itemID, reviewStatus, reviewNote string, companyID string) (*QuoteImportItem, error) {
	importID = strings.TrimSpace(importID)
	itemID = strings.TrimSpace(itemID)
	reviewStatus = strings.ToLower(strings.TrimSpace(reviewStatus))
	reviewNote = strings.TrimSpace(reviewNote)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if itemID == "" {
		return nil, errors.New("item_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}
	if _, err := uuid.Parse(itemID); err != nil {
		return nil, errors.New("ungültige item_id")
	}
	if _, ok := allowedQuoteImportReviewStatuses[reviewStatus]; !ok {
		return nil, errors.New("Ungültiger review_status")
	}

	var importStatus string
	err := s.pg.QueryRow(ctx, `
		SELECT quote_imports.status
		FROM quote_imports
		JOIN projects p ON p.id = quote_imports.project_id
		WHERE quote_imports.id=$1 AND quote_imports.source_kind='gaeb' AND p.company_id=$2
	`, importID, companyID).Scan(&importStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}
	if importStatus != "parsed" {
		return nil, errors.New("Nur geparste Importläufe können reviewt werden")
	}

	tag, err := s.pg.Exec(ctx, `
		UPDATE quote_import_items
		SET review_status=$3,
		    review_note=$4
		WHERE import_id=$1 AND id=$2
	`, importID, itemID, reviewStatus, reviewNote)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("Importposition nicht gefunden")
	}
	return s.GetImportItem(ctx, importID, itemID, companyID)
}

func (s *Service) MarkImportReviewed(ctx context.Context, importID string, companyID string) (*QuoteImport, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}

	var status string
	err := s.pg.QueryRow(ctx, `
		SELECT quote_imports.status
		FROM quote_imports
		JOIN projects p ON p.id = quote_imports.project_id
		WHERE quote_imports.id=$1 AND quote_imports.source_kind='gaeb' AND p.company_id=$2
	`, importID, companyID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}
	if status != "parsed" {
		return nil, errors.New("Nur geparste Importläufe können freigegeben werden")
	}

	var pendingCount int
	if err := s.pg.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM quote_import_items
		WHERE import_id=$1 AND review_status='pending'
	`, importID).Scan(&pendingCount); err != nil {
		return nil, err
	}
	if pendingCount > 0 {
		return nil, errors.New("Importlauf enthält noch offene Review-Positionen")
	}

	var reviewedCount int
	if err := s.pg.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM quote_import_items
		WHERE import_id=$1 AND review_status IN ('accepted','rejected')
	`, importID).Scan(&reviewedCount); err != nil {
		return nil, err
	}
	if reviewedCount == 0 {
		return nil, errors.New("Importlauf enthält keine reviewten Positionen")
	}

	// Zuordnung (ADR 0045 Frage 5, I.5.2c): kein laufender Lauf und kein
	// offener Vorschlag; Positionen ohne Vorschlag sind zulaessig.
	var matchStatus string
	var openMatches int
	if err := s.pg.QueryRow(ctx, `
		SELECT match_status,
		       (SELECT COUNT(*) FROM quote_import_item_matches WHERE import_id=$1 AND entscheidung='offen' AND code<>'')
		FROM quote_imports WHERE id=$1
	`, importID).Scan(&matchStatus, &openMatches); err != nil {
		return nil, err
	}
	if matchStatus == "laeuft" {
		return nil, errors.New("Zuordnung läuft bereits - Freigabe erst nach ihrem Ende")
	}
	if openMatches > 0 {
		return nil, fmt.Errorf("Importlauf enthält noch offene Vorschläge der Zuordnung (%d)", openMatches)
	}

	// Bedingt, damit kein Lauf zwischen Pruefung und Freigabe beginnt.
	tag, err := s.pg.Exec(ctx, `
		UPDATE quote_imports
		SET status='reviewed',
		    updated_at=now()
		WHERE id=$1 AND status='parsed' AND match_status<>'laeuft'
		  AND NOT EXISTS (SELECT 1 FROM quote_import_item_matches WHERE import_id=$1 AND entscheidung='offen' AND code<>'')
	`, importID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("Importlauf kann nicht freigegeben werden: Stand hat sich geändert, bitte neu laden")
	}
	return s.GetImport(ctx, importID, companyID)
}

func (s *Service) ApplyImportToDraftQuote(ctx context.Context, importID string, companyID string) (*QuoteImportApplyResult, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var imp QuoteImport
	err = tx.QueryRow(ctx, `
		SELECT quote_imports.id::text, quote_imports.project_id::text, COALESCE(quote_imports.contact_id::text,''), quote_imports.status, COALESCE(quote_imports.created_quote_id::text,'')
		FROM quote_imports
		JOIN projects p ON p.id = quote_imports.project_id
		WHERE quote_imports.id=$1 AND quote_imports.source_kind='gaeb' AND p.company_id=$2
		FOR UPDATE OF quote_imports
	`, importID, companyID).Scan(&imp.ID, &imp.ProjectID, &imp.ContactID, &imp.Status, &imp.CreatedQuoteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}
	if imp.Status != "reviewed" {
		return nil, errors.New("Nur freigegebene Importläufe können übernommen werden")
	}
	if strings.TrimSpace(imp.CreatedQuoteID) != "" {
		return nil, errors.New("Importlauf wurde bereits in ein Angebot übernommen")
	}

	var pendingCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM quote_import_items
		WHERE import_id=$1 AND review_status='pending'
	`, importID).Scan(&pendingCount); err != nil {
		return nil, err
	}
	if pendingCount > 0 {
		return nil, errors.New("Importlauf enthält noch offene Review-Positionen")
	}

	// Leistung aus der Zuordnung (I.5.4): bestaetigt = vorgeschlagene,
	// geaendert = gewaehlte; sonst keine. Der Preis bleibt 0.
	rows, err := tx.Query(ctx, `
		SELECT ii.id::text, ii.description, ii.qty, ii.unit, ii.position_no, ii.short_text, ii.long_text, ii.item_kind, ii.is_lump_sum,
		       ii.base_position_no, COALESCE(ii.group_id::text,''),
		       COALESCE(CASE m.entscheidung WHEN 'bestaetigt' THEN m.leistung_id WHEN 'geaendert' THEN m.entschieden_leistung_id END, '')
		FROM quote_import_items ii
		LEFT JOIN quote_import_item_matches m ON m.quote_import_item_id = ii.id
		WHERE ii.import_id=$1 AND ii.review_status='accepted'
		ORDER BY ii.sort_order ASC, ii.id ASC
	`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	acceptedItems := make([]acceptedQuoteImportApplyItem, 0)
	quoteItems := make([]QuoteItemInput, 0)
	for rows.Next() {
		var acceptedItem acceptedQuoteImportApplyItem
		var quoteItem QuoteItemInput
		if err := rows.Scan(&acceptedItem.ImportItemID, &acceptedItem.Description, &acceptedItem.Qty, &acceptedItem.Unit,
			&quoteItem.OutlineNo, &quoteItem.ShortText, &quoteItem.LongText, &quoteItem.ItemKind, &quoteItem.IsLumpSum,
			&quoteItem.BaseOutlineNo, &acceptedItem.ImportGroupID, &quoteItem.LeistungID); err != nil {
			return nil, err
		}
		quoteItem.Description = acceptedItem.Description
		quoteItem.Qty = acceptedItem.Qty
		quoteItem.Unit = acceptedItem.Unit
		quoteItem.UnitPrice = 0
		quoteItem.TaxCode = ""
		acceptedItems = append(acceptedItems, acceptedItem)
		quoteItems = append(quoteItems, quoteItem)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(quoteItems) == 0 {
		return nil, errors.New("Importlauf enthält keine akzeptierten Positionen")
	}

	note := fmt.Sprintf("Erzeugt aus GAEB-Import %s", importID)
	createdQuoteID, createdQuoteItemIDs, err := s.createQuoteTx(ctx, tx, QuoteInput{
		ProjectID: imp.ProjectID,
		ContactID: imp.ContactID,
		Currency:  "EUR",
		Note:      note,
		Items:     quoteItems,
	}, companyID)
	if err != nil {
		return nil, err
	}
	if len(createdQuoteItemIDs) != len(acceptedItems) {
		return nil, errors.New("Quote-Positionen konnten nicht eindeutig zugeordnet werden")
	}
	quoteGroupIDs, err := copyImportGroupsTx(ctx, tx, importID, createdQuoteID)
	if err != nil {
		return nil, err
	}
	for idx, acceptedItem := range acceptedItems {
		groupID, ok := quoteGroupIDs[acceptedItem.ImportGroupID]
		if !ok {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE quote_items SET group_id=$2 WHERE id=$1`, createdQuoteItemIDs[idx], groupID); err != nil {
			return nil, err
		}
	}

	for idx, acceptedItem := range acceptedItems {
		if _, err := tx.Exec(ctx, `
			INSERT INTO quote_import_item_links (
				id, quote_import_item_id, quote_id, quote_item_id, created_at
			) VALUES ($1,$2,$3,$4,now())
		`,
			uuid.NewString(),
			acceptedItem.ImportItemID,
			createdQuoteID,
			createdQuoteItemIDs[idx],
		); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE quote_imports
		SET status='applied',
		    created_quote_id=$2,
		    updated_at=now()
		WHERE id=$1
	`, importID, createdQuoteID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	updatedImport, err := s.GetImport(ctx, importID, companyID)
	if err != nil {
		return nil, err
	}
	createdQuote, err := s.Get(ctx, createdQuoteID, companyID)
	if err != nil {
		return nil, err
	}
	return &QuoteImportApplyResult{
		Import: updatedImport,
		Quote:  createdQuote,
	}, nil
}

func (s *Service) SaveImportParseResult(ctx context.Context, importID, parserVersion, detectedFormat string, items []QuoteImportItemInput, companyID string) (*QuoteImport, error) {
	return s.SaveImportParse(ctx, importID, GAEBImportParseResult{ParserVersion: parserVersion, DetectedFormat: detectedFormat, Items: items}, companyID)
}

// SaveImportParse speichert ein Parse-Ergebnis mit Gliederung und
// Vorbemerkungen und ersetzt ein frueheres Ergebnis desselben Importlaufs.
func (s *Service) SaveImportParse(ctx context.Context, importID string, result GAEBImportParseResult, companyID string) (*QuoteImport, error) {
	items := result.Items
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}
	if len(items) == 0 {
		return nil, errors.New("keine Importpositionen")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM quote_imports
			JOIN projects p ON p.id = quote_imports.project_id
			WHERE quote_imports.id=$1 AND quote_imports.source_kind='gaeb' AND p.company_id=$2
		)
	`, importID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Importlauf nicht gefunden")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM quote_import_items WHERE import_id=$1`, importID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM quote_import_groups WHERE import_id=$1`, importID); err != nil {
		return nil, err
	}

	groupIDs := map[string]string{} // outline_no -> id
	for idx, g := range result.Groups {
		outline, parent := strings.TrimSpace(g.OutlineNo), strings.TrimSpace(g.ParentOutlineNo)
		switch {
		case outline == "":
			return nil, errors.New("Gliederung ohne Ordnungszahl")
		case groupIDs[outline] != "":
			return nil, fmt.Errorf("Gliederung %s ist doppelt", outline)
		case !quoteImportGroupKinds[g.Kind]:
			return nil, fmt.Errorf("Gliederung %s: unbekannte Stufe %q", outline, g.Kind)
		case parent != "" && groupIDs[parent] == "":
			return nil, fmt.Errorf("Gliederung %s: übergeordneter Bereich %s fehlt oder steht später", outline, parent)
		}
		var parentID *string
		if parent != "" {
			id := groupIDs[parent]
			parentID = &id
		}
		id := uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO quote_import_groups (id, import_id, parent_group_id, outline_no, kind, bezeichnung, sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, id, importID, parentID, outline, g.Kind, strings.TrimSpace(g.Bezeichnung), idx+1); err != nil {
			return nil, err
		}
		groupIDs[outline] = id
	}

	for idx, item := range items {
		if strings.TrimSpace(item.PositionNo) == "" {
			return nil, errors.New("position_no erforderlich")
		}
		if strings.TrimSpace(item.Description) == "" {
			return nil, errors.New("description erforderlich")
		}
		if item.Qty < 0 {
			return nil, errors.New("qty darf nicht negativ sein")
		}
		if item.SortOrder <= 0 {
			item.SortOrder = idx + 1
		}
		if item.ItemKind == "" {
			item.ItemKind = "normal"
		}
		if !quoteImportItemKinds[item.ItemKind] {
			return nil, fmt.Errorf("Position %s: unbekannte Positionsart %q", item.PositionNo, item.ItemKind)
		}
		if (item.ItemKind == "alternativposition") != (strings.TrimSpace(item.BasePositionNo) != "") {
			return nil, fmt.Errorf("Position %s: Grundposition nur und immer bei Alternativpositionen", item.PositionNo)
		}
		var groupID *string
		if id := groupIDs[strings.TrimSpace(item.OutlineNo)]; id != "" {
			groupID = &id
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO quote_import_items (
				id, import_id, position_no, outline_no, description, qty, unit,
				is_optional, parser_hint, review_status, review_note, sort_order,
				short_text, long_text, item_kind, is_lump_sum, base_position_no, group_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending','',$10,$11,$12,$13,$14,$15,$16)
		`,
			uuid.NewString(),
			importID,
			strings.TrimSpace(item.PositionNo),
			strings.TrimSpace(item.OutlineNo),
			strings.TrimSpace(item.Description),
			item.Qty,
			strings.TrimSpace(item.Unit),
			item.IsOptional,
			strings.TrimSpace(item.ParserHint),
			item.SortOrder,
			item.ShortText,
			item.LongText,
			item.ItemKind,
			item.IsLumpSum,
			strings.TrimSpace(item.BasePositionNo),
			groupID,
		); err != nil {
			return nil, err
		}
	}

	vorbemerkungen := result.Vorbemerkungen
	if vorbemerkungen == nil {
		vorbemerkungen = []string{}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE quote_imports
		SET status='parsed',
		    parser_version=$2,
		    detected_format=$3,
		    error_message='',
		    vorbemerkungen=$4,
		    updated_at=now()
		WHERE id=$1
	`, importID, strings.TrimSpace(result.ParserVersion), strings.TrimSpace(result.DetectedFormat), vorbemerkungen); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetImport(ctx, importID, companyID)
}

func (s *Service) MarkImportFailed(ctx context.Context, importID, parserVersion, detectedFormat, errorMessage string, companyID string) (*QuoteImport, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}
	errorMessage = strings.TrimSpace(errorMessage)
	if errorMessage == "" {
		return nil, errors.New("error_message erforderlich")
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM quote_imports
			JOIN projects p ON p.id = quote_imports.project_id
			WHERE quote_imports.id=$1 AND quote_imports.source_kind='gaeb' AND p.company_id=$2
		)
	`, importID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Importlauf nicht gefunden")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM quote_import_items WHERE import_id=$1`, importID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE quote_imports
		SET status='failed',
		    parser_version=$2,
		    detected_format=$3,
		    error_message=$4,
		    updated_at=now()
		WHERE id=$1
	`, importID, strings.TrimSpace(parserVersion), strings.TrimSpace(detectedFormat), errorMessage); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetImport(ctx, importID, companyID)
}

func (s *Service) ProcessGAEBImport(ctx context.Context, importID string, companyID string) (*QuoteImport, error) {
	if s.gaebImportParser == nil {
		return nil, errors.New("GAEB-Parser nicht konfiguriert")
	}
	if s.mg == nil || strings.TrimSpace(s.mongoDB) == "" {
		return nil, errors.New("MongoDB nicht konfiguriert")
	}

	imp, err := s.GetImport(ctx, importID, companyID)
	if err != nil {
		return nil, err
	}
	if imp.SourceKind != "gaeb" {
		return nil, errors.New("Nur GAEB-Importläufe können verarbeitet werden")
	}
	if imp.Status != "uploaded" {
		return nil, errors.New("Nur hochgeladene Importläufe können verarbeitet werden")
	}

	sourceID, err := primitive.ObjectIDFromHex(strings.TrimSpace(imp.SourceDocumentID))
	if err != nil {
		return nil, errors.New("Ungültige Quelldokument-ID")
	}
	bucket, err := gridfs.NewBucket(s.mg.Database(s.mongoDB))
	if err != nil {
		return nil, err
	}
	stream, err := bucket.OpenDownloadStream(sourceID)
	if err != nil {
		return s.MarkImportFailed(ctx, imp.ID, "adapter-v1", detectedGAEBImportFormat(imp.SourceFilename), err.Error(), companyID)
	}
	defer stream.Close()

	result, err := s.gaebImportParser.ParseGAEB(ctx, stream, imp.SourceFilename)
	if err != nil {
		return s.MarkImportFailed(ctx, imp.ID, "adapter-v1", detectedGAEBImportFormat(imp.SourceFilename), err.Error(), companyID)
	}
	return s.SaveImportParse(ctx, imp.ID, result, companyID)
}

func detectedGAEBImportFormat(filename string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(filename))), ".")
}

func isAllowedGAEBImportFilename(filename string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	_, ok := allowedGAEBImportExtensions[ext]
	return ok
}
