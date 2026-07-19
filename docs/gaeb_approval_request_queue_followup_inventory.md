# GAEB-Freigabe: Inventur nach abgeschlossener Nacharbeits-Queue

## Ziel

Subtask 3.1.50.1 inventarisiert den naechsten sinnvollen GAEB- und
Freigabe-Folgeabschnitt nach der abgeschlossenen Nacharbeits-Queue.

In diesem Leaf wird keine Laufzeitlogik geaendert. Ziel ist nur, die naechste
kleinste Ausbaustufe fachlich sauber zu bestimmen.

## Ausgangslage

Der Freigabe- und Nacharbeitsstrang deckt inzwischen positionsnah und
queue-seitig viel ab:

- Zielmargenanker und Zielpreisbewertung je Quote-Position
- Freigabeanforderung als persistente Positionseinheit
- Storno, Genehmigung und Ablehnung aktiver Anforderungen
- Entscheidungskommentare
- lesbare Nutzeranzeigenamen in der Historie
- `latest_approval_decision` als positionsnahes Badge
- quote-weite Nacharbeitswarnung mit Positionsbezug
- Prozesssperren bei offener Nacharbeit
- Abschluss erledigter Nacharbeit als `rework_resolved`
- read-only Nacharbeits-Queue fuer zuletzt abgelehnte Positionen
- kompakter Entscheidungskontext direkt im Queue-Eintrag

Damit ist offene Nacharbeit nach einer Ablehnung operativ auffindbar. Die
vorherige Stufe, also offene Freigabeanforderungen mit Status `requested`, ist
aber weiterhin nur sichtbar, wenn ein Nutzer die konkrete Quote oeffnet.

## Fachliche Luecke

Freigebende brauchen vor der Nacharbeit einen Arbeitsvorrat fuer offene
Freigabeentscheidungen:

- Welche Positionen warten aktuell auf Freigabe?
- In welchem Angebot, Projekt und Kundenkontext liegen sie?
- Warum wurde die Freigabe angefordert?
- Wer hat sie angefordert und wann?
- Welche Preis-, Kosten- und Zielmargen-Snapshots liegen der Anfrage zugrunde?
- Welche Positionen sind bereits alte offene Anforderungen und sollten zuerst
  entschieden werden?

Ohne zentrale Sicht bleibt der Prozess asymmetrisch:

- abgelehnte Nacharbeit ist systemweit auffindbar
- offene Freigaben sind nur positionsnah im Quote-Editor auffindbar

Das ist fachlich unguenstig, weil die Freigabeentscheidung vor der
Nacharbeit liegt.

## Optionen

### Option A: Read-only Freigabeanforderungs-Queue

Ziel:

- neue read-only Liste fuer aktive `requested`-Anforderungen
- Quelle: `quote_item_approval_requests.status = 'requested'`
- Kontext: Quote, Projekt, Kontakt, Position, Grund, Kommentar, Anforderer,
  Zeitpunkt, Preis-/Zielmargen-Snapshots und aktueller Zielstatus

Vorteile:

- schliesst die operative Luecke vor der Nacharbeits-Queue
- baut auf vorhandener Persistenz und Nutzeranzeigenamen auf
- bleibt read-only und braucht keine neue Mutation
- bereitet spaetere Entscheidung aus einer Arbeitsliste vor
- kann fachlich klar von `approval-rework` getrennt werden

Risiken:

- braucht ein neues Listen-Readmodel und einen neuen Endpoint
- Berechtigung muss bewusst geschnitten werden
- eine direkte Entscheiden-Aktion aus der Queue waere ein eigener Folgeschritt

Bewertung:

Kleinster sinnvoller Folgeausbau mit hohem operativem Signal.

### Option B: Nacharbeits-Queue und Freigabe-Queue zusammenlegen

Ziel:

- eine gemeinsame Approval-Arbeitsliste fuer `requested` und `rejected`

Vorteile:

- ein Einstiegspunkt fuer Freigebende und Kalkulation
- spaeter gut fuer Priorisierung oder SLA geeignet

Nachteile:

- vermischt zwei unterschiedliche Prozessphasen:
  - offene Entscheidung vor Genehmigung/Ablehnung
  - Nacharbeit nach Ablehnung
- unterschiedliche Aktionen und Berechtigungen
- hoeheres Risiko fuer ein zu breites Readmodel

Bewertung:

Spaeter sinnvoll, aber nicht als naechster kleiner Schnitt.

### Option C: Integration in das kommerzielle Workflow-Cockpit

Ziel:

- offene Freigabeanforderungen als Workflow-Items neben Angebots-,
  Auftrags- und Rechnungsfolgeaktionen anzeigen

Vorteile:

- ein vorhandener Cockpit-Ort koennte genutzt werden
- Freigaben werden als kommerzielle Blocker sichtbar

Nachteile:

- bestehendes Cockpit ist beleg- und Folgebeleg-orientiert
- Freigabeanforderungen sind positionsbezogen
- Feld- und Navigationsbedarf passt eher zu einer Approval-spezifischen Liste

Bewertung:

Erst nach einem stabilen Approval-Queue-Readmodel entscheiden.

### Option D: KPI und Priorisierung

Ziel:

- Kennzahlen zu offenen Freigaben, Alter, Ablehnungsgruenden und
  Zielabweichungen

Vorteile:

