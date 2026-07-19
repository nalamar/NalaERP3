# GAEB: Folgeinventur nach der erweiterbaren Importvorschau

## Ziel

Subtask 3.1.57.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
den nun erreichbaren und widgetgetesteten Approval-Queues und
GAEB-Importlaeufen. Dieser Leaf aendert keine Laufzeit- oder Testlogik.

## Ausgangslage

Die Angebotsseite kann inzwischen:

- offene Freigabeanforderungen vollstaendig anzeigen und entscheiden
- offene Nacharbeitspositionen vollstaendig anzeigen und oeffnen
- bis zu sechs bereits geladene GAEB-Importlaeufe ein- und ausklappen
- jeden sichtbaren Importlauf ueber `Details` an den bestehenden
  `_openQuoteImportDetail(...)`-Pfad uebergeben

Der Import-Expand-/Collapse-Vertrag ist durch einen Widgettest abgesichert.
Der anschliessende read-only Detailuebergang besitzt im vorhandenen
`sales_order_context_pages_test.dart`-Harness jedoch noch keine Abdeckung:

- `_FakeApiClient` liefert nur `listQuoteImports(...)` konfigurierbar.
- `getQuoteImport(...)` und `listQuoteImportItems(...)` sind nicht fuer einen
  isolierten QuotesPage-Test ueberschrieben.
- Kein Widgettest tippt `Details` an und bestaetigt, dass der vorhandene
  Importdialog mit dem ausgewaehlten Import geladen wird.

Damit ist der wichtigste direkte Folgepfad der nun erreichbaren Importzeilen
noch ungesichert, obwohl kein neuer Produktvertrag erforderlich ist.

## Optionen

### A: Read-only Detailuebergang als Widgettest absichern

Der Fake erhaelt deterministische Detail- und Positionsdaten. Ein enger Test
laedt genau einen Import, tippt dessen `Details`-Aktion und prueft wenige
stabile Kernaussagen des bestehenden Dialogs.

Vorteile:

- sichert den direkten Nutzerpfad von Vorschau zu Importbearbeitung
- keine Runtime-Aenderung erforderlich
- kein Backend-, API- oder Permission-Ausbau
- bestehender `ApiClient`-Vertrag reicht aus
- kleiner, deterministischer Testumfang

Bewertung: kleinster Folgeausbau mit hohem Regressionssignal.

### B: Review- oder Apply-Mutation widgettesten

Der Dialog koennte bis `Zur Übernahme freigeben` oder zur Erzeugung einer
Draft-Quote durchlaufen werden.

Bewertung: fachlich wichtig, aber deutlich groesser. Er benoetigt mehrere
Fake-Mutationen, Refreshantworten, Berechtigungen, Fortschrittsdialoge und
Statusuebergaenge. Zuerst sollte der read-only Einstieg stabil abgesichert
werden.

### C: Vollstaendige Importhistorie mit Pagination

Eine eigene Seite koennte mehr als die aktuell sechs geladenen Importlaeufe
anzeigen.

Bewertung: neuer Lade-, Navigations- und API-Umfang. Keine Voraussetzung fuer
den bestehenden Detailpfad.

### D: Importpositionen im Dialog ein- und ausklappen

Der Detaildialog zeigt aktuell maximal fuenf Positionen und einen statischen
Restmengenhinweis. Diese Positionen koennten ebenfalls erreichbar gemacht
werden.

Bewertung: moegliche spaetere Erreichbarkeitsverbesserung, aber eine neue
Runtime-Funktion. Der ungetestete Einstieg in den Dialog ist davor die kleinere
und grundlegendere Luecke.

### E: Cockpit, KPI, SLA oder KI-Auswertung

Importstatus, offene Reviews und Fehler koennten aggregiert oder automatisch
priorisiert werden.

Bewertung: benoetigt neue Fachregeln und ist wesentlich groesser als die
vorhandene Testluecke.

## Entscheidung

Der naechste Ausbau sichert den bestehenden read-only Detailuebergang von der
GAEB-Importvorschau in den Importdialog mit genau einem Widgettest ab.

Der Zielumfang bleibt eng:

- ein konfigurierbarer Importdetail-Datensatz im Fake
- eine konfigurierbare Importpositionsliste im Fake
- genau ein sichtbarer Importlauf mit gesetztem Projektfilter
- `Details` antippen
- Dialogtitel und wenige stabile Metadaten pruefen
- keine Review-, Apply-, Upload- oder Positionsdetailaktion ausloesen

Eine Runtime-Aenderung ist nur dann zulaessig, wenn der Strategieleaf einen
konkreten Defekt im bestehenden read-only Pfad nachweist. Nach aktueller
Inventur ist keine erforderlich.

## Naechster Leaf

Subtask 3.1.57.2 schneidet die technische Teststrategie zu:

- minimale Fake-Felder und Overrides
- kleinster stabile Detailpayload
- eindeutige Finder trotz mehrfacher Texte
- Dialogoeffnung und Schliessen
- gezielter Test- und Analysebefehl

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- kein Review- oder Apply-Workflowtest
- kein Uploadtest
- kein Test einzelner Importposition-Dialoge
- keine vollstaendige Importhistorie oder Pagination
- keine Importfilterung, Sortierung oder Suche
- keine Approval-, Cockpit-, KPI-, SLA- oder KI-Erweiterung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.57.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein enger Widgettest fuer den bestehenden read-only `Details`-Uebergang der
GAEB-Importvorschau. Subtask 3.1.57.2 definiert dafuer das technische
Minimalmodell.
