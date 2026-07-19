# GAEB-Freigabeanforderungen: Storno-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten Storno-Flow fuer aktive
positionsbezogene Freigabeanforderungen.

Geprueft werden:

- Backend-Service
- HTTP-Route
- Integrationstest
- Client-API
- Quote-Editor-UI
- offene Kanten vor Genehmigen/Ablehnen

## 1. Ergebnis

Der Storno-Flow ist fuer den zugeschnittenen MVP-Scope umgesetzt.

Erfuellt:

- Aktive `requested`-Anforderungen koennen explizit auf `cancelled`
  gesetzt werden.
- `cancelled_by`, `cancelled_at` und `updated_at` werden gesetzt.
- Quote, Quote-Items, Preise, Summen und Angebotsstatus werden nicht
  veraendert.
- `Service.Update(...)` blockiert weiterhin aktive Anforderungen.
- Nach Storno zeigt die Quote-Detail-Lesesicht kein
  `active_approval_request` mehr.
- Nach Storno ist ein normaler Quote-PATCH wieder moeglich.
- Ein zweiter Storno-Versuch liefert `400 Bad Request`.
- Der Client bietet eine sichtbare Storno-Aktion im Zielmargenblock an.

Nicht umgesetzt und weiterhin ausserhalb dieses Flows:

- Genehmigen oder Ablehnen
- Freigabe-Queue
- Rollenmatrix fuer Freigebende
- Kommentar beim Storno
- Timeline- oder Historien-UI
- positionsstabiles Quote-Update

## 2. Backend-Befund

### 2.1 Service

`CancelApprovalRequestForQuoteItem(...)` ist als eigener Service-Schnitt
umgesetzt.

Die fachlichen Guards sind passend:

- Quote wird per `FOR UPDATE` gelockt.
- Historische Angebotsversionen bleiben schreibgeschuetzt.
- Nur Draft-Quotes duerfen storniert werden.
- Die Position muss zur Quote gehoeren.
- Nur `status = 'requested'` wird storniert.

Die Mutation ist eng:

```sql
UPDATE quote_item_approval_requests
SET status = 'cancelled',
    cancelled_by = NULLIF($3, ''),
    cancelled_at = now(),
    updated_at = now()
WHERE quote_id = $1
  AND quote_item_id = $2
  AND status = 'requested'
```

Dadurch bleiben alle Snapshots und die urspruengliche Anforderung
erhalten.

### 2.2 Persistenz

Die bestehende Migration traegt den Flow:

- Status erlaubt `requested` und `cancelled`.
- `cancelled_at` ist bei `cancelled` verpflichtend.
- Der Partial Unique Index gilt nur fuer `requested`.

Damit ist nach einem Storno eine spaetere neue Anforderung fuer dieselbe
Position technisch moeglich.

## 3. HTTP/API-Befund

