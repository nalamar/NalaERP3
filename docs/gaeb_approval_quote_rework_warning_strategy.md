# GAEB-Freigabeentscheidungen: Quote-weite Nacharbeitswarnung

## Ziel dieses Dokuments

Dieses Dokument schneidet den naechsten fachlichen Folgeblock nach der
positionsnahen Nacharbeitsmarkierung zu:

```text
Quote-weite Warnung fuer abgelehnte Positionsentscheidungen
```

Ausgangspunkt:

- Jede Position kann ein `latest_approval_decision` enthalten.
- Bei `status == rejected` zeigt die Positionskarte bereits
  `Nacharbeit erforderlich`.
- In langen Angeboten kann eine abgelehnte Position trotzdem uebersehen
  werden, wenn sie ausserhalb des sichtbaren Scrollbereichs liegt.
- Es gibt noch keine Quote-weite Warnung, keine Sperre und keine Queue.

Der naechste Schritt soll zuerst fachlich klaeren, wie eine Angebotskopf-Warnung
aus vorhandenen Positionsdaten abgeleitet wird.

## 1. Problem

Die positionsnahe Markierung loest das lokale Sichtbarkeitsproblem an genau
einer Position.

Offen bleibt:

- Nutzer sehen beim Oeffnen des Angebots nicht sofort, ob irgendwo Nacharbeit
  offen ist.
- Statusaktionen wie `Versendet`, `Annahme`, `In Rechnung` oder `In Auftrag`
  stehen optisch neben einem Angebot, auch wenn eine Position zuletzt abgelehnt
  wurde.
- Ohne Quote-weite Warnung ist die Prozesslage nur durch Scrollen der
  Positionen erkennbar.

Wichtig:

Dieser Block soll noch keine Aktion blockieren. Er soll nur die Sichtbarkeit im
Angebotskopf verbessern.

## 2. Fachliche Bedeutung

Eine Quote-weite Warnung bedeutet:

- Mindestens eine Position hat als letzte terminale Freigabeentscheidung
  `rejected`.
- Diese Position ist im aktuellen MVP als nacharbeitsbeduerftig zu betrachten.
- Das Angebot sollte vor Versand, Annahme oder Folgebeleg-Erzeugung fachlich
  geprueft werden.

Die Warnung bedeutet noch nicht:

- Der Quote-Status darf nicht gewechselt werden.
- Folgebelege sind technisch verboten.
- Das Angebot ist abgelehnt.
- Alle Positionen sind risikobehaftet.
- Eine Aufgabe oder Wiedervorlage wurde erzeugt.

## 3. Datenquelle

Der erste Implementierungsblock soll die Warnung rein aus der vorhandenen
Quote-Detailantwort ableiten:

```text
selected.items[*].latest_approval_decision.status == rejected
```

Begruendung:

- `Service.Get(...)` liefert das Badge-Readmodel bereits.
- Der Client hat die Detaildaten im Angebotsdetail bereits vorliegen.
- Kein neuer Endpoint ist noetig.
- Keine Migration und keine neue Permission sind noetig.
- Die Warnung bleibt konsistent mit der Positionsmarkierung.

Nicht als erste Datenquelle:

- separate Backend-Aggregation
- neues `quote.approval_summary`-Feld
- Workflow-Cockpit
- zentrale Approval-Queue

Diese Optionen bleiben spaeter sinnvoll, sobald Sperren, Queue oder
Listenfilter gebraucht werden.

## 4. Anzeigeort

Die Warnung soll im Angebotsdetail oberhalb oder direkt nach den Kopf-Chips
stehen.

Naheliegende Stelle im bestehenden Client:

- nach den Status-/Kunden-/Projekt-/Folgebeleg-Chips
- vor Hinweis, Quote-Date und Gueltigkeit

Begruendung:

