# GAEB: Widgettest-Strategie fuer einen generischen Uploadfehler

## Ziel

Subtask 3.1.72.2 definiert den kleinsten stabilen Widgettestvertrag fuer den
technischen, nicht als ApiException modellierten GAEB-Uploadfehler. Dieser
Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

Nach erfolgreicher Dateiauswahl oeffnet QuotesPage._importGAEB() den
nicht schliessbaren Fortschrittsdialog. Sein Catch-Pfad schliesst den Dialog
und ruft _quoteErrorMessage mit dem Fallback GAEB-Upload fehlgeschlagen auf.
Bei einem Fehler, der keine ApiException ist, liefert die Hilfsfunktion:

    GAEB-Upload fehlgeschlagen: <error.toString()>

Der vorhandene _FakeApiClient speichert den Uploadversuch vor seinem
dateinamenspezifischen Fehlerwurf. Die bereits als Object typisierte Map
quoteImportUploadErrors kann daher ohne Erweiterung einen StateError liefern.

## Minimales Testszenario

Der neue Test steht direkt hinter dem strukturierten Uploadfehlertest und
heisst:

    QuotesPage GAEB import falls back on technical upload failure

Er verwendet:

- _prepareLargeViewport(tester);
- Berechtigungen quotes.read und quotes.write;
- Projektfilter project-gaeb-upload-technical-error-1;
- lokalen Picker mit den Bytes [8, 9, 10], der Datei
  ausschreibung-upload-technisch.x83 und application/xml;
- die vorhandene quoteImportUploadErrors-Map mit
  StateError('Upload-Transport nicht verfuegbar') fuer genau diesen Dateinamen.

Die erwartete Meldung ist vollstaendig und deterministisch:

    GAEB-Upload fehlgeschlagen: Bad state: Upload-Transport nicht verfuegbar

Damit wird der generische Zweig eindeutig vom bereits getesteten
ApiException-Fehlerpfad abgegrenzt.

## Interaktion und Assertions

1. Seite mit Projektkontext aufbauen und den initialen Importlistenabruf
   abschliessen.
2. quoteImportListRequestCount == 1 feststellen.
3. GAEB-Import ausloesen und bis zum Endzustand pumpAndSettle() aufrufen.

Der Test belegt positiv:

- genau einen Uploadversuch mit Dateiname, Bytes, Projekt-ID,
  contact_id: null und application/xml;
- weiter genau einen Importlistenabruf;
- die vollstaendige generische Fehlermeldung;
- weiterhin sichtbare Aktion GAEB-Import.

Er belegt negativ:

- GAEB-Import wird hochgeladen... ist nicht mehr sichtbar;
- kein AlertDialog bleibt offen;
- der dateispezifische Erfolgshinweis fehlt;
- die strukturierte 422-Fehlermeldung fehlt.

Picker-Aufruf und Accept-Filter werden nicht wiederholt; diese sind bereits
durch die bestehenden Erfolgs- und Abbruchtests belegt.

## Stabilitaet und Verifikation

- Kein Completer, keine Zwischenframe-Assertion und keine Fake-Aenderung.
- Der Fehler ist lokal, dateinamenspezifisch und sofort abgeschlossen.
- Der vollstaendige StateError-Text belegt den generischen Fallbackzweig.

Fuer 3.1.72.3 werden ausgefuehrt:

    dart format test/sales_order_context_pages_test.dart
    flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import falls back on technical upload failure"
    flutter analyze lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
    git diff --check

Die zwei bekannten Harness-Warnungen zum ungenutzten Purchase-Orders-Import
und zu convertedQuote sind getrennt von neuen Regressionen zu bewerten.

## Nicht-Ziele

- kein Produktiv- oder Fake-Umbau;
- kein weiterer ApiException-Fall und kein Fortschritts-Zwischenzustand;
- keine Dateiinhalt-, Groessen-, Backend-, API-, DB-, Permission-, Mapping-,
  Kalkulations- oder KI-Aenderung.

## Naechster Leaf

Subtask 3.1.72.3 implementiert ausschliesslich den beschriebenen Widgettest
und fuehrt die gezielten Verifikationen aus.

## Ergebnis

3.1.72.2 ist abgeschlossen. Die Implementierung benoetigt genau einen neuen
Test und keine bestehende Codeaenderung.
