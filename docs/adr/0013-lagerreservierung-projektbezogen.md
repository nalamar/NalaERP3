# ADR 0013 — Projektbezogene Lagerreservierung

Datum: 2026-08-18
Status: entschieden (Subtask C.1.1)
Bezug: Backlog C.1 (Epic C — Waren- & Lagerwirtschaft).

## Kontext

Der bestehende Lagerbestand (`server/internal/materials/service.go`) wird
ausschließlich als additives Buchungsjournal geführt:
`stock_movements` (`material_id`, `warehouse_id`, `location_id?`,
`batch_id?`, `quantity`, `movement_type` u. a.) — der aktuelle Bestand
ergibt sich rein aus `SUM(quantity)` je Material/Lager/Ort/Batch
(`StockByMaterial`). Es gibt **keinen** Reservierungs-, Verfügbarkeits-
oder "frei verfügbar"-Begriff — jede physisch vorhandene Menge gilt heute
als für jeden Zweck frei verfügbar, unabhängig davon, ob sie bereits einem
Projekt zugesagt ist.

`warehouses`/`materials` sind eigenständig mandantengescoped
(`company_id`, ADR 0002, `057_materials_warehouses_scope.sql`);
`stock_movements` selbst hat **bewusst keine eigene** `company_id`-Spalte,
sondern erbt den Scope über `warehouse_id` — dokumentiertes,
etabliertes Muster für Ledger-/Kindtabellen dieser Domäne.

`sales_order_items` hat **kein** `material_id`-Feld (nur `quote_items`
bekam das in `047_quote_item_manual_mapping.sql`) — eine Reservierung kann
also NICHT automatisch aus einer Auftragsposition abgeleitet werden, ohne
das Auftragsschema zu ändern; das ist nicht Teil von C.1. `quotes` und
`sales_orders` haben beide ein nullable `project_id UUID REFERENCES
projects(id)`; `projects.id` ist ebenfalls `UUID`. `materials.id` und
`warehouses.id` sind dagegen `text` (uuid-formatierte Strings, kein
UUID-Spaltentyp) — bestehende Inkonsistenz aus `001_init.sql`, hier nur
zur Kenntnis genommen, nicht behoben (außerhalb des Scopes).

Alle bisherigen warehouse-/stock-bezogenen Endpunkte leben in einem
einzigen `materials.Service` (kein eigenes `warehouse`-Paket) und werden
über `stock_movements.read`/`stock_movements.write` bzw.
`warehouses.read`/`warehouses.write` geschützt.

## Optionen

**Option A — Reservierung automatisch aus Auftragspositionen ableiten**
(z. B. bei Auftragsbestätigung automatisch reservieren).
Verworfen: setzt `material_id` auf `sales_order_items` voraus — das ist
eine Schema-Änderung an einer bestehenden, produktiven Tabelle außerhalb
des C.1-Titels ("Reservierungslogik") und würde die Subtask deutlich über
die ~8-Datei/~400-Zeilen-Grenze aus §6.3 treiben. Wäre ein sinnvoller
Folge-Schritt, aber eine eigene Backlog-Position.

**Option B — eigenständige, manuell angelegte Reservierung** (`material_id`
+ `warehouse_id` + `project_id` + Menge), analog zu `stock_movements`
manuell über die API angelegt, mit Verfügbarkeitsprüfung gegen den
physischen Bestand minus bereits aktiver Reservierungen. Gewählt.

**Zu Option B — Granularität**: Reservierung auf `location_id`/`batch_id`
statt nur `warehouse_id`?
Verworfen: Batches existieren erst, wenn Ware bereits eingegangen ist
(`batch_id` wird beim Wareneingang gesetzt) — eine Reservierung muss aber
auch VOR Wareneingang möglich sein (z. B. gegen erwarteten Bestand einer
laufenden Bestellung). Location-Granularität wäre für die reine
"ist genug für Projekt X verfügbar"-Frage unnötig starr. Reservierung
bleibt auf Material+Lager-Ebene — die gleiche Granularität, auf der auch
die Verfügbarkeitsprüfung selbst rechnet.

**Zu Option B — Verfügbarkeitsprüfung**: Reservierung über den aktuell
verfügbaren (unreservierten) Bestand hinaus zulassen (rein informativ) vs.
hart blockieren?
Entscheidung: hart blockieren. Eine Reservierungslogik, die Überbuchung
zulässt, wäre keine Logik, sondern nur ein Label — der gesamte fachliche
Wert von C.1 liegt darin, dass zwei Projekte sich nicht denselben
physischen Bestand versehentlich doppelt zusagen. Wird mehr benötigt, als
aktuell frei ist, wird die Reservierung mit 400 abgelehnt (keine
Teilerfüllung, keine automatische Rückstellung — das wäre eine eigene,
größere Fachlogik).

