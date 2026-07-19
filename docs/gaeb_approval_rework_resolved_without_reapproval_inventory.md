# GAEB-Freigabe: Nacharbeit erreicht Zielmarge ohne erneute Freigabe inventarisieren

## Scope

Dieses Inventar bewertet den Sonderfall:

```text
Freigabe abgelehnt -> Position wird nachbearbeitet -> aktuelle Zielmarge wird
erreicht -> keine neue Freigabeanforderung ist noetig -> letzte terminale
Entscheidung bleibt trotzdem rejected
```

Geprueft wurden:

- Quote-Readmodel fuer `latest_approval_decision`
- Backend-Prozesssperre fuer Versand, Annahme, Rechnung und Auftrag
- Zielmargenanker als aktuelle Kalkulationssicht
- Client-Nacharbeitsanzeige und Re-Request-Gate

Keine Runtime-Aenderung ist Teil dieses Leafs.

## 1. Ist-Zustand

### 1.1 Letzte terminale Entscheidung

Quote-Items laden `latest_approval_decision` ueber die neueste terminale
Freigabeentscheidung:

- `approved`
- `rejected`

`requested` und `cancelled` zaehlen nicht als terminale Entscheidung.

Damit bleibt eine Ablehnung so lange das sichtbare letzte Ergebnis, bis eine
spaetere `approved`- oder `rejected`-Entscheidung fuer dieselbe Position
geschrieben wird.

### 1.2 Offene Nacharbeit

Die aktuelle Definition fuer offene Nacharbeit lautet faktisch:

```text
latest_approval_decision.status == rejected
```

Diese Definition wird konsistent verwendet fuer:

- Warnung im Angebotsdetail
- Marker im Quote-Editor
- Prozesssperren im Quote-Service
- Prozesssperren im Sales-Service

### 1.3 Zielmargenanker

`TargetMarginAnchorForQuoteItem(...)` bewertet die aktuelle Position gegen die
aktuelle Kostenbasis aus der letzten Preisentscheidung und gegen die
konfigurierte Zielmarge.

Moegliche relevante Ergebnisse:

- `on_target`
- `above_target`
- `below_target`
- `below_cost`
- `target_blocked_until_margin_available`

Diese Bewertung ist aktuell unabhaengig von der letzten terminalen
Freigabeentscheidung.

### 1.4 Client-Verhalten nach 3.1.47.3

Nach Positionsaenderungen invalidiert der Client alte Bewertungsanker. Der
Nutzer muss die Zielmarge neu laden.

Wenn die Position danach `on_target` oder `above_target` ist:

- `Freigabe anfordern` wird nicht angezeigt.
- Die letzte Ablehnung bleibt sichtbar.
- Die Backend-Prozesssperre bleibt aktiv, weil sie nicht die aktuelle
  Zielmarge, sondern `latest_approval_decision.status == rejected` auswertet.

## 2. Fachlicher Konflikt

Es gibt zwei fachlich plausible Wahrheiten:

1. Die Position ist kalkulatorisch wieder in Ordnung, weil sie Zielmarge oder
   mehr erreicht.
2. Die letzte formale Freigabeentscheidung lautet weiterhin `rejected`.

Der aktuelle Code priorisiert Wahrheit 2. Das ist auditierbar und sicher, kann
aber operativ blockieren, wenn keine neue Freigabe erforderlich ist.

## 3. Bewertete Loesungsoptionen

### Option A: Prozesssperre dynamisch gegen aktuelle Zielmarge pruefen

Regel:

- Eine letzte Ablehnung sperrt nur, wenn die aktuelle Position weiterhin unter
  Kostenbasis oder Zielpreis liegt.

Vorteile:

- Keine neue Benutzeraktion.
- Prozess wird automatisch frei, sobald Nacharbeit kalkulatorisch passt.

Nachteile:

- Die Bedeutung einer historischen Ablehnung wird nachtraeglich relativiert.
- Prozesssperren muessen Kostenbasis, Zielmarge und aktuelle Preise in
  mehreren Services nachrechnen.
- Bei fehlender Preisentscheidung waere das Verhalten unklar.
- Keine explizite Audit-Spur, wer die Nacharbeit als erledigt betrachtet hat.

Bewertung:

- Fuer den MVP zu implizit.

### Option B: Neue positive Freigabe trotz erreichter Zielmarge erzwingen

Regel:

- Auch wenn Zielmarge erreicht ist, muss eine neue Anforderung gestellt und
  genehmigt werden, um `latest_approval_decision` auf `approved` zu drehen.

Vorteile:

- Bestehendes Readmodel und Prozesssperren bleiben unveraendert.
- Klare Audit-Spur durch `approved`.

Nachteile:

