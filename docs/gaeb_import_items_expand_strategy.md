# GAEB: Minimalstrategie fuer erweiterbare Rohpositionen im Importdialog

## Ziel

Subtask 3.1.58.2 definiert das technische Minimalmodell fuer das Ein- und
Ausklappen der bereits geladenen Rohpositionen im GAEB-Importdetaildialog.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

`_openQuoteImportDetail(importId)` laedt Detail und vollstaendige
Positionsantwort vor dem Oeffnen des Dialogs. Innerhalb des `StatefulBuilder`
wird die Darstellung aktuell fest auf `items.take(5)` begrenzt. Der
Positionsdetail-Callback verwendet weiterhin Import- und Positions-ID.

Die Erweiterung bleibt vollstaendig innerhalb dieses bestehenden Dialog- und
Ladevertrags. `ApiClient`, Backend und Positionsdetaildialog bleiben
unveraendert.

## Lokaler Dialogzustand

Direkt nach dem initialen Laden und vor `showDialog(...)` wird eingefuehrt:

```text
bool itemsExpanded = false;
```

Der Zustand ist eine lokale Variable von `_openQuoteImportDetail(...)` und wird
ueber den vorhandenen `StatefulBuilder.setDialogState` aktualisiert. Jeder neu
geoeffnete Importdialog startet dadurch kompakt. Es entsteht kein zusaetzlicher
Zustand in `_QuotesPageState`.

## Sichtbare Rohpositionen

Die Ableitung wird minimal angepasst:

```text
source = itemsExpanded ? items : items.take(5)
```

Map-Konvertierung, Reihenfolge, Titel, Beschreibung, Chevron und
`_openQuoteImportItemDetail(...)` bleiben unveraendert. `remainingItems` und
der statische `+n weitere Positionen`-Text entfallen.

## Umschaltaktion

Nach den sichtbaren ListTiles erscheint bei `items.length > 5` ein
`TextButton`:

- kompakt: `Alle anzeigen`
- erweitert: `Weniger anzeigen`

Der Button verwendet `setDialogState`, nicht den Seiten-`setState`. Bei
hoechstens fuenf Positionen erscheint keine Aktion.

`Alle anzeigen` bezeichnet alle aktuell durch `listQuoteImportItems(...)`
geladenen Positionen. Die Funktion fuehrt keine weitere Serveranfrage aus und
behauptet keine serverseitig paginierte Gesamtheit.

## Refreshverhalten

`refreshImportState()` ersetzt weiterhin `detail` und `items`. Im selben
`setDialogState` gilt:

- mehr als fuenf neue Positionen erhalten den aktuellen Expand-State
- hoechstens fuenf neue Positionen setzen `itemsExpanded = false`

Damit bleibt eine laufende erweiterte Arbeitssicht bei einem Review-Refresh
erhalten, solange sie noch sinnvoll ist. Ein unsichtbarer Expand-State bleibt
bei kleiner gewordener Ergebnismenge nicht bestehen. Der Fehlerpfad aendert
weder Liste noch Zustand.

## Minimale Testanpassung

Der vorhandene Test

```text
QuotesPage GAEB import details open read-only from preview
```

wird erweitert, statt einen zweiten fast identischen Dialogtest anzulegen.

Der ID-gebundene Fake liefert sechs eindeutig nummerierte Positionen. Das
Detailobjekt verwendet `item_count: 6` und `pending_count: 6`.

Zusaetzlicher Testablauf nach den vorhandenen Dialogassertions:

1. Positionen eins und fuenf sind sichtbar.
2. Position sechs ist nicht sichtbar.
3. `Alle anzeigen` ist sichtbar; bei Bedarf mit `tester.ensureVisible` in den
   scrollbaren Dialogbereich bringen.
4. Button antippen und settle abwarten.
5. Position sechs und `Weniger anzeigen` sind sichtbar.
6. `Weniger anzeigen` sichtbar machen, antippen und settle abwarten.
7. Position sechs ist wieder unsichtbar und `Alle anzeigen` sichtbar.
8. Die bisherigen read-only Assertions und das kontrollierte Schliessen bleiben
   erhalten.

Die sechs Beschreibungen muessen eindeutig sein. Der Test tippt keine
Rohposition an und oeffnet keinen Positionsdetaildialog.

## Implementierungsgrenze

Subtask 3.1.58.3 aendert nur:

- `client/lib/pages/quotes_page.dart`
- `client/test/sales_order_context_pages_test.dart`

Produktionsseitig sind ausschliesslich lokaler Dialogzustand, sichtbare Quelle,
Refreshnormalisierung und Umschaltbutton betroffen. Testseitig wird nur der
vorhandene read-only Importdetail-Test auf sechs Positionen erweitert.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import details open read-only from preview"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- keine neue Positionsabfrage oder Pagination
- keine Suche, Filterung oder Sortierung
- keine Aenderung des Positionsdetaildialogs
- kein Review-, Apply-, Upload- oder Fehlerpfadtest
- keine generische Dialog- oder Listenkomponente
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.58.2 ist abgeschlossen. Subtask 3.1.58.3 implementiert genau den lokalen
Expand-/Collapse-Vertrag und erweitert den vorhandenen Widgettest.
