# GAEB-Nacharbeits-Queue: Audit der kompakten Kontextdarstellung

## Scope

Subtask 3.1.49.4 prueft den in 3.1.49.2 zugeschnittenen und in 3.1.49.3
implementierten Client-Zuschnitt fuer den Entscheidungskontext in der
Nacharbeits-Queue.

Der Audit betrachtet nur die bestehende Queue-Sicht in `QuotesPage`. Backend,
API-Vertrag, Persistenz, Filter, Navigation und Workflow-Verhalten bleiben
ausserhalb dieses kleinen Abschluss-Checks.

## Gepruefter Vertrag

- Ein Queue-Eintrag behaelt den vorhandenen Titel aus Angebotsnummer und
  Position.
- Der Kontextblock besteht aus maximal vier einzeiligen Zeilen.
- Zeile 1 zeigt die Positionsbeschreibung mit Fallback `Position`.
- Zeile 2 zeigt `Grund:` mit Prioritaet `reason_text`, deutschem
  `reason_code`-Label und Fallback `Nacharbeit erforderlich`.
- Zeile 3 wird nur bei vorhandenem Kommentar, Entscheider oder parsebarem
  Entscheidungszeitpunkt angezeigt.
- Zeile 4 wird nur bei vorhandenem `current_target_status` angezeigt und
  beginnt mit `Aktuell:`.
- Historische Snapshotpreise, Zielpreis, aktueller Preis und Marge bleiben in
  dieser kompakten Arbeitsliste verborgen.

## Implementierungsbefund

- `_approvalReworkReasonLabel(...)` bildet `negative_margin` und
  `below_target_margin` deutsch ab und laesst unbekannte nicht-leere Codes
  unveraendert sichtbar.
- `_approvalReworkDecisionContext(...)` kombiniert optional
  `decision_comment`, `decided_by_name` bzw. `decided_by` und einen parsebaren
  `decided_at` ueber die bestehende lokale `_formatDateTime(...)`-Darstellung.
- `_approvalReworkTargetStatusLabel(...)` bildet die bekannten aktuellen
  Zielstatuswerte `below_cost`, `below_target`, `on_target` und `above_target`
  deutsch ab.
- `_approvalReworkCurrentTargetContext(...)` trennt den heutigen Stand mit
  `Aktuell:` klar von der historischen Ablehnung und formatiert eine vorhandene
  `current_target_difference` mit Waehrung; positive Werte erhalten ein
  explizites `+`.
- Der bisherige zweizeilige Subtitle wurde durch eine `Column` ersetzt. Jede
  Zeile ist auf `maxLines: 1` mit `TextOverflow.ellipsis` begrenzt; die Liste
  nimmt hoechstens vier Kontextzeilen.
- `isThreeLine` ist entfernt. Die Queue bleibt weiterhin auf drei direkt
  sichtbare Eintraege begrenzt und nutzt den bestehenden Aktionsbutton.

## Abgrenzung

Kein weiterer kleiner Haertungsschritt mit gutem Signal bleibt innerhalb dieses
Blocks offen:

- Ein Detaildialog fuer Snapshotwerte waere ein neuer UI-Scope.
- Filter, Sortierung oder Priorisierung waeren ein neuer Queue-Scope.
- Backend-seitige Vorformatierung wuerde den bewusst clientseitigen Zuschnitt
  vergroessern.
- Workflow-Cockpit, KPI oder Zuweisung waeren neue Ausbaustufen.

## Verifikation

Erfolgreich ausgefuehrt:

```text
cd client
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Aus dem vorigen Implementierungsleaf war bereits erfolgreich:

```text
cd client
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

## Ergebnis

Task 3.1.49 ist abgeschlossen. Die Nacharbeits-Queue zeigt den vorhandenen
Entscheidungskontext kompakt, trennt historische Entscheidung vom aktuellen
Zielmargenstand und erweitert weder Backend noch API noch Persistenz.
