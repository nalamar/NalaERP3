# GAEB-Zielmargen-/Zuschlagsanker: Technisches Minimalzielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet das technische Minimalziel fuer den naechsten
read-only Zielmargen-/Zuschlagsanker zu.

Der Scope ist bewusst klein:

- genau eine Quote-Position
- genau eine read-only Bewertung
- Grundlage ist der vorhandene Margenanker
- ein technischer Default-Zielwert fuer die erste Stufe
- keine neue Persistenz
- keine Preisveraenderung
- keine Freigabeaktion
- keine Rollen-, Bulk-, Rabatt- oder Automatiklogik

## 1. Fachlicher Schnitt

Der Zielmargenanker beantwortet nur eine Frage:

- Erreicht der aktuelle Positionspreis einen einfachen Zielaufschlag auf die
  gespeicherte Kostenbasis?

Er ist kein Kalkulationsmodul und kein Preisautomat. Er zeigt nur, welcher
Zielpreis aus Kostenbasis und Zielwert entstehen wuerde und wie weit der
aktuelle Positionspreis davon entfernt ist.

## 2. Grundlage

Der neue Service soll direkt auf dem bestehenden Margenanker aufsetzen:

```go
marginAnchor, err := s.MarginAnchorForQuoteItem(ctx, quoteID, itemID)
```

Die Vorteile:

- keine doppelte Kostenbasislogik
- gleiche Draft- und Versionsschutzregeln
- gleiche letzte gespeicherte Preisentscheidung als Kostenbasis
- gleiche Waerung und Entscheidungsmetadaten
- ein einziger Ort fuer aktuelle Marge und Kostenbasis

Wenn der Margenanker wegen fehlender gespeicherter Preisentscheidung keine
Kostenbasis liefern kann, soll der Zielmargenanker dies read-only darstellen,
statt einen Live-Fallback zu raten.

## 3. Default-Zielwert

Fuer die erste technische Stufe wird ein konstanter Default verwendet:

```text
target_margin_percent = 20.00
```

Dieser Wert ist bewusst kein Stammdatum und keine endgueltige fachliche
Kalkulationsregel. Er dient als erster technischer Zielwert, damit der Pfad
Ende-zu-Ende modelliert und getestet werden kann.

Wichtig:

- der Prozentwert bezieht sich wie der bestehende `margin_percent` auf die
  Kostenbasis
- Formel: `(price - cost_basis) / cost_basis * 100`
- der Zielpreis ist deshalb `cost_basis * (1 + target_margin_percent / 100)`
- bei Kostenbasis `0` bleibt der Zielpreis gleich `0`; Prozentabweichungen
  bleiben leer

Spaetere Ausbaustufen koennen diesen Default ersetzen durch:

- Mandanten- oder Settings-Wert
- Materialgruppenregel
- Kunden- oder Projektregel
- Angebotsregel

Diese Varianten sind nicht Teil dieses Blocks.

## 4. Antwortmodell

Vorgeschlagenes Backend-DTO:

```go
type QuoteItemTargetMarginAnchor struct {
    TargetStatus          string     `json:"target_status"`
    TargetReason          string     `json:"target_reason"`
    TargetMarginPercent   float64    `json:"target_margin_percent"`
    CurrentUnitPrice      *float64   `json:"current_unit_price,omitempty"`
    CostBasisUnitPrice    *float64   `json:"cost_basis_unit_price,omitempty"`
    TargetUnitPrice       *float64   `json:"target_unit_price,omitempty"`
    Currency              string     `json:"currency,omitempty"`
    AbsoluteMargin        *float64   `json:"absolute_margin,omitempty"`
    MarginPercent         *float64   `json:"margin_percent,omitempty"`
    TargetDifference      *float64   `json:"target_difference,omitempty"`
    TargetDifferencePct   *float64   `json:"target_difference_percent,omitempty"`
    MarginStatus          string     `json:"margin_status,omitempty"`
    DecisionID            *uuid.UUID `json:"decision_id,omitempty"`
    DecisionType          string     `json:"decision_type,omitempty"`
    SourceLabel           string     `json:"source_label,omitempty"`
    DecisionCreatedAt     *time.Time `json:"decision_created_at,omitempty"`
}
```

Feldbedeutung:

- `target_unit_price`: Zielpreis aus Kostenbasis und Zielwert
- `target_difference`: `current_unit_price - target_unit_price`
- `target_difference_percent`: `target_difference / target_unit_price * 100`,
  wenn der Zielpreis groesser `0` ist
- `margin_percent`: bestehender Prozentwert gegen Kostenbasis

## 5. Statuswerte

Vorgeschlagene Statuswerte:

- `below_cost`: aktueller Preis liegt unter Kostenbasis
- `below_target`: aktueller Preis liegt auf oder ueber Kostenbasis, aber unter
  Zielpreis
- `on_target`: aktueller Preis erreicht den Zielpreis innerhalb der
  Cent-Toleranz
- `above_target`: aktueller Preis liegt ueber Zielpreis
- `target_blocked_until_margin_available`: keine gespeicherte Kostenbasis
  vorhanden

