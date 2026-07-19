# GAEB-Folgeausbau nach abgeschlossener Preisuebernahme: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen Preisuebernahme.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach abgeschlossenem manuellem
  Material- und Preissetzen bestimmen
- entscheiden, ob jetzt eine enge Preis-Historien-/Quellenstufe, eine kleine
  kommerzielle Priorisierung oder ein anderer Folgeschritt den besten
  Signalwert hat
- Preis-, Priorisierungs- und Automatiklogik weiterhin sauber trennen

## 1. Ausgangslage nach abgeschlossener Preisuebernahme

Der aktuelle Stand deckt bereits ab:

- manuelle Materialwahl an der Quote-Position
- read-only Preisanker nach gesetztem `material_id`
- explizite Preisuebernahme aus dem sichtbaren Preisanker
- klaren manuellen Statusraum ueber `price_mapping_status = open/manual`
- positionsnahe Arbeit im bestehenden Quote-Editor ohne separate
  Preisoberflaeche

Damit ist der kleine manuelle kommerzielle Basispfad jetzt durchgaengig:

- Material setzen
- Preisanker sichtbar machen
- Preis explizit uebernehmen

## 2. Verbleibende fachliche Luecke

Nach der Preisuebernahme bleibt vor allem diese Luecke:

- wenn kuenftig mehr als eine plausible Preisquelle existiert, wie wird der
  Preisanker nachvollziehbar erweitert, ohne sofort in Preisautomatik zu
  kippen?
- wo entsteht jetzt der groesste operative Bruch: bei der reinen
  Preisuebernahme oder bei der fehlenden Einordnung von Herkunft und
  Plausibilitaet des uebernommenen Preises?
- welcher Folgeausbau schafft Signal, ohne schon eine grosse Preisstrategie
  oder Kalkulationslogik zu versprechen?

Die relevante Luecke ist damit nicht mehr die manuelle Aktion selbst,
sondern die kleinste sinnvolle Erweiterung der Preisherkunft nach erfolgter
Uebernahme.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- automatische Preiswahl oder automatische Preisaktualisierung
- Ranking oder Scores ueber viele konkurrierende Preisquellen
- Bulk-Preislogik ueber ganze Quotes
- Vollkalkulation, Margen-, Zuschlags- oder Rabattlogik
- globale Preisarbeitsflaechen oder Preis-Cockpits

Diese Themen waeren bereits deutlich groessere Komfort-, Preisstrategie- oder
Automatikstufen.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: enge Preis-Historien-/Quellenstufe

Vorteile:

- schafft als naechstes die noetige Nachvollziehbarkeit direkt am Preis
- bleibt nah an genau einer Quote-Position
- erweitert den Preispfad, ohne schon automatische Auswahl zu versprechen

Nachteile:

- muss sehr eng zugeschnitten bleiben, damit sie nicht in Mehrquellen- oder
  Rankinglogik kippt

### Option B: kleine kommerzielle Priorisierung

Vorteile:

- koennte spaeter bei mehreren Preisquellen nuetzlich sein
- waere eine direkte Fortsetzung eines erweiterten Preisquellenbilds

Nachteile:

- setzt eigentlich schon mehrere sichtbare Preisquellen voraus
- waere vor einer kleinen Quellen-/Historienstufe fachlich zu frueh
- kippt schnell in intransparente Halbautomatik

### Option C: anderer kleiner Folgeschritt

Moegliche Varianten wie weitere UI-Hinweise oder kosmetischer Feinschliff am
gesetzten Preis haben aktuell nur schwaches Signal, weil der operative
Manuell-Pfad selbst bereits geschlossen ist.

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge Preis-Historien-/Quellenstufe direkt an genau einer bereits
  gemappten Quote-Position

Dieser Schritt soll nur eine kleine operative Luecke schliessen:

- die Herkunft des aktuell plausiblen Preisankers etwas belastbarer machen
- den Preispfad in Richtung nachvollziehbarer kommerzieller Entscheidung
  weiterfuehren
- die Grundlage fuer spaetere Priorisierung oder Mehrquellen-Logik vorbereiten

## 6. Warum jetzt die Preis-Historien-/Quellenstufe den besten Signalwert hat

Die kleine Quellenstufe ist jetzt der bessere naechste Schritt, weil:

- der aktuelle Schmerzpunkt nicht mehr das Setzen selbst ist
- nach erfolgreicher Preisuebernahme die naechste Luecke in der
  Nachvollziehbarkeit und Einordnung des Preises liegt
- Priorisierung erst dann sinnvoll wird, wenn ueberhaupt mehr als ein enger
  Herkunftsanker sichtbar ist
- eine kleine Quellenstufe operativen Nutzen schafft, ohne sofort Auswahl-,
  Ranking- oder Automatiklogik zu versprechen

Damit liefert eine enge Preis-Historien-/Quellenstufe mehr Signal als fruehe
Priorisierung.

## 7. Warum nicht zuerst Priorisierung

Eine fruehe Priorisierung waere jetzt zu frueh, weil:

- der Preispfad aktuell nur einen kleinen expliziten Preisanker kennt
- Priorisierung ohne sichtbar verbreitertes Quellenbild fachlich duenn bleibt
- die groessere operative Luecke aktuell nicht Auswahl zwischen Preisquellen,
  sondern deren kleine nachvollziehbare Erweiterung ist
- ein spaeterer Priorisierungsschritt weiterhin moeglich bleibt, falls mehrere
  Preisquellen tatsaechlich sichtbar werden

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- kleine read-only Preis-Herkunfts- oder Historienergaenzung nur fuer genau
  eine bereits gemappte Draft-Quote-Position
- weiterhin positionsnahe Anzeige im bestehenden Quote-Editor
- noch keine automatische Auswahl und noch keine stille Preisanpassung

Wichtig bleibt:

- keine Mehrquellen-Rankings im selben Schritt
- keine Bulk-Historien ueber mehrere Positionen
- keine Vermischung mit Vollkalkulation oder Automatik

## 9. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer diesen Folgepunkt der richtige Ort, weil
dort bereits zusammenlaufen:

- gesetztes `material_id`
- aktueller `unit_price`
- Preisstatus
- sichtbarer Preisanker
- manuelle Verantwortung des Nutzers

Eine separate Preis- oder Historienansicht waere fuer diesen ersten Folgepunkt
unnoetig gross und wuerde den bewusst kleinen Pfad aufbrechen.

## 10. Nutzen schon vor spaeterer Priorisierung oder Automatik

Schon ohne Priorisierung oder Automatik schafft eine kleine Quellenstufe:

- besseren Kontext fuer den bereits gesetzten Preis
- belastbarere manuelle Preisentscheidung
- Grundlage fuer spaetere Preisquellen-, Priorisierungs- oder
  Historienausbauten

## 11. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Preisuebernahme ist nicht zuerst eine kommerzielle Priorisierung, sondern eine
enge minimale Preis-Historien-/Quellenstufe.

## 12. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diese enge
  Preis-Historien-/Quellenstufe zuschneiden, bewusst noch ohne Ranking-,
  Bulk-, Kalkulations- oder Automatiklogik
