# ADR 0017 — Anfrageprozess (RFQ) vor Bestellung

Datum: 2026-08-27
Status: entschieden (Subtask D.2.1)
Bezug: Backlog D.2 (Epic D — Bestellwesen, zweite Task, nach D.1).

## Kontext

`purchasing.Service` (`server/internal/purchasing/service.go`) verwaltet
bereits `PurchaseOrder`/`PurchaseOrderItem` — ein eigenständiges,
mandantenweites Dokument mit eigener `company_id`-Spalte (nicht über eine
andere Tabelle geerbt, anders als die Lager-/Bestandstabellen aus C.1-C.3)
und eigenem Nummernkreis (`settings.NumberingService`, Entity
`purchase_order`). `purchase_orders.status` hat bewusst KEINEN
DB-CHECK-Constraint — die Werteliste (`draft, ordered, received,
canceled`) wird ausschließlich im Anwendungscode über `Statuses()`/`isIn()`
geprüft (`purchasing/service.go:32-51`).

`contacts.Rolle` kennt `customer | supplier | partner | both | other`
(`contacts/service.go:181`); `purchasing.Create` prüft AKTUELL selbst
NICHT, ob `SupplierID` tatsächlich eine Lieferantenrolle hat — nur, dass
das Feld nicht leer ist (`purchasing/service.go:109`). Ein eigener,
strengerer Rollen-Check existiert bereits, aber nur privat und in einem
anderen Paket (`contacts.Service.ensureSupplierRole`,
`contacts/system_suppliers.go:35`, genutzt von A.3
Systemlieferanten/Profilserien) — nicht von `purchasing` aus aufrufbar,
ohne eine neue Paket-Kopplung einzuführen.

Es gibt aktuell KEINEN Anfrage-/RFQ-Begriff — ein Einkauf mündet direkt in
eine `PurchaseOrder`, ohne die Möglichkeit, vorher mehrere Lieferanten um
Preise zu bitten und zu vergleichen.

## Optionen

**Option A — Gewinner je Position (Splitting über mehrere Lieferanten)**:
je `rfq_item` kann ein anderer Lieferant gewinnen, eine Anfrage kann sich
beim Abschluss in MEHRERE Bestellungen (eine je beteiligtem Lieferanten)
aufteilen.
Verworfen: deutlich höhere Komplexität (Gruppierung der Gewinner-Zeilen
nach Lieferant, mehrere `PurchaseOrder`s aus einer Anfrage, unklare
Nummernkreis-/Referenzlogik) ohne belegten Bedarf dafür im
Backlog-Titel ("Anfrageprozess vor Bestellung", Singular) — in einem
kleinen bis mittleren Metallbaubetrieb ist die Vergabe einer gesamten
Anfrage an EINEN Lieferanten der Normalfall. Würde die Subtask deutlich
über die ~8-Datei/~400-Zeilen-Grenze aus §6.3 treiben.

**Option B — ein Gewinner-Lieferant je gesamter Anfrage**: alle
Positionen einer Anfrage werden gemeinsam an genau einen Lieferanten
vergeben, der für ALLE Positionen der Anfrage geantwortet haben muss.
Umwandlung erzeugt genau EINE neue `PurchaseOrder`. Gewählt.

**Zu Option B — Speicherung von Mandant/Nummernkreis**: `rfqs` erbt Scope
über eine andere Tabelle (wie `stock_reservations` in C.1) vs. eigene
`company_id`-Spalte?
Entscheidung: eigene `company_id`-Spalte, analog zu `purchase_orders`
selbst (beides eigenständige, mandantenweite Dokumente auf derselben
fachlichen Ebene — kein Kind einer anderen Tabelle wie `warehouse_id` in
C.1-C.3). Eigener Nummernkreis (`settings.NumberingService`, Entity
`rfq`), analog zu `purchase_order`.

**Zu Option B — Status-Wertebereich per DB-CHECK erzwingen**: wie bei den
meisten neuen Spalten dieser Session (z. B. B.4.2) vs. wie beim direkten
Nachbarn `purchase_orders.status` (bewusst OHNE CHECK, Validierung nur im
Anwendungscode)?
Entscheidung: KEIN DB-CHECK, konsistent mit dem unmittelbaren
Schwester-Dokument `purchase_orders` in DERSELBEN Domäne — Konsistenz
innerhalb der Bestellwesen-Tabellenfamilie wiegt hier stärker als das
sonst in dieser Session übliche Muster (das primär für die Lager-/
Bestandsdomäne C.1-C.3/D.1 etabliert wurde).

**Zu Option B — Lieferantenrollen-Prüfung**: strengen Rollen-Check
(`supplier`/`both`) beim Registrieren einer Lieferanten-Offerte erzwingen
vs. keine Prüfung, analog zu `purchasing.Create`?
Entscheidung: keine Prüfung. `purchasing.Create` (die bestehende, direkte
Nachbarfunktion, in die eine RFQ am Ende mündet) prüft die Rolle selbst
NICHT — eine strengere Prüfung nur für RFQ wäre eine Inkonsistenz
innerhalb derselben Domäne und würde vorbestehendes Verhalten
verschärfen, ohne dass das Teil dieser Subtask wäre. Vorbestehende
Lücke, nicht im Rahmen von D.2 behoben (eigene, mögliche künftige
Backlog-Position, die dann konsequenterweise auch `purchasing.Create`
mit erfassen sollte).

