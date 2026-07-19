# GAEB-Freigabe: Audit des Queue-Entscheidungsdialog-Kontexts

## Ziel

Subtask 3.1.52.4 auditiert die in 3.1.52.3 umgesetzte Snapshot- und
Kontextzusammenfassung im bestehenden Entscheidungsdialog der
Freigabeanforderungs-Queue.

In diesem Leaf wird keine Laufzeitlogik geaendert.

## Ergebnis

Die Dialog-Erweiterung ist fuer den aktuellen kleinen Schnitt konsistent
umgesetzt.

- Queue-Genehmigen und Queue-Ablehnen uebergeben den jeweiligen Queue-Eintrag
  an den bestehenden Kommentar-Dialog.
- Der Dialog rendert den Kontext nur optional ueber `contextSummary`.
- Editor-nahe Genehmigen-, Ablehnen- und Nacharbeitsentscheidungen bleiben ohne
  Kontextblock unveraendert.
- Die Kontextanzeige nutzt ausschliesslich vorhandene Felder des
  Freigabeanforderungs-Queue-Readmodels.
- Es gibt keine neuen Backend-Endpunkte, keine neuen API-Felder und keine
  neue Persistenz.

Damit ist die vorherige Luecke aus 3.1.51 geschlossen: Freigebende sehen die
wichtigsten Anfrage-, Preis- und Zielkontextwerte direkt im
Entscheidungsdialog.

## Zielmodell-Abgleich

### Gemeinsamer Dialog

Erfuellt.

`_QuoteApprovalDecisionCommentDialog` ist weiterhin der gemeinsame Dialog fuer
Freigabeentscheidungen. Die Erweiterung ist optional:

```text
contextSummary
```

Wenn kein Kontext uebergeben wird, rendert der Dialog nur das bisherige
Kommentarfeld. Dadurch bleiben die bestehenden Editor-Pfade stabil.

### Queue-Datenuebergabe

Erfuellt.

`_promptApprovalQueueDecisionComment(...)` verlangt den Queue-`item` und baut
daraus lokal den Kontext:

```text
_buildApprovalQueueDecisionContext(item)
```

Die beiden Queue-Aktionen uebergeben jeweils ihr Item:

- `_approveApprovalRequestQueueItem(...)`
- `_rejectApprovalRequestQueueItem(...)`

Die API-Mutationsaufrufe bleiben unveraendert:

```text
approveQuoteItemApprovalRequest(quoteId, quoteItemId, comment)
rejectQuoteItemApprovalRequest(quoteId, quoteItemId, comment)
```

### Kontextinhalt

Erfuellt fuer den vereinbarten MVP.

Der Kontextblock zeigt:

- Angebot, Position und Beschreibung
- optional Projekt und Kontakt
- Anforderer und Anforderungszeitpunkt
- Freigabegrund mit bestehendem Reason-Fallback
- aktueller Einzelpreis
- Preis bei Anfrage
- Kostenbasis
- Zielpreis
- Zielmarge
- Zielabweichung mit Vorzeichen
- optional aktuellen Zielstatuskontext

Die Anzeige nutzt vorhandene Helfer fuer Fallbacks und Datumslogik:

- `_approvalRequestReasonLabel(...)`
- `_approvalRequestContext(...)`
- `_approvalReworkCurrentTargetContext(...)`

### Zahlen- und Fallback-Verhalten

Erfuellt.

Geldwerte werden mit der vorhandenen Geldformatierung angezeigt. Fuer fehlende
oder leere Waehrung wird `EUR` verwendet. Prozentwerte werden mit zwei
Nachkommastellen formatiert. Positive Zielabweichungen erhalten ein `+`.

Wichtig fuer robuste Queue-Daten: optionale Zahlen laufen ueber
`_toOptionalDouble(...)`. Dadurch werden fehlende Werte ausgeblendet, waehrend
`0` als gueltiger Wert sichtbar bleibt.

### Layout-Grenzen

Erfuellt fuer den bestehenden Dialog.

Der Kontextblock ist ein kompakter Container oberhalb des Kommentarfelds. Der
Dialog bleibt ein Entscheidungsdialog und wird nicht zu einer Detailseite:

- keine neue Arbeitsliste
- keine Tabelle
- keine Massenaktion
- kein horizontaler Scroll-Kontext
- keine Verschachtelung in weitere fachliche Dialoge

Die Beschreibung wird auf zwei Zeilen begrenzt und bei laengerem Text
ellipsiert.

### Berechtigungen und Verhalten

Erfuellt.

Die Erweiterung aendert keine Berechtigungen:

- Queue-Lesen bleibt `quotes.read`.
- Queue-Entscheiden bleibt `quotes.approve`.
- `quotes.write` wird fuer Queue-Entscheidungen nicht eingefuehrt.

Die Refresh-Strategie bleibt ebenfalls unveraendert:

- Genehmigung aktualisiert die Freigabeanforderungs-Queue und ggf. das Detail.
- Ablehnung aktualisiert Freigabeanforderungs-Queue, Nacharbeits-Queue und ggf.
  das Detail.

## Nicht-Ziele

Weiterhin nicht umgesetzt:

- neue Backend-Route fuer globale Queue-Entscheidung
- neue Felder im Queue-Readmodel
- neue Migration
- dedizierte Approval-Arbeitsliste
- Pagination
- Massenentscheidungen
- Workflow-Cockpit-Integration
- KPI, SLA oder Priorisierung
- Widgettest fuer den Dialog-Kontext

Diese Grenzen wurden eingehalten.

## Verifikation

Ausgefuehrt:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Formatierung ohne Datei-Aenderung
- Flutter-Analyse fuer die betroffenen Client-Dateien gruen

## Restrisiken

- Es gibt weiterhin keinen Widgettest fuer die optionale Kontextanzeige im
  Kommentar-Dialog.
- Der Kontextblock ist fuer den kompakten Queue-Schnitt ausreichend; bei
  groesseren Freigabevolumina braucht es spaeter eher eine dedizierte
  Arbeitsliste als noch mehr Dialoginhalt.
- Der aktuelle Zielstatus wird ueber historisch nach Rework benannte Helfer
  formatiert. Fachlich ist die Wiederverwendung korrekt, der Name bleibt ein
  spaeterer Aufraeumkandidat.

## Empfehlung

Task 3.1.52 ist fachlich abgeschlossen.

Der naechste kleinste Schritt sollte wieder eine Folgeinventur sein. Sie soll
priorisieren, ob der naechste Ausbau eine dedizierte Approval-Arbeitsliste,
Testhaertung des Dialogs, Workflow-Cockpit-Signale oder KPI/SLA-Priorisierung
sein soll.
