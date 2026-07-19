# GAEB-Zielpreis-Uebernahme: Ende-zu-Ende-Audit

## Scope

Dieses Audit bewertet den Stand nach den Leaves `3.1.31.3` und `3.1.31.4`.

Geprueft wurde der schmale Write-Pfad:

- Zielpreis-Uebernahme fuer genau eine Draft-Angebotsposition
- serverseitige Berechnung aus gespeicherter Kostenbasis und Zielmarge
- Positions- und Angebotssummenmutation
- persistierte Preisentscheidung `target_price_applied`
- Flutter-API und expliziter Button im Zielmargenblock

Nicht bewertet wurden bewusst ausgeschlossene Folgefunktionen wie Bulk,
Automatik, echter Freigabe-Workflow, Rollenmatrix, Kunden-/Projekt-Zielmargen,
Rabattlogik oder KI-Preisfindung.

## Ergebnis

Der End-to-End-Pfad ist fuer den Minimalumfang fachlich nutzbar:

1. `ApplyTargetUnitPriceForQuoteItem(...)` sperrt Quote und Position in einer
   Transaktion.
2. Historische Quotes und nicht-Draft-Quotes bleiben blockiert.
3. Ohne gespeicherte Preisentscheidung wird kein Zielpreis angewendet.
4. Der Zielpreis wird serverseitig aus `source_unit_price` der letzten
   Preisentscheidung und `quote_calculation_settings.target_margin_percent`
   berechnet.
5. Der angewendete `unit_price` wird auf zwei Nachkommastellen gerundet.
6. `net_amount`, `tax_amount` und Quote-Summen werden nach der Uebernahme neu
   berechnet.
7. `quote_item_price_decisions` erhaelt eine neue Entscheidung vom Typ
   `target_price_applied`.
8. Der Flutter-Client ruft
   `POST /api/v1/quotes/{quoteId}/items/{itemId}/apply-target-price` auf und
   ersetzt nach Erfolg den Editorstand durch die aktualisierte Quote.

Der Pfad bleibt explizit. Das Laden des Zielmargenankers mutiert keine Daten.

## Abgleich Gegen Strategie

| Bereich | Soll | Ist | Befund |
| --- | --- | --- | --- |
| Service | `ApplyTargetUnitPriceForQuoteItem(...)` | umgesetzt | ok |
| API | `POST /api/v1/quotes/{id}/items/{itemID}/apply-target-price` | umgesetzt | ok |
| Berechtigung | `quotes.write` | umgesetzt | ok |
| Guard Rails | Draft, aktuelle Version, Position, Preisentscheidung | umgesetzt | ok |
| Kostenbasis | letzte gespeicherte Preisentscheidung | umgesetzt | ok |
| Zielmarge | Settings-Wert mit bestehendem Fallback | umgesetzt | ok |
| Rundung | angewendeter Zielpreis auf zwei Nachkommastellen | umgesetzt | ok |
| Summen | Position und Quote nachziehen | umgesetzt | ok |
| Historie | `target_price_applied` persistieren | umgesetzt | ok |
| Client | Button im Zielmargenblock, kein Auto-Apply | umgesetzt | ok |

## Festgestellte Kanten

### 1. Zielanker Zeigt Ungerundeten Zielpreis

`TargetMarginAnchorForQuoteItem(...)` berechnet `target_unit_price` weiterhin
ohne Zwei-Nachkommastellen-Rundung. Die Uebernahme rundet dagegen den
angewendeten Preis. Dadurch kann der Anchor nach Apply bei Werten wie
`59.90 * 1.25 = 74.875` eine minimale Rundungsabweichung zeigen.

Empfehlung:

- kurzfristig akzeptabel, weil die Toleranz im Statuspfad greift
- spaeter Zielanker und Apply-Pfad auf dieselbe Waehrungsrundung ziehen

### 2. Wiederholtes Anwenden Erzeugt Weitere Entscheidungen

Ein erneuter Klick erzeugt eine weitere `target_price_applied`-Entscheidung.
Das ist auditierbar, aber noch nicht als No-op optimiert.

Empfehlung:

- spaeter entscheiden, ob identische Zielpreis-Uebernahmen als neue
  Entscheidung, als No-op oder mit separatem Hinweis behandelt werden sollen

### 3. Kostenbasis Folgt Der Letzten Entscheidung

Nach der Zielpreis-Uebernahme ist die neueste Entscheidung
`target_price_applied`. Der Margenanker nutzt weiterhin deren
`source_unit_price` als Kostenbasis. Das ist fuer den aktuellen Pfad korrekt,
weil die Kostenbasis im Snapshot erhalten bleibt.

Empfehlung:

- bei spaeteren Entscheidungsarten sicherstellen, dass jede Entscheidung ihre
  Kostenbasis eindeutig im Snapshot traegt

### 4. Berechtigungen Bleiben Schreiblastig

Der Apply-Endpunkt benoetigt korrekt `quotes.write`. Die read-only
Bewertungsanker nutzen ebenfalls weiterhin `quotes.write`, was aus frueheren
Audits bekannt ist.

Empfehlung:

- in einem separaten Rollen-/Rechte-Audit read-only Anker von echten
  Preis-Mutationen trennen

### 5. Kein Freigabezustand Nach Zielpreis-Uebernahme

Die Zielpreis-Uebernahme setzt keinen Freigabestatus und erzeugt keinen
Approval-Beleg. Das ist absichtlich, laesst aber weiterhin offen, wie
negative Margen, Zielabweichungen oder Sonderpreise freigegeben werden.

Empfehlung:

- naechster fachlicher Schreibpfad sollte nicht erneut eine Preisquelle sein,
  sondern ein kleines Freigabe-Zielmodell vorbereiten

## Folgepunkte

Priorisierte Folgepunkte nach diesem Audit:

1. Zielanker und Zielpreis-Apply auf identische Rundungslogik bringen.
2. No-op-Verhalten fuer wiederholte identische Zielpreis-Uebernahmen festlegen.
3. Read-only Bewertungsanker berechtigungsseitig von Write-Aktionen trennen.
4. Expliziten Zielmargenwert `0.00` in Settings fachlich entscheiden.
5. Settings-GET optional selbstheilend machen, falls `id='default'` fehlt.
6. Kleines Freigabe-Zielmodell fuer negative Marge oder Zielabweichung
   zuschneiden.

## Verifikation

Ausgefuehrte Pruefungen:

```text
go test ./internal/settings ./internal/quotes ./internal/http
dart analyze lib/api.dart lib/pages/quotes_page.dart lib/pages/settings_page.dart
```

Beide Pruefungen liefen ohne Befund.

Bekannter Rest:

```text
flutter analyze
```

bleibt global separat zu behandeln, weil fruehere Audits bereits fachfremde
Client-Luecken in Bank- und Employee-Bereichen dokumentiert haben.
