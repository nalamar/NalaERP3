# GAEB-Freigabe: Widget-Teststrategie fuer den Queue-Entscheidungsdialog

## Ziel

Subtask 3.1.53.2 schneidet den kleinsten stabilen Widgettest fuer den
Kontextblock im Entscheidungsdialog der Freigabeanforderungs-Queue zu.

Dieser Leaf aendert weder Laufzeitcode noch Tests. Er legt nur den Vertrag fuer
den naechsten Implementierungsleaf fest.

## Bestehende Testbasis

`client/test/sales_order_context_pages_test.dart` enthaelt bereits:

- `_FakeApiClient` als ueberschreibbaren `ApiClient`
- konfigurierbare Berechtigungen ueber `hasPermission(...)`
- mehrere `QuotesPage`-Widgettests
- Helfer fuer einen grossen Test-Viewport

Approval-Queue-Readmodel und Approval-Mutationen werden im Fake bislang nicht
ueberschrieben. Ohne diese Overrides wuerde ein Test mit `quotes.read` die
echten Clientmethoden fuer Import-, Nacharbeits- und Freigabe-Queues erreichen.

## Repraesentativer Pfad

Der erste Test deckt nur `Genehmigen` aus der Freigabeanforderungs-Queue ab.

Das ist ausreichend, weil Genehmigen und Ablehnen gemeinsam verwenden:

- `_promptApprovalQueueDecisionComment(...)`
- `_buildApprovalQueueDecisionContext(...)`
- `_QuoteApprovalDecisionCommentDialog`
- dieselbe Kommentartrim- und Rueckgabesemantik

Ablehnen unterscheidet sich danach nur durch Mutationsmethode, Erfolgsmeldung
und zusaetzlichen Nacharbeits-Queue-Refresh. Ein zweiter nahezu identischer
Dialogtest waere fuer diesen ersten Haertungsleaf redundant.

## Minimale Fake-API-Erweiterung

Der Fake erhaelt:

```text
approvalRequestList
approvedApprovalRequests
```

Notwendige Overrides:

```text
listQuoteImports(...) -> []
listQuoteApprovalRework(...) -> []
listQuoteApprovalRequests(...) -> approvalRequestList
approveQuoteItemApprovalRequest(...) -> Aufruf erfassen und Erfolg liefern
```

Der erfasste Aufruf soll mindestens enthalten:

```text
quote_id
quote_item_id
comment
```

Nach erfolgreicher Genehmigung darf `listQuoteApprovalRequests(...)` weiterhin
dieselbe Testliste liefern. Der Test prueft nicht das Entfernen des Eintrags,
sondern den Dialog- und Mutationsvertrag.

## Testdaten

Genau ein Queue-Eintrag mit stabilen Werten:

```text
quote_id: q-approval-1
quote_item_id: qi-approval-1
approval_request_id: ar-1
quote_number: ANG-2026-0100
quote_status: draft
position: 2
description: Brandschutztuer T30
project_name: Projekt Nord
contact_name: Metallbau Kunde
reason_text: Zielmarge unterschritten
requested_by_name: Erika Pruefer
requested_at: fester ISO-Zeitpunkt
currency: EUR
current_unit_price: 0
current_unit_price_snapshot: 950
cost_basis_unit_price_snapshot: 800
target_unit_price_snapshot: 1000
target_margin_percent_snapshot: 20
target_difference_snapshot: -50
current_target_status: below_target
current_target_difference: 0
current_target_unit_price: 1000
current_margin_percent: 20
```

Die beiden Nullwerte sind absichtlich enthalten. Damit wird die Regression
abgesichert, dass `0` nicht wie ein fehlender optionaler Zahlenwert behandelt
wird.

## Berechtigungen

Der Fake verwendet genau:

```text
quotes.read
quotes.approve
```

`quotes.read` laedt und zeigt die Queue. `quotes.approve` macht die
Genehmigen-Aktion sichtbar. `quotes.write` ist nicht notwendig und darf nicht
Teil dieses Tests sein.

## Testablauf

1. Grossen Viewport vorbereiten.
2. `QuotesPage` mit Fake-API und dem Queue-Eintrag pumpen.
3. Auf den Button mit Tooltip `Freigabe genehmigen` tippen.
4. Dialogtitel `Freigabe genehmigen` pruefen.
5. Stabile Kontextwerte pruefen.
6. In `Kommentar` einen Wert mit aeusseren Leerzeichen eingeben.
7. Dialogaktion `Genehmigen` ausloesen.
8. Genau einen erfassten API-Aufruf mit Quote-ID, Item-ID und getrimmtem
   Kommentar pruefen.

## Stabile Assertions

Pflichtassertions:

- `ANG-2026-0100 · Pos. 2` ist vor der Aktion sichtbar.
- Dialogtitel `Freigabe genehmigen` ist sichtbar.
- Beschreibung, Grund und mindestens ein Snapshotwert sind im Dialog sichtbar.
- `Aktueller Einzelpreis: 0.00 EUR` ist sichtbar.
- Ein aktueller Zielkontext mit `0.00 EUR` Abweichung ist sichtbar.
- Der erfasste Aufruf enthaelt `q-approval-1`, `qi-approval-1` und den
  getrimmten Kommentar.

Nicht auf exakte Datumsformatierung oder die gesamte Kontextzeichenkette
assertieren. Das vermeidet Zeitzonen- und Layoutkopplung.

## Implementierungsgrenze

Subtask 3.1.53.3 soll nur:

- `_FakeApiClient` um die beschriebenen Daten und Overrides erweitern
- genau einen Widgettest fuer den Genehmigen-Pfad ergaenzen
- keine Produktionsdatei aendern

Falls die Fake-Erweiterung unerwartet gross wird, ist sie in Micro-Subtasks zu
zerlegen; dann wird zuerst nur der deterministische Queue-Fake umgesetzt.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage approval queue decision dialog shows context and forwards comment"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

## Nicht-Ziele

- Ablehnen-Widgettest
- Fehler- oder Retry-Test
- Editor-nahe Approval-Dialoge
- Golden- oder Screenshot-Test
- Produktionscode-Refactoring
- neue Keys allein fuer den Test
- Backend-, API- oder Persistenz-Aenderung

## Ergebnis

3.1.53.2 ist abgeschlossen. Der naechste Leaf 3.1.53.3 implementiert genau
einen deterministischen Widgettest fuer Genehmigen aus der Approval-Queue,
einschliesslich Kontext-, Nullwert- und Kommentarweitergabe-Pruefung.
