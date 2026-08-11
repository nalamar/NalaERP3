# State

> Bei Sessionstart zuerst diese Datei und `docs/backlog.md` lesen, danach
> ausschließlich danach richten — nicht nach Chat-Erinnerung.

## Aktueller Pfad

Subtask 0.2.1.2.2 abgeschlossen (Mandanten-/Standort-Scoping für `projects`,
diesmal von Anfang an mit Expand-Contract, kein Nachbessern nötig). Nächste
Subtask: **0.2.1.2.3 — Mandanten-/Standort-Scoping für Angebote/Aufträge/
Rechnungen** (`quotes`+items+imports+import_items, `sales_orders`+items,
`invoices_out`+items+payments, `purchase_orders`+items — größte der 6
Micro-Subtasks, ggf. selbst weiter zu zerlegen).

## Letzte Änderungen (diese Session)

- `docs/00-recon.md`, `docs/01-gap-analysis.md`, `docs/adr/0001-baseline.md`
  erstellt (Phase 0, reine Recherche, kein Produktivcode geändert).
- Vier Blocker-Fragen gestellt und beantwortet, siehe `docs/open-questions.md`.
- `docs/backlog.md` mit Epic 0 (Qualität/Compliance, detailliert bis Subtask-Ebene
  für Feature 0.1) sowie Epics A–I (grob, Task-Ebene aus Gap-Analyse) angelegt.
- Subtask 0.1.1.1: `server/internal/accounting/journal_test.go` neu angelegt
  (3 Tests für die Kernregel "Soll = Haben", fehlender Kontocode, leere
  Buchungszeilen). Folgt der im Repo etablierten Konvention für Services ohne
  DB-Mock (`NewJournalService(nil)`, analog `purchasing`/`materials`
  `service_test.go`) — nur der reine, DB-unabhängige Validierungspfad wird
  unit-getestet.
- Subtask 0.1.1.2: `server/internal/accounting/ar_test.go` neu angelegt.
  Validierungstests für `createTx` (fehlender Kontakt, keine Positionen) durch
  direkten Aufruf der unexportierten Methode mit `tx=nil` (Package-interne
  Tests, gleicher Trick wie bei `journal.go` — Validierung liegt vor jedem
  DB-Zugriff). Zusätzlich reine Funktionstests für `taxRate`, `taxAccountFor`,
  `calcTotals` sowie ein Test, der die zentrale GoBD-Regel prüft: der von
  `buildJournal` erzeugte Journaleintrag muss Soll=Haben ergeben (dieselbe
  Invariante wie in `journal.go`).
- Subtask 0.1.1.3: `server/internal/accounting/payments_test.go` neu angelegt.
  Validierungstest für `Amount <= 0` (einzige vor jedem DB-Zugriff geprüfte
  Regel in `apply()`) sowie reine Funktionstests für `paymentJournal`
  (Bankkonto-Auswahl nach Zahlungsmethode, Soll=Haben-Bilanz).
- Subtask 0.1.1.4 (letzte des Task 0.1.1): `server/internal/accounting/bank_test.go`
  neu angelegt. `bank.go` hat praktisch keine DB-lose Validierung (Ingest/Match
  greifen sofort auf Postgres zu) — getestet wurde der einzige sicher
  DB-lose Pfad: der Nicht-Treffer-Zweig von `findInvoiceIDInReference`
  (leere/nicht passende Referenz). Task 0.1.1 damit abgeschlossen: 14 neue
  Testfunktionen über 4 Dateien im accounting-Paket (0 → 14; Korrektur einer
  vorherigen ungenauen Zählung "12" in dieser Datei — nachgezählt per
  `grep -c "^func Test"`).
