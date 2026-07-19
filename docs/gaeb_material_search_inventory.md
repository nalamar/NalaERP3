# GAEB-Materialsuche nach abgeschlossener Kandidatenauswahl: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen Kandidatenauswahl.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach sichtbaren Kandidaten bestimmen
- entscheiden, ob eine enge Materialsuche jetzt den besten Signalwert hat
- Such-, Ranking- und Automatiklogik weiterhin sauber trennen

## 1. Ausgangslage nach abgeschlossener Kandidatenauswahl

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- enger manueller Statusraum `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine read-only Materialkandidatenliste direkt an der Quote-Position
- explizite Uebernahme eines bereits sichtbaren Kandidaten auf genau eine
  Draft-Quote-Position

Damit ist der einfache Trefferfall jetzt geloest:

- wenn bereits ein sichtbarer Kandidat vorhanden ist, kann dieser direkt
  manuell uebernommen werden

## 2. Verbleibende fachliche Luecke

Nach der Kandidatenauswahl bleibt vor allem diese Luecke:

- was passiert bei uebernommenen Quote-Positionen ohne sichtbaren Kandidaten?
- was passiert, wenn sichtbare Kandidaten vorhanden sind, aber keiner davon
  fachlich passt?
- wie bleibt das manuelle Mapping nutzbar, ohne dass der Nutzer Material-IDs
  blind kennen oder extern suchen muss?

Die relevante Luecke ist damit nicht mehr Transparenz und nicht mehr die
Uebernahme vorhandener Kandidaten, sondern der kleinste Suchpfad fuer
Faelle ohne passenden sichtbaren Treffer.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Kandidatenranking oder Relevanzbewertung
- automatische Vorbelegung eines Suchtreffers
- Bulk-Suche oder Bulk-Mapping ueber ganze Quotes
- Preisvorschlaege aus Material oder Historie
- KI-gestuetztes Freitext-Matching
- globale Mapping- oder Suchkonsole

Diese Themen waeren bereits deutlich groessere Matching-, Komfort- oder
Automatikstufen.

## 4. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge manuelle Materialsuche direkt an der bestehenden Quote-Position

Diese Suche soll nur eine kleine operative Luecke schliessen:

- Material finden, wenn kein sichtbarer Kandidat vorhanden ist
- Material finden, wenn der sichtbare Kandidatensatz nicht ausreicht
- das Ergebnis weiter explizit manuell auf genau eine Position uebernehmen

## 5. Warum jetzt Materialsuche den besten Signalwert hat

Die enge Materialsuche ist jetzt der bessere naechste Schritt, weil:

- der direkte Kandidatenpfad bereits abgedeckt ist
- der verbleibende Schmerzpunkt jetzt die Auffindbarkeit passender
  Materialien ist
- die bestehende manuelle Eingabe von `material_id` zwar technisch moeglich,
  aber operativ zu roh bleibt
- Materialsuche eine echte Nutzungsluecke schliesst, ohne schon Ranking oder
  Automatik zu versprechen

Damit liefert die Suche mehr Signal als weiterer Feinschliff an
Kandidatenstatus oder Kandidatenliste.

## 6. Warum nicht zuerst Ranking oder bessere Kandidatenlogik

Ein frueher Ausbau von Ranking oder Matchinglogik waere jetzt zu frueh, weil:

- es bereits einen sauberen Pfad fuer einfache exakte Treffer gibt
- der groessere operative Engpass aktuell nicht Priorisierung, sondern
  Auffindbarkeit ist
- Ranking ohne kontrollierten Suchpfad schnell in intransparente
  Halbautomatik kippt
- ein Suchpfad spaeter auch bessere Kandidaten- oder Matchingstufen sauber
  vorbereiten kann

## 7. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- eine kleine Suchaktion direkt an einer Quote-Position mit offenem
  Materialmapping
- enge Trefferliste aus bestehenden Materialien
- explizite manuelle Uebernahme eines Suchtreffers auf genau eine Position

Wichtig bleibt:

- keine automatische Auswahl
- keine Rankingversprechen
- keine globale Suchoberflaeche
- keine Preislogik im selben Schritt

## 8. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer die Suche der richtige Ort, weil dort
bereits zusammenlaufen:

- Herkunftsanker
- Mapping-Status
- sichtbare Kandidaten
- manuelle Materialzuordnung

Eine separate Such- oder Mapping-Welt waere fuer diesen ersten Suchschritt
unnoetig gross und wuerde den bewusst kleinen Pfad aufbrechen.

## 9. Nutzen schon vor spaeterem Ranking oder Automatik

Schon ohne Ranking oder Automatik schafft eine enge Materialsuche:

- nutzbares manuelles Mapping auch fuer Nicht-Trefferfaelle
- weniger Abhaengigkeit von direkter Material-ID-Eingabe
- klaren Anschluss an den bereits vorhandenen Kandidatenpfad
- belastbare Grundlage fuer spaetere Matching-, Ranking- oder
  Preisvorschlagsstufen

## 10. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Kandidatenauswahl ist nicht weiterer Feinschliff an sichtbaren Kandidaten und
auch nicht sofort Ranking oder Automatik, sondern zuerst eine enge manuelle
Materialsuche fuer Quote-Positionen ohne passenden sichtbaren Treffer.

## 11. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diese enge Materialsuche
  zuschneiden, bewusst noch ohne Ranking-, Bulk- oder Automatiklogik