- Dort sieht der Nutzer sie vor Positionsliste und Aktionen.
- Sie bleibt Teil des Angebotskontexts und nicht Teil einer einzelnen Position.
- Sie steht nah an den Status- und Folgebeleginformationen.

## 5. Wording

Empfohlenes kompaktes Wording:

```text
Nacharbeit offen
Mindestens eine Position wurde zuletzt abgelehnt. Positionen pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.
```

Bei mehreren betroffenen Positionen kann optional gezählt werden:

```text
Nacharbeit offen: 3 Positionen
```

Fuer den ersten Implementierungsleaf reicht:

- Zaehlen der betroffenen Positionen
- Singular/Plural im Text
- keine Positionsnummernliste
- kein Sprunganker

## 6. UX-Regeln

### 6.1 Sichtbarkeit

Die Warnung wird nur angezeigt, wenn mindestens eine Position abgelehnt ist.

### 6.2 Read-only

Die Warnung ist ein Hinweis, kein eigener Workflow-Container.

Nicht enthalten:

- neuer Button
- Checkbox
- Statuswechsel
- Aufgabe erzeugen
- Freigabe direkt aus der Warnung anfordern

### 6.3 Keine Sperrwirkung

Status- und Folgebelegaktionen bleiben im ersten UI-Leaf technisch
unveraendert.

Begruendung:

- Eine Sperre braucht Backend-Regeln.
- Die Erledigungslogik nach Nacharbeit ist noch nicht verbindlich modelliert.
- UI-only-Sperren waeren inkonsistent und leicht umgehbar.

### 6.4 Positionsmarkierung bleibt Quelle fuer Details

Die Quote-weite Warnung zeigt nur den aggregierten Zustand.

Die Details bleiben in:

- Positionskarte
- Entscheidungsbadge
- Freigabehistorie

## 7. Minimalzielbild fuer den naechsten Implementierungsleaf

Client:

- Hilfsfunktion oder lokaler Getter im `QuotesPage`-Detailbereich:
  - Anzahl Positionen mit `latest_approval_decision.status == rejected`
- Warncontainer im Angebotsdetail, wenn Anzahl > 0
- Text mit Singular/Plural
- keine Backend-Aenderung
- keine Schreibpayload-Aenderung

Backend:

- keine Aenderung

Tests/Verifikation:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Verifikation ist nicht erforderlich, solange keine Backend-Dateien oder
API-Vertraege geaendert werden.

## 8. Bewusst ausgeschlossene Punkte

Nicht Teil des naechsten Implementierungsleafs:

- Backend-Aggregat `approval_summary`
- Quote-weite Prozesssperre
- Statuswechsel-Validierung im Backend
- Sperre fuer `accept`, `convertToInvoice` oder `convertToSalesOrder`
- Positionsnummernliste mit Scroll-to-Position
- zentrale Queue
- Aufgaben, Verantwortliche oder Faelligkeiten
- neue Permission

## 9. Spaetere Folgepunkte

Nach der quote-weiten Warnung koennen sinnvoll folgen:

1. Backend-Readmodel fuer Angebots-Freigabezustand.
2. Backend-Prozesssperre fuer kritische Status- oder Folgebelegaktionen.
3. Quote-weite Liste der betroffenen Positionen mit Positionsnummer.
4. Workflow-Cockpit fuer offene Freigaben und Nacharbeit.
5. Aufgabenmodell mit Verantwortlichen und Faelligkeiten.

## 10. Naechster Implementierungs-Leaf

```text
Subtask 3.1.40.2: Quote-weite Nacharbeitswarnung im Angebotsdetail anzeigen
```

Umfang:

- clientseitige Ableitung aus `selected['items']`
- Warnung im Angebotsdetail anzeigen
- Singular/Plural fuer betroffene Positionsanzahl
- keine Backend-Aenderung
- keine Prozesssperre

Nicht enthalten:

- Queue
- Backend-Summary
- Status- oder Folgebelegblockade
- Aufgabenmodell
