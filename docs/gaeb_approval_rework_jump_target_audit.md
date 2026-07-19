# GAEB Approval Rework Jump Target Audit

## Scope

- Leaf: `3.1.43.3`
- Ziel: Die quote-weite Nacharbeitswarnung soll direkt zur ersten betroffenen Angebotsposition springen.
- Umsetzung bleibt client-only in `client/lib/pages/quotes_page.dart`.

## Implementierung

- `_QuotesPageState` haelt Positionssprungziele in `Map<int, GlobalKey> _quoteItemJumpKeys`.
- `_quoteItemJumpKey(int position)` erzeugt stabile Keys pro sichtbarer Positionsnummer.
- `_scrollToFirstRejectedApprovalDecisionPosition()` ermittelt die erste Position aus `_quoteRejectedApprovalDecisionPositions(_selected)` und ruft `Scrollable.ensureVisible(...)` auf.
- Die Nacharbeitswarnung zeigt einen Button `Zur ersten Position` mit `Icons.arrow_downward_rounded`.
- Jede Position in der Detailansicht wird ueber `KeyedSubtree` an ihre 1-basierte Positionsnummer gebunden.

## Abgrenzung

- Kein Backend-Feld fuer Zielpositionen.
- Keine Hervorhebung der Zielposition nach dem Scroll.
- Kein Sprungmenue fuer mehrere Positionen.
- Keine API-Fehlerdetail-Erweiterung.

## Verifikation

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen liefen erfolgreich ohne Analyzer-Issues.
