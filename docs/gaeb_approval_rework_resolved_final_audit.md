# GAEB Approval Rework Resolved Final Audit

## Scope

Dieser Audit schliesst den kleinen Durchstich fuer erledigte Freigabe-Nacharbeit ab:

- terminaler Status `rework_resolved`
- Backend-Abschluss einer zuvor abgelehnten Positionsfreigabe
- Readmodel-Anzeige als letzte Entscheidung
- Aufhebung der Prozesssperren nach erledigter Nacharbeit
- minimale Client-Aktion im bestehenden Quote-Editor

Nicht Teil dieses Blocks sind ein separater Workflow-Screen, automatische Rework-Erkennung ohne Nutzeraktion, Benachrichtigungen, Bulk-Aktionen oder weitergehende Approval-Regeln.

## Ergebnis

Der Block ist fachlich und technisch geschlossen. Es bleibt innerhalb dieses kleinen Status-Durchstichs kein weiterer kleiner Haertungsschritt mit gutem Signal uebrig.

## Gepruefte Kette

### Persistenz

`server/internal/migrate/migrations/053_quote_approval_rework_resolved.sql` erweitert den Statusraum von `quote_item_approval_requests` um `rework_resolved` und haertet die Lifecycle-Constraints so, dass erledigte Nacharbeit eine entschiedene, nicht stornierte, terminale Entscheidung mit genehmigten Snapshots ist.

### Backend-Service

`ResolveApprovalReworkForQuoteItem(...)` in `server/internal/quotes/service.go` erzwingt die relevanten Guard Rails:

- nur Draft-Angebote
- keine historischen Angebotsversionen
- Position gehoert zum Angebot
- keine aktive `requested`-Freigabe
- neueste terminale Entscheidung ist `rejected`
- Preisentscheidung existiert
- aktuelle Zielmarge ist serverseitig erreicht oder uebertroffen
- Kommentar bleibt begrenzt

Der Service schreibt keine Mutation an der abgelehnten Entscheidung, sondern eine neue auditierbare terminale Entscheidung `rework_resolved`.

### API

`server/internal/http/v1.go` stellt `POST /api/v1/quotes/{id}/items/{itemID}/approval-rework/resolve` unter `quotes.approve` bereit. Validierungsfehler fuer fehlende offene Nacharbeit oder noch nicht erreichte Zielmarge werden als fachliche `400`-Antworten klassifiziert.

### Readmodel und Prozesssperren

Die Quote-Readmodels beruecksichtigen `rework_resolved` als terminale letzte Entscheidung. Die Prozesssperren in Quote-Statuswechsel, Quote-zu-Rechnung und Quote-zu-Auftrag sperren nur weiter, wenn die neueste terminale Entscheidung `rejected` ist; `rework_resolved` hebt die Sperre bewusst auf.

### Integrationstests

`server/internal/http/quotes_integration_test.go` deckt die kritischen Wege ab:

- Permission-Guard fuer Resolve-Endpunkt
- Zielmargen-Guard vor Resolve
- erfolgreicher Resolve mit Snapshots und `latest_approval_decision.status = rework_resolved`
- Statuswechsel auf `sent` nach erledigter Nacharbeit
- Quote-zu-Ausgangsrechnung nach erledigter Nacharbeit
- Quote-zu-Auftrag nach erledigter Nacharbeit

### Client

`client/lib/api.dart` bietet `resolveQuoteItemApprovalRework(...)`. `client/lib/pages/quotes_page.dart` bindet die Aktion bewusst im bestehenden Quote-Editor an. Die Aktion ist nur sichtbar, wenn:

- die neueste Entscheidung Nacharbeit verlangt (`rejected`)
- keine aktive Freigabeanforderung besteht
- der Zielmargenanker aktuell `on_target` oder `above_target` meldet
- der Nutzer `quotes.approve` hat

Nach Erfolg laedt der Editor die Quote neu, damit Badge, Nacharbeitsmarker und Snapshots aus dem Server-Readmodel kommen.

## Verifikation

Ausgefuehrt:

```text
go test ./internal/migrate ./internal/quotes ./internal/http
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Beide Checks sind gruen.

## Entscheidung

Der `rework_resolved`-Durchstich ist abgeschlossen. Der naechste sinnvolle Schritt ist kein weiterer Feinschliff an diesem Status, sondern ein neuer Folgeabschnitt. Naheliegend ist die fachliche Inventur fuer den naechsten kommerziellen GAEB-/Approval-Ausbau nach erledigter Nacharbeit, zum Beispiel Auswertbarkeit, Workflow-Uebersicht oder spaetere Automatisierung.