- Widerspricht der bisherigen Regel, dass `Freigabe anfordern` nur bei
  Unterschreitung von Kostenbasis oder Zielmarge moeglich ist.
- Erzeugt Freigaben fuer wirtschaftlich unkritische Positionen.
- Backend `RequestApprovalForQuoteItem(...)` muesste einen neuen Reason-Code
  fuer "Nacharbeit erledigt" akzeptieren.

Bewertung:

- Technisch machbar, fachlich aber unsauber als Freigabe einer nicht mehr
  freigabepflichtigen Abweichung.

### Option C: Expliziter Nacharbeitsabschluss ohne Freigabe

Regel:

- Wenn eine zuletzt abgelehnte Position aktuell `on_target` oder
  `above_target` ist, darf ein berechtigter Nutzer die Nacharbeit explizit als
  erledigt abschliessen.
- Dieser Abschluss hebt die Prozesssperre auf, ohne eine neue
  Margenabweichungsfreigabe zu simulieren.

Moegliche Persistenz:

- neuer Status in `quote_item_approval_requests`, z. B.
  `rework_resolved`
- oder separate Tabelle fuer Nacharbeitsabschluesse

Minimaler Ansatz fuer bestehenden Tabellenkontext:

- neuen terminalen Status `rework_resolved`
- Metadaten analog Entscheidung: `decided_by`, `decided_at`,
  `decision_comment`
- Snapshot der aktuellen Zielmargenbewertung beim Abschluss

Vorteile:

- Explizite Benutzeraktion und Audit-Spur.
- Keine Freigabe einer nicht mehr freigabepflichtigen Abweichung.
- Prozesssperre kann weiter am neuesten terminalen Nacharbeits-/Freigabezustand
  haengen.

Nachteile:

- Erfordert Persistenz- und Readmodel-Erweiterung.
- Der Begriff ist fachlich neu und muss in UI und API sauber eingefuehrt
  werden.

Bewertung:

- Beste fachliche Richtung fuer den naechsten Ausbau.

### Option D: Letzte Ablehnung beim Speichern automatisch als erledigt markieren

Regel:

- Wenn `Update(...)` eine Position speichert und die Zielmarge erreicht ist,
  wird die letzte Ablehnung automatisch neutralisiert.

Vorteile:

- Kein neuer Nutzerklick.

Nachteile:

- Implizite Entscheidung ohne klare Verantwortlichkeit.
- `Update(...)` ersetzt Positionen aktuell und ist kein guter Ort fuer
  fachliche Entscheidungslogik.
- Hohe Gefahr, Historie und Auditierbarkeit zu schwaechen.

Bewertung:

- Nicht geeignet.

## 4. Empfehlung

Der naechste fachliche Zielpfad sollte Option C sein:

```text
Expliziter Nacharbeitsabschluss ohne erneute Freigabe
```

Kernregel:

- Erlaubt nur fuer Draft-Angebote.
- Erlaubt nur, wenn die neueste terminale Entscheidung der Position
  `rejected` ist.
- Erlaubt nur, wenn der aktuelle Zielmargenanker `on_target` oder
  `above_target` liefert.
- Schreibt einen auditierbaren terminalen Abschlusszustand.
- Hebt Prozesssperren auf, weil die neueste terminale
  Nacharbeits-/Freigabeentscheidung nicht mehr `rejected` ist.

## 5. Offene Designfragen

Vor Runtime-Umsetzung zu entscheiden:

- Statusname: `rework_resolved`, `resolved`, `not_required_after_rework`?
- Speicherung in bestehender Approval-Request-Tabelle oder separater
  Rework-Tabelle?
- Permission: reicht `quotes.write`, oder braucht es `quotes.approve`?
- Muss ein Kommentar verpflichtend sein?
- Welche Snapshots werden beim Abschluss gespeichert?
- Soll die Historie im Client diesen Abschluss neben `approved/rejected` als
  terminale Entscheidung anzeigen?

## 6. Kleinster naechster Leaf

Der naechste Leaf sollte eine Strategie fuer Option C zuschneiden:

```text
Subtask 3.1.47.5: Strategie fuer expliziten Nacharbeitsabschluss bei erreichter Zielmarge definieren
```

Der Strategie-Leaf soll entscheiden:

- Status-/Persistenzmodell
- Guard-Regeln
- API-Endpunkt
- Permission
- UI-Ort im Zielmargenblock
- Testfaelle

## Nicht-Ziele

Nicht Teil dieses Inventars:

- Migration
- Backend-Service
- API-Route
- Client-Button
- Tests

## Verifikation

Keine Tests ausgefuehrt, da dieses Leaf nur ein fachliches Inventar-Dokument
ergaenzt.

