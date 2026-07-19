# GAEB-Freigabe: Nacharbeitsabschluss Backend-Audit

## Scope

Dieses Audit schliesst den Backend-Leaf fuer den expliziten
Nacharbeitsabschluss bei erreichter Zielmarge.

Umgesetzt wurden:

- Service-Methode `ResolveApprovalReworkForQuoteItem(...)`
- HTTP-Endpunkt
  `POST /api/v1/quotes/{id}/items/{itemID}/approval-rework/resolve`
- Permission-Gate `quotes.approve`
- Domainfehler-Klassifizierung fuer neue Nacharbeitsfehler
- Integrationstest fuer Permission, Zielmargen-Guard und Readmodel-Reload

Nicht umgesetzt wurden:

- Client-API
- Client-Button im Zielmargenblock
- gesonderte Prozesssperren-Tests fuer Folgebelege nach `rework_resolved`

## Service-Verhalten

`ResolveApprovalReworkForQuoteItem(...)` schreibt eine neue terminale
Historienzeile mit:

```text
status = rework_resolved
```

Der Service prueft:

- Quote ist `draft`
- Quote ist keine historische Version
- Position gehoert zur Quote
- keine aktive `requested`-Anforderung existiert
- neueste terminale Entscheidung ist `rejected`
- Preisentscheidung als Kostenbasis existiert
- aktuelle Zielmarge ist erreicht oder uebertroffen
- Kommentar ist maximal 500 Zeichen

Die Zielmargenbewertung wird serverseitig in derselben Transaktion aus
aktuellem Positionspreis, letzter Preisentscheidung und aktueller
Zielmargen-Konfiguration abgeleitet.

## Persistierte Snapshots

Die neue `rework_resolved`-Zeile speichert:

- aktuellen Positionspreis als `current_unit_price_snapshot`
- aktuelle Kostenbasis
- aktuellen Zielpreis
- aktuelle Zielmarge
- aktuelle Zielabweichung
- aktuelle Marge
- verwendete Preisentscheidung
- `decided_by`
- `decided_at`
- `decision_comment`
- aktuellen Positionspreis als `approved_unit_price_snapshot`
- aktuelle Zielmarge als `approved_target_margin_percent_snapshot`

`reason_code` und `reason_text` werden aus der letzten abgelehnten
Freigabeentscheidung uebernommen, damit der Abschluss fachlich auf die
urspruengliche Nacharbeit bezogen bleibt.

## API-Verhalten

Endpunkt:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-rework/resolve
```

Payload:

```json
{
  "comment": "optional"
}
```

Berechtigung:

```text
quotes.approve
```

Antwort:

- `200 OK`
- serialisierte `QuoteItemApprovalRequest`

## Verifikation

Ausgefuehrt:

```text
gofmt -w internal/quotes/service.go internal/http/v1.go internal/http/quotes_integration_test.go
go test ./internal/migrate ./internal/quotes ./internal/http
```

Ergebnis:

- alle genannten Go-Tests erfolgreich

## Offene Folge

Der naechste Leaf soll den Prozesssperrenpfad nach `rework_resolved`
ausdruecklich testen:

- Statuswechsel nach `sent`
- Rechnungserzeugung
- Auftragserzeugung

