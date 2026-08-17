# GAEB: Audit des Dateipicker-Abbruch-Widgettests

## Gegenstand

Subtask 3.1.71.4 auditiert den in 3.1.71.3 implementierten Widgettest fuer
einen abgebrochenen GAEB-Dateipicker. Produktivcode und Fake-Vertrag bleiben
unveraendert.

## Ergebnis

Das Audit ist ohne fachlichen oder technischen Befund abgeschlossen. Der Test
entspricht der Strategie aus
`docs/gaeb_file_picker_cancel_widget_test_strategy.md` und dem fruehen
`picked == null`-Return in `QuotesPage._importGAEB()`.

## Vertragsabgleich

- Der Test setzt `quotes.read` und `quotes.write` sowie das feste Projekt
  `project-gaeb-picker-cancel-1`.
- Der lokale Picker erfasst Aufrufzahl und Accept-Argument und liefert
  deterministisch `null`.
- Genau ein Pickeraufruf und der Filter `.x83,.x84,.d83,.p83,.gaeb,.xml`
  belegen, dass der Test nicht in der vorgelagerten Projektpruefung endet.
- `attemptedQuoteImportUploads` bleibt leer und der Listenabrufzaehler bleibt
  nach dem initialen Abruf bei eins.
- Fortschrittstext, `AlertDialog` und `SnackBar` bleiben aus.
- Die GAEB-Importaktion bleibt sichtbar.
- Es wurde weder der Fake noch die Runtime fuer diesen Test erweitert.

## Runtime-Abgleich

Der Picker wird erst nach erfolgreicher Projektpruefung aufgerufen. Bei
`picked == null` kehrt `_importGAEB()` vor `showDialog(...)`,
`uploadGAEBQuoteImport(...)`, `_loadQuoteImports()` und jeglicher Snackbar
zurueck. Die positiven und negativen Assertions bilden diese Grenze
vollstaendig ab.

## Regressionsnachweis

Folgende gezielte Widgettests sind gruen:

```text
QuotesPage GAEB import returns without upload when file picker is cancelled
QuotesPage GAEB import requires project before file picker
QuotesPage GAEB import uploads picked file for selected project
```

Ein erster paralleler Lauf der Nachbartests kollidierte im gemeinsam genutzten
Flutter-`build/native_assets`-Verzeichnis. Beide Tests wurden danach seriell
ohne weitere Massnahme erfolgreich wiederholt; es liegt kein Test- oder
Codefehler vor.

Die gezielte Analyse meldet ausschliesslich die zwei bekannten
Harness-Warnungen zum ungenutzten Purchase-Orders-Import und zum nie gesetzten
optionalen Fake-Parameter `convertedQuote`. `git diff --check` bleibt sauber.

## Scope-Pruefung

Der Leaf aendert ausschliesslich dieses Audit-Artefakt und `codex.md`.
Runtime, Fake, Backend, API, Datenbank, Permissions, GAEB-Mapping, Kalkulation
und KI-Verarbeitung sind nicht betroffen.

## Abschluss

Subtask 3.1.71.4 ist abgeschlossen. Projektpflicht, Picker-Abbruch,
erfolgreicher Upload und strukturierte Uploadablehnung sind als getrennte
Clientpfade stabil belegt. Der naechste Leaf kann den kleinsten verbleibenden
GAEB-Risikobereich inventarisieren.
