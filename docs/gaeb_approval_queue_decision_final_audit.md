# GAEB-Freigabe: Abschluss-Audit der Queue-Entscheidung

## Ziel

Subtask 3.1.51.4 auditiert die direkte Genehmigung und Ablehnung aus der
Freigabeanforderungs-Queue gegen das Zielmodell, die Berechtigungen, die
Refresh-Strategie und die Fehlerpfade.

In diesem Leaf wird keine Laufzeitlogik geaendert.

## Ergebnis

Die direkte Queue-Entscheidung ist fuer den aktuellen kleinen Schnitt
konsistent umgesetzt.

- Es gibt keinen neuen Backend-Endpunkt.
- Die bestehenden positionsnahen Mutationsmethoden werden wiederverwendet.
- Aktionen sind nur mit `quotes.approve` sichtbar.
- Ohne `quotes.approve` bleibt die Queue eine reine Anzeigen-/Bearbeiten-
  Navigation.
- Der Queue-Eintrag zeigt pro Aktion einen eigenen Ladezustand.
- Nach Genehmigung wird die Freigabeanforderungs-Queue aktualisiert.
- Nach Ablehnung werden Freigabeanforderungs-Queue und Nacharbeits-Queue
  aktualisiert.
- Bei selektierter Quote wird das Detail nach erfolgreicher Entscheidung neu
  geladen.

Damit ist der erste direkte Arbeitslistenpfad fuer Freigebende umgesetzt, ohne
einen zweiten fachlichen Entscheidungsvertrag einzufuehren.

## Zielmodell-Abgleich

### Kein neuer Backend-Vertrag

Erfuellt.

Die Queue-Aktionen rufen weiterhin die vorhandenen Client-Methoden auf:

```text
approveQuoteItemApprovalRequest(quoteId, quoteItemId, comment)
rejectQuoteItemApprovalRequest(quoteId, quoteItemId, comment)
```

