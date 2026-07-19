# GAEB-Freigabeanforderungen: Read-only Historienmodell-Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten sinnvollen Lesepfad fuer die Historie
positionsbezogener Freigabeanforderungen zu.

Ausgangspunkt:

- Aktive Anforderungen werden in der Quote-Detailantwort als
  `active_approval_request` sichtbar.
- `active_approval_request` zeigt bewusst nur `status = 'requested'`.
- Nach `approved`, `rejected` oder `cancelled` ist die Anforderung fachlich
  weiterhin relevant, verschwindet aber aus der aktiven Sicht.
- Die Persistenz speichert bereits alle erforderlichen Daten in
  `quote_item_approval_requests`.

Der naechste Schritt soll keine neue Mutation einfuehren. Ziel ist reine
Nachvollziehbarkeit.

## 1. Kleinster fachlicher Scope

Der erste Historienpfad soll pro Quote-Position eine chronologische Liste
aller Freigabeanforderungen liefern.

Enthalten:

- aktive Anforderungen
- genehmigte Anforderungen
- abgelehnte Anforderungen
- stornierte Anforderungen
- Request-Metadaten
- Entscheidungs- und Storno-Metadaten
- Preis-, Kosten-, Zielpreis- und Margensnapshots
- Verknuepfung zur verwendeten Preisentscheidung

Nicht enthalten:

- zentrale Freigabe-Queue
- Filter ueber alle Quotes hinweg
- Kommentardialoge
- erneutes Oeffnen entschiedener Anforderungen
- mehrstufige Freigaben
- Ereignis-/Audit-Timeline ueber andere Quote-Aktionen

## 2. Datenmodell

Keine neue Tabelle und keine Migration fuer den ersten Lesepfad.

Quelle:

```text
quote_item_approval_requests
```

Relevante Felder:

- `id`
- `quote_id`
- `quote_item_id`
- `status`
- `reason_code`
- `reason_text`
- `current_unit_price_snapshot`
- `cost_basis_unit_price_snapshot`
- `target_unit_price_snapshot`
- `target_margin_percent_snapshot`
- `target_difference_snapshot`
- `margin_percent_snapshot`
- `price_decision_id`
- `requested_by`
- `requested_at`
- `cancelled_by`
- `cancelled_at`
- `decided_by`
- `decided_at`
- `decision_comment`
- `approved_unit_price_snapshot`
- `approved_target_margin_percent_snapshot`
- `created_at`
- `updated_at`

Optional spaeter:

- Anzeigenamen fuer `requested_by`, `cancelled_by`, `decided_by` ueber
  `users.display_name`

Fuer den ersten Leaf reichen technische User-IDs. Anzeigenamen koennen
spaeter ohne Vertragsbruch ergaenzt werden.

## 3. Go-Readmodel

Das vorhandene Struct `QuoteItemApprovalRequest` enthaelt bereits die
noetigen Felder.

Der kleinste Service-Schnitt:

```text
ListApprovalRequestsForQuoteItem(ctx, quoteID, itemID uuid.UUID) ([]QuoteItemApprovalRequest, error)
```

Guards:

- Quote muss existieren.
- Position muss zur Quote gehoeren.
- Historische Angebotsversionen duerfen gelesen werden.
- Lesen ist nicht auf `draft` beschraenkt.

Begruendung:

- Historie ist Audit-/Nachvollziehbarkeitsinformation.
- Entschiedene Angebote und historische Revisionen muessen gerade lesbar
  bleiben.
- Der Service darf keine Sperre per `FOR UPDATE` brauchen, weil es nur eine
  Lesesicht ist.

Sortierung:

```sql
ORDER BY created_at DESC, requested_at DESC
```

Neueste fachliche Aktivitaet zuerst ist fuer UI und Audit am nuetzlichsten.

## 4. HTTP/API-Zuschnitt

Kleinster Endpoint:

```text
GET /api/v1/quotes/{id}/items/{itemID}/approval-requests
```

Berechtigung:

```text
quotes.read
```

Begruendung:

