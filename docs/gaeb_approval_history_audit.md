# GAEB-Freigabeanforderungen: Historien-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten read-only Historienpfad fuer
positionsbezogene Freigabeanforderungen.

Geprueft werden:

- Backend-Lesemodell
- HTTP/API und Berechtigung
- Integrationstests
- Client-API
- Quote-Editor-UI
- verbleibende Kanten vor Queue, Kommentar-UI oder mehrstufiger Freigabe

## 1. Ergebnis

Der zugeschnittene Historienblock ist fuer den aktuellen MVP-Scope
abgeschlossen.

Erfuellt:

- Alle Freigabeanforderungen einer Quote-Position sind ueber einen separaten
  read-only Endpoint lesbar.
- Aktive, genehmigte, abgelehnte und stornierte Anforderungen bleiben
  sichtbar.
- Der Lesepfad ist nicht auf Draft-Quotes beschraenkt.
- Historische Angebotsrevisionen duerfen gelesen werden.
- Die Historie wird nicht in die Quote-Detailantwort eingebettet.
- Lesen ist ueber `quotes.read` geschuetzt.
- Entscheidungen bleiben ueber `quotes.approve` geschuetzt.
- Der Client ruft die Historie nur explizit pro Position ab.
- Nach Request, Storno, Genehmigung oder Ablehnung wird eine zuvor geladene
  Historie lokal invalidiert.

Nicht umgesetzt und weiterhin bewusst ausserhalb dieses Blocks:

- zentrale Freigabe-Queue
- globale Filter ueber alle Angebote hinweg
- Kommentarerfassung im Client
- erneutes Oeffnen entschiedener Anforderungen
- mehrstufige Freigaben
- Benachrichtigungen
- vollstaendige Audit-Timeline ueber andere Quote-Aktionen

## 2. Backend-Befund

Service-Schnitt:

```text
ListApprovalRequestsForQuoteItem(ctx, quoteID, itemID)
```

Bewertung:

- Der Service prueft, dass die Position zur Quote gehoert.
- Der Service verwendet keine Draft- oder Revisionsblockade.
- Der Service ist rein lesend und benoetigt kein `FOR UPDATE`.
- Die Sortierung `created_at DESC, requested_at DESC` liefert die neuesten
  Anforderungen zuerst.
- Das vorhandene `QuoteItemApprovalRequest`-Struct deckt alle relevanten
  Historienfelder ab.

Der Scope ist richtig begrenzt: Der Historienpfad fuehrt keine neue Mutation
ein und veraendert weder Quote-Status noch Preise oder Summen.

## 3. HTTP/API-Befund

Endpoint:

```text
GET /api/v1/quotes/{id}/items/{itemID}/approval-requests
```

Berechtigung:

```text
quotes.read
```

Bewertung:

- Die Route ist semantisch klar vom schreibenden
  `POST /approval-requests` getrennt.
- Normale Angebotsleser koennen nachvollziehen, warum eine Position
  freigegeben, abgelehnt oder storniert wurde.
- Entscheidungsrechte bleiben davon getrennt.
- Die API bleibt explizit und vermeidet Wachstum der Quote-Detailantwort.

## 4. Test-Befund

Der HTTP-Integrationstest prueft im Entscheidungsumfeld:

- `approved`-Eintraege bleiben per History-GET sichtbar.
- `rejected`-Eintraege bleiben per History-GET sichtbar.
- Ein Token mit `quotes.read` darf die Historie lesen.
- Entscheidungskommentar und Entscheider-Metadaten werden geliefert.
- Approved-Snapshots erscheinen nur bei Genehmigung.
- Bei Ablehnung bleiben Approved-Snapshots leer.

Verifikation aus dem Implementierungsverlauf:

