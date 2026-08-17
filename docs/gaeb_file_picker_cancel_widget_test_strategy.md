# GAEB: Widgettest-Strategie fuer den Dateipicker-Abbruch

## Ziel

Subtask 3.1.71.2 definiert den kleinsten stabilen Widgettestvertrag fuer den
Abbruch der GAEB-Dateiauswahl bei vorhandener Projekt-ID. Dieser Leaf aendert
weder Runtime- noch Testcode.

## Bestehender Runtime-Vertrag

`QuotesPage._importGAEB()` prueft zuerst die Projekt-ID und ruft danach die
injizierte `QuoteImportFilePicker`-Funktion mit dem GAEB-Accept-Filter auf.
Liefert der Picker `null`, endet die Methode unmittelbar. Fortschrittsdialog,
Uploadmutation, Listen-Reload und Feedback liegen hinter diesem fruehen
Return.

## Fake- und Pickervertrag

`_FakeApiClient` benoetigt keine Erweiterung. Der Test verwendet lediglich die
bereits vorhandenen Nachweise:

- `attemptedQuoteImportUploads`
- `quoteImportListRequestCount`

Der lokale Picker erfasst Aufrufzahl und Accept-Argument und gibt
deterministisch `null` zurueck:

```dart
var pickerCallCount = 0;
String? pickerAccept;
Future<browser.PickedFile?> cancelQuoteImportFilePicker({
  String? accept,
}) async {
  pickerCallCount += 1;
  pickerAccept = accept;
  return null;
}
```

## Minimales Testszenario

Der Test wird direkt bei den bestehenden Uploadtests angelegt und traegt den
Namen:

```text
QuotesPage GAEB import returns without upload when file picker is cancelled
```

Er verwendet:

- `_prepareLargeViewport(tester)`
- Berechtigungen `quotes.read` und `quotes.write`
- Projektfilter `project-gaeb-picker-cancel-1`
- den lokalen, mit `null` abbrechenden Picker
- eine ansonsten unveraenderte `_FakeApiClient`-Instanz

## Interaktionsfolge

1. `QuotesPage` aufbauen und den initialen Importlistenabruf abwarten.
2. Sicherstellen, dass `quoteImportListRequestCount == 1` gilt.
3. `GAEB-Import` anklicken.
4. `pumpAndSettle()` bis nach dem fruehen Return ausfuehren.

## Verbindliche Assertions

Der Test belegt positiv:

- `pickerCallCount == 1`
- `pickerAccept == '.x83,.x84,.d83,.p83,.gaeb,.xml'`
- die Aktion `GAEB-Import` bleibt sichtbar

Der Test belegt negativ:

- `attemptedQuoteImportUploads` bleibt leer
- `quoteImportListRequestCount` bleibt bei eins
- `GAEB-Import wird hochgeladen...` ist nicht sichtbar
- kein `AlertDialog` ist sichtbar
- keine `SnackBar` ist sichtbar

Eine dateinamenbezogene Erfolgs- oder Fehlermeldung kann ohne ausgewaehlte
Datei nicht stabil adressiert werden. `find.byType(SnackBar)` bildet den
vollstaendigen Feedback-Negativvertrag daher praeziser ab.

## Stabilitaetsregeln

- Der Picker gibt synchron ueber einen abgeschlossenen Future `null` zurueck.
- Kein `Completer` und keine Zwischenframe-Assertion.
- Keine neue Fake-Konfiguration.
- Keine Wiederholung von Uploadpayload-Assertions aus Erfolg und Fehler.
- Aufrufzahl und Accept-Filter belegen, dass der Test den Picker-Abbruch und
  nicht die vorgelagerte Projektvalidierung erreicht.

## Verifikation fuer 3.1.71.3

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import returns without upload when file picker is cancelled"
flutter analyze lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
git diff --check
```

Die zwei bekannten Harness-Warnungen zum ungenutzten Purchase-Orders-Import
und zu `convertedQuote` sind getrennt von neuen Regressionen zu bewerten.

## Nicht-Ziele

- kein Produktivcode-Umbau
- kein Fortschritts-Zwischenzustand
- kein technischer Upload-Fallbackfehler
- keine Dateiinhalt- oder Groessenvalidierung
- keine Backend-, API-, DB- oder Permission-Aenderung
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.71.3 implementiert ausschliesslich den beschriebenen
Picker-Abbruch-Widgettest.

## Ergebnis

3.1.71.2 ist abgeschlossen, sobald dieser Testvertrag dokumentiert ist. Die
Implementierung benoetigt genau einen neuen Widgettest und keine weitere
Produktiv- oder Fake-Logik.