Die Cent-Toleranz sollte wie beim Margenanker `0.005` betragen.

Vorgeschlagene Gruende:

- `Aktueller Preis liegt unter der Kostenbasis`
- `Aktueller Preis erreicht den Zielaufschlag noch nicht`
- `Aktueller Preis erreicht den Zielaufschlag`
- `Aktueller Preis liegt ueber dem Zielaufschlag`
- `Keine gespeicherte Preisentscheidung als Kostenbasis vorhanden`

## 6. Service-Methode

Vorgeschlagene Service-Methode:

```go
func (s *Service) TargetMarginAnchorForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
) (*QuoteItemTargetMarginAnchor, error)
```

Die Methode soll:

- `MarginAnchorForQuoteItem(...)` aufrufen
- fachliche Fehler fuer nicht gefundene oder nicht bearbeitbare Positionen
  unveraendert durchreichen
- fehlende Preisentscheidung als read-only Status
  `target_blocked_until_margin_available` abbilden
- `target_margin_percent` mit dem Default `20.00` setzen
- `target_unit_price` aus Kostenbasis und Zielwert berechnen
- `target_difference` und optional `target_difference_percent` berechnen
- Status und Grund ableiten
- relevante Margen- und Entscheidungsdaten aus dem Margenanker kopieren

## 7. Fehlerfaelle

Weiterhin fachliche Fehler:

- Angebot oder Position nicht gefunden:
  `Angebotsposition nicht gefunden`
- historische Angebotsversion:
  `Historische Angebotsversionen sind schreibgeschützt`
- nicht bearbeitbarer Angebotsstatus:
  `nur Entwürfe sind bearbeitbar`

Kein harter Fehler fuer den Zielmargenanker:

- `keine Preisentscheidung fuer diese Position vorhanden`

Dieser Fall soll als HTTP 200 mit
`target_blocked_until_margin_available` erscheinen, damit der Client
positionsnah erklaeren kann, warum noch keine Zielbewertung moeglich ist.

## 8. API-Schnitt

Vorgeschlagener Endpunkt:

```text
GET /api/v1/quotes/{id}/items/{itemID}/target-margin-anchor
```

Er folgt dem bestehenden Muster:

- `requirePermission("quotes.write")`
- UUID-Parsing wie bei Margenanker und Approval-Hint
- `writeDomainError(...)` fuer echte fachliche Fehler
- `writeJSON(..., http.StatusOK, out)` fuer erfolgreiche Bewertung

Der Begriff `target-margin-anchor` ist absichtlich read-only formuliert. Er
soll nicht wie eine Preisuebernahme oder Freigabeaktion wirken.

## 9. Client-Schnitt

Vorgeschlagene Client-API:

```dart
Future<Map<String, dynamic>> getQuoteItemTargetMarginAnchor(
  String quoteId,
  String itemId,
)
```

Pfad:

```text
/api/v1/quotes/{quoteId}/items/{itemId}/target-margin-anchor
```

Vorgeschlagener Draft:

- `_QuoteItemTargetMarginAnchorDraft`
- Felder analog zum Backend-DTO
- nullable Preis- und Differenzfelder fuer fehlende Kostenbasis

## 10. UI-Platzierung

Die Anzeige gehoert positionsnah in den bestehenden Quote-Editor.

Empfohlen:

- eigener read-only Block `Zielmarge`
- Laden per Button, wie `Marge` und `Freigabehinweis`
- Anzeige von Status, Grund, Zielwert, Zielpreis, aktuellem Preis,
  Kostenbasis und Zielabweichung
- Anzeige der Entscheidungsquelle aus dem Margenanker

Bewusst keine UI in diesem Schritt:

- Eingabefeld fuer Zielmarge
- Button zum Anwenden des Zielpreises
- Freigabe anfordern
- Freigeben oder Ablehnen
- Rollen- oder Benutzeranzeige
- Angebotsweite Zielmarge
- Bulk-Aktion

## 11. Testziel

Minimaler Backend-Test:

- nach gespeicherter Preisentscheidung und aktuellem Preis unter Zielpreis
  liefert der Zielmargenanker `below_target`
- nach bewusst gesetztem Preis auf oder ueber Zielpreis liefert er `on_target`
  oder `above_target`
- ohne gespeicherte Preisentscheidung liefert er
  `target_blocked_until_margin_available`

Minimaler Client-Check:

- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

## 12. Naechster Implementierungsschritt

Der naechste Leaf kann die Backend-Seite umsetzen:

- DTO `QuoteItemTargetMarginAnchor`
- Service-Methode `TargetMarginAnchorForQuoteItem(...)`
- API-Route
  `GET /api/v1/quotes/{id}/items/{itemID}/target-margin-anchor`
- fokussierter Integrationstest fuer Zielstatus und fehlende Kostenbasis

Weiterhin nicht enthalten:

- Client-UI
- Persistenz
- Zielwert-Konfiguration
- Preisveraenderung
- Freigabe
- Rollen
- Bulk
- Automatik
