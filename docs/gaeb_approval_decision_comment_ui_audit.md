# GAEB-Freigabeentscheidungen: Kommentar-UI-Audit

## Ziel dieses Audits

Dieses Dokument prueft die umgesetzte Kommentar-UI fuer Genehmigen und
Ablehnen positionsbezogener Freigabeanforderungen.

Geprueft werden:

- UI-Dialog
- Verdrahtung mit bestehenden API-Methoden
- Lade-, Abbruch- und Fehlerverhalten
- Historienwirkung
- offene Kanten vor Queue, Nutzeranzeigenamen oder Entscheidungsbadge

## 1. Ergebnis

Der zugeschnittene Kommentar-UI-Block ist fuer den aktuellen MVP-Scope
abgeschlossen.

Erfuellt:

- Genehmigen und Ablehnen oeffnen vor der API-Aktion einen gemeinsamen
  Kommentar-Dialog.
- Der Kommentar ist optional.
- Abbruch im Dialog fuehrt zu keiner API-Aktion.
- Der Spinner am Entscheidungsbutton startet erst nach Bestaetigung.
- Das Kommentarfeld ist auf 500 Zeichen begrenzt.
- Der Kommentar wird getrimmt an die bestehenden API-Methoden uebergeben.
- Leere Kommentare bleiben fachlich erlaubt.
- Bestehende Erfolgs- und Fehlerlogik bleibt unveraendert.
- Nach erfolgreicher Entscheidung wird die geladene Historie invalidiert.

Nicht umgesetzt und bewusst ausserhalb dieses Blocks:

- Pflichtkommentar bei Ablehnung
- Kommentar fuer Storno oder Freigabeanforderung
- Backend-Aenderungen
- neue Migration
- neue Permission
- zentrale Freigabe-Queue
- Nutzeranzeigenamen
- Entscheidungsbadge

## 2. UI-Befund

Neuer Dialog:

```text
_QuoteApprovalDecisionCommentDialog
```

Eigenschaften:

- `AlertDialog`
- `TextField`
- `maxLines: 4`
- `maxLength: 500`
- `Kommentar` als Label
- konfigurierbarer Titel
- konfigurierbares Aktionslabel
- Abbrechen gibt `null` zurueck
- Bestaetigen gibt den getrimmten Kommentar zurueck

Bewertung:

- Der Dialog ist klein und passt zum bestehenden Quote-Editor-Muster.
- Es gibt keine neue Seite und keinen neuen Workflow-Screen.
- Die UI verlangt keinen Kommentar, blockiert also den bisherigen schnellen
  Entscheidungsfluss nicht.

## 3. Verdrahtungs-Befund

Die vorhandenen Methoden:

```text
_approveApproval(item)
_rejectApproval(item)
```

oeffnen jetzt vor dem Ladezustand:

```text
_promptApprovalDecisionComment(...)
```

Danach wird der Kommentar an die bestehenden API-Methoden uebergeben:

```text
approveQuoteItemApprovalRequest(..., comment: comment)
rejectQuoteItemApprovalRequest(..., comment: comment)
```

Bewertung:

- Der bestehende API-Vertrag wird genutzt.
- Es gibt keine Backend- oder HTTP-Vertragsaenderung.
- Die vorhandenen Berechtigungen bleiben unveraendert.
- Abbruch im Dialog mutiert keinen lokalen Zustand.

## 4. Historienwirkung

Die Historie zeigt `decision_comment` bereits an. Durch die neue UI kann der
Wert nun aus dem Quote-Editor heraus erfasst werden.

Nach erfolgreicher Entscheidung bleibt das Verhalten unveraendert:

```text
item.approvalRequest = null
item.approvalRequestsPerformed = false
item.approvalRequests = const []
```

Bewertung:

- Die aktive Anforderung verschwindet weiterhin aus der aktiven Sicht.
- Eine zuvor geladene Historie wird bewusst invalidiert.
- Beim naechsten Historienabruf kann der neue Kommentar sichtbar werden.

## 5. Test- und Verifikationsbefund

Verifizierte Befehle aus dem Implementierungs-Leaf:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Formatierung erfolgreich.
- Analyzer ohne Issues.

Keine neuen Backend-Tests waren erforderlich, weil:

- der HTTP-Vertrag fuer `comment` bereits existiert
- die Service-Validierung bereits existiert
- der Leaf nur den bestehenden Client-Pfad mit vorhandenen API-Parametern
  verdrahtet

## 6. Offene Kanten

### 6.1 Pflichtkommentar bei Ablehnung

Ein Pflichtkommentar bei Ablehnung kann fachlich sinnvoll sein. Fuer den
aktuellen MVP bleibt der Kommentar optional, weil:

- Backend und API optional modelliert sind
- Genehmigen und Ablehnen denselben kleinen Dialog nutzen
- eine Pflichtregel eine neue fachliche Entscheidung waere

### 6.2 Storno- und Request-Kommentare

Storno und Freigabeanforderung selbst haben weiterhin keinen vergleichbaren
UI-Kommentarfluss.

Das ist kein Fehler dieses Blocks, sondern ein moeglicher spaeterer Ausbau,
falls fachlich mehr Begruendung vor der Entscheidung benoetigt wird.

### 6.3 Zentrale Queue

Offene Anforderungen sind weiterhin nur positionsnah in der Quote sichtbar.
Eine zentrale Queue bleibt der naheliegende groessere Folgeausbau fuer
Freigebende.

### 6.4 Nutzeranzeigenamen

Historie und Entscheidungskommentar zeigen weiterhin technische User-IDs fuer
`requested_by`, `cancelled_by` und `decided_by`.

Anzeigenamen sind ein eigener Readmodel-Ausbau.

### 6.5 Entscheidungsbadge

Terminale Entscheidungen sind weiterhin erst ueber den expliziten
Historienabruf sichtbar.

Ein positionsnahes Badge fuer die letzte terminale Entscheidung bleibt ein
eigener Folgepfad.

## 7. Entscheidung

Innerhalb des Kommentar-UI-Blocks bleibt kein weiterer kleiner
Haertungsschritt mit gutem Signal uebrig.

Begruendung:

- Die wichtigste UI-Luecke ist geschlossen.
- Der Kommentar wird ohne neuen Backend-Scope persistierbar.
- Die Historie kann den Kommentar bereits anzeigen.
- Abbruch-, Lade- und Fehlerverhalten bleiben konsistent.
- Weitere Punkte sind neue fachliche Regeln oder neue Lesemodelle.

Naechster sinnvoller Abschnitt:

```text
Subtask 3.1.37.1: Zielstrecke nach Entscheidungskommentaren inventarisieren und den kleinsten Folgeausbau zwischen Freigabe-Queue, Nutzeranzeigenamen und Entscheidungsbadge festlegen
```

Die Entscheidung sollte bewusst inventarisiert werden, weil diese
Folgeoptionen unterschiedliche Nutzergruppen, Berechtigungen und
Response-Modelle betreffen.
