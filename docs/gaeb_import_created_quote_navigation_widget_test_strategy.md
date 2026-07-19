# GAEB: Minimalstrategie fuer den Erzeugte-Quote-Navigations-Widgettest

## Ziel

Subtask 3.1.63.2 definiert den kleinsten stabilen Widgettest fuer die
Navigation von einem angewendeten GAEB-Import zur erzeugten Draft-Quote.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

Mit `created_quote_id` und `quotes.read` zeigt der Importdetaildialog
`Quote öffnen`. Die Aktion:

1. schliesst den Importdetaildialog
2. setzt Status- und Folgebelegfilter zurueck
3. ruft `getQuote(createdQuoteId)` auf
4. setzt die Antwort als ausgewaehlte Quote
5. ruft `_load()` auf
6. zeigt `Erzeugte Quote wurde geöffnet`

`_load()` laedt die Angebotsliste und ruft fuer die bereits ausgewaehlte ID
erneut `_loadDetail(selectedId)` auf. Der Test muss daher zwei identische
`getQuote`-Abrufe erwarten.

Runtime und `ApiClient` sind fuer diesen Pfad bereits vollstaendig.

## Fake-Aufzeichnungsvertrag

Der `_FakeApiClient` erhaelt:

```text
requestedQuoteIds = <String>[]
```

Der vorhandene Override von `getQuote(id)` wird von einem Ausdruck zu einer
Methode erweitert:

1. `id` an `requestedQuoteIds` anhaengen
2. `quoteDetail` liefern
3. nur ohne konfiguriertes Detail den neutralen Fallback `{'id': id}` liefern

Bestehende Tests erhalten dieselben Antworten wie zuvor. Die neue
Aufzeichnung veraendert keine Produktivsemantik.

## Separater Test

Neuer Testname:

```text
QuotesPage GAEB created quote navigation opens selected draft
```

Der Test verwendet:

- `_prepareLargeViewport`
- nur die Berechtigung `quotes.read`
- Projektfilter `project-1`
- genau einen Import `import-open-1`
- Importstatus `applied`
- `created_quote_id: quote-gaeb-open-1`
- eine leere Importpositionsliste
- genau eine Quote-Zusammenfassung in `quoteList`
- ein eindeutiges `quoteDetail`

Quote-Zusammenfassung und Detail verwenden:

```text
id: quote-gaeb-open-1
number: ANG-GAEB-OPEN-0001
status: draft
contact_name: GAEB Navigationskunde
project_name: GAEB Navigationsprojekt
currency: EUR
```

Das Detail ergaenzt leere `items`, neutrale Datums-/Notizfelder und
Summenwerte. Dadurch kann die vorhandene rechte Detailansicht ohne
fachfremde Folgebelege gerendert werden.

## Testablauf

1. Seite mit Projektfilter laden.
2. Importdialog ueber den eindeutigen `Details`-Button oeffnen.
3. `Status: applied`, `Erzeugte Quote: quote-gaeb-open-1` und
   `Quote öffnen` pruefen.
4. `Quote öffnen` antippen und `pumpAndSettle()` abwarten.
5. Exakt folgende Abruffolge pruefen:

```text
[
  quote-gaeb-open-1,
  quote-gaeb-open-1,
]
```

Der erste Abruf stammt aus der Aktion, der zweite aus `_loadDetail` innerhalb
von `_load()`.

6. Pruefen, dass kein `AlertDialog` mehr sichtbar ist.
7. In der Angebotsdetailansicht pruefen:
   - `ANG-GAEB-OPEN-0001`
   - `Status: draft`
   - `Kunde: GAEB Navigationskunde`
   - `Projekt: GAEB Navigationsprojekt`
8. Snackbar `Erzeugte Quote wurde geöffnet` pruefen.

Der Test startet keine Bearbeitung, PDF-Erzeugung, Freigabe oder
Konvertierung.

## Listen- und Reloadgrenze

`quoteList` enthaelt dieselbe Quote wie `quoteDetail`. Damit bleibt die
Angebotsliste nach `_load()` konsistent und die ListTile-Auswahl kann dieselbe
ID markieren. Der Test behauptet keine bestimmte Anzahl der sichtbaren
Angebotsnummer, weil Nummer und Detailtitel parallel gerendert werden duerfen.

Die eindeutigen Kunde-/Projekt-Chips belegen die rechte Detailansicht. Die
zweifache ID-Aufzeichnung belegt direkten Abruf und Reload ohne interne
State-Objekte zu inspizieren.

## Finder-Grenzen

- `find.widgetWithText(TextButton, 'Details')` fuer den Importdialog
- `find.widgetWithText(FilledButton, 'Quote öffnen')` fuer die Navigation
- `find.byType(AlertDialog)` nur nach abgeschlossenem Navigation-Settle auf
  `findsNothing` pruefen
- eindeutige Chiptexte fuer die geladene Quote verwenden

Es wird weder eine Widget-Hierarchieposition noch eine ListTile-Auswahlfarbe
geprueft.

## Implementierungsgrenze

Subtask 3.1.63.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- eine Quote-ID-Aufzeichnungsliste
- die kleine instrumentierende Erweiterung von `getQuote`
- genau einen separaten read-only Widgettest

`client/lib/pages/quotes_page.dart` bleibt unveraendert, sofern kein bislang
unsichtbarer Runtime-Defekt auftritt.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB created quote navigation opens selected draft"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- keine Runtime-Aenderung
- keine erneute Apply-Mutation
- keine Quote-Bearbeitung, Freigabe oder Konvertierung
- keine Positions-, Preis-, Material- oder Kalkulationspruefung
- kein Fehler- oder Berechtigungsnegativtest
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine KI-Automatisierung

## Ergebnis

3.1.63.2 ist abgeschlossen. Subtask 3.1.63.3 instrumentiert nur den bestehenden
Quote-Fake und implementiert genau den definierten Navigations-Widgettest.
