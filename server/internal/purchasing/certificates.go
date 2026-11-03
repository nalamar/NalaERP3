package purchasing

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
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

// Zertifikate am Lieferanten (ADR 0035 §1, Backlog H.4.2a). Tabellen in
// Migration 110, Regeln in evidence.go. Nie loeschen, nur stornieren;
// Dateien in GridFS, nicht ueber den allgemeinen Download (§6).

// MaxEvidenceDocumentBytes begrenzt eine Datei (Scans, PDFs) wie bei den
// Qualifikationsnachweisen.
const MaxEvidenceDocumentBytes = 10 << 20

// Erkannt am Inhalt (http.DetectContentType), nicht an der Angabe des
// Browsers. Die Meldungen enthalten "Ungültig" (HTTP 400).
var (
	evidenceDocumentTypes   = map[string]bool{"application/pdf": true, "image/jpeg": true, "image/png": true}
	ErrEvidenceTooLarge     = errors.New("Ungültige Datei: höchstens 10 MB")
	ErrEvidenceDocumentType = errors.New("Ungültige Datei: erlaubt sind PDF, JPEG und PNG")
)

type SupplierCertificate struct {
	ID           uuid.UUID  `json:"id"`
	SupplierID   string     `json:"lieferant_id"`
	Art          string     `json:"art"`
	Nummer       string     `json:"nummer"`
	Stelle       string     `json:"stelle"`
	ExcBis       *string    `json:"exc_bis"`
	GueltigVon   *time.Time `json:"gueltig_von"`
	GueltigBis   *time.Time `json:"gueltig_bis"`
	Notiz        string     `json:"notiz"`
	StornoReason string     `json:"storno_grund"`
	StorniertAt  *time.Time `json:"storniert_am"`
	CreatedAt    time.Time  `json:"angelegt_am"`
	Hinweise     []string   `json:"hinweise"`
}

type CertificateInput struct {
	Art        string     `json:"art"`
	Nummer     string     `json:"nummer"`
	Stelle     string     `json:"stelle"`
	ExcBis     *string    `json:"exc_bis"`
	GueltigVon *time.Time `json:"gueltig_von"`
	GueltigBis *time.Time `json:"gueltig_bis"`
	Notiz      string     `json:"notiz"`
}

