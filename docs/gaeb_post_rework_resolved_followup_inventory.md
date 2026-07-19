# GAEB-Folgeausbau nach erledigter Freigabe-Nacharbeit: Inventur

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeabschnitt nach dem
abgeschlossenen `rework_resolved`-Durchstich.

Der Fokus bleibt bewusst eng:

- Workflow-Uebersicht, Auswertbarkeit und spaetere Automatisierung fachlich
  vergleichen
- offene Nacharbeit und erledigte Nacharbeit sauber unterscheiden
- den kleinsten naechsten Ausbau mit gutem Signal bestimmen
- noch keine neue Laufzeitlogik oder UI implementieren

## 1. Ausgangslage

Der aktuelle Stand deckt den positionsbezogenen Freigabe- und
Nacharbeitszyklus ab:

- Zielmargenanker und Preisentscheidungen je Quote-Position
- Freigabeanforderung fuer unterzielige Positionen
- Genehmigung und Ablehnung mit Kommentar
- Historie je Position
- letzte terminale Entscheidung als `latest_approval_decision`
- offene Nacharbeit, wenn die neueste terminale Entscheidung `rejected` ist
- expliziter Abschluss erledigter Nacharbeit als `rework_resolved`
- Prozesssperren fuer Versand, Annahme und Folgebelege nur bei offener
  Nacharbeit
- minimale Client-Aktion `Nacharbeit abschliessen`, wenn die Zielmarge aktuell
  erreicht oder uebertroffen ist

Damit ist der lokale Positionszyklus fachlich geschlossen. Die offenen Fragen
verschieben sich weg von "Wie wird eine Position entsperrt?" hin zu "Wie wird
Nacharbeit operativ gefunden, verfolgt und spaeter automatisiert?".

## 2. Bestehende Sichtbarkeit

### 2.1 Quote-Detail und Quote-Editor

Die Detailansicht und der Editor zeigen aktuell:

- quote-weite Warnung fuer offene Nacharbeit
- betroffene Positionsnummern
- Sprung/Fokus in den Editor
- letzte terminale Entscheidung je Position
- `rework_resolved` als positiver terminaler Zustand

Diese Sicht ist stark, wenn ein Nutzer bereits in der richtigen Quote ist. Sie
ist schwach fuer uebergreifende Arbeit ueber mehrere Angebote.

### 2.2 Kommerzielles Workflow-Cockpit

Das vorhandene Workflow-Cockpit ist auf offene Folgeaktionen zwischen Angebot,
Auftrag und Rechnung zugeschnitten:

- `quote_sent_pending`
- `quote_accepted_pending_followup`
- `sales_order_pending_invoice`
- `sales_order_partially_invoiced`

Es ist bewusst keine Approval- oder Nacharbeitsliste. Offene Nacharbeit ist
aber operativ ebenfalls eine Sperre vor Versand, Annahme und Folgebelegen.

### 2.3 Approval-Historie

Die Historie ist positionsnah und auditierbar, aber keine Arbeitsliste. Sie
beantwortet "Was ist passiert?", nicht "Was muss ich als Naechstes tun?".

## 3. Fachliche Luecken nach `rework_resolved`

### 3.1 Offene Nacharbeit systemweit finden

Aktuell sieht man offene Nacharbeit nur ueber die konkrete Quote. Fuer
Freigebende, Kalkulation oder Vertrieb fehlt eine Liste aller blockierenden
Nacharbeitspositionen.

Fachlicher Nutzen:

- Arbeitsvorrat fuer Nacharbeit
- schnellere Entsperrung von Angeboten
- priorisierbare Blocker vor Versand oder Folgebeleg

### 3.2 Erledigte Nacharbeit auswerten

`rework_resolved` ist auditierbar gespeichert, aber noch nicht als Kennzahl oder
Liste auswertbar.

Fachlicher Nutzen:

- wie oft Positionen nachgearbeitet werden
- welche Gruende haeufig zu Ablehnungen fuehren
- wie oft Zielmargen erst nach Korrektur erreicht werden
- Grundlage fuer spaetere Pricing- oder Materialverbesserungen

### 3.3 Prozessstatus zwischen Approval und kommerziellem Workflow

Offene Nacharbeit ist faktisch ein kommerzieller Blocker, aber fachlich eine
Approval-/Kalkulationsaufgabe. Ohne klare Einordnung droht doppelte Sicht:

- einmal als Approval-Queue
- einmal als Workflow-Cockpit-Item

Der naechste Ausbau muss entscheiden, ob Nacharbeit als eigener Workflow-Kind
in das bestehende Cockpit gehoert oder zuerst als Approval-spezifische Liste
geschnitten wird.

### 3.4 Spaetere Automatisierung

Automatisierung wird erst sinnvoll, wenn die Daten sichtbar und vergleichbar
sind. Beispiele:

- automatische Priorisierung nach Angebotswert, Alter oder Zielabweichung
- Vorschlag "Zielpreis uebernehmen und Nacharbeit abschliessen"
- KI-Auswertung von Ablehnungsgruenden
- spaetere KI-gestuetzte GAEB-Angebotsverbesserung

Diese Schritte brauchen zuerst ein belastbares Readmodel fuer offene und
erledigte Nacharbeit.

## 4. Optionenvergleich

### Option A: Nacharbeits-Queue

Ziel:

- neue read-only Liste fuer offene Nacharbeit ueber alle Quotes
- Quelle: neueste terminale Positionsentscheidung `rejected`
- Kontext: Quote, Position, Projekt, Kontakt, Grund, Kommentar,
  Zielmargenabweichung, aktueller Zielmargenstatus

Vorteile:

