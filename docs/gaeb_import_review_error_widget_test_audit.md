# GAEB: Abschlussaudit des Importlauf-Freigabefehler-Widgettests

## Ziel

Subtask 3.1.66.4 auditiert den in 3.1.66.3 implementierten Widgettest fuer
eine wegen offener Positionen serverseitig abgewiesene GAEB-Importlauf-
Freigabe. Dieser Audit-Leaf aendert weder Runtime- noch Testlogik.

## Gepruefter Umfang

Die Implementierung in
`client/test/sales_order_context_pages_test.dart` entspricht der Strategie aus
`docs/gaeb_import_review_error_widget_test_strategy.md`:

- `_FakeApiClient` besitzt die standardmaessig leere Map
  `quoteImportReviewErrors`
- `attemptedQuoteImportReviewIds` zeichnet Freigabeversuche separat auf
- `reviewedQuoteImportIds` enthaelt ausschliesslich erfolgreiche Freigaben
- `markQuoteImportReviewed` prueft konfigurierte Fehler vor dem Setzen des
  simulierten Erfolgszustands
- genau ein neuer Widgettest deckt den Freigabefehler ab

## Audit-Ergebnis des Testvertrags

Der Test
`QuotesPage GAEB import review keeps parsed state with pending item on failure`
verwendet einen eng abgegrenzten Aufbau:

- Berechtigungen `quotes.read` und `quotes.write`
- genau ein Import `import-review-failure-1`
- Ausgangsstatus `parsed`
- Summary `accepted_count: 0`, `rejected_count: 0`, `pending_count: 1`
- keine erzeugte Quote und keine Apply-Mutation
- strukturierte `ApiException` mit Status 409 und Code
  `pending_quote_import_items`

## Nachgewiesenes Verhalten

### Versuch- und Erfolgsvertrag

`attemptedQuoteImportReviewIds` enthaelt exakt:

```text
import-review-failure-1
```

`reviewedQuoteImportIds` bleibt leer. Damit sind genau ein Freigabeversuch und
das Ausbleiben eines simulierten Freigabeerfolgs belegt. Der Fake kann nach
der Exception nicht versehentlich einen `reviewed`-Zustand liefern.

### Dialog-, Status- und Summaryvertrag

Nach der fehlgeschlagenen Freigabe belegt der Test:

- genau ein `AlertDialog` bleibt sichtbar
- der Fortschrittstext ist nicht mehr sichtbar
- der erhaltene Dialog zeigt weiterhin
  `freigabe-fehler-ausschreibung.x83`
- der Status bleibt `parsed`
- `Review-Summary: 0 übernommen, 0 abgelehnt, 1 offen` bleibt sichtbar
- `Zur Übernahme freigeben` bleibt verfuegbar
- `Draft-Quote erzeugen` bleibt verborgen

Damit wird nur der Fortschrittsdialog geschlossen; der fachliche
Ausgangszustand des Importlaufs bleibt unveraendert.

### Fehler- und Negativvertrag

Sichtbar ist die strukturierte API-Meldung
`Importlauf enthält noch offene Positionen`.

Der Test belegt zugleich das Ausbleiben von:

- `Importlauf wurde freigegeben`
- `Status: reviewed`
- der Apply-Aktion
- Erzeugungs- oder Navigationssignalen

Es entsteht somit kein falscher Freigabe- oder Folgeprozess-Eindruck.

## Scope-Audit

Der abgeschlossene Block bleibt innerhalb seiner definierten Grenze:

- keine Aenderung an `QuotesPage`
- keine Aenderung an `ApiClient`
- keine Backend-, API-, DB- oder Permission-Aenderung
- keine clientseitige Vorabvalidierung von `pending_count`
- kein Positionsreview-Fehler oder Berechtigungsnegativtest
- kein Retry-, Recovery- oder Dialog-Redesign
- keine Transformations-, Mapping-, Kalkulations- oder KI-Logik

Die Erweiterungen liegen ausschliesslich im lokalen Test-Fake und sind durch
den leeren Fehler-Map-Default opt-in.

## Verifikation

Ausgefuehrt:

```powershell
cd client
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import review keeps parsed state with pending item on failure"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- gezielter Widgettest: erfolgreich
- Analyse: ausschliesslich zwei bereits bekannte Harness-Warnungen
  - unbenutzter Import `purchase_orders_page.dart`
  - optionaler Parameter `convertedQuote` wird nie gesetzt
- keine neue Analyzer-Warnung aus 3.1.66.3
- `git diff --check`: ohne Fehler

## Abschlussentscheidung

Task 3.1.66 ist fachlich und technisch abgeschlossen. Erfolgreiche und wegen
offener Positionen abgewiesene Importlauf-Freigaben sind nun als getrennte,
deterministische Widgettests abgesichert. Gemeinsam mit Apply- und
Navigationstests ist die zentrale Client-Prozesskette bis zur erzeugten
Draft-Quote fuer ihre wichtigsten Erfolgs- und Fehlerausgaenge abgedeckt.

Ein weiterer Schritt waere kein kleiner Restpunkt dieses Freigabe-Testblocks.
Der naechste Leaf soll deshalb erneut inventarisieren, welcher fachlich
wertvolle GAEB-Ausbau nach der abgeschlossenen Prozessfehlerabsicherung den
besten Signal-/Risikowert besitzt.
