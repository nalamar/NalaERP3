# ADR 0020 — Projektcontrolling (Soll-Ist im Server)

Datum: 2026-09-03
Status: entschieden (Subtask E.2.1)
Bezug: Backlog E.2 (Epic E — Finanzwesen, zweite Task, nach E.1).

## Kontext

`buildProjectCommercialContext` (`server/internal/http/commercial_context.go`)
existiert bereits — aggregiert Angebote/Aufträge/Ausgangsrechnungen eines
Projekts zu Summen (`QuoteGrossTotal`, `SalesOrderGrossTotal`,
`InvoiceGrossTotal`, `InvoiceOpenTotal`). Das ist bereits Server-seitige
Aggregation, aber REINE ERLÖS-Betrachtung (Soll = Angebotssumme, "Ist" nur
im Sinne von "schon in Rechnung gestellt") — es gibt KEINE Kosten-Seite
und damit keinen echten Deckungsbeitrag/Soll-Ist-Vergleich.

**Wichtigster Recherchebefund**: Es gibt AKTUELL KEINE strukturierte
Verknüpfung zwischen Projekten und tatsächlich angefallenen Kosten:
- `purchase_orders` hat KEIN `project_id`-Feld (per Migrationssuche
  bestätigt) — Bestellungen sind nicht zu Projekten rückverfolgbar.
- `hr` kennt `Employee`/`Team`/`LeaveRequest` (Urlaub) — es gibt KEINE
  Zeiterfassung/Lohnbuchung, die einem Projekt zugeordnet werden könnte.
- `stock_movements` hat kein `project_id`.
- Die einzige BEREITS VORHANDENE Kosten-Seite mit belastbaren Zahlen ist
  `quote_item_calculations` (B.3, ADR 0011) — eine bottom-up Kalkulation
  je Angebotsposition (Material/Lohn/Fremdleistung getrennt, je EIGENEM
  Zuschlag), aber das ist SOLL (kalkuliert), nicht IST (tatsächlich
  gebucht). Die Summenfelder (`material_total` usw.) werden dort bewusst
  NICHT in der DB gespeichert, sondern nur im Anwendungscode aus den
  Rohwerten abgeleitet (`073_quote_item_calculations.sql`, Kommentar).
- E.1 (`docs/adr/0019-kostenstellenmodell.md`) hat gerade `cost_centers`
  und `journal_lines.kostenstelle_id` eingeführt — GENAU dafür vorgesehen
  ("Soll-Ist-Auswertung je Kostenstelle/Projekt" war dort explizit als
  künftige, E.2 zugehörige Position vermerkt), aber `journal_lines` selbst
  hat KEINE `project_id` — die Verknüpfung müsste über eine Kostenstelle
  laufen, und PROJEKTE sind aktuell NICHT mit Kostenstellen verknüpft.

Ohne eine Projekt↔Kostenstelle-Verknüpfung gibt es schlicht KEINE
belastbare "Ist-Kosten"-Quelle für ein Projekt — das ist die zentrale
Lücke, die E.2 zuerst schließen muss, bevor überhaupt ein Soll-Ist-
Vergleich möglich ist.

## Optionen

**Option A — Ist-Kosten aus einer neuen, direkten
`journal_lines.project_id`-Spalte** (statt über eine Kostenstelle).
Verworfen: würde die gerade in E.1 bewusst getroffene Entscheidung
(Kostenstellen als die zentrale Kosten-Zuordnungs-Einheit) umgehen und
zwei parallele Zuordnungsmechanismen auf derselben Tabelle schaffen
(`kostenstelle_id` UND `project_id`) — Redundanz statt der in E.1 bereits
vorbereiteten Struktur.

**Option B — `projects.kostenstelle_id`** (additive, nullable FK auf
`cost_centers`, analog zum bereits etablierten 1:1-Verknüpfungsmuster)
**+ Aggregation über bestehende Daten** (`quote_item_calculations` für
Soll-Kosten, `journal_lines` gefiltert nach der Projekt-Kostenstelle für
Ist-Kosten, bestehende Erlös-Aggregation aus `buildProjectCommercialContext`
wiederverwendet). Gewählt.

