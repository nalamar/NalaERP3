# GAEB-Freigabe: Strategie fuer erneute Anforderung nach Nacharbeit

## Ziel

Dieses Dokument schneidet den kleinsten robusten Folgepfad fuer erneute
Freigabeanforderungen nach abgelehnter Positionsfreigabe zu.

Ausgangspunkt:

- Eine abgelehnte Entscheidung bleibt als `latest_approval_decision.status =
  rejected` sichtbar.
- Draft-Bearbeitung ist nach Ablehnung erlaubt.
- `RequestApprovalForQuoteItem(...)` kann nach `rejected` erneut eine aktive
  Anforderung erzeugen.
- Der Client zeigt `Freigabe anfordern` nur ueber einen geladenen
  Zielmargenanker.

Der naechste Implementierungsschritt soll keine neue Persistenz und keinen
neuen API-Endpunkt einfuehren.

## 1. Fachliche Leitentscheidung

Erneute Freigabe nach Nacharbeit bleibt eine explizite Benutzeraktion.

Nicht erlaubt im MVP:

- automatische Re-Request-Erzeugung beim Speichern
- automatische Genehmigung, wenn der Preis nach Nacharbeit die Zielmarge
  erreicht
- automatisches Entfernen der letzten Ablehnung

Begruendung:

- Eine Ablehnung ist eine kaufmaennische Entscheidung und muss historisch
  sichtbar bleiben.
- Nacharbeit kann Preis, Material, Menge oder Kostenbasis betreffen; das System
  kann ohne erneute Pruefung nicht wissen, ob eine neue Freigabe fachlich
  gewuenscht ist.
- Eine neue `requested`-Anforderung ist noch keine Freigabe und darf die
  Prozesssperre nicht aufheben.

## 2. UI-Regel nach Positionsnacharbeit

Wenn ein Nutzer eine Position mit `latestApprovalDecision.requiresRework`
bearbeitet, muessen alle lokal gecachten Bewertungsanker fuer diese Position
als veraltet gelten.

Zu invalidieren:

- `targetMarginAnchor`
- `targetMarginAnchorPerformed`
- `approvalHint`
- `approvalHintPerformed`
- `marginAnchor`
- `marginAnchorPerformed`
- `priceEvaluation`
- `priceEvaluationPerformed`

Nicht zu loeschen:

- `latestApprovalDecision`
- `approvalRequests`
- `approvalRequestsPerformed`
- `approvalRequest`, falls eine aktive Anforderung existiert

Begruendung:

- Die letzte Ablehnung bleibt fachlicher Kontext.
- Bewertungsanker enthalten Preis- und Kosten-Snapshots und koennen nach
  Eingabefeldaenderungen fachlich falsch sein.
- Eine aktive Anforderung blockiert ohnehin das Speichern und muss bewusst
  storniert oder entschieden werden.

## 3. Ausloeser fuer Invalidierung

Im Quote-Editor sollen Positionsfelder eine kleine zentrale Dirty-Markierung
nutzen.

Ausloeser:

- Beschreibung geaendert
- Menge geaendert
- Einheit geaendert
- Einzelpreis geaendert
- Steuercode geaendert
- Material-ID geaendert
- Preisstatus geaendert
- Position entfernt

Besonders wichtig:

- Einzelpreis und Material-ID invalidieren Zielmargenanker zwingend.
- Menge und Beschreibung invalidieren fuer den MVP ebenfalls, weil sie Teil der
  fachlichen Nacharbeit sein koennen und der Nutzer nicht mit alten
  Freigabehinweisen weiterarbeiten soll.

Nicht noetig im ersten Implementierungsschritt:

- diff gegen Ursprungswerte
- serverseitige Dirty-Persistenz
- Anzeige eines separaten Dirty-Badges

## 4. Wiederanbieten der Freigabeanforderung

Nach Positionsaenderung soll der Zielmargenblock wieder in einen neutralen
Zustand fallen:

- vorhandene alte Zielmargenanzeige verschwindet
- Nutzer kann `Zielmarge anzeigen` erneut laden
- danach entscheidet `targetMarginAnchor.canRequestApproval`
  ueber `Freigabe anfordern`

