# GAEB-Freigabeanforderungen: MVP-Audit

## Scope

Dieses Audit bewertet den kleinen Freigabeanforderungs-MVP nach den Leaves
`3.1.32.1` bis `3.1.32.7`.

Geprueft wurde der enge Pfad:

- Persistenz fuer positionsbezogene Freigabeanforderungen
- Backend-Service und API-Route fuer genau eine Anforderung
- Ableitung aus Kostenbasis und Zielmargenanker
- Client-API
- minimale Quote-Editor-UI im Zielmargenblock

Nicht bewertet wurden bewusst ausgeschlossene Folgefunktionen wie
Freigabeentscheidung, Storno, Request-Historie, Listenansicht, Rollenmatrix,
Benachrichtigungen, Angebotsstatus-Sperre oder KI-Risikobewertung.

## Ergebnis

Der MVP ist fuer den definierten Minimalumfang fachlich geschlossen:

1. `quote_item_approval_requests` persistiert eine positionsbezogene
   Freigabeanforderung mit Snapshots.
2. Es kann pro Quote-Position nur eine aktive `requested`-Anforderung geben.
3. `RequestApprovalForQuoteItem(...)` erzeugt nur bei Preis unter Kostenbasis
   oder unter Zielpreis eine Anforderung.
4. Fehlende Kostenbasis, erreichte Zielmarge und aktive Doppelanforderung
   werden fachlich blockiert.
5. Der API-Endpunkt
   `POST /api/v1/quotes/{id}/items/{itemID}/approval-requests` liefert bei
   Erfolg `201 Created`.
6. Der Flutter-Client kann den Endpunkt ueber
   `requestQuoteItemApproval(...)` aufrufen.
7. Der Quote-Editor zeigt im Zielmargenblock `Freigabe anfordern` nur bei
   `below_cost` oder `below_target`.
8. Nach Erfolg wird die erzeugte Anforderung lokal als `Freigabe angefordert`
   sichtbar.

Der Pfad mutiert keinen Positionspreis, keine Quote-Summen und keinen
Quote-Status.

## Abgleich Gegen Zielmodelle

| Bereich | Soll | Ist | Befund |
| --- | --- | --- | --- |
| Tabelle | `quote_item_approval_requests` | umgesetzt | ok |
| Statusraum | `requested`, `cancelled` | umgesetzt | ok |
| Reason-Codes | `negative_margin`, `below_target_margin` | umgesetzt | ok |
| Snapshots | Preis, Kostenbasis, Zielpreis, Zielmarge, Abweichung | umgesetzt | ok |
| Aktive Anforderung | maximal eine `requested` pro Position | Partial Unique Index + Service-Check | ok |
| Service | `RequestApprovalForQuoteItem(...)` | umgesetzt | ok |
| API | `POST .../approval-requests` | umgesetzt | ok |
| Auth | `requested_by` aus Auth-Kontext | umgesetzt | ok |
| Client-API | Methode fuer neuen POST-Endpunkt | umgesetzt | ok |
| UI | Button im Zielmargenblock bei Risiko | umgesetzt | ok |
| Nicht-Ziele | keine Entscheidung, kein Storno, keine Historie | eingehalten | ok |

## Festgestellte Kanten

### 1. Aktive Anforderungen Werden Nicht Mit Quote Geladen

Die erfolgreiche UI-Anforderung wird lokal am `_QuoteItemDraft` gespeichert.
Ein spaeteres erneutes Oeffnen der Quote laedt aktive Approval Requests noch
nicht mit.

Empfehlung:

- naechster kleiner Leaf sollte eine read-only Sicht auf aktive
  Freigabeanforderungen schaffen
- entweder direkt in Quote-Item-Responses oder als positionsbezogener
  Lesepfad

### 2. Kein Storno-Pfad

Die Tabelle kennt `cancelled`, aber es gibt noch keinen API-Pfad, um eine
aktive Anforderung zurueckzunehmen.

Empfehlung:

- Storno erst nach read-only Sicht modellieren
- klein halten: nur aktive eigene/zulässige Anforderung auf `cancelled`
  setzen

### 3. Keine Freigabeentscheidung

Der MVP erzeugt nur `requested`. Er kann nicht `approved` oder `rejected`
setzen.

Empfehlung:

- Entscheidungspfad erst nach Lesesicht und Storno zuschneiden
- dabei Rechte, Rollen und fachliche Auswirkungen auf Quote/PDF separat
  entscheiden

### 4. Read-Only Bewertungsanker Nutzen Weiter `quotes.write`

Der neue POST-Pfad benoetigt korrekt `quotes.write`. Die vorgelagerten
read-only Bewertungsanker haengen weiterhin ebenfalls an `quotes.write`.

Empfehlung:

- separates Rechte-Audit fuer `quotes.read` vs. `quotes.write`
- nicht mit Approval-Workflow vermischen

### 5. Zielbewertung Im Request-Pfad Ist Eng Reimplementiert

Der Request-Pfad berechnet Kostenbasis, Zielpreis und Reason direkt innerhalb
der Transaktion. Das ist fuer den ersten Write-Pfad nachvollziehbar, kann aber
spaeter mit dem read-only Zielmargenanker auseinanderlaufen.

Empfehlung:

- bei naechster Haertung gemeinsamen transaktionalen Helper pruefen
- gleiche Rundungs- und Toleranzlogik fuer Anzeige und Write-Pfad sichern

### 6. Keine UI-Kommentareingabe

Die Client-API kann einen Kommentar senden, die minimale UI fordert aber ohne
Kommentar an.

Empfehlung:

- Kommentar-Dialog erst einbauen, wenn aktive Anforderung read-only angezeigt
  und wiedergefunden werden kann
- vorerst ist der kommentarlose Request akzeptabel

## Verifikation

Ausgefuehrte Pruefungen im Verlauf des MVP:

```text
go test ./internal/migrate
go test ./internal/quotes ./internal/http
dart analyze lib/api.dart
dart analyze lib/api.dart lib/pages/quotes_page.dart
```

Alle genannten Pruefungen liefen ohne Befund.

## Nicht-Ziele Bleiben Offen

Weiterhin nicht enthalten:

- aktive Freigabeanforderungen in Quote-Responses
- Freigabeanforderung stornieren
- Freigabe erteilen
- Freigabe ablehnen
- Freigabehistorie oder Timeline
- Listenansicht offener Freigaben
- Rechte `quotes.approval.request`, `quotes.approval.decide`
- Benachrichtigungen
- Angebotsstatus- oder PDF-Sperre
- KI-basierte Risikobewertung

## Naechster Sinnvoller Schritt

Der naechste Leaf sollte die Lesesicht fuer aktive Anforderungen zuschneiden:

- kleinster Einstieg: read-only aktive Approval-Request-Daten an Quote-Items
  ausliefern
- alternativ: `GET /api/v1/quotes/{id}/items/{itemID}/approval-requests`
- UI danach beim erneuten Oeffnen der Quote konsistent anzeigen

Erst danach sollten Storno oder Entscheidungspfade gebaut werden.
