# GAEB: Abschlussaudit des read-only Positionsdetail-Widgettests

## Ziel

Subtask 3.1.59.4 auditiert den in 3.1.59.3 ergaenzten read-only Testpfad vom
GAEB-Importdialog in den Positionsdetaildialog. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Audit-Ergebnis

Die Umsetzung erfuellt das in
`docs/gaeb_import_item_detail_widget_test_strategy.md` definierte Minimalziel:

- zweistufig ID-gebundene Fake-Datenquelle
- Override fuer `getQuoteImportItem(importId, itemId)`
- Weiterverwendung des vorhandenen Importdetail-Widgettests
- Oeffnung einer sichtbaren Rohposition
- Pruefung eindeutiger Positionsdetailwerte
- read-only Berechtigungsgrenze ohne `Review setzen`
- keine Quote-Navigation ohne Verknuepfung
- getrenntes Schliessen von innerem und aeusserem Dialog

Runtime, `ApiClient`, Backend, Datenbank und Berechtigungsvertrag wurden nicht
veraendert.

## ID-Bindung

`quoteImportItemDetails` ist zuerst nach Import-ID und danach nach Item-ID
indiziert. Nur die Kombination `import-detail-1`/`import-item-1` liefert die
eindeutigen Detaildaten. Der neutrale Fallback enthaelt lediglich beide
angefragten IDs.

Die sichtbaren Werte `Los 1`, `12.5`, `Mengenansatz pruefen` und
`Detail Gelaender Nordseite` kommen im aeusseren Dialog nicht vor. Ihr Auftreten
belegt damit den erfolgreichen Lookup ueber beide weitergereichten IDs.

## Verschachtelte Dialogfuehrung

Der Test verwendet keine starre Anzahl von `AlertDialog`-Widgets. Stattdessen
belegen eindeutige Detailtexte den inneren Dialog. Der innerste gefundene
`Schließen`-Button beendet die Positionsdetailroute.

Danach gilt gleichzeitig:

- die eindeutigen Positionsdetailtexte sind verschwunden
- `detail-ausschreibung.x83` aus dem aeusseren Importdialog bleibt sichtbar

Erst der anschliessende verbleibende `Schließen`-Button beendet den
Importdialog. Damit ist die Modalrouten-Reihenfolge fachlich abgesichert, ohne
Offstage-Implementierungsdetails zu testen.

## Read-only Vertrag

Der Test verwendet weiterhin nur `quotes.read` und eine leere
`linked_quote_id`:

- `Review setzen` bleibt verborgen
- `Quote öffnen` bleibt verborgen
- keine Mutation, kein Formular und kein Seitenreload werden ausgeloest

Gliederung, Menge, Einheit, Reviewstatus, Parserhinweis und Beschreibung werden
als stabile Lesedaten geprueft.

## Verifikation

Im Verzeichnis `client/` erneut ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import details open read-only from preview"
```

Ergebnis: der gezielte Widgettest ist erfolgreich.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten Hinweise zum
ungenutzten `purchase_orders_page.dart`-Import und nie gesetzten
`convertedQuote`-Fakeparameter bleiben allgemeine Harness-Themen.

## Nicht-Ziele

- keine weitere Runtime- oder Testimplementierung
- kein Review-Mutations- oder Formulartest
- keine Quote-Navigation
- kein Fehlerpfadtest
- keine Suche, Filterung, Pagination oder Sortierung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Abschlussentscheidung

Task 3.1.59 ist fachlich und technisch abgeschlossen. Der read-only
Positionsdetail-Uebergang und die verschachtelte Dialogfuehrung sind eng
abgesichert. Der naechste Leaf soll den kleinsten fachlich wertvollen
Folgeausbau neu inventarisieren.
