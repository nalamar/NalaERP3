# GAEB-Kleine Materialsuche: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit bewertet den bewusst kleinen Ausbau der Materialsuche nach
Umsetzung von:

- fachlicher Inventur der Suchstufe
- technischem Zuschnitt auf eine enge manuelle Suche an genau einer
  Quote-Position
- kleiner Backend-Suche mit engen Guard Rails
- kleiner Client-Anbindung direkt im bestehenden Quote-Editor

Die Leitfrage bleibt eng:

- bleibt innerhalb dieses Suchblocks vor spaeterer Uebernahme-,
  Ranking- oder Automatiklogik noch genau ein kleiner Haertungsschritt mit
  gutem Signal uebrig?

## 1. Gepruefter Scope

Innerhalb dieses Blocks wurde umgesetzt:

- enger read-only Suchpfad fuer genau eine offene Draft-Quote-Position
- Guard Rails gegen leeren Suchbegriff, historische Versionen,
  Nicht-Draft-Angebote und bereits gemappte Positionen
- kleine Trefferliste aus aktiven Materialien ueber `nummer` und
  `bezeichnung`
- kleine Client-Suchzeile direkt an der bestehenden Quote-Position
- read-only Darstellung weniger Suchtreffer oder eines leeren Hinweises

Bewusst nicht Teil dieses Blocks:

- Uebernahme eines Suchtreffers
- Ranking oder Match-Scores
- automatische Vorbelegung eines Treffers
- Bulk-Suche oder Bulk-Mapping
- Preisvorschlaege oder Preislogik
- globale Such- oder Mappingansichten

## 2. Verifikation

Fuer diesen Block liegen die zielgerichteten Verifikationen vor:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Die relevanten Laeufe fuer Backend und Client sind auf dem aktuellen Stand
gruen.

## 3. Audit-Ergebnis

Die kleine Materialsuche ist in ihrem bewusst engen Scope sauber
abgeschlossen.

Vor spaeterer Suchtreffer-Uebernahme, Ranking- oder Automatiklogik bleibt
innerhalb dieses Blocks kein weiterer kleiner Haertungsschritt mit gutem
Signal uebrig.

## 4. Begruendung

Die relevante Minimalfrage dieses Blocks ist bereits beantwortet:

- kann fuer eine offene Quote-Position kontrolliert nach bestehenden
  Materialien gesucht werden, ohne daraus bereits Mapping-Uebernahme,
  Ranking oder Halbautomatik zu machen?

Das ist erreicht durch:

- Suchscope auf genau eine Quote-Position
- enge serverseitige Regeln fuer bearbeitbare offene Mapping-Faelle
- kleine, bewusst ungewichtete Trefferliste
- direkte Sichtbarkeit der Suche im bestehenden Editor
- keine Vermischung mit Uebernahme-, Bulk- oder Preislogik

## 5. Warum kein weiterer Mini-Schritt mehr sinnvoll ist

Naheliegende Folgeideen wie:

- kosmetische UI-Erweiterungen an der Suchzeile
- zusaetzliche Treffer-Metadaten ohne neue fachliche Wirkung
- weitere kleine Varianten derselben read-only Suche

haetten in diesem Stadium nur schwaches Signal.

Sobald mehr als die enge read-only Suche benoetigt wird, beginnt bereits eine
neue echte Ausbaustufe:

- explizite Uebernahme aus Suchtreffern
- spaeteres Ranking oder Matching
- globale Mapping-Unterstuetzung
- automatische Material- oder Preisableitung

## 6. Entscheidung

Entscheidung:

- Der Block `kleine Materialsuche` gilt als abgeschlossen.
- Vor spaeterer Uebernahme-, Ranking- oder Automatiklogik bleibt innerhalb
  dieses engen Scopes kein weiterer kleiner Haertungsschritt mit gutem
  Signal uebrig.

## 7. Naechster sinnvoller Schritt

Der naechste sinnvolle Abschnitt ist deshalb nicht weiterer Feinschliff an
der read-only Suche, sondern die fachliche Inventur des kleinsten
Folgeausbaus nach abgeschlossener Materialsuche:

- enge explizite Uebernahme aus Suchtreffern
- oder ein anderer minimaler Mapping-Folgeschritt mit hoeherem Signalwert
