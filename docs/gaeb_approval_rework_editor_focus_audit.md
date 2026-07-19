# GAEB Approval Rework Editor Focus Audit

## Scope

- Leaf: `3.1.46.3`
- Ziel: Abgelehnte Nacharbeitsposition aus dem Angebotsdetail direkt im Editor-Kontext sichtbar machen.
- Umsetzung bleibt client-only in `client/lib/pages/quotes_page.dart`.

## Implementierung

- `_openEditDialog(...)` akzeptiert nun optionale Fokusparameter:
  - `initialFocusItemId`
  - `initialFocusPosition`
- Die Angebotsdetail-Positionszeile zeigt bei offener Nacharbeit im Entwurf einen Button `Bearbeiten`.
- Der Detail-Button uebergibt `item['id']` und die 1-basierte Position an den Editor-Dialog.
- `_QuoteEditorDialog` akzeptiert die Fokusparameter.
- `_QuoteEditorDialogState` verwaltet Fokus-Keys fuer Item-ID und Positions-Fallback.
- Nach dem ersten Frame sucht `_focusInitialItem()` die Zielposition:
  - bevorzugt per Item-ID
  - fallback per 1-basierter Position
- Bei gefundenem Ziel wird `Scrollable.ensureVisible(...)` ausgefuehrt.
- Die Zielposition bleibt im Dialog dezent hervorgehoben.
- `_QuoteItemRow` erhaelt `highlighted` und nutzt fuer die Card `Colors.red.shade50`.

## Abgrenzung

- Keine Backend- oder API-Aenderung.
- Keine automatische Preis-, Material- oder Zielpreisaenderung.
- Keine automatische erneute Freigabeanforderung.
- Keine Nacharbeitsqueue.
- Keine Mehrpositionsnavigation im Editor.

## Verifikation

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen liefen erfolgreich ohne Analyzer-Issues.
