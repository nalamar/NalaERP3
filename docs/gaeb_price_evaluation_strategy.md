# GAEB-Preisbewertung nach Quellen-Priorisierung:
# Minimalzielbild und technischer Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das Minimalzielbild fuer die naechste kleine
Ausbaustufe nach der abgeschlossenen Quellen-Priorisierung zu.

Der Fokus bleibt bewusst eng:

- aktuellen Positionspreis gegen die primaere priorisierte Preisquelle
  bewerten
- die Bewertung read-only und positionsnah im bestehenden Quote-Editor halten
- automatische Preisentscheidung, Bulk, Vollkalkulation, Margen-, Zuschlags-
  und Freigabelogik weiterhin strikt ausklammern

## 1. Ausgangspunkt

Der aktuelle Stand bietet bereits:

- gesetztes `material_id` an der Quote-Position
- aktuellen `unit_price` an der Quote-Position
- read-only Preisquellenanzeige
- read-only Quellen-Priorisierung mit:
  - `priority_rank`
  - `priority_reason`
  - `source_label`
  - `unit_price`
  - `currency`

Die naechste kleine Luecke ist damit nicht mehr Herkunft oder Einordnung,
sondern Bewertung:

- wie steht der aktuell gesetzte Positionspreis zur primaeren Kostenquelle?

## 2. Fachliches Minimalziel

Das Minimalziel ist:

- eine enge read-only Preisbewertung fuer genau eine bereits gemappte
  Draft-Quote-Position

Diese Bewertung soll nur beantworten:

- welche Quelle aktuell als primaere Kostenbasis gilt
- welcher aktuelle Positionspreis dagegensteht
- wie gross die absolute und relative Abweichung ist
- ob der aktuelle Positionspreis unter, auf oder ueber der Kostenbasis liegt

Sie soll bewusst noch nicht beantworten:

- welcher neue Preis automatisch gesetzt werden soll
- welche Marge oder welcher Zuschlag anzuwenden ist
- ob eine Freigabe erforderlich ist
- wie ein gesamtes Angebot kalkulatorisch bewertet wird

## 3. Bewusst enger Scope

Innerhalb dieses Blocks soll es nur geben:

- read-only Bewertung direkt an genau einer Quote-Position
- Nutzung der bestehenden Quellen-Priorisierung als Grundlage
- keine neuen Tabellen und keine neue persistierte Bewertungsentscheidung
- weiterhin Arbeit im bestehenden Quote-Editor

Bewusst ausserhalb des Scopes bleiben:

- Schreibpfade fuer Preisuebernahme aus der primaeren Quelle
- Deckungsbeitrag, Zielmarge oder Verkaufspreisvorschlag
- Zuschlags-, Rabatt- oder Gemeinkostenlogik
- Bulk-Bewertung ueber ganze Quotes
- Freigabe- oder Eskalationsworkflow
- KI- oder lernende Preisbewertung

## 4. Zielbild im Backend

Das Backend soll einen engen read-only Pfad bereitstellen, der:

- genau eine bereits gemappte Draft-Quote-Position adressiert
- dieselben Guard Rails wie die Preisquellen- und Priorisierungspfade
  verwendet
- den aktuellen `unit_price` der Quote-Position liest
- die primaere Quelle aus `PriceSourcePriorityForQuoteItem(...)` nutzt
- daraus eine kleine Bewertung ableitet

Der Zielpfad soll bewusst keine Quote-Position veraendern.

## 5. Zielbild in der Rueckgabe

Die Rueckgabe soll bewusst klein bleiben. Fuer genau eine Position reicht:

- `current_unit_price`
- `currency`
- `primary_source_label`
- `primary_source_unit_price`
- `primary_source_reference`
- `primary_source_date`
- `absolute_delta`
- `relative_delta_percent`
- `evaluation_status`
- `evaluation_reason`

Die erste Statusmenge bleibt klein:

- `below_cost_basis`
- `at_cost_basis`
- `above_cost_basis`

Wichtig ist:

