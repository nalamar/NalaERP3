# GAEB-Freigabeentscheidungen: Sprungziel zur ersten Nacharbeitsposition-Strategie

## Ziel dieses Zuschnitts

Dieses Dokument schneidet den kleinsten technischen Client-Schritt fuer das
Sprungziel zu:

```text
Button in der Nacharbeitswarnung scrollt zur ersten betroffenen Position
```

Der Schritt bleibt client-only.

## 1. Technischer Befund

Die Quote-Detailansicht wird aktuell direkt in `_QuotesPageState.build(...)`
aufgebaut.

Relevante Struktur:

- rechter Detailbereich als `SingleChildScrollView`
- Kopf-Warnung oberhalb der Positionsliste
- Positionsliste als eingebettetes `ListView.separated`
- `ListView.separated` nutzt `NeverScrollableScrollPhysics`
- Positionsdaten kommen aus `selected['items']`
- betroffene Positionen werden bereits per
  `_quoteRejectedApprovalDecisionPositions(selected)` als `index + 1`
  abgeleitet

Bewertung:

Ein Sprung per `Scrollable.ensureVisible(...)` passt zur vorhandenen Struktur.
Ein eigener ScrollController mit Offset-Schaetzung ist nicht sinnvoll, weil die
Hoehe der Positionszeilen variieren kann.

## 2. Zielmodell

Die bestehende Nacharbeitswarnung erhaelt eine kleine Aktion:

```text
Zur ersten Position
```

Verhalten:

- Aktion ist nur sichtbar, wenn mindestens eine Nacharbeitsposition existiert.
- Ziel ist die erste Nummer aus `rejectedApprovalDecisionPositions`.
- Die Zielposition bekommt in der Positionsliste einen `GlobalKey`.
- Klick ruft `Scrollable.ensureVisible(...)` auf dem Zielkontext auf.
- Wenn kein Zielkontext vorhanden ist, passiert nichts.

## 3. State und Key-Verwaltung

Empfohlener kleiner State in `_QuotesPageState`:

```text
final Map<int, GlobalKey> _quoteItemJumpKeys = {};
```

Empfohlene Hilfsfunktion:

```text
GlobalKey _quoteItemJumpKey(int position)
```

Verhalten:

- nutzt `putIfAbsent`
- Key wird nach Positionsnummer `index + 1` verwaltet
- keine persistente Speicherung
- kein Backend-Bezug

Optionaler Cleanup:

- nicht zwingend im ersten Schritt
- bei sehr langen wechselnden Angeboten spaeter moeglich

Bewertung:

Die Map bleibt klein und lebt nur im Client-State. Da die Detailansicht nur ein
Angebot gleichzeitig anzeigt, ist die Komplexitaet vertretbar.

## 4. Scroll-Aktion

Empfohlene Hilfsfunktion:

```text
void _scrollToFirstRejectedApprovalDecisionPosition()
```

Logik:

- `final firstPosition = rejectedApprovalDecisionPositions.firstOrNull`
- Key holen
- `currentContext` pruefen
- `Scrollable.ensureVisible(...)`

Parameter:

```text
duration: Duration(milliseconds: 300)
curve: Curves.easeInOut
alignment: 0.08
```

Hinweis:

Dart ohne `firstOrNull`-Extension kann schlicht `isEmpty` pruefen und dann
`first` nutzen.

## 5. Einbauort in der Warnung

Die Aktion soll in der roten Warnbox stehen, unter Positionsnummern und vor dem
langen Hilfetext oder rechts neben der Warnungsueberschrift.

Kleinster stabiler Einbau:

```text
Text Titel
Text Betroffene Position(en)
TextButton.icon Zur ersten Position
Text Hilfetext
```

Begruendung:

- bleibt innerhalb der bestehenden Warnbox
- kein Layout-Umbau in Toolbar oder Header
- Aktion ist nur im Kontext offener Nacharbeit sichtbar

Icon:

```text
Icons.arrow_downward_rounded
```

## 6. Positionsliste

Die Zielposition soll mit dem Key um die vorhandene `ListTile` gewickelt werden:

```text
KeyedSubtree(
  key: _quoteItemJumpKey(index + 1),
  child: ListTile(...),
)
```

Nicht Teil des ersten Schritts:

- farbliche Hervorhebung der Zielposition
- Animation ausser Scroll
- Fokusmanagement
- Sprung zur naechsten betroffenen Position

## 7. Kompatibilitaet

Die Umsetzung bleibt kompatibel, wenn:

- `selected` null ist
- `items` leer ist
- keine Rejected-Decision vorhanden ist
- ein Zielkey noch keinen `currentContext` hat

In diesen Faellen wird keine Aktion angezeigt oder der Klick bleibt ohne Effekt.

## 8. Verifikation

Nach Umsetzung:

```text
dart format lib/pages/quotes_page.dart
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Tests sind nicht erforderlich, solange keine Backend- oder API-Dateien
geaendert werden.

## 9. Entscheidung

Subtask 3.1.43.2 ist abgeschlossen.

Naechster kleinster Implementierungsschritt:

```text
Subtask 3.1.43.3: Sprung zur ersten Nacharbeitsposition im Client implementieren
```

Der Schritt soll ausschliesslich `client/lib/pages/quotes_page.dart` betreffen.
