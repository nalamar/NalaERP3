# GAEB-Folgeausbau nach expliziter Preisentscheidung:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen expliziten Preisuebernahme aus der primaeren priorisierten
Quelle.

Der Fokus bleibt bewusst eng:

- den kleinsten kommerziellen Folgepunkt nach der expliziten Preisentscheidung
  bestimmen
- entscheiden, ob jetzt ein kalkulationsnaher Margen-/Zuschlagsanker,
  Preisentscheidungs-Transparenz oder ein anderer minimaler Schritt den
  besten Signalwert hat
- Preisentscheidung, Kalkulation, Freigabe und Automatik weiterhin sauber
  trennen

## 1. Ausgangslage nach abgeschlossener Preisentscheidung

Der aktuelle Stand deckt bereits ab:

- GAEB-Position wird kontrolliert in eine Draft-Quote uebernommen
- Quote-Position kann manuell mit einem Material verbunden werden
- Materialsuche und Materialuebernahme sind positionsnah moeglich
- Preisvorschlag und Preisquellen sind sichtbar
- Preisquellen werden priorisiert
- aktuelle Position kann gegen die primaere Quelle bewertet werden
- Nutzer kann die primaere Quelle explizit als Positionspreis uebernehmen

Damit ist erstmals ein kompletter kleiner Entscheidungsfluss vorhanden:

- Kostenanker finden
- Kostenanker bewerten
- Kostenanker bewusst uebernehmen

## 2. Verbleibende fachliche Luecke

Nach der expliziten Uebernahme bleibt eine neue Luecke sichtbar:

- die Quote-Position enthaelt den aktualisierten `unit_price`
- `price_mapping_status` zeigt nur den manuellen Charakter
- die konkrete Entscheidung selbst ist nicht als eigener fachlicher Kontext
  sichtbar

Damit ist im Editor zwar der aktuelle Preis vorhanden, aber nicht explizit
nachvollziehbar:

- welche Quelle zuletzt bewusst uebernommen wurde
- ob der aktuelle Preis noch der primaeren Quelle entspricht
- ob der Preis seit der Uebernahme wieder manuell geaendert wurde
- welche Abweichung seit der letzten Entscheidung entstanden ist

Diese Luecke ist kleiner und naheliegender als eine vollwertige Kalkulation.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Margenberechnung
- Zuschlags- oder Gemeinkostenlogik
- Rabattlogik
- Zieldeckungsbeitraege
- Freigabe- oder Eskalationsworkflow
- Bulk-Kalkulation ueber mehrere Positionen
- automatische Preisentscheidung
- KI-gestuetzte Preisoptimierung

Diese Themen setzen voraus, dass die Preisbasis und die letzte bewusste
Entscheidung nachvollziehbar genug sind.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: kalkulationsnaher Margen-/Zuschlagsanker

Vorteile:

- waere fachlich der direkte Schritt von Kostenbasis zu Verkaufspreislogik
- bereitet spaetere Angebotskalkulation und Deckungsbeitrag vor
- passt langfristig zum Metallbau-ERP-Zielbild

Nachteile:

- aktuell fehlt noch ein eigener, stabiler Entscheidungs- oder Kostenbasis-
  Kontext an der Position
- `unit_price` ist weiterhin der operative Positionspreis und nicht sauber
  als Kostenbasis, Verkaufspreis oder Kalkulationsbasis getrennt
- ein frueher Margenanker wuerde schnell neue Felder, Regeln und UI-Zustaende
  erzwingen

### Option B: Preisentscheidungs-Transparenz

Vorteile:

- baut direkt auf der gerade umgesetzten expliziten Entscheidung auf
- bleibt positionsnah und klein
- macht sichtbar, ob der aktuelle Preis zur primaeren Quelle und zur letzten
  Entscheidung passt
- bereitet Margen-, Zuschlags- und Freigabelogik fachlich sauberer vor
- kann zunaechst read-only bleiben

Nachteile:

- liefert noch keine neue Kalkulationsfunktion
- muss eng bleiben, damit keine vollwertige Entscheidungshistorie entsteht

### Option C: minimaler Freigabe- oder Abweichungsanker

Vorteile:

- koennte spaeter Preise unter Kostenbasis oder starke Abweichungen markieren
- passt zu einem ERP mit Verantwortlichkeiten

Nachteile:

- setzt belastbare Transparenz ueber Entscheidung und Abweichung voraus
- waere als erster Schritt nach der Preisuebernahme zu frueh
- fuehrt neue Workflow-Zustaende ein

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge read-only Preisentscheidungs-Transparenz direkt an genau einer
  bereits gemappten Quote-Position

Dieser Schritt soll nur sichtbar machen:

- aktuellem Positionspreis
- aktuelle primaere Preisquelle
- ob der aktuelle Positionspreis der primaeren Quelle entspricht
- absolute Abweichung zwischen aktuellem Preis und primaerer Quelle
- kleiner Entscheidungsstatus wie `entspricht primaerer Quelle`,
  `abweichend` oder `keine primaere Quelle`

Er soll bewusst noch nicht:

- eine separate Entscheidungshistorie persistieren
- Margen, Zuschlaege oder Rabatte berechnen
- eine Freigabe ausloesen
- Preise automatisch anpassen
- mehrere Positionen aggregiert bewerten

## 6. Warum nicht sofort Marge oder Zuschlag

Ein sofortiger Margen- oder Zuschlagsanker waere fachlich zu gross, weil der
aktuelle Datenstand noch nicht sauber trennt zwischen:

- Kostenanker aus Preisquelle
- aktuell gesetztem Positionspreis
- spaeterem kalkuliertem Verkaufspreis
- Freigabe- oder Abweichungsstatus

Ohne diese Trennung wuerde eine Marge entweder auf einem bereits
ueberschriebenen Preis oder auf einer implizit neu bestimmten Quelle
aufsetzen. Beides waere fuer den Nutzer schwer nachvollziehbar.

## 7. Warum Preisentscheidungs-Transparenz den besten Signalwert hat

Preisentscheidungs-Transparenz hat jetzt den besten Signalwert, weil:

- die neue explizite Aktion dadurch unmittelbar fachlich erklaert wird
- der Nutzer schnell sieht, ob die Position noch zur primaeren Quelle passt
- spaetere Kalkulation auf einer nachvollziehbaren Preisbasis aufbauen kann
- der Schritt weiterhin read-only bleiben kann
- keine neuen kommerziellen Regeln eingefuehrt werden muessen

Damit entsteht ein stabilerer Anschluss fuer den spaeteren ERP-Ausbau, ohne
schon in Vollkalkulation oder Workflowlogik zu kippen.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- kleiner read-only Transparenzanker fuer genau eine Quote-Position
- Wiederverwendung der bestehenden primaeren Preisquelle
- Vergleich aktueller Positionspreis gegen primaere Quelle
- Ausgabe eines kleinen Status und der Abweichung
- Anzeige direkt im bestehenden Quote-Editor

Wichtig bleibt:

- kein neuer Schreibpfad
- keine Marge
- kein Zuschlag
- keine Freigabe
- keine Automatik
- keine Bulk-Funktion

## 9. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen expliziten
Preisentscheidung ist nicht sofort ein kalkulationsnaher Margen- oder
Zuschlagsanker, sondern eine enge read-only Preisentscheidungs-Transparenz.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die read-only
  Preisentscheidungs-Transparenz zuschneiden, bewusst noch ohne neue
  Persistenz, Margen-, Zuschlags-, Rabatt-, Freigabe-, Bulk- oder
  Automatiklogik

