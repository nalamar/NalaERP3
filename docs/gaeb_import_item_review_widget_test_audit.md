# GAEB: Abschlussaudit des Positionsreview-Mutations-Widgettests

## Ziel

Subtask 3.1.60.4 auditiert den in 3.1.60.3 ergaenzten Widgettest fuer die
vorhandene Reviewmutation einer GAEB-Importposition. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Audit-Ergebnis

Die Umsetzung erfuellt das in
`docs/gaeb_import_item_review_widget_test_strategy.md` definierte Minimalziel:

- genau eine Mutation-Aufzeichnungsliste im bestehenden Fake
- ein Override fuer `updateQuoteImportItemReview(...)`
- genau ein separater Positionsreview-Widgettest
- Weitergabe von Import-ID und Item-ID
- Wechsel des Reviewstatus von `pending` auf `accepted`
- Trim-Normalisierung der eingegebenen Reviewnotiz
- Aktualisierung des weiterhin offenen Positionsdetaildialogs
- Pruefung der Erfolgssnackbar
- getrenntes Schliessen von Positions- und Importdialog

Runtime, `ApiClient`, Backend, Datenbank und Berechtigungsvertrag wurden nicht
veraendert.

## Fake-Vertrag

`updatedQuoteImportItemReviews` zeichnet den vollstaendigen Mutationsvertrag
mit `import_id`, `item_id`, `review_status` und `review_note` auf. Der Override
liest das Ausgangsdetail ueber beide IDs und gibt eine neue Map mit den
aktualisierten Reviewfeldern zurueck. Die konfigurierte Ausgangsmap wird nicht
mutiert.

Der Test belegt damit sowohl die korrekte ID-Weitergabe als auch die
zustandsgebundene Aktualisierung des Positionsdialogs.

## Normalisierung und sichtbares Ergebnis

Der Test gibt `  Fachlich geprueft  ` ein. Der aufgezeichnete Payload enthaelt
`Fachlich geprueft`. Damit ist die Trim-Normalisierung am Dialogabschluss
abgesichert.

Die Fake-Antwort wird danach unmittelbar im offenen Positionsdetail gerendert:

- `Review-Status: accepted`
- `Review-Notiz: Fachlich geprueft`
- `Review-Entscheidung wurde gespeichert`

Es wird kein Reload und kein zusaetzlicher Listenvertrag vorausgesetzt.

## Berechtigungs- und Scope-Grenze

Der Test verwendet nur `quotes.read` und `quotes.write`. Der Importstatus
`uploaded` und die leere Quote-Verknuepfung halten Importfreigabe,
Draft-Quote-Erzeugung und Quote-Navigation ausserhalb des Testpfads.

Nicht Bestandteil dieses Tasks sind:

- Review-Fehler- oder Validierungspfade
- Importfreigabe oder Quote-Erzeugung
- Quote-Navigation
- Backend-, API-, Datenbank- oder Permission-Aenderungen
- KI-Automatisierung

## Verifikation

Im Verzeichnis `client/` erneut ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage GAEB import item review forwards normalized decision"
```

Ergebnis: Der gezielte Widgettest ist erfolgreich.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bekannten allgemeinen
Harness-Hinweise zum ungenutzten `purchase_orders_page.dart`-Import und zum nie
gesetzten optionalen Fakeparameter `convertedQuote` bleiben unveraendert.

## Abschlussentscheidung

Task 3.1.60 ist fachlich und technisch abgeschlossen. Die vorhandene
Positionsreview-Mutation ist mit einem engen, deterministischen Widgettest fuer
Payload-Normalisierung, lokale Detailaktualisierung und Erfolgsfeedback
abgesichert. Der naechste Leaf soll den kleinsten fachlich wertvollen
Folgeausbau neu inventarisieren.
