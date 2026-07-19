# GAEB-Freigabe: Minimalstrategie fuer die erweiterbare Approval-Queue

## Ziel

Subtask 3.1.54.2 definiert das technische Minimalzielmodell fuer das Ein- und
Ausklappen der bestehenden Freigabeanforderungs-Karte. Dieser Leaf aendert
weder Runtime- noch Testcode.

## Bestehender Anker

`QuotesPage` haelt alle geladenen Freigabeanforderungen bereits in
`_approvalRequestItems`. Im `build(...)` wird daraus derzeit fest
`_approvalRequestItems.take(3)` als `visibleApprovalRequestItems` gebildet.
Weitere Eintraege werden lediglich durch einen statischen Restmengen-Text
angezeigt.

Backend, `ApiClient` und Queue-Readmodel muessen fuer den Ausbau nicht
veraendert werden.

## Lokaler Zustand

In `_QuotesPageState` wird genau ein neuer Zustand eingefuehrt:

```text
bool _approvalRequestsExpanded = false;
```

Der Zustand ist rein visuell. Er wird nicht persistiert, nicht an andere
Seiten weitergereicht und beeinflusst weder Filter noch Queue-Mutationen.

## Sichtbare Eintraege

Die bestehende Ableitung wird minimal erweitert:

```text
source = _approvalRequestsExpanded
  ? _approvalRequestItems
  : _approvalRequestItems.take(3)
```

Danach bleibt die bestehende Map-Konvertierung unveraendert. Reihenfolge und
Aktionen der Queue-Eintraege werden nicht angefasst.

## Umschaltaktion

Wenn `_approvalRequestItems.length > 3` gilt, erscheint unter den Eintraegen
ein `TextButton`:

- kompakt: `Alle anzeigen`
- erweitert: `Weniger anzeigen`

Der bisherige statische Text `+n weitere offene Anforderungen` wird durch die
interaktive Aktion ersetzt. Die Restmenge kann im kompakten Zustand weiterhin
als Zusatzinformation in der Beschriftung oder unmittelbar daneben erscheinen;
entscheidend sind die stabilen Aktionslabels.

Der Button aendert ausschliesslich `_approvalRequestsExpanded` per
`setState(...)`.

## Ruecksetzverhalten

### Vollstaendiges Laden und Filterwechsel

`_load()` setzt `_approvalRequestsExpanded` vor dem Laden der fachlichen
Listen auf `false`. Damit beginnen:

- Projektfilterwechsel
- Angebotsfilterung
- vollstaendiger Seiten-Refresh

immer wieder in der kompakten Standardansicht.

### Reiner Queue-Refresh und Entscheidungen

`_loadApprovalRequestQueue()` behaelt den bisherigen Zustand, solange die neu
geladene Liste weiterhin mehr als drei Eintraege enthaelt. Sinkt die Liste auf
drei oder weniger Eintraege, wird `_approvalRequestsExpanded` zusammen mit der
neuen Liste auf `false` normalisiert.

Damit bleibt die operative Ansicht nach Genehmigen oder Ablehnen offen, wenn
noch weitere Arbeit vorhanden ist. Zugleich kann ein spaeter wieder wachsender
Datensatz nicht unerwartet aufgrund eines alten unsichtbaren Zustands erweitert
erscheinen.

## Fehlerverhalten

Schlaegt ein Queue-Refresh fehl, bleibt die bereits dargestellte Liste samt
Ein-/Ausklappzustand erhalten. Der bestehende SnackBar-Fehlerpfad bleibt
unveraendert.

## Minimale Testabdeckung

Der vorhandene `_FakeApiClient` kann bereits deterministische
`approvalRequestList`-Daten liefern. Ein neuer enger Widgettest verwendet vier
unterscheidbare Queue-Eintraege und nur `quotes.read`:

1. `QuotesPage` pumpen.
2. Eintrag 1 bis 3 sichtbar, Eintrag 4 nicht sichtbar.
3. `Alle anzeigen` antippen.
4. Eintrag 4 und `Weniger anzeigen` sichtbar.
5. `Weniger anzeigen` antippen.
6. Eintrag 4 wieder nicht sichtbar und `Alle anzeigen` sichtbar.

Der Test prueft keine Entscheidungsmutation und wiederholt nicht den bereits
abgesicherten Dialogkontext.

## Implementierungsgrenze

Subtask 3.1.54.3 soll nur:

- den lokalen Expand-State ergaenzen
- die sichtbare Liste und Umschaltaktion anpassen
- Ruecksetz- und Normalisierungsverhalten implementieren
- genau einen Widgettest fuer Ein- und Ausklappen ergaenzen

Keine Produktionsdatei ausser `client/lib/pages/quotes_page.dart` und keine
Testdatei ausser `client/test/sales_order_context_pages_test.dart` ist dafuer
erforderlich.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage approval queue expands and collapses loaded requests"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die beiden bereits dokumentierten allgemeinen Hinweise im gemeinsamen
Test-Harness sind kein Teil dieses Leaves, solange keine neue Warnung entsteht.

## Nicht-Ziele

- dedizierte Approval-Arbeitsliste
- serverseitige Pagination oder neuer Endpoint
- neue Filter oder Sortierung
- Massenentscheidungen
- Animation, Golden- oder Screenshot-Test
- Aenderung der Genehmigen-, Ablehnen- oder Oeffnen-Aktionen
- Backend-, API-, Datenbank- oder Berechtigungsaenderung

## Ergebnis

3.1.54.2 ist abgeschlossen. Der naechste Leaf 3.1.54.3 implementiert genau die
clientseitige Ein-/Ausklappfunktion samt einem deterministischen Widgettest.
