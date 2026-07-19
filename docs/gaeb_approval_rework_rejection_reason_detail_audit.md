# GAEB Approval Rework Rejection Reason Detail Audit

## Scope

- Leaf: `3.1.45.1`
- Ziel: Abgelehnte Nacharbeitspositionen sollen im Angebotsdetail direkt den Ablehnungsgrund anzeigen.
- Umsetzung bleibt client-only in `client/lib/pages/quotes_page.dart`.

## Implementierung

- `_approvalDecisionReasonLabel(...)` formatiert `reason_text` bevorzugt und nutzt `reason_code` als Fallback.
- Bekannte Codes werden lesbar gemappt:
  - `negative_margin` -> `Negative Marge`
  - `below_target_margin` -> `Unter Zielmarge`
- `_quoteItemRejectedApprovalDecisionSummary(...)` wertet `latest_approval_decision` je Position aus.
- Nur `status == rejected` erzeugt eine Nacharbeitszeile.
- `decision_comment` wird optional als `Kommentar: ...` angehaengt.
- Die Positions-`ListTile` nutzt nun einen `Column`-Subtitle:
  - erste Zeile: bestehende Mengen-/Preis-/Steuerdaten
  - zweite Zeile nur bei Ablehnung: kompakter Nacharbeitsgrund in Rot

## Abgrenzung

- Keine Backend- oder API-Aenderung.
- Keine volle Freigabehistorie im Angebotsdetail.
- Keine Bearbeitungsaktion aus der Detailzeile.
- Keine neue Persistenz.
- Keine Erweiterung der Prozesssperren.

## Verifikation

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen liefen erfolgreich ohne Analyzer-Issues.
