# GAEB-Nacharbeits-Queue: Darstellungsstrategie fuer Entscheidungskontext

## Ziel

Subtask 3.1.49.2 schneidet die kompakte Darstellung der bereits vorhandenen
Queue-Felder technisch zu. In diesem Leaf wird noch kein Runtime-Code
geaendert.

## Grundsatz

Ein Queue-Eintrag muss zwei Zeitebenen eindeutig trennen:

- **Entscheidung:** Ablehnungsgrund, Kommentar, Entscheider und Zeitpunkt
- **Aktueller Stand:** heutiger Zielmargenstatus und aktuelle Zielabweichung

Historische Preis- und Zielmargen-Snapshots werden in diesem kleinen Ausbau
nicht angezeigt.

## Zielaufbau eines Queue-Eintrags

### Titel

```text
ANG-2026-001 · Pos. 3
```

Unveraendert:

- `quote_number`, Fallback `Angebot`
- `position`, Fallback `-`

### Kontextblock

Maximal vier sichtbare Zeilen:

1. Positionsbeschreibung
2. Ablehnungsgrund
3. optionaler Entscheidungskontext
4. optionaler aktueller Zielkontext

Der Aktionsbutton `Bearbeiten` bzw. `Anzeigen` bleibt rechts bestehen.

## Zeile 1: Positionsbeschreibung

Quelle:

- `description`
- Fallback `Position`

Darstellung:

- maximal eine Zeile
- `TextOverflow.ellipsis`

## Zeile 2: Ablehnungsgrund

Prioritaet:

1. `reason_text`
2. deutsches Label fuer `reason_code`
3. `Nacharbeit erforderlich`

Labels:

- `negative_margin` -> `Negative Marge`
- `below_target_margin` -> `Unter Zielmarge`
- unbekannter nicht-leerer Code -> unveraendert anzeigen

Darstellung:

- Praefix `Grund: `
- maximal eine Zeile
- `TextOverflow.ellipsis`

## Zeile 3: Entscheidungskontext

Die Zeile wird nur angezeigt, wenn mindestens Kommentar, Entscheider oder
Zeitpunkt vorhanden ist.

Prioritaet innerhalb der Zeile:

1. Kommentar
2. Entscheider
3. Zeitpunkt

Format:

```text
Kommentar: Materialansatz pruefen · Max Mustermann · 24.06.2026 10:15
```

Fallbacks:

- Kommentar leer: Kommentarsegment auslassen
- `decided_by_name` leer: `decided_by` verwenden
- beide Entscheiderfelder leer: Entscheidersegment auslassen
- `decided_at` leer oder nicht parsebar: Zeitpunktsegment auslassen

Datumsformat:

- vorhandene lokale Darstellung `dd.MM.yyyy HH:mm`
- dafuer `_formatDateTime(...)` wiederverwenden
- Rueckgabewert `-` nicht anzeigen

Darstellung:

- maximal eine Zeile
- `TextOverflow.ellipsis`

## Zeile 4: Aktueller Zielkontext

Die Zeile wird nur angezeigt, wenn `current_target_status` nicht leer ist.
Sie beginnt immer mit `Aktuell:`, damit keine Verwechslung mit den
Entscheidungssnapshots entsteht.

Statuslabels:

- `below_cost` -> `Unter Kostenbasis`
- `below_target` -> `Unter Zielmarge`
- `on_target` -> `Zielmarge erreicht`
- `above_target` -> `Ueber Zielmarge`
- unbekannter nicht-leerer Status -> unveraendert anzeigen

Wenn `current_target_difference` vorhanden ist:

```text
Aktuell: Unter Zielmarge · Abweichung -120,00 EUR
```

Wenn keine Abweichung vorhanden ist:

```text
Aktuell: Unter Zielmarge
```

Geldformat:

- Angebotswaehrung aus `currency`, Fallback `EUR`
- vorhandene `_formatMoney(...)`-Logik wiederverwenden
- positive Werte erhalten ein explizites `+`
- negative Werte behalten ihr Minuszeichen

`current_target_unit_price`, `current_unit_price` und
`current_margin_percent` bleiben in diesem Schnitt verborgen. Sie koennen
spaeter in einer Detailansicht relevant werden, wuerden den kompakten
Arbeitslisteneintrag jetzt aber ueberladen.

## Widget-Zuschnitt

Der bisherige einfache `subtitle: Text(...)` wird durch eine kleine
`Column(crossAxisAlignment: CrossAxisAlignment.start)` ersetzt.

Jede Kontextzeile ist ein eigener `Text` mit:

- `maxLines: 1`
- `overflow: TextOverflow.ellipsis`

`isThreeLine` wird entfernt, weil die Subtitle-Hoehe durch die `Column`
bestimmt wird. Die Queue bleibt auf maximal drei Eintraege begrenzt.

## Empfohlene lokale Helfer

In `_QuotesPageState`:

- `_approvalReworkReasonLabel(Map<String, dynamic> item)`
- `_approvalReworkDecisionContext(Map<String, dynamic> item)`
- `_approvalReworkTargetStatusLabel(String status)`
- `_approvalReworkCurrentTargetContext(Map<String, dynamic> item)`

Die Helfer bleiben reine Formatierungsfunktionen. Es entstehen keine neuen
Clientmodelle und keine API-Aenderungen.

## Abgrenzung

Nicht Teil der Implementierung:

- historische Snapshotanzeige
- Detaildialog fuer Queue-Eintraege
- neue Filter oder Sortierung
- Backend-/Datenbankaenderung
- Workflow-Cockpit
- KPI, Priorisierung oder Automation

## Ergebnis

Subtask 3.1.49.2 ist abgeschlossen. Der Darstellungsvertrag ist eindeutig,
kompakt und trennt historische Entscheidung von aktuellem Kalkulationsstand.
Der folgende Leaf kann ihn rein clientseitig in `QuotesPage` implementieren.
