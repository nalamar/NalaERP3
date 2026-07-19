# GAEB-Preisentscheidungs-Transparenz:
# Minimalzielbild und technischer Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das Minimalzielbild fuer die naechste kleine
Ausbaustufe nach der abgeschlossenen expliziten Preisuebernahme aus der
primaeren priorisierten Quelle zu.

Der Fokus bleibt bewusst eng:

- eine read-only Transparenz ueber den aktuellen Positionspreis gegen die
  aktuelle primaere Quelle bereitstellen
- den bestehenden Preisbewertungs- und Primarquellenpfad wiederverwenden
- keine neue Persistenz, keine Marge, keinen Zuschlag, keinen Rabatt, keine
  Freigabe, keine Bulk-Funktion und keine Automatik einfuehren

## 1. Ausgangspunkt

Der aktuelle Stand bietet bereits:

- gesetztes `material_id` an der Quote-Position
- aktuellen `unit_price` an der Quote-Position
- read-only Preisquellenanzeige
- read-only Quellen-Priorisierung
- read-only Preisbewertung gegen die primaere Quelle
- explizite Uebernahme der primaeren Quelle als Positionspreis

Die verbleibende kleine Luecke ist damit nicht mehr:

- welche Quelle primaer ist
- wie gross die Abweichung zur primaeren Quelle ist
- ob der Nutzer die primaere Quelle uebernehmen kann

Die verbleibende Luecke ist:

- ist der aktuelle Positionspreis aus Sicht der aktuellen primaeren Quelle
  noch nachvollziehbar als Preisentscheidung?

## 2. Fachliches Minimalziel

Das Minimalziel ist eine kleine read-only Transparenz fuer genau eine bereits
gemappte Quote-Position.

Sie soll nur beantworten:

- welcher aktuelle Positionspreis gespeichert ist
- welche aktuelle primaere Quelle dagegensteht
- ob beide Werte im Cent-Toleranzbereich uebereinstimmen
- welche absolute und relative Abweichung besteht
- welcher kleine Entscheidungsstatus daraus folgt

Sie soll bewusst noch nicht beantworten:

- welche Marge angewendet werden soll
- ob ein Zuschlag oder Rabatt gilt
- ob eine Freigabe notwendig ist
- ob die Entscheidung historisch dokumentiert werden muss
- ob mehrere Positionen gleichzeitig bewertet werden sollen

## 3. Semantik gegenueber der bestehenden Preisbewertung

Die bestehende Preisbewertung (`PriceEvaluationForQuoteItem(...)`) beschreibt
das Verhaeltnis:

- aktueller Positionspreis gegen primaere Kostenbasis

Die neue Transparenz soll auf denselben Daten aufbauen, aber eine andere
fachliche Frage ausdruecken:

- passt der aktuelle Positionspreis noch zur aktuell primaeren
  Preisentscheidung?

Damit ist die erste technische Stufe bewusst nah an der bestehenden
Preisbewertung. Sie soll keine zweite Rechenlogik erfinden.

## 4. Zielbild im Backend

Das Backend soll einen engen read-only Pfad bereitstellen, der:

- genau eine Quote-Position adressiert
- dieselben Guard Rails wie `PriceEvaluationForQuoteItem(...)` nutzt
- die bestehende primaere Quellenlogik wiederverwendet
- keine Quote-Position veraendert
- keinen neuen Datensatz schreibt
- eine kleine Transparenz-Response zurueckgibt

Der vorgeschlagene Service-Einstieg lautet:

- `PriceDecisionTransparencyForQuoteItem(ctx, quoteID, itemID) (*PriceDecisionTransparency, error)`

Der vorgeschlagene API-Endpunkt lautet:

- `GET /api/v1/quotes/{id}/items/{itemID}/price-decision-transparency`

Permission:

- analog zu den bestehenden read-only Preisankern im Quote-Editor
- aktuell konsistent mit dem lokalen Muster: `quotes.write`, solange der
  Editor diese Preisfunktionen insgesamt unter Schreibberechtigung fuehrt

## 5. Zielbild der Rueckgabe

Die Rueckgabe soll bewusst klein bleiben:

- `current_unit_price`
- `currency`
- `primary_source_label`
- `primary_source_unit_price`
- `primary_source_reference`
- `primary_source_date`
- `absolute_delta`
- `relative_delta_percent`
- `decision_status`
- `decision_reason`

Die erste Statusmenge bleibt klein:

- `matches_primary_source`
- `differs_from_primary_source`
- `no_primary_source`

