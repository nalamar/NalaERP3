package purchasing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Lieferpapiere an der Bestellung (ADR 0035 §4, Backlog H.4.3a):
// Leistungserklaerung, Zeugnisse EN 10204, Lieferschein. Tabelle in
// Migration 110. Nie loeschen, nur stornieren; Dateien in GridFS, nicht
// ueber den allgemeinen Download (§6). Dateipruefung wie bei den
// Zertifikaten (certificates.go).

type DeliveryPaperDocument struct {
	ID           uuid.UUID  `json:"id"`
	OrderID      string     `json:"bestellung_id"`
	ItemID       *string    `json:"position_id"`
	Art          string     `json:"art"`
	Charge       string     `json:"charge"`
	Notiz        string     `json:"notiz"`
	DocumentID   string     `json:"document_id"`
	Filename     string     `json:"filename"`
	ContentType  string     `json:"content_type"`
	Length       int64      `json:"length"`
	StornoReason string     `json:"storno_grund"`
	StorniertAt  *time.Time `json:"storniert_am"`
	UploadedAt   time.Time  `json:"uploaded_at"`
}

type DeliveryPaperInput struct {
	Art    string
	ItemID *string
	Charge string
	Notiz  string
}

type PaperService struct {
	pg      *pgxpool.Pool
	mg      *mongo.Client
	mongoDB string
}

func NewPaperService(pg *pgxpool.Pool, mg *mongo.Client, mongoDB string) *PaperService {
	return &PaperService{pg: pg, mg: mg, mongoDB: mongoDB}
}

const paperColumns = `id, purchase_order_id, purchase_order_item_id, art, charge, notiz, document_id, filename, content_type, length, storno_reason, storniert_at, uploaded_at`

func scanPaper(row pgx.Row) (*DeliveryPaperDocument, error) {
	var p DeliveryPaperDocument
	if err := row.Scan(&p.ID, &p.OrderID, &p.ItemID, &p.Art, &p.Charge, &p.Notiz, &p.DocumentID, &p.Filename, &p.ContentType, &p.Length, &p.StornoReason, &p.StorniertAt, &p.UploadedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// orderStatus liefert den Status einer Bestellung des Mandanten.
func (s *PaperService) orderStatus(ctx context.Context, orderID, companyID string) (string, error) {
	var status string
	err := s.pg.QueryRow(ctx, `SELECT status FROM purchase_orders WHERE id=$1 AND company_id=$2`, orderID, companyID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("Bestellung nicht gefunden")
	}
	return status, err
}

// Upload speichert ein Lieferpapier zu einer Bestellung, wahlweise zu
// einer ihrer Positionen und mit Charge. Zu stornierten Bestellungen
// (canceled) kommt nichts mehr hinzu; sonst in jedem Status (ANNAHME:
// Papiere koennen vor, mit oder nach der Ware kommen).
func (s *PaperService) Upload(ctx context.Context, orderID string, in DeliveryPaperInput, r io.Reader, filename, actorUserID, companyID string) (*DeliveryPaperDocument, error) {
	status, err := s.orderStatus(ctx, orderID, companyID)
	if err != nil {
		return nil, err
	}
	if status == "canceled" {
		return nil, errors.New("Bestellung ist storniert")
	}
	art := strings.ToLower(strings.TrimSpace(in.Art))
	if !ValidPapierArt(art) {
		return nil, errors.New("Ungültige Art des Lieferpapiers")
	}
	var itemID *string
	if in.ItemID != nil && strings.TrimSpace(*in.ItemID) != "" {
		id := strings.TrimSpace(*in.ItemID)
		var ok bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_order_items WHERE id=$1 AND order_id=$2)`, id, orderID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("Position nicht gefunden")
		}
		itemID = &id
	}
	data, contentType, err := readEvidenceFile(r)
	if err != nil {
		return nil, err
	}
	b, err := evidenceBucket(s.mg, s.mongoDB)
	if err != nil {
		return nil, err
	}
	name := cleanEvidenceFilename(filename)
	oid, err := b.UploadFromStream(name, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	p, err := scanPaper(s.pg.QueryRow(ctx, `
        INSERT INTO purchase_order_documents (id, purchase_order_id, purchase_order_item_id, art, charge, notiz, document_id, filename, content_type, length, uploaded_by_user_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        RETURNING `+paperColumns,
		uuid.New(), orderID, itemID, art, strings.TrimSpace(in.Charge), strings.TrimSpace(in.Notiz), oid.Hex(), name, contentType, int64(len(data)), nullableUser(actorUserID)))
	if err != nil {
		// Ohne Verweis waere die Datei verwaist - wieder entfernen.
		_ = b.Delete(oid)
		return nil, err
	}
	return p, nil
}

// List liefert die Lieferpapiere einer Bestellung, aelteste zuerst;
// stornierte nur auf Wunsch.
func (s *PaperService) List(ctx context.Context, orderID string, includeStorniert bool, companyID string) ([]DeliveryPaperDocument, error) {
	if _, err := s.orderStatus(ctx, orderID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `SELECT `+paperColumns+` FROM purchase_order_documents
        WHERE purchase_order_id=$1 AND ($2 OR storniert_at IS NULL) ORDER BY uploaded_at, id`, orderID, includeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeliveryPaperDocument{}
	for rows.Next() {
		p, err := scanPaper(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// get: ein Lieferpapier genau dieser Bestellung im Mandanten.
func (s *PaperService) get(ctx context.Context, orderID string, paperID uuid.UUID, companyID string) (*DeliveryPaperDocument, error) {
	if _, err := s.orderStatus(ctx, orderID, companyID); err != nil {
		return nil, err
	}
	p, err := scanPaper(s.pg.QueryRow(ctx, `SELECT `+paperColumns+` FROM purchase_order_documents WHERE id=$1 AND purchase_order_id=$2`, paperID, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Lieferpapier nicht gefunden")
	}
	return p, err
}

// Storno ist einmalig und schreibt nur die Storno-Spalten; die Datei
// bleibt lesbar.
func (s *PaperService) Storno(ctx context.Context, orderID string, paperID uuid.UUID, reason, actorUserID, companyID string) (*DeliveryPaperDocument, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Storno-Begründung erforderlich")
	}
	if _, err := s.get(ctx, orderID, paperID, companyID); err != nil {
		return nil, err
	}
	ct, err := s.pg.Exec(ctx, `
        UPDATE purchase_order_documents SET storniert_at=now(), storno_reason=$3, storniert_by_user_id=$4
         WHERE id=$1 AND purchase_order_id=$2 AND storniert_at IS NULL
    `, paperID, orderID, reason, nullableUser(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Lieferpapier ist bereits storniert")
	}
	return s.get(ctx, orderID, paperID, companyID)
}

// Open oeffnet die Datei eines Lieferpapiers zum Download.
func (s *PaperService) Open(ctx context.Context, orderID string, paperID uuid.UUID, companyID string) (io.ReadCloser, *DeliveryPaperDocument, error) {
	p, err := s.get(ctx, orderID, paperID, companyID)
	if err != nil {
		return nil, nil, err
	}
	oid, err := primitive.ObjectIDFromHex(p.DocumentID)
	if err != nil {
		return nil, nil, errors.New("Lieferpapier nicht gefunden")
	}
	b, err := evidenceBucket(s.mg, s.mongoDB)
	if err != nil {
		return nil, nil, err
	}
	rc, err := b.OpenDownloadStream(oid)
	if err != nil {
		return nil, nil, err
	}
	return rc, p, nil
}
