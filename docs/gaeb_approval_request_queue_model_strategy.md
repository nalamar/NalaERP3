# GAEB-Freigabe: Zielmodell fuer read-only Freigabeanforderungs-Queue

## Ziel

Subtask 3.1.50.2 schneidet das technische Minimalmodell fuer eine read-only
Queue aktiver Freigabeanforderungen zu.

Die Queue soll offene Entscheidungen systemweit bzw. projektbezogen sichtbar
machen. Sie fuehrt keine Genehmigung, Ablehnung, Zuweisung, Eskalation oder
neue Persistenz ein.

## Fachliche Definition

Ein Queue-Eintrag ist genau dann sichtbar, wenn fuer eine Quote-Position eine
aktive Freigabeanforderung existiert:

```text
quote_item_approval_requests.status = 'requested'
```

Damit gilt:

- `requested` ist offene Freigabearbeit
- `approved` ist erledigte Freigabe
- `rejected` ist keine offene Freigabe mehr, sondern fuehrt danach in
  Nacharbeit
- `rework_resolved` gehoert zur Nacharbeitsauflösung
- `cancelled` ist nicht mehr entscheidungspflichtig

Die neue Queue ist deshalb bewusst getrennt von:

```text
GET /api/v1/quotes/approval-rework
```

`approval-requests` zeigt offene Entscheidungen vor Genehmigung oder
Ablehnung. `approval-rework` zeigt offene Nacharbeit nach Ablehnung.

## Endpoint

Empfohlener erster Endpoint:

```text
GET /api/v1/quotes/approval-requests
```

Begruendung:

- Die Daten gehoeren fachlich zur Angebots- und Positionsfreigabe.
- Der bestehende positionsnahe Historien-Endpunkt bleibt unveraendert:
  `GET /api/v1/quotes/{id}/items/{itemID}/approval-requests`.
- Eine globale Queue unter `/quotes/approval-requests` ist analog zur
  Nacharbeits-Queue unter `/quotes/approval-rework`.
- Das kommerzielle Workflow-Cockpit bleibt in diesem MVP unangetastet.

## Berechtigung

MVP-Berechtigung:

```text
quotes.read
```

Begruendung:

- Die Route ist read-only.
- Die enthaltenen Informationen sind fuer Nutzer mit Quote-Leserecht bereits
  ueber Quote-Detail und positionsbezogene Freigabehistorie abrufbar.
- Vertrieb und Kalkulation sollen offene Freigabe-Blocker sehen koennen.
- Die eigentlichen Entscheidungen bleiben weiterhin strikt unter
  `quotes.approve`.

Spaetere Rollenverfeinerung:

- `quotes.approve` als Queue-Leserecht fuer reine Freigebende
- oder ein separates Recht wie `quotes.approval_queue.read`

Das ist nicht Teil des ersten Queue-MVP.

## Query-Parameter

Zulaessige Filter im MVP:

- `project_id`
- `contact_id`
- `quote_id`

Parsing:

- `project_id` als UUID validieren
- `quote_id` als UUID validieren
- `contact_id` bleibt analog zum bestehenden Kontaktvertrag ein String
- leere Werte werden ignoriert
- ungueltige UUIDs liefern `400 validation_error`

Bewusst nicht vorgesehen:

- freie Suche
- Statusfilter
- Sortierparameter
- Pagination
- Filter nach Anforderer
- Filter nach Zielstatus
- SLA- oder Altersklassen

## Sortierung

Feste Server-Sortierung im MVP:

```text
requested_at ASC, quote_number ASC, position ASC
```

Begruendung:

- aelteste offene Freigabe zuerst
- stabile Arbeitslistenreihenfolge
- keine fruehe fachliche Priorisierungsregel nach Wert, Kunde oder
  Zielabweichung

## Response-Form

Antwort:

