# GAEB-Freigabe: Zielmodell fuer Kontext im Queue-Entscheidungsdialog

## Ziel

Subtask 3.1.52.2 schneidet das Zielmodell fuer eine Snapshot- und
Kontextzusammenfassung im bestehenden Entscheidungsdialog der
Freigabeanforderungs-Queue.

In diesem Leaf wird keine Laufzeitlogik geaendert. Das Ergebnis soll den
naechsten Implementierungsleaf so begrenzen, dass er client-only bleibt und den
vorhandenen Queue-Read-Model-Vertrag nutzt.

## Ausgangslage

Die direkte Entscheidung aus der `Freigabeanforderungen`-Karte ist umgesetzt:

- `Genehmigen` ruft `approveQuoteItemApprovalRequest(...)` auf.
- `Ablehnen` ruft `rejectQuoteItemApprovalRequest(...)` auf.
- Beide Aktionen nutzen den bestehenden Kommentar-Dialog.
- Aktionen sind nur mit `quotes.approve` sichtbar.
- Die Queue-Zeile zeigt bereits kompakten Kontext.

Der aktuelle Dialog selbst enthaelt aber noch keine Snapshotwerte. Damit muss
ein Freigebender fuer Detailwerte weiterhin in die Position navigieren.

## Vorhandener Datenvertrag

Der bestehende Queue-Endpunkt reicht fuer den Dialog-Kontext aus:

```text
GET /api/v1/quotes/approval-requests
```

Relevante Felder aus `QuoteApprovalRequestQueueItem`:

```text
quote_id
quote_number
quote_status
project_name
contact_name
quote_item_id
position
description
current_unit_price
currency
approval_request_id
reason_code
reason_text
requested_by
requested_by_name
requested_at
current_unit_price_snapshot
cost_basis_unit_price_snapshot
target_unit_price_snapshot
target_margin_percent_snapshot
target_difference_snapshot
margin_percent_snapshot
current_target_status
current_target_difference
current_target_unit_price
current_margin_percent
```

Nicht benoetigt fuer den ersten Dialog-Kontext:

- `project_id`
- `contact_id`
- `quote_date`
- `price_decision_id`
- `current_price_decision_id`

Diese IDs bleiben fuer Navigation, Filter und spaetere Detailansichten
relevant, muessen aber nicht im Dialog sichtbar werden.

## Dialog-Zuschnitt

Der naechste Implementierungsleaf soll den vorhandenen Kommentar-Dialog nicht
fachlich ersetzen, sondern um einen optionalen Kontextblock erweitern.

Zielbild:

- Ein gemeinsamer Dialog fuer Genehmigen und Ablehnen.
- Titel und Aktionslabel bleiben aktionsspezifisch.
- Kommentar bleibt optional.
- Der Kontextblock steht oberhalb oder direkt vor dem Kommentarfeld.
- Der bestehende 500-Zeichen-Serververtrag fuer Kommentare bleibt fuehrend.

Voraussichtlicher Client-Zuschnitt:

```text
_promptApprovalQueueDecisionComment(...)
```

erhaelt optional:

```text
Map<String, dynamic>? item
```

oder einen vorbereiteten Widget-/Summary-Parameter. Der kleinere Schnitt ist
ein optionaler `item`-Parameter, weil alle benoetigten Werte bereits in den
Queue-Handlern vorliegen.

## Inhalt des Kontextblocks

### Kopf

Pflichtwerte:

- Angebotsnummer: `quote_number`
- Position: `position`
- Beschreibung: `description`

Fallbacks:

- fehlende Angebotsnummer: `Angebot`
- fehlende Position: `-`
- leere Beschreibung: `Position`

### Auftraggeber- und Projektkontext

Optionale Werte:

- Projekt: `project_name`
- Kontakt: `contact_name`

Regel:

- Nur nicht-leere Werte anzeigen.
- Wenn beide fehlen, keine Ersatzzeile anzeigen.

### Anforderungsgrund

Pflichtwert mit Fallback:

- `reason_text`
- sonst Label aus `reason_code`
- sonst `Freigabe angefordert`

Der bestehende Helper `_approvalRequestReasonLabel(...)` kann
wiederverwendet werden.

### Anforderer und Zeitpunkt

Optionale Werte:

- `requested_by_name`
- sonst `requested_by`
- `requested_at`, formatiert mit vorhandener Datumslogik

Der bestehende Helper `_approvalRequestContext(...)` kann wiederverwendet oder
in kleinere Bausteine aufgeteilt werden.

### Preis- und Snapshotwerte

Der Dialog soll eine kleine Vergleichsgruppe zeigen:

- Aktueller Einzelpreis: `current_unit_price`
- Preis bei Anforderung: `current_unit_price_snapshot`
- Kostenbasis: `cost_basis_unit_price_snapshot`
- Zielpreis: `target_unit_price_snapshot`
- Zielmarge: `target_margin_percent_snapshot`
- Abweichung zum Zielpreis: `target_difference_snapshot`

Formatierung:

