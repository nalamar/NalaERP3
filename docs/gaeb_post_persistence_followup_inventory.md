# GAEB-Folgeausbau nach Preisentscheidungs-Persistenz:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen engen Preisentscheidungs-Persistenz.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach persistiertem
  Preisentscheidungs-Snapshot bestimmen
- entscheiden, ob jetzt eine kleine Historienanzeige, ein kalkulationsnaher
  Margen-/Zuschlagsanker oder ein minimaler Abweichungs-/Freigabeanker den
  besten Signalwert hat
- Historie, Kalkulation, Freigabe und Automatik weiterhin sauber trennen

## 1. Ausgangslage nach abgeschlossener Persistenz

Der aktuelle Stand deckt bereits ab:

- Materialzuordnung an der Quote-Position
- sichtbare und priorisierte Preisquellen
- read-only Preisbewertung
- explizite Uebernahme der primaeren Quelle als Positionspreis
- read-only Preisentscheidungs-Transparenz
- persistierter Snapshot bei der expliziten Primaerpreis-Uebernahme

Damit ist die fachliche Entscheidung erstmals nicht nur ausgefuehrt, sondern
auch technisch festgehalten.

## 2. Verbleibende fachliche Luecke

Nach der Persistenz bleibt eine konkrete Luecke:

- der Snapshot existiert in `quote_item_price_decisions`
- der Nutzer sieht ihn aber noch nicht
- der Quote-Editor zeigt weiterhin nur Live-Bewertung und Live-Transparenz
- spaetere Abweichungen koennen noch nicht gegen die letzte gespeicherte
  Entscheidung gelesen werden

Die naechste Luecke liegt damit zuerst in Sichtbarkeit und Nachvollziehbarkeit,
nicht in einer neuen Kalkulationsregel.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten engen Blocks:

- Zielmarge
- Zuschlag oder Gemeinkosten
- Rabattlogik
- Freigabe-Workflow
- Eskalationsregel
- Bulk-Historie ueber ganze Angebote
- automatische Preisentscheidung
- KI-gestuetzte Preisstrategie

Diese Themen brauchen zwar den Snapshot, aber nicht als naechsten kleinsten
Schritt.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: kleine Historienanzeige

Vorteile:

- nutzt direkt die neu persistierten Snapshots
- macht die Entscheidung fuer Nutzer nachvollziehbar
- bleibt read-only und positionsnah
- braucht keine neue Kalkulationsregel
- schafft bessere Grundlage fuer spaetere Margen- und Freigabelogik

Nachteile:

- liefert noch keine neue Preisberechnung
- muss eng bleiben, damit keine vollwertige Audit- oder Freigabeakte entsteht

### Option B: kalkulationsnaher Margen-/Zuschlagsanker

Vorteile:

- waere der sichtbare Schritt Richtung Angebotskalkulation
- passt langfristig zum ERP-Zielbild
- kann auf gespeicherten Kostenentscheidungen aufsetzen

Nachteile:

- die gespeicherten Entscheidungen sind noch nicht sichtbar
- Nutzer koennen noch nicht nachvollziehen, welcher Snapshot Grundlage einer
  spaeteren Kalkulation waere
- fuehrt neue Regeln und Felder ein, bevor die neue Persistenz operativ
  lesbar ist

### Option C: minimaler Abweichungs-/Freigabeanker

Vorteile:

- koennte spaeter Preise unter Kostenbasis oder Abweichungen markieren
- passt zu Verantwortlichkeiten im ERP

Nachteile:

- Freigabe braucht sichtbare Entscheidungsgrundlagen
- ohne Historienanzeige waere die Begruendung im Editor schwach
- fuehrt neue Workflow-Zustaende ein

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine kleine read-only Historienanzeige fuer Preisentscheidungen direkt an
  genau einer Quote-Position

Diese Anzeige soll nur sichtbar machen:

- letzte oder wenige gespeicherte Entscheidungen der Position
- Entscheidungsart
- Quelle
- uebernommener Preis
- Waehrung
- Referenz und Quelldatum, soweit vorhanden
- Entscheidungszeit

Sie soll bewusst noch nicht:

- Snapshots bearbeiten
- Entscheidungen kommentieren
- Freigaben ausloesen
- Margen oder Zuschlaege berechnen
- mehrere Positionen aggregieren

## 6. Warum jetzt nicht sofort Marge oder Zuschlag

Ein Margen-/Zuschlagsanker ist fachlich naheliegend, aber der kleinste
operative Anschluss nach Persistenz ist zuerst Lesbarkeit.

Ohne Historienanzeige waere eine spaetere Kalkulation zwar technisch moeglich,
aber fuer Nutzer schwer nachvollziehbar:

- welche gespeicherte Entscheidung ist Grundlage?
- wann wurde sie erzeugt?
- aus welcher Quelle stammt der Preis?
- wurde der Positionspreis danach wieder veraendert?

Diese Fragen sollten sichtbar sein, bevor daraus Kalkulationslogik entsteht.

## 7. Warum jetzt nicht sofort Freigabe

Ein Freigabeanker braucht:

- sichtbare Entscheidungsgrundlage
- erkennbare Abweichung
- klare Verantwortlichkeit
- Workflow-Zustand

Aktuell existiert nur der Snapshot. Der naechste kleine Schritt ist deshalb,
ihn sichtbar zu machen, nicht ihn schon freizugeben.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- read-only API fuer Preisentscheidungen einer Quote-Position
- kleine Anzeige im bestehenden Quote-Editor
- keine neue Arbeitsflaeche
- keine neue Schreibaktion
- keine Kalkulations- oder Freigabelogik

Der passende fachliche Name ist:

- Preisentscheidungs-Historie

## 9. Entscheidung

Der naechste sinnvolle Folgeausbau nach der persistierten Preisentscheidung
ist nicht sofort Margen-/Zuschlagslogik oder Freigabe, sondern eine kleine
read-only Historienanzeige fuer Preisentscheidungen an der Quote-Position.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die read-only
  Preisentscheidungs-Historie zuschneiden, bewusst noch ohne
  Historienbearbeitung, Margen-, Zuschlags-, Rabatt-, Freigabe-, Bulk- oder
  Automatiklogik

