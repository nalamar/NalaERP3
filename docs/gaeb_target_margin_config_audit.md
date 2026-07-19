# GAEB-Zielmargen-Konfiguration: Ende-zu-Ende-Audit

## Scope

Dieses Audit bewertet den Stand nach den Leaves `3.1.30.3` und `3.1.30.4`.

Geprueft wurde der schmale Pfad:

- Migration und Seed fuer den globalen Zielmargenwert
- Settings-Service und Settings-Routen
- read-only Nutzung im Zielmargenanker
- minimale Flutter-API und Settings-UI
- fokussierte Backend- und Client-Verifikation

Nicht bewertet wurden bewusst ausgeschlossene Folgefunktionen wie
Zielpreis-Uebernahme, Preisautomatik, Freigabe-Workflow, Rollenmatrix,
Bulk-Bearbeitung, Regelmatrix oder Vollkalkulation.

## Ergebnis

Der End-to-End-Pfad ist fuer den Minimalumfang fachlich nutzbar:

1. `049_quote_calculation_settings.sql` legt `quote_calculation_settings`
   an und seedet `id='default'` mit `target_margin_percent = 20.00`.
2. `QuoteCalculationSettingsService` liest und schreibt den Default-Datensatz.
3. `GET/PUT /api/v1/settings/quote-calculation` sind unter
   `settings.manage` registriert.
4. `TargetMarginAnchorForQuoteItem(...)` liest den konfigurierten Wert und
   faellt bei technischen Fehlern oder ungueltigen Persistenzwerten auf `20.00`
   zurueck.
5. Der Flutter-Client hat passende API-Methoden und eine minimale
   Settings-Sektion `Angebotskalkulation`.

Der Zielmargenanker bleibt read-only. Eine Aenderung der Zielmarge erzeugt
keine automatische Preisveraenderung und keine Angebotsmutation.

## Abgleich Gegen Strategie

| Bereich | Soll | Ist | Befund |
| --- | --- | --- | --- |
| Persistenz | `quote_calculation_settings` mit Default `20.00` | umgesetzt | ok |
| Backend-Service | `Get` und `Upsert` in Settings-Domaene | umgesetzt | ok |
| API | `GET/PUT /api/v1/settings/quote-calculation` | umgesetzt | ok |
| Berechtigung | `settings.manage` | umgesetzt | ok |
| Zielmargenanker | read-only Nutzung der Settings-Tabelle | umgesetzt | ok |
| Backend-Test | GET, PUT, negativer Wert, Anchor-Nutzung | umgesetzt | ok |
| Client-API | GET/PUT-Methoden | umgesetzt | ok |
| Client-UI | kleine Settings-Flaeche | umgesetzt | ok |

## Festgestellte Kanten

### 1. Wert `0` Wird Als Default Behandelt

Der Service normalisiert `target_margin_percent == 0` aktuell auf `20.00`.

Das entspricht dem Strategiepunkt "leer oder nicht gesetzt -> 20.00", macht
aber einen expliziten Zielmargenwert `0.00` derzeit nicht speicherbar. Falls
`0.00` fachlich als gueltiger Wert gelten soll, muss der Request zwischen
"nicht gesetzt" und "gesetzt mit 0" unterscheiden, zum Beispiel ueber Pointer-
DTO oder einen separaten Request-Typ.

Empfehlung:

- kurzfristig akzeptabel, weil der Standardfall `20.00` stabil ist
- vor Freigabe einer echten Kalkulationsregel als eigene Subtask klaeren

### 2. Settings-GET Ist Nicht Selbstheilend

Der Zielmargenanker hat einen robusten Fallback auf `20.00`. Der Settings-GET
gibt dagegen einen Fehler zurueck, wenn der Default-Datensatz fehlt.

Da die Migration seedet, ist das im Normalbetrieb ok. In Alt-/Testdaten kann
der Client aber keine Settings laden, obwohl der Zielmargenanker weiter
funktioniert.

Empfehlung:

- spaeter `GetOrDefault` oder selbstheilendes Re-Seed im Settings-Service
  einfuehren

### 3. Read-Only Anchor Haengt An `quotes.write`

Die Route `target-margin-anchor` ist read-only, nutzt aber aktuell
`quotes.write`. Das passt zum bestehenden Umfeld der Positionskalkulation, ist
fachlich aber strenger als noetig.

Empfehlung:

- im Rollen-/Berechtigungs-Audit entscheiden, ob reine Bewertungsanker auf
  `quotes.read` wechseln sollen

### 4. Client-Validierung Ist Minimal

Die Flutter-UI akzeptiert Text, ersetzt Komma durch Punkt und uebergibt einen
Dezimalwert. Grenzwerte und Fehlermeldungen kommen aus dem Backend.

Empfehlung:

- fuer produktive Settings-Bedienung lokale Eingrenzung `0..1000` und
  deutlichere Feldfehlermeldung ergaenzen

### 5. Voller Flutter-Analyze Ist Durch Fremdfehler Blockiert

Die geaenderten Dateien sind isoliert sauber:

```text
dart analyze lib/api.dart lib/pages/settings_page.dart
```

Der globale Lauf:

```text
flutter analyze
```

scheitert weiterhin an bestehenden, fachfremden Fehlern in
`bank_statements_page.dart` und `employees_page.dart`, weil dort ApiClient-
Methoden fehlen. Diese Fehler gehoeren nicht zum Zielmargenpfad, blockieren
aber die globale Client-Qualitaetssicherung.

## Offene Folgepunkte

Priorisierte Folgepunkte nach diesem Audit:

1. Entscheidung zu explizitem Zielmargenwert `0.00` treffen und Backend-DTO
   bei Bedarf anpassen.
2. Settings-Service optional selbstheilend machen, falls `id='default'` fehlt.
3. Client-Feldvalidierung fuer `0..1000` und spezifische Fehlermeldung
   ergaenzen.
4. Berechtigung der read-only Bewertungsanker (`quotes.read` vs.
   `quotes.write`) gesamthaft pruefen.
5. Globale Flutter-Analyse durch fehlende ApiClient-Methoden fuer
   Bankauszuege und Mitarbeitende entblocken.
6. Danach erst Zielpreis-Uebernahme oder Freigabe-Hinweis als naechsten
   fachlichen Schreibpfad planen.

## Verifikation

Bereits ausgefuehrte Pruefungen:

```text
go test ./internal/settings ./internal/quotes ./internal/http
dart analyze lib/api.dart lib/pages/settings_page.dart
```

Bekannter Rest:

```text
flutter analyze
```

ist wegen bestehender, fachfremder Client-Fehler nicht gruen.
