# Backlog — NalaERP3

> Format: `- [ ] <Nummer> Titel — <Status: todo|wip|blocked|done>`. Nummerierung
> ist stabil und wird nie neu vergeben. Max. vier Ebenen (Epic.Feature.Task.Subtask),
> fünfte Ebene nur temporär für Micro-Subtasks. Grundlage: `docs/00-recon.md`,
> `docs/01-gap-analysis.md`, Entscheidungen aus `docs/open-questions.md`
> (Blocker-Fragen Phase 0, beantwortet 2026-08-11).
>
> Epics A–I entsprechen der Domänentabelle aus aufgabe.md §1. Epic 0 ist
> Querschnitt (Qualität/Compliance) und läuft laut Priorisierungsentscheidung
> **vor** dem nächsten größeren Ausbau von A–I. Epics A–I sind aktuell nur bis
> Task-Ebene grob geschnitten (aus der Gap-Analyse abgeleitet); Subtask-Verfeinerung
> erfolgt jeweils unmittelbar vor Bearbeitung.

---

## Epic 0 — Plattform, Qualität & Compliance-Fundament

- [x] 0.1 Test-Absicherung bestehender Finanz-/Personaldomänen — done
  - [x] 0.1.1 accounting-Paket testen (0 Tests, 887 Zeilen, `server/internal/accounting/`) — done
    - [x] 0.1.1.1 Unit-Tests für `journal.go` (Soll/Haben-Buchungslogik) — done
    - [x] 0.1.1.2 Unit-Tests für `ar.go` (Debitorenrechnung aus Quote/SalesOrder, Steuerberechnung) — done
    - [x] 0.1.1.3 Unit-Tests für `payments.go` (Zahlungsverbuchung gegen offene Rechnungen) — done
    - [x] 0.1.1.4 Unit-Tests für `bank.go` (Kontoauszugsimport/-matching) — done
  - [x] 0.1.2 sales-Paket testen (0 Tests, 1032 Zeilen, `server/internal/sales/service.go`) — done
    - [x] 0.1.2.1 Tests für Sales-Order-Erstellung aus Quote — done
    - [x] 0.1.2.2 Tests für Statusübergänge (inkl. Negativfall: unzulässiger Übergang) — done
    - [x] 0.1.2.3 Tests für `ConvertToInvoice` — done
  - [x] 0.1.3 hr-Paket testen + bekannten Bug beheben (0 Tests, `server/internal/hr/service.go`) — done
    - [x] 0.1.3.1 No-Op-Bug beheben (`service.go:98-100`, `if e.Active == false { e.Active = false }`) — done
    - [x] 0.1.3.2 Tests für `EmployeeService` (CRUD inkl. Negativfall ungültiges Patch) — done
    - [x] 0.1.3.3 Tests für `LeaveService` inkl. Negativfall (überlappende Urlaubsanträge — aktuell ungeprüft) — done
