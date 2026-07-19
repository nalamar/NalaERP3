# GAEB: Minimalstrategie fuer den Positionsreview-Mutations-Widgettest

## Ziel

Subtask 3.1.60.2 definiert den kleinsten stabilen Widgettest fuer die
vorhandene Reviewmutation einer GAEB-Importposition. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Bestehender Vertrag

Mit `quotes.write` zeigt der Positionsdetaildialog `Review setzen`. Der
Reviewdialog initialisiert Status und Notiz aus dem aktuellen Detail, erlaubt
`pending`, `accepted` oder `rejected` und trimmt die Notiz beim Speichern.

Danach ruft der Positionsdialog auf:

```text
updateQuoteImportItemReview(
  importId,
  itemId,
  reviewStatus,
  reviewNote,
)
```

Die Antwort ersetzt den lokalen Positionsdetailzustand. Eine Snackbar meldet
`Review-Entscheidung wurde gespeichert`.

Runtime, `ApiClient` und Serververtrag sind dafuer bereits vollstaendig.

## Fake-Mutationsvertrag

Der `_FakeApiClient` erhaelt eine Aufzeichnungsliste:

```text
updatedQuoteImportItemReviews = <Map<String, dynamic>>[]
```

Der Override von `updateQuoteImportItemReview(...)`:

1. zeichnet `import_id`, `item_id`, `review_status` und `review_note` auf
2. liest das ID-gebundene Ausgangsdetail aus `quoteImportItemDetails`
3. gibt eine neue Map mit allen Ausgangsfeldern sowie aktualisiertem
   `review_status` und `review_note` zurueck

Die konfigurierte Ausgangsmap wird nicht mutiert. Dadurch bleibt der Fake
deterministisch und der read-only Test unveraendert.

## Separater Test

Neuer Testname:

```text
QuotesPage GAEB import item review forwards normalized decision
```

Der Test verwendet:

- `_prepareLargeViewport`
- Berechtigungen `quotes.read` und `quotes.write`
- Projektfilter `project-1`
- genau einen Import `import-review-1`
- genau eine Rohposition `import-review-item-1`
- ein ID-gebundenes Positionsdetail mit `review_status: pending`
- keine Quote-Verknuepfung

Der aeussere Importstatus kann `uploaded` bleiben. Damit erscheinen keine
Importfreigabe- oder Apply-Aktionen und der Test bleibt auf die
Positionsmutation begrenzt.

## Testablauf

1. Importdialog ueber den eindeutigen `Details`-Button oeffnen.
2. Die einzige Rohposition `Position 02.01` antippen.
3. `Review setzen` antippen.
4. Titel `Review-Entscheidung` und initialen Status `pending` bestaetigen.
5. Das `DropdownButtonFormField<String>` oeffnen und `accepted` waehlen.
6. In das Textfeld mit `decoration.labelText == 'Review-Notiz'` den Wert
   `  Fachlich geprueft  ` eingeben.
7. `Speichern` antippen und settle abwarten.
8. Exakt aufgezeichneten Payload pruefen:

```text
import_id: import-review-1
item_id: import-review-item-1
review_status: accepted
review_note: Fachlich geprueft
```

9. Im weiterhin offenen Positionsdetaildialog pruefen:
   - `Review-Status: accepted`
   - `Review-Notiz: Fachlich geprueft`
10. Erfolgssnackbar pruefen.
11. Positionsdetail- und Importdialog nacheinander schliessen.

## Finder-Grenzen bei drei Dialogebenen

- `find.widgetWithText(TextButton, 'Details')` fuer den Importdialog
- `find.widgetWithText(ListTile, 'Position 02.01')` fuer die Rohposition
- `find.widgetWithText(FilledButton, 'Review setzen')` fuer die Mutation
- `find.byType(DropdownButtonFormField<String>)` erst nach Oeffnung des
  Reviewdialogs verwenden
- Dropdownoption mit `find.text('accepted').last` auswaehlen
- Reviewnotiz ueber einen Widget-Praedikat-Finder auf
  `TextField.decoration.labelText` bestimmen
- `find.widgetWithText(FilledButton, 'Speichern')` fuer den Abschluss
- nach dem Speichern sind nur noch Positions- und Importdialog offen; beide
  ueber den jeweils innersten `Schließen`-Button beenden

Es wird keine starre Anzahl von `AlertDialog`-Widgets geprueft.

## Implementierungsgrenze

Subtask 3.1.60.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- eine Mutation-Aufzeichnungsliste
- einen Fake-Override
- genau einen separaten Widgettest

`client/lib/pages/quotes_page.dart` bleibt unveraendert, sofern kein bislang
unsichtbarer Runtime-Defekt auftritt.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import item review forwards normalized decision"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- keine Runtime-Aenderung
- kein Review-Fehler- oder Validierungstest
- keine Importfreigabe oder Draft-Quote-Erzeugung
- keine Quote-Navigation
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine KI-Automatisierung

## Ergebnis

3.1.60.2 ist abgeschlossen. Subtask 3.1.60.3 erweitert nur den bestehenden
Fake und implementiert genau den definierten Positionsreview-Widgettest.
