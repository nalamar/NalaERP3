# GAEB-Quellen-Priorisierung: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der Quellen-Priorisierung
nach Umsetzung von:

- fachlicher Inventur der Folgestufe nach abgeschlossener
  Preis-Historien-/Quellenstufe
- technischem Zuschnitt auf eine kleine read-only Einordnung sichtbarer
  Preisquellen
- engem Backend-Pfad fuer genau eine bereits gemappte Draft-Quote-Position
- kleiner Client-Anbindung direkt im bestehenden Quote-Editor

Die Leitfrage bleibt eng:

- bleibt innerhalb dieses Blocks vor spaeterer Ranking-, Kalkulations- oder
  Automatiklogik noch genau ein kleiner Haertungsschritt mit gutem Signal
  uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- read-only Priorisierung nur fuer genau eine bereits gemappte
  Draft-Quote-Position
- Guard Rails gegen historische Versionen, Nicht-Draft-Angebote, fehlende
  Positionen und Positionen ohne gesetztes `material_id`
- Priorisierung ausschliesslich der bereits sichtbaren Preisquellen:
  - `Letzter Bestellpreis`
  - `Durchschnittlicher Einkaufspreis`
- kleine feste Priorisierungsregel:
  - `Letzter Bestellpreis` vor `Durchschnittlicher Einkaufspreis`, wenn beide
    sichtbar sind
  - ansonsten bleibt die jeweils sichtbare Quelle primaer
- Rueckgabe mit `priority_rank`, `priority_reason`, Preis- und Herkunftsdaten
- positionsnahe Anzeige im bestehenden Quote-Editor
- weiterhin rein read-only, ohne Aenderung von `unit_price`,
  `price_mapping_status` oder `material_id`

Bewusst nicht Teil dieses Blocks:

- allgemeines Ranking- oder Scoringmodell
- Bulk-Priorisierung ueber ganze Quotes
- automatische Preiswahl oder Preisaktualisierung
- Margen-, Zuschlags-, Rabatt- oder Kalkulationslogik
- globale Preis- oder Quellenoberflaechen

## 2. Verifikation

Fuer diesen Audit wurden die zielgerichteten Verifikationen geprueft:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Die Go-Pakete `internal/quotes` und `internal/http` melden `ok`; der Prozess
endet anschliessend beim Go-Cache-Trimmen mit `Access is denied` ausserhalb des
Workspace. Die Client-Analyse wurde erneut gestartet, lieferte aber innerhalb
von 300 Sekunden keine Ausgabe und lief in ein Timeout. Der vorherige Stand
dieser Client-Dateien war nach `dart format` und `flutter analyze` gruen; seit
diesem Stand wurden nur dieses Audit und `codex.md` geaendert.

## 3. Audit-Ergebnis

Die kleine Quellen-Priorisierung ist in ihrem bewusst engen Scope sauber
abgeschlossen.

Vor spaeterer Ranking-, Kalkulations- oder Automatiklogik bleibt innerhalb
dieses Blocks kein weiterer kleiner Haertungsschritt mit gutem Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- koennen die bereits sichtbaren Preisquellen direkt an genau einer
  gemappten Draft-Quote-Position nachvollziehbar eingeordnet werden, ohne
  daraus bereits Ranking-, Kalkulations- oder Halbautomatiklogik zu machen?

Das ist erreicht durch:

- engen read-only Pfad auf genau eine Position
- Nutzung ausschliesslich der bestehenden sichtbaren Preisquellen
- feste, erklaerbare Priorisierungsregel statt Score- oder Gewichtungsmodell
- klare Trennung zwischen Quellen-Einordnung und Preisuebernahme
- positionsnahe Anzeige im bestehenden Editor statt neuer Nebenoberflaeche

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- weitere kosmetische Labels fuer Prioritaeten
- kleine Zusatztexte ohne neue fachliche Wirkung
- weitere Sortier- oder Anzeigevarianten derselben zwei Quellen

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als diese kleine read-only Einordnung sichtbarer Quellen
benoetigt wird, beginnt bereits eine neue echte Ausbaustufe:

- breiteres Ranking oder Scoring mehrerer Preisquellen
- kalkulationsnahe Preisbewertung
- automatische Preisableitung oder Preisaktualisierung
- Bulk-Unterstuetzung ueber mehrere Quote-Positionen

## 6. Entscheidung

Entscheidung:

- Der Block `kleine Quellen-Priorisierung` gilt als abgeschlossen.
- Vor spaeterer Ranking-, Kalkulations- oder Automatiklogik bleibt innerhalb
  dieses engen Scopes kein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an der
Quellen-Priorisierung, sondern eine neue Ausbaustufe:

- fachliche Inventur des kleinsten Folgeausbaus nach abgeschlossener
  Quellen-Priorisierung, voraussichtlich mit Fokus auf kalkulationsnahe
  Preisbewertung, kontrollierte Preisentscheidung oder einen anderen minimalen
  kommerziellen Folgeschritt
