# GAEB-Kleine Preisuebernahme: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der Preisuebernahme nach
Umsetzung von:

- fachlicher Inventur der Preisuebernahme-Stufe
- technischem Zuschnitt auf eine explizite Aktion am bereits sichtbaren
  Preisanker
- engem Backend-Write-Pfad auf genau einer bereits gemappten
  Draft-Quote-Position
- kleiner Client-Anbindung direkt im bestehenden Quote-Editor

Die Leitfrage bleibt eng:

- bleibt innerhalb dieses Blocks vor spaeterer Ranking-, Historien- oder
  Automatiklogik noch genau ein kleiner Haertungsschritt mit gutem Signal
  uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- read-only Preisanker als Ausgangspunkt fuer eine direkte explizite Aktion
- enger Backend-Write-Pfad nur fuer genau eine bereits gemappte
  Draft-Quote-Position
- Guard Rails gegen historische Versionen, Nicht-Draft-Angebote, fehlende
  Positionen und Positionen ohne gesetztes `material_id`
- serverseitige Preisuebernahme bewusst nur aus dem bereits sichtbaren
  Materialanker `avg_purchase_price`
- Client-Aktion `Preis uebernehmen` direkt am sichtbaren Preisanker im
  bestehenden Quote-Editor
- unmittelbare Dialog-Aktualisierung aus der Serverantwort, sodass
  `unit_price`, `price_mapping_status` und der restliche Item-Zustand direkt
  den neuen Stand zeigen

Bewusst nicht Teil dieses Blocks:

- Ranking oder Priorisierung mehrerer Preisquellen
- Preis-Historienlogik oder weitere kommerzielle Herleitungen
- Bulk-Preisuebernahme ueber mehrere Quote-Positionen
- automatische Preisableitung oder automatische Preisaktualisierung
- globale Preis- oder Kalkulationsansichten

## 2. Verifikation

Fuer diesen Audit wurden die bestehenden zielgerichteten Verifikationen erneut
geprueft:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Beide Stufen sind auf dem aktuellen Stand gruen.

## 3. Audit-Ergebnis

Die kleine Preisuebernahme ist in ihrem bewusst engen Scope sauber
abgeschlossen.

Vor spaeterer Ranking-, Historien- oder Automatiklogik bleibt innerhalb dieses
Blocks kein weiterer kleiner Haertungsschritt mit gutem Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- kann ein bereits sichtbarer Preisanker direkt und kontrolliert auf genau
  eine Quote-Position uebernommen werden, ohne daraus bereits
  Mehrquellen-Logik, Historienlogik oder Halbautomatik zu machen?

Das ist erreicht durch:

- engen, expliziten Write-Pfad auf genau eine Position
- serverseitige Herleitung des Preises nur aus dem bereits sichtbaren
  Materialanker
- klare manuelle Markierung ueber `price_mapping_status = manual`
- direkte Rueckspiegelung des Ergebnisses in den bestehenden Editor
- keine Vermischung mit Ranking-, Bulk- oder Historienlogik

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- kosmetische UI-Hinweise an der Preisuebernahmeaktion
- weitere read-only Preis-Metadaten ohne neue fachliche Wirkung
- kleine Komfortvarianten derselben Preisuebernahme

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als die enge manuelle Uebernahme eines bereits sichtbaren
Preisankers benoetigt wird, beginnt bereits eine neue echte Ausbaustufe:

- Priorisierung mehrerer Preisquellen
- Preis-Historienlogik
- globale Preisunterstuetzung
- automatische Preis- oder Kalkulationsableitung

## 6. Entscheidung

Entscheidung:

- Der Block `kleine Preisuebernahme` gilt als abgeschlossen.
- Vor spaeterer Ranking-, Historien- oder Automatiklogik bleibt innerhalb
  dieses engen Scopes kein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an der
Preisuebernahme, sondern eine neue Ausbaustufe:

- fachliche Inventur des kleinsten Folgeausbaus nach abgeschlossener
  Preisuebernahme, voraussichtlich mit Fokus auf Ranking, Historienlogik oder
  einen anderen minimalen kommerziellen Folgeschritt
