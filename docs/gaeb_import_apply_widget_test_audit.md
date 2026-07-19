# GAEB: Abschlussaudit des Draft-Quote-Erzeugungs-Widgettests

## Ziel

Subtask 3.1.62.4 auditiert den in 3.1.62.3 ergaenzten Widgettest fuer die
vorhandene Draft-Quote-Erzeugung aus einem freigegebenen GAEB-Importlauf.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Audit-Ergebnis

Die Umsetzung erfuellt das in
`docs/gaeb_import_apply_widget_test_strategy.md` definierte Minimalziel:

- konfigurierbare Apply-Ergebnisse im bestehenden Fake
- eine Aufzeichnungsliste fuer angewendete Import-IDs
- ein Override fuer `applyQuoteImport(importId)`
- priorisierte, zustandsabhaengige Antworten von `getQuoteImport(id)`
- genau ein separater Draft-Quote-Erzeugungs-Widgettest
- Ausgangsstatus `reviewed` und Zielstatus `applied`
- korrekte Import-ID und Apply-Antwortstruktur
- sichtbares Ergebnis mit `created_quote_id`
- Pruefung der Snackbar mit Angebotsnummer
- keine Navigation zur erzeugten Quote

Runtime, `ApiClient`, Backend, Datenbank und Berechtigungsvertrag wurden nicht
veraendert.

## Apply- und Refreshvertrag

`quoteImportApplyResults` bindet die Apply-Antwort an `import-apply-1`.
`applyQuoteImport` zeichnet diese ID in `appliedQuoteImportIds` auf und liefert
unveraendert die konfigurierte Antwort mit `import` und `quote`.

Nach der Mutation priorisiert `getQuoteImport` fuer angewendete IDs das
`import`-Objekt der Apply-Antwort. Erst danach wird ein vorhandener
Reviewzustand beruecksichtigt. Der reale Detailrefresh kann den Status damit
nicht von `applied` auf `reviewed` zuruecksetzen.

Die konfigurierten Ausgangs- und Antwortmaps werden nicht mutiert.
`listQuoteImportItems` bleibt unveraendert. Die erfolgreiche Ausfuehrung von
`_load()` wird durch das vollstaendige Settle belegt; der Test behauptet keine
Position oder Auswahl in der Angebotsliste.

## Sichtbarer Erzeugungsuebergang

Vor Apply belegt der Test:

- `Status: reviewed`
- `Draft-Quote erzeugen` ist sichtbar
- `Quote öffnen` ist nicht sichtbar

Nach Apply, Fortschrittsroute und Refreshfolge belegt er:

- exakt `['import-apply-1']` als aufgezeichnete IDs
- `Status: applied`
- `Erzeugte Quote: quote-gaeb-1`
- den Hinweis auf die jetzt oeffenbare Quote
- `Draft-Quote erzeugen` ist nicht mehr sichtbar
- `Quote öffnen` ist sichtbar
- `Draft-Quote ANG-GAEB-0001 wurde aus dem Importlauf erzeugt`

`Quote öffnen` wird nicht angetippt. Damit endet der Test exakt an der Grenze
zur nachfolgenden Navigation.

## Fortschrittsdialog und Finder

Der synchrone Fake kann die kurzlebige Fortschrittsroute zwischen Testframes
oeffnen und wieder schliessen. Der Test verwendet stabil `pumpAndSettle()` und
belegt den Abschluss ueber Status, Erzeugungsergebnis, Aktionswechsel und
Snackbar. Er prueft weder den kurzlebigen Fortschrittstext noch eine starre
Dialoganzahl.

Die Aktionen werden ueber Widgettyp und eindeutige Beschriftung gefunden. Nach
dem Schliessen des Importdialogs verbleibt kein `AlertDialog`.

## Scope-Grenze

Der Test verwendet nur `quotes.read` und `quotes.write`, genau einen
`reviewed` Import, eine leere Positionsliste und einen minimalen Quote-Payload.

Nicht Bestandteil dieses Tasks sind:

- Navigation oder Editorfokus der erzeugten Quote
- Apply-Fehler- oder Berechtigungsnegativpfad
- Angebotspositions-, Preis-, Material- oder Kalkulationspruefung
- Backend-, API-, Datenbank- oder Permission-Aenderungen
- KI-Automatisierung

## Verifikation

Im Verzeichnis `client/` erneut ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import apply exposes created draft quote"
```

Ergebnis: Der gezielte Widgettest ist erfolgreich.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten allgemeinen
Harness-Hinweise zum ungenutzten `purchase_orders_page.dart`-Import und zum nie
gesetzten optionalen Fakeparameter `convertedQuote` bleiben unveraendert.

## Abschlussentscheidung

Task 3.1.62 ist fachlich und technisch abgeschlossen. Der zentrale
GAEB-Prozessuebergang vom freigegebenen Importlauf zur sichtbaren Draft-Quote
ist mit einem engen, deterministischen Widgettest abgesichert. Der naechste
Leaf soll den kleinsten fachlich wertvollen Folgeausbau neu inventarisieren.
