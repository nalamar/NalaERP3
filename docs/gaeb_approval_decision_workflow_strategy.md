# GAEB-Freigabeanforderungen: Entscheidungsworkflow-Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet den naechsten belastbaren Workflow fuer
Genehmigen und Ablehnen von positionsbezogenen Freigabeanforderungen zu.

Ausgangspunkt:

- Anforderungen werden in `quote_item_approval_requests` gespeichert.
- Der aktuelle Statusraum ist bewusst auf `requested` und `cancelled`
  begrenzt.
- Aktive Anforderungen blockieren Quote-Updates, damit die referenzierte
  Position und ihre Snapshots nicht per delete/insert verloren gehen.
- Storno hebt diese Sperre explizit auf, mutiert aber keine Preise, Summen
  oder Angebotsstatus.
- Preisentscheidungen liegen in `quote_item_price_decisions`; die
  Anforderung speichert den verwendeten `price_decision_id` als Anker.

Der naechste Workflow darf diese Linie nicht aufweichen: eine
Freigabeentscheidung muss fachlich explizit, auditierbar und eng an die
aktuelle Position gebunden sein.

## 1. Statusmodell

Der kleinste fachlich tragfaehige Statusraum fuer
`quote_item_approval_requests` wird:

- `requested`: offene Anforderung, blockiert Quote-Update
- `approved`: Freigabeentscheidung erteilt
- `rejected`: Freigabeentscheidung abgelehnt
- `cancelled`: Anforderung durch Anfordernden oder Bearbeiter
  zurueckgenommen

Die Entscheidung wird direkt an der Anforderung gespeichert, nicht in einer
zweiten Entscheidungstabelle.

Begruendung:

- Es gibt je Position nur eine aktive Anforderung.
- Die Tabelle enthaelt bereits alle relevanten fachlichen Snapshots.
- Genehmigen/Ablehnen sind terminale Zustaende derselben fachlichen
  Anforderung.
- Eine separate Tabelle wuerde fuer den MVP mehr Join- und
  Lebenszykluskomplexitaet erzeugen, ohne bereits mehrere Entscheider,
  Eskalationen oder Delegationen abzubilden.

Nicht-Ziel fuer diesen Schnitt:

- mehrstufige Freigaben
- Delegation
- Wiedervorlage
- separate Freigabe-Queue
- komplette Audit-Timeline im Client

## 2. Entscheidungsmetadaten

Die bestehende Tabelle braucht fuer `approved` und `rejected` folgende
Felder:

- `decided_by text REFERENCES users(id) ON DELETE SET NULL`
- `decided_at timestamptz`
- `decision_comment text NOT NULL DEFAULT ''`
- `approved_unit_price_snapshot numeric(18,4)`
- `approved_target_margin_percent_snapshot numeric(9,4)`

Regeln:

- `requested`: `decided_at IS NULL`, `cancelled_at IS NULL`
- `cancelled`: `cancelled_at IS NOT NULL`, `decided_at IS NULL`
- `approved`: `decided_at IS NOT NULL`, `cancelled_at IS NULL`
- `rejected`: `decided_at IS NOT NULL`, `cancelled_at IS NULL`
- `decision_comment` ist optional, aber auf 500 Zeichen begrenzt.

`approved_unit_price_snapshot` ist nur bei `approved` verpflichtend. Fuer
den ersten Implementierungsschritt entspricht er dem aktuell gespeicherten
`current_unit_price_snapshot`.

Wichtig: Eine Genehmigung ist damit eine dokumentierte Erlaubnis, die
angefragte unterzielige Position weiterzufuehren. Sie ist noch keine
automatische Preisneuberechnung.

## 3. Berechtigungszuschnitt

Der aktuelle MVP nutzt `quotes.write` fuer Request und Storno. Fuer
Entscheidungen soll eine engere Berechtigung eingefuehrt werden:

```text
quotes.approve
```

Regeln:

- `quotes.write`: Freigabe anfordern und eigene Bearbeitungsflows
- `quotes.approve`: offene Freigabeanforderungen genehmigen oder ablehnen
- Admin-Rollen koennen `quotes.approve` spaeter ueber die vorhandene
  Rollen-/Permissions-Seedlogik erhalten

Warum keine Wiederverwendung von `quotes.write`:

- Schreiben am Angebot und Genehmigen einer Margenabweichung sind fachlich
  verschiedene Verantwortungen.
- Der spaetere ERP-Ausbau braucht eine klare Trennung zwischen Vertrieb und
  kaufmaennischer Freigabe.

