# Post-Apply-Navigation: Abschluss-Audit

## Ziel dieses Dokuments

Dieses Dokument bewertet den kleinen Anschlussschritt nach dem GAEB-Apply-MVP:

- direkte Navigation zur erzeugten Quote aus `created_quote_id`
- minimale Transparenz im bestehenden Importdialog

Es wird geprueft, ob innerhalb dieses engen Blocks noch genau ein kleiner
Haertungsschritt mit gutem Signal uebrig ist oder ob der Block sauber
abgeschlossen werden sollte.

## 1. Abgedeckter Scope

Nach `3.1.9.3` ist folgendes umgesetzt:

- bei vorhandenem `created_quote_id` wird im GAEB-Importdialog `Quote oeffnen`
  angeboten
- Aktion schliesst den Dialog, laedt die Zielquote und aktualisiert die
  Quotes-Ansicht
- kurze Transparenzzeile zur erzeugten Quote ist sichtbar
- bestehende Fehlerdarstellung wird weiterverwendet

Damit ist die Strecke

- `Apply erfolgreich -> erzeugte Quote direkt oeffnen -> Angebotsarbeit fortsetzen`

als kleinster operativer Anschluss nun vorhanden.

## 2. Bewertung der Restluecken

Im direkten Umfeld bleiben als sinnvolle Folgepunkte:

- feinere Apply-Transparenz (z. B. uebernommene Positionen kompakt)
- globale Freigabe-/Apply-Uebersicht
- spaeter Mapping-Ausbau (Preis/Material/Steuer)

Diese Punkte sind jedoch bereits neue Ausbaustufen und keine kleine Resthaertung
des eingefuehrten Navigationsschnitts.

## 3. Kandidat fuer moegliche Resthaertung

Ein denkbarer kleiner Zusatz waere:

- automatische visuelle Fokussierung der Zielquote in der Liste

Das liefert hier aber nur begrenztes Signal, weil:

- die Zielquote bereits geladen wird
- der Nutzer den Kontextwechsel bereits eindeutig bekommt
- kein fachliches Risiko reduziert wird

## 4. Entscheidung

Innerhalb des Blocks

- direkte Quote-Navigation nach Apply

bleibt **kein weiterer kleiner Haertungsschritt mit gutem Signal** uebrig.

Der Post-Apply-Navigationsschritt ist damit sauber abgeschlossen.

## 5. Naechster sinnvoller Abschnitt

Der naechste sinnvolle Strang ist nicht weiterer Navigations-Feinschliff,
sondern ein neuer Transparenz- bzw. Mapping-Vorbereitungsabschnitt.

Als kleinster naechster Startpunkt bietet sich an:

- fachliche Inventur fuer schlanke Apply-Transparenz auf Importlauf-Ebene
  (ohne bereits in Preis-/Material-Mapping abzudriften).
