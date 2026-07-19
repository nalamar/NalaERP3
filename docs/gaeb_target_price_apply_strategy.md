# GAEB-Zielpreis-Uebernahme: Technisches Minimalzielmodell

## Ziel Dieses Dokuments

Dieses Dokument schneidet das technische Minimalziel fuer eine kontrollierte
Zielpreis-Uebernahme pro Quote-Position zu.

Der Scope bleibt bewusst klein:

- genau eine Draft-Quote-Position
- genau eine explizite Nutzeraktion
- Zielpreis aus gespeicherter Kostenbasis und globaler Zielmarge
- keine Automatik
- kein Bulk
- kein Freigabe-Workflow
- keine Regelmatrix
- keine KI-Preisfindung

## 1. Ausgangspunkt

Vorhanden sind bereits:

- gespeicherte Preisentscheidung als Kostenbasis
- read-only Margenanker
- read-only Zielmargenanker
- persistente globale Zielmarge in `quote_calculation_settings`
- explizite Primaerpreis-Uebernahme als bestehendes Write-Muster

Der Nutzer kann den Zielpreis sehen, aber noch nicht bewusst auf die Position
uebernehmen.

## 2. Fachlicher Schnitt

Die Zielpreis-Uebernahme beantwortet nur eine Frage:

- Soll der aktuell serverseitig berechnete Zielpreis fuer genau diese
  Angebotsposition als Verkaufspreis gesetzt werden?

Sie ist keine Angebotskalkulation und kein Freigabeprozess.

Sie berechnet keinen neuen Zielwert, sondern nutzt:

- die letzte gespeicherte Preisentscheidung als Kostenbasis
- den globalen Zielmargenwert aus `quote_calculation_settings`
- dieselbe Formel wie der Zielmargenanker

## 3. Service-Schnitt

Vorgeschlagene Service-Methode:

```go
func (s *Service) ApplyTargetUnitPriceForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
) (*Quote, error)
```

Die Methode soll sich am bestehenden Muster von
`ApplyPrimaryPriceSourceForQuoteItem(...)` orientieren:

- Transaktion starten
- Quote `FOR UPDATE` sperren
- Position `FOR UPDATE` sperren
- Guard Rails pruefen
- Zielpreis serverseitig berechnen
- Position aktualisieren
- Quote-Summen neu berechnen
- optionale Preisentscheidung persistieren
- aktualisierte Quote ueber `Get(...)` zurueckgeben

## 4. API-Schnitt

Vorgeschlagener Endpunkt:

```text
POST /api/v1/quotes/{id}/items/{itemID}/apply-target-price
```

Eigenschaften:

- kein Request-Body
- `requirePermission("quotes.write")`
- Antwort: aktualisierte Quote
- Fehler ueber bestehende Domain-Error-Behandlung

Der Name `apply-target-price` ist bewusst enger als
`calculate-price` oder `optimize-price`. Er beschreibt eine explizite
Uebernahme eines bereits bewerteten Zielpreises.

## 5. Guard Rails

Die Guard Rails muessen mindestens gelten:

- Quote existiert
- Quote ist aktuelle Version, also nicht superseded
- Quote ist im Status `draft`
- Position existiert und gehoert zur Quote
- gespeicherte Preisentscheidung als Kostenbasis existiert
- Kostenbasis stammt aus dem bestehenden Margenanker oder aus derselben
  Persistenzgrundlage
- Zielmargenwert ist lesbar oder der definierte Fallback `20.00` wird bewusst
  verwendet

Fachliche Fehler:

- `Angebotsposition nicht gefunden`
- `Historische Angebotsversionen sind schreibgeschützt`
- `nur Entwürfe sind bearbeitbar`
- `keine Preisentscheidung fuer diese Position vorhanden`

Kein stiller Live-Fallback auf Materialpreise:

- Wenn keine gespeicherte Preisentscheidung existiert, darf kein Zielpreis
  angewendet werden.

## 6. Zielpreis-Berechnung

Formel:

```text
target_unit_price = cost_basis_unit_price * (1 + target_margin_percent / 100)
```

Quelle fuer `cost_basis_unit_price`:

- letzte gespeicherte Preisentscheidung, wie im Margenanker

Quelle fuer `target_margin_percent`:

- `quote_calculation_settings.target_margin_percent`
- Fallback `20.00` nur nach derselben Logik wie
  `quoteTargetMarginPercent(ctx)`

Rundung:

- Zielpreis serverseitig auf zwei Nachkommastellen runden
- Summen aus gerundetem `unit_price` berechnen

Begruendung:

- `unit_price`, `net_amount`, `tax_amount` und Quote-Summen bleiben fuer
  Dokumente und UI konsistent
- der sichtbare Zielmargenanker kann bei spaeterem Reload den angewendeten
  Preis nachvollziehen

## 7. Datenmutation

Die Mutation bleibt eng:

