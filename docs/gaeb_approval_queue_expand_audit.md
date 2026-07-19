# GAEB-Freigabe: Abschlussaudit der erweiterbaren Approval-Queue

## Ziel

Subtask 3.1.54.4 auditiert die in 3.1.54.3 implementierte Ein- und
Ausklappfunktion der Freigabeanforderungs-Karte. Dieser Leaf aendert weder
Runtime- noch Testcode.

## Audit-Ergebnis

Die Implementierung erfuellt das in
`docs/gaeb_approval_queue_expand_strategy.md` definierte Minimalziel:

- genau ein lokaler Zustand `_approvalRequestsExpanded`
- kompakte Standardansicht mit maximal drei Eintraegen
- erweiterte Ansicht mit allen bereits geladenen `_approvalRequestItems`
- stabile Umschaltlabels `Alle anzeigen` und `Weniger anzeigen`
- Umschaltaktion nur bei mehr als drei Eintraegen
- Ruecksetzung bei vollstaendigem `_load()`
- Zustandserhalt bei reinem Queue-Refresh
- Normalisierung auf kompakt bei hoechstens drei neuen Ergebnissen

Backend, `ApiClient`, Readmodel, Berechtigungen und Queue-Mutationen wurden
nicht erweitert.

## Vertrags- und Zustandspruefung

Die bestehende Reihenfolge der Queue-Eintraege bleibt erhalten. Genehmigen,
Ablehnen und Oeffnen werden weiterhin durch dieselben ListTiles und Callbacks
bereitgestellt; lediglich die Quelle der sichtbaren Teilmenge wechselt zwischen
`take(3)` und der bereits geladenen Gesamtliste.

Das Ruecksetzverhalten ist konsistent:

- Filterwechsel und vollstaendiger Seiten-Refresh starten kompakt.
- Ein manueller Queue-Refresh oder eine Queue-Entscheidung klappt eine laufende
  Arbeitsansicht nicht unnoetig ein.
- Sinkt die Ergebnismenge auf drei oder weniger, bleibt kein unsichtbarer
  Expand-State fuer einen spaeter wieder wachsenden Datensatz bestehen.
- Im Fehlerpfad werden vorhandene Liste und Anzeigezustand nicht ueberschrieben.

## Testabdeckung

Der neue Widgettest verwendet vier deterministische Queue-Eintraege und nur
`quotes.read`. Er prueft:

- Eintraege eins bis drei sind anfangs sichtbar.
- Eintrag vier ist anfangs nicht sichtbar.
- `Alle anzeigen` macht Eintrag vier sichtbar.
- `Weniger anzeigen` blendet Eintrag vier wieder aus.
- Das jeweils erwartete Gegenlabel ist sichtbar.

Der bereits vorhandene Genehmigen-Dialogtest wurde gemeinsam erneut
ausgefuehrt. Damit ist abgesichert, dass der geaenderte Sichtbarkeitsbau den
bestehenden Queue-Entscheidungspfad nicht beeintraechtigt.

## Verifikation

Im Verzeichnis `client/` ausgefuehrt:

```text
flutter test test/sales_order_context_pages_test.dart --plain-name "QuotesPage approval queue"
```

Ergebnis: beide passenden Widgettests bestanden.

```text
flutter analyze test/sales_order_context_pages_test.dart lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis: keine Fehler und keine neue Warnung. Die zwei bereits dokumentierten
Hinweise zum ungenutzten `purchase_orders_page.dart`-Import und nie gesetzten
`convertedQuote`-Fakeparameter bleiben allgemeine Test-Harness-Themen.

## Bewusste Grenze

Die Erweiterung rendert auf Anforderung alle bereits geladenen Eintraege. Fuer
sehr grosse Queues ist spaeter eine dedizierte Arbeitsliste mit serverseitiger
Pagination sinnvoll. Das aktuelle Readmodel liefert jedoch bereits die
Gesamtliste; Pagination jetzt nachzuziehen waere ein neuer Backend-/API-Block
und kein notwendiger Haertungsschritt fuer diese kleine Erreichbarkeitsluecke.

## Nicht-Ziele

- keine weitere UI- oder Testimplementierung
- keine dedizierte Approval-Arbeitsliste
- keine Pagination, Filter oder Sortierung
- keine Massenaktion
- keine Cockpit-, KPI-, SLA- oder Eskalationslogik
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Abschlussentscheidung

Task 3.1.54 ist fachlich und technisch abgeschlossen. Innerhalb der kleinen
Ein-/Ausklappfunktion bleibt kein weiterer Haertungsschritt mit gutem Signal.
Der naechste Leaf soll als neuer Folgeabschnitt inventarisieren, welcher
GAEB-/Approval-Ausbau nach der nun erreichbaren und getesteten Queue den
hoechsten Nutzen bei kleinster Vertragserweiterung bietet.
