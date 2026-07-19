# GAEB-Freigabeanforderungen: Persistenzmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet das Persistenzmodell fuer positionsbezogene
Freigabeanforderungen zu.

Der Scope bleibt bewusst eng:

- genau eine Tabelle fuer Freigabeanforderungen an Quote-Positionen
- Status und fachlicher Grund als kleine Constraints
- Snapshot-Felder fuer Preis, Kostenbasis, Zielmarge und Zielabweichung
- hoechstens eine aktive Anforderung pro Quote-Position
- keine Service-Implementierung
- keine API-Route
- keine UI
- keine Genehmigungsentscheidung

## 1. Tabellenname

Vorgeschlagene Tabelle:

```text
quote_item_approval_requests
```

Begruendung:

- `quote_item` haelt den Scope explizit positionsbezogen.
- `approval_requests` macht klar, dass noch keine Entscheidung persistiert
  wird.
- Der Name laesst spaetere Tabellen wie
  `quote_item_approval_decisions` oder `quote_approval_requests` offen.

## 2. Minimaler Statusraum

Statuswerte fuer die erste Persistenzstufe:

```text
requested
cancelled
```

Nicht enthalten:

```text
approved
rejected
expired
escalated
```

Begruendung:

- `requested` ist der einzige aktive Workflow-Zustand.
- `cancelled` erhaelt Auditierbarkeit ohne physisches Loeschen.
- Entscheidungen und Eskalationen bleiben eigene Folge-Leaves.

## 3. Reason-Codes

Reason-Codes fuer die erste Stufe:

```text
negative_margin
below_target_margin
```

Semantik:

- `negative_margin`: aktueller Positionspreis liegt unter der gespeicherten
  Kostenbasis.
- `below_target_margin`: aktueller Positionspreis liegt auf oder ueber
  Kostenbasis, aber unter dem Zielpreis aus Zielmargenanker.

Nicht als Reason-Code speichern:

- fehlende Kostenbasis
- fehlende Materialzuordnung
- manuelle Sonderfreigabe ohne Preisrisiko
- KI-Empfehlung

Diese Faelle brauchen spaeter eigene Regeln und duerfen den ersten
Freigabeanker nicht verwischen.

## 4. Spaltenmodell

Vorgeschlagene Spalten:

```text
id UUID PRIMARY KEY
quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE
quote_item_id UUID NOT NULL REFERENCES quote_items(id) ON DELETE CASCADE
status text NOT NULL
reason_code text NOT NULL
reason_text text NOT NULL DEFAULT ''
current_unit_price_snapshot numeric(18,4) NOT NULL
cost_basis_unit_price_snapshot numeric(18,4) NOT NULL
target_unit_price_snapshot numeric(18,4) NOT NULL
target_margin_percent_snapshot numeric(9,4) NOT NULL
target_difference_snapshot numeric(18,4) NOT NULL
margin_percent_snapshot numeric(9,4)
price_decision_id UUID REFERENCES quote_item_price_decisions(id) ON DELETE SET NULL
requested_by text REFERENCES users(id) ON DELETE SET NULL
requested_at timestamptz NOT NULL DEFAULT now()
cancelled_by text REFERENCES users(id) ON DELETE SET NULL
cancelled_at timestamptz
created_at timestamptz NOT NULL DEFAULT now()
updated_at timestamptz NOT NULL DEFAULT now()
```

Warum Snapshots `NOT NULL` sind:

- Eine Freigabeanforderung darf nur entstehen, wenn Kostenbasis und
  Zielmargenanker berechenbar sind.
- Fehlende Kostenbasis wird vor dem Write-Pfad fachlich blockiert.
- Die gespeicherten Werte bilden den Pruefstand zum Zeitpunkt der Anforderung
  ab.

Warum `margin_percent_snapshot` nullable bleibt:

- Bei Kostenbasis `0` kann eine prozentuale Marge fachlich leer sein.
- Absolute Preis- und Zielabweichung bleiben trotzdem pruefbar.

## 5. Constraints

Pflicht-Constraints:

```text
status IN ('requested', 'cancelled')
reason_code IN ('negative_margin', 'below_target_margin')
current_unit_price_snapshot >= 0
cost_basis_unit_price_snapshot >= 0
target_unit_price_snapshot >= 0
target_margin_percent_snapshot >= 0
cancelled_at IS NULL wenn status = 'requested'
cancelled_at IS NOT NULL wenn status = 'cancelled'
```

