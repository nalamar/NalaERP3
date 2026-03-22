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

## API-Integrationstests

Für die ersten API-Integrationstests gibt es eine separate Compose-Umgebung in `docker-compose.test.yml`.

1. Test-Abhängigkeiten starten:

```bash
docker compose -f docker-compose.test.yml up -d
```

2. Test-Umgebung setzen:

```bash
set NALA_INTEGRATION=1
set TEST_POSTGRES_DSN=postgres://nala:secret@localhost:55432/nalaerp_test?sslmode=disable
set TEST_MONGO_URI=mongodb://localhost:57017
set TEST_MONGO_DB=nalaerp_test
set TEST_REDIS_ADDR=localhost:56379
```

3. Tests ausführen:

```bash
cd server
go test ./internal/http ./internal/...
```

Aktuell ist der erste End-to-End-Check für `GET /readyz` und `GET /livez` vorbereitet. Weitere API-Integrationstests sollten auf demselben Helper unter `server/internal/testutil` aufsetzen.

## CI

Eine erste GitHub-Actions-Pipeline liegt unter `.github/workflows/ci.yml`.

- `Server`: prüft Go-Formatierung, führt `go test ./...` aus und baut `./cmd/api`.
- `Integration`: startet `docker-compose.test.yml`, setzt `NALA_INTEGRATION=1` plus die `TEST_*`-Variablen und führt die HTTP-Integrationstests aus.
- `Client`: führt `flutter analyze`, `flutter test` und `flutter build web --release --no-wasm-dry-run` aus.
- `Docker`: baut `server/Dockerfile` und `client/Dockerfile` in CI einmal vollständig durch, ohne Images zu veröffentlichen.

Die Compose-basierten API-Integrationstests laufen damit bewusst getrennt vom schnellen Basis-Serverjob, damit Unit- und Integrationsfeedback unabhängig bleiben.
Zusätzlich erkennt die Pipeline geänderte Pfade vorab und startet Server-, Client-, Integrations- und Docker-Jobs nur dann, wenn die jeweils relevanten Dateien betroffen sind.

## Client-Verifikation lokal

Falls `dart.bat` oder `flutter.bat` lokal hängen, gibt es im Repo einen direkten Wrapper über `dart.exe` und `flutter_tools.snapshot`:

```powershell
pwsh -File .\scripts\client_tooling.ps1 -Action format -Paths client/lib/pages/sales_orders_page.dart,client/test/sales_order_context_pages_test.dart
pwsh -File .\scripts\client_tooling.ps1 -Action test -TestTarget test/sales_order_context_pages_test.dart
```

Weitere unterstützte Aktionen:

- `pwsh -File .\scripts\client_tooling.ps1 -Action analyze`
- `pwsh -File .\scripts\client_tooling.ps1 -Action build-web`

Hinweise:

- Das Skript sucht das Flutter-SDK zuerst über `FLUTTER_ROOT`, danach unter `C:\Projekte\flutter`, dann über `flutter` im `PATH`.
- `format` läuft vom Repo-Root, `test`/`analyze`/`build-web` automatisch aus `client/`.
- Standard-Testziel für `-Action test` ist `test/sales_order_context_pages_test.dart`.

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

## PDF-Templates & Bestellungen als PDF

Einstellungen für PDF-Templates werden in Postgres gepflegt, Bilder (Logo/Seitenhintergründe) in MongoDB GridFS gespeichert.

- GET `/api/v1/settings/pdf/{entity}` – Template lesen (z. B. `purchase_order`).
- PUT `/api/v1/settings/pdf/{entity}` – Kopf-/Fußtext und Start-Höhen setzen (`top_first_mm`, `top_other_mm`).
- POST `/api/v1/settings/pdf/{entity}/upload/{kind}` – Bild hochladen (`kind` = `logo` | `bg-first` | `bg-other`; Feld `file`).
- DELETE `/api/v1/settings/pdf/{entity}/upload/{kind}` – Bild entfernen.

Bestellung als PDF generieren:

- GET `/api/v1/purchase-orders/{id}/pdf` – liefert `application/pdf` (Dateiname `Bestellung_<Nummer>.pdf`).

Hinweise:

- Seitenhintergründe können separat für erste und folgende Seiten gesetzt werden (`bg-first`, `bg-other`).
- Der Druckbereich beginnt auf Seite 1 bei `top_first_mm` und auf Folgeseiten bei `top_other_mm`.
- Ist ein Seitenhintergrund gesetzt, wird er ganzseitig vor dem Inhalt gezeichnet; Logo/Kopftext werden weiterhin angezeigt (optional entfernbar, indem Kopftext leer bleibt und Logo entfernt wird).

Client-UI:

- Unter `Einstellungen` gibt es einen Abschnitt „PDF-Templates“ (Bestellungen), um Kopf-/Fußtext, Start-Höhen sowie Logo und Seitenhintergründe zu pflegen.
- In der Bestell-Detailansicht befindet sich ein PDF-Button in der App-Bar zum direkten Download.

## Logikal-Import & Re-Import

Quelle: SQLite-Export aus LogiKal (Struktur siehe `logi.sql`). Der Import liest Projekt-Metadaten, Lose (Phasen), Positionen (Elevations), Varianten (SingleElevations) und Materiallisten.

- Projektidentifikation: per Angebots-/Auftragsnummer oder `xGUID`. Bei Re-Import wird das bestehende Projekt aktualisiert.
- Lose (Phasen):
  - Ableitung über `ElevationGroups.PhaseId`. Für jede Phase wird ein Los mit Nummer = `PhaseId` angelegt/aktualisiert.
  - Re-Import: Lose mit numerischer Nummer, die im aktuellen Import nicht vorkommen, werden gelöscht (inkl. abhängiger Positionen/Varianten). Nicht-numerische Lose bleiben unangetastet.
- Positionen (Elevations):
  - Zuordnung zu Los über `ElevationGroupId -> PhaseId` (ein Los kann mehrere Positionen haben).
  - Matching-Reihenfolge: `external_guid` innerhalb des Loses, danach Name innerhalb des Loses; ansonsten wird eine neue Position mit laufender Nummer im Los angelegt.
  - Re-Import-Bereinigung:
    - In allen vom Import betroffenen Losen werden Positionen gelöscht, die im aktuellen Import nicht mehr vorkommen.
    - Alte Dubletten mit gleicher `external_guid` in anderen Losen werden entfernt (z. B. nach korrigierter Zuordnung).
- Varianten (SingleElevations):
  - Varianten hängen an der Hauptelevation einer Gruppe (Alternative==0 bevorzugt).
  - Matching über `external_guid`, sonst Name. Fehlende Varianten werden angelegt, bestehende aktualisiert.
  - Re-Import: Varianten, die zu einer Elevation nicht mehr im Export vorkommen, werden gelöscht.
- Materiallisten (Profile/Artikel/Glas):
  - Pro Variante wird die Materialliste vollständig ersetzt. Vorheriger Zustand wird fürs Undo protokolliert.
- Änderungsprotokoll und Undo:
  - Jeder Import erzeugt einen Import-Run mit Change-Logs (`created|updated|deleted|replaced` für `phase|elevation|variant|materials`).
  - UI/API: Auflistung unter Projekt → „Importe“; Änderungen je Import einsehbar. Ein Undo stellt den vorherigen Zustand (soweit protokolliert) wieder her.

Hinweis: Die Löschregeln sind bewusst konservativ – nur Artefakte, die eindeutig nicht mehr im aktuellen Export vorkommen, oder alte Dubletten per `external_guid`, werden entfernt.
