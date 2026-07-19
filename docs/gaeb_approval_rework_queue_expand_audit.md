# GAEB-Freigabe: Abschlussaudit der erweiterbaren Nacharbeits-Queue

## Ziel

Subtask 3.1.55.4 auditiert die in 3.1.55.3 implementierte Ein- und
Ausklappfunktion der Nacharbeits-Queue. Dieser Leaf aendert weder Runtime- noch
Testcode.

## Audit-Ergebnis

Die Implementierung erfuellt das in
`docs/gaeb_approval_rework_queue_expand_strategy.md` definierte Minimalziel:

- eigener Zustand `_approvalReworkExpanded`
- Unabhaengigkeit von `_approvalRequestsExpanded`
- kompakte Standardansicht mit maximal drei Nacharbeitspositionen
- erweiterte Ansicht mit allen bereits geladenen `_approvalReworkItems`
- stabile Labels `Alle anzeigen` und `Weniger anzeigen`
- Umschaltaktion nur bei mehr als drei Treffern
- Ruecksetzung bei vollstaendigem `_load()`
- Zustandserhalt bei reinem Rework-Refresh
- Normalisierung auf kompakt bei hoechstens drei neuen Treffern

Backend, `ApiClient`, Rework-Readmodel, Berechtigungen und Navigation wurden
nicht erweitert.

## Unabhaengigkeit der Karten

Approval-Request- und Rework-Karte besitzen getrennte Booleans und getrennte
Umschaltaktionen. Das Ein- oder Ausklappen einer Karte veraendert die andere
nicht. Nur der vollstaendige Seiten-/Filter-Reload setzt beide Zustaende auf
ihre kompakte Standardansicht zurueck.

Die identischen Buttonlabels sind fachlich konsistent. Der Rework-Widgettest
laesst die Approval-Request-Liste leer, wodurch sein Finder eindeutig bleibt,
ohne produktionsseitige Test-Keys oder abweichende Benennungen einzufuehren.

## Rework-Vertrag

Reihenfolge, Kontextzeilen und `Bearbeiten`/`Anzeigen` verwenden weiterhin
dieselben ListTiles und Callbacks. Die einzige fachliche Aenderung ist, dass
alle bereits geladenen Rework-Positionen nun erreichbar sind.

Der Refresh-Vertrag bleibt stabil:

- mehr als drei neue Treffer erhalten eine laufende erweiterte Ansicht
- hoechstens drei Treffer normalisieren den Zustand auf kompakt
- ein Fehler behaelt vorhandene Liste und Anzeigezustand

## Testabdeckung

Der neue Widgettest verwendet:

- vier deterministische Rework-Eintraege
- `quotes.read` als einzige Berechtigung
- eine leere Approval-Request-Liste
- keine Navigation oder Mutation

Er prueft die anfaengliche Drei-Eintraege-Grenze, Expansion bis zum vierten
Eintrag und anschliessendes Einklappen.

## Verifikation

Gemeinsam ausgefuehrt im Verzeichnis `client/`:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage approval"
```

Ergebnis: alle drei passenden Tests bestanden:

- Queue-Entscheidungsdialog
- Approval-Request-Expand/Collapse
- Rework-Expand/Collapse

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten Hinweise zum
ungenutzten `purchase_orders_page.dart`-Import und nie gesetzten
`convertedQuote`-Fakeparameter bleiben allgemeine Test-Harness-Themen.

## Bewusste Grenze

Beide Karten koennen nun alle bereits geladenen Eintraege darstellen. Fuer
grosse Datenmengen bleibt eine spaetere dedizierte Arbeitsliste mit
serverseitiger Pagination sinnvoll. Sie ist jedoch ein eigener Backend-/API-
und Navigationsblock und keine weitere Haertung dieser lokalen
Erreichbarkeitsfunktion.

## Nicht-Ziele

- keine weitere UI- oder Testimplementierung
- keine generische Queue-Komponente
- keine kombinierte oder dedizierte Arbeitsliste
- keine Pagination, Filter oder Sortierung
- keine Queue-Mutation oder Massenaktion
- keine Cockpit-, KPI-, SLA-, Eskalations- oder KI-Logik
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Abschlussentscheidung

Task 3.1.55 ist fachlich und technisch abgeschlossen. Innerhalb der
Nacharbeits-Queue-Erreichbarkeit bleibt kein weiterer kleiner Haertungsschritt
mit gutem Signal. Der naechste Leaf soll den kleinsten Folgeausbau nach den nun
beiden voll erreichbaren und getesteten Approval-Arbeitsbereichen neu
inventarisieren.
