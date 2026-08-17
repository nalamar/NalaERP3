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
- [x] 0.2 (0.2.1 UND 0.2.2 abgeschlossen) Mandantenfähigkeit nachrüsten (Entscheidung Blocker-Frage 1: Breaking-Change-Migration) — done. `company_id`/`branch_id` an allen in ADR 0002 gelisteten Tabellen ergänzt UND der gesamte Anwendungscode (alle 6 Domänen-Cluster + `number_sequences`) auf `company_id`-Scoping umgestellt, gegen frische DB verifiziert. Bewusst NICHT Teil dieses Tasks (siehe jeweilige Backlog-Einträge für Details, kein neuer Scope): `branch_id` wird zwar überall persistiert, aber NICHT aktiv als Filter durchgesetzt (nur `company_id`) — Standort-Scoping innerhalb eines Mandanten ist ein separates, noch nicht angelegtes künftiges Backlog-Item. Mehrere echte, HTTP-erreichbare Cross-Tenant-Lücken bei der Umsetzung gefunden UND behoben (nicht nur mechanisches Duchreichen): `ListApprovalReworkQueue`/`ListApprovalRequestQueue` (quotes), `GET/POST /invoices-out/{id}/payments`, `GET/PUT /settings/numbering/{entity}` — jeweils dokumentiert in den zugehörigen Subtask-Einträgen.
  - [x] 0.2.1 Datenmodell erweitern — done (Migrationen 054-060, alle gegen frische DB verifiziert; NOT-NULL-Verschärfung und number_sequences-PK-Umstellung bewusst auf nach Task 0.2.2 verschoben)
    - [x] 0.2.1.1 ADR: Scoping-Strategie festlegen (company_id vs. branch_id, welche Tabellen betroffen, Migrationsreihenfolge) — done, siehe `docs/adr/0002-mandanten-standort-scoping.md`
    - [x] 0.2.1.2 Migration: `company_id` (NULLABLE, Expand-Contract) + `branch_id` (NULLABLE) an den in ADR 0002 gelisteten Kern-Fachtabellen ergänzt — done, alle 6 Micro-Subtasks abgeschlossen und gegen frische DB verifiziert (§6.3-Zerlegung)
      - [x] 0.2.1.2.1 CRM: `contacts` (`server/internal/migrate/migrations/054_contacts_company_branch_scope.sql`) — done, gegen frische DB verifiziert (siehe Backlog 0.14 für den Weg dahin)
      - [x] 0.2.1.2.2 Projekte: `projects` (`server/internal/migrate/migrations/055_projects_company_branch_scope.sql`) — done, gegen frische DB verifiziert
      - [x] 0.2.1.2.3 Angebote/Aufträge/Rechnungen: `quotes`, `sales_orders`, `invoices_out`, `purchase_orders` (`server/internal/migrate/migrations/056_sales_scope.sql`) — done, gegen frische DB verifiziert. Item-/Kind-Tabellen bewusst NICHT direkt gescoped (Präzisierung in ADR 0002 — erben Scope über FK zur Kopf-Tabelle)
      - [x] 0.2.1.2.4 Material/Lager: `materials`, `warehouses` (`server/internal/migrate/migrations/057_materials_warehouses_scope.sql`) — done, gegen frische DB verifiziert. `locations`/`batches`/`stock_movements` bewusst NICHT direkt gescoped (erben Scope über `warehouse_id`/`material_id`, siehe ADR 0002)
      - [x] 0.2.1.2.5 Buchhaltung: `accounts`, `journal_entries`, `bank_statements` (nur `company_id`, kein `branch_id` laut ADR 0002; `journal_lines` erbt über `entry_id`) (`server/internal/migrate/migrations/058_accounting_scope.sql`) — done, gegen frische DB verifiziert
      - [x] 0.2.1.2.6 HR: `hr_employees`, `hr_teams` (`server/internal/migrate/migrations/059_hr_scope.sql`) — done, gegen frische DB verifiziert. `hr_leave_requests`/`hr_absences` bewusst NICHT direkt gescoped (erben über `employee_id`); `hr_holidays` bewusst ausgenommen (kein Mandanten-Scoping-Bedarf, siehe ADR 0002)
    - [x] 0.2.1.3 `users.company_id`/`users.branch_id` ergänzen; `number_sequences.company_id` ergänzen (PK-Umstellung bewusst auf nach Task 0.2.2 verschoben, siehe ADR-0002-Präzisierung) (`server/internal/migrate/migrations/060_users_numbering_scope.sql`) — done, gegen frische DB verifiziert
  - [x] 0.2.2 (alle 3 Subtasks abgeschlossen) Anwendungscode auf Mandanten-Scoping umstellen — done
    - [x] 0.2.2.1 (alle 6 Domänen-Cluster abgeschlossen) Repository-Queries um Scoping-Filter erweitern (alle Domänen-Packages) — done, in Micro-Subtasks zerlegt (§6.3: >8 Dateien/Domänen zu groß für eine Subtask), alle 6 gegen frische DB verifiziert: 0.2.2.1.1 Auth-Kern, 0.2.2.1.2 Kontakte/Projekte, 0.2.2.1.3 Angebote/Aufträge/Rechnungen/Bestellungen, 0.2.2.1.4 Material/Lager, 0.2.2.1.5 Buchhaltung, 0.2.2.1.6 HR. **Reihenfolge-Korrektur bei Umsetzung**: 0.2.2.1.1 (Auth-Kern) MUSS vor allen weiteren laufen, da die Domänen-Filter erst funktionieren, wenn `company_id`/`branch_id` aus dem Request-Context verfügbar sind — 0.2.2.2 (Middleware) war ursprünglich als separater, späterer Schritt geplant, ist aber inhaltlich Voraussetzung für 0.2.2.1 und wurde deshalb als 0.2.2.1.1 vorgezogen.
      - [x] 0.2.2.1.1 Auth-Kern: `company_id`/`branch_id` von der DB über `auth.User` → Request-Context verfügbar machen (Voraussetzung für alle weiteren Micro-Subtasks) — done, gegen frische DB verifiziert
      - [x] 0.2.2.1.2 Kontakte/Projekte: `contacts`, `projects` — done, beide Domänen vollständig gescoped (contacts: Kern-CRUD + 5 Unterressourcen; projects: Kern-CRUD + 6 Unterressourcen-Ebenen)
        - [x] 0.2.2.1.2.1 `contacts`-Kern-CRUD: `List`/`Create`/`Get`/`Update`/`DeleteSoft`/`ListActivity` um `company_id`-Filter erweitert, `Create` validiert `companyID` erforderlich, HTTP-Handler (`v1.go`) reichen `companyIDFromContext` durch — done, gegen frische DB verifiziert (nur die bereits bekannten, unabhängigen Vorbefunde 0.16-0.18 weiterhin offen, keine neuen Regressionen)
        - [x] 0.2.2.1.2.2 `contacts`-Unterressourcen — done, alle 5 Ressourcentypen umgesetzt und gegen frische DB verifiziert
          - [x] 0.2.2.1.2.2.1 Adressen (`CreateAddress`/`ListAddresses`/`UpdateAddress`/`DeleteAddress`) — done, gegen frische DB verifiziert. Behebt dabei den in Subtask 0.2.2.1.2.1 gefundenen Autorisierungs-Fund: `CreateAddress` prüfte die Kontakt-Zugehörigkeit vorher gar nicht wirklich (`// we rely on FK for existence`, Ergebnis verworfen) — jetzt echte Prüfung.
          - [x] 0.2.2.1.2.2.2 Ansprechpartner (`CreatePerson`/`ListPersons`/`UpdatePerson`/`DeletePerson`) — done, gegen frische DB verifiziert. Bestätigt: `CreatePerson` hatte GAR KEINEN Ownership-Check (nicht mal einen verworfenen wie `CreateAddress`) — jetzt behoben.
          - [x] 0.2.2.1.2.2.3 Notizen (`CreateNote`/`ListNotes`/`UpdateNote`/`DeleteNote`) — done, gegen frische DB verifiziert. `ListActivity` (`activity.go`) reicht `companyID` jetzt auch an `ListNotes` durch.
          - [x] 0.2.2.1.2.2.4 Aufgaben (`CreateTask`/`ListTasks`/`UpdateTask`/`DeleteTask`) — done, gegen frische DB verifiziert. `ListActivity` reicht `companyID` jetzt auch an `ListTasks` durch.
          - [x] 0.2.2.1.2.2.5 Dokumente (`UploadContactDocument`/`ListContactDocuments`) — done, gegen frische DB verifiziert. Ownership-Check vor dem GridFS-Upload platziert (nicht danach), um verwaiste GridFS-Dateien bei abgelehntem Zugriff zu vermeiden.
        - [x] 0.2.2.1.2.3 `projects` (inkl. Phasen/Elevationen/LogiKal-Import) — done, alle 7 Micro-Subtasks abgeschlossen und gegen frische DB verifiziert (§6.3-Zerlegung, analog zu `contacts`)
          - [x] 0.2.2.1.2.3.1 Kern-CRUD: `List`/`Create`/`Get`/`UpdateStatus`/`BuildQuoteSnapshot` um `company_id`-Filter erweitert (inkl. Cross-Package-Aufruf `quotes.Service.Accept` → `projectSvc.UpdateStatus` und `ImportLogikal`s Re-Import-Lookup) — done, gegen frische DB verifiziert
          - [x] 0.2.2.1.2.3.2 Phasen (`CreatePhase`/`ListPhases`/`GetPhase`/`UpdatePhase`/`DeletePhase`) — Ownership-Check über Projekt (`project_phases` hat kein eigenes `company_id`, erbt über `project_id`, siehe ADR 0002) — done, gegen frische DB verifiziert
          - [x] 0.2.2.1.2.3.3 Elevationen (`CreateElevation`/`ListElevations`/`GetElevation`/`UpdateElevation`/`DeleteElevation`) — Ownership-Check über Phase→Projekt — done, gegen frische DB verifiziert
          - [x] 0.2.2.1.2.3.4 Ausführungsvarianten (`CreateSingleElevation`/`ListSingleElevations`/`GetSingleElevation`/`UpdateSingleElevation`/`DeleteSingleElevation`) — Ownership-Check über Elevation→Phase→Projekt — done, gegen frische DB verifiziert
          - [x] 0.2.2.1.2.3.5 Materiallisten (`ListProfilesBySingle`/`ListArticlesBySingle`/`ListGlassBySingle`/`LinkVariantMaterial`) — Ownership-Check über SingleElevation-Kette — done, gegen frische DB verifiziert
          - [x] 0.2.2.1.2.3.6 Projekt-Assets (`UpsertProjectAsset`/`GetProjectAsset`/`ListProjectAssets`) — Ownership-Check über Projekt — done, gegen frische DB verifiziert
          - [x] 0.2.2.1.2.3.7 (letzte Micro-Subtask) Imports & LogiKal-Import (`ListImports`/`ListImportChanges(Filtered)`/`UndoImport`/`ExportImportChangesCSV`) — Ownership-Check über Projekt — done, gegen frische DB verifiziert. `AnalyzeLogikal` bewusst NICHT geändert: liest nur die hochgeladene SQLite-Datei, schreibt/liest nie Postgres, kein Projektbezug vorhanden.
      - [x] 0.2.2.1.3 Angebote/Aufträge/Rechnungen/Bestellungen: `quotes`, `sales_orders`, `invoices_out`, `purchase_orders` — done, alle 4 Subtasks abgeschlossen und gegen frische DB verifiziert (§6.3: `quotes/service.go` allein 3543 Zeilen/41 HTTP-Call-Sites — deutlich größer als `projects`, das bereits 7 Micro-Subtasks brauchte; je Kopf-Tabelle/Domänen-Package eine Subtask, Reihenfolge klein→groß)
        - [x] 0.2.2.1.3.1 `purchase_orders` (`server/internal/purchasing/service.go`, 249 Zeilen, 8 Call-Sites) — done, gegen frische DB verifiziert. Items (`purchase_order_items`) erben Scope über `order_id`, kein eigenes `company_id` (ADR 0002).
        - [x] 0.2.2.1.3.2 `invoices_out` (`server/internal/accounting/ar.go`, 394 Zeilen, 5 Call-Sites) — done, gegen frische DB verifiziert. `invoice_out_items` erbt Scope über `invoice_id`, kein eigenes `company_id`. Cross-Package-Aufrufe `quotes.Service.ConvertToInvoice`/`sales.Service.ConvertToInvoice` → `arSvc.CreateFromQuoteTx`/`CreateFromSalesOrderTx` sowie `buildContactCommercialContext` (`http/commercial_context.go`) → `arSvc.List` um `companyID` erweitert (reiner Durchreich-Parameter, `quotes`/`sales_orders` selbst noch nicht gescoped — folgt in 0.2.2.1.3.3/.4).
        - [x] 0.2.2.1.3.3 `sales_orders` (`server/internal/sales/service.go`, 1032 Zeilen, 10 Call-Sites) — done, gegen frische DB verifiziert. `sales_order_items` erbt Scope über `sales_order_id`. Zusätzlich `buildContactCommercialContext`/`buildProjectCommercialContext` (`http/commercial_context.go`) und `buildCommercialWorkflow` (`http/workflow_cockpit.go`) um `companyID` erweitert (reichen an `salesSvc.List` durch).
        - [x] 0.2.2.1.3.4 `quotes` (`server/internal/quotes/service.go`, 3543 Zeilen, ~39 Service-Methoden, 41 Call-Sites) — done, alle 5 Micro-Subtasks abgeschlossen und gegen frische DB verifiziert (§6.3: größer als alle bisherigen Domänen dieser Session; grob vier fachliche Cluster + der separate GAEB-Import in `imports.go`)
          - [x] 0.2.2.1.3.4.1 Kern-CRUD (`Create`/`createQuoteTx`/`Get`/`List`/`Update`/`UpdateStatus`/`Revise`; `Accept`/`ConvertToInvoice` haben `companyID` bereits als Parameter aus 0.2.2.1.2.3.1/0.2.2.1.3.2, hier wird er zusätzlich zum Scopen der eigenen `quotes`-Zugriffe genutzt) — done, gegen frische DB verifiziert. `quote_items` erbt Scope über `quote_id` (ADR 0002). Zusätzlich 5 Funktionen aus den späteren Clustern (`ApplyMaterialCandidate`/`ApplySearchedMaterial`/`ApplyPriceSuggestionForQuoteItem`/`ApplyPrimaryPriceSourceForQuoteItem`/`ApplyTargetUnitPriceForQuoteItem`) erhielten denselben Quote-Ownership-Guard (`WHERE id=... AND company_id=...` auf ihrer bereits vorhandenen `quotes`-Sperrabfrage), da sie mechanisch von der `Get`-Signaturänderung betroffen waren — ihre TIEFERE Logik (Material-/Preis-Zugriffe) bleibt für 0.2.2.1.3.4.2/.3 offen. `ApplyImportToDraftQuote` (`imports.go`) bekam `companyID` rein durchgereicht (an `createQuoteTx`/`Get`), ohne die GAEB-Import-Tabellen selbst zu scopen (folgt in 0.2.2.1.3.4.5).
          - [x] 0.2.2.1.3.4.2 Material-Matching (`ApplyMaterialCandidate`/`SearchMaterialsForQuoteItem`/`ApplySearchedMaterial`/`listMaterialCandidatesForQuoteItem`) — done. `ApplyMaterialCandidate`/`ApplySearchedMaterial` waren bereits in 0.2.2.1.3.4.1 vollständig gescoped (mechanischer Nebeneffekt der `Get`-Signaturänderung); diese Subtask ergänzte nur noch `SearchMaterialsForQuoteItem` (hatte keinen `companyID`-Parameter, da sie `Get` nicht aufruft). `listMaterialCandidatesForQuoteItem` braucht keinen eigenen Check — einziger Aufrufer ist `Get` selbst, `quoteItemID` kommt aus bereits company-gescopten Zeilen.
          - [x] 0.2.2.1.3.4.3 Preisfindung (`SuggestPriceForQuoteItem`/`ApplyPriceSuggestionForQuoteItem`/`ApplyPrimaryPriceSourceForQuoteItem`/`ApplyTargetUnitPriceForQuoteItem`/`PriceHistoryForQuoteItem`/`PriceDecisionHistoryForQuoteItem`/`MarginAnchorForQuoteItem`/`ApprovalHintForQuoteItem`/`TargetMarginAnchorForQuoteItem`/`PriceSourcePriorityForQuoteItem`/`PriceEvaluationForQuoteItem`/`PriceDecisionTransparencyForQuoteItem`) — done. `Apply*` waren bereits aus 0.2.2.1.3.4.1 gescoped; die übrigen 9 Funktionen bekamen `company_id`-Guards + `companyID`-Durchreichung entlang ihrer internen Aufrufkette (`ApprovalHintForQuoteItem`/`TargetMarginAnchorForQuoteItem` → `MarginAnchorForQuoteItem`; `PriceSourcePriorityForQuoteItem` → `PriceHistoryForQuoteItem`; `PriceEvaluationForQuoteItem` → `PriceSourcePriorityForQuoteItem`; `PriceDecisionTransparencyForQuoteItem` → `PriceEvaluationForQuoteItem`). `quoteTargetMarginPercent` bewusst unverändert (liest eine globale, noch nicht mandantenscharfe Einstellung `quote_calculation_settings`, außerhalb Subtask-Scope).
          - [x] 0.2.2.1.3.4.4 Freigabe-Workflow (`RequestApprovalForQuoteItem`/`CancelApprovalRequestForQuoteItem`/`ApproveApprovalRequestForQuoteItem`/`RejectApprovalRequestForQuoteItem`/`ResolveApprovalReworkForQuoteItem`/`ListApprovalRequestsForQuoteItem`/`ListApprovalReworkQueue`/`ListApprovalRequestQueue`/`decideApprovalRequestForQuoteItem`) — done, gegen frische DB verifiziert. Die beiden Queue-Funktionen (`ListApprovalReworkQueue`/`ListApprovalRequestQueue`) waren zuvor GAR NICHT mandantengefiltert — echter, funktional relevanter Fund für Listen-Endpunkte über alle Angebote hinweg.
          - [x] 0.2.2.1.3.4.5 (letzte Micro-Subtask) GAEB-Import (`server/internal/quotes/imports.go`: `ListImports`/`ListImportItems`/`GetImport`/`GetImportItem`/`CreateGAEBImport`/`ProcessGAEBImport`/`MarkImportReviewed`/`UpdateImportItemReview`/`ApplyImportToDraftQuote`/`SaveImportParseResult`/`MarkImportFailed`) — done, gegen frische DB verifiziert. `quote_imports` hat keine eigene `company_id`-Spalte — alle Funktionen scopen jetzt per JOIN auf `projects p ON p.id = quote_imports.project_id` + `p.company_id=$N` (ADR-0002-konform, analog zum LogiKal-Import-Muster aus 0.2.2.1.2.3.7). Dabei zwei echte, in dieser Subtask selbst verursachte SQL-Bugs gefunden und behoben (nicht durch Compiler erkennbar, da rohes SQL): `ListImports`/`GetImport` hatten nach Einführung des `JOIN projects p` mehrdeutige Spaltenreferenzen (`id`/`status` existieren in beiden Tabellen) — behoben durch explizite `quote_imports.`-Qualifizierung aller SELECT-Spalten. Drei weitere, unabhängige Vorbefunde bei der Verifikation entdeckt und dokumentiert statt behoben (außerhalb Subtask-Scope): Backlog 0.29 (`classifyDomainError` erkennt mehrere GAEB-Fehlermeldungen nicht als 400), 0.30 (Test `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` paniced wegen `nil`-NumberingService im Testaufbau), 0.31 (Test `TestGAEBImportProcessEndpoint` nutzt falschen Router/Pfad-Präfix, bereits vor dieser Subtask so vorhanden).
      - [x] 0.2.2.1.4 Material/Lager: `materials`, `warehouses` — done, gegen frische DB verifiziert (`server/internal/materials/service.go` 594 Zeilen + `documents.go` 112 Zeilen, 16 HTTP-Call-Sites — als eine Subtask umgesetzt, vergleichbar mit `sales_orders`). Material-Kern-CRUD (`Create`/`Update`/`DeleteSoft`/`List`/`Get`) sowie `CreateWarehouse`/`ListWarehouses` (beides Kopf-Tabellen aus Migration 057) direkt gescoped. `locations` (Ownership-Check über neuen Helfer `warehouseOwnedByCompany`), `stock_movements`/`batches` (Ownership-Check über `materials`+`warehouses` innerhalb der Transaktion in `CreateMovement`; `StockByMaterial` nutzt `s.Get` als Ownership-Gate) und `material_documents` (`UploadMaterialDocument`/`ListMaterialDocuments`, Check vor GridFS-Upload) erben den Scope über FK (ADR 0002, kein eigenes `company_id`). Facetten `ListTypes`/`ListCategories` sowie `normalizeAndValidateCategory` (Kategorie-Validierung bei Create/Update) ebenfalls gefiltert — `material_groups` selbst bewusst NICHT gescoped (globale Referenzdaten, keine `company_id`-Spalte). `OpenDocumentStream` (`/documents/{docID}`) bewusst NICHT angefasst — bereits als Backlog 0.22 dokumentiert. Neuer, permanenter Test `server/internal/http/warehouses_integration_test.go` ergänzt (vorher KEINE Testabdeckung für `/warehouses`, `/stock-movements` vorhanden): Happy-Path-Flow (Material→Lager→Standort→Bewegung→Bestand) und ein Negativfall (`stock-movements` mit unbekannter `material_id` → 400). Zusätzlich `server/internal/materials/service_test.go` um `TestCreateRejectsMissingCompanyID` ergänzt (Negativfall analog zu allen anderen Domänen). Testfixture-Fix (kein neuer Fund, direkte Folge der Signaturänderung): `TestMaterialsUpdateAllowsExistingLegacyCategory` seedete ein Material per Roh-SQL ohne `company_id` — ergänzt um `company_id='default'`.
      - [x] 0.2.2.1.5 Buchhaltung: `accounts`, `journal_entries`, `bank_statements` — done, gegen frische DB verifiziert (`server/internal/accounting/journal.go` 118 Zeilen, `bank.go` 219 Zeilen, `payments.go` 156 Zeilen, `ar.go` bereits aus 0.2.2.1.3.2 gescoped, `settings/accounting.go` 73 Zeilen — als eine Subtask umgesetzt). `JournalService.Create`/`CreateTx`/`create` (`journal_entries`) und `AccountingService.ListAccounts` (`accounts`) direkt gescoped (`create` validiert `companyID` als ersten Check vor jedem DB-Zugriff). `BankService.Ingest`/`List`/`Match`/`findInvoiceByAmount`/`findInvoiceIDInReference` (`bank_statements`, eigene `company_id`-Spalte) gescoped. `PaymentService.Apply`/`apply`/`List` (`invoice_out_payments`, KEINE eigene `company_id`-Spalte, erbt über `invoice_id` — Ownership-Check via `EXISTS`-Query auf `invoices_out` in `List`, `AND company_id=$N` direkt in der `FOR UPDATE`-Sperrabfrage in `apply`) gescoped — **echter, HTTP-erreichbarer Fund**: `GET/POST /invoices-out/{id}/payments` hatte VORHER überhaupt keine Mandantenprüfung, obwohl `invoices_out` selbst bereits seit 0.2.2.1.3.2 gescoped war (jeder Nutzer mit `invoices_out.write`-Berechtigung hätte auf JEDE Rechnung irgendeines Mandanten eine Zahlung buchen können, solange die UUID bekannt war). `ar.go`s einziger `journal.CreateTx`-Aufruf (`Book`) um `companyID` ergänzt. `BankService`/`AccountingService` sind aktuell NICHT an HTTP-Handler angebunden (kein `v1.go`-Aufruf) — dennoch vollständig gescoped, da Teil des Anwendungscodes laut Subtask-Definition; kein neuer Fund (nur dokumentiert). Neuer Fund 0.32: kein Anwendungscode-Pfad legt für einen zweiten Mandanten einen Kontenrahmen (`accounts`) an.
      - [x] 0.2.2.1.6 (letzter Task von 0.2.2.1) HR: `hr_employees`, `hr_teams` — done, gegen frische DB verifiziert (`server/internal/hr/service.go`, 238 Zeilen, als eine Subtask umgesetzt). `EmployeeService.Create`/`Get`/`Update`/`List` sowie `ListTeams` direkt auf `company_id` gescoped (`hr_employees`/`hr_teams` beide eigene Spalte aus Migration 059; `Create` validiert `companyID` als ersten Check). `LeaveService.Create`/`Approve`/`Decide`/`List` (`hr_leave_requests`, KEINE eigene `company_id`-Spalte, erbt über `employee_id`) über neue Ownership-Checks gescoped — `Approve`/`Decide` nutzen ein `UPDATE ... WHERE id=$1 AND employee_id IN (SELECT id FROM hr_employees WHERE company_id=$N)` + `RowsAffected()==0`-Prüfung statt eines separaten Pre-Checks. **Kompletter `hr`-Domänenpackage ist aktuell an KEINEN HTTP-Handler angebunden** (`grep -rn "internal/hr" internal/http/` liefert keinen Treffer) — dennoch vollständig gescoped, da Teil des Anwendungscodes laut Subtask-Definition; kein neuer Fund (nur dokumentiert), kein akutes Risiko. Da keine HTTP-Integrationstests existieren, neue direkte Integrationstests (`server/internal/hr/service_scoping_integration_test.go`, gegen echtes Postgres via `testutil.SetupIntegrationEnv`) ergänzt: `TestEmployeeGetIsScopedToCompany` und `TestLeaveCreateRejectsEmployeeFromOtherCompany` beweisen echte Mandantentrennung (zweiter Mandant `other-company` sieht/erreicht die Daten des ersten nicht).
    - [x] 0.2.2.2 Middleware: Mandanten-Kontext aus Auth-Session ableiten und in Request-Context legen — done, vorgezogen und als 0.2.2.1.1 umgesetzt (siehe dort — `requireAuth` legt bereits den vollständigen `auth.User` inkl. `CompanyID`/`BranchID` in den Context, `companyIDFromContext`/`branchIDFromContext` in `v1.go` sind die Zugriffshelfer)
    - [x] 0.2.2.3 `number_sequences` pro Mandant statt global (aktuell `entity`-PK ohne Mandantenbezug, `server/internal/migrate/migrations/005_numbering.sql:1-7`) — done, gegen frische DB verifiziert. Migration `061_number_sequences_composite_pk.sql`: `company_id` auf `NOT NULL` gesetzt (alle Zeilen bereits `'default'` aus Migration 060), alter PK auf `entity` gedroppt, neuer zusammengesetzter PK auf `(company_id, entity)` angelegt, überflüssig gewordener Index entfernt. `settings.NumberingService.Get`/`UpdatePattern`/`Preview`/`Next` um `companyID` erweitert (`Next` validiert `companyID` als ersten Check vor Transaktionsbeginn, analog zum `Create`-Muster). Alle 6 Aufrufer (`ar.go`/`Book`, `projects/import_logikal.go`/`ImportLogikal`, `projects/service.go`/`Create`, `purchasing/service.go`/`Create`, `quotes/service.go`/`createQuoteTx`, `sales/service.go`/`CreateFromQuote`) hatten `companyID` bereits als Parameter aus den jeweiligen Domänen-Subtasks verfügbar — reine Durchreichung, keine Signaturänderung der aufrufenden Funktionen nötig. **Echter, HTTP-erreichbarer Fund**: `GET/PUT /settings/numbering/{entity}` (`server/internal/http/v1.go:3176-3214`) hatte VORHER überhaupt keine Mandantenprüfung und auch keine Testabdeckung — jeder Nutzer mit `settings.manage`-Berechtigung hätte Nummernkreis-Muster (`pattern`) irgendeines Mandanten lesen/ändern können. Jetzt gescoped + neuer Test `TestNumberingSettingsFlowIsScopedToOwnCompany` (`server/internal/http/settings_integration_test.go`) ergänzt (vorher keine Tests für diesen Endpunkt vorhanden).
