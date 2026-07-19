# GAEB: Folgeinventur nach dem getesteten Importdetail-Einstieg

## Ziel

Subtask 3.1.58.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem nun abgesicherten read-only Einstieg in den GAEB-Importdetaildialog. Dieser
Leaf aendert keine Runtime- oder Testlogik.

## Ausgangslage

Der Dialog laedt bereits das vollstaendige Ergebnis von
`listQuoteImportItems(importId)` in `items`. Die Darstellung begrenzt die
erreichbare Teilmenge jedoch fest:

- `visibleItems` verwendet immer `items.take(5)`.
- `remainingItems` zaehlt die nicht dargestellten Positionen.
- weitere Positionen erscheinen nur als statischer Text
  `+n weitere Positionen`.
- der Restmengenhinweis besitzt keine Aktion.
- nur die ersten fuenf Positionen koennen gelesen oder ueber
  `_openQuoteImportItemDetail(...)` geoeffnet werden.

Damit besteht im bereits geladenen read-only Detailmodell eine konkrete
Erreichbarkeitsluecke. Der neue Importdetail-Widgettest stellt bereits die
geeigneten ID-gebundenen Fake-Datenquellen bereit.

## Optionen

### A: Geladene Rohpositionen im Dialog ein- und ausklappen

Der Dialog zeigt standardmaessig fuenf und auf Anforderung alle bereits
geladenen `items`. `Weniger anzeigen` stellt die kompakte Ansicht wieder her.

Vorteile:

- schliesst eine sichtbare Erreichbarkeitsluecke
- rein clientseitig
- bestehender API- und Ladevertrag reicht aus
- alle Positionen behalten denselben Detail-Callback
- das bewaehrte Expand-/Collapse-Muster ist wiederverwendbar
- der vorhandene ID-gebundene Fake ermoeglicht einen engen Widgettest

Bewertung: kleinster Folgeausbau mit direktem GAEB-Review-Nutzen.

### B: Review-Mutation widgettesten

`Zur Übernahme freigeben` koennte mit `quotes.write` bis zum erfolgreichen
Refresh getestet werden.

Bewertung: wichtig, aber groesser. Der Pfad umfasst Mutation,
Fortschrittsdialog, Detail- und Listenrefresh sowie Snackbar. Er behebt nicht,
dass geladene Positionen sechs und folgende derzeit unerreichbar bleiben.

### C: Apply-Mutation widgettesten

Die Erzeugung einer Draft-Quote koennte end-to-end im Widget-Harness simuliert
werden.

Bewertung: noch groesser als der Review-Test und an mehrere Status- und
Navigationsvertraege gekoppelt.

### D: Serverseitige Pagination fuer Importpositionen

Der Positionsendpunkt koennte Seiten, Limits und Gesamtanzahl liefern.

Bewertung: bei sehr grossen Leistungsverzeichnissen langfristig relevant, aber
ein neuer Backend-/API-Vertrag. Die aktuelle Luecke betrifft bereits geladene
Positionen und benoetigt keine Pagination.

### E: Positionssuche, Filterung oder KI-Priorisierung

Positionen koennten nach Nummer, Beschreibung, Reviewstatus oder KI-Risiko
gefiltert und priorisiert werden.

Bewertung: benoetigt neue Interaktions- und Fachregeln und ist groesser als die
lokale Erreichbarkeitsluecke.

## Entscheidung

Der naechste Ausbau ist eine lokale Ein-/Ausklappfunktion fuer die bereits
geladenen Rohpositionen im GAEB-Importdetaildialog.

Das Ziel bleibt eng:

- kompakt weiterhin maximal fuenf Positionen
- `Alle anzeigen` zeigt alle aktuell geladenen `items`
- `Weniger anzeigen` stellt die Fuenf-Positionen-Ansicht wieder her
- Button nur bei mehr als fuenf Positionen
- Reihenfolge, Positionstexte und `_openQuoteImportItemDetail(...)` bleiben
  unveraendert
- Zustand gilt nur fuer die Lebensdauer des geoeffneten Importdialogs
- genau ein Widgettest mit sechs deterministischen Positionen

Die Funktion behauptet nicht, serverseitig paginierte oder bislang nicht
geladene Positionen zu zeigen.

## Naechster Leaf

Subtask 3.1.58.2 schneidet das technische Minimalzielmodell zu:

- lokaler Dialogzustand innerhalb `_openQuoteImportDetail(...)`
- Ableitung von `visibleItems`
- Verhalten bei `refreshImportState()`
- Position und Labels der Umschaltaktion
- Erweiterung des vorhandenen Importdetail-Widgettests oder genau ein neuer
  isolierter Test

## Nicht-Ziele

- noch keine Implementierung
- keine Review-, Apply- oder Upload-Mutation
- keine Aenderung des Positionsdetaildialogs
- keine Pagination, Suche, Filterung oder Sortierung
- keine generische Dialog- oder Listenkomponente
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine Cockpit-, KPI-, SLA- oder KI-Logik

## Ergebnis

3.1.58.1 ist abgeschlossen. Der kleinste Folgeausbau macht alle bereits
geladenen Rohpositionen des Importdetaildialogs erreichbar. Subtask 3.1.58.2
definiert dafuer das technische Minimalmodell.
