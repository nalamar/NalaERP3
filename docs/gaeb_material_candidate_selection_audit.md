# GAEB-Kleine Kandidatenauswahl: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der Materialkandidaten-
Auswahl nach Umsetzung von:

- fachlicher Inventur der Kandidatenauswahl-Stufe
- technischem Zuschnitt auf eine explizite Aktion an bereits sichtbaren
  Kandidaten
- enger Backend-Uebernahme auf genau einer Draft-Quote-Position
- kleiner Client-Anbindung direkt in der bestehenden Kandidatenliste

Die Leitfrage bleibt eng:

- bleibt vor spaeterer Suche, Ranking- oder Automatiklogik noch genau ein
  kleiner Haertungsschritt mit gutem Signal uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- read-only Kandidatenliste als Ausgangspunkt fuer eine direkte Aktion
- enger Backend-Write-Pfad nur fuer bereits sichtbare Kandidaten
- Guard Rails gegen nicht sichtbare Kandidaten, historische Versionen,
  Nicht-Draft-Angebote und stilles Ueberschreiben vorhandener `material_id`
- Client-Aktion `Uebernehmen` direkt am sichtbaren Kandidaten im bestehenden
  Quote-Editor
- unmittelbare Dialog-Aktualisierung aus der Serverantwort, sodass
  `material_id`, `price_mapping_status`, Kandidatenstatus und Kandidatenliste
  direkt den neuen Zustand zeigen

Bewusst nicht Teil dieses Blocks:

- Materialsuche ausserhalb sichtbarer Kandidaten
- Kandidatenranking oder Priorisierung
- automatische Kandidatenvorbelegung
- Bulk-Uebernahme ueber mehrere Quote-Positionen
- Preisvorschlaege oder automatische Preislogik
- globale Mapping- oder Kandidatenansicht

## 2. Verifikation

Fuer diesen Audit wurden die bestehenden zielgerichteten Verifikationen erneut
geprueft:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Beide Laeufe sind auf dem aktuellen Stand gruen.

## 3. Audit-Ergebnis

Die kleine Kandidatenauswahl ist in ihrem bewusst engen Scope sauber
abgeschlossen.

Vor spaeterer Suche, Ranking- oder Automatiklogik bleibt innerhalb dieses
Blocks kein weiterer kleiner Haertungsschritt mit gutem Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- kann ein bereits sichtbarer Materialkandidat direkt und kontrolliert auf
  genau eine Quote-Position uebernommen werden, ohne daraus bereits Suche,
  Ranking oder Automatik zu machen?

Das ist erreicht durch:

- engen, expliziten Write-Pfad auf genau eine Position
- serverseitige Pruefung, dass nur sichtbare Kandidaten uebernommen werden
- klare manuelle Markierung ueber `price_mapping_status = manual`
- direkte Rueckspiegelung des Ergebnisses in den bestehenden Editor
- keine Vermischung mit Such-, Bulk- oder Preislogik

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- zusaetzliche kosmetische Hinweistexte an der Aktion
- weitere Kandidatenmetadaten ohne neue fachliche Wirkung
- kleine Komfortvarianten derselben Uebernahmeaktion

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als die enge manuelle Uebernahme sichtbarer Kandidaten benoetigt
wird, beginnt bereits eine neue echte Ausbaustufe:

- Materialsuche fuer Faelle ohne sichtbaren Kandidaten
- spaeteres Ranking oder Matching
- globale Mapping-Unterstuetzung
- automatische Material- oder Preisableitung

## 6. Entscheidung

Entscheidung:

- Der Block `kleine Kandidatenauswahl` gilt als abgeschlossen.
- Vor spaeterer Suche, Ranking- oder Automatiklogik bleibt innerhalb dieses
  engen Scopes kein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an der
Kandidatenauswahl, sondern eine neue Ausbaustufe:

- fachliche Inventur des kleinsten Folgeausbaus nach abgeschlossener
  Kandidatenauswahl, voraussichtlich mit Fokus auf Materialsuche fuer Faelle
  ohne sichtbaren Kandidaten oder ohne passenden sichtbaren Treffer
