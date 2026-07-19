# GAEB-Preis-Historien-/Quellenstufe: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der Preis-Historien-/
Quellenstufe nach Umsetzung von:

- fachlicher Inventur der Folgestufe nach abgeschlossener Preisuebernahme
- technischem Zuschnitt auf einen kleinen read-only Herkunftsblock
- engem Backend-Pfad fuer genau eine bereits gemappte Draft-Quote-Position
- kleiner Client-Anbindung direkt im bestehenden Quote-Editor

Die Leitfrage bleibt eng:

- bleibt innerhalb dieses Blocks vor spaeterer Ranking-, Kalkulations- oder
  Automatiklogik noch genau ein kleiner Haertungsschritt mit gutem Signal
  uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- read-only Herkunftsblock nur fuer genau eine bereits gemappte
  Draft-Quote-Position
- Guard Rails gegen historische Versionen, Nicht-Draft-Angebote, fehlende
  Positionen und Positionen ohne gesetztes `material_id`
- bewusst nur kleine vorhandene Herkunftsquellen:
  - `Durchschnittlicher Einkaufspreis` aus `materials.avg_purchase_price`
  - optional `Letzter Bestellpreis` aus der letzten nicht stornierten
    `purchase_order_items`-/`purchase_orders`-Quelle
- positionsnahe Anzeige im bestehenden Quote-Editor
- weiterhin rein read-only, ohne Aenderung von `unit_price`,
  `price_mapping_status` oder `material_id`

Bewusst nicht Teil dieses Blocks:

- Ranking oder Priorisierung mehrerer Preisquellen
- weitergehende Preis-Historienlogik
- Bulk-Quellenanzeige ueber mehrere Positionen
- Preis- oder Kalkulationsableitung aus der Quellenliste
- automatische Preisaktualisierung

## 2. Verifikation

Fuer diesen Audit wurden die bestehenden zielgerichteten Verifikationen
geprueft:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Beide Stufen sind auf dem aktuellen Stand gruen.

## 3. Audit-Ergebnis

Die kleine Preis-Historien-/Quellenstufe ist in ihrem bewusst engen Scope
sauber abgeschlossen.

Vor spaeterer Ranking-, Kalkulations- oder Automatiklogik bleibt innerhalb
dieses Blocks kein weiterer kleiner Haertungsschritt mit gutem Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- kann nach abgeschlossenem manuellem Material- und Preissetzen eine kleine,
  nachvollziehbare read-only Preisherkunft direkt an genau einer Position
  sichtbar gemacht werden, ohne daraus bereits Priorisierung, Kalkulation oder
  Halbautomatik zu machen?

Das ist erreicht durch:

- engen read-only Pfad auf genau eine Position
- Nutzung ausschliesslich bereits vorhandener Datenquellen
- klare Trennung zwischen Herkunftsanzeige und Preisuebernahme
- positionsnahe Anzeige im bestehenden Editor statt neuer Nebenoberflaeche
- keine Vermischung mit Ranking-, Bulk- oder Kalkulationslogik

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- kosmetische Text- oder Anzeigejustierungen innerhalb desselben Blocks
- weitere kleine Herkunftsmetadaten ohne neue fachliche Wirkung
- erste Sortier- oder Priorisierungsregeln innerhalb der Quellenliste

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als eine kleine read-only Herkunftssicht benoetigt wird, beginnt
bereits eine neue echte Ausbaustufe:

- Ranking oder Priorisierung mehrerer Preisquellen
- weitergehende Preis-Historienlogik
- Kalkulationsunterstuetzung
- automatische Preisableitung oder Aktualisierung

## 6. Entscheidung

Entscheidung:

- Der Block `kleine Preis-Historien-/Quellenstufe` gilt als abgeschlossen.
- Vor spaeterer Ranking-, Kalkulations- oder Automatiklogik bleibt innerhalb
  dieses engen Scopes kein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an der
Quellenanzeige, sondern eine neue Ausbaustufe:

- fachliche Inventur des kleinsten Folgeausbaus nach abgeschlossener
  Preis-Historien-/Quellenstufe, voraussichtlich mit Fokus auf Ranking,
  Kalkulationsnaehe oder einen anderen minimalen kommerziellen Folgeschritt
