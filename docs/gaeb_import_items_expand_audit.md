# GAEB: Abschlussaudit der erweiterbaren Rohpositionen

## Ziel

Subtask 3.1.58.4 auditiert die in 3.1.58.3 implementierte Ein- und
Ausklappfunktion der geladenen Rohpositionen im GAEB-Importdetaildialog. Dieser
Leaf aendert weder Runtime- noch Testcode.

## Audit-Ergebnis

Die Implementierung erfuellt das in
`docs/gaeb_import_items_expand_strategy.md` definierte Minimalziel:

- lokaler Zustand `itemsExpanded` innerhalb `_openQuoteImportDetail(...)`
- kompakter Start jedes neu geoeffneten Dialogs
- maximal fuenf Positionen in der kompakten Ansicht
- alle bereits geladenen Positionen in der erweiterten Ansicht
- stabile Labels `Alle anzeigen` und `Weniger anzeigen`
- Umschaltaktion nur bei mehr als fuenf Positionen
- Zustandserhalt bei Refresh mit weiterhin mehr als fuenf Treffern
- Normalisierung bei hoechstens fuenf neuen Treffern
- erweiterter, erfolgreicher Widgettest mit sechs Positionen

Backend, `ApiClient`, Positionsabfrage, Positionsdetaildialog und
Berechtigungsvertrag wurden nicht erweitert.

## Zustands- und Lebensdauerpruefung

`itemsExpanded` wird nach dem initialen Laden und vor `showDialog(...)`
angelegt. Der Zustand gehoert dadurch genau einer Dialoginstanz und wird nicht
in `_QuotesPageState` getragen. Schliessen und erneutes Oeffnen startet wieder
kompakt.

Die Umschaltaktion verwendet ausschliesslich `setDialogState`. Sie baut weder
die Angebotsseite neu noch beeinflusst sie die Expand-Zustaende von
Importvorschau oder Approval-Karten.

## Refreshvertrag

`refreshImportState()` ersetzt Detail und Positionen weiterhin atomar im
Dialogzustand:

- bei mehr als fuenf neuen Treffern bleibt eine erweiterte Arbeitssicht offen
- bei hoechstens fuenf Treffern wird auf kompakt normalisiert
- ein Ladefehler erreicht den `setDialogState`-Block nicht und behaelt deshalb
  die bisherigen Dialogdaten und den Anzeigezustand

Damit entsteht kein unsichtbarer Expand-State bei kleiner gewordener Liste.

## Positionsvertrag

Map-Konvertierung, Reihenfolge, Positionsnummer, Beschreibung, Chevron und
`_openQuoteImportItemDetail(importId, itemId)` bleiben fuer alle sichtbaren
Positionen identisch. Die Erweiterung veraendert nur die Quelle der gerenderten
Teilmenge.

`Alle anzeigen` zeigt alle aktuell geladenen `items`. Es wird keine zusaetzliche
Abfrage ausgeloest und keine serverseitig vollstaendige oder paginierte
Gesamtheit behauptet.

## Testabdeckung

Der vorhandene read-only Importdetail-Test verwendet nun sechs eindeutig
nummerierte, ID-gebundene Positionen und prueft:

- Positionen eins bis fuenf anfangs sichtbar
- Position sechs anfangs unsichtbar
- Expansion macht Position sechs sichtbar
- Einklappen blendet Position sechs wieder aus
- stabile Gegenlabels der Umschaltaktion
- unveraenderte Detailmetadaten und read-only Berechtigungsgrenze
- kontrolliertes Schliessen des Dialogs

`tester.ensureVisible` macht die im scrollbaren Dialog liegenden Buttons vor
dem Antippen sichtbar und vermeidet Viewport-abhaengige Gesten.

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
- keine Pagination, Suche, Filterung oder Sortierung
- keine Aenderung des Positionsdetaildialogs
- kein Review-, Apply-, Upload- oder Fehlerpfadtest
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Abschlussentscheidung

Task 3.1.58 ist fachlich und technisch abgeschlossen. Alle bereits geladenen
Rohpositionen sind im Importdialog erreichbar und der Vertrag ist eng
widgetgetestet. Der naechste Leaf soll den kleinsten fachlich wertvollen
Folgeausbau neu inventarisieren.
