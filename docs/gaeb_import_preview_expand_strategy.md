# GAEB: Minimalstrategie fuer die erweiterbare Importvorschau

## Ziel

Subtask 3.1.56.2 definiert das technische Minimalzielmodell fuer das Ein- und
Ausklappen der bereits geladenen GAEB-Importvorschau. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Bestehender Vertrag

`_loadQuoteImports()` laedt fuer den aktuellen Projektfilter maximal sechs
Importlaeufe und speichert sie in `_quoteImports`. Die Karte zeigt derzeit fest
`_quoteImports.take(3)` als `visibleImports` und weist weitere geladene
Eintraege nur statisch aus.

Der neue Zustand erweitert ausschliesslich diese lokale Vorschau. `limit: 6`,
Backend-Endpoint, `ApiClient` und `Details`-Aktion bleiben unveraendert.

## Eigener lokaler Zustand

In `_QuotesPageState` wird eingefuehrt:

```text
bool _quoteImportsExpanded = false;
```

Der Zustand ist unabhaengig von `_approvalRequestsExpanded` und
`_approvalReworkExpanded`. Jede der drei Karten bleibt separat bedienbar.

## Sichtbare Importlaeufe

Die Ableitung wird minimal angepasst:

```text
source = _quoteImportsExpanded
  ? _quoteImports
  : _quoteImports.take(3)
```

Map-Konvertierung, Reihenfolge, Dateiname, Status, Uploadzeitpunkt und
`Details`-Callback bleiben unveraendert.

## Umschaltaktion

Bei gesetztem Projektfilter und `_quoteImports.length > 3` ersetzt ein
`TextButton` den statischen Restmengenhinweis:

- kompakt: `Alle anzeigen`
- erweitert: `Weniger anzeigen`

`Alle anzeigen` bedeutet in diesem Vertrag alle aktuell geladenen Eintraege,
nicht die vollstaendige serverseitige Importhistorie. Durch das unveraenderte
`limit: 6` werden maximal sechs Zeilen gerendert.

## Ruecksetz- und Refreshverhalten

### Vollstaendiger Reload

`_load()` setzt `_quoteImportsExpanded` zusammen mit den beiden Approval-
Zustaenden auf `false`. Projektfilterwechsel, Angebotsfilterung und globaler
Seiten-Refresh starten dadurch kompakt.

### Reiner Import-Refresh

`_loadQuoteImports()` behaelt den Import-Expand-State nur, wenn:

- der Projektfilter weiterhin nicht leer ist und
- die neue Liste mehr als drei Eintraege enthaelt.

Ohne Projektfilter oder bei hoechstens drei Ergebnissen wird
`_quoteImportsExpanded` zusammen mit der neuen Liste auf `false` normalisiert.
Ein Ladefehler behaelt die bestehende Liste und den Anzeigezustand.

## Fehlender Projektfilter

Die bestehende UI zeigt ohne Projekt-ID weiterhin ausschliesslich den Hinweis
`Für projektbezogene Importe bitte oben eine Projekt-ID setzen.`. Es erscheint
kein Umschaltbutton. Dieser Leaf aendert nicht, ob der bestehende Ladepfad mit
leerem Projektfilter aufgerufen wird; er stellt nur sicher, dass dabei kein
alter Expand-State erhalten bleibt.

## Minimale Fake-API-Erweiterung

Der vorhandene `_FakeApiClient` erhaelt:

```text
quoteImportList = const []
listQuoteImports(...) -> quoteImportList
```

Approval-Request- und Rework-Listen bleiben im Importtest leer. Weitere
Overrides oder Mutationsaufzeichnungen sind nicht erforderlich.

## Minimale Testabdeckung

Ein neuer Widgettest verwendet:

- `quotes.read`
- `initialFilters: CommercialFilterContext(projectId: 'project-1')`
- vier eindeutig benannte Importlaeufe
- leere Approval-Request- und Rework-Listen

Testablauf:

1. Import eins bis drei sichtbar, Import vier nicht sichtbar.
2. `Alle anzeigen` antippen.
3. Import vier und `Weniger anzeigen` sichtbar.
4. `Weniger anzeigen` antippen.
5. Import vier wieder nicht sichtbar und `Alle anzeigen` sichtbar.

Der Test oeffnet keinen Importdetaildialog und prueft weder Importstatuslogik
noch Datumsformatierung erneut.

## Implementierungsgrenze

Subtask 3.1.56.3 soll nur:

- `_quoteImportsExpanded` ergaenzen
- sichtbare Importliste und Umschaltaktion anpassen
- Ruecksetz- und Normalisierungsverhalten implementieren
- `_FakeApiClient` um `quoteImportList` erweitern
- genau einen Importvorschau-Expand-/Collapse-Widgettest ergaenzen

Produktionsseitig ist nur `client/lib/pages/quotes_page.dart`, testseitig nur
`client/test/sales_order_context_pages_test.dart` betroffen.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format lib/pages/quotes_page.dart test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import preview expands and collapses loaded imports"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- vollstaendige Importhistorie oder Pagination
- Aenderung von `limit: 6`
- Importsuche, Filter oder Sortierung
- generische Kartenkomponente
- Importdetail-, Upload-, Review- oder Apply-Aenderung
- Approval-, Cockpit-, KPI-, SLA- oder KI-Erweiterung
- Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.56.2 ist abgeschlossen. Der naechste Leaf 3.1.56.3 implementiert genau die
Ein-/Ausklappfunktion der geladenen GAEB-Importvorschau samt einem
deterministischen Widgettest.