```text
quote_items.unit_price = rounded_target_unit_price
quote_items.net_amount = qty * rounded_target_unit_price
quote_items.tax_amount = net_amount * tax_rate
quote_items.price_mapping_status = 'manual'
quotes.net_amount = SUM(quote_items.net_amount)
quotes.tax_amount = SUM(quote_items.tax_amount)
quotes.gross_amount = net_amount + tax_amount
```

Keine Mutation:

- kein Quote-Statuswechsel
- kein Freigabestatus
- keine Zielmargen-Settings
- keine Materialzuordnung
- keine Preisquellen-Prioritaet

## 8. Preisentscheidungs-Persistenz

Empfehlung fuer die erste Implementierung:

- eine Preisentscheidung in `quote_item_price_decisions` persistieren

Vorgeschlagener `decision_type`:

```text
target_price_applied
```

Feldbelegung:

- `source_unit_price`: Kostenbasis
- `applied_unit_price`: gerundeter Zielpreis
- `source_label`: `Zielpreis aus Zielmarge`
- `currency`: Quote- oder Entscheidungswaehrung, Fallback `EUR`
- `source_reference`: optional `target_margin_percent=<wert>`

Begruendung:

- Margenanker und Zielmargenanker koennen weiter auf der letzten
  gespeicherten Entscheidung aufsetzen
- die Uebernahme bleibt historisch nachvollziehbar
- es entsteht noch keine Freigabehistorie

Wichtig:

- Die Entscheidung ist Preisentscheidungs-Historie, kein Freigabebeleg.

## 9. Testziel

Minimaler Backend-Test fuer den Implementierungs-Leaf:

- Setup mit gespeicherter Primaerpreis-Entscheidung als Kostenbasis
- Zielmarge z. B. `25.00` setzen
- `POST /api/v1/quotes/{id}/items/{itemID}/apply-target-price`
- Erwartung:
  - HTTP 200
  - Position `unit_price` entspricht gerundetem Zielpreis
  - Quote-Summen sind aktualisiert
  - neue Preisentscheidung `target_price_applied` existiert
  - nachfolgender Zielmargenanker ist `on_target` oder innerhalb Toleranz
- Fehlerfall:
  - ohne gespeicherte Preisentscheidung liefert der Endpunkt 400

Optional spaeter:

- nicht-draft Quote blockiert
- historische Quote blockiert
- negative oder ungueltige Zielmarge faellt auf Default nur nach definierter
  Settings-Fallback-Logik zurueck

## 10. Client-Zielbild

Nicht Teil des naechsten Backend-Implementierungs-Leaf, aber Anschlussbild:

- Button im vorhandenen Zielmargenblock: `Zielpreis uebernehmen`
- sichtbar nur bei geladenem Zielmargenanker mit `target_unit_price`
- deaktiviert bei `target_blocked_until_margin_available`
- nach Erfolg aktualisierte Quote in den Editor uebernehmen
- keine automatische Ausfuehrung nach Laden des Zielmargenankers

## 11. Nicht-Ziele

Nicht Teil dieser Stufe:

- Bulk-Zielpreis-Uebernahme
- Angebotsweite Zielmargenpruefung
- Freigabe anfordern
- Freigabe erteilen oder ablehnen
- Rollenmatrix
- Kommentare
- Zielpreis-Historie ausserhalb der bestehenden Preisentscheidungshistorie
- Zielwerte je Kunde, Projekt oder Materialgruppe
- automatische Preisaktualisierung
- KI-gestuetzte Zielpreisfindung
- Rabatt-, Nachlass- oder Gemeinkostenlogik

## 12. Offene Kanten Vor Implementierung

Vor oder waehrend der Implementierung bewusst entscheiden:

- Soll `target_margin_percent = 0.00` explizit erlaubt werden?
- Soll der Settings-Service fuer fehlenden Default-Datensatz selbstheilend
  sein?
- Soll die Zielpreis-Uebernahme den verwendeten Zielmargenwert im
  `source_reference` speichern?
- Soll der Zielpreis immer auf zwei Nachkommastellen gerundet werden oder die
  spaetere Waehrungslogik vorbereitet werden?

Empfehlung fuer den ersten Implementierungs-Leaf:

- zwei Nachkommastellen
- `source_reference` mit Zielmargenwert
- bestehender Default-Fallback `20.00`
- keine Aenderung an `0.00`-Semantik innerhalb desselben Leaves

## 13. Naechster Implementierungsschritt

Der naechste Leaf kann die Backend-Seite umsetzen:

- Service `ApplyTargetUnitPriceForQuoteItem(...)`
- Route `POST /api/v1/quotes/{id}/items/{itemID}/apply-target-price`
- persistierte Preisentscheidung `target_price_applied`
- fokussierter Integrationstest fuer Erfolg und fehlende Kostenbasis

Weiterhin nicht enthalten:

- Client-Button
- Freigabe-Workflow
- Bulk
- Automatik