- [x] 0.3 (0.3.1-3 abgeschlossen) GoBD-Fundament (Storno statt Delete, Festschreibung, Änderungsprotokoll) — done. Storno-Konzept für `invoices_out`/`journal_entries` (echte Umkehrbuchung statt Löschung/Änderung), Festschreibungs-Mechanismus für gebuchte/versendete Belege (drei von vier geprüften Domänen waren bereits geschützt, `purchasing` nachgerüstet) und generisches Änderungsprotokoll über die gesamte kommerzielle Belegkette (Angebot→Auftrag→Bestellung→Rechnung) umgesetzt und jeweils gegen echtes Postgres verifiziert. Zwei gravierende, unabhängige Pre-existing-Bugs bei der Verifikation entdeckt und dokumentiert, nicht behoben (Backlog 0.34, 0.35 — beide außerhalb des GoBD-Fundament-Scopes, eigenständige Fixes nötig).
  - [x] 0.3.1 Storno-Konzept für `invoices_out`/`journal_entries` (aktuell nur `draft|booked|paid`, kein Storno-Status) — done, gegen frische DB verifiziert. Migration `062_invoices_out_storno.sql`: `storno_journal_entry_id`, `storniert_am`, `storno_grund` an `invoices_out` ergänzt (kein CHECK-Constraint nötig, `status` war schon immer freier Text). GoBD-Kernprinzip umgesetzt: `ARService.Storno` ändert/löscht NIE die ursprüngliche Buchung (`journal_entry_id` bleibt unangetastet), sondern erzeugt über den bestehenden `JournalService` eine neue, vollständige Umkehrbuchung (`buildStornoJournal`: identische Konten/Beträge wie `buildJournal`, aber Soll/Haben vertauscht, `source='invoice_out_storno'`) — gegen echtes Postgres verifiziert (`journal_lines`-Vergleich Original vs. Storno zeigt exakt gespiegelte Werte, beide Buchungen bleiben bestehen). Nur aus Status `booked` UND `paid_amount=0` erlaubt — Rückabwicklung bereits erhaltener Zahlungen bewusst NICHT Teil dieses Konzepts (eigener Validierungsfehler "Rechnung hat bereits Zahlungen erhalten, Storno derzeit nicht unterstützt", `classifyDomainError` um `"zahlungen erhalten"` ergänzt). Neue Route `POST /invoices-out/{id}/storno` (Permission `invoices_out.write`, analog zu `/book`). `InvoiceOut`/`InvoiceListItem` um `storno_journal_entry_id`/`storniert_am`/`storno_grund` erweitert. **Bewusste Abgrenzung zu Task 0.3.2** (Festschreibungs-Mechanismus, noch offen): dieses Storno-Konzept macht `Update()`/andere Mutationen an gebuchten Rechnungen NICHT technisch unmöglich — die allgemeine Festschreibungs-Durchsetzung ist explizit ein eigener, separater Task. Neue Tests: `TestStornoRejectsMissingCompanyID`, `TestStornoRejectsMissingReason`, `TestBuildStornoJournalReversesDebitAndCreditButStaysBalanced` (Unit) sowie `TestInvoiceOutStornoFlow` (HTTP-Integrationstest: Storno auf `draft` abgelehnt, fehlender Grund abgelehnt, erfolgreicher Storno mit persistierten Feldern, wiederholter Storno abgelehnt) und eine Ergänzung in `TestInvoiceOutFlowWithPDFAndPayments` (Storno nach Zahlungserfassung abgelehnt).
  - [x] 0.3.2 Festschreibungs-Mechanismus für gebuchte/versendete Belege — done, gegen frische DB verifiziert. Recherche ergab: `quotes` (`Update`/alle Item-Mutationsfunktionen blockieren bei Status != `draft` bzw. bei historischen, durch `Revise` ersetzten Versionen), `sales_orders` (`isEditableStatus`/`ensureOrderEditableTx`, nur `open`/`released`) und `invoices_out` (kein `Update`-Pfad überhaupt vorhanden — implizit unveränderlich) waren bereits GoBD-konform geschützt, jeweils aus früheren Subtasks dieser Session, ohne dass es dort bewusst als "Festschreibung" benannt wurde. Der einzige echte Blast-Radius: `purchasing` (`server/internal/purchasing/service.go`) hatte GAR KEINEN Status-Check — jede Bestellung war unabhängig vom Status (auch `received`/`canceled`) frei änderbar, inkl. Positionen. Neuer Helfer `isEditableStatus` (nur `draft` editierbar) + Guard in `Update` (nur Inhaltsfelder Nummer/Datum/Währung/Notiz gesperrt, der Statuswechsel selbst bleibt möglich) sowie `CreateItem`/`UpdateItem`/`DeleteItem`. Gegen echtes Postgres per HTTP-Integrationstest bewiesen: Statuswechsel draft→ordered funktioniert weiterhin, danach werden Kopfdatenänderung, neue Position, Positionsänderung UND Positionslöschung alle korrekt mit `400` abgelehnt, die Position bleibt nachweislich unverändert (`menge=2`). Neuer Fund 0.33 (nicht behoben, andere Domäne): `quotes`-Schreibschutz-Meldungen liefern an 16 Stellen `500` statt `400`, da sie kein `classifyDomainError`-Muster treffen — der Schutz selbst greift korrekt, nur der HTTP-Status ist irreführend.
  - [x] 0.3.3 (alle 5 Micro-Subtasks abgeschlossen bzw. bewusst zurückgestellt) Generisches Änderungsprotokoll über Fachobjekte (aktuell nur domänenspezifisch bei LogiKal-Import und Quote-Preis-/Freigabe-Historie) — done, in Micro-Subtasks zerlegt (§6.3: potenziell jede Domäne betroffen, klassischer Fall für schrittweise Anbindung statt Großumbau in einer Subtask). Neue generische Infrastruktur (`entity_change_log` + `internal/auditlog`) in `invoices_out` (Book/Storno), `quotes` (Accept/Revise/Freigabe-Entscheidungen), `sales_orders` (Statuswechsel/ConvertToInvoice) und `purchase_orders` (Statuswechsel) verdrahtet. Dabei ZWEI echte, gravierende, unabhängige Pre-existing-Bugs gefunden und dokumentiert (Backlog 0.34: `quotes.Revise` "conn busy" bei jedem Angebot mit Positionen; Backlog 0.35: `sales.ConvertToInvoice` "FOR UPDATE + LEFT JOIN"-Fehler bei jedem Aufruf) — beide blockieren die betroffenen Funktionen komplett, unabhängig vom Änderungsprotokoll.
    - [x] 0.3.3.1 Generische Infrastruktur (Tabelle + Service) + Pilot-Integration in `invoices_out` — done, gegen frische DB verifiziert. Neue Tabelle `entity_change_log` (Migration `063_entity_change_log.sql`): `entity_type`/`entity_id` bewusst frei statt fester FK-Beziehung, damit dieselbe Tabelle von beliebigen Domänen genutzt werden kann (Unterschied zu den beiden vorhandenen domänenspezifischen Mustern `project_import_changes`/`quote_item_price_decisions`, die je an eine Domäne fest gebunden sind). Neues Package `server/internal/auditlog` (`Service.Record`/`List`, akzeptiert optional eine laufende Transaktion, damit der Protokolleintrag atomar mit der fachlichen Änderung geschrieben/zurückgerollt wird). Pilot-Integration in `accounting.ARService.Book`/`Storno` (neuer `actorUserID`-Parameter, aus `authUserFromContext` über den neuen Helfer `actorUserIDFromContext` in `v1.go`); neue, schreibgeschützte Route `GET /invoices-out/{id}/audit-log`. Gegen echtes Postgres per SQL-Direktabfrage bewiesen: `gebucht`- und `storniert`-Einträge enthalten korrekt `actor_user_id`, `before_data`/`after_data`-Snapshots und (beim Storno) die Begründung als `note`. Bewusste Scope-Grenze: NUR `invoices_out` angebunden — die Anbindung weiterer Domänen ist explizit auf die folgenden, noch offenen Micro-Subtasks verteilt, um die aufgabe.md-Vorgabe (max. ~8 Dateien/~400 Zeilen je Subtask) einzuhalten.
    - [x] 0.3.3.2 Anbindung `quotes` (Accept/Revise/Freigabe-Entscheidungen) — done, gegen frische DB verifiziert. `quotes.Service` bekam analog zu `WithMongo`/`WithGAEBImportParser` ein neues `WithAudit(*auditlog.Service)` (fluent Setter statt Konstruktor-Parameter, um die ~12 bestehenden Testdatei-Call-Sites von `NewService(...)` NICHT anfassen zu müssen — `s.audit` bleibt dort `nil`, Aufrufe werden dann übersprungen). Verdrahtet in `Accept` (`companyID`/neuer `actorUserID`-Parameter, EIN Aufrufer in `v1.go`), `Revise` (atomar innerhalb der bestehenden Transaktion vor `tx.Commit`, neuer `actorUserID`-Parameter, EIN Aufrufer), `decideApprovalRequestForQuoteItem` (atomar innerhalb der bestehenden Transaktion, nutzt den bereits vorhandenen Parameter `decidedBy` als Actor — KEINE Signaturänderung nötig, betrifft `Approve`/`RejectApprovalRequestForQuoteItem`). Neuer Helfer `actorUserIDFromContext` in `v1.go` (Schwester-Funktion zu `companyIDFromContext`). Gegen echtes Postgres per SQL-Direktabfrage bewiesen: `Accept` erzeugt einen `angenommen`-Eintrag, eine Freigabe-Entscheidung einen `freigabe_approved`-Eintrag mit `note`=Kommentar, beide mit korrektem `actor_user_id`. **Echter, gravierender, unabhängiger Fund** (Backlog 0.34, KRITISCH, nicht behoben): `Revise` scheitert mit `conn busy` bei JEDEM Angebot mit mindestens einer Position (also praktisch jedem real angelegten Angebot) — ein pgx-Fehler durch verschachteltes `tx.Exec` innerhalb einer noch offenen `tx.Query`-Iteration beim Kopieren der Positionen, nachweislich bereits vor dieser Session vorhanden (identisch reproduziert MIT und OHNE die neue Audit-Anbindung). `Revise`s Audit-Anbindung selbst konnte daher NICHT end-to-end über den HTTP-Pfad verifiziert werden (Code-Pfad ist aber strukturell identisch zu den zwei erfolgreich verifizierten Fällen).
    - [x] 0.3.3.3 Anbindung `sales_orders` (Statuswechsel, ConvertToInvoice) — done, gegen frische DB verifiziert. `sales.Service` bekam analog zu `quotes` ein `WithAudit(*auditlog.Service)` (fluent Setter, kein Konstruktor-Parameter — der einzige Aufrufer in `v1.go` sowie die 2 direkten Testdatei-Call-Sites bleiben dadurch minimal betroffen). Verdrahtet in `UpdateStatus` (atomar in bestehender Transaktion vor `tx.Commit`, neuer `actorUserID`-Parameter) und `ConvertToInvoice` (ebenso atomar, neuer `actorUserID`-Parameter). Neuer, permanenter Integrationstest `TestSalesOrderStatusChangeAndConvertToInvoiceAreAuditLogged` (`server/internal/http/accounting_integration_test.go`) — bewusst über `quotes`-Annahme → `convert-to-sales-order` geführt, um die bereits bekannten Vorbefunde 0.24/0.25 zu umgehen. Gegen echtes Postgres per SQL-Direktabfrage bewiesen: `UpdateStatus` (open→released) erzeugt einen korrekten `status_geaendert`-Eintrag mit echtem Actor. **Echter, gravierender, unabhängiger Fund** (Backlog 0.35, KRITISCH, nicht behoben): `ConvertToInvoice` selbst scheitert bei JEDEM Aufruf mit einem Postgres-Fehler ("FOR UPDATE cannot be applied to the nullable side of an outer join") in der bereits vor dieser Session vorhandenen `loadForInvoiceTx`-Sperrabfrage (`LEFT JOIN` + `FOR UPDATE` ohne `OF so`-Einschränkung) — `POST /sales-orders/{id}/convert-to-invoice` ist damit aktuell komplett unbenutzbar. `ConvertToInvoice`s Audit-Anbindung konnte deshalb nicht end-to-end verifiziert werden, ist aber strukturell identisch zum erfolgreich verifizierten `UpdateStatus`.
    - [x] 0.3.3.4 Anbindung `purchase_orders` (Statuswechsel, insb. der in 0.3.2 neu geschützte Übergang aus `draft`) — done, gegen frische DB verifiziert. `purchasing.Service` bekam ein `WithAudit(*auditlog.Service)` (gleiches Fluent-Setter-Muster). `Update` hatte VORHER keine eigene Transaktion (nur zwei sequenzielle `s.pg`-Aufrufe) — für atomares Audit-Logging in eine Transaktion gewickelt (`FOR UPDATE` auf die Status-Sperrabfrage ergänzt); das schließt nebenbei eine kleine, bereits vorhandene TOCTOU-Lücke des `isEditableStatus`-Content-Guards aus 0.3.2 (Nebeneffekt der korrekten Umsetzung, kein separater Fund). Protokolliert wird NUR ein tatsächlicher Statuswechsel (`u.Status != nil && currentStatus != *u.Status`) als `status_geaendert` — reine Inhaltsänderungen (Nummer/Datum/Währung/Notiz) erzeugen bewusst keinen Eintrag, analog zum engen Scope "Statuswechsel" im Task-Titel. Gegen echtes Postgres per SQL-Direktabfrage bewiesen: Übergang `draft`→`ordered` (der in 0.3.2 neu geschützte Übergang) erzeugt einen korrekten Eintrag mit echtem Actor. Bestehender Test `TestPurchaseOrdersAreLockedAfterLeavingDraftStatus` (aus 0.3.2) lief nach der Transaktions-Umstellung unverändert erfolgreich durch — bestätigt, dass die Restrukturierung das bestehende Verhalten nicht verändert hat.
    - [x] 0.3.3.5 Anbindung `contacts`/`projects`/`materials`/`hr` (falls nach 0.3.3.2-4 als sinnvoll bewertet) — done (bewertet, bewusst zurückgestellt statt umgesetzt). Nach Abschluss von 0.3.3.1-4 sind alle GoBD-primär-relevanten Belegtypen (Rechnungen inkl. Storno, Angebote inkl. Freigabe-Entscheidungen, Aufträge inkl. Rechnungsüberführung, Bestellungen) an das Änderungsprotokoll angebunden — das deckt die durchgängige kommerzielle Belegkette Angebot→Auftrag→Bestellung→Rechnung ab, die GoBD tatsächlich adressiert (Aufbewahrungs-/Nachvollziehbarkeitspflicht für steuerlich relevante Geschäftsvorfälle). `contacts`/`projects`/`materials`/`hr` sind Stammdaten- bzw. operative Domänen ohne direkten GoBD-Bezug (keine Buchungs-/Rechnungsdokumente) — eine Protokollierung dort wäre zwar techn. identisch möglich (Infrastruktur aus 0.3.3.1 ist bereits generisch genug), aber kein GoBD-Fundament-Bedarf, sondern ein allgemeines Audit-Feature. Bewusst NICHT umgesetzt, um Epic 0.3 auf seinen eigentlichen Zweck (GoBD) fokussiert zu halten; bei Bedarf als eigenständiges, neues Backlog-Item (nicht GoBD-Epic) nachträglich anlegen.
