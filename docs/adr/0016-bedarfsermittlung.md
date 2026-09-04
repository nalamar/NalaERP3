# ADR 0016 — Bedarfsermittlung aus Angebot/Mindestbestand

Datum: 2026-08-26
Status: entschieden (Subtask D.1.1)
Bezug: Backlog D.1 (Epic D — Bestellwesen, erste Task).

## Kontext

`materials` hat aktuell KEIN Feld für einen Mindestbestand — es gibt
keinerlei Mechanismus, der einen zu niedrigen Lagerbestand automatisch
erkennt. `quote_items.material_id` (seit A.1,
`047_quote_item_manual_mapping.sql`) verknüpft Angebotspositionen mit
Materialien; `quotes.status` kennt die Werte `draft, sent, accepted,
rejected` (`quotes/service.go:3107`). `sales_order_items` hat dagegen KEIN
`material_id` (bereits in ADR 0013 festgestellt) — nach Umwandlung eines
Angebots in einen Auftrag bleibt `quote_items` daher die EINZIGE Quelle
für materialbezogenen Bedarf, auch für bereits akzeptierte/umgewandelte
Angebote.

Der bestehende Lagerbestand ist mengenbasiert (`stock_movements`, C.1/C.2)
mit optionalen Reservierungen (`stock_reservations`, C.1) — "verfügbare"
Menge (physischer Bestand minus aktive Reservierungen) ist bereits als
Konzept etabliert (`availableStockTx`, `materials/service.go`), hier aber
material- statt lagerweit benötigt (Mindestbestand ist eine
Artikel-Eigenschaft, kein Lager-Attribut).

`purchasing.Service` (`server/internal/purchasing/service.go`) verwaltet
bereits `PurchaseOrder`/`PurchaseOrderItem` (mit `material_id`) — das ist
der etablierte Ort für alles, was direkt der Beschaffung dient
("Bestellwesen"), analog dazu, wie C.1-C.3 durchgängig in
`materials.Service` lebten, weil sie Lager-/Bestandsthemen waren.

## Optionen

**Option A — eine einzige, kombinierte Bedarfszahl je Material** (Formel
aus Angebots-Bedarf, Mindestbestand-Soll und verfügbarem Bestand, mit
Verrechnung/Netting zwischen den beiden Quellen).
Verworfen: es gibt keine belegte fachliche Vorgabe, WIE die beiden
Quellen zu verrechnen sind (z. B. ob Angebots-Bedarf gegen vorhandenen
Bestand genettet werden soll, oder ob vorhandener Bestand zuerst für den
Mindestbestand reserviert gilt). Eine erfundene Verrechnungsformel wäre
eine unbelegte fachliche Annahme (aufgabe.md §7.1) — der Backlog-Titel
nennt zwei Quellen ("aus Angebot/Mindestbestand"), keine vorgegebene
Kombinationsregel.

**Option B — zwei getrennte, je für sich eindeutig definierte
Bedarfs-Sichten**: (1) Mindestbestand-Unterdeckung je Material
(verfügbarer Bestand < konfiguriertes Soll), (2) Bedarf aus offenen
Angeboten je Material (Summe der Mengen aus `quote_items` offener
Angebote) — beide unabhängig voneinander lesbar, keine automatische
Verrechnung. Gewählt.

**Zu Option B — was zählt als "offenes" Angebot für die Bedarfsermittlung**:
nur `sent` (beim Kunden, Entscheidung offen) vs. `sent` UND `accepted`?
Entscheidung: `sent` UND `accepted`. Ein akzeptiertes Angebot ist eine
REALE, vom Kunden bestätigte Zusage — der Materialbedarf dafür ist
mindestens genauso relevant wie bei einem noch unentschiedenen Angebot.
`draft` (noch nicht einmal beim Kunden) und `rejected` (hinfällig) zählen
NICHT. Da `sales_order_items` kein `material_id` hat (s. Kontext), bleibt
`quote_items` selbst nach Umwandlung in einen Auftrag die einzig
verfügbare Quelle — es gibt keine Möglichkeit, "bereits umgewandelte"
Angebote gesondert zu behandeln, ohne diese Quelle zu verlieren.

