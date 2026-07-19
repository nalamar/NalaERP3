# GAEB-Freigabe: Abschluss-Audit der Freigabeanforderungs-Queue

## Ziel

Subtask 3.1.50.5 auditiert die abgeschlossene read-only Queue fuer offene
Freigabeanforderungen gegen den dokumentierten Backend-Vertrag, die
Client-Navigation und die bewusst gesetzten UI-Grenzen.

In diesem Leaf wird keine Laufzeitlogik geaendert.

## Ergebnis

Die Freigabeanforderungs-Queue ist fuer den aktuellen MVP-Schnitt konsistent
umgesetzt.

- Der Backend-Endpunkt listet nur aktive Anforderungen mit
  `quote_item_approval_requests.status = 'requested'`.
- Historische Quote-Versionen werden ueber `q.superseded_by_quote_id IS NULL`
  ausgeschlossen.
- Die Route ist unter `quotes.read` registriert und steht vor `/{id}`.
- Die Antwort liefert Quote-, Projekt-, Kontakt-, Positions-,
  Anforderungs-, Snapshot- und aktuellen Zielstatuskontext.
- Der Client ruft den Queue-Endpunkt read-only ab, zeigt eine kompakte Liste
  und navigiert zur betroffenen Quote-Position.
- Die Queue fuehrt keine Inline-Genehmigung, Inline-Ablehnung oder Storno aus.

Damit ist die fachliche Luecke zwischen positionsnaher Freigabehistorie und
systemweit auffindbaren offenen Freigabeentscheidungen geschlossen.

## Backend-Vertrag

### Endpoint

Umgesetzt:

```text
GET /api/v1/quotes/approval-requests
```

Berechtigung:

```text
quotes.read
```

Der Zuschnitt entspricht dem Zielmodell: Die Queue ist eine reine Leseliste.
Die entscheidenden Mutationen bleiben weiterhin positionsnah und gesondert
berechtigt:

- `POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve`
- `POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject`

### Routing

Die Route ist vor den variablen Quote-Routen registriert:

```text
/approval-requests
/approval-rework
/{id}
```

Damit wird `approval-requests` nicht faelschlich als Quote-ID interpretiert.

### Filter

Umgesetzt:

- `project_id`
- `contact_id`
- `quote_id`

Bewertung:

- `project_id` und `quote_id` werden als UUID validiert.
- `contact_id` bleibt String, passend zum bestehenden Kontaktvertrag.
- Ungueltige UUIDs liefern `400 validation_error`.
- Leere Werte werden nicht als harte Filter behandelt.

Nicht umgesetzt und weiterhin bewusst ausserhalb des MVP:

- freie Suche
- Statusfilter
- Sortierung aus dem Client
- Pagination
- Filter nach Anforderer
- Filter nach Zielstatus

### Readmodel

`QuoteApprovalRequestQueueItem` enthaelt die fuer den MVP noetigen
Feldgruppen:

- Quote-Kontext: ID, Nummer, Status, Datum
- Projekt- und Kontaktkontext
- Positionskontext: ID, Position, Beschreibung, aktueller Einzelpreis,
  Waehrung
- offene Anforderung: ID, Grund, Antragsteller, Zeitpunkt
- Anfrage-Snapshots: Preis, Kostenbasis, Zielpreis, Zielmarge,
  Zielabweichung, Marge, Preisentscheidungs-ID
- aktueller Zielstatus: Status, Zielpreis, Zielabweichung, Marge,
  Preisentscheidungs-ID

Die Namensauflösung fuer Antragsteller nutzt den erwarteten Fallback:

```text
display_name -> email -> requested_by
```

### Statusabgrenzung

Die Queue liest ausschliesslich:

```text
qar.status = 'requested'
```

Damit bleiben Statusphasen sauber getrennt:

- `requested`: offene Freigabeentscheidung
- `approved`: erledigt, nicht mehr in der Queue
- `rejected`: fuehrt in Nacharbeit, nicht mehr in dieser Queue
- `cancelled`: nicht mehr entscheidungspflichtig
- `rework_resolved`: Nacharbeit abgeschlossen

Die Trennung zu `GET /api/v1/quotes/approval-rework` bleibt fachlich sauber.

## Testabdeckung

Der Integrationstest
`TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests` prueft:

