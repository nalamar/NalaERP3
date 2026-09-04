# ADR 0018 — Eingangsrechnungsprüfung (3-Way-Match)

Datum: 2026-08-29
Status: entschieden (Subtask D.3.1)
Bezug: Backlog D.3 (Epic D — Bestellwesen, letzte Task, nach D.2).

## Kontext

`purchasing.PurchaseOrder`/`PurchaseOrderItem` existieren bereits
(`server/internal/purchasing/service.go`). `Statuses()` kennt `draft,
ordered, received, canceled` — `received` ist aber ein reiner
KOPF-Status der GESAMTEN Bestellung, kein granularer Wareneingang je
Position; es gibt aktuell KEINE strukturierte Erfassung, WELCHE Menge
WANN gegen WELCHE Bestellposition tatsächlich eingegangen ist.

Der physische Wareneingang wird bereits vollständig über
`stock_movements` abgebildet (C.1/C.2) — `movement_type='purchase'`
existiert bereits und löst schon heute die Durchschnittspreis-
Fortschreibung auf `materials` aus (`CreateMovement`,
`server/internal/materials/service.go:552`). Die einzige Verknüpfung zu
einer Bestellposition ist aktuell das freie Textfeld
`stock_movements.reference` — keine echte Fremdschlüsselbeziehung, damit
nicht zuverlässig für einen automatisierten Abgleich nutzbar.

Für Rechnungen gibt es bisher nur `accounting.ARService` (Accounts
Receivable, `invoices_out` — Ausgangsrechnungen an Kunden). Es gibt
KEINE `invoices_in`-Tabelle und keinen Accounts-Payable-Dienst — der
Backlog-Titel benennt explizit eine neue `/invoices-in`-Domäne.
`purchase_order_items` hat kein `tax_code`-Feld (anders als
`invoices_out`/`quote_items`) — Umsatzsteuer-Buchungslogik wurde für den
Bestellweg bisher nie gebraucht.

`invoices_out` demonstriert das etablierte GoBD-Muster für echte,
gebuchte Rechnungen (Storno-statt-Änderung, Festschreibung,
`journal_entries`-Anbindung, `062_invoices_out_storno.sql`) — das ist
deutlich umfangreicher als das, was für eine reine 3-Way-Match-PRÜFUNG
nötig ist (der Backlog-Titel nennt "-prüfung", kein Buchungs- oder
Freigabeworkflow).

## Optionen

**Option A — eigene `goods_receipts`/`goods_receipt_items`-Tabellen** für
den Wareneingang, komplett getrennt von `stock_movements`.
Verworfen: `stock_movements` bildet den physischen Wareneingang bereits
vollständig ab (inkl. Durchschnittspreis-Fortschreibung); eine zweite,
parallele Wareneingangs-Buchführung würde zwei Quellen der Wahrheit für
denselben Sachverhalt schaffen und mindestens zwei neue Tabellen mehr
benötigen als nötig.

**Option B — `stock_movements` um eine nullable
`purchase_order_item_id`-FK erweitern**, der Wareneingang für den 3-Way-
Match ergibt sich aus `SUM(quantity) WHERE purchase_order_item_id = X
AND movement_type='purchase'`. Gewählt.

**Option C — vollständiger GoBD-Buchungsworkflow für `invoices_in`**
(Festschreibung, Storno-Mechanismus, `journal_entries`-Anbindung, USt.-
Behandlung, Zahlungsstatus), analog zu `invoices_out`.
Verworfen für D.3: das wäre ein eigenständiges, deutlich größeres
Vorhaben (vergleichbar mit dem gesamten bestehenden `accounting`/AR-
Aufbau) und würde die Subtask-Größe aus §6.3 um ein Vielfaches
überschreiten. Der Backlog-Titel verlangt eine PRÜFUNG (Vergleich dreier
Datenquellen), keinen vollständigen Kreditorenbuchhaltungs-Workflow.

**Option D — `invoices_in` als reine Erfassung + reiner,
zustandsloser Lese-Abgleich** (`MatchInvoiceIn` vergleicht Bestellt/
Erhalten/Berechnet je Position, ohne den Match-Zustand zu persistieren
oder einen Freigabe-Workflow zu erzwingen). Gewählt (in Kombination mit
Option B).

**Zu Option D — Permission-Infrastruktur**: Wiederverwendung einer
bestehenden Permission (wie durchgängig in C.1-D.2) vs. neue
`invoices_in.read`/`invoices_in.write`?
Entscheidung: NEUE Permissions. Anders als C.1-D.2 (dort war die neue
Funktion jeweils eine Erweiterung einer bereits bestehenden Domäne -
Lager/Bestand bzw. Bestellwesen) ist `/invoices-in` laut Backlog-Titel
eine GENUIN NEUE Domäne ohne naheliegende bestehende Permission, die
fachlich passen würde (`purchase_orders.*` ist Bestellwesen,
`invoices_out.*` ist die AR-Gegenseite, nicht dasselbe wie AP). Analog
zur bereits bestehenden Trennung `invoices_out.read/write`.

**Zu Option D — Implementierungsort**: `purchasing.Service` (wie D.1/D.2)
vs. `accounting`-Paket (neue Datei)?
Entscheidung: `accounting`-Paket, neue Datei `accounting/ap.go`
(`APService`, Accounts Payable — Schwester-Konzept zum bestehenden
`ARService`). Eine Rechnung (ein- oder ausgehend) ist fachlich ein
Buchhaltungsdokument, unabhängig davon, ob sie an einen Bestellvorgang
gekoppelt ist — `accounting` ist der etablierte Ort für Rechnungs-
Domänenlogik in diesem Repo.

