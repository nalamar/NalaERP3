# GAEB-Freigabeentscheidungen: Positionsnummern in Nacharbeitswarnung-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten kleinen Client-Schritt:

```text
Quote-weite Nacharbeitswarnung zeigt betroffene Positionsnummern
```

Der Scope ist bewusst client-only.

## 1. Ergebnis

Der Positionsnummern-Ausbau ist fuer den aktuellen Leaf abgeschlossen.

Erfuellt:

- Die Warnung leitet betroffene Positionen aus der vorhandenen
  Quote-Detailantwort ab.
- Positionsnummern werden als `index + 1` aus `selected['items']` gebildet.
- Nur Items mit `latest_approval_decision.status == rejected` werden angezeigt.
- Die bestehende Anzahl in der Warnungsueberschrift bleibt erhalten.
- Es werden maximal drei Positionsnummern direkt angezeigt.
- Weitere betroffene Positionen werden als `+ n weitere` gekappt.
- Es gibt keine Backend-Aenderung und keinen neuen API-Vertrag.

## 2. Client-Befund

Neue Ableitungen:

```text
_quoteRejectedApprovalDecisionPositions(...)
_formatRejectedApprovalDecisionPositions(...)
```

Die erste Funktion liefert die betroffenen Positionsnummern. Die zweite Funktion
formatiert die Warnungszeile:

```text
Betroffene Position: Pos. 3
Betroffene Positionen: Pos. 3, 7
Betroffene Positionen: Pos. 3, 7, 9 + 2 weitere
```

Bewertung:

- Die Implementierung nutzt nur bereits geladene Daten.
- Fehlende oder unvollstaendige Decision-Objekte bleiben kompatibel.
- Die Anzeige passt zur sichtbaren Reihenfolge der Positionsliste.

## 3. Scope-Grenzen

Nicht umgesetzt:

- Backend-Feld `position`
- API-Fehlerdetails mit Positionsliste
- Sprunganker zur ersten betroffenen Position
- Fokus/Hervorhebung in der Positionsliste
- Nacharbeitsqueue
- Aufgabenmodell

Bewertung:

Diese Begrenzung ist korrekt. Der Schritt macht die bestehende Warnung
handlungsnaeher, ohne neue Prozess- oder Datenmodellsemantik einzufuehren.

## 4. Verifikation

Ausgefuehrter Befehl:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Flutter Analyzer ohne Issues.

Hinweis:

Der Analyzer hat lokale Dependency-Aufloesung ausgefuehrt. Inhaltliche
Lockfile-Aenderungen wurden wieder zurueckgenommen; `pubspec.lock` kann im
Arbeitsbaum wegen Line-Endings noch als beruehrt erscheinen, hat aber keinen
inhaltlichen Diff.

## 5. Entscheidung

Subtask 3.1.42.3 ist abgeschlossen.

Der naechste kleinste Folgepunkt ist:

```text
Subtask 3.1.42.4: Nacharbeitswarnung mit Positionsnummern abschliessend auditieren
```

Begruendung:

Nach der kleinen UI-Erweiterung sollte kurz geprueft werden, ob innerhalb dieses
Positionsnummern-Blocks noch ein weiterer kleiner Haertungsschritt sinnvoll ist
oder ob erst ein neuer UX-Schritt wie Sprunganker beginnen sollte.