```json
{
  "items": [
    {
      "quote_id": "...",
      "quote_number": "AN-2026-0012",
      "quote_status": "draft",
      "quote_date": "2026-04-03T00:00:00Z",
      "project_id": "...",
      "project_name": "Projekt A",
      "contact_id": "...",
      "contact_name": "Kunde GmbH",
      "quote_item_id": "...",
      "position": 3,
      "description": "Stahlbauposition",
      "current_unit_price": 72,
      "currency": "EUR",
      "approval_request_id": "...",
      "reason_code": "below_target_margin",
      "reason_text": "Zielmarge pruefen",
      "requested_by": "...",
      "requested_by_name": "Max Mustermann",
      "requested_at": "2026-04-04T09:12:00Z",
      "current_unit_price_snapshot": 50,
      "cost_basis_unit_price_snapshot": 60,
      "target_unit_price_snapshot": 75,
      "target_margin_percent_snapshot": 20,
      "target_difference_snapshot": -25,
      "margin_percent_snapshot": -20,
      "price_decision_id": "...",
      "current_target_status": "below_target",
      "current_target_difference": -3
    }
  ]
}
```

## Feldgruppen

### Quote-Kontext

- `quote_id`
- `quote_number`
- `quote_status`
- `quote_date`
- `project_id`
- `project_name`
- `contact_id`
- `contact_name`

Quelle:

- `quotes`
- `projects`
- `contacts`

Historische Angebotsversionen werden ausgeschlossen:

```text
q.superseded_by_quote_id IS NULL
```

### Positions-Kontext

- `quote_item_id`
- `position`
- `description`
- `current_unit_price`
- `currency`

Quelle:

- `quote_items`
- `quotes.currency`

### offene Anforderung

- `approval_request_id`
- `reason_code`
- `reason_text`
- `requested_by`
- `requested_by_name`
- `requested_at`

Quelle:

- aktive Zeile aus `quote_item_approval_requests`
- Join auf `users` fuer `requested_by_name`

Namensfallback:

```text
display_name -> email -> requested_by
```

### Anfrage-Snapshots

- `current_unit_price_snapshot`
- `cost_basis_unit_price_snapshot`
- `target_unit_price_snapshot`
- `target_margin_percent_snapshot`
- `target_difference_snapshot`
- `margin_percent_snapshot`
- `price_decision_id`

Quelle:

- dieselbe aktive `quote_item_approval_requests`-Zeile

Diese Werte beschreiben den Stand zum Zeitpunkt der Anforderung.

### aktueller Zielstatus

Optionale, aber empfohlene Felder:

- `current_target_status`
- `current_target_difference`
- optional spaeter `current_target_unit_price`
- optional spaeter `current_margin_percent`
- optional spaeter `current_price_decision_id`

Quelle:

- `quote_items.unit_price`
- letzte Preisentscheidung als aktuelle Kostenbasis
- aktuelle Zielmargen-Konfiguration

Die Berechnung soll dieselben Statuswerte verwenden wie die Nacharbeits-Queue:

- `below_cost`
- `below_target`
- `on_target`
- `above_target`

Falls der erste Backend-Leaf kleiner bleiben muss, duerfen aktuelle
Zielstatusfelder zunaechst entfallen. Die Queue bleibt als offene
Freigabeliste trotzdem nutzbar. Der technische Zuschnitt empfiehlt sie aber,
weil sie vorhandene Entscheidungen vor dem Oeffnen der Quote besser
priorisierbar macht.

## SQL-Ableitung

Kernidee:

```sql
SELECT ...
FROM quotes q
JOIN quote_items qi ON qi.quote_id = q.id
JOIN quote_item_approval_requests qar ON qar.quote_item_id = qi.id
LEFT JOIN users requested_user ON requested_user.id = qar.requested_by
LEFT JOIN projects p ON p.id = q.project_id
LEFT JOIN contacts c ON c.id = q.contact_id
LEFT JOIN LATERAL (
  SELECT qipd.id, qipd.source_unit_price
  FROM quote_item_price_decisions qipd
  WHERE qipd.quote_item_id = qi.id
  ORDER BY qipd.created_at DESC
  LIMIT 1
) current_price_decision ON true
WHERE q.superseded_by_quote_id IS NULL
  AND qar.status = 'requested'
```

Filter:

- `q.project_id = $projectID`
- `q.contact_id = $contactID`
- `q.id = $quoteID`

Sortierung:

