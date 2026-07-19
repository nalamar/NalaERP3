# GAEB-Freigabe-Schreibpfad: Kleines Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet das kleinste Zielmodell fuer einen spaeteren echten
Freigabe-Schreibpfad an einer Quote-Position zu.

Der Scope bleibt bewusst klein:

- genau eine Draft-Quote-Position
- Freigabebedarf nur bei negativer Marge oder Zielabweichung
- Grundlage sind vorhandener Margenanker und Zielmargenanker
- keine angebotsweite Freigabe
- keine Rollenmatrix
- keine Eskalation
- keine automatische Preisveraenderung
- keine KI-Entscheidung

## 1. Ausgangspunkt

Der aktuelle GAEB-/Angebotspfad kann positionsnah bereits anzeigen:

- gespeicherte Preisentscheidung als Kostenbasis
- aktuelle Marge gegen diese Kostenbasis
- read-only Approval-Hint fuer negative Marge
- global konfigurierte Zielmarge
- read-only Zielmargenanker mit Zielpreis und Zielabweichung
- explizite Zielpreis-Uebernahme als separate Preisentscheidung

Damit ist die fachliche Entscheidungsgrundlage vorhanden. Was noch fehlt, ist
ein persistenter Zustand, wenn ein Nutzer eine risikobehaftete Position
bewusst zur kaufmaennischen Freigabe markieren will.

## 2. Fachlicher Schnitt

Der erste Freigabe-Schreibpfad beantwortet nur eine Frage:

- Soll fuer genau diese Angebotsposition eine Freigabe angefordert werden,
  weil sie unter Kostenbasis oder unter Zielpreis liegt?

Er entscheidet die Freigabe noch nicht. Er ist nur der Startpunkt fuer einen
spaeteren Workflow.

Nicht enthalten:

- Freigabe erteilen
- Freigabe ablehnen
- Angebotsstatus blockieren
- automatische Preis- oder Zielwertkorrektur
- Regelmatrix je Kunde, Projekt, Rolle oder Materialgruppe

## 3. Freigabeausloeser

Die erste Stufe kennt nur zwei fachliche Ausloeser:

- `negative_margin`: aktueller Preis liegt unter der gespeicherten Kostenbasis
- `below_target_margin`: aktueller Preis liegt auf oder ueber Kostenbasis,
  aber unter dem Zielpreis aus Zielmargenanker

Fehlende Kostenbasis loest keine Freigabeanforderung aus. In diesem Fall muss
zuerst eine Preisentscheidung oder Kostenbasis geschaffen werden.

Ausdruecklich kein Ausloeser:

- positive Marge oberhalb Zielpreis
- fehlende Zielmargen-Konfiguration, solange der definierte Default-Fallback
  greift
- reine Materialsuche ohne angewendete Preisentscheidung

## 4. Minimaler Statusraum

Vorgeschlagener Statusraum fuer die erste Persistenzstufe:

- `requested`: Freigabe wurde angefordert
- `cancelled`: Freigabeanforderung wurde zurueckgenommen

Noch nicht Teil dieser Stufe:

- `approved`
- `rejected`
- `expired`
- `escalated`

Begruendung:

- `requested` schafft den fehlenden persistenten Workflow-Anker.
- `cancelled` erlaubt Korrekturen ohne Loeschung.
- Entscheiden, Eskalieren und Rollenpruefung sind eigene Folge-Leaves.

## 5. Datenmodell-Zielbild

Vorgeschlagene neue Tabelle fuer einen spaeteren Implementierungs-Leaf:

```text
quote_item_approval_requests
```

Minimalfelder:

```text
id
quote_id
quote_item_id
status
reason_code
reason_text
current_unit_price_snapshot
cost_basis_unit_price_snapshot
target_unit_price_snapshot
target_margin_percent_snapshot
target_difference_snapshot
margin_percent_snapshot
price_decision_id
requested_by
requested_at
cancelled_by
cancelled_at
created_at
updated_at
```

Wichtige Eigenschaften:

- Snapshots bleiben erhalten, auch wenn spaeter Preis oder Zielmarge geaendert
  werden.
- `price_decision_id` verweist auf die Kostenbasis, aus der der Bedarf
  abgeleitet wurde.
- Es sollte pro Quote-Position hoechstens eine aktive Anforderung im Status
  `requested` geben.

