# GAEB-Abweichungs-/Freigabeanker: Technisches Minimalzielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet das technische Minimalziel fuer den naechsten
read-only Abweichungs-/Freigabeanker zu.

Der Scope ist bewusst klein:

- genau eine Quote-Position
- genau ein read-only Hinweis
- Grundlage ist der vorhandene Margenanker
- keine neue Persistenz
- keine Freigabeaktion
- kein Rollenmodell
- keine Zielmarge
- keine Zuschlags-, Rabatt-, Bulk- oder Automatiklogik

## 1. Fachlicher Schnitt

Der Freigabeanker beantwortet nur eine Frage:

- Braucht diese Position wegen sichtbarer negativer Marge besondere
  Aufmerksamkeit fuer eine spaetere Freigabe?

Er ist kein Freigabeprozess. Er fordert keine Freigabe an, erteilt keine
Freigabe und blockiert keine Angebotsbearbeitung.

Der passendere technische Name ist deshalb:

- `ApprovalHint`

Damit bleibt klar, dass es sich um einen Hinweis handelt, nicht um Workflow.

## 2. Grundlage

Der neue Service soll auf dem bestehenden Margenanker aufsetzen:

```go
marginAnchor, err := s.MarginAnchorForQuoteItem(ctx, quoteID, itemID)
```

Die Vorteile:

- keine doppelte Kostenbasislogik
- gleiche Draft- und Versionsschutzregeln
- gleiche Berechnung fuer aktuelle Position, Kostenbasis und Marge
- ein einziger Ort fuer die Margenberechnung

Wenn der Margenanker wegen fehlender gespeicherter Preisentscheidung keinen
Kostenanker liefern kann, soll der Freigabehinweis dies read-only darstellen,
statt einen Live-Fallback zu raten.

## 3. Antwortmodell

Vorgeschlagenes Backend-DTO:

```go
type QuoteItemApprovalHint struct {
    ApprovalStatus     string     `json:"approval_status"`
    ApprovalReason     string     `json:"approval_reason"`
    MarginStatus       string     `json:"margin_status,omitempty"`
    CurrentUnitPrice   *float64   `json:"current_unit_price,omitempty"`
    CostBasisUnitPrice *float64   `json:"cost_basis_unit_price,omitempty"`
    Currency           string     `json:"currency,omitempty"`
    AbsoluteMargin     *float64   `json:"absolute_margin,omitempty"`
    MarginPercent      *float64   `json:"margin_percent,omitempty"`
    DecisionID         *uuid.UUID `json:"decision_id,omitempty"`
    DecisionType       string     `json:"decision_type,omitempty"`
    SourceLabel        string     `json:"source_label,omitempty"`
    DecisionCreatedAt  *time.Time `json:"decision_created_at,omitempty"`
}
```

Statuswerte:

- `approval_not_required`: Marge ist null oder positiv
- `approval_recommended`: Marge ist negativ
- `approval_blocked_until_margin_available`: keine gespeicherte Kostenbasis
  vorhanden

## 4. Statusableitung

Die Ableitung bleibt bewusst einfach:

- `negative_margin` wird zu `approval_recommended`
- `zero_margin` wird zu `approval_not_required`
- `positive_margin` wird zu `approval_not_required`
- fehlender Margenanker wird zu `approval_blocked_until_margin_available`

Vorgeschlagene Gruende:

- `Negative Marge sichtbar; spaetere Freigabe empfohlen`
- `Marge ist nicht negativ; keine Freigabeempfehlung`
- `Keine gespeicherte Preisentscheidung als Kostenbasis vorhanden`

Es gibt keine Schwellenwerte und keine Zielmarge. Die erste Version bewertet
nur die harte rote Linie: Preis unter gespeicherter Kostenbasis.

## 5. Service-Methode

Vorgeschlagene Service-Methode:

```go
func (s *Service) ApprovalHintForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
) (*QuoteItemApprovalHint, error)
```

Die Methode soll:

