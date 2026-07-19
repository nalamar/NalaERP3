# GAEB-Margenanker: Technisches Minimalzielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet das technische Minimalziel fuer den naechsten
read-only Margen-/Zuschlagsanker zu.

Der Scope ist bewusst klein:

- genau eine Quote-Position
- genau ein read-only Vergleich
- Kostenbasis aus der letzten gespeicherten Preisentscheidung
- keine neue Persistenz
- keine Schreibaktion
- keine Rabatt-, Freigabe-, Bulk- oder Automatiklogik

## 1. Fachlicher Schnitt

Der Margenanker beantwortet nur eine Frage:

- Wie steht der aktuelle Positionspreis zur letzten gespeicherten
  Preisentscheidung dieser Position?

Er ist kein Kalkulationsmodul. Er veraendert keinen Preis und erzeugt keine
neue Entscheidung.

## 2. Kostenbasis

Die Kostenbasis wird aus der letzten gespeicherten Preisentscheidung gelesen:

- Tabelle: `quote_item_price_decisions`
- Filter: `quote_id` und `quote_item_id`
- Sortierung: `created_at desc`
- Begrenzung: `LIMIT 1`
- technische Basis: `applied_unit_price`

`applied_unit_price` ist fuer diesen ersten Anker belastbarer als ein
aktueller Live-Preis aus Materialhistorie, weil er die konkret uebernommene
Preisentscheidung der Quote-Position abbildet.

## 3. Antwortmodell

Vorgeschlagenes Backend-DTO:

```go
type QuoteItemMarginAnchor struct {
    CurrentUnitPrice       float64    `json:"current_unit_price"`
    CostBasisUnitPrice     float64    `json:"cost_basis_unit_price"`
    Currency               string     `json:"currency,omitempty"`
    AbsoluteMargin         float64    `json:"absolute_margin"`
    MarginPercent          *float64   `json:"margin_percent"`
    MarginStatus           string     `json:"margin_status"`
    DecisionID             uuid.UUID  `json:"decision_id"`
    DecisionType           string     `json:"decision_type"`
    SourceLabel            string     `json:"source_label"`
    SourceReference        string     `json:"source_reference,omitempty"`
    SourceDate             *time.Time `json:"source_date,omitempty"`
    DecisionCreatedAt      time.Time  `json:"decision_created_at"`
}
```

Statuswerte:

- `negative_margin`: aktueller Positionspreis liegt unter Kostenbasis
- `zero_margin`: aktueller Positionspreis entspricht Kostenbasis
- `positive_margin`: aktueller Positionspreis liegt ueber Kostenbasis

Wenn die Kostenbasis `0` ist, bleibt `margin_percent` leer. Der absolute
Vergleich bleibt trotzdem lesbar.

## 4. Service-Methode

Vorgeschlagene Service-Methode:

```go
func (s *Service) MarginAnchorForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
) (*QuoteItemMarginAnchor, error)
```

Die Methode soll:

- Quote und Position validieren
- dieselben Draft-/Versionsschutzregeln wie die Preisentscheidungs-Historie
  verwenden
- den aktuellen `unit_price` aus `quote_items` lesen
- die letzte Preisentscheidung aus `quote_item_price_decisions` lesen
- `absolute_margin` als `current_unit_price - cost_basis_unit_price`
  berechnen
- `margin_percent` als `absolute_margin / cost_basis_unit_price * 100`
  berechnen, wenn die Kostenbasis groesser `0` ist
- den einfachen Status ableiten

## 5. Fehlerfaelle

Fachlich erwartete Fehler:

- Angebot oder Position nicht gefunden:
  `Angebotsposition nicht gefunden`
- historische Angebotsversion:
  `Historische Angebotsversionen sind schreibgeschützt`
- nicht bearbeitbarer Angebotsstatus:
  `nur Entwürfe sind bearbeitbar`
- keine gespeicherte Preisentscheidung:
  `keine Preisentscheidung fuer diese Position vorhanden`

Der letzte Fehler ist bewusst fachlich, nicht technisch: Ohne gespeicherte
Kostenbasis darf der Margenanker kein Live-Fallback raten.

## 6. API-Schnitt

Vorgeschlagener Endpunkt:

```text
GET /api/v1/quotes/{id}/items/{itemID}/margin-anchor
```

Er folgt dem bestehenden Muster:

- `requirePermission("quotes.write")`
- UUID-Parsing wie bei Preisbewertung, Transparenz und Historie
- `writeDomainError(...)` fuer fachliche Fehler
- `writeJSON(..., http.StatusOK, out)` fuer erfolgreiche Antwort

Der Endpunkt ist read-only, verwendet aber konsistent dieselbe Berechtigung wie
die bestehenden positionsnahen Quote-Editor-Werkzeuge.

## 7. Client-Schnitt

Vorgeschlagene Client-API:

```dart
Future<Map<String, dynamic>> getQuoteItemMarginAnchor(
  String quoteId,
  String itemId,
)
```

Pfad:

```text
/api/v1/quotes/{quoteId}/items/{itemId}/margin-anchor
```

Vorgeschlagener Draft:

- `_QuoteItemMarginAnchorDraft`
- Felder analog zum Backend-DTO
- Prozentwert nullable

## 8. UI-Platzierung

Die Anzeige gehoert positionsnah in den bestehenden Quote-Editor.

Empfohlen:

- eigener read-only Block `Marge`
- Laden per Button, wie Preisbewertung, Transparenz und Historie
- Anzeige von aktuellem Positionspreis, Kostenbasis, absoluter Marge,
  prozentualem Aufschlag, Status und Entscheidungsquelle

Bewusst keine UI in diesem Schritt:

- Eingabe fuer Zielmarge
- Button zum Anwenden eines Zuschlags
- Freigabeaktion
- Bulk-Anzeige auf Angebotsebene
- automatische Aktualisierung aller Positionen

## 9. Testziel

Minimaler Backend-Test:

- nach Primaerpreis-Uebernahme liefert der Margenanker Kostenbasis,
  aktuellen Positionspreis, absolute Marge, Prozentwert und Status
- ohne gespeicherte Preisentscheidung liefert der Service einen fachlichen
  Fehler

Minimaler Client-Check:

- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

## 10. Naechster Implementierungsschritt

Der naechste Leaf kann die Backend-Seite umsetzen:

- DTO `QuoteItemMarginAnchor`
- Service-Methode `MarginAnchorForQuoteItem(...)`
- API-Route `GET /api/v1/quotes/{id}/items/{itemID}/margin-anchor`
- fokussierter Integrationstest

Weiterhin nicht enthalten:

- Client-UI
- Persistenz
- Preisveraenderung
- Freigabe
- Bulk
- Automatik