```sql
ORDER BY qar.requested_at ASC, q.nummer ASC, qi.position ASC
```

## Service-Zuschnitt

Neue Typen in `server/internal/quotes/service.go`:

```text
QuoteApprovalRequestQueueFilter
QuoteApprovalRequestQueueItem
```

Neue Service-Methode:

```text
ListApprovalRequestQueue(ctx, filter) ([]QuoteApprovalRequestQueueItem, error)
```

Der Service bleibt im Quote-Service, weil er direkt Quote, Position,
Freigabeanforderung, User-Anzeige, Preisentscheidung und Zielmargenlogik
zusammenfuehrt.

Keine neue Migration.
Keine neue Tabelle.
Keine Materialisierung.

## HTTP-Zuschnitt

Route in `server/internal/http/v1.go` innerhalb der Quotes-Routen:

```text
r.With(requirePermission("quotes.read")).Get("/approval-requests", ...)
```

Routing-Regel:

- Die literale Route muss vor `/{id}` registriert werden.
- Sinnvolle Reihenfolge:
  1. `/approval-requests`
  2. `/approval-rework`
  3. `/{id}`

Response:

```json
{
  "items": [...]
}
```

## Teststrategie

Integrationstest in `server/internal/http/quotes_integration_test.go`:

1. Eine aktive `requested`-Anforderung erscheint in der Queue.
2. Genehmigte Anforderungen erscheinen nicht.
3. Abgelehnte Anforderungen erscheinen nicht.
4. Stornierte Anforderungen erscheinen nicht.
5. Historische Quote-Versionen erscheinen nicht.
6. Filter `project_id`, `contact_id`, `quote_id` grenzen korrekt ein.
7. Response enthaelt Quote-, Positions-, Anforderungs- und Snapshot-Kontext.
8. `requested_by_name` nutzt den Fallback `display_name -> email -> id`.
9. Route verlangt `quotes.read`.
10. Ungueltige `project_id` oder `quote_id` liefern `400`.

Fuer den ersten Backend-Leaf reicht ein fokussierter Integrationstest, der die
wichtigsten Statusfaelle in einem Setup kombiniert.

## Client-Folge

Die erste Client-Anbindung soll noch nicht in diesem Leaf entstehen.

Spaeterer kleinster Client-Einstieg:

- API-Methode `listQuoteApprovalRequests(...)`
- kleiner Queue-Block in `QuotesPage` oder eine spaetere Freigabeansicht
- maximal drei sichtbare Eintraege analog zur Nacharbeits-Queue
- Aktion `Anzeigen` bzw. `Bearbeiten` mit Navigation zur Quote-Position
- keine Inline-Genehmigung oder Inline-Ablehnung im ersten Client-Schritt

Begruendung:

- Entscheidungen bleiben im Quote-Editor, wo Preis-, Zielmargen- und
  Positionskontext vollstaendig sichtbar sind.
- Die Queue soll zuerst auffinden und navigieren.

## Nicht-Ziele

Nicht Teil des ersten Implementierungs-Leafs:

- neue Persistenz
- Genehmigen oder Ablehnen aus der Queue
- Zusammenlegung mit `approval-rework`
- Integration in `GET /api/v1/workflow/commercial`
- KPI, SLA, Priorisierung oder Eskalation
- Verantwortliche oder Faelligkeiten
- Pagination
- freie Suche
- neue Approval-Statuswerte
- KI-Auswertung von Freigabegruenden

## Umsetzungsvorschlag

Naechste Leaves:

1. Backend-Readmodel und Endpoint fuer
   `GET /api/v1/quotes/approval-requests` implementieren.
2. Integrationstest fuer aktive, genehmigte, abgelehnte, stornierte und
   historische Anforderungen ergaenzen.
3. Minimale Client-API und Queue-Sicht mit Navigation zur Quote-Position
   umsetzen.
4. Queue-Block auditieren und danach neu entscheiden, ob direkte
   Entscheidungen, Cockpit-Integration oder KPI/Priorisierung folgen.

## Verifikation

Keine Tests ausgefuehrt, da dieses Leaf nur das technische Zielmodell
dokumentiert.
