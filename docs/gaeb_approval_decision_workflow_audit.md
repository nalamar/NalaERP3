# GAEB-Freigabeanforderungen: Entscheidungsworkflow-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten Entscheidungsworkflow fuer aktive
positionsbezogene Freigabeanforderungen.

Geprueft werden:

- Persistenzmodell
- Backend-Service
- HTTP/API und Berechtigung
- Integrationstests
- Client-API
- Quote-Editor-UI
- offene Kanten vor Historie oder Freigabe-Queue

## 1. Ergebnis

Der zugeschnittene Entscheidungsworkflow ist fuer den aktuellen MVP-Scope
umgesetzt.

Erfuellt:

- `quote_item_approval_requests` kennt jetzt `requested`, `approved`,
  `rejected` und `cancelled`.
- Entscheidungen werden direkt an der Anforderung gespeichert.
- Genehmigen und Ablehnen sind terminale Statuswechsel.
- Nur aktive `requested`-Anforderungen koennen entschieden werden.
- Entscheidungen setzen `decided_by`, `decided_at`,
  `decision_comment` und `updated_at`.
- Genehmigen setzt zusaetzlich
  `approved_unit_price_snapshot` und
  `approved_target_margin_percent_snapshot`.
- Ablehnen setzt keine Approved-Snapshots.
- Quote, Quote-Item, Preise, Summen und Quote-Status werden durch
  Genehmigen/Ablehnen nicht mutiert.
- `quotes.approve` trennt Freigabeentscheidungen von normalem
  Angebotsschreiben.
- Client-UI zeigt Genehmigen/Ablehnen nur bei vorhandener Permission.

Nicht umgesetzt und weiterhin ausserhalb dieses Flows:

- Entscheidungs- oder Storno-Kommentar-Dialog im Client
- Historien-/Timeline-UI
- Freigabe-Queue
- mehrstufige Freigaben
- Delegation oder Wiedervorlage
- automatische Korrektur nach Ablehnung

## 2. Persistenz-Befund

Migration:

```text
server/internal/migrate/migrations/051_quote_item_approval_decisions.sql
```

Die Migration erweitert die bestehende Anforderungstabelle um:

- `decided_by`
- `decided_at`
- `decision_comment`
- `approved_unit_price_snapshot`
- `approved_target_margin_percent_snapshot`

Der Status-Check ist auf die vier erwarteten Zustaende erweitert:

```text
requested | approved | rejected | cancelled
```

Die neue Lifecycle-Constraint bildet die wichtigsten fachlichen Regeln ab:

- `requested`: keine Entscheidung, kein Storno
- `cancelled`: Storno-Zeitpunkt gesetzt, keine Entscheidung
- `approved`: Entscheidung gesetzt, Approved-Snapshots gesetzt
- `rejected`: Entscheidung gesetzt, keine Storno-Pflicht

Bewertung:

- Das Modell ist fuer einen einstufigen MVP tragfaehig.
- Die Entscheidung bleibt auf der urspruenglichen Snapshot-Basis
  nachvollziehbar.
- Der Partial Unique Index fuer aktive Anforderungen bleibt korrekt, weil
  er weiter nur `status = 'requested'` betrachtet.

Offene Persistenzkante:

- `decision_comment` existiert, wird in der UI aber noch nicht erfasst.
- Historische Anforderungen koennen noch nicht ueber ein Quote-Item-Readmodel
  gelesen werden.

## 3. Backend-Service-Befund

Service-Schnitt:

```text
ApproveApprovalRequestForQuoteItem(...)
RejectApprovalRequestForQuoteItem(...)
```

Beide Methoden nutzen einen gemeinsamen transaktionalen Helper.

Guards:

- Kommentar maximal 500 Zeichen
- Quote wird per `FOR UPDATE` gelockt
- historische Angebotsversionen bleiben schreibgeschuetzt
- nur Draft-Quotes duerfen entschieden werden
- Position muss zur Quote gehoeren
- Entscheidung nur bei `status = 'requested'`

Mutation:

- `approved` und `rejected` mutieren nur
  `quote_item_approval_requests`.
- Keine Preis-, Summen- oder Statusmutation am Angebot.
- Ein zweiter Entscheidungsversuch laeuft in `pgx.ErrNoRows` und wird als
  fachlicher Fehler behandelt.

Bewertung:

- Der Service folgt dem bereits etablierten Request-/Cancel-Muster.
- Die Trennung zwischen kaufmaennischer Entscheidung und Kalkulation bleibt
  sauber.
- Die bisherige Save-Blockade wird nach terminaler Entscheidung automatisch
  aufgehoben, weil nur `requested` blockiert.

## 4. HTTP/API-Befund

