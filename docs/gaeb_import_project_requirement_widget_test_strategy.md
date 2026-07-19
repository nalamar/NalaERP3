# GAEB: Widgettest-Strategie fuer die Projektpflicht am Importeinstieg

## Ziel

Subtask 3.1.68.2 definiert den kleinsten stabilen Testvertrag fuer den
vorhandenen Projekt-ID-Guard in `_importGAEB()`. Dieser Leaf aendert weder
Test- noch Runtimecode.

## Bestehender Runtime-Vertrag

`QuotesPage` liest beim Klick auf `GAEB-Import` zuerst den getrimmten Inhalt
des Projektfilters. Ist er leer, zeigt die Seite den Hinweis

```text
Für den GAEB-Import bitte zuerst eine Projekt-ID im Filter setzen.
```

und kehrt vor `browser.pickFile(...)`, Fortschrittsdialog und
`uploadGAEBQuoteImport(...)` zurueck.

## Minimaler Testaufbau

Der Test wird in `client/test/sales_order_context_pages_test.dart` unmittelbar
vor den bestehenden GAEB-Vorschautests angelegt und verwendet:

- `_prepareLargeViewport(tester)`
- `_FakeApiClient` mit `quotes.read` und `quotes.write`
- die bereits standardmaessig leere `quoteImportList`
- `MaterialApp` mit `QuotesPage(api: api)`
- keinen `initialProjectId`
- keine `initialFilters`
- keine Quote-, Import- oder Kontaktdaten

Der Fake benoetigt keine neue Eigenschaft und keinen neuen Override.

## Interaktionsfolge

1. Seite aufbauen und `pumpAndSettle()` abwarten.
2. Den `FilledButton` mit dem Text `GAEB-Import` finden.
3. Sicherstellen, dass die Aktion wegen `quotes.write` sichtbar ist.
4. Aktion antippen.
5. Erneut `pumpAndSettle()` abwarten.

## Verbindliche Assertions

Der Test prueft positiv:

- `GAEB-Import` ist genau einmal als schreibberechtigte Aktion sichtbar
- der vollstaendige Projekt-ID-Hinweis ist sichtbar

Der Test prueft negativ:

- `GAEB-Import wird hochgeladen...` ist nicht sichtbar
- es ist kein `AlertDialog` geoeffnet

Ein gesonderter Picker- oder Upload-Aufrufzaehler ist nicht erforderlich. Im
Widgettest waere der direkte Browserpfad ohne Testnaht nicht nutzbar; der
erfolgreiche Abschluss des Klicks zusammen mit Hinweis und fehlendem
Fortschrittsdialog belegt den fruehen Return an der fachlichen Guard-Grenze.

## Stabilitaetsregeln

- Finder verwenden die vorhandenen fachlichen Texte.
- Der Button wird ueber `find.widgetWithText(FilledButton, 'GAEB-Import')`
  gebunden.
- Der Hinweis wird vollstaendig verglichen, da er der fachliche Vertrag ist.
- Keine Assertion auf Snackbar-Widgettyp, Animationsdauer oder interne
  Browserimplementierung.
- Kein Projektwert darf nachtraeglich in das Textfeld geschrieben werden.
- Keine Erweiterung des Fakes nur zur Beobachtung eines unerreichbaren Uploads.

## Gezielter Testname

```text
QuotesPage GAEB import requires project before file picker
```

## Verifikation fuer den Implementierungsleaf

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import requires project before file picker"
flutter analyze test/sales_order_context_pages_test.dart
git diff --check
```

Die zwei bereits bekannten Harness-Warnungen zum unbenutzten Import und zum
optionalen Parameter `convertedQuote` sind keine Regression dieses Leaves.

## Nicht-Ziele

- keine Picker-Abstraktion oder Browser-Fake
- kein erfolgreicher oder fehlgeschlagener Datei-Upload
- keine Payload-, Dateityp- oder Dateigroessenpruefung
- keine Aenderung an `QuotesPage`, `ApiClient` oder `_FakeApiClient`
- keine Backend-, API-, DB- oder Permission-Aenderung
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.68.3 implementiert ausschliesslich den beschriebenen Widgettest im
vorhandenen Harness und fuehrt die gezielte Verifikation aus.

## Ergebnis

3.1.68.2 ist abgeschlossen, sobald dieser Testvertrag dokumentiert ist. Die
Implementierung in 3.1.68.3 benoetigt genau einen neuen Widgettest und keine
Produktiv- oder Fake-Aenderung.