- der Status beschreibt nur das Verhaeltnis zum primaeren Kostenanker
- er ist noch keine Kalkulations- oder Freigabeentscheidung
- er darf keine stille Preisaenderung ausloesen

## 6. Erste enge Bewertungsregel

Fuer diesen Minimalblock reicht eine kleine, feste und erklaerbare Regel.

Berechnung:

- primaere Quelle ist die Quelle mit `priority_rank = 1`
- `absolute_delta = current_unit_price - primary_source_unit_price`
- `relative_delta_percent = absolute_delta / primary_source_unit_price * 100`,
  falls `primary_source_unit_price > 0`

Status:

- `below_cost_basis`, wenn `absolute_delta < 0`
- `at_cost_basis`, wenn `absolute_delta == 0`
- `above_cost_basis`, wenn `absolute_delta > 0`

Optional darf fuer spaetere technische Umsetzung eine kleine Rundungstoleranz
fuer Cent-Abweichungen definiert werden. Diese Toleranz soll nur Rundungsrauschen
vermeiden und keine Marge modellieren.

## 7. Guard Rails

Die Guard Rails sollen bewusst eng zu den bestehenden Preisquellenpfaden
passen:

- keine historischen Quote-Versionen
- nur Draft-Quotes
- Position muss existieren
- Position muss ein gesetztes `material_id` besitzen
- mindestens eine priorisierte Preisquelle muss sichtbar sein
- Bewertung bleibt read-only

Wenn keine primaere Quelle ableitbar ist, soll der Pfad keinen leeren
Kalkulationsersatz erzeugen, sondern klar mit einem fachlichen Fehler enden.

## 8. Zielbild im Client

Der Client soll die Bewertung klein und positionsnah spiegeln:

- weiterhin direkt an genau einer Quote-Position
- weiterhin im bestehenden Quote-Editor
- weiterhin read-only

Sichtbar werden soll nur ein kleiner Bewertungsblock, zum Beispiel:

- aktuellem Preis
- primaerer Kostenbasis
- Differenz absolut
- Differenz prozentual
- Status `unter Kostenbasis`, `auf Kostenbasis` oder `ueber Kostenbasis`

Es soll bewusst noch nicht geben:

- Button zur automatischen Preisuebernahme
- Margen- oder Zuschlagseingaben
- Ampellogik mit Freigabeprozess
- globale Kalkulationsansicht

## 9. Warum diese Stufe noch keine Vollkalkulation ist

Diese Stufe bleibt bewusst kleiner als spaetere Kalkulation, weil:

- nur eine aktuelle Position bewertet wird
- nur der bestehende `unit_price` mit der primaeren Quelle verglichen wird
- keine Zielmarge berechnet wird
- keine Zuschlaege, Rabatte oder Gemeinkosten angewendet werden
- keine Angebots- oder Projektgesamtsicht entsteht

## 10. Warum diese Stufe noch keine Preisentscheidung ist

Diese Stufe bleibt bewusst vor einer Preisentscheidung, weil:

- kein Schreibpfad entsteht
- kein Preis aus der primaeren Quelle uebernommen wird
- der Nutzer nur Transparenz ueber die Abweichung bekommt
- eine spaetere explizite Entscheidung besser begruendet werden kann

## 11. Minimaler Nutzen

Schon in diesem engen Zuschnitt schafft die Preisbewertung:

- schnelle Erkennung von Preisen unter der primaeren Kostenbasis
- bessere Nachvollziehbarkeit manueller Preisentscheidungen
- klare Grundlage fuer spaetere kontrollierte Preisuebernahme
- belastbaren Anschluss fuer spaetere Margen- und Freigabelogik

## 12. Entscheidung

Das minimale technische Zielmodell fuer die naechste Stufe ist:

- ein enger read-only Bewertungsanker fuer genau eine bereits gemappte
  Draft-Quote-Position, basierend auf der primaeren Quelle aus der bestehenden
  Quellen-Priorisierung

## 13. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Schritt:

- die erste Backend-Stufe fuer diese read-only Preisbewertung vorbereiten,
  bewusst noch ohne automatische Preisentscheidung, Bulk, Margen-, Zuschlags-
  oder Freigabelogik