- `MarginAnchorForQuoteItem(...)` aufrufen
- fachliche Fehler fuer nicht gefundene oder nicht bearbeitbare Positionen
  unveraendert durchreichen
- fehlende Preisentscheidung als read-only Hinweis abbilden
- bei vorhandener Marge den Hinweisstatus ableiten
- relevante Margendaten in die Antwort kopieren

## 6. Fehlerfaelle

Weiterhin fachliche Fehler:

- Angebot oder Position nicht gefunden:
  `Angebotsposition nicht gefunden`
- historische Angebotsversion:
  `Historische Angebotsversionen sind schreibgeschützt`
- nicht bearbeitbarer Angebotsstatus:
  `nur Entwürfe sind bearbeitbar`

Kein harter Fehler fuer den Hinweis:

- `keine Preisentscheidung fuer diese Position vorhanden`

Dieser Fall soll im neuen DTO als
`approval_blocked_until_margin_available` erscheinen, damit der Client
positionsnah erklaeren kann, warum noch kein Freigabehinweis moeglich ist.

## 7. API-Schnitt

Vorgeschlagener Endpunkt:

```text
GET /api/v1/quotes/{id}/items/{itemID}/approval-hint
```

Er folgt dem bestehenden Muster:

- `requirePermission("quotes.write")`
- UUID-Parsing wie bei Margenanker, Preisbewertung, Transparenz und Historie
- `writeDomainError(...)` fuer echte fachliche Fehler
- `writeJSON(..., http.StatusOK, out)` fuer erfolgreichen Hinweis

Der Begriff `approval-hint` ist absichtlich gewaehlt, damit der Endpunkt nicht
wie ein Freigabe-Schreibpfad wirkt.

## 8. Client-Schnitt

Vorgeschlagene Client-API:

```dart
Future<Map<String, dynamic>> getQuoteItemApprovalHint(
  String quoteId,
  String itemId,
)
```

Pfad:

```text
/api/v1/quotes/{quoteId}/items/{itemId}/approval-hint
```

Vorgeschlagener Draft:

- `_QuoteItemApprovalHintDraft`
- Felder analog zum Backend-DTO
- nullable Preis- und Margenfelder fuer fehlende Kostenbasis

## 9. UI-Platzierung

Die Anzeige gehoert positionsnah in den bestehenden Quote-Editor.

Empfohlen:

- eigener read-only Block `Freigabehinweis`
- Laden per Button, wie Marge und Preisentscheidungen
- Anzeige von Status, Grund und relevanter Marge

Bewusst keine UI in diesem Schritt:

- Button `Freigabe anfordern`
- Button `Freigeben`
- Button `Ablehnen`
- Rollen- oder Benutzeranzeige
- Workflowstatus
- Kommentarfeld
- Bulk-Freigabe

## 10. Testziel

Minimaler Backend-Test:

- nach Primaerpreis-Uebernahme und nicht negativer Marge liefert der Hinweis
  `approval_not_required`
- ohne gespeicherte Preisentscheidung liefert der Hinweis
  `approval_blocked_until_margin_available`

Ein Test fuer negative Marge kann als eigener Haertungsschritt folgen, wenn
der erste read-only Pfad steht. Dafuer muesste nach dem Snapshot der
Positionspreis bewusst unter die Kostenbasis gesetzt werden.

## 11. Naechster Implementierungsschritt

Der naechste Leaf kann die Backend-Seite umsetzen:

- DTO `QuoteItemApprovalHint`
- Service-Methode `ApprovalHintForQuoteItem(...)`
- API-Route `GET /api/v1/quotes/{id}/items/{itemID}/approval-hint`
- fokussierter Integrationstest fuer nicht erforderliche Freigabe und fehlende
  Kostenbasis

Weiterhin nicht enthalten:

- Client-UI
- Persistenz
- echter Freigabeprozess
- Rollen
- Zielmarge
- Zuschlag
- Rabatt
- Bulk
- Automatik
