# GAEB-Read-only Materialkandidatenliste: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach dem
abgeschlossenen read-only Materialkandidaten- bzw. Vorschlagsanker zu.

Der Scope bleibt bewusst eng:

- erste kleine read-only Materialkandidatenliste
- direkt an uebernommenen Quote-Positionen
- ohne Suche, Ranking, Auswahlaktion oder automatische Uebernahme

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine Sichtbarkeit dieses Status im bestehenden Quote-Editor

Damit ist bereits sichtbar, ob Kandidatenpotenzial existiert. Noch nicht
sichtbar ist, welche wenigen Kandidaten konkret als erste read-only Vorschau
angezeigt werden koennen.

## 2. Kernentscheidung

Der naechste Block soll **nicht** nach Materialien suchen und **nicht**
automatisch etwas setzen.

Der kleinste sinnvolle technische Schnitt ist:

- eine kleine read-only Kandidatenliste direkt an der Quote-Position
- mit sehr kleinem Umfang
- nur als Transparenzanker, nicht als Auswahl- oder Matchingsystem

## 3. Empfohlenes Minimalzielbild

Die erste Materialkandidatenliste soll nur diese Fragen beantwortbar machen:

- gibt es fuer diese Quote-Position konkrete Kandidaten?
- wie viele wenige Kandidaten werden angezeigt?
- welche kleinen Anzeigeanker koennen dem Nutzer spaeter helfen, ohne schon
  einen besten Treffer zu behaupten?

Bewusst noch nicht Teil dieses Schritts:

- Volltextsuche im Materialstamm
- Matching- oder Rankingalgorithmus
- Auswahl- oder Uebernahmeaktion
- automatische Befuellung von `material_id`
- Preisvorschlag aus Kandidaten

## 4. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist eine read-only Liste mit wenigen
Kandidatenankern direkt in der Quote-Position.

Sinnvolle Minimalfelder pro Kandidat:

- `material_id`
- optional `material_no`
- optional `label` oder kurze Materialbezeichnung

Sinnvolle Minimalgrenze:

- nur wenige Kandidaten, z. B. maximal drei

Damit bleibt die Liste klein, transparent und klar als Vorschau lesbar.

## 5. Warum nur wenige Kandidaten gezeigt werden sollten

Der erste Schritt sollte bewusst klein bleiben, weil:

- sonst implizit bereits Suche oder Ranking versprochen wird
- eine grosse Liste ohne Interaktion wenig Signal liefert
- die Kandidatenliste eine Vorschau sein soll, keine neue Arbeitsflaeche

Eine sehr kleine Liste hat den besten Signalwert fuer diesen fruehen Ausbau.

## 6. Warum die Liste read-only an der Quote-Position liegen sollte

Die Quote-Position bleibt der richtige Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- manuelles Materialmapping
- Preisstatus
- Kandidatenstatus

Die Kandidatenliste ist damit eine direkte Konkretisierung des vorhandenen
Statusankers und benoetigt keinen neuen Screen.

## 7. Was ein Kandidat in diesem Schritt bewusst nicht bedeutet

Ein Kandidat ist in diesem Schritt nur:

- ein kleiner Anzeigeanker
- ein moeglicher spaeterer Folgehinweis

Ein Kandidat ist noch nicht:

- bestaetigtes Mapping
- bester Treffer
- Suchergebnis mit fachlicher Rangfolge
- automatische Vorbelegung

## 8. Guard Rails

Die erste read-only Materialkandidatenliste sollte mindestens diese Regeln
einhalten:

- read-only bleiben
- vorhandenes `material_id` niemals veraendern
- vorhandenes manuelles Mapping nicht ueberschreiben
- keine Aenderung an `price_mapping_status` ausloesen
- nur auf Quote-Positionen mit vorhandenem Herkunfts- bzw. Vorschlagsanker
  aufsetzen
- keinen Suchdialog oder Auswahlworkflow in denselben Block ziehen

## 9. Read-only Anschlussnutzen

Schon ohne Auswahlfunktion schafft diese kleine Liste:

- konkretisierte Transparenz fuer offene Mapping-Positionen
- bessere Vorbereitung fuer spaetere Materialsuche oder Ranking
- klareren Unterschied zwischen `available` ohne Details und echten sichtbaren
  Kandidaten

## 10. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- Materialsuche
- Ranking oder Matchinglogik
- Auswahlbutton oder Uebernahmeaktion
- Preislogik auf Basis von Kandidaten
- Bulk-Kandidaten fuer ganze Quotes

## 11. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer eine read-only Materialkandidatenliste
  vorbereiten, bewusst noch ohne Such-, Ranking- oder Auswahlaktion
