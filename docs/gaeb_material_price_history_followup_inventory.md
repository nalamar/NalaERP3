# GAEB-Folgeausbau nach abgeschlossener Preis-Historien-/Quellenstufe:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen Preis-Historien-/Quellenstufe.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach abgeschlossener read-only
  Preisquellenanzeige bestimmen
- entscheiden, ob jetzt eine enge Quellen-Priorisierung, ein kleiner
  kalkulationsnaher Folgeschritt oder ein anderer Ausbau den besten
  Signalwert hat
- Quellen-, Priorisierungs-, Kalkulations- und Automatiklogik weiterhin
  sauber trennen

## 1. Ausgangslage nach abgeschlossener Preis-Historien-/Quellenstufe

Der aktuelle Stand deckt bereits ab:

- manuelle Materialwahl an der Quote-Position
- read-only Materialsuche und explizite Materialuebernahme
- read-only Preisvorschlag nach gesetztem `material_id`
- explizite Preisuebernahme aus dem sichtbaren Preisanker
- kleine read-only Preisquellenanzeige direkt an der Position

Damit ist der kleine kommerzielle Basispfad jetzt erweitert auf:

- Material setzen
- Preisanker sehen
- Preis explizit uebernehmen
- Herkunft und kleine Preisquellen nachvollziehen

## 2. Verbleibende fachliche Luecke

Nach der sichtbaren Preisquellenanzeige bleibt vor allem diese Luecke:

- wenn mehr als eine Preisquelle sichtbar ist, welche Quelle ist fuer die
  aktuelle Position zuerst relevant?
- wo entsteht jetzt der groesste operative Bruch: bei fehlender Priorisierung
  sichtbarer Quellen oder schon bei einem fruehen kalkulatorischen Anschluss?
- welcher Folgeausbau schafft Nutzsignal, ohne schon in Kalkulations- oder
  Automatiklogik zu kippen?

Die relevante Luecke liegt damit zuerst nicht in einer neuen Berechnung,
sondern in der kleinen Einordnung der bereits sichtbaren Preisquellen.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Vollkalkulation, Margen-, Zuschlags- oder Rabattlogik
- automatische Preisaktualisierung oder Preisneuberechnung
- Bulk-Priorisierung oder Bulk-Kalkulation ueber ganze Quotes
- komplexe Scoring- oder Regelwerke ueber viele Preisquellen
- separate kommerzielle Cockpits oder globale Preisarbeitsflaechen

Diese Themen waeren bereits deutlich groessere Kalkulations- oder
Automatikstufen.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: enge Quellen-Priorisierung

Vorteile:

- baut direkt auf der jetzt sichtbaren Mehrquellenlage auf
- bleibt nah an genau einer Quote-Position
- schafft als naechstes operatives Signal, welche Quelle zuerst zaehlt
- bereitet spaetere Preisentscheidung oder Kalkulationsnaehe vor, ohne sie
  schon einzufuehren

Nachteile:

- muss eng bleiben, damit sie nicht in komplexe Rankinglogik kippt

### Option B: kleiner kalkulationsnaher Folgeschritt

Vorteile:

- waere spaeter der naechste kommerzielle Ausbau nach stabiler Preisbasis
- koennte den Schritt von Kostennaehe zu Angebotslogik vorbereiten

Nachteile:

- setzt fachlich eigentlich schon eine belastbarere Bewertung der sichtbaren
  Preisquellen voraus
- kippt schneller in groessere Preisstrategie oder Margenlogik
- ist vor einer kleinen Quellen-Priorisierung fachlich zu frueh

### Option C: anderer minimaler kommerzieller Ausbau

Moegliche Varianten wie weiterer Anzeige-Feinschliff oder rein kosmetische
Quellenmetadaten haben aktuell nur schwaches Signal, weil die read-only
Quellensicht selbst bereits steht.

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge Quellen-Priorisierung direkt an genau einer bereits gemappten
  Draft-Quote-Position

Dieser Schritt soll nur eine kleine operative Luecke schliessen:

- die bereits sichtbaren Preisquellen in eine erste nachvollziehbare Ordnung
  bringen
- den Nutzer bei der positionsnahen Preisentscheidung besser fuehren
- die Grundlage fuer spaetere kalkulationsnahe Schritte vorbereiten

## 6. Warum jetzt die Quellen-Priorisierung den besten Signalwert hat

Die kleine Quellen-Priorisierung ist jetzt der bessere naechste Schritt,
weil:

- die Quellen inzwischen sichtbar sind und damit erstmals eine kleine
  Auswahlfrage entsteht
- der aktuelle Schmerzpunkt nicht mehr fehlende Herkunft, sondern fehlende
  Einordnung der Herkunft ist
- ein kalkulationsnaher Schritt auf einer bereits etwas geordneten
  Preisgrundlage deutlich belastbarer waere
- eine enge Priorisierung operativen Nutzen schafft, ohne schon Margen-,
  Zuschlags- oder Automatiklogik zu versprechen

Damit liefert eine kleine Quellen-Priorisierung jetzt mehr Signal als ein
frueher kalkulationsnaher Ausbau.

## 7. Warum nicht zuerst Kalkulationsnaehe

Ein frueher kalkulationsnaher Schritt waere jetzt zu frueh, weil:

- die sichtbaren Preisquellen bislang nur dargestellt, aber noch nicht
  eingeordnet sind
- Kalkulationsnaehe ohne kleine Quellen-Priorisierung fachlich duenn bleibt
- sonst zwei Ebenen zugleich vermischt wuerden:
  - Welche Quelle ist fuer die Position zuerst relevant?
  - Wie wird auf dieser Basis weiter kalkuliert?
- ein spaeterer kalkulationsnaher Schritt weiterhin moeglich bleibt, sobald
  die Quelleneinordnung stabil ist

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- kleine positionsnahe read-only Priorisierung sichtbarer Preisquellen nur
  fuer genau eine bereits gemappte Draft-Quote-Position
- weiterhin Anzeige im bestehenden Quote-Editor
- noch keine automatische Preiswahl und noch keine stille Preisanpassung

Wichtig bleibt:

- keine komplexen Mehrquellen-Scores im selben Schritt
- keine Bulk-Priorisierung ueber mehrere Positionen
- keine Vermischung mit Margen-, Zuschlags- oder Rabattlogik

## 9. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer diesen Folgepunkt der richtige Ort, weil
dort bereits zusammenlaufen:

- gesetztes `material_id`
- aktueller `unit_price`
- sichtbarer Preisanker
- sichtbare Preisquellen
- manuelle Preisverantwortung des Nutzers

Eine separate Priorisierungs- oder Kalkulationsansicht waere fuer diesen
ersten Folgepunkt unnoetig gross.

## 10. Nutzen schon vor spaeterer Kalkulationsnaehe

Schon ohne Kalkulationslogik schafft eine kleine Quellen-Priorisierung:

- schnellere Einordnung sichtbarer Preisquellen
- belastbareren Kontext fuer spaetere Preisentscheidungen
- bessere Grundlage fuer spaetere kalkulationsnahe Ausbaustufen

## 11. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Preis-Historien-/Quellenstufe ist nicht zuerst ein kalkulationsnaher
Folgeschritt, sondern eine enge minimale Quellen-Priorisierung.

## 12. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diese enge
  Quellen-Priorisierung zuschneiden, bewusst noch ohne Ranking-,
  Bulk-, Kalkulations- oder Automatiklogik
