# GAEB-Approval-Hint: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit schliesst den engen Block des read-only Approval-Hints nach dem
nachgezogenen Negativmargen-Test ab.

Geprueft wird bewusst nur:

- ob der Hinweis technisch Ende-zu-Ende vorhanden ist
- ob alle fachlich relevanten read-only Statuspfade abgesichert sind
- ob innerhalb dieses engen Hinweisblocks noch ein kleiner Schritt mit gutem
  Signal uebrig ist

## 1. Ausgangspunkt

Das Zwischen-Audit hatte den Approval-Hint noch nicht abgeschlossen, weil der
zentrale Negativmargenpfad nicht explizit getestet war.

Offen war genau:

- negative Marge muss zu `approval_recommended` fuehren
- der Grundtext muss den spaeteren Freigabebedarf sichtbar machen
- die zugrunde liegenden Margenwerte muessen weiter nachvollziehbar bleiben

## 2. Umgesetzter Scope

Der umgesetzte Scope bleibt read-only:

- Backend-DTO `QuoteItemApprovalHint`
- Service `ApprovalHintForQuoteItem(...)`
- Route `GET /api/v1/quotes/{id}/items/{itemID}/approval-hint`
- Client-API `getQuoteItemApprovalHint(...)`
- Draft-Modell `_QuoteItemApprovalHintDraft`
- positionsnaher Ladehandler `_loadApprovalHint(...)`
- UI-Block `Freigabehinweis`

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

## 3. Backend-Ergebnis

`ApprovalHintForQuoteItem(...)` baut weiterhin auf
`MarginAnchorForQuoteItem(...)` auf. Dadurch bleiben Preisentscheidung,
Kostenbasis, aktueller Positionspreis und Margin-Status aus einer gemeinsamen
Quelle ableitbar.

Abgesicherte Statuspfade:

- `approval_not_required` bei nicht negativer Marge
- `approval_recommended` bei negativer Marge
- `approval_blocked_until_margin_available` ohne gespeicherte Preisentscheidung

Der nachgezogene Integrationstest setzt nach einer gespeicherten
Preisentscheidung den aktuellen Positionspreis unter die Kostenbasis und
erwartet:

- `approval_status = approval_recommended`
- `approval_reason = Negative Marge sichtbar; spaetere Freigabe empfohlen`
- `margin_status = negative_margin`
- negative absolute und prozentuale Marge

Damit ist der fachliche Kern des Approval-Hints im Backend abgesichert.

## 4. Client-Ergebnis

Der Client spiegelt den Hinweis positionsnah und ohne Schreibaktion:

- `getQuoteItemApprovalHint(...)` ruft den read-only Endpunkt ab
- `_QuoteItemApprovalHintDraft` modelliert Status, Grund und Margenwerte
- `approvalHint` und `approvalHintPerformed` halten den geladenen Zustand
- `_loadingApprovalHintItemId` begrenzt den Ladezustand auf die Position
- `_loadApprovalHint(...)` aktualisiert den Draft
- `Freigabehinweis` zeigt Status, Grund, Marge, aktuellen Preis, Kostenbasis,
  Entscheidungstyp, Quelle und Entscheidungszeit

Der Client fuehrt keine Freigabeaktion aus und veraendert keine Angebotsdaten.

## 5. Bewusst nicht umgesetzt

Die folgenden Themen sind nicht Teil dieses abgeschlossenen Blocks:

- persistenter Freigabestatus
- Freigabehistorie
- Kommentierung
- Rollen und Berechtigungen
- Zielmarge oder Zielaufschlag
- Rabatt- und Nachlassregeln
- Angebotsweite Freigabepruefung
- Bulk-Freigaben
- automatische Entscheidung

Diese Themen sind groesser als der aktuelle read-only Hinweis und gehoeren in
eigene Folgefeatures.

## 6. Audit-Entscheidung

Der read-only Approval-Hint ist abgeschlossen.

Begruendung:

- Backend, API und Client bilden dieselbe read-only Semantik ab
- alle drei Statuspfade sind fachlich nachvollziehbar
- der zuvor offene Negativmargenpfad ist per Integrationstest abgesichert
- der Block veraendert keine Angebotsdaten und fuehrt keinen Workflow ein
- weitere sinnvolle Schritte waeren neue Feature-Bloecke, keine Haertung dieses
  Hinweises

## 7. Verifikation

Fuer den Abschluss sind relevant:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

## 8. Naechster sinnvoller Schritt

Nach dem abgeschlossenen Approval-Hint sollte eine kurze fachliche Inventur
entscheiden, welcher Folgeblock den hoechsten Signalwert hat:

- Zielmarge- und Zuschlagslogik
- echter Freigabe-Workflow
- engere Positionskalkulation mit Material, Arbeit und Fremdleistung
