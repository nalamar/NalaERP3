# GAEB-Folgeausbau nach abgeschlossener Suchtreffer-Uebernahme: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen Suchtreffer-Uebernahme.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach abgeschlossenem manuellem
  Materialmapping bestimmen
- entscheiden, ob jetzt eine enge Ranking-/Priorisierungsstufe, ein minimaler
  Preisvorschlagspfad oder ein anderer Folgeschritt den besten Signalwert hat
- Mapping-, Preis- und Automatiklogik weiterhin sauber trennen

## 1. Ausgangslage nach abgeschlossener Suchtreffer-Uebernahme

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- enger manueller Statusraum `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine read-only Materialkandidatenliste direkt an der Quote-Position
- explizite Uebernahme eines bereits sichtbaren Kandidaten
- enge read-only Materialsuche direkt an offenen Draft-Quote-Positionen
- explizite Uebernahme eines sichtbaren Suchtreffers

Damit ist der kleine manuelle Materialmapping-Pfad jetzt durchgaengig:

- sichtbaren Kandidaten direkt uebernehmen
- sonst Material suchen
- sichtbaren Suchtreffer direkt uebernehmen

## 2. Verbleibende fachliche Luecke

Nach der Suchtreffer-Uebernahme bleibt vor allem diese Luecke:

- was ist der kleinste sinnvolle Folgeschritt, nachdem `material_id` jetzt
  kontrolliert gesetzt ist?
- wo entsteht jetzt der groesste operative Bruch: bei der Materialwahl selbst
  oder bei der anschliessenden Preisfindung?
- welcher Folgeausbau schafft Signal, ohne schon Ranking- oder
  Kalkulationsautomatik zu versprechen?

Die relevante Luecke ist damit nicht mehr das Materialmapping selbst,
sondern der kleinste nutzbare Anschluss nach gesetztem Material.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- groessere Ranking- oder Match-Score-Modelle
- automatische Materialvorbelegung
- Bulk-Mapping oder Bulk-Preislogik ueber ganze Quotes
- automatische Kalkulation kompletter Positionen
- globale Mapping-, Such- oder Preisarbeitsflaechen
- KI-gestuetzte Matching- oder Preisautomatik

Diese Themen waeren bereits deutlich groessere Komfort-, Matching- oder
Automatikstufen.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: enge Ranking-/Priorisierungsstufe

Vorteile:

- koennte die Auswahl unter mehreren Treffern erleichtern
- waere eine direkte Fortsetzung des Suchkontexts

Nachteile:

- der aktuelle kleine Pfad ist bereits manuell benutzbar
- Ranking verbessert Priorisierung, schliesst aber keinen harten operativen
  Folgebruch nach gesetztem Material
- Ranking kippt schnell in intransparente Halbautomatik

### Option B: minimaler Preisvorschlagspfad nach gesetztem Material

Vorteile:

- schliesst direkt den naechsten operativen Bruch nach erfolgreichem Mapping
- nutzt, dass ein konkretes `material_id` jetzt bereits kontrolliert gesetzt
  ist
- bleibt kleiner und transparenter als fruehes Ranking oder Vollkalkulation

Nachteile:

- beruehrt Preisdomäne und braucht deshalb klare Guard Rails
- darf nicht still in automatische Kalkulation umschlagen

### Option C: anderer kleiner Mapping-Folgeschritt

Moegliche Varianten wie mehr Suchmetadaten oder Komfortanzeigen haben aktuell
nur schwaches Signal, weil der Mapping-Pfad selbst bereits geschlossen ist.

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- ein enger, expliziter Preisvorschlagspfad nach bereits gesetztem
  `material_id` direkt an genau einer Quote-Position

Dieser Schritt soll nur eine kleine operative Luecke schliessen:

- Preisfindung erleichtern, nachdem das Material bereits manuell feststeht
- die Abhaengigkeit von blindem manuellen `unit_price`-Eintrag reduzieren
- den Materialmapping-Pfad in Richtung kommerziell nutzbarer Position
  weiterfuehren

## 6. Warum jetzt der Preisvorschlag den besten Signalwert hat

Der minimale Preisvorschlag ist jetzt der bessere naechste Schritt, weil:

- der aktuelle Schmerzpunkt nicht mehr Auffindbarkeit oder Materialwahl ist
- nach gesetztem Material der naechste Bruch in der Preisfindung liegt
- Ranking den Nutzer nur frueher im Pfad unterstuetzen wuerde, waehrend der
  aktuelle manuelle Auswahlpfad bereits tragfaehig ist
- ein kleiner Preisvorschlag operativen Nutzen schafft, ohne gleich
  Vollkalkulation oder Preisautomatik zu versprechen

Damit liefert ein enger Preisvorschlag mehr Signal als weiterer Feinschliff an
Suchtreffer-Priorisierung.

## 7. Warum nicht zuerst Ranking/Priorisierung

Ein frueher Ausbau von Ranking oder Priorisierung waere jetzt zu frueh, weil:

- Kandidaten- und Suchtreffer-Auswahl bereits manuell kontrolliert moeglich ist
- der groessere operative Engpass aktuell nicht Priorisierung, sondern der
  anschliessende kommerzielle Anschluss ist
- Ranking ohne echten Folgewert schnell nur Komfortschicht bleibt
- ein spaeterer Ranking-Schritt weiterhin moeglich ist, falls Suchmengen
  wachsen oder Suchqualitaet zum Engpass wird

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- Preisvorschlag nur fuer genau eine Quote-Position mit bereits gesetztem
  `material_id`
- read-only Preisanker oder expliziter einzelner Preisvorschlag
- separate explizite Uebernahmeaktion statt stiller Preisanpassung

Wichtig bleibt:

- keine automatische Aenderung ohne explizite Aktion
- keine Vollkalkulation aus Material, Zuschlaegen und Steuern im selben Schritt
- keine Bulk-Preisvorschlaege ueber mehrere Positionen
- keine Vermischung mit Ranking- oder Suchlogik

## 9. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer den Preisvorschlag der richtige Ort, weil
dort bereits zusammenlaufen:

- Herkunftsanker
- gesetztes `material_id`
- Mapping-Status
- bestehender `unit_price`
- manuelle Verantwortung des Nutzers

Eine separate Preisoberflaeche waere fuer diesen ersten Schritt unnoetig gross
und wuerde den bewusst kleinen Pfad aufbrechen.

## 10. Nutzen schon vor spaeterer Preis- oder Kalkulationsautomatik

Schon ohne Automatik schafft ein enger Preisvorschlag:

- besseren operativen Anschluss nach erfolgreichem Materialmapping
- weniger manuelle Preisblindleistung
- belastbare Grundlage fuer spaetere Preis-, Kalkulations- oder
  Historienstufen

## 11. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Suchtreffer-Uebernahme ist nicht zuerst Ranking/Priorisierung, sondern ein
enger minimaler Preisvorschlagspfad nach bereits gesetztem Material.

## 12. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diesen engen Preisvorschlagspfad
  zuschneiden, bewusst noch ohne Ranking-, Bulk-, Historien- oder
  Automatiklogik