```text
go test ./internal/quotes ./internal/http
dart format lib/api.dart
flutter analyze lib/api.dart
dart format lib/pages/quotes_page.dart lib/api.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

## 5. Client-Befund

### 5.1 API

`client/lib/api.dart` enthaelt:

```text
getQuoteItemApprovalRequests(quoteId, itemId)
```

Die Methode ist ein schmaler read-only Wrapper auf den neuen Endpoint.

### 5.2 UI

`client/lib/pages/quotes_page.dart` zeigt im bestehenden Quote-Editor pro
Position einen separaten Abschnitt:

```text
Freigabehistorie
```

Der Abschnitt:

- wird nur in bestehenden Quotes angezeigt
- ist ueber `quotes.read` gated
- laedt nur auf expliziten Klick
- zeigt Status, Grund, Request-/Cancel-/Decision-Metadaten
- zeigt Preis-, Kosten-, Zielpreis-, Zielmargen- und Margensnapshots
- zeigt Entscheidungskommentar und genehmigte Snapshots, falls vorhanden
- zeigt bei leerer Historie eine kompakte Leeranzeige

Bewertung:

- Die UI bleibt positionsnah und passt zum bestehenden Quote-Editor.
- Es gibt keine neue Seite und keine Workflow-Konsole.
- Terminale Anforderungen verschwinden nicht mehr fachlich aus der Sicht.
- Die Historie bleibt bewusst read-only.

## 6. Offene Kanten

### 6.1 Kommentarerfassung im Client

Backend und Client-API koennen Kommentare fuer Genehmigen/Ablehnen senden.
Die aktuelle UI nutzt diese Moeglichkeit noch nicht.

Das ist kein Historienfehler, sondern ein eigener kleiner Folgepfad:

- Entscheidungsdialog fuer Genehmigen/Ablehnen
- optionaler Kommentar
- vorhandene Kommentarlimits weiterverwenden

### 6.2 Zentrale Queue

Offene Freigaben sind weiterhin positionsnah in der Quote sichtbar. Eine
zentrale Queue fuer Freigebende bleibt fachlich sinnvoll, ist aber groesser
als der Historienabschluss.

### 6.3 Anzeigenamen fuer Nutzer

Die Historie zeigt technische User-IDs aus `requested_by`, `cancelled_by` und
`decided_by`. Anzeigenamen waeren angenehmer, benoetigen aber einen
separaten Join oder ein kleines User-Readmodel.

Fuer den aktuellen Scope ist die technische ID ausreichend, weil die
Nachvollziehbarkeit nicht verloren geht.

### 6.4 Vollstaendige Timeline

Die Historie zeigt nur Freigabeanforderungen. Preisentscheidungen,
Quote-Speicherungen, Statuswechsel und GAEB-Herkunft sind weiterhin getrennte
Sichten.

Eine gemeinsame Timeline waere ein spaeterer Querschnitt und nicht Teil des
kleinen Freigabehistorienpfads.

## 7. Entscheidung

Innerhalb des Historienblocks bleibt kein weiterer kleiner Haertungsschritt
mit gutem Signal uebrig.

Begruendung:

- Persistenz existiert bereits.
- Backend-Lesepfad ist separat und getestet.
- Client-API ist angebunden.
- UI zeigt die Historie bei Bedarf an.
- Die wichtigsten terminalen Zustaende sind sichtbar.
- Weitere Verbesserungen sind neue Funktionsbloecke, nicht Korrekturen des
  aktuellen Blocks.

Naechster sinnvoller Abschnitt:

```text
Subtask 3.1.36.1: Zielstrecke nach Freigabehistorie inventarisieren und den kleinsten Folgeausbau zwischen Kommentar-UI, Freigabe-Queue und Nutzeranzeigenamen festlegen
```

Naheliegende Kandidaten:

- Kommentar-UI fuer Genehmigen/Ablehnen
- zentrale Freigabe-Queue fuer offene Anforderungen
- Anzeigenamen statt technischer User-IDs
- Entscheidungsbadge an Positionen mit letzter terminaler Entscheidung

Die Auswahl sollte bewusst inventarisiert werden, weil diese Punkte
unterschiedliche Nutzergruppen und Berechtigungsmodelle betreffen.
