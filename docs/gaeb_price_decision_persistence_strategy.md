# GAEB-Preisentscheidungs-Persistenz:
# Minimalzielbild und technischer Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das Minimalzielbild fuer die erste enge
Preisentscheidungs-Persistenz nach abgeschlossener
Preisentscheidungs-Transparenz zu.

Der Fokus bleibt bewusst eng:

- genau die explizite Primaerpreis-Uebernahme an einer Quote-Position als
  kleinen Snapshot festhalten
- keine Margen-, Zuschlags-, Rabatt-, Freigabe-, Bulk- oder Automatiklogik
  einfuehren
- keine neue UI-Entscheidung und keine neue Arbeitsflaeche schaffen

## 1. Ausgangspunkt

Der aktuelle Stand bietet bereits:

- gesetztes `material_id` an der Quote-Position
- serverseitige Bestimmung der primaeren Preisquelle
- explizite Uebernahme der primaeren Quelle als Positionspreis
- read-only Transparenz, ob der aktuelle Positionspreis der aktuellen
  primaeren Quelle entspricht oder abweicht

Die verbleibende technische Luecke ist:

- die konkrete Entscheidung zur Uebernahme wird nicht als stabiler Snapshot
  gespeichert

## 2. Fachliches Minimalziel

Das Minimalziel ist:

- beim Ausfuehren von `ApplyPrimaryPriceSourceForQuoteItem(...)` genau einen
  kleinen Preisentscheidungs-Snapshot zur Quote-Position zu speichern

Dieser Snapshot soll nur festhalten:

- welche Quote-Position betroffen war
- welches Material zu diesem Zeitpunkt gesetzt war
- welche primaere Quelle verwendet wurde
- welcher Preis uebernommen wurde
- welche Waehrung, Referenz und welches Quelldatum zur Quelle gehoerten
- welche Entscheidungsart ausgefuehrt wurde
- wann die Entscheidung gespeichert wurde

Er soll bewusst noch nicht festhalten:

- Marge
- Zuschlag
- Rabatt
- Freigabestatus
- Benutzerkommentar
- Workflow-Zustaende
- Bulk-Aktionsbezug

## 3. Vorgeschlagene Tabelle

Die kleinste Tabelle kann lauten:

- `quote_item_price_decisions`

Vorgeschlagene Spalten:

- `id uuid primary key`
- `quote_id uuid not null references quotes(id) on delete cascade`
- `quote_item_id uuid not null references quote_items(id) on delete cascade`
- `material_id text references materials(id) on delete set null`
- `decision_type text not null`
- `source_label text not null`
- `source_unit_price numeric(18,4) not null`
- `applied_unit_price numeric(18,4) not null`
- `currency char(3) not null default 'EUR'`
- `source_reference text not null default ''`
- `source_date timestamptz`
- `created_at timestamptz not null default now()`

Erste erlaubte `decision_type`:

- `primary_source_applied`

Bewusst nicht enthalten:

- `margin_percent`
- `surcharge_percent`
- `discount_percent`
- `approval_status`
- `approved_by`
- `comment`

Diese Felder wuerden bereits groessere Kalkulations- oder Workflowlogik
andeuten.

## 4. Indizes und Constraints

Minimal sinnvolle Indizes:

- `idx_quote_item_price_decisions_quote_item_created`
  auf `(quote_item_id, created_at desc)`
- `idx_quote_item_price_decisions_quote_created`
  auf `(quote_id, created_at desc)`

Minimal sinnvolle Constraints:

- `decision_type in ('primary_source_applied')`
- `source_unit_price >= 0`
- `applied_unit_price >= 0`
- `currency` nicht leer

Es soll keine Unique-Constraint auf `quote_item_id` geben. Eine Position darf
spaeter mehrere Entscheidungen haben, wenn der Nutzer die primaere Quelle
erneut explizit uebernimmt.

## 5. Zielbild im Backend

Der bestehende Write-Pfad bleibt der einzige Einstieg:

- `ApplyPrimaryPriceSourceForQuoteItem(ctx, quoteID, itemID)`

Die Methode soll nach erfolgreicher serverseitiger Bestimmung der primaeren
Quelle und vor dem Commit in derselben Transaktion:

- den Positionspreis aktualisieren
- Quote-Summen aktualisieren
- einen Snapshot in `quote_item_price_decisions` schreiben

Der Snapshot soll die Daten aus der bereits bestimmten `primary`-Quelle und
dem aktuellen `material_id` verwenden.

## 6. Vorgeschlagene Hilfsfunktion

Eine kleine interne Hilfsfunktion reicht:

- `insertQuoteItemPriceDecisionTx(ctx, tx, input)`

Oder, noch enger fuer die erste Stufe:

- `insertPrimarySourceAppliedDecisionTx(ctx, tx, quoteID, itemID, materialID, primary)`

Sie soll keine fachliche Entscheidung treffen, sondern nur den Snapshot
schreiben.

## 7. API-Zuschnitt

Fuer die erste Persistenzstufe ist kein neuer API-Endpunkt erforderlich.

Begruendung:

- die Entscheidung wird bereits ueber
  `POST /api/v1/quotes/{id}/items/{itemID}/apply-primary-price-source`
  explizit ausgeloest
- der Snapshot ist ein Nebenprodukt genau dieser Entscheidung
- eine separate Historienanzeige waere ein eigener spaeterer Block

Die bestehende Response der aktualisierten Quote kann unveraendert bleiben.

## 8. Tests

Die bestehende Integrationsteststrecke fuer die Primaerpreis-Uebernahme soll
erweitert werden:

- nach erfolgreicher Uebernahme existiert genau ein
  `quote_item_price_decisions`-Datensatz fuer die Position
- `decision_type = primary_source_applied`
- `source_label` und `source_reference` entsprechen der primaeren Quelle
- `source_unit_price` und `applied_unit_price` entsprechen dem uebernommenen
  Preis
- ohne Material wird kein Snapshot geschrieben

Optional fuer spaeter:

- zweiter Klick erzeugt zweiten Snapshot statt den ersten zu ueberschreiben

## 9. Warum keine Client-Aenderung

Die erste Persistenzstufe braucht keine Client-Aenderung, weil:

- der Nutzer bereits explizit klickt
- der Backend-Pfad die Entscheidung eindeutig kennt
- keine neue Eingabe erforderlich ist
- keine Historienanzeige Teil dieses Blocks ist

Der Client kann unveraendert die aktualisierte Quote anzeigen.

## 10. Warum diese Stufe noch keine Kalkulation ist

Diese Stufe bleibt bewusst kleiner als Kalkulation, weil:

- sie nur einen ausgefuehrten Preisentscheidungs-Snapshot speichert
- keine Marge berechnet wird
- kein Zuschlag oder Rabatt angewendet wird
- keine Freigabe entsteht
- keine Angebotsgesamtbetrachtung entsteht

## 11. Entscheidung

Das minimale technische Zielmodell fuer die naechste Stufe ist:

- eine neue enge Tabelle `quote_item_price_decisions`
- ein Snapshot-Schreibschritt innerhalb von
  `ApplyPrimaryPriceSourceForQuoteItem(...)`
- keine neue API, keine Client-Aenderung und keine Kalkulationslogik

## 12. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Schritt:

- die Backend-Persistenz fuer `quote_item_price_decisions` mit Migration,
  transaktionalem Insert und Integrationstest umsetzen

