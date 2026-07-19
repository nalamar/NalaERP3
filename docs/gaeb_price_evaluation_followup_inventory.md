# GAEB-Folgeausbau nach abgeschlossener Quellen-Priorisierung:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen Quellen-Priorisierung.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach sichtbarer und priorisierter
  Preisquellenlage bestimmen
- entscheiden, ob jetzt kalkulationsnahe Preisbewertung, kontrollierte
  Preisentscheidung oder ein anderer minimaler kommerzieller Folgeschritt den
  besten Signalwert hat
- Preisquellen, Bewertung, Entscheidung und spaetere Kalkulationsautomatik
  sauber trennen

## 1. Ausgangslage nach abgeschlossener Quellen-Priorisierung

Der aktuelle Stand deckt bereits ab:

- manuelle Materialwahl an der Quote-Position
- read-only Materialsuche und explizite Materialuebernahme
- read-only Preisvorschlag nach gesetztem `material_id`
- explizite Preisuebernahme aus dem sichtbaren Preisanker
- kleine read-only Preisquellenanzeige direkt an der Position
- kleine read-only Priorisierung der sichtbaren Preisquellen

Damit ist der kleine kommerzielle Basispfad jetzt erweitert auf:

- Material setzen
- Preisanker sehen
- Preis explizit uebernehmen
- Herkunft und Preisquellen nachvollziehen
- sichtbare Quellen in eine erste fachliche Reihenfolge bringen

## 2. Verbleibende fachliche Luecke

Nach der priorisierten Quellenlage bleibt vor allem diese Luecke:

- passt der aktuell gesetzte `unit_price` zur primaeren Preisquelle?
- wie gross ist die Abweichung zwischen aktuellem Positionspreis und
  priorisierter Kostenbasis?
- entsteht jetzt zuerst Bedarf an einer Entscheidung, welcher Preis uebernommen
  wird, oder zuerst an einer kleinen Bewertung des bereits gesetzten Preises?

Die relevante Luecke liegt damit zuerst nicht in einem neuen Schreibpfad,
sondern in der kleinen Bewertung des bestehenden Positionspreises gegen die
priorisierte Preisquelle.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Vollkalkulation mit Marge, Gemeinkosten, Zuschlaegen oder Rabatten
- automatische Preiswahl aus der priorisierten Quelle
- Preisaktualisierung ohne explizite Nutzeraktion
- Bulk-Bewertung ueber ganze Angebote
- Freigabe- oder Eskalationsworkflow fuer Deckungsbeitrag
- KI-gestuetzte Preisoptimierung oder lernende Preisstrategie

Diese Themen waeren bereits deutlich groessere Kalkulations- oder
Automatikstufen.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: kalkulationsnahe Preisbewertung

Vorteile:

- baut direkt auf der priorisierten primaeren Quelle auf
- bleibt read-only und positionsnah
- macht die Abweichung zwischen aktuellem Verkaufspreis und Kostenbasis
  sichtbar
- bereitet spaetere Margen-, Zuschlags- und Freigabelogik vor, ohne sie schon
  einzufuehren

Nachteile:

- muss eng bleiben, damit keine Vollkalkulation entsteht
- darf noch keine automatische Preisentscheidung treffen

### Option B: kontrollierte Preisentscheidung

Vorteile:

- koennte dem Nutzer erlauben, die primaere Quelle explizit als Preisbasis zu
  uebernehmen
- schliesst fachlich an die bisherige explizite Preisuebernahme an

Nachteile:

- waere ein neuer Write-Pfad und damit riskanter als reine Bewertung
- ohne sichtbare Abweichung ist die Entscheidung fachlich schwach erklaert
- kippt schneller in automatische Preiswahl oder Preisstrategie

### Option C: anderer minimaler kommerzieller Ausbau

Moegliche Varianten wie weitere Herkunftslabels, kosmetische
Prioritaetsanzeigen oder eine globale Quellenuebersicht haben aktuell weniger
Signal, weil die direkte positionsnahe Quellenlage bereits sichtbar und
eingeordnet ist.

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge read-only Preisbewertung direkt an genau einer bereits gemappten
  Draft-Quote-Position

