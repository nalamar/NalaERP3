# GAEB Approval Rework Editor Context Inventory

## Scope

- Leaf: `3.1.46.1`
- Ziel: Fachlich und technisch inventarisieren, wie eine abgelehnte Angebotsdetail-Position spaeter im Editor-Kontext vorselektiert oder sichtbar gemacht werden kann.
- Fokus bleibt client-only in `client/lib/pages/quotes_page.dart`.

## Ausgangspunkt

Der Nacharbeits-Kontext ist im Angebotsdetail mittlerweile sichtbar:

- quote-weite Nacharbeitswarnung
- betroffene Positionsnummern
- Sprung zur ersten Nacharbeitsposition
- kurzlebige Hervorhebung der Zielposition
- Ablehnungsgrund und optionaler Entscheidungskommentar in der Positionszeile

Die naechste Luecke liegt beim Uebergang zur eigentlichen Nacharbeit:

```text
Detailposition -> Bearbeitungsdialog -> richtige Editor-Position sichtbar
```

## Technischer Befund

### Detailseite

- `_openEditDialog()` oeffnet `_QuoteEditorDialog(api: widget.api, initial: selected)`.
- Es wird aktuell keine Zielposition, Item-ID oder Highlight-Information an den Dialog uebergeben.
- Die Detail-Positionsliste kennt `position = index + 1`.
- Die Detail-Positionsliste kann bereits `latest_approval_decision.status == rejected` erkennen.
- Fuer Detailpositionen existieren `GlobalKey`s in `_quoteItemJumpKeys`, aber nur fuer die Detailansicht.

### Editor-Dialog

- `_QuoteEditorDialog` akzeptiert aktuell:
  - `api`
  - `initial`
  - `initialFilters`
- `_QuoteEditorDialogState` baut `_items` aus `initial['items']`.
- Der Dialog nutzt in `build(...)` einen `SingleChildScrollView`.
- Es gibt keinen `ScrollController` im Dialog.
- Es gibt keine Positions-`GlobalKey`s im Dialog.
- Positionen werden ueber eine `for`-Schleife gerendert:

```text
for (var i = 0; i < _items.length; i++) ...[
  _QuoteItemRow(
    key: ValueKey(_items[i]),
    item: _items[i],
    index: i,
    ...
  ),
]
```

- `_QuoteItemRow` ist ein eigenes `StatefulWidget`.
- `_QuoteItemRow` rendert jede Position als `Card`.
- Die Zeile zeigt bereits `latestApprovalDecision`.
- Bei `latestApprovalDecision.requiresRework` zeigt die Row bereits:

```text
Nacharbeit erforderlich
Preis, Material oder Zielpreis anpassen und erneut Freigabe anfordern.
```

## Fachliche Bewertung

Die Editor-Zeile besitzt die fachliche Nacharbeitsinformation bereits. Was fehlt,
ist nur die Uebergabe und Sichtbarmachung der Zielposition beim Oeffnen des
Dialogs.

Die kleinste Loesung sollte deshalb nicht neue Fachlogik einfuehren, sondern:

- Zielposition oder Item-ID aus dem Angebotsdetail an den Editor uebergeben
- Editor-Zeile im Dialog adressierbar machen
- nach erstem Rendern per `Scrollable.ensureVisible(...)` zur Position springen
- Position kurz hervorheben

## Zielidentifikation

Moegliche Zielanker:

### Option A: 1-basierte Positionsnummer

Vorteile:

- passt zur Detailansicht und Warnbox
- einfach aus `index + 1`
- keine zusaetzliche API-Annahme

Risiken:

- bei spaeterem Reordering weniger stabil

### Option B: Quote-Item-ID

Vorteile:

- stabiler als Position
- `_QuoteItemDraft.id` existiert im Editor

Risiken:

- neue Uebergabelogik muss aus Detailposition sicher die ID lesen
- fuer neu angelegte Positionen leer

Empfehlung fuer den kleinsten naechsten Schritt:

```text
initialFocusItemId bevorzugen, fallback initialFocusPosition
```

Damit bleibt der Pfad robust, ohne Backend- oder API-Aenderung.

## Technischer Zuschnitt fuer naechsten Schritt

Kleiner client-only Umsetzungspfad:

1. `_openEditDialog(...)` optional um Zielparameter erweitern.
2. In der Detail-Positionszeile bei abgelehnter Position einen kleinen Button
   `Bearbeiten` anzeigen.
3. Button ruft `_openEditDialog(initialFocusItemId: item['id'], initialFocusPosition: index + 1)`.
4. `_QuoteEditorDialog` erhaelt optionale Felder:
   - `initialFocusItemId`
   - `initialFocusPosition`
5. `_QuoteEditorDialogState` erhaelt:
   - `Map<String, GlobalKey> _itemFocusKeys` oder `Map<int, GlobalKey> _itemFocusPositionKeys`
   - `_highlightedItemId` oder `_highlightedPosition`
6. Nach erstem Frame im Dialog:
   - Ziel-Key ermitteln
   - `Scrollable.ensureVisible(...)`
   - Zielposition kurz markieren
7. `_QuoteItemRow` erhaelt optional `highlighted` und nutzt eine dezente
   Hintergrund-/Rahmenfarbe der bestehenden Card.

## Nicht Teil des naechsten kleinen Schritts

- keine automatische Preis-/Materialaenderung
- keine automatische erneute Freigabeanforderung
- keine neue Nacharbeitsqueue
- keine Mehrpositionsnavigation im Editor
- keine Backend- oder API-Aenderung
- keine Umstellung der Dialogstruktur

## Risiken und Hinweise

- Der Dialog nutzt bereits `SingleChildScrollView`; `Scrollable.ensureVisible(...)`
  passt zur vorhandenen Struktur.
- `_QuoteItemRow` hat bereits `ValueKey(_items[i])`; zusaetzliche `GlobalKey`s
  sollten um die Row herum per `KeyedSubtree` oder Wrapper gesetzt werden, um
  die bestehende Widget-Identitaet nicht unnoetig zu stoeren.
- Highlighting sollte im Editor laenger als im Detail sichtbar sein oder bis zur
  ersten Nutzerinteraktion gehalten werden, da der Dialog viele controls enthaelt.
- Nach `_replaceDraftFromQuote(...)` koennen Draft-Objekte ersetzt werden; ein
  ID-basierter Fokus ist daher stabiler als Objektidentitaet.

## Entscheidung

Subtask `3.1.46.1` bleibt eine Inventur ohne Runtime-Code-Aenderung.

Naechster kleinster Leaf:

```text
Subtask 3.1.46.2: Editor-Fokus fuer Nacharbeitsposition technisch zuschneiden
```

Dieser Leaf soll den exakten Implementierungsplan fuer Parameter, Keys,
Scrollzeitpunkt und Highlighting festlegen, bevor Runtime-Code geaendert wird.
