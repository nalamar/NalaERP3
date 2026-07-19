# GAEB Approval Rework Editor Focus Strategy

## Scope

- Leaf: `3.1.46.2`
- Ziel: Exakten technischen Zuschnitt fuer den Editor-Fokus einer Nacharbeitsposition festlegen.
- Basis: `docs/gaeb_approval_rework_editor_context_inventory.md`
- Umsetzung soll im naechsten Leaf client-only in `client/lib/pages/quotes_page.dart` erfolgen.

## Zielverhalten

Wenn eine Angebotsdetail-Position wegen `latest_approval_decision.status == rejected`
Nacharbeit zeigt, soll der Nutzer von dieser Detailposition direkt in den Editor
springen koennen.

Erwartetes Verhalten:

1. Detail-Positionszeile zeigt bei Ablehnung einen kleinen Button `Bearbeiten`.
2. Klick oeffnet den bestehenden `_QuoteEditorDialog`.
3. Der Dialog scrollt nach dem ersten Rendern zur betroffenen Position.
4. Die Editor-Position wird dezent hervorgehoben.
5. Bestehende Nacharbeitsinfos in `_QuoteItemRow` bleiben unveraendert sichtbar.

## Zielanker

Der Fokus soll zweistufig arbeiten:

```text
initialFocusItemId bevorzugen
initialFocusPosition als Fallback
```

Begruendung:

- `item['id']` ist stabiler als die sichtbare Reihenfolge.
- `index + 1` passt als Fallback zur bestehenden Detailanzeige.
- Neue Backend- oder API-Felder sind nicht erforderlich.

## Detailseite

### `_openEditDialog`

Bestehende Signatur:

```text
Future<void> _openEditDialog() async
```

Zielsignatur:

```text
Future<void> _openEditDialog({
  String? initialFocusItemId,
  int? initialFocusPosition,
}) async
```

Beim normalen Header-Button bleibt der Aufruf ohne Parameter.

Beim Nacharbeitsbutton in der Positionszeile:

```text
_openEditDialog(
  initialFocusItemId: (item['id'] ?? '').toString(),
  initialFocusPosition: index + 1,
)
```

### Button in Detail-Positionszeile

Nur anzeigen, wenn:

- `canWrite == true`
- `selectedStatus == 'draft'`
- `_quoteItemRejectedApprovalDecisionSummary(item).isNotEmpty`

Position:

- am besten im `trailing` als `Wrap` oder `Column`, damit Zeilensumme und Aktion
  zusammen sichtbar bleiben
- Label `Bearbeiten`
- Icon optional `Icons.edit_rounded`

Bewertung:

Der Button soll nicht bei versendeten/angenommenen Angeboten erscheinen, weil
die bestehende Bearbeiten-Aktion im Header ebenfalls nur fuer `draft` sichtbar ist.

## `_QuoteEditorDialog`

### Konstruktorfelder

Neue optionale Felder:

```text
final String? initialFocusItemId;
final int? initialFocusPosition;
```

Konstruktor:

```text
const _QuoteEditorDialog({
  required this.api,
  this.initial,
  this.initialFilters,
  this.initialFocusItemId,
  this.initialFocusPosition,
});
```

## `_QuoteEditorDialogState`

### State

Neue Felder:

```text
final Map<String, GlobalKey> _itemFocusKeys = {};
final Map<int, GlobalKey> _positionFocusKeys = {};
String? _highlightedFocusItemId;
int? _highlightedFocusPosition;
bool _initialFocusScheduled = false;
```

Empfohlene Helper:

```text
GlobalKey _itemFocusKey(_QuoteItemDraft item, int position)
bool _isFocusedItem(_QuoteItemDraft item, int position)
void _scheduleInitialFocus()
void _focusInitialItem()
```

### Key-Auswahl

`_itemFocusKey(...)` soll:

- bei nicht leerer `item.id` `_itemFocusKeys[item.id]` verwenden
- sonst `_positionFocusKeys[position]` verwenden

