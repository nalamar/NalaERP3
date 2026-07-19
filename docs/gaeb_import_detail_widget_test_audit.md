# GAEB: Abschlussaudit des read-only Importdetail-Widgettests

## Ziel

Subtask 3.1.57.4 auditiert den in 3.1.57.3 ergaenzten Widgettest fuer den
read-only Uebergang von der Importvorschau in den GAEB-Importdetaildialog.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Audit-Ergebnis

Die Umsetzung erfuellt das in
`docs/gaeb_import_detail_widget_test_strategy.md` definierte Minimalziel:

- zwei ID-gebundene Fake-Datenquellen
- Overrides fuer `getQuoteImport(...)` und `listQuoteImportItems(...)`
- genau ein neuer Widgettest
- `quotes.read` als einzige Berechtigung
- gesetzter Projektfilter und genau ein Importlauf
- Oeffnung ueber den vorhandenen `Details`-Button
- Pruefung stabiler Import- und Positionsdaten
- Ausschluss schreibender Review- und Apply-Aktionen
- kontrolliertes Schliessen des Dialogs

Runtime, `ApiClient`, Backend, Datenbank und Berechtigungsvertrag wurden nicht
veraendert.

## ID-Bindung

Vorschau- und Detaildateiname unterscheiden sich bewusst. Der Test findet in
der Liste `vorschau-ausschreibung.x83`, erwartet im Dialog aber
`detail-ausschreibung.x83`. Zusammen mit den nach Import-ID indizierten
Fake-Maps verhindert dies einen Test, der trotz falsch weitergereichter ID
zufaellig erfolgreich waere.

Die Fallbacks der Overrides bleiben neutral:

- unbekanntes Detail liefert nur die angefragte ID
- unbekannte Positionsliste liefert eine leere Liste

Aufrufzaehler sind fuer diesen Vertrag nicht erforderlich, weil die sichtbaren
ID-gebundenen Detaildaten den erfolgreichen Ladepfad bereits belegen.

## Read-only Grenze

Der Test gewaehrt nur `quotes.read`. Beim Detailstatus `parsed` waere
`Zur Übernahme freigeben` mit `quotes.write` sichtbar; sein geprueftes Fehlen
belegt daher die Berechtigungsgrenze. `Draft-Quote erzeugen` bleibt ebenfalls
unsichtbar.

Keine Mutation, kein Fortschrittsdialog und kein Refreshpfad werden ausgeloest.
Die einzelne Rohposition wird nur angezeigt und nicht geoeffnet.

## Stabilitaet der Assertions

Geprueft werden:

- genau ein `AlertDialog`
- Detaildateiname
- Status
- Projekt
- Positionsanzahl
- Positionsnummer und Beschreibung
- fehlende Schreibaktionen
- verschwundener Dialog nach `Schließen`

Zeitformatierung, vollstaendige Review-Summary und der gesamte Widgetbaum
bleiben bewusst ungeprueft. Dadurch bleibt der Test eng am Navigations- und
Lesedatenvertrag.

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

- keine weitere Test- oder Runtime-Implementierung
- kein Review-, Apply-, Upload- oder Fehlerpfadtest
- kein Test des Importpositionsdialogs
- keine Importhistorie oder Pagination
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Abschlussentscheidung

Task 3.1.57 ist fachlich und technisch abgeschlossen. Der read-only Einstieg
in den GAEB-Importdialog ist mit gutem Signal abgesichert. Der naechste Leaf
soll den kleinsten fachlich wertvollen Folgeausbau nach dem nun getesteten
Importdetail-Einstieg neu inventarisieren.
