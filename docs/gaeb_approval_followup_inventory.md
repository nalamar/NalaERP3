# GAEB-Freigabe: Folgeinventur nach Freigabeanforderungs-Queue

## Ziel

Subtask 3.1.51.1 inventarisiert den naechsten kleinsten Approval-Ausbau nach
der abgeschlossenen read-only Freigabeanforderungs-Queue.

In diesem Leaf wird keine Laufzeitlogik geaendert. Ziel ist nur, den naechsten
fachlich und technisch kleinsten Schnitt festzulegen.

## Ausgangslage

Der GAEB-/Approval-Strang hat inzwischen einen durchgaengigen operativen
MVP-Fluss:

- Preis-, Kosten- und Zielmargenbewertung je Quote-Position
- Freigabehinweis je Position
- persistente Freigabeanforderung mit Snapshots
- Storno aktiver Anforderungen
- Genehmigung und Ablehnung mit Kommentar
- positionsnahe Historie
- `active_approval_request` im Quote-Detail
- `latest_approval_decision` als Badge
- Prozesssperre bei aktiver Freigabeanforderung
- Nacharbeitsstatus nach Ablehnung
- Abschluss erledigter Nacharbeit als `rework_resolved`
- read-only Nacharbeits-Queue
- read-only Freigabeanforderungs-Queue

Damit sind beide operativen Phasen auffindbar:

- offene Entscheidungen vor Genehmigung oder Ablehnung
- offene Nacharbeit nach Ablehnung

Die eigentliche Entscheidung passiert weiterhin im Quote-Editor auf der
betroffenen Position.

## Aktuelle Luecke

Die neue Freigabeanforderungs-Queue macht offene Entscheidungen sichtbar, aber
Freigebende muessen fuer jede Entscheidung weiterhin:

1. Queue-Eintrag oeffnen
2. Quote laden
3. Position fokussieren
4. Entscheidung im Editor ausfuehren
5. zur Queue zurueckkehren

Das ist fuer den ersten MVP korrekt, weil der vollstaendige Kontext im Editor
liegt. Nach stabiler Queue wird aber die naechste operative Reibung sichtbar:

- wiederholte Kontextwechsel bei vielen offenen Freigaben
- keine kompakte Entscheidungsansicht fuer Freigebende
- kein klarer Ort fuer schnelle Genehmigung oder Ablehnung aus einer
  Arbeitsliste
- keine dedizierte Fehler- und Refresh-Strategie nach Queue-Entscheidungen

## Optionen

### Option A: Direkte Entscheidung aus einer Freigabe-Queue vorbereiten

Ziel:

- Genehmigen und Ablehnen aus einer dedizierten Queue-Sicht heraus ermoeglichen
- vorhandene Endpunkte weiterverwenden:
  - `POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve`
  - `POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject`
- Kommentar-Dialog und Fehlerpfade aus dem Quote-Editor fachlich spiegeln
- Queue nach erfolgreicher Entscheidung aktualisieren

Vorteile:

- schliesst die groesste verbleibende operative Luecke fuer Freigebende
- nutzt bestehende Backend-Mutationen und Berechtigungen
- braucht keine neue Persistenz
- kann auf dem vorhandenen Queue-Readmodel aufbauen
- trennt offene Entscheidung weiterhin von Nacharbeit

Risiken:

- Entscheidung braucht ausreichend Kontext direkt in der Queue
- Kommentar-Dialog und Ladezustand duerfen nicht doppelte Semantik erzeugen
- Ablehnung erzeugt Nacharbeit; die UI muss nach Ablehnung die Verschiebung in
  die Nacharbeits-Queue klar machen
- die kompakte Drei-Zeilen-Karte in `QuotesPage` ist nicht zwingend der beste
  Ort fuer Mutationen

Bewertung:

Fachlich naechster kleinster Ausbau, aber zuerst als Zielmodell schneiden. Die
Implementierung sollte erst danach erfolgen.

### Option B: Approval-Queues in das kommerzielle Workflow-Cockpit integrieren

Ziel:

- offene Freigaben und offene Nacharbeit als Workflow-Items in
  `GET /api/v1/workflow/commercial` aufnehmen

Vorteile:

- ein zentraler Dashboard-Ort fuer kommerzielle Blocker
- vorhandene Workflow-Karte im Dashboard kann genutzt werden
- Projekt- und Kontaktfilter existieren bereits

Nachteile:

- das vorhandene Cockpit ist aktuell beleg- und Folgebeleg-orientiert
- Approval ist positionsbezogen und braucht andere Felder
- Berechtigungen unterscheiden sich:
  - Sichtbarkeit ueber `quotes.read`
  - Entscheidung ueber `quotes.approve`
- Risiko, die noch junge Approval-Queue zu frueh in ein breiteres Cockpit zu
  ziehen

Bewertung:

Sinnvoll spaeter, aber nicht der naechste kleinste Schritt. Erst muss klar
sein, ob Approval als eigene Arbeitsliste oder nur als Cockpit-Signal gefuehrt
wird.

### Option C: Gemeinsame Approval-Arbeitsliste fuer Freigabe und Nacharbeit

Ziel:

- `requested` und `rejected` in einer gemeinsamen Approval-Liste anzeigen

Vorteile:

- ein Einstieg fuer Freigebende und Kalkulation
- spaeter gut fuer Priorisierung, SLA und Verantwortlichkeiten
- vermeidet zwei getrennte Karten in der Angebotsseite

Nachteile:

- vermischt zwei unterschiedliche Prozessphasen
- unterschiedliche primaere Aktionen:
  - `requested`: genehmigen oder ablehnen
  - `rejected`: nacharbeiten und erneut anfordern