- [x] 0.4 (0.4.1-2 abgeschlossen) Migrationsverzeichnis bereinigen — done. ADR 0003 (Nummernkollisionen: Historie belassen, Konvention für neue Migrationen formalisiert) und ADR 0004 (verwaistes `server/migrations/`-Verzeichnis: als byte-identischer Merge-Rest identifiziert und gelöscht).
  - [x] 0.4.1 ADR: Umgang mit doppelten Nummernketten in `server/internal/migrate/migrations/` (001/007/013/014 doppelt vorhanden) — done, siehe `docs/adr/0003-migration-numbering.md`. Vollständige Bestandsaufnahme ergab 14 betroffene Nummern-Präfixe (nicht nur die im Titel genannten 4): `001`-`008`, `013`, `014`, `017`-`020` (bei `007` sogar DREI Dateien). Entscheidung: historische Duplikate bleiben unangetastet (funktionieren nachweislich seit dutzenden Migrationsläufen dieser Session gegen frische DB, kein Versions-Tracking existiert, das den alten Dateinamen referenziert — Umbenennen hätte keinen Laufzeit-Nutzen, nur Risiko); ab sofort gilt für neue Migrationen eine explizit dokumentierte Kontrakt-Regel ("vor dem Anlegen die höchste vorhandene Nummer prüfen, strikt darüber wählen" — bereits gelebte Praxis dieser Session ab Migration 054, jetzt formalisiert). Zeitstempel-Präfixe und ein externes Migrationstool (`golang-migrate`/`goose`) als Alternativen erwogen und mit Begründung verworfen (Stilbruch bzw. deutlich größerer, hier nicht angemessener Umfang — siehe ADR). Kurzer Hinweis-Kommentar in `server/internal/migrate/migrate.go` ergänzt, der auf die ADR verweist (keine funktionale Änderung, gegen frische DB verifiziert: kompletter Migrationslauf 001-063 weiterhin fehlerfrei).
  - [x] 0.4.2 (letzte Subtask von Task 0.4) ADR: Umgang mit verwaistem `server/migrations/`-Verzeichnis (9 Dateien, nicht eingebunden) — done, siehe `docs/adr/0004-orphaned-migrations-directory.md`. Alle 9 Dateien per `diff` als byte-identisch mit gleichnamigen, bereits aktiven Dateien in `server/internal/migrate/migrations/` bestätigt (löst nebenbei einen Teil des Duplikat-Rätsels aus 0.4.1 auf: `server/migrations/` war der alte Verzeichnisstandort vor der Verschiebung für `go:embed`-Kompatibilität). Repo-weite Suche bestätigt: keine Referenz in Go-Code/Docker/CI außerhalb eigener Recon-/Backlog-Notizen. Entscheidung: Verzeichnis vollständig löschen (`git rm -r server/migrations/`, uncommitted — nur gestaged, wie bei allen Änderungen dieser Session). Gegen frische DB verifiziert: kompletter Migrationslauf 001-063 weiterhin fehlerfrei, `go build`/`go vet` clean. **Task 0.4 (Migrationsverzeichnis bereinigen) ist damit vollständig abgeschlossen.**
- [x] 0.5 Auth-Härtung — done (alle vier Subtasks 0.5.1-0.5.4 abgeschlossen)
  - [x] 0.5.1 Startup-Guard gegen Default-`JWT_SECRET` in Produktivumgebung — done, verifiziert per Binary-Smoke-Test. `server/internal/config/config.go`: `cfg.JWTSecret = getenv("JWT_SECRET", "dev-secret-change-me")` fiel bisher in JEDER Umgebung inkl. Produktion still auf einen bekannten, im öffentlichen Repo sichtbaren Default zurück. Neues `Config.AppEnv`-Feld (liest `APP_ENV`, Default `"development"` — bereits in `.env.example` als Intention vorhanden, aber vom Code bisher nie gelesen). Neue, pur testbare Funktion `config.ValidateForStartup` (gibt nur einen Fehler zurück statt selbst `os.Exit` aufzurufen) prüft `AppEnv=="production"` (case-insensitiv) gegen `len(JWTSecret) < 32` — bewusst eine Längenschwelle statt einer Liste bekannter unsicherer Werte, damit auch künftige/andere schwache Secrets abgefangen werden, nicht nur die zwei aktuell bekannten (`dev-secret-change-me`, `change-me`). `cmd/api/main.go` ruft die Funktion vor jedem DB-/Service-Connect auf und beendet mit `os.Exit(1)`, falls sie einen Fehler liefert (Fail-Fast, gleiches Muster wie der bestehende `app.New`-Fehlerpfad). Per echtem Binary-Smoke-Test bewiesen (nicht nur Unit-Test): `APP_ENV=production JWT_SECRET=short` → Exit-Code 1 mit klarer Fehlermeldung; `APP_ENV=development JWT_SECRET=short` → startet normal weiter (kein Verhaltensbruch für bestehende Dev-/Test-Umgebungen, da weder `.env` noch `docker-compose.test.yml` `APP_ENV=production` setzen). `.env.example` um einen Hinweis auf den Guard ergänzt. Neue Tests `server/internal/config/config_test.go` (8 Fälle: production+Code-Default/Platzhalter/leer/Großschreibung/ausreichend lang, development+Code-Default, leeres AppEnv+Default, staging+kurz — jeweils erwartetes Ergebnis) sowie `TestLoadDefaultsAppEnvToDevelopment`.
  - [x] 0.5.2 Rate-Limiting auf `/auth/login` — done, verifiziert gegen frische DB/Redis. `auth.Service.Login`
    hatte zuvor keinerlei Brute-Force-Schutz (`user.IsLocked` ist nur ein manuell gesetztes Flag, kein
    automatischer Fehlversuchs-Zähler). Neue `auth.LoginRateLimiter` (`server/internal/auth/ratelimit.go`)
    nutzt den bereits vorhandenen Redis-Client (denselben wie `auth.SessionStore`, keine neue Abhängigkeit)
    als Fixed-Window-Zähler pro IP-Adresse (`net.SplitHostPort` trennt den Port ab). Zählt bewusst NUR
    fehlgeschlagene Versuche (`RegisterFailure`), Reset bei Erfolg (`Reset`) — nicht alle Requests, weil
    `httptest.NewRequest` ohne explizites `RemoteAddr` für jeden Request dieselbe Default-IP (`192.0.2.1`)
    vergibt und `loginIntegrationUser` (~40 Aufrufstellen im `internal/http`-Testpaket) genau diese Default-IP
    für erfolgreiche Logins nutzt — ein "alle Requests zählen"-Design hätte in einem vollen Testlauf jeden
    sich einloggenden Integrationstest geblockt. `Service.WithRateLimiter(*LoginRateLimiter)` als optionaler,
    nil-sicherer Fluent-Setter (analog `WithAudit`, kein bestehender Call-Site-Bruch). `config.Config` neu:
    `LoginRateLimitMaxAttempts` (Default 10, env `LOGIN_RATE_LIMIT_MAX_ATTEMPTS`),
    `LoginRateLimitWindowSeconds` (Default 60, env `LOGIN_RATE_LIMIT_WINDOW_SECONDS`). Handler in `v1.go`
    mappt neues `auth.ErrRateLimited` auf HTTP 429 (`rate_limited`). Tests: DB-lose Unit-Tests
    (`server/internal/auth/ratelimit_test.go` — Port-Stripping, Nil-Sicherheit, Deaktivierung bei
    `maxAttempts<=0`) sowie echter Integrationstest `TestAuthLoginRateLimitedAfterRepeatedFailures`
    (`server/internal/http/auth_integration_test.go`) mit eigener, isolierter Test-IP (nicht die geteilte
    Default-IP) und abgesenkter Schwelle (3 statt 10): 3× falsches Passwort → je 401, 4. Versuch → 429
    `rate_limited`, danach auch KORREKTES Passwort → weiterhin 429 (Sperre gilt pro IP, unabhängig vom
    Ausgang). Verifiziert per `go build ./...`, `go vet ./...`, `go test ./internal/auth/... ./internal/config/...`
    sowie gezieltem `go test ./internal/http/... -run TestAuthLogin` gegen frische DB (beide Tests PASS) —
    bewusst NICHT der volle `internal/http`-Lauf als Nachweis, da der bereits dokumentierte Backlog-0.20-Kipppunkt
    (Migrationen 050/051) sonst irrelevante Fehlschläge verursacht (siehe Update in 0.20).
  - [x] 0.5.3 `users.manage`-Bypass in `requirePermission()` durch dedizierte `admin.superuser`-Permission
    ersetzt — done, verifiziert gegen frische DB. Analyse: `requirePermission()` (`server/internal/http/v1.go`,
    Zeile hat sich durch diese Session verschoben, urspr. Fundstelle `:3634` bezog sich auf einen früheren
    Stand) ließ JEDE Berechtigungsprüfung durch, sobald der Nutzer `users.manage` besaß (`p == permission ||
    p == "users.manage"`) — ein zweites, identisches Bypass-Muster fand sich zusätzlich inline im
    Quote-Accept-Handler (`permission == "projects.write" || permission == "users.manage"`, für den optionalen
    Projektstatus-Wechsel). `users.manage` ist laut Seed-Daten (`017_auth.sql:77`) aber als ENGES
    "Benutzer/Rollen verwalten"-Recht gedacht (`context='platform'`, Name "Benutzer verwalten") — der Code
    missbrauchte es zusätzlich als Universal-Vollzugriff. Aktuell betrifft das praktisch nur die Rolle `admin`
    (einzige Rolle mit `users.manage`, und `admin` hat ohnehin bereits JEDE Einzelberechtigung direkt zugewiesen
    via `INSERT INTO role_permissions SELECT 'role-admin', p.id FROM permissions p` ohne WHERE-Filter, also kein
    aktueller Exploit) — der Bypass war aber eine stille Falle für die Zukunft: die in Subtask 0.5.4 geplante
    User-Management-API könnte eine neue, gezielt auf Benutzerverwaltung beschränkte Rolle mit NUR
    `users.manage` anlegen, die dadurch ungewollt systemweiten Vollzugriff erhalten hätte. Entscheidung (Option
    "ersetzen" statt nur "dokumentieren" gewählt, da Epic 0.5 explizit Auth-HÄRTUNG ist — ein Kommentar allein
    hätte die Falle nicht entschärft): neue Migration `064_admin_superuser_permission.sql` legt dediziertes
    `admin.superuser`-Permission an und vergibt es NUR an `role-admin` (idempotent, `ON CONFLICT DO NOTHING`,
    konsistent mit der etablierten Kein-Versions-Tracking-Konvention aus Backlog 0.13/ADR 0003). Beide
    Bypass-Stellen in `v1.go` auf neue Konstante `adminSuperuserPermission = "admin.superuser"` umgestellt (mit
    Kommentar, der die Trennung von `users.manage` begründet). Kein Verhaltensbruch für `admin` (hatte schon
    vorher jede Einzelberechtigung, bekommt jetzt zusätzlich explizit `admin.superuser`) und keine andere
    bestehende Rolle hat `users.manage`, verliert also nichts. Test: neuer Integrationstest
    `TestUsersManagePermissionNoLongerBypassesOtherPermissionChecks`
    (`server/internal/http/auth_integration_test.go`) legt gezielt eine Test-Rolle mit AUSSCHLIESSLICH
    `users.manage` an (da keine bestehende Rolle das isoliert testen könnte) und beweist: Zugriff auf
    `materials.write`-Endpunkt jetzt korrekt `403 forbidden` (vorher, mit dem Bypass, wäre es `201` gewesen).
    Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean für alle geänderten Dateien, gezielter
    `go test ./internal/http/... -run "TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"`
    gegen frische, per `\dt` bestätigt leere DB — alle 4 Tests PASS (inkl. bestehendem
    `TestMaterialsCreateIsForbiddenForSalesRole` als Regressionscheck). Per direkter SQL-Abfrage zusätzlich
    bestätigt: `admin.superuser` existiert genau einmal und ist ausschließlich an `role-admin` vergeben.
  - [x] 0.5.4 User-Management-API (Anlegen/Sperren/Rollenzuweisung) statt Direkt-SQL — done. Vor der ersten
    Umsetzung in drei Mikro-Subtasks zerlegt (Titel legt die Dreiteilung bereits nahe, geschätzter Umfang der
    Gesamt-Subtask lag klar über der ~400-Zeilen/~8-Dateien-Schwelle für eine einzelne Subtask): 0.5.4.1 Anlegen,
    0.5.4.2 Sperren/Entsperren, 0.5.4.3 Rollenzuweisung. Bisher gab es ÜBERHAUPT keinen HTTP-Weg für
    Benutzerverwaltung - ausschließlich Direkt-SQL (siehe `testutil.SeedAuthUser`, testonly, sowie manuelle
    Admin-Eingriffe in Produktion).
    - [x] 0.5.4.1 Anlegen (`POST /users`) — done, verifiziert gegen frische DB. Neue `auth.UserCreate`
      (`server/internal/auth/types.go`) als Eingabe-DTO (bewusst OHNE Rollenzuweisung - eigene Subtask 0.5.4.3,
      ein neu angelegter Nutzer hat zunächst keine Rollen/Berechtigungen). `Repository.CreateUser` +
      `EmailExists`/`UsernameExists` (Duplikat-Vorabprüfung nach dem in `contacts.Service.ensureNoDuplicate`
      etablierten Muster - SELECT vor INSERT statt Auswertung des Unique-Constraint-Fehlers, damit die
      Fehlermeldung `classifyDomainError`s bereits vorhandenes `"bereits vorhanden"`-Muster trifft, ohne dort
      etwas ändern zu müssen). `Service.CreateUser` validiert E-Mail-Format (enthält `@`), Mandant
      (`companyID` Pflichtparameter, aus `companyIDFromContext` im Handler), Passwort (wiederverwendet
      bestehendes `HashPassword`, dessen Leer-Prüfung/Fehlermeldung "passwort erforderlich" unverändert bleibt),
      setzt sinnvolle Defaults (Locale `de-DE`, Timezone `Europe/Berlin`, `display_name` aus Vor-/Nachname,
      falls nicht explizit gesetzt) sowie `is_active=true`/`is_locked=false`. Neue Route
      `protected.Route("/users", ...)` in `v1.go`, `POST /` mit `requirePermission("users.manage")` geschützt -
      die aus Subtask 0.5.3 nun korrekt eng skopierte Permission wird hier erstmals für ihren eigentlichen,
      benannten Zweck verwendet. Tests: DB-lose Validierungstests
      (`server/internal/auth/service_create_user_test.go` — ungültige/fehlende E-Mail, fehlender Mandant,
      leeres Passwort, alle vor dem ersten DB-Zugriff abgefangen, daher mit `repo=nil` testbar wie
      `newTestService()`) sowie zwei Integrationstests
      (`server/internal/http/auth_integration_test.go`): `TestUsersCreateEndpointCreatesUserWithinCallersCompany`
      (201, Mandant korrekt auf den des Aufrufers gesetzt, `password_hash` nie in der Antwort, **echter
      Login mit dem soeben vergebenen Passwort als Ende-zu-Ende-Beweis für korrektes Hashing** — nicht nur
      Strukturprüfung der Create-Antwort —, Duplikat-E-Mail liefert 400) und
      `TestUsersCreateEndpointIsForbiddenWithoutUsersManagePermission` (403 für eine Rolle ohne
      `users.manage`, wichtig als Regressionsbeleg nach 0.5.3s Verengung dieser Permission). Verifiziert:
      `go build ./...`, `go vet ./...`, `gofmt -l` clean für alle neuen/geänderten Dateien,
      `go test ./internal/auth/... ./internal/config/...` PASS, gezielt `go test ./internal/http/... -run
      "TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
      -count=1` gegen frische, per `\dt` bestätigt leere DB — alle 6 Tests PASS.
    - [x] 0.5.4.2 Sperren/Entsperren (`is_locked` umschalten) — done, verifiziert gegen frische DB. Neue
      Aktions-Endpunkte `POST /users/{id}/lock` und `POST /users/{id}/unlock` (analog zu bestehenden
      Status-Aktionen wie `.../accept`, `.../revise`, `.../storno` — bewusst kein rohes PATCH mit
      `is_locked`-Feld). `Repository.SetUserLocked` ist Mandanten-scoped über die UPDATE-WHERE-Klausel
      (`WHERE id=$2 AND company_id=$3`) — ein Sperr-Versuch gegen einen Benutzer eines ANDEREN Mandanten
      liefert `404` (`ErrUserNotFound`, neu), verrät also nicht einmal die Existenz des fremden Kontos
      (konsistent mit dem in Task 0.2 etablierten Mandanten-Scoping-Muster in den anderen Domänen).
      `Service.SetUserLocked` validiert nur `userID`/`companyID` (keine weitere Fachlogik nötig). Bewusst NICHT
      implementiert: ein Selbstsperr-Schutz (verhindern, dass sich der letzte `users.manage`-Inhaber selbst
      aussperrt) — das ist eine eigenständige Design-Entscheidung außerhalb des engen Task-Titels
      ("`is_locked` umschalten"), nicht Teil dieser Subtask, bei Bedarf als eigenes Backlog-Item nachtragbar.
      **Wichtiger Fund/Beweis bei der Verifikation**: die Sperre wirkt nicht nur gegen NEUE Logins (das prüft
      `Login()` bereits seit jeher via `user.IsLocked`), sondern auch gegen ein BEREITS ausgestelltes
      Access-Token, weil `AuthenticateAccessToken` `user.IsLocked` bei JEDEM authentifizierten Request frisch
      aus der DB neu liest, nicht nur beim Login — Sperren wirkt also spätestens ab dem nächsten Request, nicht
      erst nach Tokenablauf. Das wurde nicht nur im Code verifiziert, sondern per Integrationstest end-to-end
      bewiesen (bestehendes Access-Token wird nach der Sperre mit `401` abgelehnt). Tests: 2 DB-lose
      Validierungstests (`server/internal/auth/service_lock_user_test.go`) sowie 3 Integrationstests
      (`server/internal/http/auth_integration_test.go`):
      `TestUsersLockEndpointBlocksNewLoginsAndInvalidatesExistingSessions` (voller Zyklus: Zugriff vor Sperre
      OK → Sperren → bestehendes Token 401 UND neuer Login 401 → Entsperren → neuer Login wieder OK),
      `TestUsersLockEndpointReturnsNotFoundForUserInAnotherCompany` (per Direkt-SQL zweiter Mandant + Nutzer
      angelegt, da `testutil.SeedAuthUser` company_id fest auf `'default'` setzt und daher kein
      Cross-Tenant-Szenario liefern kann), `TestUsersLockEndpointIsForbiddenWithoutUsersManagePermission`.
      Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test ./internal/auth/...
      ./internal/config/...` PASS, gezielt `go test ./internal/http/... -run
      "TestUsersLock|TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
      -count=1` gegen frische, per `\dt` bestätigt leere DB — alle 9 Tests PASS.
    - [x] 0.5.4.3 Rollenzuweisung — done, verifiziert gegen frische DB. **Letzte Subtask von Task 0.5.4 und
      damit von Epic 0.5 (Auth-Härtung) — Epic 0.5 ist hiermit vollständig abgeschlossen.** Neue Endpunkte
      `GET /users/roles` (listet alle im System bekannten Rollen, Grundlage für eine künftige Rollenauswahl-UI)
      und `PUT /users/{id}/roles` (ERSETZT die komplette Rollenmenge eines Nutzers, bewusst PUT statt POST, da
      idempotent/ersetzend statt inkrementell hinzufügend). `Repository.ReplaceUserRoles`: Mandanten-Scoping wie
      bei `SetUserLocked` (Zielbenutzer muss zum Mandanten des Aufrufers gehören, sonst `404`/`ErrUserNotFound`,
      kein Existenz-Leak), validiert ALLE übergebenen Rollen-Codes VOR jeder Änderung (Alles-oder-nichts statt
      teilweise angewendeter Zuweisung bei einem Tippfehler in der Mitte der Liste — Fehlermeldung bewusst mit
      "ungültig" formuliert, damit `classifyDomainError`s bereits vorhandenes Muster greift), führt
      DELETE+INSERT atomar in einer Transaktion aus, trägt `assigned_by` (`actorUserIDFromContext`) in die
      bereits vorhandene `user_roles.assigned_by`-Spalte ein. `Service.ReplaceUserRoles` normalisiert die
      Eingabe (trimmt, verwirft Duplikate/Leerstrings) vor dem Repository-Aufruf. Tests: 2 DB-lose
      Validierungstests (`server/internal/auth/service_replace_user_roles_test.go`) sowie 6 Integrationstests
      (`server/internal/http/auth_integration_test.go`): `TestUsersRolesEndpointListsAvailableRoles`,
      `TestUsersRolesEndpointReplacesAssignmentAndTakesEffectOnNextLogin` (zentraler Test — beweist per ECHTEM
      Login VOR und NACH der Umzuweisung, dass die alte Rolle wirklich weg und die neue wirklich wirksam ist,
      nicht nur, dass die PUT-Antwort das behauptet; prüft zusätzlich, dass doppelte/leere Rollen-Codes in der
      Eingabe korrekt normalisiert werden), `TestUsersRolesEndpointRejectsUnknownRoleCode`,
      `TestUsersRolesEndpointReturnsNotFoundForUserInAnotherCompany`,
      `TestUsersRolesEndpointIsForbiddenWithoutUsersManagePermission`. Ein Tippfehler wurde bei der ersten
      Testausführung gefunden und sofort korrigiert: die Fehlermeldung nutzte zunächst das ASCII-"ungueltiger"
      (ohne Umlaut, konsistent mit anderen Fehlermeldungen in `repository.go`), traf damit aber NICHT
      `classifyDomainError`s Substring-Prüfung auf "ungültig" (mit ü) — führte zu `500` statt `400` im
      `TestUsersRolesEndpointRejectsUnknownRoleCode`-Testlauf; behoben durch Verwendung des Umlauts, erneuter
      Lauf danach grün. Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test
      ./internal/auth/... ./internal/config/...` PASS, gezielt `go test ./internal/http/... -run
      "TestUsersRoles|TestUsersLock|TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
      -count=1` gegen frische, per `\dt` bestätigt leere DB — alle 14 Tests PASS.
