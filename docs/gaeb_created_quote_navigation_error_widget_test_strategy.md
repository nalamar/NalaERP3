# GAEB: Minimalstrategie fuer den Erzeugte-Quote-Navigationsfehler-Widgettest

## Ziel

Subtask 3.1.64.2 definiert den kleinsten stabilen Widgettest fuer den bereits
vorhandenen Fehlerpfad beim Oeffnen einer ueber `created_quote_id`
referenzierten Draft-Quote. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Zu pruefender Runtime-Vertrag

Im Detaildialog eines `applied` GAEB-Imports wird `Quote öffnen` angezeigt,
wenn:

- `created_quote_id` nicht leer ist
- `quotes.read` vorhanden ist

Die Aktion schliesst zuerst den Importdialog, setzt die lokalen Listenfilter
zurueck und ruft `getQuote(createdQuoteId)` auf. Erst nach einem erfolgreichen
Abruf wird `_selected` ersetzt und `_load()` ausgefuehrt. Bei einer Exception
bleibt die bisherige Auswahl deshalb erhalten und `_quoteErrorMessage` wird in
einer Snackbar angezeigt.

## Kleinste Fake-Erweiterung

Der `_FakeApiClient` erhaelt genau eine optionale Fehler-Map:

```dart
this.quoteDetailErrors = const {},

final Map<String, Object> quoteDetailErrors;
```

`getQuote` bleibt fuer alle bestehenden Tests unveraendert und wird nur um die
ziel-ID-spezifische Abzweigung ergaenzt:

```dart
@override
Future<Map<String, dynamic>> getQuote(String id) async {
  requestedQuoteIds.add(id);
  final error = quoteDetailErrors[id];
  if (error != null) throw error;
  return quoteDetail ?? <String, dynamic>{'id': id};
}
```

Warum eine Map statt eines globalen Fehlerfeldes:

- die bereits ausgewaehlte Quote kann beim initialen Seitenladen erfolgreich
  geladen werden
- nur die durch `created_quote_id` bezeichnete Zielquote schlaegt fehl
- der Fake bleibt deterministisch und wiederverwendbar
- der Default `const {}` veraendert keinen bestehenden Test

## Testname und Position

Der neue Test steht unmittelbar hinter
`QuotesPage GAEB created quote navigation opens selected draft`:

```text
QuotesPage GAEB created quote navigation keeps selection on load failure
```

Damit bleiben Erfolgs- und Fehlervertrag derselben Aktion direkt
nebeneinander, ohne den bestehenden Erfolgstest zu erweitern.

## Minimaler Testaufbau

### Berechtigung

Nur:

```dart
permissions: const {'quotes.read'}
```

### Bestehende Quote

Die Seite startet ueber
`CommercialListContext.detail('quote-existing-1')` mit einer eindeutig
erkennbaren Quote:

- ID: `quote-existing-1`
- Nummer: `ANG-BESTEHEND-0001`
- Status: `draft`
- Kunde: `Bestehender Kunde`
- Projekt: `Bestehendes Projekt`

`quoteList` und `quoteDetail` liefern diesen bestehenden Kontext. Dadurch ist
vor der Fehleraktion nachweisbar eine valide Auswahl vorhanden.

### GAEB-Import

Der Importpayload bleibt minimal:

- ID: `import-open-failure-1`
- Status: `applied`
- Projekt: `project-1`
- `created_quote_id: quote-missing-1`
- leere Importpositionsliste

### Deterministischer Fehler

Nur `quote-missing-1` wird in `quoteDetailErrors` konfiguriert:

```dart
const ApiException(
  statusCode: 404,
  code: 'quote_not_found',
  message: 'Erzeugte Quote ist nicht mehr verfügbar',
)
```

Die fachliche API-Meldung ist stabiler als eine Assertion auf die
Stringdarstellung einer generischen Exception. Zugleich belegt sie den
vorhandenen `ApiException`-Zweig von `_quoteErrorMessage`.

## Ablauf

1. `QuotesPage` mit bestehendem Detailkontext und Projektfilter rendern.
2. Initiales Laden abwarten und `ANG-BESTEHEND-0001` pruefen.
3. Importdetail ueber `Details` oeffnen.
4. `Status: applied`, `Erzeugte Quote: quote-missing-1` und `Quote öffnen`
   pruefen.
5. `Quote öffnen` antippen und `pumpAndSettle()` abwarten.
6. Abruffolge, Dialogschluss, erhaltene Auswahl und Fehlerfeedback pruefen.

## Stabile Assertions

### Abruffolge

```dart
expect(api.requestedQuoteIds, const [
  'quote-existing-1',
  'quote-missing-1',
]);
```

Der erste Abruf stammt aus dem initialen Detailkontext, der zweite aus der
GAEB-Navigation. Da der zweite Abruf scheitert, gibt es keinen anschliessenden
`_load()`-Reload.

### Dialog und Auswahl

- `find.byType(AlertDialog)` findet nichts mehr
- `ANG-BESTEHEND-0001` bleibt sichtbar
- `Status: draft` bleibt sichtbar
- `Kunde: Bestehender Kunde` bleibt sichtbar
- `Projekt: Bestehendes Projekt` bleibt sichtbar
- eine Nummer oder ein Detail der fehlenden Zielquote wird nicht erwartet

### Feedback

- `Erzeugte Quote ist nicht mehr verfügbar` ist genau einmal sichtbar
- `Erzeugte Quote wurde geöffnet` ist nicht sichtbar

Nicht auf Snackbar-Animationen, Zeitpunkte oder den kompletten Widgetbaum
asserten.

## Bewusste Abgrenzung

Der Implementierungsleaf aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- eine optionale Fehler-Map im Fake
- eine ziel-ID-spezifische Abzweigung in `getQuote`
- genau einen read-only Widgettest

Nicht Teil des Scopes:

- Aenderung an `QuotesPage` oder `ApiClient`
- Apply-Fehler oder erneute Quote-Erzeugung
- Berechtigungsnegativtest
- Filter-, Retry- oder Dialog-Redesign
- Positions-, Preis-, Material-, Kalkulations- oder KI-Logik
- Backend-, API-, Datenbank- oder Permission-Aenderung

## Verifikation des Folgeleafs

Nach der Implementierung genuegen:

```powershell
cd client
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB created quote navigation keeps selection on load failure"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Zusaetzlich `git diff --check` ausfuehren. Bereits bekannte, sachfremde
Harness-Warnungen werden dokumentiert, aber nicht in diesem Leaf bereinigt.

## Ergebnis

3.1.64.2 ist abgeschlossen. Subtask 3.1.64.3 erweitert ausschliesslich den
bestehenden Fake um die ziel-ID-spezifische Fehlerkonfiguration und
implementiert genau den beschriebenen read-only Fehler-Widgettest.
