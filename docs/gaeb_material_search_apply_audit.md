# GAEB-Kleine Suchtreffer-Uebernahme: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der Suchtreffer-Uebernahme
nach Umsetzung von:

- fachlicher Inventur der Suchtreffer-Uebernahme-Stufe
- technischem Zuschnitt auf eine explizite Aktion an bereits sichtbaren
  Suchtreffern
- enger Backend-Uebernahme auf genau einer Draft-Quote-Position
- kleiner Client-Anbindung direkt in der bestehenden Suchtrefferliste

Die Leitfrage bleibt eng:

- bleibt innerhalb dieses Blocks vor spaeterer Ranking-, Preis- oder
  Automatiklogik noch genau ein kleiner Haertungsschritt mit gutem Signal
  uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- read-only Suchtrefferliste als Ausgangspunkt fuer eine direkte Aktion
- enger Backend-Write-Pfad nur fuer sichtbare Suchtreffer desselben
  Suchkontexts
- Guard Rails gegen leeren Suchbegriff, historische Versionen,
  Nicht-Draft-Angebote, bereits gemappte Positionen und nicht sichtbare
  Suchtreffer
- Client-Aktion `Uebernehmen` direkt am sichtbaren Suchtreffer im
  bestehenden Quote-Editor
- unmittelbare Dialog-Aktualisierung aus der Serverantwort, sodass
  `material_id`, `price_mapping_status`, Kandidaten- und Suchzustand direkt
  den neuen Zustand zeigen

Bewusst nicht Teil dieses Blocks:

- Ranking oder Priorisierung unter Suchtreffern
- automatische Vorbelegung oder automatische Auswahl
- Preisvorschlaege oder Preislogik aus dem uebernommenen Material
- Bulk-Uebernahme ueber mehrere Quote-Positionen
- globale Mapping- oder Suchansichten

## 2. Verifikation

Fuer diesen Audit wurden die bestehenden zielgerichteten Verifikationen erneut
geprueft:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Beide Laeufe sind auf dem aktuellen Stand gruen.

## 3. Audit-Ergebnis

Die kleine Suchtreffer-Uebernahme ist in ihrem bewusst engen Scope sauber
abgeschlossen.

Vor spaeterer Ranking-, Preis- oder Automatiklogik bleibt innerhalb dieses
Blocks kein weiterer kleiner Haertungsschritt mit gutem Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- kann ein bereits sichtbarer Suchtreffer direkt und kontrolliert auf genau
  eine Quote-Position uebernommen werden, ohne daraus bereits Ranking,
  Preislogik oder Halbautomatik zu machen?

Das ist erreicht durch:

- engen, expliziten Write-Pfad auf genau eine Position
- serverseitige Pruefung, dass nur sichtbare Suchtreffer desselben
  Suchkontexts uebernommen werden
- klare manuelle Markierung ueber `price_mapping_status = manual`
- direkte Rueckspiegelung des Ergebnisses in den bestehenden Editor
- keine Vermischung mit Ranking-, Bulk- oder Preislogik

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- kosmetische UI-Hinweise an der Uebernahmeaktion
- weitere Suchtreffer-Metadaten ohne neue fachliche Wirkung
- kleine Komfortvarianten derselben Uebernahmeaktion

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als die enge manuelle Uebernahme sichtbarer Suchtreffer benoetigt
wird, beginnt bereits eine neue echte Ausbaustufe:

- Ranking oder Matching zwischen Suchtreffern
- Preisvorschlaege aus Material oder Historie
- globale Mapping-Unterstuetzung
- automatische Material- oder Preisableitung

## 6. Entscheidung

Entscheidung:

- Der Block `kleine Suchtreffer-Uebernahme` gilt als abgeschlossen.
- Vor spaeterer Ranking-, Preis- oder Automatiklogik bleibt innerhalb dieses
  engen Scopes kein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an
der Suchtreffer-Uebernahme, sondern eine neue Ausbaustufe:

- fachliche Inventur des kleinsten Folgeausbaus nach abgeschlossener
  Suchtreffer-Uebernahme, voraussichtlich mit Fokus auf Ranking,
  Preisvorschlag oder einen anderen minimalen Mapping-Folgeschritt