## 4. Erlaubte Mutationen

### 4.1 Genehmigen

Genehmigen mutiert nur die Anforderung:

- `status = 'approved'`
- `decided_by`
- `decided_at`
- `decision_comment`
- `approved_unit_price_snapshot`
- `approved_target_margin_percent_snapshot`
- `updated_at`

Quote, Quote-Item, Preise, Summen und Quote-Status werden nicht automatisch
veraendert.

Begruendung:

- Die angefragte Position enthaelt den aktuellen Preis bereits.
- Automatische Preis- oder Statusmutation wuerde Entscheidung und
  Kalkulation vermischen.
- Der bestehende Save-Guard kann nach `approved` entfallen, weil nur
  `requested` blockiert.

### 4.2 Ablehnen

Ablehnen mutiert nur die Anforderung:

- `status = 'rejected'`
- `decided_by`
- `decided_at`
- `decision_comment`
- `updated_at`

Quote, Quote-Item, Preise, Summen und Quote-Status werden nicht automatisch
veraendert.

Folge:

- Die Position bleibt zunaechst unveraendert sichtbar.
- Der Nutzer muss danach den Preis korrigieren, Zielpreis anwenden oder die
  Position anderweitig bearbeiten.
- Weil `rejected` nicht mehr aktiv ist, blockiert der bestehende Save-Guard
  nicht mehr. Die fachliche UI sollte die Ablehnung sichtbar machen, damit
  die Korrektur nicht uebersehen wird.

## 5. Bindung an Quote-Item und Preisentscheidung

Der Entscheidungsworkflow bleibt positionsgebunden:

- Service lockt Quote und aktive Anforderung in einer Transaktion.
- Entscheidung ist nur erlaubt, wenn die Quote `draft` ist.
- Historische Angebotsversionen bleiben schreibgeschuetzt.
- Entscheidung ist nur fuer `status = 'requested'` erlaubt.
- `quote_item_id` muss weiterhin zur Quote gehoeren.
- `price_decision_id`, Kostenbasis, Zielmarge und Margensnapshot bleiben
  unveraendert erhalten.

Keine Revalidierung gegen neue Preisentscheidungen im ersten Schritt:

- Aktive Anforderungen blockieren Quote-Update.
- Dadurch kann die referenzierte Position nicht normal ueberschrieben
  werden.
- Spaetere Sonderpfade muessen diese Garantie erneut pruefen.

## 6. API-Zuschnitt

Zwei explizite Endpunkte:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject
```

Payload:

```json
{
  "comment": "optional, max. 500 Zeichen"
}
```

Antwort:

- `200 OK`
- serialisierte `QuoteItemApprovalRequest`

Fehler:

- `400 Bad Request`, wenn keine aktive Anforderung existiert
- `400 Bad Request`, wenn Quote nicht `draft` ist
- `400 Bad Request`, wenn historische Revision
- `403 Forbidden`, wenn `quotes.approve` fehlt
- `400 Bad Request`, wenn Kommentar zu lang

Die Endpunkte sind nicht idempotent. Ein zweites Genehmigen oder Ablehnen
liefert `400`, weil keine aktive `requested`-Anforderung mehr existiert.

## 7. Lesemodell und UI-Folge

Der aktuelle Quote-Reload zeigt nur `active_approval_request`. Nach
Genehmigung oder Ablehnung verschwindet die Anforderung aus dieser aktiven
Sicht.

Das ist fuer den ersten Backend-Leaf akzeptabel, aber der naechste UI-Pfad
braucht mindestens eine kompakte Historie:

- letzte Freigabeanforderung je Position
- Status `approved`, `rejected`, `cancelled`
- Entscheider/Stornierer
- Zeitpunkte
- Kommentar

Ohne diese Lesesicht waere eine genehmigte oder abgelehnte Position nach
Reload fachlich zu schwer nachvollziehbar.

## 8. Naechster Implementierungs-Leaf

Der naechste kleine Leaf soll nur die Persistenz vorbereiten:

`Subtask 3.1.34.2: Persistenzmodell fuer Freigabeentscheidungen erweitern`

Umfang:

- Migration fuer `approved/rejected`
- Entscheidungsmetadaten in `quote_item_approval_requests`
- Go-Struct `QuoteItemApprovalRequest` um Entscheidungsfelder erweitern
- Scan-/Readmodel fuer bestehende aktive Anforderungen kompatibel halten

Nicht enthalten:

- Approve-/Reject-Service
- HTTP-Routen
- Client-Buttons
- Historien-UI