## 6. Service-Schnitt

Vorgeschlagene Service-Methode fuer den ersten Write-Leaf:

```go
func (s *Service) RequestApprovalForQuoteItem(
    ctx context.Context,
    quoteID uuid.UUID,
    itemID uuid.UUID,
    requestedBy *uuid.UUID,
    comment string,
) (*QuoteItemApprovalRequest, error)
```

Die Methode soll:

- Quote und Position transaktional pruefen
- historische Quotes blockieren
- nur Draft-Quotes zulassen
- Zielmargenanker serverseitig neu berechnen
- daraus den Freigabeausloeser ableiten
- bei fehlender Kostenbasis fachlich abbrechen
- bei `on_target` oder `above_target` fachlich abbrechen
- bei bereits aktiver Anforderung idempotent die aktive Anforderung
  zurueckgeben oder einen klaren Domainfehler liefern
- Snapshots persistieren

Empfehlung fuer die erste Umsetzung:

- bereits aktive Anforderung als Domainfehler behandeln
- spaeter bewusst entscheiden, ob idempotente Rueckgabe besser zur UI passt

## 7. API-Schnitt

Vorgeschlagener Endpunkt:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests
```

Request-Body:

```json
{
  "comment": "Knappe fachliche Begruendung"
}
```

Antwort:

```text
201 Created
```

mit der erzeugten Freigabeanforderung.

Berechtigung:

- zunaechst `quotes.write`
- spaeter in `quotes.approval.request` oder vergleichbar aufteilen

## 8. Guard Rails

Pflichtregeln fuer den spaeteren Implementierungs-Leaf:

- Quote existiert
- Quote ist aktuelle Version
- Quote ist im Status `draft`
- Position gehoert zur Quote
- gespeicherte Preisentscheidung als Kostenbasis existiert
- Zielmargenanker ist berechenbar
- Status ist `below_cost` oder `below_target`
- keine aktive Freigabeanforderung existiert bereits fuer diese Position

Fachliche Fehlertexte sollten kurz und nutzerverstaendlich bleiben:

- `keine Preisentscheidung fuer diese Position vorhanden`
- `Position erreicht den Zielpreis; keine Freigabeanforderung erforderlich`
- `Freigabeanforderung ist bereits aktiv`
- `Historische Angebotsversionen sind schreibgeschützt`
- `nur Entwürfe sind bearbeitbar`

## 9. UI-Zielbild

Nicht Teil dieses Dokumentations-Leaves, aber Anschlussbild:

- Button `Freigabe anfordern` im Zielmargenblock
- sichtbar bei `below_cost` oder `below_target`
- optionales Kommentarfeld in kleinem Dialog
- nach Erfolg Anzeige des aktiven Status `requested`
- kein Button fuer `Freigeben` oder `Ablehnen` in dieser Stufe

Der bestehende Button `Zielpreis uebernehmen` bleibt getrennt. Eine
Freigabeanforderung darf den Preis nicht veraendern.

## 10. Testziel Fuer Die Erste Implementierung

Backend-Minimum:

- Erfolg: Position unter Ziel erzeugt `requested`
- Fehler: Position ohne gespeicherte Preisentscheidung
- Fehler: Position auf oder ueber Zielpreis
- Fehler: zweite aktive Anforderung fuer dieselbe Position

Noch nicht testen:

- Genehmigen oder Ablehnen
- Rollenmatrix
- Benachrichtigungen
- angebotsweite Aggregation
- Client-Dialog

## 11. Nicht-Ziele

Nicht Teil dieses kleinen Zielmodells:

- Freigabeentscheidung
- Workflow-Historie ueber mehrere Entscheider
- Eskalation
- Wiedervorlage
- E-Mail oder Benachrichtigung
- Angebotsstatuswechsel
- Sperren des PDF-Exports
- KI-Risikobewertung
- Mandanten-, Kunden- oder Projektregeln

## 12. Naechster Schritt

Der naechste Leaf sollte dokumentierend oder implementierend das
Persistenzmodell zuschneiden:

- Migration fuer `quote_item_approval_requests`
- Status- und Reason-Constraints
- Unique-Regel fuer eine aktive Anforderung pro Position
- Snapshot-Felder fuer Kostenbasis, Zielpreis und Zielabweichung

Erst danach sollte der Service- und API-Write-Pfad gebaut werden.