- Subtask 0.1.2.1: `server/internal/sales/service_test.go` neu angelegt (erste
  Testdatei des Pakets). `CreateFromQuote()` selbst beginnt sofort eine echte
  Transaktion (nicht DB-los testbar), aber `quoteHasOpenApprovalRework()` —
  die zentrale Sperre "Angebote mit offener Freigabe-Nacharbeit dürfen nicht
  in einen Auftrag überführt werden" — nutzt bewusst eine schmale
  `QueryRow`-only-Schnittstelle (`quoteApprovalReworkQuerier`) statt `pgx.Tx`
  und ließ sich daher mit einem einfachen Fake statt echter DB testen (3 Tests:
  true/false/Fehler-Propagation). Zusätzlich `taxRate()` als reine Funktion
  getestet. Die übrigen DB-abhängigen Guards von `CreateFromQuote` (Status
  muss "accepted" sein, Angebot nicht bereits verlinkt, keine leeren
  Positionen) sind bereits über `server/internal/http/quotes_integration_test.go`
  (convert-to-sales-order-Endpunkt) integrationsgetestet — kein neuer
  Backlog-Fund nötig, anders als bei payments.go/bank.go.
- Subtask 0.1.2.2: `service_test.go` erweitert. `UpdateStatus()` validiert den
  Zielstatus (`isStatus`) vor jedem DB-Zugriff — Negativfall direkt über den
  echten Einstiegspunkt testbar. `validateStatusTransition` (die eigentliche
  Zustandsmaschine: open/released/invoiced/completed/canceled) ist vollständig
  rein und wurde mit 16 tabellengetriebenen Fällen erschöpfend abgedeckt
  (alle erlaubten Übergänge + alle Verbotsfälle je Ausgangsstatus + die
  spezielle Terminal-Status-Fehlermeldung). Auffällig, aber nicht als Bug
  gewertet: eine bereits fakturierte Order (`invoiced`) kann laut Code-Logik
  NICHT mehr storniert werden (`invoiced -> canceled` ist in keinem Zweig
  erlaubt) — konsistent mit dem noch ausstehenden GoBD-Storno-Konzept
  (Backlog 0.3), kein neuer Fund nötig.
- Subtask 0.1.2.3 (letzte des Task 0.1.2): `service_test.go` erweitert.
  `ConvertToInvoice()` prüft `arSvc == nil` vor jedem DB-Zugriff (Negativfall
  direkt testbar). `selectInvoiceQuantities` (Teilfakturierungs-Logik) ist
  vollständig rein und wurde mit 8 Tests erschöpfend abgedeckt (Default ohne
  Auswahl, leere/unbekannte Item-ID, Menge ≤ 0, bereits vollständig
  fakturiert, Menge über Restmenge, gültige Teilauswahl). Per Grep vorab
  bestätigt: keiner dieser Fälle war zuvor irgendwo im Repo getestet (weder
  Unit noch Integration) — Task 0.1.2 schließt diese Lücke vollständig, kein
  separater Backlog-Eintrag nötig. sales-Paket damit bei 25 Tests (0 → 25).
- Subtask 0.1.3.1: No-Op-Block in `server/internal/hr/service.go:98-100`
  entfernt (`if e.Active == false { e.Active = false }` — wirkungsloser
  Code, entfernt statt spekulativ "repariert"; keine Verhaltensänderung).
  Dabei aufgefallen: unklar, ob `Employee.Active` beim Anlegen standardmäßig
  `true` sein sollte — als offene Fachfrage dokumentiert
  (`docs/open-questions.md`), nicht selbst entschieden.
- Subtask 0.1.3.2: `server/internal/hr/service_test.go` neu angelegt (erste
  Testdatei des Pakets), 5 Tests: `Create()`-Namensvalidierung (3 Fälle) sowie
  zwei No-Op-Pfade von `Update()` (leerer Patch; Patch, der ausschließlich aus
  unbekannten Keys besteht). Letzteres deckt einen echten Fund auf: unbekannte
  Patch-Keys werden still ignoriert statt einen Fehler zu liefern (z. B. bei
  Tippfehlern) — als Backlog 0.10 dokumentiert.
