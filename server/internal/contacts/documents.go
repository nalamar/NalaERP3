package contacts

import (
    "context"
    "errors"
    "io"
    "time"

    "github.com/google/uuid"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "go.mongodb.org/mongo-driver/mongo/gridfs"
)

type Document struct {
    ID          string    `json:"id"`
    ContactID   string    `json:"contact_id"`
    DocumentID  string    `json:"document_id"`
    Filename    string    `json:"filename"`
    ContentType string    `json:"content_type"`
    Length      int64     `json:"length"`
    UploadedAt  time.Time `json:"uploaded_at"`
}

func (s *Service) UploadContactDocument(ctx context.Context, contactID string, r io.Reader, filename, contentType string) (*Document, error) {
    if s.mg == nil || s.mongoDB == "" {
        return nil, errors.New("MongoDB nicht konfiguriert")
    }
    if s.pg == nil {
        return nil, errors.New("Postgres nicht konfiguriert")
    }
    if contactID == "" {
        return nil, errors.New("KontaktID erforderlich")
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

    var length int64
    var uploadedAt time.Time
    storedFilename := filename
    storedContentType := contentType

    var fileDoc struct {
        ID         primitive.ObjectID `bson:"_id"`
        Length     int64              `bson:"length"`
        UploadDate time.Time          `bson:"uploadDate"`
        Filename   string             `bson:"filename"`
    }
    if err := db.Collection("fs.files").FindOne(ctx, bson.M{"_id": oid}).Decode(&fileDoc); err == nil {
        length = fileDoc.Length
        uploadedAt = fileDoc.UploadDate
        if fileDoc.Filename != "" {
            storedFilename = fileDoc.Filename
        }
    } else {
        uploadedAt = time.Now()
    }

    id := uuid.NewString()
    _, err = s.pg.Exec(ctx, `
        INSERT INTO contact_documents (id, contact_id, document_id, filename, content_type, length, uploaded_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, id, contactID, oid.Hex(), storedFilename, storedContentType, length, uploadedAt)
    if err != nil {
        return nil, err
    }

    return &Document{
        ID:          id,
        ContactID:   contactID,
        DocumentID:  oid.Hex(),
        Filename:    storedFilename,
        ContentType: storedContentType,
        Length:      length,
        UploadedAt:  uploadedAt,
    }, nil
}

func (s *Service) ListContactDocuments(ctx context.Context, contactID string) ([]Document, error) {
    rows, err := s.pg.Query(ctx, `
        SELECT id, contact_id, document_id, filename, content_type, length, uploaded_at
        FROM contact_documents
        WHERE contact_id=$1
        ORDER BY uploaded_at DESC
    `, contactID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    out := make([]Document, 0)
    for rows.Next() {
        var d Document
        if err := rows.Scan(&d.ID, &d.ContactID, &d.DocumentID, &d.Filename, &d.ContentType, &d.Length, &d.UploadedAt); err != nil {
            return nil, err
        }
        out = append(out, d)
    }
    return out, nil
}

func (s *Service) OpenDocumentStream(ctx context.Context, documentID string) (io.ReadCloser, string, string, int64, error) {
    if s.mg == nil || s.mongoDB == "" {
        return nil, "", "", 0, errors.New("MongoDB nicht konfiguriert")
    }
    db := s.mg.Database(s.mongoDB)
    bucket, err := gridfs.NewBucket(db)
    if err != nil {
        return nil, "", "", 0, err
    }

    oid, err := primitive.ObjectIDFromHex(documentID)
    if err != nil {
        return nil, "", "", 0, errors.New("Ungültige Dokument-ID")
    }

    ds, err := bucket.OpenDownloadStream(oid)
    if err != nil {
        return nil, "", "", 0, err
    }

    var filename, contentType string
    var length int64
    if s.pg != nil {
        _ = s.pg.QueryRow(ctx, `
            SELECT filename, content_type, length
            FROM contact_documents
            WHERE document_id=$1
            LIMIT 1
        `, documentID).Scan(&filename, &contentType, &length)
    }
    return ds, filename, contentType, length, nil
}
