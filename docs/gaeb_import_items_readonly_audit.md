## Ziel

Abschluss-Audit fuer den kleinen Block `3.1.6`, bevor echte Review-Bearbeitung,
Quote-Uebernahme oder weitergehende Parserlogik begonnen werden.

## Gepruefter Scope

- `quote_import_items` als flacher Rohpositionsspeicher ist vorhanden.
- Importstatus `uploaded -> parsed/failed` ist technisch aktiv nutzbar.
- Read-only Backend-Pfade fuer Importpositionen sind vorhanden:
  - `GET /api/v1/quotes/imports/{id}/items`
  - `GET /api/v1/quotes/imports/{id}/items/{itemID}`
- Read-only Client-Sicht ist in der Angebotsumgebung sichtbar:
  - Rohpositionsliste im GAEB-Import-Detaildialog
  - kleines Importpositions-Detail ohne Schreibpfade

## Ergebnis

Der parsernahe Read-only-Block ist fuer seinen bewusst engen Scope sauber
abgeschlossen. Es bleibt innerhalb dieses Blocks kein weiterer kleiner
Haertungsschritt mit gutem Signal uebrig.

## Begruendung

- Der Datenpfad ist Ende-zu-Ende sichtbar:
  - Upload/Importlauf
  - geparste Rohpositionen
  - read-only Liste
  - read-only Detail
- Die naechsten sinnvollen Schritte waeren keine lokale Haertung mehr, sondern
  bereits neue fachliche Ausbaustufen.
- Zusätzliche Kleinschritte wie mehr Felder im Dialog, andere Sortierung oder
  optische Verdichtung wuerden den Block nicht wesentlich robuster machen.

## Bewusst nicht Teil dieses Blocks

- Review-Schreibpfade fuer `review_status` oder `review_note`
- Parser-Engine oder Formatabdeckung fuer konkrete GAEB-Varianten
- Hierarchie-/Outline-Rekonstruktion ueber einen flachen Rohpositionsspeicher hinaus
- Quote-Uebernahme aus Importpositionen
- Preis-, Material- oder KI-Mapping

## Naechster sinnvoller Abschnitt

Der naechste sinnvolle Ausbau ist nicht weiterer Read-only-Feinschliff,
sondern `3.1.7.1`: die Review-Zielstrecke fuer importierte GAEB-Positionen
fachlich inventarisieren und den kleinsten schreibenden Review-Einstiegspunkt
festlegen.
