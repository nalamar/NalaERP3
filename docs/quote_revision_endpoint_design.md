# Quote Revision Endpoint Design

## Ziel

Der erste Schreibpfad fuer Angebotsrevisionen soll bewusst klein bleiben:

- genau ein neuer Endpoint
- keine UI-Umsetzung im selben Schritt
- keine Parallelrevisionen
- keine PDF- oder Nummernkreis-Neukonzeption

Der Endpoint soll nur eine neue Quote-Version aus einer bestehenden Quote
ableiten und die alte Version als ueberholt markieren.

## Geplanter Endpoint

- `POST /api/v1/quotes/{id}/revise`

Berechtigung:

- `quotes.write`

Antwort:

- `201 Created`
- Body enthaelt:
  - `source_quote`
  - `revised_quote`

## Request-Scope im MVP

Im ersten Schritt sollte der Endpoint keinen grossen Request-Body benoetigen.

Empfehlung:

- leeres Body-Payload akzeptieren
- optional spaeter:
  - `note_append`
  - `valid_until`

Grund:

- der Endpoint soll zuerst nur sauberes Klonen und Guard Rails liefern
- inhaltliche Bearbeitung geschieht danach ueber den bestehenden Quote-Update-Pfad

## Guard Rails

### Erlaubt

- Quote existiert
- Quote ist aktuelle Version:
  - `superseded_by_quote_id IS NULL`
- Quote hat noch keinen Folgebeleg:
  - `linked_sales_order_id IS NULL`
  - `linked_invoice_out_id IS NULL`
- Quote-Status ist in einem bearbeitbaren Bereich:
  - `draft`
  - `sent`
  - optional auch `rejected`

### Nicht erlaubt

- Quote ist bereits durch neuere Revision ersetzt
- Quote wurde bereits in Auftrag ueberfuehrt
- Quote wurde bereits direkt in Rechnung ueberfuehrt
- Quote ist `accepted` und damit kaufmaennisch bereits weitergelaufen
- Quote hat inkonsistente Revisionsdaten

Empfohlene Fehlermeldungen:

- `Angebot wurde bereits revidiert`
- `Angebot mit Folgebeleg kann nicht revidiert werden`
- `Angenommene Angebote koennen nicht revidiert werden`
- `Revisionsfamilie inkonsistent`

## Schreiblogik

Der Ablauf sollte innerhalb einer DB-Transaktion erfolgen.

### Schritt 1: Ausgangsquote sperren

- `SELECT ... FOR UPDATE` auf der Quellquote

Zu laden:

- `id`
- `number`
- `root_quote_id`
- `revision_no`
- `superseded_by_quote_id`
- `linked_sales_order_id`
- `linked_invoice_out_id`
- `status`
- Kopfdaten

### Schritt 2: Guard Rails pruefen

- aktuelle Version?
- keine Folgebelege?
- erlaubter Status?

### Schritt 3: Revisionsnummer bestimmen

Empfehlung:

- `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM quotes WHERE root_quote_id = $1`

### Schritt 4: Neue Quote anlegen

Die neue Quote:

- bekommt eine neue `id`
- uebernimmt die gleiche `number`
- uebernimmt `root_quote_id`
- bekommt neue `revision_no`
- bekommt `status = 'draft'`
- bekommt `accepted_at = NULL`
- bekommt keine Folgebeleg-Links
- uebernimmt Projekt, Kontakt, Waehrung, Notiz, Datumsfelder und Summen

### Schritt 5: Positionen kopieren

- `quote_items` 1:1 in neue Quote kopieren
- neue Item-IDs
- gleiche Reihenfolge und Werte

### Schritt 6: Ausgangsquote markieren

- `UPDATE quotes SET superseded_by_quote_id = $newID WHERE id = $oldID`

Wichtig:

- der alte Status bleibt erhalten
- Historie bleibt lesbar

## Nummerierung im MVP

Die Quote-Nummer bleibt absichtlich unveraendert.

Das heisst:

- alle Revisionen einer Familie teilen dieselbe kaufmaennische Hauptnummer
- Unterscheidung erfolgt ueber `revision_no`

Anzeige spaeter im UI:

- `ANG-2026-0042 / Rev. 1`
- `ANG-2026-0042 / Rev. 2`

## Auswirkungen auf bestehende Endpoints

### Bereits im naechsten technischen Schritt sinnvoll

- `GET /api/v1/quotes/{id}`
  - liefert schon Revisionsmetadaten
- `GET /api/v1/quotes/`
  - liefert schon Revisionsmetadaten

### Im darauffolgenden Schritt noetig

- `PATCH /api/v1/quotes/{id}`
  - Historienversionen sperren
- `POST /api/v1/quotes/{id}/status`
  - nur auf aktueller Version erlauben
- `POST /api/v1/quotes/{id}/convert-to-sales-order`
  - nur auf aktueller Version erlauben
- `POST /api/v1/quotes/{id}/convert-to-invoice`
  - nur auf aktueller Version erlauben

## Test-MVP

Mindestens ein Integrationstest fuer den neuen Endpoint:

- Quote anlegen
- `POST /api/v1/quotes/{id}/revise`
- pruefen:
  - neue Quote existiert
  - gleiche `number`
  - gleiche `root_quote_id`
  - `revision_no` alt = 1, neu = 2
  - neue Quote `status = draft`
  - alte Quote `superseded_by_quote_id = neue id`
  - Items wurden kopiert

Zusatztests mit gutem Signal:

- zweiter Revisionsversuch auf bereits ersetzter Quote -> `400`
- Revisionsversuch auf Quote mit `linked_sales_order_id` -> `400`
- Revisionsversuch auf Quote mit `linked_invoice_out_id` -> `400`

## Bewusst nicht im selben Schritt

- Revisionsliste im UI
- Button `Neue Revision` in `quotes_page.dart`
- Default-Filter auf nur aktuelle Versionen
- Diff-Ansicht zwischen Revisionen
- PDF-Label `Rev. X`

## Nächster sinnvoller technischer Schritt

Der naechste technische Schritt soll exakt diesen kleinen Scope umsetzen:

1. Service-Methode `Revise(ctx, id)` im Quote-Service
2. Route `POST /api/v1/quotes/{id}/revise`
3. 1-3 Integrationstests fuer Erfolg und Guard Rails
