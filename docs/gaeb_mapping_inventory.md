# GAEB-Mapping nach abgeschlossenem Import-/Review-/Apply-Pfad: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbau nach dem
abgeschlossenen GAEB-Pfad aus:

- Upload
- Parse-Speicher
- Read-only-Sicht
- Positions-Review
- kontrollierter Quote-Uebernahme
- schlanker Apply-Transparenz

Fokus ist bewusst eng:

- den kleinsten sinnvollen Mapping-Einstiegspunkt identifizieren
- ohne sofortige Preis-, Steuer- oder Materialautomatik

## 1. Ausgangslage nach abgeschlossenem Apply-Pfad

Der aktuelle Stand deckt bereits ab:

- `uploaded -> parsed -> reviewed -> applied`
- neue Draft-Quote nur aus `accepted`-Positionen
- erste Rueckverfolgbarkeit ueber `created_quote_id`
- kleine Transparenz fuer `accepted`, `rejected`, `pending`

Die fachliche Kernluecke liegt damit nicht mehr im Importlauf selbst, sondern
im Anschluss an die erzeugte Quote.

## 2. Was heute bei der Uebernahme bewusst noch roh bleibt

Die erste Quote-Uebernahme ist bewusst minimal:

- `description` wird uebernommen
- `qty` wird uebernommen
- `unit` wird uebernommen
- `unit_price = 0`
- `tax_code = ''`

Damit wird zwar eine bearbeitbare Draft-Quote erzeugt, aber noch keine
brauchbare strukturierte Bruecke in Preis-, Steuer- oder Materiallogik.

## 3. Verbleibende fachliche Luecke

Nach erfolgreichem Apply fehlt aktuell vor allem:

- nachvollziehbar, welche Quote-Position aus welcher akzeptierten
  Importposition entstanden ist
- ein kleiner Anker fuer spaeteres Material- oder Preis-Mapping
- eine kontrollierte Bruecke zwischen GAEB-Rohdaten und Angebotsposition

Diese Luecke ist groesser als reine Transparenz, aber noch kleiner als echte
Automatik.

## 4. Was jetzt bewusst nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- automatische Preisermittlung
- automatische Steuercodeableitung
- automatisches Materialmatching gegen `materials`
- KI-Vorschlaege fuer Leistungen oder Materialien
- Bulk-Mapping ueber ganze Importlaeufe

Diese Themen sind spaeter sinnvoll, aber fuer den ersten Mapping-Einstieg zu
gross und zu fehleranfaellig.

## 5. Kleinster sinnvoller Einstiegspunkt

Der kleinste sinnvolle Folgepunkt ist:

- zuerst die Rueckverfolgbarkeit `accepted import item -> created quote item`
  als kleiner Mapping-Anker sauber inventarisieren und technisch vorbereiten

Warum genau dieser Schnitt:

- er bleibt noch nahe an der bereits erzeugten Draft-Quote
- er schafft die Voraussetzung fuer spaeteres Mapping, ohne schon Logik
  vorzutäuschen
- er verbessert die Nachvollziehbarkeit der Uebernahme sofort

## 6. Empfohlenes Minimalziel fuer den naechsten Strang

Der naechste Mapping-Strang sollte zunaechst nicht auf Preise oder Materialien
zielen, sondern auf:

- eine kleine Herkunftsverknuepfung zwischen Importposition und Quote-Position
- optional spaeter sichtbare Herkunftsinformation im Quote-Kontext

Erst auf dieser Basis sind spaeter sinnvoll:

- Materialvorschlaege
- Preisvorschlaege
- Steuer-/Kontierungsableitung

## 7. Warum dieser Einstieg den besten Signalwert hat

- minimaler Eingriff in den bestehenden Angebotsprozess
- keine fachlich riskante Automatik
- schafft die Grundvoraussetzung fuer spaetere Mapping-Schritte
- reduziert Black-Box-Wirkung der GAEB-Uebernahme

## 8. Entscheidung

Der naechste sinnvolle GAEB-Folgeabschnitt ist nicht sofort Preis- oder
Materialautomatik, sondern zuerst ein kleiner Herkunfts- und Mapping-Anker
zwischen akzeptierter Importposition und erzeugter Quote-Position.

## 9. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diesen ersten
  Herkunfts-/Mapping-Anker zuschneiden, bewusst noch ohne Preis-,
  Steuer- oder Materialautomatik.