Position bleibt 1-basiert.

### Zielsuche

`_focusInitialItem()` soll:

1. `initialFocusItemId?.trim()` lesen.
2. Falls vorhanden, passendes `_items`-Element per `item.id.trim()` suchen.
3. Falls nicht gefunden, `initialFocusPosition` pruefen.
4. Position validieren (`>= 1`, `<= _items.length`).
5. passenden Key holen.
6. bei vorhandenem `currentContext`:
   - Highlight-State setzen
   - `Scrollable.ensureVisible(...)` ausfuehren

Empfohlene Scrollparameter:

```text
duration: Duration(milliseconds: 300)
curve: Curves.easeInOut
alignment: 0.08
```

### Zeitpunkt

In `initState()`:

```text
WidgetsBinding.instance.addPostFrameCallback((_) {
  if (!mounted) return;
  _focusInitialItem();
});
```

`_initialFocusScheduled` ist optional, aber sinnvoll, falls der Fokus spaeter
bei Draft-Ersetzungen erneut geplant werden soll. Fuer den ersten Implementierungsschritt
reicht ein einmaliger Post-Frame-Callback.

### Highlight

Empfehlung:

- Highlight im Editor nicht automatisch nach drei Sekunden loeschen.
- Stattdessen bis zum Schliessen des Dialogs oder bis zu einer spaeteren
  expliziten Fokusaktion halten.

Begruendung:

- Der Editor enthaelt viele Controls.
- Nutzer sollen die Position nach dem Scroll nicht sofort wieder verlieren.
- Es gibt aktuell keine weitere Fokusaktion im Dialog.

## `_QuoteItemRow`

Neue optionale Property:

```text
final bool highlighted;
```

Default:

```text
this.highlighted = false
```

Card-Dekoration:

- bestehende `Card(margin: EdgeInsets.zero, child: ...)` kann beibehalten werden
- bei `highlighted == true`:
  - `color: Colors.red.shade50`
  - optional `shape` mit rotem Rand

Kleinster stabiler Schritt:

```text
Card(
  margin: EdgeInsets.zero,
  color: widget.highlighted ? Colors.red.shade50 : null,
  child: ...
)
```

Keine Aenderung an den fachlichen Nacharbeitsbloecken der Row.

## Wrapper um Row

Beim Rendern in der `for`-Schleife:

```text
final position = i + 1;
final item = _items[i];
KeyedSubtree(
  key: _itemFocusKey(item, position),
  child: _QuoteItemRow(
    key: ValueKey(item),
    highlighted: _isFocusedItem(item, position),
    ...
  ),
)
```

Die bestehende `ValueKey(item)` bleibt erhalten.

## Fallback- und Fehlerfaelle

Kein Effekt, wenn:

- Dialog nicht im Edit-Modus ist
- kein Zielparameter uebergeben wurde
- Ziel-ID nicht gefunden wird und Position ungueltig ist
- Ziel-Key noch keinen `currentContext` hat
- Angebot keine Positionen besitzt

In diesen Faellen oeffnet sich der Editor normal.

## Abgrenzung

Nicht im naechsten Implementierungsleaf:

- keine automatische Aenderung von Preis, Material oder Zielpreis
- keine automatische erneute Freigabeanforderung
- keine Freigabehistorie im Angebotsdetail
- keine Nacharbeitsqueue
- keine Mehrpositionsnavigation
- keine Backend-/API-Aenderung

## Verifikation nach Implementierung

Nach Runtime-Code-Aenderung:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Tests sind nicht erforderlich, solange keine Backend- oder API-Dateien
geaendert werden.

## Entscheidung

Subtask `3.1.46.2` ist ein technischer Zuschnitt ohne Runtime-Code-Aenderung.

Naechster Leaf:

```text
Subtask 3.1.46.3: Editor-Fokus fuer Nacharbeitsposition im Client implementieren
```