**Zu Option B — Mindestbestand-Granularität**: je Lager vs. lagerübergreifend
je Material?
Entscheidung: lagerübergreifend je Material (ein Feld `mindestbestand` auf
`materials`, nicht auf einer Lager-Material-Kombination). Mindestbestand
ist fachlich eine Artikel-Eigenschaft ("wie viel wollen wir von Material X
insgesamt nie unterschreiten"), keine reine Lagerplatz-Eigenschaft — das
entspricht auch der bereits bestehenden Struktur von `materials` als
alleinige, mandantenweite Artikel-Stammdaten-Tabelle (ADR 0002).
Verfügbarer Bestand wird dafür über ALLE Lager des Mandanten aggregiert
(`SUM(stock_movements.quantity) - SUM(stock_reservations.qty WHERE
status='aktiv')`, materialweit statt lagerweit wie bei
`availableStockTx`).

**Zu Option B — Implementierungsort**: `materials.Service` (wie C.1-C.3)
vs. `purchasing.Service`?
Entscheidung: `purchasing.Service`, NEUE Datei `purchasing/demand.go`.
Das neue Mindestbestand-FELD (`materials.mindestbestand`) wird als
additive Spalte auf `materials` ergänzt (dort ist der bestehende Ort für
Artikel-Stammdaten), die BERECHNUNG von Bedarf ist aber fachlich
Beschaffungslogik ("was muss eingekauft werden") — Direkt-SQL gegen
`materials`/`stock_movements`/`stock_reservations`/`quotes`/`quote_items`
aus `purchasing.Service` heraus, ohne neue Kopplung zwischen den
Go-Paketen (gleiches Muster wie B.4.1: Direkt-SQL statt neuer
Paket-Schnittstelle, um die Blast-Radius klein zu halten).

## Entscheidung

**Option B.**

- **`materials.mindestbestand`**: neue, additive Spalte `numeric(18,6)`,
  NULLABLE (NULL = kein Mindestbestand konfiguriert, Default-Zustand für
  alle bestehenden UND neuen Materialien — kein automatischer Bedarf ohne
  explizite Konfiguration), `CHECK (mindestbestand IS NULL OR
  mindestbestand >= 0)`. Feld wird wie `rc_klasse`/`u_wert` (A.1.3) additiv
  in `Material`/`MaterialCreate`/`MaterialUpdate` aufgenommen — bestehende
  `POST`/`PATCH /materials`-Routen brauchen kein neues HTTP-Wiring
  (generisches Struct-Passthrough, bereits mehrfach in dieser Session so
  gehandhabt).
- **`purchasing.MinStockShortfalls(ctx, companyID)`**: liefert je Material
  mit gesetztem `mindestbestand` UND `verfügbar < mindestbestand` die
  Unterdeckung (`shortfall_qty = mindestbestand - verfügbar`). Materialien
  ohne Unterdeckung oder ohne konfigurierten Mindestbestand erscheinen
  NICHT in der Liste.
- **`purchasing.QuoteDemand(ctx, companyID)`**: liefert je Material die
  Summe der `qty` aus `quote_items` aller Angebote mit `status IN ('sent',
  'accepted')` und gesetztem `material_id`. Keine Verrechnung gegen
  vorhandenen Bestand (siehe Optionen) — die reine Bedarfssumme.
- **Keine Schreiblogik** — D.1 legt nur Lese-/Berechnungspfade an, keine
  automatische Bestellvorschlagserzeugung (das wäre eine eigene,
  weiterführende Fachlogik, ggf. spätere Backlog-Position, die auf D.1
  aufbaut).
- **HTTP-Wiring**: neue Route-Gruppe `/purchasing/demand`
  (`GET /min-stock-shortfalls`, `GET /quote-demand`) — Wiederverwendung
  von `purchase_orders.read`, keine neue Permission-Infrastruktur.
- **`CreateMovement`/`StockByMaterial`/`availableStockTx`/
  `purchasing.Create` bleiben unangetastet** — rein additive, lesende
  Ergänzung.

## Konsequenzen

- **D.1.2** (Folge-Subtask): additive Migration
  `078_materials_mindestbestand.sql` (eine neue Spalte + CHECK auf
  `materials`, keine andere Tabelle geändert), reversibel.
- **D.1.3** (Folge-Subtask): Anwendungscode in zwei Dateien —
  `materials/service.go` (Feld + Validierung, minimal) und NEU
  `purchasing/demand.go` (`MinStockShortfalls`/`QuoteDemand`), HTTP-Wiring,
  Tests.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von D.1):
  automatische Bestellvorschläge/-vorlagen aus den beiden Bedarfs-Sichten,
  Mindestbestand je Lager statt lagerübergreifend, Verrechnung der beiden
  Bedarfsquellen gegeneinander (sobald eine fachliche Vorgabe dafür
  existiert).
- Kein Einfluss auf bestehende `materials`-/`stock_movements`-/
  `stock_reservations`-/`quotes`-Daten oder deren Lesepfade.
