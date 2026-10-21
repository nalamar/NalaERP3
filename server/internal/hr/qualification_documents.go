package hr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

// Nachweisdokumente zu Qualifikationen (Backlog F.10.1): Scan des
// Zertifikats, unterschriebene Teilnehmerliste. Datei in GridFS, Verweis
// in hr_qualification_documents (Migration 096). Keine Loeschung -
// Beweisfunktion wie der Nachweis selbst.

// MaxQualificationDocumentBytes begrenzt eine Datei (Scans, PDFs).
const MaxQualificationDocumentBytes = 10 << 20

// allowedDocumentTypes: was als Nachweis taugt. Erkannt am Inhalt
// (http.DetectContentType), nicht an der Angabe des Browsers.
var allowedDocumentTypes = map[string]bool{"application/pdf": true, "image/jpeg": true, "image/png": true}

// ErrDocumentTooLarge und ErrDocumentType melden unpassende Dateien; die
// Meldungen enthalten "Ungültig", damit sie ueber HTTP als 400 ankommen.
var (
	ErrDocumentTooLarge = errors.New("Ungültige Datei: höchstens 10 MB")
	ErrDocumentType     = errors.New("Ungültige Datei: erlaubt sind PDF, JPEG und PNG")
)

// QualificationDocument ist der Verweis auf eine hochgeladene Datei.
type QualificationDocument struct {
	ID              uuid.UUID `json:"id"`
	QualificationID uuid.UUID `json:"qualification_id"`
	DocumentID      string    `json:"document_id"`
	Filename        string    `json:"filename"`
	ContentType     string    `json:"content_type"`
	Length          int64     `json:"length"`
	UploadedAt      time.Time `json:"uploaded_at"`
}

// QualificationDocumentService braucht Postgres (Verweise) und MongoDB
// (Dateien).
type QualificationDocumentService struct {
	quals   *QualificationService
	mg      *mongo.Client
	mongoDB string
}

func NewQualificationDocumentService(quals *QualificationService, mg *mongo.Client, mongoDB string) *QualificationDocumentService {
	return &QualificationDocumentService{quals: quals, mg: mg, mongoDB: mongoDB}
}

func (s *QualificationDocumentService) bucket() (*gridfs.Bucket, error) {
	if s.mg == nil || s.mongoDB == "" {
		return nil, errors.New("MongoDB nicht konfiguriert")
	}
	return gridfs.NewBucket(s.mg.Database(s.mongoDB))
}

// cleanFilename: nur der Dateiname, ohne Pfad; leer = "nachweis".
// path.Base statt filepath.Base: der Server laeuft unter Linux, wo
// filepath einen Windows-Pfad (Backslashes) nicht zerlegt - die
// Backslashes werden deshalb vorher zu "/".
func cleanFilename(name string) string {
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return "nachweis"
	}
	return name
}

// Upload speichert eine Datei zu einem Nachweis. Liest hoechstens
// MaxQualificationDocumentBytes+1 Bytes, prueft Groesse und Art am Inhalt.
// Zu stornierten Nachweisen wird nichts mehr hochgeladen.
func (s *QualificationDocumentService) Upload(ctx context.Context, qualificationID uuid.UUID, r io.Reader, filename, actorUserID, companyID string) (*QualificationDocument, error) {
	q, err := s.quals.Get(ctx, qualificationID, companyID)
	if err != nil {
		return nil, err
	}
	if q.Status != QualificationRecordActive {
		return nil, errors.New("Nachweis ist storniert")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxQualificationDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxQualificationDocumentBytes {
		return nil, ErrDocumentTooLarge
	}
	if len(data) == 0 {
		return nil, errors.New("Datei erforderlich")
	}
	contentType := http.DetectContentType(data)
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	if !allowedDocumentTypes[contentType] {
		return nil, ErrDocumentType
	}
	b, err := s.bucket()
	if err != nil {
		return nil, err
	}
	name := cleanFilename(filename)
	oid, err := b.UploadFromStream(name, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	d := QualificationDocument{ID: uuid.New(), QualificationID: qualificationID, DocumentID: oid.Hex(), Filename: name, ContentType: contentType, Length: int64(len(data))}
	err = s.quals.pg.QueryRow(ctx, `
        INSERT INTO hr_qualification_documents (id, qualification_id, document_id, filename, content_type, length, uploaded_by_user_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING uploaded_at
    `, d.ID, d.QualificationID, d.DocumentID, d.Filename, d.ContentType, d.Length, nullableUserID(actorUserID)).Scan(&d.UploadedAt)
	if err != nil {
		// Ohne Verweis waere die Datei verwaist - wieder entfernen.
		_ = b.Delete(oid)
		return nil, err
	}
	return &d, nil
}

// List liefert die Dokumente eines Nachweises, aelteste zuerst.
func (s *QualificationDocumentService) List(ctx context.Context, qualificationID uuid.UUID, companyID string) ([]QualificationDocument, error) {
	if _, err := s.quals.Get(ctx, qualificationID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.quals.pg.Query(ctx, `
        SELECT id, qualification_id, document_id, filename, content_type, length, uploaded_at
          FROM hr_qualification_documents WHERE qualification_id=$1 ORDER BY uploaded_at, id
    `, qualificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualificationDocument{}
	for rows.Next() {
		var d QualificationDocument
		if err := rows.Scan(&d.ID, &d.QualificationID, &d.DocumentID, &d.Filename, &d.ContentType, &d.Length, &d.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Open oeffnet ein Dokument zum Download - nur, wenn es zu genau diesem
// Nachweis gehoert und der Nachweis zum Mandanten (Get prueft das).
func (s *QualificationDocumentService) Open(ctx context.Context, qualificationID, documentID uuid.UUID, companyID string) (io.ReadCloser, *QualificationDocument, error) {
	if _, err := s.quals.Get(ctx, qualificationID, companyID); err != nil {
		return nil, nil, err
	}
	var d QualificationDocument
	err := s.quals.pg.QueryRow(ctx, `
        SELECT id, qualification_id, document_id, filename, content_type, length, uploaded_at
          FROM hr_qualification_documents WHERE id=$1 AND qualification_id=$2
    `, documentID, qualificationID).Scan(&d.ID, &d.QualificationID, &d.DocumentID, &d.Filename, &d.ContentType, &d.Length, &d.UploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, errors.New("Dokument nicht gefunden")
	}
	if err != nil {
		return nil, nil, err
	}
	oid, err := primitive.ObjectIDFromHex(d.DocumentID)
	if err != nil {
		return nil, nil, errors.New("Dokument nicht gefunden")
	}
	b, err := s.bucket()
	if err != nil {
		return nil, nil, err
	}
	rc, err := b.OpenDownloadStream(oid)
	if err != nil {
		return nil, nil, err
	}
	return rc, &d, nil
}
