# GAEB-Materialkandidaten und Vorschlagsanker: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach dem
abgeschlossenen manuellen Preis-/Material-Mapping-Einstieg zu.

Der Scope bleibt bewusst eng:

- erster read-only Kandidaten- und Vorschlagsanker
- Fokus auf spaetere Materialkandidaten, nicht auf Preisautomatik
- keine automatische Uebernahme und kein neuer Massen-Workflow

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- `accepted import item -> created quote item` ist persistent verknuepft
- die Herkunft ist im Importpositionsdialog sichtbar
- Quote-Positionen besitzen optional `material_id`
- Quote-Positionen besitzen den kleinen Statusraum
  `price_mapping_status = open/manual`
- manueller Mapping-Einstieg ist im bestehenden Quote-Editor moeglich

Damit existiert jetzt ein manueller Anker, aber noch kein technisches Modell
fuer spaetere Vorschlaege oder Kandidaten.

## 2. Kernentscheidung

Der erste Vorschlagsblock soll **nicht** selbst Material oder Preise setzen.

Der kleinste sinnvolle technische Schnitt ist:

- zunaechst nur ein kleiner read-only Kandidatenanker
- bevorzugt fuer moegliche Materialkandidaten an bereits uebernommenen
  Quote-Positionen

## 3. Warum Materialkandidaten vor Preisvorschlaegen kommen sollten

Material ist der bessere erste Vorschlagsanker, weil:

- Material der stabilere strukturelle Bezugspunkt ist
- Preisvorschlaege haeufig erst nach Materialbezug belastbar werden
- Fehlvorschlaege bei Material leichter transparent gemacht werden koennen als
  direkte Preisuebernahmen

## 4. Empfohlenes Minimalzielbild

Der erste Vorschlagsanker soll nur diese Fragen beantwortbar machen:

- ist eine Quote-Position fuer spaetere Materialkandidaten ueberhaupt
  geeignet?
- gibt es fuer diese Position bereits kleine read-only Kandidatenhinweise?
- bleibt die finale Zuordnung weiterhin voll manuell?

Bewusst noch nicht Teil dieses Schritts:

- Suche ueber den gesamten Materialstamm
- Ranking mit komplexer Matchinglogik
- automatische Vorbelegung von `material_id`
- automatische Aenderung von `unit_price`

## 5. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist ein kleiner read-only Vorschlagsanker direkt
an der Quote-Position.

Sinnvolle Minimalfelder:

- `material_candidate_status`
- optional eine kleine read-only Kandidatenliste mit wenigen IDs oder
  Anzeigeankern

Empfohlener minimaler Statusraum:

- `none`
- `available`

Damit ist sichtbar, ob spaeter Kandidaten vorhanden sind, ohne bereits eine
automatische Entscheidung zu behaupten.

## 6. Warum der Vorschlagsanker an der Quote-Position liegen sollte

Die Quote-Position ist erneut der richtige Ort, weil:

- dort bereits manuelles Mapping stattfindet
- dort Herkunft, Materialbezug und Preisstatus zusammenlaufen
- keine parallele Vorschlagswelt auf Import-Ebene aufgebaut werden muss

## 7. Was ein Kandidat in diesem ersten Schritt bewusst nicht ist

Ein Kandidat ist im ersten Schritt nur ein Hinweisanker, nicht:

- ein bestätigtes Mapping
- ein automatisch gesetztes Material
- eine Preisfreigabe
- eine fachliche Aussage ueber beste Eignung

## 8. Guard Rails

Der erste Vorschlagsanker sollte mindestens diese Regeln einhalten:

- read-only bleiben
- vorhandenes manuelles Mapping niemals ueberschreiben
- keine Aenderung an `material_id`, `unit_price` oder `price_mapping_status`
  ausloesen
- nur auf Quote-Positionen mit vorhandenem Herkunfts- bzw. Mapping-Anker
  aufsetzen
- keine globale Bulk- oder Auto-Apply-Funktion mit in denselben Block ziehen

## 9. Read-only Anschlussnutzen

Schon ohne automatische Uebernahme schafft dieser kleine Schritt:

- sichtbare Trennung zwischen rein manueller Position und Position mit
  spaeterem Vorschlagspotenzial
- Grundlage fuer spaetere Materialsuche oder Kandidatenranking
- bessere Priorisierung offener Mapping-Arbeit

## 10. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- automatische Materialuebernahme
- Preisvorschlaege aus Material oder Historie
- KI- oder Freitext-Matching
- Bulk-Vorschlaege fuer ganze Quotes oder Importlaeufe
- Rueckschreiben von Vorschlaegen in Importdaten

## 11. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer einen read-only Materialkandidaten- oder
  Vorschlagsanker vorbereiten, bewusst noch ohne automatische Uebernahme
