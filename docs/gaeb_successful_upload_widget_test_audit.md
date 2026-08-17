# GAEB: Abschlussaudit der Picker-Testnaht und des Upload-Widgettests

## Ziel

Subtask 3.1.69.5 auditiert die Umsetzung aus 3.1.69.3 und 3.1.69.4 gegen die
dokumentierte Strategie. Dieser Leaf nimmt keine weitere Runtime- oder
Testaenderung vor.

## Auditumfang

Geprueft wurden ausschliesslich:

- lokale Picker-Testnaht und Produktionsfallback
- Fake-Uploadpayload und Importlisten-Zaehler
- erfolgreicher projektgebundener Upload-Widgettest
- UI-Abschlusszustand und Erfolgssignal
- gezielte Test- und Analyseausgabe

## Picker-Vertrag

Die Implementierung erfuellt den lokalen Abhaengigkeitsvertrag:

- `QuoteImportFilePicker` bildet den vorhandenen optionalen `accept`-Parameter
  und `browser.PickedFile?` ab
- `QuotesPage.quoteImportFilePicker` ist nullable und instanzlokal
- der konstante Konstruktor bleibt erhalten
- `_importGAEB()` verwendet den injizierten Callback nur, wenn er gesetzt ist
- Produktion faellt unveraendert auf `browser.pickFile` zurueck
- `web/browser.dart` und andere Dateipicker bleiben unberuehrt

Es existiert kein globaler Testzustand.

## Fake-Vertrag

`_FakeApiClient` erfuellt den vorgesehenen Orchestrierungsvertrag:

- `attemptedQuoteImportUploads` zeichnet Dateiname, Bytes als Werteliste,
  Projekt-ID, Kontakt-ID und Content-Type auf
- `quoteImportUploadResult` liefert eine deterministische Erfolgsantwort
- `quoteImportListRequestCount` zaehlt initialen Abruf und Erfolgs-Reload
- der echte `ApiClient`- und HTTP-Vertrag wurde nicht geaendert

## Widgettest-Vertrag

Der Test
`QuotesPage GAEB import uploads picked file for selected project` belegt:

- genau einen Aufruf des lokalen Pickers
- den exakten Accept-Filter `.x83,.x84,.d83,.p83,.gaeb,.xml`
- Dateiname `ausschreibung-upload.x83`
- Bytes `[1, 2, 3, 4]`
- Content-Type `application/xml`
- getrimmte Projekt-ID `project-gaeb-upload-1`
- nicht gesetzte Kontakt-ID
- Anstieg der Importlistenabrufe von eins auf zwei
- geschlossenen Fortschrittsdialog und keinen verbleibenden `AlertDialog`
- Erfolgshinweis mit dem ausgewaehlten Dateinamen

Damit ist die positive Client-Orchestrierung vom Projektkontext ueber die
Dateiauswahl bis zum Reload belegt.

## Verifikationsnachweis

Ausgefuehrt wurden:

```text
dart format lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter analyze lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import uploads picked file for selected project"
git diff --check
```

Ergebnis:

- Formatierung sauber
- gezielter Widgettest bestanden
- gezielte Analyse nur mit den zwei bekannten Harness-Warnungen zum
  unbenutzten `purchase_orders_page.dart`-Import und zum nie gesetzten
  optionalen Parameter `convertedQuote`
- `git diff --check` ohne Whitespacefehler; nur Zeilenenden-Warnungen

## Scope-Abgleich

Nicht umgesetzt wurden:

- globaler Browserhook
- Uploadfehler-Test
- Server-, HTTP-, API- oder Datenbankaenderung
- Berechtigungs- oder Kontakt-Auswahlfluss
- Dateiinhalt- oder Groessenvalidierung
- Mapping-, Kalkulations- oder KI-Logik

## Abschlussentscheidung

Testnaht, Fake-Vertrag und Widgettest entsprechen der Strategie. Task 3.1.69
ist fachlich abgeschlossen. Der positive GAEB-Upload ist nun zusammen mit der
Projektpflicht und der nachgelagerten Review-/Apply-/Navigationskette
deterministisch im Client-Harness belegt.

Als naechstes ist read-only zu inventarisieren, ob ein Uploadfehler-Test oder
ein fachlich tieferer Mapping-/Automatisierungsblock den kleinsten wertvollen
Folgeausbau bildet.

## Ergebnis

3.1.69.5 ist abgeschlossen. Der produktive Fallback bleibt unveraendert, und
der erfolgreiche projektgebundene Upload ist durch einen gruenen Widgettest
abgesichert.
