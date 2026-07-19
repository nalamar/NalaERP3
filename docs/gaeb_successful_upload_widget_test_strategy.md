# GAEB: Strategie fuer Picker-Testnaht und erfolgreichen Upload-Widgettest

## Ziel

Subtask 3.1.69.2 definiert den minimalen Vertrag fuer die lokale
Dateipicker-Testnaht, den Fake-Upload und den spaeteren Erfolgswidgettest.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Lokaler Picker-Vertrag

In `quotes_page.dart` wird ein oeffentlicher Funktionsalias eingefuehrt:

```dart
typedef QuoteImportFilePicker = Future<browser.PickedFile?> Function({
  String? accept,
});
```

`QuotesPage` erhaelt genau ein neues optionales Konstruktorfeld:

```dart
final QuoteImportFilePicker? quoteImportFilePicker;
```

Der bestehende konstante Konstruktor bleibt erhalten. `_importGAEB()` waehlt
lokal:

```dart
final pickFile = widget.quoteImportFilePicker ?? browser.pickFile;
final picked = await pickFile(
  accept: '.x83,.x84,.d83,.p83,.gaeb,.xml',
);
```

Damit bleibt das Produktionsverhalten unveraendert. Es gibt keinen globalen
Hook und keine Aenderung in `web/browser.dart`.

## Minimaler Fake-Uploadvertrag

`_FakeApiClient` wird in 3.1.69.3 nur additiv erweitert:

- Konstruktorwert `quoteImportUploadResult` mit leerem Map-Default
- `quoteImportListRequestCount`, inkrementiert bei jedem
  `listQuoteImports(...)`
- `attemptedQuoteImportUploads` als Liste normalisierter Payloads
- Override von `uploadGAEBQuoteImport(...)`

Das aufgezeichnete Payload besitzt genau:

```dart
{
  'filename': filename,
  'bytes': bytes.toList(),
  'project_id': projectId,
  'contact_id': contactId,
  'content_type': contentType,
}
```

Der Override gibt `quoteImportUploadResult` unveraendert zurueck. Fehlerkarten
oder Erfolgszustandsmutation sind in diesem Task nicht erforderlich.

## Leaf-Grenze 3.1.69.3

3.1.69.3 implementiert ausschliesslich:

- Typalias, optionales Feld und Produktionsfallback in `QuotesPage`
- Fake-Uploadresultat, Versuchsliste und Importlisten-Zaehler
- benoetigte `dart:typed_data`- und `browser.dart`-Imports im Test-Harness
- Formatierung und statische Verifikation

Noch kein neuer Widgettest wird in diesem Leaf angelegt.

## Erfolgswidgettest fuer 3.1.69.4

Der Testname lautet:

```text
QuotesPage GAEB import uploads picked file for selected project
```

Der Test verwendet:

- `_prepareLargeViewport(tester)`
- `quotes.read` und `quotes.write`
- `initialFilters` mit `projectId: '  project-gaeb-upload-1  '`, sofern der
  bestehende Filterkontext den Wert normalisiert; andernfalls wird der
  Projektwert nach dem Seitenaufbau mit Leerzeichen in das sichtbare Feld
  geschrieben
- einen lokalen Picker-Callback, der den `accept`-Wert aufzeichnet
- `browser.PickedFile(Uint8List.fromList([1, 2, 3, 4]),
  'ausschreibung-upload.x83', 'application/xml')`
- ein kleines `quoteImportUploadResult` mit ID, Dateiname und Status `uploaded`

Vor dem Klick muss `quoteImportListRequestCount == 1` gelten. Nach erfolgreichem
Upload muss der Zaehler `2` sein und damit den Reload belegen.

## Verbindliche Assertions fuer 3.1.69.4

Der Test prueft:

- Picker wurde genau einmal aufgerufen
- Accept-Filter ist exakt `.x83,.x84,.d83,.p83,.gaeb,.xml`
- `attemptedQuoteImportUploads` enthaelt genau Dateiname, Bytes,
  `project-gaeb-upload-1`, `contact_id: null` und `application/xml`
- Importliste wurde initial und nach Upload geladen
- `GAEB-Import wird hochgeladen...` ist nach Abschluss nicht sichtbar
- kein `AlertDialog` bleibt geoeffnet
- `GAEB-Datei ausschreibung-upload.x83 wurde hochgeladen` ist sichtbar

Der Fortschrittsdialog muss nicht waehrend eines manuell angehaltenen Futures
beobachtet werden. Sein Abschluss und das Erfolgssignal reichen fuer diesen
ersten positiven Orchestrierungstest; eine kontrollierte Zwischenphase waere
zusaetzlicher asynchroner Harness-Umfang ohne neuen Fachnachweis.

## Stabilitaetsregeln

- Callback bleibt instanzlokal und nullable.
- Produktionsfallback bleibt `browser.pickFile`.
- Fake zeichnet Bytes als Werteliste statt als Objektidentitaet auf.
- Importlisten-Aufrufe werden nur gezaehlt, nicht zeitlich ausgewertet.
- Keine Assertion auf interne Dialogkeys oder Animationsframes.
- Der bereits vorhandene Projektpflicht-Test muss ohne injizierten Picker
  unveraendert weiterlaufen.

## Verifikation

Fuer 3.1.69.3:

```text
dart format lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter analyze lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
git diff --check
```

Fuer 3.1.69.4 zusaetzlich:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import uploads picked file for selected project"
```

Bestehende Harness-Warnungen werden von neuen Regressionen getrennt bewertet.

## Nicht-Ziele

- keine globale Browser-Testkonfiguration
- kein Uploadfehler-Test
- keine Aenderung am Server- oder HTTP-Vertrag
- kein Kontakt-Auswahlfluss
- keine Dateiinhalt- oder Groessenvalidierung
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.69.3 implementiert ausschliesslich die lokale Picker-Testnaht und
den beschriebenen Fake-Uploadvertrag, noch ohne neuen Widgettest.

## Ergebnis

3.1.69.2 ist abgeschlossen, sobald dieser Vertrag dokumentiert ist. Die
Testnaht bleibt lokal, der Produktionsfallback unveraendert und der spaetere
Widgettest auf einen erfolgreichen projektgebundenen Upload begrenzt.
