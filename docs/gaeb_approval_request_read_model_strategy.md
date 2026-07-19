# GAEB-Freigabeanforderungen: Lesesicht-Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet die naechste kleine Lesesicht fuer aktive
positionsbezogene Freigabeanforderungen zu.

Der Scope bleibt bewusst eng:

- genau aktive `requested`-Anforderung je Quote-Position
- read-only Ausgabe in bestehenden Quote-Detail-Responses
- Wiederanzeige im bestehenden Quote-Editor nach erneutem Oeffnen
- keine Liste offener Freigaben
- kein Storno
- keine Freigabeentscheidung
- keine Rollenmatrix
- keine Benachrichtigungen

## 1. Ausgangspunkt

Der Freigabeanforderungs-MVP kann bereits:

- eine aktive Anforderung pro Quote-Position persistieren
- `requested_by`, `requested_at`, Reason und Snapshots speichern
- im Quote-Editor nach erfolgreicher Aktion lokal anzeigen

Die verbleibende Luecke:

- Beim erneuten Laden der Quote gehen aktive Anforderungen in der UI verloren,
  weil Quote-Items diese Daten noch nicht aus der API erhalten.

## 2. Entscheidung

Der kleinste sinnvolle Folgeschnitt ist:

- aktive Freigabeanforderung direkt optional an `QuoteItemInput` ausliefern

Nicht zuerst bauen:

- separater Listen-Endpunkt
- globale Freigabeuebersicht
- Freigabehistorie
- Storno
- Entscheidungspfad

Begruendung:

- Der bestehende Quote-Editor arbeitet bereits mit `Quote.Items`.
- Die minimale UI besitzt bereits `_QuoteItemApprovalRequestDraft`.
- Eine aktive Anforderung ist positionsbezogen und passt fachlich zum Item.
- Eine separate Liste loest das unmittelbare Reload-Problem schlechter.

## 3. Backend-DTO-Erweiterung

Vorgeschlagene Erweiterung:

```go
type QuoteItemInput struct {
    ...
    ActiveApprovalRequest *QuoteItemApprovalRequest `json:"active_approval_request,omitempty"`
}
```

Warum `QuoteItemInput`:

- Das bestehende `Quote`-DTO nutzt `[]QuoteItemInput`.
- Der Client baut `_QuoteItemDraft.fromJson(...)` aus genau diesen Item-Daten.
- Kein neues Quote-Response-Modell noetig.

Nicht im `toJson`-Pfad verwenden:

- `active_approval_request` ist read-only.
- Update/Create sollen dieses Feld ignorieren.

## 4. Backend-Leseverhalten

`Service.Get(...)` soll pro Quote-Item die aktive Anforderung mitlesen:

```text
status = 'requested'
```

Empfohlener Zugriff:

- nach dem Laden der Quote-Items pro Item eine kleine Helper-Funktion aufrufen
- oder per `LEFT JOIN LATERAL` direkt im Item-Query mitlesen

Empfehlung fuer den ersten Implementierungs-Leaf:

- `LEFT JOIN LATERAL`, weil maximal eine aktive Anforderung durch den Partial
  Unique Index existiert
- Scan-Felder nullable halten
- nur bei vorhandener ID `ActiveApprovalRequest` setzen

Skizze:

```sql
LEFT JOIN LATERAL (
    SELECT ...
    FROM quote_item_approval_requests qar
    WHERE qar.quote_item_id = qi.id
      AND qar.status = 'requested'
    ORDER BY qar.requested_at DESC
    LIMIT 1
) active_approval ON true
```

## 5. Auszugebende Felder

Das bestehende DTO `QuoteItemApprovalRequest` kann wiederverwendet werden.

Mindestens benoetigt der Client:

- `id`
- `status`
- `reason_code`
- `reason_text`
- `requested_at`

Sinnvoll mitzuliefern, weil bereits im Backend-DTO vorhanden:

- Preis- und Ziel-Snapshots
- `price_decision_id`
- `requested_by`
- `created_at`
- `updated_at`

Damit bleibt die Lesesicht konsistent mit der POST-Antwort.

## 6. Client-Erweiterung

`_QuoteItemDraft.fromJson(...)` soll lesen:

```dart
approvalRequestJson: json['active_approval_request']
```

oder direkt:

```dart
approvalRequest: _QuoteItemApprovalRequestDraft.fromJson(...)
```

Die bestehende UI kann unveraendert bleiben:

- Ist `approvalRequest != null`, zeigt sie `Freigabe angefordert`.
- Der Button `Freigabe anfordern` bleibt ausgeblendet.
- Es gibt weiterhin kein Storno und keine Entscheidung.

## 7. API-Schnitt

Kein neuer Endpunkt fuer diesen Leaf.

Genutzte bestehende Endpunkte:

- `GET /api/v1/quotes/{id}`
- alle bestehenden Endpunkte, die `Quote` zurueckgeben und intern
  `Service.Get(...)` nutzen

Wichtig:

- `POST .../approval-requests` liefert weiterhin die erzeugte Anforderung.
- Die neue Lesesicht sorgt nur dafuer, dass spaetere Quote-Reads denselben
  aktiven Zustand wieder anzeigen.

## 8. Testziel

Backend-Integrationstest:

- Freigabeanforderung erzeugen
- Quote erneut per `GET /api/v1/quotes/{id}` laden
- Erwartung:
  - betroffenes Item enthaelt `active_approval_request`
  - Status ist `requested`
  - Reason-Code ist passend
  - ID entspricht der erzeugten Anforderung

Client-Analyse:

- `dart analyze lib/api.dart lib/pages/quotes_page.dart`

Optional spaeter:

- Quote-Update loescht aktive Freigabeanforderung nicht unbeabsichtigt
- Revisionspfad kopiert aktive Anforderungen nicht automatisch

## 9. Nicht-Ziele

Nicht Teil dieser Lesesicht:

- `GET /approval-requests` als eigene Liste
- Storno-Endpunkt
- Genehmigen oder Ablehnen
- Rollen- oder Rechteaufteilung
- Freigabehistorie
- Timeline
- Benachrichtigungen
- Angebotsstatus- oder PDF-Sperre
- Kommentar-Dialog

## 10. Naechster Schritt

Der naechste Implementierungs-Leaf sollte:

- `QuoteItemInput` um `ActiveApprovalRequest` erweitern
- `Service.Get(...)` um aktive Request-Daten ergaenzen
- `_QuoteItemDraft.fromJson(...)` im Client anpassen
- bestehenden Quote-Integrationstest um Reload-Sicht erweitern

Danach kann entschieden werden, ob Storno oder Entscheidung der naechste
kleine Schreibpfad ist.
