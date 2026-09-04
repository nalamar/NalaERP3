# ADR 0014 — Inventurprozess

Datum: 2026-08-20
Status: entschieden (Subtask C.2.1)
Bezug: Backlog C.2 (Epic C — Waren- & Lagerwirtschaft).

## Kontext

Der Lagerbestand wird ausschließlich als additives Buchungsjournal
geführt (`stock_movements`, `server/internal/materials/service.go`) — der
aktuelle ("Soll"-)Bestand ergibt sich rein aus `SUM(quantity)` je
Material/Lager/Ort/Batch. `StockMovementCreate.Typ` kennt laut
bestehendem Kommentar bereits die Werte `purchase, in, out, transfer,
adjust` — `adjust` ist also bereits als Korrekturbuchungstyp vorgesehen,
wird aber aktuell nirgends erzeugt; es gibt keinen Prozess, der eine
Zählung (Ist) gegen den Buchbestand (Soll) stellt und daraus automatisch
`adjust`-Buchungen erzeugt. `CreateMovement` behandelt `adjust` technisch
identisch zu jedem anderen Typ (keine Sonderlogik, außer der
`purchase`-spezifischen Durchschnittspreis-Fortschreibung).

C.1 (`docs/adr/0013-lagerreservierung-projektbezogen.md`) hat
`stock_reservations` eingeführt — ein rein planerischer "Soft Hold", der
den physischen Bestand nicht verändert. Eine Inventur zählt den
physischen Bestand; Reservierungen sind davon fachlich unabhängig und
bleiben von einer Inventur unberührt.

Etabliertes GoBD-Muster in diesem Repo (Storno-statt-Änderung,
Festschreibung ab einem bestimmten Workflow-Schritt) — zuletzt in ADR
0010 (Nachtragsmanagement: `beantragt` friert die Positionen ein) und ADR
0012 (Abschlags-/Schlussrechnung) angewendet. `stock_movements` selbst ist
bereits von Natur aus Storno-sicher (nie mutiert, nur addiert) — eine
Inventur-Korrektur fügt sich als weitere `adjust`-Zeile in genau dieses
Muster ein, ohne dass ein eigener Storno-Mechanismus für die Inventur
selbst nötig wäre.

Alle bisherigen Lager-/Bestandsfunktionen (inkl. C.1) leben in einem
einzigen `materials.Service`, geschützt über
`stock_movements.read`/`stock_movements.write`.

## Optionen

**Option A — Inventur als reiner Vorschau-/Report-Endpunkt** (Soll/Ist
gegenüberstellen, Korrektur bleibt manuelle `POST /stock-movements/` mit
`typ=adjust` durch den Nutzer).
Verworfen: würde die Zählungen selbst nicht persistieren (kein Nachweis,
WAS zu welchem Zeitpunkt gezählt wurde — GoBD-Nachvollziehbarkeit fehlt),
und die Korrekturbuchung bliebe manuelle Nutzerarbeit statt Teil eines
geführten Prozesses. Der Backlog-Titel "Inventurprozess" verlangt einen
Prozess, kein bloßes Zahlen-Gegenüberstellen.

**Option B — eigenständiger, geführter Inventurprozess** mit zwei neuen
Tabellen (`inventories` Header, `inventory_lines` Zählpositionen),
Status-Workflow `laufend → abgeschlossen`, automatische Erzeugung der
`adjust`-Buchungen beim Abschluss. Gewählt.

**Zu Option B — Zeitpunkt der Soll-Wert-Erfassung**: beim Start der
gesamten Inventur (ein Snapshot für alle Positionen) vs. je Zählposition
beim Hinzufügen?
Entscheidung: je Zählposition beim Hinzufügen. Ein globaler
Warehouse-weiter Snapshot würde voraussetzen, dass während der gesamten
Inventur keine weiteren Wareneingänge/-ausgänge im Lager stattfinden
(ein "Bewegungsstopp") — das ist eine eigene, deutlich größere Fachlogik
(Sperren von `CreateMovement` pro Lager), die `CreateMovement` anfassen
müsste und außerhalb des C.2-Titels liegt. Stattdessen zeigt das System
beim Erfassen jeder Zählposition den ZU DIESEM ZEITPUNKT aktuellen
Buchbestand (praxisnah: der Zähler geht Position für Position durch das
Lager, das System zeigt live den Soll-Wert an).

**Zu Option B — Granularität/Eindeutigkeit je Zählposition**: harte
`UNIQUE`-Constraint auf (Inventur, Material, Ort) vs. keine Constraint?
Entscheidung: keine Constraint. `location_id` ist wie bei
`stock_movements` optional (nullable); Postgres behandelt mehrere `NULL`
in einer `UNIQUE`-Constraint als paarweise verschieden, eine korrekte
Eindeutigkeitsprüfung ohne Ort bräuchte einen partiellen Index mit
`COALESCE`-Ausdruck. Eine versehentliche Doppelzählung ist ein
Anwender-/Schulungsproblem, keine Datenintegritätsverletzung — sie führt
beim Abschluss lediglich zu zwei additiven `adjust`-Buchungen, die sich
korrekt aufsummieren, nicht zu Dateninkonsistenz. Bewusste Vereinfachung,
um die Subtask im Rahmen von §6.3 zu halten.