- [x] 0.6 Pre-existing Testfehler beheben: `purchasing.TestCreateRejectsInvalidItem` panict — done, verifiziert.
  - Gefunden bei Verifikation von Subtask 0.1.1.1 (`go test ./...`, unrelated zu Subtask). `Service.Create`
    (`server/internal/purchasing/service.go:86`) ruft `s.pg.Begin(ctx)` auf, **bevor** die Item-Validierung
    ("Ungültige Position") greift. Der Test instanziiert den Service mit `NewService(nil)`
    (`server/internal/purchasing/service_test.go:41-56`) — `s.pg.Begin(ctx)` auf nil `*pgxpool.Pool`
    löst einen `nil pointer dereference`-Panic in `pgxpool.(*Pool).Acquire` aus, statt den erwarteten
    Validierungsfehler zurückzugeben. Der Panic bricht den gesamten Testlauf des Pakets `purchasing` ab.
    **Fix umgesetzt**: neue Vorab-Prüfschleife über `in.Items` direkt nach den bestehenden frühen
    Validierungen (Mandant/Lieferant/Status), noch VOR `id := uuid.NewString()`/`s.pg.Begin(ctx)` — identische
    Prüflogik (`MaterialID leer || Qty==0 || UOM leer` → "Ungültige Position") wie zuvor, nur vorgezogen. Die
    jetzt redundante Prüfung INNERHALB der bestehenden Insert-Schleife (die nach `tx.Begin` läuft) wurde
    entfernt (tote Prüfung, da die Vorab-Schleife dieselbe Bedingung bereits garantiert hat) — kein
    Verhaltensunterschied, nur ein sauberer Diff ohne doppelte Logik. Produktivverhalten unverändert (im
    Produktivbetrieb ist `s.pg` nie nil, nur die Prüfreihenfolge hat sich geändert, das Fehlerergebnis bei
    einer ungültigen Position bleibt identisch: "Ungültige Position", kein Datenbank-Roundtrip mehr nötig, um
    das zu erkennen). Der jetzt veraltete Erklär-Kommentar über dem Test (der den Panic als bekannten,
    bewusst nicht behobenen Bug beschrieb) wurde entfernt.
    Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. `go test ./internal/purchasing/... -v
    -count=1`: alle 7 Tests PASS, `TestCreateRejectsInvalidItem` paniced NICHT mehr. Vollständiger `go test
    ./... -count=1` (alle Nicht-Integrationspakete): keine Panics mehr irgendwo im Modul. Regressionscheck
    gegen frische DB: `go test ./internal/http/... -run "TestPurchaseOrders" -count=1` — 3 von 4 Tests PASS,
    `TestPurchaseOrdersCreateAndGetFlow` schlägt weiterhin fehl, aber NACHWEISLICH aus einem völlig anderen,
    bereits dokumentierten Grund (Backlog 0.21, "Ungültige Materialkategorie" bei leerer `material_groups` auf
    frischer DB — per isoliertem `-run "^TestPurchaseOrdersCreateAndGetFlow$"`-Lauf bestätigt, Fehler tritt
    schon VOR jedem `purchasing`-Aufruf beim Anlegen des Test-Materials auf, also unabhängig von dieser
    Subtask).
- [x] 0.7 Steuerkennzeichen-Validierung in `accounting` gegen Stammdaten absichern — done, verifiziert gegen
  frische DB.
  - Gefunden bei Verifikation von Subtask 0.1.1.2 (`server/internal/accounting/ar.go:374-393`, Zeilennummern
    haben sich seither durch andere Subtasks verschoben). `taxRate()`/`taxAccountFor()` kannten hartcodiert nur
    `DE19`/`DE7`; jeder andere Wert (Tippfehler, zukünftig in `tax_codes` gepflegter Code) wurde `taxRate()`
    zufolge stillschweigend als steuerfrei (0 %) behandelt, `taxAccountFor()` fiel gleichzeitig auf das
    DE19-Steuerkonto `1776` zurück — inkonsistentes Fallback-Verhalten ohne Fehler/Warnung. Die bereits
    vorhandene `tax_codes`-Tabelle (`server/internal/migrate/migrations/017_accounting_basics.sql`) wurde dabei
    nicht konsultiert.
  - **Fix umgesetzt**: neue `loadTaxCodes(ctx, tx)` liest alle aktiven Steuerkennzeichen samt zugehörigem
    USt-Verbindlichkeitskonto (`accounts.type='liability' AND accounts.tax_code=tax_codes.code`) in eine
    `map[string]taxCodeInfo{Rate, LiabilityAccount}` — einzige Quelle der Wahrheit statt des hartcodierten
    Switches. `taxRate(codes, code)`/`taxAccountFor(codes, code)` bekamen neue Signaturen (`map[string]taxCodeInfo`
    als erster Parameter, zusätzlicher `error`-Rückgabewert): ein leerer Code bleibt bewusst kein Fehler (0 %,
    z.B. Skonto-/Durchlaufposten), aber ein nicht-leerer unbekannter/inaktiver Code liefert jetzt einen Fehler
    statt still zurückzufallen; `taxAccountFor` liefert ebenfalls einen Fehler, wenn ein bekannter Code KEIN
    Konto konfiguriert hat (z.B. `DE0`/`EU-RC`/`RC` in den aktuellen Seed-Daten). `calcTotals`/`buildJournal`/
    `buildStornoJournal` (allesamt zuvor pure Funktionen ohne DB-Zugriff) bekamen dieselbe
    `map[string]taxCodeInfo`-Signaturerweiterung plus `error`-Rückgabe, bleiben dadurch weiterhin pur/synchron
    testbar (die eigentliche DB-Konsultation passiert EINMAL pro Transaktion via `loadTaxCodes`, nicht verteilt
    über die Helper-Funktionen). Alle drei Aufrufstellen (`createTx`, `Book`, `Storno`) rufen `loadTaxCodes`
    jeweils innerhalb ihrer bestehenden Transaktion auf und propagieren Fehler korrekt.
  - Tests: bestehende `TestTaxRateKnownAndUnknownCodes`/`TestTaxAccountForKnownAndUnknownCodes` umgeschrieben
    (erwarten jetzt Fehler statt stillen Fallback für unbekannte Codes, neue `testTaxCodes()`-Fixtur simuliert
    die Stammdaten ohne echte DB), `TestCalcTotalsSumsNetAndTax`/`TestBuildJournalProducesBalancedEntry`/
    `TestBuildStornoJournalReversesDebitAndCreditButStaysBalanced` an die neuen Signaturen angepasst, neuer Test
    `TestCalcTotalsRejectsUnknownTaxCode`. Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean,
    `go test ./internal/accounting/...` (alle 20 Tests PASS), vollständiger `go test ./...` über alle
    Nicht-Integrationspakete ohne Fehler, sowie `go test ./internal/http/... -run "TestInvoiceOut" -count=1`
    gegen frische, per `\dt` bestätigt leere DB (`TestInvoiceOutFlowWithPDFAndPayments`,
    `TestInvoiceOutStornoFlow` — beide PASS, beweisen die volle Kette Anlegen→Buchen→Storno mit den echten,
    aus Postgres gelesenen Steuerkennzeichen).
  - **Neuer, verwandter Fund bei der Umsetzung** (bewusst NICHT mitgefixt, siehe Backlog 0.36): dasselbe
    hartcodierte `taxRate`-Muster (mit demselben stillen 0%-Fallback) existiert UNABHÄNGIG dupliziert in
    `internal/quotes/service.go` und `internal/sales/service.go` — der ursprüngliche Backlog-0.7-Fund erwähnte
    nur `accounting/ar.go`.
- [x] 0.8 Integrationstests für die zentralen Zahlungs-Guards in `payments.go` ergänzen — done, verifiziert
  gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.1.1.3. `PaymentService.apply()` (`server/internal/accounting/payments.go:70-86`)
    prüft drei fachlich zentrale Regeln erst NACH `tx.QueryRow` (Statusguard "Rechnung ist nicht gebucht",
    Währungsabgleich "Währung stimmt nicht mit Rechnung überein", Überzahlungsschutz "Zahlung übersteigt
    offenen Betrag") — diese sind ohne echte DB-Transaktion nicht unit-testbar (kein Mock/Testcontainer im
    Repo, siehe ADR 0001 / docs/00-recon.md). Grep über `server/internal/http/accounting_integration_test.go`
    zeigte: keine dieser drei Fehlermeldungen wurde dort geprüft — die einzige bestehende Integrationstest
    (`TestInvoiceOutFlowWithPDFAndPayments`) deckte nur den Happy-Path ab.
  - **Umgesetzt**: neuer Integrationstest `TestInvoiceOutPaymentGuardsRejectInvalidPayments`
    (`server/internal/http/accounting_integration_test.go`) deckt alle drei Regeln explizit ab, inkl.
    Prüfung der EXAKTEN Fehlermeldung (nicht nur Statuscode): (1) Zahlung auf eine noch nicht gebuchte
    (`draft`) Rechnung → `400` "Rechnung ist nicht gebucht"; (2) nach dem Buchen eine Zahlung in einer
    abweichenden Währung (`USD` statt `EUR`) → `400` "Währung stimmt nicht mit Rechnung überein"; (3) eine
    Zahlung über dem offenen Betrag (`gross_amount`=357, Zahlung=1000) → `400` "Zahlung übersteigt offenen
    Betrag". Zusätzlicher Regressionscheck am Ende: eine gültige Zahlung (100 EUR, innerhalb des offenen
    Betrags) wird trotz der drei vorherigen Ablehnungen weiterhin akzeptiert (`201`) — die Guards blockieren
    nur die ungültigen Fälle, nicht den Normalfall. Alle drei Fehlermeldungen waren bereits über
    `classifyDomainError`s bestehende Substring-Muster ("nicht gebucht", "stimmt nicht", "übersteigt") korrekt
    auf `400` gemappt — keine Änderung an `v1.go`/`classifyDomainError` nötig, reine Testergänzung wie im
    Backlog-Eintrag vorgesehen.
    Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test ./internal/http/... -run
    "TestInvoiceOut" -count=1` gegen frische, per `\dt` bestätigt leere DB — alle 3 Tests PASS (der neue plus
    die 2 bestehenden `TestInvoiceOutFlowWithPDFAndPayments`/`TestInvoiceOutStornoFlow` als Regressionscheck).
- [x] 0.9 Integrationstests für Bankabgleich/-matching ergänzen (`bank.go`) — done, verifiziert gegen frische
  DB.
  - Gefunden bei Verifikation von Subtask 0.1.1.4. `Ingest()`, `Match()` und `findInvoiceByAmount()`
    (`server/internal/accounting/bank.go:36-76,120-170,172-195`) greifen ohne jede Vorab-Validierung sofort
    auf Postgres zu — anders als bei `journal.go`/`ar.go`/`payments.go` gibt es hier praktisch keine
    DB-lose Validierungslogik. Grep über `accounting_integration_test.go` bestätigte: keine Integrationstests
    für Bankauszugs-Import oder -Matching vorhanden. Subtask 0.1.1.4 deckte nur den DB-losen
    Nicht-Treffer-Zweig der Referenz-Erkennung ab (`server/internal/accounting/bank_test.go`).
  - **Wichtige Erkenntnis vor der Umsetzung**: `NewBankService` wird per `grep -rln "NewBankService"` über das
    gesamte Modul NIRGENDS außerhalb von `bank.go` selbst aufgerufen — `BankService` ist an KEINEN
    HTTP-Handler angebunden, Bankabgleich ist über die API aktuell komplett unerreichbar. Als eigenständiger,
    größerer Fund NEU dokumentiert unter Backlog 0.37 (nicht Teil dieser Test-Subtask). Da kein HTTP-Pfad
    existiert, sind die neuen Tests zwangsläufig service-level (direkter Aufruf von `BankService`/`ARService`
    gegen echtes Postgres statt über HTTP) — exakt das bereits etablierte Muster aus
    `internal/hr/service_scoping_integration_test.go` für Domänen ohne HTTP-Anbindung, hier übernommen.
  - **Umgesetzt**: neue Datei `server/internal/accounting/bank_integration_test.go`, 6 Tests, decken exakt
    den im Backlog-Eintrag benannten Umfang ab: `TestBankIngestPersistsStatementAndAppliesPaymentWhenInvoiceIDProvided`,
    `TestBankMatchRejectsAlreadyMatchedStatement` ("Statement bereits gematcht"-Schutz),
    `TestBankMatchFindsInvoiceByReferenceNumber` (beweist per zwei Rechnungen mit IDENTISCHEM offenem Betrag,
    dass der Referenz-Pfad vor der Betrags-Heuristik greift — sonst zwingend Mehrdeutigkeitsfehler),
    `TestBankMatchFindsInvoiceByAmountWhenReferenceHasNoMatch`, `TestBankMatchReturnsErrorForAmbiguousAmount`,
    `TestBankMatchReturnsErrorWhenNoInvoiceMatches`.
  - **Zwei genuine, vorher unbekannte Bugs beim Schreiben dieser Tests gefunden UND behoben** (beide direkt in
    den durch diese Subtask zu testenden Funktionen selbst, daher im Scope, anders als z.B. Backlog 0.34/0.35,
    die in FREMDEN Funktionen bei anderer Gelegenheit auftauchten):
    1. `Ingest()`: `var raw any = in.Raw; if raw == nil { ... }` griff NIE (klassische Go-"typed nil in
       interface"-Falle: eine nil-Map, in ein `any` verpackt, ist als Interface != nil). Jeder Ingest ohne
       explizites `Raw`-Feld verletzte dadurch `bank_statements.raw NOT NULL`. Fix: Nil-Check auf die Map
       VOR dem Verpacken in ein Interface.
    2. `findInvoiceIDInReference()`: das erste Regex-Muster erkennt ein "RE"-Präfix, fing aber nur den
       nachfolgenden Zahlenteil ein (`m[1]`), OHNE das Präfix für den DB-Lookup wieder anzuhängen —
       `invoices_out.nummer` enthält laut Nummernkreis-Pattern (`017_accounting_basics.sql`:
       `'RE-{YYYY}-{NNNN}'`) aber immer das volle Präfix. Der Referenz-Erkennungs-Pfad fand dadurch NIE eine
       reale Rechnung, fiel immer auf die Betrags-Heuristik zurück. Fix: Präfix pro Muster mitführen und beim
       DB-Lookup wieder anhängen.
  - **Dritter Bug gefunden, bewusst NICHT hier mitgefixt** (liegt außerhalb von `bank.go`, im bereits in
    Backlog 0.7 angefassten `ar.go`): `createTx()` setzt bei leerem `TaxCode` keine SQL-NULL, sondern die
    leere Zeichenkette in `invoice_out_items.tax_code` — verletzt den Fremdschlüssel auf `tax_codes(code)`.
    Umgangen durch Verwendung von `TaxCode: "DE19"` in den Test-Rechnungen statt eines leeren Codes. Als neues
    Backlog 0.38 dokumentiert.
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. `go test ./internal/accounting/... -v
    -count=1` (alle 26 Tests im Paket PASS, inkl. der 6 neuen Bank-Tests) gegen frische, per `\dt` bestätigt
    leere DB. Vollständiger `go test ./...` über alle Nicht-Integrationspakete ohne Fehler.
- [x] 0.14 KRITISCH: gesamte Migrationskette systematisch gegen frische DB verifizieren und reparieren — done.
  Status-Korrektur (diese Session, nach 0.12): war noch als "wip, PRIORISIERT VOR 0.2.1.2.2+" markiert, obwohl
  alle drei Unterpunkte bereits `[x]` waren und der eigene Text schon "Kern-Ziel dieser Backlog-Position
  erreicht" festhielt — reine veraltete Buchführung, keine offene Arbeit. Die "PRIORISIERT VOR 0.2.1.2.2+"-
  Bedingung ist ohnehin gegenstandslos, da Task 0.2 (inkl. 0.2.1.2.2) längst abgeschlossen ist. Auf
  Nutzeranfrage erneut gegen eine wirklich frische, leere DB verifiziert (nicht nur den alten Notizen
  vertraut): `docker compose down -v`/`up`, `\dt` bestätigt leer, danach lief die komplette Kette 001-064
  (inkl. aller in dieser Session neu hinzugekommenen Migrationen bis 064) fehlerfrei durch, exakt derselbe
  `migrate.Run`-Codepfad wie beim echten Server-Start (`internal/app/server.go:56`). Die drei separat
  dokumentierten Folgefunde (0.16-0.18, siehe unten) wurden bei derselben Gelegenheit erneut gegen frische DB
  reproduziert und sind nach wie vor unverändert offen (kein Bezug zu 0.14 selbst, bleiben eigene
  Backlog-Positionen).
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
    - [x] 0.17 `TestContactDocumentsUploadListAndDownloadFlow` schlägt auf frischer DB fehl: "expected
      content disposition header" (`server/internal/http/contacts_integration_test.go:613`) — done, als
      Nebeneffekt von Backlog 0.22 gelöst.
      **Update**: bei Umsetzung von Backlog 0.22 (`contacts.Service.OpenDocumentStream` komplett neu gefasst,
      siehe dort) verifiziert per zweimaligem Testlauf (isoliert und im vollen Suite-Lauf), dass dieser Test
      jetzt zuverlässig grün ist. Plausibelste Erklärung (nicht tiefer debuggt, da der eigentliche Fix ohnehin
      nötig war): die alte Implementierung las Dateiname/Content-Type per `_ = s.pg.QueryRow(...).Scan(...)`
      NACH dem Öffnen des GridFS-Streams, mit STILL VERWORFENEM Fehler — schlug dieser Scan aus irgendeinem
      Grund fehl, blieb `filename` leer, wodurch der Handler in `v1.go` keinen `Content-Disposition`-Header
      setzte (`if filename != "" { ... }`). Die neue Implementierung liest dieselben Daten JETZT ZWINGEND
      (mit geprüftem Fehler) VOR dem Öffnen des Streams — ein Fehlschlag führt jetzt zu einem klaren `404`
      statt zu einem stillen, leeren Dateinamen.
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
- [x] 0.15 CI-Format-Check würde aktuell fehlschlagen: 40 Go-Dateien nicht gofmt-konform — done, verifiziert.
  - Gefunden beim Ausführen des exakten CI-Befehls (`.github/workflows/ci.yml:75-79`:
    `find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l`) während der Verifikation von
    Subtask 0.2.1.2.1. Ursprünglich 40 Dateien quer durchs gesamte `server/`-Modul betroffen — bis zur
    Bearbeitung dieser Subtask bereits auf 28 gesunken, weil mehrere zuvor bearbeitete Dateien (u. a.
    `internal/migrate/migrate.go`, `internal/accounting/ar.go`, `internal/hr/service.go`) durch substanzielle
    Neufassungen im Rahmen anderer Subtasks nebenbei vollständig gofmt-konform wurden.
  - **Umgesetzt**: `gofmt -w` über exakt die verbliebenen 28 per `gofmt -l` ermittelten Dateien (u. a.
    `cmd/api/main.go`, `internal/app/server.go`, `internal/auth/{service,store}.go` + Test, `internal/config/config.go`,
    `internal/db/{mongo,postgres,redis}.go`, `internal/http/{health,observability,router}.go` + Test,
    `internal/materials/{documents,jsonb}.go`, `internal/projects/analyze_logikal.go`, sieben Dateien in
    `internal/settings/`, `internal/testutil/integration.go`, `internal/version/version.go`) — keine anderen
    Dateien angefasst.
  - **Diff-Review** (wie im Fix-Vorschlag selbst als Sorgfaltsmaßnahme vorgesehen): `git diff -w` (ignoriert
    Zeilen-interne Whitespace-Unterschiede) zeigte für einen Teil der Dateien noch Differenzen — bei
    Überprüfung stellten sich diese ausschließlich als (a) bereits von FRÜHEREN Subtasks dieser Session
    eingefügter, unabhängiger Funktionscode (z. B. `auth/service.go`s Rate-Limiting/User-Management aus
    0.5.2/0.5.4.1, der beim Diff gegen den letzten Commit ohnehin sichtbar wird) und (b) legitime,
    semantik-neutrale gofmt-Normalisierungen heraus: entfernte überflüssige Leerzeilen am Dateiende
    (`db/postgres.go`, `db/redis.go`, `version/version.go`) und Import-Neusortierung
    (`http/router.go`: `nalaerp3/internal/version` alphabetisch hinter `nalaerp3/internal/config`
    einsortiert — ändert die Programmsemantik nicht, Go-Imports sind reihenfolge-unabhängig). Keine
    Einzeiler-`if`-Umformatierung tatsächlich angetroffen (Go erzwingt ohnehin überall Klammern, anders als
    z. B. C — die im ursprünglichen Fund befürchtete Kategorie kam in der Praxis nicht vor).
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` (leer, alle 28 Dateien jetzt konform). Voller
    `go test ./...` (Nicht-Integrationspakete) ohne Fehler. Vollständiger `NALA_INTEGRATION=1 go test ./...`
    gegen frische DB: IDENTISCHE Fehlerliste wie vor der Reformatierung (27 bekannte HTTP-Fehlschläge + 5
    bekannte `contacts.telefon`-Fehlschläge in `quotes`, alle bereits anderweitig dokumentiert) — keine einzige
    neue Regression durch die Reformatierung.
