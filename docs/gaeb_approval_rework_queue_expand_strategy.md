# GAEB-Freigabe: Minimalstrategie fuer die erweiterbare Nacharbeits-Queue

## Ziel

Subtask 3.1.55.2 definiert das technische Minimalzielmodell fuer das Ein- und
Ausklappen der bestehenden Nacharbeits-Queue. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Bestehender Anker

`QuotesPage` haelt die vollstaendig geladene Nacharbeitsliste bereits in
`_approvalReworkItems`. Im `build(...)` wird sie derzeit fest ueber `take(3)`
zu `visibleApprovalReworkItems` begrenzt. Weitere Treffer erscheinen nur als
nicht interaktiver Restmengen-Text.

Die direkt benachbarte Freigabeanforderungs-Karte besitzt bereits das passende
Ein-/Ausklappmuster. Dieses Verhalten wird fuer Nacharbeit eng wiederverwendet,
ohne beide Karten in eine generische Komponente umzubauen.

## Eigener lokaler Zustand

In `_QuotesPageState` wird genau ein weiterer visueller Zustand eingefuehrt:

```text
bool _approvalReworkExpanded = false;
```

Der Zustand ist unabhaengig von `_approvalRequestsExpanded`. Das Ein- oder
Ausklappen einer Karte darf die andere Karte nicht veraendern.

## Sichtbare Nacharbeitspositionen

Die Ableitung wird auf dasselbe enge Muster umgestellt:

```text
source = _approvalReworkExpanded
  ? _approvalReworkItems
  : _approvalReworkItems.take(3)
```

Die anschliessende Map-Konvertierung, Reihenfolge, Kontextzeilen und
`Bearbeiten`/`Anzeigen`-Aktion bleiben unveraendert.

## Umschaltaktion

Wenn `_approvalReworkItems.length > 3` gilt, ersetzt ein `TextButton` den
bisherigen statischen Restmengenhinweis:

- kompakt: `Alle anzeigen`
- erweitert: `Weniger anzeigen`

Die identischen Labels beider Karten sind bewusst konsistent. Sie sind
innerhalb der jeweiligen Karte semantisch eindeutig; der isolierte
Rework-Widgettest liefert keine Approval-Request-Eintraege und vermeidet damit
mehrdeutige Finder.

## Ruecksetz- und Refreshverhalten

### Vollstaendiger Reload

`_load()` setzt neben `_approvalRequestsExpanded` auch
`_approvalReworkExpanded` auf `false`. Projektfilterwechsel, Angebotsfilterung
und globaler Seiten-Refresh beginnen damit fuer beide Karten kompakt.

### Reiner Rework-Refresh

`_loadApprovalReworkQueue()` behaelt den Rework-Expand-State, solange die neu
geladene Liste mehr als drei Positionen enthaelt. Bei hoechstens drei
Positionen wird `_approvalReworkExpanded` zusammen mit der neuen Liste auf
`false` normalisiert.

Ein Fehler laesst vorhandene Liste und Anzeigezustand unveraendert; der
bestehende SnackBar-Pfad bleibt bestehen.

## Minimale Fake-API-Erweiterung

Der vorhandene `_FakeApiClient` ueberschreibt
`listQuoteApprovalRework(...)` bereits deterministisch mit einer leeren Liste.
Fuer den Test wird dieser Fake minimal konfigurierbar:

```text
approvalReworkList = const []
listQuoteApprovalRework(...) -> approvalReworkList
```

`approvalRequestList` bleibt im Rework-Test leer. Weitere API-Overrides oder
Mutationsaufzeichnungen sind nicht erforderlich.

## Minimale Testabdeckung

Ein neuer Widgettest verwendet vier unterscheidbare Rework-Eintraege und nur
`quotes.read`:

1. `QuotesPage` mit leerer Approval-Request-Liste und vier Rework-Eintraegen
   pumpen.
2. Rework-Eintraege eins bis drei sichtbar, Eintrag vier nicht sichtbar.
3. `Alle anzeigen` antippen.
4. Eintrag vier und `Weniger anzeigen` sichtbar.
5. `Weniger anzeigen` antippen.
6. Eintrag vier wieder nicht sichtbar und `Alle anzeigen` sichtbar.

Der Test oeffnet keine Quote und prueft weder Rework-Kontextformatierung noch
Editorfokus erneut.

## Implementierungsgrenze

Subtask 3.1.55.3 soll nur:

- `_approvalReworkExpanded` ergaenzen
- sichtbare Rework-Liste und Umschaltaktion anpassen
- Ruecksetz- und Normalisierungsverhalten implementieren
- `_FakeApiClient` um `approvalReworkList` erweitern
- genau einen Rework-Expand-/Collapse-Widgettest ergaenzen

Produktionsseitig ist nur `client/lib/pages/quotes_page.dart`, testseitig nur
`client/test/sales_order_context_pages_test.dart` betroffen.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage approval rework queue expands and collapses loaded items"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- generische Queue-Komponente
- kombinierte oder dedizierte Approval-Arbeitsliste
- Pagination, neue Filter oder Sortierung
- Rework-Mutation oder Massenaktion
- erneuter Kontext-, Navigation- oder Editor-Fokus-Test
- Cockpit-, KPI-, SLA-, Eskalations- oder KI-Logik
- Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.55.2 ist abgeschlossen. Der naechste Leaf 3.1.55.3 implementiert genau die
eigenstaendige Rework-Ein-/Ausklappfunktion samt einem deterministischen
Widgettest.
