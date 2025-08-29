# NalaERP3 – Grundgerüst

Deutschsprachiges ERP für die Metallbau-Branche. Client-Server-Architektur mit Go (API) und Flutter (Web/Desktop). Alle Komponenten sind UTF-8-kompatibel und containerisiert.

## Schnellstart

1. `.env` anlegen (siehe `.env.example`).
2. Docker bauen und starten:
   
   ```bash
   docker compose up --build
   ```

3. Dienste:
   - API: http://localhost:8080 (`/healthz`, `/version`)
   - Client (Flutter Web): http://localhost:3000
   - PostgreSQL: `localhost:5432` (DB `nalaerp`, User `nala`, PW `secret`)
   - MongoDB: `localhost:27017`
   - Redis: `localhost:6379`

## Struktur

- `server/`: Go 1.24 API (Docker Multi-Stage)
- `client/`: Flutter Web-Client (Entwicklung auf Port 3000)
- `docker-compose.yml`: Orchestrierung (Postgres, Mongo, Redis, API, Client)

## Hinweise

- Sprachen/Encoding: Standard `de-DE`, UTF-8.
- Versionen: Aktuelle Hauptversionen (Go 1.24, Postgres 17, Mongo 7, Redis 7).
- Nächste Schritte: Materialverwaltung Domainmodell + REST-Endpunkte, DB-Schemata (Migrations), Uploads (Mongo GridFS), Auth (JWT + Redis), Tests.

## Dokumente (Upload/Download)

Dokumente werden in MongoDB GridFS (Bucket `fs`) gespeichert und in Postgres über die Tabelle `material_documents` mit Materialien verknüpft.

Voraussetzungen:

- `.env` enthält `MONGO_URI` und `MONGO_DB` (siehe `.env.example`).
- API startet die DB-Migrationen automatisch.

Endpunkte:

- POST `/api/v1/materials/{id}/documents` – Upload (multipart/form-data, Feld `file`).
- GET `/api/v1/materials/{id}/documents` – Auflistung der verknüpften Dokumente.
- GET `/api/v1/documents/{docID}` – Download per GridFS ObjectID (Hex).

Beispiele (curl):

1) Material anlegen und `id` merken

```bash
MAT_ID=$(curl -sS -X POST http://localhost:8080/api/v1/materials/ \
  -H 'Content-Type: application/json' \
  -d '{
        "nummer":"MAT-0001",
        "bezeichnung":"Aluminiumblech",
        "typ":"rohstoff",
        "norm":"",
        "werkstoffnummer":"",
        "einheit":"kg",
        "dichte":2.7,
        "kategorie":"metall",
        "attribute":{}
      }' | jq -r .id)
echo "$MAT_ID"
```

2) Dokument zu Material hochladen

```bash
curl -sS -X POST http://localhost:8080/api/v1/materials/$MAT_ID/documents \
  -F file=@/path/zertifikat.pdf
# Antwort enthält u.a. fields: id, document_id (GridFS), filename, content_type, length
```

3) Dokumente eines Materials auflisten

```bash
curl -sS http://localhost:8080/api/v1/materials/$MAT_ID/documents | jq
```

4) Dokument herunterladen (per `document_id` aus der Liste)

```bash
DOC_ID=<document_id_hex>
curl -sS -L http://localhost:8080/api/v1/documents/$DOC_ID -o download.bin
```

Hinweise:

- Upload-Feldname ist `file`. Max. In-Memory-Größe derzeit 32 MB (Server kann größere Dateien streamen; je nach Proxy anpassen).
- Content-Type beim Download wird aus Postgres übernommen, sonst `application/octet-stream`.