- [x] 0.20 KRITISCH: Migrationen 050/051 sind nicht sicher wiederholt ausführbar (Check-Constraint-Kollision) — done,
  als Nebeneffekt von Backlog 0.13 gelöst.
  - Gefunden bei Verifikation von Subtask 0.2.1.2.3 (voller `go test ./internal/http`-Lauf gegen dieselbe
    Postgres-Instanz über viele Testfunktionen hinweg — jede ruft `testutil.SetupIntegrationEnv` auf, die
    `migrate.Run` erneut komplett durchlaufen lässt, siehe Backlog 0.13: kein Versions-Tracking). Muster:
    `050_quote_item_approval_requests.sql:60-67` fügt (nur falls noch nicht vorhanden, per `pg_constraint`-
    Check) den Constraint `chk_quote_item_approval_requests_cancelled_state` hinzu; `051_quote_item_approval_decisions.sql:23-30`
    entfernt ihn **im selben Durchlauf sofort wieder** (falls vorhanden). Nach dem ersten `migrate.Run`-Durchlauf
    ist der Constraint also wieder weg — beim NÄCHSTEN `migrate.Run`-Durchlauf (nächste Testfunktion, gleiche
    DB, jetzt mit Testdaten aus vorherigen Läufen) versucht 050 ihn erneut anzulegen, was fehlschlägt, sobald
    zwischenzeitlich Zeilen eingefügt wurden, die die ursprüngliche (durch 051 eigentlich schon abgelöste)
    Regel verletzen: `ERROR: check constraint ... is violated by some row (SQLSTATE 23514)`.
    **Tragweite**: bricht `migrate.Run` bei JEDEM zweiten+ Aufruf gegen dieselbe, bereits benutzte DB ab —
    betrifft ca. 30 Testfunktionen in `quotes_integration_test.go`/`settings_integration_test.go` in einem
    vollen `go test ./internal/http`-Lauf (nicht nur bei `-run`-gefiltertem Einzelaufruf).
    **Update (Verifikation von Subtask 0.5.2)**: erneut beobachtet, diesmal mit präziserer Tragweite - in
    einem vollen `go test ./internal/http/... -count=1`-Lauf waren ca. 70 von ~75 Testfunktionen im GESAMTEN
    Paket betroffen (jede Testfunktion, die nach dem Kipppunkt in der Ausführungsreihenfolge des Testbinaries
    liegt, nicht nur die beiden ursprünglich genannten Dateien) - die ursprüngliche Schätzung "~30" war zu
    niedrig. Bestätigt außerdem, dass die Reihenfolge deterministisch ist (Dateireihenfolge + Deklarationsreihenfolge,
    kein `t.Parallel()` im Paket), der Fehler also reproduzierbar an derselben Stelle auftritt. Nicht behoben
    (weit außerhalb Subtask-Scope, eigenständiges Migrations-Architekturproblem). Fix: entweder 050s
    Constraint-Definition direkt an die 051-Fassung anpassen (Constraint nicht in 050 hinzufügen und in 051
    sofort wieder entfernen, sondern gleich korrekt in 050 definieren), oder Migrationsrunner um
    Versions-Tracking ergänzen (Backlog 0.13), damit jede Migration nur einmal läuft.
  - **Gelöst bei Umsetzung von Backlog 0.13** (Versions-Tracking, siehe dort und ADR 0005): mit
    `schema_migrations` läuft jede Migration nur noch EINMAL, ein erneuter `migrate.Run()` gegen dieselbe DB
    (z. B. jede weitere Testfunktion über `testutil.SetupIntegrationEnv`) überspringt 050/051 komplett, sobald
    sie einmal angewendet wurden — der Konflikt kann strukturell nicht mehr auftreten. Verifiziert per vollem
    `go test ./internal/http/... -count=1`-Lauf gegen frische DB: die zuvor ca. 70 von ~75 kaskadierenden
    Fehlschlägen sind auf 27 zurückgegangen, ALLE 27 einzeln auf bereits anderweitig dokumentierte Ursachen
    zurückgeführt (0.19/0.21/0.24-artiges Muster/0.26/0.28/0.29/0.34/0.35/0.38) bis auf einen neuen,
    unabhängigen Fund (Backlog 0.39). Kein Bezug mehr zu 050/051 in der verbliebenen Fehlerliste.
- [x] 0.21 `TestMaterialsCreateListAndGetFlow` schlägt auf frischer DB fehl: "Ungültige Materialkategorie" —
  done, verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.2.1.2.4. Testfixture nutzt `"kategorie":"integration"`
    (`server/internal/http/materials_integration_test.go:25`) — auf einer wirklich leeren DB ist
    `material_groups` leer (wird laut `039_material_groups.sql` nur aus bereits vorhandenen
    `materials.kategorie`-Werten befüllt), daher lehnt die Validierung die unbekannte Kategorie ab.
    Gleiches Muster wie Backlog 0.18/0.19 (Tests wurden nie gegen eine wirklich leere, ungeseedete DB
    verifiziert). Unabhängig von `company_id`/`branch_id` bestätigt (Fehlermeldung ohne jeden Bezug dazu).
  - **Root-Cause-Analyse**: `Service.normalizeAndValidateCategory` (`server/internal/materials/service.go:616-644`)
    lässt eine Kategorie nur zu, wenn sie ENTWEDER bereits in `material_groups` steht ODER bereits mindestens
    ein `materials`-Datensatz mit genau dieser Kategorie existiert — auf einer wirklich leeren DB kann WEDER
    Bedingung je erfüllt sein: `material_groups` ist leer (nur rückwirkende Befüllung aus vorhandenen
    `materials`), und es gibt noch keine `materials`-Zeile. Ein echter Chicken-Egg-Zustand, der NICHT nur ein
    Testfixture-Problem ist, sondern jede echte Neuinstallation gleichermaßenträfe: ohne manuellen Bootstrap
    könnte NIEMAND je das erste Material irgendeiner neuen Kategorie anlegen. Die Architektur sieht dafür
    bereits den korrekten, vorgesehenen Weg vor: eine dedizierte Materialgruppen-Verwaltung
    (`POST/GET/DELETE /settings/material-groups`, `settings.MaterialGroupService`, gated hinter
    `settings.manage`) — Kategorien müssen dort zuerst explizit angelegt werden, bevor Materialien sie
    verwenden können. Die betroffenen Tests bildeten diesen vorgesehenen Ablauf bisher nicht nach.
  - **Umgesetzt**: `TestMaterialsCreateListAndGetFlow` legt jetzt zuerst per `POST /settings/material-groups/`
    die Kategorie "integration" an, bevor das Material erstellt wird — genau der reale, vorgesehene Ablauf.
    Zusätzlich neuer, wiederverwendbarer Test-Helper `ensureIntegrationMaterialGroup`
    (`server/internal/http/purchase_orders_integration_test.go`, gleiches Package `apihttp`), den
    `createIntegrationMaterial` jetzt automatisch aufruft, wenn ein `kategorie`-Feld im Request-Body gesetzt
    ist — behebt dadurch NEBENBEI auch `TestPurchaseOrdersCreateAndGetFlow` (nutzt denselben Helper, war
    ebenfalls betroffen, aber nicht im ursprünglichen Fund benannt) ohne Änderung an dessen eigenem Code.
    Zusätzlich 6 Aufrufe von `ensureIntegrationMaterialGroup(t, handler, accessToken, "profile")` in
    `server/internal/http/quotes_integration_test.go` ergänzt (je einer direkt nach dem Login in
    `TestQuoteMaterialSearchEndpointSupportsOpenDraftItemSearch`,
    `TestQuoteMaterialSearchApplyEndpointSupportsVisibleSearchResultApply`,
    `TestQuotePriceSuggestionEndpointSupportsMappedDraftItem`,
    `TestQuoteApplyPriceSuggestionEndpointSupportsMappedDraftItem`,
    `TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem`,
    `TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources` — alle sechs nutzen dieselbe lokale
    `kategorie:"profile"`-Inline-Fixture und waren vom selben Root Cause betroffen, obwohl im ursprünglichen
    Fund nicht namentlich genannt).
  - **Ergebnis**: `TestMaterialsCreateListAndGetFlow` und `TestPurchaseOrdersCreateAndGetFlow` sind jetzt
    vollständig grün. Die 6 Quote-Tests scheitern weiterhin, aber NACHWEISLICH an einer jeweils SPÄTEREN,
    zuvor durch 0.21 verdeckten, unabhängigen Stelle im selben Testablauf (nicht mehr an der
    Materialkategorie) — als neue Backlog-Positionen 0.40 und 0.41 dokumentiert, bewusst NICHT hier
    mitgefixt (anderer Bug, anderer Scope).
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Gezielt `go test ./internal/http/... -run
    "TestMaterials|TestPurchaseOrders|TestQuoteMaterialSearch...|TestQuotePrice..."` gegen frische, per `\dt`
    bestätigt leere DB. Vollständiger `go test ./internal/http/... -count=1`: von 27 auf 25 Fehlschläge
    zurückgegangen (die beiden jetzt behobenen Tests fehlen in der Liste, alle anderen unverändert). Voller
    `go test ./...` (Nicht-Integrationspakete) ohne Fehler.
- [x] 0.40 `classifyDomainError` erkennt weitere `quotes`-Domänen-Validierungsfehler nicht als 400 (Material-
  Mapping) — done, zusammen mit 0.24 und 0.29 behoben, siehe Notiz bei 0.24 für Details.
  - Gefunden bei Umsetzung von Backlog 0.21 (nachdem der Materialkategorie-Fehler behoben war, kamen fünf
    Quote-Tests weiter und scheiterten an einer NEUEN, vorher verdeckten Stelle). Gleiches Muster wie Backlog
    0.29 (dort GAEB-Import-spezifische Meldungen), hier andere `errors.New(...)`-Meldungen aus der
    `quotes`-Domäne (`server/internal/quotes/service.go`, Material-Mapping-Bereich, per `grep` verifiziert),
    die keins der Substring-Muster in `classifyDomainError` trafen und daher auf `500 internal_error` statt
    `400 validation_error` fielen: "Angebotsposition hat bereits ein Material"
    (`TestQuoteMaterialSearchEndpointSupportsOpenDraftItemSearch`), "material_id ist kein sichtbarer
    Suchtreffer" (`TestQuoteMaterialSearchApplyEndpointSupportsVisibleSearchResultApply`), "Angebotsposition
    hat kein Material" (`TestQuotePriceSuggestionEndpointSupportsMappedDraftItem`,
    `TestQuoteApplyPriceSuggestionEndpointSupportsMappedDraftItem`,
    `TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem`).
  - Verifiziert: alle fünf genannten Tests PASS gegen frische DB (siehe Backlog 0.24 für den vollständigen
    Verifikationslauf, der alle drei Backlog-Positionen gemeinsam abdeckt).
- [ ] 0.41 `TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources` schlägt auf frischer DB fehl:
  Positionsbeschreibung eines fremden Tests taucht im Ergebnis auf — todo
  - Gefunden bei Umsetzung von Backlog 0.21 (nachdem der Materialkategorie-Fehler behoben war, kam dieser Test
    weiter und scheiterte an einer NEUEN, vorher verdeckten Stelle: `quotes_integration_test.go:5257`,
    erwartete Angebotsposition mit `Description:"Preisquellen Position"`, bekam stattdessen
    `Description:"Preispriorisierung Position"` — letztere stammt erkennbar aus einem ANDEREN Test in
    derselben Datei). Ursache noch nicht analysiert; möglicherweise verwandt mit Backlog 0.19 (Tests teilen
    sich dieselbe Postgres-Instanz ohne Datenbereinigung zwischen Testfunktionen, wodurch bei ungenügend
    scharfer Filterung Datensätze aus anderen Testläufen sichtbar werden). Nicht behoben (außerhalb des auf
    Materialkategorien beschränkten Scopes von 0.21, eigenständige Ursache noch zu klären). Fix: zuerst
    reproduzieren und Ursache ermitteln (z. B. fehlender Filter nach Quote-ID oder Contact-ID in der
    Abfrage, die die Positionsliste liefert).
- [x] 0.22 `GET /api/v1/documents/{docID}` prüft keine Mandantenzugehörigkeit — done, verifiziert gegen
  frische DB.
  - Gefunden bei Subtask 0.2.2.1.2.2.5. Der generische Download-Endpunkt (`server/internal/http/v1.go`,
    Permission `documents.read`) probierte erst `matSvc.OpenDocumentStream(docID)`, dann als Fallback
    `conSvc.OpenDocumentStream(docID)` — beide Methoden nahmen nur eine GridFS-`documentID` entgegen, OHNE
    jemals die Mandantenzugehörigkeit des zugehörigen Datensatzes (`material_documents`/`contact_documents`)
    zu prüfen. Tatsächlich noch schwerwiegender als im ursprünglichen Fund beschrieben: `OpenDocumentStream`
    öffnete den GridFS-Stream direkt per ObjectID OHNE JEDE Prüfung, ob überhaupt eine
    `material_documents`/`contact_documents`-Verknüpfung existiert — jeder authentifizierte Nutzer mit
    `documents.read` konnte damit JEDE Datei im gesamten GridFS-Bucket herunterladen (nicht nur
    mandantenübergreifend, sondern komplett ungebunden an irgendeinen Datensatz), solange die ObjectID bekannt
    oder erraten war.
  - **Umgesetzt**: `materials.Service.OpenDocumentStream`/`contacts.Service.OpenDocumentStream` bekamen einen
    neuen Pflichtparameter `companyID`. Beide prüfen JETZT VOR dem Öffnen des GridFS-Streams zwingend (mit
    geprüftem, nicht mehr verworfenem Fehler) per `JOIN` auf die übergeordnete Tabelle
    (`material_documents ⋈ materials` bzw. `contact_documents ⋈ contacts`), dass ein Eintrag mit dieser
    `document_id` existiert UND zum `companyID` des Aufrufers gehört — sonst `"Dokument nicht gefunden"`
    (→ `404`, kein Existenz-Leak über Mandantengrenzen, konsistent mit dem in Task 0.2 etablierten Muster).
    Handler in `v1.go` liest `companyID` jetzt aus dem Context und reicht ihn an beide Aufrufe durch.
  - **Wertvoller Nebeneffekt**: dabei wurde nebenbei auch der bereits dokumentierte, unabhängige Fund Backlog
    0.17 gelöst (`TestContactDocumentsUploadListAndDownloadFlow` fehlte bisher der `Content-Disposition`-
    Header) — die alte Implementierung las Dateiname/Content-Type NACH dem Öffnen des Streams mit STILL
    VERWORFENEM Fehler; die neue liest dieselben Daten ZWINGEND davor, siehe Update-Notiz bei 0.17.
  - Tests: neue Datei `server/internal/http/documents_integration_test.go`,
    `TestDocumentDownloadIsScopedToCompany` — legt per Direkt-SQL einen zweiten Mandanten samt Nutzer an
    (`testutil.SeedAuthUser` setzt `company_id` fest auf `'default'`, kein Cross-Tenant-Szenario möglich),
    lädt je ein Dokument für einen Kontakt UND ein Material im ersten Mandanten hoch, beweist: Eigentümer
    kann beide herunterladen (`200`), der andere Mandant erhält für BEIDE `404`. Neuer Helper
    `uploadIntegrationDocument` (multipart-Upload, wiederverwendbar).
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, gezielter Testlauf (neuer Test +
    `TestContactDocumentsUploadListAndDownloadFlow` + alle `TestMaterials*`) gegen frische, per `\dt`
    bestätigt leere DB — alle PASS. Vollständiger `go test ./internal/http/... -count=1`: von 25 auf 24
    Fehlschläge zurückgegangen (0.17 verschwindet aus der Liste). Voller `go test ./...`
    (Nicht-Integrationspakete) ohne Fehler.