Der Button `Freigabe anfordern` bleibt an die bestehende Regel gebunden:

```text
targetMarginAnchor != null
AND targetMarginAnchor.canRequestApproval
AND approvalRequest == null
```

Damit wird keine neue UI-Entscheidungslogik eingefuehrt. Der bestehende
Backend-Endpunkt bleibt die Quelle der Wahrheit.

## 5. Fall: Nacharbeit erreicht Zielmarge

Wenn die nachbearbeitete Position die Zielmarge erreicht, soll keine neue
Freigabe angefordert werden.

Erwartetes Verhalten:

- `Zielmarge anzeigen` liefert `on_target` oder `above_target`.
- `Freigabe anfordern` wird nicht angezeigt.
- Die letzte Ablehnung bleibt sichtbar, bis ein separater fachlicher
  Abschluss definiert wird.

Wichtig:

Dieser MVP loest noch nicht den fachlichen Konflikt, dass die kommerzielle
Prozesssperre weiterhin an der letzten terminalen Entscheidung `rejected`
haengt. Fuer einen vollstaendigen Abschluss braucht es spaeter entweder:

- eine neue positive Freigabeentscheidung trotz erreichter Zielmarge, oder
- einen expliziten Status "Nacharbeit erledigt / Freigabe nicht mehr noetig".

Der naechste Runtime-Leaf soll diesen Sonderfall nicht automatisch loesen,
sondern nur verhindern, dass veraltete UI-Anker eine erneute Anfrage anbieten.

## 6. Fall: Nacharbeit bleibt unter Zielmarge

Wenn die Position nach Korrektur weiterhin unter Kostenbasis oder Zielpreis
liegt:

- Nutzer laedt Zielmarge neu.
- `Freigabe anfordern` wird wieder sichtbar.
- `RequestApprovalForQuoteItem(...)` erzeugt eine neue `requested`-Anforderung
  mit neuen Snapshots.
- Prozesssperren bleiben bis zu einer spaeteren `approved`-Entscheidung aktiv.

## 7. Implementierungsschnitt fuer den naechsten Leaf

Der naechste kleine Implementierungs-Leaf soll clientseitig bleiben:

1. `_QuoteItemDraft` bekommt eine Methode zum Invalidieren lokaler
   Kalkulations-/Freigabeanker.
2. `_QuoteItemRow` bekommt einen Callback wie `onCommercialFieldsChanged`.
3. Die Positions-Textfelder und der Preisstatus-Dropdown rufen diesen Callback
   bei Aenderungen auf.
4. Der Callback invalidiert die lokalen Anker fuer genau diese Position.
5. Keine API-, Backend- oder Migrationsaenderung.

Optional, wenn klein genug:

- Im Nacharbeitskasten den Text praezisieren:
  `Nach Bearbeitung Zielmarge neu anzeigen und bei Bedarf Freigabe erneut
  anfordern.`

## 8. Nicht-Ziele

Nicht Teil des naechsten Implementierungs-Leafs:

- neuer Backend-Status fuer Nacharbeit
- neue Tabelle fuer Rework-Zyklen
- automatische Re-Request-Erzeugung
- automatische Entsperrung bei erreichter Zielmarge
- Entscheidervergleich alter/neuer Snapshots
- neue Tests gegen Prozesssperren

## 9. Risiko und Folgeentscheidung

Das groesste verbleibende fachliche Risiko ist der Fall:

```text
Ablehnung -> Preis wird auf Zielpreis korrigiert -> keine neue Freigabe noetig,
aber latest_approval_decision bleibt rejected
```

Dieser Fall braucht einen eigenen Folge-Leaf, weil er eine fachliche
Statusentscheidung ist. Fuer den naechsten kleinen UI-Leaf ist es ausreichend,
veraltete Zielmargen- und Freigabedaten nach Eingabeaenderungen nicht weiter
anzuzeigen.

## Verifikation

Keine Tests ausgefuehrt, da dieses Leaf nur die Strategie fuer die kommende
Client-Aenderung dokumentiert.