Diese Methoden zeigen auf die bestehenden Serverrouten:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject
```

Es gibt keine neue globale Route wie:

```text
POST /api/v1/quotes/approval-requests/{id}/approve
```

Damit bleiben Statuswechsel, Historie, Kommentarvalidierung und
Nacharbeitsfolge an der vorhandenen fachlichen Stelle.

### UI-Ort

Erfuellt.

Die Umsetzung sitzt in der bestehenden `Freigabeanforderungen`-Karte von
`QuotesPage`.

Die Karte bleibt kompakt:

- maximal drei sichtbare Eintraege
- keine Pagination
- keine Massenaktion
- keine neue dedizierte Arbeitsliste

Das entspricht dem vereinbarten ersten UI-Schnitt.

### Berechtigungen

Erfuellt.

Die Queue-Sicht bleibt ueber `quotes.read` geladen. Die neuen Aktionen werden
aber nur in diesem Zweig angezeigt:

```text
widget.api.hasPermission('quotes.approve')
```

Ohne `quotes.approve` bleibt der vorhandene Fallback aktiv:

- `Bearbeiten`, wenn `quotes.write` und Quote-Status `draft`
- sonst `Anzeigen`

Wichtig: Die Freigabeaktionen haengen nicht von `quotes.write` ab. Das ist
korrekt, weil `quotes.approve` der fachliche Entscheidungs-Schnitt ist.

## Mutationsablauf

### Genehmigen

Umgesetzt:

1. IconButton `Freigabe genehmigen`.
2. Kommentar-Dialog mit Titel `Freigabe genehmigen`.
3. Aufruf von `approveQuoteItemApprovalRequest(...)`.
4. Ladezustand ueber `_approvingApprovalRequestQueueItemId`.
5. Nach Erfolg:
   - `_loadApprovalRequestQueue()`
   - bei selektierter Quote `_loadDetail(quoteId)`
   - SnackBar `Freigabe genehmigt`

Bewertung:

Der Ablauf passt. Die Nacharbeits-Queue wird bei Genehmigung nicht neu geladen,
weil keine Nacharbeit entsteht. Das ist eine bewusste kleinere Variante des
Zielmodells und fachlich korrekt.

### Ablehnen

Umgesetzt:

1. IconButton `Freigabe ablehnen`.
2. Kommentar-Dialog mit Titel `Freigabe ablehnen`.
3. Aufruf von `rejectQuoteItemApprovalRequest(...)`.
4. Ladezustand ueber `_rejectingApprovalRequestQueueItemId`.
5. Nach Erfolg:
   - `_loadApprovalRequestQueue()`
   - `_loadApprovalReworkQueue()`
   - bei selektierter Quote `_loadDetail(quoteId)`
   - SnackBar `Freigabe abgelehnt - Nacharbeit erforderlich`

Bewertung:

Der Ablauf passt. Die Ablehnung verschiebt die Position aus der
Freigabeentscheidung in die Nacharbeit; der Refresh beider Queues bildet diese
fachliche Bewegung ab.

## Ladezustand

Erfuellt.

Neue State-Felder:

```text
_approvingApprovalRequestQueueItemId
_rejectingApprovalRequestQueueItemId
```

Der Eintrag bildet daraus:

```text
approving
rejecting
actionRunning
```

Bei laufender Aktion werden alle drei Eintragsaktionen deaktiviert:

- genehmigen
- ablehnen
- Position oeffnen

Der jeweils betroffene Button zeigt einen kleinen Spinner. Das verhindert
Doppelentscheidungen aus der UI.

## Fehler- und Race-Handling

Erfuellt fuer den ersten Schnitt.

Bei Fehlern:

- wird `_quoteErrorMessage(...)` genutzt,
- eine SnackBar angezeigt,
- die Freigabeanforderungs-Queue neu geladen,
- der Ladezustand im `finally` zurueckgesetzt.

Damit werden typische Race-Faelle abgefangen:

- Anforderung parallel entschieden
- Anforderung parallel storniert
- fehlende Berechtigung
- abgelaufene Session
- Kommentar zu lang

Bei Fehlern im Ablehnungspfad wird die Nacharbeits-Queue nicht optimistisch
veraendert. Das entspricht dem Zielmodell.

## Kontextanzeige

Teilweise erfuellt.

Die Queue-Zeile zeigt weiterhin:

- Angebotsnummer
- Positionsnummer
- Beschreibung
- Grund
- Antragsteller/Zeitpunkt
- aktuellen Zielstatuskontext

Nicht vollstaendig umgesetzt ist eine erweiterte Dialoganzeige fuer alle
Snapshotwerte. Der Kommentar-Dialog bleibt derselbe einfache Dialog wie im
Quote-Editor.

Bewertung:

Fuer den ersten Implementierungsschnitt ist das akzeptabel, weil die
Navigation zur Quote-Position weiterhin direkt verfuegbar bleibt und das
Backend-Readmodel die Snapshotwerte bereits liefert. Eine erweiterte
Entscheidungsdialog-Zusammenfassung sollte ein eigener spaeterer Leaf sein,
wenn Freigebende mehr Kontext direkt im Dialog brauchen.

## Nicht-Ziele

Weiterhin nicht umgesetzt:

- neue Backend-Endpunkte
- neue Migrationen
- dedizierte Approval-Arbeitsliste
- Pagination
- Massenentscheidungen
- Workflow-Cockpit-Integration
- KPI, SLA oder Priorisierung
- Verantwortliche oder Eskalation
- KI-Auswertung von Entscheidungsgruenden

Diese Grenzen wurden eingehalten.

## Verifikation

Ausgefuehrt:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
go test ./internal/migrate ./internal/quotes ./internal/http
```

Ergebnis:

- Flutter-Analyse gruen
- Go-Tests fuer die relevanten Backend-Pakete gruen

Vorher im Implementierungsleaf ebenfalls ausgefuehrt:

```text
dart format lib/pages/quotes_page.dart
```

## Restrisiken

- Es gibt noch keinen Widgettest fuer die neuen Queue-Aktionsbuttons.
- Der Kommentar-Dialog zeigt keine Snapshot-Zusammenfassung.
- Die kompakte Drei-Eintrags-Karte kann bei groesseren Freigabevolumina zu eng
  werden.
- Die Zielstatus-Helfer sind historisch nach Rework benannt, werden aber nun
  auch fuer Freigabeanforderungen genutzt.

Diese Punkte sind nicht blockierend fuer den aktuellen Leaf.

## Empfehlung

Task 3.1.51 sollte als fachlich abgeschlossen betrachtet werden.

Der naechste kleinste Folgeausbau sollte wieder eine Inventur sein. Naheliegende
Optionen:

- dedizierte Approval-Arbeitsliste mit Pagination,
- Snapshot-Zusammenfassung im Entscheidungsdialog,
- Workflow-Cockpit-Signal fuer offene Approval-Arbeit,
- KPI/SLA/Priorisierung,
- Testhaertung fuer Berechtigung und Race-Faelle.

Der sinnvollste naechste Leaf ist eine kurze Folgeinventur, die diese Optionen
nach operativem Nutzen und technischem Risiko priorisiert.
