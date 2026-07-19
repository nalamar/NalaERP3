# GAEB: Minimalstrategie fuer den read-only Positionsdetail-Widgettest

## Ziel

Subtask 3.1.59.2 definiert den kleinsten stabilen Widgettest fuer den
bestehenden Uebergang von einer GAEB-Rohposition in ihren Positionsdetaildialog.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

Ein Rohpositions-`ListTile` ruft
`_openQuoteImportItemDetail(importId, itemId)` auf. Der Pfad laedt genau
`getQuoteImportItem(importId, itemId)` und zeigt einen verschachtelten
`AlertDialog`.

Mit nur `quotes.read` bleibt der Dialog lesend:

- `Review setzen` erfordert `quotes.write` und bleibt verborgen.
- `Quote öffnen` erscheint nur mit gesetzter `linked_quote_id`.

Runtime, `ApiClient` und Serververtrag reichen unveraendert aus.

## Minimale Fake-Erweiterung

Der `_FakeApiClient` erhaelt eine zweistufig ID-gebundene Datenquelle:

```text
quoteImportItemDetails:
  Map<String, Map<String, Map<String, dynamic>>> = const {}
```

Die erste Ebene ist die Import-ID, die zweite Ebene die Positions-ID. Der
Override lautet sinngemaess:

```text
getQuoteImportItem(importId, itemId)
  -> quoteImportItemDetails[importId]?[itemId]
     ?? {'id': itemId, 'import_id': importId}
```

Damit kann der Test nicht trotz falsch weitergereichter Import- oder Item-ID
zufaellig das erwartete Detail erhalten. Aufrufzaehler sind nicht erforderlich,
weil die eindeutigen sichtbaren Detailwerte den erfolgreichen Lookup belegen.

## Stabiler Positionsdetail-Payload

Fuer `import-detail-1` und `import-item-1` wird verwendet:

```text
id: import-item-1
import_id: import-detail-1
position_no: 01.01
outline_no: Los 1
qty: 12.5
unit: m
is_optional: false
review_status: pending
parser_hint: Mengenansatz pruefen
review_note: ''
description: Detail Gelaender Nordseite
linked_quote_id: ''
```

Die Werte `Los 1`, `12.5`, `Mengenansatz pruefen` und
`Detail Gelaender Nordseite` kommen im aeusseren Dialog nicht vor und eignen
sich als eindeutige Nachweise des inneren Detailpfads.

## Testanpassung

Der bestehende Test

```text
QuotesPage GAEB import details open read-only from preview
```

wird nach der bisherigen Expand-/Collapse-Pruefung erweitert. Es entsteht kein
zweiter nahezu identischer Setup-Test.

Ablauf:

1. Nach dem Einklappen `Position 01.01` mit `tester.ensureVisible` sichtbar
   machen.
2. Den zugehoerigen `ListTile` antippen.
3. Auf den Positionsdetail-Ladepfad warten.
4. Eindeutige Texte pruefen:
   - `Gliederung: Los 1`
   - `Menge: 12.5`
   - `Einheit: m`
   - `Review-Status: pending`
   - `Parser-Hinweis: Mengenansatz pruefen`
   - `Detail Gelaender Nordseite`
5. `Review setzen` und `Quote öffnen` sind nicht sichtbar.
6. Den innersten gefundenen `TextButton` `Schließen` antippen.
7. Pruefen, dass die eindeutigen Positionsdetailtexte verschwunden sind, der
   aeussere Titel `detail-ausschreibung.x83` aber weiterhin sichtbar ist.
8. Anschliessend den verbleibenden aeusseren `Schließen`-Button antippen und
   den bisherigen Abschluss beibehalten.

## Finder-Grenzen bei zwei Dialogebenen

- Den Rohpositions-Tile mit
  `find.widgetWithText(ListTile, 'Position 01.01')` finden.
- Keine starre Anzahl aller `AlertDialog`-Widgets pruefen; das Offstage-Verhalten
  verschachtelter Modalrouten ist kein Fachvertrag.
- Eindeutige Detailtexte belegen den inneren Dialog stabiler als ein mehrfach
  vorkommender Positionstitel.
- Zum Schliessen des inneren Dialogs
  `find.widgetWithText(TextButton, 'Schließen').last` verwenden.
- Nach dessen Schliessen den verbleibenden `Schließen`-Button verwenden.

## Implementierungsgrenze

Subtask 3.1.59.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- ein verschachteltes Fake-Feld
- einen read-only Override
- den vorhandenen Importdetail-Test um den inneren Dialogpfad erweitern

`client/lib/pages/quotes_page.dart` bleibt unveraendert, sofern kein bislang
unsichtbarer Runtime-Defekt auftritt.

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
- kein Review-Mutations- oder Formulartest
- keine Quote-Navigation
- kein Fehlerpfadtest
- keine Suche, Filterung, Pagination oder Sortierung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.59.2 ist abgeschlossen. Subtask 3.1.59.3 erweitert nur den bestehenden
Fake und Widgettest um den read-only Positionsdetailpfad.
