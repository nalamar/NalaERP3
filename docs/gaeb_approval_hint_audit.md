# GAEB-Approval-Hint: Zwischen-Audit und Haertungsentscheidung

## Ziel dieses Audits

Dieses Audit prueft den engen Block des read-only Approval-Hints nach der
ersten Backend- und Client-Umsetzung.

Geprueft wird bewusst nur:

- ob der Hinweis technisch Ende-zu-Ende vorhanden ist
- ob Backend, API und Client dieselbe read-only Semantik abbilden
- ob vor dem Abschluss dieses Hinweisblocks noch ein kleiner Haertungsschritt
  mit gutem Signal uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- read-only Margenanker
- aktuelle Position gegen gespeicherte Kostenbasis
- Status `negative_margin`, `zero_margin` und `positive_margin`
- Entscheidung, daraus zuerst nur einen Freigabehinweis zu bauen

Die danach identifizierte Luecke war:

- negative Marge war sichtbar
- ein Hinweis auf spaetere Freigaberelevanz fehlte aber noch

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt bewusst klein:

- read-only Backend-DTO `QuoteItemApprovalHint`
- Service `ApprovalHintForQuoteItem(...)`
- Route `GET /api/v1/quotes/{id}/items/{itemID}/approval-hint`
- Client-API `getQuoteItemApprovalHint(...)`
- Draft `_QuoteItemApprovalHintDraft`
- positionsnaher Ladehandler `_loadApprovalHint(...)`
- read-only UI-Block `Freigabehinweis`

Weiterhin nicht enthalten:

- Freigabe speichern
- Freigabe anfordern
- Freigabe erteilen oder ablehnen
- Rollenmodell
- Zielmarge
- Zuschlagsregel
- Rabattlogik
- Bulk-Aktion
- Automatik

## 3. Ergebnis im Backend

Das Backend bildet den Hinweis eng ab:

- `ApprovalHintForQuoteItem(...)` baut auf `MarginAnchorForQuoteItem(...)` auf
- vorhandene Marge wird in einen Hinweisstatus uebersetzt
- fehlende Preisentscheidung wird als
  `approval_blocked_until_margin_available` mit HTTP 200 dargestellt
- echte Positions- oder Statusfehler bleiben Domain-Fehler

Aktuell getestete Backend-Pfade:

- `approval_not_required` nach Primaerpreis-Uebernahme und nicht negativer
  Marge
- `approval_blocked_until_margin_available` ohne gespeicherte Preisentscheidung

## 4. Ergebnis im Client

Der Client spiegelt den Hinweis positionsnah:

- `getQuoteItemApprovalHint(...)`
- `_QuoteItemApprovalHintDraft`
- Felder `approvalHint` und `approvalHintPerformed`
- Ladezustand `_loadingApprovalHintItemId`
- Handler `_loadApprovalHint(...)`
- read-only Block `Freigabehinweis`

Der Block zeigt:

- Hinweisstatus
- Grundtext
- vorhandene Marge
- aktuellen Positionspreis
- Kostenbasis
- Entscheidungstyp
- Quelle
- Entscheidungszeit

## 5. Bewusst nicht umgesetzt

Weiterhin ausserhalb dieses Blocks bleiben:

- echte Freigabeaktion
- Persistenz eines Freigabestatus
- Freigabehistorie
- Kommentierung
- Schwellwerte
- Zielmarge
- Rollen oder Berechtigungsregeln
- Angebotsweite Freigabepruefung

Diese Themen waeren jeweils eigene Folgeausbauten.

## 6. Offener kleiner Haertungsschritt

Innerhalb dieses Hinweisblocks bleibt genau ein kleiner Haertungsschritt mit
gutem Signal offen:

- der negative Margenpfad muss explizit gegen
  `approval_recommended` abgesichert werden

Warum dieser Schritt noch zum aktuellen Block gehoert:

- `approval_recommended` ist der fachliche Kern des Approval-Hints
- die erste Backend-Implementierung enthaelt diesen Branch bereits
- der Branch ist noch nicht durch einen fokussierten Integrationstest
  abgesichert
- der Test braucht keinen neuen Scope, keine UI und keine Persistenz

Warum jetzt noch keine weiteren Funktionen folgen:

- echte Freigabe waere Workflow
- Zielmarge waere Kalkulationsregel
- Rollen waeren Berechtigungsmodell
- Bulk waere Angebotsaggregation

## 7. Audit-Entscheidung

Der Approval-Hint-Block ist noch nicht vollstaendig abgeschlossen.

Vor dem Abschluss soll genau ein weiterer enger Haertungsschritt erfolgen:

- Integrationstest fuer negative Marge und Status `approval_recommended`

Danach kann der Block erneut auditiert und voraussichtlich abgeschlossen
werden.

## 8. Verifikation

Vor diesem Audit waren gruen:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Fuer den naechsten Haertungsschritt reicht voraussichtlich ein fokussierter
Backend-Test plus `go test ./internal/quotes ./internal/http`.

## 9. Naechster sinnvoller Schritt

Der naechste kleine Schritt ist:

- den bestehenden Approval-Hint-Integrationstest um den negativen Margenpfad
  erweitern

Der Test soll nach einer gespeicherten Preisentscheidung den aktuellen
Positionspreis unter die Kostenbasis setzen und anschliessend erwarten:

- `approval_status = approval_recommended`
- `approval_reason = Negative Marge sichtbar; spaetere Freigabe empfohlen`
- `margin_status = negative_margin`

Weiterhin nicht enthalten:

- Client-UI-Aenderung
- Freigabe-Schreibpfad
- Rollen
- Zielmarge
- Rabatt
- Bulk
- Automatik
