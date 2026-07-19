# GAEB: Abschlussaudit des Erzeugte-Quote-Navigations-Widgettests

## Ziel

Subtask 3.1.63.4 auditiert den in 3.1.63.3 ergaenzten read-only Widgettest fuer
die Navigation von einem angewendeten GAEB-Import zur erzeugten Draft-Quote.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Audit-Ergebnis

Die Umsetzung erfuellt das in
`docs/gaeb_import_created_quote_navigation_widget_test_strategy.md`
definierte Minimalziel:

- eine Quote-ID-Aufzeichnungsliste im bestehenden Fake
- instrumentiertes `getQuote(id)` bei unveraenderter Antwortsemantik
- genau ein separater read-only Navigations-Widgettest
- ein `applied` Import mit eindeutiger `created_quote_id`
- konsistente Quote-Zusammenfassung und Quote-Detailantwort
- zwei erwartete Abrufe derselben Quote-ID
- Schliessen des Importdetaildialogs
- sichtbare Auswahl der erzeugten Draft-Quote
- Pruefung der Erfolgssnackbar

Runtime, `ApiClient`, Backend, Datenbank und Berechtigungsvertrag wurden nicht
veraendert.

## Quote-ID- und Reloadvertrag

`requestedQuoteIds` zeichnet jeden `getQuote`-Aufruf auf. Nach
`Quote öffnen` enthaelt die Liste exakt zweimal `quote-gaeb-open-1`:

1. direkter Abruf innerhalb der Navigationsaktion
2. erneuter Detailabruf durch `_loadDetail` innerhalb von `_load()`

Damit ist sowohl die Weitergabe von `created_quote_id` als auch der
anschliessende Reload belegt. `getQuote` liefert weiterhin unveraendert das
konfigurierte `quoteDetail`; bestehende Fake-Aufrufer behalten ihr bisheriges
Verhalten.

`quoteList` und `quoteDetail` verwenden dieselbe ID und Angebotsnummer. Dadurch
bleiben Angebotsliste, Auswahl und rechte Detailansicht nach dem Reload
konsistent.

## Sichtbares Navigationsergebnis

Vor der Navigation belegt der Test:

- `Status: applied`
- `Erzeugte Quote: quote-gaeb-open-1`
- `Quote öffnen` ist sichtbar

Nach der Navigation belegt er:

- kein `AlertDialog` ist mehr sichtbar
- `ANG-GAEB-OPEN-0001` wird in der Angebotsansicht gerendert
- `Status: draft`
- `Kunde: GAEB Navigationskunde`
- `Projekt: GAEB Navigationsprojekt`
- `Erzeugte Quote wurde geöffnet`

Die eindeutigen Chiptexte belegen das geladene Detail, ohne interne
State-Objekte, Auswahlfarben oder Widgetpositionen zu inspizieren.

## Scope-Grenze

Der Test verwendet nur `quotes.read`, genau einen angewendeten Import und eine
minimale Draft-Quote ohne Positionen oder Folgebelege.

Nicht Bestandteil dieses Tasks sind:

- erneute Apply-Mutation
- Quote-Bearbeitung, Freigabe, PDF oder Konvertierung
- Positions-, Preis-, Material- oder Kalkulationspruefung
- Fehler- oder Berechtigungsnegativpfad
- Backend-, API-, Datenbank- oder Permission-Aenderungen
- KI-Automatisierung

## Verifikation

Im Verzeichnis `client/` erneut ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB created quote navigation opens selected draft"
```

Ergebnis: Der gezielte Widgettest ist erfolgreich.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten allgemeinen
Harness-Hinweise zum ungenutzten `purchase_orders_page.dart`-Import und zum nie
gesetzten optionalen Fakeparameter `convertedQuote` bleiben unveraendert.

## Abschlussentscheidung

Task 3.1.63 ist fachlich und technisch abgeschlossen. Die clientseitige
GAEB-Prozesskette reicht nun vom Importdetail ueber Review, Freigabe und
Draft-Quote-Erzeugung bis zur ausgewaehlten Angebotsdetailansicht. Der naechste
Leaf soll den kleinsten fachlich wertvollen Folgeausbau neu inventarisieren.
