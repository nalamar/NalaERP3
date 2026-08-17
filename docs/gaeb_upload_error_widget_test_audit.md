# GAEB: Audit des Uploadfehler-Widgettests

## Gegenstand

Subtask 3.1.70.4 auditiert die in 3.1.70.3 implementierte
Fake-Fehlerkonfiguration und den Widgettest fuer einen strukturiert
abgewiesenen GAEB-Upload. Runtime-, Backend-, API- und Datenbankcode bleiben
unveraendert.

## Ergebnis

Das Audit ist ohne fachlichen oder technischen Befund abgeschlossen. Die
Implementierung entspricht der Strategie aus
`docs/gaeb_upload_error_widget_test_strategy.md` und dem vorhandenen
Runtime-Vertrag in `QuotesPage`.

## Vertragsabgleich

- `_FakeApiClient` besitzt die additive, standardmaessig leere Map
  `quoteImportUploadErrors`.
- Der Fake zeichnet Dateiname, Bytes, Projekt, Kontakt und Inhaltstyp vor der
  Fehlerpruefung auf.
- Ein Fehler wird ausschliesslich ueber den Dateinamen ausgewaehlt; ohne
  passenden Eintrag bleibt der vorhandene Erfolgsvertrag erhalten.
- Der Test verwendet die festgelegte X83-Datei, Bytes `[5, 6, 7]`, das Projekt
  `project-gaeb-upload-error-1` und die strukturierte 422-`ApiException` mit
  Code `invalid_gaeb_file`.
- Der Test belegt exakt einen Uploadversuch und weiterhin genau einen
  Importlistenabruf. Damit ist ausgeschlossen, dass der Catch-Pfad einen
  Erfolgs-Reload ausloest.
- Die fachliche API-Meldung bleibt sichtbar, waehrend Fortschrittstext und
  Dialog geschlossen sind und kein Erfolgshinweis erscheint.
- Die GAEB-Importaktion bleibt nach dem Fehler verfuegbar.

## Runtime-Abgleich

`_importGAEB()` oeffnet vor dem Upload einen nicht schliessbaren
Fortschrittsdialog. Nur der Erfolgszweig laedt die Importliste neu und zeigt
die Upload-Erfolgsmeldung. Der Catch-Zweig schliesst den Dialog und reicht bei
einer `ApiException` deren `message` ueber `_quoteErrorMessage(...)` an die
Snackbar weiter. Die Testassertions bilden genau diese Trennung ab.

## Regressionsnachweis

Folgende gezielte Widgettests sind gruen:

```text
QuotesPage GAEB import closes progress and keeps list on upload failure
QuotesPage GAEB import uploads picked file for selected project
```

Die gezielte Analyse von `quotes_page.dart` und
`sales_order_context_pages_test.dart` meldet ausschliesslich zwei bereits
bekannte Harness-Warnungen:

- ungenutzter Import `purchase_orders_page.dart`
- nie gesetzter optionaler Fake-Parameter `convertedQuote`

Es wurde keine neue Analysewarnung erzeugt. `git diff --check` bleibt sauber.

## Scope-Pruefung

Der Leaf veraendert keinen Produktivcode. Backend, API, Datenbank, Permissions,
GAEB-Mapping, Kalkulation und KI-Verarbeitung sind nicht betroffen. Die
additive Fake-Konfiguration beeinflusst bestehende Tests ohne konfigurierten
Fehler nicht.

## Abschluss

Subtask 3.1.70.4 ist abgeschlossen. Der abgewiesene Uploadpfad ist zusammen
mit dem direkten Erfolgspfad stabil abgedeckt. Der naechste Leaf kann den
kleinsten noch offenen GAEB-Import-Risikobereich inventarisieren.