- aktive `requested`-Anforderung erscheint
- genehmigte Anforderung erscheint nicht
- abgelehnte Anforderung erscheint nicht
- stornierte Anforderung erscheint nicht
- historische Quote-Version erscheint nicht
- `project_id` filtert korrekt
- `contact_id` filtert korrekt
- `quote_id` auf erledigter Anforderung liefert keine Queue-Zeile
- ungueltige `project_id` liefert `400`
- ungueltige `quote_id` liefert `400`
- Response enthaelt Kontext-, Snapshot-, Nutzer- und Zielstatusfelder

Nicht explizit isoliert getestet:

- reine Berechtigungsverweigerung ohne `quotes.read`
- Sortierreihenfolge bei mehreren aktiven Anforderungen
- Namensfallback-Stufen `email` und `requested_by`, wenn `display_name` fehlt

Diese Luecken sind fuer den aktuellen MVP nicht blockierend, aber gute
Kandidaten fuer eine spaetere Testhaertung.

## Client-Vertrag

### API

Umgesetzt:

```dart
ApiClient.listQuoteApprovalRequests({
  String? projectId,
  String? contactId,
  String? quoteId,
})
```

Die Methode ruft `/api/v1/quotes/approval-requests` ab und liest die
`items`-Liste aus der Response.

### Queue-Sicht

`QuotesPage` laedt die Freigabeanforderungs-Queue beim normalen Listenladen
und bietet einen eigenen Refresh.

Die Karte `Freigabeanforderungen` zeigt maximal drei sichtbare Eintraege mit:

- Angebotsnummer
- Position
- Beschreibung
- Grund
- Antragsteller und Zeitpunkt
- aktuellem Zielstatuskontext

Die Darstellung ist bewusst kompakt und analog zur Nacharbeits-Queue
geschnitten.

### Navigation

`_openApprovalRequestItem` laedt die Quote, setzt den Detailkontext und laedt
Folgebelege. Danach gilt:

- Wenn `quotes.write` vorhanden ist und die Quote im Status `draft` ist, wird
  der Editor mit `initialFocusItemId` und Positionsfallback geoeffnet.
- Sonst wird in der Detailansicht zur betroffenen Position gescrollt.

Damit bleibt die Queue ein Auffindungs- und Navigationswerkzeug.

## UI-Grenzen

Bewusst nicht umgesetzt:

- Genehmigen direkt aus der Queue
- Ablehnen direkt aus der Queue
- Storno direkt aus der Queue
- Kommentar-Dialog in der Queue
- neue Filter-Controls fuer Kontakt oder Quote
- Pagination oder Vollbild-Arbeitsliste
- Zusammenlegung mit der Nacharbeits-Queue
- Einbindung in das kommerzielle Workflow-Cockpit
- KPI, SLA, Priorisierung oder Eskalation

Diese Grenzen sind wichtig, weil Entscheidungen weiterhin den vollstaendigen
Positions-, Preis-, Zielmargen- und Historienkontext im Quote-Editor nutzen.

## Restrisiken

- Die Client-Karte zeigt nur drei Eintraege; bei groesseren Mengen braucht es
  spaeter eine dedizierte Arbeitsliste oder Pagination.
- Es gibt noch keine Sortier- oder SLA-Visualisierung im Client; die
  serverseitige Reihenfolge ist aber stabil und fachlich nachvollziehbar.
- Die Wiederverwendung des Zielstatuskontexts aus der Nacharbeits-Queue ist
  sinnvoll, der Helper-Name ist im Client aber historisch auf Rework bezogen.
- Die Navigation in die Detailansicht nutzt denselben Positionshighlight-Pfad
  wie Nacharbeit; fachlich ist das akzeptabel, spaeter koennte ein neutralerer
  Highlight-Name folgen.

## Verifikation

Ausgefuehrt:

```text
go test ./internal/migrate ./internal/quotes ./internal/http
flutter analyze lib/api.dart lib/pages/quotes_page.dart
```

Ergebnis:

- Go-Tests gruen
- Flutter-Analyse fuer die betroffenen Client-Dateien gruen

## Empfehlung

Task 3.1.50 ist fachlich abgeschlossen.

Der naechste kleinste Schritt sollte wieder eine Inventur sein, bevor neue
Runtime-Logik entsteht. Zu entscheiden ist, ob als naechster Ausbau:

- direkte Genehmigung/Ablehnung aus einer dedizierten Queue,
- Integration in das kommerzielle Workflow-Cockpit,
- eine gemeinsame Approval-Arbeitsliste,
- oder KPI/Priorisierung fuer Freigabe- und Nacharbeitsarbeit

den groessten operativen Nutzen bringt.
