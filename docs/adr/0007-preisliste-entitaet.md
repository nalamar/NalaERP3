# ADR 0007 — Preisliste als eigene Entität (Gültigkeitszeiträume, Staffelpreise)

Datum: 2026-08-17
Status: entschieden (Subtask A.2.1)
Bezug: Backlog A.2 (Epic A — Stammdaten), aufgabe.md §1 (Domäne A:
"Preislisten" explizit als Kernumfang genannt). `docs/01-gap-analysis.md:18`
hält bereits fest: "Preislisten | rudimentär | nur
`materials.avg_purchase_price` + `quote_item_price_decisions`-Historie;
keine eigenständige Preisliste-Entität mit Gültigkeitszeiträumen/
Staffelpreisen".

## Kontext

Repo-weite Suche (Migrationen, Go, Flutter-Client) bestätigt: es existiert
**keine** Preisliste-Entität und **kein** Verkaufspreis-Konzept irgendwo im
Repo. `materials` (`server/internal/migrate/migrations/001_init.sql:3-20`)
trägt ausschließlich einkaufsseitige Werte
(`avg_purchase_price`/`currency`/`purchase_total_qty`/`purchase_total_value`,
ein laufender, transaktional fortgeschriebener Durchschnittspreis aus
Wareneingängen, siehe `server/internal/materials/service.go` `CreateMovement`).
Kein `sales_price`/`verkaufspreis`/`list_price` existiert.

Für Angebotspositionen (`quote_items`) existiert bereits eine ausgereifte,
mehrstufige Preisfindungs-Kette in `server/internal/quotes/service.go`
(gebaut in einer vorangegangenen, umfangreichen Session zum GAEB-Import,
siehe `docs/gaeb_price_*.md`):

1. **Letzter Bestellpreis** (`primaryPriceSourceForMaterialTx`,
   `service.go:2805-2830`) — aus `purchase_order_items`/`purchase_orders`.
2. **Durchschnittlicher Einkaufspreis** (`service.go:2833-2849`) — Fallback
   auf `materials.avg_purchase_price`.
3. **Zielmarge auf Kostenbasis** (`ApplyTargetUnitPriceForQuoteItem`,
   `service.go:1191ff`) — `quote_calculation_settings` (Migration
   `049_quote_calculation_settings.sql`) angewandt auf die zuletzt
   dokumentierte Kostenbasis.

Jede Entscheidung wird in `quote_item_price_decisions`
(`server/internal/migrate/migrations/048_quote_item_price_decisions.sql`,
`decision_type`-CHECK-Constraint zuletzt erweitert in Migration
`066_quote_item_price_decisions_target_price_type.sql`) protokolliert.

**Abgrenzung**: diese Kette ist bereits umfangreich getestet und produktiv
verdrahtet. A.2 fügt bewusst **keine** vierte Preisquelle in diese Kette
ein — das wäre eine Änderung an `quotes/service.go` (Domäne B, nicht A) und
würde die bestehende, funktionierende Logik ohne fachliche Notwendigkeit
anfassen (aufgabe.md §7.5: keine stillen Änderungen außerhalb der aktuellen
Subtask). A.2 baut ausschließlich die **Stammdaten-Entität** "Preisliste"
gemäß Domäne A; die spätere Integration als weitere Preisquelle für
Angebote ist explizit als eigene, neue Backlog-Position vorgesehen (siehe
Konsequenzen).

## Optionen

**Option A — Preisliste als Spalten direkt auf `materials`** (z. B.
`verkaufspreis`, `preis_gueltig_von`/`_bis`).
Verworfen: ein Material kann dann nur genau EINEN Verkaufspreis zu einem
Zeitpunkt haben — keine Staffelpreise (mehrere Preise nach Abnahmemenge),
keine mehreren parallelen/aufeinanderfolgenden Preislisten (z. B. je
Lieferant oder je Quartal). Widerspricht direkt dem Backlog-Titel "Preisliste
als EIGENE Entität".

**Option B — Eine einzige, flache Tabelle** (`price_lists` mit `material_id`,
`min_menge`, `unit_price`, `gueltig_von`/`_bis` direkt in derselben Zeile,
kein Header/Items-Split).
Verworfen: eine Preisliste ist konzeptionell ein Container mit vielen
Zeilen (viele Materialien, je mit ggf. mehreren Staffeln) unter einem
gemeinsamen Namen/einer gemeinsamen Gültigkeit. Ohne Header-Tabelle müsste
jede Zeile ihre Gültigkeit einzeln tragen — Redundanz und Inkonsistenzrisiko
bei Änderungen (z. B. Verlängerung der Gültigkeit einer ganzen Liste würde
ein Massen-Update aller Zeilen statt einer Header-Zeile erfordern).

**Option C — Zwei Tabellen: `price_lists` (Header) + `price_list_items`
(Zeilen, Staffelpreise).** Gewählt.

## Entscheidung

**Option C.**

