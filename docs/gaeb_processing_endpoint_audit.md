# GAEB: Audit des Process-Endpunkts

## Ergebnis

Subtask 3.1.74.4 ist ohne Befund abgeschlossen. Der Endpunktvertrag,
Optionskonstruktor, Berechtigungsgrenze und Fehlerklassifikation entsprechen
der Strategie.

## Befunde

- NewV1Router bleibt kompatibel; nur NewV1RouterWithOptions nimmt den
  optionalen Parser entgegen.
- POST /quotes/imports/{id}/process ist bodylos, durch quotes.write geschützt
  und delegiert genau an ProcessGAEBImport.
- Erfolgreiche Verarbeitung liefert 200 und parsed.
- Ein zweiter Aufruf liefert 409 conflict ohne weiteren Parseraufruf.
- Fehlende Berechtigung endet vor dem Service mit 403.
- Fehlender Parser liefert 500 internal_error; der Import bleibt uploaded.

## Nachweis

go test ./internal/http -run TestGAEBImportProcessEndpoint -count=1 und
git diff --check sind gruen.

## Scope

Kein konkreter Parser, Uploadcallback, Worker, UI-, Mapping-, Kalkulations-
oder KI-Ausbau wurde ergänzt.
