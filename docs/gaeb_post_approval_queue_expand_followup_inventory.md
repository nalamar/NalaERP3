# GAEB-Freigabe: Folgeinventur nach der erweiterbaren Approval-Queue

## Ziel

Subtask 3.1.55.1 bestimmt den kleinsten fachlich sinnvollen Folgeausbau nach
der abgeschlossenen, erreichbaren und widgetgetesteten
Freigabeanforderungs-Queue. Dieser Leaf aendert keine Laufzeitlogik.

## Ausgangslage

Die Freigabeanforderungs-Karte kann inzwischen:

- offene Anforderungen vollstaendig laden
- standardmaessig drei Eintraege kompakt anzeigen
- alle geladenen Eintraege ueber `Alle anzeigen` erreichbar machen
- wieder auf `Weniger anzeigen` einklappen
- Genehmigen, Ablehnen und Oeffnen unveraendert anbieten
- Dialog- und Expand-Vertrag durch Widgettests absichern

Direkt darunter verwendet die bestehende Nacharbeits-Queue weiterhin das alte
Darstellungsmuster:

- `_approvalReworkItems` enthaelt die bereits geladene Gesamtliste.
- `visibleApprovalReworkItems` verwendet fest `take(3)`.
- weitere Positionen erscheinen nur als statischer Text
  `+n weitere offene Positionen`.
- der Restmengenhinweis ist nicht interaktiv.

Damit besteht dieselbe Erreichbarkeitsluecke fuer abgelehnte Positionen, die
bei der Freigabeanforderungs-Queue gerade geschlossen wurde.

## Optionen

### A: Ein- und Ausklappen der bestehenden Nacharbeits-Queue

Die Nacharbeits-Karte erhaelt einen eigenen lokalen Anzeigezustand und dieselben
stabilen Labels `Alle anzeigen`/`Weniger anzeigen`.

Vorteile:

- schliesst eine konkret vorhandene Erreichbarkeitsluecke
- rein clientseitig
- bestehendes Rework-Readmodel reicht aus
- Reihenfolge, Kontext und `Bearbeiten`/`Anzeigen` bleiben unveraendert
- bereits bewaehrtes Zustands- und Testmuster kann eng wiederverwendet werden

Bewertung: kleinster Folgeausbau mit direktem Nutzen.

### B: Gemeinsame generische Queue-Komponente

Approval- und Rework-Karte koennten in ein allgemeines Widget abstrahiert
werden.

Bewertung: Beide Karten unterscheiden sich in Kontext, Aktionen, Farben und
Mutation. Ein Refactoring vergroessert den Scope ohne zusaetzlichen Fachnutzen
fuer den aktuellen Leaf.

### C: Dedizierte kombinierte Approval-Arbeitsliste

Eine eigene Seite koennte offene Entscheidungen und offene Nacharbeit gemeinsam
mit Pagination, Filtern und Sortierung darstellen.

Bewertung: langfristig sinnvoll bei Mengendruck, aktuell aber ein neuer
UI-/API-Block und deutlich groesser als die lokale Erreichbarkeitsluecke.

### D: Serverseitige Pagination

Die Approval- und Rework-Endpunkte koennten `limit`, `offset` und Gesamtanzahl
erhalten.

Bewertung: fuer grosse Queues spaeter relevant. Beide Gesamtdatenmengen werden
heute bereits geladen; Pagination ist deshalb keine Voraussetzung fuer den
kleinen Client-Schritt.

### E: Cockpit, KPI, SLA oder Priorisierung

Nacharbeitsalter, Abweichung und Verantwortlichkeit koennten spaeter fuer
Eskalation oder Reporting verwendet werden.

Bewertung: benoetigt neue fachliche Regeln und bleibt ausserhalb des
read-only Navigationsvertrags der Rework-Queue.

## Entscheidung

Der naechste Ausbau ist die clientseitige Ein-/Ausklappfunktion fuer die
bestehende Nacharbeits-Queue.

Das Ziel bleibt bewusst getrennt von der Approval-Karte:

- eigener lokaler Rework-Expand-State
- kompakt weiterhin maximal drei Positionen
- `Alle anzeigen` macht alle bereits geladenen `_approvalReworkItems` sichtbar
- `Weniger anzeigen` stellt die kompakte Ansicht wieder her
- bestehender Kontext und `Bearbeiten`/`Anzeigen` bleiben unveraendert

Ein gemeinsamer Zustand waere fachlich falsch: Der Nutzer soll Freigaben und
Nacharbeiten unabhaengig ein- oder ausklappen koennen.

## Naechster Leaf

Subtask 3.1.55.2 schneidet das technische Minimalzielmodell zu:

- eigener lokaler Zustand fuer die Nacharbeits-Karte
- Ableitung von `visibleApprovalReworkItems`
- Ruecksetz- und Refreshverhalten
- Position und Labels der Umschaltaktion
- minimale Fake-API-Erweiterung und Widgettestgrenze

## Nicht-Ziele

- noch keine Implementierung
- kein Refactoring in eine generische Queue-Komponente
- keine kombinierte oder dedizierte Arbeitsliste
- keine Pagination, neuen Filter oder Sortierung
- keine Queue-Mutation oder Massenaktion
- keine Cockpit-, KPI-, SLA-, Eskalations- oder KI-Logik
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.55.1 ist abgeschlossen. Der kleinste Folgeausbau ist die eigenstaendige
clientseitige Ein-/Ausklappfunktion der vorhandenen Nacharbeits-Queue. Der
naechste Leaf 3.1.55.2 definiert dafuer das technische Minimalzielmodell.
