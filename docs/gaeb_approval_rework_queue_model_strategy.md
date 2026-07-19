# GAEB-Freigabe: Zielmodell fuer read-only Nacharbeits-Queue

## Ziel

Dieses Dokument schneidet das technische Minimalmodell fuer eine read-only
Nacharbeits-Queue zu.

Die Queue soll offene Freigabe-Nacharbeit ueber alle Angebote sichtbar machen.
Sie ist keine neue Mutation, keine neue Persistenz und keine KPI-Auswertung.

## Fachliche Definition

Ein Queue-Eintrag ist genau dann sichtbar, wenn fuer eine Quote-Position die
neueste terminale Freigabe-/Nacharbeitsentscheidung den Status `rejected` hat.

Damit gilt:

- `approved` ist nicht offen
- `rework_resolved` ist nicht offen
- `cancelled` ist nicht terminale Nacharbeit
- aktive `requested`-Anforderungen sind Freigabearbeit, aber keine erledigte
  oder offene Nacharbeit nach Ablehnung

Diese Definition entspricht den bestehenden Prozesssperren und der
quote-weiten Nacharbeitswarnung.

## Endpoint

Empfohlener erster Endpoint:

```text
GET /api/v1/quotes/approval-rework
```

Begruendung:

- Die Queue ist positionsbezogene Quote-Freigabe-Nacharbeit.
- Sie gehoert fachlich naeher zu `quotes` als zum allgemeinen
  `workflow/commercial`.
- Das bestehende kommerzielle Workflow-Cockpit bleibt beleg- und
  Folgebeleg-orientiert.
- Eine spaetere Cockpit-Integration kann auf demselben Readmodel aufbauen.

## Berechtigung

Minimal:

```text
quotes.read
```

Begruendung:

- Die Route ist read-only.
- Sie zeigt nur Informationen, die ein Nutzer mit Quote-Leserecht ueber
  Quote-Detail und Positionshistorie bereits sehen kann.
- Die spaetere Aktion zum Abschliessen bleibt separat unter `quotes.approve`.

Optional fuer spaetere Rollenverfeinerung:

```text
quotes.approve
```

Das sollte aber nicht im ersten Queue-MVP erzwungen werden, damit Vertrieb und
Kalkulation offene Blocker ebenfalls sehen koennen.

## Query-Parameter

Zulaessige Filter im MVP:

- `project_id`
- `contact_id`
- `quote_id`

Bewusst noch nicht vorgesehen:

- freie Suche
- Statusfilter
- Sortierparameter
- Pagination
- KPI-Zeitraeume
- Filter nach Entscheider

Die erste Server-Sortierung ist fest:

```text
decided_at ASC, quote_number ASC, position ASC
```

Begruendung:

- aelteste offene Nacharbeit zuerst
- stabile und einfache Reihenfolge
- keine Diskussion ueber Priorisierungslogik im ersten Schritt

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
      "reason_code": "below_target_margin",
      "reason_text": "Zielmarge pruefen",
      "decision_comment": "Preis nacharbeiten",
      "decided_by": "...",
      "decided_by_name": "Integration Test",
      "decided_at": "2026-04-04T09:12:00Z",
      "current_unit_price_snapshot": 50,
      "cost_basis_unit_price_snapshot": 60,
      "target_unit_price_snapshot": 75,
      "target_margin_percent_snapshot": 20,
      "target_difference_snapshot": -25,
      "margin_percent_snapshot": -20,
      "price_decision_id": "...",
      "current_unit_price": 72,
      "current_target_status": "on_target",
      "current_target_difference": 0,
      "currency": "EUR"
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

### Positions-Kontext

- `quote_item_id`
- `position`
- `description`

Quelle:

- `quote_items`

### letzte Ablehnung

- `approval_request_id`
- `reason_code`
- `reason_text`
- `decision_comment`
- `decided_by`
- `decided_by_name`
- `decided_at`

Quelle:

- neueste terminale Zeile aus `quote_item_approval_requests`
- Join auf `users` fuer `decided_by_name` mit Fallback:
  `display_name`, dann `email`, dann technische ID

### Ablehnungs-Snapshots

- `current_unit_price_snapshot`
- `cost_basis_unit_price_snapshot`
- `target_unit_price_snapshot`
- `target_margin_percent_snapshot`
- `target_difference_snapshot`
- `margin_percent_snapshot`
- `price_decision_id`

Quelle:

- dieselbe `quote_item_approval_requests`-Zeile

### aktueller Rework-Stand

- `current_unit_price`
- `current_target_status`
- `current_target_difference`
- `currency`

Quelle:

- `quote_items.unit_price`
- letzte Preisentscheidung als Kostenbasis
- aktuelle Zielmargen-Konfiguration

