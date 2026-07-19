# GAEB-Freigabeentscheidungen: Status-Guard bei offener Nacharbeit-Audit

## Ziel dieses Audits

Dieses Dokument prueft den ersten Runtime-Schritt der Backend-Prozesssperre:

```text
Quote-Statuswechsel nach sent oder accepted bei offener Positionsnacharbeit sperren
```

Der Scope ist bewusst kleiner als die vollstaendige Prozesssperre. Rechnung und
Auftrag folgen in separaten Leaves.

## 1. Ergebnis

Der Status-Guard ist fuer den aktuellen Leaf abgeschlossen.

Erfuellt:

- `quotes.Service.UpdateStatus` prueft vor `sent` und `accepted`, ob offene
  Positionsnacharbeit existiert.
- Offene Nacharbeit wird aus der letzten terminalen Positionsfreigabeentscheidung
  abgeleitet.
- `rejected` als Quote-Level-Status bleibt erlaubt.
- Eine spaetere `approved`-Entscheidung hebt die Sperre auf.
- `Accept` wird indirekt mitgesperrt, weil der Flow `UpdateStatus(...,
  "accepted")` verwendet.

Nicht umgesetzt und bewusst ausserhalb dieses Leaves:

- Guard in `ConvertToInvoice`
- Guard in `sales.Service.CreateFromQuote`
- neue API-Felder
- neue Migration
- neue Permission
- Client-Sperre

## 2. Backend-Befund

Die neue Hilfsfunktion:

```text
quoteHasOpenApprovalRework(...)
```

verwendet dieselbe fachliche Regel wie die bestehende Anzeige:

- pro Position wird die letzte terminale Entscheidung gesucht
- terminal sind `approved` und `rejected`
- nur `latest.status = rejected` blockiert

Der Guard sitzt in `UpdateStatus` nur fuer:

- `sent`
- `accepted`

Damit bleiben Remediation-Pfade frei:

- `draft`
- `rejected`
- Entwurfsbearbeitung nach bestehenden Regeln
- erneute Freigabe

## 3. Fehlertext

Verwendeter Domainfehler:

```text
Angebot enthaelt abgelehnte Freigabeentscheidungen; Nacharbeit vor Versand, Annahme oder Folgebeleg erforderlich
```

Bewertung:

- konsistent mit dem fachlichen Zuschnitt
- fuer HTTP-Domainfehler ohne neuen Error-Code geeignet
- fuer spaetere Folgebeleg-Guards wiederverwendbar

## 4. Test-Befund

Neuer Integrationstest:

```text
TestQuoteStatusBlocksOpenApprovalRework
```

Gepruefte Pfade:

- Quote mit zuletzt `rejected` Position blockiert Status `sent`
- Quote mit zuletzt `rejected` Position blockiert Status `accepted`
- Quote-Level-Status `rejected` bleibt erlaubt
- Quote mit frueherem `rejected`, aber spaeterem `approved`, darf auf `sent`
  wechseln

Ausgefuehrte Befehle:

```text
gofmt -w internal/quotes/service.go internal/http/quotes_integration_test.go
go test ./internal/quotes ./internal/http
```

Ergebnis:

- Go-Tests gruen

## 5. Entscheidung

Subtask 3.1.41.2 ist abgeschlossen.

Der naechste kleinste Folgepunkt ist:

```text
Subtask 3.1.41.3: Backend-Guard fuer Rechnungserzeugung aus Angebot implementieren
```

Begruendung:

Der Statuswechsel ist jetzt abgesichert. Die direkte Rechnungserzeugung bleibt
aber ein eigener Folgepfad, weil sie in `ConvertToInvoice` transaktional einen
Folgebeleg erzeugt und das Angebot faktisch annimmt.