- Die Historie ist Lesedaten am Angebot.
- Eine Entscheidung bleibt durch `quotes.approve` geschuetzt, aber Lesen darf
  den normalen Angebotslesern offenstehen.
- Die API kollidiert nicht mit dem bestehenden
  `POST /approval-requests`-Endpoint, weil HTTP-Methode und Semantik klar
  getrennt sind.

Response:

```json
[
  {
    "id": "...",
    "quote_id": "...",
    "quote_item_id": "...",
    "status": "approved",
    "reason_code": "below_target_margin",
    "reason_text": "Zielmarge pruefen",
    "current_unit_price_snapshot": 59.9,
    "cost_basis_unit_price_snapshot": 59.9,
    "target_unit_price_snapshot": 71.88,
    "target_margin_percent_snapshot": 20,
    "target_difference_snapshot": -11.98,
    "margin_percent_snapshot": 0,
    "price_decision_id": "...",
    "requested_by": "...",
    "requested_at": "...",
    "decided_by": "...",
    "decided_at": "...",
    "decision_comment": "",
    "approved_unit_price_snapshot": 59.9,
    "approved_target_margin_percent_snapshot": 20,
    "created_at": "...",
    "updated_at": "..."
  }
]
```

Fehler:

- `400 Bad Request` bei ungueltiger Quote- oder Positions-ID
- `404` ist aktuell im Quote-Bereich nicht einheitlich getrennt; falls die
  bestehende Domainfehler-Klassifizierung `Angebotsposition nicht gefunden`
  als `400` behandelt, bleibt der erste Leaf konsistent beim bestehenden
  Verhalten
- `403 Forbidden` ohne `quotes.read`

## 5. Quote-Detail-Integration

Nicht im ersten Historien-Leaf.

Warum kein direktes Einbetten in `GET /quotes/{id}`:

- Quote-Detail ist bereits umfangreich.
- Historie kann bei mehrfachen Request-/Cancel-/Decision-Zyklen wachsen.
- Ein separater Lesepfad ist leichter zu testen und im Client nur bei Bedarf
  aufzurufen.

Spaeter kann optional ein kompaktes Feld ergaenzt werden:

```text
latest_approval_request
```

Der erste Schritt bleibt aber bewusst ein expliziter Detail-Endpoint.

## 6. Client-Zuschnitt

Der erste Client-Schritt nach Backend sollte nur die API-Methode liefern:

```text
getQuoteItemApprovalRequests(quoteId, itemId)
```

Eine UI-Historie ist danach ein separater Leaf.

Minimale spaetere UI:

- Button `Historie` im bestehenden Zielmargen-/Freigabeblock
- kleiner Dialog oder Inline-Abschnitt
- Status, Grund, Zeitpunkt, Entscheider/Stornierer
- Snapshots kompakt

Nicht im ersten Backend-Leaf:

- farbige Timeline
- Filter
- Queue
- Kommentarbearbeitung

## 7. Testschnitt

Backend-Integrationstest im bestehenden Quote-/Approval-Umfeld:

1. Quote mit Position und Preisentscheidung vorbereiten
2. Freigabeanforderung erzeugen
3. Genehmigen
4. `GET .../approval-requests` liefert einen Eintrag mit `approved`
5. zweite Quote/Position oder neuer Request nach Storno optional pruefen
6. Reihenfolge `created_at DESC`
7. Sales-/Read-Token mit `quotes.read` darf lesen

Wichtig:

- Der Test soll pruefen, dass terminale Anforderungen sichtbar bleiben.
- Der Test soll nicht auf UI-Verhalten warten.

## 8. Naechster Implementierungs-Leaf

```text
Subtask 3.1.35.2: Backend-Lesepfad fuer Freigabeanforderungshistorie implementieren
```

Umfang:

- Service-Methode `ListApprovalRequestsForQuoteItem(...)`
- HTTP-Endpoint `GET /api/v1/quotes/{id}/items/{itemID}/approval-requests`
- Integrationstest fuer terminale Sichtbarkeit

Nicht enthalten:

- Client-API
- UI-Historie
- Queue
- Kommentardialog
- neue Migration

