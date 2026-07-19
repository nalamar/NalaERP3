# GAEB-Freigabe: `rework_resolved` Persistenz-/Readmodel-Audit

## Scope

Dieses Audit schliesst den Leaf zur Vorbereitung von Persistenz und Readmodel
fuer den expliziten Nacharbeitsabschluss.

Umgesetzt wurden:

- Datenbankstatus `rework_resolved`
- Lifecycle-Constraint fuer `rework_resolved`
- Quote-Readmodel fuer neueste terminale Entscheidung
- Prozesssperren-Readmodel in Quote- und Sales-Service
- Client-Anzeigetexte fuer den neuen Status

Nicht umgesetzt wurden:

- Service-Methode fuer Nacharbeitsabschluss
- HTTP-Endpunkt
- Client-Button
- neue Business-Mutation

## Persistenz

Neue Migration:

```text
server/internal/migrate/migrations/053_quote_approval_rework_resolved.sql
```

Sie erweitert `quote_item_approval_requests.status` um:

```text
rework_resolved
```

Die Lifecycle-Constraint erlaubt `rework_resolved` nur terminal:

```text
cancelled_at IS NULL
decided_at IS NOT NULL
approved_unit_price_snapshot IS NOT NULL
approved_target_margin_percent_snapshot IS NOT NULL
```

Damit kann der spaetere Service den Abschluss mit denselben Auditfeldern wie
eine Entscheidung schreiben.

## Readmodel

`latest_approval_decision` beruecksichtigt nun:

```text
approved, rejected, rework_resolved
```

Damit kann ein spaeterer Nacharbeitsabschluss als neuester terminaler Zustand
im Quote-Item erscheinen.

Die Prozesssperre bleibt semantisch unveraendert:

```text
Nur neuester terminaler Status rejected sperrt.
```

Da `rework_resolved` nun in der terminalen Auswahl liegt, hebt ein spaeterer
Abschluss die Sperre auf, ohne als `approved` modelliert zu werden.

## Client-Lesemodell

Der Flutter-Client zeigt `rework_resolved` als:

```text
Nacharbeit erledigt
```

Der Status gilt nicht als `requiresRework`; der rote Nacharbeitskasten bleibt
also nur fuer `rejected` sichtbar.

Im Editor wird `rework_resolved` farblich wie ein positiver terminaler Zustand
behandelt.

## Verifikation

Ausgefuehrt:

```text
gofmt -w internal/quotes/service.go internal/sales/service.go
dart format lib/pages/quotes_page.dart
go test ./internal/migrate ./internal/quotes ./internal/http
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Go-Tests erfolgreich
- Flutter-Analyse ohne Befund

## Naechster Schritt

Der naechste Leaf kann die eigentliche Mutation implementieren:

```text
ResolveApprovalReworkForQuoteItem(...)
POST /api/v1/quotes/{id}/items/{itemID}/approval-rework/resolve
```

Dabei muss serverseitig geprueft werden, dass die letzte terminale Entscheidung
`rejected` ist und die aktuelle Zielmargenbewertung `on_target` oder
`above_target` liefert.