**Zu Option B — Storno einer bereits abgeschlossenen Inventur**?
Entscheidung: kein eigener Storno-Mechanismus. `stock_movements` ist
bereits additiv/Storno-sicher; eine fehlerhafte Inventur wird durch eine
NEUE Inventur oder eine manuelle Korrekturbuchung richtiggestellt, nicht
durch Rückgängigmachen der alten. Deckt sich mit dem bestehenden
GoBD-Muster (Korrektur statt Mutation) ohne zusätzlichen Code.

## Entscheidung

**Option B.**

- **`inventories`** (Header): `id`, `warehouse_id` (FK, wie
  `stock_movements`/`stock_reservations` — Scope wird darüber geerbt,
  kein eigenes `company_id`), `status text DEFAULT 'laufend' CHECK
  (status IN ('laufend', 'abgeschlossen'))`, `note text DEFAULT ''`,
  `started_at timestamptz DEFAULT now()`, `closed_at timestamptz`
  (NULL bis Abschluss).
- **`inventory_lines`** (Zählpositionen): `id`, `inventory_id` (FK, `ON
  DELETE CASCADE`), `material_id` (FK), `location_id` (FK, nullable, wie
  `stock_movements`), `soll_qty numeric(18,6) NOT NULL` (beim Hinzufügen
  aus dem aktuellen Buchbestand berechnet und fixiert — ändert sich
  danach nicht mehr rückwirkend, auch wenn weitere `stock_movements` für
  dasselbe Material gebucht werden), `ist_qty numeric(18,6) NOT NULL
  CHECK (ist_qty >= 0)`, `counted_at timestamptz DEFAULT now()`. Die
  Differenz (`ist_qty - soll_qty`) wird NICHT gespeichert, sondern beim
  Lesen/Abschließen berechnet (keine redundante, potenziell driftende
  Spalte).
- **Zählpositionen nur im Status `laufend` hinzufügbar** — nach Abschluss
  ist die Inventur (Header UND alle Zeilen) vollständig schreibgeschützt
  (Festschreibung, wie bei `sales_order_addenda`/Angeboten etabliert).
- **Abschluss (`POST /inventories/{id}/close`)**: für JEDE Zählposition,
  deren `ist_qty ≠ soll_qty`, wird transaktional genau eine
  `stock_movements`-Zeile erzeugt (`quantity = ist_qty - soll_qty`,
  `movement_type = 'adjust'`, `reference = 'inventur:<inventory_id>'`,
  `reason = 'Inventurkorrektur'`) — der bereits bestehende, bislang
  ungenutzte `adjust`-Typ wird damit erstmals produktiv befüllt.
  Anschließend `status='abgeschlossen'`, `closed_at=now()`. Zeilen ohne
  Differenz erzeugen bewusst KEINE Leerbuchung.
- **Keine Kopplung an `stock_reservations`** — eine Inventur zählt den
  physischen Bestand, unabhängig von aktiven Reservierungen; C.1 bleibt
  komplett unangetastet.
- **`CreateMovement`/`StockByMaterial` bleiben unangetastet** — der
  Abschluss-Pfad RUFT `CreateMovement`s zugrunde liegende Insert-Logik
  nicht separat auf, sondern fügt direkt eine `stock_movements`-Zeile in
  derselben Transaktion ein (analog dazu, wie B.4.3
  `accounting.ARService.createTx` nicht verändert, sondern nur ergänzend
  genutzt hat) — keine Vermischung der bestehenden, produktiven
  Buchungslogik mit dem neuen Inventurpfad.
- **Implementierung in `materials.Service`** (kein neues Paket) — exakt
  der bestehende Ort für alles Lager-/Bestandsbezogene.
- **Keine neue Permission-Infrastruktur** — Wiederverwendung von
  `stock_movements.read`/`stock_movements.write`, analog zu C.1.

## Konsequenzen

- **C.2.2** (Folge-Subtask): additive Migration `076_inventories.sql`
  (zwei neue Tabellen, keine bestehende Tabelle geändert), reversibel.
- **C.2.3** (Folge-Subtask): Anwendungscode nur in `materials/service.go`
  (`StartInventory`/`AddInventoryLine`/`CloseInventory`/`ListInventories`/
  `GetInventory`), HTTP-Wiring, Tests. `CreateMovement`/`StockByMaterial`
  bleiben komplett unangetastet.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von C.2):
  warehouse-weiter Bewegungsstopp während einer laufenden Inventur,
  Stichproben-/Zykluszählung (nur Teilmenge der Artikel), Inventurbericht/
  -export.
- Kein Einfluss auf bestehende `stock_movements`-/`stock_reservations`-
  Daten oder deren Lesepfade.
