# GAEB: Abschlussaudit der erweiterbaren Importvorschau

## Ziel

Subtask 3.1.56.4 auditiert die in 3.1.56.3 implementierte Ein- und
Ausklappfunktion der geladenen GAEB-Importvorschau. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Audit-Ergebnis

Die Implementierung erfuellt das in
`docs/gaeb_import_preview_expand_strategy.md` definierte Minimalziel:

- eigener lokaler Zustand `_quoteImportsExpanded`
- Unabhaengigkeit von beiden Approval-Expand-Zustaenden
- kompakte Standardansicht mit maximal drei Importlaeufen
- erweiterte Ansicht mit allen bereits geladenen `_quoteImports`
- stabile Labels `Alle anzeigen` und `Weniger anzeigen`
- Umschaltaktion nur bei mehr als drei geladenen Eintraegen
- Ruecksetzung bei vollstaendigem `_load()`
- Zustandserhalt bei reinem Import-Refresh mit weiterhin mehr als drei Treffern
- Normalisierung ohne Projektfilter oder bei hoechstens drei neuen Treffern

`limit: 6`, Backend, `ApiClient`, Import-Readmodel, Berechtigungen und
Detailnavigation wurden nicht erweitert.

## Vertrags- und Zustandspruefung

Reihenfolge, Dateiname, Status, Uploadzeitpunkt und `Details`-Callback verwenden
weiterhin dieselben ListTiles. Die einzige fachliche Aenderung ist die lokale
Auswahl zwischen `take(3)` und der bereits geladenen Gesamtliste.

Das Ruecksetzverhalten ist konsistent:

- Projektfilterwechsel und globaler Seiten-Refresh starten kompakt.
- Ein reiner Import-Refresh behaelt eine erweiterte Ansicht, solange weiterhin
  ein Projektfilter und mehr als drei Ergebnisse vorliegen.
- Ohne Projektfilter oder bei hoechstens drei Ergebnissen bleibt kein
  unsichtbarer Expand-State bestehen.
- Im Fehlerpfad werden vorhandene Liste und Anzeigezustand nicht ueberschrieben.

Ohne Projektfilter bleibt ausschliesslich der vorhandene Projekthinweis
sichtbar. Der Umschaltbutton liegt nur im Zweig einer nicht leeren sichtbaren
Importliste und kann dort nicht erscheinen.

## Testabdeckung

Der Widgettest verwendet vier deterministische Importlaeufe, `quotes.read` und
einen initialen Projektfilter. Approval-Request- und Rework-Listen bleiben leer,
damit die identischen Buttonlabels eindeutig gefunden werden.

Geprueft werden die anfaengliche Drei-Eintraege-Grenze, Expansion bis zum
vierten Import und anschliessendes Einklappen. Detaildialog, Statuslogik und
Datumsformatierung bleiben bewusst ausserhalb dieses Tests.

## Verifikation

Im Verzeichnis `client/` erneut ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import preview expands and collapses loaded imports"
```

Ergebnis: der gezielte Widgettest ist erfolgreich.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten Hinweise zum
ungenutzten `purchase_orders_page.dart`-Import und nie gesetzten
`convertedQuote`-Fakeparameter bleiben allgemeine Test-Harness-Themen.

## Bewusste Grenze

`Alle anzeigen` bezeichnet alle aktuell geladenen Eintraege. Wegen des
unveraenderten `limit: 6` ist dies keine vollstaendige Importhistorie. Eine
Historienansicht oder serverseitige Pagination waere ein eigener API- und
Navigationsblock und keine weitere Haertung dieser lokalen Vorschau.

## Nicht-Ziele

- keine weitere UI- oder Testimplementierung
- keine Aenderung von `limit: 6`
- keine Importhistorie, Pagination, Suche, Filterung oder Sortierung
- keine Importdetail-, Upload-, Review- oder Apply-Aenderung
- keine generische Kartenkomponente
- keine Approval-, Cockpit-, KPI-, SLA- oder KI-Erweiterung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Abschlussentscheidung

Task 3.1.56 ist fachlich und technisch abgeschlossen. Innerhalb der lokalen
Importvorschau bleibt kein weiterer kleiner Haertungsschritt mit gutem Signal.
Der naechste Leaf soll den kleinsten Folgeausbau nach den nun erreichbaren und
getesteten Approval-Queues und GAEB-Importlaeufen neu inventarisieren.