Falls die aktuelle Zielmargenberechnung im ersten Implementierungs-Leaf zu
gross wird, darf `current_target_status` zunaechst entfallen. Dann bleibt die
Queue trotzdem als offene Nacharbeitsliste nutzbar. Der Abschluss-Button im
Editor prueft den Zielstatus weiterhin serverseitig.

## SQL-Ableitung

Kernidee:

```sql
SELECT ...
FROM quotes q
JOIN quote_items qi ON qi.quote_id = q.id
JOIN LATERAL (
  SELECT qar.*
  FROM quote_item_approval_requests qar
  WHERE qar.quote_item_id = qi.id
    AND qar.status IN ('approved', 'rejected', 'rework_resolved')
  ORDER BY qar.decided_at DESC, qar.created_at DESC
  LIMIT 1
) latest_approval_decision ON true
LEFT JOIN users decided_user ON decided_user.id = latest_approval_decision.decided_by
LEFT JOIN projects p ON p.id = q.project_id
LEFT JOIN contacts c ON c.id = q.contact_id
WHERE latest_approval_decision.status = 'rejected'
  AND q.superseded_by_quote_id IS NULL
```

Filter:

- `q.project_id = $projectID`
- `q.contact_id = $contactID`
- `q.id = $quoteID`

## Service-Zuschnitt

Neuer Typ im Quote-Service:

```text
QuoteApprovalReworkQueueItem
```

Neuer Filter:

```text
QuoteApprovalReworkQueueFilter
```

Neuer Service:

```text
ListApprovalReworkQueue(ctx, filter) ([]QuoteApprovalReworkQueueItem, error)
```

Der Service sollte in `server/internal/quotes/service.go` bleiben, weil er
direkt auf Quote, Quote-Items und Approval-Readmodels zugreift.

Keine neue Migration.
Keine neue Tabelle.
Keine Materialisierung.

## HTTP-Zuschnitt

Route in `server/internal/http/v1.go` innerhalb der Quotes-Routen:

```text
r.With(requirePermission("quotes.read")).Get("/approval-rework", ...)
```

Wichtig fuer Routing:

- Die Route muss vor `/{id}` registriert werden, damit `approval-rework` nicht
  als Quote-ID interpretiert wird.

Query-Parsing:

- optionale UUID-Parameter validieren
- ungueltige UUIDs als `400`
- keine leeren Strings an den Service weitergeben

Response:

```json
{
  "items": [...]
}
```

## Teststrategie

Integrationstest in `server/internal/http/quotes_integration_test.go`:

1. Quote mit abgelehnter Position erscheint in der Queue.
2. Quote mit spaeterem `rework_resolved` erscheint nicht.
3. Quote mit spaeterer `approved`-Entscheidung erscheint nicht.
4. Historische Quote-Version erscheint nicht.
5. Filter `project_id`, `contact_id`, `quote_id` grenzen korrekt ein.
6. Response enthaelt Quote-, Positions-, Entscheidungs- und Snapshot-Kontext.
7. Route verlangt `quotes.read`.

Fuer den ersten Backend-Leaf reicht ein fokussierter Integrationstest, der zwei
bis drei Queueszenarien in einem Testfall kombiniert.

## Client-Folge

Die erste Client-Anbindung soll noch nicht in diesem Leaf entstehen.

Spaeterer kleinster Client-Einstieg:

- API-Methode `listQuoteApprovalReworkQueue(...)`
- kleiner Dashboard- oder Workflow-Block
- Aktion `Quote oeffnen` mit `initialFocusItemId`
- keine Inline-Erledigung aus der Queue im ersten Client-Schritt

Begruendung:

- Der bestehende sichere Abschluss lebt im Quote-Editor.
- Die Queue soll zuerst sichtbar machen und navigieren, nicht parallel
  Prozessentscheidungen ausloesen.

## Nicht-Ziele

Nicht Teil des ersten Implementierungs-Leafs:

- neue Persistenz
- Schreibaktionen aus der Queue
- KPI-Auswertungen
- freie Suche
- Pagination
- Prioritaetsberechnung nach Geldwert oder Zielabweichung
- Integration in `GET /api/v1/workflow/commercial`
- automatische Nacharbeitsabschluesse
- KI-Auswertung von Ablehnungsgruenden

## Umsetzungsvorschlag

Naechste Leaves:

1. Backend-Readmodel und Endpoint fuer `GET /api/v1/quotes/approval-rework`
   implementieren.
2. Integrationstest fuer offene, erledigte und genehmigte Nacharbeit ergaenzen.
3. Minimale Client-API und UI-Sicht mit Navigation zur Quote-Position
   umsetzen.
4. Queue-Block auditieren und danach neu entscheiden, ob Cockpit-Integration,
   Auswertung oder Priorisierung folgt.

## Verifikation

Keine Tests ausgefuehrt, da dieses Leaf nur das technische Zielmodell
dokumentiert.
