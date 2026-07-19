# GAEB-Preisentscheidungs-Historie:
# Minimalzielbild und technischer Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das Minimalzielbild fuer die read-only
Preisentscheidungs-Historie nach persistierter Preisentscheidung zu.

Der Fokus bleibt bewusst eng:

- gespeicherte Snapshots aus `quote_item_price_decisions` an genau einer
  Quote-Position sichtbar machen
- keine Historienbearbeitung, keine Marge, keinen Zuschlag, keinen Rabatt,
  keine Freigabe, keinen Bulk und keine Automatik einfuehren
- die Anzeige im bestehenden Quote-Editor halten

## 1. Ausgangspunkt

Der aktuelle Stand bietet bereits:

- explizite Primaerpreis-Uebernahme
- persistierten Snapshot in `quote_item_price_decisions`
- read-only Live-Transparenz gegen die aktuelle primaere Quelle

Die verbleibende kleine Luecke ist:

- gespeicherte Preisentscheidungen sind technisch vorhanden, aber fuer Nutzer
  noch nicht sichtbar

## 2. Fachliches Minimalziel

Das Minimalziel ist:

- eine read-only Liste der gespeicherten Preisentscheidungen fuer genau eine
  Quote-Position

Diese Liste soll nur beantworten:

- welche Entscheidung wurde gespeichert?
- wann wurde sie gespeichert?
- welche Quelle und Referenz lagen zugrunde?
- welcher Preis wurde uebernommen?

Sie soll bewusst noch nicht beantworten:

- welche Marge daraus folgt
- ob eine Freigabe noetig ist
- ob eine Entscheidung korrigiert oder kommentiert werden soll
- ob mehrere Positionen gemeinsam bewertet werden sollen

## 3. Zielbild im Backend

Das Backend soll einen engen read-only Pfad bereitstellen:

- `PriceDecisionHistoryForQuoteItem(ctx, quoteID, itemID) ([]PriceDecisionHistoryEntry, error)`

Der Pfad soll:

- Quote und Position validieren
- historische Quote-Versionen analog zu bestehenden positionsnahen
  Preisankern ablehnen
- im bestehenden Editorfluss bei Draft-Quotes bleiben
- nur Datensaetze aus `quote_item_price_decisions` fuer genau diese Position
  lesen
- nach `created_at desc` sortieren
- optional eng limitieren, zum Beispiel `LIMIT 10`

Der Pfad soll keine Daten schreiben.

## 4. Zielbild der Rueckgabe

Eine kleine Response-Struktur reicht:

- `id`
- `decision_type`
- `material_id`
- `source_label`
- `source_unit_price`
- `applied_unit_price`
- `currency`
- `source_reference`
- `source_date`
- `created_at`

Bewusst nicht enthalten:

- Marge
- Zuschlag
- Rabatt
- Freigabestatus
- Bearbeiterkommentar
- Workflow-Zustand

## 5. Vorgeschlagener API-Endpunkt

Der minimale API-Endpunkt kann lauten:

- `GET /api/v1/quotes/{id}/items/{itemID}/price-decision-history`

Permission:

- konsistent mit den bestehenden positionsnahen Preisankern im Quote-Editor:
  `quotes.write`

Response:

- Liste von `PriceDecisionHistoryEntry`

Fehlerverhalten:

- ungueltige Quote-ID oder Positions-ID: Validation Error
- Quote/Position nicht gefunden: fachlicher Fehler
- historische Quote-Version: fachlicher Fehler
- nicht Draft im bestehenden Editorfluss: fachlicher Fehler

## 6. Warum diese Historie nicht die Material-Preis-Historie ersetzt

Es gibt bereits eine Material-Preis-Historie bzw. Preisquellenanzeige.

Die neue Preisentscheidungs-Historie hat eine andere Semantik:

- Material-Preis-Historie zeigt moegliche Quellen
- Preisentscheidungs-Historie zeigt ausgefuehrte Entscheidungen an der
  Quote-Position

Beide duerfen nicht vermischt werden.

## 7. Zielbild im Client

Der Client soll die Historie klein und positionsnah anzeigen:

- API-Methode `getQuoteItemPriceDecisionHistory(...)`
- eigener Ladezustand pro Position
- kleines Draft-Modell fuer History Entries
- Block `Preisentscheidungen` im bestehenden Quote-Editor

Sichtbar werden soll:

- Entscheidungsart
- uebernommener Preis
- Quelle
- Referenz/Quelldatum
- Entscheidungszeit

Bewusst nicht sichtbar werden:

- Bearbeiten-Buttons
- Loeschen-Buttons
- Freigabeaktionen
- Margen- oder Zuschlagsfelder

## 8. Tests

Die Backend-Integrationstests sollen pruefen:

- nach Primaerpreis-Uebernahme liefert der neue Endpunkt genau einen Eintrag
  fuer die Position
- Eintrag enthaelt `primary_source_applied`, Quelle, Referenz, Preis,
  Waehrung und Entscheidungszeit
- eine Position ohne gespeicherte Entscheidung liefert eine leere Liste

Ein Client-Test ist fuer diese kleine bestehende Dialogstruktur optional; die
erste Umsetzung kann mit `flutter analyze` abgesichert werden.

## 9. Warum keine Historienbearbeitung

Historienbearbeitung waere fachlich groesser, weil sofort Fragen entstehen:

- wer darf korrigieren?
- bleiben Korrekturen auditierbar?
- gibt es Storno statt Loeschen?
- wie wirkt sich eine Korrektur auf spaetere Kalkulation aus?

Diese Fragen gehoeren nicht in den ersten read-only Historienblock.

## 10. Warum diese Stufe noch keine Kalkulation ist

Diese Stufe bleibt kleiner als Kalkulation, weil:

- nur gespeicherte Entscheidungen angezeigt werden
- kein neuer Preis berechnet wird
- keine Marge oder kein Zuschlag entsteht
- keine Freigabe ausgeloest wird
- keine Angebotsgesamtbetrachtung entsteht

## 11. Entscheidung

Das minimale technische Zielmodell fuer die naechste Stufe ist:

- ein read-only Backend-Endpunkt fuer `quote_item_price_decisions` je
  Quote-Position
- eine kleine Anzeige im bestehenden Quote-Editor
- keine neue Persistenz, keine Schreibaktion und keine Kalkulationslogik

## 12. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Schritt:

- die erste Backend-Stufe fuer
  `PriceDecisionHistoryForQuoteItem(...)` und
  `GET /api/v1/quotes/{id}/items/{itemID}/price-decision-history` umsetzen,
  bewusst noch ohne Client-Anbindung, Historienbearbeitung, Margen-,
  Zuschlags-, Rabatt-, Freigabe-, Bulk- oder Automatiklogik

