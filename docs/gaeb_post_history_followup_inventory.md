# GAEB-Folgeausbau nach sichtbarer Preisentscheidungs-Historie:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen read-only Preisentscheidungs-Historie.

Der Fokus bleibt bewusst eng:

- entscheiden, ob jetzt ein kalkulationsnaher Margen-/Zuschlagsanker, ein
  minimaler Abweichungs-/Freigabeanker oder eine engere Historien-/Auditfunktion
  den besten Signalwert hat
- den naechsten kleinsten fachlichen Schnitt bestimmen
- Kalkulation, Freigabe und Historienaudit weiterhin sauber trennen

## 1. Ausgangslage nach sichtbarer Historie

Der aktuelle Stand deckt bereits ab:

- Materialzuordnung an der Quote-Position
- sichtbare und priorisierte Preisquellen
- read-only Preisbewertung
- explizite Uebernahme der primaeren Quelle als Positionspreis
- read-only Preisentscheidungs-Transparenz
- persistierter Preisentscheidungs-Snapshot
- read-only Historie gespeicherter Preisentscheidungen an genau einer Position

Damit ist erstmals nachvollziehbar, welche gespeicherte Preisentscheidung als
kommerzieller Bezugspunkt fuer eine Quote-Position existiert.

## 2. Verbleibende fachliche Luecke

Nach der sichtbaren Historie bleibt eine neue konkrete Luecke:

- Nutzer sehen die gespeicherte Preisentscheidung
- Nutzer sehen den aktuellen Positionspreis
- Nutzer sehen aber noch nicht, ob der aktuelle Positionspreis oberhalb,
  auf oder unterhalb der letzten gespeicherten Kostenbasis liegt

Die naechste Luecke liegt damit nicht mehr in Nachvollziehbarkeit der
Entscheidung, sondern in einem ersten kalkulationsnahen Lesesignal.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten engen Blocks:

- Zielmarge
- Zuschlagsregel oder Gemeinkostenmodell
- Rabattlogik
- Freigabe-Workflow
- Eskalationsregel
- Rollen- oder Berechtigungsmatrix fuer Freigaben
- Bulk-Kalkulation ueber mehrere Positionen
- automatische Preisentscheidung
- KI-gestuetzte Angebotskalkulation

Diese Themen brauchen ein kalkulationsnahes Signal, sind aber groesser als der
naechste kleinste Schnitt.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: kalkulationsnaher Margen-/Zuschlagsanker

Vorteile:

- nutzt die jetzt sichtbare gespeicherte Preisentscheidung direkt
- liefert ein klares naechstes Signal Richtung Angebotskalkulation
- kann read-only und positionsnah bleiben
- braucht noch keine neue Persistenz und keinen Workflow
- bereitet spaetere Zielmargen, Zuschlaege, Rabatte und Freigaben vor

Nachteile:

- muss eng begrenzt werden, damit daraus keine vollwertige Kalkulation entsteht
- darf keine implizite Preisautomatik einfuehren
- braucht eine klare Kostenbasis, damit der Vergleich fachlich belastbar bleibt

### Option B: minimaler Abweichungs-/Freigabeanker

Vorteile:

- waere fuer Preise unter Kostenbasis oder starke Abweichungen fachlich relevant
- passt langfristig zu Verantwortlichkeiten im ERP
- kann spaeter mit Rollen und Schwellwerten kombiniert werden

Nachteile:

- Freigabe braucht zuerst ein sichtbares Margen- oder Abweichungssignal
- Schwellwerte, Rollen und Workflow-Zustaende waeren neue fachliche Achsen
- ohne vorherigen Kalkulationsanker waere die Freigabebegruendung zu duenn

### Option C: engere Historien-/Auditfunktion

Vorteile:

- koennte spaeter Kommentare, Korrekturen oder Filterung ergaenzen
- staerkt Nachvollziehbarkeit und Revisionsfaehigkeit
- passt zu spaeteren Audit- und Verantwortlichkeitsfragen

Nachteile:

- die Kernluecke der Sichtbarkeit ist bereits geschlossen
- weitere Historienfunktionen liefern aktuell weniger Signal als ein erster
  kalkulationsnaher Vergleich
- Kommentare, Korrektur und Detailaudit waeren ein eigener groesserer Block

## 5. Entscheidung

Der naechste sinnvolle Folgeausbau ist ein kleiner read-only
Margen-/Zuschlagsanker an genau einer Quote-Position.

Dieser Block soll nicht kalkulieren, wie ein Angebot neu bepreist werden muss.
Er soll nur sichtbar machen, wie der aktuelle Positionspreis zur letzten
gespeicherten Preisentscheidung steht.

Die fachliche Grundlage ist:

- aktuelle `unit_price` der Quote-Position
- letzte gespeicherte Preisentscheidung der Position als Kostenbasis
- Differenz zwischen aktuellem Preis und Kostenbasis
- prozentualer Aufschlag bezogen auf die Kostenbasis
- einfacher read-only Status fuer das Verhaeltnis zur Kostenbasis

## 6. Warum jetzt Marge und nicht Freigabe

Ein Freigabeanker braucht eine begruendbare Abweichung.

Diese Abweichung sollte zuerst als read-only Signal sichtbar werden:

- was ist die letzte gespeicherte Kostenbasis?
- wie weit liegt der aktuelle Positionspreis darueber oder darunter?
- ist das eine negative, neutrale oder positive Marge?

Erst danach sind Schwellwerte, Rollen und Freigabeentscheidungen sinnvoll.

## 7. Warum jetzt keine engere Historienfunktion

Die Historie ist als Minimalziel erfuellt:

- gespeicherte Entscheidungen sind sichtbar
- Quelle, Preis und Entscheidungszeit sind lesbar
- der Block bleibt read-only

Weitere Historienfunktionen waeren wertvoll, aber sie wuerden zuerst die
Nachvollziehbarkeit vertiefen. Der hoehere Signalwert liegt jetzt in der
kommerziellen Einordnung des sichtbaren Snapshots.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- read-only Berechnung fuer genau eine Quote-Position
- Kostenbasis aus der letzten gespeicherten Preisentscheidung
- Anzeige von aktuellem Positionspreis, Kostenbasis, absoluter Differenz und
  prozentualem Aufschlag
- einfacher Status wie `negative_margin`, `zero_margin` oder
  `positive_margin`
- Anzeige der zugrunde liegenden Quelle und Entscheidungszeit, soweit vorhanden

Bewusst nicht enthalten:

- Schreibaktion
- neue persistierte Margenfelder
- Zielmarge
- Zuschlagsregel
- Rabatt
- Freigabe
- Bulk-Aktion
- automatische Preisveraenderung

## 9. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer einen read-only
  Margen-/Zuschlagsanker an genau einer Quote-Position zuschneiden

Dieses Zielmodell muss entscheiden:

- welche Service-Methode den Vergleich liefert
- wie fehlende Preisentscheidung oder fehlende Kostenbasis dargestellt wird
- welche API-Antwort der Client braucht
- wo der bestehende Quote-Editor das Signal positionsnah anzeigen soll
