# GAEB: Folgeinventur nach den erreichbaren Rohpositionen

## Ziel

Subtask 3.1.59.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
den nun voll erreichbaren und widgetgetesteten Rohpositionen im
GAEB-Importdetaildialog. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Ausgangslage

Der Importdetaildialog kann inzwischen alle bereits geladenen Rohpositionen
anzeigen. Jede Position besitzt einen bestehenden Navigationspfad:

```text
ListTile.onTap -> _openQuoteImportItemDetail(importId, itemId)
```

Dieser Pfad laedt `getQuoteImportItem(importId, itemId)` und zeigt einen zweiten
`AlertDialog` mit:

- Positionsnummer und Gliederung
- Menge und Einheit
- Optional-Kennzeichen
- Reviewstatus, Parserhinweis und Reviewnotiz
- Beschreibung
- optionaler Quote-Verknuepfung

Im vorhandenen Flutter-Harness besteht dafuer noch keine Abdeckung:

- `_FakeApiClient` stellt keine ID-gebundenen Importpositionsdetails bereit.
- Der read-only Importdetail-Test zeigt Positionen nur an.
- Keine Position wird angetippt.
- Dialogtitel, Kerndaten und read-only Berechtigungsgrenze des
  Positionsdetaildialogs sind ungesichert.

## Optionen

### A: Read-only Positionsdetail-Uebergang widgettesten

Der Fake erhaelt einen nach Import- und Positions-ID gebundenen Detailvertrag.
Der vorhandene Importdetail-Test oeffnet eine sichtbare Position, prueft wenige
stabile Kerndaten und schliesst den inneren Dialog wieder.

Vorteile:

- sichert den direkten Folgepfad aller nun erreichbaren Rohpositionen
- keine Runtime-Aenderung erforderlich
- bestehender `ApiClient`-Vertrag reicht aus
- nutzt den bereits aufgebauten Importdetail-Test
- belegt die Weitergabe beider IDs
- bleibt mit `quotes.read` rein lesend

Bewertung: kleinster Folgeausbau mit hohem Regressionssignal.

### B: Review-Entscheidung einer Position widgettesten

Mit `quotes.write` koennte `Review setzen` bis zur gespeicherten Entscheidung
durchlaufen werden.

Bewertung: fachlich wichtig, aber groesser. Der Pfad umfasst einen dritten
Dialog, Formzustand, Mutation, aktualisiertes Detail und Snackbar. Zuerst sollte
der read-only Einstieg stabil abgesichert werden.

### C: Quote-Verknuepfung oeffnen widgettesten

Eine bereits uebernommene Position koennte zur verknuepften Quote navigieren.

Bewertung: benoetigt zusaetzlichen Quote-Detail-, Reload- und
Navigationsvertrag. Fuer noch nicht uebernommene Rohpositionen nicht allgemein
anwendbar.

### D: Positionssuche oder Filterung

Die geladenen Rohpositionen koennten nach Nummer, Text oder Reviewstatus
gefiltert werden.

Bewertung: neuer Interaktionszustand und fuer die aktuelle Testluecke nicht
erforderlich.

### E: KI-Priorisierung oder automatische Reviewentscheidung

Positionen koennten anhand fachlicher Risiken priorisiert oder vorgeschlagen
bewertet werden.

Bewertung: langfristig zentral, benoetigt aber Modell-, Nachvollziehbarkeits-
und Freigaberegeln und ist kein kleiner Folgeausbau.

## Entscheidung

Der naechste Ausbau sichert den bestehenden read-only Uebergang von einer
Rohposition in den Positionsdetaildialog mit einem engen Widgettest ab.

Das Ziel bleibt begrenzt:

- ID-gebundener Fake fuer `getQuoteImportItem(importId, itemId)`
- vorhandenen Importdetail-Test weiterverwenden
- genau eine sichtbare Rohposition antippen
- inneren `AlertDialog` anhand Titel und weniger stabiler Kerndaten pruefen
- bei nur `quotes.read` das Fehlen von `Review setzen` bestaetigen
- inneren Dialog ueber `Schließen` schliessen
- anschliessend auch den aeusseren Importdialog wie bisher schliessen

Runtime-Code wird nur geaendert, wenn der Strategieleaf einen konkreten Defekt
im bestehenden read-only Pfad nachweist. Nach aktueller Inventur ist keiner
erkennbar.

## Naechster Leaf

Subtask 3.1.59.2 definiert das technische Minimalmodell fuer:

- verschachtelte ID-Struktur im Fake
- kleinsten stabilen Positionsdetail-Payload
- eindeutige Finder bei zwei Dialogebenen
- Reihenfolge des inneren und aeusseren Schliessens
- gezielten Test- und Analysebefehl

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- kein Review-Mutations- oder Formulartest
- keine Quote-Navigation
- keine Suche, Filterung, Pagination oder Sortierung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine Cockpit-, KPI-, SLA- oder KI-Logik

## Ergebnis

3.1.59.1 ist abgeschlossen. Der kleinste Folgeausbau ist die read-only
Widgettest-Abdeckung des bestehenden Importpositionsdetail-Uebergangs.
Subtask 3.1.59.2 definiert dafuer das technische Minimalmodell.