- Geldwerte mit `_formatMoney(..., currency)`.
- Prozentwerte mit zwei Nachkommastellen und Prozentzeichen.
- Positive Abweichungen mit `+`, negative Werte mit normalem Minus.
- Waehrung aus `currency`, Fallback `EUR`.

### Aktuelle Zielbewertung

Wenn vorhanden:

- Status: `current_target_status`
- Abweichung: `current_target_difference`
- aktueller Zielpreis: `current_target_unit_price`
- aktuelle Marge: `current_margin_percent`

Regel:

- Der bestehende Zielstatus-Labelhelper kann weiterverwendet werden.
- Die Zeile soll als aktueller Stand markiert sein, damit Snapshot und
  heutiger Stand nicht verwechselt werden.
- Wenn kein aktueller Zielstatus vorhanden ist, wird diese Gruppe
  ausgeblendet.

## Layout-Regeln

Der Dialog bleibt ein Entscheidungsdialog, keine Detailseite.

Pflicht:

- maximal zwei kurze Textgruppen vor dem Kommentarfeld
- keine Tabelle mit horizontalem Scrollen
- keine verschachtelten Cards
- laengere Beschreibungen maximal zweizeilig und dann ellipsieren
- auf schmalen Viewports untereinander statt mehrspaltig
- Aktionsbuttons bleiben am unteren Dialogrand sichtbar

Empfohlene Struktur:

1. Kopfzeile mit Angebot und Position.
2. Ein kurzer Kontexttext fuer Projekt, Kontakt, Grund und Anforderung.
3. Kompakte Wertezeilen fuer Preis, Kostenbasis, Zielpreis, Zielmarge und
   Abweichung.
4. Optionaler aktueller Zielstatus.
5. Kommentarfeld.

## Berechtigungen und Verhalten

Keine Aenderung:

- Sichtbarkeit der Queue bleibt `quotes.read`.
- Entscheidungsaktionen bleiben `quotes.approve`.
- `quotes.write` bleibt fuer die Entscheidung irrelevant.
- Approve-/Reject-Endpunkte bleiben unveraendert.
- Refresh-Strategie nach Erfolg oder Fehler bleibt unveraendert.

Der Kontextblock darf keine eigene Mutation ausloesen.

## Fehler- und Fallback-Verhalten

Der Dialog muss mit unvollstaendigen Queue-Daten robust bleiben:

- fehlende Strings werden ausgelassen oder erhalten die bestehenden Fallbacks
- fehlende optionale Zahlenwerte werden nicht angezeigt
- `0` ist ein gueltiger Zahlenwert und darf nicht als fehlend behandelt werden
- nicht parsebare Datumswerte werden nicht formatiert angezeigt
- unbekannte Statuscodes werden roh oder ueber bestehenden Fallback angezeigt

## Teststrategie fuer den naechsten Implementierungsleaf

Mindestens auszufuehren:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Wenn der Schnitt klein bleibt, reicht zusaetzlich eine manuelle Auditnotiz. Ein
Widgettest ist sinnvoll, aber nicht Bedingung fuer den ersten Dialog-Kontext,
solange in diesem Bereich keine stabile Widgettest-Struktur etabliert ist.

Prueffokus:

- Dialog oeffnet fuer Genehmigen und Ablehnen.
- Kontextblock nutzt vorhandene Queue-Daten.
- Kommentar wird weiterhin unveraendert an die bestehenden API-Methoden
  uebergeben.
- Fehlende optionale Werte brechen den Dialog nicht.
- Aktionen bleiben nur mit `quotes.approve` erreichbar.

## Nicht-Ziele

Nicht Teil des naechsten Implementierungsleafs:

- neue Backend-Endpunkte
- neue Felder im Queue-Readmodel
- neue Migrationen
- dedizierte Approval-Arbeitsliste
- Pagination
- Dashboard- oder Workflow-Cockpit-Integration
- KPI, SLA oder Priorisierung
- Massenaktionen
- Aenderung der Genehmigungs- oder Ablehnungssemantik
- automatische KI-Bewertung der Freigabe

## Naechster Implementierungsschnitt

Subtask 3.1.52.3 sollte genau eine Runtime-Aenderung umsetzen:

```text
Den bestehenden Queue-Kommentar-Dialog in `QuotesPage` um eine kompakte
Snapshot- und Kontextzusammenfassung aus dem vorhandenen Queue-Eintrag
erweitern.
```

Wenn die Aenderung beim Implementieren zu gross wird, ist sie in
Micro-Subtasks zu zerlegen:

1. Dialogsignatur und Kontextdatenuebergabe erweitern.
2. Wiederverwendbare Format-/Summary-Helfer fuer Snapshotwerte ergaenzen.
3. Kontextblock im Dialog rendern.
4. Genehmigen-/Ablehnen-Handler anpassen.
5. Formatierung und Analyse ausfuehren.

Bearbeitet werden sollte dann nur Micro-Subtask 1.

## Ergebnis

3.1.52.2 ist mit diesem Zielmodell abgeschlossen. Der naechste Leaf ist
3.1.52.3: Umsetzung der Snapshot- und Kontextzusammenfassung im bestehenden
Queue-Entscheidungsdialog von `QuotesPage`.
