# GAEB-Folgeausbau nach abgeschlossener Materialsuche: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen Materialsuche.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach der read-only Suche bestimmen
- entscheiden, ob die explizite Uebernahme aus Suchtreffern jetzt den besten
  Signalwert hat
- Uebernahme-, Ranking- und Automatiklogik weiterhin sauber trennen

## 1. Ausgangslage nach abgeschlossener Materialsuche

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- enger manueller Statusraum `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine read-only Materialkandidatenliste direkt an der Quote-Position
- explizite Uebernahme eines bereits sichtbaren Kandidaten
- enge read-only Materialsuche direkt an offenen Draft-Quote-Positionen

Damit sind jetzt zwei Vorstufen geloest:

- sichtbaren Kandidaten direkt uebernehmen
- bei Nicht-Trefferfaellen vorhandene Materialien gezielt suchen

## 2. Verbleibende fachliche Luecke

Nach der Materialsuche bleibt vor allem diese Luecke:

- was passiert, wenn ein passendes Material in der Suchtrefferliste sichtbar
  ist?
- wie wird aus dem read-only Suchergebnis wieder ein kontrolliertes manuelles
  Mapping auf genau eine Quote-Position?
- wie bleibt dieser Schritt klein, ohne sofort Preislogik, Ranking oder
  Halbautomatik mitzuziehen?

Die relevante Luecke ist damit nicht mehr Auffindbarkeit, sondern die
kontrollierte explizite Uebernahme eines bereits sichtbaren Suchtreffers.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Ranking oder Priorisierung unter Suchtreffern
- automatische Vorbelegung oder automatische Auswahl
- Bulk-Uebernahme ueber mehrere Quote-Positionen
- Preisvorschlaege oder Preislogik aus dem uebernommenen Material
- globale Mapping- oder Suchkonsole
- KI-gestuetztes Matching zwischen Importtext und Materialstamm

Diese Themen waeren bereits deutlich groessere Matching-, Komfort- oder
Automatikstufen.

## 4. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge explizite Uebernahme eines bereits sichtbaren Suchtreffers direkt
  auf genau eine offene Draft-Quote-Position

Dieser Schritt soll nur eine kleine operative Luecke schliessen:

- aus sichtbaren Suchtreffern kontrolliert ein Material setzen
- das bestehende manuelle Mapping ohne Material-ID-Eingabe abschliessen
- den Suchpfad auf derselben Quote-Position zu Ende fuehren

## 5. Warum jetzt die Suchtreffer-Uebernahme den besten Signalwert hat

Die explizite Uebernahme aus Suchtreffern ist jetzt der bessere naechste
Schritt, weil:

- die Suchauffindung bereits geloest ist
- der verbleibende Schmerzpunkt jetzt der letzte manuelle Mapping-Schritt ist
- ein sichtbarer Treffer ohne Uebernahmepfad noch keinen operativen Abschluss
  bringt
- dieser Schritt den vorhandenen Kandidaten-Uebernahmepfad logisch ergaenzt,
  ohne den Scope auf Ranking oder Automatik auszudehnen

Damit liefert die Suchtreffer-Uebernahme mehr Signal als weiterer Feinschliff
an der read-only Suchanzeige.

## 6. Warum nicht zuerst Ranking oder Preislogik

Ein frueher Ausbau von Ranking, Match-Scores oder Preislogik waere jetzt zu
frueh, weil:

- bereits sichtbare Suchtreffer fuer den Nutzer manuell bewertbar sind
- der groessere operative Engpass aktuell nicht Priorisierung, sondern
  Abschluss des Mapping-Schritts ist
- Preislogik fachlich ein separater Block bleibt und nicht still in die
  Materialwahl hineingezogen werden sollte
- ein sauberer manueller Uebernahmepfad spaeter auch bessere Matching- oder
  Preisstufen vorbereitet

## 7. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- pro sichtbarem Suchtreffer eine explizite Aktion `Uebernehmen`
- enger Backend-Write-Pfad fuer genau eine Draft-Quote-Position
- unmittelbare Rueckspiegelung des neuen Mapping-Zustands aus der
  Serverantwort

Wichtig bleibt:

- keine automatische Auswahl
- keine Rankingversprechen
- kein Bulk-Mapping
- keine Aenderung von `unit_price`, `tax_code` oder anderen
  Kalkulationsfeldern im selben Schritt

## 8. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer die Suchtreffer-Uebernahme der richtige
Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- Mapping-Status
- sichtbare Kandidaten
- sichtbare Suchtreffer
- manuelle Materialzuordnung

Eine separate Uebernahmeoberflaeche waere fuer diesen Schritt unnoetig gross
und wuerde den bewusst kleinen Pfad aufbrechen.

## 9. Nutzen schon vor spaeterem Ranking oder Automatik

Schon ohne Ranking oder Automatik schafft die explizite Suchtreffer-
Uebernahme:

- einen durchgaengigen kleinen Mapping-Pfad fuer Nicht-Trefferfaelle
- weniger Abhaengigkeit von direkter Material-ID-Eingabe
- saubere Symmetrie zwischen Kandidaten-Uebernahme und Suchtreffer-Uebernahme
- belastbare Grundlage fuer spaetere Matching-, Ranking- oder
  Preisvorschlagsstufen

## 10. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Materialsuche ist nicht weiterer Feinschliff an der read-only Suche und auch
nicht sofort Ranking oder Automatik, sondern zuerst eine enge explizite
Uebernahme aus bereits sichtbaren Suchtreffern.

## 11. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die enge Suchtreffer-Uebernahme
  zuschneiden, bewusst noch ohne Ranking-, Bulk-, Preis- oder Automatiklogik
