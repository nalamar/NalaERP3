# GAEB: Widgettest-Strategie fuer abgewiesenen Einzelpositionsreview

## Ziel

Subtask 3.1.67.2 definiert den kleinsten stabilen Testvertrag fuer einen
serverseitig abgewiesenen Aufruf von `updateQuoteImportItemReview(...)`.
Dieser Leaf aendert weder Test- noch Runtimecode.

## Bestehender Vertrag

Der Erfolgsfall in `sales_order_context_pages_test.dart` belegt bereits:

- Oeffnen eines Importlaufs und einer konkreten Position
- Auswahl des Reviewstatus `accepted`
- Trimmen der Review-Notiz vor dem API-Aufruf
- Aktualisierung des Positionsdetails nach erfolgreicher Mutation
- Erfolgssnackbar `Review-Entscheidung wurde gespeichert`

Im produktiven `QuotesPage` wird der Entscheidungsdialog vor dem API-Aufruf
geschlossen. Nur das von der API gelieferte Ergebnis ersetzt anschliessend den
lokalen `detail`-Zustand. Eine `ApiException` wird als Snackbar dargestellt,
ohne den Positionsdetaildialog zu schliessen oder `detail` zu veraendern.

## Minimaler Fake-Ausbau

`_FakeApiClient` erhaelt genau zwei additive Testhilfen:

```dart
final Map<String, Object> quoteImportItemReviewErrors;
final List<Map<String, dynamic>> attemptedQuoteImportItemReviews = [];
```

Die Fehler-Map wird ueber einen optionalen Konstruktorparameter mit
`const {}` initialisiert. Der Schluessel bindet Import und Position eindeutig,
beispielsweise `import-review-error-1/import-review-error-item-1`.

`updateQuoteImportItemReview(...)` baut einmal das normalisierte Payload:

```dart
{
  'import_id': importId,
  'item_id': itemId,
  'review_status': reviewStatus,
  'review_note': reviewNote,
}
```

Danach gilt diese feste Reihenfolge:

1. Payload in `attemptedQuoteImportItemReviews` eintragen.
2. Import-/Item-Schluessel in `quoteImportItemReviewErrors` pruefen.
3. Konfigurierten Fehler werfen, falls vorhanden.
4. Nur andernfalls Payload in `updatedQuoteImportItemReviews` eintragen und
   den aktualisierten Detailzustand zurueckgeben.

Damit repraesentiert die bestehende Erfolgsliste weiterhin ausschliesslich
persistierte Mutationen.

## Minimales Testszenario

Der neue Test wird unmittelbar neben dem vorhandenen Erfolgsfall angelegt und
verwendet:

- Berechtigungen `quotes.read` und `quotes.write`
- genau einen Import `import-review-error-1` mit Status `parsed`
- genau eine Listenposition `import-review-error-item-1`
- ein Positionsdetail mit `review_status: pending` und `review_note: ''`
- die Benutzerwahl `accepted`
- die Eingabe `  Fachlich geprueft  `
- eine schluesselspezifische `ApiException` mit Status `409`, einem stabilen
  Fachcode und der Meldung `Review-Konflikt fuer diese Importposition`

Der Importstatus `parsed` haelt das Szenario konsistent mit dem fachlichen
Reviewablauf. Weitere Positionen, Quotes oder Backenddaten sind nicht noetig.

## Interaktionsfolge

1. `QuotesPage` mit dem Projektfilter aufbauen.
2. Importdetails ueber `Details` oeffnen.
3. `Position 02.02` oeffnen.
4. `Review setzen` auswaehlen.
5. Im Dialog `accepted` waehlen und die Notiz eingeben.
6. `Speichern` ausloesen und `pumpAndSettle()` abwarten.

## Verbindliche Assertions

Der Test prueft positiv:

- `attemptedQuoteImportItemReviews` enthaelt exakt das erwartete Payload mit
  den richtigen Import-/Item-IDs, `accepted` und getrimmter Notiz
- `updatedQuoteImportItemReviews` ist leer
- genau zwei aeussere `AlertDialog`s fuer Import- und Positionsdetail sind
  weiterhin vorhanden
- im erhaltenen Positionsdetail steht `Review-Status: pending`
- die API-Meldung `Review-Konflikt fuer diese Importposition` ist sichtbar
- `Review setzen` bleibt als Aktion sichtbar

Der Test prueft negativ:

- `Review-Entscheidung` ist nicht mehr sichtbar; der innere
  Entscheidungsdialog wurde geschlossen
- `Review-Status: accepted` ist nicht sichtbar
- `Review-Notiz: Fachlich geprueft` ist nicht sichtbar
- `Review-Entscheidung wurde gespeichert` ist nicht sichtbar

Die leere Notiz wird ueber das Ausbleiben der neuen Notiz abgesichert. Eine
Assertion auf interne Widgettypen, Dialogreihenfolge oder Snackbar-Laufzeit ist
nicht erforderlich.

## Stabilitaetsregeln

- Import- und Item-ID muessen gemeinsam den Fehler bestimmen.
- Versuch und Erfolg duerfen nicht dieselbe Liste verwenden.
- Assertions verwenden sichtbare fachliche Texte und das Fake-Payload.
- Keine Volltextassertion des gesamten Dialogs.
- Keine Abhaengigkeit von Zeitstempeln, Animationstakten oder Finder-Indizes,
  soweit fachlich benannte Widgets verfuegbar sind.
- Der bestehende Erfolgsfall muss unveraendert weiterlaufen.

## Verifikation fuer den Implementierungsleaf

Nach der Implementierung genuegen:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import item review keeps pending detail on failure"
flutter analyze
```

Bekannte, bereits vorhandene Analysewarnungen sind getrennt von neuen
Regressionen zu bewerten.

## Nicht-Ziele

- keine Aenderung an `QuotesPage`
- keine Aenderung an `ApiClient`
- keine Backend-, API-, DB- oder Permission-Aenderung
- kein Retry- oder Dialog-Redesign
- kein Importlauf-Freigabe-, Apply- oder Navigationstest
- kein weiterer Erfolgsstatus
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.67.3 erweitert ausschliesslich `_FakeApiClient` um die
import-/item-spezifische Fehlerkonfiguration und getrennte
Versuch-/Erfolgsaufzeichnung und implementiert den einen beschriebenen
Widgettest.

## Ergebnis

3.1.67.2 ist abgeschlossen, sobald dieser Testvertrag dokumentiert ist. Der
Implementierungsumfang fuer 3.1.67.3 ist auf eine Fake-Erweiterung und einen
isolierten Widgettest begrenzt.
