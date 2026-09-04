# ADR 0019 — Kostenstellenmodell

Datum: 2026-09-01
Status: entschieden (Subtask E.1.1)
Bezug: Backlog E.1 (Epic E — Finanzwesen, erste Task).

## Kontext

Es gibt aktuell KEINEN Kostenstellen-Begriff im Repo (per Suche über
`server/internal` bestätigt — kein Treffer für "Kostenstelle" außerhalb
zufälliger Teilstring-Überschneidungen mit "Kosten" in
`quotes.calculations`/Testdaten). Buchungen laufen über
`journal_entries`/`journal_lines` (`accounting/journal.go`,
`018_journal_and_ar.sql`) — `journal_lines` hat `account_code` (Soll/
Haben je Konto), aber kein Feld, das eine Buchungszeile einer
organisatorischen Kosteneinheit (Werkstatt, Verwaltung, Vertrieb usw.)
zuordnet.

`journal_lines` bekam in `067_accounts_company_scoped_pk.sql`
NACHTRÄGLICH eine eigene `company_id`-Spalte (der ursprüngliche Kommentar
in `058_accounting_scope.sql` — "erbt den Scope über `entry_id`, daher
keine eigene Spalte" — ist damit überholt; hier nur zur Kenntnis
genommen, kein Widerspruch, sondern spätere, bewusste Korrektur). Analog
zu `account_code` prüft `JournalService.create` die Existenz einer
Referenz NUR über die DB-Fremdschlüssel-Constraint, nicht per
Vorab-Query auf Mandanten-Zugehörigkeit — dieses bestehende, etablierte
Rigor-Niveau wird für ein neues, additives Feld übernommen, nicht
verschärft.

Epic E umfasst laut `docs/backlog.md` auch E.2 "Projektcontrolling
(Soll-Ist im Server)" — das ist die tatsächliche AUSWERTUNG/Reporting auf
Basis von Kostenstellen; E.1 selbst liefert laut Backlog-Titel nur DAS
MODELL (Stammdaten + Buchungs-Anbindung), keine Auswertungslogik.

## Optionen

**Option A — Kostenstellen als reines Feld auf `journal_lines`** (freier
Text statt eigener Stammdaten-Tabelle).
Verworfen: keine Stammdatenpflege, keine Aktiv/Inaktiv-Steuerung, keine
Eindeutigkeit — Tippfehler würden Kostenstellen faktisch duplizieren.
Jede andere Stammdaten-Domäne in diesem Repo (Materialien, Lieferanten,
Lagerorte) hat eine eigene Tabelle, keine freie Texteingabe.

**Option B — eigene `cost_centers`-Stammdatentabelle** (mandantenweit,
wie `materials`/`warehouses`) **+ additive, nullable
`journal_lines.kostenstelle_id`-FK**. Gewählt.

**Zu Option B — Granularität der Buchungs-Anbindung**: Kostenstelle auf
`journal_entries` (Kopf, EINE Kostenstelle je gesamter Buchung) vs.
`journal_lines` (je Buchungszeile)?
Entscheidung: `journal_lines` (Zeilenebene). Eine einzelne Buchung kann
mehrere Konten UND mehrere Kostenstellen gleichzeitig betreffen (z. B.
eine Rechnung, die Material für zwei verschiedene Werkstätten enthält) —
Kopf-Ebene wäre zu grob und würde die spätere Soll-Ist-Auswertung (E.2)
künstlich einschränken.
- **Kostenstellen-Zuordnung ist optional** (nullable) — nicht jede
  Buchung muss GoBD-technisch einer Kostenstelle zugeordnet werden;
  ein Zwang dazu wäre eine unbelegte fachliche Vorgabe.
- **Keine Mandanten-Zugehörigkeits-Vorabprüfung in `JournalService.create`**,
  konsistent mit der bestehenden Behandlung von `account_code` in
  derselben Funktion (nur DB-FK-Constraint) — keine neue Inkonsistenz
  innerhalb derselben Funktion einführen.

**Zu Option B — Implementierungsort**: neues Paket vs. `accounting`?
Entscheidung: `accounting`-Paket, neue Datei `accounting/cost_centers.go`
— Kostenstellen sind fachlich Rechnungswesen-Stammdaten, `accounting`
ist bereits der etablierte Ort für `ARService`/`APService`/
`JournalService`/`PaymentService`/`BankService`.

**Zu Option B — Permission-Infrastruktur**: neue Permissions vs.
Wiederverwendung?
Entscheidung: NEUE Permissions `cost_centers.read`/`cost_centers.write`
(zugewiesen an `role-finance` und `role-admin`) — es gibt keine
bestehende, fachlich passende Permission (Kontenrahmen/`accounts` hat
selbst noch kein HTTP-Wiring und damit keine Permission dafür), analog
zur bereits in D.3 getroffenen Entscheidung für `invoices_in.*`.

## Entscheidung

**Option B.**

- **`cost_centers`**: `id`, `company_id` (eigene Spalte, mandantenweite
  Stammdaten wie `materials`/`warehouses`), `code` (eindeutig je
  Mandant), `name`, `aktiv boolean DEFAULT true`, `note`, `created_at`.
- **`journal_lines.kostenstelle_id`**: additive, nullable FK auf
  `cost_centers`, `ON DELETE SET NULL` (eine gelöschte/deaktivierte
  Kostenstelle darf bestehende Buchungshistorie nicht mitreißen).
- **`JournalLineInput` bekommt `KostenstelleID *string`** — additiv,
  optional, `JournalService.create`/`CreateTx` bleiben strukturell
  unverändert bis auf das eine neue Feld in INSERT/Struct.
- **CRUD in `accounting/cost_centers.go`**: `CreateCostCenter`/
  `GetCostCenter`/`ListCostCenters`/`UpdateCostCenter` (Umbenennen,
  Aktiv/Inaktiv-Umschaltung) — kein `DeleteCostCenter` (Soft-Delete über
  `aktiv=false`, analog zu `materials.DeleteSoft`, da Kostenstellen nach
  Verwendung in Buchungen nicht mehr spurlos verschwinden dürfen).
- **Keine Auswertungs-/Reporting-Logik** (Soll-Ist-Vergleich, Aggregation
  je Kostenstelle/Projekt) — das ist explizit E.2, nicht E.1.
- **Neue Permissions** `cost_centers.read`/`cost_centers.write`.

## Konsequenzen

- **E.1.2** (Folge-Subtask): additive Migration `081_cost_centers.sql`
  (eine neue Tabelle, eine neue Spalte auf `journal_lines`, zwei neue
  Permissions), reversibel.
- **E.1.3** (Folge-Subtask): Anwendungscode — neue Datei
  `accounting/cost_centers.go` (CRUD), `journal.go` um `KostenstelleID`
  ergänzt (minimal), HTTP-Wiring, Tests. `JournalService.create`s
  bestehende Bilanzierungs-/Validierungslogik (Soll/Haben-Ausgleich)
  bleibt unangetastet.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von E.1,
  voraussichtlich E.2): Soll-Ist-Auswertung je Kostenstelle/Projekt,
  Kostenstellen-Hierarchien (Ober-/Unterkostenstellen), Budget je
  Kostenstelle.
- Kein Einfluss auf bestehende `journal_entries`-/`journal_lines`-/
  `accounts`-Daten oder deren Lesepfade (die neue Spalte ist NULLABLE).
