# GAEB-Freigabe: Strategie fuer expliziten Nacharbeitsabschluss

## Ziel

Dieses Dokument definiert den kleinsten belastbaren Zuschnitt fuer einen
expliziten Nacharbeitsabschluss, wenn eine zuvor abgelehnte Position nach der
Korrektur die Zielmarge erreicht.

Ausgangspunkt:

- Offene Nacharbeit wird aktuell ueber die neueste terminale
  Freigabeentscheidung `rejected` erkannt.
- Der aktuelle Zielmargenanker kann nach Nacharbeit `on_target` oder
  `above_target` liefern.
- In diesem Fall ist keine neue Margenabweichungsfreigabe noetig, aber die
  Prozesssperre bleibt aktiv, solange `latest_approval_decision.status ==
  rejected` ist.

## 1. Fachliche Entscheidung

Der Abschluss soll explizit und auditierbar erfolgen:

```text
Nacharbeit erledigt
```

Der Abschluss ist keine Freigabe einer Margenabweichung. Er dokumentiert, dass
die zuvor abgelehnte Position inzwischen kalkulatorisch wieder im Zielbereich
liegt.

Nicht erlaubt:

- automatische Erledigung beim Speichern
- dynamische Prozessfreigabe nur anhand aktueller Zielmarge
- erzwungene neue Freigabeanforderung fuer eine nicht mehr unterzielige
  Position

## 2. Statusmodell

Die bestehende Tabelle `quote_item_approval_requests` soll um einen
terminalen Status erweitert werden:

```text
rework_resolved
```

Statusraum danach:

- `requested`: aktive Freigabeanforderung
- `approved`: unterzielige Position wurde genehmigt
- `rejected`: Freigabeanforderung wurde abgelehnt, Nacharbeit offen
- `cancelled`: Anforderung wurde zurueckgenommen
- `rework_resolved`: zuvor abgelehnte Position wurde nach Korrektur als
  nicht mehr freigabepflichtig abgeschlossen

Begruendung fuer dieselbe Tabelle:

- Die bestehende Historie und Quote-Item-Readmodels lesen bereits aus
  `quote_item_approval_requests`.
- Der Abschluss ist fachlich Teil desselben Freigabe-/Nacharbeitszyklus.
- Die Prozesssperre kann weiterhin ueber den neuesten terminalen Zustand je
  Position arbeiten.
- Keine zweite Timeline muss synchronisiert werden.

## 3. Persistenz und Snapshots

Der erste MVP soll bestehende Entscheidungsfelder wiederverwenden:

- `status = 'rework_resolved'`
- `decided_by`
- `decided_at`
- `decision_comment`
- `approved_unit_price_snapshot`
- `approved_target_margin_percent_snapshot`
- `updated_at`

Interpretation:

- `approved_unit_price_snapshot` speichert beim Nacharbeitsabschluss den
  aktuellen Positionspreis.
- `approved_target_margin_percent_snapshot` speichert die zum Abschluss
  verwendete Zielmarge.
- `decision_comment` bleibt optional, aber weiter auf 500 Zeichen begrenzt.

Keine neuen Spalten im MVP:

- aktueller Zielpreis-Snapshot
- aktuelle Kostenbasis-Snapshot
- aktuelle Zielabweichung

Begruendung:

- Die Tabelle hat bereits Request-Snapshots aus der urspruenglichen Ablehnung.
- Fuer den ersten Abschluss reichen aktueller Preis und Zielmarge als kompakter
  Abschlussanker.
- Falls spaeter Vergleichsberichte gebraucht werden, kann eine eigene
  Rework-Snapshot-Erweiterung folgen.

## 4. Lifecycle-Constraints

Die bestehende Lifecycle-Constraint soll erweitert werden:

```text
status = 'rework_resolved'
AND cancelled_at IS NULL
AND decided_at IS NOT NULL
AND approved_unit_price_snapshot IS NOT NULL
AND approved_target_margin_percent_snapshot IS NOT NULL
```

Der Status ist terminal. Er darf nicht wieder in `requested`, `approved` oder
`rejected` mutiert werden.

Eine spaetere erneute Freigabeanforderung fuer dieselbe Position bleibt
moeglich, wenn die Position danach wieder unter Zielpreis/Kostenbasis faellt.
Dann entsteht eine neue Zeile mit `requested`.

## 5. Guard-Regeln im Service

Neuer Service-Schnitt:

```text
ResolveApprovalReworkForQuoteItem(ctx, quoteID, itemID uuid.UUID, resolvedBy, comment string) (*QuoteItemApprovalRequest, error)
```

Regeln:

- Quote muss existieren und per `FOR UPDATE` gelockt werden.
- Quote muss `draft` sein.
- Historische Angebotsversionen sind gesperrt.
- Position muss zur Quote gehoeren und per `FOR UPDATE` gelesen werden.
- Es darf keine aktive `requested`-Anforderung fuer die Position geben.
- Die neueste terminale Entscheidung der Position muss `rejected` sein.
- Es muss eine Preisentscheidung als Kostenbasis vorhanden sein.
- Der aktuelle Zielmargenanker muss `on_target` oder `above_target` ergeben.
- Kommentar maximal 500 Zeichen.

