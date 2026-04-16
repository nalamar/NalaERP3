# GAEB-Herkunfts- und Mapping-Anker: Abschluss-Audit

## Ziel dieses Dokuments

Dieses Dokument bewertet, ob der kleine Herkunfts- und Mapping-Anker nach
Backend- und Client-Anbindung sauber abgeschlossen ist oder ob innerhalb
dieses engen Scopes noch genau ein weiterer kleiner Haertungsschritt mit gutem
Signal verbleibt.

## 1. Gepruefter Umfang

Innerhalb dieses Blocks wurden abgeschlossen:

- fachliche Inventur der Mapping-Zielstrecke nach dem stabilen
  Import-/Review-/Apply-/Transparenzpfad
- technischer Zuschnitt auf einen kleinen Herkunftsanker statt frueher
  Preis-, Steuer- oder Materialautomatik
- neue persistente Verknuepfung zwischen `quote_import_items` und
  `quote_items`
- transaktionales Mitschreiben dieser Verknuepfung direkt im erfolgreichen
  Apply-Pfad
- read-only Backend-Sicht auf diese Verknuepfung ueber bestehende
  Importpositions-Responses
- kleine Client-Sicht im bestehenden Importpositionsdialog inklusive
  `Quote oeffnen`

Bewusst ausserhalb des Blocks geblieben:

- Preisvorschlaege
- Materialmapping gegen `materials`
- Steuer- oder Kontierungsvorschlaege
- Editierbarkeit der Verknuepfung
- globale Mapping-Uebersicht
- Re-Apply oder Umbiegen vorhandener Links

## 2. Audit-Ergebnis

Der Herkunfts- und Mapping-Anker ist innerhalb seines bewusst kleinen
Zuschnitts sauber abgeschlossen.

Die urspruengliche Luecke wurde geschlossen:

- uebernommene `accepted`-Importpositionen sind jetzt bis zur erzeugten
  Quote-Position rueckverfolgbar
- die Verknuepfung entsteht direkt im erfolgreichen Apply-Pfad und bleibt
  dadurch konsistent zum erzeugten Angebot
- die Verknuepfung ist auf bestehenden Lese- und Dialogpfaden bereits
  sichtbar, ohne neuen Workflow-Screen

## 3. Warum kein weiterer kleiner Haertungsschritt uebrig ist

Innerhalb dieses Blocks bleibt kein einzelner kleiner Haertungsschritt mit
gutem Signal uebrig.

Gruende:

- die wesentliche Fachfaehigkeit des Blocks ist bereits vollstaendig:
  Rueckverfolgbarkeit von GAEB-Quelle zur Quote
- Integritaet ist bereits transaktional im Apply-Pfad abgesichert
- Backend- und Client-Sicht auf den Herkunftsanker sind vorhanden
- parsernahe Positionen ohne Uebernahme bleiben korrekt ohne Link sichtbar

Naheliegende Folgeideen waeren bereits neue Ausbaustufen und nicht mehr
kleine Haertung:

- Sicht der Verknuepfungen direkt in der Quote
- nachtraegliche manuelle Neuzuordnung
- Preis-, Steuer- oder Materialvorschlaege entlang der Herkunftslinks
- globale Mapping-Liste oder Mapping-Cockpit

## 4. Risiko- und Signalbewertung

Der Block hat gutes Signal, weil er:

- keine bestehende Preis- oder Beleglogik fachlich aendert
- den bereits vorhandenen Apply-Pfad nur um belastbare Herkunftsdaten
  erweitert
- spaeteres Mapping vorbereitet, ohne es vorwegzunehmen

Ein weiterer Mini-Schritt innerhalb desselben Blocks wuerde dagegen kaum neue
Fachfaehigkeit liefern und bereits in Richtung spaeterer Mapping-Produkte
kippen.

## 5. Entscheidung

Der Block `Herkunfts- und Mapping-Anker` kann vor spaeterem Preis-,
Steuer- oder Material-Mapping bewusst als sauber beendet betrachtet werden.

## 6. Naechster sinnvoller Schritt

Der naechste sinnvolle Ausbau liegt nicht in weiterer Feinhaertung des
Herkunftsankers, sondern in der naechsten fachlichen Inventur fuer den
spaeteren Mapping-Strang.

Der beste Kandidat mit neuem Signal ist:

- den kleinsten sinnvollen Einstiegspunkt fuer echtes Preis-/Material-
  Mapping nach vorhandenem Herkunftsanker fachlich inventarisieren

Fuer den aktuellen Block gilt jedoch:

- **abgeschlossen**
