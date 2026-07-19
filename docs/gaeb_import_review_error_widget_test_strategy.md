# GAEB: Minimalstrategie fuer den Importlauf-Freigabefehler-Widgettest

## Ziel

Subtask 3.1.66.2 definiert den kleinsten stabilen Widgettest fuer eine wegen
offener Positionen serverseitig abgewiesene GAEB-Importlauf-Freigabe. Dieser
Leaf aendert keine Runtime- oder Testlogik.

## Zu pruefender Runtime-Vertrag

Im Detaildialog eines `parsed` GAEB-Imports ist
`Zur Übernahme freigeben` mit `quotes.write` sichtbar. Die Aktion oeffnet einen
nicht dismissbaren Fortschrittsdialog und ruft
`markQuoteImportReviewed(importId)` auf.

Bei einer Exception:

- wird nur der Fortschrittsdialog geschlossen
- der Importdetaildialog bleibt geoeffnet
- der lokale Status wird nicht auf `reviewed` gesetzt
- Detail- und Listenrefresh werden nicht ausgefuehrt
- `_quoteErrorMessage` zeigt die `ApiException.message` in einer Snackbar

Die fachliche Entscheidung, ob offene Positionen eine Freigabe verhindern,
bleibt serverseitig. Der Test sichert die korrekte Clientreaktion auf diese
Guard Rail ab.

## Fake-Erweiterung mit sauberer Zustandssemantik

Der vorhandene Fake verwendet `reviewedQuoteImportIds` nicht nur zur
Aufzeichnung, sondern auch als simulierten Erfolgszustand in
`getQuoteImport`. Eine fehlgeschlagene Freigabe darf deshalb nicht in diese
Liste eingetragen werden.

Der `_FakeApiClient` erhaelt:

```dart
this.quoteImportReviewErrors = const {},

final Map<String, Object> quoteImportReviewErrors;
final List<String> attemptedQuoteImportReviewIds = [];
```

`markQuoteImportReviewed` trennt Versuch und Erfolg explizit:

```dart
@override
Future<Map<String, dynamic>> markQuoteImportReviewed(String importId) async {
  attemptedQuoteImportReviewIds.add(importId);
  final error = quoteImportReviewErrors[importId];
  if (error != null) throw error;
  reviewedQuoteImportIds.add(importId);
  final current =
      quoteImportDetails[importId] ?? <String, dynamic>{'id': importId};
  return <String, dynamic>{...current, 'status': 'reviewed'};
}
```

Der Default `const {}` erhaelt alle bestehenden Erfolgstests. Die separate
Versuchsliste verhindert, dass der Fake nach einer Exception spaeter
faelschlich einen `reviewed`-Zustand liefert.

## Testname und Position

Der neue Test steht unmittelbar hinter
`QuotesPage GAEB import review enables draft quote action`:

```text
QuotesPage GAEB import review keeps parsed state with pending item on failure
```

Damit liegen Erfolgs- und Fehlervertrag derselben Freigabeaktion direkt
nebeneinander. Der bestehende Erfolgstest bleibt inhaltlich unveraendert.

## Minimaler Testaufbau

### Berechtigungen

Genau:

```dart
permissions: const {'quotes.read', 'quotes.write'}
```

### Importlauf

Ein einzelner Import reicht:

- ID: `import-review-failure-1`
- Dateiname: `freigabe-fehler-ausschreibung.x83`
- Status: `parsed`
- Projekt: `project-1`
- `item_count: 1`
- `accepted_count: 0`
- `rejected_count: 0`
- `pending_count: 1`
- leere Importpositionsliste fuer diesen Dialogtest

Die Summary bildet die offene fachliche Entscheidung sichtbar ab, ohne einen
separaten Positionsdialog zu testen.

### Deterministischer Fehler

Nur `import-review-failure-1` wird in `quoteImportReviewErrors` konfiguriert:

```dart
const ApiException(
  statusCode: 409,
  code: 'pending_quote_import_items',
  message: 'Importlauf enthält noch offene Positionen',
)
```

Die strukturierte Meldung belegt den `ApiException`-Zweig von
`_quoteErrorMessage` und beschreibt die reale fachliche Guard Rail.

## Testablauf

1. `QuotesPage` mit Projektfilter rendern und initiales Laden abwarten.
2. Importdetail ueber `Details` oeffnen.
3. `Status: parsed` und die Summary mit einer offenen Position pruefen.
4. `Zur Übernahme freigeben` als sichtbar und `Draft-Quote erzeugen` als
   verborgen pruefen.
5. Freigabeaktion antippen und `pumpAndSettle()` abwarten.
6. Versuch, ausbleibenden Erfolg, beendeten Fortschritt, erhaltenen Dialog,
   unveraenderten Zustand und Fehlerfeedback pruefen.

## Stabile Assertions

### Versuch und Erfolgszustand

```dart
expect(
  api.attemptedQuoteImportReviewIds,
  const ['import-review-failure-1'],
);
expect(api.reviewedQuoteImportIds, isEmpty);
```

Damit sind genau ein Versuch und kein simulierter Freigabeerfolg belegt.

### Dialog-, Status- und Summaryvertrag

- `find.byType(AlertDialog)` findet genau einen Dialog
- der Dialog enthaelt `freigabe-fehler-ausschreibung.x83`
- `Importlauf wird zur Übernahme freigegeben...` ist nicht mehr sichtbar
- `Status: parsed` bleibt sichtbar
- `Review-Summary: 0 übernommen, 0 abgelehnt, 1 offen` bleibt sichtbar
- `Zur Übernahme freigeben` bleibt sichtbar
- `Draft-Quote erzeugen` bleibt verborgen

Den Dateinamen innerhalb des `AlertDialog` suchen, weil derselbe Text zugleich
in der Hintergrundliste stehen kann.

### Fehler- und Negativvertrag

- `Importlauf enthält noch offene Positionen` ist genau einmal sichtbar
- `Importlauf wurde freigegeben` ist nicht sichtbar
- `Status: reviewed` ist nicht sichtbar
- kein Apply-, Erzeugungs- oder Navigationssignal wird behauptet

Nicht auf Snackbar-Dauer, einzelne Animationsframes oder den gesamten
Dialogtext assertieren.

## Bewusste Abgrenzung

Subtask 3.1.66.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- eine optionale Freigabefehler-Map im Fake
- eine separate Versuchsliste
- die semantisch korrekte Reihenfolge in `markQuoteImportReviewed`
- genau einen Widgettest fuer den Fehlerausgang

Nicht Teil des Scopes:

- Aenderung an `QuotesPage` oder `ApiClient`
- Backend-, API-, Datenbank- oder Permission-Aenderung
- clientseitige Vorabvalidierung von `pending_count`
- Positionsreview-Fehler oder Berechtigungsnegativtest
- Retry-, Recovery- oder Dialog-Redesign
- Transformations-, Mapping-, Kalkulations- oder KI-Logik

## Verifikation des Folgeleafs

Nach der Implementierung genuegen:

```powershell
cd client
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import review keeps parsed state with pending item on failure"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Zusaetzlich `git diff --check` ausfuehren. Bereits bekannte, sachfremde
Harness-Warnungen werden dokumentiert, aber nicht in diesem Leaf bereinigt.

## Ergebnis

3.1.66.2 ist abgeschlossen. Subtask 3.1.66.3 erweitert ausschliesslich den
Test-Fake um die import-ID-spezifische Freigabefehler-Konfiguration und eine
separate Versuchsliste und implementiert genau den beschriebenen Widgettest.
