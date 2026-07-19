# GAEB-Preisbewertung: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der read-only Preisbewertung
nach Umsetzung von:

- fachlicher Inventur der Folgestufe nach abgeschlossener
  Quellen-Priorisierung
- technischem Zuschnitt auf einen kleinen read-only Bewertungsanker
- engem Backend-Pfad fuer genau eine bereits gemappte Draft-Quote-Position
- kleiner Client-Anbindung direkt im bestehenden Quote-Editor

Die Leitfrage bleibt eng:

- bleibt innerhalb dieses Blocks vor Preisentscheidung, Bulk, Margen-,
  Zuschlags- oder Freigabelogik noch genau ein kleiner Haertungsschritt mit
  gutem Signal uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- read-only Preisbewertung nur fuer genau eine bereits gemappte
  Draft-Quote-Position
- Nutzung der bestehenden Quellen-Priorisierung als primaere Kostenbasis
- Guard Rails gegen historische Versionen, Nicht-Draft-Angebote, fehlende
  Positionen und Positionen ohne gesetztes `material_id`
- Rueckgabe von:
  - aktuellem `unit_price`
  - primaerer Preisquelle
  - absoluter Abweichung
  - relativer Abweichung, soweit berechenbar
  - kleinem Bewertungsstatus
  - kurzer Bewertungsbegruendung
- kleine Statusmenge:
  - `below_cost_basis`
  - `at_cost_basis`
  - `above_cost_basis`
- positionsnahe Anzeige im bestehenden Quote-Editor
- weiterhin rein read-only, ohne Aenderung von `unit_price`,
  `price_mapping_status`, `material_id` oder Quote-Summen

Bewusst nicht Teil dieses Blocks:

- automatische Preisentscheidung
- Preisuebernahme aus der primaeren Quelle
- Bulk-Bewertung ueber ganze Quotes
- Margen-, Zuschlags-, Rabatt- oder Gemeinkostenlogik
- Freigabe- oder Eskalationsworkflow
- neue Tabellen oder persistierte Bewertungsentscheidungen
- globale Kalkulations- oder Preisarbeitsflaechen

## 2. Verifikation

Fuer diesen Audit wurden die zielgerichteten Verifikationen erneut geprueft:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Beide Laeufe sind auf dem aktuellen Stand gruen.

## 3. Audit-Ergebnis

Die kleine read-only Preisbewertung ist in ihrem bewusst engen Scope sauber
abgeschlossen.

Vor Preisentscheidung, Bulk, Margen-, Zuschlags- oder Freigabelogik bleibt
innerhalb dieses Blocks kein weiterer kleiner Haertungsschritt mit gutem
Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- kann der aktuell gesetzte Positionspreis direkt an genau einer gemappten
  Draft-Quote-Position gegen die primaere priorisierte Preisquelle bewertet
  werden, ohne daraus bereits Preisentscheidung, Kalkulation oder
  Freigabelogik zu machen?

Das ist erreicht durch:

- engen read-only Pfad auf genau eine Position
- Wiederverwendung der bestehenden Quellen-Priorisierung statt neuer
  Scoring- oder Kalkulationslogik
- kleine, erklaerbare Statusmenge
- explizite Trennung zwischen Bewertung und Preisuebernahme
- positionsnahe Anzeige im bestehenden Editor
- keine Mutation von Quote-, Positions- oder Materialdaten

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- weitere kosmetische Statuslabels
- zusaetzliche Warntexte ohne neue fachliche Wirkung
- weitere Rundungs- oder Formatierungsvarianten
- erste visuelle Ampellogik ohne Freigabekonzept

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als diese kleine read-only Bewertung benoetigt wird, beginnt
bereits eine neue echte Ausbaustufe:

- kontrollierte Preisentscheidung oder Uebernahme aus der primaeren Quelle
- kalkulationsnahe Margen- oder Zuschlagslogik
- Bulk-Bewertung ueber mehrere Positionen
- Freigabe- oder Eskalationslogik

## 6. Entscheidung

Entscheidung:

- Der Block `kleine read-only Preisbewertung` gilt als abgeschlossen.
- Vor Preisentscheidung, Bulk, Margen-, Zuschlags- oder Freigabelogik bleibt
  innerhalb dieses engen Scopes kein weiterer kleiner Haertungsschritt mit
  gutem Signal uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an der
Preisbewertung, sondern eine neue Ausbaustufe:

- fachliche Inventur des kleinsten Folgeausbaus nach abgeschlossener
  Preisbewertung, voraussichtlich mit Fokus auf kontrollierte
  Preisentscheidung, explizite Preisuebernahme aus der primaeren Quelle oder
  einen anderen minimalen kommerziellen Folgeschritt
