# GAEB-Freigabe: Folgeinventur nach dem Queue-Entscheidungsdialog-Kontext

## Ziel

Subtask 3.1.53.1 bestimmt nach dem abgeschlossenen Kontextblock im
Queue-Entscheidungsdialog den kleinsten sinnvollen Folgeausbau. Dieser Leaf
aendert keine Laufzeitlogik.

## Ausgangslage

Der Approval-Strang bietet bereits globale Freigabeanforderungs- und
Nacharbeits-Queues, Navigation zur Position, direkte Einzelentscheidungen und
einen optionalen Kontextblock im gemeinsamen Entscheidungsdialog. Formatierung
und statische Analyse sind gruen. Ein Widgettest fuer Queue-Aktion,
Kontextanzeige und Kommentarweitergabe fehlt jedoch.

## Optionen

### A: Dedizierte Approval-Arbeitsliste

Die vorhandene `Freigabeanforderungen`-Karte ist bereits eine operative
Arbeitsliste mit Kontext, Navigation und Einzelentscheidungen. Eine neue Seite
liefert erst bei Pagination, Massenaktionen oder komplexeren Filtern klaren
Zusatznutzen.

Bewertung: spaeter bei nachgewiesenem Mengendruck.

### B: Widget-Testhaertung

Der Testbestand besitzt bereits eine `QuotesPage`-Widgetteststruktur und
Fake-API, deckt die Approval-Queue aber noch nicht ab. Ein enger Test kann
absichern, dass der Queue-Eintrag den Dialog oeffnet, Kontext und `0`-Werte
sichtbar bleiben und ein Kommentar den bestehenden API-Aufruf erreicht.

Bewertung: kleinster, vertragserhaltender Ausbau mit direktem Nutzen.

### C: Workflow-Cockpit-Signal

Das vorhandene Cockpit ist beleg- und Folgebeleg-orientiert. Positionsbezogene
Approval-Anforderungen benoetigen eine Aggregationsentscheidung und einen
erweiterten Workflow-Vertrag.

Bewertung: erst nach stabiler UI-Testbasis und eigenem Zielmodell.

### D: KPI, SLA und Priorisierung

Alter, Zielabweichung und offene Anzahl koennen spaeter priorisieren. Es fehlen
aber fachlich vereinbarte SLA-Grenzen, Verantwortlichkeit und Eskalationsregeln.

Bewertung: nachgelagert zu Cockpit-/Aufgabenmodell und SLA-Definition.

## Entscheidung

Als naechstes wird der Queue-Entscheidungsdialog mit einem engen Widgettest
gehaertet:

- Das schliesst das im Abschluss-Audit konkret benannte Restrisiko.
- Vorhandene UI- und API-Vertraege bleiben unveraendert.
- Berechtigung, Kontext, Kommentar und Mutation werden vor weiteren
  Verbrauchern gemeinsam abgesichert.
- Die vorhandene `QuotesPage`-Teststruktur haelt den Schnitt klein.

## Naechster Leaf

Subtask 3.1.53.2 schneidet ausschliesslich die minimale Widget-Teststrategie zu:

- notwendige Fake-API-Erweiterung fuer Queue-Daten und Mutation
- repraesentativer Genehmigen- oder Ablehnen-Pfad
- stabile Texte, Snapshotwerte und `0`-Wert
- Bereitstellung von `quotes.approve` im Testkontext
- gezielte Test- und Analysebefehle

## Nicht-Ziele

- noch keine Testimplementierung oder Runtime-Aenderung
- keine neue Approval-Seite oder Massenaktion
- keine Backend-, API- oder Persistenz-Erweiterung
- keine Cockpit-, KPI-, SLA-, Eskalations- oder Aufgabenlogik

## Ergebnis

3.1.53.1 ist abgeschlossen. Der naechste Leaf ist 3.1.53.2: minimales
Widget-Testzielmodell fuer den bestehenden Queue-Entscheidungsdialog festlegen.
