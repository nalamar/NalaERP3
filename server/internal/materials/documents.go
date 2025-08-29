package materials

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

type MaterialDocument struct {
    ID          string    `json:"id"`
    MaterialID  string    `json:"material_id"`
    DocumentID  string    `json:"document_id"` // GridFS ObjectID (hex)
    Filename    string    `json:"filename"`
    ContentType string    `json:"content_type"`
    Length      int64     `json:"length"`
    UploadedAt  time.Time `json:"uploaded_at"`
}

// UploadMaterialDocument speichert eine Datei in GridFS und verknüpft sie in Postgres.
func (s *Service) UploadMaterialDocument(ctx context.Context, materialID string, r io.Reader, filename, contentType string) (*MaterialDocument, error) {
    if s.mg == nil || s.mongoDB == "" { return nil, errors.New("MongoDB nicht konfiguriert") }
    if s.pg == nil { return nil, errors.New("Postgres nicht konfiguriert") }
    if materialID == "" { return nil, errors.New("MaterialID erforderlich") }

    db := s.mg.Database(s.mongoDB)
    bucket, err := gridfs.NewBucket(db)
    if err != nil { return nil, err }

    oid, err := bucket.UploadFromStream(filename, r)
    if err != nil { return nil, err }

    // Metadaten aus GridFS abfragen (Länge/UploadDate)
    var length int64 = 0
    var uploadedAt time.Time
    var storedFilename string = filename
    var storedContentType string = contentType

    var fileDoc struct {
        ID         primitive.ObjectID `bson:"_id"`
        Length     int64              `bson:"length"`
        UploadDate time.Time          `bson:"uploadDate"`
        Filename   string             `bson:"filename"`
    }
    if err := db.Collection("fs.files").FindOne(ctx, bson.M{"_id": oid}).Decode(&fileDoc); err == nil {
        length = fileDoc.Length
        uploadedAt = fileDoc.UploadDate
        if fileDoc.Filename != "" { storedFilename = fileDoc.Filename }
    } else {
        uploadedAt = time.Now()
    }

    id := uuid.NewString()
    // In Link-Tabelle speichern
    _, err = s.pg.Exec(ctx, `
        INSERT INTO material_documents (id, material_id, document_id, filename, content_type, length, uploaded_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, id, materialID, oid.Hex(), storedFilename, storedContentType, length, uploadedAt)
    if err != nil { return nil, err }

    return &MaterialDocument{
        ID: id, MaterialID: materialID, DocumentID: oid.Hex(), Filename: storedFilename,
        ContentType: storedContentType, Length: length, UploadedAt: uploadedAt,
    }, nil
}

// ListMaterialDocuments gibt die verknüpften Dokumente eines Materials aus Postgres zurück.
func (s *Service) ListMaterialDocuments(ctx context.Context, materialID string) ([]MaterialDocument, error) {
    rows, err := s.pg.Query(ctx, `
        SELECT id, material_id, document_id, filename, content_type, length, uploaded_at
        FROM material_documents
        WHERE material_id=$1
        ORDER BY uploaded_at DESC
    `, materialID)
    if err != nil { return nil, err }
    defer rows.Close()
    out := make([]MaterialDocument, 0)
    for rows.Next() {
        var d MaterialDocument
        if err := rows.Scan(&d.ID, &d.MaterialID, &d.DocumentID, &d.Filename, &d.ContentType, &d.Length, &d.UploadedAt); err != nil {
            return nil, err
        }
        out = append(out, d)
    }
    return out, nil
}

// OpenDocumentStream öffnet einen Download-Stream aus GridFS.
func (s *Service) OpenDocumentStream(ctx context.Context, documentID string) (io.ReadCloser, string, string, int64, error) {
    if s.mg == nil || s.mongoDB == "" { return nil, "", "", 0, errors.New("MongoDB nicht konfiguriert") }
    db := s.mg.Database(s.mongoDB)
    bucket, err := gridfs.NewBucket(db)
    if err != nil { return nil, "", "", 0, err }

    oid, err := primitive.ObjectIDFromHex(documentID)
    if err != nil { return nil, "", "", 0, errors.New("Ungültige Dokument-ID") }

    ds, err := bucket.OpenDownloadStream(oid)
    if err != nil { return nil, "", "", 0, err }
    // Dateiname/ContentType aus Postgres (falls vorhanden) lesen
    var filename, contentType string
    var length int64
    if s.pg != nil {
        _ = s.pg.QueryRow(ctx, `SELECT filename, content_type, length FROM material_documents WHERE document_id=$1 LIMIT 1`, documentID).Scan(&filename, &contentType, &length)
    }
    return ds, filename, contentType, length, nil
}
