# GAEB: Abschlussaudit des Importlauf-Freigabe-Widgettests

## Ziel

Subtask 3.1.61.4 auditiert den in 3.1.61.3 ergaenzten Widgettest fuer die
vorhandene Freigabe eines vollstaendig geprueften GAEB-Importlaufs. Dieser
Leaf aendert weder Runtime- noch Testcode.

## Audit-Ergebnis

Die Umsetzung erfuellt das in
`docs/gaeb_import_review_widget_test_strategy.md` definierte Minimalziel:

- eine Importfreigabe-Aufzeichnungsliste im bestehenden Fake
- ein Override fuer `markQuoteImportReviewed(importId)`
- zustandsabhaengige Antworten von `getQuoteImport(id)`
- genau ein separater Importlauf-Freigabe-Widgettest
- Ausgangsstatus `parsed` und Zielstatus `reviewed`
- korrekte Weitergabe der Import-ID
- Wechsel der sichtbaren Prozessaktion
- Pruefung der Erfolgssnackbar
- keine Ausfuehrung der Apply-Mutation

Runtime, `ApiClient`, Backend, Datenbank und Berechtigungsvertrag wurden nicht
veraendert.

## Zustands- und Refreshvertrag

`reviewedQuoteImportIds` zeichnet den einzigen Freigabeaufruf fuer
`import-review-run-1` auf. `markQuoteImportReviewed` gibt das konfigurierte
Ausgangsdetail als neue Map mit `status: reviewed` zurueck.

Jeder folgende `getQuoteImport`-Aufruf erkennt die aufgezeichnete ID und
liefert ebenfalls eine neue Map mit `status: reviewed`. Damit bleibt der
erfolgreiche Status auch nach `refreshImportState()` erhalten. Die
konfigurierte Ausgangsmap wird nicht mutiert.

`listQuoteImportItems` bleibt unveraendert und liefert fuer den Test eine leere
Liste. Der anschliessende Vorschau-Refresh ist kein behaupteter
Statusanzeigevertrag; die Erfolgskriterien liegen im weiterhin offenen
Importdetaildialog.

## Sichtbarer Prozessuebergang

Vor der Mutation belegt der Test:

- `Status: parsed`
- `Zur Übernahme freigeben` ist sichtbar
- `Draft-Quote erzeugen` ist nicht sichtbar

Nach Mutation, Fortschrittsroute und Refreshfolge belegt er:

- exakt `['import-review-run-1']` als aufgezeichnete IDs
- `Status: reviewed`
- `Zur Übernahme freigeben` ist nicht mehr sichtbar
- `Draft-Quote erzeugen` ist sichtbar
- `Importlauf wurde freigegeben` ist sichtbar

Die neue Apply-Aktion wird nicht angetippt. Damit endet der Test exakt an der
Grenze zum naechsten GAEB-Prozessschritt.

## Fortschrittsdialog und Finder

Der synchrone Fake kann die kurzlebige Fortschrittsroute zwischen Testframes
oeffnen und schliessen. Der Test verwendet deshalb stabil `pumpAndSettle()` und
belegt den Abschluss ueber Status, Aktionswechsel und Snackbar. Er macht keine
zeitkritische Assertion auf den Fortschrittstext und zaehlt keine
`AlertDialog`-Zwischenzustaende.

Alle fachlichen Aktionen werden ueber Widgettyp und eindeutige Beschriftung
gefunden. Nach dem Schliessen des Importdialogs verbleibt kein `AlertDialog`.

## Scope-Grenze

Der Test verwendet nur `quotes.read` und `quotes.write` sowie einen einzelnen
Import mit einem akzeptierten und keinem offenen Item.

Nicht Bestandteil dieses Tasks sind:

- Draft-Quote-Erzeugung oder Apply-Payload
- Quote-Navigation
- Fehler- oder Berechtigungsnegativpfad
- weitere Reviewvarianten
- Backend-, API-, Datenbank- oder Permission-Aenderungen
- Sammelreview oder KI-Automatisierung

## Verifikation

Im Verzeichnis `client/` erneut ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import review enables draft quote action"
```

Ergebnis: Der gezielte Widgettest ist erfolgreich.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten allgemeinen
Harness-Hinweise zum ungenutzten `purchase_orders_page.dart`-Import und zum nie
gesetzten optionalen Fakeparameter `convertedQuote` bleiben unveraendert.

## Abschlussentscheidung

Task 3.1.61 ist fachlich und technisch abgeschlossen. Der vorhandene
Prozessuebergang vom geprueften Importlauf zur Apply-Bereitschaft ist mit einem
engen, deterministischen Widgettest abgesichert. Der naechste Leaf soll den
kleinsten fachlich wertvollen Folgeausbau neu inventarisieren.