- **`price_lists`** (Header, mandantengescoped): `id` (text, wie
  `materials.id` — Domäne A, konsistent mit dem Stammdaten-Muster, nicht mit
  dem `UUID`-Typ aus der `quotes`-Domäne), `company_id` (text NOT NULL,
  direkt von Anfang an NOT NULL statt Expand-Contract — anders als bei den
  0.2.1.2-Migrationen gibt es hier keine bestehenden Zeilen, die
  nachträglich befüllt werden müssten, da die Tabelle neu ist), `name`
  (Bezeichnung der Liste, z. B. "Schüco Preisliste 2026 Q1"), `lieferant`
  (freier Text, DEFAULT '' — bewusst KEINE FK auf eine Lieferanten-Tabelle,
  da Backlog A.3 "Systemlieferanten-Konzept" diese Struktur erst einführt;
  A.2 nimmt A.3 nicht vorweg, gleiches Muster wie `profilserie` in ADR
  0006), `currency` (char(3) NOT NULL DEFAULT 'EUR' — EINE Währung pro
  Liste, nicht pro Zeile, da eine veröffentlichte Preisliste eines
  Lieferanten typischerweise durchgängig in einer Währung geführt wird),
  `gueltig_von` (date NOT NULL), `gueltig_bis` (date NULL = unbefristet
  gültig), `aktiv` (boolean NOT NULL DEFAULT true, analog `materials.aktiv`
  für Soft-Delete), `created_at`/`updated_at`.
- **`price_list_items`** (Zeilen, Staffelpreise, Scope über `price_list_id`
  geerbt — kein eigenes `company_id`, analog zu `quote_items`/
  `sales_order_items` laut ADR 0002): `id` (text), `price_list_id` (FK →
  `price_lists(id) ON DELETE CASCADE`), `material_id` (FK → `materials(id)
  ON DELETE CASCADE` — eine Preislistenzeile ohne zugehöriges Material ist
  bedeutungslos, harte statt weiche Kaskade), `min_menge` (numeric(18,6) NOT
  NULL DEFAULT 0 — "ab dieser Menge gilt dieser Preis", klassisches
  Staffelmodell), `unit_price` (numeric(18,4) NOT NULL, Genauigkeit
  konsistent mit `quote_item_price_decisions.applied_unit_price`),
  `created_at`. `UNIQUE (price_list_id, material_id, min_menge)` verhindert
  doppelte Staffeln für dieselbe Menge; `CHECK (min_menge >= 0)`, `CHECK
  (unit_price >= 0)`.
- `CHECK (gueltig_bis IS NULL OR gueltig_bis >= gueltig_von)` auf
  `price_lists`.
- **Lookup-Logik** ("welcher Preis gilt für Material X bei Menge Y am Datum
  Z, wenn mehrere aktive/gültige Preislisten existieren") wird bewusst
  NICHT in dieser ADR festgelegt — sie ist Teil des Anwendungscodes
  (Subtask A.2.3) und hängt von Fragen ab, die erst dort konkret werden
  (z. B. Priorisierung bei mehreren gleichzeitig gültigen Listen). Die
  Grundregel für Staffelpreise selbst ist aber bereits hier klar: die Zeile
  mit der größten `min_menge <= bestellte Menge` gewinnt (Standard-Staffel-
  semantik).

## Konsequenzen

- **A.2.2** (Folge-Subtask): additive Migration `069_...sql` (zwei neue
  Tabellen, keine bestehenden Tabellen betroffen — daher reversibel via
  einfaches `DROP TABLE`, kein Datenverlustrisiko für bereits vorhandene
  Fachdaten).
- **A.2.3** (Folge-Subtask): neue Datei im bestehenden `materials`-Paket
  (`server/internal/materials/price_lists.go` o. ä. — Preislisten sind
  Stammdaten wie Materialien/Lager, kein eigenes Package nötig), CRUD für
  `price_lists`/`price_list_items`, ein Lookup-Helfer für "effektiver Preis
  bei gegebener Menge/gegebenem Datum", HTTP-Wiring, Tests.
- **Neue, separate Backlog-Position** (nicht Teil von A.2, bei Bedarf später
  anzulegen): Integration der Preisliste als vierte Preisquelle in
  `quotes.Service.primaryPriceSourceForMaterialTx`/
  `PriceSourcePriorityForQuoteItem` (neuer `decision_type`-Wert
  `'price_list_applied'`, Erweiterung des CHECK-Constraints analog zu
  Migration 066) — bewusst NICHT Teil dieser ADR/dieses Backlog-Items, da
  Domäne B (Angebotswesen), nicht Domäne A (Stammdaten).
- **A.3** (Systemlieferanten-Konzept) kann `price_lists.lieferant` später
  additiv um eine FK/Lookup-Beziehung ergänzen, ohne diese ADR zu
  widersprechen (Text bleibt als Anzeigename/Fallback nutzbar) — exakt
  dasselbe Muster wie bei `materials.profilserie` (ADR 0006).
- Kein Einfluss auf bestehende Daten, Tabellen oder Endpunkte.
