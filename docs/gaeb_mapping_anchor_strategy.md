# GAEB-Herkunfts- und Mapping-Anker: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach dem
abgeschlossenen GAEB-Import-, Review-, Apply- und Transparenzpfad zu.

Der Scope bleibt bewusst eng:

- Herkunftsverknuepfung zwischen akzeptierter Importposition und erzeugter
  Quote-Position
- keine Preis-, Steuer- oder Materialautomatik
- kein neuer globaler Mapping-Workflow

## 1. Ausgangslage

Der aktuelle Apply-Pfad erzeugt aus `accepted`-Importpositionen eine neue
Draft-Quote.

Heute ist die Uebernahme bewusst roh:

- `description`
- `qty`
- `unit`

werden uebernommen, waehrend

- `unit_price = 0`
- `tax_code = ''`

bleiben.

Dadurch fehlt noch die kleine technische Bruecke fuer spaeteres Mapping.

## 2. Kernentscheidung

Der erste Mapping-Schritt soll **nicht** bereits Preise, Steuern oder
Materialien ableiten.

Der kleinste sinnvolle technische Schnitt ist:

- fuer jede uebernommene `accepted`-Importposition eine Herkunftsverknuepfung
  zur erzeugten Quote-Position speichern

Damit wird zuerst nur die Rueckverfolgbarkeit aufgebaut.

## 3. Warum genau dieser Schnitt klein genug ist

Dieser Schritt bleibt klein, weil er:

- keine bestehenden Quote-Werte fachlich veraendert
- keine neue Bewertungslogik einführt
- nur den bereits vorhandenen Apply-Pfad um Herkunftsdaten ergänzt
- spaetere Mapping-Ausbaustufen vorbereitet, ohne sie vorwegzunehmen

## 4. Minimales Zielbild der Verknuepfung

Benötigt wird eine kleine Zuordnung:

- `quote_import_item_id`
- `quote_id`
- `quote_item_id`

Optional, aber im ersten Schritt nicht zwingend:

- `mapping_status`
- `material_id`
- `price_source`

Diese Felder gehoeren erst in spaetere Ausbaustufen.

## 5. Empfohlene technische Form

Der kleinste saubere Zuschnitt ist eine neue Mapping-Tabelle, zum Beispiel:

- `quote_import_item_links`

Minimaler Inhalt:

- `id`
- `quote_import_item_id`
- `quote_id`
- `quote_item_id`
- `created_at`

Integritaet:

- `quote_import_item_id` referenziert `quote_import_items(id)`
- `quote_id` referenziert `quotes(id)`
- `quote_item_id` referenziert `quote_items(id)`

Empfohlene Guard Rail:

- pro `quote_import_item_id` im ersten MVP nur genau eine Link-Zeile

## 6. Ort der Erzeugung

Die Verknuepfung sollte direkt im bestehenden Apply-Pfad entstehen:

- `ApplyImportToDraftQuote(...)` in `server/internal/quotes/imports.go`

Warum dort:

- dort liegen sowohl akzeptierte Importpositionen als auch die frisch erzeugte
  Quote vor
- keine zweite Nachbearbeitungslogik ist notwendig

## 7. Minimaler Orchestrierungsansatz

Der bestehende Apply-Pfad laedt akzeptierte Importpositionen derzeit nur als
flache Quelle fuer `QuoteItemInput`.

Der kleinste technische Ausbau waere:

1. akzeptierte Importpositionen inklusive ihrer `id` laden
2. Quote wie bisher erzeugen
3. erzeugte Quote-Items in derselben Reihenfolge auslesen
4. pro uebernommener Position genau eine Herkunftsverknuepfung schreiben

Wichtige Annahme fuer den kleinen MVP:

- die Reihenfolge `accepted`-Importpositionen -> erzeugte Quote-Items bleibt
  stabil und reicht als Zuordnungsbasis aus

## 8. Was bewusst noch nicht Teil dieses Blocks ist

Nicht in denselben Schritt ziehen:

- sichtbare Anzeige dieser Herkunftslinks in der UI
- Materialmapping gegen `materials`
- Preisvorschlaege
- Steuercode-Vorschlaege
- nachtraegliche Neuzuordnung oder Editierdialoge

Diese Themen bauen spaeter auf dem Herkunftsanker auf.

## 9. Guard Rails

Der erste Herkunftsanker sollte mindestens diese Regeln einhalten:

- Links nur fuer `accepted`-Positionen anlegen
- Links nur im erfolgreichen Apply-Fall schreiben
- keine Duplikate fuer dieselbe `quote_import_item_id`
- bei fehlgeschriebener Verknuepfung den Apply-Vorgang transaktional scheitern

Damit bleibt die Rueckverfolgbarkeit konsistent.

## 10. Erwarteter Nutzen

Dieser kleine Schritt schafft sofort:

- technische Rueckverfolgbarkeit von GAEB-Quelle zur Quote
- eine belastbare Basis fuer spaeteres Mapping
- weniger Black-Box-Wirkung im Apply-Pfad

Ohne bereits riskante Fachlogik einzufuehren.

## 11. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste Backend-Stufe fuer den Herkunfts-/Mapping-Anker umsetzen:
  Migration fuer die Link-Tabelle plus kleine Erweiterung von
  `ApplyImportToDraftQuote(...)`, die die Links beim erfolgreichen Apply
  mitschreibt.
