# GAEB-Freigabe: Zielmodell fuer Queue-Entscheidungen

## Ziel

Subtask 3.1.51.2 schneidet das Zielmodell fuer direkte Genehmigung und
Ablehnung aus einer Freigabe-Queue-Sicht.

In diesem Leaf wird keine Laufzeitlogik geaendert. Das Zielmodell soll
festlegen, wie der naechste Implementierungsleaf klein bleibt und trotzdem
keinen zweiten fachlichen Entscheidungsvertrag einfuehrt.

## Ausgangslage

Die read-only Freigabeanforderungs-Queue ist umgesetzt:

```text
GET /api/v1/quotes/approval-requests
```

Sie liefert aktive Anforderungen mit:

```text
quote_item_approval_requests.status = 'requested'
```

Der Client zeigt diese Anforderungen in `QuotesPage` kompakt an und navigiert
zur betroffenen Quote-Position.

Die positionsnahen Mutationsendpunkte existieren bereits:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject
```

Sie verlangen:

```text
quotes.approve
```

Der optionale Kommentar wird ueber `comment` uebergeben und serverseitig auf
maximal 500 Zeichen begrenzt.

## Fachliche Entscheidung

Queue-Entscheidungen sollen keinen neuen Backend-Vertrag einfuehren.

Der naechste Implementierungsleaf soll nur die vorhandenen positionsnahen
Approve-/Reject-Endpunkte aus der Queue-Sicht aufrufen. Damit bleiben
Historie, Statuswechsel, Nacharbeitsfolge und Validierung an genau einer
fachlichen Stelle.

## UI-Ort

Der kleinste sinnvolle Startpunkt ist die bestehende Karte
`Freigabeanforderungen` in `QuotesPage`.

Begruendung:

- Die Karte laedt bereits das passende Readmodel.
- Der Nutzer sieht die offene Anforderung direkt im Angebotsbereich.
- Die vorhandene Navigation zur Quote-Position bleibt erhalten.
- Der Implementierungsumfang bleibt klein.

Grenze:

- Die Karte zeigt weiterhin maximal drei Eintraege.
- Sie wird keine vollwertige Approval-Arbeitsliste mit Pagination,
  Mehrfachauswahl oder Priorisierung.
- Wenn die Menge offener Freigaben groesser wird, ist eine dedizierte
  Arbeitsliste ein spaeterer eigener Ausbau.

## Sichtbarkeit und Aktionen

Sichtbarkeit bleibt:

```text
quotes.read
```

Aktionen sind nur sichtbar bzw. aktiv, wenn:

```text
quotes.approve
```

vorhanden ist.

Ohne `quotes.approve` bleibt ein Queue-Eintrag rein navigierend:

- `Anzeigen`
- oder bei bearbeitbarem Entwurf und `quotes.write`: `Bearbeiten`

Mit `quotes.approve` bekommt ein Eintrag zusaetzlich:

- `Genehmigen`
- `Ablehnen`

Die Aktionen duerfen nicht von `quotes.write` abhaengen. Freigeben ist ein
eigener fachlicher Pfad und wird bereits durch `quotes.approve` geschnitten.

## Mindestkontext vor Entscheidung

Vor einer direkten Entscheidung muss der Queue-Eintrag mindestens anzeigen:

- Angebotsnummer
- Projektname, falls vorhanden
- Kontaktname
- Positionsnummer
- Positionsbeschreibung
- Grund der Anforderung
- Antragsteller und Zeitpunkt
- aktueller Zielstatus
- aktuelle Zielabweichung

Der vorhandene Queue-Eintrag enthaelt ausserdem Snapshots. Fuer den ersten
Implementierungsleaf muessen nicht alle Snapshots sichtbar sein, aber der
Entscheidungsdialog soll auf die wichtigsten Werte hinweisen:

- aktueller Positionspreis
- Kostenbasis-Snapshot
- Zielpreis-Snapshot
- Zielmarge-Snapshot
- Zielabweichung-Snapshot

Damit kann ein Freigebender entscheiden, ohne zwingend zuerst den Editor zu
oeffnen. Die Navigation bleibt trotzdem verfuegbar.

## Kommentar-Dialog

Die Queue nutzt denselben fachlichen Dialog wie der Quote-Editor:

- Titel bei Genehmigung: `Freigabe genehmigen`
- Titel bei Ablehnung: `Freigabe ablehnen`
- optionaler Kommentar
- maximale Laenge: 500 Zeichen nach Backend-Vertrag

Clientseitig sollte die 500-Zeichen-Grenze nicht als alleinige Sicherheit
gelten. Der Server bleibt fuehrend. Ein clientseitiger Hinweis ist aber
nuetzlich, damit Nutzer nicht erst nach Absenden scheitern.

## Mutationsablauf

### Genehmigen

1. Nutzer klickt `Genehmigen`.
2. Kommentar-Dialog oeffnet sich.
3. Bei Bestaetigung wird
   `approveQuoteItemApprovalRequest(quoteId, quoteItemId, comment)` aufgerufen.
4. Der konkrete Queue-Eintrag zeigt Ladezustand.
5. Nach Erfolg:
   - Eintrag aus Freigabeanforderungs-Queue entfernen oder Queue neu laden.
   - Quote-Detail neu laden, falls diese Quote gerade ausgewaehlt ist.
   - SnackBar: `Freigabe genehmigt`.

### Ablehnen

1. Nutzer klickt `Ablehnen`.
2. Kommentar-Dialog oeffnet sich.
3. Bei Bestaetigung wird
   `rejectQuoteItemApprovalRequest(quoteId, quoteItemId, comment)` aufgerufen.
4. Der konkrete Queue-Eintrag zeigt Ladezustand.
5. Nach Erfolg:
   - Eintrag aus Freigabeanforderungs-Queue entfernen oder Queue neu laden.
   - Nacharbeits-Queue neu laden, weil die Position dorthin wechseln kann.
   - Quote-Detail neu laden, falls diese Quote gerade ausgewaehlt ist.
   - SnackBar: `Freigabe abgelehnt - Nacharbeit erforderlich`.

## Refresh-Strategie

Der kleinste robuste Refresh nach jeder Entscheidung:

```text
_loadApprovalRequestQueue()
_loadApprovalReworkQueue()
```

Wenn die aktuell selektierte Quote der entschiedenen Queue-Zeile entspricht,
soll zusaetzlich:

```text
_loadDetail(quoteId)
```

ausgefuehrt werden.

Ein vollstaendiges `_load()` ist nicht zwingend noetig und kann zu viel
Seiteneffekt haben, weil Angebotsliste, Importe und Selektion komplett neu
geladen werden.

## Fehler- und Race-Handling

Die Queue muss folgende Faelle akzeptieren:

- Die Anforderung wurde parallel bereits entschieden.
- Die Anforderung wurde parallel storniert.
- Der Nutzer hat kein `quotes.approve`.
- Die Quote oder Position ist nicht mehr verfuegbar.
- Der Kommentar ist zu lang.
- Die Session ist abgelaufen.

Verhalten:

- Serverfehlermeldung ueber `_quoteErrorMessage(...)` anzeigen.
- Ladezustand immer zuruecksetzen.
- Danach mindestens die Freigabeanforderungs-Queue neu laden, damit veraltete
  Eintraege verschwinden.
- Bei Ablehnungsfehlern die Nacharbeits-Queue nicht optimistisch veraendern.

## Client-Zuschnitt

Voraussichtliche minimale Erweiterung in `client/lib/pages/quotes_page.dart`:

- neue Loading-IDs:
  - `_approvingApprovalRequestQueueItemId`
  - `_rejectingApprovalRequestQueueItemId`
- neue Handler:
  - `_approveApprovalRequestQueueItem(Map<String, dynamic> item)`
  - `_rejectApprovalRequestQueueItem(Map<String, dynamic> item)`
- Wiederverwendung von `_promptApprovalDecisionComment(...)`
- Wiederverwendung von `widget.api.approveQuoteItemApprovalRequest(...)`
- Wiederverwendung von `widget.api.rejectQuoteItemApprovalRequest(...)`
- Aktionen nur anzeigen, wenn `widget.api.hasPermission('quotes.approve')`

Keine neue Methode in `ApiClient` ist noetig.

## Backend-Zuschnitt

Keine Backend-Aenderung im ersten Implementierungsleaf.

Der bestehende Vertrag reicht:

- `quotes.approve`
- UUID-Validierung in der Route
- Kommentarvalidierung
- Domainfehler bei fehlender aktiver Anforderung
- Statuswechsel auf `approved` oder `rejected`

Neue Endpunkte wie eine globale Queue-Entscheidung:

```text
POST /api/v1/quotes/approval-requests/{id}/approve
```

sind bewusst nicht Teil des naechsten Schnitts. Sie wuerden einen zweiten
Mutationspfad einfuehren.

## Teststrategie

### Backend

Keine neuen Backend-Tests erforderlich, solange keine Serverlogik geaendert
wird.

Bestehende Integrationstests decken die positionsnahen Approve-/Reject-Pfade
und die Queue-Sicht ab.

### Client

Mindestens auszufuehren:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Wenn Widgettests in diesem Bereich etabliert werden, sollten sie pruefen:

- Aktionen erscheinen nur mit `quotes.approve`.
- `Genehmigen` ruft den bestehenden API-Client mit Quote-ID und Item-ID auf.
- `Ablehnen` laedt nach Erfolg Freigabe- und Nacharbeits-Queue neu.
- Fehler zeigen SnackBar und setzen Ladezustand zurueck.

Falls keine Widgettest-Infrastruktur fuer diese Seite stabil vorhanden ist,
reicht fuer den ersten Implementierungsleaf eine fokussierte statische Analyse
plus manuelle Verifikationsnotiz.

## Nicht-Ziele

Nicht Teil des naechsten Implementierungsleafs:

- neue Backend-Endpunkte
- neue Migrationen
- Queue-Pagination
- dedizierte Approval-Arbeitsliste
- Zusammenlegung mit Nacharbeits-Queue
- Workflow-Cockpit-Integration
- KPI, SLA oder Priorisierung
- Massenaktionen
- Verantwortliche oder Eskalation
- KI-Auswertung von Entscheidungsgruenden

## Naechster Implementierungsschnitt

Subtask 3.1.51.3 sollte genau eine Runtime-Aenderung umsetzen:

```text
Genehmigen und Ablehnen in der bestehenden Freigabeanforderungs-Queue-Karte
ueber die vorhandenen positionsnahen API-Methoden.
```

Wenn sich die UI-Aenderung als zu gross erweist, ist sie in Micro-Subtasks zu
zerlegen:

1. Queue-Aktionshandler und Ladezustand ergaenzen.
2. Buttons nur fuer `quotes.approve` anzeigen.
3. Refresh von Freigabe- und Nacharbeits-Queue nach Erfolg ergaenzen.
4. Fehler-/Race-Verhalten pruefen und dokumentieren.

Bearbeitet werden sollte dann nur der erste Micro-Subtask.

## Ergebnis

Subtask 3.1.51.2 ist abgeschlossen. Das Zielmodell legt fest, dass direkte
Queue-Entscheidungen im ersten Schritt clientseitig auf die bestehende
Freigabeanforderungs-Queue aufsetzen und die vorhandenen positionsnahen
Approve-/Reject-Endpunkte wiederverwenden. Neue Backend-Vertraege,
Arbeitslisten, Cockpit-Integration und KPI bleiben bewusst nachgelagert.
