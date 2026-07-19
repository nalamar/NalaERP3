# GAEB Approval Rework Jump Target Final Audit

## Scope

- Leaf: `3.1.43.4`
- Ziel: Sprungziel-Folgepfad nach Implementierung abschliessend pruefen und die naechste kleine GAEB-Nacharbeitsluecke ableiten.
- Bezug:
  - `docs/gaeb_approval_rework_jump_target_inventory.md`
  - `docs/gaeb_approval_rework_jump_target_strategy.md`
  - `docs/gaeb_approval_rework_jump_target_audit.md`

## Abschlusspruefung

Der Sprungpfad ist fuer den aktuellen MVP-Zuschnitt geschlossen:

- Die quote-weite Warnung wird nur angezeigt, wenn mindestens eine Position eine letzte Freigabeentscheidung mit `status == rejected` besitzt.
- Die betroffenen Positionen werden aus der vorhandenen `selected['items']`-Reihenfolge 1-basiert abgeleitet.
- Die erste betroffene Position ist ueber `Map<int, GlobalKey>` und `KeyedSubtree` adressierbar.
- Der Button `Zur ersten Position` bleibt im Kontext der roten Nacharbeitswarnung.
- Der Scroll erfolgt ueber `Scrollable.ensureVisible(...)` und ist damit unabhaengig von variablen Positionshoehen.
- Der Pfad bleibt client-only und veraendert keine Server-, API- oder Datenbankvertraege.

## Kompatibilitaetsbewertung

Robust behandelte Faelle:

- kein ausgewaehltes Angebot
- Angebot ohne Positionen
- Angebot ohne abgelehnte letzte Freigabeentscheidung
- Zielposition ohne aktuellen `BuildContext`

In diesen Faellen wird keine Warnung angezeigt oder die Aktion bleibt ohne Seiteneffekt.

## Bewusst offene Punkte

Diese Punkte bleiben ausserhalb von Task `3.1.43`:

- keine farbliche oder temporaere Hervorhebung der Zielposition nach dem Scroll
- kein Sprung zu weiteren betroffenen Positionen
- kein positionsbezogener Deep-Link aus API-Fehlern
- kein Backend-Feld fuer offene Nacharbeitspositionen
- kein automatischer Editor-Fokus auf die betroffene Position

## Naechste kleine Nacharbeitsluecke

Nach dem Scroll ist fuer Nutzer sichtbar, welche Position betroffen ist, aber sie bekommt kein visuelles Fokus-/Rework-Signal im Detailbereich. Die positionnahe Markierung existiert bereits im Editor-Kontext ueber die Freigabeentscheidung, im Angebotsdetail bleibt die Zielposition jedoch eine normale `ListTile`.

Kleinster sinnvoller Anschluss:

```text
Subtask 3.1.44.1: Zielposition nach Sprung im Angebotsdetail temporaer hervorheben
```

Vorgeschlagener Zuschnitt:

- client-only in `client/lib/pages/quotes_page.dart`
- State-Feld fuer hervorgehobene Positionsnummer, z. B. `_highlightedRejectedApprovalPosition`
- beim Klick auf `Zur ersten Position` Zielposition setzen und danach scrollen
- Positionszeile bei Treffer mit dezenter roter Hintergrundfarbe oder `ListTile.tileColor` markieren
- Hervorhebung nach kurzer Dauer wieder entfernen oder bis zum naechsten Sprung halten
- keine Backend-/API-Aenderung

## Verifikation dieses Leafs

Keine Runtime-Aenderung in diesem Leaf. Bestehende Verifikation aus `3.1.43.3` bleibt gueltig:

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen waren erfolgreich.
