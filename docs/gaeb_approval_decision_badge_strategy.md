# GAEB-Freigabeentscheidungen: Entscheidungsbadge-Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das kleinste technische Zielmodell fuer ein
positionsnahes Entscheidungsbadge im Quote-Editor zu.

Ausgangspunkt:

- Aktive Freigabeanforderungen sind am Quote-Item als
  `active_approval_request` sichtbar.
- Terminale Entscheidungen sind aktuell erst nach explizitem Laden der
  Freigabehistorie sichtbar.
- Die Historie enthaelt Entscheidungskommentar, genehmigte Snapshots und
  lesbare Nutzeranzeigenamen.
- Der naechste Schritt soll keine zentrale Queue und keine neue Mutation
  einfuehren.

## 1. Kleinster fachlicher Scope

Enthalten:

- optionales Readmodel fuer die letzte terminale Entscheidung je Quote-Item
- Badge-Anzeige direkt in der bestehenden Positionskarte
- Statusvarianten `approved` und `rejected`
- Entscheider-Anzeigename mit ID-Fallback
- Entscheidungszeitpunkt und optionaler Kommentar
- keine neue Permission
- keine Migration
- keine Queue

Nicht enthalten:

- zentrale Freigabe-Queue
- Filter, Sortierung oder Sammelansicht fuer Freigebende
- neue Entscheidungs- oder Storno-Mutation
- automatische Historienliste im Quote-Detail
- User-Snapshot in der Approval-Tabelle
- Badge fuer `cancelled`

## 2. Abgrenzung zu aktiven Anforderungen

`active_approval_request` bleibt die Sicht auf eine offene Anforderung:

```text
status = requested
```

Das neue Badge ist die Sicht auf die letzte abgeschlossene Entscheidung:

```text
status IN ('approved', 'rejected')
```

Beide Felder duerfen gleichzeitig technisch moeglich sein, falls nach einer
abgelehnten Entscheidung spaeter erneut eine Freigabe angefordert wird.

UI-Regel:

- aktive Anforderung bleibt handlungsfuehrend
- Badge zeigt nur die letzte terminale Entscheidung als Kontext
- Badge ersetzt nicht die Historie

## 3. Response-Modell

Neues optionales Feld an `QuoteItemInput`:

```go
LatestApprovalDecision *QuoteItemApprovalDecisionBadge `json:"latest_approval_decision,omitempty"`
```

Neues kleines DTO:

```go
type QuoteItemApprovalDecisionBadge struct {
    ID                          uuid.UUID  `json:"id"`
    Status                      string     `json:"status"`
    ReasonCode                  string     `json:"reason_code,omitempty"`
    ReasonText                  string     `json:"reason_text,omitempty"`
    DecidedBy                   string     `json:"decided_by,omitempty"`
    DecidedByName               string     `json:"decided_by_name,omitempty"`
    DecidedAt                   time.Time  `json:"decided_at"`
    DecisionComment             string     `json:"decision_comment,omitempty"`
    ApprovedUnitPriceSnapshot   *float64   `json:"approved_unit_price_snapshot,omitempty"`
    ApprovedTargetMarginPercent *float64   `json:"approved_target_margin_percent_snapshot,omitempty"`
}
```

Warum eigenes DTO statt Wiederverwendung von `QuoteItemApprovalRequest`:

- Badge braucht nur die kompakten Entscheidungsdaten.
- Das bestehende Approval-DTO ist fuer Historie und aktive Requests breiter.
- Ein eigenes DTO verhindert, dass die Quote-Detailantwort unnoetig gross wird.
- Die fachliche Bedeutung `latest_approval_decision` bleibt klarer als ein
  zweites optionales `QuoteItemApprovalRequest`.

## 4. Backend-Zuschnitt

### 4.1 Query-Ort

Der erste Implementierungs-Leaf erweitert nur den bestehenden Quote-Detailpfad:

```text
Service.Get(...)
```

Dort wird je Quote-Item ein weiteres `LEFT JOIN LATERAL` ergaenzt:

```sql
LEFT JOIN LATERAL (
  SELECT ...
  FROM quote_item_approval_requests qar
  LEFT JOIN users decided_user ON decided_user.id = qar.decided_by
  WHERE qar.quote_item_id = qi.id
    AND qar.status IN ('approved', 'rejected')
  ORDER BY qar.decided_at DESC NULLS LAST, qar.updated_at DESC
  LIMIT 1
) latest_approval_decision ON true
```