- [ ] 0.2 Mandantenfähigkeit nachrüsten (Entscheidung Blocker-Frage 1: Breaking-Change-Migration) — wip
  - [ ] 0.2.1 Datenmodell erweitern — wip
    - [x] 0.2.1.1 ADR: Scoping-Strategie festlegen (company_id vs. branch_id, welche Tabellen betroffen, Migrationsreihenfolge) — done, siehe `docs/adr/0002-mandanten-standort-scoping.md`
    - [ ] 0.2.1.2 Migration: `company_id` (NOT NULL) + `branch_id` (NULLABLE) an den in ADR 0002 gelisteten Kern-Fachtabellen ergänzen — wip, in 6 Micro-Subtasks zerlegt (§6.3: >8 Dateien/Domänen zu groß für eine Subtask)
      - [x] 0.2.1.2.1 CRM: `contacts` (`server/internal/migrate/migrations/054_contacts_company_branch_scope.sql`) — done, gegen frische DB verifiziert (siehe Backlog 0.14 für den Weg dahin)
      - [x] 0.2.1.2.2 Projekte: `projects` (`server/internal/migrate/migrations/055_projects_company_branch_scope.sql`) — done, gegen frische DB verifiziert
      - [ ] 0.2.1.2.3 Angebote/Aufträge/Rechnungen: `quotes`(+items+imports+import_items), `sales_orders`(+items), `invoices_out`(+items+payments), `purchase_orders`(+items) — todo
      - [ ] 0.2.1.2.4 Material/Lager: `materials`, `warehouses`, `locations`, `stock_movements`, `batches` — todo
      - [ ] 0.2.1.2.5 Buchhaltung: `journal_entries`, `accounts` (nur `company_id`, kein `branch_id` laut ADR 0002) — todo
      - [ ] 0.2.1.2.6 HR: `hr_employees`, `hr_teams`, `hr_leave_requests`, `hr_absences` — todo
    - [ ] 0.2.1.3 `users.company_id`/`users.branch_id` ergänzen; `number_sequences`-PK auf `(company_id, entity)` erweitern (laut ADR 0002) — todo
  - [ ] 0.2.2 Anwendungscode auf Mandanten-Scoping umstellen — blocked (wartet auf 0.2.1)
    - [ ] 0.2.2.1 Repository-Queries um Scoping-Filter erweitern (alle Domänen-Packages) — todo
    - [ ] 0.2.2.2 Middleware: Mandanten-Kontext aus Auth-Session ableiten und in Request-Context legen — todo
    - [ ] 0.2.2.3 `number_sequences` pro Mandant statt global (aktuell `entity`-PK ohne Mandantenbezug, `server/internal/migrate/migrations/005_numbering.sql:1-7`) — todo
- [ ] 0.3 GoBD-Fundament (Storno statt Delete, Festschreibung, Änderungsprotokoll) — todo
  - [ ] 0.3.1 Storno-Konzept für `invoices_out`/`journal_entries` (aktuell nur `draft|booked|paid`, kein Storno-Status) — todo
  - [ ] 0.3.2 Festschreibungs-Mechanismus für gebuchte/versendete Belege — todo
  - [ ] 0.3.3 Generisches Änderungsprotokoll über Fachobjekte (aktuell nur domänenspezifisch bei LogiKal-Import und Quote-Preis-/Freigabe-Historie) — todo
- [ ] 0.4 Migrationsverzeichnis bereinigen — todo
  - [ ] 0.4.1 ADR: Umgang mit doppelten Nummernketten in `server/internal/migrate/migrations/` (001/007/013/014 doppelt vorhanden) — todo
  - [ ] 0.4.2 ADR: Umgang mit verwaistem `server/migrations/`-Verzeichnis (9 Dateien, nicht eingebunden) — todo
- [ ] 0.5 Auth-Härtung — todo
  - [ ] 0.5.1 Startup-Guard gegen Default-`JWT_SECRET` in Produktivumgebung — todo
  - [ ] 0.5.2 Rate-Limiting auf `/auth/login` — todo
  - [ ] 0.5.3 `users.manage`-Bypass in `requirePermission()` dokumentieren oder durch dedizierte `admin.superuser`-Permission ersetzen (`server/internal/http/v1.go:3634`) — todo
  - [ ] 0.5.4 User-Management-API (Anlegen/Sperren/Rollenzuweisung) statt Direkt-SQL — todo
- [ ] 0.6 Pre-existing Testfehler beheben: `purchasing.TestCreateRejectsInvalidItem` panict — todo
  - Gefunden bei Verifikation von Subtask 0.1.1.1 (`go test ./...`, unrelated zu Subtask). `Service.Create`
    (`server/internal/purchasing/service.go:86`) ruft `s.pg.Begin(ctx)` auf, **bevor** die Item-Validierung
    ("Ungültige Position") greift. Der Test instanziiert den Service mit `NewService(nil)`
    (`server/internal/purchasing/service_test.go:41-56`) — `s.pg.Begin(ctx)` auf nil `*pgxpool.Pool`
    löst einen `nil pointer dereference`-Panic in `pgxpool.(*Pool).Acquire` aus, statt den erwarteten
    Validierungsfehler zurückzugeben. Der Panic bricht den gesamten Testlauf des Pakets `purchasing` ab.
    Fix: Item-Validierung vor `s.pg.Begin(ctx)` ziehen (analog zu den anderen Validierungen in derselben
    Funktion) — Produktivverhalten ändert sich nicht (im Produktivbetrieb ist `s.pg` nie nil), nur die
    Reihenfolge der Prüfungen.