- Subtask 0.1.3.3 (letzte des Task 0.1.3 UND von Feature 0.1): `service_test.go`
  erweitert um `LeaveService`-Tests. `Create()` validiert `employee_id` und
  `start/end` vor jedem DB-Zugriff (4 Negativfälle). Zusätzlich zwei echte
  Funde: (1) die Tage-Berechnung prüft nicht, ob `EndDate` nach `StartDate`
  liegt, und kann bei vertauschten Daten einen Wert ≤ 0 ungeprüft in die DB
  schreiben — durch reine Zeitarithmetik belegt (Backlog 0.11); (2) es gibt
  gar keine Überschneidungsprüfung für Urlaubsanträge desselben Mitarbeiters
  (bereits aus der Recon bekannt, jetzt bestätigt) — da das eine fehlende
  Regel statt eines vorhandenen, testbaren Verhaltens ist, wurde sie nicht
  implementiert (Scope-Creep), nur dokumentiert (Backlog 0.12). **Feature 0.1
  insgesamt (`go test -v` PASS-Zeilen inkl. Subtests, Stand jetzt):
  accounting 14, sales 33, hr 12 = 59 grüne Testfälle über drei zuvor
  ungetestete Pakete. Sieben Bugs/Lücken dokumentiert statt stillschweigend
  übersprungen (Backlog 0.6–0.12).**
- Subtask 0.2.1.1: `docs/adr/0002-mandanten-standort-scoping.md` verfasst
  (reine Doku, kein Code). Entscheidung: zweistufiges Scoping —
  `company_id` (NOT NULL) auf allen Kern-Fach-/Transaktionstabellen +
  `branch_id` (NULLABLE, FK → `company_branches`) auf standortgebundenen
  Tabellen (nicht bei `materials`, Buchhaltungstabellen, `tax_codes` —
  bewusst mandantenweit statt standortweit). `users` erhält `company_id`
  (1:n) + optionales `branch_id`. `number_sequences`-PK wird auf
  `(company_id, entity)` erweitert, bewusst OHNE `branch_id` (kein
  dokumentierter Bedarf für standortspezifische Nummernkreise — vermeidet
  Überdesign). Migrationsreihenfolge grob skizziert (Details folgen in
  0.2.1.2/0.2.1.3 als eigene Migrationsdateien mit Down-Pfad).
- Subtask 0.2.1.2 als zu groß erkannt (6 Domänen-Gruppen über >8 Tabellen,
  §6.3) und in 6 Micro-Subtasks zerlegt (`docs/backlog.md`), nur die erste
  umgesetzt: 0.2.1.2.1 — `server/internal/migrate/migrations/054_contacts_company_branch_scope.sql`
  neu (additiv: `company_id` NOT NULL + Backfill `'default'`, `branch_id`
  NULLABLE, Indizes, DOWN-SQL als Kommentar + Datenverlustrisiko-Hinweis).
  **Verifikationslücke ehrlich benannt**: Migration konnte NICHT gegen echtes
  Postgres ausgeführt werden — `docker compose -f docker-compose.test.yml up`
  schlägt fehl, da der Docker-Desktop-Daemon in dieser Umgebung nicht läuft
  (`open //./pipe/dockerDesktopLinuxEngine: The system cannot find the file
  specified`, per `docker info` bestätigt). Die SQL folgt exakt dem bereits
  68-fach bewährten Idiom dieses Repos (ADD COLUMN nullable → Backfill → SET
  NOT NULL, identisch zu `041_quote_revisions.sql`), ist aber NICHT
  laufzeitverifiziert. Vor dem nächsten Deploy/CI-Lauf muss ein Mensch oder
  CI mit funktionierendem Docker `NALA_INTEGRATION=1 go test
  ./internal/http` laufen lassen, um die Migration real zu bestätigen. Dabei
  zusätzlich festgestellt: der Migrationsrunner selbst unterstützt gar keine
  Down-Migrationen (kein Versions-Tracking, kein Rollback-Tooling) —
  dokumentiert als Backlog 0.13.
