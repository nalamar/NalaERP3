# GAEB-Freigabeentscheidungen: Nacharbeitsmarkierung-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten kleinsten UI-Folgepunkt nach dem
positionsnahen Entscheidungsbadge:

```text
Nacharbeitsmarkierung bei zuletzt abgelehnter Freigabeentscheidung
```

Geprueft werden:

- fachliche Bedeutung von `rejected`
- Client-Auswertung des bestehenden Badge-Readmodels
- UI-Darstellung in der Positionskarte
- bewusst ausgeschlossene Prozesslogik
- Abschlussverifikation

## 1. Ergebnis

Die positionsnahe Nacharbeitsmarkierung ist fuer den aktuellen MVP-Scope
abgeschlossen.

Erfuellt:

- `latestApprovalDecision.status == rejected` wird clientseitig als
  Nacharbeitsbedarf interpretiert.
- Die Markierung erscheint direkt unter dem Entscheidungsbadge.
- Die Anzeige bleibt read-only.
- Der Hinweis benennt die naechste fachliche Richtung:
  Preis, Material oder Zielpreis anpassen und erneut Freigabe anfordern.
- Es gibt keine Backend-Aenderung.
- Es gibt keine neue Persistenz, Queue, Permission oder Prozesssperre.
- Schreibpayloads bleiben unveraendert.

Nicht umgesetzt und bewusst ausserhalb dieses Blocks:

- Aufgabenmodell oder Wiedervorlage mit Verantwortlichen
- Faelligkeiten oder Eskalation
- Quote-weite Warnung
- Backend-Sperre bei Statuswechsel oder Folgebelegen
- automatische Preis-, Material- oder Zielpreiskorrektur
- automatische Angebotsfreigabe
- zentrale Freigabe-Queue

## 2. Fachlicher Befund

`approved` bleibt im aktuellen Scope ein Abschluss der konkreten
Freigabeanforderung.

`rejected` erzeugt dagegen eine offene operative Erwartung:

- Rueckmeldung aus dem Entscheidungskommentar pruefen
- Position fachlich nacharbeiten
- Preis, Materialzuordnung oder Zielpreis korrigieren
- danach bei Bedarf erneut Freigabe anfordern

Bewertung:

- Die UI bildet diese fachliche Asymmetrie korrekt ab.
- Genehmigte Entscheidungen erzeugen keine kuenstliche Folgeaktion.
- Abgelehnte Entscheidungen werden deutlicher sichtbar, ohne einen neuen
  Workflow zu erzwingen.

## 3. Client-Befund

`_QuoteItemApprovalDecisionBadgeDraft` enthaelt nun:

```text
requiresRework
```

Die Ableitung ist bewusst klein:

```text
status == rejected
```

Bewertung:

- Kein neues Backend-Feld ist erforderlich.
- Das bestehende `latest_approval_decision` bleibt die Quelle.
- Antworten ohne Badge bleiben unveraendert kompatibel.

## 4. UI-Befund

`_QuoteItemRow` zeigt bei `requiresRework` innerhalb des bestehenden
Entscheidungsbereichs:

```text
Nacharbeit erforderlich
Preis, Material oder Zielpreis anpassen und erneut Freigabe anfordern.
```

Bewertung:

- Die Markierung sitzt positionsnah und direkt beim Entscheidungskontext.
- Sie ersetzt weder Badge noch Historie.
- Sie fuehrt Nutzer zur naechsten fachlichen Aktion, ohne einen neuen Button
  oder eine neue Mutation einzufuehren.
- Aktive Freigabeanforderungen bleiben weiterhin im Zielmargen-/Aktionsbereich
  handlungsfuehrend.

## 5. Scope-Grenzen

### 5.1 Keine Quote-weite Sperre

Das Angebot wird durch die Markierung noch nicht automatisch blockiert.

Bewertung:

Das ist fuer diesen Leaf korrekt. Eine Sperre muss definieren, welche Aktionen
betroffen sind und wann die Sperre als erledigt gilt.

### 5.2 Keine Aufgabe

Es wird keine Aufgabe mit Verantwortlichem oder Faelligkeit erzeugt.

Bewertung:

Die Markierung ist bewusst eine UI-Klarstellung. Ein Aufgabenmodell waere ein
eigener Workflow-Block.

### 5.3 Keine automatische Re-Freigabe

Eine erneute Freigabeanforderung bleibt an die bestehenden fachlichen
Voraussetzungen gebunden.

Bewertung:

Das verhindert, dass eine reine Anzeige versehentlich neue Prozesssemantik
einfuehrt.

## 6. Verifikation

Ausgefuehrter Abschlussbefehl:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Flutter Analyzer ohne Issues.

Backend-Tests sind fuer diesen Leaf nicht erforderlich, weil keine
Backend-Dateien oder API-Vertraege geaendert wurden.

## 7. Entscheidung

Der Nacharbeitsmarkierungs-Block ist nach erfolgreicher Client-Verifikation
abgeschlossen.

Naechster sinnvoller Folgepfad:

```text
Subtask 3.1.40.1: Quote-weite Warnung fuer abgelehnte Positionsentscheidungen fachlich zuschneiden
```

Begruendung:

- Die Position selbst markiert Nacharbeit jetzt sichtbar.
- Der naechste Nutzen liegt in einer Angebotskopf- oder Quote-weiten Sicht,
  damit abgelehnte Positionen in langen Angeboten nicht uebersehen werden.
- Eine echte Backend-Sperre sollte erst nach diesem sichtbaren Warnmodell
  zugeschnitten werden.
