# GAEB: Minimalstrategie fuer den Importlauf-Freigabe-Widgettest

## Ziel

Subtask 3.1.61.2 definiert den kleinsten stabilen Widgettest fuer die
vorhandene Freigabe eines vollstaendig geprueften GAEB-Importlaufs. Dieser
Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

Im Importdetaildialog gilt:

```text
quotes.write + status parsed   -> Zur Übernahme freigeben
quotes.write + status reviewed -> Draft-Quote erzeugen
```

Die Freigabe ruft `markQuoteImportReviewed(importId)` auf. Danach setzt die
Runtime zunaechst die Mutationsantwort als lokales Detail, laedt anschliessend
Detail und Positionen erneut und aktualisiert zuletzt die Importvorschau. Die
Snackbar meldet `Importlauf wurde freigegeben`.

Runtime, `ApiClient` und Serververtrag sind dafuer bereits vollstaendig.

## Fake-Zustandsvertrag

Der `_FakeApiClient` erhaelt:

```text
reviewedQuoteImportIds = <String>[]
```

Der Override von `markQuoteImportReviewed(importId)`:

1. zeichnet die Import-ID genau einmal in `reviewedQuoteImportIds` auf
2. liest das konfigurierte Ausgangsdetail aus `quoteImportDetails`
3. gibt eine neue Map mit allen Ausgangsfeldern und `status: reviewed` zurueck

`getQuoteImport(id)` muss denselben Zustand respektieren: Ist die ID bereits
aufgezeichnet, liefert es ebenfalls eine neue Detailmap mit
`status: reviewed`. Dadurch ueberschreibt `refreshImportState()` die
erfolgreiche Mutationsantwort nicht wieder mit `parsed`.

Die konfigurierten Ausgangsmaps bleiben unveraendert. `listQuoteImportItems`
kann unveraendert die leere Liste liefern. `listQuoteImports` muss fuer diesen
Test nicht zustandsabhaengig erweitert werden, weil die Erfolgskriterien im
weiterhin offenen Detaildialog liegen und keine Vorschau-Statusanzeige
behauptet wird.

## Separater Test

Neuer Testname:

```text
QuotesPage GAEB import review enables draft quote action
```

Der Test verwendet:

- `_prepareLargeViewport`
- Berechtigungen `quotes.read` und `quotes.write`
- Projektfilter `project-1`
- genau einen Import `import-review-run-1`
- Ausgangsstatus `parsed`
- `item_count: 1`
- `accepted_count: 1`, `rejected_count: 0`, `pending_count: 0`
- eine leere Positionsliste, da Positionsdarstellung nicht Testziel ist
- keine erzeugte Quote und keine Apply-Antwort

Die Importvorschau verwendet denselben eindeutigen Dateinamen wie das Detail,
zum Beispiel `freigabe-ausschreibung.x83`.

## Testablauf

1. Seite mit dem Projektfilter laden.
2. Importdialog ueber den eindeutigen `Details`-Button oeffnen.
3. `Status: parsed` und `Zur Übernahme freigeben` pruefen.
4. Sicherstellen, dass `Draft-Quote erzeugen` noch nicht sichtbar ist.
5. `Zur Übernahme freigeben` antippen.
6. Mit `pumpAndSettle()` Mutation, Fortschrittsroute sowie Detail-, Positions-
   und Vorschau-Refresh abschliessen lassen.
7. Exakt `['import-review-run-1']` in `reviewedQuoteImportIds` pruefen.
8. Im weiterhin offenen Importdetaildialog pruefen:
   - `Status: reviewed`
   - `Zur Übernahme freigeben` ist nicht mehr sichtbar
   - `Draft-Quote erzeugen` ist sichtbar
   - `Importlauf wurde freigegeben` ist sichtbar
9. Importdialog schliessen und bestaetigen, dass kein `AlertDialog` verbleibt.

Der Test tippt `Draft-Quote erzeugen` nicht an.

## Fortschrittsdialog-Grenze

Der Fake antwortet synchron. Deshalb kann der nicht schliessbare
Fortschrittsdialog zwischen zwei Testframes vollstaendig erscheinen und wieder
verschwinden. Eine zeitkritische Assertion auf seine kurzlebige Textzeile waere
unnötig fragil.

`pumpAndSettle()` ist hier die stabile Interaktion: Es wartet, bis die
Fortschrittsroute geschlossen, alle drei Refreshschritte beendet und der
Importdetaildialog neu aufgebaut ist. Belegt wird das Ergebnis ueber den
Statuswechsel und die neue Aktion, nicht ueber eine starre Dialoganzahl.

## Finder-Grenzen

- `find.widgetWithText(TextButton, 'Details')` fuer den Importdialog
- `find.widgetWithText(FilledButton, 'Zur Übernahme freigeben')` fuer die
  Mutation
- `find.widgetWithText(FilledButton, 'Draft-Quote erzeugen')` fuer die
  nachfolgende, nur sichtbar zu pruefende Aktion
- `find.widgetWithText(TextButton, 'Schließen')` fuer den verbleibenden
  Importdialog

Vor und nach dem Statuswechsel werden keine Finder ueber Iconpositionen oder
eine starre Anzahl von `AlertDialog`-Widgets verwendet.

## Implementierungsgrenze

Subtask 3.1.61.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- eine Importfreigabe-Aufzeichnungsliste
- einen Fake-Override
- eine kleine zustandsabhaengige Erweiterung von `getQuoteImport`
- genau einen separaten Widgettest

`client/lib/pages/quotes_page.dart` bleibt unveraendert, sofern kein bislang
unsichtbarer Runtime-Defekt auftritt.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import review enables draft quote action"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- keine Runtime-Aenderung
- keine Apply-Mutation oder Draft-Quote-Erzeugung
- keine Quote-Navigation
- kein Fehler- oder Berechtigungsnegativtest
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine Sammelreview- oder KI-Automatisierung

## Ergebnis

3.1.61.2 ist abgeschlossen. Subtask 3.1.61.3 erweitert nur den bestehenden
Fake-Zustand und implementiert genau den definierten Importlauf-Freigabe-
Widgettest.