Fehlertexte:

```text
Keine offene Nacharbeit fuer diese Position vorhanden
Nacharbeit erreicht die Zielmarge noch nicht
keine Preisentscheidung fuer diese Position vorhanden
Kommentar darf nicht laenger als 500 Zeichen sein
```

## 6. Zielmargenbewertung

Der Service soll nicht blind UI-Daten vertrauen.

Stattdessen muss er die Zielmargenbewertung serverseitig in derselben
Transaktion aus aktuellen Daten ableiten:

- aktueller Positionspreis
- letzte Preisentscheidung als Kostenbasis
- aktuelle Zielmargen-Konfiguration
- Cent-Toleranz wie bei `TargetMarginAnchorForQuoteItem(...)`

Empfehlung:

- Die bestehende Zielmargenlogik mittelfristig in einen gemeinsamen Helper
  schneiden.
- Fuer den ersten Implementierungs-Leaf darf die kleine Berechnung lokal im
  neuen Service erfolgen, wenn dieselbe Toleranz und Statuslogik genutzt wird.

## 7. API-Zuschnitt

Neuer Endpunkt:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-rework/resolve
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

Berechtigung:

```text
quotes.approve
```

Begruendung:

- Der Abschluss hebt kommerzielle Prozesssperren auf.
- Das ist naeher an Freigabe-/Entscheidungsverantwortung als an normaler
  Angebotsbearbeitung.
- Admin hat `quotes.approve` bereits.

## 8. Readmodel- und Prozesssperren

`latest_approval_decision` soll `rework_resolved` als terminalen Zustand
beruecksichtigen.

Anpassung:

```text
status IN ('approved', 'rejected', 'rework_resolved')
```

Die bestehende Prozesssperre bleibt semantisch:

```text
Sperren nur, wenn neuester terminaler Zustand rejected ist.
```

Damit hebt `rework_resolved` die Sperre auf, ohne als `approved` angezeigt zu
werden.

Client-Darstellung:

- Statuslabel: `Nacharbeit erledigt`
- Rework-Hinweis nicht mehr anzeigen
- Historie zeigt Kommentar, Entscheider und Zeitpunkt

## 9. UI-Zuschnitt

Ort:

- Quote-Editor im bestehenden Zielmargenblock.

Button:

```text
Nacharbeit abschliessen
```

Sichtbar nur, wenn:

- `latestApprovalDecision.requiresRework == true`
- kein `approvalRequest` aktiv ist
- Zielmargenanker geladen ist
- `targetMarginAnchor.targetStatus` ist `on_target` oder `above_target`
- Nutzer hat `quotes.approve`

Interaktion:

- Kommentar-Dialog wie bei Genehmigen/Ablehnen wiederverwenden.
- Nach Erfolg `latestApprovalDecision` lokal auf `rework_resolved` aktualisieren
  oder Quote neu laden.

Fuer den ersten Client-Leaf ist ein Reload der Quote robuster, weil das
Readmodel den neuen terminalen Zustand serverseitig erzeugt.

## 10. Teststrategie

Backend-Integrationstests:

- abgelehnte Position unter Zielmarge kann nicht abgeschlossen werden
- abgelehnte Position nach Zielpreis-Uebernahme kann abgeschlossen werden
- Abschluss schreibt `rework_resolved` mit Entscheider, Zeit und Snapshots
- Quote-Reload liefert `latest_approval_decision.status = rework_resolved`
- Statuswechsel nach `sent` ist danach erlaubt
- Convert-to-invoice und convert-to-sales-order sind danach nicht mehr wegen
  Nacharbeit gesperrt
- Abschluss ohne `quotes.approve` liefert `403`
- Abschluss ohne letzte `rejected`-Entscheidung liefert `400`

Client-Verifikation:

- `dart format`
- `flutter analyze`
- optional Widget-Test erst, wenn bestehende Quote-Editor-Tests erweitert
  werden.

## 11. Umsetzung in Micro-Subtasks

Der Runtime-Ausbau soll in kleine Leaves geteilt werden:

- Subtask 3.1.47.6: Persistenz und Readmodel fuer `rework_resolved`
  vorbereiten
- Subtask 3.1.47.7: Backend-Service und API-Endpunkt fuer
  Nacharbeitsabschluss implementieren
- Subtask 3.1.47.8: Prozesssperren und Integrationstests fuer
  `rework_resolved` absichern
- Subtask 3.1.47.9: Client-API und Zielmargenblock-Button fuer
  Nacharbeitsabschluss einbauen
- Subtask 3.1.47.10: UI-/Workflow-Audit fuer Nacharbeitsabschluss

## Nicht-Ziele

Nicht Teil des naechsten Implementierungs-Leafs:

- separate Rework-Tabelle
- automatische Erledigung
- mehrstufige Freigabe
- globale Nacharbeits-Queue
- Benachrichtigungen
- KI-gestuetzte Nacharbeitsbewertung

## Verifikation

Keine Tests ausgefuehrt, da dieses Leaf nur eine Strategie dokumentiert.

