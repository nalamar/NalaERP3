# GAEB-Freigabeanforderungen: Backend-Request-Pfad

## Ziel dieses Dokuments

Dieses Dokument schneidet den Backend-Service- und Request-Pfad fuer
positionsbezogene Freigabeanforderungen zu.

Der Scope bleibt bewusst klein:

- genau ein Service-DTO fuer eine Freigabeanforderung
- genau eine Service-Methode zum Anfordern
- genau eine HTTP-Route
- serverseitige Ableitung aus dem Zielmargenanker
- keine Client-UI
- kein Genehmigen oder Ablehnen
- keine Rollenmatrix
- keine Benachrichtigungen

## 1. Ausgangspunkt

Vorhanden sind:

- `QuoteItemTargetMarginAnchor` als serverseitige Zielbewertung
- `quote_item_approval_requests` als persistentes Zielmodell
- bestehende positionsbezogene Write-Pfade wie
  `ApplyTargetUnitPriceForQuoteItem(...)`
- HTTP-Muster unter
  `/api/v1/quotes/{id}/items/{itemID}/...`
- Auth-Kontext mit `authUserFromContext(req.Context())`

Der neue Pfad soll diese vorhandenen Muster nutzen und keine neue
Workflow-Abstraktion einfuehren.

## 2. Response-DTO

Vorgeschlagenes DTO in `server/internal/quotes/service.go`:

```go
type QuoteItemApprovalRequest struct {
    ID                          uuid.UUID  `json:"id"`
    QuoteID                     uuid.UUID  `json:"quote_id"`
    QuoteItemID                 uuid.UUID  `json:"quote_item_id"`
    Status                      string     `json:"status"`
    ReasonCode                  string     `json:"reason_code"`
    ReasonText                  string     `json:"reason_text,omitempty"`
    CurrentUnitPriceSnapshot    float64    `json:"current_unit_price_snapshot"`
    CostBasisUnitPriceSnapshot  float64    `json:"cost_basis_unit_price_snapshot"`
    TargetUnitPriceSnapshot     float64    `json:"target_unit_price_snapshot"`
    TargetMarginPercentSnapshot float64    `json:"target_margin_percent_snapshot"`
    TargetDifferenceSnapshot    float64    `json:"target_difference_snapshot"`
    MarginPercentSnapshot       *float64   `json:"margin_percent_snapshot,omitempty"`
    PriceDecisionID             *uuid.UUID `json:"price_decision_id,omitempty"`
    RequestedBy                 string     `json:"requested_by,omitempty"`
    RequestedAt                 time.Time  `json:"requested_at"`
    CancelledBy                 string     `json:"cancelled_by,omitempty"`
    CancelledAt                 *time.Time `json:"cancelled_at,omitempty"`
    CreatedAt                   time.Time  `json:"created_at"`
    UpdatedAt                   time.Time  `json:"updated_at"`
}
```

Warum nur ein DTO:

- Die erste Stufe liefert genau den erzeugten Request zurueck.
- Eine Listen- oder Detail-API fuer Freigabeanforderungen ist ein eigener
  Folge-Leaf.

## 3. Service-Signatur

Vorgeschlagene Service-Methode:

```go
func (s *Service) RequestApprovalForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
    requestedBy string,
    comment string,
) (*QuoteItemApprovalRequest, error)
```

Parameter:

- `quoteID`: Angebot
- `itemID`: Position
- `requestedBy`: technische User-ID aus Auth-Kontext, leer erlaubt als
  defensiver Fallback
- `comment`: optionale knappe Begruendung, gespeichert als `reason_text`

Kommentarregeln:

- `strings.TrimSpace(comment)`
- maximaler erster Grenzwert: 500 Zeichen
- bei leerem Kommentar bleibt `reason_text = ''`

## 4. Service-Ablauf

Die Methode soll transaktional arbeiten:

1. Transaktion starten.
2. Quote `FOR UPDATE` laden.
3. Historische Quotes blockieren.
4. Nur Status `draft` zulassen.
5. Position `FOR UPDATE` laden und Zugehoerigkeit zur Quote pruefen.
6. Zielmargenanker fachlich neu berechnen.
7. Status auf `below_cost` oder `below_target` begrenzen.
8. Reason-Code ableiten.
9. Aktive vorhandene Anforderung pruefen.
10. Snapshot-Werte in `quote_item_approval_requests` insertieren.
11. Erzeugten Request zuruecklesen.
12. Commit.

Wichtig:

- Der Pfad mutiert keinen Positionspreis.
- Der Pfad mutiert keine Quote-Summen.
- Der Pfad mutiert keinen Quote-Status.
- Die Zielbewertung muss serverseitig zum Zeitpunkt der Anforderung neu
  berechnet werden.

## 5. Zielmargenanker Innerhalb Der Transaktion

Die bestehende Methode `TargetMarginAnchorForQuoteItem(...)` ist read-only und
arbeitet nicht innerhalb einer uebergebenen Transaktion.