Die letzte Regel sollte als Check formuliert werden:

```text
(status = 'requested' AND cancelled_at IS NULL)
OR
(status = 'cancelled' AND cancelled_at IS NOT NULL)
```

Nicht als DB-Constraint abbilden:

- `negative_margin` muss mathematisch wirklich negativ sein
- `below_target_margin` muss wirklich unter Ziel liegen

Diese Regeln haengen an der serverseitigen Ableitung aus dem aktuellen
Zielmargenanker und sollten im Service getestet werden. Die Datenbank prueft
den Statusraum, nicht die komplette Kalkulationslogik.

## 6. Aktive Anforderung

Es darf pro Quote-Position hoechstens eine aktive Anforderung geben.

Vorgeschlagener Partial Unique Index:

```text
CREATE UNIQUE INDEX IF NOT EXISTS ux_quote_item_approval_requests_active
    ON quote_item_approval_requests(quote_item_id)
    WHERE status = 'requested';
```

Begruendung:

- Mehrere offene Anforderungen fuer dieselbe Position waeren fachlich
  uneindeutig.
- Historische oder zurueckgenommene Anforderungen bleiben weiterhin sichtbar.

## 7. Lese- und Suchindizes

Vorgeschlagene Indizes:

```text
idx_quote_item_approval_requests_quote_created
    ON quote_item_approval_requests(quote_id, created_at DESC)

idx_quote_item_approval_requests_item_created
    ON quote_item_approval_requests(quote_item_id, created_at DESC)

idx_quote_item_approval_requests_status_requested
    ON quote_item_approval_requests(status, requested_at DESC)
```

Minimal fuer den ersten Implementierungs-Leaf reichen:

- aktiver Unique-Index
- Quote-Zeit-Index
- Position-Zeit-Index

Der Statusindex kann folgen, sobald es eine offene Freigabeuebersicht gibt.

## 8. Beziehung Zu Bestehenden Tabellen

`quote_id`:

- FK auf `quotes(id)` mit `ON DELETE CASCADE`
- erlaubt spaetere angebotsbezogene Historie und Listenabfragen

`quote_item_id`:

- FK auf `quote_items(id)` mit `ON DELETE CASCADE`
- zentraler Scope der Anforderung

`price_decision_id`:

- FK auf `quote_item_price_decisions(id)` mit `ON DELETE SET NULL`
- der Snapshot bleibt auch erhalten, falls die referenzierte Entscheidung aus
  technischen Gruenden nicht mehr existiert

`requested_by` und `cancelled_by`:

- FK auf `users(id)` mit `ON DELETE SET NULL`
- passend zur bestehenden Auth-Migration, die Nutzer-IDs als `text` fuehrt

## 9. Migration-Zielbild

Der naechste Implementierungs-Leaf sollte eine Migration mit folgender Nummer
anlegen:

```text
050_quote_item_approval_requests.sql
```

Sie sollte enthalten:

- `CREATE TABLE IF NOT EXISTS quote_item_approval_requests`
- defensive `DO $$`-Bloecke fuer Check-Constraints
- Partial Unique Index fuer aktive Anforderungen
- Leseindizes fuer Quote und Position

Die Migration sollte noch keine Permissions, Seeds oder Service-DTOs einfuehren.

## 10. Nicht-Ziele

Nicht Teil des Persistenzmodells:

- API `POST /approval-requests`
- Service `RequestApprovalForQuoteItem(...)`
- DTOs
- Client-Button
- Freigabeentscheidung
- Rollen- oder Rechteaufteilung
- Benachrichtigungen
- Angebotsstatus-Sperren
- PDF-Export-Sperren
- KI-Risikobewertung

## 11. Naechster Schritt

Der naechste Leaf kann die Migration umsetzen:

- `server/internal/migrate/migrations/050_quote_item_approval_requests.sql`
- Constraints fuer Status, Reason, Snapshot-Werte und Cancel-Konsistenz
- Partial Unique Index fuer genau eine aktive Anforderung pro Position
- minimale Indizes fuer spaetere Lesezugriffe

Erst danach sollte der Service-Write-Pfad gebaut werden.