Diese Bewertung soll nur zeigen:

- aktuellem `unit_price`
- primaere Preisquelle aus der bestehenden Quellen-Priorisierung
- absolute Abweichung zwischen aktuellem Preis und primaerer Quelle
- relative Abweichung in Prozent, soweit sinnvoll berechenbar
- kleine Bewertung wie `unter Kostenbasis`, `auf Kostenbasis` oder `ueber
  Kostenbasis`

Sie soll bewusst noch nicht:

- einen neuen Preis berechnen
- einen Preis automatisch setzen
- Margen, Zuschlaege oder Rabatte anwenden
- mehrere Positionen aggregiert bewerten

## 6. Warum jetzt Preisbewertung den besten Signalwert hat

Die kleine Preisbewertung ist jetzt der beste naechste Schritt, weil:

- die relevante Quelle inzwischen nicht nur sichtbar, sondern priorisiert ist
- der aktuelle Positionspreis bereits im Angebotsmodell existiert
- die Differenz zwischen gesetztem Preis und primaerer Quelle den naechsten
  operativen Erkenntnisgewinn liefert
- eine spaetere kontrollierte Preisentscheidung dadurch fachlich besser
  begruendet werden kann
- der Schritt read-only bleiben kann und damit weniger Risiko hat als ein
  weiterer Schreibpfad

Damit liefert eine kleine Preisbewertung jetzt mehr Signal als eine sofortige
Preisentscheidung.

## 7. Warum nicht zuerst kontrollierte Preisentscheidung

Eine sofortige Entscheidung oder Uebernahme aus der priorisierten Quelle waere
jetzt zu frueh, weil:

- die Auswirkungen auf den aktuellen Positionspreis noch nicht transparent
  sind
- es noch keine kleine Bewertung gibt, ob der aktuelle Preis unter, auf oder
  ueber der Kostenbasis liegt
- sonst zwei Ebenen zugleich vermischt wuerden:
  - welche Quelle ist primaer?
  - soll daraus direkt ein neuer Preis werden?
- ein spaeterer Schreibpfad weiterhin moeglich bleibt, sobald die read-only
  Bewertung stabil ist

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- kleiner read-only Bewertungsanker fuer genau eine bereits gemappte
  Draft-Quote-Position
- Nutzung der bestehenden Quellen-Priorisierung als primaere Kostenbasis
- Ausgabe von aktuellem Positionspreis, primaerer Quelle, absoluter und
  relativer Abweichung sowie kleinem Bewertungsstatus
- Anzeige direkt im bestehenden Quote-Editor

Wichtig bleibt:

- keine automatische Preissetzung
- keine Vollkalkulation
- keine Margen- oder Zuschlagslogik
- keine Bulk-Bewertung
- keine neuen globalen Preisarbeitsflaechen

## 9. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer diesen Folgepunkt der richtige Ort, weil
dort bereits zusammenlaufen:

- gesetztes `material_id`
- aktueller `unit_price`
- Preisvorschlag
- Preisquellen
- Quellen-Priorisierung
- manuelle Preisverantwortung des Nutzers

Eine separate Kalkulationsansicht waere fuer diesen ersten
Bewertungsanker unnoetig gross.

## 10. Nutzen vor spaeterer Kalkulationslogik

Schon ohne Kalkulationslogik schafft eine kleine Preisbewertung:

- schnelle Erkennung von Preisen unter der primaeren Kostenbasis
- bessere Nachvollziehbarkeit der manuellen Preisentscheidung
- belastbarere Grundlage fuer spaetere Zuschlags-, Margen- und
  Freigabelogik
- klaren Anschluss fuer eine spaetere kontrollierte Preisentscheidung

## 11. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Quellen-Priorisierung ist nicht sofort ein weiterer Write-Pfad, sondern eine
enge kalkulationsnahe read-only Preisbewertung.

## 12. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diese read-only Preisbewertung
  zuschneiden, bewusst noch ohne automatische Preisentscheidung, Bulk,
  Margen-, Zuschlags- oder Freigabelogik