- [ ] 0.7 Steuerkennzeichen-Validierung in `accounting` gegen Stammdaten absichern — todo
  - Gefunden bei Verifikation von Subtask 0.1.1.2 (`server/internal/accounting/ar.go:374-393`).
    `taxRate()`/`taxAccountFor()` kennen hartcodiert nur `DE19`/`DE7`; jeder andere Wert (Tippfehler,
    zukünftig in `tax_codes` gepflegter Code) wird `taxRate()` zufolge stillschweigend als steuerfrei (0 %)
    behandelt, `taxAccountFor()` fällt gleichzeitig auf das DE19-Steuerkonto `1776` zurück — inkonsistentes
    Fallback-Verhalten ohne Fehler/Warnung. Die bereits vorhandene `tax_codes`-Tabelle
    (`server/internal/migrate/migrations/017_accounting_basics.sql`) wird dabei nicht konsultiert. Risiko:
    fehlerhafte USt-Buchungen bleiben unbemerkt. Nicht behoben (außerhalb Subtask-Scope), Verhalten ist per
    Test dokumentiert (`server/internal/accounting/ar_test.go: TestTaxRateKnownAndUnknownCodes`,
    `TestTaxAccountForKnownAndUnknownCodes`).
- [ ] 0.8 Integrationstests für die zentralen Zahlungs-Guards in `payments.go` ergänzen — todo
  - Gefunden bei Verifikation von Subtask 0.1.1.3. `PaymentService.apply()` (`server/internal/accounting/payments.go:70-86`)
    prüft drei fachlich zentrale Regeln erst NACH `tx.QueryRow` (Statusguard "Rechnung ist nicht gebucht",
    Währungsabgleich "Währung stimmt nicht mit Rechnung überein", Überzahlungsschutz "Zahlung übersteigt
    offenen Betrag") — diese sind ohne echte DB-Transaktion nicht unit-testbar (kein Mock/Testcontainer im
    Repo, siehe ADR 0001 / docs/00-recon.md). Grep über `server/internal/http/accounting_integration_test.go`
    zeigt: keine dieser drei Fehlermeldungen wird dort geprüft — die einzige bestehende Integrationstest
    (`TestInvoiceOutFlowWithPDFAndPayments`) deckt nur den Happy-Path ab. Diese drei Regeln sind damit aktuell
    **komplett ungetestet** (weder Unit noch Integration). Fix: neue Fälle in
    `accounting_integration_test.go` (oder eigene Datei) mit `NALA_INTEGRATION=1`.
- [ ] 0.9 Integrationstests für Bankabgleich/-matching ergänzen (`bank.go`) — todo
  - Gefunden bei Verifikation von Subtask 0.1.1.4. `Ingest()`, `Match()` und `findInvoiceByAmount()`
    (`server/internal/accounting/bank.go:36-76,120-170,172-195`) greifen ohne jede Vorab-Validierung sofort
    auf Postgres zu — anders als bei `journal.go`/`ar.go`/`payments.go` gibt es hier praktisch keine
    DB-lose Validierungslogik. Grep über `accounting_integration_test.go` bestätigt: keine Integrationstests
    für Bankauszugs-Import oder -Matching vorhanden. Damit sind Ingest, Match (inkl. "Statement bereits
    gematcht"-Schutz), die Betrags-Heuristik (`findInvoiceByAmount`, inkl. Mehrdeutigkeits-Fehler bei
    mehreren offenen Posten mit gleichem Betrag) und der DB-Lookup-Zweig von `findInvoiceIDInReference`
    komplett ungetestet. Fachlich relevant, da Bankabgleich Zahlungen automatisch verbucht (GoBD-relevant).
    Subtask 0.1.1.4 deckt nur den DB-losen Nicht-Treffer-Zweig der Referenz-Erkennung ab
    (`server/internal/accounting/bank_test.go`). Fix: Integrationstests mit `NALA_INTEGRATION=1`.
