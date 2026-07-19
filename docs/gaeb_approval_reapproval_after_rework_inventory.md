# GAEB-Freigabe: Erneute Anforderung nach Nacharbeit inventarisieren

## Scope

Dieses Inventar bewertet den bestehenden Pfad, nachdem eine
positionsbezogene Freigabe abgelehnt wurde und die Position nachgearbeitet
werden soll.

Geprueft wurde nur der fachliche und technische Ist-Zustand fuer eine spaetere
erneute Freigabeanforderung:

- aktive Freigabeanforderungen
- terminale Freigabeentscheidungen
- Zielmargenanker und Freigabefaehigkeit
- Quote-Update-Guard
- Prozesssperren fuer Versand, Annahme und Folgebelege
- Client-Editor-Aktionen fuer Nacharbeit

Keine Runtime-Aenderung ist Teil dieses Leafs.

## Ist-Zustand

### 1. Freigabeanforderung

`RequestApprovalForQuoteItem(...)` erstellt eine neue
`quote_item_approval_requests`-Zeile nur fuer Draft-Angebote und nur, wenn die
aktuelle Position unter Kostenbasis oder unter Zielpreis liegt.

Die Anforderung speichert Snapshots fuer:

- aktuellen Positionspreis
- Kostenbasis aus letzter Preisentscheidung
- Zielpreis
- Zielmarge
- Zielabweichung
- Margenstatus
- Preisentscheidungsanker

Es gibt pro Position maximal eine aktive Anforderung mit `status =
'requested'`. Eine abgelehnte Anforderung ist terminal und nicht mehr aktiv.

### 2. Entscheidung und Nacharbeitsdefinition

Der aktuelle Nacharbeitsbegriff ist readmodel-basiert:

- `latest_approval_decision.status == rejected` bedeutet offene Nacharbeit.
- `latest_approval_decision.status == approved` hebt eine fruehere Ablehnung
  fachlich auf.
- `requested` und `cancelled` zaehlen nicht als terminale Entscheidung.

Diese Definition ist in Quote-Responses sichtbar und wird fuer UI-Warnungen
sowie Backend-Prozesssperren genutzt.

### 3. Bearbeitung nach Ablehnung

Nach einer Ablehnung gibt es keine aktive Anforderung mehr. Deshalb blockiert
der Quote-Update-Guard nicht:

- `Update(...)` blockiert nur `status = 'requested'`.
- `rejected` bleibt als Historie erhalten.
- Draft-Bearbeitung bleibt erlaubt.

Das ist fachlich richtig fuer Nacharbeit, erzeugt aber eine Luecke: Eine
Positionsaenderung allein hebt die offene Nacharbeit nicht auf, weil die letzte
terminale Entscheidung weiterhin `rejected` bleibt.

### 4. Erneute Anforderung ist technisch bereits moeglich

Nach einer Ablehnung kann technisch erneut eine Freigabeanforderung erstellt
werden, sofern:

- das Angebot weiterhin `draft` ist,
- die Position noch existiert,
- keine aktive `requested`-Anforderung existiert,
- eine Preisentscheidung als Kostenbasis vorhanden ist,
- der aktuelle Preis weiterhin unter Kostenbasis oder Zielpreis liegt.

Der bestehende `RequestApprovalForQuoteItem(...)`-Pfad verhindert keine
erneute Anforderung nach `rejected`. Das ist fuer den Rework-Flow nutzbar.

### 5. Zielmargenanker als UI-Gate

Der Client zeigt `Freigabe anfordern` nur an, wenn:

- ein Zielmargenanker geladen wurde,
- `targetMarginAnchor.canRequestApproval == true`,
- keine aktive `approvalRequest` am Item haengt.

Damit ist der Button im Editor fachlich an die aktuelle Kalkulation gebunden.
Nach einer Ablehnung muss der Nutzer aber den Zielmargenanker erneut laden
oder aktualisieren, damit die UI die neue Freigabefaehigkeit verlaesslich
bewertet.

### 6. Prozesssperren bleiben bis zur spaeteren Genehmigung aktiv

