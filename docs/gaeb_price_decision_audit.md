# GAEB-Preisentscheidung: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit schliesst den engen Block der expliziten Preisuebernahme aus
der primaeren priorisierten Quelle ab.

Geprueft wird bewusst nur:

- ob der umgesetzte Pfad fachlich klein genug geblieben ist
- ob Backend und Client dieselbe explizite Nutzerentscheidung abbilden
- ob vor Bulk, Margen-, Zuschlags-, Rabatt-, Freigabe- oder Automatiklogik
  noch ein weiterer kleiner Haertungsschritt mit gutem Signal uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- Materialzuordnung an einer Angebotsposition
- manuelle Preisuebernahme aus dem Materialdurchschnitt
- Materialpreisquellen
- Priorisierung dieser Quellen
- read-only Preisbewertung gegen die primaere Quelle

Die danach identifizierte kleine Luecke war nicht weitere Sichtbarkeit,
sondern eine kontrollierte Entscheidung:

- den Preis der primaeren priorisierten Quelle explizit als Positionspreis
  uebernehmen

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt auf genau eine Angebotsposition begrenzt:

- genau eine Draft-Quote-Position pro Anfrage
- serverseitige Neubestimmung der primaeren Quelle
- Uebernahme von `primary_source.unit_price` nach `quote_items.unit_price`
- Setzen von `price_mapping_status = manual`
- Aktualisierung von Positions- und Quote-Summen im gleichen Ablauf
- Rueckgabe der aktualisierten Quote
- Client-Aktion direkt im bestehenden Quote-Editor

Damit ist die Nutzerentscheidung fachlich explizit und technisch
transaktional gekapselt.

## 3. Guard Rails

Die Guard Rails passen zum bestehenden Angebots- und Preis-Mapping-Modell:

- historische Quote-Versionen werden nicht veraendert
- nur Draft-Quotes duerfen veraendert werden
- die Position muss zur Quote gehoeren
- die Position muss ein gesetztes Material besitzen
- die primaere Preisquelle wird nicht vom Client geliefert, sondern erneut im
  Backend bestimmt
- ohne primaere Preisquelle bricht der Pfad fachlich ab

Wichtig ist dabei, dass der Client keine Preisquelle als Wahrheit an das
Backend uebergibt. Die UI loest nur die Entscheidung aus; die Quelle wird
serverseitig anhand der bestehenden Priorisierung bestimmt.

## 4. Bewusst nicht umgesetzt

Folgende Themen bleiben weiterhin ausserhalb dieses Blocks:

- Bulk-Uebernahme mehrerer Positionen
- automatische Preisuebernahme nach Bewertung
- Margen- oder Zuschlagsberechnung
- Rabattlogik
- Freigabe- oder Eskalationsworkflow
- separate Preisentscheidungsakte
- globale Preisarbeitsflaeche

Diese Abgrenzung ist korrekt, weil jedes dieser Themen einen eigenen
fachlichen Zustand oder eine neue kommerzielle Regel einfuehren wuerde.

## 5. Ergebnis im Backend

Der Backend-Pfad ist ausreichend eng:

- `ApplyPrimaryPriceSourceForQuoteItem(...)` bildet genau die eine
  Entscheidung ab
- `primaryPriceSourceForMaterialTx(...)` kapselt die serverseitige
  Primarquellenbestimmung innerhalb der Transaktion
- der API-Endpunkt
  `POST /api/v1/quotes/{id}/items/{itemID}/apply-primary-price-source`
  benoetigt keinen Request-Body
- die Integrationstests pruefen Erfolg und fehlendes Material

Die Quote-Summen werden nach der Positionsaenderung konsistent neu
berechnet. Damit entsteht kein Zwischenzustand, in dem Position und
Angebotskopf fachlich auseinanderlaufen.

## 6. Ergebnis im Client

Der Client spiegelt die Entscheidung ausreichend positionsnah:

- die Aktion ist im Block `Preisbewertung` sichtbar
- der Button `Primaerpreis uebernehmen` ist explizit benannt
- waehrend der Aktion wird nur die betroffene Position blockiert
- nach Erfolg wird der Dialog aus der aktualisierten Quote-Response neu
  aufgebaut

Damit bleibt die Bedienung an der Stelle, an der die Bewertung bereits
sichtbar ist. Es entsteht keine zweite Preisarbeitsflaeche.

## 7. Audit-Entscheidung

Der Block ist abgeschlossen.

Innerhalb dieses engen Preisentscheidungsblocks gibt es keinen weiteren
kleinen Haertungsschritt mit gutem Signal, der vor Bulk, Margen-, Zuschlags-,
Rabatt-, Freigabe- oder Automatiklogik noch sinnvoll waere.

Moegliche Zusatzschritte wie weitere Hinweistexte, ein zusaetzlicher Dialog
oder reine UI-Kosmetik wuerden den fachlichen Pfad kaum haerten. Die naechsten
wirklich wertvollen Schritte liegen bereits in einem neuen Block:

- kalkulationsnaher Margen- oder Zuschlagsanker
- Preisentscheidungs-Transparenz bzw. Historisierung
- kommerzielle Freigabe- oder Abweichungslogik
- spaetere Bulk- und Automatikfunktionen

## 8. Naechster sinnvoller Schritt

Der naechste Schritt soll deshalb nicht mehr in dieser Preisuebernahme
liegen. Sinnvoll ist eine neue fachliche Inventur nach abgeschlossener
expliziter Preisentscheidung:

- welcher minimale kommerzielle Folgeschritt hat den hoechsten Signalwert?

Die naheliegenden Kandidaten sind:

- ein kalkulationsnaher Margen-/Zuschlagsanker
- eine kleine Preisentscheidungs-Transparenz
- ein minimaler Freigabe- oder Abweichungsanker