Fuer den Implementierungs-Leaf gibt es zwei vertretbare Optionen:

- eine interne Helper-Funktion fuer die Zielbewertung einfuehren, die mit
  `pgx.Tx` arbeiten kann
- oder die relevante Query- und Ableitungslogik eng in
  `RequestApprovalForQuoteItem(...)` wiederholen und spaeter extrahieren

Empfehlung:

- einen kleinen internen Helper bauen, zum Beispiel
  `targetMarginAnchorForQuoteItemTx(ctx, tx, quoteID, itemID)`
- dadurch bleiben Kostenbasis, Zielpreis und Reason-Ableitung konsistent
- bestehende read-only Methode kann spaeter optional auf denselben Helper
  umgestellt werden

## 6. Reason-Ableitung

Mapping aus Zielmargenanker:

```text
below_cost    -> negative_margin
below_target  -> below_target_margin
```

Nicht erlaubte Status:

```text
target_blocked_until_margin_available
on_target
above_target
```

Fachliche Fehler:

- `keine Preisentscheidung fuer diese Position vorhanden`
- `Position erreicht den Zielpreis; keine Freigabeanforderung erforderlich`

Bei `target_blocked_until_margin_available` soll kein Approval Request
entstehen.

## 7. Aktive Anforderung

Vor dem Insert soll der Service pruefen:

```text
SELECT id
FROM quote_item_approval_requests
WHERE quote_item_id = $1
  AND status = 'requested'
LIMIT 1
```

Wenn vorhanden:

- Domainfehler `Freigabeanforderung ist bereits aktiv`

Die Datenbank sichert denselben Fall zusaetzlich ueber den Partial Unique
Index ab. Der Servicefehler ist fuer die API-Meldung besser lesbar.

## 8. HTTP-Route

Vorgeschlagener Endpunkt:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests
```

Berechtigung:

```go
r.With(requirePermission("quotes.write")).Post(...)
```

Request-Body:

```json
{
  "comment": "Optionale Begruendung"
}
```

Leerer Body sollte erlaubt sein. Der Handler kann analog zu anderen
optionalen Bodies `io.EOF` tolerieren.

Response:

```text
201 Created
```

mit `QuoteItemApprovalRequest`.

## 9. User-ID Aus Auth-Kontext

Der Handler soll den angemeldeten Nutzer aus dem bestehenden Kontext lesen:

```go
user, ok := authUserFromContext(req.Context())
```

Wenn vorhanden:

- `requestedBy = user.ID`

Wenn nicht vorhanden:

- defensiv mit leerem String weitergeben oder `401/403` liefern

Empfehlung fuer den ersten Implementierungs-Leaf:

- fehlenden Auth-Kontext als `401 unauthorized` behandeln
- die Route liegt ohnehin hinter `requireAuth` und `requirePermission`

## 10. Fehlerfaelle

Fachliche Fehler ueber `writeDomainError(...)`:

- `Angebotsposition nicht gefunden`
- `Historische Angebotsversionen sind schreibgeschützt`
- `nur Entwürfe sind bearbeitbar`
- `keine Preisentscheidung fuer diese Position vorhanden`
- `Position erreicht den Zielpreis; keine Freigabeanforderung erforderlich`
- `Freigabeanforderung ist bereits aktiv`

Validierungsfehler im Handler:

- ungueltige Angebots-ID
- ungueltige Positions-ID
- ungueltige JSON-Eingabe
- Kommentar zu lang

## 11. Testziel Fuer Den Implementierungs-Leaf

Backend-Integrationstest in `server/internal/http/quotes_integration_test.go`:

- Erfolg: Position unter Ziel erzeugt HTTP 201 und Status `requested`
- Fehler: zweite aktive Anforderung liefert Fehler
- Fehler: fehlende Preisentscheidung liefert Fehler
- Fehler: Position auf Ziel oder ueber Ziel liefert Fehler

Minimal reicht fuer den ersten Service-/Route-Leaf:

- ein Erfolgspfad
- ein Doppelanforderungs-Fehler
- ein fehlende-Kostenbasis-Fehler

## 12. Nicht-Ziele

Nicht Teil des naechsten Implementierungs-Leaf:

- Client-Button
- Liste offener Freigaben
- Freigabe stornieren
- Freigeben
- Ablehnen
- Rechte `quotes.approval.request`
- Benachrichtigungen
- Angebotsstatus- oder PDF-Sperren
- KI-Bewertung

## 13. Naechster Schritt

Der naechste Leaf kann die Backend-Implementierung umsetzen:

- DTO `QuoteItemApprovalRequest`
- Service `RequestApprovalForQuoteItem(...)`
- Route `POST /api/v1/quotes/{id}/items/{itemID}/approval-requests`
- Integrationstest fuer Erfolg und zentrale Guard Rails

Client-UI bleibt danach weiterhin ein separater Leaf.
