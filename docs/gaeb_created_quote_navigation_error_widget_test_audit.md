# GAEB: Abschlussaudit des Erzeugte-Quote-Navigationsfehler-Widgettests

## Ziel

Subtask 3.1.64.4 auditiert den in 3.1.64.3 implementierten read-only
Widgettest fuer den Fehler beim Oeffnen einer ueber `created_quote_id`
referenzierten Draft-Quote. Der Audit-Leaf aendert keine Runtime- oder
Testlogik.

## Gepruefter Umfang

Die Implementierung in
`client/test/sales_order_context_pages_test.dart` entspricht der Strategie aus
`docs/gaeb_created_quote_navigation_error_widget_test_strategy.md`:

- `_FakeApiClient` besitzt die standardmaessig leere Map
  `quoteDetailErrors`
- `getQuote` zeichnet die angefragte ID vor der Fehlerpruefung auf
- nur eine explizit konfigurierte Ziel-ID wirft den hinterlegten Fehler
- bestehende Fake-Aufrufer behalten ohne Konfiguration ihr bisheriges Verhalten
- genau ein neuer read-only Widgettest deckt den Navigationsfehler ab

## Audit-Ergebnis des Testvertrags

Der Test
`QuotesPage GAEB created quote navigation keeps selection on load failure`
verwendet einen eng abgegrenzten Aufbau:

- Berechtigung ausschliesslich `quotes.read`
- initialer Detailkontext `quote-existing-1`
- sichtbare Bestandsquote `ANG-BESTEHEND-0001`
- ein `applied` GAEB-Import
- `created_quote_id: quote-missing-1`
- ziel-ID-spezifische `ApiException` mit Status 404 und Code
  `quote_not_found`
- keine Apply-, Bearbeitungs- oder Konvertierungsmutation

## Nachgewiesenes Verhalten

### Abrufvertrag

`requestedQuoteIds` enthaelt exakt:

```text
quote-existing-1
quote-missing-1
```

Damit sind der initiale Bestandsabruf und der anschliessende fehlgeschlagene
Zielabruf belegt. Da der Zielabruf fehlschlaegt, wird kein erfolgreicher
Navigations-Reload ausgeloest.

### Dialog- und Auswahlvertrag

Nach `Quote öffnen` belegt der Test:

- der Importdialog ist geschlossen
- `ANG-BESTEHEND-0001` bleibt sichtbar
- `Status: draft` bleibt sichtbar
- `Kunde: Bestehender Kunde` bleibt sichtbar
- `Projekt: Bestehendes Projekt` bleibt sichtbar

Die Zielquote ersetzt die vorhandene Auswahl nicht, weil `_selected` erst nach
einem erfolgreichen `getQuote`-Abruf gesetzt wird.

### Feedbackvertrag

Die strukturierte API-Meldung
`Erzeugte Quote ist nicht mehr verfügbar` ist sichtbar. Die
Erfolgsmeldung `Erzeugte Quote wurde geöffnet` bleibt verborgen.

Damit ist sowohl das fachliche Fehlerfeedback als auch das Ausbleiben eines
falschen Erfolgssignals abgesichert.

## Scope-Audit

Der abgeschlossene Block bleibt innerhalb seiner definierten Grenze:

- keine Aenderung an `QuotesPage`
- keine Aenderung an `ApiClient`
- keine Backend-, API-, DB- oder Permission-Aenderung
- kein Apply-Fehler oder erneute Quote-Erzeugung
- kein Berechtigungsnegativtest
- kein Retry-, Filter- oder Dialog-Redesign
- keine Positions-, Preis-, Material-, Kalkulations- oder KI-Logik

Die einzige Runtime-nahe Erweiterung liegt im lokalen Test-Fake und ist durch
den leeren Default vollstaendig opt-in.

## Verifikation

Ausgefuehrt:

```powershell
cd client
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB created quote navigation keeps selection on load failure"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- gezielter Widgettest: erfolgreich
- Analyse: ausschliesslich zwei bereits bekannte Harness-Warnungen
  - unbenutzter Import `purchase_orders_page.dart`
  - optionaler Parameter `convertedQuote` wird nie gesetzt
- keine neue Analyzer-Warnung aus 3.1.64.3
- `git diff --check`: ohne Fehler

## Abschlussentscheidung

Task 3.1.64 ist fachlich und technisch abgeschlossen. Erfolgs- und Fehlerpfad
der direkten Navigation vom `applied` GAEB-Import zur erzeugten Draft-Quote
sind nun als getrennte, deterministische read-only Widgettests abgesichert.

Ein weiterer Schritt waere kein kleiner Restpunkt dieses Testblocks mehr.
Der naechste Leaf soll deshalb erneut inventarisieren, welcher fachlich
wertvolle GAEB-Ausbau nach der abgeschlossenen Navigationsabsicherung den
besten Signal-/Risikowert besitzt.
