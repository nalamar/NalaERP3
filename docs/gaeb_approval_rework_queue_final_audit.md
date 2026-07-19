# GAEB-Freigabe: Abschlussaudit der Nacharbeits-Queue

## Scope

Subtask 3.1.48.5 prueft den in 3.1.48.3 und 3.1.48.4 umgesetzten
Ende-zu-Ende-Pfad fuer offene Nacharbeit:

`GET /api/v1/quotes/approval-rework` -> Client-API -> Queue-Sicht ->
Angebotsdetail bzw. fokussierter Angebotseditor.

## Backend-Vertrag

- Die literale Route `/approval-rework` ist vor `/{id}` registriert und wird
  deshalb nicht als Angebots-ID interpretiert.
- Der Zugriff erfordert `quotes.read`.
- `project_id` und `quote_id` werden als UUID validiert; `contact_id` bleibt
  entsprechend dem bestehenden Kontaktvertrag ein String.
- Die Antwort verwendet den Wrapper `{ "items": [...] }`.
- Pro Angebotsposition ist nur die neueste terminale Entscheidung relevant.
  Nur `rejected` bleibt sichtbar; spaetere Entscheidungen `approved` oder
  `rework_resolved` entfernen die Position aus der Queue.
- Historische Angebotsversionen mit `superseded_by_quote_id` werden
  ausgeschlossen.

## Client-Vertrag und Filter

- `ApiClient.listQuoteApprovalRework(...)` bildet die drei Backend-Filter
  `project_id`, `contact_id` und `quote_id` ab und liest den `items`-Wrapper.
- `QuotesPage` verwendet fuer die sichtbare Queue bewusst nur den bereits
  vorhandenen Projektfilter. Damit stimmen Angebotsliste, GAEB-Importe und
  Nacharbeits-Queue im Projektkontext ueberein.
- Sowohl der globale Seiten-Refresh als auch der Filter-Button rufen `_load()`
  auf. `_load()` aktualisiert danach auch die Queue.
- Nach erfolgreichem Speichern im Angebotseditor ruft `_openEditDialog()`
  ebenfalls `_load()` auf. Eine erledigte oder neu entschiedene Nacharbeit
  verschwindet dadurch ohne separaten Queue-Workflow.

## Navigation

- Ein Queue-Eintrag laedt die Quote ueber die stabile `quote_id`.
- Bei `quotes.write` und Quote-Status `draft` oeffnet die Aktion `Bearbeiten`
  den vorhandenen Angebotseditor.
- Das Ziel wird zuerst ueber `quote_item_id` aufgeloest. Die 1-basierte
  `position` dient nur als Fallback.
- Ohne Schreibrecht oder bei nicht bearbeitbarer Quote bleibt die Aktion als
  `Anzeigen` verfuegbar. Sie oeffnet die Angebotsdetails, markiert die Position
  und scrollt zu ihr.
- Es entsteht kein zweiter Mutationspfad; die Queue bleibt eine Read-Sicht mit
  Navigation in den bestehenden Angebotsworkflow.

## UI-Grenzen

- Maximal drei Queue-Eintraege werden direkt gezeigt.
- Weitere Treffer werden als Anzahl ausgewiesen.
- Die Sicht ist absichtlich kein Workflow-Cockpit: keine Queue-Mutation, keine
  KPI-Aggregation, keine Zuweisung und keine Automation.

## Verifikation

Erfolgreich ausgefuehrt:

```text
cd server
go test ./internal/migrate ./internal/quotes ./internal/http

cd client
dart format --output=none --set-exit-if-changed lib/api.dart lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

`git diff --check` meldet keine Whitespace-Fehler. Die ausgegebenen
LF/CRLF-Hinweise betreffen die bestehende Windows-Arbeitskopie.

## Ergebnis

Task 3.1.48 ist abgeschlossen. Die Nacharbeits-Queue besitzt einen konsistenten
Backend-/Client-Vertrag, folgt dem vorhandenen Projektfilter und navigiert
berechtigungsabhaengig in Detail- oder Editoransicht zur betroffenen Position.
