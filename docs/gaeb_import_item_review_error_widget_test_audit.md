# GAEB: Abschlussaudit des Einzelpositionsreview-Fehlerwidgettests

## Ziel

Subtask 3.1.67.4 auditiert die Umsetzung aus 3.1.67.3 gegen die festgelegte
Strategie. Dieser Leaf nimmt keine weitere Runtime- oder Testaenderung vor.

## Auditumfang

Geprueft wurden ausschliesslich:

- die additive Erweiterung von `_FakeApiClient`
- die Reihenfolge von Versuch, Fehlerpruefung und Erfolgseintrag
- das isolierte Fehlerszenario im Widgettest
- die positiven und negativen Zustandsassertions
- die gezielte Test- und Analyseausgabe

Der kumulative, bereits vor diesem Leaf veraenderte Test-Harness wurde nicht
als neuer Gesamtumfang bewertet.

## Fake-Vertrag

Der Fake erfuellt den dokumentierten Vertrag:

- `quoteImportItemReviewErrors` ist optional und standardmaessig leer
- der kombinierte Schluessel `$importId/$itemId` bindet den Fehler an genau
  eine Importposition
- ein Payload wird einmal aus Import-ID, Item-ID, Status und Notiz aufgebaut
- `attemptedQuoteImportItemReviews` wird vor der Fehlerpruefung befuellt
- ein konfigurierter Fehler wird vor dem Erfolgseintrag geworfen
- `updatedQuoteImportItemReviews` enthaelt weiterhin nur erfolgreiche
  Mutationen
- der bestehende Erfolgsrueckgabewert bleibt unveraendert

Damit kann der Test zwischen versuchter und persistierter Mutation
deterministisch unterscheiden.

## Widgettest-Vertrag

Der Test
`QuotesPage GAEB import item review keeps pending detail on failure` verwendet
genau den vorgesehenen Kontext:

- `quotes.read` und `quotes.write`
- einen `parsed` Import
- eine `pending` Position mit leerer Review-Notiz
- Auswahl `accepted`
- Eingabe einer absichtlich gepolsterten Notiz
- eine positionsspezifische 409-`ApiException`

Die Interaktion laeuft ueber die sichtbaren Aktionen `Details`, die konkrete
Position, `Review setzen` und `Speichern`. Es gibt keine Runtime-Hilfseinstiege
oder direkte Zustandsmanipulation.

## Assertions

Der Test belegt positiv:

- exakt das getrimmte Versuchspayload
- keine erfolgreiche Mutation im Fake
- zwei erhaltene aeussere Dialoge fuer Import- und Positionsdetail
- weiterhin `Review-Status: pending`
- weiterhin sichtbare Aktion `Review setzen`
- die strukturierte API-Meldung

Er belegt negativ:

- der innere Dialog `Review-Entscheidung` ist geschlossen
- `Review-Status: accepted` erscheint nicht
- die eingegebene Review-Notiz erscheint nicht im Detail
- die Erfolgssnackbar erscheint nicht

Die bei der ersten Testausfuehrung festgestellte Dialoghierarchie wurde korrekt
praezisiert: Importdetail und Positionsdetail bleiben geoeffnet; nur der
Entscheidungsdialog schliesst. Diese Anpassung aendert den fachlichen Vertrag
nicht.

## Verifikationsnachweis

Ausgefuehrt wurden:

```text
dart format test/sales_order_context_pages_test.dart
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import item review keeps pending detail on failure"
flutter analyze test/sales_order_context_pages_test.dart
flutter analyze
git diff --check
```

Ergebnis:

- Formatierung ohne weitere Aenderung
- gezielter Widgettest bestanden
- gezielte Dateianalyse nur mit zwei bereits bekannten Harness-Warnungen:
  unbenutzter `purchase_orders_page.dart`-Import und nie gesetzter optionaler
  Parameter `convertedQuote`
- Gesamtanalyse weiterhin rot durch bestehende, ausserhalb dieses Leaves
  liegende fehlende Bank-/HR-Methoden in `ApiClient`, einen
  Navigations-Override sowie bekannte Hinweise
- `git diff --check` ohne Whitespacefehler; nur Zeilenenden-Warnungen

Die roten Gesamtanalysebefunde wurden durch diesen Testleaf weder verursacht
noch veraendert.

## Scope-Abgleich

Nicht geaendert wurden:

- `QuotesPage`
- `ApiClient`
- Server, API und Datenbank
- Berechtigungsmodell
- Importlauf-Freigabe, Apply und Navigation
- Mapping-, Kalkulations- oder KI-Logik

## Abschlussentscheidung

Die Implementierung entspricht der Strategie und schliesst die unmittelbare
Mutationsfehlerluecke des Einzelpositionsreviews. Task 3.1.67 ist fachlich
abgeschlossen. Der naechste Leaf soll erneut nur inventarisieren, welcher
kleinste fachlich wertvolle GAEB-Ausbau nach der nun vollstaendig abgesicherten
Client-Fehlerkette folgt.

## Ergebnis

3.1.67.4 ist abgeschlossen. Fake-Vertrag, Widgetzustand und Fehlerfeedback sind
durch einen gruenen gezielten Test belegt; es besteht kein Anlass fuer eine
Runtime-, Backend- oder API-Erweiterung in diesem Task.
