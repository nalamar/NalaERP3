# GAEB-Importlauf: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument beschreibt das kleinste technische Zielmodell fuer einen
spaeteren GAEB-Importpfad in der Angebotsvorbereitung.

Der Scope bleibt bewusst eng:

- Upload einer Quelldatei
- Metadaten und Status eines Importlaufs
- Review-Anker fuer spaetere Parser- und Review-Schritte

Nicht Teil dieses Schritts:

- Parsing von GAEB-Dateien
- Speicherung geparster Positionsstrukturen
- automatische Quote-Erzeugung
- KI-Logik

## 1. Kernentscheidung

Der erste technische GAEB-Pfad soll als **eigener Importlauf** modelliert
werden und nicht als direkter Side-Effect auf `quotes`.

Empfohlenes neues fachliches Aggregat:

- `quote_imports`

Dieses Aggregat ist zunaechst nur der Container fuer:

- Quelldatei
- fachlichen Kontext
- Verarbeitungsstatus
- Review-Anker

## 2. Warum zunaechst nur `quote_imports`

Ein frueher Einstieg mit `quote_import_items` waere noch zu frueh, weil
aktuell weder Parser noch stabile Normalisierungslogik vorhanden sind.

Deshalb soll Phase 1 nur das Import-Run-Modell schaffen:

- Datei annehmen
- Kontext speichern
- Status sichtbar machen
- spaeter an Parser/Review andocken

Damit bleibt der erste Pfad klein und reversibel.

## 3. Empfohlenes Tabellenmodell

### 3.1 Tabelle `quote_imports`

Empfohlenes Minimalfeldset:

- `id UUID PRIMARY KEY`
- `project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE`
- `contact_id UUID NULL REFERENCES contacts(id) ON DELETE SET NULL`
- `source_kind TEXT NOT NULL`
- `source_filename TEXT NOT NULL`
- `source_document_id TEXT NOT NULL`
- `status TEXT NOT NULL`
- `parser_version TEXT NOT NULL DEFAULT ''`
- `detected_format TEXT NOT NULL DEFAULT ''`
- `error_message TEXT NOT NULL DEFAULT ''`
- `created_quote_id UUID NULL REFERENCES quotes(id) ON DELETE SET NULL`
- `uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

### 3.2 Bedeutung der Felder

- `project_id`
  - importierter LV-Kontext haengt zuerst an einem Projekt
- `contact_id`
  - optionaler kaufmaennischer Kontext; nicht jeder erste Upload muss bereits
    einen finalen Ansprechpartner tragen
- `source_kind`
  - im MVP eng begrenzt auf `gaeb`
- `source_filename`
  - urspruenglicher Dateiname fuer Anzeige und Nachvollziehbarkeit
- `source_document_id`
  - Verweis auf GridFS-Dokument
- `status`
  - technischer und fachlicher Bearbeitungsstand
- `parser_version`
  - spaeter wichtig fuer reproduzierbare Parsing-Laeufe; im MVP leer erlaubt
- `detected_format`
  - spaeter fuer `x83`, `x84`, `gaeb-xml` etc.; im MVP leer erlaubt
- `error_message`
  - Platz fuer spaetere Parser-/Validierungsfehler
- `created_quote_id`
  - spaeterer Rueckverweis, wenn aus dem Importlauf eine Quote entstanden ist

## 4. Minimaler Statusraum

Der erste Statusraum bleibt klein:

- `uploaded`
- `parsed`
- `reviewed`
- `applied`
- `failed`

MVP-Regel fuer die erste technische Stufe:

- neue Importlaeufe starten immer in `uploaded`
- alle anderen Stati werden erst in spaeteren Ausbaustufen aktiv verwendet

## 5. Storage-Entscheidung

### 5.1 Quelldatei in GridFS

Die Quelldatei sollte nicht in Postgres als Blob abgelegt werden, sondern
analog zu bestehenden Dokumentpfaden in GridFS.

Begruendung:

- vorhandenes Upload-Muster im Repo
- Dateidownload spaeter direkt moeglich
- Trennung zwischen Dateispeicher und fachlichen Metadaten bleibt sauber

### 5.2 Metadaten in Postgres

Alle fachlichen Importmetadaten gehoeren in Postgres:

- Kontext
- Status
- Review-Anker
- spaeterer Quote-Bezug

Damit folgt der GAEB-Pfad denselben Grundsaetzen wie andere fachliche Flows.

## 6. HTTP-Zielbild fuer die erste technische Stufe

Der erste technische Pfad soll bewusst klein bleiben.

Empfohlene erste Endpunkte:

- `POST /api/v1/quotes/imports/gaeb`
- `GET /api/v1/quotes/imports`
- `GET /api/v1/quotes/imports/{id}`

Optional spaeter:

- `GET /api/v1/quotes/imports/{id}/document`

### 6.1 Bedeutung

- `POST`
  - Datei hochladen und Importlauf anlegen
- `GET /imports`
  - kleine Uebersicht aller Importlaeufe
- `GET /imports/{id}`
  - Detailansicht fuer Status, Quelldatei, Projekt/Kontakt und spaeter Parser-
    bzw. Review-Ergebnisse

## 7. Minimaler Request fuer den Upload

Der erste Upload-Request sollte noch kein komplexes JSON-Modell sein, sondern
ein kleines Multipart-Formular mit:

- `file`
- `project_id`
- optional `contact_id`

Bewusst noch nicht erforderlich:

- parser options
- format override
- price profile
- import mode

## 8. Minimaler Response-Shape

Die erste Antwort nach Upload darf klein bleiben:

```json
{
  "id": "uuid",
  "project_id": "uuid",
  "contact_id": "uuid-or-empty",
  "source_kind": "gaeb",
  "source_filename": "lv.x83",
  "source_document_id": "gridfs-id",
  "status": "uploaded",
  "detected_format": "",
  "error_message": "",
  "created_quote_id": "",
  "uploaded_at": "2026-04-06T10:00:00Z",
  "updated_at": "2026-04-06T10:00:00Z"
}
```

## 9. Rechte- und Sichtbarkeitsidee

Fuer die erste technische Stufe sollte kein neues separates Rollenmodell
erfunden werden.

Pragmatischer Einstieg:

- Upload nur mit `quotes.write`
- Lesen mit `quotes.read`

Wenn spaeter ein eigener Import-/Review-Bereich entsteht, kann ein separater
Permission-Schnitt folgen.

## 10. Abgrenzung zur spaeteren Parser-Stufe

Dieses Modell ist bewusst nur der Review-Anker.

Erst in einer spaeteren Stufe kommen dazu:

- `quote_import_items`
- strukturierte LV-Hierarchie
- Mengeneinheiten-Normalisierung
- Parserfehler pro Position
- manuelle Freigabe einzelner Positionen

## 11. Nächster sinnvoller technischer Schritt

Nach diesem Strategiedokument ist der nächste sinnvolle Umsetzungsschritt:

1. Migration fuer `quote_imports`
2. kleiner Service fuer `Create/List/Get`
3. Upload in GridFS analog zu bestehenden Dokumentpfaden
4. kleine HTTP-Route fuer `POST /api/v1/quotes/imports/gaeb`
5. noch keine Parserlogik

## 12. Entscheidung

Der erste technische GAEB-Pfad wird bewusst auf **Importlauf + Quelldatei +
Metadaten + Review-Anker** begrenzt.

Das ist klein genug fuer einen risikoarmen Einstieg und stabil genug, um
spaeter Parser-, Review- und KI-Logik darauf aufzubauen.
