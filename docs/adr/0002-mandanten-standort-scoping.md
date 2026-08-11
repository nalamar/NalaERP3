# ADR 0002 — Mandanten-/Standort-Scoping-Strategie

Datum: 2026-08-11
Status: entschieden (Subtask 0.2.1.1)
Bezug: Blocker-Frage 1 (Phase 0, beantwortet 2026-08-11, siehe
`docs/open-questions.md`) — Mandantenfähigkeit wird nachgerüstet.
ADR 0001 (Entscheidung 5) hält den vorgefundenen Single-Tenant-Ist-Stand fest;
diese ADR beschreibt, wie er verlassen wird.

## Kontext

aufgabe.md §2 fordert **Mehrmandanten- und Mehrstandortfähigkeit** "von
Anfang an im Datenmodell". `docs/00-recon.md` (Abschnitt 3) hat belegt: das
Schema ist faktisch Single-Tenant. Es existiert bereits eine
Standort-Stammdatentabelle `company_branches` (FK `company_id` →
`company_profiles`, `server/internal/migrate/migrations/026_company_branches.sql`),
die aber von keiner Fach-/Transaktionstabelle referenziert wird —
`company_profiles` ist zudem als Singleton (`id='default'`) angelegt
(`025_company_profile.sql:19-24`).

Bevor die Migrations-Subtasks (0.2.1.2, 0.2.1.3) beginnen, muss geklärt sein:
auf welcher Ebene wird gescoped (Mandant, Standort, oder beides), welche
Tabellen sind betroffen, wie werden bestehende Daten migriert, und wie werden
`users` und `number_sequences` eingebunden.

## Optionen

**Option A — nur `company_id` (Mandantenebene), `company_branches` bleibt rein
informativ.**
Einfachste Umsetzung: eine Spalte, ein Filterkriterium pro Query. Erfüllt aber
nur "Mehrmandantenfähigkeit", nicht die in aufgabe.md §2 gleichrangig
geforderte "Mehrstandortfähigkeit" (z. B. getrennte Sichtbarkeit von Aufträgen
je Niederlassung).

**Option B — zweistufiges Scoping: `company_id` (NOT NULL) + `branch_id`
(NULLABLE) auf allen Fach-/Transaktionstabellen.**
Erfüllt beide Anforderungen aus §2. `branch_id` nullable, weil nicht jeder
Datensatz zwingend einem Standort zugeordnet sein muss (z. B. ein mandantenweit
gültiger Artikelstamm-Eintrag). Höherer Migrations- und Abfrageaufwand
(zwei Filterspalten statt einer), aber additiv und mit Index gut
performant.

**Option C — nur `branch_id`, `company_id` implizit über
`branch_id → company_branches.company_id` abgeleitet (JOIN statt Spalte).**
Spart eine Spalte, erzwingt aber einen JOIN in jeder mandantenweiten Abfrage
(z. B. "alle Aufträge des Mandanten unabhängig vom Standort") und macht
mandantenweite, standortlose Datensätze (siehe Option B) nicht sauber
abbildbar, da jeder Datensatz zwingend einem Standort zugeordnet sein müsste.

## Entscheidung

**Option B.** Begründung:
1. aufgabe.md §2 nennt Mandanten- und Standortfähigkeit gleichrangig
   ("Mehrmandanten-/Mehrstandortfähigkeit: ja") — Option A würde nur die
   Hälfte der Anforderung erfüllen.
2. Ein nullable `branch_id` bildet sowohl standortgebundene als auch
   mandantenweite Datensätze sauber ab, ohne Kompromisse.
3. `company_branches` existiert bereits strukturell passend (FK zu
   `company_profiles`, `is_default`-Flag für den Vorbelegungsfall) — die
   Migration nutzt vorhandene Infrastruktur statt neue zu schaffen.
4. Ein einziger, zweistufiger Umbau jetzt ist günstiger als zwei getrennte
   spätere Migrationen (erst Mandant, später Standort nachrüsten) —
   entspricht aufgabe.md §2 ("nicht nachgerüstet").