- wichtig fuer Steuerung, Controlling und spaetere Automatisierung
- gute Grundlage fuer KI-gestuetzte Verbesserungen

Nachteile:

- braucht zuerst eine belastbare operative Liste
- Priorisierungsregeln koennen fachlich schnell ausufern
- weniger unmittelbarer Nutzen als Auffindbarkeit offener Anforderungen

Bewertung:

Nachgelagert.

### Option E: Entscheidung direkt aus der Nacharbeits- oder Freigabe-Queue

Ziel:

- Genehmigen/Ablehnen direkt aus einer Arbeitsliste

Vorteile:

- sehr effizient fuer Freigebende
- reduziert Wechsel in den Quote-Editor

Nachteile:

- braucht sichere Kontextdarstellung vor der Entscheidung
- Kommentar- und Fehlerpfade muessen erneut fuer Listenaktionen geschnitten
  werden
- Risiko doppelter Mutationspfade

Bewertung:

Nicht der naechste Schritt. Zuerst read-only Queue und Navigation.

## Entscheidung

Der naechste kleinste Folgeausbau ist eine read-only Queue fuer offene
Freigabeanforderungen.

Begruendung:

- Nacharbeit nach Ablehnung ist jetzt auffindbar; offene Freigaben davor noch
  nicht.
- Kommentare, Nutzeranzeigenamen, Badge und Snapshots sind inzwischen
  aussagekraeftig genug fuer eine Arbeitsliste.
- Das bestehende Datenmodell enthaelt aktive Anforderungen bereits eindeutig
  als `status = 'requested'`.
- Eine read-only Queue bleibt kleiner als direkte Entscheidungen,
  Cockpit-Integration oder KPI.
- Die spaetere Entscheidung aus der Queue kann auf demselben Readmodel
  aufbauen, ohne jetzt schon einen zweiten Mutationspfad einzufuehren.

## Minimalziel fuer den naechsten Leaf

Der naechste Leaf sollte nur das technische Minimalzielmodell fuer eine
read-only Freigabeanforderungs-Queue zuschneiden.

Zu entscheiden:

- Endpoint-Form, wahrscheinlich:

```text
GET /api/v1/quotes/approval-requests
```

- Berechtigung:
  - `quotes.approve` fuer Freigebende
  - oder `quotes.read` fuer reine Sichtbarkeit durch Vertrieb/Kalkulation
  - gegebenenfalls bewusst als MVP-Entscheidung dokumentieren
- Filter:
  - `project_id`
  - `contact_id`
  - `quote_id`
  - optional spaeter `requested_by`
- Sortierung:
  - aelteste offene Anforderung zuerst
  - danach Angebotsnummer und Position
- minimale Felder:
  - Quote-Kontext: `quote_id`, `quote_number`, `quote_status`, `quote_date`
  - Projekt/Kontakt: `project_id`, `project_name`, `contact_id`,
    `contact_name`
  - Position: `quote_item_id`, `position`, `description`, `currency`
  - Anfrage: `approval_request_id`, `reason_code`, `reason_text`,
    `requested_by`, `requested_by_name`, `requested_at`
  - Snapshots: aktueller Preis, Kostenbasis, Zielpreis, Zielmarge,
    Zielabweichung, Marge, Preisentscheidungs-ID
  - aktueller Zielstatus und aktuelle Zielabweichung, falls guenstig
    wiederverwendbar
- Navigation:
  - Quote oeffnen
  - betroffene Position im Editor fokussieren
- Abgrenzung zu `GET /api/v1/quotes/approval-rework`:
  - `approval-requests` zeigt offene Entscheidungen
  - `approval-rework` zeigt offene Nacharbeit nach Ablehnung

## Nicht-Ziele des naechsten Leaf

Nicht enthalten:

- Implementierung des Endpoints
- Client-Anbindung
- Genehmigen oder Ablehnen direkt aus der Queue
- Zusammenlegung mit der Nacharbeits-Queue
- Workflow-Cockpit-Integration
- KPI, SLA, Priorisierung oder Eskalation
- Verantwortliche, Faelligkeiten oder Aufgabenmodell
- neue Persistenz
- neue Approval-Statuswerte
- KI-Auswertung oder Automatisierung

## Reihenfolge danach

Empfohlene Reihenfolge:

1. Technisches Minimalzielmodell fuer die Freigabeanforderungs-Queue
   zuschneiden.
2. Backend-Readmodel und Endpoint implementieren.
3. Integrationstests fuer offene, entschiedene, stornierte und historische
   Anforderungen.
4. Minimale Client-API und Queue-Sicht mit Navigation zur Quote-Position.
5. Audit des Queue-Blocks.
6. Danach neu entscheiden zwischen:
   - Entscheidung direkt aus der Queue
   - Cockpit-Integration
   - gemeinsame Approval-Arbeitsliste
   - KPI/Priorisierung
   - Verantwortlichkeit und Eskalation

## Ergebnis

Subtask 3.1.50.1 ist abgeschlossen. Der naechste sinnvolle Ausbau nach der
Nacharbeits-Queue ist eine getrennte read-only Queue fuer aktive
Freigabeanforderungen. Sie schliesst die operative Luecke vor der Ablehnung,
bleibt kleiner als ein Cockpit- oder KPI-Ausbau und fuehrt noch keinen zweiten
Entscheidungspfad ein.
