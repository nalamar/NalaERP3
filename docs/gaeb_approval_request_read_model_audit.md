# GAEB-Freigabeanforderungen: Lesesicht-Audit

## Ziel dieses Audits

Dieses Dokument prueft die umgesetzte Lesesicht fuer aktive
positionsbezogene Freigabeanforderungen und entscheidet den naechsten
kleinen Folgepfad.

Scope dieses Audits:

- aktive `requested`-Anforderung in Quote-Item-Responses
- Wiederanzeige im bestehenden Quote-Editor nach Reload
- API- und Persistenzkanten vor Storno oder Entscheidung
- keine neue Implementierung

## 1. Ergebnis

Die Lesesicht ist fuer den zugeschnittenen Zweck umgesetzt.

Erfuellt:

- `QuoteItemInput` besitzt `active_approval_request` als optionales
  read-only Feld.
- `Service.Get(...)` liest pro Quote-Position maximal eine aktive
  `requested`-Anforderung per `LEFT JOIN LATERAL`.
- Der Client liest `active_approval_request` beim Oeffnen einer Quote.
- Die bestehende Zielmargen-UI zeigt nach Reload wieder
  `Freigabe angefordert`.
- Der Integrationstest prueft den Reload nach erzeugter Anforderung.

Nicht umgesetzt und weiterhin bewusst ausserhalb des Lesesicht-Leafs:

- separate Freigabeliste
- Storno-Endpunkt
- Genehmigen oder Ablehnen
- Rollenmatrix
- Benachrichtigungen
- Historien- oder Timeline-Ansicht

## 2. Backend-Befund

### 2.1 DTO und Ausgabe

`QuoteItemInput` traegt die aktive Anforderung direkt am Item:

```go
ActiveApprovalRequest *QuoteItemApprovalRequest `json:"active_approval_request,omitempty"`
```

Das passt zum bestehenden Quote-Response-Modell, weil der Editor seine
Positionen bereits aus `Quote.Items` rekonstruiert.

### 2.2 Query-Verhalten

`Service.Get(...)` filtert auf:

```sql
qar.quote_item_id = qi.id
AND qar.status = 'requested'
```

Durch den Partial Unique Index auf aktive Anforderungen je
`quote_item_id` ist die Kardinalitaet fachlich eindeutig. Der
`LEFT JOIN LATERAL` ist fuer diesen schmalen Read-Case angemessen.

### 2.3 Write-Pfad bleibt unveraendert

`RequestApprovalForQuoteItem(...)` erzeugt weiterhin nur neue
`requested`-Anforderungen. Die Lesesicht fuehrt keine implizite
Mutation ein.

## 3. Client-Befund

Der Client liest `active_approval_request` in `_QuoteItemDraft.fromJson(...)`
und setzt daraus `_QuoteItemApprovalRequestDraft`.

Das vorhandene UI-Verhalten bleibt damit konsistent:

- keine aktive Anforderung: Button `Freigabe anfordern`, wenn der
  Zielmargenanker dies erlaubt
- aktive Anforderung: Status `Freigabe angefordert`
- keine Storno- oder Entscheidungsaktion sichtbar

## 4. Tests

Der Integrationstest deckt den wichtigsten API-Fall ab:

1. Quote mit unterzieliger Position erzeugen
2. Freigabeanforderung per POST erzeugen
3. Quote erneut laden
4. `active_approval_request` am betroffenen Item erwarten

Verifizierte Befehle aus dem Implementierungs-Leaf:

```text
go test ./internal/quotes ./internal/http
dart analyze lib/api.dart lib/pages/quotes_page.dart
```

## 5. Wichtige Kanten

### 5.1 Quote-Update loescht aktive Anforderungen

Aktuell ersetzt `Service.Update(...)` alle Positionen einer Draft-Quote:

```sql
DELETE FROM quote_items WHERE quote_id=$1
```

Die Tabelle `quote_item_approval_requests` referenziert `quote_items(id)`
mit `ON DELETE CASCADE`. Dadurch werden aktive Freigabeanforderungen beim
normalen Speichern eines Entwurfs geloescht, wenn der Update-Pfad genutzt
wird.

Das ist fuer Storno und Entscheidung relevant:

- Ein Storno-Endpunkt kann verschwundene Anforderungen nicht mehr stornieren.
- Ein Entscheidungsworkflow kann nicht stabil auf eine aktive Anforderung
  referenzieren, wenn ein Quote-Save sie vorher kaskadiert loescht.
- Die UI kann nach einem Speichern wieder so aussehen, als sei nie eine
  Freigabe angefordert worden.

### 5.2 Statusmodell ist noch eng

Die Migration erlaubt derzeit nur:

- `requested`
- `cancelled`

Ein Entscheidungsworkflow braucht mindestens neue Zustands- oder
Folgetabellen fuer `approved` und `rejected`. Das ist groesser als ein
Storno-MVP.

### 5.3 Keine eigene Queue

Die Lesesicht ist positionsnah und nicht als Arbeitsvorrat fuer
Freigebende geeignet. Eine spaetere Queue sollte separat geschnitten
werden, sobald Storno und Entscheidung fachlich stabil sind.

### 5.4 Zielmargenberechnung ist dupliziert

`RequestApprovalForQuoteItem(...)` berechnet Zielpreis und
Zielmargenabweichung erneut aus Preisentscheidung und Einstellung. Die
read-only Zielmargenanker nutzen eine verwandte, aber separate Logik.
Vor Entscheidungslogik sollte diese fachliche Bewertung nicht weiter
auseinanderlaufen.

## 6. Entscheidung

Nicht direkt mit Genehmigen/Ablehnen fortfahren.

Begruendung:

- Der Entscheidungsworkflow ist fachlich groesser als die aktuelle
  Persistenz tragen kann.
- Aktive Anforderungen sind durch den Quote-Update-Pfad noch nicht stabil
  genug.
- Ein reiner Storno-Endpunkt waere klein, loest aber nicht die
  Cascade-Loeschkante beim Speichern.

Naechster sinnvoller Leaf:

`Subtask 3.1.33.4: Lebenszyklusregel fuer aktive Freigabeanforderungen bei Quote-Update/Positionsersetzung zuschneiden`

Dieser Leaf soll klaeren:

- ob Quote-Save bei aktiven Anforderungen blockiert
- ob aktive Anforderungen vor Positionsersetzung automatisch storniert werden
- ob Item-Updates positionsstabil statt delete/insert werden muessen
- welche Regel fuer den ersten kleinen Implementierungsschritt gilt

Erst danach sollte der Schreibpfad fuer Storno oder Entscheidung umgesetzt
werden.