- [x] 0.23 `projects.BuildQuoteSnapshot` referenziert nicht existierende Spalte `contacts.telefon` — done,
  verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.2.3.1 (`GET /api/v1/projects/{id}/quote-pdf`,
    `server/internal/projects/service.go`, vor meiner Änderung bereits so vorhanden — bestätigt per
    `git show HEAD:server/internal/projects/service.go`, Zeile unverändert von mir übernommen). Die Query
    selektierte `COALESCE(c.telefon,'')` aus `contacts`, die Spalte heißt dort aber `phone`
    (`server/internal/migrate/migrations/003_contacts.sql:9`). Jeder Aufruf von `BuildQuoteSnapshot`
    (Angebots-PDF aus Projektkontext) schlug daher mit `SQLSTATE 42703` fehl — reproduzierbar gegen
    frische DB (`TestProjectQuotePDFFlow`). Unabhängig von `company_id`/`branch_id` (reiner Tippfehler im
    Spaltennamen, vermutlich schon immer kaputt).
  - **Umgesetzt**: `COALESCE(c.telefon,'')` → `COALESCE(c.phone,'')`. Per `grep -rn "telefon"` über das
    gesamte `server/`-Modul (ohne `_test.go`) bestätigt: keine weitere Fundstelle in Produktivcode — die
    JSON-Feldnamen `Telefon`/`"telefon"` in `internal/contacts/service.go` sind KEIN Bug, sondern die
    (bewusst deutschsprachige) API-Vertragsbezeichnung, die dort korrekt auf die Spalte `phone` gemappt wird
    (`INSERT INTO contacts (..., phone, ...) VALUES (..., in.Telefon, ...)`, verifiziert).
  - **Gleicher Tippfehler auch in vier Test-Fixtures gefunden und mitkorrigiert** (identisches Muster wie
    Backlog 0.27, per `grep -rn "telefon" --include="*_test.go"` systematisch aufgespürt — rohe SQL-`INSERT`-
    Statements, die `telefon` statt `phone` als Spaltennamen verwenden, nicht die JSON-Bodies der Tests, die
    korrekterweise `"telefon"` als API-Feld verwenden): `server/internal/quotes/approval_decisions_test.go:31`
    (= Backlog 0.27s eigentliches Ziel), `server/internal/quotes/imports_test.go:42` und `:130`,
    `server/internal/http/quotes_integration_test.go:47`.
  - **Ergebnis, inkl. neu sichtbar gewordener Folgefunde**: `TestProjectQuotePDFFlow` (0.23s eigentliches
    Ziel) und `TestApprovalRequestDecisionsMutateOnlyRequest` (0.27) sind jetzt vollständig grün. Die drei
    `TestProcessGAEBImport*`-Tests sowie `TestQuoteImportParseResultStoresItemsAndUpdatesStatus` kommen jetzt
    weiter, scheitern aber an einer NEUEN, vorher verdeckten Stelle (`projects.nummer NOT NULL` ohne Wert in
    einem rohen Test-INSERT) — als neues Backlog 0.42 dokumentiert, bewusst NICHT hier mitgefixt.
    `TestGAEBImportProcessEndpoint` kommt ebenfalls weiter, scheitert aber jetzt sichtbar an dem bereits
    dokumentierten Backlog 0.31 (fehlendes `/api/v1`-Präfix bei `NewV1RouterWithOptions`).
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. `go test ./internal/quotes/... -v
    -count=1` gegen frische, per `\dt` bestätigt leere DB. Gezielt `go test ./internal/http/... -run
    "TestGAEBImportProcessEndpoint|TestProjectQuotePDFFlow"`. Vollständiger `go test ./internal/http/...
    -count=1`: von 24 auf 23 Fehlschläge zurückgegangen. Voller `go test ./...` (Nicht-Integrationspakete)
    ohne Fehler.
- [ ] 0.42 `projects`-Testfixtures in `internal/quotes` legen Projekte per Direkt-SQL ohne Pflichtfeld
  `nummer` an — todo
  - Gefunden bei Umsetzung von Backlog 0.23 (nachdem der `telefon`-Tippfehler behoben war, kamen vier Tests
    weiter und scheiterten an einer NEUEN, vorher verdeckten Stelle). `projects.nummer TEXT NOT NULL`
    (`server/internal/migrate/migrations/007_projects.sql:4`, kein Default) — die Test-Helper
    `createUploadedGAEBImportForProcessingTest` (`server/internal/quotes/imports_test.go:37-47`, genutzt von
    `TestProcessGAEBImportStoresParserResult`, `TestProcessGAEBImportMarksParserFailure`,
    `TestProcessGAEBImportRequiresConfiguredParserWithoutMutation`) sowie eine weitere, separate Stelle
    (`internal/quotes/imports_test.go:129-132`, genutzt von
    `TestQuoteImportParseResultStoresItemsAndUpdatesStatus`) legen Projekte per rohem
    `INSERT INTO projects (id, name, kunde_id, status, company_id) VALUES (...)` an, OHNE `nummer`
    anzugeben — `ERROR: null value in column "nummer" of relation "projects" violates not-null constraint
    (SQLSTATE 23502)`. Betrifft alle vier genannten Tests identisch. Nicht behoben (außerhalb des auf den
    `telefon`-Tippfehler beschränkten Scopes von 0.23). Fix: `nummer` in den betroffenen `INSERT`-Statements
    ergänzen (beliebiger eindeutiger Platzhalterwert genügt, da `nummer` hier nicht fachlich geprüft wird).
- [x] 0.24 `DELETE /sales-orders/{id}/items/{itemID}` liefert 500 statt 400 beim Löschen der letzten Position —
  done, zusammen mit 0.29 und 0.40 in einem Zug behoben (identischer Fix-Mechanismus).
  - Gefunden bei Verifikation von Subtask 0.2.2.1.2.3.1 (`TestQuoteFlowWithPricingAndPDF`, gegen frische DB
    reproduziert, `server/internal/sales/service.go`, von mir nicht angefasst). Die Guard-Regel "Auftrag
    muss mindestens eine Position enthalten" wurde als generischer `errors.New(...)` zurückgegeben, den
    `classifyDomainError` nicht als Validierungsfehler (400) erkannte, sondern als `internal_error` (500)
    einstufte. Fachlich korrekt verhindert, aber falscher HTTP-Status. Unabhängig von `company_id`/`branch_id`.
  - **Umgesetzt zusammen mit Backlog 0.29 und 0.40**: alle drei Funde sind strukturell identisch (eine
    `errors.New(...)`-Meldung in einer Domäne trifft keins der Substring-Muster in `classifyDomainError`,
    `server/internal/http/v1.go`, und fällt auf `500 internal_error` statt `400 validation_error`) — statt
    dreimal denselben mechanischen Fix separat umzusetzen, in EINEM Edit neue Substring-Muster für alle drei
    Fund-Cluster ergänzt: `"mindestens eine position"` (0.24), `"sind zulässig"`/`"können reviewt werden"`/
    `"offene review-positionen"` (0.29), `"hat bereits ein material"`/`"kein sichtbarer suchtreffer"`/
    `"hat kein material"` (0.40).
  - **Ergebnis**: `TestQuoteFlowWithPricingAndPDF` kommt jetzt an der ursprünglich gemeldeten Stelle vorbei
    (0.24 korrekt behoben), scheitert aber SPÄTER an der bereits dokumentierten, unabhängigen Backlog-0.35-
    Ursache (`FOR UPDATE`+`LEFT JOIN`-Fehler in `sales.ConvertToInvoice`) — kein neuer Fund, bereits als
    KRITISCH bekannt.
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Gezielter Testlauf aller 9 betroffenen
    Tests aus 0.24/0.29/0.40 gegen frische, per `\dt` bestätigt leere DB — 8 davon vollständig grün, der
    neunte (`TestQuoteFlowWithPricingAndPDF`) trifft wie erwartet auf 0.35. Vollständiger `go test
    ./internal/http/... -count=1`: von 23 auf 15 Fehlschläge zurückgegangen. Voller `go test ./...`
    (Nicht-Integrationspakete) ohne Fehler.
- [x] 0.25 `POST /quotes/{id}/convert-to-invoice` lässt sich direkt nach Anlage nicht ohne vorherigen
  Status-Übergang testen/nutzen — done, verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.2.3.1 (`TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`,
    reproduzierbar gegen frische, verifiziert leere DB — keine Testreihenfolge-/Datenverschmutzungsursache,
    siehe Backlog 0.20 zur Abgrenzung). `quotes.Service`-Guard (`server/internal/quotes/service.go`, von mir
    nicht angefasst) lässt `convert-to-invoice` nur bei Status `sent`/`accepted` zu; ein direkt angelegtes
    Angebot (`POST /quotes/`) hat den Default-Status `draft` und wurde daher abgelehnt (`nur versendete oder
    angenommene Angebote können in Rechnungen überführt werden`). Der Test rief `convert-to-invoice` ohne
    vorherigen Status-Übergang auf.
  - **Klärung der im Fund offen gelassenen Frage**: die Statuslogik selbst ist fachlich korrekt und bewusst so
    (`quotes.Service.UpdateStatus` erlaubt den Übergang `draft`→`sent` problemlos, kein Hinweis auf eine
    kürzlich verschärfte Regel) — der Test war schlicht unvollständig, kein Verhalten der Anwendung geändert.
  - **Umgesetzt**: `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices` ruft jetzt zuerst
    `POST /quotes/{id}/status` mit `{"status":"sent"}`, bevor `convert-to-invoice` aufgerufen wird — genau der
    reale, vorgesehene Ablauf. Per `grep -n "convert-to-invoice" internal/http/*_test.go` geprüft, ob
    dasselbe Muster (Konvertierung ohne vorherigen Status-Übergang) auch anderswo vorkommt — alle anderen
    Fundstellen sind entweder bereits korrekt (bestehende, nicht in der Fehlerliste stehende Tests) oder
    bewusste Negativtests, die genau diese Ablehnung erwarten (Kommentar-Referenz auf Backlog 0.25 in
    `accounting_integration_test.go:490`).
  - **Ergebnis**: der Test kommt jetzt deutlich weiter (direkte Angebot-Konvertierung, Status-Übergang,
    Auftragsumwandlung laufen alle erfolgreich durch), scheitert aber SPÄTER bei der Umwandlung des daraus
    entstandenen AUFTRAGS in eine Rechnung an der bereits dokumentierten, unabhängigen Backlog-0.35-Ursache
    (`FOR UPDATE`+`LEFT JOIN`-Fehler in `sales.ConvertToInvoice`) — kein neuer Fund, 0.25s eigentliches Ziel
    (die Angebots-Konvertierung) ist nachweislich behoben.
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Gezielter Testlauf gegen frische, per
    `\dt` bestätigt leere DB (Log bestätigt: `POST .../status` → 200, `POST .../convert-to-invoice` → 201 für
    das direkte Angebot). Vollständiger `go test ./internal/http/... -count=1`: weiterhin 15 Fehlschläge
    (derselbe Test bleibt in der Liste, aber jetzt aus dem bereits bekannten 0.35-Grund statt aus dem
    0.25-Grund) — kein Rückschritt, 0.25 selbst korrekt behoben. Voller `go test ./...`
    (Nicht-Integrationspakete) ohne Fehler.
- [x] 0.26 `TestQuoteApplyVisibleMaterialCandidateSetsManualMapping` schlägt fehl: falscher Upload-Pfad im
  Test — done, verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.3.4.1 (gegen frische DB reproduziert, isoliert nachgestellt).
    Der Test (`server/internal/http/quotes_integration_test.go`) sandte den GAEB-Upload an
    `POST /api/v1/quotes/imports`, registriert ist aber ausschließlich `POST /api/v1/quotes/imports/gaeb`
    (`server/internal/http/v1.go`) — Antwort `405 Method Not Allowed` statt der erwarteten `201`. Andere Tests
    im selben File (z. B. `TestQuoteGAEBImportApplyCreatesDraftQuoteFromAcceptedItems`) nutzen korrekt
    `/imports/gaeb` und laufen fehlerfrei durch. Unabhängig von `company_id`/`branch_id` (reiner
    Pfad-Tippfehler im Test, vermutlich aus der vorherigen, nicht von mir stammenden Session, siehe
    `codex.md`).
  - **Umgesetzt**: Pfad im Test auf `/api/v1/quotes/imports/gaeb` korrigiert (per `grep` bestätigt: einzige
    Fundstelle im gesamten Modul).
  - **Ergebnis**: der Test kommt jetzt deutlich weiter (Upload erfolgreich), PANICT aber danach mit exakt
    demselben Absturz wie das bereits dokumentierte Backlog 0.30
    (`quotes.NewService(env.PG, nil)` ohne NumberingService, nil-Pointer-Panic in
    `settings.NumberingService.Next`) — kein neuer Fund, 0.30-Notiz um diese zweite betroffene Testfunktion
    ergänzt. Da ein Panic den gesamten Testbinary-Lauf abbricht, wird dieser Test ab sofort zusätzlich zu
    `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` per `-skip` von vollen Suite-Läufen
    ausgeschlossen, bis 0.30 behoben ist.
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Isolierter Testlauf bestätigt Fortschritt
    bis zum bekannten 0.30-Panic. Vollständiger `go test ./internal/http/... -count=1 -skip
    "TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates|TestQuoteApplyVisibleMaterialCandidateSetsManualMapping"`:
    von 15 auf 14 Fehlschläge zurückgegangen. Voller `go test ./...` (Nicht-Integrationspakete) ohne Fehler.
- [x] 0.27 `TestApprovalRequestDecisionsMutateOnlyRequest` schlägt fehl: Testfixture nutzt nicht existierende
  Spalte `contacts.telefon` — done, als Teil von Backlog 0.23 gelöst (gleicher Tippfehler, im selben Zug
  systematisch per `grep` in allen Testfixtures korrigiert).
  - Gefunden bei Verifikation von Subtask 0.2.2.1.3.4.4 (`server/internal/quotes/approval_decisions_test.go:31`,
    per `git show HEAD` bestätigt bereits im letzten Commit vorhanden, also nicht von dieser Session
    verursacht). Die Testfixture fügte einen Kontakt per direktem `INSERT INTO contacts (..., telefon, ...)`
    ein — die tatsächliche Spalte heißt `phone` (`server/internal/migrate/migrations/003_contacts.sql:9`,
    gleiches Muster wie Backlog 0.23, dort aber in Produktivcode statt einer Testfixture). Fehler:
    `ERROR: column "telefon" of relation "contacts" does not exist (SQLSTATE 42703)`. Dieser Test ist der
    EINZIGE direkte (nicht-HTTP) Integrationstest für den Freigabe-Workflow-Cluster der `quotes`-Domäne
    (`RequestApprovalForQuoteItem`/`ApproveApprovalRequestForQuoteItem`/`RejectApprovalRequestForQuoteItem`)
    und lief vermutlich noch nie erfolgreich gegen eine reale Postgres-Instanz. Unabhängig von
    `company_id`/`branch_id`.
  - Verifiziert (bei Umsetzung von 0.23): `go test ./internal/quotes/... -run
    "^TestApprovalRequestDecisionsMutateOnlyRequest$" -v -count=1` gegen frische DB — PASS.
- [x] 0.28 `TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests` schlägt fehl: Testfixture erzeugt
  `reason_code=negative_margin` statt des erwarteten `below_target_margin` — done, verifiziert gegen frische
  DB.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.3.4.4 (gegen frische, verifiziert leere DB reproduziert;
    Fixture-Aufruf `seedHTTPApprovalDecisionQuote(..., "ANG-HTTP-APPROVAL-REQUEST-QUEUE-OPEN", 50, 60)`
    per `git show HEAD` bereits im letzten Commit vorhanden, ebenso die Reason-Code-Berechnung in
    `RequestApprovalForQuoteItem` — von mir nicht angefasst). Rechnerisch: `unit_price=50 < cost_basis=60`
    ⇒ `absoluteMargin=-10` ⇒ per `server/internal/quotes/service.go` korrekt `"negative_margin"`. Der Test
    (`server/internal/http/quotes_integration_test.go`) erwartete aber `"below_target_margin"`.
  - **Klärung, was fachlich gewollt war**: die Fixture-Preise NICHT geändert, sondern die Testerwartung
    korrigiert — die restlichen Assertions IM SELBEN Test (`TargetDifferenceSnapshot != -22`,
    `MarginPercentSnapshot` ≈ `-16.67%`, `CurrentTargetStatus != "below_cost"`) sind bereits alle
    ausschließlich mit `negative_margin`-Semantik konsistent (rechnerisch exakt zu `unitPrice=50,
    costBasis=60` passend) — nur die EINE `ReasonCode`-Assertion widersprach dem Rest des eigenen Tests.
    `seedHTTPApprovalDecisionQuote(..., 50, 60)` wird zusätzlich an 17 WEITEREN Stellen im selben Testfile
    identisch verwendet (per `grep` bestätigt) — die Fixture-Preise zu ändern hätte diese unnötig riskiert,
    ohne Vorteil.
  - **Beim Beheben zwei weitere, bisher nicht einzeln benannte Fundstellen desselben Kopier-Fehlers
    entdeckt** (per `grep -n "below_target_margin\|negative_margin"` systematisch gesucht, dann für jede
    Fundstelle die Fixture-Preise und ggf. begleitende Snapshot-Assertions geprüft, um jede fälschlich als
    "below_target_margin" erwartete Stelle von der einen ECHTEN `below_target_margin`-Stelle
    (`TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`, Preise 59.9/59.9, rechnerisch
    korrekt) zu unterscheiden): `TestQuoteApprovalDecisionEndpointsRequireApprovePermission` und
    `TestQuoteApprovalReworkQueueEndpointListsOnlyOpenLatestRejections` — beide nutzen ebenfalls
    `seedHTTPApprovalDecisionQuote(..., 50, 60)` und erwarteten ebenfalls fälschlich `"below_target_margin"`,
    beide korrigiert auf `"negative_margin"`.
  - **Nebenbei ein weiterer, bisher unbekannter `classifyDomainError`-Fund behoben** (identisches Muster wie
    0.24/0.29/0.40, direkt beim Verifizieren von `TestQuoteApprovalDecisionEndpointsRequireApprovePermission`
    aufgefallen): die Meldung "Keine aktive Freigabeanforderung vorhanden"
    (`server/internal/quotes/service.go`, zwei Fundstellen) traf keins der bestehenden Substring-Muster und
    fiel auf `500 internal_error` statt `400 validation_error`. Neues Substring-Muster `"keine aktive
    freigabeanforderung"` ergänzt.
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Gezielter Testlauf aller drei
    korrigierten Tests gegen frische, per `\dt` bestätigt leere DB — alle PASS. Vollständiger `go test
    ./internal/http/... -count=1 -skip "...2 bekannte Panic-Tests..."`: von 14 auf 11 Fehlschläge
    zurückgegangen. Voller `go test ./...` (Nicht-Integrationspakete) ohne Fehler.
- [x] 0.29 `classifyDomainError` erkennt mehrere GAEB-Import-Validierungsfehler nicht als 400 — done, zusammen
  mit 0.24 und 0.40 behoben, siehe dortige Notiz für Details.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.3.4.5 (gegen frische, verifiziert leere DB reproduziert;
    `server/internal/http/v1.go`, von mir nicht angefasst außer dem bereits vorhandenen Eintrag für
    "nur hochgeladene importläufe..."). Mindestens drei `errors.New(...)`-Meldungen aus
    `server/internal/quotes/imports.go` matchten keins der Substring-Muster in `classifyDomainError` und
    fielen daher auf den `default`-Zweig (500 `internal_error`) statt 400 `validation_error`:
    "Nur GAEB-Dateien mit den Endungen .x83, .x84, .d83, .p83, .gaeb oder .xml sind zulässig"
    (`TestQuoteGAEBImportRejectsNonGAEBFiles`), "Nur geparste Importläufe können reviewt werden"
    (`TestQuoteGAEBImportItemReviewEndpointRejectsNonParsedImport`), "Importlauf enthält noch offene
    Review-Positionen" (`TestQuoteGAEBImportReviewEndpointRejectsPendingItems`). Gleiches Muster wie Backlog
    0.24, hier aber in der `quotes`-Domäne. Unabhängig von `company_id`/`branch_id`.
  - Verifiziert: alle drei genannten Tests PASS gegen frische DB (siehe Backlog 0.24 für den vollständigen
    Verifikationslauf, der alle drei Backlog-Positionen gemeinsam abdeckt).
