# Nummernkreisstrategie

## Ziel
- Alle kaufmaennischen und operativen Belege verwenden den zentralen Mechanismus `number_sequences`.
- Jede Belegart hat genau einen fachlich benannten `entity`-Schluessel.
- Nummern sind fuer Benutzer lesbar, pro Belegart eindeutig und standardmaessig jahresbezogen.
- Die Vergabe erfolgt nur an fachlich belastbaren Erzeugungspunkten, nicht mehrfach entlang desselben Workflows.

## Technische Basis
- Tabelle: `number_sequences(entity, pattern, next_value, last_year, updated_at)`.
- Service: `server/internal/settings/numbering.go`.
- Platzhalter heute: `{YYYY}`, `{YY}`, `{MM}`, `{DD}`, `{NN}`, `{NNN}`, `{NNNN}`.
- Reset-Logik heute: Wenn das Pattern `{YYYY}` oder `{YY}` enthaelt, startet der Zaehler pro Kalenderjahr wieder bei `1`.
- Konfiguration heute: `GET/PUT /api/v1/settings/numbering/{entity}` und `GET /api/v1/settings/numbering/{entity}/preview`.

## Bereits aktive Nummernkreise
| Entity | Standard-Pattern | Vergabezeitpunkt | Quelle |
|---|---|---|---|
| `project` | `PRJ-{YYYY}-{NNNN}` | bei Projekterzeugung bzw. LogiKal-Fallback | `projects.Service.Create`, `projects/import_logikal.go` |
| `purchase_order` | `PO-{YYYY}-{NNNN}` | bei Bestellungserzeugung | `purchasing.Service.Create` |
| `quote` | `ANG-{YYYY}-{NNNN}` | bei Angebotserzeugung | `quotes.Service.Create` |
| `sales_order` | `AUF-{YYYY}-{NNNN}` | bei Auftragsableitung aus Angebot | `sales.Service.CreateFromQuote` |
| `invoice_out` | `RE-{YYYY}-{NNNN}` | beim Buchen, nicht beim Entwurf | `accounting.ARService.Book` |
| `invoice_in` | `ER-{YYYY}-{NNNN}` | fachlich vorgesehen, noch ohne belegten Vergabepunkt | Migration vorhanden |

## Fachregeln
### 1. Entity-Schluessel bleiben stabil
- `entity` ist technischer Primärschluessel fuer Konfiguration, Preview und Vergabe.
- UI-, API- und Migrationsnamen muessen identisch sein.
- Neue Belegarten bekommen keinen frei erfundenen Pattern-Text ohne vorherigen `entity`-Schluessel in `number_sequences`.

### 2. Vergabe erfolgt genau einmal
- `project`, `purchase_order`, `quote`, `sales_order`: Nummer bei fachlicher Erstellung vergeben.
- `invoice_out`: Nummer erst beim Buchen vergeben, damit Entwuerfe keine finalen Rechnungsnummern verbrennen.
- Folgebelege uebernehmen nie die Nummer des Quellbelegs, sondern ziehen ihren eigenen Nummernkreis.

### 3. Jahresbezug ist Standard
- Kaufmaennische Belege bleiben standardmaessig jahresbezogen.
- Daher sollen Standard-Pattern fuer Belege immer `{YYYY}` enthalten.
- Ein Pattern ohne Jahresanteil ist nur fuer bewusst globale Sequenzen gedacht und muss als Ausnahme betrachtet werden.

### 4. Nummern sind lesbar, kurz und rollenfest
- Praefixe sollen dem deutschen Fachbegriff folgen.
- Empfohlene Breite des numerischen Teils: `NNNN`.
- Belegnummern muessen im PDF, in Listen, Suchfeldern und Fremdreferenzen stabil verwendbar sein.

## Zielmatrix fuer alle Belegarten
| Domäne | Entity | Empfohlenes Pattern | Status |
|---|---|---|---|
| Projekte | `project` | `PRJ-{YYYY}-{NNNN}` | aktiv |
| Einkauf | `purchase_order` | `PO-{YYYY}-{NNNN}` | aktiv |
| Vertrieb | `quote` | `ANG-{YYYY}-{NNNN}` | aktiv |
| Vertrieb | `sales_order` | `AUF-{YYYY}-{NNNN}` | aktiv |
| Finance AR | `invoice_out` | `RE-{YYYY}-{NNNN}` | aktiv |
| Finance AP | `invoice_in` | `ER-{YYYY}-{NNNN}` | vorbereitet |
| Vertrieb | `quote_revision` | `ANGR-{YYYY}-{NNNN}` oder Revision als Suffix am Angebot | fachlich offen |
| Einkauf | `goods_receipt` | `WE-{YYYY}-{NNNN}` | noch nicht implementiert |
| Einkauf | `supplier_return` | `LRET-{YYYY}-{NNNN}` | noch nicht implementiert |
| Lager | `stock_adjustment` | `LB-{YYYY}-{NNNN}` | noch nicht implementiert |
| Produktion | `production_order` | `FA-{YYYY}-{NNNN}` | noch nicht implementiert |
| Produktion | `work_order` | `AG-{YYYY}-{NNNN}` | noch nicht implementiert |
| HR | `employee` | `MA-{YYYY}-{NNNN}` oder standortunabhaengige Personalnummer | noch nicht implementiert |
| HR | `vacation_request` | `URL-{YYYY}-{NNNN}` | noch nicht implementiert |
| Fleet | `fleet_asset` | `FZG-{YYYY}-{NNNN}` | noch nicht implementiert |

## Entscheidungen fuer den weiteren Ausbau
### Angebote und Revisionen
- Die Basiskennung eines Angebots bleibt `quote`.
- Versionierung eines Angebots sollte nicht sofort einen neuen Hauptnummernkreis erzwingen.
- Bevorzugt: Hauptnummer bleibt stabil, Revision als Suffix oder eigenes Feld, z. B. `ANG-2026-0007-R02`.

### Rechnungen
- `invoice_out` bleibt strikt an den Buchungszeitpunkt gekoppelt.
- Ein spaeteres Storno benoetigt einen eigenen Nummernkreis statt Wiederverwendung der Ursprungsnummer.
- Fuer Eingangsrechnungen ist zu klaeren, ob `ER-*` eine interne ERP-Referenz oder die Lieferantenbelegnummer repraesentiert.
  Aktuelle Empfehlung: `invoice_in` erzeugt eine interne ERP-Referenz, waehrend die Lieferantenrechnungsnummer separat gespeichert bleibt.

### Projekte
- Projekt-Hauptnummer bleibt vom internen Aufbau aus Phasen und Positionen getrennt.
- Phasen- und Positionsnummern sind keine Nummernkreis-Entities, sondern strukturbezogene Teilkennungen innerhalb eines Projekts.

## Konkrete technische Folgeaufgaben
1. `invoice_in` mit echtem Service- und UI-Vergabepunkt anbinden oder den vorbereiteten Seed wieder entfernen, falls AP verschoben wird.
2. Belegartenliste im Settings-UI von einer festen Zweierliste auf eine konfigurierbare Matrix erweitern.
3. `NumberingService.Next` fuer unbekannte Entities nicht mehr stillschweigend mit `PO-{YYYY}-{NNNN}` bootstrappen, sondern fachlich validieren.
4. Optional eine erlaubte Entity-Liste zentral hinterlegen, damit Tippfehler in API oder Migrationen keine Schatten-Nummernkreise erzeugen.
5. Bei kuenftigen Storno-, Retouren- und Produktionsbelegen zuerst `entity`, Pattern und Vergabezeitpunkt definieren, dann Migration und UI nachziehen.
