# GAEB-Freigabeentscheidungen: Entscheidungsbadge-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten Read-only-Ausbau fuer das
positionsnahe Entscheidungsbadge im Quote-Editor.

Geprueft werden:

- Backend-Readmodel am Quote-Item
- HTTP-Vertrag und Testabdeckung
- Client-Parsing
- Client-Anzeige
- Scope-Grenzen vor Queue oder erweiterten Freigabeprozessen

## 1. Ergebnis

Der zugeschnittene Entscheidungsbadge-Block ist fuer den aktuellen MVP-Scope
abgeschlossen.

Erfuellt:

- Quote-Details liefern optional `latest_approval_decision` je Position.
- Das Readmodel enthaelt nur terminale Entscheidungen `approved` und
  `rejected`.
- `active_approval_request` bleibt die separate Sicht auf offene
  Freigabeanforderungen.
- Der Entscheidername nutzt denselben Fallback wie die Historie:
  `display_name -> email -> id`.
- Der Client parst das Readmodel in ein eigenes Draft.
- Der Quote-Editor zeigt das Badge ohne zusaetzlichen Ladezustand.
- Schreibpayloads bleiben unveraendert.
- Es gibt keine neue Migration, Permission, Mutation oder Queue.

Nicht umgesetzt und bewusst ausserhalb dieses Blocks:

- zentrale Freigabe-Queue
- Filter oder Sammelansicht fuer Freigebende
- automatische Historienliste im Quote-Detail
- Badge fuer `cancelled`
- User-Snapshot in `quote_item_approval_requests`
- neue API-Route fuer Badge-Daten

## 2. Backend-Befund

`QuoteItemInput` enthaelt nun optional:

```text
latest_approval_decision
```

Das neue DTO ist kompakter als `QuoteItemApprovalRequest` und enthaelt nur die
fuer das Badge benoetigten Entscheidungsdaten:

- ID
- Status
- Reason-Code und Reason-Text
- Entscheider-ID und Entscheider-Anzeigename
- Entscheidungszeitpunkt
- optionaler Entscheidungskommentar
- optionale genehmigte Snapshots

Die Daten werden im bestehenden `Service.Get(...)` geladen. Je Quote-Item wird
ein `LEFT JOIN LATERAL` auf `quote_item_approval_requests` verwendet:

```text
status IN ('approved', 'rejected')
ORDER BY decided_at DESC NULLS LAST, updated_at DESC
LIMIT 1
```

Bewertung:

- Das Badge kommt mit dem vorhandenen Quote-Detail und braucht keinen weiteren
  Roundtrip.
- Das Readmodel bleibt positionsnah und read-only.
- Die Historienroute bleibt die vollstaendige Audit-Quelle.
- `LEFT JOIN users` verhindert Datenverlust, falls der Entscheider-User fehlt.

## 3. HTTP- und Testbefund

Der bestehende HTTP-Test fuer Freigabeentscheidungen prueft nun auch die
Quote-Reloads nach terminaler Entscheidung.

Abgedeckt:

- Nach Genehmigung ist `active_approval_request` leer.
- Nach Genehmigung ist `latest_approval_decision.status == approved`.
- Genehmigtes Badge enthaelt Entscheider-ID, Entscheider-Anzeigename,
  Entscheidungszeitpunkt, Kommentar, Grund und genehmigte Snapshots.
- Nach Ablehnung ist `active_approval_request` leer.
- Nach Ablehnung ist `latest_approval_decision.status == rejected`.
- Abgelehntes Badge enthaelt Entscheiderdaten und Kommentar.
- Abgelehntes Badge enthaelt keine genehmigten Snapshots.

Bewertung:

- Der HTTP-Vertrag ist fuer approved/rejected abgedeckt.
- Die Trennung zwischen aktiver Anforderung und terminalem Badge wird
  explizit geprueft.
- Kein neuer Permission-Pfad wurde eingefuehrt.

## 4. Client-Befund

Der Quote-Editor parst das neue Feld in:

```text
_QuoteItemApprovalDecisionBadgeDraft
```

Das Draft enthaelt Anzeige-Getter fuer:

- Status
- Grund
- Entscheidungszeitpunkt
- Entscheider
- genehmigten Snapshot

`_QuoteItemDraft.toJson()` wurde nicht erweitert.

Bewertung:

- Das Readmodel bleibt read-only.
- Bestehende Save-/Update-Pfade senden keine Badge-Daten zurueck.
- Antworten ohne `latest_approval_decision` bleiben kompatibel.

## 5. UI-Befund

`_QuoteItemRow` zeigt bei vorhandenem Badge einen kompakten Container vor dem
Zielmargenblock.

Angezeigt werden:

- `Freigabe genehmigt` oder `Freigabe abgelehnt`
- fachlicher Grund, falls vorhanden
- `Entschieden <Zeitpunkt> von <Name>`
- optionaler Kommentar
- optional `Genehmigt <Preis> EUR  •  Zielmarge <Prozent> %`

Bewertung:

- Das Badge ist direkt an der Position sichtbar.
- Eine aktive Freigabeanforderung bleibt im bestehenden Aktionsbereich
  handlungsfuehrend.
- Die Historie bleibt weiterhin manuell ladbar und ersetzt das Badge nicht.
- Die Darstellung ist bewusst kompakt und vermeidet eine neue Timeline oder
  Sammelansicht.

## 6. Risiken und Grenzen

### 6.1 Breitere Quote-Detailquery

Die bestehende Detailquery wurde um einen weiteren lateralen Join erweitert.

Bewertung:

Fuer den aktuellen Scope ist das akzeptabel, weil der Join pro geladener
Quote-Position arbeitet und keine neue Screen- oder Ladezustandslogik
erfordert.

### 6.2 Mehrere terminale Entscheidungen

Das Badge zeigt nur die letzte terminale Entscheidung.

Bewertung:

Das ist fuer einen Schnellindikator korrekt. Die vollstaendige Historie bleibt
die Audit-Quelle.

### 6.3 Erneute Freigabe nach Ablehnung

Eine neue aktive Anforderung kann neben einer frueheren terminalen Entscheidung
sichtbar sein.

Bewertung:

Das ist fachlich gewollt: die aktive Anforderung fuehrt die naechste Aktion,
das Badge liefert Kontext zur letzten Entscheidung.

### 6.4 Keine Queue

Freigebende erhalten noch keine zentrale Arbeitsliste.

Bewertung:

Eine Queue bleibt ein groesserer Folgeblock mit Listenendpoint, Filtermodell,
Rollenwirkung und eigener UI.

## 7. Verifikation

Ausgefuehrte Befehle fuer den Abschluss dieses Blocks:

```text
go test ./internal/quotes ./internal/http
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Backend-Tests gruen.
- Flutter Analyzer ohne Issues.

## 8. Entscheidung

Der Entscheidungsbadge-Block ist nach erfolgreicher Verifikation
abgeschlossen.

Naechster sinnvoller Folgepfad:

```text
Subtask 3.1.39.1: Freigabe-Folgeaktionen nach terminaler Entscheidung fachlich zuschneiden
```

Begruendung:

- Request, Storno, Entscheidung, Historie, Nutzeranzeigenamen und Badge sind
  jetzt positionsnah sichtbar.
- Der naechste Engpass ist nicht mehr Darstellung, sondern was nach einer
  Genehmigung oder Ablehnung operativ passieren soll.
- Moegliche Folgeaktionen sind Wiedervorlage, Preisnacharbeit,
  Angebotsfreigabe, Sperren von Folgeprozessen oder Queue-Ausbau.
