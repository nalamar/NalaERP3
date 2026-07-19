# GAEB-Freigabe: Folgeinventur nach direkter Queue-Entscheidung

## Ziel

Subtask 3.1.52.1 inventarisiert den naechsten kleinsten Ausbau nach der direkten Entscheidung aus der Freigabeanforderungs-Queue. Diese Subtask aendert keine Laufzeitlogik; sie legt nur die naechste sinnvolle Arbeitskante fest.

## Ausgangslage

- Die Freigabeanforderungs-Queue ist als kompakte, read-only geladene Karte in `QuotesPage` vorhanden.
- Direkte Entscheidungen aus der Queue nutzen die bestehenden positionsnahen Approve-/Reject-Endpunkte.
- Sichtbarkeit und Aktion sind getrennt: Lesen ueber `quotes.read`, Entscheiden nur mit `quotes.approve`.
- Ablehnungen aktualisieren die Nacharbeits-Queue; Genehmigungen aktualisieren die Freigabeanforderungen und die selektierte Quote.
- Die vollstaendige Bearbeitung bleibt weiterhin im Positionseditor moeglich.
- Der bestehende Backend-Vertrag reicht fuer den naechsten kleinen UI-Ausbau aus.

## Offene Luecken

- Der Entscheidungsdialog enthaelt aktuell Kommentar und Aktion, aber keine kompakte Snapshot-Zusammenfassung der Position.
- Die Queue zeigt nur eine kleine Auswahl von Eintraegen; eine dedizierte Arbeitsliste mit Pagination fehlt noch.
- Dashboard und Workflow-Cockpit signalisieren Freigabeanforderungen noch nicht als eigene Folgeaktion.
- SLA, Priorisierung und KPI-Sichten sind noch nicht modelliert.
- Fuer die direkte Queue-Entscheidung gibt es noch keine Widgettests.

## Ausbauoptionen

### A. Snapshot-Zusammenfassung im Entscheidungsdialog

Kleinster fachlicher Ausbau. Der Entscheider sieht vor Genehmigung oder Ablehnung die wesentlichen Kontextwerte direkt im Dialog. Kein neuer Backend-Vertrag, keine neue Route, kein neues Modul. Die bestehende Queue-Position liefert bereits genug Daten fuer einen ersten Kontextblock.

### B. Dedizierte Approval-Worklist

Fachlich sinnvoll fuer groessere Volumina, aber groesserer Schnitt: neue Navigation, Listenlayout, Filter, Pagination und ggf. spaeter Server-Paging. Erst nach stabiler Entscheidungslogik sinnvoll.

### C. Workflow-Cockpit-Signal

Erhoeht Sichtbarkeit auf dem Dashboard, loest aber nicht das Kontextproblem beim Entscheiden. Als Folgeschritt brauchbar, sobald die Entscheidungsoberflaeche stabil ist.

### D. KPI-/SLA-/Priorisierungsmodell

Wertvoll fuer operative Steuerung, benoetigt aber stabile Arbeitsliste und belastbare Statusmetriken. Noch kein kleinster naechster Schritt.

### E. Test-Haertung

Soll beim naechsten Client-Ausbau mitlaufen, liefert allein aber keinen neuen fachlichen Bedienwert. Sinnvoll als Begleitpruefung fuer die Dialogaenderung.

## Entscheidung

Der naechste kleinste Approval-Ausbau ist die Snapshot- und Kontextzusammenfassung im Queue-Entscheidungsdialog.

Begruendung:

- Sie reduziert Fehlentscheidungen direkt im bestehenden Entscheidungsfluss.
- Sie nutzt den vorhandenen Queue-Read-Model-Vertrag.
- Sie bleibt client-only und vermeidet neue Persistenz oder Routing-Oberflaechen.
- Sie bereitet eine spaetere dedizierte Approval-Worklist vor, ohne diese bereits zu bauen.

## Minimalziel fuer die naechste Subtask

Subtask 3.1.52.2 soll ein Zielmodell fuer den erweiterten Entscheidungsdialog schneiden:

- Welche Felder werden im Dialog gezeigt?
- Welche Werte kommen direkt aus dem vorhandenen Queue-Eintrag?
- Welche Werte bleiben bewusst ausserhalb des Dialogs?
- Wie bleibt der Dialog fuer Genehmigen und Ablehnen wiederverwendbar?
- Welche UI-Grenzen gelten fuer kompakte Desktop- und schmale Viewports?

Kandidaten fuer den Kontextblock:

- Angebotsnummer oder Angebotskennung
- Projekt und Kontakt
- Positionsnummer und Kurzbeschreibung
- Freigabegrund
- Anforderer und Anforderungszeitpunkt
- aktueller Einzelpreis
- Kostenbasis-Snapshot
- Zielpreis-Snapshot
- Zielmargen-Snapshot
- Abweichung zum Zielwert
- aktueller Zielstatus

## Nicht-Ziele

- Keine neuen Backend-Endpunkte.
- Keine neue Datenbankmigration.
- Keine dedizierte Approval-Seite.
- Keine Pagination.
- Keine Dashboard- oder KPI-Integration.
- Keine Aenderung am Approve-/Reject-Vertrag.
- Keine KI-/GAEB-Import-Erweiterung in diesem Schnitt.

## Empfohlene Reihenfolge

1. Zielmodell fuer Snapshot- und Kontextzusammenfassung im Queue-Entscheidungsdialog.
2. Client-Implementierung des Kontextblocks in `QuotesPage`.
3. Audit der Dialogaenderung inklusive Berechtigungs-, Refresh- und Fehlerpfad.
4. Danach erneute Inventur: dedizierte Approval-Worklist, Dashboard-Signal oder KPI-/SLA-Modell.

## Ergebnis

3.1.52.1 ist mit dieser Inventur abgeschlossen. Der naechste Leaf ist 3.1.52.2: Zielmodell fuer Snapshot- und Kontextzusammenfassung im Queue-Entscheidungsdialog.
