# GAEB-Freigabe: Client-Invalidierung nach Nacharbeit Audit

## Scope

Dieses Audit schliesst den Runtime-Leaf zur clientseitigen Invalidierung
veralteter Kalkulations- und Freigabeanker nach Positionsnacharbeit.

Geaendert wurde nur der Flutter-Client in `client/lib/pages/quotes_page.dart`.
Es gibt keine Backend-, API- oder Migrationsaenderung.

## Umsetzung

### 1. Zentrale Invalidierung am Draft

`_QuoteItemDraft.invalidateCommercialAnchors()` setzt lokale, snapshotbasierte
Bewertungsdaten zurueck:

- `targetMarginAnchor`
- `targetMarginAnchorPerformed`
- `approvalHint`
- `approvalHintPerformed`
- `marginAnchor`
- `marginAnchorPerformed`
- `priceEvaluation`
- `priceEvaluationPerformed`

Bewusst erhalten bleiben:

- `latestApprovalDecision`
- `approvalRequests`
- `approvalRequestsPerformed`
- `approvalRequest`

Damit bleibt die abgelehnte Entscheidung sichtbar, waehrend veraltete
Bewertungsanker nicht mehr als Grundlage fuer eine erneute Freigabeanforderung
angezeigt werden.

### 2. Row-Callback fuer Positionsaenderungen

`_QuoteItemRow` bekommt `onCommercialFieldsChanged`.

Ausgeloest wird der Callback bei:

- Beschreibung
- Menge
- Einheit
- Einzelpreis
- Steuercode
- Material-ID
- Preisstatus
- Entfernen einer Position

Der Parent invalidiert genau den betroffenen `_QuoteItemDraft`.

### 3. Bestehender Re-Request-Flow bleibt unveraendert

Der Button `Freigabe anfordern` bleibt weiterhin an die bestehende Regel
gebunden:

```text
targetMarginAnchor != null
AND targetMarginAnchor.canRequestApproval
AND approvalRequest == null
```

Nach einer Eingabeaenderung muss der Nutzer daher die Zielmarge neu laden.
Erst danach kann eine erneute Freigabeanforderung sichtbar werden.

## Ergebnis

Der Client zeigt nach Positionsnacharbeit keine alten Zielmargen-,
Margenanker-, Freigabehinweis- oder Preisbewertungsdaten mehr an.

Damit wird verhindert, dass Nutzer auf Basis eines alten Snapshots erneut
Freigabe anfordern oder alte Margeninformationen als aktuell interpretieren.

## Verifikation

Ausgefuehrt:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- `dart format`: erfolgreich, keine weiteren Format-Aenderungen
- `flutter analyze`: `No issues found`

## Nicht geloest

Weiterhin offen bleibt der fachliche Sonderfall:

```text
Ablehnung -> Nacharbeit erreicht Zielmarge -> keine neue Freigabe noetig,
aber latest_approval_decision bleibt rejected
```

Dieser Fall braucht einen eigenen Folge-Leaf, weil er eine fachliche
Statusentscheidung erfordert.