**Zu Option B — mehrfache Offerten desselben Lieferanten für dieselbe
Position**: neue Zeile je Offerte (Historie) vs. Upsert (nur die
aktuellste Offerte zählt)?
Entscheidung: Upsert (`UNIQUE (rfq_item_id, supplier_id)`,
`ON CONFLICT DO UPDATE`). Eine Lieferantenkorrektur ("mein Preis war
falsch, hier der richtige") ersetzt die vorherige Offerte — anders als
GoBD-relevante Buchhaltungsbelege (Storno-Pflicht) ist eine Anfrage-Offerte
vor jeder Bestellung eine reine, noch unverbindliche Verhandlungsangabe;
Historie ist hier fachlich nicht gefordert.

## Entscheidung

**Option B.**

- **`rfqs`** (Anfrage-Header): `id`, `company_id` (eigene Spalte, wie
  `purchase_orders`), `nummer` (Autonummer, Entity `rfq`), `status text
  DEFAULT 'offen'` (`offen`/`abgeschlossen`/`storniert`, Validierung nur im
  Anwendungscode, KEIN DB-CHECK — Konsistenz mit `purchase_orders`),
  `note`, `created_at`, `closed_at` (NULL bis Abschluss/Stornierung).
- **`rfq_items`**: `id`, `rfq_id` (FK, `ON DELETE CASCADE`), `position`,
  `material_id` (FK, `ON DELETE RESTRICT` wie `purchase_order_items`),
  `qty numeric(18,6)`, `uom`, `description` — Spaltenset bewusst identisch
  zu `purchase_order_items` (ohne Preis, da der Preis erst durch die
  Lieferanten-Offerte entsteht).
- **`rfq_supplier_quotes`**: `id`, `rfq_item_id` (FK, `ON DELETE CASCADE`),
  `supplier_id` (FK auf `contacts`, `ON DELETE RESTRICT`), `unit_price
  numeric(18,6)`, `currency char(3) DEFAULT 'EUR'`, `delivery_date`
  (nullable), `note`, `quoted_at`. `UNIQUE (rfq_item_id, supplier_id)` für
  das Upsert-Verhalten.
- **`RegisterSupplierQuote`**: legt eine Lieferanten-Offerte für eine
  Position an oder aktualisiert sie (Upsert); lehnt ab, wenn die
  zugehörige Anfrage nicht mehr `offen` ist (Festschreibung nach Abschluss/
  Stornierung, wie bei `sales_order_addenda`/Angeboten etabliert). Keine
  Lieferantenrollen-Prüfung (siehe Optionen).
- **`ConvertToPurchaseOrder(rfqID, supplierID)`**: lehnt ab, wenn die
  Anfrage nicht `offen` ist ODER der gewählte Lieferant NICHT für JEDE
  Position der Anfrage eine Offerte abgegeben hat (eine Bestellposition
  ohne Preis wäre unvollständig). Erzeugt transaktional GENAU EINE neue
  `PurchaseOrder` (Wiederverwendung der bestehenden `purchasing.Create`-
  Logik/-Tabellen, `unit_price` je Position aus der jeweiligen
  `rfq_supplier_quotes`-Zeile), setzt `rfqs.status='abgeschlossen'`,
  `closed_at=now()`.
- **`CancelRFQ`**: `status: offen → storniert` (Festschreibung, keine
  weiteren Offerten danach), unabhängig davon, ob bereits Offerten
  vorliegen.
- **Implementierung in `purchasing.Service`** (kein neues Paket) — RFQ ist
  fachlich derselben Bestellwesen-Domäne zugehörig wie `PurchaseOrder`.
- **Keine neue Permission-Infrastruktur** — Wiederverwendung von
  `purchase_orders.read`/`purchase_orders.write`.

## Konsequenzen

- **D.2.2** (Folge-Subtask): additive Migration `079_rfqs.sql` (drei neue
  Tabellen, keine bestehende Tabelle geändert; neuer `number_sequences`-
  Eintrag für Entity `rfq`), reversibel.
- **D.2.3** (Folge-Subtask): Anwendungscode nur in `purchasing/service.go`
  bzw. einer neuen Datei `purchasing/rfq.go`
  (`CreateRFQ`/`RegisterSupplierQuote`/`ListSupplierQuotes`/
  `ConvertToPurchaseOrder`/`CancelRFQ`), HTTP-Wiring, Tests.
  `purchasing.Create`/`Get`/`List`/`Update` bleiben komplett unangetastet.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von D.2):
  Gewinner-Splitting über mehrere Lieferanten je Anfrage (Option A oben),
  Lieferantenrollen-Prüfung sowohl in `RegisterSupplierQuote` als auch
  rückwirkend in `purchasing.Create`, automatischer Versand der Anfrage
  per E-Mail an Lieferanten.
- Kein Einfluss auf bestehende `purchase_orders`-/`purchase_order_items`-
  Daten oder deren Lesepfade.