Route:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/cancel
```

Bewertung:

- `POST` passt, weil Storno eine fachliche Zustandsaenderung ist.
- `quotes.write` ist konsistent mit Erzeugen und Quote-Bearbeitung.
- Die Route liefert `200 OK` mit `QuoteItemApprovalRequest`.
- Nicht-idempotentes Verhalten ist umgesetzt und getestet.

Der Fehler `Keine aktive Freigabeanforderung vorhanden` wird ueber die
bestehende Freigabeanforderungs-Klassifizierung als `400 validation_error`
behandelt.

## 4. Test-Befund

Der Integrationstest deckt die kritische Sequenz ab:

1. aktive Anforderung erzeugen
2. Quote-Save wird bei aktiver Anforderung blockiert
3. Storno-Route liefert `status = "cancelled"`
4. `cancelled_by` und `cancelled_at` sind gesetzt
5. Quote-Reload enthaelt kein `active_approval_request`
6. zweiter Storno liefert 400
7. Quote-PATCH ist nach Storno wieder moeglich

Damit ist der urspruengliche Cascade-Verlust nicht nur verhindert, sondern
auch bewusst aufloesbar.

## 5. Client-Befund

### 5.1 API

`cancelQuoteItemApprovalRequest(...)` ruft den neuen Storno-Endpunkt auf,
erwartet `200 OK` und gibt die Response als Map zurueck.

Das passt zum bestehenden Pattern von `requestQuoteItemApproval(...)`.

### 5.2 UI

Der Quote-Editor nutzt:

- `_cancellingApprovalItemId`
- `_cancelApproval(...)`
- `onCancelApproval`
- `cancellingApproval`

Bei aktiver `approvalRequest` wird im Zielmargenblock
`Freigabe stornieren` angezeigt.

Nach Erfolg:

- `item.approvalRequest = null`
- SnackBar `Freigabeanforderung storniert`
- der Button `Freigabe anfordern` kann wieder erscheinen, wenn der
  Zielmargenanker weiter eine Anforderung erlaubt

Das ist fuer den MVP akzeptabel, weil der Server die Quelle der Wahrheit
bleibt und ein spaeterer Reload denselben Zustand liefert.

## 6. Offene Kanten

### 6.1 Kein Storno-Kommentar

Der Storno speichert keinen Grund. Fuer den MVP ist das vertretbar, weil der
Flow primaer den Save-Block aufloest. Fuer Auditierbarkeit im ERP-Kontext
sollte spaeter ein optionaler Storno-Kommentar ergaenzt werden.

### 6.2 Kein sichtbarer Verlauf

Stornierte Anforderungen sind nicht in der Quote-UI sichtbar. Die Daten
bleiben persistiert, aber es gibt noch keine Historienansicht.

### 6.3 Neue Anforderung nach Storno

Nach Storno kann fuer dieselbe Position erneut eine Anforderung erzeugt
werden, sofern die Preis-/Zielmargenregeln weiterhin greifen. Das ist
technisch korrekt, braucht spaeter aber Historienkontext, damit Nutzer
mehrfache Request-/Cancel-Zyklen verstehen.

### 6.4 Quote-Update bleibt delete/insert

Storno loest den unmittelbaren Save-Block. Nach dem anschliessenden
Quote-PATCH werden Positionen weiterhin ersetzt. Das ist fuer den
aktuellen MVP bewusst akzeptiert, weil keine aktive Anforderung mehr an
den alten Positionen haengt.

Fuer Genehmigen/Ablehnen bleibt positionsstabiles Update oder eine klare
Entscheidungsbindung trotzdem ein relevantes Architekturthema.

### 6.5 Keine Freigabeentscheidung

`cancelled` bedeutet Ruecknahme der Anforderung, nicht fachliche Freigabe.
Der naechste Entscheidungsworkflow darf Storno nicht als Genehmigung
interpretieren.

## 7. Verifikation

Bereits im Implementierungsverlauf ausgefuehrt:

```text
go test ./internal/quotes ./internal/http
dart analyze lib/api.dart lib/pages/quotes_page.dart
```

Beide Pruefungen waren ohne Befund.

## 8. Entscheidung fuer den Folgepfad

Request, Lesesicht, Save-Guard und Storno bilden jetzt einen geschlossenen
MVP-Kreis.

Der naechste sinnvolle Pfad ist nicht ein weiterer Storno-Ausbau, sondern
der Zuschnitt des eigentlichen Entscheidungsworkflows:

- Welche Status oder Folgetabellen braucht Genehmigen/Ablehnen?
- Wer darf entscheiden?
- Welche Preis-/Statusmutation ist bei Genehmigung erlaubt?
- Wie bleibt die Bindung an Quote-Item, Preisentscheidung und Snapshots
  nachvollziehbar?

Naechster Leaf:

`Subtask 3.1.34.1: Entscheidungsworkflow fuer Freigabeanforderungen zuschneiden`
