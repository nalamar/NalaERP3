# GAEB-Freigabeentscheidungen: Prozesssperren bei offener Nacharbeit-Abschlussaudit

## Ziel dieses Audits

Dieses Dokument prueft den abgeschlossenen Block:

```text
Backend-Prozesssperren fuer offene Positionsnacharbeit
```

Geprueft wird gegen den Zuschnitt aus:

```text
docs/gaeb_approval_rework_process_lock_strategy.md
```

## 1. Ergebnis

Der Prozesssperren-Block ist fuer den aktuellen MVP-Scope abgeschlossen.

Abgesichert sind:

- Quote-Status `sent`
- Quote-Status `accepted`
- explizite Angebotsannahme ueber `Accept`
- direkte Rechnungserzeugung aus Angebot
- direkte Auftragserzeugung aus Angebot

Bewusst nicht Teil dieses Blocks:

- neue Persistenz
- neue Permission
- neuer API-Response-Typ
- Client-only-Sperren
- zentrale Nacharbeitsqueue
- Aufgabenmodell
- UX-Liste betroffener Positionen

## 2. Definition "offene Nacharbeit"

Die technische Definition entspricht dem fachlichen Zuschnitt:

- pro Angebotsposition wird die letzte terminale Freigabeentscheidung gelesen
- terminal sind `approved` und `rejected`
- nur `latest.status = rejected` blockiert
- eine spaetere `approved`-Entscheidung hebt die Sperre auf

Die Abfrage nutzt jeweils eine `LATERAL`-Selektion auf
`quote_item_approval_requests`.

## 3. Guard-Abdeckung

### 3.1 Status und Annahme

Umgesetzt in:

```text
quotes.Service.UpdateStatus
```

Blockiert:

- `sent`
- `accepted`

Indirekt mit blockiert:

- `quotes.Service.Accept`, weil der Flow `UpdateStatus(..., "accepted")` nutzt

Erlaubt bleiben:

- `draft`
- quote-level `rejected`

### 3.2 Rechnung

Umgesetzt in:

```text
quotes.Service.ConvertToInvoice
```

Der Guard laeuft innerhalb der bestehenden Transaktion nach den Guards fuer:

- historische Angebotsversionen
- vorhandene Folgebelege
- erlaubte Quote-Status

Er laeuft vor:

- Lesen der Rechnungspositionen
- `CreateFromQuoteTx`
- Setzen von `linked_invoice_out_id`
- faktischer Annahme des Angebots

### 3.3 Auftrag

Umgesetzt in:

```text
sales.Service.CreateFromQuote
```

Der Guard laeuft innerhalb der Sales-Transaktion nach den Guards fuer:

- historische Angebotsversionen
- Quote-Status `accepted`
- vorhandene Folgebelege

Er laeuft vor:

- Lesen der Auftragspositionen
- Schreiben von `sales_orders`
- Setzen von `linked_sales_order_id`

## 4. Testabdeckung

Relevante Integrationstests:

```text
TestQuoteStatusBlocksOpenApprovalRework
TestQuoteConvertToInvoiceBlocksOpenApprovalRework
TestQuoteConvertToSalesOrderBlocksOpenApprovalRework
```

Abgedeckte Assertions:

- `sent` scheitert bei zuletzt `rejected`
- `accepted` scheitert bei zuletzt `rejected`
- quote-level `rejected` bleibt erlaubt
- spaetere `approved`-Entscheidung erlaubt wieder `sent`
- direkte Rechnungserzeugung scheitert bei offener Nacharbeit
- blockierte Rechnung erzeugt keinen `linked_invoice_out_id`
- direkte Auftragserzeugung scheitert bei offener Nacharbeit
- blockierter Auftrag erzeugt keinen `linked_sales_order_id`
- blockierter Auftrag erzeugt keinen `sales_orders`-Datensatz

Ausgefuehrte Abschlussverifikation:

```text
go test ./internal/quotes ./internal/sales ./internal/http
```

Ergebnis:

- Go-Tests gruen
- `internal/sales` hat keine eigenen Testdateien; die Sales-Guard-Wirkung wird
  ueber HTTP-Integration getestet

## 5. Bewertung der Scope-Grenzen

### 5.1 Keine Client-Sperre

Bewertung:

Korrekt. Die fachlich belastbare Sperre liegt serverseitig. Die bestehende
Client-Warnung bleibt eine Sichtbarkeits- und Orientierungshilfe.

### 5.2 Keine neue Persistenz

Bewertung:

Korrekt. Der Status ergibt sich aus vorhandenen Freigabeentscheidungen.

### 5.3 Doppelte Helper in Quotes und Sales

Bewertung:

Akzeptabel fuer diesen Schritt. Der Guard sitzt an zwei Service-Boundaries. Ein
Shared-Paket waere erst sinnvoll, wenn weitere Domains dieselbe Regel brauchen
oder zusaetzliche Freigabe-Aggregate eingefuehrt werden.

### 5.4 Kein eigener Accept-Test

Bewertung:

Akzeptabel. `Accept` delegiert ohne eigene Umgehung auf `UpdateStatus(...,
"accepted")`. Der blockierte Zielstatus `accepted` ist im Status-Test
abgedeckt.

## 6. Entscheidung

Subtask 3.1.41.5 ist abgeschlossen.

Innerhalb des Prozesssperren-Blocks ist kein weiterer kleiner Haertungsschritt
mit gutem Signal offen.

Der naechste sinnvolle Folgepfad ist kein weiterer Guard, sondern eine neue
fachliche Inventur:

```text
Subtask 3.1.42.1: Nacharbeits-Folgepfad nach blockierten Prozessaktionen fachlich inventarisieren
```

Moegliche Richtungen fuer diesen Folgepfad:

- betroffene Positionen im Angebotskopf gezielter sichtbar machen
- Sprung von Warnung zu betroffenen Positionen
- Remediation-Status nach Korrektur klarer machen
- zentrale Freigabe-/Nacharbeitsuebersicht vorbereiten
- spaetere Aufgaben- oder Queue-Logik abgrenzen
