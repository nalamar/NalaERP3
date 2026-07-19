# GAEB-Freigabeentscheidungen: Positionsnummern-Warnung Abschlussaudit

## Ziel dieses Abschlussaudits

Dieses Dokument prueft, ob nach der clientseitigen Anzeige betroffener
Positionsnummern in der quote-weiten Nacharbeitswarnung noch ein weiterer
kleiner Haertungsschritt innerhalb desselben Blocks offen ist.

## 1. Ergebnis

Der Positionsnummern-Block ist abgeschlossen.

Erfuellt:

- Die quote-weite Warnung zeigt weiterhin die Anzahl offener
  Nacharbeitspositionen.
- Die Warnung zeigt nun zusaetzlich betroffene Positionsnummern.
- Die Positionsnummern werden clientseitig aus der vorhandenen
  `selected['items']`-Reihenfolge abgeleitet.
- Die Anzeige bleibt kompatibel mit fehlenden oder unvollstaendigen
  `latest_approval_decision`-Objekten.
- Die Anzeige wird bei mehr als drei betroffenen Positionen gekappt.
- Es gibt keine Backend-Aenderung und keinen neuen API-Vertrag.

## 2. Umsetzung gegen Zuschnitt

Zuschnitt:

```text
Positionsnummern aus selected['items'] per index + 1 ableiten
```

Umsetzung:

```text
_quoteRejectedApprovalDecisionPositions(...)
```

Zuschnitt:

```text
Betroffene Position: Pos. 3
Betroffene Positionen: Pos. 3, 7
Betroffene Positionen: Pos. 3, 7, 9 + 2 weitere
```

Umsetzung:

```text
_formatRejectedApprovalDecisionPositions(...)
```

Bewertung:

Die Umsetzung entspricht dem technischen Zuschnitt.

## 3. Scope-Bewertung

Bewusst nicht umgesetzt:

- Backend-Feld `position`
- strukturierte API-Fehlerdetails
- Sprunganker zur ersten betroffenen Position
- automatische Hervorhebung der Positionskarte
- Queue oder Aufgabenmodell

Bewertung:

Diese Punkte sind echte Folgeausbauten. Sie sollten nicht in denselben Block
gezogen werden, weil sie entweder neue UI-Mechanik, neue API-Semantik oder ein
neues Arbeitslistenmodell brauchen.

## 4. Verifikation

Bereits im Implementierungsleaf ausgefuehrt:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Flutter Analyzer ohne Issues.

Backend-Tests sind fuer diesen Block nicht erforderlich, weil kein Backend-Code
und kein API-Vertrag geaendert wurden.

## 5. Entscheidung

Subtask 3.1.42.4 ist abgeschlossen.

Innerhalb des Positionsnummern-Warnungsblocks ist kein weiterer kleiner
Haertungsschritt mit gutem Signal offen.

Der naechste sinnvolle Folgepfad ist:

```text
Subtask 3.1.43.1: Sprungziel zur ersten Nacharbeitsposition fachlich inventarisieren
```

Begruendung:

Die Warnung nennt nun die betroffenen Positionen. Der naechste eigenstaendige
UX-Schritt waere nicht mehr reine Information, sondern Navigation zur ersten
betroffenen Position.