type EvidenceDocument struct {
	ID          uuid.UUID `json:"id"`
	DocumentID  string    `json:"document_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Length      int64     `json:"length"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// CertificateService braucht Postgres (Zertifikate, Verweise) und MongoDB
// (Dateien). now ist fuer Tests austauschbar.
type CertificateService struct {
	pg      *pgxpool.Pool
	mg      *mongo.Client
	mongoDB string
	now     func() time.Time
}

func NewCertificateService(pg *pgxpool.Pool, mg *mongo.Client, mongoDB string) *CertificateService {
	return &CertificateService{pg: pg, mg: mg, mongoDB: mongoDB, now: time.Now}
}

func nullableUser(id string) *string {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return &id
}

// supplierOwned: Kontakt im Mandanten mit Rolle Lieferant (supplier/both).
func (s *CertificateService) supplierOwned(ctx context.Context, supplierID, companyID string) error {
	var ok bool
	if err := s.pg.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM contacts WHERE id=$1 AND company_id=$2 AND rolle IN ('supplier','both'))
    `, strings.TrimSpace(supplierID), companyID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("Lieferant nicht gefunden")
	}
	return nil
}

const certificateColumns = `id, supplier_id, art, nummer, stelle, exc_bis, gueltig_von, gueltig_bis, notiz, storno_reason, storniert_at, created_at`

func scanCertificate(row pgx.Row) (*SupplierCertificate, error) {
	var c SupplierCertificate
	if err := row.Scan(&c.ID, &c.SupplierID, &c.Art, &c.Nummer, &c.Stelle, &c.ExcBis, &c.GueltigVon, &c.GueltigBis, &c.Notiz, &c.StornoReason, &c.StorniertAt, &c.CreatedAt); err != nil {
		return nil, err
	}
	c.Hinweise = []string{}
	return &c, nil
}

func (c SupplierCertificate) rule() Certificate {
	return Certificate{Art: c.Art, ExcBis: c.ExcBis, GueltigVon: c.GueltigVon, GueltigBis: c.GueltigBis, Storniert: c.StorniertAt != nil}
}

func (s *CertificateService) Create(ctx context.Context, supplierID string, in CertificateInput, actorUserID, companyID string) (*SupplierCertificate, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if err := s.supplierOwned(ctx, supplierID, companyID); err != nil {
		return nil, err
	}
	r := Certificate{Art: in.Art, ExcBis: in.ExcBis, GueltigVon: in.GueltigVon, GueltigBis: in.GueltigBis}
	if err := NormalizeCertificate(&r); err != nil {
		return nil, err
	}
	return scanCertificate(s.pg.QueryRow(ctx, `
        INSERT INTO supplier_certificates (id, company_id, supplier_id, art, nummer, stelle, exc_bis, gueltig_von, gueltig_bis, notiz, created_by_user_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        RETURNING `+certificateColumns,
		uuid.New(), companyID, strings.TrimSpace(supplierID), r.Art, strings.TrimSpace(in.Nummer), strings.TrimSpace(in.Stelle),
		r.ExcBis, r.GueltigVon, r.GueltigBis, strings.TrimSpace(in.Notiz), nullableUser(actorUserID)))
}

func (s *CertificateService) Get(ctx context.Context, id uuid.UUID, companyID string) (*SupplierCertificate, error) {
	c, err := scanCertificate(s.pg.QueryRow(ctx, `SELECT `+certificateColumns+` FROM supplier_certificates WHERE id=$1 AND company_id=$2`, id, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Zertifikat nicht gefunden")
	}
	return c, err
}

// List liefert die Zertifikate eines Lieferanten, juengstes Ende zuerst
// (ohne Ende ganz oben), mit dem Hinweis "laeuft ab" (evidence.go).
// Stornierte nur auf Wunsch; der Hinweis rechnet sie nie mit.
func (s *CertificateService) List(ctx context.Context, supplierID string, includeStorniert bool, companyID string) ([]SupplierCertificate, error) {
	if err := s.supplierOwned(ctx, supplierID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `SELECT `+certificateColumns+` FROM supplier_certificates
        WHERE supplier_id=$1 AND company_id=$2 ORDER BY gueltig_bis DESC NULLS FIRST, created_at DESC, id`,
		strings.TrimSpace(supplierID), companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []SupplierCertificate{}
	for rows.Next() {
		c, err := scanCertificate(rows)
		if err != nil {
			return nil, err
		}
		all = append(all, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rules := make([]Certificate, len(all))
	for i, c := range all {
		rules[i] = c.rule()
	}
	for _, i := range ExpiringCertificates(rules, s.now()) {
		all[i].Hinweise = append(all[i].Hinweise, HintZertifikatLaeuftAb)
	}
	out := []SupplierCertificate{}
	for _, c := range all {
		if includeStorniert || c.StorniertAt == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// Storno ist einmalig und schreibt nur die Storno-Spalten.
func (s *CertificateService) Storno(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*SupplierCertificate, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Storno-Begründung erforderlich")
	}
	if _, err := s.Get(ctx, id, companyID); err != nil {
		return nil, err
	}
	ct, err := s.pg.Exec(ctx, `
        UPDATE supplier_certificates SET storniert_at=now(), storno_reason=$3, storniert_by_user_id=$4
         WHERE id=$1 AND company_id=$2 AND storniert_at IS NULL
    `, id, companyID, reason, nullableUser(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Zertifikat ist bereits storniert")
	}
	return s.Get(ctx, id, companyID)
}

func (s *CertificateService) bucket() (*gridfs.Bucket, error) {
	if s.mg == nil || s.mongoDB == "" {
		return nil, errors.New("MongoDB nicht konfiguriert")
	}
	return gridfs.NewBucket(s.mg.Database(s.mongoDB))
}

// cleanEvidenceFilename: nur der Dateiname ohne Pfad (auch Windows-Pfad);
// leer = "nachweis".
func cleanEvidenceFilename(name string) string {
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return "nachweis"
	}
	return name
}

// readEvidenceFile liest hoechstens MaxEvidenceDocumentBytes+1 Bytes und
// prueft Groesse und Art am Inhalt.
func readEvidenceFile(r io.Reader) ([]byte, string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxEvidenceDocumentBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > MaxEvidenceDocumentBytes {
		return nil, "", ErrEvidenceTooLarge
	}
	if len(data) == 0 {
		return nil, "", errors.New("Datei erforderlich")
	}
	contentType := http.DetectContentType(data)
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	if !evidenceDocumentTypes[contentType] {
		return nil, "", ErrEvidenceDocumentType
	}
	return data, contentType, nil
}

// UploadDocument speichert eine Datei zu einem Zertifikat; zu stornierten
// wird nichts mehr hochgeladen.
func (s *CertificateService) UploadDocument(ctx context.Context, certificateID uuid.UUID, r io.Reader, filename, actorUserID, companyID string) (*EvidenceDocument, error) {
	c, err := s.Get(ctx, certificateID, companyID)
	if err != nil {
		return nil, err
	}
	if c.StorniertAt != nil {
		return nil, errors.New("Zertifikat ist storniert")
	}
	data, contentType, err := readEvidenceFile(r)
	if err != nil {
		return nil, err
	}
	b, err := s.bucket()
	if err != nil {
		return nil, err
	}
	name := cleanEvidenceFilename(filename)
	oid, err := b.UploadFromStream(name, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	d := EvidenceDocument{ID: uuid.New(), DocumentID: oid.Hex(), Filename: name, ContentType: contentType, Length: int64(len(data))}
	if err := s.pg.QueryRow(ctx, `
        INSERT INTO supplier_certificate_documents (id, certificate_id, document_id, filename, content_type, length, uploaded_by_user_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING uploaded_at
    `, d.ID, certificateID, d.DocumentID, d.Filename, d.ContentType, d.Length, nullableUser(actorUserID)).Scan(&d.UploadedAt); err != nil {
		// Ohne Verweis waere die Datei verwaist - wieder entfernen.
		_ = b.Delete(oid)
		return nil, err
	}
	return &d, nil
}

// ListDocuments liefert die Dateien eines Zertifikats, aelteste zuerst.
func (s *CertificateService) ListDocuments(ctx context.Context, certificateID uuid.UUID, companyID string) ([]EvidenceDocument, error) {
	if _, err := s.Get(ctx, certificateID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, document_id, filename, content_type, length, uploaded_at
          FROM supplier_certificate_documents WHERE certificate_id=$1 ORDER BY uploaded_at, id
    `, certificateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EvidenceDocument{}
	for rows.Next() {
		var d EvidenceDocument
		if err := rows.Scan(&d.ID, &d.DocumentID, &d.Filename, &d.ContentType, &d.Length, &d.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// OpenDocument oeffnet eine Datei zum Download - nur, wenn sie zu genau
// diesem Zertifikat gehoert und das Zertifikat zum Mandanten.
func (s *CertificateService) OpenDocument(ctx context.Context, certificateID, documentID uuid.UUID, companyID string) (io.ReadCloser, *EvidenceDocument, error) {
	if _, err := s.Get(ctx, certificateID, companyID); err != nil {
		return nil, nil, err
	}
	var d EvidenceDocument
	err := s.pg.QueryRow(ctx, `
        SELECT id, document_id, filename, content_type, length, uploaded_at
          FROM supplier_certificate_documents WHERE id=$1 AND certificate_id=$2
    `, documentID, certificateID).Scan(&d.ID, &d.DocumentID, &d.Filename, &d.ContentType, &d.Length, &d.UploadedAt)
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
