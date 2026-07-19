# GAEB: Minimalstrategie fuer den read-only Importdetail-Widgettest

## Ziel

Subtask 3.1.57.2 definiert den kleinsten stabilen Widgettest fuer den
bestehenden Uebergang von einer GAEB-Importzeile in den Importdetaildialog.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

Die `Details`-Aktion ruft `_openQuoteImportDetail(importId)` auf. Der Pfad laedt
nacheinander:

- `getQuoteImport(importId)`
- `listQuoteImportItems(importId)`

Danach zeigt er einen `AlertDialog` mit Importmetadaten, Review-Summary und bis
zu fuenf Rohpositionen. Fuer einen reinen Lesetest genuegt `quotes.read`; ohne
`quotes.write` erscheinen weder `Zur Übernahme freigeben` noch
`Draft-Quote erzeugen`.

Runtime, `ApiClient` und Serververtrag reichen unveraendert aus.

## Minimale Fake-Erweiterung

Der vorhandene `_FakeApiClient` erhaelt zwei ID-gebundene Datenquellen:

```text
quoteImportDetails: Map<String, Map<String, dynamic>> = const {}
quoteImportItemMap: Map<String, List<dynamic>> = const {}
```

Die Overrides verwenden die angefragte Import-ID:

```text
getQuoteImport(id) -> quoteImportDetails[id] ?? {'id': id}
listQuoteImportItems(id) -> quoteImportItemMap[id] ?? const []
```

Die ID-Bindung ist dem einzelnen globalen Detailobjekt vorzuziehen: Ein falsch
weitergereichter Import-Identifier liefert dann nicht versehentlich trotzdem
den erwarteten Dialoginhalt.

Aufrufzaehler oder Mutationsaufzeichnungen sind fuer diesen read-only Test
nicht erforderlich.

## Deterministischer Testpayload

Der Test verwendet:

- Berechtigung `quotes.read`
- `initialFilters.projectId = project-1`
- genau einen Listeneintrag mit ID `import-detail-1`
- ein Detailobjekt unter derselben ID
- genau eine Rohposition unter derselben ID
- leere Approval-Request- und Rework-Listen durch die Fake-Defaults

Stabile Detailwerte:

```text
source_filename: detail-ausschreibung.x83
status: parsed
source_kind: gaeb_xml
project_id: project-1
item_count: 1
accepted_count: 0
rejected_count: 0
pending_count: 1
```

Stabile Rohposition:

```text
id: import-item-1
position_no: 01.01
description: Gelaender Nordseite
```

Upload- und Aktualisierungszeitpunkte koennen entfallen; der Dialog stellt
fehlende Werte bereits stabil als `-` dar. Der Test soll diese Formatierung
nicht erneut absichern.

## Testablauf

Neuer Testname:

```text
QuotesPage GAEB import details open read-only from preview
```

Ablauf:

1. Seite mit Projektfilter und einem Import laden.
2. Den eindeutigen `TextButton` mit `Details` antippen.
3. Auf den abgeschlossenen asynchronen Detail- und Positionsladepfad warten.
4. Einen `AlertDialog` und den Titel `detail-ausschreibung.x83` pruefen.
5. `Status: parsed`, `Projekt: project-1` und `Positionen: 1` pruefen.
6. `Position 01.01` und `Gelaender Nordseite` pruefen.
7. Sicherstellen, dass keine Review- oder Apply-Aktion sichtbar ist.
8. `Schließen` antippen und pruefen, dass der Dialog verschwunden ist.

Der Listeneintrag darf einen abweichenden Vorschau-Dateinamen tragen. Dadurch
belegt der Dialogtitel, dass wirklich das ID-gebundene Detail geladen wurde.

## Finder-Grenzen

- `find.widgetWithText(TextButton, 'Details')` vermeidet zufaellige Texttreffer.
- `find.byType(AlertDialog)` prueft den Navigationsuebergang direkt.
- Fachtexte werden einzeln statt als vollstaendiger Dialogbaum geprueft.
- Keine Assertion auf formatierte Zeitpunkte oder die gesamte Review-Summary.
- Keine Position antippen; dies wuerde einen zweiten Detailvertrag oeffnen.

## Implementierungsgrenze

Subtask 3.1.57.3 soll ausschliesslich
`client/test/sales_order_context_pages_test.dart` aendern:

- zwei Fake-Felder ergaenzen
- zwei read-only Overrides ergaenzen
- genau einen Widgettest ergaenzen

`client/lib/pages/quotes_page.dart` bleibt unveraendert, sofern die Umsetzung
keinen bislang unsichtbaren Defekt nachweist.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import details open read-only from preview"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- keine Runtime-Aenderung
- kein Review-, Apply- oder Uploadtest
- kein Test des Importpositionsdialogs
- kein Fehlerpfadtest
- keine Datumsformatierungs- oder vollstaendige Summary-Pruefung
- keine Importhistorie, Pagination, Suche, Filterung oder Sortierung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.57.2 ist abgeschlossen. Subtask 3.1.57.3 erweitert nur den bestehenden
Fake und implementiert genau den definierten read-only Importdetail-Widgettest.
