# GAEB Approval Rework Jump Highlight Audit

## Scope

- Leaf: `3.1.44.1`
- Ziel: Nach dem Sprung aus der Nacharbeitswarnung soll die Zielposition im Angebotsdetail kurz sichtbar hervorgehoben werden.
- Umsetzung bleibt client-only in `client/lib/pages/quotes_page.dart`.

## Implementierung

- `_QuotesPageState` haelt die aktuell hervorgehobene Positionsnummer in `_highlightedRejectedApprovalPosition`.
- `_quoteItemHighlightToken` verhindert, dass ein alter Auto-Reset eine neuere Hervorhebung loescht.
- `_highlightRejectedApprovalPosition(int position)` setzt die Hervorhebung und entfernt sie nach drei Sekunden wieder.
- `_scrollToFirstRejectedApprovalDecisionPosition()` hebt die erste betroffene Position direkt vor `Scrollable.ensureVisible(...)` hervor.
- Beim Laden eines Angebotsdetails wird `_highlightedRejectedApprovalPosition` zurueckgesetzt.
- Die Positions-`ListTile` nutzt fuer die hervorgehobene Position `tileColor: Colors.red.shade50`.

## Abgrenzung

- Keine Backend- oder API-Aenderung.
- Keine persistente UI-Markierung.
- Kein Sprung zu weiteren Positionen.
- Kein automatischer Editor-Fokus.
- Keine Erweiterung der Prozesssperren.

## Verifikation

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen liefen erfolgreich ohne Analyzer-Issues.
