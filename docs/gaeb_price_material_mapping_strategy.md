# GAEB-Preis-/Material-Mapping: Minimales technisches Zielmodell fuer den ersten manuellen Einstieg

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach dem
abgeschlossenen Herkunftsanker zu.

Der Scope bleibt bewusst eng:

- erster manueller Mapping-Einstieg auf Ebene der erzeugten Quote-Position
- kleiner Material- und Preisanker, aber keine Vorschlagslogik
- keine Automatik, keine globale Mapping-Konsole

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- `accepted import item -> created quote item` ist persistent verknuepft
- die Verknuepfung ist im Importpositionsdialog sichtbar
- die erzeugte Quote bleibt fachlich roh mit
  - `unit_price = 0`
  - `tax_code = ''`

Damit ist die Herkunft gesichert, aber noch kein erster manueller
Mapping-Entscheid technisch modelliert.

## 2. Kernentscheidung

Der erste Preis-/Material-Mapping-Schritt soll **nicht** automatisch passende
Materialien oder Preise vorschlagen.

Der kleinste sinnvolle technische Schnitt ist:

- eine kleine manuelle Mapping-Struktur direkt an der uebernommenen
  Quote-Position
- zunaechst nur fuer optionalen Materialbezug und einen kleinen
  Preisstatusanker

## 3. Warum der Einstieg an der Quote-Position liegen sollte

Die Quote-Position ist der richtige Einstiegspunkt, weil:

- dort spaetere Preis- und Materialarbeit real fachlich stattfindet
- der Herkunftsanker dorthin bereits existiert
- keine zweite editierbare Mapping-Welt auf Import-Ebene entsteht

## 4. Empfohlenes Minimalzielbild

Der erste manuelle Mapping-Einstieg sollte nur diese Fragen abbilden:

- ist an einer uebernommenen Quote-Position bereits ein Material manuell
  zugeordnet?
- ist der Preis fuer diese Position noch offen oder bereits manuell
  entschieden?

Bewusst noch nicht im ersten Schritt:

- Preishistorie
- Preisquelle mit Rechenlogik
- Steuerableitung

## 5. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist eine kleine optionale Mapping-Erweiterung
fuer `quote_items`.

Sinnvolle Minimalfelder:

- `material_id` nullable
- `price_mapping_status` als kleiner Status

Empfohlener minimaler Statusraum:

- `open`
- `manual`

Damit kann ein erster manueller Mapping-Entscheid sichtbar werden, ohne schon
komplexe Preislogik zu behaupten.

## 6. Warum kein separates Mapping-Objekt im ersten Schritt

Ein separates grosses Mapping-Objekt waere fuer den ersten Einstieg zu schwer:

- mehr Schreibpfade
- mehr Joins
- mehr Workflow-Flaeche

Fuer den kleinen MVP ist es sauberer, den manuellen Mapping-Entscheid direkt
an der erzeugten Quote-Position zu verankern.

## 7. Empfohlene erste Schreiboberflaeche

Der erste Schreibschnitt sollte bewusst klein bleiben:

- genau ein kleiner Patch-Pfad fuer eine bestehende Quote-Position
- nur `material_id` und `price_mapping_status`

Bewusst noch nicht:

- direkte Preisberechnung
- gleichzeitige Bearbeitung vieler Positionen
- Schreiben auf Importpositions-Ebene

## 8. Guard Rails

Der erste manuelle Mapping-Pfad sollte mindestens diese Regeln einhalten:

- nur auf bestehende Quote-Positionen schreiben
- `material_id` nur auf existierende `materials(id)` setzen
- `price_mapping_status` nur im engen erlaubten Statusraum akzeptieren
- keine automatische Aenderung von `unit_price` im selben Schritt
- keine Rueckmutation an `quote_import_items` oder Herkunftslinks

## 9. Read-only Anschlussnutzen

Schon ohne Automatik schafft dieser kleine Zuschnitt sofort:

- sichtbaren Unterschied zwischen roher uebernommener Position und manuell
  bestaetigter Position
- Basis fuer spaetere Filter wie `Mapping offen`
- belastbare Grundlage fuer spaetere Preis- oder Materialvorschlaege

## 10. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- Vorschlagslogik gegen `materials`
- automatische Preisbefuellung aus Material oder Historie
- Steuer- oder Kontierungsautomatik
- Bulk-Mapping fuer ganze Quotes oder Importlaeufe
- KI-Unterstuetzung

## 11. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste Backend-Stufe fuer den manuellen Preis-/Material-Mapping-Einstieg
  vorbereiten, bewusst noch ohne Client-Komfort und ohne Vorschlagslogik