- hoeheres Readmodel-Risiko
- Navigations- und Berechtigungsschnitt wird breiter

Bewertung:

Architektonisch plausibel, aber fuer den naechsten Leaf zu breit.

### Option D: KPI, Priorisierung und SLA

Ziel:

- Kennzahlen und Prioritaeten fuer offene Freigaben und Nacharbeit schaffen

Vorteile:

- wichtig fuer Steuerung und spaetere Automatisierung
- gute Grundlage fuer Management-Sichten
- kann Zielabweichung, Alter, Kunde und Projektwert kombinieren

Nachteile:

- braucht zuerst stabile Arbeitslisten und Entscheidungsablaeufe
- Priorisierungsregeln koennen fachlich schnell ausufern
- loest nicht direkt die aktuelle Entscheidungsreibung

Bewertung:

Nachgelagert.

### Option E: Testhaertung ohne neue Funktion

Ziel:

- Sortierung, Berechtigung und Namensfallbacks der Queue isoliert absichern

Vorteile:

- reduziert Regressionrisiko
- sehr kleiner technischer Schnitt
- keine neue UI-Komplexitaet

Nachteile:

- schafft keinen neuen operativen Nutzen
- die wichtigsten End-to-End-Faelle sind bereits abgedeckt
- Tests koennen beim naechsten Funktionsausbau sinnvoller mitgeschrieben
  werden

Bewertung:

Guter Begleitanteil fuer den naechsten Implementierungsleaf, aber nicht als
alleiniger Folgeausbau.

## Entscheidung

Der naechste kleinste Approval-Ausbau ist:

```text
Direkte Entscheidung aus der Freigabeanforderungs-Queue vorbereiten.
```

Wichtig: Der naechste Leaf soll noch keine Runtime-Implementierung sein,
sondern zuerst das Zielmodell fuer diesen Mutationsschnitt dokumentieren.

Begruendung:

- Die read-only Queue hat die Auffindbarkeit geloest.
- Die vorhandenen Mutationsendpunkte fuer Genehmigen und Ablehnen existieren
  bereits und sind positionsbezogen abgesichert.
- Der Quote-Editor enthaelt die passende Logik fuer Kommentar, Ladezustand und
  Fehleranzeige als Vorbild.
- Eine direkte Entscheidung aus der Queue ist operativ wirksamer als
  Cockpit-Integration oder KPI.
- Ein Zielmodell verhindert, dass Mutationen vorschnell in die kompakte
  Drei-Eintrags-Karte eingebaut werden.

## Minimalziel fuer den naechsten Leaf

Subtask 3.1.51.2 sollte ein Zielmodell fuer Queue-Entscheidungen erstellen.

Zu klaeren:

- Ort der Aktion:
  - bestehende kompakte `QuotesPage`-Karte
  - oder dedizierte Approval-Arbeitsliste
- Mindestkontext vor Entscheidung:
  - Quote, Projekt, Kontakt
  - Position und Beschreibung
  - aktueller Preis, Kostenbasis, Zielpreis, Zielabweichung
  - Anfrage-Snapshots
  - aktueller Zielstatus
  - Antragsteller und Zeitpunkt
- Berechtigung:
  - Sichtbarkeit weiter `quotes.read`
  - Aktionen nur bei `quotes.approve`
- Mutationsvertrag:
  - bestehende approve/reject-Endpunkte verwenden
  - Kommentar optional, maximal 500 Zeichen nach bestehendem Vertrag
  - nach Erfolg Queue aktualisieren
  - nach Ablehnung Eintrag aus Freigabe-Queue entfernen und Nacharbeit
    sichtbar machen
- Fehler- und Race-Handling:
  - bereits entschiedene Anforderung
  - fehlende Berechtigung
  - Quote/Position nicht mehr aktuell
  - parallele Entscheidung durch andere Nutzer
- Teststrategie:
  - Client-API braucht wahrscheinlich keine neue Methode
  - Widget- oder manuelle UI-Verifikation fuer Queue-Aktionszustand
  - Backend-Integrationstests nur ergaenzen, wenn der Serververtrag erweitert
    wird

## Nicht-Ziele des naechsten Leaf

Nicht enthalten:

- direkte Implementierung der Queue-Aktionen
- neue Backend-Endpunkte
- neue Tabellen oder Migrationen
- Zusammenlegung mit Nacharbeits-Queue
- Workflow-Cockpit-Integration
- KPI, SLA oder Priorisierung
- Verantwortliche oder Eskalation
- KI-Auswertung von Freigabegruenden

## Reihenfolge danach

Empfohlene Reihenfolge:

1. Zielmodell fuer direkte Queue-Entscheidungen schneiden.
2. Kleinsten UI-Ort fuer Aktionen festlegen.
3. Implementierung nur fuer Genehmigen/Ablehnen mit bestehendem Backendvertrag.
4. Nach Erfolgs-/Fehlerpfaden auditieren.
5. Danach neu entscheiden zwischen:
   - dedizierter Approval-Arbeitsliste,
   - Workflow-Cockpit-Signal,
   - gemeinsamer Approval-/Nacharbeitsliste,
   - KPI/SLA/Priorisierung.

## Ergebnis

Subtask 3.1.51.1 ist abgeschlossen. Der naechste sinnvolle Ausbau nach der
read-only Freigabeanforderungs-Queue ist nicht sofort Cockpit oder KPI,
sondern ein enger Zielmodell-Leaf fuer direkte Genehmigung und Ablehnung aus
einer Queue-Sicht auf Basis der bestehenden positionsnahen Mutationsendpunkte.
