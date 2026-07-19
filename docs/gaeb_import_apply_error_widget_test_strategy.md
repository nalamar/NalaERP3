# GAEB: Minimalstrategie fuer den Apply-Fehler-Widgettest

## Ziel

Subtask 3.1.65.2 definiert den kleinsten stabilen Widgettest fuer einen
serverseitig abgewiesenen Versuch, aus einem `reviewed` GAEB-Import eine
Draft-Quote zu erzeugen. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Zu pruefender Runtime-Vertrag

Im GAEB-Importdetail ist `Draft-Quote erzeugen` sichtbar, wenn:

- der Importstatus `reviewed` ist
- `quotes.write` vorhanden ist

Die Aktion oeffnet einen nicht dismissbaren Fortschrittsdialog und ruft
`applyQuoteImport(importId)` auf. Bei einer Exception:

- wird nur der Fortschrittsdialog geschlossen
- der bestehende Importdetaildialog bleibt geoeffnet
- der lokale Detailzustand wird nicht auf `applied` gesetzt
- `_load()` und der Erfolgsrefresh werden nicht ausgefuehrt
- `_quoteErrorMessage` zeigt eine `ApiException.message` in einer Snackbar

## Kleinste Fake-Erweiterung

Der `_FakeApiClient` erhaelt genau eine optionale Fehler-Map:

```dart
this.quoteImportApplyErrors = const {},

final Map<String, Object> quoteImportApplyErrors;
```

`applyQuoteImport` zeichnet den Versuch weiterhin zuerst auf und prueft dann
nur die konfigurierte Import-ID:

```dart
@override
Future<Map<String, dynamic>> applyQuoteImport(String importId) async {
  appliedQuoteImportIds.add(importId);
  final error = quoteImportApplyErrors[importId];
  if (error != null) throw error;
  final configured = quoteImportApplyResults[importId];
  // bestehendes Erfolgsverhalten unveraendert
}
```

Der Default `const {}` haelt alle bestehenden Tests unveraendert. Eine Map ist
einem globalen Fehlerfeld vorzuziehen, weil Erfolg und Fehler weiterhin pro
Import-ID deterministisch konfigurierbar bleiben.

## Testname und Position

Der neue Test steht unmittelbar hinter
`QuotesPage GAEB import apply exposes created draft quote`:

```text
QuotesPage GAEB import apply keeps reviewed state on failure
```

Damit liegen Erfolgs- und Fehlervertrag derselben Mutation direkt
nebeneinander. Der bestehende Erfolgstest wird nicht erweitert.

## Minimaler Testaufbau

### Berechtigungen

Genau:

```dart
permissions: const {'quotes.read', 'quotes.write'}
```

### Importlauf

Ein einzelner Import reicht:

- ID: `import-apply-failure-1`
- Dateiname: `apply-fehler-ausschreibung.x83`
- Status: `reviewed`
- Projekt: `project-1`
- eine akzeptierte Position in den Summary-Zaehlern
- kein `created_quote_id`
- leere Importpositionsliste

Der Import startet direkt auf `reviewed`; der vorgelagerte Review-Test wird
nicht wiederholt.

### Deterministischer Fehler

Nur `import-apply-failure-1` wird in `quoteImportApplyErrors` konfiguriert:

```dart
const ApiException(
  statusCode: 409,
  code: 'invalid_quote_import_status',
  message: 'Importlauf kann aktuell nicht angewendet werden',
)
```

Die strukturierte API-Meldung prueft den vorhandenen `ApiException`-Zweig von
`_quoteErrorMessage`, ohne von der Stringdarstellung einer generischen
Exception abzuhaengen.

## Testablauf

1. `QuotesPage` mit Projektfilter rendern und initiales Laden abwarten.
2. Importdetail ueber `Details` oeffnen.
3. Ausgangszustand `Status: reviewed` pruefen.
4. `Draft-Quote erzeugen` als sichtbar und `Quote öffnen` als verborgen
   pruefen.
5. `Draft-Quote erzeugen` antippen und `pumpAndSettle()` abwarten.
6. Apply-Aufruf, beendeten Fortschritt, erhaltenen Dialogzustand,
   Fehlerfeedback und Negativausgaenge pruefen.

## Stabile Assertions

### Mutationsaufruf

```dart
expect(api.appliedQuoteImportIds, const ['import-apply-failure-1']);
```

Die Aufzeichnung erfolgt vor dem konfigurierten Fehler und belegt genau einen
Versuch.

### Dialog- und Statusvertrag

- `find.byType(AlertDialog)` findet genau einen Dialog
- `apply-fehler-ausschreibung.x83` bleibt sichtbar
- `Status: reviewed` bleibt sichtbar
- `Draft-Quote erzeugen` bleibt sichtbar
- `Draft-Quote aus Importlauf wird erzeugt...` ist nicht mehr sichtbar

Die Assertion auf genau einen `AlertDialog` belegt, dass der
Fortschrittsdialog geschlossen und der Importdetaildialog erhalten ist.

### Fehler- und Negativvertrag

- `Importlauf kann aktuell nicht angewendet werden` ist genau einmal sichtbar
- `Erzeugte Quote:` ist nicht sichtbar
- `Die Quote wurde erzeugt und kann jetzt geöffnet werden.` ist nicht sichtbar
- `Quote öffnen` ist nicht sichtbar
- `Draft-Quote wurde aus dem Importlauf erzeugt` ist nicht sichtbar
- keine Quote-Navigation und keine weitere Mutation wird ausgefuehrt

Nicht auf Snackbar-Dauer, Animationsframes oder den gesamten Dialogtext
asserten.

## Bewusste Abgrenzung

Subtask 3.1.65.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- eine optionale Apply-Fehler-Map im Fake
- eine import-ID-spezifische Abzweigung in `applyQuoteImport`
- genau einen Widgettest fuer den Fehlerausgang

Nicht Teil des Scopes:

- Aenderung an `QuotesPage` oder `ApiClient`
- Backend-, API-, Datenbank- oder Permission-Aenderung
- Review-Fehler- oder Berechtigungsnegativtest
- Retry-, Recovery- oder Dialog-Redesign
- Positions-, Preis-, Material-, Kalkulations- oder KI-Logik

## Verifikation des Folgeleafs

Nach der Implementierung genuegen:

```powershell
cd client
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import apply keeps reviewed state on failure"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Zusaetzlich `git diff --check` ausfuehren. Bereits bekannte, sachfremde
Harness-Warnungen werden dokumentiert, aber nicht in diesem Leaf bereinigt.

## Ergebnis

3.1.65.2 ist abgeschlossen. Subtask 3.1.65.3 erweitert ausschliesslich den
bestehenden Fake um die import-ID-spezifische Apply-Fehlerkonfiguration und
implementiert genau den beschriebenen Fehler-Widgettest.