**Zu Option B — Freigabe**: automatisch bei Bestandsverbrauch (Verknüpfung
mit `stock_movements`) vs. manuell per Endpunkt?
Entscheidung: manuell (`status: 'aktiv' → 'freigegeben'`). Eine
automatische Verknüpfung würde `CreateMovement` anfassen müssen (das
bestehende, produktive Buchungsjournal) und setzt voraus, dass jede
Warenentnahme exakt einer Reservierung zugeordnet werden kann — das ist
mangels `material_id` auf `sales_order_items` (s. o.) ohnehin nicht
zuverlässig automatisierbar. Bleibt additiv und minimal-invasiv, analog
zu B.2/B.3/B.4.

## Entscheidung

**Option B.**

- Neue Tabelle `stock_reservations`: `id text` (uuid-formatiert, wie
  `materials`/`warehouses`/`stock_movements`), `material_id text NOT NULL
  REFERENCES materials(id) ON DELETE CASCADE`, `warehouse_id text NOT NULL
  REFERENCES warehouses(id) ON DELETE RESTRICT`, `project_id UUID NOT NULL
  REFERENCES projects(id) ON DELETE RESTRICT` ("projektbezogen" ist
  laut Backlog-Titel konstitutiv, daher NOT NULL — anders als das
  nullable `project_id` auf `quotes`/`sales_orders`), `qty numeric(18,6)
  NOT NULL CHECK (qty > 0)` (gleiche Präzision wie
  `stock_movements.quantity`), `status text NOT NULL DEFAULT 'aktiv' CHECK
  (status IN ('aktiv', 'freigegeben'))`, `grund text NOT NULL DEFAULT ''`,
  `referenz text NOT NULL DEFAULT ''` (freier Text, analog
  `stock_movements.reason`/`.reference` — keine FK auf eine einzelne
  Quelltabelle, da eine Reservierung sich auf ein Angebot, einen Auftrag
  oder rein manuell begründen kann), `created_at timestamptz NOT NULL
  DEFAULT now()`, `released_at timestamptz` (NULL bis Freigabe).
- **Kein eigenes `company_id`/`branch_id`** auf `stock_reservations` — Scope
  wird über `warehouse_id → warehouses.company_id` geerbt, exakt wie bei
  `stock_movements` (ADR 0002, `057_materials_warehouses_scope.sql`).
  Zusätzlich wird bei Anlage geprüft, dass `material_id` UND `project_id`
  ebenfalls zum aufrufenden Mandanten gehören (Guard im Anwendungscode,
  analog zu `CreateMovement`s bestehenden `materialOwned`/`warehouseOwned`-
  Prüfungen).
- **Keine Batch-/Ortsgranularität** — Reservierung ist auf Material+Lager
  gebunden, unabhängig von `location_id`/`batch_id`.
- **Verfügbarkeitsprüfung als Kernregel**: `verfügbar(material, warehouse)
  = SUM(stock_movements.quantity WHERE material_id, warehouse_id) -
  SUM(stock_reservations.qty WHERE material_id, warehouse_id, status='aktiv')`.
  Eine neue Reservierung wird abgelehnt (400), wenn ihre `qty` die so
  berechnete verfügbare Menge übersteigt — Prüfung und Insert in einer
  Transaktion (Race-Sicherheit über `SELECT ... FOR UPDATE` auf die
  betroffenen `stock_movements`-Zeilen des Material/Lager-Paars, analog
  zum bestehenden Sperr-Muster in `sales.ensureOrderEditableTx`).
- **Freigabe ist eine eigene, manuelle Aktion** (`status → 'freigegeben'`,
  `released_at` gesetzt) — keine automatische Verknüpfung mit
  `CreateMovement`/Wareneingang oder -ausgang. Eine freigegebene
  Reservierung zählt nicht mehr in der Verfügbarkeitsberechnung.
- **Implementierung in `materials.Service`** (kein neues Paket) — exakt
  der bestehende Ort für alles Lager-/Bestandsbezogene
  (`Warehouse`/`Location`/`StockMovement`). Ein neues, eigenes Paket wäre
  für eine einzelne, eng verwandte Fachlogik unbegründete Zersplitterung.
- **Keine neue Permission-Infrastruktur** — Reservierungen werden über die
  bestehenden `stock_movements.read`/`stock_movements.write` geschützt
  (gleicher fachlicher Kontext "Bestandsbewegung/-bindung", analog zur
  Wiederverwendung von `sales_orders.read/write` in B.2/B.3).

## Konsequenzen

- **C.1.2** (Folge-Subtask): additive Migration `075_stock_reservations.sql`
  (neue Tabelle, keine bestehende Tabelle geändert), reversibel.
- **C.1.3** (Folge-Subtask): Anwendungscode nur in `materials/service.go`
  (`CreateReservation`/`ReleaseReservation`/`ListReservations`/
  `AvailableStock`-Helfer), HTTP-Wiring, Tests. `CreateMovement` bleibt
  komplett unangetastet (keine Kopplung an Reservierungen).
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von C.1):
  automatische Reservierung aus Auftragspositionen (setzt `material_id`
  auf `sales_order_items` voraus, Option A oben), automatische Freigabe
  bei Warenausgang, Reservierung auf Location-/Batch-Ebene.
- Kein Einfluss auf bestehende `stock_movements`-Daten oder den
  bestehenden `StockByMaterial`-Lesepfad.
