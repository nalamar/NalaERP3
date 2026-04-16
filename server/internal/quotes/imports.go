package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
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
}

type QuoteImportItemInput struct {
	PositionNo  string
	OutlineNo   string
	Description string
	Qty         float64
	Unit        string
	IsOptional  bool
	ParserHint  string
	SortOrder   int
}

type acceptedQuoteImportApplyItem struct {
	ImportItemID string
	Description  string
	Qty          float64
	Unit         string
}

type QuoteImportApplyResult struct {
	Import *QuoteImport `json:"import"`
	Quote  *Quote       `json:"quote"`
}

type QuoteImportCreateInput struct {
	ProjectID string
	ContactID string
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
	".d83":  {},
	".p83":  {},
	".gaeb": {},
	".xml":  {},
}

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

func (s *Service) CreateGAEBImport(ctx context.Context, in QuoteImportCreateInput, r io.Reader, filename string) (*QuoteImport, error) {
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
	if !isAllowedGAEBImportFilename(filename) {
		return nil, errors.New("Nur GAEB-Dateien mit den Endungen .x83, .x84, .d83, .p83, .gaeb oder .xml sind zulässig")
	}

	var projectExists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)`, in.ProjectID).Scan(&projectExists); err != nil {
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
	oid, err := bucket.UploadFromStream(filename, r)
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
	return s.GetImport(ctx, id)
}

func (s *Service) ListImports(ctx context.Context, f QuoteImportFilter) ([]QuoteImport, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args := make([]any, 0, 4)
	conds := make([]string, 0, 2)
	if strings.TrimSpace(f.ProjectID) != "" {
		args = append(args, strings.TrimSpace(f.ProjectID))
		conds = append(conds, fmt.Sprintf("project_id::text=$%d", len(args)))
	}
	if strings.TrimSpace(f.ContactID) != "" {
		args = append(args, strings.TrimSpace(f.ContactID))
		conds = append(conds, fmt.Sprintf("contact_id::text=$%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, project_id::text, COALESCE(contact_id::text,''), source_kind, source_filename,
		       source_document_id, status, parser_version, detected_format, error_message,
		       COALESCE(created_quote_id::text,''),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='accepted'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='rejected'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='pending'), 0),
		       uploaded_at, updated_at
		FROM quote_imports`+where+`
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

func (s *Service) GetImport(ctx context.Context, id string) (*QuoteImport, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("id erforderlich")
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, errors.New("ungültige id")
	}

	var item QuoteImport
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, project_id::text, COALESCE(contact_id::text,''), source_kind, source_filename,
		       source_document_id, status, parser_version, detected_format, error_message,
		       COALESCE(created_quote_id::text,''),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='accepted'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='rejected'), 0),
		       COALESCE((SELECT COUNT(*) FROM quote_import_items qi WHERE qi.import_id = quote_imports.id AND qi.review_status='pending'), 0),
		       uploaded_at, updated_at
		FROM quote_imports
		WHERE id=$1
	`, id).Scan(
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

func (s *Service) ListImportItems(ctx context.Context, importID string) ([]QuoteImportItem, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}
	if _, err := s.GetImport(ctx, importID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}

	rows, err := s.pg.Query(ctx, `
		SELECT ii.id::text, ii.import_id::text, ii.position_no, ii.outline_no, ii.description, ii.qty, ii.unit,
		       ii.is_optional, ii.parser_hint, ii.review_status, ii.review_note, ii.sort_order,
		       COALESCE(link.quote_id::text,''), COALESCE(link.quote_item_id::text,''), COALESCE(qi.position, 0)
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
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) GetImportItem(ctx context.Context, importID, itemID string) (*QuoteImportItem, error) {
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
	if _, err := s.GetImport(ctx, importID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importlauf nicht gefunden")
		}
		return nil, err
	}

	var item QuoteImportItem
	err := s.pg.QueryRow(ctx, `
		SELECT ii.id::text, ii.import_id::text, ii.position_no, ii.outline_no, ii.description, ii.qty, ii.unit,
		       ii.is_optional, ii.parser_hint, ii.review_status, ii.review_note, ii.sort_order,
		       COALESCE(link.quote_id::text,''), COALESCE(link.quote_item_id::text,''), COALESCE(qi.position, 0)
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
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Importposition nicht gefunden")
		}
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdateImportItemReview(ctx context.Context, importID, itemID, reviewStatus, reviewNote string) (*QuoteImportItem, error) {
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
		SELECT status
		FROM quote_imports
		WHERE id=$1 AND source_kind='gaeb'
	`, importID).Scan(&importStatus)
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
	return s.GetImportItem(ctx, importID, itemID)
}

func (s *Service) MarkImportReviewed(ctx context.Context, importID string) (*QuoteImport, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, errors.New("import_id erforderlich")
	}
	if _, err := uuid.Parse(importID); err != nil {
		return nil, errors.New("ungültige import_id")
	}

	var status string
	err := s.pg.QueryRow(ctx, `
		SELECT status
		FROM quote_imports
		WHERE id=$1 AND source_kind='gaeb'
	`, importID).Scan(&status)
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

	if _, err := s.pg.Exec(ctx, `
		UPDATE quote_imports
		SET status='reviewed',
		    updated_at=now()
		WHERE id=$1
	`, importID); err != nil {
		return nil, err
	}
	return s.GetImport(ctx, importID)
}

func (s *Service) ApplyImportToDraftQuote(ctx context.Context, importID string) (*QuoteImportApplyResult, error) {
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
		SELECT id::text, project_id::text, COALESCE(contact_id::text,''), status, COALESCE(created_quote_id::text,'')
		FROM quote_imports
		WHERE id=$1 AND source_kind='gaeb'
		FOR UPDATE
	`, importID).Scan(&imp.ID, &imp.ProjectID, &imp.ContactID, &imp.Status, &imp.CreatedQuoteID)
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

	rows, err := tx.Query(ctx, `
		SELECT id::text, description, qty, unit
		FROM quote_import_items
		WHERE import_id=$1 AND review_status='accepted'
		ORDER BY sort_order ASC, id ASC
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
		if err := rows.Scan(&acceptedItem.ImportItemID, &acceptedItem.Description, &acceptedItem.Qty, &acceptedItem.Unit); err != nil {
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
	})
	if err != nil {
		return nil, err
	}
	if len(createdQuoteItemIDs) != len(acceptedItems) {
		return nil, errors.New("Quote-Positionen konnten nicht eindeutig zugeordnet werden")
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

	updatedImport, err := s.GetImport(ctx, importID)
	if err != nil {
		return nil, err
	}
	createdQuote, err := s.Get(ctx, createdQuoteID)
	if err != nil {
		return nil, err
	}
	return &QuoteImportApplyResult{
		Import: updatedImport,
		Quote:  createdQuote,
	}, nil
}

func (s *Service) SaveImportParseResult(ctx context.Context, importID, parserVersion, detectedFormat string, items []QuoteImportItemInput) (*QuoteImport, error) {
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
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM quote_imports WHERE id=$1 AND source_kind='gaeb')`, importID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("Importlauf nicht gefunden")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM quote_import_items WHERE import_id=$1`, importID); err != nil {
		return nil, err
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
		if _, err := tx.Exec(ctx, `
			INSERT INTO quote_import_items (
				id, import_id, position_no, outline_no, description, qty, unit,
				is_optional, parser_hint, review_status, review_note, sort_order
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending','',$10)
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
		); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE quote_imports
		SET status='parsed',
		    parser_version=$2,
		    detected_format=$3,
		    error_message='',
		    updated_at=now()
		WHERE id=$1
	`, importID, strings.TrimSpace(parserVersion), strings.TrimSpace(detectedFormat)); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetImport(ctx, importID)
}

func (s *Service) MarkImportFailed(ctx context.Context, importID, parserVersion, detectedFormat, errorMessage string) (*QuoteImport, error) {
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
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM quote_imports WHERE id=$1 AND source_kind='gaeb')`, importID).Scan(&exists); err != nil {
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
	return s.GetImport(ctx, importID)
}

func isAllowedGAEBImportFilename(filename string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	_, ok := allowedGAEBImportExtensions[ext]
	return ok
}