Versand, Annahme, Rechnungserzeugung und Auftragserzeugung werden gesperrt,
solange die letzte terminale Entscheidung einer Position `rejected` ist.

Eine neue `requested`-Anforderung hebt diese Sperre noch nicht auf. Erst eine
spaetere `approved`-Entscheidung wird im bestehenden Readmodel zur neuesten
terminalen Entscheidung und entsperrt die kommerzielle Weiterverarbeitung.

Das ist fachlich korrekt:

- Nacharbeit allein ist keine Freigabe.
- Eine erneute Anfrage ist noch keine Genehmigung.
- Die Sperre endet erst mit positiver neuer Entscheidung.

## Gefundene Luecken

### 1. Kein expliziter Rework-Zustand

Das System kennt keinen eigenen Zustand wie `rework_required` oder
`rework_submitted`. Offene Nacharbeit wird nur aus der letzten terminalen
Entscheidung abgeleitet.

Bewertung:

- Fuer den MVP ausreichend.
- Fuer spaetere Auswertungen und Benachrichtigungen wahrscheinlich zu wenig.
- Kein neuer Status im naechsten kleinen Leaf erforderlich.

### 2. Positionsaenderung erzeugt keine neue Kostenbasis

Wenn der Nutzer nur Beschreibung, Menge oder Preis bearbeitet, wird dadurch
nicht automatisch eine neue `quote_item_price_decisions`-Zeile erzeugt.

Folge:

- Eine erneute Anforderung nutzt weiterhin die letzte gespeicherte
  Preisentscheidung als Kostenbasis.
- Das ist fuer reine Verkaufspreis-Nacharbeit akzeptabel.
- Bei Materialwechsel oder geaenderter Kostenbasis muss vorher eine neue
  Preisentscheidung ueber Material-/Preisuebernahme entstehen.

### 3. UI-Kommunikation unterscheidet nicht zwischen Nacharbeit und Re-Request

Die Warnung sagt bereits, dass erneut Freigabe angefordert werden soll. Der
Editor fuehrt den Nutzer zur Position. Der Zielmargenblock selbst kennt aber
keinen speziellen Hinweis wie:

```text
Nacharbeit gespeichert; Freigabe erneut anfordern, falls Zielpreis weiter
unterschritten wird.
```

Das ist ein UI-Folgepunkt, aber nicht zwingend fuer den Backend-MVP.

### 4. Kein stale Snapshot Hinweis

Eine neue Anforderung nimmt neue Snapshots. Die alte Ablehnung bleibt mit alten
Snapshots in der Historie. Es gibt aber keinen expliziten Vergleich zwischen
alter Ablehnung und neuer Anfrage.

Bewertung:

- Fuer Audit-Historie tolerierbar.
- Fuer spaetere Entscheideransicht sinnvoll: "Preis seit Ablehnung geaendert".

## Fachliche Entscheidung fuer den naechsten Schritt

Der bestehende Backend-Pfad kann fuer erneute Freigabeanforderungen
wiederverwendet werden. Es wird keine neue Persistenz und kein neuer Endpunkt
benoetigt.

Der naechste kleinste Strategie-Leaf soll festlegen:

- wann die UI nach Nacharbeit den Zielmargenanker invalidiert,
- ob nach erfolgreichem Speichern einer nacharbeiteten Position die
  Freigabeanforderung explizit wieder angeboten wird,
- welche Fehlermeldung genutzt wird, wenn die Position durch Nacharbeit den
  Zielpreis erreicht und deshalb keine neue Freigabe mehr braucht,
- dass eine neue `requested`-Anforderung die Prozesssperre noch nicht aufhebt.

## Nicht-Ziele

Nicht Teil des direkten Folgepfads:

- neuer Status `rework_required`
- automatische Re-Request-Erstellung nach Speichern
- automatische Genehmigung bei erreichter Zielmarge
- neue Queue offener Nacharbeiten
- Migration fuer Versionierung von Nacharbeit
- KI-gestuetzte Rework-Empfehlungen

## Verifikation

Keine Tests ausgefuehrt, da dieses Leaf nur ein fachliches Inventar-Dokument
ergaenzt.