- direkt operativ nutzbar
- macht blockierende Nacharbeit systemweit sichtbar
- klare Trennung zu erledigter Nacharbeit
- baut auf vorhandenen Readmodels auf

Nachteile:

- braucht neuen Endpoint oder Erweiterung eines bestehenden Workflow-Endpoints
- braucht Client-Sicht oder Dashboard-Block
- muss Navigation zur Quote-Position sauber loesen

Bewertung:

Hoechstes operatives Signal, aber etwas groesser als eine reine Auswertung.

### Option B: Rework-Auswertung

Ziel:

- read-only Auswertung oder Liste erledigter und offener Nacharbeitsfaelle
- Fokus auf Kennzahlen und Analyse statt Tagesarbeit

Vorteile:

- nutzt `rework_resolved` sofort als Auditquelle
- gute Grundlage fuer spaetere Automatisierung
- kann ohne neue Mutation starten

Nachteile:

- weniger unmittelbarer Nutzen fuer laufende Angebote
- KPI-Scope kann schnell ausufern
- ohne offene Queue bleiben operative Blocker schwer auffindbar

Bewertung:

Sinnvoll, aber nachrangig gegenueber dem Finden offener Nacharbeit.

### Option C: Erweiterung des kommerziellen Workflow-Cockpits

Ziel:

- neues Workflow-Item, z. B. `quote_approval_rework_pending`
- offene Nacharbeit erscheint neben `quote_sent_pending` und
  `quote_accepted_pending_followup`

Vorteile:

- nutzt bestehendes Cockpit-Konzept
- Nacharbeit wird als kommerzieller Blocker sichtbar
- keine separate neue Navigationsflaeche zwingend noetig, falls Cockpit schon
  genutzt wird

Nachteile:

- vermischt Approval-Arbeit mit Folgebeleg-Arbeit
- bestehendes Cockpit ist dokumentiert als Folgeaktionen zwischen Angebot,
  Auftrag und Rechnung
- Feldbedarf je Position passt nicht sauber zum bisherigen belegzentrierten
  WorkflowItem

Bewertung:

Attraktiv, aber fachlich riskanter als eine schmale Approval-/Nacharbeitsliste.

### Option D: Automatisierung direkt starten

Ziel:

- automatische Vorschlaege oder automatische Abschluesse nach erreichter
  Zielmarge

Vorteile:

- langfristig wertvoll
- passt zur spaeteren KI-/GAEB-Zielrichtung

Nachteile:

- zu frueh ohne systemweite Sicht und Metriken
- hohes Risiko fuer falsche Prozessentscheidungen
- braucht klare Audit- und Override-Regeln

Bewertung:

Nicht der naechste Schritt.

## 5. Entscheidung

Der kleinste sinnvolle Folgeausbau ist zuerst eine fachlich schmale
Nacharbeits-Queue fuer offene Nacharbeit.

Begruendung:

- Offene Nacharbeit ist der aktuelle operative Engpass.
- Der lokale Positionsfluss ist geloest; jetzt fehlt die systemweite
  Auffindbarkeit.
- `rework_resolved` sorgt bereits dafuer, dass erledigte Faelle aus der offenen
  Queue verschwinden koennen.
- Eine Queue erzeugt ein sauberes Readmodel, auf dem spaeter Auswertung,
  Priorisierung und Automatisierung aufbauen koennen.
- Eine direkte Cockpit-Erweiterung waere moeglich, sollte aber erst nach einem
  klaren Nacharbeits-Readmodel entschieden werden.

## 6. Minimalziel fuer den naechsten Leaf

Der naechste Leaf sollte nur das technische Zielmodell fuer eine read-only
Nacharbeits-Queue zuschneiden.

Zu entscheiden:

- Endpoint-Form, z. B. `GET /api/v1/quotes/approval-rework`
- Berechtigung, vermutlich `quotes.read` plus spaeter optional
  `quotes.approve`
- minimale Felder:
  - `quote_id`, `quote_number`, `quote_status`
  - `quote_item_id`, `position`, `description`
  - `project_id`, `project_name`
  - `contact_id`, `contact_name`
  - `reason_code`, `reason_text`, `decision_comment`
  - `decided_by`, `decided_by_name`, `decided_at`
  - `current_unit_price_snapshot`, `target_unit_price_snapshot`,
    `target_difference_snapshot`, `margin_percent_snapshot`
  - aktueller Zielmargenstatus, falls guenstig ableitbar
- Sortierung: zuerst aelteste offene Nacharbeit oder hoechste Abweichung?
- Filter: `project_id`, `contact_id`, optional `quote_id`
- Navigation: Quote oeffnen und Position fokussieren

Nicht im naechsten Leaf:

- Implementierung des Endpoints
- Client-Seite
- KPI-/Reporting-Ansicht
- automatische Abschluesse
- neue Persistenz
- Erweiterung des kommerziellen Workflow-Cockpits

## 7. Reihenfolge danach

Empfohlene Reihenfolge:

1. Zielmodell fuer read-only Nacharbeits-Queue zuschneiden
2. Backend-Readmodel und Endpoint implementieren
3. Integrationstests fuer offene und erledigte Nacharbeit
4. minimale Client-Sicht oder Cockpit-Block mit Navigation zur Quote-Position
5. Audit des Queue-Blocks
6. Danach neu entscheiden zwischen:
   - Integration in das kommerzielle Workflow-Cockpit
   - Auswertung/Kennzahlen fuer erledigte Nacharbeit
   - Priorisierung und spaetere Automatisierung

Die Queue ist der stabilste naechste Anker, weil sie offene Blocker sichtbar
macht und erledigte Nacharbeit durch `rework_resolved` automatisch aus der
Arbeitsliste herausfaellt.