- [x] 0.30 `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` paniced: `quotes.NewService(env.PG, nil)`
  ohne NumberingService — done, verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.3.4.5 (gegen frische, verifiziert leere DB reproduziert;
    `server/internal/http/quotes_integration_test.go`, per `git show HEAD` bereits im letzten Commit so
    vorhanden, von mir nicht angefasst). Der Test konstruiert `quoteSvc` mit `nil` als `NumberingService`
    (zweites Argument von `quotes.NewService`). `ApplyImportToDraftQuote` → `createQuoteTx`
    (`server/internal/quotes/service.go`) ruft aber `numSvc.Next(...)` auf, um die Angebotsnummer zu
    vergeben — bei `nil` führt das zu einem Nil-Pointer-Panic in
    `internal/settings.(*NumberingService).Next`, der den gesamten Testprozess abbricht (nicht nur den einen
    Test). Unabhängig von `company_id`/`branch_id`.
  - **Update (Verifikation von Backlog 0.26)**: derselbe Anti-Pattern tritt IDENTISCH auch in
    `TestQuoteApplyVisibleMaterialCandidateSetsManualMapping` auf (eigenes lokales `quoteSvc :=
    quotes.NewService(env.PG, nil)`, separat vom über `NewRouterWithDeps` konstruierten HTTP-`handler` der
    Testfunktion). Betrifft also ZWEI Testfunktionen mit demselben Fix-Bedarf.
  - **Fix**: in `server/internal/http/quotes_integration_test.go` in beiden Testfunktionen
    `quotes.NewService(env.PG, nil)` durch `quotes.NewService(env.PG, settings.NewNumberingService(env.PG))`
    ersetzt (Import `nalaerp3/internal/settings` ergänzt). Die übrigen 5 Vorkommen von
    `quotes.NewService(env.PG, nil)` im selben Testfile (Zeilen um 2574/2738/2906/3115/3213, je in eigenen
    Testfunktionen) bewusst NICHT angefasst — geprüft, dass keine davon `ApplyImportToDraftQuote` (oder eine
    andere `numSvc.Next(...)`-aufrufende Methode) direkt auf dieser lokalen Instanz aufruft, sondern
    stattdessen nur `SaveImportParseResult`/`ListImportItems`/`UpdateImportItemReview`/`MarkImportReviewed`
    (keine Nummernvergabe) bzw. den tatsächlichen Apply-Schritt über den HTTP-`handler` (mit echtem
    `NumberingService`) laufen lassen.
  - Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`, `go vet ./...`, `gofmt -l`
    clean. Beide vormals panicenden Tests einzeln PASS (kein Absturz mehr, `-skip` nicht mehr nötig).
  - **Cascading-Fund beim Verifizieren des vollen Suite-Laufs (ohne `-skip`)**: `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates`
    kommt jetzt (ohne Panic) bis zur eigentlichen Fachassertion durch und schlägt DORT neu fehl — erwartet
    genau einen Material-Kandidaten, bekommt aber zwei (einen aus der eigenen Company, einen aus einer
    FREMDEN Company/Testfunktion mit textuell identischer Bezeichnung "Aluminium Profil 70mm"). Root-Cause
    identifiziert und als eigenständigen, mandantenübergreifenden Datenleck-Bug dokumentiert: Backlog 0.43
    (NICHT in diesem Subtask behoben — andere Codepfad-Ebene als der hier behobene Panic, SQL-Fix mit
    eigenem Verifikationsaufwand nötig).
  - Voller `go test ./internal/http/... -count=1` gegen frische DB (kein `-skip` mehr nötig): 12
    Fehlschläge (11 unveränderte Altfälle + 1 neu sichtbar gewordener Fund, siehe 0.43) — vorher 11 Fehlschläge
    bei `-skip`-Ausschluss beider 0.30-Tests; rechnerisch konsistent (0.30 selbst verursacht keinen
    Netto-Regress, deckt nur einen vorher durch den Panic maskierten Folgefehler auf).
- [x] 0.31 `TestGAEBImportProcessEndpoint` ruft `NewV1RouterWithOptions` direkt auf, testet aber Pfade mit
  `/api/v1`-Präfix — done, verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.2.2.1.3.4.5 (gegen frische, verifiziert leere DB reproduziert;
    `server/internal/http/quotes_integration_test.go:38-94`). Der Test baute den Handler über
    `NewV1RouterWithOptions(...)`, dessen Routen direkt unter `/auth/...` liegen, nicht unter
    `/api/v1/auth/...` — dieses Prefixing passiert erst, wenn `NewV1Router`-Ergebnisse via `NewRouterWithDeps`
    (`server/internal/http/router.go`, `r.Mount("/api/v1", ...)`) gemountet werden. `loginIntegrationUser`
    fordert aber fest `/api/v1/auth/login` an → `404 page not found`.
  - **Fix (Option „NewRouterWithDeps um Options erweitern" aus dem ursprünglichen Fund gewählt)**: in
    `server/internal/http/router.go` `NewRouterWithDeps` zu einem dünnen Wrapper um eine neue Funktion
    `NewRouterWithDepsAndOptions(pg, mg, rd, cfg, options V1RouterOptions) http.Handler` umgebaut, die intern
    `NewV1RouterWithOptions(...)` (statt des bisherigen `NewV1Router(...)`) unter `/api/v1` mountet.
    `NewRouterWithDeps` selbst ruft `NewRouterWithDepsAndOptions(..., V1RouterOptions{GAEBImportParser:
    quotes.GAEBXMLSubsetParser{}})` — bewusst mit demselben Default-Parser wie `NewV1Router`, um KEINE der 12
    anderen Aufrufstellen von `NewRouterWithDeps` (11 weitere Testdateien + `internal/app/server.go`) zu
    beeinflussen (siehe Regressions-Fund unten). Diese Variante wurde der Alternative „Testpfade ohne
    `/api/v1`-Präfix ändern" bewusst vorgezogen: sie testet den Router in genau der Form, wie er auch
    produktiv (`internal/app/server.go`) tatsächlich gemountet wird, statt gegen eine im Produktivbetrieb nie
    auftretende, unpräfixierte Router-Form zu testen.
    In `TestGAEBImportProcessEndpoint` selbst: `handler` jetzt über `NewRouterWithDepsAndOptions(...,
    V1RouterOptions{GAEBImportParser: parser})` statt `NewV1RouterWithOptions(...)` gebaut; die lokale
    `call`-Closure sowie die beiden weiteren Router-Konstruktionen in derselben Testfunktion (`NewV1Router`
    für den Default-Parser-Fall, `NewV1RouterWithOptions(..., V1RouterOptions{})` für den Fehlt-Parser-Fall)
    auf `NewRouterWithDeps(...)` bzw. `NewRouterWithDepsAndOptions(..., V1RouterOptions{})` umgestellt, alle
    Pfade im Test mit `/api/v1`-Präfix versehen.
  - **Sofort selbst gefundener und behobener Regressions-Bug**: die erste Fassung von
    `NewRouterWithDepsAndOptions` (aufgerufen von `NewRouterWithDeps` mit einem LEEREN `V1RouterOptions{}`)
    hätte den `GAEBImportParser` alter alle 12 anderen `NewRouterWithDeps`-Aufrufstellen (11 Testdateien +
    Produktivcode `internal/app/server.go`) auf `nil` gesetzt statt auf den bisherigen Default
    `quotes.GAEBXMLSubsetParser{}` — sofort bei der ersten Testausführung als `500 "GAEB-Parser nicht
    konfiguriert"` bei der eigentlich als Erfolgsfall erwarteten `default parser`-Prüfung DESSELBEN Tests
    aufgefallen, noch bevor der volle Suite-Lauf gestartet wurde. Vor jedem echten Produktivfix behoben (siehe
    oben) — kein Restrisiko für andere Aufrufer.
  - **Nebenbei ein weiterer, bisher unbekannter, mechanisch identischer Fund wie 0.42 im selben Testfile
    behoben** (im exakt selben Testfunktions-Scope, den dieser Subtask ohnehin bearbeitet — kein Scope-Creep):
    `TestGAEBImportProcessEndpoint` legte das Test-Projekt per Direkt-SQL `INSERT INTO projects (id, name,
    kunde_id, status, company_id) VALUES (...)` OHNE das Pflichtfeld `nummer` an (`projects.nummer TEXT NOT
    NULL`, kein Default) — erst durch den Router-Fix sichtbar geworden, da der Test vorher nie so weit kam.
    `nummer` (Platzhalterwert `"PRJ-GAEB-PROCESS-0001"`) ergänzt.
  - Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`, `go vet ./...`, `gofmt -l`
    clean. `TestGAEBImportProcessEndpoint` isoliert PASS. Voller `go test ./...` (Nicht-Integrationspakete)
    ohne Fehler. Voller `go test ./internal/http/... -count=1`: von 12 auf 11 Fehlschläge zurückgegangen (kein
    Regress bei den übrigen 11 Aufrufstellen von `NewRouterWithDeps`, da deren Default-Parser-Verhalten
    unverändert blieb).
- [ ] 0.32 Kein Anwendungscode-Pfad legt einen Kontenrahmen (`accounts`) für einen NEUEN Mandanten an — todo
  - Gefunden bei Verifikation von Subtask 0.2.2.1.5 (`server/internal/settings/accounting.go`,
    `AccountingService.ListAccounts`, jetzt nach `company_id` gefiltert). Migration 058 hat allen
    bestehenden `accounts`-Zeilen `company_id='default'` zugewiesen (Backfill), aber es gibt in der
    gesamten Anwendung KEINEN `Create`/`Insert`-Pfad für `accounts` — der Kontenrahmen wird ausschließlich
    per Migration/Seed-SQL befüllt. Für einen hypothetischen zweiten Mandanten (`company_id != 'default'`)
    würde `ListAccounts` daher eine leere Liste liefern, ohne dass es einen Weg gibt, ihm einen
    Kontenrahmen zuzuweisen. `AccountingService` ist zusätzlich aktuell GAR NICHT an einen HTTP-Handler
    angebunden (`grep -rn "AccountingService" internal/http/` liefert keinen Treffer) — kein akutes
    Sicherheitsproblem, aber ein Funktionslücke, die bei Mandanten-Onboarding relevant wird. Unabhängig
    vom eigentlichen Scoping (das selbst korrekt ist). Nicht behoben (außerhalb Subtask-Scope, würde ein
    Mandanten-Onboarding-Konzept voraussetzen). Fix: entweder Kontenrahmen-Vorlage beim Anlegen eines neuen
    `company_profiles`-Eintrags automatisch kopieren, oder einen expliziten
    "Kontenrahmen aus Vorlage übernehmen"-Endpunkt ergänzen.
- [ ] 0.33 `quotes`-Domäne: Draft-/Versions-Schreibschutz liefert 500 statt 400 (16 Fundstellen) — todo
  - Gefunden bei Recherche zu Subtask 0.3.2 (Festschreibungs-Mechanismus), beim Prüfen, welche Domänen
    bereits einen Schreibschutz für nicht mehr im Entwurf befindliche Belege haben. `server/internal/quotes/service.go`
    enthält an 16 Stellen (`grep -n "nur Entwürfe sind bearbeitbar\|Historische Angebotsversionen sind
    schreibgeschützt"`, u. a. Zeilen 802-805, 3424-3427 in `Update`) exakt dieselben zwei
    `errors.New(...)`-Meldungen ("nur Entwürfe sind bearbeitbar" bzw. "Historische Angebotsversionen sind
    schreibgeschützt") — der eigentliche GoBD-relevante Schreibschutz für versendete/akzeptierte bzw.
    historische (durch `Revise` ersetzte) Angebote GREIFT dabei korrekt. Beide Meldungen matchen aber KEIN
    Substring-Muster in `classifyDomainError` (`server/internal/http/v1.go:3723-3747` — geprüft: weder
    "erforderlich" noch "nicht im status" noch ein anderes vorhandenes Muster passt auf "bearbeitbar" oder
    "schreibgeschützt"), fallen daher auf den `default`-Zweig zurück und liefern `500 internal_error` statt
    `400 validation_error`. Alle 16 Fundstellen bereits vor dieser Session vorhanden (per `git show HEAD`
    bestätigt, nicht in dieser Subtask eingeführt). Gleiches Muster wie Backlog 0.24/0.29. Nicht behoben
    (außerhalb Subtask-Scope, gehört in die `quotes`-Domäne, nicht in die für 0.3.2 tatsächlich bearbeitete
    `purchasing`-Domäne). Fix: `classifyDomainError` um `"bearbeitbar"` und `"schreibgeschützt"` (oder
    präzisere Substrings) ergänzen.
- [ ] 0.34 KRITISCH: `quotes.Service.Revise` scheitert mit "conn busy" bei JEDEM Angebot mit Positionen — todo
  - Gefunden bei Verifikation von Subtask 0.3.3.2 (gegen frische, verifiziert leere DB reproduziert,
    `TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource`, sowohl MIT als auch OHNE meine neue
    `auditlog`-Anbindung identisch reproduziert — also nachweislich unabhängig davon). `server/internal/quotes/service.go:3340`
    (`rows, err := tx.Query(ctx, "SELECT position, description, ... FROM quote_items WHERE quote_id=$1 ...")`,
    per `git show HEAD` bestätigt identisch bereits im letzten Commit vorhanden, nicht von dieser Session
    verursacht) öffnet eine `Rows`-Iteration über die Quell-Positionen; INNERHALB der `for rows.Next()`-Schleife
    wird pro Position `tx.Exec(ctx, "INSERT INTO quote_items ...")` auf DERSELBEN Transaktion/Connection
    ausgeführt, bevor die äußere `rows`-Iteration abgeschlossen ist — ein klassischer pgx-v5-Fehler (eine
    Connection kann laut pgx-Protokoll kein zweites Statement ausführen, während ein vorheriges `Query`-Ergebnis
    noch nicht vollständig gelesen/geschlossen ist). Ergebnis: `ERROR: conn busy`, `POST /api/v1/quotes/{id}/revise`
    liefert `500` für JEDES Angebot mit mindestens einer Position — da `quotes.Create` mindestens eine Position
    zwingend voraussetzt (`"keine Positionen"`-Guard), ist `/revise` de facto für JEDES real angelegte Angebot
    nicht nutzbar. Nicht behoben (außerhalb Subtask-Scope — Subtask 0.3.3.2 war die Anbindung an das
    Änderungsprotokoll, nicht ein Bugfix an der Kopierlogik selbst; zudem als eigenständiges, in sich
    geschlossenes Problem klar von 0.3.3.2 abgrenzbar). Fix: die Positionen zuerst vollständig in einen
    Go-Slice einlesen (`rows.Next()`-Schleife OHNE `tx.Exec(...)` darin, analog zum bereits korrekten Muster
    in `Book`/`Storno` in `accounting/ar.go`, die Items erst vollständig lesen und danach in einer zweiten
    Schleife schreiben), danach in einer separaten Schleife die `INSERT`-Statements ausführen.
- [ ] 0.35 KRITISCH: `sales.Service.ConvertToInvoice` scheitert immer mit Postgres-Fehler "FOR UPDATE cannot be applied to the nullable side of an outer join" — todo
  - Gefunden bei Verifikation von Subtask 0.3.3.3 (gegen frische, verifiziert leere DB reproduziert,
    `TestSalesOrderStatusChangeAndConvertToInvoiceAreAuditLogged`, neu für diese Subtask geschrieben).
    `server/internal/sales/service.go:714-724` (`loadForInvoiceTx`, per `git show HEAD` bestätigt identisch
    bereits im letzten Commit vorhanden — Zeile 719 dort, nicht von dieser Session verursacht) führt
    `SELECT ... FROM sales_orders so LEFT JOIN projects p ON ... LEFT JOIN contacts c ON ... WHERE so.id=$1
    ... FOR UPDATE` aus. Postgres verbietet `FOR UPDATE` auf einer Query mit `LEFT JOIN`, sobald keine
    Zeilensperrung auf die nullable Seite des Joins eingeschränkt wird (`ERROR: FOR UPDATE cannot be applied
    to the nullable side of an outer join`, SQLSTATE 0A000) — jeder Aufruf von `ConvertToInvoice` (und damit
    `POST /api/v1/sales-orders/{id}/convert-to-invoice`) scheitert daher IMMER mit `500`, unabhängig von
    Status/Daten. Damit ist die Umwandlung von Aufträgen in Rechnungen über diesen Pfad aktuell komplett
    unbenutzbar (analog schwerwiegend zu Backlog 0.34). Die neue `entity_change_log`-Anbindung aus Subtask
    0.3.3.3 für `ConvertToInvoice` konnte deshalb NICHT end-to-end über HTTP verifiziert werden (Code-Pfad
    aber strukturell identisch zum erfolgreich verifizierten `UpdateStatus`). Nicht behoben (außerhalb
    Subtask-Scope — 0.3.3.3 war die Anbindung ans Änderungsprotokoll, nicht ein Bugfix an der
    Sperrabfrage). Fix: `FOR UPDATE OF so` ergänzen, um die Zeilensperre explizit auf `sales_orders`
    einzuschränken (analog zum bereits korrekten Muster `FOR UPDATE OF quote_imports` in
    `server/internal/quotes/imports.go`, das genau dieses Problem in einer früheren Subtask dieser Session
    schon einmal gelöst hat).
- [ ] 0.36 Steuerkennzeichen-Fallback-Bug (Backlog 0.7) ist dupliziert in `quotes`/`sales`, dort NICHT
  gegen Stammdaten abgesichert — todo
  - Gefunden bei Umsetzung von Backlog 0.7 (`accounting/ar.go`). Beim Fixen dort per `grep -rn "taxRate("`
    entdeckt: `internal/quotes/service.go` (eigene `taxRate`-Funktion, Zeile ~3548, Aufrufstellen u.a.
    Zeilen 436, 456, 1133, 1238, 3507, 3528, 3543) und `internal/sales/service.go` (eigene `taxRate`-Funktion,
    Zeile ~870, Aufrufstellen u.a. Zeilen 205, 232, 483, 556, 900) enthalten JEWEILS eine eigene, unabhängige
    Kopie desselben hartcodierten `DE19`/`DE7`-Switches mit demselben stillen 0%-Fallback für unbekannte Codes
    — exakt das in 0.7 für `accounting` behobene Muster, hier aber noch nicht. Keine gemeinsame Quelle
    zwischen den drei Paketen (`accounting`/`quotes`/`sales` haben je ihre eigene `taxRate`-Funktion, kein
    Shared-Package). Nicht behoben (außerhalb des auf `accounting/ar.go` beschränkten Scopes von Backlog 0.7,
    eigener, größerer Fund: zwei weitere Domänen, mehr Aufrufstellen als in `ar.go`, potenziell auch andere
    Fallback-Konten). Fix: entweder denselben `loadTaxCodes`-Ansatz in `quotes`/`sales` wiederholen, oder
    (sauberer, vermeidet Drittfachcode) `taxCodeInfo`/`loadTaxCodes`/`taxRate` in ein gemeinsames Package
    (z.B. `internal/settings` oder ein neues `internal/tax`) verschieben und von allen drei Domänen
    referenzieren lassen — letzteres wäre ein größerer Schnitt (Modul-Umbau), erste Bewertung bei
    Bearbeitung nötig.
- [ ] 0.37 `BankService` (Bankabgleich/-matching) ist an KEINEN HTTP-Handler angebunden — todo
  - Gefunden bei Umsetzung von Backlog 0.9. `grep -rln "NewBankService"` über das gesamte `server/`-Modul
    zeigt nur `server/internal/accounting/bank.go` selbst — nirgendwo in `internal/http/v1.go` (oder
    anderswo) wird `accounting.NewBankService(...)` aufgerufen bzw. eine Route dafür registriert. Damit ist
    das komplette, ansonsten fertig implementierte Bankabgleich-Feature (Kontoauszug-Import `Ingest()`,
    manuelles/automatisches Matching `Match()`, Betrags-Heuristik, Referenz-Erkennung) über die API AKTUELL
    UNERREICHBAR — kein Client kann Kontoauszüge importieren oder abgleichen. Nicht behoben (außerhalb des auf
    Tests beschränkten Scopes von 0.9, eigener, nicht-trivialer Fund: braucht mindestens Routen für
    Ingest/List/Match sowie eine Entscheidung zur Berechtigungs-Modellierung, z.B. neue Permissions
    `bank.read`/`bank.write`). Fix: `BankService` in `v1.go` konstruieren (analog zu `paymentSvc`/`arSvc`) und
    `POST /bank-statements`, `GET /bank-statements`, `POST /bank-statements/{id}/match` (oder ähnlich)
    ergänzen.
- [ ] 0.38 `accounting.ARService.createTx` verletzt Fremdschlüssel bei leerem `TaxCode` (leere Position ohne
  Steuerkennzeichen) — todo
  - Gefunden bei Umsetzung von Backlog 0.9 (beim Versuch, eine Testrechnung mit einer steuerfreien Position
    anzulegen — schlug mit `ERROR: insert or update on table "invoice_out_items" violates foreign key
    constraint "invoice_out_items_tax_code_fkey"` fehl). `createTx()` (`server/internal/accounting/ar.go`)
    übergibt `it.TaxCode` (ein Go-`string`, keinen `*string`) direkt als Parameter für die nullable Spalte
    `invoice_out_items.tax_code text REFERENCES tax_codes(code)`. Bei `TaxCode: ""` wird die LEERE
    ZEICHENKETTE eingefügt statt SQL NULL — verletzt den Fremdschlüssel, da kein `tax_codes`-Eintrag mit
    `code=''` existiert. Betrifft JEDE reale Rechnungsposition ohne Steuerkennzeichen (z.B. durchlaufende
    Posten/Skonto), obwohl `calcTotals`/`buildJournal`/`taxRate` (seit Backlog 0.7) einen leeren Code bewusst
    als gültigen "steuerfrei"-Fall behandeln — der DB-Layer widerspricht dem eigentlichen fachlichen Verhalten
    der darüberliegenden Funktionen. Nicht behoben (außerhalb des auf `bank.go` beschränkten Scopes von 0.9,
    liegt in `ar.go`, das zuletzt in Backlog 0.7 bearbeitet wurde). Fix: leeren `TaxCode`/`AccountCode` beim
    Insert auf SQL NULL abbilden (z.B. `nullIfEmpty(it.TaxCode)`-Hilfsfunktion, wie sie an anderer Stelle im
    Repo bereits existiert, z.B. `quotes/service.go`).
- [ ] 0.39 `TestMaterialGroupDeleteRejectsTrimmedLegacyReferences` schlägt auf frischer DB fehl: Testfixture
  referenziert nicht existierende Spalte `materials.updated_at` — todo
  - Gefunden bei Vollverifikation von Backlog 0.13 (voller `go test ./internal/http`-Lauf gegen frische DB,
    erstmals ohne die Backlog-0.20-Kaskade sichtbar — dieser Test kam vorher nie so weit, seine eigene Logik
    tatsächlich auszuführen). `server/internal/http/settings_integration_test.go:762` (Stand dieser Session)
    seedet ein Material per Direkt-SQL mit `ON CONFLICT (id) DO UPDATE SET kategorie = EXCLUDED.kategorie,
    updated_at = now()` — die Spalte `materials.updated_at` existiert laut Schema aber nicht
    (`\d materials` in Postgres bestätigt: keine `updated_at`-Spalte). Per `git show HEAD` bestätigt: exakt
    dieselbe SQL-Zeile war bereits im letzten Commit vorhanden, also nachweislich vorbestehend und nicht durch
    die Migrations-Versionierung aus Backlog 0.13 verursacht — nur vorher durch die 0.20-Kaskade nie
    reproduzierbar zum eigentlichen Fehler durchgedrungen. Isoliert reproduziert (`-run
    "^TestMaterialGroupDeleteRejectsTrimmedLegacyReferences$"` gegen eine eigene frische DB, unabhängig von
    jeder anderen Testfunktion). Nicht behoben (außerhalb des auf den Migrationsrunner beschränkten Scopes von
    0.13). Fix: entweder `updated_at`-Spalte in `materials` ergänzen (falls fachlich gewünscht), oder die
    `updated_at = now()`-Klausel aus dem Test-Fixture-SQL entfernen.
- [ ] 0.43 KRITISCH: `listMaterialCandidatesForQuoteItem` (GAEB-Import-Materialkandidaten) ist NICHT nach
  `company_id` gescoped — Materialien FREMDER Mandanten werden als Kandidaten vorgeschlagen — todo
  - Gefunden bei Verifikation von Backlog 0.30 (nachdem der dortige Nil-Pointer-Panic behoben war, kam
    `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` erstmals bis zur eigentlichen Fachassertion
    durch und schlug DORT neu fehl — vorher durch den Panic maskiert, gegen frische, verifiziert leere DB
    reproduziert). Der Test erwartet genau einen Material-Kandidaten (aus der eigenen Company), bekommt aber
    zwei: `{MaterialNo:MAT-GAEB-0001 ...}` (eigene Company) UND
    `{MaterialNo:MAT-GAEB-CAND-0001 ...}` (aus einer VÖLLIG ANDEREN, in einer anderen Testfunktion
    (`TestQuoteUpdateAllowsManualMaterialMappingOnItems`, eigener Login/eigene Company) angelegten Company,
    per `git show HEAD` bestätigt bereits vorbestehend, von mir nicht angefasst) — beide Materialien tragen
    zufällig dieselbe Bezeichnung `"Aluminium Profil 70mm"`.
  - Root Cause (`server/internal/quotes/service.go`, Funktion `listMaterialCandidatesForQuoteItem`, aktuell
    um Zeile 2968): das SQL-Statement joint `materials m` ausschließlich über
    `LOWER(m.bezeichnung) = LOWER(BTRIM(qii.description)) OR LOWER(m.nummer) = LOWER(BTRIM(qii.description))`
    — OHNE jede Einschränkung auf `m.company_id`. Jede Company, deren importierte Positionsbeschreibung
    textuell mit der Materialbezeichnung/-nummer EINER BELIEBIGEN ANDEREN Company übereinstimmt, bekommt
    deren Material als "Kandidat" vorgeschlagen — ein waschechtes mandantenübergreifendes Datenleck
    (vergleichbare Schwere wie der bereits behobene Backlog 0.22, hier aber ein anderer Codepfad:
    GAEB-Import-Materialkandidaten statt Dokumenten-Download). Die Funktion erhält aktuell nicht einmal einen
    `companyID`-Parameter (Signatur: `listMaterialCandidatesForQuoteItem(ctx, quoteItemID uuid.UUID,
    materialID, candidateStatus string)`) — der Fix erfordert also sowohl eine Signaturänderung als auch das
    Durchreichen von `companyID` an allen Aufrufstellen (aktuell ein Aufrufer, Zeile ~777, selbst innerhalb
    einer Funktion, die `companyID` bereits im Scope hat — sollte unproblematisch durchreichbar sein) und
    eine zusätzliche `AND m.company_id = $N`-Bedingung im SQL.
  - Nicht behoben (außerhalb des auf `quotes.NewService(env.PG, nil)`-Panics beschränkten Scopes von 0.30 —
    andere Code-Ebene, eigener SQL-Fix mit eigener Verifikation nötig). Fix: `companyID string`-Parameter
    ergänzen, an der einzigen Aufrufstelle durchreichen, SQL um `AND m.company_id = $2` erweitern, gegen
    frische DB mit einem gezielten Cross-Tenant-Testfall verifizieren (zwei Companies, identische
    Materialbezeichnung, erwartet: nur die eigene wird als Kandidat vorgeschlagen).
- [x] 0.13 Migrationsrunner unterstützt keine Down-Migrationen — done, siehe
  `docs/adr/0005-migration-versioning-and-down-migrations.md`. Verifiziert gegen frische DB UND gegen
  parallele Testpakete (Race-Condition-Fix).
  - Gefunden bei Verifikation von Subtask 0.2.1.2.1 (`server/internal/migrate/migrate.go:17-33`). `migrate.Run`
    führte jede `.sql`-Datei im aktiven Verzeichnis alphabetisch sortiert vorwärts aus — es gab kein
    Versions-Tracking (keine `schema_migrations`-Tabelle), keine Down-Skripte, keinen Rollback-Mechanismus.
    Damit war aufgabe.md §7.8 ("Keine Migration ohne Down-Pfad") nur per Kommentar im Migrationsfile
    erfüllbar (manuelles Rollback-SQL als Dokumentation, siehe `054_contacts_company_branch_scope.sql`),
    nicht durch automatisiertes Tooling. Direkte Konsequenz des fehlenden Versions-Trackings: Backlog 0.20
    (jede Migration lief bei jedem `Run()`-Aufruf erneut, was den Check-Constraint-Konflikt in 050/051
    auslöste).
  - **Entscheidung (ADR 0005)**: Versions-Tracking einführen (neue Tabelle `schema_migrations`, jede Datei
    läuft nur noch einmal), Down-Migrationen bleiben bewusst manuell/dokumentiert statt automatisiert
    (Kommentarblock-Konvention, bereits gelebte Praxis, jetzt als verbindlicher Standard festgehalten). Kein
    Wechsel auf ein externes Migrationstool, keine rückwirkenden Down-Kommentare für die bestehenden 001-064.
  - **Umgesetzt**: `server/internal/migrate/migrate.go` — `schema_migrations (filename PRIMARY KEY,
    applied_at)`, idempotent angelegt zu Beginn jedes `Run()` (kein separates Migrationsfile, vermeidet das
    Henne-Ei-Problem). Bereits angewendete Dateien werden uebersprungen; jede neu ausgeführte Migration läuft
    atomar zusammen mit ihrem `schema_migrations`-Eintrag in EINER Transaktion (kein Zwischenzustand
    "angewendet, aber nicht markiert" oder umgekehrt).
  - **Kritischer Fund WÄHREND der Verifikation dieser Subtask selbst, sofort behoben**: ein erster Testlauf
    mehrerer Go-Pakete gleichzeitig (`go test ./internal/accounting/... ./internal/hr/... ...` — Standard-
    Parallelverhalten von `go test` über mehrere Pakete) deckte eine Race Condition auf: mehrere Prozesse
    riefen `testutil.SetupIntegrationEnv` (→ `migrate.Run`) gleichzeitig gegen dieselbe Test-DB auf, lasen
    denselben "noch nicht angewendet"-Zustand, und versuchten dieselbe Migration parallel auszuführen —
    `duplicate key value violates unique constraint "schema_migrations_pkey"` bzw. Konflikte direkt in der
    Migrations-DDL (z. B. doppelt angelegte Typen). Fix: `pg_advisory_lock`/`pg_advisory_unlock` um den
    gesamten `Run()`-Ablauf, gehalten über eine explizit aus dem Pool bezogene Einzelverbindung (Advisory
    Locks sind session-, nicht transaktionsgebunden — ein `*pgxpool.Pool` reicht dafür nicht, da er
    Verbindungen zwischen Aufrufen rotieren kann). Serialisiert nebenläufige `Run()`-Aufrufe: ein zweiter
    Prozess wartet, bis der erste fertig ist, und überspringt danach alles. Nach dem Fix mit `-p 8` und `-p
    16` (erhöhte Parallelität über 8-9 Pakete gleichzeitig) mehrfach reproduzierbar OHNE einen einzigen
    `schema_migrations`- oder DDL-Konflikt verifiziert.
  - Tests: neue Datei `server/internal/migrate/migrate_test.go` (externes Testpaket `migrate_test`, um einen
    Importzyklus mit `testutil` zu vermeiden — `testutil.SetupIntegrationEnv` importiert `migrate`).
    `TestRunTracksEveryMigrationFile` (Zeilenanzahl in `schema_migrations` entspricht exakt der Anzahl
    `.sql`-Dateien, dynamisch ermittelt statt hartcodiert), `TestRunIsIdempotentOnRepeatedInvocation` (zwei
    weitere `Run()`-Aufrufe gegen dieselbe, bereits migrierte DB verändern die Zeilenanzahl nicht — würde die
    Skip-Logik versagen, bräche der zweite `INSERT` mit einem echten Fehler ab statt still doppelt zu zählen).
  - **Nebeneffekt**: Backlog 0.20 (kritisch) dadurch vollständig gelöst, siehe dort. Neuer, unabhängiger Fund
    bei der Vollverifikation dokumentiert als Backlog 0.39.
  - Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean (Datei dabei vollständig gofmt-konform
    geworden, war zuvor Teil von Backlog 0.15). `go test ./internal/migrate/... -v -count=1` gegen frische DB
    (beide Tests PASS). `go test ./internal/accounting/... ./internal/hr/... ./internal/auth/...
    ./internal/purchasing/... ./internal/config/... ./internal/migrate/... ./internal/quotes/...
    ./internal/sales/... ./internal/contacts/... -count=1 -p 16` (hohe Parallelität, genau das Szenario, das
    die Race Condition ursprünglich aufdeckte) — keine schema_migrations-/DDL-Konflikte mehr, verbleibende
    Fehlschläge ausschließlich die bereits bekannten `contacts.telefon`-Funde (0.23/0.27). Vollständiger `go
    test ./internal/http/... -count=1` gegen frische DB: von ~70/75 auf 27 Fehlschläge zurückgegangen, alle
    einzeln auf bereits dokumentierte Ursachen zurückgeführt (Details bei Backlog 0.20).
- [x] 0.10 `EmployeeService.Update()` soll unbekannte Patch-Keys ablehnen statt still zu ignorieren — done,
  verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.1.3.2 (`server/internal/hr/service.go:121-150`). Der `switch`
    über die Patch-Keys übernahm nur eine feste Whitelist (`first_name`, `last_name`, `email`, `phone`,
    `role`, `location`, `cost_center`, `active`, `team_id`) in die SQL-`SET`-Klausel; jeder andere Key
    (z. B. Tippfehler wie `activ` statt `active`) fiel durch den `switch` und wurde stillschweigend
    verworfen — der Aufruf kehrte ohne Fehler zurück, obwohl nichts geändert wurde.
  - **Umgesetzt**: neue package-level `employeeUpdatableFields`-Whitelist (`map[string]bool`); `Update()`
    validiert jetzt JEDEN Patch-Key VOR jedem DB-Zugriff gegen diese Whitelist und lehnt bei mindestens einem
    unbekannten Key den GESAMTEN Patch ab (`fmt.Errorf("unbekanntes Feld: %s", k)`, alles-oder-nichts, kein
    teilweises Anwenden der bekannten Felder). Die drei zuvor identischen `switch`-Case-Zweige (die
    ausschließlich `first_name`/... vs. `active` vs. `team_id` künstlich getrennt hatten, obwohl sie exakt
    denselben Code ausführten) zu einer einzigen Schleife vereinfacht — funktional unverändert, aber weniger
    Duplikation. Der jetzt unerreichbare `if len(sets) == 0 { return nil }`-Nachlauf-Check entfernt (nach der
    Vorab-Validierung ist `sets` bei nicht-leerem Patch garantiert nicht-leer).
  - `grep -rn "patch map\[string\]any"` über das gesamte `server/`-Modul bestätigt: dieses Muster kommt
    NIRGENDS sonst vor — die im Backlog-Eintrag angeregte Prüfung, ob andere `Update`-Handler dasselbe Problem
    haben, ergab: nein, isolierter Einzelfall.
  - Tests: bestehender `TestUpdateWithOnlyUnknownKeysIsSilentNoOp` in `TestUpdateRejectsUnknownKeys`
    umbenannt/umgekehrt (erwartet jetzt einen Fehler statt eines stillen No-Op), neuer Test
    `TestUpdateRejectsPatchWithAnySingleUnknownKey` (ein gemischter Patch mit einem bekannten UND einem
    unbekannten Key wird komplett abgelehnt). Neuer Integrationstest
    `TestEmployeeUpdateAppliesKnownFieldsAndRejectsUnknownKey`
    (`server/internal/hr/service_scoping_integration_test.go`, service-level gegen echtes Postgres, da
    `hr` an keinen HTTP-Handler angebunden ist) beweist end-to-end: ein Patch mit ausschließlich bekannten
    Feldern wird korrekt angewendet (per `Get()` nachgeprüft), UND ein anschließender Patch mit einem
    unbekannten Key wird abgelehnt, OHNE dass das im selben Patch enthaltene bekannte Feld (`first_name`)
    trotzdem geändert wurde — beweist das Alles-oder-nichts-Verhalten nicht nur an der Validierungsschicht,
    sondern auch am tatsächlichen DB-Zustand.
    Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test ./internal/hr/... -v -count=1`
    gegen frische, per `\dt` bestätigt leere DB — alle Tests PASS. Vollständiger `go test ./...` über alle
    Nicht-Integrationspakete ohne Fehler.
- [x] 0.11 `LeaveService.Create()`: Tage-Berechnung kann bei vertauschten Daten ≤ 0 ergeben — done,
  verifiziert gegen frische DB.
  - Gefunden bei Verifikation von Subtask 0.1.3.3 (`server/internal/hr/service.go:177-182`). Wenn `Days`
    nicht explizit gesetzt war (`<= 0`), wurde es aus `EndDate.Sub(StartDate).Hours()/24 + 1` berechnet —
    ohne vorherige Prüfung, dass `EndDate` nach `StartDate` liegt. Bei vertauschten Daten (z. B. `StartDate`
    = 2026-08-20, `EndDate` = 2026-08-18) ergab die Formel `-1` und wurde ungeprüft in die DB geschrieben
    (`hr_leave_requests.days`).
  - **Umgesetzt**: neue Prüfung `if lr.EndDate.Before(lr.StartDate) { return nil, errors.New("Enddatum darf
    nicht vor Startdatum liegen") }`, platziert direkt nach der bestehenden `StartDate.IsZero() ||
    EndDate.IsZero()`-Prüfung und damit VOR dem `employeeOwned`-DB-Zugriff — dadurch ist `Create()` für genau
    dieses Szenario jetzt (anders als vorher) direkt mit `NewLeaveService(nil)` testbar, ohne dass ein
    DB-Zugriff nötig wäre. Ein eintägiger Antrag (`StartDate == EndDate`) bleibt bewusst gültig
    (`EndDate.Before(StartDate)` liefert dafür `false`).
  - Tests: bestehender `TestLeaveCreateDaysFormulaCanProduceNonPositiveDaysForInvertedDateRange` (reproduzierte
    den Fund bisher nur über reine Zeitarithmetik, da `Create()` selbst mit nil-Pool an dieser Stelle noch
    gepanict hätte) ersetzt durch `TestLeaveCreateRejectsEndDateBeforeStartDate`, das jetzt direkt `Create()`
    aufruft und die neue Fehlermeldung prüft. Neuer Integrationstest
    `TestLeaveCreateComputesDaysForValidDateRange`
    (`server/internal/hr/service_scoping_integration_test.go`, service-level gegen echtes Postgres) beweist,
    dass die neue Prüfung den Normalfall NICHT versehentlich mitblockiert: ein regulärer 5-Tage-Zeitraum
    liefert weiterhin korrekt `Days=5`, ein eintägiger Antrag (`StartDate==EndDate`) liefert korrekt `Days=1`
    — reiner Unit-Test der Validierungsschicht allein hätte eine Regression im Erfolgspfad nicht aufgedeckt.
    Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test ./internal/hr/... -v -count=1`
    gegen frische, per `\dt` bestätigt leere DB — alle Tests PASS. Vollständiger `go test ./...` über alle
    Nicht-Integrationspakete ohne Fehler.
- [x] 0.12 `LeaveService.Create()`: keine Überschneidungsprüfung für Urlaubsanträge — done, verifiziert gegen
  frische DB.
  - Bereits in `docs/00-recon.md`/`docs/01-gap-analysis.md` (Domäne F) als Lücke vermerkt, bei Subtask
    0.1.3.3 bestätigt: `Create()` fragte vor dem Insert keine bereits bestehenden `hr_leave_requests` desselben
    Mitarbeiters ab — zwei sich überschneidende Anträge (auch mehrfach genehmigte) waren möglich. Da die
    Prüfung eine Datenbankabfrage erfordert, war sie ohne DB-Mock nicht unit-, sondern nur integrationstestbar
    — in Subtask 0.1.3.3 bewusst nicht nachgebaut (wäre Scope-Creep gewesen: Implementierung einer fehlenden
    fachlichen Regel statt nur Tests für vorhandene Regeln).
  - **Umgesetzt**: neue Prüfung direkt nach dem `employeeOwned`-Check (Mitarbeiter muss ohnehin schon
    existieren, bevor eine Überschneidung mit SEINEN Anträgen sinnvoll geprüft werden kann) und vor dem
    Insert: `SELECT EXISTS(SELECT 1 FROM hr_leave_requests WHERE employee_id=$1 AND status IN
    ('pending','approved') AND start_date <= $2 AND end_date >= $3)` (Standard-Intervall-Überlappungstest).
    Bewusst NUR `pending`/`approved` — ein bereits `rejected`er Antrag blockiert den Zeitraum nicht dauerhaft
    (fachlich richtig: eine Ablehnung soll spätere, neue Anträge für denselben Zeitraum nicht verhindern).
  - Tests (nur integrationstestbar, wie im Backlog-Eintrag selbst schon vermerkt): neuer
    `TestLeaveCreateRejectsOverlappingDateRange` (`server/internal/hr/service_scoping_integration_test.go`)
    deckt alle vier fachlich relevanten Fälle ab: (1) Überschneidung mit einem `pending` Antrag wird
    abgelehnt, (2) ein direkt ANSCHLIESSENDER, nicht überlappender Zeitraum bleibt erlaubt (Grenzfall der
    Intervall-Arithmetik), (3) ein `approved` Antrag blockiert Überschneidungen genauso wie `pending`
    (entspricht der im Backlog-Eintrag explizit genannten Sorge "auch mehrfach genehmigte"), (4) ein
    `rejected` Antrag blockiert NICHT mehr — derselbe Zeitraum kann danach erneut beantragt werden.
    Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test ./internal/hr/... -v -count=1`
    gegen frische, per `\dt` bestätigt leere DB — alle Tests PASS. Vollständiger `go test ./...` über alle
    Nicht-Integrationspakete ohne Fehler.
  - **Damit sind alle Epic-0-Funde aus dem numerischen Bereich 0.6-0.12 (die urspünglich bei den
    Test-Absicherungs-Subtasks 0.1.1.x-0.1.3.x gefundenen Mängel) vollständig abgearbeitet.** Verbleibend im
    numerisch niedrigeren Bereich: 0.13 (Migrationsrunner ohne Down-Migrationen), 0.14 (KRITISCH, Status
    vermutlich veraltet), 0.15 (gofmt), danach die höher nummerierten Funde 0.20-0.38.

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
