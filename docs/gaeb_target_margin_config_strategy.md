# GAEB-Zielwert-Konfiguration: Technisches Minimalzielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet das technische Minimalziel fuer die persistente
Zielwertquelle des Zielmargenankers zu.

Der Scope ist bewusst klein:

- genau ein globaler Zielmargenwert
- genau eine persistente Quelle
- Nutzung durch den bestehenden read-only Zielmargenanker
- keine Preisveraenderung
- keine Zielpreis-Uebernahme
- keine Freigabeaktion
- keine Regelmatrix
- keine Rollen-, Bulk- oder Automatiklogik

## 1. Fachlicher Schnitt

Die Zielwert-Konfiguration beantwortet nur eine Frage:

- Welcher Zielmargenwert gilt als fachliche Standardgrundlage fuer
  Angebotspositionen?

Sie ist noch kein Kalkulationsregelwerk. Sie kennt keine Kunden-, Projekt-,
Materialgruppen- oder Angebotsausnahmen.

Der passendste technische Schnitt ist deshalb ein kleiner globaler
Kalkulations-Settingssatz.

## 2. Persistenzmodell

Vorgeschlagene Migration:

```text
049_quote_calculation_settings.sql
```

Vorgeschlagene Tabelle:

```sql
CREATE TABLE IF NOT EXISTS quote_calculation_settings (
    id TEXT PRIMARY KEY,
    target_margin_percent NUMERIC(6,2) NOT NULL DEFAULT 20.00,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO quote_calculation_settings (
    id,
    target_margin_percent
) VALUES (
    'default',
    20.00
)
ON CONFLICT (id) DO NOTHING;
```

Begruendung:

- passt zum bestehenden Settings-Muster `id='default'`
- vermeidet eine zu fruehe allgemeine Key-Value-Struktur
- bleibt fachlich klarer als ein unspezifischer Parameterkatalog
- erlaubt spaeter Erweiterungen wie Gemeinkosten- oder Rabatt-Defaults, ohne
  jetzt schon eine Regelmatrix einzufuehren

## 3. Backend-Modell

Vorgeschlagenes Settings-DTO:

```go
type QuoteCalculationSettings struct {
    ID                  string    `json:"id"`
    TargetMarginPercent float64   `json:"target_margin_percent"`
    UpdatedAt           time.Time `json:"updated_at"`
}
```

Vorgeschlagener Service:

```go
type QuoteCalculationSettingsService struct {
    pg *pgxpool.Pool
}

func NewQuoteCalculationSettingsService(pg *pgxpool.Pool) *QuoteCalculationSettingsService
func (s *QuoteCalculationSettingsService) Get(ctx context.Context) (*QuoteCalculationSettings, error)
func (s *QuoteCalculationSettingsService) Upsert(ctx context.Context, in QuoteCalculationSettings) error
```

Der Service gehoert in die bestehende Settings-Domaene:

```text
server/internal/settings/quote_calculation.go
```

## 4. Validierung

Die erste Validierung bleibt eng:

- leer oder nicht gesetzt im Request wird als `20.00` normalisiert
- Werte kleiner `0` sind ungueltig
- Werte groesser `1000` sind ungueltig
- Speicherung erfolgt mit zwei Nachkommastellen durch `NUMERIC(6,2)`

Fehlermeldung:

```text
Zielmarge ist ungueltig
```

Warum Obergrenze `1000`:

- verhindert offensichtliche Fehleingaben
- bleibt gross genug fuer Sonderfaelle
- fuehrt noch kein echtes Kalkulationsregelwerk ein

## 5. API-Schnitt

Vorgeschlagene Route unter bestehenden Settings:

```text
GET /api/v1/settings/quote-calculation
PUT /api/v1/settings/quote-calculation
```

Berechtigung:

```text
settings.manage
```

Verhalten:

- `GET` liefert den aktuellen Settingssatz
- `PUT` schreibt nur den globalen Zielmargenwert
- erfolgreiche `PUT`-Antwort bleibt `204 No Content`, passend zu bestehenden
  Settings-Routen

Bewusst keine Route in diesem Schritt:

- `POST /quotes/.../apply-target-price`
- `POST /quotes/.../request-approval`
- angebotsweite Zielmargenpruefung

## 6. Anschluss an den Zielmargenanker

Der bestehende Service:

```go
TargetMarginAnchorForQuoteItem(...)
```

soll den Zielwert aus der neuen Settings-Tabelle lesen.

Empfohlene interne Helferfunktion in der Quotes-Domaene:

```go
func (s *Service) quoteTargetMarginPercent(ctx context.Context) float64
```

Verhalten:

- liest `target_margin_percent` aus `quote_calculation_settings`
- wenn kein Datensatz existiert oder ein technischer Lesefehler auftritt, gilt
  weiter `20.00`
- der Zielmargenanker bleibt read-only
- die API-Antwort bleibt kompatibel, weil `target_margin_percent` bereits
  existiert

Warum kein harter Fehler bei fehlender Konfiguration:

- die Migration seedet zwar `default`, aber Alt- oder Testdaten koennen
  unvollstaendig sein
- der Zielmargenanker soll als Bewertung stabil bleiben
- der Default ist weiterhin sichtbar und kann spaeter ueberschrieben werden

## 7. Client-Schnitt

Minimaler Client-Folgepunkt:

- API-Methoden fuer `GET` und `PUT` der Quote-Calculation-Settings
- kleine Settings-Fläche fuer den globalen Zielmargenwert
- keine Veraenderung am Quote-Editor ausser indirekt geaenderten Werten nach
  erneutem Laden des Zielmargenankers

Moegliche API-Methoden:

```dart
Future<Map<String, dynamic>> getQuoteCalculationSettings()
Future<void> updateQuoteCalculationSettings(Map<String, dynamic> body)
```

Pfad:

```text
/api/v1/settings/quote-calculation
```

## 8. Testziel

Minimaler Backend-Test:

- initialer `GET` liefert `target_margin_percent = 20.00`
- `PUT` mit validem Wert schreibt den Wert
- nach `PUT` nutzt `target-margin-anchor` den neuen Wert
- `PUT` mit negativem Wert liefert `400`

Minimaler Client-Check:

- `flutter analyze lib/api.dart lib/pages/settings_page.dart`

## 9. Nicht-Ziele

Nicht Teil dieser Stufe:

- Zielwert-Historie
- Zielwert je Angebot
- Zielwert je Kunde, Projekt oder Materialgruppe
- Zielpreis-Uebernahme
- automatische Preisveraenderung
- Freigabe-Workflow
- Rollen oder Berechtigungsmatrix jenseits `settings.manage`
- Angebotsweite Aggregation
- KI-gestuetzte Zielwertfindung

## 10. Naechster Implementierungsschritt

Der naechste Leaf kann die Backend-Seite umsetzen:

- Migration `049_quote_calculation_settings.sql`
- Settings-Service `QuoteCalculationSettingsService`
- Routes `GET/PUT /api/v1/settings/quote-calculation`
- Zielmargenanker liest den konfigurierten Wert
- fokussierter Integrationstest fuer Settings und Zielmargenanker

Weiterhin nicht enthalten:

- Client-Settings-UI
- Zielpreis-Uebernahme
- Freigabe
- Regelmatrix
- Automatik
