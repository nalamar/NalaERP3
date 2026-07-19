# GAEB-Freigabeanforderungen: Storno-Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten expliziten Storno-Pfad fuer aktive
positionsbezogene Freigabeanforderungen zu.

Der Storno-Pfad soll den mit `3.1.33.5` eingefuehrten Save-Block bewusst
aufloesen:

- aktive Anforderung stornieren
- danach Quote wieder bearbeitbar machen
- keine Genehmigungsentscheidung treffen
- keine Preise, Summen oder Angebotsstatus veraendern

## 1. Ausgangspunkt

Vorhanden:

- `quote_item_approval_requests.status` erlaubt `requested` und `cancelled`
- `cancelled_by` und `cancelled_at` sind vorhanden
- aktive Anforderungen sind durch Partial Unique Index je `quote_item_id`
  eindeutig
- `Service.Update(...)` blockiert Draft-Speichern bei aktiven
  `requested`-Anforderungen
- `active_approval_request` wird read-only im Quote-Item ausgeliefert

Fehlend:

- expliziter Backend-Service zum Stornieren
- HTTP-Route
- Client-API-Methode
- UI-Aktion im bestehenden Zielmargenblock

## 2. Entscheidung

Der naechste Implementierungs-Leaf soll nur Backend und API bauen.

Client-UI folgt danach separat, weil der Backend-Schnitt zuerst stabil sein
muss.

## 3. Backend-Service

Neuer Service:

```go
func (s *Service) CancelApprovalRequestForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
    cancelledBy string,
) (*QuoteItemApprovalRequest, error)
```

Regeln:

- Quote muss existieren.
- Historische Angebotsversionen bleiben schreibgeschuetzt.
- Nur Draft-Quotes duerfen storniert werden.
- Item muss zur Quote gehoeren.
- Es muss genau eine aktive `requested`-Anforderung fuer das Item geben.
- Die Anforderung wird auf `cancelled` gesetzt.
- `cancelled_by` wird mit dem aktuellen User befuellt, wenn vorhanden.
- `cancelled_at` und `updated_at` werden auf `now()` gesetzt.
- Keine Mutation an Quote, Quote-Items, Preisen, Summen oder Status.

Fehler:

```text
Angebotsposition nicht gefunden
Keine aktive Freigabeanforderung vorhanden
Historische Angebotsversionen sind schreibgeschützt
nur Entwürfe sind bearbeitbar
```

## 4. HTTP-Route

Neue Route:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/cancel
```

Warum POST statt DELETE:

- Storno ist eine fachliche Zustandsaenderung, kein hartes Loeschen.
- Der Datensatz bleibt fuer Historie und Audit erhalten.
- Die Route passt zum bestehenden action-orientierten Stil wie
  `convert-to-sales-order`.

Permission:

```text
quotes.write
```

Response:

- `200 OK`
- JSON `QuoteItemApprovalRequest` mit `status = "cancelled"`
- `cancelled_at` gesetzt

Nicht idempotent:

- Ein zweiter Storno-Versuch liefert `400 validation_error`.
- Begruendung: Fuer den Nutzer ist wichtig, dass keine aktive Anforderung
  mehr vorhanden war. Stilles Wiederholen wuerde diesen Zustand verdecken.

## 5. API-Fehlerklassifizierung

`Keine aktive Freigabeanforderung vorhanden` muss als `400 validation_error`
klassifiziert werden.

Die bestehende Freigabeanforderungs-Klassifizierung deckt bereits viele
Meldungen ab, der genaue Text soll aber im Test abgesichert werden.

## 6. Client-Schnitt

Neue API-Methode:

```dart
Future<Map<String, dynamic>> cancelQuoteItemApprovalRequest(
  String quoteId,
  String itemId,
)
```

Route:

```text
POST /api/v1/quotes/{quoteId}/items/{itemId}/approval-requests/cancel
```

Erwartet:

- `200 OK`
- JSON der stornierten Anforderung

UI noch nicht in diesem Leaf:

- kein Button
- kein Dialog
- kein lokales Entfernen der aktiven Anforderung

## 7. Testziel

Integrationstest im bestehenden Approval-Flow:

1. aktive Freigabeanforderung erzeugen
2. Quote-Save ist blockiert
3. Storno-Route aufrufen
4. Response hat `status = "cancelled"` und `cancelled_at`
5. Quote erneut laden
6. betroffenes Item hat kein `active_approval_request`
7. Quote-Save ist wieder moeglich
8. zweiter Storno-Versuch liefert `400 Bad Request`

Damit wird belegt, dass Storno den Save-Block bewusst aufloest, ohne Preise
oder Summen direkt zu veraendern.

## 8. Nicht-Ziele

Nicht Teil des Storno-MVP:

- Genehmigen oder Ablehnen
- Kommentar beim Storno
- separate Freigabeliste
- Rollenmatrix fuer Freigebende
- Benachrichtigung
- Timeline-UI
- positionsstabiler Quote-Update

## 9. Folgepfad

Nach Backend/API-Storno:

1. Client-Button im Zielmargenblock zum Stornieren aktiver Anforderungen
2. Audit des Storno-Flows
3. Danach erst Entscheidungsworkflow fuer Genehmigen/Ablehnen zuschneiden
