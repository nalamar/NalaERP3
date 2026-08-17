# GAEB: Widgettest-Strategie fuer einen strukturierten Uploadfehler

## Ziel

Subtask 3.1.70.2 definiert den kleinsten stabilen Fake- und Widgettestvertrag
fuer einen serverseitig abgewiesenen GAEB-Upload. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Bestehender Runtime-Vertrag

Nach erfolgreicher Dateiauswahl oeffnet `_importGAEB()` einen nicht
schliessbaren Fortschrittsdialog. Wirft `uploadGAEBQuoteImport(...)`, schliesst
der Catch-Pfad den Dialog und zeigt bei einer `ApiException` deren fachliche
Meldung. Der Importlisten-Reload und die Erfolgssnackbar liegen ausschliesslich
im Erfolgszweig.

## Minimaler Fake-Ausbau

`_FakeApiClient` erhaelt genau eine additive Konfiguration:

```dart
final Map<String, Object> quoteImportUploadErrors;
```

Der Konstruktor initialisiert sie mit `const {}`. Der Schluessel ist der
Dateiname. Im bestehenden `uploadGAEBQuoteImport(...)` gilt diese Reihenfolge:

1. normalisiertes Payload in `attemptedQuoteImportUploads` eintragen
2. Fehler fuer `filename` aus `quoteImportUploadErrors` lesen
3. konfigurierten Fehler werfen, falls vorhanden
4. nur andernfalls `quoteImportUploadResult` zurueckgeben

Damit bleibt der bereits bestehende Erfolgstest unveraendert und jeder
abgewiesene Upload trotzdem als Versuch nachweisbar.

## Minimales Testszenario

Der neue Test wird direkt neben dem erfolgreichen Uploadtest angelegt und
traegt den Namen:

```text
QuotesPage GAEB import closes progress and keeps list on upload failure
```

Er verwendet:

- `_prepareLargeViewport(tester)`
- Berechtigungen `quotes.read` und `quotes.write`
- Projektfilter `project-gaeb-upload-error-1`
- lokalen Picker mit
  `browser.PickedFile(Uint8List.fromList([5, 6, 7]),
  'ausschreibung-upload-fehler.x83', 'application/xml')`
- dateinamenspezifische `ApiException` mit Status `422`, Code
  `invalid_gaeb_file` und Meldung
  `GAEB-Datei konnte nicht verarbeitet werden`

Ein `quoteImportUploadResult` wird im Fehlerfall nicht benoetigt.

## Interaktionsfolge

1. Seite aufbauen und initialen Importlistenabruf abwarten.
2. Sicherstellen, dass `quoteImportListRequestCount == 1` gilt.
3. `GAEB-Import` anklicken.
4. `pumpAndSettle()` bis zum abgeschlossenen Fehlerpfad ausfuehren.

## Verbindliche Assertions

Der Test belegt positiv:

- genau ein Uploadversuch mit Dateiname, Bytes `[5, 6, 7]`, Projekt-ID,
  `contact_id: null` und `application/xml`
- weiterhin exakt ein Importlistenabruf; kein Reload im Fehlerfall
- sichtbare Meldung `GAEB-Datei konnte nicht verarbeitet werden`
- `GAEB-Import` bleibt als Aktion sichtbar

Der Test belegt negativ:

- `GAEB-Import wird hochgeladen...` ist nicht mehr sichtbar
- kein `AlertDialog` bleibt geoeffnet
- `GAEB-Datei ausschreibung-upload-fehler.x83 wurde hochgeladen` ist nicht
  sichtbar

Der Accept-Filter und der Picker-Aufruf muessen nicht erneut vollstaendig
assertiert werden; sie sind bereits im direkten Erfolgstest belegt. Der
Fehlertest fokussiert Zustandserhalt und Feedback nach der Mutation.

## Stabilitaetsregeln

- Fehler wird dateinamenspezifisch konfiguriert.
- Versuchserfassung geschieht vor dem Fehlerwurf.
- Kein manuell angehaltener Future und keine Zwischenframe-Assertion.
- Reload wird ueber den stabilen Zaehler ausgeschlossen.
- Erfolg und Fehler verwenden unterschiedliche Dateinamen.
- Keine Aenderung an `QuotesPage`, Picker-Testnaht oder produktivem API-Vertrag.

## Verifikation fuer 3.1.70.3

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import closes progress and keeps list on upload failure"
flutter analyze lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
git diff --check
```

Die zwei bekannten Harness-Warnungen sind getrennt von neuen Regressionen zu
bewerten.

## Nicht-Ziele

- kein Picker-Abbruch-Test
- kein angehaltener Fortschrittsdialog-Test
- keine Runtime-, Backend-, API-, DB- oder Permission-Aenderung
- keine Dateiinhalt- oder Groessenvalidierung
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.70.3 implementiert ausschliesslich die dateinamenspezifische
Fake-Fehlerkonfiguration und genau den beschriebenen Widgettest.

## Ergebnis

3.1.70.2 ist abgeschlossen, sobald dieser Vertrag dokumentiert ist. Der
Implementierungsleaf bleibt auf eine Fake-Konfiguration und einen Widgettest
begrenzt.
