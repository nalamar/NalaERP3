# GAEB-Read-only Materialkandidatenliste: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach dem
abgeschlossenen read-only Materialkandidaten- bzw. Vorschlagsanker.

Fokus bleibt bewusst eng:

- kleinsten sinnvollen Einstiegspunkt fuer eine read-only
  Materialkandidatenliste festlegen
- noch ohne Materialsuche, Rankinglogik oder automatische Uebernahme
- auf Basis des bereits vorhandenen Herkunfts-, Mapping- und
  Vorschlagsankers

## 1. Ausgangslage nach read-only Vorschlagsanker

Der aktuelle Stand deckt bereits ab:

- `accepted import item -> created quote item` ist persistent verknuepft
- Quote-Positionen besitzen optionales `material_id`
- Quote-Positionen besitzen `price_mapping_status = open/manual`
- Quote-Positionen besitzen read-only `material_candidate_status`
  (`none/available`)
- der bestehende Quote-Editor macht den Kandidatenstatus sichtbar

Damit ist bereits sichtbar, ob eine Position fuer spaetere Kandidaten
grundsaetzlich geeignet ist. Noch nicht sichtbar ist jedoch, **welche**
konkreten Kandidaten spaeter denkbar waeren.

## 2. Verbleibende fachliche Luecke

Nach dem Statusanker fehlt vor allem:

- eine kleine read-only Sicht auf konkrete Materialkandidaten
- nachvollziehbare Trennung zwischen:
  - Position ohne Kandidaten
  - Position mit Kandidatenpotenzial
  - Position mit wenigen konkreten Kandidaten
- eine Vorstufe vor spaeterer Suche oder manuellem Auswaehlen aus Kandidaten

Die Luecke ist damit nicht mehr generelle Eignung, sondern erste konkrete
Kandidaten-Transparenz.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Volltext- oder Materialstammsuche
- Ranking mit groesserer Matchinglogik
- automatische Vorbelegung von `material_id`
- automatische Preislogik
- Bulk-Vorschlaege ueber die ganze Quote
- neue globale Vorschlags- oder Mapping-Konsole

Diese Punkte waeren bereits eine groessere Such- oder Automatikstufe.

## 4. Kleinster sinnvoller Einstiegspunkt

Der kleinste sinnvolle Folgepunkt ist:

- zuerst eine kleine read-only Materialkandidatenliste direkt an der
  Quote-Position sichtbar machen

Der Einstiegspunkt soll nur beantworten:

- gibt es fuer diese Position konkrete Kandidaten?
- wie klein ist diese Kandidatenmenge?
- lassen sich erste Kandidaten transparent anzeigen, ohne bereits eine
  Auswahlentscheidung zu treffen?

## 5. Warum eine kleine Kandidatenliste vor Suche oder Ranking kommen sollte

Die kleine Kandidatenliste ist der bessere naechste Schritt, weil:

- sie den vorhandenen Statusanker sinnvoll konkretisiert
- sie spaetere Such- und Rankinglogik fachlich entkoppelt vorbereitet
- sie operativ mehr Signal liefert als nur `available/none`
- sie weiterhin read-only bleiben kann

So entsteht konkrete Transparenz, ohne dass bereits Matchingqualitaet oder
Interaktion versprochen wird.

## 6. Empfohlenes Minimalzielbild

Das Minimalzielbild fuer den naechsten Strang ist:

- eine kleine read-only Kandidatenliste an der Quote-Position
- bewusst mit sehr kleinem Umfang, z. B. nur wenige Material-IDs oder kleine
  Anzeigeanker
- keine direkte Auswahl- oder Uebernahmeaktion

Wichtig ist, dass die Kandidatenliste zunaechst nicht als fachlich
„bester Treffer“ verkauft wird, sondern als transparente Kandidatenvorschau.

## 7. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt der richtige Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- manuelles Materialmapping
- Preisstatus
- read-only Kandidatenstatus

Eine parallele Kandidatenliste auf Import-Ebene oder in einem separaten
Screen wuerde den Flow frueh unnötig aufsplitten.

## 8. Read-only Nutzen schon vor spaeterer Interaktion

Schon ohne Auswahlfunktion schafft eine kleine Kandidatenliste:

- bessere Priorisierung offener Mapping-Positionen
- mehr Nachvollziehbarkeit, warum eine Position als Kandidatenfall gilt
- bessere Vorbereitung fuer spaetere Materialsuche oder Ranking
- sauberere Trennung zwischen Transparenzstufe und Interaktionsstufe

## 9. Entscheidung

Der naechste sinnvolle Folgeausbau nach dem read-only Vorschlagsanker ist
nicht sofort Suche, Ranking oder Automatik, sondern zuerst eine kleine
read-only Materialkandidatenliste direkt an der uebernommenen Quote-Position.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die read-only
  Materialkandidatenliste zuschneiden, bewusst noch ohne Such-,
  Ranking- oder Uebernahmelogik
