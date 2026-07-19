# GAEB-Freigabehistorie: Nutzeranzeigenamen-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten Ausbau fuer lesbare Nutzeranzeigenamen
in der positionsbezogenen Freigabehistorie.

Geprueft werden:

- Backend-Readmodel
- Scanner-Kompatibilitaet
- HTTP-Vertrag und Testabdeckung
- Client-Darstellung
- offene Kanten vor Queue oder Entscheidungsbadge

## 1. Ergebnis

Der zugeschnittene Anzeigenamen-Block ist fuer den aktuellen MVP-Scope
abgeschlossen.

Erfuellt:

- Die Historie liefert optionale Anzeigename-Felder fuer Request, Storno und
  Entscheidung.
- Die technischen ID-Felder bleiben erhalten.
- Die Namensbildung nutzt den Fallback `display_name -> email -> id`.
- Fehlende User-Joins verlieren keine Historieneintraege.
- Write-Pfade bleiben mit dem gemeinsamen Scanner kompatibel.
- Die Client-Historie zeigt bevorzugt Anzeigenamen und faellt auf IDs zurueck.
- Bestehende Permissions bleiben unveraendert.

Nicht umgesetzt und bewusst ausserhalb dieses Blocks:

- zentrale Freigabe-Queue
- Entscheidungsbadge an Quote-Positionen
- User-Snapshot in `quote_item_approval_requests`
- Namensfelder in allen aktiven Approval-Antworten
- automatische Historienaktualisierung nach Entscheidung
- neue Migration
- neue Permission

## 2. Backend-Befund

Das Readmodel `QuoteItemApprovalRequest` enthaelt nun:

```text
requested_by_name
cancelled_by_name
decided_by_name
```

Die Erweiterung sitzt neben den bestehenden technischen ID-Feldern. Damit
bleiben bestehende Clients und Audit-Auswertungen kompatibel.

Die Historienquery in `ListApprovalRequestsForQuoteItem(...)` nutzt drei
`LEFT JOIN users`:

```text
requested_user
cancelled_user
decided_user
```

Bewertung:

- `LEFT JOIN` ist korrekt, weil geloeschte oder fehlende User die Historie
  nicht ausblenden duerfen.
- Die Fallback-Reihenfolge ist fuer Anzeige und Audit ausreichend robust.
- Leere technische IDs erzeugen leere Name-Felder statt irrefuehrender Werte.

## 3. Scanner-Befund

`scanApprovalRequest(...)` wurde um drei optionale Namenswerte erweitert.

Alle Write-Pfade, die denselben Scanner verwenden, liefern leere Alias-Spalten:

```text
'' AS requested_by_name
'' AS cancelled_by_name
'' AS decided_by_name
```

Bewertung:

- Die gemeinsame Scanner-Funktion bleibt erhalten.
- Die Spaltenreihenfolge ist explizit und konsistent.
- Schreibantworten bleiben im aktuellen Scope namenlos, ohne den Scanner zu
  brechen.

Das ist fuer den MVP akzeptabel, weil die lesbaren Namen erst nach explizitem
Historienabruf benoetigt werden.

## 4. HTTP- und Testbefund

Der bestehende HTTP-Test fuer Freigabeentscheidungen prueft nun auch die
Historiennamen.

Abgedeckt:

- genehmigte Historie enthaelt `requested_by_name`
- genehmigte Historie enthaelt `decided_by_name`
- abgelehnte Historie enthaelt `requested_by_name`
- abgelehnte Historie enthaelt `decided_by_name`
- technische IDs bleiben parallel vorhanden
- genehmigte Snapshots und abgelehnte Null-Snapshots bleiben unveraendert

Verifizierter Befehl:

```text
go test ./internal/quotes ./internal/http
```

Ergebnis:

- Backend-Tests gruen.

## 5. Client-Befund

`_QuoteItemApprovalRequestDraft` liest die neuen Felder:

```text
requestedByName
cancelledByName
decidedByName
```

Neue Display-Getter:

```text
displayRequestedBy
displayCancelledBy
displayDecidedBy
```

Die Historienzeilen verwenden diese Getter fuer:

- `Angefordert ... von ...`
- `Storniert ... von ...`
- `Entschieden ... von ...`

Bewertung:

- Die UI bleibt rueckwaertskompatibel mit Antworten ohne Name-Felder.
- Der Fallback verhindert leere Anzeigen, solange eine technische ID vorhanden
  ist.
- Es entsteht kein neuer Screen und kein neuer API-Client-Typ.

Verifizierte Befehle:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Formatierung erfolgreich.
- Analyzer ohne Issues.

## 6. Offene Kanten

### 6.1 Aktive Approval-Antworten

`active_approval_request` und direkte Schreibantworten liefern weiterhin keine
Anzeigenamen.

Bewertung:

Das ist fuer diesen Block akzeptabel, weil die aktive Positionssicht aktuell
keine Personenmetadaten prominent darstellt. Falls ein Badge oder eine Queue
auf direkte Antworten statt Historie setzt, muss dieses Readmodel gezielt
erweitert werden.

### 6.2 User-Snapshot

Die Historie zeigt aktuelle User-Daten, keinen historischen Snapshot.

Bewertung:

Fuer den MVP ist das korrekt, weil keine neue Migration und kein
personenbezogener Snapshot eingefuehrt werden sollte. Ein unveraenderlicher
Snapshot waere ein spaeterer Compliance-Ausbau.

### 6.3 Automatische Historienaktualisierung

Nach Genehmigen oder Ablehnen wird die geladene Historie weiterhin invalidiert
und muss explizit neu geladen werden.

Bewertung:

Das ist konsistent mit dem bisherigen UI-Verhalten. Eine automatische
Aktualisierung waere Komfort, aber nicht Voraussetzung fuer den Abschluss des
Namen-Blocks.

### 6.4 Zentrale Queue

Eine zentrale Freigabe-Queue bleibt fachlich wertvoll, ist aber groesser:

- neuer Listenendpoint
- Filter- und Sortiermodell
- Rollen- und Rechtewirkung
- neue UI-Seite oder Dashboard-Abschnitt

### 6.5 Entscheidungsbadge

Ein positionsnahes Badge fuer die letzte terminale Entscheidung bleibt der
kleinere naechste Ausbau:

- nutzt vorhandene Historien- und Approval-Daten
- verbessert die Quote-Editor-Sicht ohne neue Queue-Seite
- macht abgeschlossene Entscheidungen sichtbar, ohne erst Historie zu oeffnen

## 7. Entscheidung

Der Nutzeranzeigenamen-Block ist abgeschlossen.

Naechster sinnvoller fachlicher Folgepfad:

```text
Subtask 3.1.38.1: Entscheidungsbadge an Quote-Positionen technisch zuschneiden
```

Begruendung:

- Die Queue ist weiterhin sinnvoll, aber deutlich groesser.
- Ein Badge ist der kleinere UX-Schritt nach Historie, Kommentar und Namen.
- Freigebende und Vertrieb sehen terminale Entscheidungen schneller.
- Der Badge kann auf dem jetzt lesbaren Approval-Readmodel aufbauen.
- Die Umsetzung kann zunaechst read-only bleiben.

Minimalziel fuer den naechsten Leaf:

- klären, ob das Badge aus der bestehenden Historie oder einem eigenen
  `latest_approval_decision`-Readmodel kommt
- Statusvarianten `approved`, `rejected`, optional `cancelled` bewerten
- notwendige Felder fuer Client-Darstellung festlegen
- keine Queue, keine neue Mutation und keine neue Permission im ersten Schritt