- **Nachtrag (nach Nutzer-Antwort "Docker Desktop starten lassen")**: Docker
  Desktop lief anschließend, `docker compose -f docker-compose.test.yml up -d
  --wait` erfolgreich, Container gesund. Beim Ausführen von
  `NALA_INTEGRATION=1 go test ./internal/http -run TestContact` gegen die
  frische DB bricht `migrate.Run` aber bereits bei der **vorbestehenden**
  Migration `038_sales_order_partial_invoicing.sql` ab (`invalid reference to
  FROM-clause entry for table "ioi"`, SQLSTATE 42P01) — weit vor meiner neuen
  Migration 054. Ursache: fehlerhaftes `UPDATE...FROM...JOIN...ON`-SQL, das
  den UPDATE-Ziel-Alias `ioi` im `JOIN...ON` referenziert, wo er in Postgres
  nicht sichtbar ist. **Kritischer, unabhängiger Fund**: das bedeutet, jede
  frische Installation dieses Produkts (`docker compose up` auf leerer DB)
  schlägt aktuell fehl — nicht nur meine Verifikation. Als Backlog 0.14
  (kritisch) dokumentiert, NICHT behoben (außerhalb Subtask-Scope). Damit
  bleibt 054 weiterhin unverifiziert — der Docker-Blocker ist gelöst, ein
  neuer, unabhängiger Blocker ist an seine Stelle getreten. Test-Stack
  (`docker-compose.test.yml`) läuft weiter, für den nächsten Verifikationsversuch
  bereit.
- **Nutzer wählte "Sofort als eigene Mini-Subtask fixen"** für Migration 038:
  `WHERE`-Klausel statt `JOIN...ON` für den `ioi`-Bezug (siehe Kommentar in
  der Datei). Mit `docker compose -f docker-compose.test.yml down -v && up`
  (garantiert frische DB) und `NALA_INTEGRATION=1 go test ./internal/http
  -run TestContact` verifiziert: 038 läuft jetzt durch. Die Kette bricht
  danach aber sofort bei der NÄCHSTEN, ebenfalls vorbestehenden, unabhängigen
  Migration ab: `044_quote_imports.sql:4` (`contact_id UUID` referenziert
  `contacts.id text` — Typkonflikt, SQLSTATE 42804). **Dieses Muster wird
  jetzt als eigenes, größeres Problem behandelt statt einzeln
  weiterzuflicken** — siehe Backlog 0.14 (jetzt als Sammel-Item mit
  Unterpunkten geführt) und die Frage an den Nutzer am Ende dieser Antwort,
  wie mit dem gesamten Muster umgegangen werden soll, statt Migrationen
  einzeln reaktiv zu reparieren.
- Zusätzlich beim exakten CI-Formatbefehl (`find . -name '*.go' -not -path
  './vendor/*' | xargs gofmt -l`) entdeckt: 40 vorbestehende, von dieser
  Session nicht verursachte Go-Dateien sind nicht gofmt-konform — der
  CI-Job „Server" würde aktuell unabhängig von dieser Session fehlschlagen.
  Bestätigt: keine meiner eigenen neuen Dateien (accounting/sales/hr-Tests)
  sind betroffen. Dokumentiert als Backlog 0.15, nicht behoben.
