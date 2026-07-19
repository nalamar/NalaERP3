# GAEB-Nacharbeits-Queue: Inventur des kleinsten Folgeausbaus

## Ziel

Subtask 3.1.49.1 bestimmt nach Abschluss der operativen Nacharbeits-Queue den
kleinsten fachlich sinnvollen Folgeausbau. In diesem Leaf wird keine
Laufzeitlogik implementiert.

## Ausgangslage

Die Queue kann offene Nacharbeit systemweit bzw. projektbezogen auffinden und
zur betroffenen Angebotsposition navigieren. Der Backend-Readmodel
`QuoteApprovalReworkQueueItem` liefert bereits:

- Angebot, Projekt, Kontakt und Position
- Ablehnungsgrund und Entscheidungskommentar
- Entscheider und Entscheidungszeitpunkt
- Preis- und Zielmargen-Snapshots zum Entscheidungszeitpunkt
- aktuellen Positionspreis
- aktuellen Zielmargenstatus
- aktuelle Zielpreis- und Zielabweichungswerte

Die kompakte Client-Sicht verwendet aktuell nur:

- Angebotsnummer
- Position
- Positionsbeschreibung
- `reason_text`, ersatzweise `reason_code`
- Quote-Status zur Wahl zwischen `Bearbeiten` und `Anzeigen`

## Verbleibende Reibung

Der Nutzer erkennt, welche Position nachzuarbeiten ist, aber nicht direkt:

- wer die Ablehnung entschieden hat
- wann die Ablehnung erfolgt ist
- welcher Kommentar zur Entscheidung gehoert
- ob die Position aktuell noch unter Ziel liegt
- wie gross die aktuelle Abweichung zum Zielpreis ist

Diese Informationen sind fuer die Priorisierung und Bearbeitung relevant,
liegen aber bereits im Readmodel vor.

## Optionen

### Option A: Bestehenden Queue-Eintrag um Entscheidungskontext anreichern

Moeglicher Inhalt:

- Entscheider und Zeitpunkt
- Entscheidungskommentar
- aktueller Zielstatus
- aktuelle Abweichung zum Zielpreis

Vorteile:

- rein clientseitig
- kein neuer Endpoint und keine neue Persistenz
- verbessert die Bearbeitungsentscheidung unmittelbar
- bleibt im bestehenden Queue- und Navigationspfad

Risiken:

- der kompakte Queue-Block darf nicht ueberladen werden
- fehlende optionale Werte brauchen klare Fallbacks
- Snapshots und aktuelle Werte duerfen sprachlich nicht vermischt werden

### Option B: Integration in das kommerzielle Workflow-Cockpit

Vorteile:

- Nacharbeit wird neben anderen kommerziellen Folgeaktionen sichtbar

Nachteile:

- vermischt positionsbezogene Approval-Arbeit mit belegbezogenen Folgeaktionen
- braucht Mapping auf ein anderes Readmodel
- vergroessert Navigation und Scope ohne neue Fachdaten

Bewertung:

Nicht der kleinste Folgeausbau.

### Option C: Kennzahlen fuer offene und erledigte Nacharbeit

Vorteile:

- Grundlage fuer Controlling und spaetere Automatisierung

Nachteile:

- braucht Aggregationsregeln, Zeitbezug und fachliche KPI-Definitionen
- hilft der aktuellen Einzelbearbeitung weniger als vorhandener Kontext

Bewertung:

Sinnvoll nach einer ausreichend informativen Arbeitsliste.

### Option D: Priorisierung oder automatische Nacharbeit

Vorteile:

- langfristig relevant fuer KI- und Automationsziele

Nachteile:

- benoetigt belastbare Priorisierungsregeln und Auditierbarkeit
- waere vor transparenter Darstellung der vorhandenen Daten zu frueh

Bewertung:

Kein unmittelbarer Folge-Leaf.

## Entscheidung

Der kleinste naechste Ausbau ist eine rein clientseitige
Kontextanreicherung der vorhandenen Queue-Eintraege.

Prioritaet der darzustellenden Informationen:

1. Entscheidungskommentar, wenn vorhanden
2. Entscheider und Entscheidungszeitpunkt
3. aktueller Zielmargenstatus
4. aktuelle Zielabweichung in Angebotswaehrung, wenn vorhanden

Der Ablehnungsgrund bleibt die primaere Zeile. Snapshotwerte werden in diesem
kleinen Schnitt nicht zusaetzlich angezeigt, damit historische
Entscheidungswerte und aktueller Kalkulationsstand nicht verwechselt werden.

## Technische Abgrenzung

Der naechste Leaf soll zuerst die konkrete Darstellung zuschneiden:

- vorhandene Felder aus `QuoteApprovalReworkQueueItem` wiederverwenden
- vorhandene Datums- und Geldformatierung in `QuotesPage` verwenden
- Statuscodes in kurze deutsche Labels abbilden
- Zeilenanzahl und Fallbacks fuer fehlende optionale Werte festlegen
- keine Backend-, Datenbank- oder Berechtigungsanpassung

Nicht enthalten:

- Workflow-Cockpit-Integration
- Queue-Mutation
- Zuweisung, Faelligkeit oder Eskalation
- KPI-/Reporting-Endpoint
- Sortier- oder Priorisierungsautomatik
- KI-Vorschlag

## Ergebnis

Subtask 3.1.49.1 ist abgeschlossen. Das bestehende Readmodel ist fuer den
naechsten kleinen Ausbau ausreichend. Der Folgeschnitt bleibt client-only und
erhoeht die Entscheidungstransparenz direkt in der Nacharbeits-Queue.