## Entscheidung

**Optionen B + D.**

- **`stock_movements.purchase_order_item_id`**: additive, nullable FK auf
  `purchase_order_items` (`ON DELETE SET NULL` — ein gelöschtes
  Bestellsystem-Objekt darf die physische Wareneingangs-Historie nicht
  mitreißen). `CreateMovement` selbst bleibt UNVERÄNDERT bis auf das neue,
  optionale Feld in `StockMovementCreate` — kein Zwang, es zu setzen (ein
  Wareneingang ohne PO-Bezug bleibt weiterhin möglich, wie bisher).
- **`invoices_in`** (Kopf): `id`, `company_id` (eigene Spalte, wie
  `invoices_out`/`purchase_orders` — top-level Dokument), `supplier_id`
  (FK `contacts`, `ON DELETE RESTRICT`), `purchase_order_id` (FK
  `purchase_orders`, NULLABLE — nicht jede Eingangsrechnung hat einen
  Bestellbezug, z. B. Miete/Nebenkosten), `invoice_number` (Rechnungs-
  nummer DES LIEFERANTEN, freier Text, keine eigene Nummernkreis-
  Vergabe — anders als `purchase_order`/`rfq`, das ist ein externes,
  vom Lieferanten vorgegebenes Dokument), `invoice_date`, `currency`,
  `status text DEFAULT 'erfasst'` (aktuell nur dieser eine Wert, kein
  CHECK — Buchungs-/Freigabe-Workflow ist explizit außerhalb des
  D.3-Scopes, siehe Konsequenzen), `note`, `created_at`.
- **`invoice_in_items`**: `id`, `invoice_in_id` (FK, `ON DELETE CASCADE`),
  `purchase_order_item_id` (FK, NULLABLE — nur damit matchbare Positionen
  haben einen Bezug), `description`, `qty numeric(18,6)`, `unit_price
  numeric(18,6)`, `currency`. KEIN `tax_code` (wie
  `purchase_order_items` selbst — USt.-Behandlung bewusst außerhalb des
  Scopes, siehe Konsequenzen).
- **`MatchInvoiceIn(invoiceInID, companyID)`**: reine Lese-/
  Berechnungsfunktion (kein Zustand wird persistiert). Für jede
  `invoice_in_item` MIT `purchase_order_item_id`: `bestellt` (aus
  `purchase_order_items.qty`/`.unit_price`), `erhalten` (`SUM(stock_
  movements.quantity) WHERE purchase_order_item_id=X`), `berechnet`
  (aus der Rechnungsposition selbst) — plus Boolesche Flags
  `MengeStimmt`/`PreisStimmt` (exakter Vergleich, keine Toleranzschwelle
  — dafür gibt es keine belegte fachliche Vorgabe, analog zur
  Entscheidung gegen eine erfundene Verrechnungsformel in ADR 0016).
  Positionen OHNE `purchase_order_item_id` erscheinen im Ergebnis mit
  einem expliziten "nicht zuordenbar"-Hinweis statt eines Abgleichs.
- **Keine Buchung, kein Storno, keine `journal_entries`-Anbindung, keine
  USt.-Behandlung, kein Zahlungsstatus** — bewusst außerhalb des Scopes
  (Option C verworfen).
- **Neue Permissions** `invoices_in.read`/`invoices_in.write`.
- **Implementierung in `accounting`-Paket**, neue Datei `accounting/ap.go`
  (`APService`). `ARService`/`JournalService`/`purchasing.Service`/
  `materials.CreateMovement` bleiben bis auf die eine additive Spalte
  komplett unangetastet.

## Konsequenzen

- **D.3.2** (Folge-Subtask): additive Migration `080_invoices_in.sql`
  (zwei neue Tabellen `invoices_in`/`invoice_in_items`, eine neue Spalte
  `stock_movements.purchase_order_item_id`, zwei neue Permissions),
  reversibel.
- **D.3.3** (Folge-Subtask): Anwendungscode — `StockMovementCreate` um
  das neue Feld ergänzen (`materials/service.go`, minimal), neue Datei
  `accounting/ap.go` (`APService`: `CreateInvoiceIn`/`GetInvoiceIn`/
  `ListInvoicesIn`/`MatchInvoiceIn`), HTTP-Wiring, Tests. Voraussichtlich
  groß genug (2 Pakete, neue Domäne, Match-Logik), um selbst in
  Micro-Subtasks zerlegt zu werden (§6.3) — Entscheidung darüber bei
  Erreichen von D.3.3.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von D.3, gehören
  voraussichtlich zu Epic E — Finanzwesen): vollständiger Buchungs-/
  Freigabe-/Zahlungsworkflow für `invoices_in`, USt.-Behandlung,
  Toleranzschwellen für den Mengen-/Preisabgleich, automatische
  Rechnungsprüfung beim Erfassen statt On-Demand-Abfrage.
- Kein Einfluss auf bestehende `invoices_out`-/`purchase_orders`-/
  `stock_movements`-Daten oder deren Lesepfade (die neue Spalte ist
  NULLABLE, bestehende Zeilen bleiben `NULL`).
