# GAEB-Freigabeentscheidungen: Auftrags-Guard bei offener Nacharbeit-Audit

## Ziel dieses Audits

Dieses Dokument prueft den dritten Runtime-Schritt der Backend-Prozesssperre:

```text
Auftragserzeugung aus Angebot bei offener Positionsnacharbeit sperren
```

Der Scope betrifft `sales.Service.CreateFromQuote`, weil der HTTP-Endpunkt fuer
die Angebot-zu-Auftrag-Uebergabe direkt in diesen Service delegiert.

## 1. Ergebnis

Der Auftrags-Guard ist fuer den aktuellen Leaf abgeschlossen.

Erfuellt:

- `sales.Service.CreateFromQuote` prueft innerhalb der bestehenden Transaktion
  auf offene Positionsnacharbeit.
- Die Pruefung sitzt nach den bestehenden Guards fuer historische Versionen,
  Quote-Status und Folgebelege.
- Die Pruefung laeuft vor dem Lesen der Auftragspositionen und vor dem
  Schreiben von `sales_orders`.
- Der vorhandene Fehlertext wird wiederverwendet.
- Bei Blockade bleibt das Angebot unveraendert und ohne
  `linked_sales_order_id`.
- Es wird kein `sales_orders`-Datensatz erzeugt.

Nicht umgesetzt und bewusst ausserhalb dieses Leaves:

- neuer API-Response-Typ
- neue Migration
- neue Permission
- Client-Sperre
- Refactoring in ein neues Shared-Paket

## 2. Backend-Befund

Der Sales-Service besitzt jetzt lokal:

```text
quoteHasOpenApprovalRework(...)
```

Die SQL-Regel entspricht den Guards im Quote-Service:

- letzte terminale Entscheidung pro Position
- terminal sind `approved` und `rejected`
- nur eine zuletzt `rejected` Position blockiert

Die kleine lokale Hilfsfunktion ist fuer diesen Leaf bewusst akzeptiert. Der
Guard sitzt an einer anderen Service-Boundary; ein neues Shared-Paket waere fuer
diesen engen Schritt mehr Architekturbewegung als fachlicher Gewinn.

## 3. Test-Befund

Neuer Integrationstest:

```text
TestQuoteConvertToSalesOrderBlocksOpenApprovalRework
```

Gepruefte Pfade:

- Angebot wird mit zuletzt abgelehnter Positionsfreigabe vorbereitet.
- Quote-Status wird direkt auf `accepted` gesetzt, um den Status-Guard bewusst
  zu umgehen und den Sales-Service-Pfad isoliert zu testen.
- `POST /api/v1/quotes/{id}/convert-to-sales-order` liefert `400`.
- Fehlertext enthaelt den Nacharbeits-Hinweis.
- Das Angebot bleibt `accepted`.
- `linked_sales_order_id` bleibt leer.
- Es entsteht kein `sales_orders`-Datensatz fuer die Quote.

Ausgefuehrte Befehle:

```text
gofmt -w internal/sales/service.go internal/http/quotes_integration_test.go
go test ./internal/quotes ./internal/sales ./internal/http
```

Ergebnis:

- Go-Tests gruen
- `internal/sales` hat weiterhin keine eigenen Testdateien; die Guard-Wirkung
  ist ueber den HTTP-Integrationstest abgedeckt.

## 4. Entscheidung

Subtask 3.1.41.4 ist abgeschlossen.

Der naechste kleinste Folgepunkt ist:

```text
Subtask 3.1.41.5: Prozesssperren fuer offene Nacharbeit abschliessend auditieren
```

Begruendung:

Statuswechsel, Annahme, Rechnung und Auftrag sind jetzt serverseitig
abgesichert. Der letzte Leaf dieses Blocks sollte die Abdeckung gegen den
urspruenglichen Zuschnitt pruefen und entscheiden, ob vor neuen UX- oder
Queue-Ausbaustufen noch ein kleiner Haertungsschritt fehlt.
