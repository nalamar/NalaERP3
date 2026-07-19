# GAEB: Minimalstrategie fuer den Draft-Quote-Erzeugungs-Widgettest

## Ziel

Subtask 3.1.62.2 definiert den kleinsten stabilen Widgettest fuer die
vorhandene Draft-Quote-Erzeugung aus einem freigegebenen GAEB-Importlauf.
Dieser Leaf aendert weder Runtime- noch Testcode.

## Bestehender Vertrag

Mit `quotes.write` und Importstatus `reviewed` zeigt der Importdetaildialog
`Draft-Quote erzeugen`. Die Aktion ruft auf:

```text
applyQuoteImport(importId)
```

Die Antwort besitzt zwei Top-Level-Felder:

```text
import: aktualisierter Importlauf
quote: erzeugte Draft-Quote
```

Danach setzt die Runtime das zurueckgegebene Importdetail, laedt Detail und
Positionen erneut, aktualisiert die Angebotsseite und zeigt eine Snackbar mit
der Angebotsnummer. `created_quote_id` schaltet `Quote öffnen` frei.

Runtime, `ApiClient` und Serververtrag sind dafuer bereits vollstaendig.

## Fake-Antwortvertrag

Der `_FakeApiClient` erhaelt einen konfigurierbaren Ergebnisindex:

```text
quoteImportApplyResults: Map<Import-ID, Apply-Antwort>
```

Ausserdem zeichnet er angewendete IDs auf:

```text
appliedQuoteImportIds = <String>[]
```

Der Override von `applyQuoteImport(importId)`:

1. zeichnet die Import-ID in `appliedQuoteImportIds` auf
2. liefert `quoteImportApplyResults[importId]`
3. verwendet nur fuer neutrale Fremdaufrufe einen kleinen Fallback mit
   `status: applied`

Die konfigurierte Apply-Antwort wird nicht mutiert.

## Zustandsgebundener Detailrefresh

`getQuoteImport(id)` prueft in dieser Reihenfolge:

1. ist die ID in `appliedQuoteImportIds`, wird das `import`-Objekt aus
   `quoteImportApplyResults[id]` als neue Map geliefert
2. andernfalls gilt weiterhin der vorhandene Reviewzustand aus
   `reviewedQuoteImportIds`
3. andernfalls wird das konfigurierte Ausgangsdetail geliefert

Die Apply-Pruefung muss vor der Review-Pruefung stehen. So kann ein Import,
der zuvor im Fake freigegeben wurde, nach Apply nicht wieder auf `reviewed`
zurueckfallen.

`listQuoteImportItems` bleibt unveraendert. `listQuotes` darf fuer diesen Test
leer bleiben: Die Runtime-Ausfuehrung von `_load()` wird durch das erfolgreiche
Settle belegt, aber der Test behauptet keine Listenposition oder Auswahl der
erzeugten Quote.

## Separater Test

Neuer Testname:

```text
QuotesPage GAEB import apply exposes created draft quote
```

Der Test verwendet:

- `_prepareLargeViewport`
- Berechtigungen `quotes.read` und `quotes.write`
- Projektfilter `project-1`
- genau einen Import `import-apply-1`
- Ausgangsstatus `reviewed`
- genau ein akzeptiertes und kein offenes Item
- eine leere Positionsliste
- Apply-Importstatus `applied`
- `created_quote_id: quote-gaeb-1`
- Quote-ID `quote-gaeb-1`
- Angebotsnummer `ANG-GAEB-0001`
- Quotestatus `draft`

Ausgangsdetail und Apply-Import verwenden denselben eindeutigen Dateinamen
`apply-ausschreibung.x83`.

## Testablauf

1. Seite mit Projektfilter laden.
2. Importdialog ueber den eindeutigen `Details`-Button oeffnen.
3. `Status: reviewed` und `Draft-Quote erzeugen` pruefen.
4. Sicherstellen, dass `Quote öffnen` noch nicht sichtbar ist.
5. `Draft-Quote erzeugen` antippen.
6. Mit `pumpAndSettle()` Mutation, Fortschrittsroute, Detail-/Positionsrefresh
   und `_load()` abschliessen lassen.
7. Exakt `['import-apply-1']` in `appliedQuoteImportIds` pruefen.
8. Im weiterhin offenen Importdetaildialog pruefen:
   - `Status: applied`
   - `Erzeugte Quote: quote-gaeb-1`
   - der Hinweis, dass die Quote geoeffnet werden kann
   - `Draft-Quote erzeugen` ist nicht mehr sichtbar
   - `Quote öffnen` ist sichtbar
   - `Draft-Quote ANG-GAEB-0001 wurde aus dem Importlauf erzeugt` ist sichtbar
9. Importdialog ueber `Schließen` beenden und bestaetigen, dass kein
   `AlertDialog` verbleibt.

Der Test tippt `Quote öffnen` nicht an.

## Fortschritts- und Finder-Grenzen

Wie bei der Importlauf-Freigabe kann der synchrone Fake die Fortschrittsroute
zwischen zwei Testframes oeffnen und schliessen. `pumpAndSettle()` begleitet
den gesamten Ablauf stabil. Es wird weder der kurzlebige Fortschrittstext noch
eine starre Anzahl von Dialogen geprueft.

- `find.widgetWithText(TextButton, 'Details')` fuer den Importdialog
- `find.widgetWithText(FilledButton, 'Draft-Quote erzeugen')` fuer Apply
- `find.widgetWithText(FilledButton, 'Quote öffnen')` nur fuer die sichtbare
  Folgeaktion
- `find.widgetWithText(TextButton, 'Schließen')` fuer den Importdialog

## Implementierungsgrenze

Subtask 3.1.62.3 aendert ausschliesslich
`client/test/sales_order_context_pages_test.dart`:

- einen konfigurierbaren Apply-Ergebnisindex
- eine Apply-Aufzeichnungsliste
- einen Fake-Override fuer `applyQuoteImport`
- die zustandsabhaengige Erweiterung von `getQuoteImport`
- genau einen separaten Widgettest

`client/lib/pages/quotes_page.dart` bleibt unveraendert, sofern kein bislang
unsichtbarer Runtime-Defekt auftritt.

## Verifikation

Im Verzeichnis `client/`:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import apply exposes created draft quote"
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Die zwei bekannten allgemeinen Harness-Hinweise bleiben ausserhalb, solange
keine neue Warnung entsteht.

## Nicht-Ziele

- keine Runtime-Aenderung
- keine Navigation zur erzeugten Quote
- kein Apply-Fehler- oder Berechtigungsnegativtest
- keine Angebotspositions-, Preis- oder Materialpruefung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine KI-Automatisierung

## Ergebnis

3.1.62.2 ist abgeschlossen. Subtask 3.1.62.3 erweitert nur den bestehenden
Fake-Zustand und implementiert genau den definierten Draft-Quote-Erzeugungs-
Widgettest.