- [ ] 0.14 KRITISCH: gesamte Migrationskette systematisch gegen frische DB verifizieren und reparieren — wip, PRIORISIERT VOR 0.2.1.2.2+
  - Nutzerentscheidung 2026-08-11: systematisch die komplette Kette 001–053(+054) einmal am Stück gegen eine
    frische DB durchlaufen lassen und jeden Fehler beheben, BEVOR mit den weiteren Mandanten-Scoping-Subtasks
    (0.2.1.2.2 ff.) fortgefahren wird. `migrate.Run` (`server/internal/migrate/migrate.go:28-30`) bricht beim
    ERSTEN Fehler ab — da alle Migrationsdateien bei jedem Serverstart erneut ausgeführt werden (kein
    Versions-Tracking, Backlog 0.13), wurde die Kette anscheinend nie vollständig gegen eine wirklich leere DB
    verifiziert. **Tragweite: jede Neuinstallation (`docker compose up` auf leerer DB) schlägt aktuell fehl.**
    - [x] `038_sales_order_partial_invoicing.sql:7-15` — UPDATE-Ziel-Alias `ioi` im `JOIN...ON` referenziert
      (in Postgres dort nicht sichtbar) → `SQLSTATE 42P01`. Behoben: Bedingung in `WHERE` verschoben.
      Verifiziert mit frischer Test-DB.
    - [x] `044_quote_imports.sql:4` — `contact_id UUID` referenzierte `contacts.id text` → Typkonflikt,
      `SQLSTATE 42804`. Behoben: Spaltentyp auf `TEXT` geändert (analog allen anderen `contact_id`-Spalten
      im Schema). Verifiziert mit frischer Test-DB.
    - [x] Migrationen 045–053 sowie die neue 054 laufen jetzt vollständig gegen frische DB durch — **die
      komplette Kette 001–054 ist damit erstmals verifiziert lauffähig**, `migrate.Run` schlägt nicht mehr
      fehl. Zusätzlich musste `054_contacts_company_branch_scope.sql` selbst korrigiert werden: das initial
      gesetzte `company_id SET NOT NULL` brach die bestehende `POST /api/v1/contacts/`-Route, weil
      `server/internal/contacts` `company_id` noch nicht setzt (Subtask 0.2.2.1 kommt erst später) —
      auf Expand-Contract-Strategie umgestellt (Spalte bleibt vorerst nullable, `NOT NULL` folgt erst nach
      0.2.2.1), siehe aktualisierte `docs/adr/0002-mandanten-standort-scoping.md`.
  - **Feststellung**: `docker compose up` auf einer leeren Datenbank funktioniert jetzt wieder (Kern-Ziel
    dieser Backlog-Position erreicht). Beim Testlauf gegen frische DB zusätzlich drei **weitere,
    unabhängige** vorbestehende Test-/Anwendungsfehler entdeckt (nicht migrationsbezogen, nicht durch
    `company_id`/`branch_id` verursacht — geprüft anhand der Fehlermeldungen). Bewusst NICHT mehr in dieser
    bereits stark erweiterten Subtask verfolgt, sondern separat dokumentiert:
    - [ ] 0.16 `TestContactTasksCreateListUpdateAndDeleteFlow` schlägt auf frischer DB fehl: "expected due
      date to roundtrip" (`server/internal/http/contacts_integration_test.go:403`) — todo, Ursache noch
      nicht analysiert.
    - [ ] 0.17 `TestContactDocumentsUploadListAndDownloadFlow` schlägt auf frischer DB fehl: "expected
      content disposition header" (`server/internal/http/contacts_integration_test.go:613`) — todo, Ursache
      noch nicht analysiert.
    - [ ] 0.18 `TestContactCommercialContextAggregatesQuotesSalesOrdersAndInvoices` schlägt auf frischer DB
      fehl: `quote_items_tax_code_fkey`-Verletzung beim Anlegen eines Angebots
      (`server/internal/http/contacts_integration_test.go:843`, SQLSTATE 23503) — vermutlich verwendet der
      Testaufbau/die Seed-Logik einen `tax_code`, der in `tax_codes` auf einer wirklich frischen DB fehlt —
      todo, Ursache noch nicht analysiert.
    - [ ] 0.19 `TestProjectQuotePDFFlow`/`TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
      kollidieren bei gemeinsamem Testlauf: beide legen offenbar einen Kontakt mit identischem
      Name+E-Mail-Fixture an, der zweite Aufruf schlägt mit "Kontakt mit gleichem Namen und gleicher E-Mail
      bereits vorhanden" fehl (`server/internal/http/projects_integration_test.go:20,133`). Ursache: Tests
      teilen sich dieselbe Postgres-Instanz ohne Datenbereinigung zwischen Testfunktionen
      (`testutil.SetupIntegrationEnv` setzt nur das Schema neu auf, nicht die Daten) — bei gefiltertem
      `-run`-Lauf reproduzierbar, betrifft potenziell auch volle Testläufe. Nicht analysiert, ob dies auch
      bei `go test ./internal/http` ohne Filter auftritt. Gefunden bei Verifikation von Subtask 0.2.1.2.2,
      nicht behoben (unabhängig von `company_id`/`branch_id`, per Fehlermeldung bestätigt).
- [ ] 0.15 CI-Format-Check würde aktuell fehlschlagen: 40 Go-Dateien nicht gofmt-konform — todo
  - Gefunden beim Ausführen des exakten CI-Befehls (`.github/workflows/ci.yml:75-79`:
    `find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l`) während der Verifikation von
    Subtask 0.2.1.2.1. 40 Dateien quer durchs gesamte `server/`-Modul (u. a. `internal/migrate/migrate.go`,
    `internal/auth/*.go`, `internal/app/server.go`, `internal/db/*.go`, mehrere `*_test.go`) sind NICHT
    gofmt-formatiert (u. a. 4-Leerzeichen-Einrückung statt Tabs, Einzeiler-`if`-Blöcke ohne Zeilenumbruch) —
    von mir in dieser Session **nicht verursacht** (keine dieser Dateien wurde angefasst). Das bedeutet, der
    CI-Job „Server" (`Verify formatting`) würde beim nächsten Push fehlschlagen, unabhängig von dieser
    Session. Nicht behoben (weit außerhalb jeder aktuellen Subtask, 40 Dateien = klarer Fall für eine eigene,
    dedizierte Subtask). Fix: `gofmt -w` über die betroffenen Dateien, als eigene Subtask mit Review (gofmt
    kann bei Einzeiler-`if`-Umformatierung Verhalten unverändert lassen, aber Diff-Review ist trotzdem
    angebracht, um versehentliche semantische Änderungen auszuschließen — gofmt selbst ändert nie Semantik,
    rein zur Sorgfalt).
- [ ] 0.13 Migrationsrunner unterstützt keine Down-Migrationen — todo
  - Gefunden bei Verifikation von Subtask 0.2.1.2.1 (`server/internal/migrate/migrate.go:17-33`). `migrate.Run`
    führt jede `.sql`-Datei im aktiven Verzeichnis alphabetisch sortiert vorwärts aus — es gibt kein
    Versions-Tracking (keine `schema_migrations`-Tabelle), keine Down-Skripte, keinen Rollback-Mechanismus.
    Damit ist aufgabe.md §7.8 ("Keine Migration ohne Down-Pfad") aktuell nur per Kommentar im Migrationsfile
    erfüllbar (manuelles Rollback-SQL als Dokumentation, siehe `054_contacts_company_branch_scope.sql`),
    nicht durch automatisiertes Tooling. Betrifft potenziell alle 68 bisherigen Migrationen. Fix: entweder
    ein Down-Migrations-Konzept einführen (z. B. gepaarte `NNN_up.sql`/`NNN_down.sql`-Dateien +
    Versions-Tabelle) oder bewusst als Produktentscheidung dokumentieren, dass Rollbacks manuell erfolgen.
- [ ] 0.10 `EmployeeService.Update()` soll unbekannte Patch-Keys ablehnen statt still zu ignorieren — todo
  - Gefunden bei Verifikation von Subtask 0.1.3.2 (`server/internal/hr/service.go:121-150`). Der `switch`
    über die Patch-Keys übernimmt nur eine feste Whitelist (`first_name`, `last_name`, `email`, `phone`,
    `role`, `location`, `cost_center`, `active`, `team_id`) in die SQL-`SET`-Klausel; jeder andere Key
    (z. B. Tippfehler wie `activ` statt `active`) fällt durch den `switch` und wird stillschweigend
    verworfen — der Aufruf kehrt ohne Fehler zurück, obwohl nichts geändert wurde. Verhalten per Test
    dokumentiert (`server/internal/hr/service_test.go: TestUpdateWithOnlyUnknownKeysIsSilentNoOp`). Fix:
    unbekannte Keys mit Fehler ablehnen statt zu ignorieren (analog zu anderen `Update`-Handlern im Repo
    prüfen, ob dasselbe Muster auch dort vorkommt).
- [ ] 0.11 `LeaveService.Create()`: Tage-Berechnung kann bei vertauschten Daten ≤ 0 ergeben — todo
  - Gefunden bei Verifikation von Subtask 0.1.3.3 (`server/internal/hr/service.go:177-182`). Wenn `Days`
    nicht explizit gesetzt ist (`<= 0`), wird es aus `EndDate.Sub(StartDate).Hours()/24 + 1` berechnet —
    ohne vorherige Prüfung, dass `EndDate` nach `StartDate` liegt. Bei vertauschten Daten (z. B. `StartDate`
    = 2026-08-20, `EndDate` = 2026-08-18) ergibt die Formel `-1` und wird ungeprüft in die DB geschrieben
    (`hr_leave_requests.days`). Durch reine Zeitarithmetik belegt (nicht über `Create()` selbst, das ohne
    echten DB-Zugriff bei diesem Pfad bereits vor der Formel `s.pg.Exec` erreichen würde), siehe
    `server/internal/hr/service_test.go: TestLeaveCreateDaysFormulaCanProduceNonPositiveDaysForInvertedDateRange`.
    Fix: `EndDate < StartDate` vor der Berechnung explizit ablehnen.
- [ ] 0.12 `LeaveService.Create()`: keine Überschneidungsprüfung für Urlaubsanträge — todo
  - Bereits in `docs/00-recon.md`/`docs/01-gap-analysis.md` (Domäne F) als Lücke vermerkt, bei Subtask
    0.1.3.3 bestätigt: `Create()` fragt vor dem Insert keine bereits bestehenden `hr_leave_requests` desselben
    Mitarbeiters ab — zwei sich überschneidende Anträge (auch mehrfach genehmigte) sind aktuell möglich.
    Da die Prüfung eine Datenbankabfrage erfordert, ist sie ohne DB-Mock nicht unit-, sondern nur
    integrationstestbar — in Subtask 0.1.3.3 bewusst nicht nachgebaut (wäre Scope-Creep: Implementierung
    einer fehlenden fachlichen Regel statt nur Tests für vorhandene Regeln). Fix: Überschneidungsprüfung
    implementieren + Integrationstest.

## Epic A — Stammdaten

- [ ] A.1 Metallbau-spezifisches Artikel-/Profilattributschema (Profilserie, RC-Klasse, U-Wert, Brandschutzklasse) — todo
- [ ] A.2 Preisliste als eigene Entität (Gültigkeitszeiträume, Staffelpreise) — todo
- [ ] A.3 Systemlieferanten-Konzept (Bindung Lieferant↔Profilserie) — todo

## Epic B — Angebots- & Auftragswesen

- [ ] B.1 LV-Hierarchie (Los/Titel/Untertitel) im Positionsmodell nachrüsten — todo
- [ ] B.2 Nachtragsmanagement für bestehende Aufträge — todo
- [ ] B.3 Vollständiges Kalkulationsschema (Material/Lohn/Fremdleistung/Zuschläge getrennt) — todo
- [ ] B.4 Abschlags-/Schlussrechnung nach VOB/B §16 — todo

## Epic C — Waren- & Lagerwirtschaft

- [ ] C.1 Reservierungslogik (projektbezogen) — todo
- [ ] C.2 Inventurprozess — todo
- [ ] C.3 Verschnitt-/Reststückverwaltung für Profile — todo

## Epic D — Bestellwesen

- [ ] D.1 Bedarfsermittlung aus Angebot/Mindestbestand — todo
- [ ] D.2 Anfrageprozess (RFQ) vor Bestellung — todo
- [ ] D.3 Eingangsrechnungsprüfung (3-Way-Match PO↔Wareneingang↔Rechnung), `/invoices-in`-Domäne — todo

## Epic E — Finanzwesen

- [ ] E.1 Kostenstellenmodell — todo
- [ ] E.2 Projektcontrolling (Soll-Ist im Server, nicht nur Client-Aggregation) — todo
- [ ] E.3 DATEV-Export (EXTF, Format-Version 13) — todo
- [ ] E.4 E-Rechnung Ausgang (XRechnung, ZUGFeRD 2.x) — todo
- [ ] E.5 E-Rechnung Eingang (mind. Parsen) — todo

## Epic F — Personal & HR

- [ ] F.1 Zeiterfassung (revisionssicher, ArbZG/BAG-konform) — todo
- [ ] F.2 Weiterbildung/Qualifikationen/Unterweisungen — todo
- [ ] F.3 Asset-Zuordnung (Werkzeuge/IT/PSA) — todo

## Epic G — Fuhrpark

- [ ] G.1 Fahrzeugstamm — todo
- [ ] G.2 Termine (HU/AU, UVV, Wartung) — todo
- [ ] G.3 Fahrtenbuch, Kosten, Führerscheinkontrolle — todo

## Epic H — Produktionssteuerung

- [ ] H.1 Fertigungsaufträge aus Projektpositionen — todo
- [ ] H.2 Stücklisten/Arbeitsgänge/Kapazitäten — todo
- [ ] H.3 BDE, Kommissionierung, Montageplanung — todo

## Epic I — KI-gestützte Angebotserzeugung aus GAEB (Zielwert des Systems)

- [ ] I.1 Evaluationsset zuerst (Testkorpus + Kennzahlen Trefferquote/Fehlzuordnungsrate) — todo, **muss vor I.3 stehen** (aufgabe.md §4)
- [ ] I.2 Echter GAEB-DA-XML-3.x-Parser (Entscheidung Blocker-Frage 3: ersetzt `gaeb_xml_subset_parser.go`) — todo
  - [ ] I.2.1 D81/D83/D86-Einlesen mit voller Hierarchie (Los/Titel/Untertitel), Positionsart, Vorbemerkungen — todo
  - [ ] I.2.2 Migration bestehender GAEB-Import-Tests/-Fixtures auf reales Format — todo
- [ ] I.3 Provider-Interface für austauschbares Sprachmodell — todo, blocked bis I.1
- [ ] I.4 LLM-gestütztes Matching (LV-Position → interne Leistung/Stückliste) — todo, blocked bis I.1, I.3
- [ ] I.5 Confidence-Score + Prüfliste für automatische Zuordnungen — todo, blocked bis I.4
- [ ] I.6 GAEB-D84-Rückschreibung (Angebotsabgabe) — todo, blocked bis I.2