Begruendung:

- Der Quote-Editor erhaelt das Badge ohne separaten HTTP-Roundtrip.
- Das Readmodel bleibt positionsnah.
- Die Historienroute bleibt unveraendert.

### 4.2 Namensbildung

`decided_by_name` nutzt denselben Fallback wie die Historie:

```sql
CASE
  WHEN COALESCE(qar.decided_by, '') = '' THEN ''
  ELSE COALESCE(NULLIF(decided_user.display_name, ''), NULLIF(decided_user.email, ''), qar.decided_by)
END AS decided_by_name
```

### 4.3 Statusumfang

Im ersten Schritt nur:

- `approved`
- `rejected`

Nicht enthalten:

- `requested`, weil dafuer `active_approval_request` existiert
- `cancelled`, weil Storno kein fachlicher Entscheid ist

## 5. Client-Zuschnitt

### 5.1 Draft-Modell

Neuer Client-Draft:

```text
_QuoteItemApprovalDecisionBadgeDraft
```

Felder:

- `id`
- `status`
- `reasonCode`
- `reasonText`
- `decidedBy`
- `decidedByName`
- `decidedAt`
- `decisionComment`
- `approvedUnitPriceSnapshot`
- `approvedTargetMarginPercentSnapshot`

Parsing aus:

```text
latest_approval_decision
```

### 5.2 Anzeige

Das Badge sitzt in der bestehenden Positionskarte nahe beim
Freigabe-/Zielmargenbereich.

Empfohlene Texte:

- `Freigabe genehmigt`
- `Freigabe abgelehnt`

Zusatzzeile:

```text
Entschieden <timestamp> von <displayDecidedBy>
```

Kommentar wird kurz unter dem Badge angezeigt, wenn vorhanden.

Genehmigte Snapshots koennen kompakt angezeigt werden:

```text
Genehmigt 123.45 EUR  •  Zielmarge 20.00 %
```

Nicht im ersten UI-Leaf:

- farbige komplexe Timeline
- Historie automatisch aufklappen
- Sammelbadge in der Quote-Liste
- Filter nach Entscheidungsstatus

## 6. Testschnitt

Backend:

- bestehenden HTTP-Quote-Detailtest oder Entscheidungs-Integrationstest
  erweitern
- nach Approval Quote erneut laden
- `items[n].latest_approval_decision.status == "approved"` erwarten
- `decided_by` und `decided_by_name` pruefen
- nach Rejection `status == "rejected"` erwarten
- aktive Anforderung bleibt nach terminaler Entscheidung leer

Client:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Verifikation:

```text
go test ./internal/quotes ./internal/http
```

## 7. Risiken und Grenzen

### 7.1 Quote-Detailquery wird breiter

Ein weiteres `LEFT JOIN LATERAL` macht die bestehende Quote-Detailquery
umfangreicher.

Akzeptanz:

- Scope ist nur pro Quote-Item.
- Query bleibt read-only.
- Keine neue Route und kein separater Client-Ladezustand sind noetig.

### 7.2 Mehrere Entscheidungen je Position

Bei mehreren terminalen Entscheidungen zeigt das Badge nur die letzte.

Akzeptanz:

- Das Badge ist ein Schnellindikator.
- Die vollstaendige Historie bleibt die Quelle fuer Audit und Verlauf.

### 7.3 Erneute Freigabe nach Ablehnung

Eine neue aktive Anforderung kann neben einer alten terminalen Entscheidung
stehen.

Akzeptanz:

- Aktive Anforderung bleibt handlungsfuehrend.
- Badge zeigt bewusst Kontext zur letzten Entscheidung.

## 8. Naechster Implementierungs-Leaf

```text
Subtask 3.1.38.2: Backend-Readmodel fuer latest_approval_decision am Quote-Item implementieren
```

Umfang:

- neues Go-DTO `QuoteItemApprovalDecisionBadge`
- `QuoteItemInput` um `LatestApprovalDecision` erweitern
- `Service.Get(...)` um terminalen Decision-Lateral-Join erweitern
- Scan und Mapping im Quote-Item-Aufbau ergaenzen
- HTTP-Integrationstest fuer genehmigten und abgelehnten Badge erweitern

Nicht enthalten:

- Client-Anzeige
- Queue
- neue Permission
- Migration