- **Nutzerentscheidung "Systematisch alle prüfen und fixen"**: 044 behoben
  (`contact_id` UUID→TEXT). Danach lief die komplette Kette 001–054 bis zum
  Ende durch — dabei aber aufgedeckt, dass `054`s eigenes `SET NOT NULL` auf
  `company_id` die bestehende `POST /api/v1/contacts/`-Route bricht (Test:
  `null value in column "company_id" violates not-null constraint`), da
  `server/internal/contacts` diese Spalte noch nicht befüllt (kommt erst mit
  Subtask 0.2.2.1). Korrektur: `054` auf Expand-Contract umgestellt (Spalte
  bleibt vorerst NULLABLE, `NOT NULL` folgt in einer späteren Migration nach
  0.2.2.1) — ADR 0002 entsprechend aktualisiert. Danach erneut mit frischer
  DB verifiziert: **komplette Migrationskette 001–054 läuft durch, alle
  contacts-scoping-relevanten Tests grün.** Drei weitere, von `company_id`/
  `branch_id` unabhängige Testfehler auf frischer DB entdeckt (Fälligkeitsdatum-
  Roundtrip, Content-Disposition-Header, `quote_items_tax_code_fkey`) — als
  Backlog 0.16–0.18 dokumentiert, bewusst NICHT mehr in dieser bereits stark
  erweiterten Subtask verfolgt (klare Grenze gezogen, um nicht unbegrenzt in
  fresh-DB-Bugfixing abzudriften). Test-Stack läuft weiter für die nächste
  Subtask (0.2.1.2.2).
- Subtask 0.2.1.2.2: `055_projects_company_branch_scope.sql` neu — diesmal
  von Anfang an mit Expand-Contract (kein `SET NOT NULL`), Lektion aus
  0.2.1.2.1 direkt angewendet. Gegen frische DB verifiziert
  (`down -v && up`, `NALA_INTEGRATION=1 go test ./internal/http -run
  TestProject`): keine Migrationsfehler. Zwei Testfehler gefunden, aber
  anhand der Fehlermeldung ("Kontakt mit gleichem Namen und gleicher E-Mail
  bereits vorhanden") als unabhängig von `company_id`/`branch_id`
  identifiziert — Ursache: Testfunktionen teilen sich dieselbe DB ohne
  Datenbereinigung zwischen Läufen, Kollision bei gleichem
  Kontakt-Fixture. Dokumentiert als Backlog 0.19, nicht behoben (gleiche
  bewusste Scope-Grenze wie bei 0.16-0.18).

## Offene Punkte

Siehe `docs/open-questions.md` — Detailfragen zu Mandanten-Scoping-Design
(0.2.1.1) und DATEV-Zielsystem (E.3) sind vorgemerkt, aber keine Blocker für
den aktuellen Arbeitsbeginn.

Bei der Verifikation von 0.1.1.1 (`go test ./...`) wurde ein **vorbestehender,
unabhängiger Testfehler** in `server/internal/purchasing` gefunden (Panic durch
Nil-Pool-Zugriff vor Validierung). Nicht behoben (außerhalb der Subtask, §7.5),
als neue Backlog-Position 0.6 eingetragen. Bei 0.1.1.2 zusätzlich ein
inkonsistentes Fallback-Verhalten bei unbekannten Steuerkennzeichen gefunden
und dokumentiert (Backlog 0.7). Bei 0.1.1.3 zusätzlich festgestellt, dass die
drei zentralen Zahlungs-Guards in `payments.go` (Statusprüfung,
Währungsabgleich, Überzahlungsschutz) weder unit- noch integrationsgetestet
sind — dokumentiert als Backlog 0.8. Bei 0.1.1.4 zusätzlich festgestellt, dass
Bankabgleich/-matching (`bank.go`: Ingest, Match, findInvoiceByAmount) komplett
ungetestet ist — dokumentiert als Backlog 0.9. Bei 0.1.3.2 zusätzlich
festgestellt, dass `EmployeeService.Update()` unbekannte Patch-Keys still
ignoriert statt einen Fehler zu liefern — dokumentiert als Backlog 0.10.

## Korrekturen

*(noch keine — wird gepflegt, sobald der Nutzer eine Anweisung korrigiert.)*

## Verworfen

*(noch keine fehlgeschlagenen Ansätze in dieser Session.)*