Routen:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject
```

Berechtigung:

```text
quotes.approve
```

Migration:

```text
server/internal/migrate/migrations/052_quote_approval_permissions.sql
```

Bewertung:

- Die Endpunkte sind explizit und vermeiden implizite Status-PATCHes.
- `quotes.approve` ist bewusst enger als `quotes.write`.
- Initiale Zuordnung nur an `role-admin` ist konservativ.
- Der HTTP-Layer validiert Kommentarlaenge wie der Service; die doppelte
  Validierung ist fuer API-Feedback und Service-Sicherheit akzeptabel.

Offene API-Kante:

- Es gibt noch keinen Lesepfad fuer entschiedene oder stornierte
  Anforderungen je Position.
- `active_approval_request` bleibt bewusst nur eine aktive Sicht.

## 5. Test-Befund

Backend-Service:

```text
server/internal/quotes/approval_decisions_test.go
```

Der Test prueft:

- Genehmigen einer aktiven Anforderung
- Ablehnen einer aktiven Anforderung
- Duplicate-Decision-Guard
- Kommentarlimit
- keine Mutation von Quote-Preis oder Quote-Summe

HTTP:

```text
TestQuoteApprovalDecisionEndpointsRequireApprovePermission
```

Der Test prueft:

- Nutzer ohne `quotes.approve` erhaelt `403`
- Admin kann genehmigen
- Admin kann ablehnen
- doppelte Entscheidung liefert `400`
- Approved-Snapshots erscheinen nur bei Genehmigung

Verifikation:

```text
go test ./internal/quotes ./internal/http
flutter analyze lib/api.dart lib/pages/quotes_page.dart
```

Beide Pruefungen waren im Implementierungsverlauf ohne Befund.

## 6. Client-Befund

### 6.1 API

`client/lib/api.dart` enthaelt:

- `approveQuoteItemApprovalRequest(...)`
- `rejectQuoteItemApprovalRequest(...)`

Beide Methoden:

- senden optional `comment`
- erwarten `200 OK`
- geben die API-Response als `Map<String, dynamic>` zurueck

### 6.2 UI

`client/lib/pages/quotes_page.dart` zeigt im bestehenden Zielmargenblock bei
aktiver Anforderung:

- `Genehmigen`
- `Ablehnen`
- `Freigabe stornieren`

Genehmigen/Ablehnen erscheinen nur, wenn:

```text
ApiClient.hasPermission('quotes.approve')
```

Nach erfolgreicher Entscheidung:

- `item.approvalRequest = null`
- aktive Anforderung verschwindet lokal
- SnackBar zeigt Erfolg

Bewertung:

- Der UI-Schnitt ist klein und passt zum MVP.
- Keine neue Seite und keine Queue wurden eingefuehrt.
- Die aktive Blockade ist unmittelbar aufloesbar.

Offene UI-Kanten:

- Es gibt keinen Kommentardialog, obwohl Backend und API Kommentare
  unterstuetzen.
- Nach Reload sind genehmigte oder abgelehnte Anforderungen nicht sichtbar,
  weil der Quote-Reload nur `active_approval_request` liefert.
- Ablehnung ist ohne Historie fachlich zu leicht zu uebersehen.

## 7. Offene Kanten

### 7.1 Fehlende Historie

Terminale Entscheidungen verschwinden aus der aktiven Sicht. Das ist
technisch korrekt, aber fachlich unvollstaendig.

Kleinster sinnvoller Folgeausbau:

- letzte Freigabeanforderung je Quote-Position als read-only Lesemodell
- Status `approved`, `rejected`, `cancelled`
- Entscheider/Stornierer
- Zeitpunkte
- Kommentar
- Snapshots

### 7.2 Kein Kommentar im UI

Die API nimmt Kommentare an, aber die UI sendet aktuell keinen Kommentar.
Das ist fuer einen schnellen Entscheidungs-MVP akzeptabel, sollte aber vor
einer produktiven Freigabe sauber nachgezogen werden.

### 7.3 Ablehnung ohne Korrekturführung

Ablehnen mutiert bewusst keine Preise. Danach muss der Nutzer den Preis
korrigieren, Zielpreis anwenden oder die Position bearbeiten. Ohne Historie
oder Hinweis ist diese Folgeaktion noch nicht ausreichend sichtbar.

### 7.4 Keine Queue

Offene Freigaben sind nur in der Quote selbst sichtbar. Eine zentrale Queue
ist fachlich sinnvoll, aber groesser als der naechste kleinste
Haertungsschritt.

## 8. Entscheidung fuer den Folgepfad

Vor Freigabe-Queue, Kommentardialog oder mehrstufigem Workflow sollte zuerst
die Leseseite stabilisiert werden.

Naechster Leaf:

```text
Subtask 3.1.35.1: Read-only Historienmodell fuer Freigabeanforderungen je Quote-Position zuschneiden
```

Begruendung:

- Die Daten sind bereits persistiert.
- Der groesste fachliche Blindspot ist Nachvollziehbarkeit nach terminaler
  Entscheidung.
- Ein read-only Modell ist kleiner und risikoaermer als neue Mutation oder
  Queue.
- Darauf koennen spaeter Kommentar-UI, Entscheidungsbadges und Freigabe-Queue
  aufbauen.

