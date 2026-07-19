# GAEB-Freigabehistorie: Nutzeranzeigenamen-Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das kleinste technische Zielmodell fuer lesbare
Nutzeranzeigenamen in der bestehenden Freigabehistorie zu.

Ausgangspunkt:

- Freigabehistorie ist positionsbezogen ueber
  `GET /api/v1/quotes/{id}/items/{itemID}/approval-requests` lesbar.
- Historie enthaelt technische IDs in `requested_by`, `cancelled_by` und
  `decided_by`.
- Auth/User-Daten enthalten bereits `display_name` und `email`.
- Es soll keine Queue, kein Badge und keine neue Mutation entstehen.

## 1. Kleinster fachlicher Scope

Enthalten:

- optionale Anzeigename-Felder fuer die bestehende Freigabehistorie
- Fallback `display_name -> email -> id`
- Client-Anzeige nutzt Anzeigename, faellt aber auf technische ID zurueck
- keine Persistenzmigration
- keine neue Permission

Nicht enthalten:

- zentrale Freigabe-Queue
- Entscheidungsbadge an Quote-Positionen
- User-Snapshot in `quote_item_approval_requests`
- Aenderung an Auth/User-Modell
- Aenderung an Genehmigen/Ablehnen/Storno
- automatische Historienaktualisierung nach Entscheidung

## 2. Response-Felder

Das bestehende Readmodel `QuoteItemApprovalRequest` soll um drei optionale
Felder erweitert werden:

```text
requested_by_name
cancelled_by_name
decided_by_name
```

Begruendung:

- Die technischen ID-Felder bleiben fuer Auditierbarkeit und
  Rueckwaertskompatibilitaet erhalten.
- Neue Felder sind optional und brechen bestehende Clients nicht.
- Die Namen sind reine Darstellungsfelder.

Nicht verwenden:

- `requested_by_display_name`
- `cancelled_by_display_name`
- `decided_by_display_name`

Grund: Die kuerzere `*_name`-Variante ist fuer UI und spaetere Queue
ausreichend klar und bleibt nahe an bisherigen JSON-Konventionen.

## 3. Backend-Zuschnitt

### 3.1 Struct

`server/internal/quotes/service.go`:

```go
RequestedByName string `json:"requested_by_name,omitempty"`
CancelledByName string `json:"cancelled_by_name,omitempty"`
DecidedByName   string `json:"decided_by_name,omitempty"`
```

Position im Struct:

- direkt nach dem jeweiligen technischen ID-Feld

### 3.2 Namensbildung

Fallback-Expression:

```sql
COALESCE(NULLIF(u.display_name, ''), NULLIF(u.email, ''), user_id)
```

Fuer die drei Rollen:

- `requested_by` -> `requested_user`
- `cancelled_by` -> `cancelled_user`
- `decided_by` -> `decided_user`

Bei leerer technischer ID bleibt auch `*_by_name` leer.

### 3.3 Scope des ersten Backend-Leafs

Nur dieser Lesepfad wird erweitert:

```text
ListApprovalRequestsForQuoteItem(...)
```

Nicht erweitern im ersten Backend-Leaf:

- `active_approval_request` in `Service.Get(...)`
- Rueckgabe von `RequestApprovalForQuoteItem(...)`
- Rueckgabe von `CancelApprovalRequestForQuoteItem(...)`
- Rueckgabe von `ApproveApprovalRequestForQuoteItem(...)`
- Rueckgabe von `RejectApprovalRequestForQuoteItem(...)`

Begruendung:

- Ziel ist die bestehende Historienanzeige.
- Der aktive Quote-Detailpfad ist bereits umfangreich.
- Schreibantworten muessen fuer diese UI nicht direkt Namen liefern.
- Die Historie kann nach explizitem Abruf die lesbaren Namen zeigen.

### 3.4 Query-Form

Der Historien-Query kann drei `LEFT JOIN users` erhalten:

```sql
LEFT JOIN users requested_user ON requested_user.id = qar.requested_by
LEFT JOIN users cancelled_user ON cancelled_user.id = qar.cancelled_by
LEFT JOIN users decided_user ON decided_user.id = qar.decided_by
```

Die Select-Liste wird nach den technischen IDs um die Namen erweitert.

Wichtig:

- Joins duerfen keine Historieneintraege verlieren.
- geloeschte oder fehlende User muessen weiterhin technische IDs anzeigen.
- `ON DELETE SET NULL` oder fehlende User duerfen die Historie nicht
  unlesbar machen.

## 4. Scan-Strategie

`scanApprovalRequest(...)` wird erweitert um:

- `requestedByName sql.NullString`
- `cancelledByName sql.NullString`
- `decidedByName sql.NullString`

Reihenfolge im Scan muss exakt zur Select-Liste passen.

Da `scanApprovalRequest(...)` auch von Write-Pfaden genutzt wird, muessen alle
Select-Listen, die diesen Scanner verwenden, um leere Name-Ausdruecke
erweitert werden.

Minimal fuer Write-Pfade:

```sql
'' AS requested_by_name,
'' AS cancelled_by_name,
'' AS decided_by_name
```

Alternative waere ein zweiter Scanner nur fuer Historie. Fuer den ersten
Implementierungs-Leaf ist die einheitliche Scanner-Erweiterung mit leeren
Aliasen einfacher und weniger fehleranfaellig.

## 5. Client-Zuschnitt

### 5.1 Draft

`_QuoteItemApprovalRequestDraft` wird erweitert:

```text
requestedByName
cancelledByName
decidedByName
```

Parsing:

```text
requested_by_name
cancelled_by_name
decided_by_name
```

### 5.2 Anzeige

Neue Helper:

```text
displayRequestedBy
displayCancelledBy
displayDecidedBy
```

Fallback:

```text
name.trim().isNotEmpty ? name : technicalId
```

Die bestehenden Historienzeilen wechseln von:

```text
von ${entry.requestedBy}
von ${entry.cancelledBy}
von ${entry.decidedBy}
```

auf:

```text
von ${entry.displayRequestedBy}
von ${entry.displayCancelledBy}
von ${entry.displayDecidedBy}
```

Wenn auch der Fallback leer ist, bleibt die bisherige Variante ohne `von ...`.

## 6. Testschnitt

Backend:

- bestehenden HTTP-Historientest erweitern
- Testuser mit `display_name` wird bereits ueber Testutil oder Seed erzeugt
- History-Response enthaelt `decided_by` und `decided_by_name`
- bei Approval sollte `decided_by_name` nicht leer sein
- optional: `requested_by_name` fuer Sales-/Requester-User pruefen

Client:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Verifikation:

```text
go test ./internal/quotes ./internal/http
```

## 7. Nicht-Ziele des Implementierungs-Blocks

Nicht Teil der naechsten Implementierung:

- Queue-Endpoint
- Queue-UI
- Entscheidungsbadge
- Erweiterung aller aktiven Approval-Antworten
- User-Snapshot in Approval-Tabelle
- Migration
- neue Permissions
- Umbenennung bestehender technischer ID-Felder

## 8. Naechster Implementierungs-Leaf

```text
Subtask 3.1.37.3: Backend-Historien-Readmodel um Nutzeranzeigenamen erweitern
```

Umfang:

- `QuoteItemApprovalRequest` um Name-Felder erweitern
- `ListApprovalRequestsForQuoteItem(...)` mit User-Joins erweitern
- alle `scanApprovalRequest(...)`-Select-Listen kompatibel halten
- HTTP-Test fuer Name-Felder erweitern

Nicht enthalten:

- Client-Anzeige
- Queue
- Badge
