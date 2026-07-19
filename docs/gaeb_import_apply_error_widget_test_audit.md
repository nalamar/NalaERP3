# GAEB: Abschlussaudit des Apply-Fehler-Widgettests

## Ziel

Subtask 3.1.65.4 auditiert den in 3.1.65.3 implementierten Widgettest fuer
einen serverseitig abgewiesenen Versuch, aus einem `reviewed` GAEB-Import eine
Draft-Quote zu erzeugen. Dieser Audit-Leaf aendert weder Runtime- noch
Testlogik.

## Gepruefter Umfang

Die Implementierung in
`client/test/sales_order_context_pages_test.dart` entspricht der Strategie aus
`docs/gaeb_import_apply_error_widget_test_strategy.md`:

- `_FakeApiClient` besitzt die standardmaessig leere Map
  `quoteImportApplyErrors`
- `applyQuoteImport` zeichnet die Import-ID vor der Fehlerpruefung auf
- nur eine explizit konfigurierte Import-ID wirft den hinterlegten Fehler
- das bisherige Erfolgsverhalten bleibt ohne Fehlerkonfiguration unveraendert
- genau ein neuer Widgettest deckt den Apply-Fehlerausgang ab

## Audit-Ergebnis des Testvertrags

Der Test `QuotesPage GAEB import apply keeps reviewed state on failure`
verwendet einen eng abgegrenzten Aufbau:

- Berechtigungen `quotes.read` und `quotes.write`
- genau ein Import `import-apply-failure-1`
- Ausgangsstatus `reviewed`
- eine akzeptierte Position in der Summary
- keine `created_quote_id`
- leere Importpositionsliste
- strukturierte `ApiException` mit Status 409 und Code
  `invalid_quote_import_status`

## Nachgewiesenes Verhalten

### Mutationsvertrag

`appliedQuoteImportIds` enthaelt exakt:

```text
import-apply-failure-1
```

Damit ist genau ein Apply-Versuch belegt. Die Aufzeichnung erfolgt vor dem
konfigurierten Fehler; es wird kein zweiter Versuch und kein Erfolgsrefresh
ausgeloest.

### Dialog- und Zustandsvertrag

Nach dem fehlgeschlagenen Apply belegt der Test:

- genau ein `AlertDialog` bleibt sichtbar
- der Fortschrittstext ist nicht mehr sichtbar
- der erhaltene Dialog zeigt weiterhin
  `apply-fehler-ausschreibung.x83`
- der Status bleibt `reviewed`
- `Draft-Quote erzeugen` bleibt verfuegbar

Damit ist nachgewiesen, dass nur der Fortschrittsdialog geschlossen wird und
der Importdetaildialog seinen fachlichen Ausgangszustand behaelt.

### Fehler- und Negativvertrag

Sichtbar ist ausschliesslich die strukturierte API-Meldung
`Importlauf kann aktuell nicht angewendet werden`.

Der Test belegt zugleich das Ausbleiben von:

- `Erzeugte Quote:`
- dem Hinweis auf eine oeffenbare erzeugte Quote
- `Quote öffnen`
- der Erfolgsmeldung fuer eine erzeugte Draft-Quote

Es entsteht somit weder ein falscher Zustandswechsel noch ein irrefuehrendes
Erfolgs- oder Navigationssignal.

## Scope-Audit

Der abgeschlossene Block bleibt innerhalb seiner definierten Grenze:

- keine Aenderung an `QuotesPage`
- keine Aenderung an `ApiClient`
- keine Backend-, API-, DB- oder Permission-Aenderung
- kein Review-Fehler- oder Berechtigungsnegativtest
- kein Retry-, Recovery- oder Dialog-Redesign
- keine Positions-, Preis-, Material-, Kalkulations- oder KI-Logik

Die einzige Erweiterung liegt im lokalen Test-Fake und ist durch den leeren
Default vollstaendig opt-in.

## Verifikation

Ausgefuehrt:

```powershell
cd client
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import apply keeps reviewed state on failure"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- gezielter Widgettest: erfolgreich
- Analyse: ausschliesslich zwei bereits bekannte Harness-Warnungen
  - unbenutzter Import `purchase_orders_page.dart`
  - optionaler Parameter `convertedQuote` wird nie gesetzt
- keine neue Analyzer-Warnung aus 3.1.65.3
- `git diff --check`: ohne Fehler

## Abschlussentscheidung

Task 3.1.65 ist fachlich und technisch abgeschlossen. Der erfolgreiche
Apply-Uebergang und der serverseitig abgewiesene Apply-Versuch sind nun als
getrennte, deterministische Widgettests abgesichert. Zusammen mit den
Navigationstests ist die Clientstrecke vom `reviewed` Import bis zur
erzeugten beziehungsweise kontrolliert nicht erzeugten Draft-Quote robust
abgedeckt.

Ein weiterer Schritt waere kein kleiner Restpunkt dieses Fehler-Testblocks.
Der naechste Leaf soll deshalb erneut inventarisieren, welcher fachlich
wertvolle GAEB-Ausbau nach der abgeschlossenen Apply- und
Navigationsabsicherung den besten Signal-/Risikowert besitzt.
