# GAEB-Freigabeentscheidungen: Kommentar-UI-Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das kleinste technische Zielmodell fuer
Entscheidungskommentare beim Genehmigen und Ablehnen von
Freigabeanforderungen zu.

Ausgangspunkt:

- Backend, HTTP und Client-API koennen bereits optionale Kommentare fuer
  `approve` und `reject` verarbeiten.
- Die Historie zeigt `decision_comment` bereits an.
- Die Quote-Editor-UI ruft Genehmigen/Ablehnen aktuell ohne Kommentar auf.

Der naechste Implementierungs-Leaf soll nur diese UI-Luecke schliessen.

## 1. Kleinster fachlicher Scope

Enthalten:

- ein gemeinsamer kleiner Dialog fuer Genehmigen und Ablehnen
- optionales Kommentarfeld
- clientseitige Laengenpruefung bis maximal 500 Zeichen
- Uebergabe des Kommentars an die vorhandenen API-Methoden
- unveraendertes Erfolgsverhalten nach Entscheidung
- unveraenderte Historieninvalidierung nach Entscheidung

Nicht enthalten:

- Pflichtkommentar bei Ablehnung
- Kommentar fuer Storno
- Kommentar fuer Freigabeanforderung
- Backend-Aenderung
- Migration
- neue Permission
- Freigabe-Queue
- Nutzeranzeigenamen
- Entscheidungsbadge

## 2. Vorhandene technische Basis

### 2.1 Backend und HTTP

Die Endpunkte akzeptieren bereits:

```json
{
  "comment": "..."
}
```

Betroffene Routen:

```text
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/approve
POST /api/v1/quotes/{id}/items/{itemID}/approval-requests/reject
```

Beide Routen:

- sind ueber `quotes.approve` geschuetzt
- trimmen und pruefen den Kommentar auf maximal 500 Zeichen
- speichern den Kommentar als `decision_comment`
- liefern die aktualisierte Freigabeanforderung zurueck

### 2.2 Client-API

Die API-Methoden sind bereits vorbereitet:

```text
approveQuoteItemApprovalRequest(quoteId, itemId, {comment})
rejectQuoteItemApprovalRequest(quoteId, itemId, {comment})
```

Sie senden `comment` nur, wenn der getrimmte Wert nicht leer ist.

### 2.3 Aktuelle UI-Luecke

Die Quote-Editor-Methoden rufen die API derzeit ohne Kommentar auf:

```text
_approveApproval(item)
_rejectApproval(item)
```

Damit entsteht zwar ein korrekter terminaler Status, aber kein fachlicher
Kommentar fuer die Historie.

## 3. UI-Zielmodell

### 3.1 Dialog

Ein neuer kleiner Stateful-Dialog:

```text
_QuoteApprovalDecisionCommentDialog
```

Eingaben:

- `title`
- `actionLabel`
- optionaler Hinweistext

Rueckgabe:

```text
String? comment
```

Semantik:

- `null`: Nutzer hat abgebrochen, keine API-Aktion
- `''`: Nutzer bestaetigt ohne Kommentar
- nicht leer: getrimmter Kommentar

Feld:

- `TextField`
- `maxLines: 4`
- `maxLength: 500`
- Label `Kommentar`
- kein Pflichtfeld

### 3.2 Aufrufpunkte

Die bestehenden Button-Callbacks bleiben semantisch gleich:

```text
onApproveApproval -> _approveApproval(item)
onRejectApproval -> _rejectApproval(item)
```

Die Methoden selbst oeffnen vor dem Setzen des Ladezustands den Dialog:

1. Quote- und Item-ID pruefen
2. aktive Anforderung pruefen
3. Dialog oeffnen
4. bei Abbruch return
5. passenden Ladezustand setzen
6. API mit `comment` aufrufen
7. aktive Anforderung entfernen
8. geladene Historie invalidieren
9. SnackBar anzeigen

Wichtig:

- Waehrend der Dialog offen ist, soll noch kein Spinner am Button laufen.
- Der bestehende Spinner startet erst nach Bestaetigung.
- Wird der Dialog abgebrochen, bleibt der UI-Zustand unveraendert.

## 4. Validierung

Clientseitig:

- `maxLength: 500` am Textfeld
- vor API-Aufruf `trim()`
- wenn nach `trim()` leer, leeren Kommentar uebergeben

Backend bleibt weiterhin die letzte Sicherheitsinstanz:

- HTTP validiert maximal 500 Zeichen
- Service validiert maximal 500 Zeichen

Keine zusaetzliche lokale Fehlermeldung ist im ersten Implementierungs-Leaf
notwendig, solange `maxLength` die Eingabe begrenzt.

## 5. Status- und Fehlerverhalten

Genehmigen:

- nutzt weiterhin `_approvingApprovalItemId`
- Erfolgssnack bleibt `Freigabe genehmigt`
- Fehlerfallback bleibt
  `Genehmigung der Freigabeanforderung fehlgeschlagen`

Ablehnen:

- nutzt weiterhin `_rejectingApprovalItemId`
- Erfolgssnack bleibt `Freigabe abgelehnt`
- Fehlerfallback bleibt
  `Ablehnung der Freigabeanforderung fehlgeschlagen`

Nach Erfolg:

```text
item.approvalRequest = null
item.approvalRequestsPerformed = false
item.approvalRequests = const []
```

Dieses Verhalten bleibt korrekt, weil eine zuvor geladene Historie nach der
Entscheidung veraltet waere.

## 6. Nicht-Ziele des Implementierungs-Leafs

Nicht im naechsten Leaf:

- bestehende API-Methoden umbauen
- Backend-Tests erweitern
- HTTP-Vertrag aendern
- Kommentar zur Pflicht machen
- Historie automatisch nach Entscheidung neu laden
- neue Felder in `_QuoteItemApprovalRequestDraft`
- neue Berechtigung
- Freigabe-Queue

## 7. Testschnitt

Fuer den Implementierungs-Leaf reicht eine Client-Verifikation:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Begruendung:

- Backend- und HTTP-Vertrag fuer Kommentare ist bereits vorhanden und im
  Entscheidungsworkflow getestet.
- Der kommende Leaf verdrahtet nur den bestehenden UI-Pfad mit der bestehenden
  Client-API.

Optional, falls spaeter Widget-Tests etabliert werden:

- Dialog gibt `null` bei Abbruch zurueck
- Dialog gibt getrimmten Kommentar bei Bestaetigung zurueck

## 8. Naechster Implementierungs-Leaf

```text
Subtask 3.1.36.3: Entscheidungskommentar-Dialog im Quote-Editor implementieren
```

Umfang:

- `_QuoteApprovalDecisionCommentDialog` ergaenzen
- `_approveApproval(...)` und `_rejectApproval(...)` vor API-Aufruf ueber den
  Dialog fuehren
- Kommentar an bestehende API-Methoden uebergeben
- bestehende Lade-, Fehler- und Erfolgslogik beibehalten

Nicht enthalten:

- Backend
- Migration
- Queue
- Anzeigenamen
- Badge