In der ersten technischen Umsetzung kann `no_primary_source` auch als
fachlicher Fehler abgebildet werden, wenn das besser zum bestehenden
Fehlerverhalten passt. Wichtig ist nur, dass daraus keine stille
Fallback-Kalkulation entsteht.

## 6. Erste enge Entscheidungsregel

Die Transparenzregel soll auf derselben Cent-Toleranz wie die bestehende
Preisbewertung beruhen:

- `absolute_delta = current_unit_price - primary_source_unit_price`
- `relative_delta_percent = absolute_delta / primary_source_unit_price * 100`,
  falls `primary_source_unit_price > 0`
- `matches_primary_source`, wenn `absolute_delta` innerhalb der Cent-Toleranz
  liegt
- `differs_from_primary_source`, wenn `absolute_delta` ausserhalb der
  Cent-Toleranz liegt

Beispielgruende:

- `Aktueller Preis entspricht der primaeren Preisquelle`
- `Aktueller Preis weicht von der primaeren Preisquelle ab`

Diese Regel ist absichtlich keine Marge und kein Zuschlag. Sie beschreibt nur
Uebereinstimmung oder Abweichung.

## 7. Guard Rails

Die Guard Rails sollen eng am bestehenden Preisbewertungspfad bleiben:

- Quote muss existieren
- keine historischen Quote-Versionen
- nur Draft-Quotes im bestehenden Editorfluss
- Position muss existieren und zur Quote gehoeren
- Position muss ein gesetztes `material_id` besitzen
- primaere Quelle muss bestimmbar sein
- Pfad bleibt read-only

Wenn keine primaere Quelle bestimmbar ist, soll der Pfad klar abbrechen oder
einen engen Status liefern. Er darf keine Ersatzquelle erfinden.

## 8. Zielbild im Client

Der Client soll die Transparenz direkt an der Quote-Position anzeigen:

- im bestehenden Quote-Editor
- nahe am Block `Preisbewertung`
- weiterhin read-only
- ohne neuen Dialog und ohne neue Arbeitsflaeche

Sichtbar werden soll nur:

- aktueller Preis
- primaere Quelle
- Abweichung
- Status `Entspricht primaerer Quelle` oder `Weicht von primaerer Quelle ab`

Bewusst nicht sichtbar werden:

- Margenfelder
- Zuschlagsfelder
- Freigabeschalter
- automatische Preisanpassungen
- Bulk-Aktionen

## 9. Warum keine neue Persistenz

Eine separate Preisentscheidungsakte waere spaeter sinnvoll, aber fuer diesen
Schritt zu gross.

Gruende:

- der aktuelle Pfad hat noch keine Freigabe- oder Verantwortlichkeitslogik
- es gibt noch keinen Zielmargen- oder Kalkulationskontext
- die erste Transparenz kann aus vorhandenen Daten berechnet werden
- neue Persistenz wuerde sofort Fragen nach Historie, Nutzer, Zeitpunkt,
  Revision und Audit-Sicherheit oeffnen

Diese Fragen sollen erst beantwortet werden, wenn der fachliche Bedarf dafuer
konkreter ist.

## 10. Warum diese Stufe noch keine Kalkulation ist

Diese Stufe bleibt bewusst kleiner als Kalkulation, weil:

- nur ein vorhandener Positionspreis verglichen wird
- keine Zielmarge entsteht
- kein Verkaufspreisvorschlag berechnet wird
- kein Zuschlag und kein Rabatt angewendet wird
- kein Freigabestatus entsteht
- keine Angebotsgesamtbetrachtung entsteht

## 11. Minimaler Nutzen

Schon in diesem engen Zuschnitt schafft die Transparenz:

- bessere Nachvollziehbarkeit der expliziten Preisuebernahme
- schnelle Erkennung spaeterer manueller Abweichungen
- klareren Anschluss fuer spaetere Margen- und Freigabelogik
- weniger fachliche Ueberladung des bestehenden Preisbewertungsblocks

## 12. Entscheidung

Das minimale technische Zielmodell fuer die naechste Stufe ist:

- ein enger read-only Transparenzanker fuer genau eine Quote-Position, der
  die aktuelle primaere Quelle mit dem aktuellen Positionspreis vergleicht
  und daraus einen kleinen Entscheidungsstatus ableitet

## 13. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Schritt:

- die erste Backend-Stufe fuer
  `PriceDecisionTransparencyForQuoteItem(...)` vorbereiten, bewusst noch ohne
  neue Persistenz, Margen-, Zuschlags-, Rabatt-, Freigabe-, Bulk- oder
  Automatiklogik

