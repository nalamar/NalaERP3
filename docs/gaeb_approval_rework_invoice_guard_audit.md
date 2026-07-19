# GAEB-Freigabeentscheidungen: Rechnungs-Guard bei offener Nacharbeit-Audit

## Ziel dieses Audits

Dieses Dokument prueft den zweiten Runtime-Schritt der Backend-Prozesssperre:

```text
Rechnungserzeugung aus Angebot bei offener Positionsnacharbeit sperren
```

Der Scope betrifft nur `quotes.Service.ConvertToInvoice`. Die
Auftragserzeugung bleibt ein separater Leaf.

## 1. Ergebnis

Der Rechnungs-Guard ist fuer den aktuellen Leaf abgeschlossen.

Erfuellt:

- `ConvertToInvoice` prueft innerhalb der bestehenden Transaktion auf offene
  Positionsnacharbeit.
- Die Pruefung sitzt nach den bestehenden Guards fuer historische Versionen,
  Folgebelege und erlaubte Quote-Status.
- Die Pruefung laeuft vor dem Lesen der Rechnungspositionen und vor
  `CreateFromQuoteTx`.
- Der vorhandene Fehlertext wird wiederverwendet.
- Bei Blockade bleibt das Angebot unveraendert und ohne
  `linked_invoice_out_id`.

Nicht umgesetzt und bewusst ausserhalb dieses Leaves:

- Guard in `sales.Service.CreateFromQuote`
- neuer API-Response-Typ
- neue Migration
- neue Permission
- Client-Sperre

## 2. Backend-Befund

`ConvertToInvoice` nutzt jetzt:

```text
quoteHasOpenApprovalRework(ctx, tx, quoteID)
```

Damit wird dieselbe Definition wie beim Status-Guard verwendet:

- letzte terminale Entscheidung pro Position
- terminal sind `approved` und `rejected`
- nur eine zuletzt `rejected` Position blockiert

Die Verwendung von `tx` ist wichtig, weil der Pfad anschliessend einen
Folgebeleg erzeugt und das Angebot faktisch auf `accepted` setzt.

## 3. Test-Befund

Neuer Integrationstest:

```text
TestQuoteConvertToInvoiceBlocksOpenApprovalRework
```

Gepruefte Pfade:

- Angebot wird mit zuletzt abgelehnter Positionsfreigabe vorbereitet.
- Quote-Status wird direkt auf `sent` gesetzt, um den Status-Guard bewusst zu
  umgehen und den Rechnungspfad isoliert zu testen.
- `POST /api/v1/quotes/{id}/convert-to-invoice` liefert `400`.
- Fehlertext enthaelt den Nacharbeits-Hinweis.
- Das Angebot bleibt `sent`.
- `linked_invoice_out_id` bleibt leer.

Ausgefuehrte Befehle:

```text
gofmt -w internal/quotes/service.go internal/http/quotes_integration_test.go
go test ./internal/quotes ./internal/http
```

Ergebnis:

- Go-Tests gruen

## 4. Entscheidung

Subtask 3.1.41.3 ist abgeschlossen.

Der naechste kleinste Folgepunkt ist:

```text
Subtask 3.1.41.4: Backend-Guard fuer Auftragserzeugung aus Angebot implementieren
```

Begruendung:

Statuswechsel und direkte Rechnungserzeugung sind jetzt serverseitig
abgesichert. Die Auftragserzeugung liegt technisch im Sales-Service und muss
deshalb separat an dessen Transaktionsgrenze abgesichert werden.