**Zu Option B — Verknüpfung ist optional**: Zwang zur Kostenstellen-
Zuordnung bei jedem Projekt vs. optional?
Entscheidung: optional (nullable). Nicht jedes Projekt braucht formales
Kosten-Controlling (kleine Projekte, Bestandsprojekte ohne rückwirkende
Zuordnung) — ein Zwang wäre eine unbelegte fachliche Vorgabe. Ist keine
Kostenstelle zugeordnet, liefert die Ist-Kosten-Berechnung explizit
`null` mit einem Hinweis, NICHT `0` (0 würde fälschlich "keine Kosten
angefallen" suggerieren statt "nicht messbar").

**Zu Option B — Soll-Kosten-Berechnung**: `quote_item_calculations`-Summen
in Go nachbilden (Export der bisher privaten `computeCalculationTotals`
aus `quotes` heraus, neue Paket-Kopplung) vs. dieselbe, stabile Formel
direkt als SQL-Ausdruck?
Entscheidung: SQL-Ausdruck (`material_cost * (1+material_zuschlag_percent/100)
+ lohn_stunden*lohn_stundensatz*(1+lohn_zuschlag_percent/100) +
fremdleistung_cost*(1+fremdleistung_zuschlag_percent/100)`, jeweils
multipliziert mit `quote_items.qty` und aufsummiert). Die Formel ist
bereits seit B.3.3 stabil und per Unit-Test exakt nachgerechnet; eine
direkte SQL-Aggregation vermeidet eine neue Cross-Package-Abhängigkeit
für eine einzelne, kleine Berechnung (gleiches Prinzip wie D.1/D.3:
Direkt-SQL statt neuer Paket-Schnittstelle). Nur Positionen aus NICHT
überholten Angebotsrevisionen zählen (`quotes.superseded_by_quote_id IS
NULL`) — dasselbe Filterkriterium, das `quotes.Service.List` bereits
standardmäßig anwendet (Konsistenz mit der bestehenden Erlös-Aggregation
in `buildProjectCommercialContext`, die über `quoteSvc.List` läuft).

**Zu Option B — Ist-Kosten-Berechnung**: alle `journal_lines` der
Projekt-Kostenstelle vs. nur Aufwandskonten?
Entscheidung: nur Aufwandskonten (`accounts.type='expense'`), Netto-Soll
(`SUM(debit) - SUM(credit)`, Aufwand steht buchhalterisch im Soll) — eine
Kostenstelle könnte theoretisch auch auf Bestands-/Ertragskonten bebucht
werden, das wäre aber keine "Kosten" im Sinne von Projektcontrolling.

**Zu Option B — Implementierungsort**: `projects.Service` (neue,
service-eigene Funktion) vs. `http`-Paket (wie das bereits bestehende
`buildProjectCommercialContext`)?
Entscheidung: `http`-Paket, neue Datei `http/project_controlling.go` —
konsistent mit dem bereits etablierten, funktionierenden Präzedenzfall
`commercial_context.go` für exakt diese Art von Cross-Domain-Aggregation
(projects+quotes+accounting+cost_centers); kein neues architektonisches
Muster einführen, wo bereits ein funktionierendes existiert.

## Entscheidung

**Option B.**

- **`projects.kostenstelle_id`**: additive, nullable FK auf
  `cost_centers`, `ON DELETE SET NULL`.
- **`GET /projects/{id}/controlling`**: neuer Endpunkt, liefert:
  - `soll_erloes`/`ist_erloes` (wiederverwendet die bereits bestehende
    Aggregation aus `buildProjectCommercialContext` bzw. deren
    Bausteine `quoteSvc.List`/`listProjectInvoices`),
  - `soll_kosten` (SQL-Aggregation über `quote_item_calculations` ×
    `quote_items.qty`, nur nicht überholte Angebotsrevisionen),
  - `ist_kosten` (SQL-Aggregation über `journal_lines` gefiltert auf
    `kostenstelle_id = projects.kostenstelle_id` UND
    `accounts.type='expense'`; `null` statt `0`, wenn keine Kostenstelle
    zugeordnet ist),
  - `deckungsbeitrag_soll`/`deckungsbeitrag_ist` (einfache Differenz
    Erlös−Kosten, keine neue Datenquelle).
- **Keine Schreiblogik außer der neuen `kostenstelle_id`-Zuordnung**: `projects.Service`
  hat KEINEN generischen Update-Pfad (nur die enge, bereits bestehende
  `UpdateStatus`) — für die Zuordnung wird eine neue, ebenso enge
  Funktion `SetKostenstelle` ergänzt, demselben Muster folgend, statt
  einen generischen `ProjectUpdate` einzuführen, den E.2 gar nicht
  braucht. E.2 liefert sonst nur Auswertung, keinen neuen Buchungs-/
  Planungsworkflow.
- **`quote_item_calculations`/`journal_lines`/`cost_centers`-Schreibpfade
  bleiben komplett unangetastet** — rein lesende, zusätzliche Aggregation.

## Konsequenzen

- **E.2.2** (Folge-Subtask): additive Migration `082_projects_kostenstelle.sql`
  (eine neue Spalte auf `projects`), reversibel.
- **E.2.3** (Folge-Subtask): Anwendungscode — neue, enge Funktion
  `projects.Service.SetKostenstelle` (analog zu `UpdateStatus`), neue Datei
  `http/project_controlling.go` (Soll-Ist-Aggregation), HTTP-Wiring,
  Tests. Voraussichtlich groß genug (mehrere Datenquellen, neue
  Aggregationslogik) für eine eigene Größeneinschätzung/ggf.
  Micro-Subtask-Zerlegung bei Erreichen von E.2.3 (analog zu D.3.3).
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von E.2):
  Zeiterfassung/Lohnbuchung mit Projektbezug (würde eine echte,
  vollständige Ist-Lohnkosten-Quelle liefern, aktuell nicht vorhanden),
  Bestellungen mit Projektbezug (`purchase_orders.project_id`),
  Budget-/Plankosten-Verwaltung unabhängig von Angebotskalkulationen.
- Kein Einfluss auf bestehende `projects`-/`quotes`-/`journal_lines`-/
  `cost_centers`-Daten oder deren Lesepfade (die neue Spalte ist
  NULLABLE).