### Betroffene Tabellen (Ergänzung `company_id NOT NULL`, `branch_id
NULLABLE`, beide mit Index)

| Tabelle | company_id | branch_id | Begründung |
|---|---|---|---|
| `contacts` | ja | ja | Kunde/Lieferant kann standortspezifisch gepflegt sein |
| `projects` | ja | ja | Projekt gehört zu einer abwickelnden Niederlassung |
| `quotes`, `quote_items`, `quote_imports`, `quote_import_items` | ja | ja (über `quotes`) | Angebot gehört zu Mandant + ausstellendem Standort |
| `sales_orders`, `sales_order_items` | ja | ja (über `sales_orders`) | Analog Angebote |
| `invoices_out`, `invoice_out_items`, `invoice_out_payments` | ja | ja (über `invoices_out`) | GoBD-relevant, muss Standort-Nummernkreis zuordenbar sein |
| `purchase_orders`, `purchase_order_items` | ja | ja (über `purchase_orders`) | Bestellung kann standortspezifisch sein |
| `materials` | ja | nein (mandantenweit) | Artikelstamm i. d. R. mandantenweit gepflegt, nicht je Standort dupliziert |
| `warehouses`, `locations`, `stock_movements`, `batches` | ja | ja | Lager ist typischerweise physisch an einen Standort gebunden |
| `bank_statements`, `journal_entries`, `journal_lines`, `accounts` | ja | nein (mandantenweit) | Buchhaltung i. d. R. auf Mandantenebene konsolidiert (ein Kontenrahmen je Mandant, nicht je Standort) |
| `hr_employees`, `hr_teams`, `hr_leave_requests`, `hr_absences` | ja | ja | Mitarbeiter ist einem Standort zugeordnet |

Nicht gescoped (bewusst global): `tax_codes` (gesetzlich fixe Steuersätze,
mandantenübergreifend gültig), `pdf_templates` (folgt eigenem
`entity`-Schlüssel, kann in einer Folge-ADR pro Mandant erweitert werden,
sobald ein zweiter Mandant real existiert).

### `users`

`users.company_id` (NOT NULL, 1:n — ein User gehört zu genau einem Mandanten).
Kein m:n, da das Produkt für einzelne Metallbaubetriebe ausgelegt ist, nicht
als Multi-Company-SaaS mit geteilten Logins. `users.branch_id` NULLABLE
(ein User ohne `branch_id` sieht alle Standorte seines Mandanten — z. B.
Admin/Buchhaltung; ein User mit gesetztem `branch_id` ist auf diesen Standort
eingeschränkt).

### `number_sequences`

Primärschlüssel wird von `entity` (text) auf `(company_id, entity)`
erweitert. **Bewusst ohne `branch_id`** in dieser ADR: Nummernkreise gelten
zunächst je Mandant, nicht je Standort — vermeidet Überdesign, solange kein
konkreter fachlicher Bedarf für standortspezifische Nummernkreise
dokumentiert ist. Falls das später gebraucht wird, ist es eine additive
Folgemigration (weitere Spalte + zusammengesetzter Schlüssel), keine
Neukonzeption.

### Migrationsreihenfolge (für Subtask 0.2.1.2/0.2.1.3, hier nur grob
skizziert — Detailmigrationen folgen als eigene Subtasks)

1. `company_id` als NULLABLE-Spalte an alle betroffenen Tabellen anfügen.
2. Backfill: alle bestehenden Zeilen erhalten `company_id = 'default'`
   (bestehender Mandant aus `company_profiles`).
3. `branch_id` als NULLABLE-Spalte (FK → `company_branches`) anfügen, kein
   Backfill nötig (bleibt initial `NULL` = "alle Standorte/kein Standort").
4. `number_sequences`: neue Spalte `company_id`, Primärschlüssel auf
   `(company_id, entity)` ändern, Backfill wie oben.
5. `users.company_id`/`users.branch_id` analog.
6. **Erst nachdem** Subtask 0.2.2.1 (Repository-Queries aller betroffenen
   Domänen-Packages) `company_id` beim Anlegen zuverlässig setzt: eigene,
   spätere Migration(en), die `company_id` je Tabelle auf `NOT NULL`
   umstellt (**Expand-Contract-Strategie statt sofortigem NOT NULL** —
   Korrektur gegenüber der ursprünglichen Fassung dieser ADR: ein Testlauf
   von Subtask 0.2.1.2.1 gegen frische DB hat gezeigt, dass ein sofortiges
   `SET NOT NULL` den bestehenden Anwendungscode sofort bricht, da dieser
   `company_id` noch nicht setzt).

Jede dieser Migrationen bekommt einen Down-Pfad (Spalten droppen) und einen
expliziten Hinweis auf das Datenverlustrisiko bei Downgrade, sobald echte
Mehrmandantendaten existieren (aufgabe.md §7.8) — wird in der jeweiligen
Migrations-Subtask (0.2.1.2) umgesetzt, nicht hier.

## Konsequenzen

- Jede Domänen-Query muss künftig nach `company_id` (und ggf. `branch_id`)
  filtern — Subtask 0.2.2.1 (Repository-Queries) und 0.2.2.2 (Middleware:
  Mandanten-Kontext aus Auth-Session) sind direkte Folgearbeiten dieser ADR.
- Erheblicher Umbauaufwand über praktisch alle Domänen-Packages hinweg
  (in mehrere Subtasks zu zerlegen, siehe `docs/backlog.md` 0.2.2).
- Geringer Performance-Overhead durch zusätzliche(n) Filter/Index — bei den
  aktuellen Datenmengen vernachlässigbar.
- Reversibilität: additive Migrationen (neue Spalten) sind technisch mit
  Down-Migration reversibel; sobald ein zweiter Mandant real angelegt wurde,
  ist ein Downgrade mit Datenverlust verbunden (Mandantentrennung geht
  verloren) — muss in jeder betroffenen Migration explizit vermerkt werden.
- `pdf_templates` und weitere hier nicht gescopte Tabellen bleiben bewusst
  offen für eine spätere, separate ADR, falls sich der Bedarf zeigt.
