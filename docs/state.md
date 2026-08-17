# State

> Bei Sessionstart zuerst diese Datei und `docs/backlog.md` lesen, danach
> ausschließlich danach richten — nicht nach Chat-Erinnerung.

## Aktueller Pfad

**Subtask 0.3.3.4 (Anbindung `purchase_orders` an das Änderungsprotokoll)
abgeschlossen — damit sind Task 0.3.3, UND DAMIT DAS GESAMTE EPIC 0.3
(GoBD-Fundament: Storno, Festschreibung, Änderungsprotokoll) VOLLSTÄNDIG
ABGESCHLOSSEN.** 0.3.3.5 (Anbindung `contacts`/`projects`/`materials`/`hr`)
wurde bewusst NICHT umgesetzt, sondern als "geprüft und zurückgestellt"
abgeschlossen: die vier bereits angebundenen Domänen
(`invoices_out`/`quotes`/`sales_orders`/`purchase_orders`) decken die
komplette GoBD-primär-relevante kommerzielle Belegkette ab
(Angebot→Auftrag→Bestellung→Rechnung inkl. Storno); Stammdaten-Domänen wie
`contacts`/`materials`/`hr` haben keinen direkten GoBD-Bezug (keine
Buchungs-/Rechnungsdokumente) — eine Protokollierung dort wäre ein
allgemeines Audit-Feature, kein GoBD-Fundament-Bedarf, und würde Epic 0.3
unnötig aufblähen.

**Subtask 0.4.2 (ADR: Umgang mit verwaistem `server/migrations/`-Verzeichnis)
abgeschlossen — damit ist Task 0.4 (Migrationsverzeichnis bereinigen)
VOLLSTÄNDIG ABGESCHLOSSEN.** Siehe `docs/adr/0004-orphaned-migrations-directory.md`.
`server/migrations/` (9 Dateien, byte-identisch mit bereits aktiven
Dateien) wurde per `git rm -r` entfernt (uncommitted, nur gestaged).

**Subtask 0.5.2 (Rate-Limiting auf `/auth/login`) abgeschlossen**,
verifiziert gegen frische DB. Kontext: `auth.Service.Login` hatte zuvor
KEINERLEI Schutz gegen Brute-Force/Credential-Stuffing — `user.IsLocked`
(Backlog-Recherche 0.5.1→0.5.2) ist nur ein manuell gesetztes Flag, kein
automatischer Fehlversuchs-Zähler. Neue, eigenständige `auth.LoginRateLimiter`
(`server/internal/auth/ratelimit.go`) nutzt den bereits vorhandenen
Redis-Client (denselben, den `auth.SessionStore` verwendet — keine neue
Abhängigkeit) als Fixed-Window-Zähler PRO IP-ADRESSE (Port via
`net.SplitHostPort` abgetrennt). Bewusste Design-Entscheidung: **nur
FEHLGESCHLAGENE Versuche zählen** (`RegisterFailure`), erfolgreiche
Logins setzen den Zähler zurück (`Reset`) — kein Zählen aller Requests.
Grund: `httptest.NewRequest` vergibt ohne explizites `RemoteAddr` für
JEDEN Request dieselbe Default-IP (`192.0.2.1`), und `loginIntegrationUser`
(über ~40 Aufrufstellen quer durchs `internal/http`-Testpaket) nutzt genau
diese Default-IP für erfolgreiche Logins — ein "alle Versuche zählen"-Design
hätte bei einem vollen Testlauf ausnahmslos JEDEN Integrationstest, der
sich einloggt, geblockt. Mit "nur Fehlversuche zählen + Reset bei Erfolg"
bleiben alle bestehenden (immer korrekten) Test-Logins unberührt. Neue
`Service.WithRateLimiter(*LoginRateLimiter)`-Fluent-Setter (analog zu
`WithAudit` etc. — optional, nil-sicher, kein bestehender Call-Site-Bruch).
`config.Config` bekam `LoginRateLimitMaxAttempts` (Default 10, env
`LOGIN_RATE_LIMIT_MAX_ATTEMPTS`) und `LoginRateLimitWindowSeconds` (Default
60, env `LOGIN_RATE_LIMIT_WINDOW_SECONDS`). Handler in `v1.go` mappt neues
`auth.ErrRateLimited` auf HTTP 429 (`rate_limited`). Verifiziert: (1) reine
Unit-Tests ohne Redis (`server/internal/auth/ratelimit_test.go` — Port-
Stripping, Nil-Sicherheit, deaktiviert bei `maxAttempts<=0`); (2) echter
Integrationstest `TestAuthLoginRateLimitedAfterRepeatedFailures`
(`server/internal/http/auth_integration_test.go`) gegen frische DB/Redis
mit eigener, isolierter IP (`198.51.100.42`, NICHT die geteilte Default-IP)
und abgesenkter Schwelle (3 statt 10) — 3× falsches Passwort → je 401,
4. Versuch → 429 `rate_limited`, danach auch KORREKTES Passwort → weiterhin
429 (Sperre gilt pro IP, nicht pro Ausgang). `go build ./...`, `go vet
./...`, `go test ./internal/auth/... ./internal/config/...` (alle grün),
sowie gezielter `go test ./internal/http/... -run TestAuthLogin` gegen
frische DB (beide Tests PASS) — bewusst NICHT der volle
`internal/http`-Lauf als Nachweis verwendet, siehe nächster Absatz.

**Subtask 0.5.3 (`users.manage`-Bypass in `requirePermission()`) abgeschlossen**
— ersetzt statt nur dokumentiert (Epic 0.5 ist Auth-HÄRTUNG, ein Kommentar
hätte die stille Privilegien-Eskalationsfalle nicht entschärft). Neue,
dedizierte `admin.superuser`-Permission (Migration
`064_admin_superuser_permission.sql`, nur an `role-admin` vergeben) ersetzt
in BEIDEN Fundstellen (`requirePermission()`-Middleware UND dem inline
duplizierten Bypass im Quote-Accept-Handler für den optionalen
Projektstatus-Wechsel) den bisherigen `p == "users.manage"`-Univeral-Bypass.
`users.manage` bleibt ein enges, korrekt benanntes "Benutzer/Rollen
verwalten"-Recht — relevant für die in 0.5.4 geplante User-Management-API,
die sonst eine neue, gezielt beschränkte Rolle ungewollt zum Systemadmin
gemacht hätte. Kein Verhaltensbruch (nur `admin` hatte `users.manage`, und
`admin` besaß ohnehin schon jede Einzelberechtigung direkt). Neuer
Integrationstest `TestUsersManagePermissionNoLongerBypassesOtherPermissionChecks`
verifiziert gegen frische DB (403 statt vorherigem 201-Bypass). Details
siehe `docs/backlog.md` Subtask 0.5.3.

**Task 0.5.4 (User-Management-API) begonnen — vor der Umsetzung in drei
Mikro-Subtasks zerlegt** (geschätzter Gesamtumfang lag klar über der
~400-Zeilen-Schwelle; der Backlog-Titel selbst legt die Dreiteilung schon
nahe): **0.5.4.1 Anlegen** (done, diese Runde), **0.5.4.2
Sperren/Entsperren** (offen), **0.5.4.3 Rollenzuweisung** (offen).

**Subtask 0.5.4.1 (`POST /users` — Anlegen) abgeschlossen**, verifiziert
gegen frische DB. Vorher gab es ÜBERHAUPT keinen HTTP-Weg für
Benutzeranlage, nur Direkt-SQL (`testutil.SeedAuthUser`, testonly, plus
manuelle Admin-Eingriffe). Neues `auth.UserCreate`-DTO bewusst OHNE
Rollenzuweisung (das ist 0.5.4.3) — ein neuer Nutzer hat zunächst keine
Rollen. `Repository.CreateUser`/`EmailExists`/`UsernameExists` (Duplikat-
Vorabprüfung nach dem `contacts.ensureNoDuplicate`-Muster, damit die
Fehlermeldung "bereits vorhanden" `classifyDomainError`s bestehendes Muster
trifft, ohne dort etwas ändern zu müssen). `Service.CreateUser` validiert
E-Mail/Mandant/Passwort (letzteres über bestehendes `HashPassword`), setzt
Defaults (Locale/Timezone/DisplayName), `is_active=true`/`is_locked=false`.
Neue Route `POST /users/` mit `requirePermission("users.manage")` — die aus
0.5.3 nun korrekt eng skopierte Permission wird hier erstmals für ihren
eigentlichen Zweck verwendet (schöner Beleg, dass 0.5.3 sinnvoll war).
**End-zu-Ende-Verifikation, nicht nur Statuscode-Prüfung**: der
Integrationstest loggt sich nach der Anlage tatsächlich mit dem soeben
gesetzten Passwort ein (beweist korrektes Hashing über die volle Kette),
plus Duplikat-E-Mail→400 und fehlende `users.manage`-Permission→403 (letzteres
als Regressionsbeleg nach 0.5.3). DB-lose Validierungstests für die drei
frühen Fehlerpfade (ungültige E-Mail, fehlender Mandant, leeres Passwort —
alle vor dem ersten Repo-Zugriff, daher mit `repo=nil` testbar).
`go build ./...`/`go vet ./...`/`gofmt -l` clean, gezielter `go test
./internal/http/... -run "TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
-count=1` gegen frische DB: alle 6 Tests PASS.

**Subtask 0.5.4.2 (`POST /users/{id}/lock` + `/unlock` — Sperren/Entsperren)
abgeschlossen**, verifiziert gegen frische DB. Neue Aktions-Endpunkte
(analog zu `.../accept`/`.../revise`/`.../storno`, kein rohes PATCH).
`Repository.SetUserLocked` Mandanten-scoped über die UPDATE-WHERE-Klausel —
ein Sperr-Versuch gegen einen Benutzer eines ANDEREN Mandanten liefert
`404` statt die Existenz des fremden Kontos zu verraten. **Wichtiger
Beweis**: die Sperre wirkt nicht nur gegen neue Logins, sondern auch gegen
ein BEREITS ausgestelltes Access-Token — `AuthenticateAccessToken` liest
`user.IsLocked` bei JEDEM Request frisch aus der DB, nicht nur beim Login.
Integrationstest zeigt den vollen Zyklus end-to-end: Zugriff vor Sperre OK
→ Sperren → bestehendes Token 401 + neuer Login 401 → Entsperren → Login
wieder OK. Bewusst NICHT implementiert: Selbstsperr-Schutz (außerhalb des
engen Task-Titels, eigenständige Design-Entscheidung, bei Bedarf separates
Backlog-Item). `go test ./internal/http/... -run
"TestUsersLock|TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
-count=1` gegen frische DB: alle 9 Tests PASS. Details siehe
`docs/backlog.md` Subtask 0.5.4.2.

**Subtask 0.5.4.3 (Rollenzuweisung) abgeschlossen — damit ist Task 0.5.4
(User-Management-API) UND DAMIT DAS GESAMTE EPIC 0.5 (Auth-Härtung)
VOLLSTÄNDIG ABGESCHLOSSEN.** Neue Endpunkte `GET /users/roles` (listet alle
Rollen) und `PUT /users/{id}/roles` (ersetzt die komplette Rollenmenge
eines Nutzers, bewusst PUT statt POST). `Repository.ReplaceUserRoles`:
Mandanten-Scoping wie bei `SetUserLocked` (404 statt Existenz-Leak),
Alles-oder-nichts-Validierung aller Rollen-Codes vor jeder Änderung,
atomares Delete+Insert in einer Transaktion, `assigned_by` wird protokolliert.
Zentraler Integrationstest beweist per ECHTEM Login vor/nach der
Umzuweisung, dass die alte Rolle wirklich weg und die neue wirklich wirksam
ist — nicht nur, dass die PUT-Antwort das behauptet. Ein Tippfehler
("ungueltiger" ohne Umlaut traf `classifyDomainError`s Muster "ungültig"
nicht, führte zu 500 statt 400) wurde beim ersten Testlauf gefunden und
sofort korrigiert. `go test ./internal/http/... -run
"TestUsersRoles|TestUsersLock|TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
-count=1` gegen frische DB: alle 14 Tests PASS. Details siehe
`docs/backlog.md` Subtask 0.5.4.3.

**Backlog 0.6 (Pre-existing Testfehler: `purchasing.TestCreateRejectsInvalidItem`
panict) behoben.** Mit Epic 0.5 vollständig abgeschlossen bestand die
Restarbeit in Epic 0 nur noch aus einer flachen Liste unabhängiger,
chronologisch nummerierter Funde (0.6-0.35, keine strikte Prioritätsordnung
außer den explizit als KRITISCH markierten). Gewählte Fortsetzung:
niedrigste offene Nummer zuerst (0.6), da klein, bereits vollständig
diagnostiziert (Ursache/Fix stand schon im Backlog-Eintrag) und risikoarm.
`purchasing.Service.Create` rief `s.pg.Begin(ctx)` VOR der Item-Validierung
auf — mit `NewService(nil)` (wie im Test) ein nil-Pointer-Panic statt des
erwarteten Validierungsfehlers. Fix: Item-Validierung als eigene Schleife
vor `s.pg.Begin(ctx)` gezogen (analog zu den bereits davor stehenden
Mandant-/Lieferant-/Status-Prüfungen), die jetzt redundante Prüfung
innerhalb der bestehenden Insert-Schleife entfernt. Kein
Produktivverhaltensunterschied. Verifiziert: `go test
./internal/purchasing/... -v -count=1` (alle 7 PASS, kein Panic mehr),
vollständiger `go test ./...` (keine Panics im gesamten Modul mehr), sowie
ein Regressionscheck gegen frische DB (`go test ./internal/http/... -run
"TestPurchaseOrders" -count=1`) — 3/4 PASS, der vierte
(`TestPurchaseOrdersCreateAndGetFlow`) schlägt weiterhin fehl, aber
NACHWEISLICH (per isoliertem Einzellauf bestätigt) aus dem bereits
dokumentierten, unabhängigen Grund Backlog 0.21 (leere `material_groups`
auf frischer DB), nicht aus dieser Änderung.

**Backlog 0.7 (Steuerkennzeichen-Validierung in `accounting` gegen
Stammdaten absichern) behoben.** `taxRate()`/`taxAccountFor()` in
`server/internal/accounting/ar.go` kannten zuvor hartcodiert nur
`DE19`/`DE7` und fielen bei jedem anderen Code STILLSCHWEIGEND auf 0%
Steuer bzw. das DE19-Konto `1776` zurück — die bereits vorhandene
`tax_codes`-Tabelle wurde nie konsultiert. Neue `loadTaxCodes(ctx, tx)`
liest alle aktiven Steuerkennzeichen samt USt-Verbindlichkeitskonto
(`accounts.type='liability'`) aus den Stammdaten in eine
`map[string]taxCodeInfo` — einzige Quelle der Wahrheit. `taxRate`/
`taxAccountFor`/`calcTotals`/`buildJournal`/`buildStornoJournal` bekamen
diese Map als Parameter plus `error`-Rückgabe (leerer Code bleibt 0% ohne
Fehler — legitime steuerfreie Position; unbekannter/inaktiver Code oder
fehlendes Konto liefert jetzt einen Fehler statt still zurückzufallen).
Bleiben dabei weiterhin pure/synchron testbare Funktionen, die DB-
Konsultation passiert zentral einmal pro Transaktion. Alle 3 Aufrufstellen
(`createTx`, `Book`, `Storno`) angepasst. Tests umgeschrieben (erwarten
jetzt Fehler statt stillen Fallback), ein neuer Test ergänzt. Verifiziert:
`go test ./internal/accounting/...` (20/20 PASS), voller `go test ./...`
sauber, sowie `go test ./internal/http/... -run "TestInvoiceOut" -count=1`
gegen frische DB (beide End-to-End-Tests PASS — beweisen die volle Kette
Anlegen→Buchen→Storno mit echten, aus Postgres gelesenen
Steuerkennzeichen).

**Neuer Fund bei der Umsetzung, bewusst nicht mitgefixt**: dasselbe
hartcodierte `taxRate`-Fallback-Muster existiert unabhängig dupliziert in
`internal/quotes/service.go` und `internal/sales/service.go` (eigene
Kopien, kein Shared-Package) — als neues Backlog 0.36 dokumentiert statt
in dieser Subtask miterledigt (der ursprüngliche 0.7-Fund war explizit auf
`accounting/ar.go` beschränkt; 0.36 ist größer: zwei weitere Domänen, mehr
Aufrufstellen, ggf. ein gemeinsames Package als sauberere Lösung).

**Backlog 0.8 (Integrationstests für die zentralen Zahlungs-Guards in
`payments.go`) ergänzt.** `PaymentService.apply()` prüft drei fachlich
zentrale Regeln (Statusguard "Rechnung ist nicht gebucht", Währungsabgleich
"Währung stimmt nicht mit Rechnung überein", Überzahlungsschutz "Zahlung
übersteigt offenen Betrag") erst NACH dem `tx.QueryRow`-Laden der Rechnung
— ohne echte DB-Transaktion nicht unit-testbar, und bisher komplett
ungetestet (die einzige bestehende Integrationstest deckte nur den
Happy-Path ab). Neuer Test `TestInvoiceOutPaymentGuardsRejectInvalidPayments`
deckt alle drei Regeln mit exakter Fehlermeldungsprüfung ab (nicht nur
Statuscode), plus Regressionscheck, dass eine gültige Zahlung trotzdem noch
funktioniert. Alle drei Meldungen waren bereits korrekt in
`classifyDomainError` auf 400 gemappt — reine Testergänzung, keine
Code-Änderung nötig. `go test ./internal/http/... -run "TestInvoiceOut"
-count=1` gegen frische DB: alle 3 Tests PASS.

**Backlog 0.9 (Integrationstests für Bankabgleich/-matching, `bank.go`)
ergänzt.** Wichtige Vorab-Erkenntnis: `BankService` ist an KEINEN
HTTP-Handler angebunden (`grep -rln "NewBankService"` findet nur die
Definition selbst) — Bankabgleich ist über die API aktuell komplett
unerreichbar, als neues Backlog 0.37 dokumentiert. Da kein HTTP-Pfad
existiert, sind die 6 neuen Tests (`server/internal/accounting/bank_integration_test.go`)
service-level (direkter Aufruf gegen echtes Postgres), analog zum bereits
etablierten Muster für HTTP-lose Domänen (`internal/hr/service_scoping_integration_test.go`).
Decken Ingest, "Statement bereits gematcht"-Schutz, Referenz-Erkennung VOR
Betrags-Heuristik (bewiesen per zwei Rechnungen mit identischem offenem
Betrag), Betrags-Heuristik, Mehrdeutigkeits- und Kein-Treffer-Fehler ab.

**Zwei genuine Bugs beim Schreiben dieser Tests gefunden UND behoben**
(beide direkt in den zu testenden Funktionen selbst, daher im Scope): (1)
`Ingest()`s Nil-Check auf `raw` griff wegen der klassischen Go-"typed nil
in interface"-Falle NIE — jeder Ingest ohne explizites `Raw`-Feld verletzte
`bank_statements.raw NOT NULL`; (2) `findInvoiceIDInReference()`s erstes
Regex-Muster erkannte ein "RE"-Präfix, hängte es aber beim DB-Lookup nie
wieder an den erfassten Zahlenteil an — fand dadurch NIE eine reale
Rechnung. Beide behoben. Ein dritter Bug (leerer `TaxCode` verletzt FK in
`ar.go`s `createTx`) liegt außerhalb von `bank.go` und wurde bewusst nur
umgangen (Test nutzt `DE19` statt leerem Code), als Backlog 0.38
dokumentiert statt hier mitgefixt. `go test ./internal/accounting/... -v
-count=1` gegen frische DB: alle 26 Tests im Paket PASS.

**Backlog 0.10 (`EmployeeService.Update()` lehnt unbekannte Patch-Keys jetzt
ab) behoben.** Der `switch` über die Patch-Keys übernahm nur eine feste
Whitelist in die SQL-`SET`-Klausel; jeder andere Key (Tippfehler wie
`activ` statt `active`) fiel still durch und der Aufruf kehrte ohne Fehler
zurück, obwohl nichts geändert wurde. Neue `employeeUpdatableFields`-
Whitelist validiert jetzt JEDEN Patch-Key VOR jedem DB-Zugriff, lehnt bei
mindestens einem unbekannten Key den GESAMTEN Patch ab (alles-oder-nichts).
Die drei zuvor identischen `switch`-Zweige zu einer Schleife vereinfacht
(funktional unverändert). `grep` bestätigt: dieses Patch-Map-Muster kommt
nirgendwo sonst im Modul vor — isolierter Einzelfall. Tests: bestehender
Test umgekehrt (erwartet jetzt Fehler statt stillem No-Op), neuer Test für
gemischte Patches, neuer Integrationstest beweist end-to-end (gegen echtes
Postgres, da `hr` keine HTTP-Anbindung hat), dass bekannte Felder weiterhin
korrekt angewendet werden UND ein abgelehnter Patch das im selben Patch
enthaltene bekannte Feld NICHT trotzdem ändert. `go test ./internal/hr/...
-count=1` gegen frische DB: alle Tests PASS.

**Backlog 0.11 (`LeaveService.Create()`: Tage-Berechnung bei vertauschten
Daten) behoben.** Neue Prüfung `EndDate.Before(StartDate)` direkt nach der
bestehenden Start/End-Pflichtfeld-Prüfung, damit VOR dem
`employeeOwned`-DB-Zugriff — dadurch jetzt direkt mit `NewLeaveService(nil)`
testbar (vorher nur über reine Zeitarithmetik nachweisbar, da `Create()`
selbst gepanict hätte). Eintägige Anträge (`StartDate==EndDate`) bleiben
gültig. Neuer Integrationstest beweist, dass der Normalfall (5-Tage- und
eintägiger Zeitraum) weiterhin korrekt funktioniert — reiner Unit-Test der
Validierung allein hätte eine Regression im Erfolgspfad nicht aufgedeckt.
`go test ./internal/hr/... -count=1` gegen frische DB: alle Tests PASS.

**Backlog 0.12 (`LeaveService.Create()`: keine Überschneidungsprüfung)
behoben.** Neue Prüfung nach dem `employeeOwned`-Check, vor dem Insert:
Standard-Intervall-Überlappungstest gegen bereits bestehende
`pending`/`approved` Anträge desselben Mitarbeiters. Bewusst NICHT
`rejected` — eine Ablehnung soll den Zeitraum nicht dauerhaft blockieren.
Neuer Integrationstest (nur DB-testbar) deckt alle vier Fälle ab:
Überschneidung mit pending/approved wird abgelehnt, ein direkt
anschließender nicht-überlappender Zeitraum bleibt erlaubt, ein rejected
Antrag blockiert nicht mehr. `go test ./internal/hr/... -count=1` gegen
frische DB: alle Tests PASS.

**Damit sind alle Epic-0-Funde 0.6-0.12 (aus den Test-Absicherungs-
Subtasks 0.1.1.x-0.1.3.x) vollständig abgearbeitet.** Verbleibend im
numerisch niedrigeren Bereich vor den später gefundenen 0.20-0.38: 0.13
(Migrationsrunner ohne Down-Migrationen), 0.14 (KRITISCH, Status
vermutlich veraltet, Kernziel laut eigenem Text bereits erreicht — braucht
eine bewusste Entscheidung/Prüfung, nicht einfach weiterarbeiten), 0.15
(gofmt, 40 Dateien).

**Backlog 0.14 als abgeschlossen markiert** (Statuskorrektur auf
Nutzeranfrage, kein Code-Fix nötig). Untersuchung ergab: alle drei
Unterpunkte waren bereits `[x]`, der eigene Text hielt "Kernziel erreicht"
fest, nur die Top-Level-Checkbox/das "wip, PRIORISIERT VOR 0.2.1.2.2+"-Label
waren veraltete Buchführung (die Priorisierungsbedingung ist ohnehin
gegenstandslos, Task 0.2 ist längst fertig). Auf Nutzeranfrage erneut
gegen eine frische, leere DB verifiziert statt den alten Notizen zu
vertrauen: komplette Migrationskette 001-064 läuft fehlerfrei
(`docker compose down -v`/`up`, `\dt` bestätigt leer, dann Testlauf über
denselben `migrate.Run`-Codepfad wie der echte Server-Start). Die drei
separat dokumentierten Folgefunde 0.16/0.17/0.18 wurden bei derselben
Gelegenheit erneut geprüft und sind nach wie vor unverändert offen (eigene
Backlog-Positionen, kein Bezug zu 0.14 selbst).

**Backlog 0.13 (Migrationsrunner unterstützt keine Down-Migrationen)
behoben — und als bedeutender Nebeneffekt Backlog 0.20 (KRITISCH,
Migrationen 050/051-Konstraint-Kollision) vollständig gelöst.** Siehe ADR
0005 (`docs/adr/0005-migration-versioning-and-down-migrations.md`) für die
vollständige Entscheidung. Kern: `server/internal/migrate/migrate.go`
bekam echtes Versions-Tracking (`schema_migrations`-Tabelle, jede Datei
läuft nur noch einmal statt bei jedem `Run()`-Aufruf erneut). Down-
Migrationen bleiben BEWUSST manuell/dokumentiert (Kommentarblock-Konvention,
bereits gelebte Praxis) statt automatisiert — kein Wechsel auf ein
externes Migrationstool, keine rückwirkenden Down-Kommentare für 001-064.

**Kritischer Fund WÄHREND der eigenen Verifikation, sofort behoben**: ein
Testlauf mehrerer Go-Pakete gleichzeitig deckte eine Race Condition auf —
mehrere Prozesse riefen `migrate.Run` parallel gegen dieselbe Test-DB auf
und versuchten dieselbe, noch nicht angewendete Migration gleichzeitig
auszuführen (`duplicate key value violates unique constraint
"schema_migrations_pkey"` bzw. direkte DDL-Konflikte). Fix: Postgres
Advisory Lock (`pg_advisory_lock`/`pg_advisory_unlock`) um den gesamten
`Run()`-Ablauf, über eine explizit aus dem Pool bezogene Einzelverbindung
(Advisory Locks sind session-, nicht transaktionsgebunden). Nach dem Fix
mehrfach mit erhöhter Parallelität (`-p 8`, `-p 16`, 9 Pakete gleichzeitig)
verifiziert — keine Konflikte mehr.

**Backlog 0.20 als Nebeneffekt gelöst**: voller `go test
./internal/http/... -count=1`-Lauf gegen frische DB zeigt jetzt nur noch
27 Fehlschläge statt zuvor ~70/75 — alle 27 einzeln auf bereits
dokumentierte Ursachen zurückgeführt (0.19/0.21/0.24-Muster/0.26/0.28/0.29/
0.34/0.35/0.38), bis auf EINEN neuen, unabhängigen Fund: Backlog 0.39
(`TestMaterialGroupDeleteRejectsTrimmedLegacyReferences` referenziert eine
nicht existierende Spalte `materials.updated_at` — per `git show HEAD`
bestätigt vorbestehend, nur vorher nie bis zu diesem Fehler durchgedrungen,
weil die 0.20-Kaskade den vollen Testlauf immer vorher abbrach).

Tests: neue Datei `server/internal/migrate/migrate_test.go` (erstmals
Tests für dieses Paket — externes Testpaket `migrate_test`, um einen
Importzyklus mit `testutil` zu vermeiden).

**Backlog 0.15 (CI-Format-Check würde fehlschlagen: 40 Go-Dateien nicht
gofmt-konform) behoben.** Von den ursprünglich 40 waren bis zu dieser
Subtask bereits 12 durch andere Subtasks nebenbei gofmt-konform geworden
(migrate.go, ar.go, hr/service.go, ...) — die verbliebenen 28 per `gofmt
-w` reformatiert. Diff-Review via `git diff -w` bestätigte: ausschließlich
semantik-neutrale Änderungen (Tab-statt-Leerzeichen-Einrückung, entfernte
überflüssige Leerzeilen am Dateiende, eine alphabetische Import-
Neusortierung in `router.go`) — keine Einzeiler-`if`-Umformatierung
tatsächlich angetroffen (Go erzwingt ohnehin überall Klammern). `gofmt -l`
danach leer. Voller `go test ./...` (Nicht-Integration) sauber; voller
`NALA_INTEGRATION=1 go test ./...` gegen frische DB zeigt eine IDENTISCHE
Fehlerliste wie vor der Reformatierung (27 HTTP- + 5 quotes-Fehlschläge,
alle bereits bekannt) — keine einzige neue Regression.

**Backlog 0.21 (`TestMaterialsCreateListAndGetFlow`: "Ungültige
Materialkategorie" auf frischer DB) behoben.** Root Cause: echter
Chicken-Egg-Zustand, kein reines Testfixture-Problem — `material_groups`
wird nur rückwirkend aus vorhandenen `materials.kategorie`-Werten befüllt,
auf einer leeren DB kann daher NIEMAND je das erste Material einer neuen
Kategorie anlegen, ohne die Kategorie zuerst explizit über die bereits
vorhandene Materialgruppen-Verwaltung (`POST /settings/material-groups`)
anzulegen. Tests entsprechend korrigiert (folgen jetzt dem realen,
vorgesehenen Ablauf) — neuer Helper `ensureIntegrationMaterialGroup`
behebt dabei NEBENBEI auch `TestPurchaseOrdersCreateAndGetFlow` (nicht im
ursprünglichen Fund benannt, aber vom selben Root Cause betroffen). 6
weitere `quotes`-Tests wurden ebenfalls korrigiert, kommen jetzt weiter,
scheitern aber an je einer NEUEN, vorher verdeckten Stelle — als 0.40
(classifyDomainError-Lücken, gleiches Muster wie 0.29) und 0.41
(Testdaten-Vermischung, evtl. verwandt mit 0.19) dokumentiert, bewusst
nicht mitgefixt. `go test ./internal/http/... -count=1` gegen frische DB:
von 27 auf 25 Fehlschläge zurückgegangen.

**Backlog 0.22 (`GET /api/v1/documents/{docID}` prüft keine
Mandantenzugehörigkeit) behoben — ECHTE Sicherheitslücke, schwerwiegender
als ursprünglich dokumentiert.** `OpenDocumentStream` öffnete den
GridFS-Stream vorher OHNE JEDE Prüfung, ob überhaupt eine
`material_documents`/`contact_documents`-Verknüpfung existiert — jeder
authentifizierte Nutzer mit `documents.read` konnte JEDE Datei im gesamten
GridFS-Bucket herunterladen (nicht nur mandantenübergreifend, komplett
ungebunden), solange die ObjectID bekannt/erraten war. Beide
`OpenDocumentStream`-Implementierungen (`materials`/`contacts`) prüfen
jetzt zwingend per JOIN gegen `company_id`, bevor der Stream geöffnet
wird — Fehlschlag → `404`, kein Existenz-Leak. **Nebeneffekt**: löst
dabei auch das unabhängig dokumentierte Backlog 0.17
(`TestContactDocumentsUploadListAndDownloadFlow` fehlte der
Content-Disposition-Header — alte Implementierung verwarf einen Lesefehler
still, neue prüft ihn zwingend). Neuer Integrationstest beweist Download
durch Eigentümer (200) vs. anderen Mandanten (404) für BEIDE Domänen
(Kontakt- und Material-Dokumente). `go test ./internal/http/... -count=1`
gegen frische DB: von 25 auf 24 Fehlschläge zurückgegangen.

**Backlog 0.23 (`projects.BuildQuoteSnapshot` referenziert nicht
existierende Spalte `contacts.telefon`) behoben — und dabei gleich Backlog
0.27 mit (identischer Tippfehler in einer Testfixture).** Einfacher
Tippfehler in Produktivcode (`c.telefon` → `c.phone`), aber per `grep -rn
"telefon"` über das gesamte Modul systematisch nach dem GLEICHEN Muster in
Testfixtures gesucht (nicht nur die eine im Backlog-Eintrag genannte Stelle)
— 3 weitere rohe SQL-`INSERT`-Statements gefunden und mitkorrigiert
(`quotes/approval_decisions_test.go`, 2× `quotes/imports_test.go`,
`http/quotes_integration_test.go`). Wichtig: die JSON-Feldnamen
`"telefon"` in Request-Bodies sind KEIN Bug (bewusste deutschsprachige
API-Konvention, korrekt auf `phone` gemappt) — nur rohe SQL-Spaltennamen
waren betroffen.
`TestProjectQuotePDFFlow` (0.23) und `TestApprovalRequestDecisionsMutateOnlyRequest`
(0.27) sind jetzt grün. Vier weitere GAEB-Import-Tests kommen jetzt
weiter, scheitern aber an einer NEUEN, vorher verdeckten Stelle
(`projects.nummer NOT NULL` fehlt in rohen Test-Inserts) — als neues
Backlog 0.42 dokumentiert. `TestGAEBImportProcessEndpoint` kommt ebenfalls
weiter, trifft jetzt sichtbar auf das bereits bekannte Backlog 0.31. `go
test ./internal/http/... -count=1` gegen frische DB: von 24 auf 23
Fehlschläge zurückgegangen.

**Backlog 0.24, 0.29 und 0.40 gemeinsam behoben** (alle drei strukturell
identisch: `classifyDomainError` erkannte bestimmte `errors.New(...)`-
Meldungen aus `sales`/`quotes` nicht als Validierungsfehler und lieferte
500 statt 400). Statt drei separate, mechanisch identische Fixes
nacheinander umzusetzen, in EINEM Edit an `classifyDomainError`
(`server/internal/http/v1.go`) alle benötigten Substring-Muster ergänzt:
"mindestens eine position" (0.24), "sind zulässig"/"können reviewt
werden"/"offene review-positionen" (0.29), "hat bereits ein material"/
"kein sichtbarer suchtreffer"/"hat kein material" (0.40). 8 von 9
betroffenen Tests sind jetzt vollständig grün; der neunte
(`TestQuoteFlowWithPricingAndPDF`) kommt an der ursprünglich gemeldeten
Stelle vorbei, trifft aber später auf die bereits bekannte, unabhängige
Backlog-0.35-Ursache. `go test ./internal/http/... -count=1` gegen
frische DB: von 23 auf 15 Fehlschläge zurückgegangen — der größte
Einzelfortschritt bisher in dieser Fund-Aufarbeitung.

**Backlog 0.25 (`POST /quotes/{id}/convert-to-invoice` ohne vorherigen
Status-Übergang) behoben.** Reiner Testfehler, kein Anwendungsbug: die
Guard-Regel (`convert-to-invoice` nur bei Status `sent`/`accepted`) ist
fachlich korrekt, der Test rief die Konvertierung nur ohne den nötigen
`POST /quotes/{id}/status`-Übergang vorher auf. Fix ergänzt genau diesen
Schritt. `grep` bestätigt: kein anderer Test hat dasselbe Problem
(existierende Convert-Tests sind entweder bereits korrekt oder bewusste
Negativtests). Der Test kommt jetzt deutlich weiter, scheitert aber später
an der bereits bekannten, unabhängigen Backlog-0.35-Ursache bei der
AUFTRAGS-Konvertierung — kein neuer Fund, 0.25s eigentliches Ziel (Angebots-
Konvertierung) ist nachweislich behoben. `go test ./internal/http/...
-count=1` gegen frische DB: weiterhin 15 Fehlschläge (derselbe Test bleibt
in der Liste, aber aus dem bereits bekannten 0.35-Grund statt 0.25) — kein
Rückschritt.

**Backlog 0.26 (`TestQuoteApplyVisibleMaterialCandidateSetsManualMapping`:
falscher Upload-Pfad) behoben.** Reiner Pfad-Tippfehler im Test
(`/api/v1/quotes/imports` statt `/api/v1/quotes/imports/gaeb`), korrigiert.
Test kommt danach deutlich weiter, PANICT aber mit demselben Absturz wie
das bereits dokumentierte Backlog 0.30 — bei genauerer Prüfung bestätigt:
der Test hat eine EIGENE, direkt (nicht über den HTTP-`handler`)
konstruierte `quoteSvc := quotes.NewService(env.PG, nil)`-Instanz für
Aufrufe ohne dedizierten HTTP-Endpunkt (`UpdateImportItemReview`,
`MarkImportReviewed`, `ApplyImportToDraftQuote`) — derselbe
`nil`-NumberingService-Anti-Pattern wie in 0.30, nicht der über
`NewRouterWithDeps` konstruierte Router (das war eine falsche
Zwischenvermutung, per Codelesen korrigiert). 0.30 um diese zweite
betroffene Testfunktion ergänzt; beide Panics werden bis zur Behebung von
0.30 per `-skip` ausgeschlossen. `go test ./internal/http/... -count=1
-skip "TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates|TestQuoteApplyVisibleMaterialCandidateSetsManualMapping"`
gegen frische DB: von 15 auf 14 Fehlschläge zurückgegangen.

**Backlog 0.28 (`TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests`:
falscher `reason_code` in Testfixture) behoben.** Kein Produktivcode-Bug:
die Reason-Code-Berechnung in `RequestApprovalForQuoteItem`
(`server/internal/quotes/service.go`) war korrekt; bei den Fixture-Preisen
`unitPrice=50 < costBasis=60` ergibt sich rechnerisch zwingend
`absoluteMargin=-10` ⇒ `reasonCode="negative_margin"`. Die übrigen
Assertions IM SELBEN Test (`TargetDifferenceSnapshot=-22`,
`MarginPercentSnapshot≈-16.67%`, `CurrentTargetStatus="below_cost"`) waren
bereits ausschließlich mit `negative_margin`-Semantik konsistent — nur die
eine `ReasonCode`-Assertion erwartete fälschlich `"below_target_margin"`.
Fixture-Preise bewusst NICHT geändert (`(50, 60)` wird an 17 weiteren
Stellen im selben Testfile identisch verwendet). Per systematischem
`grep -n "below_target_margin\|negative_margin"` zwei weitere, bisher
nicht einzeln benannte Fundstellen desselben Kopier-Fehlers entdeckt und
identisch korrigiert: `TestQuoteApprovalDecisionEndpointsRequireApprovePermission`,
`TestQuoteApprovalReworkQueueEndpointListsOnlyOpenLatestRejections` (beide
nutzen ebenfalls `seedHTTPApprovalDecisionQuote(..., 50, 60)`). Eine vierte,
oberflächlich ähnliche Stelle
(`TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`,
Preise 59.9/59.9) wurde geprüft und als rechnerisch korrekt
`"below_target_margin"` bestätigt — bewusst NICHT angefasst.

Beim Verifizieren der Korrektur zusätzlich einen neuen, bisher unbekannten
`classifyDomainError`-Fund (gleiches Muster wie 0.24/0.29/0.40) entdeckt und
direkt mitbehoben: `"Keine aktive Freigabeanforderung vorhanden"`
(`server/internal/quotes/service.go`, zwei Fundstellen) fiel auf
`500 internal_error` statt `400 validation_error`. Neues Substring-Muster
`"keine aktive freigabeanforderung"` in `server/internal/http/v1.go`
ergänzt.

Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Alle drei
korrigierten Tests einzeln gegen frische, per `\dt` bestätigt leere DB —
PASS. Voller `go test ./internal/http/... -count=1 -skip
"TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates|TestQuoteApplyVisibleMaterialCandidateSetsManualMapping"`:
von 14 auf 11 Fehlschläge zurückgegangen. `go test ./... -count=1`
(Nicht-Integrationspakete) ohne Fehler.

**Backlog 0.30 (`quotes.NewService(env.PG, nil)`-Panics) behoben.** In
`server/internal/http/quotes_integration_test.go` in den beiden
betroffenen Testfunktionen
(`TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates`,
`TestQuoteApplyVisibleMaterialCandidateSetsManualMapping`)
`quotes.NewService(env.PG, nil)` durch `quotes.NewService(env.PG,
settings.NewNumberingService(env.PG))` ersetzt (Import
`nalaerp3/internal/settings` ergänzt). Die übrigen 5 Vorkommen desselben
Musters im selben Testfile bewusst NICHT angefasst — geprüft, dass keine
davon `ApplyImportToDraftQuote` (oder sonst eine `numSvc.Next(...)`-Methode)
auf der lokalen Instanz aufruft. Beide Tests panicen nicht mehr, `-skip` ist
nicht mehr nötig.

**Cascading-Fund beim Vollsuite-Lauf ohne `-skip`**:
`TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` kommt jetzt bis
zur eigentlichen Fachassertion durch und deckt dort einen ECHTEN, vorher
durch den Panic maskierten mandantenübergreifenden Datenleck-Bug auf:
`listMaterialCandidatesForQuoteItem`
(`server/internal/quotes/service.go:~2968`) joint `materials` rein über
Text-Match auf Bezeichnung/Nummer, OHNE jede `company_id`-Einschränkung —
Materialien fremder Companies mit zufällig identischer Bezeichnung werden
als Kandidat vorgeschlagen. Als eigenständiger, NICHT in diesem Subtask
behobener Fund dokumentiert: **Backlog 0.43 (KRITISCH)**.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Beide vormals panicenden Tests einzeln
PASS. Voller `go test ./internal/http/... -count=1` (kein `-skip` mehr
nötig): 12 Fehlschläge (11 unveränderte Altfälle + 1 neu sichtbarer Fund,
siehe 0.43) — rechnerisch konsistent mit den vorherigen 11 bei
`-skip`-Ausschluss beider 0.30-Tests, kein Netto-Regress. **Hinweis zur
Verifikationsdisziplin**: ein erster Vollsuite-Lauf direkt nach einem
vorherigen Lauf OHNE zwischenzeitlichen DB-Reset ergab fälschlich 51
Fehlschläge (Testdaten-Kollisionen durch Wiederverwendung derselben
bereits befüllten DB) — nach korrektem Reset (`down -v` + `up -d --wait` +
`\dt`-Leerprüfung) reproduzierbar 12. Bestätigt erneut: JEDER
Verifikationslauf braucht einen frischen DB-Reset unmittelbar davor, auch
wenn kurz zuvor bereits einer gemacht wurde.

**Backlog 0.31 (`TestGAEBImportProcessEndpoint`: `/api/v1`-Präfix fehlt)
behoben.** `server/internal/http/router.go`: `NewRouterWithDeps` zu einem
Wrapper um eine neue `NewRouterWithDepsAndOptions(pg, mg, rd, cfg, options
V1RouterOptions) http.Handler` umgebaut, die `NewV1RouterWithOptions(...)`
(statt bisher fest `NewV1Router(...)`) unter `/api/v1` mountet;
`NewRouterWithDeps` ruft sie mit demselben Default-Parser
(`quotes.GAEBXMLSubsetParser{}`) wie zuvor `NewV1Router`, um alle 12
übrigen Aufrufstellen (11 Testdateien + `internal/app/server.go`)
unverändert zu lassen. In `TestGAEBImportProcessEndpoint`
(`quotes_integration_test.go`) auf diese neue, korrekt unter `/api/v1`
gemountete Konstruktion umgestellt und alle Testpfade entsprechend
präfixiert — damit testet der Test jetzt den Router in derselben Form, wie
er auch produktiv gemountet wird (statt einer im Betrieb nie auftretenden
unpräfixierten Form).

Beim ersten eigenen Testlauf sofort einen selbst eingeführten
Regressions-Bug bemerkt und noch vor jeder Weiterarbeit korrigiert: die
erste `NewRouterWithDepsAndOptions`-Fassung hätte den `GAEBImportParser`
alter Aufrufer via `NewRouterWithDeps` auf `nil` statt auf den
ursprünglichen Default gesetzt — behoben, indem `NewRouterWithDeps`
explizit denselben Default übergibt.

Nebenbei ein mechanisch identischer Fund wie Backlog 0.42 im selben,
ohnehin bearbeiteten Testfunktions-Scope behoben (kein Scope-Creep):
`TestGAEBImportProcessEndpoint` legte das Test-Projekt per Direkt-SQL ohne
Pflichtfeld `nummer` an — erst durch den Router-Fix sichtbar geworden, da
der Test vorher nie so weit kam. Platzhalterwert ergänzt.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. `TestGAEBImportProcessEndpoint` isoliert
PASS. Voller `go test ./...` (Nicht-Integrationspakete) ohne Fehler. Voller
`go test ./internal/http/... -count=1`: von 12 auf 11 Fehlschläge
zurückgegangen.

**Backlog 0.32 erneut geprüft, NICHT als normaler Bugfix umgesetzt.** Beim
Versuch, 0.32 (kein Pfad legt einen Kontenrahmen für einen neuen Mandanten
an) als kleinen Subtask umzusetzen, stellte sich heraus, dass der Scope
deutlich größer ist als ursprünglich dokumentiert:
`server/internal/settings/company.go` (`CompanyService`) hat AUSNAHMSLOS
jede Methode fest auf `id='default'`/`company_id='default'` verdrahtet
(`Upsert` hat sogar `'default'` als SQL-Literal statt Parameter) — es gibt
also nicht nur keinen Kontenrahmen-Kopier-Schritt, sondern GAR KEINEN Weg,
überhaupt eine zweite `company_profiles`-Zeile anzulegen. Die
Mandantenfähigkeit aus Backlog 0.2 hat nur Datenmodell + Lese-/Schreib-
Scoping fertiggestellt, kein tatsächliches Onboarding-Feature. Als
eigenständiges Epic-Feature ("Mandanten-Onboarding", mit eigenen offenen
Produktentscheidungen) neu eingeordnet und bewusst NICHT als Bugfix
durchgemogelt (ein isolierter Kopier-Helfer ohne echten Aufrufer wäre
Attrappen-Code). Bleibt offen bis zu einer Nutzerentscheidung — kein
Blocker, da produktiv aktuell ohnehin nur ein Mandant existiert.
`docs/backlog.md` entsprechend aktualisiert, mit dem neuen Nächster-Schritt
direkt zu 0.33 übergegangen (kein Blocker).

**Backlog 0.33 (Draft-/Versions-Schreibschutz in `quotes` liefert 500 statt
400) behoben.** In `server/internal/http/v1.go`: `classifyDomainError` um
`"nur entwürfe sind bearbeitbar"` und `"schreibgeschützt"` im
`validation_error`(400)-Zweig ergänzt (vorher geprüft: `"schreibgeschützt"`
kommt im gesamten Nicht-Test-Code ausschließlich in
`server/internal/quotes/service.go` mit dieser einen Bedeutung vor, kein
Kollisionsrisiko). Da keine bestehende Testabdeckung diesen Pfad
tatsächlich erreichte (der naheliegende Kandidat
`TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource` scheitert
vorher an Backlog 0.34), neuen Test
`TestQuoteUpdateRejectsNonDraftStatusWithValidationError`
(`quotes_integration_test.go`) ergänzt: Entwurf anlegen, direkt auf
`accepted` setzen, `PATCH /quotes/{id}` versuchen, `400 validation_error
"nur Entwürfe sind bearbeitbar"` erwarten.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Neuer Test isoliert PASS. Voller `go test
./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: weiterhin 11 Fehlschläge (unverändert, da
dieser Fund keine vorher fehlschlagende Testabdeckung hatte — jetzt aber
dauerhaft neu abgedeckt).

**Backlog 0.34 (KRITISCH: `quotes.Service.Revise` "conn busy") behoben.**
Root Cause in `server/internal/quotes/service.go` (`Revise`): eine offene
`Rows`-Iteration über die Quell-Positionen wurde INNERHALB derselben
`for rows.Next()`-Schleife mit einem `tx.Exec(INSERT ...)` auf derselben
Connection kombiniert — klassischer pgx-v5-"conn busy"-Fehler, machte
`/revise` für JEDES real angelegte Angebot (mind. eine Position Pflicht)
komplett unbenutzbar. Fix: Positionen zuerst vollständig in einen Slice
eingelesen, `rows.Close()` VOR der zweiten (schreibenden) Schleife.

**Cascading-Fund**: mit funktionierendem `Revise` erreichte
`TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource` erstmals die
nachfolgenden Schreibschutz-Assertions und deckte einen weiteren
`classifyDomainError`-Gap auf (dieselbe Mechanik wie 0.33, andere
Meldungsfamilie): 9 "…überführt…"-Validierungsmeldungen rund um
Rechnungs-/Auftragsüberführung, verteilt über `quotes/service.go` UND
`sales/service.go` — dabei eine WORTGLEICH duplizierte Meldung in beiden
Domänen gefunden ("Historische Angebotsversionen können nicht in
Folgebelege überführt werden"), analog zum bereits dokumentierten
Duplizierungsmuster aus Backlog 0.36. `classifyDomainError` um
`"überführt"` (deckt alle 9 Treffer ab) und `"kann nicht manuell
umgestellt werden"` ergänzt.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Der komplette Test PASST jetzt
End-to-End (alle 6 Teilschritte). Voller `go test ./...`
(Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 11 auf 10 Fehlschläge zurückgegangen.

**Backlog 0.35 (KRITISCH: `sales.Service.ConvertToInvoice` "FOR UPDATE
cannot be applied to the nullable side of an outer join") behoben.** Root
Cause in `server/internal/sales/service.go` (`loadForInvoiceTx`): `SELECT
... FROM sales_orders so LEFT JOIN projects p ... LEFT JOIN contacts c
... FOR UPDATE` — Postgres verbietet `FOR UPDATE` auf einer Query mit
`LEFT JOIN`, sobald die Zeilensperre nicht explizit auf eine Tabelle
eingeschränkt wird. Jeder Aufruf von `ConvertToInvoice` scheiterte daher
immer mit `500`, unabhängig von Status/Daten — Aufträge konnten über
diesen Pfad überhaupt nicht in Rechnungen umgewandelt werden (analog
schwerwiegend zu 0.34). Fix: `FOR UPDATE` → `FOR UPDATE OF so` (identisches
Muster wie das bereits vorhandene, korrekte `FOR UPDATE OF quote_imports`
in `quotes/imports.go`). Vorher `grep`-geprüft: die übrigen 4
`FOR UPDATE`-Stellen in `sales/service.go` sowie alle Stellen in
`quotes/service.go` sind Single-Table-Queries ohne `JOIN`, nicht
betroffen.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean.
`TestSalesOrderStatusChangeAndConvertToInvoiceAreAuditLogged` isoliert:
PASS (`convert-to-invoice` liefert jetzt `201` statt `500`). Voller `go
test ./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 10 auf **8** Fehlschläge
zurückgegangen — neben dem Zieltest verschwand auch
`TestCommercialWorkflowEndpointListsOpenFollowActions` (nutzte denselben
blockierten Pfad für eine Teilrechnung, kein eigener Backlog-Eintrag,
durch denselben Fix mitbehoben).

**Backlog 0.36 (Steuerkennzeichen-Fallback-Bug dupliziert in
`quotes`/`sales`) behoben.** `quotes/service.go` und `sales/service.go`
hatten je eine eigene, unabhängige `taxRate`-Kopie mit hartcodiertem
`DE19`/`DE7`-Switch und stillem 0%-Fallback für unbekannte Codes — exakt
das in Backlog 0.7 für `accounting` behobene Muster. Fix: in beiden
Paketen unabhängig (Design-Entscheidung: `loadTaxCodes`-Ansatz
wiederholen statt sofort in ein Shared-Package `internal/tax` zu
extrahieren, um `accounting/ar.go` nicht anzufassen) `taxCodeInfo`/
`loadTaxCodesTx`/`taxRate` nach demselben Muster wie `accounting/ar.go`
ergänzt — unbekannter/inaktiver Code liefert jetzt einen Fehler statt
stillem 0%. `quotes.calcTotals` entsprechend um einen Error-Rückgabewert
erweitert. Alle 6+6 Aufrufstellen in beiden Dateien umgestellt.

Bestehenden `sales`-Unit-Test korrigiert (testete bisher explizit das
ALTE Verhalten). Neue Unit-Tests in `quotes/service_test.go` ergänzt
(hatte bisher GAR KEINE Tests für `taxRate`/`calcTotals`). Cascading-Fund:
die neue Fehlermeldung traf kein `classifyDomainError`-Muster (betrifft
rückwirkend AUCH `accounting`, dessen HTTP-Status für diesen Fall nie
verifiziert wurde) — `classifyDomainError` um `"unbekanntes oder
inaktives steuerkennzeichen"` und `"kein umsatzsteuer-konto"` ergänzt.
Neuer HTTP-Test `TestQuoteCreateRejectsUnknownTaxCode` bestätigt: `POST
/quotes/` mit unbekanntem Tax-Code liefert jetzt `400` statt vorher
stillem `201` mit falscher (zu niedriger) Steuer.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. `go test
./internal/{quotes,sales,accounting}/... -count=1`: alle PASS. Voller `go
test ./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: weiterhin **8** Fehlschläge (unverändert,
kein Regress, aber ein zuvor kritischer und ungetesteter Silent-Bug jetzt
korrekt validiert und dauerhaft abgesichert).

**Backlog 0.37 (`BankService` ohne HTTP-Anbindung) behoben.**
`bankSvc := accounting.NewBankService(pg, paymentSvc)` in
`server/internal/http/v1.go` ergänzt sowie eine neue Routengruppe
`/bank-statements` (`GET /`, `POST /`, `POST /{id}/match`) mit neuen
Permissions `bank.read`/`bank.write` (Migration `065_bank_permissions.sql`,
zugewiesen an `role-finance`/`role-admin`, analog zu
`030_accounting_permissions.sql`). Cascading-Fund: `"Statement bereits
gematcht"` traf kein `classifyDomainError`-Muster (500 statt 400) —
ergänzt. Neuer HTTP-Test `TestBankStatementsIngestListAndMatchFlow`
deckt 403/201/200/200/400 über den kompletten Flow ab (inkl.
Zahlungsanwendung: Rechnung wechselt zu `paid` mit korrektem
`paid_amount`).

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean, Migration `065` wendet sich sauber an.
Bestehende `TestBank*`-Tests aus Backlog 0.9 weiterhin PASS (Service-Ebene
unverändert). Neuer HTTP-Test isoliert: PASS. Voller `go test ./...`
(Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: weiterhin **8** Fehlschläge (unverändert,
kein Regress).

**Backlog 0.38 (`accounting.ARService.createTx` verletzt FK bei leerem
TaxCode) behoben.** `createTx()` übergab `it.TaxCode` als rohen Go-String
direkt an den INSERT — bei `TaxCode: ""` wurde die leere Zeichenkette
statt SQL NULL eingefügt, was die FK-Constraint
`invoice_out_items_tax_code_fkey` verletzte (kein `tax_codes`-Eintrag mit
`code=''`), obwohl `calcTotals`/`taxRate` einen leeren Code seit Backlog
0.7 bewusst als "steuerfrei" behandeln. Fix: neue `nullIfEmpty`-Helferfunktion
in `accounting/ar.go` (identisches Muster wie `quotes`/`sales`/`projects`),
angewendet auf `tax_code` beim Insert.

**Korrektur der ursprünglichen Fund-Beschreibung**: der Fund schlug vor,
AUCH `AccountCode` auf NULL abzubilden — bei Prüfung des Schemas stellte
sich heraus, dass `account_code` NOT NULL ist. Stattdessen wird ein
leerer `AccountCode` jetzt VOR dem Insert als echter Validierungsfehler
abgefangen (`"account_code fehlt"`, 400 statt roher Postgres-Fehler).

Neuer Test `TestInvoiceOutCreateAcceptsEmptyTaxCodeAndRejectsEmptyAccountCode`
deckt beide Fälle ab. Verifiziert gegen frische, per `\dt` bestätigt
leere DB: `go build ./...`, `go vet ./...`, `gofmt -l` clean. Voller `go
test ./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 8 auf **7** Fehlschläge
zurückgegangen — neben dem Zieltest verschwand auch
`TestQuoteConvertToInvoiceBlocksOpenApprovalRework` (nutzte denselben
FK-Bug, kein eigener Backlog-Eintrag, mitbehoben).

**Backlog 0.39 (`TestMaterialGroupDeleteRejectsTrimmedLegacyReferences`:
nicht existierende Spalte `materials.updated_at`) behoben.**
`server/internal/http/settings_integration_test.go` seedete ein Material
per Direkt-SQL mit einer `ON CONFLICT ... SET kategorie = ...,
updated_at = now()`-Klausel — `materials` hat aber gar keine
`updated_at`-Spalte (nur `angelegt_am`, kein späteres ALTER TABLE fügt
sie hinzu). Root Cause: reines Kopier-Artefakt, vermutlich vom
strukturell ähnlichen `material_groups`-Upsert direkt darüber übernommen
(der TATSÄCHLICH `updated_at` hat). Fix: die fehlerhafte
`updated_at = now()`-Klausel aus dem Test-Fixture entfernt (nicht die
Alternative — eine neue Spalte einführen —, da nirgends im
Anwendungscode `materials.updated_at` gelesen/erwartet wird).

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Test isoliert: PASS. Voller `go test
./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 7 auf **6** Fehlschläge
zurückgegangen.

**Backlog 0.41 (`TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`
schlägt fehl) behoben.** Root Cause war KEIN Cross-Test-Datenleck (wie im
ursprünglichen Fund vermutet und explizit gegen Backlog 0.19 geprüft und
verworfen), sondern eine falsche Assertion — Copy-Paste-Fehler von der
strukturell ähnlichen `TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem`
(erwartete deren `"Preisquellen Position"` statt der eigenen
`"Preispriorisierung Position"`), exakt dasselbe Muster wie Backlog 0.28.
Assertion korrigiert.

**Cascading-Fund**: danach deckte der Test einen echten Produktivbug auf:
`POST .../apply-target-price` scheiterte IMMER mit `500` — die
CHECK-Constraint `chk_quote_item_price_decisions_type`
(Migration 048) erlaubte nur `decision_type='primary_source_applied'`,
aber `insertTargetPriceAppliedDecisionTx` fügt seit jeher
`'target_price_applied'` ein; die Migration wurde offenbar nie
aktualisiert, als die Zielpreis-Funktion hinzukam — der Endpunkt war seit
Einführung komplett unbenutzbar. Neue Migration
`066_quote_item_price_decisions_target_price_type.sql` erweitert die
Constraint um beide tatsächlich verwendeten Werte.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean, Migration `066` wendet sich sauber an.
Test isoliert: PASS, komplett durchgelaufen. Voller `go test ./...`
(Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 6 auf **5** Fehlschläge
zurückgegangen.

**Backlog 0.42 (`projects`-Testfixtures in `internal/quotes` ohne
Pflichtfeld `nummer`) behoben.** In
`server/internal/quotes/imports_test.go` beide betroffenen
`INSERT INTO projects`-Statements (in
`createUploadedGAEBImportForProcessingTest`, genutzt von 3 Tests, sowie
in `TestQuoteImportParseResultStoresItemsAndUpdatesStatus`) um `nummer`
mit eindeutigem Platzhalterwert ergänzt (analog zum identischen Fix aus
Backlog 0.31). Alle vier betroffenen Tests isoliert: PASS. Voller `go
test ./...` (alle 15 Pakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: weiterhin 5 Fehlschläge (unverändert, da
die betroffenen Tests in `internal/quotes` liegen, nicht in
`internal/http` — kein Regress).

**Backlog 0.43 (KRITISCH: `listMaterialCandidatesForQuoteItem` nicht nach
`company_id` gescoped) behoben.** Die Funktion jointe `materials` bisher
rein textuell (Bezeichnung/Nummer) OHNE `company_id`-Einschränkung — ein
waschechtes mandantenübergreifendes Datenleck. Fix: `companyID`-Parameter
ergänzt, SQL um `AND m.company_id = $2` erweitert.

**Wichtige Korrektur bei der Verifikation**: der `company_id`-Fix allein
behob den vollen Suite-Lauf NICHT — Root Cause war eigentlich eine reine
Testdaten-Namenskollision INNERHALB derselben Company (`default`), da es
aktuell (Backlog 0.32) gar keinen Weg gibt, über die Anwendung einen
echten zweiten Mandanten anzulegen. Der ursprüngliche Fund hatte
fälschlich angenommen, die beiden kollidierenden Test-Materialien
gehörten zu unterschiedlichen Companies. Testfixture umbenannt, um die
Kollision zu beseitigen. Zusätzlich einen ECHTEN Cross-Tenant-Test
`TestQuoteGAEBImportMaterialCandidatesAreScopedToCompany` ergänzt (per
Direkt-SQL zweiter `company_profiles`-Eintrag + Nutzer, analog zum
bestehenden Muster aus Backlog 0.22), der den `company_id`-Fix tatsächlich
beweist.

**Nebenbei beim abschließenden vollen Suite-Lauf**: `classifyDomainError`
um `"können bearbeitet werden"` ergänzt (direkt gefunden, da
`TestQuoteFlowWithPricingAndPDF` nach der 0.35-Behebung neu bis zu dieser
Stelle durchkam). Beim systematischen Abgleich ALLER `errors.New(...)`-
Meldungen in `quotes`/`sales`/`accounting` gegen die aktuellen
`classifyDomainError`-Muster eine ganze Reihe WEITERER, noch offener
Lücken gefunden — darunter das fundamentale `"keine Positionen"`-Guard in
ALLEN DREI kommerziellen Kern-Domänen (7 Fundstellen). Bewusst NICHT mehr
spontan in dieser Subtask mitbehoben (Umfang zu groß für ein triviales
Anhängen), sondern als eigener, neuer Fund dokumentiert: **Backlog 0.44**.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Beide neuen Tests isoliert: PASS. Voller
`go test ./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 5 auf **4** Fehlschläge
zurückgegangen.

**Korrektur einer eigenen Fehleinschätzung**: bei einer ersten Durchsicht
wurde 0.43 fälschlich als "letzter offener Fund der flachen Epic-0-Liste"
eingestuft. Bei genauerer Prüfung des GESAMTEN `docs/backlog.md` (nicht
nur des chronologisch zuletzt bearbeiteten Abschnitts) zeigt sich: die
Backlog-Punkte **0.16**, **0.18** und **0.19** sind ebenfalls noch offen
(`- [ ]`) — sie wurden früh in dieser Session gefunden (verschachtelt
unter Subtask 0.2.1.2.1, NICHT Teil der später chronologisch
angehängten "flachen Liste" 0.6ff., daher beim Abarbeiten in
aufsteigender Nummernfolge bisher übersehen) und betreffen exakt die
JETZT noch verbleibenden `internal/http`-Testfehlschläge:
`TestContactTasksCreateListUpdateAndDeleteFlow` (0.16),
`TestContactCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
(0.18), `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
(0.19, gemeinsam mit `TestProjectQuotePDFFlow` — Namenskollision bei
Kontakt-Fixtures). `TestQuoteFlowWithPricingAndPDF` ist dagegen NEU
(Backlog 0.44, s. o.) — dieser Test war zuvor durch die inzwischen
behobenen 0.24/0.35 blockiert und erreicht jetzt erstmals die
`classifyDomainError`-Lücken.

**Backlog 0.16 (`TestContactTasksCreateListUpdateAndDeleteFlow`: "expected
due date to roundtrip") behoben — SYSTEMWEITER Fix.** Erste Hypothese
(Postgres-Sitzungszeitzone `Europe/Berlin` statt UTC im Testcontainer)
per `RuntimeParams["timezone"]="UTC"` getestet und VERWORFEN — die
Sitzungszeitzone war danach nachweislich UTC, der Test schlug trotzdem
weiter fehl. Per gezieltem Diagnose-Programm die tatsächliche Ursache
gefunden: pgx v5 dekodiert `timestamptz` standardmäßig ohne gesetzte
`ScanLocation` und fällt intern auf `time.Unix(...)` zurück, was
`time.Local` liefert — die Zeitzone des GO-PROZESSES (hier
`Europe/Berlin`), UNABHÄNGIG von der Postgres-Sitzungszeitzone. Betrifft
JEDES `timestamptz`-Feld der gesamten Anwendung, nicht nur diesen einen
Test. Fix: in `server/internal/db/postgres.go` (der EINEN zentralen
Stelle für `pgxpool`-Aufbau, von Produktivcode UND
`testutil.SetupIntegrationEnv` gleichermaßen genutzt) ein
`cfg.AfterConnect`-Hook, der für jede Verbindung `ScanLocation: time.UTC`
im `TimestamptzCodec` erzwingt. Verworfenen Zwischenfix wieder entfernt.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Zieltest isoliert: PASS. Voller `go
test ./...` (alle 15 Pakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 4 auf **3** Fehlschläge
zurückgegangen.

**Backlog 0.18 (`TestContactCommercialContextAggregatesQuotesSalesOrdersAndInvoices`:
`quote_items_tax_code_fkey`-Verletzung) behoben.** Root Cause: reiner
Tippfehler im Test — `"tax_code":"19"` statt `"DE19"`. Kein
`tax_codes`-Eintrag mit `code='19'` existiert. Vor Backlog 0.36 führte
das zu einer rohen FK-Verletzung (`500`); seit 0.36 liefert derselbe
Tippfehler bereits sauber `400 "unbekanntes oder inaktives
Steuerkennzeichen: 19"` — der Test bestand aber weiterhin auf `201`.
Fix: Tippfehler auf `"DE19"` korrigiert.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Zieltest isoliert: PASS (kompletter
Flow Angebot→Annahme→Auftrag→Rechnung→Commercial-Context). Voller `go
test ./...` (alle 15 Pakete) ohne Fehler. Voller `go test
./internal/http/... -count=1`: von 3 auf **2** Fehlschläge
zurückgegangen.

**Backlog 0.19 behoben — ursprüngliche Diagnose war STALE.** Die
ursprünglich dokumentierte Kontakt-Namenskollision zwischen
`TestProjectQuotePDFFlow` und
`TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices` ist
NICHT mehr reproduzierbar — beide Tests nutzen bereits unterschiedliche
Fixtures (vermutlich in einer früheren, nicht separat dokumentierten
Subtask dieser Session behoben, ähnlich wie bei 0.14). Beim erneuten
gemeinsamen Testlauf stattdessen einen ECHTEN, unabhängigen SQL-Bug
gefunden: `GET /projects/{id}/commercial-context` scheiterte IMMER
(auch isoliert) mit `500 ERROR: for SELECT DISTINCT, ORDER BY
expressions must appear in select list` — `listProjectInvoices`
(`server/internal/http/commercial_context.go`) sortiert nach
`i.created_at`, ohne diese Spalte in der `SELECT DISTINCT`-Liste zu
haben. Fix: `i.created_at` zur Select-Liste ergänzt (Scan-Ziel ohne
Weiterverwendung).

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Beide ursprünglich genannten Tests
gemeinsam: PASS, keine Kollision mehr. Voller `go test ./...` (alle 15
Pakete) ohne Fehler. Voller `go test ./internal/http/... -count=1` gegen
frische DB: von 2 auf **1** Fehlschlag zurückgegangen (nur noch
`TestQuoteFlowWithPricingAndPDF`, bereits als Backlog 0.44 dokumentiert).

**Backlog 0.44 (letzter offener technischer Fund der flachen Epic-0-Liste)
behoben — 🎉 `go test ./internal/http/... -count=1` läuft jetzt mit
0 Fehlschlägen gegen eine frische DB durch.** Alle 10 in der
Vollanalyse gefundenen `classifyDomainError`-Lücken (`"vollständig
fakturiert"`, `"muss größer als 0 sein"`, `"überschreitet die offene
restmenge"`, `"können nicht erneut umgestellt werden"`, `"keine
positionen"` — das fundamentale Positions-Pflichtguard in ALLEN DREI
Kern-Domänen —, `"dürfen nicht"`, `"hat keinen kunden"`, `"keine
priorisierte preisquelle"`, `"ungueltiger freigabeentscheid"`) in einem
Zug ergänzt, je auf Kollisionsfreiheit geprüft. Zwei vermutete Kandidaten
(`"Zielmarge ist ungueltig"`, die drei Auth-`"ungueltig..."`-Meldungen)
liefen bereits über andere, unabhängige Fehlerpfade — kein Fix nötig, zur
Vollständigkeit dokumentiert.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. `TestQuoteFlowWithPricingAndPDF`
isoliert: PASS, läuft komplett durch — entgegen der ursprünglichen
Erwartung war KEINE weitere Iteration nötig, die Vollanalyse hatte
bereits alle Lücken erfasst. Voller `go test ./...` (alle 15 Pakete)
ohne Fehler. Voller `go test ./internal/http/... -count=1` gegen frische
DB: von 1 auf **0** Fehlschläge zurückgegangen.

**Meilenstein**: die GESAMTE flache Epic-0-Fundliste (0.6-0.44 sowie die
früh gefundenen, zunächst übersehenen 0.16/0.18/0.19) ist damit
VOLLSTÄNDIG abgearbeitet. Einzig verbleibend offen in Epic 0: das
bewusst zurückgestellte, größere Feature **0.32** (Mandanten-Onboarding
— erfordert eine Produktentscheidung des Nutzers, siehe dortige
Begründung). Per erneutem `grep -n "\- \[ \] "` über das GESAMTE
`docs/backlog.md` bestätigt: außerhalb von Epic 0 sind nur noch die
separaten, nie in dieser Sweep-Serie bearbeiteten Feature-Epics A-I
offen (eigenständige künftige Produktarbeit, kein Teil dieser
Qualitäts-/Compliance-Sweep).

**Nutzerentscheidung**: Backlog 0.32 (Mandanten-Onboarding) jetzt umsetzen,
statt es auf später/andere Epics zu verschieben. Offene Produktfrage per
`AskUserQuestion` geklärt: EIN neuer HTTP-Endpoint, gesperrt über die
bestehende `admin.superuser`-Berechtigung (Backlog 0.5.3) — NICHT ein
separates CLI-/Ops-Tool.

**Subtask 0.32.1 (Voraussetzung) abgeschlossen und verifiziert:**
`CompanyService` (`server/internal/settings/company.go`) hatte
AUSNAHMSLOS jede Methode fest auf `company_id='default'` verdrahtet —
ein latenter Cross-Tenant-Bug (ähnliche Schwere wie 0.22), der VOR dem
eigentlichen Onboarding-Endpunkt behoben werden musste, da ein neuer
Mandant sonst nie seine EIGENEN Firmendaten/Niederlassungen hätte
verwalten können (jeder `PUT`/`POST`/`PATCH`/`DELETE` wäre auf die Daten
des `default`-Mandanten durchgeschlagen). Alle 8 Methoden um
`companyID`-Parameter erweitert, HTTP-Handler reichen
`companyIDFromContext(...)` durch. Neuer Cross-Tenant-Test
`TestCompanyProfileAndBranchesAreScopedToCompany` (per Direkt-SQL
zweiter Mandant, analog zu 0.22/0.43) bestätigt vollständige Isolation
inkl. `404` bei Cross-Tenant-Löschversuch. `branding.go`/
`localization.go`/`quote_calculation.go` bewusst NICHT angefasst — deren
Tabellen haben laut Schema gar keine `company_id`-Spalte (echte globale
Singleton-Einstellungen, kein Scoping-Bug).

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Neuer Test isoliert: PASS. Voller `go
test ./...` (alle 15 Pakete) ohne Fehler. Voller `go test
./internal/http/... -count=1` gegen frische DB: weiterhin **0**
Fehlschläge (kein Regress).

**Vorab-Analyse für Subtask 0.32.2** (Onboarding-Endpunkt selbst):
`tax_codes` (global, keine `company_id`-Spalte) und `number_sequences`
(self-initialisierend bei erstem `Next(...)`-Aufruf) brauchen KEIN
Seeding; `roles`/`permissions`/`role_permissions` sind ebenfalls global,
daher kann dem neuen Erst-Nutzer die bestehende `admin`-Rolle direkt
zugewiesen werden. Einzig `accounts` (Kontenrahmen, seit Migration 058
`company_id`-gescoped) muss aus einer Vorlage kopiert werden — das war
der ursprüngliche, engere 0.32-Fund.

**Subtask 0.32.2a (kritischer Blocker) gefunden UND behoben: `accounts.code`
war der GLOBALE Primärschlüssel, nicht `(company_id, code)`.** Beim
Entwurf des Onboarding-Endpunkts festgestellt: obwohl Migration 058
bereits eine `company_id`-Spalte auf `accounts` ergänzt hatte, blieb der
PK unangetastet (`code text PRIMARY KEY` allein) — ein zweiter Mandant
hätte NIE dieselben Standard-Kontonummern (1000, 1200, 8000, ...) wie
`default` bekommen können, "Kontenrahmen kopieren" war strukturell
unmöglich. Dem Nutzer per `AskUserQuestion` vorgelegt (echte
Architekturentscheidung mit Migrationsrisiko) — Entscheidung: **Schema
richtig fixen**.

**Fix**: neue Migration `067_accounts_company_scoped_pk.sql` —
`journal_lines`/`invoice_out_items` um eigene `company_id`-Spalte
ergänzt (per JOIN aus Kopftabelle deterministisch befüllt, dann `NOT
NULL`), `accounts.company_id` `NOT NULL` erzwungen, alte
Einzelspalten-FKs/-PK entfernt, neuer zusammengesetzter PK
`(company_id, code)` samt drei zusammengesetzten FKs (inkl.
`parent_code`-Selbstreferenz) angelegt. Zwei Go-Anpassungen als Folge
(`ar.go`/`journal.go` setzen `company_id` beim Insert). **Cascading-
Sicherheitsfund**: `loadTaxCodes` jointe `accounts` bisher OHNE
`company_id`-Filter — dasselbe Fehlermuster wie das bereits behobene
Backlog 0.43, hier aber erst durch den PK-Fix real gefährlich (zwei
Mandanten könnten jetzt echte, eigene, gleich benannte Konten haben).
Behoben: `companyID`-Parameter ergänzt, JOIN gescoped, alle 3
Aufrufstellen angepasst.

End-to-End bewiesen (nicht nur Schema-Inspektion): zwei Mandanten mit
JEWEILS Konto `code='1400'` koexistieren jetzt nachweislich
kollisionsfrei. Verifiziert gegen frische, per `\dt` bestätigt leere DB:
`go build ./...`, `go vet ./...`, `gofmt -l` clean, Migration `067`
wendet sich sauber an (auch unter `-p 8`). Voller `go test ./...` (alle
15 Pakete) ohne Fehler. Voller `go test ./internal/http/... -count=1`
gegen frische DB: weiterhin **0** Fehlschläge trotz PK-Umbaus.

**Subtask 0.32.2b (Onboarding-Endpunkt) abgeschlossen — 🎉 Backlog 0.32
VOLLSTÄNDIG umgesetzt.** Neue Route `POST /api/v1/platform/tenants/`,
gesperrt über `admin.superuser` (nicht tenant-scoped, da Mandanten-
Anlage grundsätzlich Plattform-Ebene ist). Neue Datei
`server/internal/http/tenant_onboarding.go`: `createTenant(...)` legt in
EINER gemeinsamen Transaktion atomar an: `company_profiles`-Zeile (ID
explizit oder per Slug aus dem Namen), Kopie ALLER `'default'`-Konten,
ersten Admin-Nutzer (direkt in der Transaktion statt über den
Pool-gebundenen `auth`-Service, um echte Atomarität zu garantieren —
kein "Mandant ohne Admin"-Zombie-Zustand), Zuweisung der globalen
`admin`-Rolle. Uniqueness-Prüfungen (ID, E-Mail) innerhalb derselben
Transaktion (keine TOCTOU-Lücke). Zwei neue `classifyDomainError`-
Substrings ergänzt (`"existiert bereits"`, `"abgeleitet werden"`).

Neuer End-to-End-Test `TestPlatformTenantsCreateOnboardsNewTenant`
beweist den KOMPLETTEN Flow im echten Produktivpfad: `403` ohne
Berechtigung, `201` für die Anlage, erfolgreicher LOGIN als neuer
Mandanten-Admin, eigenes Firmenprofil sichtbar, Kontenrahmen-Kopie
zahlenmäßig korrekt UND Kontonummer `1400` koexistiert nachweislich für
beide Mandanten ohne Kollision, `400` bei doppelter ID/E-Mail.

Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
`go vet ./...`, `gofmt -l` clean. Neuer Test isoliert: PASS. Voller `go
test ./...` (alle 15 Pakete) ohne Fehler. Voller `go test
./internal/http/... -count=1` gegen frische DB: weiterhin **0**
Fehlschläge.

Nächster Schritt: Backlog 0.32 ist damit VOLLSTÄNDIG abgeschlossen — vom
ursprünglich bewusst zurückgestellten Fund bis zum funktionierenden,
end-to-end verifizierten Mandanten-Onboarding-Feature. Damit sind sowohl
die flache Epic-0-Fundliste (0.6-0.44 + 0.16/0.18/0.19) als auch der
einzige zurückgestellte Punkt (0.32) abgeschlossen — Epic 0 ist komplett
abgearbeitet. Alle verbleibenden offenen Punkte in `docs/backlog.md`
gehören zu den separaten Feature-Epics A-I; hier braucht es eine neue
Priorisierungsentscheidung des Nutzers für die Fortsetzung.

**Zusatzbeobachtung bei der 0.5.2-Vollverifikation**: ein voller `go test
./internal/http/... -count=1`-Lauf (nicht Teil der eigentlichen
Verifikation, siehe oben) bestätigte Backlog 0.20 (Migrationen 050/051
nicht sicher wiederholt ausführbar) erneut — sobald der Kipppunkt erreicht
ist, bricht `migrate.Run` für JEDE danach aufgerufene Testfunktion in der
Datei-Reihenfolge des Testbinaries ab (nicht nur ~30 wie in 0.20 grob
geschätzt, sondern in diesem Lauf ca. 70 von ~75 Testfunktionen im
gesamten `internal/http`-Paket, inkl. der beiden neuen 0.5.2-Tests). Rein
eine Bestätigung/Präzisierung des bereits dokumentierten Fundes, keine neue
Ursache — separat durch gezielten `-run TestAuthLogin`-Lauf gegen frische DB
bestätigt, dass die 0.5.2-Tests für sich genommen sauber grün sind.

**Offene, nicht in dieser Subtask behobene KRITISCHE Funde** (Backlog
0.34: `quotes.Revise` "conn busy" bei jedem Angebot mit Positionen;
Backlog 0.35: `sales.ConvertToInvoice` "FOR UPDATE + LEFT JOIN"-Fehler bei
jedem Aufruf) — beide blockieren zentrale Funktionen komplett. Falls der
Nutzer eine Priorisierung dieser Bugfixes VOR der weiteren
Backlog-Reihenfolge wünscht, wäre das ein expliziter Hinweis wert (bisher
folgt diese Session strikt der numerischen Epic-Reihenfolge, kritische
Funde werden dokumentiert, nicht automatisch vorgezogen, außer wenn sie
laufende Arbeit blockieren — hier ist das nicht der Fall, da 0.5
auth-bezogen und unabhängig von `quotes`/`sales` ist).

**Wichtige Hinweise aus 0.3.3.x, für künftige Audit-Anbindungen weiterhin
gültig**: (1) beim Verdrahten IMMER prüfen, ob die Zielfunktion intern
eine `rows.Next()`-Iteration mit `tx.Exec(...)` INNERHALB der Schleife
kombiniert (pgx-"conn busy"-Falle, Backlog 0.34) oder eine `FOR UPDATE`
mit `LEFT JOIN` ohne `OF <table>`-Einschränkung nutzt (Backlog 0.35) —
beide Muster führen zu Fehlern, die NICHT mit der neuen Audit-Anbindung
verwechselt werden dürfen; isoliert (mit/ohne `WithAudit(...)`)
gegenprüfen, bevor eine Ursache zugeordnet wird. (2) Bereits bekannte
Blocker in Standard-Testflows (Backlog 0.21/0.24/0.25) machen es oft
nötig, für die end-to-end-Verifikation einen alternativen, nicht
blockierten HTTP-Pfad zu konstruieren, statt einen bestehenden, bereits
fehlschlagenden Testfluss wiederzuverwenden. (3) Funktionen ganz ohne
eigene Transaktion (wie `purchasing.Update` vor dieser Subtask) lohnt es
sich, für atomares Audit-Logging in eine Transaktion einzuwickeln — das
verbessert nebenbei oft auch die fachliche Korrektheit (hier: TOCTOU-Lücke
im Status-Guard geschlossen).

## Commits (diese Session)

- `a35be9a` — Phase 0 Recon, Feature 0.1 Tests, Mandanten-Scoping-ADR und
  Migrationsfixes (18 Dateien: docs/00-recon.md, 01-gap-analysis.md,
  adr/0001-0002, backlog.md, open-questions.md, state.md,
  accounting/{ar,bank,journal,payments}_test.go, hr/service.go(+test),
  sales/service_test.go, migrations 038/044-Fixes, 054/055-neu). Bewusst
  NICHT mitcommittet: vorbestehende, nicht von dieser Session stammende
  unversionierte Änderungen (`codex.md`, `client/lib/api.dart`,
  `client/lib/pages/quotes_page.dart`,
  `client/test/sales_order_context_pages_test.dart`,
  `server/internal/http/{quotes_integration_test,v1}.go`,
  `server/internal/quotes/{imports,imports_test,service,gaeb_xml_subset_parser,gaeb_xml_subset_parser_test}.go`,
  `aufgabe.md`, alle `docs/gaeb_*.md`) — diese stammen aus einer früheren
  Agent-Session (siehe `codex.md`) und wurden nie von mir gelesen/verändert;
  ihre Committierung ist eine gesonderte Entscheidung des Nutzers.
- `9fd01c6` — Task 0.2.1 abgeschlossen: Mandanten-/Standort-Scoping im
  Datenmodell (8 Dateien: docs/adr/0002 aktualisiert, backlog.md, state.md,
  Migrationen 056-060). Gleiche Abgrenzung wie beim ersten Commit — die
  vorbestehenden Alt-Änderungen bleiben unversioniert.

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
- Subtask 0.2.1.2.3: `056_sales_scope.sql` neu — `company_id`/`branch_id`
  (nullable, Expand-Contract) für `quotes`, `sales_orders`, `invoices_out`,
  `purchase_orders`. ADR 0002 präzisiert: Item-/Kind-Tabellen bekommen
  bewusst KEINE eigene Scoping-Spalte, sondern erben den Scope über ihren FK
  zur Kopf-Tabelle — reduziert die Migration von potenziell 11 auf 4
  Tabellen. Gegen frische DB verifiziert: voller `go test ./internal/http`-
  Lauf zeigt, dass die Migrationskette bis inkl. 056 bei den ersten ~9
  Testfunktionen fehlerfrei durchläuft (Beweis: 056 funktioniert korrekt auf
  frischer DB). Danach kaskadierendes Scheitern von ca. 30 weiteren Tests —
  geprüft und als **komplett unabhängig von `company_id`/`branch_id`**
  bestätigt: Ursache ist ein eigenständiges, gravierendes Migrations-
  Architekturproblem (`050_quote_item_approval_requests.sql` fügt einen
  Check-Constraint hinzu, den `051_quote_item_approval_decisions.sql` im
  selben Lauf sofort wieder entfernt — beim nächsten `migrate.Run`-Durchlauf
  gegen dieselbe, inzwischen befüllte DB versucht 050 den Constraint erneut
  anzulegen und scheitert an zwischenzeitlich eingefügten Testdaten). Als
  kritischer Backlog-Punkt 0.20 dokumentiert, bewusst NICHT behoben —
  eigenständiges Problem jenseits der Mandanten-Scoping-Arbeit, erfordert
  eigene Entscheidung (Migrationsrunner-Redesign mit Versions-Tracking oder
  Korrektur der 050/051-Constraint-Logik).
- Subtask 0.2.1.2.4: `057_materials_warehouses_scope.sql` neu — nur
  `materials` (company_id, mandantenweit) und `warehouses` (company_id +
  branch_id, natürlicher Standort-Anker) bekommen eigene Spalten.
  `locations`/`batches`/`stock_movements` bewusst NICHT gescoped (erben über
  `warehouse_id`/`material_id`) — ADR 0002 entsprechend präzisiert, gleiches
  Muster wie bei den Item-Tabellen aus 0.2.1.2.3. Gegen frische DB
  verifiziert (`NALA_INTEGRATION=1 go test ./internal/http -run
  TestMaterials`): Migration läuft bei allen 7 Testläufen fehlerfrei durch.
  Ein Testfehler gefunden (`TestMaterialsCreateListAndGetFlow`: "Ungültige
  Materialkategorie") und als unabhängig von `company_id`/`branch_id`
  bestätigt — Testfixture nutzt eine auf leerer DB unbekannte Kategorie
  (gleiches Muster wie 0.18/0.19). Dokumentiert als Backlog 0.21, nicht
  behoben.
- Subtask 0.2.1.2.5: `058_accounting_scope.sql` neu — `company_id` (kein
  `branch_id`, laut ADR 0002 mandantenweite Konsolidierung) für `accounts`,
  `journal_entries`, `bank_statements` (letzteres in der ursprünglichen
  Backlog-Kurzbeschreibung nicht explizit genannt, aber laut
  ADR-0002-Tabelle Teil derselben Buchhaltungs-Gruppe — ergänzend
  aufgenommen). `journal_lines` bewusst NICHT gescoped (erbt über
  `entry_id` von `journal_entries`). Gegen frische DB verifiziert
  (`TestInvoiceOutFlowWithPDFAndPayments`, deckt Buchung + Zahlung ab):
  PASS, Migration lief ohne Fehler.
- Subtask 0.2.1.2.6 (letzte des Task 0.2.1.2): `059_hr_scope.sql` neu —
  `hr_employees` und `hr_teams` bekommen jeweils eigene `company_id`/
  `branch_id` (kein Eltern-Kind-Verhältnis zueinander, `team_id` an Employee
  ist nullable). `hr_leave_requests`/`hr_absences` bewusst NICHT gescoped
  (erben über verpflichtendes `employee_id`). `hr_holidays` bewusst
  ausgenommen (Feiertagskalender, kein Mandantenbezug). Kein
  HTTP-Integrationstest für hr vorhanden (bestätigt Recon-Befund) — Migration
  über einen anderen, funktionierenden Integrationstest verifiziert
  (`TestMaterialsCreateIsForbiddenForSalesRole`, zeigt vollständigen
  fehlerfreien Migrationslauf inkl. 059). **Task 0.2.1.2 damit komplett: 6
  Migrationen (054-059), alle gegen frische DB verifiziert, durchgängig
  Expand-Contract (kein sofortiges NOT NULL).**
- Subtask 0.2.1.3 (letzte des Task 0.2.1): `060_users_numbering_scope.sql`
  neu. `users` bekommt eigene `company_id`/`branch_id` (Expand-Contract).
  `number_sequences` bekommt NUR die `company_id`-Spalte + Backfill + Index
  — die geplante PK-Umstellung auf `(company_id, entity)` wurde bewusst NICHT
  in dieser Migration umgesetzt: ein zusammengesetzter Primärschlüssel würde
  `company_id` sofort `NOT NULL` erzwingen (Postgres-Regel für PK-Spalten),
  was der Expand-Contract-Linie widerspricht, und `settings.NumberingService`
  filtert aktuell ausschließlich nach `entity` — eine sofortige PK-Änderung
  wäre technisch unauffällig (nur ein Mandant existiert bisher), aber eine
  stille Falle für später. ADR 0002 entsprechend präzisiert: PK-Umstellung
  folgt in einer eigenen Migration NACH Task 0.2.2. Gegen frische DB
  verifiziert (`TestAuthLoginAndMeFlow`: PASS, bestätigt dass die neuen
  `users`-Spalten den Login-Flow nicht brechen; `TestPurchaseOrdersCreate
  ReturnsStructuredValidationError`: PASS). **Task 0.2.1 (Datenmodell
  erweitern) damit komplett: 7 Migrationen (054-060) diese Session, alle
  gegen frische DB verifiziert.**
- Subtask 0.2.2.1.1: Beim Start von Task 0.2.2 eine Reihenfolge-Abhängigkeit
  erkannt: die ursprünglich als 0.2.2.2 geplante Middleware-Arbeit
  ("Mandanten-Kontext aus Auth-Session ableiten und in Request-Context
  legen") ist tatsächlich VORAUSSETZUNG für 0.2.2.1 (Repository-Filter),
  nicht ein separater, späterer Schritt — ohne Context-Verfügbarkeit kann
  keine Domänen-Query nach `company_id` filtern. Backlog entsprechend
  korrigiert: als 0.2.2.1.1 vorgezogen, 0.2.2.2 auf "erledigt durch 0.2.2.1.1"
  verwiesen. Umgesetzt: `auth.User` um `CompanyID`/`BranchID` erweitert
  (`server/internal/auth/types.go`), `FindUserByLogin`/`GetUserByID`
  (`server/internal/auth/repository.go`) laden beide Felder aus der DB,
  `requireAuth` (`server/internal/http/v1.go`) legt den vollständigen User
  bereits in den Context — neu: `companyIDFromContext`/`branchIDFromContext`
  als Zugriffshelfer für alle folgenden Micro-Subtasks. Zusätzlich
  `testutil.SeedAuthUser` um `company_id='default'` ergänzt (sonst hätten
  alle künftigen Integrationstests einen User mit leerer CompanyID — das ist
  kein unabhängiger Fund, sondern direkte Voraussetzung für diese Subtask,
  daher direkt behoben statt nur dokumentiert). 4 neue reine Unit-Tests
  (`server/internal/http/context_test.go`, kein DB-Zugriff). Nebenbei
  `gofmt -w` auf die drei tatsächlich bearbeiteten Dateien angewendet (waren
  bereits vorher nicht-konform, Teil von Backlog 0.15 — nur die drei hier
  ohnehin geänderten Dateien, nicht die übrigen 37). Gegen frische DB
  verifiziert: `TestAuthLoginAndMeFlow` PASS.
- Subtask 0.2.2.1.2.1: `contacts`-Kern-CRUD (`List`, `Create`, `Get`,
  `Update`, `DeleteSoft`, `ListActivity`) um `company_id`-Filter erweitert.
  `Create` validiert `companyID` jetzt explizit ("Mandant erforderlich",
  neuer Negativfall-Test). `Get`/`Update`/`DeleteSoft`/`List` filtern per
  `WHERE company_id=$N` — ein Zugriff auf einen fremden Mandanten liefert
  "nicht gefunden" statt Daten. HTTP-Handler (`v1.go`, 6 Call-Sites) reichen
  `companyIDFromContext(req.Context())` durch. Bestehende Validierungstests
  in `contacts/service_test.go` angepasst (fehlendes Argument ergänzt) +
  1 neuer Test für die Mandant-Pflichtprüfung. Gegen frische DB verifiziert
  (`go test ./internal/http -run TestContact`): 11 von 14 Tests PASS,
  identisch zu den bereits bekannten 3 unabhängigen Vorbefunden (0.16-0.18)
  — **keine neuen Regressionen durch die Scoping-Änderung**.
  **Echter Sicherheitsfund dabei entdeckt** (nicht von mir verursacht,
  vorbestehend): `CreateAddress()` (`server/internal/contacts/service.go:523-525`)
  prüft die Existenz/Zugehörigkeit des übergeordneten Kontakts gar nicht
  wirklich — der Code kommentiert selbst "we rely on FK for existence" und
  verwirft das Ergebnis der Prüfung. Damit können Unterressourcen (Adressen,
  vermutlich auch Personen/Notizen/Aufgaben) aktuell für JEDEN existierenden
  `contactID` angelegt werden, unabhängig vom Mandanten des Aufrufers.
  Nicht in dieser Subtask behoben (gehört zu 0.2.2.1.2.2), aber präzise
  dokumentiert statt nur pauschal "noch nicht gescoped" vermerkt.
- Subtask 0.2.2.1.2.2.1: Adress-CRUD (`CreateAddress`/`ListAddresses`/
  `UpdateAddress`/`DeleteAddress`) um echten Ownership-Check erweitert:
  `s.Get(ctx, contactID, companyID)` wird jetzt tatsächlich ausgewertet
  (statt wie vorher verworfen), platziert NACH reiner Feldvalidierung und
  VOR jedem DB-Zugriff (wichtige Reihenfolge-Korrektur während der Umsetzung:
  ein erster Versuch hatte den Check vor die `Art`-Validierung in
  `UpdateAddress` gesetzt, was `TestUpdateRejectsInvalidAddressKind` mit
  `NewService(nil)` hätte zum Absturz gebracht — analog zum bekannten
  purchasing-Bug, Backlog 0.6 — rechtzeitig beim Testlauf bemerkt und
  korrigiert, bevor es committet wurde). Damit ist der in 0.2.2.1.2.1
  gefundene Autorisierungs-Fund behoben. Bestehender Test angepasst (1 neues
  Argument). Gegen frische DB verifiziert: identische 11/14 PASS wie zuvor,
  keine neuen Regressionen.
- Subtask 0.2.2.1.2.2.2: Person-CRUD (`CreatePerson`/`ListPersons`/
  `UpdatePerson`/`DeletePerson`) um Ownership-Check erweitert, diesmal von
  Anfang an in korrekter Reihenfolge (Validierung → `s.Get` → DB-Zugriff).
  Bestätigt: `CreatePerson` hatte vorher GAR KEINEN Existenz-/Zugehörigkeits-
  Check (schlimmer als `CreateAddress`, das wenigstens einen — wenn auch
  verworfenen — Check-Versuch hatte). 3 bestehende Validierungstests
  angepasst (1 neues Argument je Aufruf). Gegen frische DB verifiziert:
  `TestContactPersonsRoleAndChannelRoundtripFlow` PASS, identische 11/14
  wie zuvor, keine neuen Regressionen.
- Subtask 0.2.2.1.2.2.3: Note-CRUD (`CreateNote`/`ListNotes`/`UpdateNote`/
  `DeleteNote`) um Ownership-Check erweitert (korrekte Reihenfolge von
  Anfang an). `ListActivity` (`activity.go`) musste angepasst werden, da sie
  `ListNotes` intern aufruft — reicht jetzt `companyID` durch (bereits als
  Parameter vorhanden aus 0.2.2.1.1). Keine bestehenden Note-Tests zum
  Anpassen vorhanden. Gegen frische DB verifiziert:
  `TestContactNotesCreateListUpdateAndDeleteFlow` PASS, identische 11/14
  wie zuvor, keine neuen Regressionen.
- Subtask 0.2.2.1.2.2.4: Task-CRUD (`CreateTask`/`ListTasks`/`UpdateTask`/
  `DeleteTask`) um Ownership-Check erweitert. Besonderheit gegenüber
  Address/Person/Note: `UpdateTask` lud schon vorher `current` per
  `getTask(taskID)` VOR jeder Feldvalidierung (bestehendes, nicht von mir
  verursachtes Verhalten) — da keine bestehenden Tests dieser Funktion mit
  `nil`-Pool existieren, war die "Validierung-vor-DB"-Reihenfolgeregel hier
  nicht anwendbar/nötig; Ownership-Check wurde ganz an den Anfang gesetzt
  (fail-fast vor jedem DB-Zugriff). `ListActivity` reicht `companyID` jetzt
  auch an `ListTasks` durch. Gegen frische DB verifiziert: identische 3
  Vorbefunde (0.16-0.18, exakt gleiche Fehlermeldungen wie zuvor),
  `TestContactActivityFeedAggregatesNotesTasksAndDocuments` weiterhin PASS
  (bestätigt `ListTasks`-Integration über Activity-Feed) — keine neuen
  Regressionen.
- Subtask 0.2.2.1.2.2.5 (letzte der 5 Unterressourcen-Subtasks):
  `UploadContactDocument`/`ListContactDocuments`
  (`server/internal/contacts/documents.go`) um Ownership-Check erweitert.
  Ownership-Check bewusst VOR dem GridFS-Upload platziert (nicht danach),
  um bei abgelehntem Zugriff keine verwaisten GridFS-Dateien zu erzeugen.
  `ListActivity` reicht `companyID` jetzt auch an `ListContactDocuments`
  durch. **Damit ist Subtask 0.2.2.1.2.2 (contacts-Unterressourcen)
  vollständig abgeschlossen** — alle 5 Ressourcentypen gescoped, zwei echte
  Autorisierungsfunde dabei behoben (Adressen, Ansprechpartner). Gegen
  frische DB verifiziert: identische 3 Vorbefunde, keine neuen Regressionen.
  **Neuer Fund** (nicht behoben, außerhalb Subtask-Scope): der generische
  Download-Endpunkt `GET /api/v1/documents/{docID}` prüft in KEINEM der
  beiden Zweige (`materials`/`contacts`) die Mandantenzugehörigkeit — eine
  bekannte GridFS-ObjectID reicht für mandantenübergreifenden Download.
  Betrifft zwei Pakete gleichzeitig, daher größer als diese Subtask;
  dokumentiert als Backlog 0.22.
- Task 0.2.2.1.2.3 (`projects`) bei Start als zu groß erkannt (30 Endpunkte
  über `server/internal/projects/service.go` (1021 Zeilen) +
  `import_logikal.go`/`analyze_logikal.go`, §6.3) und in 7 Micro-Subtasks
  zerlegt (`docs/backlog.md`), analog zum `contacts`-Muster (ein Kern-CRUD-
  Schritt, dann je Unterressourcentyp ein Schritt). Nur die erste umgesetzt:
  **0.2.2.1.2.3.1** — `List`/`Create`/`Get`/`UpdateStatus`/`BuildQuoteSnapshot`
  (`server/internal/projects/service.go`) um `company_id`-Filter erweitert;
  `Create` validiert `companyID` erforderlich ("Mandant erforderlich", neuer
  Negativfall-Test analog zu contacts). Da `projects.Service.UpdateStatus`
  nicht direkt per HTTP-Handler aufgerufen wird, sondern ausschließlich aus
  `quotes.Service.Accept()` (`server/internal/quotes/service.go:3379-3393`,
  beim Angebot-Annehmen mit optionalem Projektstatus-Wechsel) — musste
  `Accept()` selbst um einen `companyID`-Parameter erweitert werden, um ihn
  durchzureichen (einziger Aufrufer: `POST /quotes/{id}/accept` in `v1.go`).
  Zusätzlich `ImportLogikal()` (`server/internal/projects/import_logikal.go`)
  um `companyID` erweitert, da die Funktion intern `s.Get`/`s.Create` aufruft
  (Signaturänderung erzwingt Anpassung) — dabei **echten, vorbestehenden
  Sicherheitsfund behoben**: der Re-Import-Lookup
  (`SELECT id FROM projects WHERE nummer=$1`) suchte bisher OHNE
  Mandantenfilter nach einer vorhandenen `nummer` und hätte bei einer
  Nummernkollision zwischen zwei Mandanten das Projekt des FALSCHEN Mandanten
  per Re-Import überschrieben — jetzt zusätzlich `AND company_id=$2` in
  Lookup und UPDATE. HTTP-Handler (`v1.go`, 6 Call-Sites: List/Create/Get/
  BuildQuoteSnapshot/ImportLogikal/quoteSvc.Accept) reichen
  `companyIDFromContext` durch. `service_test.go` angepasst (1 neues
  Argument je Create-Aufruf) + 1 neuer Test für die Mandant-Pflichtprüfung.
  **Nebenbei entdeckt**: mein Edit-Tool hatte beim ersten Speichern von
  `service_test.go` versehentlich CRLF-Zeilenenden in eine zuvor LF-reine
  Datei eingeführt (bestätigt per `git show HEAD` vs. Arbeitskopie) — vor dem
  Verifikationslauf korrigiert (`sed 's/\r$//'`), da sonst die gesamte Datei
  als geändert erschienen wäre. `import_logikal.go` war dagegen schon VORHER
  CRLF und nicht-gofmt-konform (Teil von Backlog 0.15) — `gofmt -w` darauf
  angewendet (gleiche Konvention wie bei 0.2.2.1.1: nur auf tatsächlich
  berührte Dateien).
  **Verifikationsmethodik-Lektion**: ein erster Versuch, gegen frische DB zu
  verifizieren, zeigte scheinbar den bekannten Migrations-050/051-Fehler
  (Backlog 0.20) bereits beim ALLERERSTEN Testlauf nach einem
  `down -v && up` — das widersprach der bisherigen Annahme, dass der Fehler
  erst bei WIEDERHOLTEM `migrate.Run` gegen dieselbe DB auftritt. Ursache
  nachgewiesen: der DB-Reset war zum Testzeitpunkt noch nicht wirklich leer
  (vermutlich Race zwischen `docker compose down -v`/`up --wait` und dem
  vorherigen, sehr großen Testlauf). Ab sofort vor jedem Verifikationslauf
  zusätzlich per `docker exec ... psql -c '\dt'` bestätigt, dass die DB
  wirklich leer ist, BEVOR `go test` läuft — mit dieser Absicherung lief die
  komplette Kette 001–060 fehlerfrei durch.
  **Environment-Notiz**: `docker compose -f docker-compose.test.yml up` für
  `mongo_test` schlug in dieser Session mit einem Windows-Port-Exclusion-
  Konflikt auf Port 57017 fehl (`netsh interface ipv4 show
  excludedportrange protocol=tcp` bestätigt 57015–57114 als reserviert,
  vermutlich Hyper-V/WSL-NAT). Workaround: separater, nicht in
  `docker-compose.test.yml` verwalteter Container
  `docker run -d --name nalaerp3-mongo-adhoc -p 57150:27017 mongo:7`
  + `TEST_MONGO_URI=mongodb://localhost:57150` als Env-Override beim
  Testlauf. `docker-compose.test.yml` selbst NICHT verändert (kein
  reproduzierbarer, dauerhafter Fix — reines lokales Workaround, ggf. in
  einer Folgesession erneut nötig, falls der Windows-Portausschlussbereich
  sich nicht ändert).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/projects/... ./internal/quotes/...` PASS,
  `go test ./internal/http -run TestProject` — 2 Fehlschläge, BEIDE
  reproduzierbar als unabhängig von `company_id`/`branch_id` bestätigt (neue
  Funde, siehe Backlog 0.23 `c.telefon`-Spaltenfehler in
  `BuildQuoteSnapshot`, 0.25 Status-Guard blockiert Direkt-Convert-Test).
  Zusätzlich `-run TestQuoteFlowWithPricingAndPDF` einzeln laufen lassen
  (deckt den `/accept`-Endpunkt ab, der jetzt `companyID` durchreicht):
  `POST /quotes/{id}/accept` liefert `200` (Projektstatus-Wechsel
  funktioniert), Test scheitert erst SPÄTER an einer unabhängigen,
  vorbestehenden Stelle (Backlog 0.24, 500 statt 400 beim Löschen der
  letzten Auftragsposition, `server/internal/sales/service.go:577`, von mir
  nicht angefasst) — **kein Hinweis auf eine Regression durch die
  `company_id`-Änderung**. `go test ./internal/http -run TestContact` gegen
  dieselbe frische DB: identische 11/14 PASS wie beim letzten bekannten
  Stand (0.16-0.18 weiterhin offen) — keine neuen Regressionen durch die
  `v1.go`-Änderungen dieser Subtask.
- Subtask 0.2.2.1.2.3.2: Phasen-CRUD (`CreatePhase`/`ListPhases`/`GetPhase`/
  `UpdatePhase`/`DeletePhase`) um Ownership-Check über das übergeordnete
  Projekt erweitert — `project_phases` hat kein eigenes `company_id` (ADR
  0002, erbt über `project_id`), daher prüft jede Funktion jetzt
  `s.Get(ctx, projectID, companyID)` (die bereits gescopte Projekt-Abfrage
  aus 0.2.2.1.2.3.1) vor jedem DB-Zugriff — nach reiner Feldvalidierung, vor
  der eigentlichen Query (gleiche Reihenfolge wie bei den contacts-
  Unterressourcen). `GetPhase`/`UpdatePhase`/`DeletePhase` nehmen jetzt
  zusätzlich `projectID` entgegen (Route liefert es als `{id}` ohnehin mit)
  und scopen ihre Query zusätzlich per `WHERE id=$1 AND project_id=$2`
  (Doppel-Absicherung wie bei `contact_addresses`). HTTP-Handler (`v1.go`,
  5 Call-Sites) reichen `id` (Projekt-ID aus der Route) und
  `companyIDFromContext` durch. `import_logikal.go` ruft `CreatePhase`
  intern zweimal auf (Standard-Phase + Phase-je-Los) — beide Stellen um das
  bereits vorhandene `companyID`-Argument der Funktion ergänzt (keine neue
  Scope-Erweiterung nötig, nur Signaturanpassung). `service_test.go`
  angepasst (3 bestehende Tests um `projectID`/`companyID`-Argumente
  ergänzt; Validierungsreihenfolge in `UpdatePhase` bewusst so gewählt, dass
  Feldvalidierung vor dem Ownership-Check läuft — `NewService(nil)`-Tests
  bleiben dadurch ohne echten DB-Zugriff lauffähig).
  **Nebenbei erneut festgestellt**: das Edit-Tool wandelt bei jedem
  Speichern Zeilenenden der bearbeiteten Dateien zu CRLF um (bereits in
  0.2.2.1.2.3.1 notiert) — diesmal betraf es alle 4 in dieser Subtask
  bearbeiteten Dateien (`service.go`, `service_test.go`,
  `import_logikal.go`, `v1.go`); vor dem Verifikationslauf jeweils per
  `sed 's/\r$//'` auf LF zurückgesetzt und `gofmt -l` als sauber bestätigt.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/projects/...` PASS (alle bestehenden
  Validierungstests weiterhin grün). `go test ./internal/http -run
  TestProject`: identische 2 Fehlschläge wie bei 0.2.2.1.2.3.1 (Backlog
  0.23, 0.25 — beide unverändert unabhängig von dieser Änderung), Phasen-/
  Elevations-Create innerhalb von `TestProjectQuotePDFFlow` liefert `201`
  (Ownership-Check lässt den legitimen Eigentümer durch). `go test
  ./internal/http -run TestContact` gegen dieselbe frische DB: identische
  11/14 PASS — keine neuen Regressionen durch die `v1.go`-Änderungen dieser
  Subtask.
- Subtask 0.2.2.1.2.3.3: Elevationen-CRUD (`CreateElevation`/
  `ListElevations`/`GetElevation`/`UpdateElevation`/`DeleteElevation`) um
  Ownership-Check über die Kette Elevation→Phase→Projekt erweitert.
  `project_elevations` hat kein eigenes `company_id` (ADR 0002, erbt über
  `phase_id`) — statt die Kette erneut komplett nachzubilden, ruft jede
  Funktion `s.GetPhase(ctx, projectID, phaseID, companyID)` auf (die bereits
  in 0.2.2.1.2.3.2 gebaute, vollständige Ownership-Prüfung Projekt+Phase) und
  scopt die eigentliche Elevation-Query zusätzlich per `WHERE id=$1 AND
  phase_id=$2` — spart die Duplizierung der Prüfkette. Alle 5 Funktionen
  nehmen jetzt zusätzlich `projectID` und `phaseID` entgegen (Route liefert
  beide als `{id}`/`{phaseID}` ohnehin mit). `import_logikal.go` ruft
  `CreateElevation` an einer Stelle auf — um `p.ID` (Projekt, bereits im
  Funktionsscope vorhanden) und `companyID` ergänzt. HTTP-Handler (`v1.go`,
  5 Call-Sites) reichen `id`/`phaseID` aus der Route sowie
  `companyIDFromContext` durch. `service_test.go` angepasst (3 bestehende
  Tests um `projectID`/`phaseID`/`companyID`-Argumente ergänzt; gleiche
  Validierung-vor-Ownership-Check-Reihenfolge wie bei Phasen, daher
  weiterhin ohne DB mit `NewService(nil)` testbar).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/projects/...` PASS. `go test ./internal/http -run
  TestProject`: identische 2 bekannte, unabhängige Fehlschläge (Backlog
  0.23, 0.25), `POST .../phases` UND `POST .../phases/.../elevations`
  liefern beide `201` (gesamte Ownership-Kette funktioniert für den
  legitimen Eigentümer). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.2.3.4: Ausführungsvarianten-CRUD (`CreateSingleElevation`/
  `ListSingleElevations`/`GetSingleElevation`/`UpdateSingleElevation`/
  `DeleteSingleElevation`) um Ownership-Check erweitert. **Abweichung vom
  bisherigen Muster nötig**: die HTTP-Routen dieser Ressource
  (`/projects/{id}/elevations/{elevID}/single-elevations[/…]`) führen —
  anders als bei Phasen/Elevationen — KEINE `phaseID` im Pfad, daher konnte
  nicht wie zuvor `s.GetElevation(projectID, phaseID, elevID, companyID)`
  zur Ownership-Prüfung wiederverwendet werden (fehlender Parameter). Neuer,
  unexportierter Helfer `elevationOwnedByProject(ctx, projectID,
  elevationID, companyID)` ergänzt: prüft Projekt-Zugehörigkeit
  (`s.Get`) und lädt die Elevation über einen JOIN
  `project_elevations el JOIN project_phases ph ON ph.id=el.phase_id WHERE
  el.id=$1 AND ph.project_id=$2` — schließt die komplette Kette in einer
  Abfrage, ohne dass der Aufrufer die Phase kennen muss. Alle 5
  SingleElevation-Funktionen nutzen diesen Helfer vor jedem DB-Zugriff und
  scopen ihre eigene Query zusätzlich per `WHERE id=$1 AND
  elevation_id=$2`. `import_logikal.go` ruft `CreateSingleElevation` an zwei
  Stellen auf (davon eine in einer Closure, die `p`/`companyID` aus dem
  umschließenden Funktionsscope erfasst) — beide um `p.ID`/`companyID`
  ergänzt. HTTP-Handler (`v1.go`, 5 Call-Sites) reichen `id`/`elevID` aus
  der Route sowie `companyIDFromContext` durch. `service_test.go`
  angepasst (2 bestehende Tests um `projectID`/`elevationID`/
  `companyID`-Argumente ergänzt).
  **Testabdeckungslücke festgestellt** (nicht behoben, außerhalb Scope):
  für die `single-elevations`-Endpunkte existiert kein einziger
  Integrationstest (`grep -rl single-elevations internal/http/*_test.go`
  liefert keine Treffer) — die Ownership-Prüfung dieser Subtask ist daher
  nur durch Unit-Tests (reine Validierungspfade) und den erfolgreichen
  `go build` abgesichert, nicht durch einen End-to-End-HTTP-Testlauf.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/projects/...` PASS. `go test ./internal/http -run
  TestProject`: identische 2 bekannte, unabhängige Fehlschläge (Backlog
  0.23, 0.25 — beide decken keine single-elevations-Pfade ab, daher keine
  neue Aussage zu dieser Änderung möglich). `go test ./internal/http -run
  TestContact`: identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.2.3.5 (letzte der 4 verbleibenden Micro-Subtasks vor
  Assets/Imports): Materiallisten-CRUD (`ListProfilesBySingle`/
  `ListArticlesBySingle`/`ListGlassBySingle`/`LinkVariantMaterial`) um
  Ownership-Check erweitert. Route führt auch hier weder `phaseID` noch
  `elevationID` (`/projects/{id}/single-elevations/{sid}/materials...`) —
  analog zu 0.2.2.1.2.3.4 neuer, unexportierter Helfer
  `singleElevationOwnedByProject(ctx, projectID, singleElevationID,
  companyID)` ergänzt: löst die komplette Kette Projekt→Phase→Elevation→
  SingleElevation über einen doppelten JOIN in einer Abfrage auf. Alle 3
  List-Funktionen rufen ihn vor der jeweiligen (bereits vorhandenen,
  unveränderten) `WHERE single_elevation_id=$1`-Query auf. `LinkVariantMaterial`
  bekam zusätzlich eine echte Scoping-Härtung: die UPDATE-Statements
  (`single_elevation_profiles`/`_articles`/`_glass`) filterten bisher nur
  nach der Item-`id` — jetzt zusätzlich `AND single_elevation_id=$3`, sodass
  ein `itemID`, das nicht zur behaupteten `sid` gehört, folgenlos bleibt
  (Defense-in-Depth, gleiches Muster wie bei den übrigen Doppel-Scopes
  dieser Session). **Reihenfolge-Korrektur während der Umsetzung**: die
  reine `kind`-Validierung (der `switch`, der `"Ungültiger Typ"` liefert)
  wurde bewusst VOR den neuen, DB-gebundenen Ownership-Check gezogen (nicht
  danach, wie ein erster Entwurf es hatte) — sonst hätte
  `TestLinkVariantMaterialRejectsInvalidKind` mit `NewService(nil)`
  abstürzen können, da der Ownership-Check `s.pg` benötigt (identisches
  Muster zum bereits bekannten purchasing-Bug, Backlog 0.6, und zur
  Korrektur in Subtask 0.2.2.1.2.2.1) — beim Testlauf rechtzeitig bemerkt
  und vor dem Verifikationslauf korrigiert. HTTP-Handler (`v1.go`, 2
  Call-Sites) reichen `id`/`sid` sowie `companyIDFromContext` durch.
  `service_test.go` angepasst (2 bestehende Tests um `projectID`/
  `singleID`/`companyID`-Argumente ergänzt). Keine Änderung an
  `import_logikal.go` nötig (LogiKal-Import ruft keine dieser vier
  Funktionen auf).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/projects/...` PASS (inkl. beider
  `LinkVariantMaterial`-Tests, die die korrigierte Reihenfolge exercisen).
  `go test ./internal/http -run TestProject`: identische 2 bekannte,
  unabhängige Fehlschläge (Backlog 0.23, 0.25). `go test ./internal/http
  -run TestContact`: identische 11/14 PASS — keine neuen Regressionen.
  (Gleiche Testabdeckungslücke wie bei 0.2.2.1.2.3.4: auch für
  `single-elevations/.../materials` existiert kein Integrationstest.)
- Subtask 0.2.2.1.2.3.6: Projekt-Assets (`UpsertProjectAsset`/
  `GetProjectAsset`/`ListProjectAssets`) um Ownership-Check über das direkt
  übergeordnete Projekt erweitert — einfachster Fall dieser Serie, da
  `project_assets.project_id` direkt am Projekt hängt, kein Mehrebenen-JOIN
  nötig (`s.Get(ctx, projectID, companyID)` reicht). `UpsertProjectAsset`
  wird 6× aus dem großen ZIP-Upload-Handler (`POST /{id}/assets`,
  EMF→PNG-Konvertierungspipeline) aufgerufen — `companyID` einmal am
  Handler-Anfang ermittelt, gilt für alle 6 Aufrufe. **Selbstverschuldeter
  Fehler beim Umsetzen**: die sed-Ersetzung, die `companyID` an alle
  `UpsertProjectAsset(..., length)`-Aufrufe anhängen sollte, traf per
  End-of-Line-Anker auch eine direkt folgende `log.Printf(...)`-Zeile mit
  demselben Suffix (`..., length)`) — Build brach mit `log.Printf call
  needs 2 args but has 3 args` ab, sofort beim ersten `go build` nach der
  Änderung bemerkt und korrigiert (log.Printf-Zeile zurückgesetzt). Vor dem
  Verifikationslauf zusätzlich per Grep bestätigt, dass keine weitere
  `log.Printf`-Zeile durch dieselbe oder die zweite sed-Ersetzung
  (`"image/png", meta2.Length)`) getroffen wurde. `import_logikal.go`/
  `analyze_logikal.go` rufen keine dieser drei Funktionen auf — keine
  Änderung dort nötig.
  **Testabdeckungslücke**: auch für `/{id}/assets*` existiert kein
  Integrationstest (kein GAEB-fähiges Test-ZIP im Repo) — Absicherung nur
  über Unit-Build und die bereits etablierten TestProject-Regressionsläufe.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean
  (nach der Korrektur), `go test ./internal/projects/...` PASS (unverändert
  — diese Funktionen haben keine unit-testbare Validierung). `go test
  ./internal/http -run TestProject`: identische 2 bekannte, unabhängige
  Fehlschläge (0.23, 0.25). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.2.3.7 (letzte der 7 Micro-Subtasks): Import-Protokolle
  (`ListImports`/`ListImportChanges`/`ListImportChangesFiltered`/
  `UndoImport`/`ExportImportChangesCSV`) um Ownership-Check erweitert.
  `ListImports` direkt über `s.Get(projectID, companyID)` (Import-Läufe
  hängen 1:1 am Projekt). Für die vier import-ID-basierten Funktionen neuer
  Helfer `importOwnedByProject(ctx, projectID, importID, companyID)`
  ergänzt (Projekt-Ownership + `WHERE id=$1 AND project_id=$2` auf
  `project_imports`) — analog zu den JOIN-Helfern der Vorsubtasks, hier aber
  ohne JOIN, da `project_imports.project_id` direkt vorhanden ist.
  `ExportImportChangesCSV` ruft intern `ListImportChanges` auf — Signatur
  entsprechend mitgezogen. **Bewusst NICHT geändert**: `AnalyzeLogikal`
  (`server/internal/projects/analyze_logikal.go`) — liest ausschließlich die
  hochgeladene SQLite-Datei in ein `map[string]any`, berührt an keiner
  Stelle `s.pg`/Postgres und hat auch aktuell keinen `projectID`-Parameter;
  es gibt schlicht keinen Mandantenbezug zu prüfen (der Endpunkt analysiert
  eine Datei, keinen Datensatz). `import_logikal.go` schreibt
  Import-Protokolleinträge über direktes `s.pg.Exec` (nicht über die hier
  geänderten Service-Methoden) — keine Anpassung nötig, Build bestätigt dies
  bereits (keine Fehler in dieser Datei nach der Signaturänderung).
  HTTP-Handler (`v1.go`, 4 Call-Sites: List/Changes inkl. CSV-Export und
  gefiltert/Undo) reichen `id` (Projekt-ID aus der Route) und
  `companyIDFromContext` durch.
  **Damit ist Task 0.2.2.1.2.3 (`projects`, alle 30 Endpunkte) und in der
  Folge Subtask 0.2.2.1.2 (Kontakte/Projekte) komplett abgeschlossen** —
  beide Domänen vollständig mandantenscharf gescoped, jeweils über die
  gesamte Ressourcenhierarchie (contacts: 5 Unterressourcen; projects: 6
  Unterressourcen-Ebenen inkl. zweier neuer JOIN-Helfer für Routen ohne
  vollständige Pfad-Hierarchie).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/projects/...` PASS (unverändert). `go test
  ./internal/http -run TestProject`: identische 2 bekannte, unabhängige
  Fehlschläge (0.23, 0.25). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen. (Auch für
  `/{id}/imports*` existiert kein Integrationstest — gleiche
  Testabdeckungslücke wie bei 0.2.2.1.2.3.4-.6.)
- Task 0.2.2.1.3 bei Start als zu groß erkannt (§6.3: `quotes/service.go`
  allein 3543 Zeilen/41 HTTP-Call-Sites — deutlich größer als `projects`,
  das bereits 7 Micro-Subtasks brauchte) und in 4 Micro-Subtasks je
  Kopf-Tabelle/Domänen-Package zerlegt (`docs/backlog.md`), Reihenfolge
  klein→groß. Nur die erste umgesetzt: **0.2.2.1.3.1 — `purchase_orders`**
  (`server/internal/purchasing/service.go`, 249 Zeilen, kompakt genug für
  eine einzelne Subtask ohne weitere Zerlegung). `Create`/`Get`/`List`/
  `Update` um `company_id`-Filter erweitert (identisches Muster wie
  `contacts`/`projects`-Kern-CRUD: `Create` validiert `companyID`
  erforderlich vor allen anderen Prüfungen). `purchase_order_items` hat kein
  eigenes `company_id` (ADR 0002, erbt über `order_id`) — `CreateItem`/
  `UpdateItem`/`DeleteItem` rufen daher `s.Get(orderID, companyID)` als
  Ownership-Check auf (Wiederverwendung der bereits gescopten Kopf-Get,
  gleiches Muster wie bei `projects`-Phasen). HTTP-Handler (`v1.go`, 8
  Call-Sites) reichen `companyIDFromContext` durch.
  **Vorbestehender, unabhängiger Fund bestätigt (nicht behoben)**:
  `TestCreateRejectsInvalidItem` (`server/internal/purchasing/service_test.go`)
  panict weiterhin mit `NewService(nil)` — Backlog 0.6 (`s.pg.Begin(ctx)` vor
  Item-Validierung). Test-Signatur nur mitgezogen (companyID-Argument
  ergänzt), Panic-Ursache unverändert, per erneutem Einzellauf (`-run
  ^TestCreateRejectsInvalidItem$`) bestätigt identisch zur vorherigen
  Fundstelle (`service.go:87`, nur um eine Zeile verschoben durch die neue
  `Mandant erforderlich`-Prüfung). Übrige 5 Unit-Tests separat mit `-run`
  unter Ausschluss dieses einen Tests verifiziert (PASS), da ein Panic in
  Go den gesamten Testbinary-Lauf abbricht statt nur den einzelnen Test zu
  markieren.
  `purchasing/service.go` war bereits vor dieser Session nicht
  gofmt-konform (Teil von Backlog 0.15, bestätigt per `git show HEAD | gofmt
  -l`) — `gofmt -w` angewendet (gleiche Konvention wie bei den übrigen
  in dieser Session berührten Dateien).
  **Verifikations-Zwischenfall**: ein erster Lauf von `-run
  "TestPurchaseOrders"` (3 Tests) gegen eine per `\dt` als leer bestätigte
  DB zeigte einen Fehlschlag bei `TestPurchaseOrdersCreateAndGetFlow` mit
  "Kontakt mit gleichem Namen und gleicher E-Mail bereits vorhanden" — bei
  Isolations-Nachstellung (`-run
  ^TestPurchaseOrdersCreateAndGetFlow$`, erneut frische, verifiziert leere
  DB) trat dieser Fehler NICHT mehr auf; stattdessen (reproduzierbar über
  zwei weitere Läufe, auch im 3-Test-Filter) ein anderer, bereits bekannter
  Fehlschlag: "Ungültige Materialkategorie" beim Anlegen des
  Test-Lieferantenmaterials mit `"kategorie":"integration"` — identisches
  Muster zu Backlog 0.21 (`TestMaterialsCreateListAndGetFlow`), auf einer
  wirklich leeren DB ist `material_groups` leer. Die einmalige
  "Kontakt"-Fehlermeldung wird als Flake/Timing-Artefakt gewertet (nicht
  reproduzierbar), nicht als neuer Fund dokumentiert — festgehalten für den
  Fall, dass sie in einer Folgesession erneut auftritt.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/purchasing/...` (5 von 6 Tests via `-run`-Ausschluss)
  PASS. `go test ./internal/http -run TestPurchaseOrders`: 2/3 PASS,
  `TestPurchaseOrdersCreateAndGetFlow` scheitert an Backlog 0.21 (bestätigt
  unabhängig von `company_id`/`branch_id`). `go test ./internal/http -run
  TestContact`: identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.3.2: `invoices_out` (`server/internal/accounting/ar.go`,
  `ARService`) um `company_id`-Scoping erweitert. `Get`/`List`/`Book` um
  `company_id`-Filter, `Create` validiert `companyID` erforderlich VOR
  `s.pg.Begin(ctx)` (Lektion aus Backlog 0.6 direkt angewendet: die Prüfung
  steht bewusst vor dem Transaktionsstart). `invoice_out_items` hat kein
  eigenes `company_id` (ADR 0002, erbt über `invoice_id`).
  **Größer als bei `purchase_orders`, weil `ARService` von zwei anderen
  Domänen-Packages aufgerufen wird** (Rechnungserzeugung aus Angebot/
  Auftrag): `CreateFromQuoteTx`/`CreateFromSalesOrderTx`/`createTx` um
  `companyID` erweitert; dadurch mussten auch die Aufrufer angepasst werden
  — `quotes.Service.ConvertToInvoice` (`server/internal/quotes/service.go`)
  und `sales.Service.ConvertToInvoice` (`server/internal/sales/service.go`)
  bekamen je einen neuen `companyID`-Parameter, der NUR an den
  `arSvc`-Aufruf durchgereicht wird — bewusst KEINE Scoping-Änderung an
  `quotes`/`sales_orders` selbst (deren eigene Kern-CRUD-Absicherung folgt
  in den separaten Micro-Subtasks 0.2.2.1.3.3/.4). Zusätzlich
  `buildContactCommercialContext` (`server/internal/http/commercial_context.go`)
  um `companyID` erweitert (reicht an `arSvc.List` durch); die beiden
  anderen dort aggregierten Listen (`quoteSvc.List`/`salesSvc.List`) bleiben
  bewusst unscoped bis zu den entsprechenden Subtasks.
  `buildProjectCommercialContext`/`listProjectInvoices` NICHT angefasst —
  diese nutzen eine eigene, direkte SQL-Query statt `arSvc.List` und waren
  daher von der Signaturänderung nicht betroffen.
  HTTP-Handler (`v1.go`, 7 Call-Sites: List/Create/Get×2/Book/
  ConvertToInvoice×2) reichen `companyIDFromContext` durch. Neuer Unit-Test
  `TestCreateRejectsMissingCompanyID` (`ar_test.go`) ergänzt; bestehende
  `createTx`-Tests und der `sales`-Test `TestConvertToInvoiceRequiresARService`
  an neue Signaturen angepasst.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/accounting/... ./internal/sales/... ./internal/quotes/...`
  PASS. `go test ./internal/http -run ^TestInvoiceOutFlowWithPDFAndPayments$`:
  **voller End-to-End-Durchlauf PASS** (Create→List→Get→Book→PDF→Payment
  create/list, alle 200er/201er) — starker Beleg, dass das neue
  `company_id`-Scoping den legitimen Eigentümer-Flow nicht bricht. `go test
  ./internal/http -run ^TestQuoteFlowWithPricingAndPDF$`: Angebot annehmen →
  in Auftrag überführen → Auftragspositionen bearbeiten laufen fehlerfrei
  durch (inkl. des `/accept`-Pfads aus 0.2.2.1.2.3.1), Test scheitert weiterhin
  erst an der bereits bekannten, unabhängigen Stelle (Backlog 0.24, 500 statt
  400 beim Löschen der letzten Auftragsposition) — keine neue Fehlerquelle.
  `go test ./internal/http -run TestContact`: identische 11/14 PASS;
  `TestContactCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
  gezielt einzeln nachgeprüft — scheitert weiterhin VOR jedem Erreichen von
  `buildContactCommercialContext`/`arSvc.List` (Backlog 0.18,
  `quote_items_tax_code_fkey`-Verletzung beim Anlegen des Test-Angebots) —
  von dieser Subtask nicht berührt.
- Subtask 0.2.2.1.3.3: `sales_orders` (`server/internal/sales/service.go`,
  `Service`) um `company_id`-Scoping erweitert — größte Einzeldatei bisher
  in Task 0.2.2.1.3. `Get`/`List`/`Update`/`UpdateStatus` um
  `company_id`-Filter; `CreateFromQuote` validiert `companyID` erforderlich
  VOR `s.pg.Begin(ctx)` (Standardmuster). `sales_order_items` hat kein
  eigenes `company_id` (ADR 0002, erbt über `sales_order_id`) — der
  gemeinsame private Helfer `ensureOrderEditableTx` (bisheriger
  Status-Check vor jeder Item-Mutation) wurde um `companyID` erweitert und
  übernimmt damit zugleich den Ownership-Check für `CreateItem`/
  `UpdateItem`/`DeleteItem` — eine Stelle statt drei separate Prüfungen.
  `ConvertToInvoice` (bekam `companyID` bereits in 0.2.2.1.3.2 als reinen
  Durchreich-Parameter für `arSvc`) nutzt den Parameter jetzt zusätzlich
  selbst: `loadForInvoiceTx` wurde um `companyID` erweitert und sperrt den
  Auftrag jetzt nur noch, wenn er dem aufrufenden Mandanten gehört.
  **Aggregations-Handler nachgezogen** (Pflicht durch die `salesSvc.List`-
  Signaturänderung, keine gesonderte Design-Entscheidung): sowohl
  `buildContactCommercialContext` als auch `buildProjectCommercialContext`
  (`server/internal/http/commercial_context.go`) sowie
  `buildCommercialWorkflow` (`server/internal/http/workflow_cockpit.go`,
  bislang nicht in diesem Backlog erwähnt — beim Kompilieren entdeckt)
  riefen `salesSvc.List` ohne jede Mandantenfilterung auf; alle drei
  bekamen einen neuen `companyID`-Parameter, der durchgereicht wird.
  `buildProjectCommercialContext` hatte bislang GAR KEIN `companyID` im
  Scope (anders als die contact-Variante) — musste bis zu seinem
  HTTP-Handler in `v1.go` durchgezogen werden.
  `service_test.go` angepasst: `TestUpdateStatusRejectsUnknownStatus` um
  `companyID`-Argument ergänzt, neuer Test
  `TestCreateFromQuoteRejectsMissingCompanyID` ergänzt (der erklärende
  Kommentar über den DB-losen Testbarkeitsteilen wurde entsprechend
  präzisiert).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/sales/... ./internal/quotes/... ./internal/accounting/...`
  PASS (purchasing bewusst separat/exkludiert wegen Backlog 0.6). `go test
  ./internal/http -run ^TestQuoteFlowWithPricingAndPDF$`: convert-to-sales-order
  → Get → Update → CreateItem → UpdateItem → erstes DeleteItem laufen alle
  fehlerfrei (200er/201er) über die neu gescopten Funktionen, Test scheitert
  weiterhin erst an der bereits bekannten Stelle (Backlog 0.24). `go test
  ./internal/http -run ^TestInvoiceOutFlowWithPDFAndPayments$`: weiterhin
  vollständig PASS (bestätigt, dass `ConvertToInvoice`s companyID-Nutzung
  den AR-Flow nicht bricht). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.3.4.1 (erste von 5 Micro-Subtasks des größten Einzeltasks
  dieser Session): `quotes`-Kern-CRUD (`Create`/`createQuoteTx`/`Get`/
  `List`/`Update`/`UpdateStatus`/`Revise`) um `company_id`-Scoping erweitert.
  Gleiches Muster wie bei `sales_orders`: `Create` validiert `companyID`
  erforderlich vor `s.pg.Begin(ctx)`; `Get`/`List`/`Update`/`UpdateStatus`
  filtern nach `company_id`; `Revise` scopt sowohl die Quell-Sperrabfrage als
  auch die neu angelegte Revisions-Zeile (INSERT bekommt `company_id`
  mitgegeben, geerbt vom bereits geprüften Quell-Angebot). `Accept`
  (companyID bereits aus 0.2.2.1.2.3.1 vorhanden) nutzt ihn jetzt auch für
  seinen eigenen `UpdateStatus`-Aufruf; `ConvertToInvoice` (companyID
  bereits aus 0.2.2.1.3.2 vorhanden) scopt jetzt zusätzlich seine eigene
  `quotes`-Sperrabfrage. `quote_items` hat kein eigenes `company_id` (ADR
  0002, erbt über `quote_id`).
  **Ungeplante, aber notwendige Ausweitung**: das Ändern von `Get`s Signatur
  zwang 5 Funktionen aus den NOCH NICHT dran befindlichen Clustern
  (`ApplyMaterialCandidate`, `ApplySearchedMaterial` — Material-Matching,
  0.2.2.1.3.4.2; `ApplyPriceSuggestionForQuoteItem`,
  `ApplyPrimaryPriceSourceForQuoteItem`, `ApplyTargetUnitPriceForQuoteItem`
  — Preisfindung, 0.2.2.1.3.4.3) zur Anpassung, da sie den finalen Zustand
  über `s.Get` zurückgeben. Alle 5 teilen exakt dieselbe
  `SELECT status, superseded_by_quote_id FROM quotes WHERE id=$1 FOR
  UPDATE`-Eröffnungssperre wie die Kern-CRUD-Funktionen — daher wurde
  bewusst der VOLLE Quote-Ownership-Guard ergänzt (`AND company_id=$2`),
  nicht nur ein durchgereichter Parameter für den finalen `Get`-Aufruf, da
  der Mehraufwand identisch zur reinen Signaturanpassung war. Die TIEFERE
  Logik dieser 5 Funktionen (Material-Kandidaten-Suche,
  Preisvorschlags-Berechnung, jeweils eigene `quote_items`/`materials`-
  Zugriffe) bleibt bewusst UNVERÄNDERT — das ist die eigentliche Arbeit der
  Subtasks 0.2.2.1.3.4.2/.3.
  Analog musste `ApplyImportToDraftQuote` (`server/internal/quotes/imports.go`,
  Teil von 0.2.2.1.3.4.5 GAEB-Import) einen `companyID`-Parameter erhalten,
  da sie `createQuoteTx`/`Get` aufruft — rein durchgereicht an diese beiden
  Aufrufe, OHNE die `quote_imports`/`quote_import_items`-Tabellen selbst zu
  scopen (bleibt Aufgabe von 0.2.2.1.3.4.5).
  **Aggregations-Handler nachgezogen** (wie schon bei `sales_orders`):
  `buildContactCommercialContext`, `buildProjectCommercialContext`
  (`commercial_context.go`) und `buildCommercialWorkflow`
  (`workflow_cockpit.go`) riefen `quoteSvc.List` ungescoped auf — alle
  reichen jetzt ihr bereits vorhandenes `companyID` durch.
  `service_test.go` (quotes-Paket, erste Testdatei) neu angelegt:
  `TestCreateRejectsMissingCompanyID`, analog zu allen anderen
  Domänen-Paketen dieser Session.
  **Zwei Integrationstest-Call-Sites** (`quotes_integration_test.go`, direkte
  `quoteSvc.ApplyImportToDraftQuote`-Aufrufe ohne HTTP-Layer) benötigten das
  neue Argument — mit `"default"` ergänzt (demselben Wert, den
  `testutil.SeedAuthUser` als `company_id` seedet).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/quotes/... ./internal/sales/... ./internal/accounting/...`
  PASS. `go test ./internal/http -run ^TestQuoteFlowWithPricingAndPDF$`:
  Create→List→Get→Accept→convert-to-sales-order→Get laufen fehlerfrei über
  die neu gescopten Funktionen, Test scheitert weiterhin erst an der
  bekannten, unabhängigen Stelle (Backlog 0.24). `go test ./internal/http
  -run ^TestQuoteGAEBImportApplyCreatesDraftQuoteFromAcceptedItems$`:
  vollständig PASS (bestätigt `ApplyImportToDraftQuote`s companyID-Nutzung).
  Drei weitere Material-/Preis-Endpunkt-Tests einzeln nachgeprüft, um zu
  bestätigen, dass die 5 mechanisch mitgescopten Funktionen keine Regression
  auslösen — alle drei scheitern NACHWEISLICH VOR jedem Erreichen dieser
  Funktionen: zwei an Backlog 0.21 (Materialkategorie auf leerer DB), einer
  an einem neuen, unabhängigen Fund (Backlog 0.26: Testdatei nutzt falschen
  Upload-Pfad `/imports` statt `/imports/gaeb`, `405` statt `201`). `go test
  ./internal/http -run TestContact`: identische 11/14 PASS — keine neuen
  Regressionen.
- Subtask 0.2.2.1.3.4.2: Material-Matching-Cluster abgeschlossen. Bestand
  aus 4 Funktionen — Bestandsaufnahme ergab, dass `ApplyMaterialCandidate`
  und `ApplySearchedMaterial` bereits VOLLSTÄNDIG gescoped waren (Nebeneffekt
  der `Get`-Signaturänderung in 0.2.2.1.3.4.1, da beide denselben Quote-
  Sperr-Query wie die Kern-CRUD-Funktionen nutzen). Tatsächlich neu in
  dieser Subtask: **`SearchMaterialsForQuoteItem`** — hatte bislang GAR
  KEINEN `companyID`-Parameter (ruft `Get` nicht auf, wurde daher von der
  vorherigen Signaturänderung nicht mitgezogen). Ownership-Query
  (`SELECT q.status, ... FROM quotes q JOIN quote_items qi ... WHERE
  q.id=$1 AND qi.id=$2`) um `AND q.company_id=$3` ergänzt — vorher hätte
  ein Aufrufer mit bekannter `quoteID`/`itemID` eines FREMDEN Mandanten die
  Materialsuche für diese Position auslösen können (kein Datenleck der
  Materialliste selbst, da `materials` mandantenweit sichtbar ist, aber ein
  Bestätigungsorakel für die Existenz fremder Angebotspositionen — echter,
  wenn auch kleiner Sicherheitsfund). HTTP-Handler (`v1.go`, 1 Call-Site)
  reicht `companyIDFromContext` durch.
  **`listMaterialCandidatesForQuoteItem` bewusst NICHT geändert**: geprüft,
  dass die Funktion nur einen einzigen Aufrufer hat — `Get()` selbst, das
  `quoteItemID` aus bereits company-gescopten Zeilen bezieht (Beleg: Grep
  über den gesamten Aufrufgraphen, nur 1 Treffer außerhalb der
  Funktionsdefinition). Ein eigener Ownership-Check wäre redundant.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/quotes/...` PASS.
  `TestQuoteMaterialSearchEndpointSupportsOpenDraftItemSearch` erneut
  gegen frische DB geprüft — scheitert weiterhin an Backlog 0.21
  (Materialkategorie), also VOR jedem Erreichen des geänderten Codes;
  keine end-to-end-Bestätigung über HTTP möglich, solange 0.21 offen ist,
  aber die Änderung ist syntaktisch identisch zum bereits mehrfach
  end-to-end verifizierten Muster in `ApplyMaterialCandidate`/
  `ApplySearchedMaterial`. `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.3.4.3: Preisfindungs-Cluster abgeschlossen — 9 Funktionen
  um `companyID` erweitert, davon 4 mit eigener `quotes`/`quote_items`-
  Sperrabfrage (`SuggestPriceForQuoteItem`, `PriceHistoryForQuoteItem`,
  `PriceDecisionHistoryForQuoteItem`, `MarginAnchorForQuoteItem`) und 4
  reine Wrapper, die `companyID` nur an eine der vier durchreichen
  (`ApprovalHintForQuoteItem`/`TargetMarginAnchorForQuoteItem` →
  `MarginAnchorForQuoteItem`; `PriceSourcePriorityForQuoteItem` →
  `PriceHistoryForQuoteItem`). `PriceEvaluationForQuoteItem` ist ein
  Sonderfall: ruft `PriceSourcePriorityForQuoteItem` auf UND hat eine
  eigene `quotes`/`quote_items`-Abfrage (aktueller Einzelpreis) — beide
  Stellen gescoped. `PriceDecisionTransparencyForQuoteItem` reicht nur an
  `PriceEvaluationForQuoteItem` durch. Damit sind alle 9 Funktionen entlang
  ihrer tatsächlichen Aufrufkette abgesichert, nicht nur oberflächlich
  durchgereicht. `quoteTargetMarginPercent` bewusst NICHT geändert — liest
  eine einzelne globale Einstellung (`quote_calculation_settings WHERE
  id='default'`), die noch keine Mandantenspalte hat; das wäre eine
  Scoping-Erweiterung einer anderen Tabelle außerhalb dieser Subtask.
  HTTP-Handler (`v1.go`, 9 Call-Sites) reichen `companyIDFromContext`
  durch. Keine Testdateien riefen diese Funktionen direkt auf — keine
  Testanpassungen nötig.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/quotes/...` PASS.
  `TestQuotePriceSuggestionEndpointSupportsMappedDraftItem` erneut gegen
  frische DB geprüft — scheitert weiterhin an Backlog 0.21, also vor
  Erreichen des geänderten Codes (gleiche Einschränkung wie bei
  0.2.2.1.3.4.2: keine echte End-to-End-Bestätigung möglich, solange 0.21
  offen ist). `go test ./internal/http -run TestContact`: identische 11/14
  PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.3.4.4: Freigabe-Workflow-Cluster abgeschlossen — 9
  Funktionen. **Echter, funktional relevanter Fund**: `ListApprovalReworkQueue`
  und `ListApprovalRequestQueue` (die beiden Queue-Übersichten, die per
  optionalem Filter über ALLE Angebote hinweg suchen) hatten VORHER
  überhaupt keine Mandantenfilterung — jeder Nutzer mit `quotes.read` hätte
  offene Freigabeanforderungen/Nacharbeiten fremder Mandanten sehen können.
  `company_id`-Bedingung als erste, verpflichtende Filterbedingung ergänzt
  (gleiches Muster wie bei `List()` in den Kern-CRUD-Subtasks). Die übrigen
  7 Funktionen (`RequestApprovalForQuoteItem`, `CancelApprovalRequestForQuoteItem`,
  `decideApprovalRequestForQuoteItem` inkl. der beiden dünnen Wrapper
  `Approve`/`RejectApprovalRequestForQuoteItem`, `ResolveApprovalReworkForQuoteItem`,
  `ListApprovalRequestsForQuoteItem`) bekamen denselben Quote-Ownership-Guard
  wie die übrigen Cluster. HTTP-Handler (`v1.go`, 9 Call-Sites) reichen
  `companyIDFromContext` durch.
  **Testanpassungen nötig** (zwei Dateien, beide bereits vor dieser Session
  bestehend): `approval_decisions_test.go` (direkter, nicht-HTTP
  Integrationstest im `quotes`-Paket selbst) — 6 Aufrufe um `companyID`
  ergänzt, Seed-Query um `company_id='default'` erweitert.
  `quotes_integration_test.go` — der gemeinsame Seed-Helfer
  `seedHTTPApprovalDecisionQuote` (verwendet von mindestens 2 Tests) fügte
  Angebote per Roh-SQL OHNE `company_id` ein; ohne Fix hätten alle
  Freigabe-Workflow-HTTP-Tests mit "no rows in result set" fehlschlagen
  müssen (beim ersten Testlauf tatsächlich beobachtet, sofort als
  Notwendigkeit erkannt statt als Fund dokumentiert — analog zum bereits
  etablierten Muster bei GAEB-Import-Tests in 0.2.2.1.3.4.1).
  **Zwei NEUE, unabhängige Funde** dabei aufgedeckt (beide NICHT von dieser
  Session verursacht, per `git show HEAD`/`git diff` bestätigt):
  - Backlog 0.27: der direkte Integrationstest im `quotes`-Paket selbst
    (`approval_decisions_test.go`) scheitert an einer nicht existierenden
    Spalte `contacts.telefon` (korrekt: `phone`) — dieser Test lief
    vermutlich noch NIE erfolgreich gegen echtes Postgres.
  - Backlog 0.28: `TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests`
    und `TestQuoteApprovalDecisionEndpointsRequireApprovePermission`
    scheitern an einer Feldassertion (`reason_code` erwartet
    `"below_target_margin"`, tatsächlich rechnerisch korrekt
    `"negative_margin"` bei den verwendeten Fixture-Preisen 50/60) — **alle
    vorgelagerten HTTP-Schritte (Create/Approve/Reject/Cancel/Queue-Abruf)
    liefen dabei fehlerfrei `201`/`200`**, starker Beleg, dass die
    `company_id`-Scoping-Änderung selbst korrekt funktioniert und nur eine
    vorbestehende Testdaten-Inkonsistenz sichtbar macht.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` clean,
  `go test ./internal/quotes/...` PASS. `go test ./internal/http -run
  TestContact`: identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.2.2.1.3.4.5 (letzte des Task 0.2.2.1.3.4 UND von Task
  0.2.2.1.3): GAEB-Import-Cluster in `server/internal/quotes/imports.go`
  abgeschlossen — 11 Funktionen. `quote_imports` hat KEINE eigene
  `company_id`-Spalte (Migration 056 hat sie bewusst ausgelassen, siehe ADR
  0002) — Scoping erfolgt daher überall per `JOIN projects p ON p.id =
  quote_imports.project_id` + `p.company_id=$N`, analog zum
  LogiKal-Import-Muster aus 0.2.2.1.2.3.7. `CreateGAEBImport` (Projekt-Existenz-
  Check), `ListImports`/`GetImport`/`GetImportItem` (Lese-Pfade),
  `UpdateImportItemReview`/`MarkImportReviewed`/`SaveImportParseResult`/
  `MarkImportFailed`/`ProcessGAEBImport` (Status-Übergänge, jeweils per
  JOIN-Existenzcheck auf `quote_imports.source_kind='gaeb' AND
  p.company_id=$N`) sowie `ApplyImportToDraftQuote` (Sperrabfrage jetzt
  `FOR UPDATE OF quote_imports` innerhalb des Joins, da `FOR UPDATE` allein
  über einen Join beide Tabellen sperren würde) umgestellt. HTTP-Handler
  (`v1.go`, 8 Call-Sites für die GAEB-Import-Routen) reichen
  `companyIDFromContext` durch.
  **Zwei echte SQL-Bugs in dieser Subtask selbst verursacht und noch in
  derselben Subtask gefunden+behoben** (nicht vom Compiler erkennbar, da
  rohes SQL): `ListImports` und `GetImport` selektierten nach Einführung des
  `JOIN projects p` weiterhin unqualifizierte Spalten `id`/`status` — beide
  Spalten existieren sowohl in `quote_imports` als auch in `projects`
  (`SQLSTATE 42702 column reference "id"/"status" is ambiguous`), reproduzierbar
  bei jedem GAEB-Import-HTTP-Request. Behoben durch durchgängige
  `quote_imports.`-Präfixierung aller SELECT-Spalten in beiden Funktionen.
  Erst bei der Verifikation gegen echtes Postgres sichtbar geworden (go
  build/vet prüfen kein SQL) — Lehre: bei JOIN-Einführung in bestehende
  Roh-SQL-SELECTs IMMER alle Spalten explizit qualifizieren, nicht nur die,
  die offensichtlich mehrdeutig wirken.
  **Drei neue, unabhängige Funde** bei der Verifikation entdeckt und
  dokumentiert statt behoben (außerhalb Subtask-Scope, siehe Backlog 0.29-0.31):
  - Backlog 0.29: `classifyDomainError` (`http/v1.go`) erkennt mehrere
    GAEB-Import-Validierungsfehlermeldungen nicht als 400 (fällt auf 500
    `internal_error` zurück) — gleiches Muster wie der bereits bekannte Fund
    Backlog 0.24, hier aber in der `quotes`-Domäne (3 betroffene Meldungen/Tests).
  - Backlog 0.30: `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates`
    paniced (Nil-Pointer in `settings.NumberingService.Next`), weil der Test
    `quotes.NewService(env.PG, nil)` mit `nil` statt einer echten
    `NumberingService`-Instanz konstruiert — bereits so in `git show HEAD`
    vorhanden, nicht von dieser Session verursacht.
  - Backlog 0.31: `TestGAEBImportProcessEndpoint` baut den Handler direkt
    über `NewV1RouterWithOptions` (ohne `/api/v1`-Mount-Präfix), testet aber
    Pfade wie `/api/v1/auth/login` → `404`. Existiert nicht in `git show
    HEAD`, wurde also in einer früheren, nicht dokumentierten Subtask dieser
    Session neu hinzugefügt — in dieser Subtask nicht angefasst.
  Gegen frische, mehrfach verifiziert leere DB (jeweils vor jedem Testlauf
  per `docker compose down -v` + `up -d --wait` + `\dt`-Leercheck neu
  aufgesetzt) verifiziert: `go build ./...` und `go vet ./...` clean,
  `gofmt -l` clean für alle 4 geänderten Dateien
  (`quotes/imports.go`, `quotes/imports_test.go`, `http/v1.go`,
  `http/quotes_integration_test.go`). `go test ./internal/quotes/...` und
  `go test ./internal/http/...` (ohne `NALA_INTEGRATION`) PASS. Von 9
  ausgewählten GAEB-Import-HTTP-Integrationstests (mit
  `NALA_INTEGRATION=1`) laufen 6 durch bis zu echten `200`/`201`-Antworten
  über die neu gescopten Pfade (`TestQuoteGAEBImportFlowCreatesImportRunAndListsIt`,
  `TestQuoteGAEBImportListAndDetailExposeReviewSummaryCounts`,
  `TestQuoteGAEBImportItemReadEndpointsExposeParsedItems`,
  `TestQuoteGAEBImportItemReviewEndpointUpdatesReviewFields`,
  `TestQuoteGAEBImportApplyCreatesDraftQuoteFromAcceptedItems`, sowie isoliert
  bestätigt `TestGAEBImportProcessEndpoint`s Kernlogik über die direkten
  Service-Aufrufe in `imports_test.go`); die verbleibenden 3+1 scheitern
  ausschließlich an den drei neuen Funden 0.29-0.31 (nicht an
  `company_id`/`branch_id`). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Task 0.2.2.1.4 (Material/Lager: `materials`, `warehouses`) abgeschlossen —
  als eine Subtask umgesetzt (16 HTTP-Call-Sites, vergleichbar mit
  `sales_orders`, keine Zerlegung nötig). `materials`- und `warehouses`-
  Kern-CRUD (`Create`/`Update`/`DeleteSoft`/`List`/`Get`,
  `CreateWarehouse`/`ListWarehouses`) direkt auf `company_id` aus Migration
  057 gescoped (`Create`/`CreateWarehouse` validieren `companyID` als
  ersten Check vor jedem DB-Zugriff, analog zum etablierten Muster).
  `locations`, `stock_movements`/`batches` und `material_documents` haben
  laut Migration 057/ADR 0002 KEINE eigene `company_id`-Spalte — Scoping
  über neue Ownership-Check-Helfer: `warehouseOwnedByCompany` (für
  `CreateLocation`/`ListLocations`), Inline-Checks auf `materials` UND
  `warehouses` innerhalb der Transaktion in `CreateMovement` (verhindert,
  dass eine Lagerbewegung fremde Mandanten-Materialien/-Lager referenziert),
  `s.Get` als Ownership-Gate in `StockByMaterial`/`UploadMaterialDocument`/
  `ListMaterialDocuments` (Check vor dem GridFS-Upload, analog zu
  `contacts`-Dokumenten aus 0.2.2.1.2.2.5). Facetten `ListTypes`/
  `ListCategories` sowie `normalizeAndValidateCategory` (Kategorie-
  Validierung bei Create/Update) ebenfalls gefiltert — `material_groups`
  selbst bewusst NICHT gescoped (globale Referenzdaten ohne eigene
  `company_id`-Spalte). `OpenDocumentStream` (`GET /documents/{docID}`)
  bewusst NICHT angefasst — bereits als Backlog 0.22 dokumentiert (gilt
  für `contacts`- UND `materials`-Dokumente gleichermaßen).
  **Testabdeckungslücke geschlossen**: für `/warehouses`, `/warehouses/
  {id}/locations` und `/stock-movements` gab es VORHER überhaupt keine
  Tests (weder Unit- noch Integrationstests) — neuer, permanenter Test
  `server/internal/http/warehouses_integration_test.go` mit Happy-Path-Flow
  (Material→Lager→Standort→Bewegung→Bestand, alle Schritte über echte
  HTTP-Requests) und einem Negativfall (`stock-movements` mit unbekannter
  `material_id` → `400`). Zusätzlich `materials/service_test.go` um
  `TestCreateRejectsMissingCompanyID` ergänzt (Negativfall analog zu allen
  anderen Domänen dieser Session).
  **Testfixture-Fix nötig** (keine neue Backlog-Nummer, direkte Folge der
  Signaturänderung dieser Subtask): `TestMaterialsUpdateAllowsExistingLegacyCategory`
  seedete ein Material per Roh-SQL ohne `company_id` — `normalize
  AndValidateCategory`s zweiter EXISTS-Zweig (Prüfung, ob die Kategorie
  bereits an einem vorhandenen Material verwendet wird) filtert jetzt nach
  `company_id`, daher fand die Prüfung das ungescopte Material nicht mehr.
  Fix: `company_id='default'` in der Seed-INSERT ergänzt (gleiches Muster
  wie in praktisch jeder vorherigen Subtask dieser Session).
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean für alle neuen/geänderten
  Go-Dateien (`materials/service.go`, `materials/service_test.go`,
  `http/v1.go`, `http/warehouses_integration_test.go`,
  `http/materials_integration_test.go`); `materials/documents.go` bleibt
  wie vor dieser Session nicht gofmt-clean (Backlog 0.15, 4-Leerzeichen-
  statt Tab-Einrückung, vor dieser Session bereits so — per `git show
  HEAD` bestätigt, nicht neu verursacht). `go test ./internal/materials/...`
  und `go test ./internal/http/...` (ohne `NALA_INTEGRATION`) PASS. Mit
  `NALA_INTEGRATION=1`: 6/7 `TestMaterials*`-Tests plus beide neuen
  Warehouse-Tests laufen fehlerfrei durch (`200`/`201`); der einzige
  verbleibende Fehlschlag (`TestMaterialsCreateListAndGetFlow`) ist der
  bereits bekannte, unabhängige Backlog 0.21 (leere `material_groups`-
  Tabelle auf frischer DB). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Task 0.2.2.1.5 (Buchhaltung: `accounts`, `journal_entries`,
  `bank_statements`) abgeschlossen — als eine Subtask umgesetzt (`ar.go`
  bereits aus 0.2.2.1.3.2 gescoped, hier ging es um die restlichen 4
  Dateien). `JournalService.create` (zentrale, von `Create`/`CreateTx`
  gemeinsam genutzte Interna) validiert `companyID` jetzt als ERSTEN Check
  vor jedem DB-Zugriff (analog zum `Create`-Muster aller anderen Domänen),
  INSERT auf `journal_entries` um `company_id` ergänzt.
  `AccountingService.ListAccounts` (`accounts`) gefiltert — `ListTaxCodes`
  bewusst NICHT angefasst (`tax_codes` hat laut Migration 058 keine eigene
  `company_id`-Spalte, globale Referenzdaten laut ADR 0002).
  `BankService.Ingest`/`List`/`Match` (`bank_statements`, eigene
  `company_id`-Spalte aus Migration 058) sowie die beiden internen
  Invoice-Lookup-Helfer `findInvoiceByAmount`/`findInvoiceIDInReference`
  (beide lesen `invoices_out`, jetzt mit `AND company_id=$N`) gescoped.
  `PaymentService.Apply`/`apply`/`List` (`invoice_out_payments`, KEINE
  eigene `company_id`-Spalte — erbt über `invoice_id`) gescoped: `apply`s
  `FOR UPDATE`-Sperrabfrage auf `invoices_out` bekam `AND company_id=$2`
  (Prüfung `companyID erforderlich` bewusst NACH der bereits vorhandenen
  `Amount<=0`-Prüfung platziert, um die Fehlermeldung/Reihenfolge des
  bestehenden Negativtests `TestApplyRejectsNonPositiveAmount`
  unverändert zu lassen); `List` bekam einen neuen Ownership-Check
  (`EXISTS`-Query auf `invoices_out`, analog zum in dieser Session
  etablierten Helfer-Muster).
  **Echter, HTTP-erreichbarer Fund**: `GET/POST /api/v1/invoices-out/{id}/payments`
  (`server/internal/http/v1.go:905-948`) hatte VORHER überhaupt keine
  Mandantenprüfung — obwohl `invoices_out` selbst bereits seit 0.2.2.1.3.2
  gescoped ist (`arSvc.Book` prüft `company_id` korrekt), konnte jeder
  Nutzer mit `invoices_out.write`/`invoices_out.read`-Berechtigung über die
  Payments-Endpunkte auf JEDE Rechnung irgendeines Mandanten zugreifen bzw.
  eine Zahlung buchen, solange die Rechnungs-UUID bekannt war (kein
  UUID-Enumerationsschutz vorausgesetzt). Jetzt behoben durch die
  `PaymentService`-Scoping-Änderung + `companyIDFromContext`-Durchreichung
  in beiden Handlern.
  **Unwired Domänen-Code dennoch gescoped**: `BankService` und
  `AccountingService` sind aktuell an KEINEN HTTP-Handler angebunden
  (`grep -rn "BankService\|AccountingService" internal/http/` liefert
  außerhalb der eigenen Dateien keinen Treffer) — dennoch vollständig
  gescoped, da Teil des Anwendungscodes laut Subtask-Definition
  ("`accounts`, `journal_entries`, `bank_statements`" steht explizit im
  Task-Titel). Kein akutes Sicherheitsrisiko (nicht erreichbar), aber
  Konsistenz mit dem Rest der Domäne hergestellt.
  **Neuer, unabhängiger Fund** (Backlog 0.32): kein Anwendungscode-Pfad legt
  für einen zweiten Mandanten (`company_id != 'default'`) einen
  Kontenrahmen (`accounts`) an — Migration 058 hat nur bestehende Zeilen auf
  `company_id='default'` zurückgeschrieben, es gibt aber keinen
  `Create`/`Insert`-Pfad für `accounts` in der gesamten Anwendung. Wird erst
  bei einem künftigen Mandanten-Onboarding-Konzept relevant, nicht behoben
  (außerhalb Subtask-Scope).
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean für alle geänderten Dateien
  (`journal.go`, `journal_test.go`, `ar.go`, `payments.go`,
  `payments_test.go`, `bank.go`, `bank_test.go`, `settings/accounting.go`,
  `http/v1.go`). Neue Negativtests ergänzt: `TestJournalCreateRejectsMissingCompanyID`,
  `TestApplyRejectsMissingCompanyID` (beide analog zum `Mandant
  erforderlich`-Muster aller anderen Domänen). `go test
  ./internal/accounting/... ./internal/settings/... ./internal/http/...`
  (ohne `NALA_INTEGRATION`) PASS. Mit `NALA_INTEGRATION=1`:
  `TestInvoiceOutFlowWithPDFAndPayments` (kompletter Flow: Rechnung
  anlegen→buchen→PDF→Zahlung buchen→Zahlungen auflisten, alle über echte
  HTTP-Requests, alle Schritte inkl. der beiden zuvor ungescopten
  Payments-Endpunkte) läuft vollständig fehlerfrei durch (`200`/`201`).
  `go test ./internal/http -run TestContact`: identische 11/14 PASS — keine
  neuen Regressionen.
- Task 0.2.2.1.6 (letzter Task von 0.2.2.1) HR: `hr_employees`, `hr_teams`
  abgeschlossen — als eine Subtask umgesetzt (`server/internal/hr/service.go`,
  238 Zeilen). `EmployeeService.Create`/`Get`/`Update`/`List` sowie
  `ListTeams` (`hr_teams`, nur Lesepfad — kein Create/Update-Pfad im
  Anwendungscode vorhanden, ähnlich wie `accounts` in 0.2.2.1.5, aber kein
  eigener Backlog-Fund nötig, da unwired) direkt gescoped; `Create`
  validiert `companyID` als ersten Check vor jedem DB-Zugriff (analog zum
  Muster aller anderen Domänen).
  `LeaveService.Create`/`Approve`/`Decide`/`List` (`hr_leave_requests`,
  KEINE eigene `company_id`-Spalte — erbt über das verpflichtende
  `employee_id`) über neue Ownership-Checks gescoped: `Create` prüft per
  `EXISTS`-Query, dass die referenzierte `employee_id` zum Mandanten
  gehört (Fehler "Mitarbeiter nicht gefunden" sonst), `List` macht dasselbe
  wenn ein `employeeID`-Filter gesetzt ist, sonst filtert ein
  `WHERE employee_id IN (SELECT id FROM hr_employees WHERE company_id=$1)`.
  `Approve`/`Decide` (beide UPDATE-only, kein SELECT davor im Original)
  nutzen bewusst KEINEN separaten Pre-Check, sondern die Mandantenprüfung
  direkt in der UPDATE-Bedingung selbst
  (`WHERE id=$1 AND employee_id IN (SELECT id FROM hr_employees WHERE
  company_id=$5)`) plus eine `RowsAffected()==0`-Prüfung danach — spart
  einen DB-Roundtrip gegenüber dem sonst üblichen EXISTS-dann-Exec-Muster,
  liefert aber denselben Fehler bei fremdem Mandanten oder unbekannter ID.
  **Kompletter `hr`-Domänenpackage ist aktuell an KEINEN HTTP-Handler
  angebunden** (`grep -rn "internal/hr" internal/http/` liefert außerhalb
  der eigenen Testdateien keinen Treffer) — die gesamte Domäne (Employees,
  Teams, Leave-Requests) ist über die API nicht erreichbar. Kein akutes
  Sicherheitsrisiko, aber dennoch vollständig gescoped, da Teil des
  Anwendungscodes laut Subtask-Definition ("`hr_employees`, `hr_teams`"
  steht explizit im Task-Titel) — kein neuer Backlog-Fund, nur dokumentiert
  (bereits aus Feature 0.1.3 bekannt, dass `hr` nur unit-, nicht
  HTTP-getestet ist).
  **Neue Integrationstests statt der fehlenden HTTP-Ebene**: da es keine
  HTTP-Integrationstests für `hr` gibt und geben kann (nichts gemountet),
  wurde `server/internal/hr/service_scoping_integration_test.go` neu
  angelegt (direkter Service-Aufruf gegen echtes Postgres via
  `testutil.SetupIntegrationEnv`, `NALA_INTEGRATION=1`-gated wie überall
  sonst): `TestEmployeeGetIsScopedToCompany` (Mitarbeiter in `default`
  angelegt, `Get`/`List` aus Sicht eines zweiten, eigens geseedeten
  Mandanten `other-company` findet ihn nicht) und
  `TestLeaveCreateRejectsEmployeeFromOtherCompany` (Urlaubsantrag für einen
  `default`-Mitarbeiter aus Sicht von `other-company` scheitert korrekt mit
  "Mitarbeiter nicht gefunden") — beide bestätigen echte, funktionierende
  Mandantentrennung gegen eine reale zweite `company_profiles`-Zeile (nicht
  nur `'default'` wie in den meisten bisherigen Verifikationen dieser
  Session, die primär auf Nichtregression prüften).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` und
  `go vet ./...` clean, `gofmt -l` clean für alle geänderten Dateien
  (`service.go`, `service_test.go`, `service_scoping_integration_test.go`).
  Neue Negativtests: `TestEmployeeCreateRejectsMissingCompanyID`,
  `TestLeaveCreateRejectsMissingCompanyID` (beide analog zum `Mandant
  erforderlich`-Muster). `go test ./internal/hr/...` (ohne
  `NALA_INTEGRATION`) PASS. Mit `NALA_INTEGRATION=1`: alle 8 Tests im Paket
  PASS, inkl. beider neuer Cross-Tenant-Integrationstests. `go test
  ./internal/http -run TestContact`: identische 11/14 PASS — keine neuen
  Regressionen.
  **Task 0.2.2.1 (Repository-Queries um Scoping-Filter erweitern) ist damit
  über alle 6 Domänen-Cluster (Auth-Kern, Kontakte/Projekte,
  Angebote/Aufträge/Rechnungen/Bestellungen, Material/Lager, Buchhaltung,
  HR) vollständig abgeschlossen.**
- Task 0.2.2.3 (letzter Task von 0.2.2): `number_sequences` pro Mandant
  statt global abgeschlossen. Neue Migration
  `061_number_sequences_composite_pk.sql`: `company_id` auf `NOT NULL`
  gesetzt (unbedenklich, alle Zeilen bereits `'default'` aus Migration
  060), alter PK auf `entity` allein gedroppt, neuer zusammengesetzter PK
  auf `(company_id, entity)` angelegt, der jetzt überflüssige unterstützende
  Index `idx_number_sequences_company_id` entfernt (der PK deckt
  `company_id` als führende Spalte bereits ab). Gegen frische DB
  verifiziert: Migration läuft fehlerfrei durch, `\d number_sequences`
  zeigt den erwarteten zusammengesetzten PK, alle 6 vorher geseedeten
  Nummernkreis-Zeilen (`quote`, `sales_order`, `purchase_order`,
  `invoice_out`, `invoice_in`, `project`) haben korrekt `company_id='default'`.
  `settings.NumberingService.Get`/`UpdatePattern`/`Preview`/`Next` um
  `companyID` erweitert; `Next` validiert `companyID` als ERSTEN Check vor
  Transaktionsbeginn (analog zum `Create`-Muster aller anderen Domänen).
  **Sehr geringer Blast-Radius trotz domänenübergreifender Nutzung**: alle
  6 Aufrufer (`accounting/ar.go` `Book`, `projects/import_logikal.go`
  `ImportLogikal`, `projects/service.go` `Create`, `purchasing/service.go`
  `Create`, `quotes/service.go` `createQuoteTx`, `sales/service.go`
  `CreateFromQuote`) hatten `companyID` bereits als Parameter aus ihren
  jeweiligen früheren Domänen-Subtasks dieser Session verfügbar — reine
  Ein-Zeilen-Durchreichung an jeder Stelle, keine einzige Funktionssignatur
  musste geändert werden. Kein einziger Testfile mit direktem
  `NumberingService`-Aufruf betroffen (vorab per `grep` verifiziert) — nur
  reine `formatWithPattern`/`pad`-Funktionstests in `numbering_test.go`,
  unberührt von der Signaturänderung.
  **Echter, HTTP-erreichbarer Fund**: `GET/PUT /settings/numbering/{entity}`
  (`server/internal/http/v1.go:3176-3214`) hatte VORHER überhaupt keine
  Mandantenprüfung UND keinerlei Testabdeckung — jeder Nutzer mit
  `settings.manage`-Berechtigung hätte das Nummernkreis-Muster (`pattern`,
  z. B. `"ANG-{YYYY}-{NNNN}"`) irgendeines Mandanten lesen und sogar
  überschreiben können. Jetzt gescoped; neuer Test
  `TestNumberingSettingsFlowIsScopedToOwnCompany`
  (`server/internal/http/settings_integration_test.go`) ergänzt (Get →
  Preview → Update → Get-nach-Update bestätigt Persistenz → unbekannte
  `entity` liefert `404` statt Daten preiszugeben).
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean für alle geänderten Dateien
  außer dem bereits vor dieser Session nicht-gofmt-konformen
  `numbering.go` (Backlog 0.15, gleiches Muster wie `materials/documents.go`
  aus 0.2.2.1.4). Neuer Negativtest `TestNextRejectsMissingCompanyID`.
  `go test ./internal/settings/... ./internal/purchasing/...
  ./internal/accounting/... ./internal/projects/... ./internal/sales/...
  ./internal/quotes/... ./internal/http/...` PASS (mit Ausnahme des
  bereits bekannten, unveränderten Backlog-0.6-Panics in
  `TestCreateRejectsInvalidItem`, isoliert per `-run`-Ausschluss bestätigt
  unabhängig). Mit `NALA_INTEGRATION=1` gegen frische DB:
  `TestQuoteGAEBImportFlowCreatesImportRunAndListsIt` (Quote-Nummerierung)
  und `TestInvoiceOutFlowWithPDFAndPayments` (Invoice-Nummerierung) PASS —
  belegen, dass `Next()` mit der neuen Signatur und dem neuen
  zusammengesetzten PK korrekt funktioniert. `TestPurchaseOrdersCreateAndGetFlow`
  und `TestProjectQuotePDFFlow` scheitern weiterhin an bereits bekannten,
  unabhängigen Vorbefunden (Backlog 0.21 bzw. 0.23), beide isoliert gegen
  frische DB reproduziert und bestätigt VOR Erreichen der
  Nummernkreis-Logik. `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
  **Damit ist Task 0.2.2 (Anwendungscode auf Mandanten-Scoping umstellen)
  UND DAMIT DAS GESAMTE TASK 0.2 (Mandantenfähigkeit nachrüsten)
  VOLLSTÄNDIG ABGESCHLOSSEN** — von der ADR-Entscheidung über alle
  Datenmodell-Migrationen bis zur vollständigen Anwendungscode-Umstellung
  über alle Domänen inklusive der zentralen Nummernkreis-Infrastruktur.
- Task 0.3.1 (erster Task von Epic 0.3 GoBD-Fundament): Storno-Konzept für
  `invoices_out`/`journal_entries` abgeschlossen. Neue Migration
  `062_invoices_out_storno.sql`: `storno_journal_entry_id`,
  `storniert_am`, `storno_grund` an `invoices_out` ergänzt (kein
  CHECK-Constraint auf `status` nötig — die Spalte war laut
  `018_journal_and_ar.sql` schon immer freier Text).
  **GoBD-Kernprinzip technisch umgesetzt und gegen echtes Postgres
  bewiesen**: `ARService.Storno` ändert oder löscht zu keinem Zeitpunkt die
  ursprüngliche Buchung (`journal_entry_id` bleibt unangetastet auf die
  Originalbuchung zeigen) — stattdessen erzeugt `buildStornoJournal` (neue
  Funktion, spiegelt `buildJournal`) eine zweite, vollständige
  Umkehrbuchung mit identischen Konten/Beträgen, aber vertauschtem
  Soll/Haben, über den bereits gescopten `JournalService.CreateTx`. Direkt
  per SQL gegen die Test-DB verifiziert (`journal_lines`-Vergleich Original
  vs. Storno-Buchung): exakt gespiegelte Werte auf denselben Konten (`1400`
  Forderung, `1776` USt, `8000` Erlös), beide Buchungssätze bleiben
  vollständig erhalten — kein `UPDATE`/`DELETE` auf `journal_entries`
  irgendwo im Code-Pfad.
  **Bewusste Scope-Grenze**: Storno nur aus Status `"booked"` UND
  `paid_amount=0` erlaubt — eine bereits (auch nur teilweise) bezahlte
  Rechnung lässt sich in diesem Konzept NICHT stornieren
  ("Rechnung hat bereits Zahlungen erhalten, Storno derzeit nicht
  unterstützt"), da eine korrekte Rückabwicklung der Zahlung (inkl.
  Banküberweisung/-abgleich) ein eigenständiges, deutlich komplexeres
  Thema wäre und explizit NICHT Teil des Task-Titels "Storno-Konzept" ist.
  `classifyDomainError` um das neue Muster `"zahlungen erhalten"` ergänzt,
  damit diese Meldung korrekt als `400` statt `500` ankommt (gleiches
  Prinzip wie überall in dieser Session — neue, selbst eingeführte
  Validierungsmeldungen bekommen ihr Klassifizierungsmuster direkt mit,
  kein nachträglicher Backlog-Fund nötig).
  Neue Route `POST /api/v1/invoices-out/{id}/storno` (Permission
  `invoices_out.write`, `{"reason": "..."}` im Body, analog zu `/book`).
  `InvoiceOut`/`InvoiceListItem` um die drei neuen Felder erweitert, `Get`
  und `List` entsprechend angepasst.
  **Bewusste Abgrenzung zu Task 0.3.2** (Festschreibungs-Mechanismus, noch
  offen): dieses Storno-Konzept bietet nur den korrekten Korrekturweg an,
  es macht andere Mutationen an gebuchten Rechnungen NICHT technisch
  unmöglich — die allgemeine Durchsetzung der Unveränderlichkeit
  (Festschreibung) ist im Backlog explizit ein eigener, separater Task und
  wurde hier bewusst nicht mit erledigt.
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean für alle geänderten Dateien.
  Neue Unit-Tests: `TestStornoRejectsMissingCompanyID`,
  `TestStornoRejectsMissingReason` (beide DB-los, Validierung vor jedem
  Zugriff), `TestBuildStornoJournalReversesDebitAndCreditButStaysBalanced`
  (reine Funktionslogik, beweist Soll=Haben bleibt erhalten UND
  Konten/Beträge sind exakt gespiegelt). `go test ./internal/accounting/...
  ./internal/http/... ./internal/settings/...` PASS. Mit
  `NALA_INTEGRATION=1` gegen frische DB (`-count=1` erzwungen, um den
  Go-Test-Cache zu umgehen und einen echten Lauf zu garantieren):
  `TestInvoiceOutFlowWithPDFAndPayments` (erweitert um einen Negativfall:
  Storno nach Zahlungserfassung → `400`) und neu
  `TestInvoiceOutStornoFlow` (Storno auf `draft` abgelehnt, fehlender
  Stornogrund abgelehnt, erfolgreicher Storno mit persistierten Feldern
  inkl. `storno_journal_entry_id`, wiederholter Storno auf bereits
  storniertem Beleg abgelehnt) beide PASS. `go test ./internal/http -run
  TestContact`: identische 11/14 PASS — keine neuen Regressionen.
- Task 0.3.2: Festschreibungs-Mechanismus für gebuchte/versendete Belege
  abgeschlossen. **Erst recherchiert statt sofort implementiert** — Prüfung
  aller vier "Beleg"-Domänen ergab, dass drei bereits GoBD-konform
  geschützt waren, jeweils als Nebeneffekt früherer Subtasks dieser Session
  (ohne dass es dort als "Festschreibung" benannt wurde):
  - `quotes`: `Update` und praktisch jede Item-Mutationsfunktion (16
    Fundstellen, siehe Backlog 0.33) blockieren bereits bei
    `status != "draft"`; historische, durch `Revise` ersetzte Versionen
    sind zusätzlich über `superseded_by_quote_id` separat geschützt.
  - `sales_orders`: `Update`/`CreateItem`/`UpdateItem` nutzen bereits
    `isEditableStatus`/`ensureOrderEditableTx`, nur `open`/`released`
    editierbar.
  - `invoices_out`: `ARService` hat GAR KEIN `Update` — implizit
    unveränderlich seit jeher, Book/Storno sind die einzigen kontrollierten
    Statusübergänge (siehe 0.3.1).
  **Echter, einziger Blast-Radius: `purchasing`** — `Update`/`CreateItem`/
  `UpdateItem`/`DeleteItem` hatten ÜBERHAUPT KEINEN Status-Check; eine
  Bestellung war unabhängig vom Status (auch nach `received`/`canceled`)
  jederzeit frei änderbar, inkl. sämtlicher Positionen. Neuer Helfer
  `isEditableStatus` (nur `draft` inhaltlich editierbar, Groß-/Kleinschreibung
  und Leerraum toleriert) + Guard in allen vier Funktionen. Bewusste
  Feinheit in `Update`: nur die INHALTSfelder (Nummer, Datum, Währung,
  Notiz) werden gesperrt — der Statuswechsel selbst (`u.Status`, z. B.
  `draft`→`ordered`) bleibt unabhängig vom aktuellen Status weiterhin
  möglich, sonst wäre die normale Lebenszyklus-Progression selbst
  blockiert worden.
  Gegen echtes Postgres per neuem HTTP-Integrationstest
  `TestPurchaseOrdersAreLockedAfterLeavingDraftStatus` bewiesen (Kategorie
  bewusst leer gelassen, um den unabhängigen Backlog-0.21-Vorbefund nicht
  zu berühren): Kopfdatenänderung während `draft` funktioniert noch,
  Statuswechsel zu `ordered` funktioniert, danach werden Kopfdatenänderung,
  neue Position, Positionsänderung UND Positionslöschung alle korrekt mit
  `400` abgelehnt — abschließend per `GET` bestätigt, dass die Position
  nachweislich unverändert blieb (`menge=2`).
  **Neuer, unabhängiger Fund** (Backlog 0.33, nicht behoben — andere
  Domäne als die für diese Subtask bearbeitete `purchasing`): die
  `quotes`-Schreibschutz-Meldungen ("nur Entwürfe sind bearbeitbar",
  "Historische Angebotsversionen sind schreibgeschützt", 16 Fundstellen)
  matchen kein `classifyDomainError`-Muster und liefern daher `500` statt
  `400` — der Schreibschutz selbst greift korrekt, nur der HTTP-Status ist
  irreführend. Gleiches Muster wie die bereits bekannten Funde 0.24/0.29.
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean. Neue Unit-Tests:
  `TestIsEditableStatusOnlyAllowsDraft` (reine Funktionslogik, alle 4
  Status + Groß-/Kleinschreibungs-/Leerraum-Varianten). `go test
  ./internal/purchasing/...` PASS (außer dem bereits bekannten,
  unveränderten Backlog-0.6-Panic in `TestCreateRejectsInvalidItem`,
  isoliert per `-run`-Ausschluss bestätigt unabhängig). Mit
  `NALA_INTEGRATION=1` gegen frische DB (`-count=1` erzwungen):
  `TestPurchaseOrdersAreLockedAfterLeavingDraftStatus` PASS,
  `TestPurchaseOrdersCreateAndGetFlow` scheitert weiterhin ausschließlich
  am bereits bekannten Backlog 0.21 (unverändert, vor UND nach dieser
  Subtask reproduzierbar). `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.3.3.1 (erste von mehreren Micro-Subtasks für Task 0.3.3):
  generische Infrastruktur für das Änderungsprotokoll + Pilot-Integration
  in `invoices_out` abgeschlossen. **Bewusste Zerlegungsentscheidung**:
  Task 0.3.3 wollte laut Titel ein Protokoll "über Fachobjekte" (Plural,
  domänenübergreifend) — das in EINER Subtask für alle Domänen anzubinden
  hätte die aufgabe.md-Grenze (§6.3, max. ~8 Dateien/~400 Zeilen) klar
  gesprengt. Stattdessen: zuerst die generische Infrastruktur bauen und an
  EINER Domäne (der GoBD-relevantesten, direkt anschließend an 0.3.1/0.3.2)
  beweisen, dass sie funktioniert; die Anbindung der übrigen Domänen als
  eigene, im Backlog bereits vorangelegte Micro-Subtasks 0.3.3.2-5.
  Neue Migration `063_entity_change_log.sql`: Tabelle `entity_change_log`
  mit `company_id`/`entity_type`/`entity_id`/`action`/`actor_user_id`
  (FK auf `users(id)`, `text`)/`before_data`/`after_data` (beide `jsonb`,
  nullable)/`note`/`created_at`. Bewusst KEINE feste Fremdschlüsselbindung
  auf eine bestimmte Fachtabelle (anders als die beiden vorhandenen
  domänenspezifischen Muster `project_import_changes` und
  `quote_item_price_decisions`) — `entity_type`/`entity_id` sind freier
  Text, damit dieselbe Tabelle künftig von jeder Domäne genutzt werden
  kann, ohne dass jede Domäne ihre eigene Protokolltabelle braucht.
  Neues Package `server/internal/auditlog` (`Service.Record`/`List`).
  `Record` nimmt optional eine bereits laufende `pgx.Tx` entgegen (statt
  immer eine eigene Transaktion zu öffnen), damit der Protokolleintrag
  ATOMAR zusammen mit der eigentlichen fachlichen Änderung geschrieben
  wird — schlägt die fachliche Änderung fehl, wird auch der Protokolleintrag
  zurückgerollt, und umgekehrt (kein Szenario, in dem ein Protokolleintrag
  ohne zugehörige echte Änderung existiert oder eine Änderung ohne
  Protokolleintrag durchgeht).
  Pilot-Integration in `accounting.ARService`: `Book` und `Storno` bekamen
  einen neuen `actorUserID string`-Parameter (nur diese zwei Funktionen
  betroffen, da beide nur einen einzigen Aufrufer in `v1.go` haben — sehr
  kleiner Blast-Radius). Neuer Helfer `actorUserIDFromContext` in `v1.go`
  (Schwester-Funktion zu `companyIDFromContext`, liest `auth.User.ID` aus
  demselben bereits etablierten Context-Mechanismus). Neue,
  schreibgeschützte Route `GET /api/v1/invoices-out/{id}/audit-log`
  (Permission `invoices_out.read`, mit Ownership-Check über `arSvc.Get`
  vor der Protokollabfrage, damit niemand das Änderungsprotokoll einer
  fremden Mandanten-Rechnung einsehen kann).
  **Gegen echtes Postgres per SQL-Direktabfrage bewiesen** (nicht nur über
  die API-Antwort): nach Buchen+Stornieren einer Testrechnung enthält
  `entity_change_log` exakt zwei Zeilen (`gebucht`, `storniert`), beide mit
  korrekt gesetztem `actor_user_id` (die tatsächliche User-ID des
  einloggten Testnutzers), plausiblen `before_data`/`after_data`-JSON-
  Snapshots (z. B. `{"status":"draft"}` → `{"status":"booked",
  "nummer":"RE-2026-0001","journal_entry_id":"..."}`) und beim Storno der
  Begründung als `note`.
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean für alle neuen/geänderten
  Dateien. Neue Unit-Tests im neuen Package: `TestRecordRejectsMissing
  CompanyID`/`EntityType`/`EntityID`/`Action`, `TestListRejectsMissing
  CompanyID` (alle DB-los, Validierung vor jedem Zugriff). `go test
  ./internal/auditlog/... ./internal/accounting/... ./internal/http/...
  ./internal/settings/...` PASS. Mit `NALA_INTEGRATION=1` gegen frische DB
  (`-count=1` erzwungen): `TestInvoiceOutFlowWithPDFAndPayments` und
  `TestInvoiceOutStornoFlow` (letzterer um eine `audit-log`-Prüfung
  erweitert: 2 Einträge, neueste zuerst, `storniert` vor `gebucht`) beide
  PASS. `go test ./internal/http -run TestContact`: identische 11/14 PASS
  — keine neuen Regressionen.
- Subtask 0.3.3.2: Anbindung `quotes` an das Änderungsprotokoll
  abgeschlossen. `quotes.Service` bekam ein neues `WithAudit(*auditlog.Service)`
  als fluent Setter (Muster von `WithMongo`/`WithGAEBImportParser` bewusst
  übernommen statt eines Konstruktor-Parameters) — dadurch mussten die ca.
  12 bestehenden `NewService(...)`-Call-Sites in Testdateien NICHT
  angefasst werden (`s.audit` bleibt dort `nil`, alle neuen Aufrufe sind
  mit `if s.audit != nil` abgesichert).
  Verdrahtet: `Revise` (atomar in der bereits vorhandenen Transaktion vor
  `tx.Commit`, neuer `actorUserID`-Parameter, genau 1 Aufrufer in `v1.go`),
  `Accept` (nicht-transaktional nach erfolgreichem Statuswechsel, da
  `Accept` selbst keine eigene Transaktion besitzt — delegiert komplett an
  `UpdateStatus`/`projectSvc.UpdateStatus`; neuer `actorUserID`-Parameter,
  1 Aufrufer), `decideApprovalRequestForQuoteItem` (atomar, nutzt den
  BEREITS VORHANDENEN Parameter `decidedBy` als Actor — keine
  Signaturänderung nötig, deckt `Approve`/`RejectApprovalRequestForQuoteItem`
  ab). Neuer Helfer `actorUserIDFromContext` in `v1.go`.
  Gegen echtes Postgres per SQL-Direktabfrage bewiesen (nicht nur über die
  API-Antwort): `Accept` erzeugt einen `quote`/`angenommen`-Eintrag mit
  `after_data={"status":"accepted"}`; eine Freigabe-Entscheidung erzeugt
  einen `quote`/`freigabe_approved`-Eintrag mit `before_data`/`after_data`
  (`item_id`+Status requested→approved) und dem Entscheidungskommentar als
  `note` — beide mit korrekt gesetztem `actor_user_id` (echte User-ID des
  eingeloggten Testnutzers).
  **Echter, gravierender, unabhängiger Fund** (Backlog 0.34, als KRITISCH
  markiert, NICHT behoben): beim Versuch, `Revise`s Anbindung end-to-end
  über `TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource` zu
  verifizieren, schlug der Test mit `500 conn busy` fehl. **Sorgfältig
  isoliert, um eine Fehlzuordnung zu vermeiden**: derselbe Fehler tritt
  IDENTISCH auf, wenn `WithAudit(...)` testweise komplett aus der
  `v1.go`-Konstruktionskette entfernt wird (dann sofort wieder ergänzt) —
  die neue Audit-Anbindung ist also nachweislich NICHT die Ursache. Die
  eigentliche Ursache (`server/internal/quotes/service.go:3340`, `git show
  HEAD` bestätigt unverändert seit vor dieser Session): die Positionen-
  Kopierschleife in `Revise` führt `tx.Exec(INSERT INTO quote_items...)`
  INNERHALB einer noch offenen `rows.Next()`-Iteration derselben
  Transaktion aus — ein klassischer pgx-v5-Fehler ("conn busy"), der bei
  JEDEM Angebot mit mindestens einer Position auftritt. Da `quotes.Create`
  mindestens eine Position zwingend voraussetzt, ist `/revise` damit für
  praktisch jedes real angelegte Angebot nicht nutzbar — ein schwerwiegender,
  aber komplett unabhängiger Vorbefund. `Revise`s Audit-Code selbst wurde
  daher NICHT end-to-end über HTTP verifiziert (blockiert durch 0.34),
  strukturell aber identisch zu den zwei erfolgreich verifizierten Fällen
  (`Accept`, Freigabe-Entscheidung) und durch dieselben Unit-Tests des
  `auditlog`-Packages abgesichert.
  Andere, bereits bekannte Vorbefunde bei der Verifikation erneut bestätigt
  (nicht neu, nur zur Einordnung): `TestQuoteFlowWithPricingAndPDF`
  scheitert weiterhin an Backlog 0.24 (`DELETE .../items/{itemID}` liefert
  500 statt 400 bei letzter Position), NACHDEM der `/accept`-Schritt selbst
  bereits erfolgreich durchgelaufen und protokolliert war.
  `TestQuoteApprovalDecisionEndpointsRequireApprovePermission` scheitert
  weiterhin an Backlog 0.28 (Fixture-`reason_code`-Diskrepanz), NACHDEM der
  `/approve`-Schritt selbst bereits erfolgreich durchgelaufen und
  protokolliert war.
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean. `go test ./internal/quotes/...
  ./internal/http/... ./internal/auditlog/...` PASS. `go test
  ./internal/http -run TestContact`: identische 11/14 PASS — keine neuen
  Regressionen.
- Subtask 0.3.3.3: Anbindung `sales_orders` an das Änderungsprotokoll
  abgeschlossen. `sales.Service` bekam ein neues `WithAudit(*auditlog.Service)`
  (gleiches Fluent-Setter-Muster wie bei `quotes` aus 0.3.3.2 — hier
  besonders einfach, da `sales.NewService` ohnehin nur EINEN Aufrufer in
  `v1.go` und nur 2 direkte Testdatei-Call-Sites hat).
  Verdrahtet: `UpdateStatus` (atomar in der bereits vorhandenen Transaktion
  vor `tx.Commit`, neuer `actorUserID`-Parameter, 1 Aufrufer) und
  `ConvertToInvoice` (ebenso atomar, neuer `actorUserID`-Parameter, 1
  Aufrufer). Beide Funktionen hatten bereits vor dieser Subtask eine eigene
  Transaktion — kein Ausweichen auf nicht-transaktionale Aufzeichnung nötig
  (anders als bei `quotes.Accept` in 0.3.3.2).
  **Neuer, permanenter Integrationstest** `TestSalesOrderStatusChangeAnd
  ConvertToInvoiceAreAuditLogged` (`server/internal/http/accounting_integration_test.go`)
  — bewusst über `quotes`-Annahme → `POST .../convert-to-sales-order` →
  `POST .../status` geführt statt über den naheliegenderen, aber bereits an
  Backlog 0.24/0.25 scheiternden Standardfluss aus `TestQuoteFlowWithPricingAndPDF`/
  `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices` (beide
  isoliert erneut reproduziert, um das zu bestätigen, BEVOR der neue Test
  geschrieben wurde). Da es (noch) keinen `GET .../sales-orders/{id}/audit-log`-
  Endpunkt gibt (nur für `invoices_out` aus 0.3.3.1), verifiziert der Test
  das Protokoll direkt per SQL gegen `env.PG`.
  Gegen echtes Postgres bewiesen: `UpdateStatus` (`open`→`released`) erzeugt
  korrekt einen `status_geaendert`-Eintrag mit echtem `actor_user_id`.
  **Echter, gravierender, unabhängiger Fund** (Backlog 0.35, KRITISCH,
  NICHT behoben): `ConvertToInvoice` selbst scheitert bei JEDEM Aufruf mit
  `ERROR: FOR UPDATE cannot be applied to the nullable side of an outer
  join` — `loadForInvoiceTx` (`server/internal/sales/service.go:714-724`,
  `git show HEAD` bestätigt unverändert seit vor dieser Session) sperrt via
  `FOR UPDATE` eine Query mit zwei `LEFT JOIN`s (auf `projects`/`contacts`),
  ohne die Sperre über `FOR UPDATE OF so` auf die Haupttabelle
  einzuschränken — Postgres lehnt das grundsätzlich ab, sobald die
  gejointe Seite nullable ist. `POST /sales-orders/{id}/convert-to-invoice`
  ist damit AKTUELL KOMPLETT UNBENUTZBAR, unabhängig von Status/Daten.
  Genau dasselbe Muster (`FOR UPDATE` + `LEFT JOIN`) wurde in einer
  früheren Subtask dieser Session (GAEB-Import, `quotes/imports.go`)
  bereits einmal korrekt gelöst (`FOR UPDATE OF quote_imports`) — derselbe
  Fix würde hier greifen, ist aber bewusst nicht Teil dieser Subtask
  (0.3.3.3 war die Audit-Anbindung, kein Bugfix an der Sperrabfrage).
  `ConvertToInvoice`s Audit-Code konnte deshalb NICHT end-to-end über HTTP
  verifiziert werden, ist aber strukturell identisch zum erfolgreich
  verifizierten `UpdateStatus` (gleiches atomares `tx`-Muster, gleiche
  `RecordInput`-Konstruktion).
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean. `go test ./internal/sales/...
  ./internal/http/... ./internal/auditlog/...` PASS. `go test
  ./internal/http -run TestContact`: identische 11/14 PASS — keine neuen
  Regressionen.
- Subtask 0.3.3.4 (letzte Micro-Subtask von Task 0.3.3): Anbindung
  `purchase_orders` an das Änderungsprotokoll abgeschlossen. `purchasing.Service`
  bekam ein `WithAudit(*auditlog.Service)` (gleiches Fluent-Setter-Muster).
  `Update` hatte VORHER keine eigene Transaktion (nur zwei sequenzielle
  `s.pg`-Aufrufe: SELECT dann Exec, nur wenn Inhaltsfelder betroffen) — für
  atomares Audit-Logging in eine Transaktion mit `FOR UPDATE` auf die
  Status-Sperrabfrage umgebaut. **Positiver Nebeneffekt, kein Scope
  Creep**: das schließt eine kleine, bereits vorhandene TOCTOU-Lücke des
  `isEditableStatus`-Content-Guards aus Subtask 0.3.2 (der Read-Then-Write
  war vorher nicht atomar) — eine direkte, unvermeidliche Konsequenz der
  korrekten Umsetzung des Audit-Loggings selbst, nicht ein zusätzlicher,
  separat entschiedener Fix.
  Protokolliert wird bewusst NUR ein tatsächlicher Statuswechsel
  (`u.Status != nil && currentStatus != *u.Status`) als `status_geaendert`
  — reine Inhaltsänderungen (Nummer/Datum/Währung/Notiz) erzeugen keinen
  Eintrag, passend zum engen Scope "Statuswechsel" im Task-Titel.
  Gegen echtes Postgres per SQL-Direktabfrage bewiesen: der Übergang
  `draft`→`ordered` (der in 0.3.2 neu geschützte Übergang) erzeugt einen
  korrekten Eintrag mit echtem Actor. Der bestehende Test
  `TestPurchaseOrdersAreLockedAfterLeavingDraftStatus` (aus 0.3.2) lief
  nach der Transaktions-Restrukturierung unverändert erfolgreich durch —
  bestätigt, dass die Umstellung auf eine Transaktion das bestehende
  Verhalten nicht verändert hat.
  Gegen frische, mehrfach verifiziert leere DB verifiziert: `go build ./...`
  und `go vet ./...` clean, `gofmt -l` clean. `go test
  ./internal/purchasing/...` PASS (außer dem bereits bekannten,
  unveränderten Backlog-0.6-Panic in `TestCreateRejectsInvalidItem`,
  isoliert per `-run`-Ausschluss bestätigt unabhängig). `go test
  ./internal/http -run TestPurchaseOrders`: identisches Muster wie zuvor
  (nur der bereits bekannte Backlog-0.21-Fehlschlag in
  `TestPurchaseOrdersCreateAndGetFlow`, keine neuen Regressionen). `go test
  ./internal/http -run TestContact`: identische 11/14 PASS.
  **Task 0.3.3 (Generisches Änderungsprotokoll) UND DAMIT DAS GESAMTE EPIC
  0.3 (GoBD-Fundament) SIND DAMIT VOLLSTÄNDIG ABGESCHLOSSEN.** Subtask
  0.3.3.5 (Anbindung `contacts`/`projects`/`materials`/`hr`) wurde bewusst
  NICHT umgesetzt, sondern nach Bewertung als "kein GoBD-Bedarf"
  zurückgestellt und als erledigt markiert (siehe Backlog-Eintrag für die
  vollständige Begründung: die vier bereits angebundenen Domänen decken
  die komplette GoBD-relevante kommerzielle Belegkette ab; Stammdaten-
  Domänen ohne Buchungs-/Rechnungsbezug sind kein GoBD-Fundament-Bedarf).
  Insgesamt in Epic 0.3 gefunden und dokumentiert (nicht behoben, jeweils
  außerhalb des eigentlichen Subtask-Scopes): Backlog 0.33 (`quotes`-
  Schreibschutz liefert 500 statt 400 an 16 Stellen), 0.34 (KRITISCH,
  `quotes.Revise` "conn busy" bei jedem Angebot mit Positionen), 0.35
  (KRITISCH, `sales.ConvertToInvoice` "FOR UPDATE + LEFT JOIN"-Fehler bei
  jedem Aufruf).
- Subtask 0.4.1 (erste von zwei Subtasks von Task 0.4, Migrationsverzeichnis
  bereinigen): ADR zum Umgang mit doppelten Nummernketten abgeschlossen.
  Vollständige Bestandsaufnahme (nicht nur die im Backlog-Titel genannten
  4 Beispielnummern) ergab **14 betroffene Nummern-Präfixe** mit jeweils 2
  Dateien (bei `007` sogar 3): `001`-`008`, `013`, `014`, `017`-`020`. Ab
  `021` ist die Kette durchgängig eindeutig bis `063` (aktueller Stand).
  Neue `docs/adr/0003-migration-numbering.md`: **Entscheidung** — die 14
  historischen Duplikate bleiben UNVERÄNDERT (sie funktionieren
  nachweislich seit dutzenden Migrationsläufen dieser Session gegen
  frische DB; `server/internal/migrate/migrate.go` sortiert per
  `sort.Strings` über den VOLLEN Dateinamen, nicht nur die Nummer, wodurch
  die Ausführungsreihenfolge innerhalb einer Nummerngruppe bereits
  deterministisch und funktional korrekt ist; kein Versions-Tracking
  existiert, das den alten Dateinamen referenziert — ein Umbenennen hätte
  keinen Laufzeit-Nutzen, nur unnötiges Risiko und würde `docs/state.md`-
  Log-Einträge entwerten, die Migrationsnummern wörtlich zitieren). Für
  NEUE Migrationen gilt ab sofort eine explizit dokumentierte Regel: vor
  dem Anlegen die höchste vorhandene Nummer prüfen (`ls migrations | sort
  | tail -3`) und strikt darüber wählen — das war bereits die gelebte
  Praxis dieser Session ab Migration 054 (die Duplikate 001-020 stammen
  alle aus der Zeit VOR dieser Session), wird hiermit nur formalisiert.
  Zwei Alternativen erwogen und mit Begründung verworfen: Zeitstempel-Präfixe
  (Stilbruch mitten in einer bislang durchgängig 3-stelligen Kette, kein
  Mehrwert gegenüber der Nachsehen-Regel, SOFERN sie befolgt wird) und ein
  externes Migrationstool wie `golang-migrate`/`goose` (würde das Problem
  strukturell lösen, aber einen deutlich größeren, für Task 0.4 nicht
  angemessenen Umbau erfordern — neue Abhängigkeit, Runner-Umbau, Weg
  finden müssen, die bereits gelaufenen 63 Dateien als "angewendet" zu
  markieren, damit sie beim Tool-Wechsel nicht erneut versucht werden).
  Kurzer Hinweis-Kommentar in `migrate.go` ergänzt (verweist auf die ADR,
  direkt an der Stelle sichtbar, wo künftig neue Migrationen erstellt
  werden) — keine funktionale Codeänderung.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` und
  `go vet ./...` clean; kompletter Migrationslauf 001-063 weiterhin
  fehlerfrei (per `TestEmployeeGetIsScopedToCompany`-Lauf bestätigt, der
  die volle Migrationskette durchläuft).
- Subtask 0.4.2 (letzte Subtask von Task 0.4): ADR zum verwaisten
  `server/migrations/`-Verzeichnis abgeschlossen. Alle 9 Dateien per
  `diff` byte-für-byte mit gleichnamigen, bereits aktiven Dateien in
  `server/internal/migrate/migrations/` verglichen — **exakt identisch**
  (`001_projects.sql`, `002_project_phases.sql`, `003_elevations.sql`,
  `004_single_elevations.sql`, `005_single_elevation_materials.sql`,
  `006_import_logs.sql`, `007_alter_import_logs.sql`,
  `007_elevation_attrs.sql`, `008_seed_project_numbering.sql`). Das löst
  nebenbei einen Teil des Duplikat-Rätsels aus 0.4.1 auf: diese 9 Dateien
  sind exakt die Hälfte der Duplikat-Paare bei den Nummern `001`-`008` —
  `server/migrations/` war offensichtlich der alte Verzeichnisstandort,
  bevor die Dateien (vermutlich wegen der `go:embed`-Anforderung, dass
  eingebettete Dateien innerhalb des Go-Package-Baums liegen müssen) nach
  `server/internal/migrate/migrations/` verschoben wurden — der alte
  Ordner blieb zurück.
  Repo-weite Suche (`grep -rn "server/migrations"` über Go-Code, YAML,
  Dockerfiles, Markdown) bestätigt: außerhalb der eigenen Recon-/Backlog-
  /ADR-Notizen dieser Session keine einzige Referenz. `git log --oneline
  -- server/migrations/` zeigt einen einzelnen, lange zurückliegenden
  Commit, seither unverändert, sauberer Git-Status vor der Löschung.
  Neue `docs/adr/0004-orphaned-migrations-directory.md`: **Entscheidung**
  — Verzeichnis vollständig löschen (anders als die in ADR 0003 belassenen
  historischen Duplikate im AKTIVEN Verzeichnis, wo ein Umbenennen keinen
  Nutzen hätte, hat das Löschen dieses komplett redundanten, unreferenzierten
  Alt-Verzeichnisses einen klaren Nutzen: es beseitigt eine Quelle
  künftiger Verwirrung und erfüllt den Auftrag von Task 0.4 tatsächlich).
  Durchgeführt: `git rm -r server/migrations/` (9 Dateien entfernt, **nur
  gestaged, nicht committet** — wie bei jeder Änderung dieser Session, da
  Commits ausschließlich auf explizite Anweisung erfolgen).
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` und
  `go vet ./...` clean; kompletter Migrationslauf 001-063 weiterhin
  fehlerfrei (Verzeichnis war nie eingebunden, daher erwartungsgemäß keine
  Verhaltensänderung).
  **Task 0.4 (Migrationsverzeichnis bereinigen) ist damit vollständig
  abgeschlossen.**
- Subtask 0.5.1 (erste von vier Subtasks von Task 0.5, Auth-Härtung):
  Startup-Guard gegen Default-`JWT_SECRET` in Produktivumgebung
  abgeschlossen. `server/internal/config/config.go:55` fiel bisher in
  JEDER Umgebung inkl. Produktion still auf `"dev-secret-change-me"`
  zurück, falls `JWT_SECRET` nicht gesetzt war — ein im öffentlichen Repo
  sichtbarer, damit wertloser Signierschlüssel, mit dem sich beliebige
  Access-Tokens fälschen ließen (siehe ADR 0001 Entscheidung 3: Eigenbau-
  HMAC-Signing statt geprüfter JWT-Bibliothek — macht einen schwachen
  Schlüssel besonders kritisch, da keine externe Bibliothek zusätzlich vor
  offensichtlich unsicheren Schlüsseln warnt).
  Neues `Config.AppEnv`-Feld liest `APP_ENV` (Default `"development"`) —
  `.env.example` hatte diese Variable bereits als Intention dokumentiert,
  der Code hat sie vorher nie gelesen (kein einziges
  Environment-Detection-Vorkommen im gesamten Repo, verifiziert per
  `grep`). Neue, PUR TESTBARE Funktion `config.ValidateForStartup(cfg)`
  (gibt nur einen `error` zurück, ruft selbst kein `os.Exit`/`log.Fatal`
  auf) prüft `AppEnv=="production"` (case-insensitiv, deckt
  `Production`/`PRODUCTION` etc. ab) gegen `len(JWTSecret) < 32` — bewusst
  eine LÄNGENSCHWELLE statt einer Liste bekannter unsicherer Werte
  (`"dev-secret-change-me"`, `"change-me"` aus `.env.example`), damit auch
  künftige oder andere schwache Secrets abgefangen werden, nicht nur die
  zwei aktuell bekannten. `cmd/api/main.go` ruft die Funktion als
  ALLERERSTES nach `config.Load()` auf (vor jedem DB-/Mongo-/Redis-Connect)
  und beendet mit `os.Exit(1)`, falls sie einen Fehler liefert — exakt das
  bereits etablierte Fail-Fast-Muster des bestehenden `app.New`-Fehlerpfads
  direkt darunter.
  **Per echtem Binary-Smoke-Test bewiesen** (nicht nur Unit-Test — der
  eigentliche Server wurde kompiliert und tatsächlich gestartet):
  `APP_ENV=production JWT_SECRET=short ./nala_smoke_test` → Exit-Code `1`
  mit der erwarteten deutschen Fehlermeldung, VOR jedem Verbindungsversuch;
  `APP_ENV=development JWT_SECRET=short ./nala_smoke_test` → läuft am
  Guard vorbei und erreicht den normalen "[Start]"-Log (bricht danach
  erwartungsgemäß beim echten Postgres-Connect ab, da kein Docker-Netzwerk
  vorhanden — irrelevant für den Test des Guards selbst).
  **Kein Verhaltensbruch für bestehende Umgebungen**: weder die lokale
  `.env` dieser Session noch `docker-compose.test.yml` setzen
  `APP_ENV=production` (verifiziert per `grep`) — der Guard greift also in
  keinem bisher genutzten Dev-/Test-Pfad. `.env.example` um einen Hinweis
  auf den Guard direkt beim `JWT_SECRET`-Eintrag ergänzt.
  Gegen frische, verifiziert leere DB verifiziert: `go build ./...` und
  `go vet ./...` clean, `gofmt -l` clean (`config.go`/`main.go` bleiben wie
  vor dieser Session nicht gofmt-konform, Backlog 0.15, per `git show
  HEAD` bestätigt vorbestehend). Neue Tests
  `server/internal/config/config_test.go`: `TestValidateForStartupRejectsShortJWTSecretInProduction`
  (8 tabellengetriebene Fälle: production+Code-Default/`.env.example`-
  Platzhalter/leer/Großschreibung → alle abgelehnt; production+ausreichend
  langes Secret, development+Code-Default, leeres AppEnv+Default,
  staging+kurzes Secret → alle erlaubt) und
  `TestLoadDefaultsAppEnvToDevelopment`. `go test ./internal/config/...
  ./internal/auth/...` PASS. `go test ./internal/http -run TestContact`:
  identische 11/14 PASS — keine neuen Regressionen.
- Subtask 0.5.2 (zweite von vier Subtasks von Task 0.5, Auth-Härtung):
  Rate-Limiting auf `/auth/login` abgeschlossen. `auth.Service.Login` hatte
  zuvor keinerlei Brute-Force-/Credential-Stuffing-Schutz — recherchiert:
  `user.IsLocked` (`server/internal/auth/service.go:61,202`,
  `repository.go:15`) ist nur ein manuell gesetztes Flag, kein
  automatischer Fehlversuchs-Zähler, keine Rate-Limiting-Begriffe irgendwo
  im `auth`-Paket gefunden (per `grep`).
  Neue eigenständige `auth.LoginRateLimiter` (neue Datei
  `server/internal/auth/ratelimit.go`) nutzt den bereits vorhandenen
  Redis-Client wieder (denselben, den `auth.SessionStore`,
  `server/internal/auth/store.go`, für Sessions verwendet — KEINE neue
  Infrastruktur-Abhängigkeit eingeführt, wie im Plan aus 0.5.1 vorgesehen)
  als Fixed-Window-Zähler PRO IP-ADRESSE (`net.SplitHostPort` trennt den
  Port von `req.RemoteAddr` ab, Fallback auf Rohwert falls kein Port
  vorhanden).
  **Design-Entscheidung, die einen realen Kompatibilitätsbruch verhindert
  hat**: der Zähler zählt bewusst NUR fehlgeschlagene Versuche
  (`RegisterFailure`, inkrementiert bei jedem der vier bestehenden
  Fehlerpfade in `Login` — Nutzer nicht gefunden/inaktiv/gesperrt/falsches
  Passwort) und wird bei Erfolg zurückgesetzt (`Reset`) — NICHT alle
  Requests gezählt. Grund: `httptest.NewRequest()` vergibt ohne explizites
  `RemoteAddr` für JEDEN Request dieselbe feste Default-IP (`192.0.2.1`),
  und der Test-Helper `loginIntegrationUser` (per `grep` an ~40
  Aufrufstellen quer durchs `internal/http`-Testpaket verifiziert) nutzt
  genau diese Default-IP für erfolgreiche Logins. Ein "alle Versuche
  zählen"-Design hätte in jedem vollen Testlauf ausnahmslos JEDEN
  Integrationstest, der sich einloggt, nach wenigen Testfunktionen
  blockiert — kein hypothetisches Risiko, sondern zwingend bei diesem
  konkreten Testaufbau. Diese Design-Wahl wurde VOR der Implementierung
  getroffen, nicht als nachträglicher Fix.
  `Service.WithRateLimiter(*LoginRateLimiter)` als optionaler,
  nil-sicherer Fluent-Setter (identisches Muster wie `WithAudit` etc. aus
  Epic 0.3 — kein bestehender Call-Site-Bruch, auch nicht in
  `internal/auth/service_test.go`s `newTestService()`, die den Setter
  gar nicht aufruft und damit unverändert mit deaktiviertem Rate-Limiting
  läuft). `config.Config` (`server/internal/config/config.go`) neu:
  `LoginRateLimitMaxAttempts` (Default 10, env
  `LOGIN_RATE_LIMIT_MAX_ATTEMPTS`), `LoginRateLimitWindowSeconds` (Default
  60, env `LOGIN_RATE_LIMIT_WINDOW_SECONDS`) — Werte 0/negativ deaktivieren
  das Limit vollständig (nil-sicher in `LoginRateLimiter.Blocked/
  RegisterFailure`). `server/internal/http/v1.go`: Konstruktion
  `auth.NewLoginRateLimiter(rd, cfg.LoginRateLimitMaxAttempts,
  time.Duration(cfg.LoginRateLimitWindowSeconds)*time.Second)`, an
  `authSvc` durchgereicht; Login-Handler mappt neues `auth.ErrRateLimited`
  auf HTTP 429 (`rate_limited`, deutsche Fehlermeldung). `.env.example` um
  die beiden neuen Variablen ergänzt (gleiche Stelle wie der 0.5.1-Hinweis).
  **Tests**: (1) DB-lose Unit-Tests `server/internal/auth/ratelimit_test.go`
  — `loginRateLimitKey` trennt Port korrekt ab (inkl. Leerstring-Fall),
  nil-`*LoginRateLimiter` blockiert nie (Noop), `maxAttempts<=0` deaktiviert
  ebenfalls; (2) echter Integrationstest
  `TestAuthLoginRateLimitedAfterRepeatedFailures`
  (`server/internal/http/auth_integration_test.go`) mit eigener, isolierter
  Test-IP (`198.51.100.42`, ausdrücklich NICHT die von `httptest` geteilte
  Default-IP, um andere Tests im selben Lauf nicht zu kontaminieren — inkl.
  `t.Cleanup`, das den Redis-Schlüssel danach löscht) und lokal
  abgesenkter Schwelle (`limitedCfg.LoginRateLimitMaxAttempts = 3` statt
  Produktions-Default 10, damit der Test nicht 10+ Requests braucht): 3×
  falsches Passwort → je `401 auth_failed`, 4. Versuch → `429
  rate_limited` (Fehlercode im Response-Body verifiziert), danach auch mit
  dem KORREKTEN Passwort → weiterhin `429` (Sperre gilt pro IP-Adresse,
  unabhängig vom Ausgang des einzelnen Versuchs — bewusst geprüft, da genau
  dieses Verhalten die eigentliche Schutzwirkung ist).
  Verifiziert gegen frische, per `docker exec ... psql -c "\dt"` bestätigt
  leere DB: `go build ./...` und `go vet ./...` clean (keine neuen
  gofmt-Verstöße eingeführt — `internal/auth/service.go` und
  `internal/testutil/integration.go` per `git show HEAD` als bereits VOR
  dieser Session nicht gofmt-konform bestätigt, deckungsgleich mit dem
  bereits dokumentierten Backlog 0.15). `go test ./internal/auth/...
  ./internal/config/...` PASS. Gezielt `go test ./internal/http/... -run
  TestAuthLogin -count=1` gegen frische DB: beide Tests
  (`TestAuthLoginAndMeFlow`, `TestAuthLoginRateLimitedAfterRepeatedFailures`)
  PASS.
  **Bewusst NICHT als Nachweis verwendet**: der volle `go test
  ./internal/http/... -count=1`-Lauf — dieser bestätigte erneut den
  bereits dokumentierten Backlog-0.20-Kipppunkt (Migrationen 050/051 nicht
  sicher wiederholt ausführbar), der ab einem bestimmten Punkt in der
  Testreihenfolge JEDE nachfolgende Testfunktion inkl. der beiden neuen
  0.5.2-Tests mit einem irrelevanten `migrate.Run`-Fehler fehlschlagen
  lässt (in diesem Lauf ca. 70 von ~75 Testfunktionen — deutlich mehr als
  die ursprünglich in 0.20 geschätzten ~30, siehe aktualisierte Notiz dort).
  Isoliert bestätigt: derselbe volle Lauf schlägt identisch fehl, wenn man
  NUR die vorbestehenden Tests betrachtet (Kipppunkt liegt vor
  `auth_integration_test.go` in der Ausführungsreihenfolge) — kein
  Zusammenhang mit dieser Subtask.
- Subtask 0.5.3 (dritte von vier Subtasks von Task 0.5, Auth-Härtung):
  `users.manage`-Bypass in `requirePermission()` durch dedizierte
  `admin.superuser`-Permission ersetzt. Fund: `requirePermission()`
  (`server/internal/http/v1.go`) ließ JEDE Berechtigungsprüfung durch,
  sobald der Nutzer irgendeine Permission namens `users.manage` besaß
  (`p == permission || p == "users.manage"`) — ein zweites, identisches
  Muster fand sich zusätzlich inline im Quote-Accept-Handler (optionaler
  Projektstatus-Wechsel beim Akzeptieren eines Angebots, benötigt normal
  `projects.write`). Per `grep -rn "users.manage"` über das gesamte
  `server/`-Modul bestätigt: nur diese zwei Code-Stellen plus die
  Permission-Seed-Zeile selbst (`017_auth.sql:77`) — keine weiteren
  versteckten Vorkommen.
  Recherche der Seed-Daten (`017_auth.sql`) ergab den eigentlichen Fehler:
  `users.manage` ist explizit als ENGES "Benutzer/Rollen verwalten"-Recht
  beschrieben (Name "Benutzer verwalten", `context='platform'`) — der Code
  hat es zusätzlich, ohne Bezug zu dieser Beschreibung, als
  Universal-Vollzugriffs-Schalter zweckentfremdet. Praktische Auswirkung
  HEUTE: keine, da nur die Rolle `admin` `users.manage` besitzt, und `admin`
  ohnehin bereits JEDE Einzelberechtigung direkt zugewiesen bekommt (`017_auth.sql:95-98`:
  `INSERT INTO role_permissions SELECT 'role-admin', p.id FROM permissions p`
  — ohne WHERE-Filter, literal alle). Die eigentliche Gefahr lag in der
  ZUKUNFT: die für Subtask 0.5.4 geplante User-Management-API wird
  vermutlich Rollen mit gezielt eingeschränkten Rechten anlegen können —
  eine Rolle "Benutzeradministrator" mit NUR `users.manage` (fachlich
  korrekt gedacht: darf Nutzerkonten pflegen, aber keine Rechnungen/
  Bestellungen/Angebote anfassen) hätte durch diesen Bypass STILLSCHWEIGEND
  vollen Systemzugriff erhalten — ein klassischer, schwer zu entdeckender
  Privilegien-Eskalationsfehler, weil er in den Permission-Daten selbst
  (`role_permissions`-Tabelle) unsichtbar bleibt und nur im Code sichtbar
  ist.
  **Entscheidung "ersetzen" statt nur "dokumentieren"** (beide Optionen
  waren im Backlog-Titel selbst vorgesehen): Epic 0.5 heißt explizit
  Auth-HÄRTUNG — ein reiner Kommentar hätte den Bypass-Mechanismus
  unverändert gelassen und die Falle für 0.5.4 offen gehalten; die
  Ersetzung ist die einzige Variante, die das eigentliche Risiko behebt,
  und blieb mit ~15 Zeilen Migration + 2 kleinen Code-Stellen weit
  innerhalb des Umfangs für eine einzelne Subtask (keine Zerlegung in
  Mikro-Subtasks nötig).
  Neue Migration `server/internal/migrate/migrations/064_admin_superuser_permission.sql`
  legt Permission `admin.superuser` an (`ON CONFLICT (code) DO NOTHING`,
  konsistent mit der etablierten Kein-Versions-Tracking-Konvention aus
  Backlog 0.13/ADR 0003 — jede Migration muss idempotent bleiben, da sie bei
  jedem `migrate.Run()`-Aufruf erneut ausgeführt wird) und vergibt sie
  ausschließlich an `role-admin`. Beide Code-Stellen in `v1.go` auf neue
  Package-Konstante `adminSuperuserPermission = "admin.superuser"`
  umgestellt (mit Kommentar, der begründet, warum bewusst NICHT
  `users.manage` wiederverwendet wird).
  **Kein Verhaltensbruch**: `role-admin` hatte vor dieser Änderung bereits
  jede Einzelberechtigung direkt zugewiesen (unabhängig vom Bypass) und
  bekommt jetzt zusätzlich explizit `admin.superuser` — funktional
  identisch. Keine andere bestehende Rolle (`sales`/`procurement`/
  `inventory`/`finance`/`hr`/`production`/`fleet`) besitzt `users.manage`,
  verliert also keine Berechtigung, die sie vorher (fälschlich) gehabt
  hätte.
  **Test**: da keine bestehende Rolle isoliert NUR `users.manage` besitzt,
  legt der neue Integrationstest
  `TestUsersManagePermissionNoLongerBypassesOtherPermissionChecks`
  (`server/internal/http/auth_integration_test.go`) gezielt per Direkt-SQL
  eine Test-Rolle mit ausschließlich dieser einen Permission an (derselbe
  Ansatz, den `0.5.4` später durch eine echte API ersetzen soll) und
  beweist: Zugriff auf einen `materials.write`-Endpunkt liefert jetzt
  korrekt `403 forbidden` (vor der Korrektur wäre es `201 Created`
  gewesen — der Bypass hätte gegriffen).
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean für alle geänderten Dateien (keine neuen
  gofmt-Verstöße). `go test ./internal/auth/... ./internal/config/...`
  PASS. Gezielt `go test ./internal/http/... -run
  "TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
  -count=1` gegen frische DB: alle 4 Tests PASS, inkl. des bestehenden
  `TestMaterialsCreateIsForbiddenForSalesRole` als Regressionscheck (beweist,
  dass die normale, nicht-bypass-relevante Forbidden-Logik unverändert
  funktioniert). Zusätzlich per direkter SQL-Abfrage bestätigt: Permission
  `admin.superuser` existiert genau einmal und ist ausschließlich an
  `role-admin` vergeben (`SELECT r.code, p.code FROM role_permissions rp
  JOIN roles r ... JOIN permissions p ... WHERE p.code='admin.superuser'`
  → genau eine Zeile, `admin | admin.superuser`).
- Task 0.5.4 (User-Management-API) begonnen, vor der Umsetzung in drei
  Mikro-Subtasks zerlegt (0.5.4.1 Anlegen, 0.5.4.2 Sperren/Entsperren,
  0.5.4.3 Rollenzuweisung — geschätzter Gesamtumfang klar über der
  ~400-Zeilen-Schwelle, der Backlog-Titel legte die Dreiteilung ohnehin
  schon nahe).
  Subtask 0.5.4.1 (Anlegen, `POST /users`) abgeschlossen. Vorher gab es
  KEINEN HTTP-Weg, einen Benutzer anzulegen - ausschließlich Direkt-SQL
  (`testutil.SeedAuthUser`, testonly, sowie vermutlich manuelle
  Admin-Eingriffe in Produktion, per `grep -rn "INSERT INTO users"` über
  das gesamte `server/`-Modul bestätigt: nur zwei Testdateien).
  Neues `auth.UserCreate`-Eingabe-DTO (`server/internal/auth/types.go`)
  bewusst OHNE Rollenzuweisung - das ist die eigene Subtask 0.5.4.3, ein
  neu angelegter Nutzer hat zunächst keine Rollen/Berechtigungen (reines
  Skelettkonto).
  `Repository.CreateUser` (`server/internal/auth/repository.go`) plus
  `EmailExists`/`UsernameExists` als Duplikat-Vorabprüfung - folgt dem in
  `contacts.Service.ensureNoDuplicate` bereits etablierten Muster (SELECT
  vor INSERT statt Auswertung des Postgres-Unique-Constraint-Fehlers), vor
  allem damit die Fehlermeldung ("... bereits vorhanden") das in
  `classifyDomainError` bereits vorhandene Substring-Muster trifft, ohne
  dort etwas ändern zu müssen.
  `Service.CreateUser` (`server/internal/auth/service.go`) validiert:
  E-Mail-Format (nicht leer, enthält `@`), Mandant (`companyID` als
  Pflichtparameter - der Handler liest ihn aus `companyIDFromContext`),
  Passwort (wiederverwendet das bestehende `HashPassword` unverändert,
  inkl. dessen Leer-Prüfung "passwort erforderlich"). Setzt sinnvolle
  Defaults, falls nicht vom Aufrufer gesetzt: Locale `de-DE`, Timezone
  `Europe/Berlin`, `display_name` aus Vor-/Nachname zusammengesetzt.
  Erzwingt `is_active=true`/`is_locked=false` für neue Nutzer (Sperren ist
  0.5.4.2s Aufgabe, nicht Teil von "Anlegen").
  Neue Route in `v1.go`: `protected.Route("/users", ...)`,
  `POST /` mit `requirePermission("users.manage")` geschützt - die aus
  Subtask 0.5.3 gerade erst korrekt eng skopierte Permission (vorher ein
  impliziter Universal-Bypass) wird hier zum ersten Mal für ihren
  eigentlichen, benannten Zweck verwendet ("Benutzer verwalten") - ein
  schöner konkreter Beleg dafür, dass 0.5.3s Härtung sinnvoll war und
  keine tote Berechtigung hinterlassen hat.
  **Tests**: DB-lose Validierungstests
  (`server/internal/auth/service_create_user_test.go`) für die drei frühen
  Fehlerpfade (ungültige/fehlende E-Mail, fehlender Mandant, leeres
  Passwort) - alle drei greifen VOR dem ersten Repository-Zugriff, daher
  mit `repo=nil` (`newTestService()`) sicher testbar, ohne echte DB.
  Zwei neue Integrationstests
  (`server/internal/http/auth_integration_test.go`):
  `TestUsersCreateEndpointCreatesUserWithinCallersCompany` — 201, Mandant
  korrekt auf `company_id` des Aufrufers gesetzt (`'default'` bei
  `SeedAuthUser`-Testnutzern), `password_hash` nie im Response-Body
  enthalten (per String-Suche im Rohbody zusätzlich zur Struct-Decodierung
  geprüft, da `json:"-"` sonst nur "vertraut, nicht geprüft" wäre), UND ALS
  ENDE-ZU-ENDE-BEWEIS FÜR KORREKTES PASSWORT-HASHING: unmittelbar nach der
  Anlage ein ECHTER Login-Request mit exakt dem beim Anlegen gesetzten
  Klartext-Passwort - nicht nur eine Struktur-/Statuscode-Prüfung der
  Create-Antwort, sondern der tatsächliche Beweis, dass `bcrypt`-Hash und
  spätere `bcrypt.CompareHashAndPassword`-Prüfung in `Login()`
  zusammenpassen. Anschließend Duplikat-E-Mail → `400 validation_error`.
  `TestUsersCreateEndpointIsForbiddenWithoutUsersManagePermission` — eine
  Rolle ohne `users.manage` (hier: `sales`) erhält `403 forbidden`, wichtig
  als expliziter Regressionsbeleg dafür, dass 0.5.3s Verengung von
  `users.manage` (kein Universal-Bypass mehr) diese neue Route korrekt vor
  unautorisiertem Zugriff schützt.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...` clean, `gofmt -l` clean für alle neuen/geänderten Dateien
  (keine neuen Verstöße; `service.go` bleibt wie vor dieser Session nicht
  gofmt-konform, Backlog 0.15, unverändert). `go test ./internal/auth/...
  ./internal/config/...` PASS. Gezielt `go test ./internal/http/... -run
  "TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
  -count=1`: alle 6 Tests PASS (inkl. der drei bereits bestehenden
  Auth-Härtungs-Tests aus 0.5.2/0.5.3 sowie dem bestehenden
  `TestMaterialsCreateIsForbiddenForSalesRole` als weiterer
  Regressionscheck).
- Subtask 0.5.4.2 (Sperren/Entsperren, `POST /users/{id}/lock` +
  `POST /users/{id}/unlock`) abgeschlossen. Neue Aktions-Endpunkte statt
  eines rohen PATCH mit `is_locked`-Feld — analog zum bereits im Projekt
  etablierten Muster fuer Status-Aktionen (`.../accept`, `.../revise`,
  `.../storno`).
  Neuer Fehler `auth.ErrUserNotFound` ("Benutzer nicht gefunden") neben den
  bestehenden Sentinel-Errors in `repository.go`. Neue
  `Repository.SetUserLocked(ctx, userID, locked, companyID)`: ein einziges
  `UPDATE users SET is_locked=$1 ... WHERE id=$2 AND company_id=$3` -
  Mandanten-Scoping direkt in der WHERE-Klausel statt als separater
  Nachher-Check, dadurch atomar und ohne TOCTOU-Luecke. `RowsAffected()==0`
  (kein Treffer, weil ID nicht existiert ODER zu einem anderen Mandanten
  gehoert) mappt auf `ErrUserNotFound` -> `classifyDomainError`s bereits
  vorhandenes `"nicht gefunden"`-Muster -> `404`, OHNE dass der Aufrufer
  unterscheiden kann, ob der Benutzer gar nicht existiert oder nur einem
  anderen Mandanten gehoert (verhindert Existenz-Leaks ueber Mandanten
  hinweg, konsistent mit dem in Task 0.2 etablierten Muster in den anderen
  Domaenen). `Service.SetUserLocked` validiert nur `userID`/`companyID` -
  keine weitere Fachlogik noetig, das Sperren selbst hat keine
  Status-Vorbedingungen (anders als z.B. `purchasing.Update`s
  `isEditableStatus`-Guard aus Epic 0.3).
  **Bewusst NICHT implementiert**: ein Selbstsperr-Schutz (verhindern, dass
  sich der letzte verbleibende `users.manage`-Inhaber selbst aussperrt).
  Das waere eine sinnvolle, aber EIGENSTAENDIGE Design-Entscheidung, die
  ueber den engen Subtask-Titel ("`is_locked` umschalten") hinausgeht -
  bewusst nicht mitgemacht, um Scope-Creep zu vermeiden; bei Bedarf als
  eigenes Backlog-Item nachtragbar.
  **Zentraler Fund/Beweis bei der Verifikation, der die Subtask erst
  wirklich sinnvoll macht**: eine Sperre wirkt nicht nur gegen NEUE Logins
  (das pruefte `Login()` bereits vor dieser Subtask via `user.IsLocked`),
  sondern auch gegen ein BEREITS ausgestelltes Access-Token - weil
  `AuthenticateAccessToken` (`server/internal/auth/service.go:202`)
  `user.IsLocked` bei JEDEM authentifizierten Request frisch aus der DB
  neu liest (nicht nur beim Login, nicht aus dem im Access-Token
  eingebetteten Claim). Eine Sperre wirkt also spaetestens ab dem naechsten
  Request des betroffenen Nutzers, nicht erst nach Ablauf seines
  Access-Tokens (bis zu `ACCESS_TOKEN_TTL_MINUTES`, Standard 15 Minuten,
  waere sonst ein Zeitfenster fuer Missbrauch). Das war VORHER schon so im
  Code angelegt (keine Aenderung an `AuthenticateAccessToken` noetig) -
  diese Subtask hat es lediglich ERSTMALS end-to-end per Integrationstest
  bewiesen, statt es nur aus dem Code abzuleiten.
  Tests: 2 DB-lose Validierungstests
  (`server/internal/auth/service_lock_user_test.go` — leere `userID`, leere
  `companyID`) sowie 3 Integrationstests
  (`server/internal/http/auth_integration_test.go`):
  `TestUsersLockEndpointBlocksNewLoginsAndInvalidatesExistingSessions`
  (voller Zyklus: `GET /auth/me` mit bestehendem Token vor der Sperre `200`
  → Sperren `200` mit `is_locked:true` in der Antwort → dasselbe Token
  danach `401`, UND ein neuer Login-Versuch ebenfalls `401` → Entsperren
  `200` mit `is_locked:false` → neuer Login wieder `200`),
  `TestUsersLockEndpointReturnsNotFoundForUserInAnotherCompany` (legt per
  Direkt-SQL einen zweiten Mandanten (`company_profiles`) und einen dort
  zugehoerigen Nutzer an, da `testutil.SeedAuthUser` `company_id` fest auf
  `'default'` setzt und daher kein Cross-Tenant-Szenario liefern kann -
  beweist `404` statt Existenz-Leak), sowie
  `TestUsersLockEndpointIsForbiddenWithoutUsersManagePermission`.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean (keine neuen Verstöße). `go test
  ./internal/auth/... ./internal/config/...` PASS. Gezielt `go test
  ./internal/http/... -run
  "TestUsersLock|TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
  -count=1`: alle 9 Tests PASS (3 neue Lock/Unlock-Tests plus die 6
  bereits bestehenden Auth-Härtungs-/Regressionstests aus
  0.5.2/0.5.3/0.5.4.1).
- Subtask 0.5.4.3 (Rollenzuweisung, `GET /users/roles` +
  `PUT /users/{id}/roles`) abgeschlossen — **letzte Subtask von Task 0.5.4,
  damit ist Epic 0.5 (Auth-Härtung) vollständig abgeschlossen.**
  `Repository.ListRoles` (einfache Liste aller Rollen, Grundlage für eine
  künftige Rollenauswahl-UI). `Repository.ReplaceUserRoles` — bewusst PUT
  auf HTTP-Ebene statt POST, weil die Aktion die komplette Rollenmenge
  ERSETZT (idempotent), nicht inkrementell hinzufügt: (1) Mandanten-Scoping
  wie bei `SetUserLocked` aus 0.5.4.2 (Zielbenutzer muss zum Mandanten des
  Aufrufers gehören, sonst `ErrUserNotFound` -> `404`, kein Existenz-Leak
  über Mandantengrenzen), (2) validiert ALLE übergebenen Rollen-Codes VOR
  jeder Änderung mittels `SELECT code FROM roles WHERE code = ANY($1)` und
  Soll-/Ist-Abgleich — Alles-oder-nichts, damit ein Tippfehler in der Mitte
  einer langen Rollen-Liste nicht zu einer teilweise angewendeten Zuweisung
  führt, (3) führt `DELETE FROM user_roles WHERE user_id=$1` und den
  anschließenden `INSERT ... SELECT ... WHERE r.code = ANY($3)` atomar in
  einer Transaktion aus (kein Zwischenzustand ohne jede Rolle sichtbar für
  parallele Requests), (4) trägt den Aufrufer in die bereits VORHANDENE,
  bisher ungenutzte Spalte `user_roles.assigned_by`
  (`actorUserIDFromContext`) ein — keine Schema-Änderung nötig, die Spalte
  existierte schon seit `017_auth.sql`, wurde aber vom Code bisher nie
  befüllt. `Service.ReplaceUserRoles` normalisiert die Eingabe (trimmt,
  verwirft Duplikate/Leerstrings) VOR dem Repository-Aufruf, damit z.B.
  `["inventory", " inventory ", "", "  "]` korrekt zu `["inventory"]` wird.
  **Zentraler Testbeweis**: `TestUsersRolesEndpointReplacesAssignmentAndTakesEffectOnNextLogin`
  loggt den Zielnutzer VOR der Umzuweisung ein (Rolle `sales`, hat
  `quotes.write`, nicht `materials.write`), weist per `PUT` die Rolle
  `inventory` zu, loggt danach ERNEUT ein und prüft: `quotes.write` ist weg,
  `materials.write` ist da — nicht nur, dass die `PUT`-Antwort das
  behauptet (das wäre nur ein Beleg, dass die Antwort korrekt zusammengebaut
  wird, nicht dass die Zuweisung tatsächlich in der DB gelandet ist und vom
  Rest des Systems gelesen wird).
  **Fehler gefunden und sofort korrigiert**: die erste Fassung der
  Validierungsfehlermeldung nutzte "ungueltiger Rollen-Code: %s" (ASCII
  ohne Umlaut, konsistent mit dem Stil anderer Fehlermeldungen in
  `repository.go`, z.B. "ungueltige anmeldedaten"). `classifyDomainError`
  (`server/internal/http/v1.go`) prüft aber auf den Substring "ungültig"
  MIT ü — die ASCII-Variante traf das Muster nicht und fiel durch zum
  `default`-Fall (`500 internal_error`) statt `400 validation_error`. Beim
  ersten Lauf von `TestUsersRolesEndpointRejectsUnknownRoleCode` sofort
  aufgefallen (Test erwartete 400, bekam 500 mit der Fehlermeldung im
  Body sichtbar) — behoben durch Verwendung des Umlauts ("ungültiger
  Rollen-Code: %s"), erneuter Lauf danach grün. Kein Einzelfall-Risiko für
  andere Fehlermeldungen in diesem Modul: alle anderen `auth`-Fehlermeldungen
  sind bewusst durchgehend ASCII (aus Gewohnheit dieses Pakets), treffen
  aber zufällig keine der ü/ä/ö-abhängigen `classifyDomainError`-Muster, da
  sie andere Substrings ("erforderlich", "bereits vorhanden", "nicht
  gefunden") verwenden, die keinen Umlaut enthalten — nur dieser eine neue
  Fall war betroffen.
  Tests: 2 DB-lose Validierungstests
  (`server/internal/auth/service_replace_user_roles_test.go` — leere
  `userID`, leere `companyID`) sowie 6 Integrationstests
  (`server/internal/http/auth_integration_test.go`):
  `TestUsersRolesEndpointListsAvailableRoles`,
  `TestUsersRolesEndpointReplacesAssignmentAndTakesEffectOnNextLogin` (s.o.),
  `TestUsersRolesEndpointRejectsUnknownRoleCode`,
  `TestUsersRolesEndpointReturnsNotFoundForUserInAnotherCompany` (gleiches
  Cross-Tenant-Muster wie in 0.5.4.2: eigener Mandant + Nutzer per
  Direkt-SQL, da `SeedAuthUser` `company_id` fest auf `'default'` setzt),
  `TestUsersRolesEndpointIsForbiddenWithoutUsersManagePermission`.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean (keine neuen Verstöße). `go test
  ./internal/auth/... ./internal/config/...` PASS. Gezielt `go test
  ./internal/http/... -run
  "TestUsersRoles|TestUsersLock|TestUsersCreate|TestAuthLogin|TestUsersManagePermission|TestMaterialsCreateIsForbiddenForSalesRole"
  -count=1`: alle 14 Tests PASS (6 neue Rollenzuweisungs-Tests plus die 8
  bereits bestehenden Auth-Härtungs-/Regressionstests aus
  0.5.2/0.5.3/0.5.4.1/0.5.4.2).
  **Damit ist Epic 0.5 (Auth-Härtung) vollständig abgeschlossen**: 0.5.1
  (Startup-Guard gegen Default-JWT_SECRET), 0.5.2 (Rate-Limiting auf
  `/auth/login`), 0.5.3 (`users.manage`-Bypass durch dediziertes
  `admin.superuser` ersetzt), 0.5.4.1-3 (vollständige User-Management-API:
  Anlegen, Sperren/Entsperren, Rollenzuweisung — löst die im Titel von
  0.5.4 benannte Direkt-SQL-Abhängigkeit vollständig ab, `testutil.SeedAuthUser`
  bleibt als Test-only-Helper bestehen, ist aber für Produktivbetrieb nicht
  mehr die einzige Option).
- Backlog 0.6 (Pre-existing Testfehler: `purchasing.TestCreateRejectsInvalidItem`
  panict) behoben. Mit Epic 0.5 abgeschlossen bestand die restliche Arbeit
  in Epic 0 nur noch aus einer flachen, chronologisch nummerierten Liste
  unabhängiger Funde (0.6-0.35) ohne strikte Priorität außer den explizit
  als KRITISCH markierten. Fortsetzung gewählt: niedrigste offene Nummer
  zuerst (0.6) — klein, bereits im Backlog-Eintrag vollständig
  diagnostiziert (Ursache UND Fix standen schon da), risikoarm.
  `purchasing.Service.Create` (`server/internal/purchasing/service.go`)
  rief `s.pg.Begin(ctx)` auf, BEVOR die Item-Validierung
  (`MaterialID leer || Qty==0 || UOM leer` → "Ungültige Position") lief.
  `TestCreateRejectsInvalidItem` instanziiert den Service mit
  `NewService(nil)` (kein `*pgxpool.Pool`) — `s.pg.Begin(ctx)` auf nil löst
  einen `nil pointer dereference`-Panic in `pgxpool.(*Pool).Acquire` aus,
  statt den erwarteten Validierungsfehler zurückzugeben, und reißt den
  gesamten Testlauf des Pakets `purchasing` mit sich.
  Fix: neue Vorab-Prüfschleife über `in.Items`, platziert direkt nach den
  bereits bestehenden frühen Validierungen (Mandant/Lieferant/Status) und
  VOR `id := uuid.NewString()`/`s.pg.Begin(ctx)` — identische Prüflogik,
  nur vorgezogen. Die jetzt redundante Prüfung innerhalb der bestehenden
  Insert-Schleife (die erst nach `tx.Begin` läuft) entfernt, um keine tote,
  doppelte Logik im Code zu hinterlassen. Kein
  Produktivverhaltensunterschied: im echten Betrieb ist `s.pg` nie nil, nur
  die Prüfreihenfolge hat sich geändert — eine ungültige Position liefert
  weiterhin exakt "Ungültige Position" zurück, jetzt nur ohne unnötigen
  DB-Roundtrip (INSERT der Bestellung selbst) davor. Den veralteten
  Erklär-Kommentar über dem Test (der den Panic als bekannten, bewusst
  nicht behobenen Bug beschrieb, `server/internal/purchasing/service_test.go`)
  entfernt.
  Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean. `go test
  ./internal/purchasing/... -v -count=1`: alle 7 Tests PASS,
  `TestCreateRejectsInvalidItem` paniced nicht mehr. Zusätzlich
  `go test ./...` über das GESAMTE Modul (alle Nicht-Integrationspakete)
  lief ohne einen einzigen Panic durch. Regressionscheck gegen frische DB:
  `go test ./internal/http/... -run "TestPurchaseOrders" -count=1` — 3 von
  4 Tests PASS; der vierte, `TestPurchaseOrdersCreateAndGetFlow`, schlägt
  weiterhin fehl, aber per isoliertem `-run
  "^TestPurchaseOrdersCreateAndGetFlow$"`-Lauf NACHWEISLICH aus einem
  völlig anderen, bereits dokumentierten Grund (Backlog 0.21: leere
  `material_groups` auf frischer DB, Fehler tritt schon beim Anlegen des
  Test-Materials auf, lange bevor überhaupt `purchasing`-Code läuft) — kein
  Zusammenhang mit dieser Änderung.
- Backlog 0.7 (Steuerkennzeichen-Validierung in `accounting` gegen
  Stammdaten absichern) behoben. `server/internal/accounting/ar.go`s
  `taxRate()`/`taxAccountFor()` kannten hartcodiert nur `DE19`/`DE7`; jeder
  andere Wert wurde von `taxRate()` STILLSCHWEIGEND als 0% behandelt,
  `taxAccountFor()` fiel gleichzeitig auf das DE19-Konto `1776` zurück —
  ein inkonsistenter, fehlerloser Fallback, der eine falsch gebuchte USt
  unbemerkt lassen konnte. Die bereits vorhandene `tax_codes`-Tabelle
  (`017_accounting_basics.sql`, samt Seed-Daten DE19/DE7/DE0/EU-RC/RC)
  wurde dabei nie konsultiert.
  Neue `loadTaxCodes(ctx, tx pgx.Tx) (map[string]taxCodeInfo, error)`
  liest ALLE aktiven Steuerkennzeichen in einer Abfrage samt zugehörigem
  USt-Verbindlichkeitskonto (`LEFT JOIN accounts a ON a.tax_code=tc.code
  AND a.type='liability' AND a.is_active`) — das Verbindlichkeitskonto ist
  bewusst per `type='liability'` gefiltert, weil dieselbe `tax_code`-Spalte
  in `accounts` von MEHREREN Konten gleichzeitig referenziert wird (z.B.
  `DE19` sowohl von `1576` Vorsteuer/asset als auch `1776` USt/liability
  als auch `3400` Wareneingang/expense als auch `8000` Umsatzerlöse/revenue
  — ohne den Typ-Filter wäre die Zuordnung mehrdeutig).
  `taxCodeInfo{Rate, LiabilityAccount}` neuer Typ. `taxRate`/`taxAccountFor`
  bekamen die Map als ersten Parameter und einen `error`-Rückgabewert:
  leerer Code bleibt bewusst KEIN Fehler (0%, z.B. Skonto-/Durchlaufposten
  ohne Steuerkennzeichen — dieser Fall war schon vorher gewollt, siehe
  bestehender Test `TestCalcTotalsSumsNetAndTax`), ein nicht-leerer, aber
  unbekannter ODER inaktiver Code liefert jetzt einen Fehler statt still
  auf 0% zurückzufallen; `taxAccountFor` liefert ebenfalls einen Fehler,
  wenn der Code zwar bekannt ist, aber KEIN Liability-Konto konfiguriert
  wurde (z.B. `DE0`/`EU-RC`/`RC` in den aktuellen Seed-Daten haben keine
  zugeordnete `accounts`-Zeile).
  `calcTotals`/`buildJournal`/`buildStornoJournal` (zuvor pure Funktionen
  ohne jeden DB-Bezug) bekamen dieselbe Map als zusätzlichen Parameter
  sowie eine `error`-Rückgabe, BLEIBEN dabei aber weiterhin pure, synchron
  testbare Funktionen ohne eigenen DB-Zugriff — die eigentliche
  DB-Konsultation passiert bewusst NUR EINMAL pro Transaktion ganz am
  Anfang (`loadTaxCodes`), nicht verteilt über jeden einzelnen Helper-
  Aufruf (vermeidet N+1-Abfragen pro Rechnungsposition). Alle drei
  Aufrufstellen angepasst: `createTx` (ruft `loadTaxCodes` einmal vor der
  Summenbildung UND vor der Item-Insert-Schleife auf, die den Steuerbetrag
  jetzt ueber den zurückgegebenen `rate`-Wert statt direkt `taxRate(...)`
  berechnet), `Book` (vor `buildJournal`), `Storno` (vor
  `buildStornoJournal`) — jeweils innerhalb der bereits bestehenden
  Transaktion, kein zusätzlicher Commit/Rollback-Pfad nötig.
  Tests: `TestTaxRateKnownAndUnknownCodes`/`TestTaxAccountForKnownAndUnknownCodes`
  umgeschrieben (erwarten jetzt Fehler für unbekannte/fehlkonfigurierte
  Codes statt stillen Fallback, neue `testTaxCodes()`-Fixtur simuliert die
  Stammdaten ohne echte DB-Verbindung), `TestCalcTotalsSumsNetAndTax`/
  `TestBuildJournalProducesBalancedEntry`/
  `TestBuildStornoJournalReversesDebitAndCreditButStaysBalanced` an die
  neuen Signaturen angepasst (Map + Fehlerbehandlung ergänzt, sonst
  unverändertes Verhalten), neuer Test `TestCalcTotalsRejectsUnknownTaxCode`.
  Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `go test
  ./internal/accounting/... -v -count=1` (alle 20 Tests PASS), voller `go
  test ./...` über alle Nicht-Integrationspakete ohne einen einzigen
  Fehler. Regressionscheck gegen frische, per `\dt` bestätigt leere DB:
  `go test ./internal/http/... -run "TestInvoiceOut" -count=1` —
  `TestInvoiceOutFlowWithPDFAndPayments` und `TestInvoiceOutStornoFlow`
  beide PASS, beweisen die komplette Kette Anlegen→Buchen→Storno mit den
  TATSÄCHLICH aus Postgres gelesenen Steuerkennzeichen (nicht nur mit der
  Test-Fixtur `testTaxCodes()`).
  **Neuer, verwandter Fund bei der Umsetzung, bewusst NICHT mitgefixt**:
  per `grep -rn "taxRate("` über das gesamte Modul entdeckt, dass
  `internal/quotes/service.go` und `internal/sales/service.go` JEWEILS
  eine eigene, unabhängige Kopie derselben hartcodierten
  `DE19`/`DE7`-Switch-Funktion mit demselben stillen 0%-Fallback besitzen
  (kein Shared-Package zwischen den drei Domänen) — der ursprüngliche
  Backlog-0.7-Fund war explizit nur auf `accounting/ar.go` bezogen. Als
  neues, eigenständiges Backlog 0.36 dokumentiert statt hier stillschweigend
  mitgefixt (deutlich größerer Umfang: zwei weitere Domänen, mehr
  Aufrufstellen, und die sauberste Lösung wäre vermutlich ein gemeinsames
  Package statt dreifacher Code-Duplikation — eine Design-Entscheidung, die
  bei Bearbeitung von 0.36 getroffen werden sollte, nicht hier).
- Backlog 0.8 (Integrationstests für die zentralen Zahlungs-Guards in
  `payments.go` ergänzen) umgesetzt. `PaymentService.apply()`
  (`server/internal/accounting/payments.go:47ff`) prüft drei fachlich
  zentrale Regeln erst NACH dem `tx.QueryRow`-Laden der Rechnung
  (Statusguard, Währungsabgleich, Überzahlungsschutz) — ohne echte
  DB-Transaktion nicht unit-testbar (kein Mock/Testcontainer im Repo,
  siehe ADR 0001) und bisher komplett ungetestet: die einzige bestehende
  Integrationstest (`TestInvoiceOutFlowWithPDFAndPayments`) deckte nur den
  Happy-Path (eine einzelne, gültige Teilzahlung) ab.
  Neuer Integrationstest `TestInvoiceOutPaymentGuardsRejectInvalidPayments`
  (`server/internal/http/accounting_integration_test.go`), aufgebaut nach
  demselben Muster wie die bestehenden Rechnungs-Tests (Kontakt anlegen,
  Rechnung mit `DE19`-Position anlegen: net 300 + USt 57 = gross 357):
  (1) Zahlung auf die noch nicht gebuchte (`draft`) Rechnung → `400`
  "Rechnung ist nicht gebucht" (Statusguard greift VOR jeder anderen
  Prüfung); (2) nach dem Buchen eine Zahlung in `USD` statt der
  Rechnungswährung `EUR` → `400` "Währung stimmt nicht mit Rechnung
  überein"; (3) eine Zahlung über 1000 (bei offenem Betrag 357) → `400`
  "Zahlung übersteigt offenen Betrag". Bewusst wird nicht nur der
  Statuscode geprüft, sondern die EXAKTE Fehlermeldung aus dem
  Response-Body dekodiert und verglichen — das ist der eigentliche Sinn
  dieser Subtask laut Backlog-Titel ("diese drei Fehlermeldungen wird dort
  geprüft"), ein reiner 400-Statuscode-Check hätte auch einen ganz anderen
  Validierungsfehler durchgehen lassen.
  Abschließender Regressionscheck: eine gültige Zahlung (100 EUR,
  innerhalb des offenen Betrags) wird TROTZ der drei vorherigen
  Ablehnungen weiterhin akzeptiert (`201`) — beweist, dass die drei Guards
  wirklich nur die ungültigen Fälle blockieren und den Normalfall nicht
  versehentlich mit betreffen.
  Keine Code-Änderung an `payments.go`/`v1.go`/`classifyDomainError`
  nötig: alle drei Fehlermeldungen waren bereits über bestehende
  Substring-Muster ("nicht gebucht", "stimmt nicht", "übersteigt")
  korrekt auf `400 validation_error` gemappt — reine Testergänzung, exakt
  wie im Backlog-Eintrag als Fix vorgesehen ("Fix: neue Fälle in
  `accounting_integration_test.go` ... ergänzen").
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean. `go test ./internal/http/... -run
  "TestInvoiceOut" -count=1`: alle 3 Tests PASS (der neue Guard-Test plus
  die 2 bestehenden `TestInvoiceOutFlowWithPDFAndPayments`/
  `TestInvoiceOutStornoFlow` als Regressionscheck, unverändert erfolgreich).
- Backlog 0.9 (Integrationstests für Bankabgleich/-matching ergänzen,
  `bank.go`) umgesetzt.
  **Wichtige Erkenntnis VOR der Umsetzung**: `grep -rln "NewBankService"`
  über das gesamte `server/`-Modul findet NUR `server/internal/accounting/bank.go`
  selbst — `BankService` wird nirgendwo (weder `v1.go` noch anderswo)
  konstruiert oder an eine Route gebunden. Das komplette, ansonsten fertig
  implementierte Bankabgleich-Feature (Kontoauszug-Import, manuelles/
  automatisches Matching, Betrags-Heuristik, Referenz-Erkennung) ist über
  die API damit AKTUELL UNERREICHBAR. Als eigenständiger, größerer Fund neu
  dokumentiert (Backlog 0.37) statt in dieser reinen Test-Subtask
  mitgelöst — das Wiring bräuchte mindestens neue Routen UND eine
  Entscheidung zur Berechtigungsmodellierung (neue Permissions?), das
  sprengt den Rahmen von "Integrationstests ergänzen".
  Da kein HTTP-Pfad existiert, sind die neuen Tests zwangsläufig
  service-level: direkter Aufruf von `BankService`/`ARService` gegen
  echtes Postgres statt über `httptest`/HTTP-Handler — exakt das bereits
  etablierte Muster für HTTP-lose Domänen aus
  `internal/hr/service_scoping_integration_test.go` (dort mit
  demselben Begründungs-Kommentar-Stil übernommen).
  Neue Datei `server/internal/accounting/bank_integration_test.go`, 6
  Tests, decken exakt den im Backlog-Eintrag benannten Umfang ab:
  `TestBankIngestPersistsStatementAndAppliesPaymentWhenInvoiceIDProvided`
  (Ingest plus automatische Zahlungsanwendung, wenn `invoice_id` direkt
  mitgegeben wird), `TestBankMatchRejectsAlreadyMatchedStatement`
  ("Statement bereits gematcht"-Schutz: zweiter `Match`-Aufruf auf
  dasselbe Statement wird abgelehnt), `TestBankMatchFindsInvoiceByReferenceNumber`
  (beweist per zwei Rechnungen mit IDENTISCHEM offenem Betrag, dass der
  Referenz-Erkennungs-Pfad VOR der Betrags-Heuristik greift — ohne diese
  Priorität wäre der Match zwingend mehrdeutig gewesen),
  `TestBankMatchFindsInvoiceByAmountWhenReferenceHasNoMatch` (Fallback auf
  die Betrags-Heuristik, wenn die Referenz keine erkennbare
  Rechnungsnummer enthält), `TestBankMatchReturnsErrorForAmbiguousAmount`
  (zwei offene Rechnungen mit identischem Betrag, keine brauchbare
  Referenz → "Mehrere mögliche offene Posten gefunden, bitte manuell
  zuordnen"), `TestBankMatchReturnsErrorWhenNoInvoiceMatches` (Betrag ohne
  jeden passenden offenen Posten → "Kein passender offener Posten
  gefunden").
  **Zwei genuine, vorher unbekannte Bugs beim Schreiben dieser Tests
  gefunden UND behoben** — beide direkt in den durch diese Subtask zu
  testenden Funktionen selbst (anders als z.B. Backlog 0.34/0.35, die in
  FREMDEN Funktionen bei ganz anderer Gelegenheit auftauchten und deshalb
  bewusst nur dokumentiert wurden; hier ist der Bug der eigentliche
  Gegenstand der Subtask, ein Fix macht die neuen Tests erst aussagekräftig
  statt nur "beweist, dass alles kaputt ist"):
  1. `Ingest()`: `var raw any = in.Raw; if raw == nil { raw = map[string]any{} }`
     griff NIE — klassische Go-"typed nil in interface"-Falle: `in.Raw` hat
     den statischen Typ `map[string]any`; wird eine nil-Map in ein
     `any`-Interface verpackt, hat das resultierende Interface einen
     gesetzten TYP (auch wenn der WERT nil ist) und ist damit `!= nil`.
     Jeder Ingest ohne explizit gesetztes `Raw`-Feld (der Normalfall bei
     manuell erfassten Kontoauszügen) blieb dadurch bei einer nil-Map, die
     pgx als SQL NULL sendet — Verletzung von `bank_statements.raw NOT
     NULL` bei praktisch JEDEM Ingest-Aufruf ohne Rohdaten. Fix: den
     Nil-Check auf die Map VOR dem Verpacken in ein Interface anwenden
     (`raw := in.Raw; if raw == nil { ... }`).
  2. `findInvoiceIDInReference()`: das erste Regex-Muster
     (`re[-\s]?(\d{2,4}[-/]?\d{2,6})`) erkennt ein "RE"-Präfix im Text, die
     Capture-Gruppe `m[1]` fängt aber nur den NACHFOLGENDEN Zahlenteil ein
     — der DB-Lookup suchte dann exakt nach diesem Zahlenteil OHNE das
     Präfix. `invoices_out.nummer` enthält laut Nummernkreis-Pattern
     (`017_accounting_basics.sql`: `'RE-{YYYY}-{NNNN}'`) aber IMMER das
     volle Präfix (z.B. "RE-2026-0001"). Der Referenz-Erkennungs-Pfad fand
     dadurch NIE eine reale Rechnung und fiel bei jedem Match-Versuch
     sofort auf die Betrags-Heuristik zurück — ein Feature, das laut
     Doc-Kommentar ("find by invoice number in reference") explizit den
     gegenteiligen Zweck hatte. Fix: das Präfix pro Muster mitführen
     (Umbau von `[]*regexp.Regexp` auf `[]struct{rx *regexp.Regexp; prefix
     string}`) und beim DB-Lookup wieder voranstellen.
  **Dritter Bug gefunden, bewusst NICHT hier mitgefixt**: beim ersten
  Versuch, eine Test-Rechnung mit einer Position OHNE Steuerkennzeichen
  anzulegen, schlug `arSvc.Create` mit `ERROR: insert or update on table
  "invoice_out_items" violates foreign key constraint
  "invoice_out_items_tax_code_fkey"` fehl. `createTx()`
  (`server/internal/accounting/ar.go`, zuletzt in Backlog 0.7 bearbeitet)
  übergibt `it.TaxCode` (ein Go-`string`, kein `*string`) direkt als
  Parameter für die nullable Spalte `invoice_out_items.tax_code text
  REFERENCES tax_codes(code)` — bei `TaxCode: ""` wird die LEERE
  ZEICHENKETTE eingefügt statt SQL NULL, was den Fremdschlüssel verletzt
  (kein `tax_codes`-Eintrag mit `code=''`). Widerspricht dem Verhalten der
  darüberliegenden Funktionen (`calcTotals`/`buildJournal`/`taxRate`
  behandeln einen leeren Code seit Backlog 0.7 explizit als gültigen
  "steuerfrei"-Fall). Da dieser Bug in `ar.go` liegt, nicht in `bank.go`
  (dem eigentlichen Gegenstand dieser Subtask), wurde er NICHT hier
  gefixt, sondern in den neuen Bank-Tests umgangen (`TaxCode: "DE19"`
  statt leer) und als eigenständiges Backlog 0.38 dokumentiert.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean (inkl. einer kuriosen gofmt-eigenen
  Geraden-zu-Kurvenanführungszeichen-Normalisierung in einem
  Kommentar — rein kosmetisch, keine Verhaltensänderung). `go test
  ./internal/accounting/... -v -count=1`: alle 26 Tests im Paket PASS
  (inkl. der 6 neuen Bank-Tests sowie aller bereits bestehenden Journal-/
  AR-/Payment-Tests als Regressionscheck). Vollständiger `go test ./...`
  über alle Nicht-Integrationspakete ohne einen einzigen Fehler.
- Backlog 0.10 (`EmployeeService.Update()` soll unbekannte Patch-Keys
  ablehnen statt still zu ignorieren) behoben.
  `server/internal/hr/service.go`s `Update()` übernahm bisher nur eine
  feste Whitelist von Patch-Keys (`first_name`, `last_name`, `email`,
  `phone`, `role`, `location`, `cost_center`, `active`, `team_id`) über
  einen `switch` OHNE `default`-Fall in die SQL-`SET`-Klausel. Jeder andere
  Key (z.B. ein Tippfehler wie `activ` statt `active`) fiel dadurch
  stillschweigend durch den `switch` und wurde verworfen — der Aufruf
  kehrte erfolgreich (kein Fehler) zurück, obwohl in der DB NICHTS
  geändert wurde. Ein trügerisches, stilles Fehlschlagen genau der Art,
  die am schwersten zu debuggen ist (kein Fehler im Log, aber der Effekt
  bleibt aus).
  Neue package-level `employeeUpdatableFields`-Whitelist
  (`map[string]bool`). `Update()` iteriert jetzt ZUERST über alle
  Patch-Keys und prüft jeden einzelnen gegen diese Whitelist, BEVOR
  irgendetwas an der SQL-`SET`-Klausel gebaut oder die DB angefasst wird —
  findet sich auch nur EIN unbekannter Key, wird der GESAMTE Patch mit
  `fmt.Errorf("unbekanntes Feld: %s", k)` abgelehnt (alles-oder-nichts,
  kein teilweises Anwenden der im selben Patch enthaltenen bekannten
  Felder). Die drei vorher IDENTISCHEN `switch`-Case-Zweige (die
  `first_name`/... von `active` von `team_id` künstlich in drei separate
  `case`-Arme aufteilten, obwohl jeder exakt denselben Code ausführte) auf
  eine einzige Schleife vereinfacht — rein kosmetisch/lesbarkeitsverbessernd,
  keine Verhaltensänderung. Der danach unerreichbare
  `if len(sets) == 0 { return nil }`-Nachlauf-Check entfernt: nach der
  neuen Vorab-Validierung ist `sets` bei einem nicht-leeren Patch
  garantiert nicht-leer (jeder Key hat entweder einen `SET`-Eintrag
  erzeugt oder die Funktion ist bereits vorher mit einem Fehler
  zurückgekehrt).
  Per `grep -rn "patch map\[string\]any"` über das GESAMTE `server/`-Modul
  geprüft, ob dasselbe generische Patch-Map-Muster (wie im Backlog-Eintrag
  angeregt) auch in anderen `Update`-Handlern vorkommt — Ergebnis: NEIN,
  dieses Muster ist auf `EmployeeService.Update` beschränkt, ein isolierter
  Einzelfall, keine weiteren Domänen betroffen.
  Tests: bestehender `TestUpdateWithOnlyUnknownKeysIsSilentNoOp`
  (`server/internal/hr/service_test.go`) in `TestUpdateRejectsUnknownKeys`
  umbenannt und in sein Gegenteil verkehrt (erwartete vorher `nil`-Fehler,
  erwartet jetzt einen Fehler). Neuer Test
  `TestUpdateRejectsPatchWithAnySingleUnknownKey`: ein gemischter Patch mit
  GENAU EINEM bekannten (`first_name`) und EINEM unbekannten (`activ`) Key
  wird komplett abgelehnt, nicht nur der unbekannte Teil verworfen. Neuer
  Integrationstest `TestEmployeeUpdateAppliesKnownFieldsAndRejectsUnknownKey`
  (`server/internal/hr/service_scoping_integration_test.go`, service-level
  gegen echtes Postgres — `hr` hat wie schon in 0.9 bei `bank.go` festgestellt
  keine HTTP-Anbindung) beweist END-ZU-ENDE: (1) ein Patch mit
  ausschließlich bekannten Feldern (`first_name`, `active`) wird korrekt
  angewendet, per `Get()` nachgeprüft; (2) ein anschließender Patch, der
  `first_name` erneut ändern will UND einen unbekannten Key (`activ`
  statt `active`) enthält, wird komplett abgelehnt — UND per erneutem
  `Get()` bestätigt, dass `first_name` dabei NICHT trotzdem geändert wurde
  (der eigentliche Beweis des Alles-oder-nichts-Verhaltens am tatsächlichen
  DB-Zustand, nicht nur an der Fehlerrückgabe der Validierungsschicht).
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean. `go test ./internal/hr/... -v
  -count=1`: alle Tests PASS. Vollständiger `go test ./...` über alle
  Nicht-Integrationspakete ohne Fehler.
- Backlog 0.11 (`LeaveService.Create()`: Tage-Berechnung kann bei
  vertauschten Daten ≤ 0 ergeben) behoben.
  `server/internal/hr/service.go`s `Create()` berechnete `Days` (falls
  nicht explizit gesetzt) aus `EndDate.Sub(StartDate).Hours()/24 + 1` OHNE
  vorherige Prüfung, dass `EndDate` tatsächlich nach `StartDate` liegt. Bei
  vertauschten Daten (z.B. `StartDate`=2026-08-20, `EndDate`=2026-08-18)
  ergab die Formel `-1` und wurde ungeprüft in `hr_leave_requests.days`
  geschrieben — ein negativer Urlaubstage-Wert in der Datenbank, fachlich
  unsinnig und potenziell folgenreich (z.B. falls dieser Wert später in
  eine Urlaubskonto-Saldo-Berechnung einfließt).
  Neue Prüfung `if lr.EndDate.Before(lr.StartDate) { return nil,
  errors.New("Enddatum darf nicht vor Startdatum liegen") }`, platziert
  DIREKT nach der bestehenden `StartDate.IsZero() ||
  EndDate.IsZero()`-Prüfung — also VOR dem `employeeOwned`-DB-Zugriff, der
  als nächstes folgt. Diese Platzierung hat einen angenehmen
  Testbarkeits-Nebeneffekt: `Create()` ist für genau dieses Szenario jetzt
  (anders als vorher) DIREKT mit `NewLeaveService(nil)` aufrufbar, ohne
  dass die Funktion auf dem Weg dorthin auf den nil-Pool zugreift und
  panict (dasselbe Grundmuster wie der in Backlog 0.6 behobene
  `purchasing`-Panic — hier aber bereits vorher korrekt vermieden, da die
  neue Prüfung VOR jedem DB-Zugriff sitzt, nicht danach).
  Eintägige Urlaubsanträge (`StartDate == EndDate`) bleiben bewusst gültig:
  `EndDate.Before(StartDate)` liefert für gleiche Zeitpunkte `false`, die
  Formel ergibt dafür korrekt `Days=1`.
  Tests: der bestehende
  `TestLeaveCreateDaysFormulaCanProduceNonPositiveDaysForInvertedDateRange`
  (`server/internal/hr/service_test.go`) hatte den Fund bisher nur indirekt
  belegen können — er reproduzierte die Formel `end.Sub(start).Hours()/24 +
  1` manuell als reine Zeitarithmetik, weil `Create()` selbst mit nil-Pool
  an dieser Stelle vorher gepanict hätte (kein DB-Zugriff-Schutz vor der
  Formel). Ersetzt durch `TestLeaveCreateRejectsEndDateBeforeStartDate`,
  das jetzt DIREKT `Create()` mit vertauschten Daten aufruft und die neue,
  konkrete Fehlermeldung prüft — ein direkterer, aussagekräftigerer Beweis
  als die vorherige Formel-Nachstellung.
  Neuer Integrationstest `TestLeaveCreateComputesDaysForValidDateRange`
  (`server/internal/hr/service_scoping_integration_test.go`, service-level
  gegen echtes Postgres, da `hr` keine HTTP-Anbindung hat) beweist, dass
  die neue Prüfung den NORMALFALL nicht versehentlich mitblockiert: ein
  regulärer 5-Tage-Zeitraum (1.-5. September) liefert weiterhin korrekt
  `Days=5`, ein eintägiger Antrag liefert korrekt `Days=1` — ein reiner
  Unit-Test der Validierungsschicht allein hätte eine Regression im
  Erfolgspfad (z.B. eine versehentlich zu strenge Bedingung, die auch
  gültige Bereiche ablehnt) nicht aufgedeckt.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean. `go test ./internal/hr/... -v
  -count=1`: alle Tests PASS (inkl. beider neuer Tests sowie aller
  bestehenden `hr`-Tests als Regressionscheck, u.a. der in Backlog 0.10
  gerade erst hinzugefügten `EmployeeService.Update`-Tests). Vollständiger
  `go test ./...` über alle Nicht-Integrationspakete ohne Fehler.
- Backlog 0.12 (`LeaveService.Create()`: keine Überschneidungsprüfung für
  Urlaubsanträge) behoben — letzte offene Position aus dem Block 0.6-0.12,
  der bei den Test-Absicherungs-Subtasks 0.1.1.x-0.1.3.x gefunden wurde.
  Bereits in `docs/00-recon.md`/`docs/01-gap-analysis.md` (Domäne F) als
  fachliche Lücke vermerkt: `Create()` fragte vor dem Insert keine
  bereits bestehenden `hr_leave_requests` desselben Mitarbeiters ab — zwei
  (oder beliebig viele) sich überschneidende Urlaubsanträge waren möglich,
  selbst wenn beide bereits genehmigt waren. Da die Prüfung zwingend eine
  Datenbankabfrage braucht, war sie schon im ursprünglichen Fund als "nur
  integrationstestbar" markiert, und bei Subtask 0.1.3.3 bewusst NICHT
  nachgebaut worden (das wäre Implementierung einer fehlenden fachlichen
  Regel gewesen, nicht nur ein Test für eine vorhandene Regel — korrekt
  als Scope-Creep vermieden und stattdessen als eigene Backlog-Position
  0.12 verewigt).
  Neue Prüfung in `server/internal/hr/service.go`s `Create()`, platziert
  NACH dem `employeeOwned`-Check (der Mitarbeiter muss ohnehin schon als
  existent bestätigt sein, bevor eine Überschneidung mit SEINEN eigenen
  Anträgen ueberhaupt sinnvoll geprüft werden kann) und VOR dem eigentlichen
  Insert: `SELECT EXISTS(SELECT 1 FROM hr_leave_requests WHERE
  employee_id=$1 AND status IN ('pending','approved') AND start_date <=
  $2 AND end_date >= $3)` — der Standard-Algorithmus für
  Intervall-Überlappung (zwei Intervalle [a,b] und [c,d] überschneiden
  sich genau dann, wenn a<=d UND c<=b).
  Bewusste fachliche Entscheidung bei der Statusauswahl: nur `pending` UND
  `approved` Anträge zählen als blockierend, `rejected` bewusst NICHT — ein
  bereits abgelehnter Antrag darf den Zeitraum nicht dauerhaft für neue
  Anträge sperren (sonst könnte ein einziger, versehentlich abgelehnter
  Antrag einen Mitarbeiter faktisch aussperren, den Zeitraum je wieder zu
  beantragen). Diese Unterscheidung war im ursprünglichen Backlog-Eintrag
  nicht explizit vorgegeben, ergibt sich aber zwingend aus der
  Tabellenspalte `status text NOT NULL DEFAULT 'pending' -- pending |
  approved | rejected` (`020_hr_base.sql`) und der fachlichen Logik der
  Domäne.
  Test (wie im Backlog-Eintrag selbst schon korrekt antizipiert: nur
  integrationstestbar, da die Regel zwingend einen DB-Zugriff braucht):
  neuer `TestLeaveCreateRejectsOverlappingDateRange`
  (`server/internal/hr/service_scoping_integration_test.go`) deckt alle
  VIER fachlich relevanten Fälle in einem zusammenhängenden Szenario ab:
  (1) ein neuer Antrag, der sich mit einem bestehenden `pending` Antrag
  überschneidet, wird abgelehnt; (2) ein direkt ANSCHLIESSENDER, NICHT
  überlappender Zeitraum (beginnt exakt einen Tag nach dem Ende des
  ersten Antrags) bleibt weiterhin erlaubt — wichtiger Grenzfalltest für
  die Intervall-Arithmetik, der eine zu grosszügige "off-by-one"-Prüfung
  aufgedeckt hätte; (3) nachdem der erste Antrag genehmigt (`approved`)
  wurde, blockiert er Überschneidungen GENAUSO wie zuvor im `pending`-
  Zustand — entspricht direkt der im Backlog-Eintrag ausdrücklich
  genannten Sorge "auch mehrfach genehmigte"; (4) ein separat angelegter
  und dann per `Approve(..., approve=false)` ABGELEHNTER Antrag blockiert
  NICHT mehr — derselbe Zeitraum kann danach erfolgreich erneut beantragt
  werden.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build ./...`,
  `go vet ./...`, `gofmt -l` clean. `go test ./internal/hr/... -v
  -count=1`: alle Tests PASS (inkl. aller bereits bestehenden `hr`-Tests
  aus 0.10/0.11 als Regressionscheck). Vollständiger `go test ./...` über
  alle Nicht-Integrationspakete ohne Fehler.
  **Damit sind alle Epic-0-Funde 0.6-0.12 vollständig abgearbeitet** — der
  gesamte Block an Mängeln, der bei den Test-Absicherungs-Subtasks
  0.1.1.x-0.1.3.x dieser Session entdeckt wurde. Als nächstes stehen die
  numerisch niedrigeren, aber SPÄTER (bei anderen Subtasks) gefundenen
  Positionen 0.13/0.14/0.15 an, bevor die höher nummerierten 0.20-0.38 an
  der Reihe wären.
- Backlog 0.14 als abgeschlossen markiert (reine Statuskorrektur, auf
  explizite Nutzeranfrage "untersuchen, ob noch WIP"). Befund: alle drei
  Unterpunkte waren bereits `[x]`, der eigene Text hielt bereits "Kernziel
  dieser Backlog-Position erreicht" fest (`docker compose up` auf leerer DB
  funktioniert) — nur die Top-Level-Checkbox und das Label "wip,
  PRIORISIERT VOR 0.2.1.2.2+" waren stehengeblieben, obwohl die
  Priorisierungsbedingung (Task 0.2.1.2.2) längst erledigt ist. Statt den
  alten Notizen zu vertrauen, wurde vor der Statusänderung erneut gegen
  eine WIRKLICH frische, leere DB verifiziert: `docker compose -f
  docker-compose.test.yml down -v` + `up --wait`, `\dt` bestätigt leer,
  danach ein Testlauf, der `migrate.Run` durchlaufen laesst (Migrationen
  001-064, inkl. aller in dieser Session neu hinzugekommenen bis 064) —
  lief vollstaendig fehlerfrei durch. Derselbe Codepfad
  (`server/internal/migrate/migrate.go`) wird auch vom echten
  Server-Start (`internal/app/server.go:56`) verwendet, die Verifikation
  ist also eine echte Aussage über `docker compose up`, nicht nur über den
  Testharness. Zusaetzlich die drei separat dokumentierten Folgefunde
  0.16/0.17/0.18 (waren nie Teil von 0.14s eigenem Scope, sondern schon
  vorher bewusst ausgelagert) gegen dieselbe frische DB erneut reproduziert
  — alle drei schlagen weiterhin fehl, sind also keine veralteten
  Karteileichen, sondern nach wie vor akkurat als offen dokumentiert.
  Kein Code geändert, nur `docs/backlog.md`/`docs/state.md` aktualisiert.
- Backlog 0.13 (Migrationsrunner unterstützt keine Down-Migrationen)
  behoben — auf explizite Nutzeranweisung nach 0.14 ("mit dem nächsten
  Backlog Eintrag fortfahren"). Größter, riskantester Einzeleingriff dieser
  Session bislang: betrifft die Kernmechanik, von der buchstäblich JEDER
  Integrationstest im gesamten Modul abhängt.
  `server/internal/migrate/migrate.go` führte zuvor jede `.sql`-Datei bei
  JEDEM `Run()`-Aufruf erneut aus — kein Versions-Tracking, keine
  Down-Skripte, kein Rollback-Mechanismus. Das erfüllte aufgabe.md §7.8
  ("Keine Migration ohne Down-Pfad") nur ueber Kommentar-Dokumentation, nicht
  automatisiert, UND war die direkte Ursache von Backlog 0.20 (Migration 050
  fuegt einen Constraint hinzu, 051 entfernt ihn im selben Durchlauf wieder
  - bei jedem erneuten `Run()` gegen dieselbe DB versuchte 050 den
  Constraint erneut anzulegen, was fehlschlug, sobald zwischenzeitlich
  Daten eingefuegt wurden, die die urspruengliche Regel verletzten).
  **Entscheidung dokumentiert in neuer ADR
  `docs/adr/0005-migration-versioning-and-down-migrations.md`**: Option B
  aus drei erwogenen (A: vollstaendiges Up/Down-Tool mit Migrationswechsel,
  zu gross; B: nur Versions-Tracking, Down bleibt manuell/dokumentiert -
  gewaehlt; C: nichts aendern, nur dokumentieren - verworfen, da 0.20 damit
  ungeloest bliebe). Neue Tabelle `schema_migrations (filename PRIMARY KEY,
  applied_at)`, idempotent angelegt (kein separates Migrationsfile - vermeidet
  das Henne-Ei-Problem, dass die Tracking-Tabelle nicht per getrackter
  Migration entstehen kann, bevor das Tracking existiert). `Run()` liest die
  bereits angewendeten Dateinamen, ueberspringt sie, fuehrt jede NEUE
  Migration atomar zusammen mit ihrem `schema_migrations`-Eintrag in EINER
  Transaktion aus (kein Zwischenzustand moeglich). Down-Migrationen bleiben
  bewusst manuell (Kommentarblock-Konvention, bereits gelebte Praxis seit
  ~Migration 054, jetzt als verbindlicher Standard fuer neue Migrationen mit
  strukturellem Risiko festgehalten) - keine rueckwirkenden Down-Kommentare
  fuer die bestehenden 001-064, kein externes Migrationstool.
  **Kritischer Fund WAEHREND der eigenen Verifikation, sofort erkannt und
  behoben** (nicht als separate Backlog-Position dokumentiert, da es
  innerhalb derselben Subtask sofort behoben wurde - anders als Funde in
  FREMDEM Code): beim Testen mehrerer Pakete gleichzeitig
  (`go test ./internal/accounting/... ./internal/hr/... ./internal/auth/...
  ./internal/purchasing/... ./internal/config/... ./internal/migrate/...`
  - Standard-Parallelverhalten von `go test` ueber mehrere Pakete hinweg,
  jedes Paket sein eigenes Testbinary, standardmaessig nebenlaeufig)
  schlugen `hr`- und `migrate`-Tests mit `ERROR: duplicate key value
  violates unique constraint "schema_migrations_pkey"` bzw. direkten
  DDL-Konflikten (`pg_type_typname_nsp_index`) fehl. Ursache: mehrere
  Prozesse riefen `testutil.SetupIntegrationEnv` (-> `migrate.Run`)
  gleichzeitig gegen dieselbe Test-DB auf, lasen denselben
  "noch-nicht-angewendet"-Zustand, und versuchten dieselbe Migration
  PARALLEL auszufuehren - eine klassische Race Condition zwischen "geprueft"
  und "markiert", die die alte, ungetrackte Implementierung nie hatte
  (sie fuehrte ohnehin bei jedem Aufruf alles erneut aus, "Race" war
  bedeutungslos, da es keinen geteilten Zustand gab, um den man
  konkurrieren koennte).
  Fix: `pg_advisory_lock`/`pg_advisory_unlock` um den GESAMTEN
  `Run()`-Ablauf. Wichtige Implementierungsdetail: Advisory Locks sind
  SESSION-, nicht transaktionsgebunden - ein `*pgxpool.Pool` allein reicht
  dafuer nicht (pgxpool rotiert Verbindungen zwischen einzelnen
  Exec/Query-Aufrufen), daher `pool.Acquire(ctx)` fuer eine EXPLIZITE
  Einzelverbindung, auf der Lock, Migrationen UND Unlock alle gemeinsam
  laufen, per `defer conn.Release()` am Ende freigegeben. Serialisiert
  nebenlaeufige `Run()`-Aufrufe: ein zweiter Prozess wartet (blockierend,
  kein Busy-Fail), bis der erste fertig ist, und ueberspringt danach alles
  (bereits als angewendet markiert). Nach dem Fix mehrfach mit erhoehter
  Parallelitaet verifiziert (`-p 8`, dann `-p 16` ueber 9 Pakete
  gleichzeitig inkl. `quotes`/`sales`/`contacts`) - keine
  schema_migrations- oder DDL-Konflikte mehr, einzige verbleibende
  Fehlschlaege die bereits bekannten, unabhaengigen `contacts.telefon`-Funde
  (0.23/0.27).
  **Backlog 0.20 (KRITISCH) als Nebeneffekt vollstaendig geloest**: mit
  Versions-Tracking kann der 050/051-Konflikt strukturell nicht mehr
  auftreten (beide laufen nur noch EINMAL). Verifiziert per komplettem `go
  test ./internal/http/... -count=1`-Lauf gegen frische DB: die zuvor ca.
  70 von ~75 kaskadierenden Fehlschlaegen sind auf 27 zurueckgegangen -
  reproduzierbar identisch bei zwei aufeinanderfolgenden vollen Laeufen.
  Jeder einzelne der 27 verbleibenden Fehlschlaege wurde per gezieltem
  `-run`-Filter samt Fehlermeldung geprueft und auf eine BEREITS
  dokumentierte Ursache zurueckgefuehrt: `contacts.telefon` (0.16-0.19 im
  Kontext), `Ungueltige Materialkategorie` (0.21, betrifft 6 weitere
  Quote-Material-Tests, die bisher nie so weit kamen), `reason_code`-
  Mismatch (0.28, betrifft 3 weitere Approval-Tests), `classifyDomainError`
  erkennt Validierungsfehler nicht (0.29-Muster, GAEB-Importfehler UND ein
  analoger Sales-Order-Fall), `invoice_out_items_tax_code_fkey` (0.38, neu
  aus der letzten Runde), `FOR UPDATE`+`LEFT JOIN` (0.35), `conn busy`
  (0.34), falscher Upload-Pfad im Test (0.26). NUR EIN einziger, wirklich
  neuer Fund blieb übrig: `TestMaterialGroupDeleteRejectsTrimmedLegacyReferences`
  referenziert per Direkt-SQL-Fixture eine nicht existierende Spalte
  `materials.updated_at` - per isoliertem `-run`-Lauf UND per `git show
  HEAD` bestaetigt vorbestehend (identische SQL-Zeile bereits im letzten
  Commit), nur bisher NIE bis zu diesem Fehler durchgedrungen, weil die
  0.20-Kaskade den vollen Testlauf zuvor immer schon vorher abbrach. Als
  neues Backlog 0.39 dokumentiert, bewusst NICHT hier mitgefixt (ausserhalb
  des auf den Migrationsrunner beschraenkten Scopes von 0.13).
  Tests: neue Datei `server/internal/migrate/migrate_test.go` - ERSTMALS
  Tests fuer dieses Paket ueberhaupt. Bewusst als externes Testpaket
  `package migrate_test` (nicht `package migrate`) deklariert, um einen
  Importzyklus zu vermeiden: `testutil.SetupIntegrationEnv` importiert
  bereits `migrate`, ein `package migrate`-Testfile haette `testutil` nicht
  importieren koennen, ohne einen Zyklus zu erzeugen.
  `TestRunTracksEveryMigrationFile` (Zeilenanzahl in `schema_migrations`
  entspricht exakt der per `os.ReadDir` dynamisch ermittelten Anzahl
  `.sql`-Dateien - kein hartcodierter Wert, der bei jeder neuen Migration
  angepasst werden muesste), `TestRunIsIdempotentOnRepeatedInvocation`
  (zwei weitere `Run()`-Aufrufe gegen dieselbe, bereits migrierte DB
  veraendern die Zeilenanzahl nicht - ist die Skip-Logik fehlerhaft, wuerde
  der zweite `INSERT` mit einem echten `schema_migrations_pkey`-Fehler
  abbrechen statt still doppelt zu zaehlen, der Test ist also nicht nur
  eine Zeilenanzahl-Pruefung, sondern deckt Skip-Logik-Fehler direkt per
  Fehlerrueckgabe auf).
  Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` clean (die
  Datei wurde durch die vollstaendige Neufassung nebenbei komplett
  gofmt-konform - war zuvor Teil von Backlog 0.15, jetzt eine Datei weniger
  in dieser Liste, aber Backlog 0.15 selbst bleibt als eigene Position
  bestehen, da noch 39 weitere Dateien betroffen sind). `go test
  ./internal/migrate/... -v -count=1` (beide neuen Tests PASS). `go test
  ./internal/accounting/... ./internal/hr/... ./internal/auth/...
  ./internal/purchasing/... ./internal/config/... ./internal/migrate/...
  ./internal/quotes/... ./internal/sales/... ./internal/contacts/...
  -count=1 -p 16` (alle PASS außer den bereits bekannten
  `contacts.telefon`-Faellen). Vollstaendiger `go test ./...` (alle
  Nicht-Integrationspakete) ohne Fehler.
- Backlog 0.15 (CI-Format-Check würde aktuell fehlschlagen: 40 Go-Dateien
  nicht gofmt-konform) behoben.
  Aktuellen Stand per `find server -name '*.go' -not -path '*/vendor/*' |
  xargs gofmt -l` neu ermittelt statt dem ursprünglichen Fund blind zu
  vertrauen: von den urspünglich 40 waren bereits 12 durch andere Subtasks
  dieser Session (die diese Dateien im Rahmen substanzieller Neufassungen
  nebenbei komplett gofmt-konform gemacht hatten, z. B.
  `internal/migrate/migrate.go` bei Backlog 0.13, `internal/accounting/ar.go`
  bei Backlog 0.7, `internal/hr/service.go` bei Backlog 0.10/0.11/0.12) auf
  28 gesunken.
  `gofmt -w` über genau diese 28 verbliebenen Dateien angewendet (keine
  anderen Dateien angefasst) — quer durchs Modul: `cmd/api/main.go`,
  `internal/app/server.go`, `internal/auth/{service,store}.go` + Test,
  `internal/config/config.go`, `internal/db/{mongo,postgres,redis}.go`,
  `internal/http/{health,observability,router}.go` + Test,
  `internal/materials/{documents,jsonb}.go`,
  `internal/projects/analyze_logikal.go`, sieben Dateien in
  `internal/settings/`, `internal/testutil/integration.go`,
  `internal/version/version.go`.
  **Diff-Review, wie im ursprünglichen Fund selbst als Sorgfaltsmaßnahme
  vorgesehen** (gofmt ändert nie Semantik, aber der Fund empfahl trotzdem
  Review "um versehentliche semantische Änderungen auszuschließen"):
  `git diff -w` (ignoriert zeileninterne Whitespace-Unterschiede, zeigt aber
  echte Struktur-/Inhaltsänderungen) wies für einen Teil der Dateien noch
  Differenzen aus. Bei Durchsicht stellte sich heraus: das waren zum einen
  (a) bereits VORHER in dieser Session eingefügter, unabhängiger
  Funktionscode, der beim Diff gegen den letzten Commit ohnehin sichtbar
  wird (z. B. `auth/service.go`s Rate-Limiting/User-Management-Code aus
  0.5.2/0.5.4.1 — nichts mit dieser Reformatierung zu tun, nur zufällig in
  derselben Datei), und zum anderen (b) tatsächlich zwei Kategorien
  legitimer, semantik-neutraler gofmt-Normalisierung: entfernte
  überflüssige Leerzeilen am Dateiende (`db/postgres.go`, `db/redis.go`,
  `version/version.go` — jeweils eine einzelne leere Zeile ganz am Ende
  entfernt) und eine alphabetische Import-Neusortierung
  (`http/router.go`: `nalaerp3/internal/version` wurde hinter
  `nalaerp3/internal/config` einsortiert — Go-Imports sind reihenfolge-
  unabhängig, ändert also die Programmsemantik nicht). Die im
  ursprünglichen Fund konkret befürchtete Kategorie ("Einzeiler-`if`-
  Umformatierung") kam in der Praxis NICHT vor — Go erzwingt ohnehin überall
  Klammern um `if`-Bodies (anders als z. B. C, wo klammerlose
  Einzeiler-Bodies erlaubt sind), es gab also gar keine Gelegenheit dafür.
  Verifiziert: `go build ./...`, `go vet ./...`, `gofmt -l` (leer — alle 28
  Dateien jetzt konform, exakt der vom CI-Job verwendete Befehl). Voller
  `go test ./...` (Nicht-Integrationspakete) ohne Fehler. Vollständiger
  `NALA_INTEGRATION=1 go test ./...` (alle Pakete, inkl. `internal/http`
  und `internal/quotes`) gegen frische, per `\dt` bestätigt leere DB:
  **identische Fehlerliste wie unmittelbar VOR der Reformatierung** (27
  bereits bekannte HTTP-Fehlschläge + 5 bereits bekannte
  `contacts.telefon`-Fehlschläge in `quotes`, Wort für Wort dieselben
  Testnamen) — keine einzige neue Regression durch die reine
  Formatierungsänderung eingeführt.
- Backlog 0.21 (`TestMaterialsCreateListAndGetFlow` schlägt auf frischer DB
  fehl: "Ungültige Materialkategorie") behoben.
  **Root-Cause-Analyse ergab einen echten Chicken-Egg-Zustand, kein reines
  Testfixture-Problem**: `Service.normalizeAndValidateCategory`
  (`server/internal/materials/service.go:616-644`) lässt eine Kategorie nur
  zu, wenn sie ENTWEDER bereits in `material_groups` steht ODER bereits
  mindestens ein `materials`-Datensatz mit genau dieser Kategorie
  existiert. `material_groups` wird laut `039_material_groups.sql` aber
  NUR rückwirkend aus bereits VORHANDENEN `materials.kategorie`-Werten
  befüllt (`INSERT ... SELECT DISTINCT ... FROM materials WHERE ...`). Auf
  einer wirklich leeren DB kann WEDER Bedingung je erfüllt sein — ohne
  manuellen Bootstrap könnte buchstäblich NIEMAND je das erste Material
  irgendeiner neuen Kategorie anlegen, auch nicht in einer echten
  Neuinstallation, nicht nur im Test. Die Architektur sieht dafür aber
  bereits den korrekten, vorgesehenen Weg vor: eine dedizierte
  Materialgruppen-Verwaltung (`settings.MaterialGroupService`,
  `POST/GET/DELETE /api/v1/settings/material-groups`, gated hinter
  `settings.manage`) — neue Kategorien müssen dort zuerst EXPLIZIT
  angelegt werden, bevor Materialien sie verwenden dürfen. Die betroffenen
  Tests bildeten diesen vorgesehenen Ablauf bisher einfach nicht nach
  (sprangen direkt zur Materialanlage, ohne die Kategorie vorher
  anzulegen).
  **Umgesetzt**: `TestMaterialsCreateListAndGetFlow` legt jetzt zuerst per
  `POST /settings/material-groups/` die Kategorie "integration" an (204),
  bevor das Material erstellt wird.
  Neuer, wiederverwendbarer Test-Helper `ensureIntegrationMaterialGroup`
  (`server/internal/http/purchase_orders_integration_test.go`, gleiches
  Package `apihttp` wie alle HTTP-Integrationstests, daher paketweit
  aufrufbar) — legt eine Materialgruppe idempotent per Upsert an. Der
  bereits bestehende, gemeinsam genutzte Helper `createIntegrationMaterial`
  ruft ihn jetzt AUTOMATISCH auf, wenn ein `kategorie`-Feld im
  Request-Body gesetzt ist — das behob NEBENBEI auch
  `TestPurchaseOrdersCreateAndGetFlow` (nutzt denselben Helper, war
  nachweislich vom selben Root Cause betroffen, aber im ursprünglichen
  Fund nicht namentlich genannt — nur durch die Recherche zum Helper
  entdeckt), OHNE dessen eigenen Testcode überhaupt anfassen zu müssen.
  `TestPurchaseOrdersAreLockedAfterLeavingDraftStatus` (aus Epic 0.3.2,
  früher in dieser Session) hatte den Bug bereits durch einen expliziten
  Kommentar UND eine bewusst LEERE Kategorie umgangen — dieser Workaround
  bleibt unverändert bestehen, kein Regressionsrisiko, da eine leere
  Kategorie schon immer ohne Validierung durchgeht.
  Zusätzlich 6 Aufrufe von `ensureIntegrationMaterialGroup(t, handler,
  accessToken, "profile")` in `server/internal/http/quotes_integration_test.go`
  ergänzt, je einer direkt nach dem Login: alle sechs Tests
  (`TestQuoteMaterialSearchEndpointSupportsOpenDraftItemSearch`,
  `TestQuoteMaterialSearchApplyEndpointSupportsVisibleSearchResultApply`,
  `TestQuotePriceSuggestionEndpointSupportsMappedDraftItem`,
  `TestQuoteApplyPriceSuggestionEndpointSupportsMappedDraftItem`,
  `TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem`,
  `TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`)
  nutzen eine eigene, LOKALE `createMaterial`-Closure mit hartcodiertem
  `kategorie:"profile"` statt des gemeinsamen Helpers — per `grep -n
  "\"kategorie\":"` gezielt gefunden, keins davon im ursprünglichen
  Backlog-0.21-Fund erwähnt, aber alle sechs am selben Root Cause
  gescheitert.
  **Wichtige ehrliche Feststellung bei der Verifikation**: die 6
  Quote-Tests springen nach der Korrektur zwar über die Materialkategorie-
  Huerde, scheitern aber SOFORT DANACH an einer jeweils NEUEN, vorher
  einfach verdeckten Stelle im selben Testablauf (5× ein
  `classifyDomainError`-Substring-Muster, das eine `quotes`-Domänen-
  Fehlermeldung nicht erkennt — exakt dasselbe strukturelle Muster wie
  Backlog 0.29, nur andere Meldungen; 1× eine Positionsbeschreibung aus
  einem ANDEREN Test, die im Ergebnis auftaucht, vermutlich verwandt mit
  Backlog 0.19s Testdaten-Vermischung). Beide NICHT hier mitgefixt (klar
  außerhalb des auf Materialkategorien beschränkten Scopes von 0.21,
  jeweils ein eigenständiger, anderer Bug), sondern als neue Backlog-
  Positionen 0.40 (classifyDomainError-Lücken, konkrete Nachrichten
  "Angebotsposition hat bereits ein Material"/"material_id ist kein
  sichtbarer Suchtreffer"/"Angebotsposition hat kein Material", alle in
  `internal/quotes/service.go` verifiziert per `grep`) und 0.41
  (Testdaten-Vermischung, Ursache noch nicht analysiert) dokumentiert. Das
  ist kein Rückschritt: der ORIGINALE 0.21-Bug ist vollständig und korrekt
  behoben (bewiesen dadurch, dass die Tests jetzt WEITER kommen als vorher
  und an einer ANDEREN Stelle scheitern), die neu sichtbaren Probleme
  waren immer schon da, nur vorher durch 0.21 maskiert.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Gezielte `-run`-Läufe für alle
  betroffenen Tests. Vollständiger `go test ./internal/http/... -count=1`:
  von 27 auf 25 Fehlschläge zurückgegangen (`TestMaterialsCreateListAndGetFlow`
  und `TestPurchaseOrdersCreateAndGetFlow` fehlen jetzt in der Liste, alle
  anderen unverändert). Vollständiger `go test ./...`
  (Nicht-Integrationspakete) ohne Fehler.
- Backlog 0.22 (`GET /api/v1/documents/{docID}` prüft keine
  Mandantenzugehörigkeit) behoben — **echte, schwerwiegendere
  Sicherheitslücke als im ursprünglichen Fund beschrieben.**
  Der ursprüngliche Fund (Subtask 0.2.2.1.2.2.5) beschrieb "fehlende
  Mandantenprüfung" — bei der Analyse stellte sich heraus, dass
  `OpenDocumentStream` (`server/internal/materials/documents.go`,
  `server/internal/contacts/documents.go`) den GridFS-Stream VORHER
  komplett OHNE JEDE Prüfung öffnete: `bucket.OpenDownloadStream(oid)`
  lief direkt, die anschließende `SELECT filename, content_type, length
  FROM material_documents/contact_documents WHERE document_id=$1`-Abfrage
  war rein informativ (`_ = ....Scan(...)`, Fehler verworfen) und lieferte
  bei Nichtfund einfach leere Strings statt den Download zu verweigern.
  Praktische Konsequenz: JEDER authentifizierte Nutzer mit der
  `documents.read`-Permission (ein generisches, plattformweites Recht,
  nicht `contacts.read`/`materials.read`) konnte JEDE Datei im GESAMTEN
  GridFS-Bucket herunterladen — nicht nur mandantenübergreifend, sondern
  komplett UNGEBUNDEN an irgendeinen `material_documents`/
  `contact_documents`-Datensatz — solange die 24-stellige hexadezimale
  MongoDB-ObjectID bekannt oder erraten war. Eine echte
  Autorisierungslücke, nicht nur eine Mandanten-Scoping-Schwäche.
  **Umgesetzt**: beide `OpenDocumentStream`-Methoden bekamen einen neuen
  Pflichtparameter `companyID string`. Die Reihenfolge wurde umgedreht:
  ZUERST wird zwingend (mit geprüftem, nicht mehr verworfenem Fehler)
  per `JOIN` auf die übergeordnete, mandanten-gescopte Tabelle geprüft,
  dass ein `material_documents`/`contact_documents`-Eintrag mit dieser
  `document_id` existiert UND das zugehörige Material/der zugehörige
  Kontakt zum `companyID` des Aufrufers gehört
  (`material_documents md JOIN materials m ON m.id=md.material_id WHERE
  md.document_id=$1 AND m.company_id=$2`, analog für Kontakte) — NUR wenn
  das erfolgreich ist, wird der GridFS-Stream überhaupt geöffnet. Bei
  Fehlschlag: `"Dokument nicht gefunden"` → HTTP `404` (kein
  Existenz-Leak über Mandantengrenzen hinweg, konsistent mit dem in Task
  0.2 durchgängig etablierten Muster für Cross-Tenant-Zugriffe). Handler
  in `server/internal/http/v1.go` liest `companyID` jetzt aus dem Context
  und reicht ihn an beide Aufrufe (`matSvc`/`conSvc`) durch. Nur 3
  Call-Sites im gesamten Modul betroffen (die zwei Methodendefinitionen
  plus die zwei Aufrufe in `v1.go`, per `grep -rn "OpenDocumentStream("`
  bestätigt), kein weiterer Code angepasst werden musste.
  **Wertvoller, verifizierter Nebeneffekt**: dieselbe Änderung löste auch
  das unabhängig dokumentierte Backlog 0.17
  (`TestContactDocumentsUploadListAndDownloadFlow` fehlte der
  `Content-Disposition`-Header) — per zweimaligem Testlauf (isoliert und
  im vollen Suite-Lauf) bestätigt zuverlässig grün. Plausibelste Erklärung
  (nicht tiefer eigens debuggt, da der Fix ohnehin nötig war): die alte
  Implementierung las Dateiname/Content-Type NACH dem Öffnen des Streams
  mit STILL VERWORFENEM Fehler — schlug der Scan aus irgendeinem Grund
  fehl, blieb `filename` leer, der Handler in `v1.go` setzte dadurch
  keinen `Content-Disposition`-Header (`if filename != "" { ... }`). Die
  neue Implementierung liest dieselben Daten jetzt ZWINGEND mit geprüftem
  Fehler VOR dem Öffnen des Streams.
  Tests: neue Datei `server/internal/http/documents_integration_test.go`.
  `TestDocumentDownloadIsScopedToCompany` legt per Direkt-SQL einen
  zweiten Mandanten samt eigenem Admin-Nutzer an (`testutil.SeedAuthUser`
  setzt `company_id` fest auf `'default'`, kann also kein
  Cross-Tenant-Szenario liefern), lädt je EIN Dokument für einen Kontakt
  UND ein Material im ERSTEN Mandanten hoch, und beweist für BEIDE
  Dokumenttypen: der Eigentümer-Mandant kann herunterladen (`200`), der
  ANDERE Mandant erhält `404` — deckt beide im ursprünglichen Fund
  ausdrücklich genannten betroffenen Domänen ab (`materials` UND
  `contacts`). Neuer wiederverwendbarer Helper
  `uploadIntegrationDocument` (multipart-Upload-Boilerplate).
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Gezielter Testlauf (neuer Test
  + `TestContactDocumentsUploadListAndDownloadFlow` + alle
  `TestMaterials*` + `TestContactActivityFeedAggregatesNotesTasksAndDocuments`
  als weiterer Regressionscheck) — alle PASS. Vollständiger `go test
  ./internal/http/... -count=1`: von 25 auf 24 Fehlschläge zurückgegangen
  (0.17 verschwindet aus der Liste, sonst keine Veränderung). Voller `go
  test ./...` (Nicht-Integrationspakete) ohne Fehler.
- Backlog 0.23 (`projects.BuildQuoteSnapshot` referenziert nicht
  existierende Spalte `contacts.telefon`) behoben — und dabei gleich
  Backlog 0.27 mit erledigt (identischer Tippfehler, nur in einer
  Testfixture statt Produktivcode).
  Einfacher, aber folgenreicher Tippfehler: `BuildQuoteSnapshot()`
  (`server/internal/projects/service.go`) selektierte `COALESCE(c.telefon,
  '')` aus `contacts` — die tatsächliche Spalte heißt `phone`
  (`server/internal/migrate/migrations/003_contacts.sql:9`). Jeder Aufruf
  (Angebots-PDF aus Projektkontext, `GET /api/v1/projects/{id}/quote-pdf`)
  schlug mit `SQLSTATE 42703` fehl. Fix: `c.telefon` → `c.phone`.
  Per `grep -rn "telefon"` über das GESAMTE `server/`-Modul (ohne
  `_test.go`) systematisch nach weiteren Vorkommen gesucht, um
  auszuschließen, dass derselbe Tippfehler noch woanders im Produktivcode
  lauert — Ergebnis: keine weitere Fundstelle. Die JSON-Feldnamen
  `Telefon`/`"telefon"` in `internal/contacts/service.go` sind KEIN Bug,
  sondern die (bewusst deutschsprachige) API-Vertragsbezeichnung, die dort
  korrekt und schon immer auf die tatsächliche Spalte `phone` gemappt wird
  (explizit verifiziert: `INSERT INTO contacts (..., phone, ...) VALUES
  (..., in.Telefon, ...)`).
  Zusätzlich per `grep -rn "telefon" --include="*_test.go"` ALLE
  Test-Dateien nach dem GLEICHEN Muster durchsucht — dabei zwischen zwei
  grundverschiedenen Kategorien unterschieden: JSON-Request-Bodies wie
  `"telefon": "+49 ..."` (korrekt, API-Konvention, nicht angefasst) versus
  ROHE SQL-`INSERT`-Statements, die `telefon` wörtlich als Spaltennamen
  verwenden (derselbe Bug wie in `BuildQuoteSnapshot`, da diese Test-Inserts
  direkt gegen Postgres gehen, nicht über die API). Vier weitere Stellen
  der zweiten Kategorie gefunden und korrigiert:
  `server/internal/quotes/approval_decisions_test.go:31` (= der eigentliche
  Ziel-Fund von Backlog 0.27), `server/internal/quotes/imports_test.go:42`
  und `:130`, `server/internal/http/quotes_integration_test.go:47`.
  **Ergebnis, inkl. neu sichtbar gewordener Folgefunde** (dasselbe Muster
  wie bei den letzten Subtasks: ein Fix deckt eine vorher maskierte,
  UNABHÄNGIGE zweite Stelle im selben Testablauf auf): `TestProjectQuotePDFFlow`
  (0.23s eigentliches Ziel) und `TestApprovalRequestDecisionsMutateOnlyRequest`
  (0.27) sind jetzt vollständig grün. Die drei `TestProcessGAEBImport*`-Tests
  sowie `TestQuoteImportParseResultStoresItemsAndUpdatesStatus` (alle in
  `internal/quotes/imports_test.go`) kommen jetzt weiter, scheitern aber an
  `ERROR: null value in column "nummer" of relation "projects" violates
  not-null constraint` — die rohen Test-`INSERT INTO projects (...)`-
  Statements dort lassen die NOT-NULL-Spalte `nummer`
  (`007_projects.sql:4`, kein Default) komplett weg. Als neues Backlog
  0.42 dokumentiert, bewusst NICHT hier mitgefixt (anderer Bug, anderer
  Scope als der `telefon`-Tippfehler). `TestGAEBImportProcessEndpoint`
  kommt ebenfalls weiter, trifft jetzt sichtbar auf das bereits
  dokumentierte Backlog 0.31 (`NewV1RouterWithOptions` ohne `/api/v1`-
  Präfix, Login liefert `404` statt `200`).
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. `go test ./internal/quotes/...
  -v -count=1`. Gezielt `go test ./internal/http/... -run
  "TestGAEBImportProcessEndpoint|TestProjectQuotePDFFlow"`. Vollständiger
  `go test ./internal/http/... -count=1`: von 24 auf 23 Fehlschläge
  zurückgegangen. Voller `go test ./...` (Nicht-Integrationspakete) ohne
  Fehler.
- Backlog 0.24, 0.29 und 0.40 gemeinsam behoben (alle drei strukturell
  identisch, siehe unten) — größter Einzelfortschritt bisher in der
  Aufarbeitung der Epic-0-Funde.
  Alle drei Backlog-Positionen beschreiben exakt dasselbe Muster: eine
  fachlich korrekte Guard-Regel (`errors.New("...")`) irgendwo in
  `sales`/`quotes` verhindert eine unerlaubte Operation richtig, aber die
  konkrete deutsche Fehlermeldung trifft keins der Substring-Muster in
  `classifyDomainError` (`server/internal/http/v1.go`) und fällt daher auf
  den `default`-Zweig (`500 internal_error`) statt auf `400
  validation_error` — fachlich korrekt verhindert, aber falscher
  HTTP-Status, der Clients irreführt (500 signalisiert "Serverfehler,
  nicht meine Schuld", 400 signalisiert "deine Anfrage war ungültig").
  0.24: "Auftrag muss mindestens eine Position enthalten"
  (`sales.Service`, `TestQuoteFlowWithPricingAndPDF`). 0.29: drei
  GAEB-Import-Meldungen aus `quotes/imports.go` ("... sind zulässig",
  "... können reviewt werden", "... offene Review-Positionen"). 0.40: drei
  Material-Mapping-Meldungen aus `quotes/service.go` ("... hat bereits ein
  Material", "... kein sichtbarer Suchtreffer", "... hat kein Material").
  0.29 und 0.40 waren beide erst NEBENBEI bei früheren Subtasks entdeckt
  worden (0.29 bei einer GAEB-Verifikation, 0.40 beim Beheben von Backlog
  0.21) — beide explizit als "gleiches strukturelles Muster, ggf. zusammen
  mit 0.24/0.29 in einer gemeinsamen Subtask" vermerkt worden. Statt drei
  fast identische Einzel-Edits nacheinander umzusetzen (dieselbe Datei,
  derselbe `switch`-Block, dieselbe Art von Änderung), wurden ALLE
  benötigten Substring-Muster in EINEM einzigen Edit an
  `classifyDomainError` ergänzt: `"mindestens eine position"`,
  `"sind zulässig"`, `"können reviewt werden"`,
  `"offene review-positionen"`, `"hat bereits ein material"`,
  `"kein sichtbarer suchtreffer"`, `"hat kein material"` — sieben neue
  Substring-Einträge im bestehenden `400`-Zweig des `switch`, keine
  weiteren Code-Änderungen nötig (der Bug liegt ausschließlich in der
  fehlenden Erkennung, nicht in der zugrundeliegenden Fachlogik selbst,
  die bereits korrekt war).
  **Ergebnis**: 8 der 9 betroffenen Tests
  (`TestQuoteGAEBImportRejectsNonGAEBFiles`,
  `TestQuoteGAEBImportItemReviewEndpointRejectsNonParsedImport`,
  `TestQuoteGAEBImportReviewEndpointRejectsPendingItems`,
  `TestQuoteMaterialSearchEndpointSupportsOpenDraftItemSearch`,
  `TestQuoteMaterialSearchApplyEndpointSupportsVisibleSearchResultApply`,
  `TestQuotePriceSuggestionEndpointSupportsMappedDraftItem`,
  `TestQuoteApplyPriceSuggestionEndpointSupportsMappedDraftItem`,
  `TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem`)
  sind jetzt VOLLSTÄNDIG grün. Der neunte,
  `TestQuoteFlowWithPricingAndPDF` (0.24s eigentliches Ziel), kommt an der
  ursprünglich gemeldeten Stelle korrekt vorbei (bewiesen: die
  "mindestens eine Position"-Guard liefert jetzt `400`, der Test kommt
  spürbar weiter als vorher), scheitert aber SPÄTER an
  `ERROR: FOR UPDATE cannot be applied to the nullable side of an outer
  join` — das ist die bereits als KRITISCH dokumentierte Backlog-0.35-
  Ursache (`sales.Service.ConvertToInvoice`), kein neuer Fund, keine
  Regression.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Gezielter Testlauf aller 9
  betroffenen Tests. Vollständiger `go test ./internal/http/... -count=1`:
  **von 23 auf 15 Fehlschläge zurückgegangen** — der mit Abstand größte
  Einzelfortschritt seit Beginn der Epic-0-Fundaufarbeitung, weil ein
  einziger, kleiner Fix (7 neue Substring-Zeilen) gleich drei separate
  Backlog-Positionen und 8 Testfunktionen auf einmal lösen konnte. Voller
  `go test ./...` (Nicht-Integrationspakete) ohne Fehler.
- Backlog 0.25 (`POST /quotes/{id}/convert-to-invoice` lässt sich direkt
  nach Anlage nicht ohne vorherigen Status-Übergang testen/nutzen)
  behoben. Reiner Testfehler, KEIN Anwendungsbug.
  `quotes.Service`s Guard für `convert-to-invoice` (`server/internal/quotes/service.go`)
  lässt die Konvertierung nur bei Status `sent`/`accepted` zu — fachlich
  korrekt, da nur bereits versendete/angenommene Angebote in Rechnungen
  überführt werden sollen, nicht jeder Entwurf. Ein per `POST /quotes/`
  frisch angelegtes Angebot hat den Default-Status `draft` und wurde
  daher korrekt abgelehnt. `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
  rief `convert-to-invoice` aber direkt nach der Anlage auf, ohne den
  nötigen Status-Übergang vorher durchzuführen.
  Die im ursprünglichen Fund offen gelassene Frage ("Unklar, ob der Test
  veraltet ist oder ein Status-Übergang fehlt") geklärt: `quotes.Service.UpdateStatus`
  erlaubt den Übergang `draft`→`sent` problemlos (kein Hinweis in Code oder
  Historie auf eine kürzlich verschärfte Regel) — der Test war schlicht von
  Anfang an unvollständig, kein Anwendungsverhalten wurde geändert.
  Fix: neuer `POST /quotes/{id}/status`-Aufruf mit `{"status":"sent"}` VOR
  dem `convert-to-invoice`-Aufruf für das direkt angelegte Angebot — genau
  der reale, vorgesehene Ablauf (Angebot → versendet → Rechnung).
  Per `grep -n "convert-to-invoice" internal/http/*_test.go` alle
  Vorkommen dieses Endpunkt-Aufrufs im gesamten `internal/http`-Testpaket
  durchsucht, um zu prüfen, ob dasselbe "Konvertierung ohne vorherigen
  Status-Übergang"-Muster noch anderswo auftritt — keine weitere
  betroffene Stelle gefunden: alle anderen Vorkommen sind entweder bereits
  korrekt aufgebaute Tests (nicht in der aktuellen Fehlerliste) oder
  bewusste NEGATIVTESTS, die genau diese Ablehnung als erwartetes Verhalten
  prüfen (ein Kommentar in `accounting_integration_test.go:490` verweist
  bereits explizit auf Backlog 0.25 als Kontext für einen solchen
  Negativtest).
  **Ergebnis**: der Test kommt nach dem Fix deutlich weiter — die direkte
  Angebots-Konvertierung, der Status-Übergang und die separate
  Auftragsumwandlung (Angebot→Auftrag über einen anderen Pfad im selben
  Test) laufen alle erfolgreich durch (per Log verifiziert: `POST
  .../status` → `200`, `POST .../convert-to-invoice` für das Angebot →
  `201`). Der Test scheitert aber SPÄTER, beim Versuch, den aus dem
  zweiten Angebot entstandenen AUFTRAG in eine Rechnung umzuwandeln, an
  der bereits dokumentierten, unabhängigen Backlog-0.35-Ursache (`FOR
  UPDATE`+`LEFT JOIN`-Fehler in `sales.Service.ConvertToInvoice`) — kein
  neuer Fund, keine Regression. 0.25s eigentliches Ziel (die
  Angebots-Konvertierung selbst) ist nachweislich und vollständig behoben.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Gezielter Testlauf (Log
  bestätigt den erwarteten Fortschritt bis zum bekannten 0.35-Punkt).
  Vollständiger `go test ./internal/http/... -count=1`: weiterhin 15
  Fehlschläge (derselbe Test bleibt in der Liste, aber jetzt aus dem
  bereits bekannten 0.35-Grund statt aus dem ursprünglichen 0.25-Grund) —
  kein Rückschritt, da die Gesamtzahl gleich bleibt, aber die URSACHE
  dieses einen Fehlschlags sich nachweislich verschoben hat. Voller `go
  test ./...` (Nicht-Integrationspakete) ohne Fehler.
- Backlog 0.26 (`TestQuoteApplyVisibleMaterialCandidateSetsManualMapping`
  schlägt fehl: falscher Upload-Pfad im Test) behoben.
  Reiner Pfad-Tippfehler: der Test sandte den GAEB-Upload an `POST
  /api/v1/quotes/imports`, registriert ist aber ausschließlich `POST
  /api/v1/quotes/imports/gaeb` (`server/internal/http/v1.go`) —
  `405 Method Not Allowed` statt der erwarteten `201`. Andere Tests im
  selben File nutzen bereits korrekt `/imports/gaeb`. Fix: Pfad im Test
  korrigiert, per `grep` als einzige Fundstelle im Modul bestätigt.
  **Nach dem Fix, bei der Verifikation, eine interessante Verwicklung
  aufgeklärt**: der Test kommt danach deutlich weiter (Upload gelingt),
  PANICT dann aber - zunächst überraschend, da die Panic-Trace exakt
  denselben Funktionsaufrufpfad zeigte wie das bereits dokumentierte
  Backlog 0.30 (`ApplyImportToDraftQuote → createQuoteTx →
  NumberingService.Next` mit nil-Receiver). Auf den ersten Blick schien
  das ein NEUER, schwerwiegenderer Fund zu sein: der Test benutzt
  `handler := NewRouterWithDeps(...)` (den REGULÄREN, korrekt verdrahteten
  HTTP-Router mit einer echten `settings.NumberingService`-Instanz,
  `server/internal/http/v1.go`) für alle anderen HTTP-Aufrufe im selben
  Test — wie könnte darüber dieselbe Nil-Panic entstehen, wenn die
  Verdrahtung dort nachweislich korrekt ist (das ist derselbe Router, über
  den in dutzenden anderen Tests dieser Session erfolgreich Angebote
  angelegt werden)?
  Per Code-Lesen aufgeklärt statt spekuliert (aufgabe.md §7.4: "Keine
  Diagnose ohne Beleg"): der Test hat eine ZWEITE, komplett SEPARATE,
  lokale Variable `quoteSvc := quotes.NewService(env.PG, nil)`
  (`server/internal/http/quotes_integration_test.go`), die NICHT über den
  HTTP-`handler`/`ServeHTTP` läuft, sondern direkt als Go-Funktionsaufruf
  verwendet wird - für drei Methoden ohne dedizierten HTTP-Endpunkt
  (`UpdateImportItemReview`, `MarkImportReviewed`,
  `ApplyImportToDraftQuote`). Diese zweite `quoteSvc`-Instanz bekommt `nil`
  als `NumberingService` übergeben - EXAKT derselbe Anti-Pattern wie im
  bereits dokumentierten Backlog 0.30, nur eine zweite, unabhängige
  Fundstelle desselben Musters, keine neue oder schwerwiegendere
  Bug-Kategorie. Die anfängliche Vermutung (Router-Ebene betroffen) wurde
  damit widerlegt, bevor sie fälschlich als neuer kritischer Fund
  dokumentiert worden wäre - ein Beispiel für die in dieser Session
  durchgängig befolgte Disziplin, Fehlerursachen durch tatsächliches
  Codelesen zu belegen statt aus einer oberflächlich ähnlichen
  Panic-Signatur zu raten.
  Backlog 0.30 um diese zweite betroffene Testfunktion ergänzt (derselbe
  Fix - eine echte `settings.NewNumberingService(env.PG)`-Instanz statt
  `nil` - würde beide Vorkommen gleichzeitig lösen). Da ein Panic den
  GESAMTEN Testbinary-Prozess abbricht (nicht nur die eine Testfunktion),
  wird dieser Test ab sofort zusätzlich zu
  `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` per `-skip`
  von vollen Suite-Läufen ausgeschlossen, bis 0.30 tatsächlich behoben
  wird.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Isolierter Testlauf bestätigt
  Fortschritt bis zum bekannten 0.30-Panic-Punkt. Vollständiger `go test
  ./internal/http/... -count=1 -skip
  "TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates|TestQuoteApplyVisibleMaterialCandidateSetsManualMapping"`:
  von 15 auf 14 Fehlschläge zurückgegangen. Voller `go test ./...`
  (Nicht-Integrationspakete) ohne Fehler.
- **Backlog 0.28 behoben** (`TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests`:
  falscher `reason_code` in Testfixture). Root-Cause-Analyse: Fixture-Preise
  `unitPrice=50 < costBasis=60` in `seedHTTPApprovalDecisionQuote(...)`
  ergeben über `RequestApprovalForQuoteItem`
  (`server/internal/quotes/service.go`) rechnerisch zwingend
  `absoluteMargin = 50-60 = -10` ⇒ da `-10 < -centTolerance(-0.005)` greift
  der `if absoluteMargin < -centTolerance`-Zweig zuerst ⇒
  `reasonCode="negative_margin"`. Die Testassertion erwartete jedoch
  `"below_target_margin"`. Verifiziert, dass dies KEIN Produktivbug ist: die
  übrigen Assertions im selben Test
  (`TargetUnitPriceSnapshot=72`, `TargetDifferenceSnapshot=-22`,
  `MarginPercentSnapshot≈-16.67%`, `CurrentTargetStatus="below_cost"`) sind
  bereits ausschließlich mit `negative_margin`-Semantik konsistent — nur die
  eine `ReasonCode`-Assertion widersprach dem Rest des eigenen Tests
  (Kopier-Fehler). Fixture-Preise bewusst NICHT geändert, da
  `seedHTTPApprovalDecisionQuote(..., 50, 60)` an 17 weiteren Stellen im
  selben Testfile identisch verwendet wird — eine Preisänderung hätte
  unnötig weitere, aktuell grüne Tests riskiert.
  In `server/internal/http/quotes_integration_test.go` die Assertion in
  `TestQuoteApprovalRequestQueueEndpointListsOnlyActiveRequests` von
  `"below_target_margin"` auf `"negative_margin"` korrigiert (mit
  erklärendem Kommentar zur Rechnung). Per systematischem `grep -n
  "below_target_margin\|negative_margin"` über das gesamte Testfile zwei
  weitere, bisher nicht einzeln benannte Fundstellen desselben
  Kopier-Fehlers entdeckt (beide nutzen ebenfalls
  `seedHTTPApprovalDecisionQuote(..., 50, 60)` und dieselbe falsche
  Erwartung): `TestQuoteApprovalDecisionEndpointsRequireApprovePermission`
  und `TestQuoteApprovalReworkQueueEndpointListsOnlyOpenLatestRejections` —
  beide identisch auf `"negative_margin"` korrigiert. Eine vierte,
  oberflächlich ähnliche Fundstelle
  (`TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`,
  Fixture-Preise 59.9/59.9, `absoluteMargin≈0`, `targetDifference≈-11.98`)
  wurde geprüft und als rechnerisch korrekt `"below_target_margin"`
  bestätigt — bewusst NICHT angefasst.
  Beim isolierten Verifizieren von
  `TestQuoteApprovalDecisionEndpointsRequireApprovePermission` (nach der
  Assertion-Korrektur) kam der Test weiter voran und traf einen NEUEN,
  bisher unbekannten `classifyDomainError`-Fund (gleiches Muster wie
  0.24/0.29/0.40): die Meldung `"Keine aktive Freigabeanforderung
  vorhanden"` (`server/internal/quotes/service.go`, zwei Fundstellen, Zeile
  ~1528 und ~2243) traf keins der bestehenden Substring-Muster in
  `classifyDomainError` und fiel dadurch auf `500 internal_error` statt
  `400 validation_error` (Testschritt: erneute Ablehnung einer bereits
  entschiedenen Freigabeanforderung). Neues Substring-Muster
  `strings.Contains(msg, "keine aktive freigabeanforderung")` im
  `validation_error`-Zweig von `classifyDomainError`
  (`server/internal/http/v1.go`) ergänzt, direkt neben den anderen
  Freigabe-bezogenen Mustern aus 0.24/0.29/0.40.
  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean (nach CRLF-Normalisierung).
  Alle drei korrigierten Tests einzeln PASS; `TestQuoteApprovalDecisionEndpointsRequireApprovePermission`
  log-bestätigt: `POST .../reject` auf bereits entschiedener Anforderung
  liefert jetzt korrekt `400 validation_error` statt `500`. Voller `go test
  ./internal/http/... -count=1 -skip
  "TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates|TestQuoteApplyVisibleMaterialCandidateSetsManualMapping"`:
  von 14 auf 11 Fehlschläge zurückgegangen. Voller `go test ./...`
  (Nicht-Integrationspakete) ohne Fehler.
  **Lehre für künftige ähnliche Funde**: sobald eine falsche Testerwartung
  in einer gemeinsam genutzten Fixture-Funktion gefunden wird, lohnt es
  sich, proaktiv per `grep` nach allen weiteren Vorkommen desselben
  erwarteten Strings/Werts im selben Testfile zu suchen, statt nur die
  ursprünglich benannte Stelle zu fixen — hier wurden dadurch 2 zusätzliche,
  bisher nicht als eigene Backlog-Punkte erfasste Fundstellen im selben
  Aufwasch mitbehoben, während eine vierte, oberflächlich ähnliche Stelle
  korrekt unangetastet blieb (Verifikation per Nachrechnen, nicht per
  Analogieschluss).
- **Backlog 0.30 behoben** (`quotes.NewService(env.PG, nil)`-Panics in zwei
  Testfunktionen). In `server/internal/http/quotes_integration_test.go`:
  Import `nalaerp3/internal/settings` ergänzt; in
  `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` und
  `TestQuoteApplyVisibleMaterialCandidateSetsManualMapping` jeweils
  `quotes.NewService(env.PG, nil)` durch `quotes.NewService(env.PG,
  settings.NewNumberingService(env.PG))` ersetzt. Vor dem Fix rief
  `ApplyImportToDraftQuote` → `createQuoteTx`
  (`server/internal/quotes/service.go`) `numSvc.Next(...)` auf einem
  `nil`-Interface auf ⇒ Nil-Pointer-Panic in
  `internal/settings.(*NumberingService).Next`, der den GESAMTEN
  Testbinary-Prozess abbrach. Geprüft, dass die übrigen 5 Vorkommen von
  `quotes.NewService(env.PG, nil)` im selben Testfile (in
  `TestQuoteGAEBImportListAndDetailExposeReviewSummaryCounts`,
  `TestQuoteGAEBImportItemReadEndpointsExposeParsedItems`,
  `TestQuoteGAEBImportItemReviewEndpointUpdatesReviewFields`,
  `TestQuoteGAEBImportReviewEndpointRejectsPendingItems`,
  `TestQuoteGAEBImportApplyCreatesDraftQuoteFromAcceptedItems`) KEINE
  `numSvc.Next(...)`-aufrufende Methode auf der lokalen Instanz nutzen
  (nur `SaveImportParseResult`/`ListImportItems`/`UpdateImportItemReview`/
  `MarkImportReviewed`, bzw. der eigentliche Apply-Schritt läuft dort über
  den HTTP-`handler` mit echtem `NumberingService`) — bewusst nicht
  angefasst, kein Fix nötig.

  **Cascading-Fund beim Vollsuite-Lauf ohne `-skip`**: nach dem Fix kommt
  `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` erstmals ohne
  Absturz bis zur eigentlichen Fachassertion durch und schlägt DORT neu
  fehl: `quotes_integration_test.go:3536: expected one material candidate,
  got [{MaterialID:... MaterialNo:MAT-GAEB-0001 ...}
  {MaterialID:... MaterialNo:MAT-GAEB-CAND-0001 ...}]` — zwei Kandidaten
  statt einem. Root-Cause identifiziert:
  `TestQuoteUpdateAllowsManualMaterialMappingOnItems` (eigener Login, eigene
  Company) legt an anderer Stelle im selben Testfile (Zeile ~2168) ein
  Material mit IDENTISCHER Bezeichnung `"Aluminium Profil 70mm"` an
  (Nummer `MAT-GAEB-0001`) wie das in
  `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates` selbst
  angelegte Material (Nummer `MAT-GAEB-CAND-0001`, gleiche Bezeichnung).
  Beide Tests laufen gegen dieselbe (gemeinsam genutzte) Test-DB, aber mit
  unterschiedlichen Companies. `listMaterialCandidatesForQuoteItem`
  (`server/internal/quotes/service.go:~2968`) joint `materials m` rein per
  `LOWER(m.bezeichnung) = LOWER(BTRIM(qii.description)) OR LOWER(m.nummer)
  = LOWER(BTRIM(qii.description))` — OHNE jede `company_id`-Bedingung, die
  Funktion erhält nicht einmal einen `companyID`-Parameter. Ein echter,
  mandantenübergreifender Datenleck-Bug (vergleichbare Schwere wie das
  bereits behobene Backlog 0.22, anderer Codepfad). NICHT in diesem
  Subtask behoben (andere Code-Ebene als der hier behobene Panic-Fix,
  eigener SQL-Fix mit eigener Cross-Tenant-Testverifikation nötig) —
  dokumentiert als neues **Backlog 0.43 (KRITISCH)**.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Beide vormals panicenden Tests
  isoliert: PASS. Voller `go test ./internal/http/... -count=1` (ohne
  `-skip`): **12 Fehlschläge** (11 unveränderte Altfälle + 1 neu sichtbar
  gewordener Fund = Backlog 0.43) — rechnerisch konsistent zu den vorher
  bei `-skip`-Ausschluss beider 0.30-Tests gemessenen 11 Fehlschlägen,
  kein Netto-Regress durch den 0.30-Fix selbst.

  **Wichtige Verifikations-Erkenntnis**: ein ERSTER Vollsuite-Lauf, direkt
  im Anschluss an einen vorherigen Testlauf OHNE zwischenzeitlichen
  DB-Reset gestartet, ergab fälschlich **51 Fehlschläge** — durch
  Testdaten-Kollisionen, weil beide Läufe gegen dieselbe, bereits von Lauf
  1 befüllte DB liefen (z. B. doppelte E-Mail-/Nummern-Constraints). Nach
  korrektem Reset (`docker compose down -v` + `up -d --wait` +
  `\dt`-Leerprüfung) unmittelbar vor dem eigentlichen Verifikationslauf:
  reproduzierbar 12. Bestätigt erneut die in dieser Session etablierte
  Disziplin, dass JEDER einzelne Verifikationslauf einen unmittelbar
  vorangehenden DB-Reset braucht — auch wenn im selben Turn bereits einmal
  zurückgesetzt wurde, aber dazwischen ein weiterer Testlauf stattfand.
- **Backlog 0.31 behoben** (`TestGAEBImportProcessEndpoint`:
  `NewV1RouterWithOptions` direkt aufgerufen, aber Testpfade mit
  `/api/v1`-Präfix erwartet → `404` beim Login). Root Cause: `NewV1Router`/
  `NewV1RouterWithOptions` (`server/internal/http/v1.go`) mounten Routen
  DIREKT (z. B. `/auth/login`); das `/api/v1`-Prefixing passiert erst über
  `NewRouterWithDeps` (`server/internal/http/router.go`, `r.Mount("/api/v1",
  NewV1Router(...))`). Der Test baute den Handler aber direkt über
  `NewV1RouterWithOptions(...)` (um den `GAEBImportParser` per
  `V1RouterOptions` zu injizieren), während der geteilte Helper
  `loginIntegrationUser` (`contacts_integration_test.go`) fest
  `/api/v1/auth/login` anfordert — Router-Konstruktion und Testerwartung
  widersprachen sich.

  **Fix**: `NewRouterWithDeps` in `router.go` zu einem dünnen Wrapper um
  eine neue `NewRouterWithDepsAndOptions(pg, mg, rd, cfg, options
  V1RouterOptions) http.Handler` umgebaut, die intern
  `NewV1RouterWithOptions(...)` (statt bisher hart codiert
  `NewV1Router(...)`) unter `/api/v1` mountet. `NewRouterWithDeps` selbst
  ruft sie mit `V1RouterOptions{GAEBImportParser: quotes.GAEBXMLSubsetParser{}}`
  — demselben Default, den vorher `NewV1Router` intern gesetzt hat — damit
  keine der 12 anderen Aufrufstellen von `NewRouterWithDeps` (11
  Testdateien: `projects_integration_test.go`,
  `documents_integration_test.go`, `materials_integration_test.go`,
  `purchase_orders_integration_test.go`, `health_integration_test.go`,
  `accounting_integration_test.go`, `auth_integration_test.go`,
  `settings_integration_test.go`, `warehouses_integration_test.go`,
  `contacts_integration_test.go`, sowie diese Testdatei selbst an anderen
  Stellen; plus Produktivcode `internal/app/server.go`) ein anderes
  Verhalten bekommt. `TestGAEBImportProcessEndpoint`
  (`quotes_integration_test.go`) selbst auf `NewRouterWithDepsAndOptions(...,
  V1RouterOptions{GAEBImportParser: parser})` bzw. (für die beiden
  weiteren, im selben Test genutzten Router-Varianten) auf
  `NewRouterWithDeps(...)` / `NewRouterWithDepsAndOptions(...,
  V1RouterOptions{})` umgestellt; alle Testpfade der lokalen `call`-Closure
  mit `/api/v1` präfixiert. Diese Lösung (Router-Konstruktion an die
  Testerwartung anpassen) wurde bewusst der im ursprünglichen Fund
  genannten Alternative (Testpfade OHNE `/api/v1`-Präfix) vorgezogen, weil
  sie den Test gegen dieselbe Router-Form laufen lässt, die auch produktiv
  tatsächlich gemountet wird.

  **Selbst eingeführten Regressions-Bug sofort bemerkt und behoben**: die
  erste Fassung von `NewRouterWithDepsAndOptions` — aufgerufen von
  `NewRouterWithDeps` mit einem LEEREN `V1RouterOptions{}` — hätte den
  `GAEBImportParser` für ALLE 12 übrigen Aufrufstellen von
  `NewRouterWithDeps` auf `nil` gesetzt statt auf den bisherigen Default.
  Beim allerersten eigenen Testlauf (noch innerhalb desselben Subtasks,
  vor jedem Suite-weiten Verifikationslauf) fiel das sofort auf: die
  eigentlich als Erfolgsfall erwartete "default parser"-Prüfung IN
  DEMSELBEN Test schlug mit `500 "GAEB-Parser nicht konfiguriert"` fehl.
  Behoben, bevor irgendein anderer Test oder Aufrufer davon betroffen sein
  konnte.

  Nebenbei ein mechanisch identischer Fund wie Backlog 0.42 im selben,
  ohnehin für diesen Subtask bearbeiteten Testfunktions-Scope behoben
  (kein Scope-Creep, da exakt dieselbe Testfunktion): `TestGAEBImportProcessEndpoint`
  legte das Test-Projekt per Direkt-SQL `INSERT INTO projects (id, name,
  kunde_id, status, company_id) VALUES (...)` OHNE das Pflichtfeld `nummer`
  an (`projects.nummer TEXT NOT NULL`, kein Default) — dieser Fehler war
  vorher nie sichtbar, weil der Test schon beim Login (404) abbrach, bevor
  er die INSERT-Zeile überhaupt erreichte. `nummer` mit Platzhalterwert
  `"PRJ-GAEB-PROCESS-0001"` ergänzt.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. `TestGAEBImportProcessEndpoint`
  isoliert: PASS (Login, Erfolgsfall, Konflikt-409, Forbidden-403,
  Default-Parser-Erfolg, Fehlt-Parser-500 — alle Teilschritte grün). Voller
  `go test ./...` (Nicht-Integrationspakete, alle 15 Pakete inkl.
  `internal/http`) ohne Fehler — bestätigt, dass keine der 12 anderen
  `NewRouterWithDeps`-Aufrufstellen durch den Umbau beeinträchtigt wurde.
  Voller `go test ./internal/http/... -count=1` gegen frische DB: von 12
  auf 11 Fehlschläge zurückgegangen (exakt der behobene Test, kein
  Netto-Regress).
- **Backlog 0.32 erneut geprüft und NICHT als Bugfix umgesetzt** (kein
  Anwendungscode-Pfad legt einen Kontenrahmen für einen neuen Mandanten
  an). Beim Versuch der Umsetzung als normaler Epic-0-Subtask festgestellt,
  dass der ursprüngliche Fund den tatsächlichen Umfang unterschätzte:
  `server/internal/settings/company.go` (`CompanyService`) hat
  AUSNAHMSLOS jede Methode (`Get`, `Upsert`, `ListBranches`,
  `ensureBranchCodeUnique`, u. a.) fest auf `id='default'` bzw.
  `company_id='default'` verdrahtet — `Upsert`s INSERT-Statement hat
  `'default'` sogar als SQL-Literal statt als Parameter (Zeile 99). Es gibt
  in der gesamten Anwendung folglich nicht nur keinen
  Kontenrahmen-Kopier-Schritt, sondern GAR KEINEN Weg, überhaupt eine
  ZWEITE `company_profiles`-Zeile anzulegen — die Mandantenfähigkeit aus
  Backlog 0.2 (Epic 0.2.1/0.2.2) hat ausschließlich das Datenmodell
  (`company_id`-Spalten) und den Lese-/Schreib-Scoping-Filter fertig
  gestellt, kein tatsächliches "neuen Mandanten anlegen"-Feature.
  Eingeschätzt als eigenständiges Epic-Feature ("Mandanten-Onboarding") mit
  offenen Produktentscheidungen (Self-Service vs. Ops-Vorgang; was genau
  wird beim Anlegen kopiert), das nicht in den Rahmen eines einzelnen
  Epic-0-Qualitäts-Subtasks passt. Bewusst NICHT als Attrappen-Fix
  umgesetzt (ein isolierter `CopyChartOfAccounts`-Helfer ohne echten
  Aufrufer hätte keinen Nutzen). `docs/backlog.md`-Eintrag entsprechend um
  diese präzisere Einschätzung erweitert, bleibt offen. Kein Blocker für
  die weitere Epic-0-Abarbeitung (produktiv existiert aktuell ohnehin nur
  ein Mandant, `default`) — direkt mit 0.33 fortgefahren.
- **Backlog 0.33 behoben** (`quotes`-Domäne: Draft-/Versions-Schreibschutz
  liefert 500 statt 400). Root Cause:
  `server/internal/quotes/service.go` enthält in praktisch jeder
  schreibenden Methode dieselben zwei `errors.New(...)`-Meldungen ("nur
  Entwürfe sind bearbeitbar" bei `currentStatus != "draft"`, "Historische
  Angebotsversionen sind schreibgeschützt" bei gesetztem
  `superseded_by_quote_id`) — der GoBD-relevante Schreibschutz selbst
  greift korrekt, aber keine der beiden Meldungen matchte ein Substring-
  Muster in `classifyDomainError`, sodass sie auf den `default`-Zweig
  fielen und `500 internal_error` statt `400 validation_error` lieferten.
  Per `grep -c` präzisiert: 15× "nur Entwürfe sind bearbeitbar" und 16×
  "Historische Angebotsversionen sind schreibgeschützt" im Code (mehr als
  die ursprünglich grob geschätzten "16 Fundstellen" insgesamt).

  Fix: in `server/internal/http/v1.go` `classifyDomainError` um
  `"nur entwürfe sind bearbeitbar"` und `"schreibgeschützt"` im
  `validation_error`(400)-Zweig ergänzt. Vorher geprüft (per `grep -rn
  "schreibgeschützt" internal --include=*.go`), dass dieser Substring im
  gesamten Nicht-Test-Code AUSSCHLIESSLICH in `quotes/service.go` mit
  dieser einen Bedeutung vorkommt — kein Kollisionsrisiko mit einer
  anderen Domäne, die denselben Substring für einen anderen HTTP-Status
  bräuchte.

  **Testabdeckung**: keine bestehende Testfunktion erreichte diesen Pfad
  tatsächlich — der naheliegende Kandidat
  `TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource`
  (`quotes_integration_test.go:5821`, PATCH auf eine per `/revise`
  historisch gewordene Quote, erwartet bereits `400`) scheitert VORHER an
  Backlog 0.34 ("conn busy" in `Revise` selbst), sodass diese Assertion nie
  erreicht wird — konnte also nicht als Beleg genutzt werden. Stattdessen
  neuen, eigenständigen Test `TestQuoteUpdateRejectsNonDraftStatusWithValidationError`
  ergänzt (direkt nach `TestQuoteStatusBlocksOpenApprovalRework`, um den
  bereits vorhandenen `seedHTTPApprovalDecisionQuote`/`updateHTTPQuoteStatus`-Helferbaukasten
  wiederzuverwenden): Entwurf per `seedHTTPApprovalDecisionQuote` anlegen,
  per `POST /quotes/{id}/status` direkt auf `accepted` setzen (gültiger
  Direktübergang ohne offene Freigabe-Nacharbeit, siehe `UpdateStatus`),
  danach `PATCH /quotes/{id}` mit leerem Body versuchen (die
  Status-Prüfung in `Update` greift bereits vor jeder Feldvalidierung) und
  `400 validation_error` mit der Meldung "nur Entwürfe sind bearbeitbar"
  erwarten. Der `supersededByQuoteID`-Zweig ("schreibgeschützt") bleibt
  vorerst nur indirekt über `TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource`
  abgedeckt, sobald Backlog 0.34 behoben ist und dieser Test wieder bis zu
  seinen eigenen Assertions durchläuft.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Neuer Test isoliert: PASS
  (Log bestätigt: `PATCH .../quotes/{id}` auf einer `accepted`-Quote
  liefert jetzt `400 validation_error "nur Entwürfe sind bearbeitbar"`
  statt vorher `500`). Voller `go test ./...` (Nicht-Integrationspakete)
  ohne Fehler. Voller `go test ./internal/http/... -count=1` gegen frische
  DB: weiterhin **11 Fehlschläge** (unveränderte Zahl — dieser Fund hatte
  keine vorher bereits fehlschlagende Testabdeckung, daher kein direkter
  Rückgang der Fehlerzahl möglich, aber dauerhafte neue Testabdeckung für
  einen zuvor komplett ungetesteten Pfad).
- **Backlog 0.34 behoben** (KRITISCH: `quotes.Service.Revise` scheitert
  mit "conn busy" bei JEDEM Angebot mit Positionen). Root Cause in
  `server/internal/quotes/service.go` (`Revise`, ab Zeile ~3340): eine
  `tx.Query(...)`-Iteration über die Quell-Positionen (`quote_items` der
  Ursprungs-Quote) wurde INNERHALB derselben `for rows.Next()`-Schleife mit
  einem `tx.Exec(INSERT INTO quote_items ...)` auf DERSELBEN
  Transaktion/Connection kombiniert, bevor die äußere `rows`-Iteration
  vollständig gelesen/geschlossen war — ein klassischer pgx-v5-Fehler
  (eine Connection kann laut pgx-Protokoll kein zweites Statement
  ausführen, während ein vorheriges `Query`-Ergebnis noch offen ist).
  Ergebnis: `ERROR: conn busy`, `POST /api/v1/quotes/{id}/revise` lieferte
  `500` für JEDES Angebot mit mindestens einer Position — da
  `quotes.Create` mindestens eine Position zwingend voraussetzt, war
  `/revise` de facto für JEDES real angelegte Angebot komplett unbenutzbar
  (daher KRITISCH).

  **Fix**: die Positionen werden jetzt zuerst vollständig in einen
  Go-Slice (`[]sourceQuoteItem`) eingelesen — die `rows.Next()`-Schleife
  enthält KEIN `tx.Exec(...)` mehr, `rows.Close()` wird direkt nach dem
  Einlesen aufgerufen, VOR der zweiten (schreibenden) Schleife über den
  Slice, die die `INSERT INTO quote_items ...`-Statements ausführt. Analog
  zum bereits korrekten Zwei-Phasen-Muster in `Book`/`Storno`
  (`accounting/ar.go`).

  **Cascading-Fund beim End-to-End-Verifizieren**: mit funktionierendem
  `Revise` durchlief `TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource`
  erstmals bis zu seinen nachfolgenden Schreibschutz-Assertions auf der
  historischen (superseded) Ursprungs-Quote — dort schlug
  `POST /convert-to-invoice` mit `500` statt dem erwarteten `400` fehl.
  Root Cause: derselbe `classifyDomainError`-Mechanismus wie Backlog 0.33,
  aber eine ANDERE, dort noch nicht abgedeckte Meldungsfamilie. Per
  `grep -rn "überführt" internal --include=*.go` (Nicht-Test-Code)
  systematisch ALLE 9 Fundstellen dieser Meldungsfamilie geprüft, verteilt
  über `server/internal/quotes/service.go` (`ConvertToInvoice`) UND
  `server/internal/sales/service.go` (`CreateFromQuote`,
  `ConvertToInvoice`) — dabei festgestellt, dass die Meldung "Historische
  Angebotsversionen können nicht in Folgebelege überführt werden"
  WORTGLEICH in BEIDEN Domänen dupliziert ist (analog zum bereits in
  Backlog 0.36 dokumentierten Duplizierungsmuster, dort für
  Steuerkennzeichen-Fallbacks — 0.36 in `state.md`/`backlog.md` um diese
  Bestätigung ergänzt). Alle 9 Treffer sind reine Validierungsfehler (400,
  nie 500): "Historische Angebotsversionen können nicht in Folgebelege
  überführt werden", "Angebot wurde bereits in eine Rechnung überführt",
  "Angebot wurde bereits in einen Auftrag überführt", "nur versendete oder
  angenommene Angebote können in Rechnungen überführt werden", "nur
  angenommene Angebote können in Aufträge überführt werden", "nur offene
  oder freigegebene Aufträge können in Rechnungen überführt werden". Dazu
  separat "Angebot mit Folgebeleg kann nicht manuell umgestellt werden"
  (`UpdateStatus`, dieselbe Meldungsfamilie, einzige Fundstelle) geprüft
  und mitergänzt. `classifyDomainError`
  (`server/internal/http/v1.go`) um `"überführt"` (deckt alle 9 Treffer
  ab) und `"kann nicht manuell umgestellt werden"` ergänzt — beide vorher
  auf Kollisionsfreiheit mit anderen, absichtlich 500 bleibenden Meldungen
  geprüft (keine gefunden).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean.
  `TestQuoteReviseEndpointClonesQuoteAndGuardsSupersededSource` isoliert:
  PASS, alle 6 Teilschritte grün (Revise → 201; erneutes Revise auf
  bereits revidierter Quote → 400 "Angebot darf nicht erneut revidiert
  werden"; PATCH bzw. Status-Update auf superseded Quote → je 400
  "schreibgeschützt"; convert-to-invoice auf superseded Quote → 400
  "…überführt werden"; Annahme der revidierten Quote → 200;
  convert-to-sales-order auf superseded Quote → 400). Voller `go test
  ./...` (Nicht-Integrationspakete) ohne Fehler. Voller `go test
  ./internal/http/... -count=1` gegen frische DB: von 11 auf 10
  Fehlschläge zurückgegangen (exakt der behobene Test, kein Netto-Regress).
- **Backlog 0.35 behoben** (KRITISCH: `sales.Service.ConvertToInvoice`
  scheitert immer mit Postgres-Fehler "FOR UPDATE cannot be applied to the
  nullable side of an outer join"). Root Cause in
  `server/internal/sales/service.go` (`loadForInvoiceTx`): `SELECT so.id,
  ... FROM sales_orders so LEFT JOIN projects p ON p.id = so.project_id
  LEFT JOIN contacts c ON c.id = so.contact_id WHERE so.id=$1 AND
  so.company_id=$2 FOR UPDATE` — Postgres verbietet ein `FOR UPDATE` ohne
  explizite Tabellenangabe auf einer Query mit `LEFT JOIN`, sobald die
  Zeilensperre die nullable Seite eines Outer Joins betreffen könnte
  (`project_id` ist nullable, daher die nullable Seite des ersten
  `LEFT JOIN`). Ergebnis: `ERROR: FOR UPDATE cannot be applied to the
  nullable side of an outer join` (SQLSTATE 0A000) bei JEDEM Aufruf,
  unabhängig von Status oder Daten — `POST
  /api/v1/sales-orders/{id}/convert-to-invoice` war dadurch komplett
  unbenutzbar (analog schwerwiegend zu Backlog 0.34, dem einzigen
  verbliebenen KRITISCH-Fund neben diesem).

  **Fix**: `FOR UPDATE` zu `FOR UPDATE OF so` geändert — schränkt die
  Zeilensperre explizit auf die `sales_orders`-Zeile ein und umgeht damit
  die Postgres-Restriktion für die gejointen (nullable) Tabellen. Exakt
  dasselbe Muster wie das bereits vorhandene, korrekte
  `FOR UPDATE OF quote_imports` in `server/internal/quotes/imports.go`,
  das dieses identische Problem in einer früheren Subtask dieser Session
  schon einmal gelöst hatte — hier lediglich dieselbe bekannte Lösung auf
  eine bis dahin übersehene zweite Fundstelle angewendet.

  **Kollisionsprüfung vor dem Fix**: `grep -n "FOR UPDATE"
  internal/sales/service.go` zeigt 5 Treffer — nur `loadForInvoiceTx` hat
  einen `JOIN`, die übrigen 4 (`UpdateStatus`, `MarkInvoiced` u. a.) sind
  Single-Table-Queries (`FROM sales_orders`, keine `JOIN`s) und daher vom
  Bug nicht betroffen. Ebenso stichprobenartig `internal/quotes/service.go`
  geprüft: viele `FOR UPDATE`-Treffer, alle Single-Table (`FROM quotes`
  bzw. `FROM quote_items`), kein zusätzlicher Fund in dieser Datei.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean.
  `TestSalesOrderStatusChangeAndConvertToInvoiceAreAuditLogged` isoliert:
  PASS (Log bestätigt: `POST .../convert-to-invoice` liefert jetzt `201`
  mit vollständigem Rechnungs-Payload statt `500`). Voller `go test ./...`
  (Nicht-Integrationspakete) ohne Fehler. Voller `go test
  ./internal/http/... -count=1` gegen frische DB: von 10 auf **8**
  Fehlschläge zurückgegangen — neben dem Zieltest verschwand auch
  `TestCommercialWorkflowEndpointListsOpenFollowActions` aus der
  Fehlerliste (dieser Test nutzte für eine Teilrechnungs-Prüfung denselben
  vorher blockierten `convert-to-invoice`-Pfad, hatte aber keinen eigenen
  Backlog-Eintrag — durch denselben Fix ohne weiteres Zutun mitbehoben).
  Damit sind BEIDE als KRITISCH markierten Funde dieser Session (0.34,
  0.35) abgeschlossen.
- **Backlog 0.36 behoben** (Steuerkennzeichen-Fallback-Bug aus Backlog 0.7
  war dupliziert in `quotes`/`sales`, dort nicht gegen Stammdaten
  abgesichert). Root Cause: `internal/quotes/service.go` (`taxRate(code
  string) float64`) und `internal/sales/service.go` (identische eigene
  `taxRate`-Funktion) enthielten JEWEILS eine eigene, unabhängige Kopie
  desselben hartcodierten `switch { case "DE19": 0.19; case "DE7": 0.07;
  default: 0 }` — unbekannte/inaktive Steuerkennzeichen wurden STILL als
  0% behandelt statt einen Fehler zu liefern, exakt das bereits in Backlog
  0.7 für `accounting/ar.go` behobene Muster, hier aber noch offen.

  **Design-Entscheidung**: von den beiden im ursprünglichen Fund
  vorgeschlagenen Optionen wurde bewusst die kleinere gewählt — denselben
  `loadTaxCodes`-Ansatz UNABHÄNGIG in `quotes` und `sales` wiederholen
  (analog zu `accounting/ar.go`), NICHT die sauberere, aber deutlich
  größere Alternative (`taxCodeInfo`/`loadTaxCodes`/`taxRate` in ein
  gemeinsames Package wie `internal/tax` verschieben und alle drei
  Domänen inkl. `accounting` darauf umstellen). Begründung: Letzteres hätte
  zusätzlich `accounting/ar.go` und dessen bereits grün laufende,
  etablierte Tests angefasst, ohne dass das für die Behebung DIESES Funds
  notwendig gewesen wäre — höheres Risiko für denselben Nutzen. Als
  möglicher separater, größerer Folge-Refactor dokumentiert, falls vom
  Nutzer gewünscht.

  **Fix**: in `internal/quotes/service.go` und `internal/sales/service.go`
  jeweils unabhängig ein `taxCodeInfo{Rate float64}`-Typ, eine
  `loadTaxCodesTx(ctx, tx pgx.Tx) (map[string]taxCodeInfo, error)`
  (`SELECT code, rate FROM tax_codes WHERE is_active`) sowie eine neue
  Signatur `taxRate(codes map[string]taxCodeInfo, code string) (float64,
  error)` ergänzt — leerer Code bleibt 0%/kein Fehler (Skonto-/Durchlaufposten),
  ein nicht-leerer aber unbekannter/inaktiver Code liefert jetzt
  `fmt.Errorf("unbekanntes oder inaktives Steuerkennzeichen: %s", code)`.
  `quotes.calcTotals` von `(items []QuoteItemInput) (net, tax float64)` auf
  `(codes map[string]taxCodeInfo, items []QuoteItemInput) (net, tax
  float64, err error)` umgestellt. Alle Aufrufstellen angepasst und
  jeweils `codes, err := loadTaxCodesTx(ctx, tx)` EINMAL pro Transaktion
  geladen (nicht pro Item, um unnötige wiederholte Queries zu vermeiden):
  - `quotes/service.go`: `createQuoteTx`, `ApplyPrimaryPriceSourceForQuoteItem`,
    `ApplyTargetUnitPriceForQuoteItem`, `Update` — `tx` war in allen vier
    Funktionen bereits im Scope vorhanden, keine Signaturänderung an
    exportierten Methoden nötig.
  - `sales/service.go`: `CreateFromQuote`, `CreateItem`, `UpdateItem`,
    `recalculateTotalsTx` (freie Helferfunktion, von `Update`/`CreateItem`/
    `UpdateItem`/`DeleteItem` über `refreshOrderTotalsTx` aufgerufen —
    laedt `codes` intern selbst, da sie von vier unterschiedlichen
    Top-Level-Operationen aus aufgerufen wird und kein gemeinsamer
    Aufrufer existiert, der `codes` einmalig haette durchreichen koennen).

  **Bestehenden Unit-Test korrigiert**: `sales/service_test.go`
  (`TestSalesOrderTaxRateKnownAndUnknownCodes`) testete bisher EXPLIZIT
  das alte, jetzt korrigierte Verhalten (`cases := map[string]float64{...,
  "XX": 0}` — erwartete für einen unbekannten Code `0.0` statt eines
  Fehlers). Umgeschrieben auf die neue Fehler-Semantik, exakt analog zu
  `accounting/ar_test.go` (`TestTaxRateKnownAndUnknownCodes`, bereits
  seit Backlog 0.7 in diesem Format vorhanden) — bewusst dasselbe
  Testmuster übernommen statt ein eigenes zu erfinden.

  **Fehlende Testabdeckung in `quotes` ergänzt**: `quotes/service_test.go`
  hatte bisher GAR KEINE Tests für `taxRate`/`calcTotals` (nur einen
  einzigen Test zu `Create`s companyID-Validierung). Drei neue Tests
  ergänzt: `TestQuoteTaxRateKnownAndUnknownCodes` (bekannte Codes,
  leerer Code, unbekannter Code → Fehler), `TestQuoteCalcTotalsSumsNetAndTax`
  (Summenbildung über mehrere Positionen mit unterschiedlichen
  Steuersätzen), `TestQuoteCalcTotalsRejectsUnknownTaxCode`.

  **Cascading-Fund beim HTTP-Verifizieren**: die neue Fehlermeldung
  `"unbekanntes oder inaktives Steuerkennzeichen: %s"` traf KEIN
  Substring-Muster in `classifyDomainError` und fiel auf `500
  internal_error` statt `400 validation_error` — dieser Gap existierte
  bereits SEIT Backlog 0.7 (dort wurde nur die Service-Ebene in
  `accounting/ar.go` korrigiert, aber nie über einen HTTP-Test verifiziert,
  da kein bestehender Test dort einen unbekannten Steuercode auslöst; bei
  `quotes` wird der Pfad jetzt ERSTMALS über HTTP erreichbar, da `quotes`
  vorher gar keine Tax-Code-Validierung hatte). `classifyDomainError`
  (`server/internal/http/v1.go`) um `"unbekanntes oder inaktives
  steuerkennzeichen"` sowie (gleiche Meldungsfamilie, `taxAccountFor` in
  `accounting/ar.go`) `"kein umsatzsteuer-konto"` ergänzt — betrifft
  rückwirkend auch `accounting`, obwohl dort nicht Teil des heutigen
  Codeaenderungs-Scopes.

  **Neuer HTTP-Test**: `TestQuoteCreateRejectsUnknownTaxCode`
  (`server/internal/http/quotes_integration_test.go`) — `POST /quotes/`
  mit `tax_code:"XX99"` erwartet jetzt `400 validation_error` mit der
  neuen Meldung (vorher: stille 0%-Behandlung, `201` mit fachlich falscher
  — zu niedriger — Steuer, ein reales, bisher unbemerktes GoBD-relevantes
  Korrektheitsproblem). Für `sales` existierte für denselben `tax_code`-Wert
  (`"XX99"`) bereits ein Test (`POST /sales-orders/{id}/items` in
  `quotes_integration_test.go`) — bei genauerer Prüfung festgestellt, dass
  dieser NICHT über den hier geänderten `taxRate`-Pfad läuft, sondern über
  eine separate, bereits VORHER bestehende `isSupportedTaxCode`/
  `validateItemInput`-Whitelist-Prüfung in `sales/service.go`
  (`CreateItem`/`UpdateItem`), die unbekannte Codes schon VOR `taxRate`
  abfängt (eigener, unabhängiger Validierungspfad, nicht Teil dieses
  Fixes). Mein Fix greift dort zusätzlich als Sicherheitsnetz für Pfade
  OHNE diese Whitelist-Prüfung (`CreateFromQuote`, `recalculateTotalsTx` —
  relevant z. B. bei per GAEB-Import oder Direkt-SQL eingespielten,
  nicht durch die Whitelist geprüften Tax-Codes).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. `go test
  ./internal/quotes/... ./internal/sales/... ./internal/accounting/...
  -count=1 -v`: alle PASS (inkl. der 3 neuen und 1 korrigierten
  Unit-Tests). `TestQuoteCreateRejectsUnknownTaxCode` isoliert gegen
  frische DB: PASS (Log bestätigt `400 validation_error "unbekanntes oder
  inaktives Steuerkennzeichen: XX99"`). Voller `go test ./...`
  (Nicht-Integrationspakete) ohne Fehler. Voller `go test
  ./internal/http/... -count=1`: weiterhin **8** Fehlschläge (identische
  Liste wie vor diesem Subtask — keiner der 8 verbleibenden Fälle hing mit
  Steuerkennzeichen zusammen, daher kein direkter Rückgang, aber kein
  Regress und ein zuvor stiller GoBD-relevanter Korrektheitsfehler ist
  jetzt dauerhaft abgesichert).
- **Backlog 0.37 behoben** (`BankService` an keinen HTTP-Handler
  angebunden). Root Cause: `server/internal/accounting/bank.go`
  (`BankService` mit `Ingest`/`List`/`Match`, vollständig implementiert
  und seit Backlog 0.9 durch Service-Ebenen-Tests abgedeckt) wurde
  nirgends in `server/internal/http/v1.go` konstruiert oder an eine Route
  gebunden (`grep -rln "NewBankService"` zeigte nur die Definition selbst)
  — das gesamte Bankabgleich-Feature war über die API vollständig
  unerreichbar.

  **Fix**: `bankSvc := accounting.NewBankService(pg, paymentSvc)` in
  `v1.go` ergänzt (direkt neben der bestehenden `arSvc`/`paymentSvc`-
  Konstruktion). Neue Routengruppe `protected.Route("/bank-statements",
  ...)` zwischen den bestehenden `/invoices-out`- und `/sales-orders`-
  Gruppen eingefügt:
  - `GET /bank-statements/` (Permission `bank.read`) → `bankSvc.List(ctx,
    limit, companyID)`, `?limit=` optional.
  - `POST /bank-statements/` (Permission `bank.write`) → dekodiert direkt
    in `accounting.BankStatementInput` (JSON-Tags bereits im Service-Typ
    definiert, keine separate HTTP-DTO nötig), ruft `bankSvc.Ingest(...)`.
  - `POST /bank-statements/{id}/match` (Permission `bank.write`) →
    optionaler Body `{"invoice_id": "..."}` (leer/kein Body = automatische
    Referenz-/Betrags-Heuristik in `Match`), ruft `bankSvc.Match(...)`.

  Neue Migration `server/internal/migrate/migrations/065_bank_permissions.sql`:
  Permissions `bank.read`/`bank.write` angelegt und an `role-finance` UND
  `role-admin` vergeben — exakt dasselbe Muster wie das bereits bestehende
  `030_accounting_permissions.sql` für `invoices_out.read`/`.write` (admin
  besitzt zusätzlich `admin.superuser` als Bypass, aber die explizite
  Vergabe folgt der etablierten Konvention).

  **Cascading-Fund beim HTTP-Verifizieren**: die Fehlermeldung "Statement
  bereits gematcht" (`Match`, ausgelöst beim erneuten Matching-Versuch
  eines bereits verknüpften Kontoauszugs) traf kein Substring-Muster in
  `classifyDomainError` und fiel auf `500 internal_error` statt `400
  validation_error` — dieser Pfad war vorher nie über HTTP erreichbar,
  daher auch nie über einen HTTP-Test sichtbar geworden. Substring
  `"bereits gematcht"` ergänzt (per `grep` als einzige Fundstelle im
  Nicht-Test-Code bestätigt, kollisionsfrei).

  **Neuer HTTP-Test** `TestBankStatementsIngestListAndMatchFlow`
  (`server/internal/http/accounting_integration_test.go`) deckt den
  kompletten neuen Flow end-to-end ab: Rechnung anlegen und buchen (Status
  `booked`), `403` beim Ingest-Versuch mit einer Rolle ohne `bank.write`
  (`procurement`), `201` für den eigentlichen Ingest, `200` für die Liste
  (bestätigt, dass der eingespielte Kontoauszug enthalten ist), `200` für
  den Match mit explizitem `invoice_id` (inkl. tatsächlicher
  Zahlungsanwendung — die Rechnung wechselt zu Status `paid` mit exaktem
  `paid_amount`), `400` beim Versuch, denselben, bereits gematchten
  Kontoauszug erneut zu matchen. Amount/Tax-Code bewusst auf `DE19`/119
  (statt eines leeren TaxCode) gewählt, um NICHT versehentlich in den
  bereits separat dokumentierten, noch offenen Backlog 0.38 zu laufen
  (leerer TaxCode verletzt aktuell eine Fremdschlüssel-Constraint in
  `ar.go`) — beim ersten Testlauf mit `tax_code:""` tatsächlich genau
  diesen bekannten Fehler reproduziert und danach bewusst auf `DE19`
  umgestellt, um den Test unabhängig von 0.38 zu halten.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Migration `065` wendet sich
  sauber an (Log bestätigt, keine Konflikte). Bestehende `TestBank*`-Tests
  aus Backlog 0.9 (`internal/accounting/bank_integration_test.go`)
  weiterhin alle PASS (reine Service-Ebene, durch die HTTP-Verdrahtung
  unverändert). Neuer HTTP-Test isoliert: PASS (alle 5 Teilschritte
  grün). Voller `go test ./...` (Nicht-Integrationspakete) ohne Fehler.
  Voller `go test ./internal/http/... -count=1` gegen frische DB:
  weiterhin **8** Fehlschläge (identische Liste, kein Regress).
- **Backlog 0.38 behoben** (`accounting.ARService.createTx` verletzt
  Fremdschlüssel bei leerem `TaxCode`). Root Cause:
  `server/internal/accounting/ar.go` (`createTx`, `INSERT INTO
  invoice_out_items`) übergab `it.TaxCode` als rohen Go-`string` direkt an
  `tx.Exec(...)` — bei `TaxCode: ""` interpretiert pgx das als leere
  SQL-Zeichenkette `''`, nicht als `NULL`. Die Spalte
  `invoice_out_items.tax_code text REFERENCES tax_codes(code)` ist zwar
  nullable, aber es existiert kein `tax_codes`-Eintrag mit `code=''` —
  jeder Insert-Versuch mit leerem `tax_code` verletzte damit die
  Fremdschlüssel-Constraint `invoice_out_items_tax_code_fkey` und lieferte
  `500`. Das widersprach dem fachlichen Verhalten der darüberliegenden
  Funktionen: `calcTotals`/`taxRate` (`accounting/ar.go`, seit Backlog
  0.7) behandeln einen leeren `TaxCode` explizit und bewusst als gültigen
  "steuerfrei"-Fall (0% Steuer, kein Fehler) — betraf also JEDE reale
  Rechnungsposition ohne Steuerkennzeichen (z. B. durchlaufende
  Posten/Skonto).

  **Fix**: neue Hilfsfunktion `nullIfEmpty(v string) any` in
  `accounting/ar.go` ergänzt — identisches Muster wie die bereits in
  `quotes/service.go`, `sales/service.go` und `projects/service.go`
  jeweils unabhängig vorhandenen gleichnamigen Funktionen (`if
  strings.TrimSpace(v) == "" { return nil }; return v`). Im INSERT von
  `createTx` `it.TaxCode` durch `nullIfEmpty(it.TaxCode)` ersetzt.

  **Wichtige Korrektur der ursprünglichen Fund-Beschreibung**: der
  ursprüngliche Fund schlug vor, DENSELBEN `nullIfEmpty`-Fix AUCH auf
  `AccountCode` anzuwenden. Beim genauen Nachlesen des Schemas
  (`server/internal/migrate/migrations/018_journal_and_ar.sql:49`)
  stellte sich heraus: `account_code text NOT NULL REFERENCES
  accounts(code)` — im Gegensatz zu `tax_code` ist diese Spalte NICHT
  nullable. `nullIfEmpty(it.AccountCode)` hätte also nur einen ANDEREN
  rohen Postgres-Fehler erzeugt (NOT-NULL-Verletzung statt FK-Verletzung),
  keinen echten Fix dargestellt. Stattdessen: ein leerer `AccountCode` ist
  tatsächlich ein Pflichtfeld-Validierungsfehler und wird jetzt VOR dem
  Insert explizit geprüft — `if strings.TrimSpace(it.AccountCode) == ""
  { return nil, errors.New("account_code fehlt") }`, exakt nach demselben
  "X fehlt"-Meldungsmuster wie das bereits bestehende `"contact_id
  fehlt"` in derselben Funktion — liefert jetzt sauber `400
  validation_error` statt einer rohen SQL-Fehlermeldung.

  **Neuer Test**
  `TestInvoiceOutCreateAcceptsEmptyTaxCodeAndRejectsEmptyAccountCode`
  (`server/internal/http/accounting_integration_test.go`) deckt beide
  Fälle in einem Test ab: zuerst `POST /invoices-out/` mit leerem
  `account_code` → erwartet `400` mit `"account_code fehlt"`; danach
  dieselbe Rechnung mit zwei Positionen (eine mit leerem `tax_code` +
  gültigem `account_code`, eine mit `tax_code:"DE19"`) → erwartet `201`
  und prüft `net_amount=125`, `tax_amount=19` (nur die DE19-Position wird
  besteuert), `gross_amount=144`, sowie dass das leere `tax_code` im JSON-
  Response korrekt als `""` erhalten bleibt (kein stiller Datenverlust).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Neuer Test isoliert: PASS (Log
  bestätigt beide Teilschritte: `400 validation_error "account_code
  fehlt"`, danach `201` mit korrekten Summen). Voller `go test ./...`
  (Nicht-Integrationspakete) ohne Fehler. Voller `go test
  ./internal/http/... -count=1` gegen frische DB: von 8 auf **7**
  Fehlschläge zurückgegangen — neben dem Zieltest verschwand auch
  `TestQuoteConvertToInvoiceBlocksOpenApprovalRework` aus der Fehlerliste
  (dieser Test erzeugte im Rahmen seiner eigenen Prüfung ebenfalls eine
  Rechnungsposition mit leerem `tax_code` und war daher vom selben FK-Bug
  betroffen, ohne einen eigenen Backlog-Eintrag zu haben — durch denselben
  Fix ohne weiteres Zutun mitbehoben).
- **Backlog 0.39 behoben** (`TestMaterialGroupDeleteRejectsTrimmedLegacyReferences`
  schlägt auf frischer DB fehl: Testfixture referenziert nicht
  existierende Spalte `materials.updated_at`). Root Cause:
  `server/internal/http/settings_integration_test.go`
  (`TestMaterialGroupDeleteRejectsTrimmedLegacyReferences`) seedete ein
  Material per Direkt-SQL mit `INSERT INTO materials (...) ... ON
  CONFLICT (id) DO UPDATE SET kategorie = EXCLUDED.kategorie, updated_at
  = now()` — die Spalte `materials.updated_at` existiert im Schema
  jedoch nicht: `server/internal/migrate/migrations/001_init.sql`
  (`CREATE TABLE materials (...)`) definiert nur `angelegt_am
  timestamptz`, kein `updated_at`; per `grep -rn "ALTER TABLE materials"`
  über alle Migrationen bestätigt, dass keine spätere Migration diese
  Spalte je hinzufügt. `ERROR: column "updated_at" of relation
  "materials" does not exist`.

  **Fix-Entscheidung**: von den beiden im ursprünglichen Fund genannten
  Optionen (Spalte ergänzen vs. Testfixture korrigieren) wurde die
  Testfixture-Korrektur gewählt — geprüft per `grep -rn
  "materials.*updated_at\|\.UpdatedAt" internal/materials`, dass NIRGENDS
  im Anwendungscode (Service-Layer, HTTP-Handler, andere Tests) ein
  `materials.updated_at`-Feld gelesen, geschrieben oder erwartet wird. Die
  fehlerhafte Klausel war ein reines Kopier-Artefakt: der strukturell sehr
  ähnliche `material_groups`-Upsert direkt darüber im selben Test (Zeile
  733-743) hat TATSÄCHLICH ein gültiges `updated_at`
  (`material_groups.updated_at timestamptz`, siehe
  `039_material_groups.sql`) — die `updated_at = now()`-Zeile wurde beim
  Schreiben des zweiten (materials-)Upserts offensichtlich unverändert
  mitkopiert, ohne zu prüfen, ob die Zielspalte in der ZWEITEN Tabelle
  überhaupt existiert. Eine neue Spalte einzuführen nur damit ein
  Test-Fixture kompiliert, wäre eine unbegründete Schema-Erweiterung ohne
  jeden fachlichen Bedarf gewesen.

  **Fix**: in `settings_integration_test.go` die Zeile `updated_at =
  now()` aus der `ON CONFLICT (id) DO UPDATE SET ...`-Klausel des
  `materials`-Inserts entfernt (das `material_groups`-Upsert direkt
  darüber bleibt unverändert, da dessen `updated_at`-Spalte real
  existiert).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Test isoliert: PASS (Log
  bestätigt: `DELETE /settings/material-groups/stahl` liefert korrekt
  `400 "Materialgruppe wird noch von Materialien verwendet"`, da das
  Material mit getrimmter Kategorie-Referenz `' stahl '` jetzt erfolgreich
  geseedet werden kann). Voller `go test ./...` (Nicht-Integrationspakete)
  ohne Fehler. Voller `go test ./internal/http/... -count=1` gegen
  frische DB: von 7 auf **6** Fehlschläge zurückgegangen (exakt der
  behobene Test, kein Netto-Regress).
- **Backlog 0.41 behoben** (`TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`
  schlägt auf frischer DB fehl). Der ursprüngliche Fund vermutete ein
  Cross-Test-Datenleck (verwandt mit Backlog 0.19, Tests teilen sich
  dieselbe Postgres-Instanz ohne Datenbereinigung). Diese Hypothese wurde
  zuerst explizit geprüft: die Fixture-Erzeugung der Angebotsposition
  innerhalb DIESES Tests (`quotes_integration_test.go`,
  `TestQuotePriceSourcePriorityEndpointReturnsPrioritizedVisibleSources`,
  `POST /quotes/` mit `"description":"Preispriorisierung Position"`) ist
  korrekt und eindeutig — es gibt keinen fehlenden Filter, keine
  Vermischung von Datensätzen. Stattdessen: die fehlschlagende Assertion
  (`blockedUpdateReload.Items[0].Description != "Preisquellen
  Position"`) erwartete einen Wert aus einem STRUKTURELL sehr ähnlichen,
  aber ANDEREN Test
  (`TestQuotePriceHistoryEndpointReturnsVisibleSourcesForMappedDraftItem`,
  dessen Fixture `"description":"Preisquellen Position"` verwendet) — ein
  klassischer Copy-Paste-Fehler beim Ableiten der einen Testfunktion von
  der anderen, bei dem diese eine Assertion nicht mit angepasst wurde.
  Exakt dasselbe Fehlermuster wie bereits ausführlich bei Backlog 0.28
  dokumentiert (dort ebenfalls: Fixture korrekt, Assertion falsch,
  copy-paste-verursacht). Fix: Assertion auf `"Preispriorisierung
  Position"` korrigiert.

  **Cascading-Fund**: nach der Assertion-Korrektur lief der Test weiter
  und deckte einen ECHTEN, von der Testfixture unabhängigen Produktivbug
  auf: `POST /quotes/{id}/items/{itemID}/apply-target-price` scheiterte
  IMMER mit `500 ERROR: new row for relation
  "quote_item_price_decisions" violates check constraint
  "chk_quote_item_price_decisions_type"` (SQLSTATE 23514). Root Cause:
  `server/internal/migrate/migrations/048_quote_item_price_decisions.sql`
  legt eine CHECK-Constraint an, die NUR `decision_type IN
  ('primary_source_applied')` erlaubt. `insertTargetPriceAppliedDecisionTx`
  (`server/internal/quotes/service.go`, aufgerufen von
  `ApplyTargetUnitPriceForQuoteItem`, dem Handler hinter
  `apply-target-price`) fügt aber seit jeher `decision_type =
  'target_price_applied'` ein — ein zweiter, in der Migration schlicht
  fehlender erlaubter Wert. Die Migration wurde offensichtlich nie
  aktualisiert, als die Zielpreis-Funktionalität (Zielmarge →
  Zielpreis-Anwendung) zum Code hinzukam; der Endpunkt war seit seiner
  Einführung im Code komplett unbenutzbar, aber vorher durch die falsche
  Testassertion nie bis zu diesem Punkt erreichbar.

  **Fix**: neue Migration
  `server/internal/migrate/migrations/066_quote_item_price_decisions_target_price_type.sql`
  — droppt die bestehende Constraint (falls vorhanden, idempotent) und
  legt sie mit `decision_type IN ('primary_source_applied',
  'target_price_applied')` neu an. Per `grep -n "decision_type.*=.*'"
  internal/quotes/service.go` bestätigt: dies sind die einzigen ZWEI im
  Code tatsächlich als Literal verwendeten `decision_type`-Werte, keine
  weiteren fehlenden Werte übersehen.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Migration `066` wendet sich
  sauber an (Log bestätigt). Test isoliert: PASS, läuft jetzt komplett
  durch (inkl. `apply-target-price` → `200` statt `500`, alle
  nachfolgenden Prüfungen zu Freigabeanforderungen und Angebots-Reset
  ebenfalls grün). Voller `go test ./...` (Nicht-Integrationspakete) ohne
  Fehler. Voller `go test ./internal/http/... -count=1` gegen frische DB:
  von 6 auf **5** Fehlschläge zurückgegangen (exakt der behobene Test,
  kein Netto-Regress).
- **Backlog 0.42 behoben** (`projects`-Testfixtures in `internal/quotes`
  legen Projekte per Direkt-SQL ohne Pflichtfeld `nummer` an). Root
  Cause: `projects.nummer TEXT NOT NULL`
  (`server/internal/migrate/migrations/007_projects.sql`, kein Default) —
  zwei Stellen in `server/internal/quotes/imports_test.go` legten
  Projekte per rohem `INSERT INTO projects (id, name, kunde_id, status,
  company_id) VALUES (...)` an, OHNE `nummer` anzugeben: der Helper
  `createUploadedGAEBImportForProcessingTest` (genutzt von
  `TestProcessGAEBImportStoresParserResult`,
  `TestProcessGAEBImportMarksParserFailure`,
  `TestProcessGAEBImportRequiresConfiguredParserWithoutMutation` — 3
  Tests über einen gemeinsamen Helper) sowie eine separate Stelle in
  `TestQuoteImportParseResultStoresItemsAndUpdatesStatus`. Beide
  scheiterten mit `ERROR: null value in column "nummer" of relation
  "projects" violates not-null constraint` (SQLSTATE 23502).

  **Fix**: in beiden `INSERT`-Statements `nummer` mit je einem
  eindeutigen Platzhalterwert ergänzt (`"PRJ-GAEB-PROCESSING-0001"` bzw.
  `"PRJ-GAEB-IMPORT-0001"`) — `nummer` wird in diesen Tests fachlich
  nicht geprüft, ein Platzhalter genügt, exakt analog zum bereits
  identischen Fix in Backlog 0.31
  (`TestGAEBImportProcessEndpoint`/`quotes_integration_test.go`). Per
  `grep -rn "INSERT INTO projects" internal/quotes/` bestätigt: dies sind
  die einzigen zwei betroffenen Fundstellen im gesamten `quotes`-Paket,
  keine weiteren übersehen.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Alle vier betroffenen Tests
  isoliert (`go test ./internal/quotes/... -run
  "TestProcessGAEBImportStoresParserResult|TestProcessGAEBImportMarksParserFailure|TestProcessGAEBImportRequiresConfiguredParserWithoutMutation|TestQuoteImportParseResultStoresItemsAndUpdatesStatus"`):
  alle PASS. Voller `go test ./...` (alle 15 Pakete) ohne Fehler. Voller
  `go test ./internal/http/... -count=1` gegen frische DB: weiterhin **5**
  Fehlschläge (unverändert — die vier behobenen Tests liegen im
  `internal/quotes`-Paket, nicht in `internal/http`, daher keine
  Veränderung dieser Zahl erwartet; kein Regress).
- **Backlog 0.43 behoben** (KRITISCH: `listMaterialCandidatesForQuoteItem`
  ist nicht nach `company_id` gescoped — Materialien FREMDER Mandanten
  wurden als GAEB-Import-Materialkandidaten vorgeschlagen). Root Cause in
  `server/internal/quotes/service.go`: die Funktion jointe `materials m`
  ausschließlich über `LOWER(m.bezeichnung) = LOWER(BTRIM(qii.description))
  OR LOWER(m.nummer) = LOWER(BTRIM(qii.description))`, OHNE jede
  Einschränkung auf `m.company_id` — ein echtes, mandantenübergreifendes
  Datenleck (vergleichbare Schwere wie das bereits behobene Backlog 0.22).

  **Fix**: `listMaterialCandidatesForQuoteItem` um einen `companyID
  string`-Parameter erweitert, an der einzigen Aufrufstelle (in `Get`,
  wo `companyID` bereits im Scope war) durchgereicht, SQL um `AND
  m.company_id = $2` erweitert. Vorher geprüft: `materials.company_id`
  ist seit Migration 057 für ALLE Zeilen mindestens auf `'default'`
  befüllt (kein NULL-Risiko, das die Filterung unbeabsichtigt leer
  laufen ließe).

  **Wichtige Korrektur während der Verifikation**: nach dem
  `company_id`-Fix schlug `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates`
  im VOLLEN Suite-Lauf WEITERHIN mit demselben Symptom fehl (zwei statt
  ein Kandidat). Grund: `MAT-GAEB-0001` (aus der unabhängigen
  `TestQuoteUpdateAllowsManualMaterialMappingOnItems`) gehört — wie
  praktisch alle Testdaten dieser Session — ebenfalls zu
  `company_id='default'`, da es aktuell (Backlog 0.32) GAR KEINEN Weg
  gibt, über die Anwendung einen ECHTEN zweiten Mandanten anzulegen. Der
  ursprüngliche Fund (0.43) hatte die beiden kollidierenden
  Test-Materialien fälschlich als "aus unterschiedlichen Companies"
  beschrieben — tatsächlich handelte es sich um eine reine
  Testdaten-Kollision INNERHALB derselben Company (zwei unabhängige
  Tests verwenden zufällig dieselbe Materialbezeichnung "Aluminium
  Profil 70mm"), die der `company_id`-Fix allein nicht lösen konnte (er
  ist trotzdem die korrekte, notwendige Absicherung für eine echte
  Mehr-Mandanten-Zukunft). Zusätzlicher Fix: die Materialbezeichnung/
  Positionsbeschreibung in `TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates`
  auf `"Aluminium Profil 70mm Kandidatenpruefung"` umbenannt (4
  Fundstellen: Material-Anlage, Import-Item-Beschreibung, 2 Assertions).

  **Neuer, echter Cross-Tenant-Test**
  `TestQuoteGAEBImportMaterialCandidatesAreScopedToCompany`
  (`server/internal/http/quotes_integration_test.go`) — da es keinen Weg
  gibt, über die Anwendung selbst einen zweiten Mandanten anzulegen
  (Backlog 0.32), wird analog zum bereits etablierten Muster in
  `TestDocumentDownloadIsScopedToCompany` (Backlog 0.22) ein ECHTER
  zweiter `company_profiles`-Eintrag samt eigenem Nutzer per Direkt-SQL
  angelegt. Der Fremdmandant legt ein Material mit einer bestimmten
  Bezeichnung an; der eigene Mandant importiert eine GAEB-Position mit
  EXAKT derselben Beschreibung, hat aber selbst kein passendes Material —
  erwartet und verifiziert: `0` Materialkandidaten (NICHT das
  Fremdmandant-Material).

  **Nebenbei beim abschließenden vollen Suite-Lauf**: `TestQuoteFlowWithPricingAndPDF`
  kam nach der (in dieser Session bereits erfolgten) Behebung von
  Backlog 0.35 neu bis zu einer bisher unerreichten Stelle durch und
  deckte einen weiteren `classifyDomainError`-Gap auf: `"nur offene oder
  freigegebene Aufträge können bearbeitet werden"` (`sales/service.go`,
  2 Fundstellen) fiel auf `500` statt `400`. Substring `"können
  bearbeitet werden"` ergänzt (kollisionsfrei geprüft). Beim
  systematischen Nachprüfen ALLER `errors.New(...)`-Meldungen in
  `quotes/service.go`, `sales/service.go` und `accounting/ar.go` gegen
  die aktuellen `classifyDomainError`-Muster eine ganze Reihe WEITERER,
  noch offener Lücken gefunden (u. a. das fundamentale "keine
  Positionen"-Guard in ALLEN DREI kommerziellen Kern-Domänen — 7
  Fundstellen) — bewusst NICHT mehr spontan in dieser Subtask
  mitbehoben (zu großer Umfang für ein triviales Anhängen: mindestens 8
  weitere Substring-Muster über 3 Dateien, jedes einzeln auf
  Kollisionsfreiheit zu prüfen), sondern als eigenständiger neuer Fund
  dokumentiert: **Backlog 0.44**.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Beide neuen Tests
  (`TestQuoteGAEBImportApplyExposesReadOnlyMaterialCandidates`,
  `TestQuoteGAEBImportMaterialCandidatesAreScopedToCompany`) isoliert:
  PASS. Voller `go test ./...` (Nicht-Integrationspakete) ohne Fehler.
  Voller `go test ./internal/http/... -count=1` gegen frische DB: von 5
  auf **4** Fehlschläge zurückgegangen.

  **Korrektur einer eigenen Fehleinschätzung (wichtig für die
  Weiterarbeit)**: unmittelbar nach Abschluss von 0.43 wurde in einem
  ersten Entwurf dieser Sektion fälschlich behauptet, 0.43 sei der
  "letzte offene Fund der flachen Epic-0-Liste". Beim tatsächlichen
  Durchsuchen des GESAMTEN `docs/backlog.md` nach `- [ ] 0.` (nicht nur
  des chronologisch zuletzt bearbeiteten Abschnitts) zeigte sich: die
  Punkte **0.16**, **0.18** und **0.19** sind ebenfalls noch offen — sie
  wurden FRÜH in dieser Session gefunden und liegen verschachtelt unter
  Subtask 0.2.1.2.1 (NICHT Teil der später chronologisch angehängten
  "flachen Liste" 0.6ff.), wodurch sie beim strikt aufsteigenden
  Abarbeiten der später gefundenen Punkte 0.6-0.43 bisher schlicht
  übersehen wurden. Sie erklären exakt die zu diesem Zeitpunkt noch
  verbleibenden `internal/http`-Testfehlschläge
  (`TestContactTasksCreateListUpdateAndDeleteFlow`=0.16,
  `TestContactCommercialContextAggregatesQuotesSalesOrdersAndInvoices`=0.18,
  `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`=0.19).
  Diese Erkenntnis wurde noch VOR dem Turn-Abschluss korrigiert (State-
  Dokumentation entsprechend berichtigt) — als Lehre für künftige
  Sessions festgehalten: beim Prüfen auf "ist die Liste vollständig
  abgearbeitet" IMMER das gesamte Dokument nach offenen `- [ ]`-Punkten
  durchsuchen, nicht nur den zuletzt bearbeiteten Abschnitt.
- **Backlog 0.16 behoben** (`TestContactTasksCreateListUpdateAndDeleteFlow`:
  "expected due date to roundtrip") — **SYSTEMWEITER Fix, nicht nur ein
  Testfixture-Problem**. Symptom: `POST /contacts/{id}/tasks` mit
  `"faellig_am":"2026-04-10T00:00:00Z"` lieferte in der Antwort denselben
  ZEITPUNKT, aber mit einem ANDEREN JSON-Offset (z. B.
  `"2026-04-10T02:00:00+02:00"` statt `"2026-04-10T00:00:00Z"`) — der
  Test vergleicht den String byte-genau und schlug daher fehl.

  **Erste Hypothese, per Diagnose GEPRÜFT und VERWORFEN**: die
  Postgres-Test-Container-Sitzung läuft laut `docker-compose.test.yml`
  (`TZ: Europe/Berlin`) mit `TimeZone = Europe/Berlin` statt UTC — als
  naheliegende Ursache vermutet. Testweise in
  `server/internal/db/postgres.go` `cfg.ConnConfig.RuntimeParams["timezone"]
  = "UTC"` ergänzt. Per eigens geschriebenem, kurzlebigem
  Diagnose-Hilfsprogramm (`server/cmd/tzcheck/main.go`, nach Gebrauch
  wieder gelöscht — kein Teil der finalen Änderung) bestätigt: `SHOW
  timezone` lieferte danach tatsächlich `UTC` für die App-Verbindung —
  ABER `TestContactTasksCreateListUpdateAndDeleteFlow` schlug TROTZDEM
  weiterhin exakt gleich fehl. Diese Hypothese wurde damit sauber
  widerlegt, nicht einfach angenommen.

  **Tatsächliche Root Cause** (per demselben Diagnose-Programm ermittelt,
  diesmal mit einer direkten `SELECT '...'::timestamptz`-Abfrage und
  Ausgabe von `time.Time.Location()` sowie der JSON-Serialisierung):
  pgx v5 (`github.com/jackc/pgx/v5/pgtype`, `TimestamptzCodec`) dekodiert
  `timestamptz`-Spalten standardmäßig OHNE gesetzte `ScanLocation`. Der
  interne Scan-Pfad
  (`pgtype/timestamptz.go:scanPlanBinaryTimestamptzToTimestamptzScanner.Scan`)
  ruft `time.Unix(sec, nsec)` auf — laut Go-Standardbibliothek liefert
  `time.Unix` einen Wert MIT `time.Local`-Zeitzone, NICHT UTC, sofern
  nicht explizit per `plan.location` überschrieben. `time.Local` ist die
  Zeitzone des GO-PROZESSES (Betriebssystem-/Umgebungskonfiguration,
  hier `Europe/Berlin` auf der Entwicklungsmaschine) — UNABHÄNGIG von der
  Postgres-SITZUNGS-Zeitzone, weshalb der erste Fix-Versuch
  (SQL-seitiges `SET timezone`) wirkungslos bleiben musste: er betrifft
  eine völlig andere Schicht des Problems. Da produktive/CI-Umgebungen
  potenziell ebenfalls nicht auf UTC als Systemzeitzone laufen, betraf
  dieser Bug NICHT nur `contact_tasks.faellig_am`, sondern JEDES
  `timestamptz`-Feld der GESAMTEN Anwendung (Rechnungsdaten,
  Angebotsdaten, Auftragsdaten, Zahlungsdaten, Audit-Log-Zeitstempel
  usw.) — überall dort, wo ein aus der DB gelesener Zeitstempel per JSON
  an einen API-Client zurückgegeben wird.

  **Fix**: in `server/internal/db/postgres.go` (`ConnectPostgres`) — der
  EINEN zentralen Stelle im gesamten Repo, an der der `pgxpool` aufgebaut
  wird, verifiziert per `grep -rln "pgxpool.New"`, genutzt sowohl vom
  Produktiv-Entry-Point als auch von `testutil.SetupIntegrationEnv` (Zeile
  53 dort ruft exakt dieselbe Funktion auf) — ein
  `cfg.AfterConnect`-Hook ergänzt:
  ```go
  cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
      conn.TypeMap().RegisterType(&pgtype.Type{
          Name:  "timestamptz",
          OID:   pgtype.TimestamptzOID,
          Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
      })
      return nil
  }
  ```
  Dieser Hook läuft für JEDE neue physische Verbindung im Pool und
  überschreibt die Standard-Typregistrierung für `timestamptz` so, dass
  `ScanLocation` explizit auf `time.UTC` gesetzt ist — unabhängig von
  Prozess- ODER Postgres-Sitzungszeitzone. Die zuvor testweise gesetzte,
  als Ursache widerlegte `RuntimeParams["timezone"]`-Zeile wieder
  entfernt (keine Wirkung auf das eigentliche Problem, hätte nur
  unnötige Verwirrung im Code hinterlassen).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Zieltest isoliert: PASS (Log
  bestätigt vollständigen Flow: Create/List/Update/Delete, inkl. korrekt
  roundtrippendem `faellig_am`). Voller `go test ./...` (alle 15
  Nicht-Integrationspakete) ohne Fehler — insbesondere die Domänen mit
  besonders vielen `timestamptz`-Feldern (`accounting`, `hr`, `contacts`)
  liefen unauffällig durch, was das Fehlen negativer Nebenwirkungen
  dieser systemweiten Änderung stützt. Voller `go test
  ./internal/http/... -count=1` gegen frische DB: von 4 auf **3**
  Fehlschläge zurückgegangen.

  **Lehre für künftige ähnliche Funde**: eine naheliegende erste
  Hypothese (hier: Server-Zeitzonenkonfiguration) sollte durch ein
  eigenständiges, gezieltes Diagnose-Programm VERIFIZIERT werden, bevor
  der zugehörige Fix als erledigt gilt — hier hätte das blinde Vertrauen
  auf die erste, plausibel klingende Hypothese zu einer wirkungslosen
  Änderung geführt, während die eigentliche Ursache (Go-Prozess-lokale
  Zeitzone in pgx' Default-Dekodierung) unentdeckt geblieben wäre.
- **Backlog 0.18 behoben** (`TestContactCommercialContextAggregatesQuotesSalesOrdersAndInvoices`:
  `quote_items_tax_code_fkey`-Verletzung beim Anlegen eines Angebots).
  Root Cause: `server/internal/http/contacts_integration_test.go` legt
  eine Angebotsposition mit `"tax_code":"19"` an — ein reiner Tippfehler,
  gültige Steuerkennzeichen laut Stammdaten
  (`server/internal/migrate/migrations/017_accounting_basics.sql`) sind
  `"DE19"`, `"DE7"`, `"DE0"` (mit Länderpräfix). Kein `tax_codes`-Eintrag
  mit `code='19'` existiert. Isoliert gegen frische DB reproduziert: vor
  Backlog 0.36 (Steuerkennzeichen-Validierung in `quotes`) führte das zu
  einer rohen Fremdschlüssel-Verletzung
  (`quote_items_tax_code_fkey`, SQLSTATE 23503, `500`); NACH 0.36 (in
  dieser Session bereits behoben) liefert derselbe Tippfehler jetzt
  bereits sauber `400 validation_error "unbekanntes oder inaktives
  Steuerkennzeichen: 19"` statt der rohen SQL-Exception — der Test selbst
  bestand aber unverändert auf dem Erfolgsfall (`201`), weshalb er trotz
  der saubereren Fehlermeldung weiterhin fehlschlug.

  **Fix**: `"tax_code":"19"` auf `"tax_code":"DE19"` korrigiert — einzige
  Fundstelle dieses spezifischen Tippfehlers, per `grep -rn
  '"tax_code":"19"'` über das gesamte `internal/http`-Paket bestätigt
  (keine weiteren betroffenen Tests).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Zieltest isoliert: PASS (Log
  bestätigt den kompletten kommerziellen Beleg-Fluss: Angebot anlegen →
  annehmen → in Auftrag überführen → in Rechnung überführen →
  `GET .../commercial-context` aggregiert alle vier Belege korrekt).
  Voller `go test ./...` (alle 15 Nicht-Integrationspakete) ohne Fehler.
  Voller `go test ./internal/http/... -count=1` gegen frische DB: von 3
  auf **2** Fehlschläge zurückgegangen.
- **Backlog 0.19 behoben — ursprüngliche Diagnose war STALE, tatsächlich
  gefundener Fund war ein anderer, echter Bug.** Der ursprüngliche Fund
  (aus einer früheren Subtask dieser Session) behauptete, dass
  `TestProjectQuotePDFFlow` und
  `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
  bei gemeinsamem Testlauf kollidieren, weil beide einen Kontakt mit
  identischem Name+E-Mail-Fixture anlegen ("Kontakt mit gleichem Namen
  und gleicher E-Mail bereits vorhanden").

  **Re-Verifikation**: beim direkten Lesen beider Testfunktionen
  (`server/internal/http/projects_integration_test.go`) zeigte sich, dass
  sie bereits UNTERSCHIEDLICHE Kontakt-Fixtures verwenden
  (`TestProjectQuotePDFFlow`: `"Projektkunde Metallbau GmbH"` /
  `"projektkunde@example.com"`; `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`:
  `"Projekt Kontext Kunde GmbH"` / `"project-context@example.com"`) — der
  gemeinsame Testlauf beider Funktionen gegen eine frische DB bestätigte:
  KEINE Namenskollision mehr reproduzierbar. Vermutlich wurde dies in
  einer früheren, nicht separat als eigener Fix dokumentierten Subtask
  dieser Session bereits behoben (ähnliches Muster wie bei der
  Bestandsaufnahme zu Backlog 0.14 zu Sessionbeginn — ein als "offen"
  geführter Fund, der durch spätere, andere Arbeit bereits nebenbei
  gelöst wurde, ohne dass der Backlog-Eintrag selbst aktualisiert wurde).

  **Tatsächlich gefundener, unabhängiger Bug**: beim erneuten gemeinsamen
  Testlauf schlug `TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices`
  TROTZDEM fehl — mit einem völlig anderen, echten SQL-Fehler:
  `GET /projects/{id}/commercial-context` lieferte `500 ERROR: for SELECT
  DISTINCT, ORDER BY expressions must appear in select list` (SQLSTATE
  42P10). Isoliert reproduziert (auch ohne den zweiten Test, unabhängig
  von jeder Testkollision) — ein rein syntaktischer SQL-Fehler, der IMMER
  auftritt, unabhängig von Datenmenge oder -inhalt. Root Cause:
  `listProjectInvoices` (`server/internal/http/commercial_context.go`)
  verwendet
  ```sql
  SELECT DISTINCT i.id, ..., i.gross_amount, i.paid_amount
  FROM invoices_out i
  LEFT JOIN contacts c ON c.id = i.contact_id
  LEFT JOIN quotes q ON q.id = i.source_quote_id
  LEFT JOIN sales_orders so ON so.id = i.source_sales_order_id
  WHERE q.project_id = $1 OR so.project_id = $1
  ORDER BY i.invoice_date DESC, i.created_at DESC
  ```
  `i.created_at` wird im `ORDER BY` referenziert, fehlte aber in der
  `SELECT DISTINCT`-Liste — Postgres verlangt bei `SELECT DISTINCT`
  zwingend, dass JEDE `ORDER BY`-Spalte auch in der Select-Liste
  erscheint (anders als bei einem einfachen `SELECT`). Das `DISTINCT`
  wurde hier gebraucht, weil der `JOIN` über `quotes`/`sales_orders` mit
  `OR`-Bedingung potenziell Duplikate erzeugen kann.

  Zum Vergleich geprüft: `accounting.ARService.List` (`ar.go`) nutzt
  exakt denselben `ORDER BY i.invoice_date DESC, i.created_at DESC`-
  Ausdruck, aber OHNE `DISTINCT` — dort daher unproblematisch (kein Bug).
  Nur diese eine, projektbezogene Variante mit `DISTINCT` war betroffen.
  Der analoge Pfad für Kontakte (`buildContactCommercialContext`) nutzt
  stattdessen direkt `arSvc.List(..., ContactID: ...)` (kein eigenes SQL,
  kein `DISTINCT`) und war nie betroffen — erklärt auch, warum Backlog
  0.18 (Kontakt-Variante) diesen Fehler nie zeigte.

  **Fix**: `i.created_at` zur `SELECT DISTINCT`-Liste in
  `listProjectInvoices` ergänzt; korrespondierendes `Scan`-Ziel
  (`var createdAt time.Time`, nicht im Response-Struct
  `accounting.InvoiceListItem` benötigt, daher nur als reines Scan-Ziel
  ohne Weiterverwendung) ergänzt, `time`-Import hinzugefügt.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Beide ursprünglich genannten
  Tests gemeinsam (`-run
  "^TestProjectQuotePDFFlow$|^TestProjectCommercialContextAggregatesQuotesSalesOrdersAndInvoices$"`):
  beide PASS, keine Kollision, kein SQL-Fehler mehr (Log bestätigt `200`
  für `GET .../commercial-context` mit vollständig aggregierten
  Angeboten/Aufträgen/Rechnungen). Voller `go test ./...` (alle 15
  Pakete) ohne Fehler. Voller `go test ./internal/http/... -count=1`
  gegen frische DB: von 2 auf **1** Fehlschlag zurückgegangen — nur noch
  `TestQuoteFlowWithPricingAndPDF`, bereits vollständig als Backlog 0.44
  dokumentiert.

  **Bestätigung der GESAMTEN Epic-0-Fundliste**: per erneutem `grep -n
  "\- \[ \] "` über das komplette `docs/backlog.md` bestätigt, dass
  innerhalb von Epic 0 nur noch **0.32** (bewusst zurückgestelltes
  Mandanten-Onboarding-Feature) und **0.44** (classifyDomainError-Lücken)
  offen sind — alle Treffer danach gehören zu den völlig separaten,
  nie in dieser Sweep-Serie bearbeiteten Feature-Epics A-I.
- **Backlog 0.44 behoben — 🎉 LETZTER offener technischer Fund der
  flachen Epic-0-Liste, `go test ./internal/http/... -count=1` läuft ab
  jetzt mit 0 Fehlschlägen gegen eine frische DB durch.** Root Cause:
  `TestQuoteFlowWithPricingAndPDF` durchlief nach der Behebung von
  Backlog 0.34/0.35 (beide Male in dieser Session) erstmals einen
  deutlich längeren Codepfad und stieß dabei nacheinander auf zehn
  weitere, bisher unentdeckte `classifyDomainError`-Lücken — bereits bei
  der Vollverifikation von Backlog 0.43 systematisch durch Abgleich ALLER
  `errors.New(...)`-Meldungen in `quotes/service.go`, `sales/service.go`
  und `accounting/ar.go` gegen die bestehenden Substring-Muster
  identifiziert (per `grep -oE 'errors\.New\("[^"]*"\)'` extrahiert,
  manuell klassifiziert) und als eigener Fund dokumentiert, um die
  0.43-Subtask nicht unkontrolliert auszuweiten.

  **Vor dem Fix zusätzlich zwei vermutete Kandidaten geprüft und
  AUSGESCHLOSSEN** (kein Fix nötig, da sie unabhängig von
  `classifyDomainError` laufen): `"Zielmarge ist ungueltig"`
  (`server/internal/settings/quote_calculation.go`) wird vom Handler
  `PUT /settings/quote-calculation` (`v1.go`) über ein fest verdrahtetes
  `writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)`
  ausgegeben — bereits immer `400`, unabhängig vom Nachrichtentext. Die
  drei `"ungueltig..."`-Meldungen aus `auth/repository.go`/
  `auth/service.go` (`ErrInvalidCredentials`, `ErrInvalidToken` etc.)
  laufen über einen dedizierten `switch err`-Vergleich auf konkrete
  Fehler-WERTE im Login-Handler (`v1.go`, `POST /auth/login`), zugeordnet
  zu `401`/`429` — ebenfalls unabhängig von `classifyDomainError`. Beide
  Prüfungen bewusst dokumentiert, um zu zeigen, dass die
  Vollständigkeits-Analyse nicht blind jeden oberflächlich ähnlichen
  Treffer übernommen hat.

  **Fix**: `classifyDomainError` (`server/internal/http/v1.go`) um ALLE
  zehn in der Untersuchung identifizierten Substrings in einem Zug
  ergänzt: `"vollständig fakturiert"` (2 Fundstellen in `sales`: "Auftrag
  ist bereits..." / "Auftragsposition ist bereits..."), `"muss größer
  als 0 sein"` (2 Fundstellen: "Menge..." / "Teilfaktura-Menge..."),
  `"überschreitet die offene restmenge"`, `"können nicht erneut
  umgestellt werden"`, **`"keine positionen"`** (7 Fundstellen quer über
  `accounting/ar.go` `createTx`, `quotes/service.go` 3×,
  `sales/service.go` 3× — das fundamentale "mindestens eine Position
  erforderlich"-Guard beim Anlegen JEDES Angebots/Auftrags/jeder Rechnung
  in der gesamten Anwendung, vermutlich der schwerwiegendste Einzelfund
  dieser Untersuchung), `"dürfen nicht"` (Pluralform, ergänzend zum
  bereits vorhandenen `"darf nicht"`), `"hat keinen kunden"`, `"keine
  priorisierte preisquelle"`, `"ungueltiger freigabeentscheid"` (ASCII-
  Transliterationsvariante von `"ungültig"`). Jeder Substring vorab per
  `grep` auf Kollisionsfreiheit mit anderen, absichtlich `500` bleibenden
  Meldungen geprüft (keine gefunden) — dasselbe bewährte Vorgehen wie bei
  jeder vorherigen `classifyDomainError`-Erweiterung dieser Session
  (0.24/0.29/0.33/0.34/0.36/0.40/0.43).

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. `TestQuoteFlowWithPricingAndPDF`
  isoliert: PASS, läuft jetzt vollständig durch (jeder zuvor
  dokumentierte Zwischenschritt liefert jetzt korrekt `400 validation_error`
  statt `500 internal_error`) — entgegen der ursprünglichen Erwartung
  ("mit hoher Wahrscheinlichkeit kommt der Test danach noch weiter und
  deckt zusätzliche Lücken auf") war KEINE weitere Iteration nötig: die
  systematische Vollanalyse aller `errors.New(...)`-Aufrufe hatte bereits
  ALLE tatsächlich betroffenen Stellen erfasst, bevor der erste Fix-Versuch
  überhaupt lief. Voller `go test ./...` (alle 15 Nicht-Integrationspakete)
  ohne Fehler. Voller `go test ./internal/http/... -count=1` gegen frische
  DB: von 1 auf **0** Fehlschläge zurückgegangen.

  **Meilenstein**: die GESAMTE flache Fundliste aus Epic 0 dieser Session
  — 0.6 bis 0.44 in der später chronologisch angehängten Liste, PLUS die
  früh in der Session gefundenen, zunächst beim Abarbeiten übersehenen
  0.16/0.18/0.19 — ist damit VOLLSTÄNDIG abgearbeitet. Der komplette
  `go test ./internal/http/... -count=1`-Lauf gegen eine garantiert
  frische, leere Postgres-Instanz liefert 0 Fehlschläge. Einzig
  verbleibend offen innerhalb von Epic 0: das bewusst zurückgestellte,
  größere Feature **0.32** (Mandanten-Onboarding), das eine
  Produktentscheidung des Nutzers voraussetzt und deshalb nicht
  eigenständig als Qualitäts-Fix umgesetzt wurde. Alle weiteren offenen
  Punkte in `docs/backlog.md` gehören zu den separaten Feature-Epics A-I
  (Stammdaten, Angebots-/Auftragswesen, Waren-/Lagerwirtschaft,
  Bestellwesen, Controlling/DATEV, Personal, Fuhrpark,
  Produktionssteuerung, KI-gestützte GAEB-Angebotserzeugung) — eigenständige
  künftige Produktarbeit, kein Teil dieser Qualitäts-/Compliance-Sweep.
- **Backlog 0.32 (Mandanten-Onboarding) begonnen — Subtask 0.32.1
  abgeschlossen und verifiziert.** Nutzer bestätigte explizit, das
  Feature jetzt umzusetzen statt es (wie zunächst dokumentiert) auf
  später zurückzustellen, mit der Begründung, spätere Nacharbeiten in
  anderen Epics zu vermeiden. Offene Produktfrage (wer darf einen neuen
  Mandanten anlegen?) per `AskUserQuestion` geklärt: ein neuer
  HTTP-Endpoint, gesperrt über die bestehende `admin.superuser`-
  Berechtigung (aus Backlog 0.5.3, "systemweiter Vollzugriff") —
  ausdrücklich NICHT ein separates CLI-/Ops-Tool ohne HTTP-Oberfläche.

  **Subtask 0.32.1 (notwendige Voraussetzung, VOR dem eigentlichen
  Onboarding-Endpunkt)**: beim Planen des Onboarding-Endpunkts
  festgestellt, dass `CompanyService`
  (`server/internal/settings/company.go`, zuständig für
  `company_profiles`/`company_branches`) AUSNAHMSLOS jede Methode
  (`Get`, `Upsert`, `ListBranches`, `CreateBranch`, `UpdateBranch`,
  `DeleteBranch`, `getBranch`, `ensureBranchCodeUnique`) fest auf
  `id='default'` bzw. `company_id='default'` verdrahtet hatte — ein
  latenter Cross-Tenant-Bug, der zwingend VOR dem Onboarding-Endpunkt
  behoben werden musste: sobald ein zweiter Mandant existiert, hätte
  dessen Admin beim Aufruf von `PUT /settings/company/` oder
  `POST/PATCH/DELETE /settings/company/branches/...` versehentlich die
  Firmendaten/Niederlassungen des ERSTEN (`default`) Mandanten gelesen
  bzw. überschrieben/gelöscht — der neue Mandant hätte NIE seine eigenen
  Stammdaten pflegen können, und `getBranch`/`DeleteBranch` hatten vorher
  sogar GAR KEINEN Ownership-Check (jede Branch-ID war global adressierbar,
  unabhängig vom aufrufenden Mandanten) — vergleichbare Schwere wie das
  bereits behobene Backlog 0.22.

  **Fix**: alle 8 Methoden in `company.go` um einen `companyID
  string`-Parameter erweitert; jede SQL-Query entsprechend auf
  `company_id=$N` eingeschränkt (`Get`, `ListBranches`, `getBranch`,
  `ensureBranchCodeUnique` als zusätzlicher `WHERE`-Filter;
  `Upsert`/`CreateBranch` verwenden `companyID` jetzt als Parameter statt
  als SQL-Literal `'default'` im `INSERT`; `UpdateBranch`/`DeleteBranch`
  scopen ihr `UPDATE`/`DELETE` zusätzlich auf `company_id=$N`, sodass ein
  Zugriff auf eine fremde Branch-ID ins Leere läuft statt versehentlich
  eine fremde Zeile zu treffen). Die 6 betroffenen HTTP-Handler in `v1.go`
  (`GET/PUT /settings/company/`, `GET/POST /settings/company/branches`,
  `PATCH/DELETE /settings/company/branches/{branchID}`) wurden angepasst,
  um `companyIDFromContext(req.Context())` zu ermitteln und
  durchzureichen — exakt dasselbe etablierte Muster wie bei jeder anderen
  Domäne seit Backlog 0.2.

  **Bewusst NICHT angefasst**: `settings/branding.go`,
  `settings/localization.go`, `settings/quote_calculation.go` haben
  denselben `id='default'`-Literal-Stil, wurden aber per Schema-Prüfung
  (`grep` über die zugehörigen `CREATE TABLE`-Migrationen
  `028_company_branding.sql`, `027_company_localization.sql`,
  `049_quote_calculation_settings.sql`) als echte GLOBALE
  Singleton-Einstellungen bestätigt — ihre Tabellen haben schlicht KEINE
  `company_id`-Spalte, `id='default'` ist dort nur ein fixer
  Zeilen-Bezeichner wie bei einer klassischen Konfigurationstabelle mit
  genau einer Zeile, kein Scoping-Bug.

  **Neuer Test** `TestCompanyProfileAndBranchesAreScopedToCompany`
  (`server/internal/http/settings_integration_test.go`) — da es zu diesem
  Zeitpunkt noch keinen Self-Service-Weg gibt, einen zweiten Mandanten
  anzulegen (das wird ja gerade erst gebaut), wird analog zum etablierten
  Muster aus `TestDocumentDownloadIsScopedToCompany` (Backlog 0.22) und
  `TestQuoteGAEBImportMaterialCandidatesAreScopedToCompany` (Backlog 0.43)
  ein echter zweiter `company_profiles`-Eintrag samt Nutzer per
  Direkt-SQL angelegt. Verifiziert: beide Mandanten setzen ihren eigenen
  Firmennamen ("Firma A"/"Firma B") und ihre eigene Niederlassung
  ("A-HQ"/"B-HQ") unabhängig voneinander, ohne dass einer die Daten des
  anderen überschreibt oder sieht; ein Löschversuch der EIGENEN Branch-ID
  durch den ANDEREN Mandanten liefert `404 "Niederlassung nicht
  gefunden"`, die Branch bleibt danach nachweislich unangetastet
  bestehen.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Neuer Test isoliert: PASS
  (alle Teilschritte inkl. Cross-Tenant-Löschversuch grün). Voller `go
  test ./...` (alle 15 Nicht-Integrationspakete) ohne Fehler. Voller `go
  test ./internal/http/... -count=1` gegen frische DB: weiterhin **0**
  Fehlschläge — der bereits erreichte vollständig grüne Zustand blieb
  durch diesen Refactor unangetastet, kein Regress.

  **Vorab-Analyse für Subtask 0.32.2** (der eigentliche
  Onboarding-Endpunkt, noch offen): per Schema-Prüfung ermittelt, was für
  einen funktionsfähigen neuen Mandanten tatsächlich geseedet werden
  muss. `tax_codes` (Steuerkennzeichen-Stammdaten) haben KEINE
  `company_id`-Spalte — global, kein Seeding nötig. `number_sequences`
  initialisiert sich selbst beim ersten `NumberingService.Next(...)`-
  Aufruf für eine neue `(company_id, entity)`-Kombination
  (`settings/numbering.go`) — ebenfalls kein Seeding nötig.
  `roles`/`permissions`/`role_permissions` sind laut Schema
  (`017_auth.sql`) global (keine `company_id`-Spalte) — die bestehende
  `admin`-Rolle kann dem ersten Nutzer des neuen Mandanten direkt
  zugewiesen werden, ohne eigenes Rollen-Seeding. Einzig `accounts`
  (Kontenrahmen) ist seit Migration 058 `company_id`-gescoped und wird
  bisher NUR per Migration/Seed-SQL für `'default'` befüllt — das war der
  ursprüngliche, engere 0.32-Fund und bleibt der einzige echte
  Seeding-Bedarf für Subtask 0.32.2.
- **Subtask 0.32.2a behoben** (kritischer Blocker für "Kontenrahmen
  kopieren": `accounts.code` war GLOBALER Primärschlüssel statt
  `(company_id, code)`). Beim Entwurf des eigentlichen
  Onboarding-Endpunkts (Subtask 0.32.2) festgestellt: Migration 058 hatte
  zwar eine `company_id`-Spalte auf `accounts` ergänzt, den
  Primärschlüssel selbst aber nie angepasst — `code text PRIMARY KEY`
  allein, global eindeutig über ALLE Mandanten hinweg. Damit war das
  eigentliche Ziel von Backlog 0.32 ("Kontenrahmen-Vorlage kopieren")
  strukturell UNERREICHBAR: ein zweiter Mandant hätte niemals dieselben
  Standard-Kontonummern (`1000` Kasse, `1200` Bank, `1400` Forderungen,
  `8000`/`8100` Umsatzerlöse, ...) wie `default` bekommen können — der
  allererste Kopierversuch für den allerersten zusätzlichen Mandanten
  wäre sofort an einer PK-Kollision gescheitert. Zusätzlich
  referenzierten `journal_lines.account_code` und
  `invoice_out_items.account_code` diesen globalen PK direkt als
  Fremdschlüssel (`journal_lines_account_code_fkey`,
  `invoice_out_items_account_code_fkey`, jeweils `FOREIGN KEY
  (account_code) REFERENCES accounts(code)`).

  **Dem Nutzer explizit vorgelegt** (`AskUserQuestion`, da eine echte
  Architekturentscheidung mit Migrationsrisiko, nicht einfach
  eigenmächtig entschieden): Option "Schema richtig fixen" (PK-Umbau,
  mehrere Migrationsschritte, Backfill per JOIN) vs. Option "Onboarding
  ohne Kontenrahmen-Kopie" (kleinerer, risikoärmerer Schnitt, aber
  Buchhaltung für neue Mandanten bliebe vorerst manuell nachzupflegen).
  Nutzerentscheidung: **Schema richtig fixen**.

  **Fix (neue Migration `server/internal/migrate/migrations/067_accounts_company_scoped_pk.sql`)**:
  1. `journal_lines` um eigene `company_id`-Spalte ergänzt (vorher nur
     indirekt über `journal_entries.company_id` bekannt), per `UPDATE
     ... FROM journal_entries je WHERE je.id = jl.entry_id` deterministisch
     zurückwirkend befüllt, danach `NOT NULL` erzwungen, FK auf
     `company_profiles(id)` ergänzt, Index ergänzt.
  2. Dieselbe Behandlung für `invoice_out_items` (Backfill über
     `invoices_out.company_id` via `invoice_id`-Join).
  3. `accounts.company_id` `NOT NULL` erzwungen (seit Migration 058
     bereits für ALLE Zeilen auf `'default'` befüllt, daher gefahrlos —
     Voraussetzung dafür, dass die Spalte Teil eines Primärschlüssels
     sein kann, Postgres verlangt NOT NULL für PK-Spalten).
  4. Alte Einzelspalten-Fremdschlüssel (`journal_lines_account_code_fkey`,
     `invoice_out_items_account_code_fkey`, `accounts_parent_code_fkey`)
     VOR dem PK-Umbau entfernt — Postgres verbietet das Droppen einer PK,
     solange abhängige FKs bestehen.
  5. Alten `accounts_pkey` (nur `code`) gedroppt, neuen zusammengesetzten
     `accounts_pkey` `(company_id, code)` angelegt.
  6. Alle drei Fremdschlüssel als zusammengesetzte FKs auf `(company_id,
     code)` neu angelegt — inklusive der Selbstreferenz
     `accounts.parent_code` (aktuell in JEDER Zeile `NULL`, da kein
     Anwendungscode sie je setzt — bestätigt per `grep`, daher gefahrlos
     auf eine zusammengesetzte Form umstellbar; NULL in einer
     FK-Spalte gilt bei Postgres' Standard-`MATCH SIMPLE` automatisch als
     erfüllt).

  **Zwei direkt notwendige Go-Anpassungen**: `accounting/ar.go`
  (`createTx`, INSERT in `invoice_out_items`) und
  `accounting/journal.go` (`create`, INSERT in `journal_lines`) ergänzen
  jetzt `company_id` beim Insert — beide Funktionen hatten `companyID`
  bereits als Parameter im Scope, minimal-invasive Ergänzung um eine
  Spalte/einen Wert.

  **Cascading-Sicherheitsfund beim Verifizieren, sofort mitbehoben**:
  `loadTaxCodes` (`accounting/ar.go`) jointe `accounts` bisher OHNE
  `company_id`-Filter: `LEFT JOIN accounts a ON a.tax_code = tc.code AND
  a.type = 'liability' AND a.is_active`. Solange es nur EINEN Mandanten
  mit global eindeutigen Konten gab, war das harmlos — sobald aber (durch
  genau diesen PK-Fix) ein zweiter Mandant ein EIGENES Konto mit
  demselben `tax_code`/`type='liability'` haben kann, hätte diese
  Funktion mandantenübergreifend das FALSCHE
  Umsatzsteuer-Verbindlichkeitskonto zurückliefern oder unvorhersehbare
  Ergebnisse liefern können (kein `DISTINCT`, mehrdeutiger Match möglich)
  — exakt dasselbe Fehlermuster wie das bereits behobene Backlog 0.43
  (fehlendes `company_id`-Scoping bei einem `JOIN`), hier nur bisher
  UNSICHTBAR, weil es vorher gar keine zwei Mandanten mit eigenen Konten
  geben konnte. `loadTaxCodes` um einen `companyID string`-Parameter
  erweitert, `JOIN` um `AND a.company_id = $1` ergänzt, alle 3
  Aufrufstellen (`createTx`, `Book`, `Storno` — `companyID` war überall
  bereits im Scope) angepasst. Per `grep -rn "FROM accounts\|JOIN
  accounts\|INSERT INTO accounts\|UPDATE accounts"` über das gesamte
  `internal/`-Verzeichnis bestätigt: dies war die letzte verbleibende
  ungescopte `accounts`-Abfrage im gesamten Code (`settings/accounting.go`
  `ListAccounts` war bereits korrekt gescoped seit dem ursprünglichen
  0.32-Fund; `quotes`/`sales` lesen ausschließlich aus dem globalen
  `tax_codes`, nie direkt aus `accounts`).

  **End-to-End-Beweis** (nicht nur Schema-Inspektion): per Direkt-SQL
  einen zweiten `company_profiles`-Eintrag angelegt und beiden Mandanten
  je ein Konto mit identischem `code='1400'` gegeben —
  `SELECT company_id, code, name FROM accounts WHERE code='1400'`
  liefert zwei Zeilen (`default`/"Forderungen aus Lieferungen und
  Leistungen" und `itest-second-tenant`/"Forderungen (Mandant 2)"), kein
  Konflikt, keine Fehlermeldung — der ursprüngliche Blocker ist damit
  nachweislich beseitigt.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Migration `067` wendet sich
  sauber an, auch unter `go test ./... -p 8` (mehrere Pakete parallel,
  der Advisory-Lock-Mechanismus aus Backlog 0.13 serialisiert weiterhin
  korrekt). Voller `go test ./...` (alle 15 Nicht-Integrationspakete)
  ohne Fehler. Voller `go test ./internal/http/... -count=1` gegen
  frische DB: weiterhin **0** Fehlschläge — der PK-Umbau auf einer von
  mehreren Kern-Domänen (Buchhaltung, Rechnungen) gemeinsam genutzten
  Tabelle verursachte keinerlei Regression.
- **Subtask 0.32.2b behoben — 🎉 Backlog 0.32 (Mandanten-Onboarding)
  DAMIT VOLLSTÄNDIG UMGESETZT**, vom ursprünglich bewusst
  zurückgestellten Fund bis zum funktionierenden, end-to-end
  verifizierten Feature. Mit dem in Subtask 0.32.2a beseitigten
  Architekturblocker (`accounts`-PK) war der Weg frei für den
  eigentlichen Onboarding-Endpunkt.

  **Neue Route** `POST /api/v1/platform/tenants/`, registriert direkt
  nach der bestehenden `/users`-Routengruppe in `v1.go`, gesperrt über
  `requirePermission(adminSuperuserPermission)` — bewusst NICHT über
  eine tenant-scoped Permission wie `settings.manage`: das Anlegen eines
  NEUEN Mandanten ist grundsätzlich eine Plattform-Ebenen-Operation, ein
  regulärer Admin des EIGENEN Mandanten darf keine Geschwister-Mandanten
  erzeugen können (matcht die vom Nutzer in einer früheren
  `AskUserQuestion` getroffene Entscheidung: HTTP-Endpoint mit
  `admin.superuser`, kein separates CLI-Tool).

  **Neue Datei** `server/internal/http/tenant_onboarding.go` mit
  `createTenant(ctx, pg *pgxpool.Pool, in tenantCreateInput)
  (*tenantCreateResult, error)`. Läuft komplett innerhalb EINER
  gemeinsamen `pgx.Tx` (mit `defer tx.Rollback(ctx)` als
  Sicherheitsnetz, `tx.Commit(ctx)` erst ganz am Ende) — bewusst NICHT
  über den bestehenden, Pool-gebundenen `auth.Service.CreateUser(...)`
  für den Erst-Benutzer, da dieser keine externe Transaktion
  entgegennimmt; stattdessen direkte SQL-Inserts mit derselben
  Spaltenliste wie `auth.Repository.CreateUser` (Konsistenz zum
  bestehenden Muster), aber innerhalb der gemeinsamen Transaktion, um
  echte Atomarität zu garantieren. Grund: ohne diese Atomarität hätte
  ein Fehler beim Erst-Benutzer-Anlegen (z. B. E-Mail-Kollision, erst
  beim vierten von vier Schritten erkennbar) einen "Mandant ohne jeden
  Admin-Zugang"-Zombie-Zustand hinterlassen können, der über die
  bestehende API nie mehr behebbar gewesen wäre (da `POST /users`
  ausschließlich Nutzer für die EIGENE Company des Aufrufers anlegen
  kann, siehe Subtask-0.32-Vorüberlegung).

  Ablauf innerhalb der Transaktion:
  1. `companyID` bestimmen: entweder explizit aus dem Request-Feld `id`,
     oder per neuer Hilfsfunktion `slugifyTenantID(name)`
     (Kleinbuchstaben, alles außer `[a-z0-9]` durch `-` ersetzt, führende/
     nachgestellte `-` entfernt) aus dem Firmennamen abgeleitet.
  2. Uniqueness-Prüfungen INNERHALB derselben Transaktion (nicht per
     separatem Vorab-Query, um eine TOCTOU-Lücke zwischen Prüfung und
     Insert zu vermeiden): `SELECT EXISTS(...FROM company_profiles WHERE
     id=$1)` und `SELECT EXISTS(...FROM users WHERE LOWER(email)=$1)`,
     bei Treffer `errors.New("Mandant mit dieser ID existiert bereits")`
     bzw. `errors.New("Benutzer mit dieser E-Mail-Adresse bereits
     vorhanden")` (letztere Meldung ist bewusst identisch zur bereits
     bestehenden Meldung aus `auth.Service.CreateUser`, matcht daher
     automatisch das bereits vorhandene `"bereits vorhanden"`-Muster in
     `classifyDomainError`, ohne dass dafür ein Sonderfall nötig wäre).
  3. `INSERT INTO company_profiles (id, name, updated_at) VALUES ($1,
     $2, now())` — bewusst minimal, alle anderen Spalten haben
     Datenbank-seitige Defaults (leerer String); der neue Mandant kann
     sein vollständiges Profil danach jederzeit selbst per `PUT
     /settings/company/` vervollständigen (Subtask 0.32.1 hat genau
     diesen Endpunkt bereits korrekt company_id-scoped gemacht).
  4. Kontenrahmen-Kopie: `INSERT INTO accounts (code, name, type,
     parent_code, tax_code, is_active, company_id) SELECT code, name,
     type, parent_code, tax_code, is_active, $1 FROM accounts WHERE
     company_id = 'default'` — eine einzige `INSERT ... SELECT`-Anweisung
     kopiert den GESAMTEN aktuellen Kontenrahmen von `'default'` für den
     neuen Mandanten. Erst durch den in Subtask 0.32.2a behobenen
     zusammengesetzten Primärschlüssel `(company_id, code)` überhaupt
     möglich, ohne mit den Original-Kontonummern zu kollidieren.
  5. Erst-Admin-Benutzer: `auth.HashPassword(in.AdminPassword)`
     (wiederverwendet, liefert bereits einen sauberen
     `"passwort erforderlich"`-Fehler bei leerem Passwort, passt ins
     bestehende `"erforderlich"`-Muster), dann direktes `INSERT INTO
     users (id, email, password_hash, first_name, last_name,
     display_name, locale, timezone, is_active, is_locked, company_id,
     created_at, updated_at) VALUES (...)` mit `uuid.NewString()` als ID
     (exakt dieselbe ID-Konvention wie `auth.Repository.CreateUser`),
     `first_name`/`last_name` leer gelassen (nur `display_name` wird
     verwendet — Default `"<Firmenname> Admin"`, falls nicht explizit im
     Request angegeben), `locale`/`timezone` mit denselben Defaults wie
     überall sonst im Code (`de-DE`/`Europe/Berlin`).
  6. Rollenzuweisung: `INSERT INTO user_roles (user_id, role_id) SELECT
     $1, r.id FROM roles r WHERE r.code = 'admin'` — `roles` ist global
     (keine `company_id`-Spalte, siehe Vorab-Analyse aus Subtask
     0.32.2), der `admin`-Rolle-Datensatz existiert garantiert bereits
     seit der initialen Auth-Migration (`017_auth.sql`), kein
     Rollen-Seeding nötig.
  7. `tx.Commit(ctx)` — erst hier wird alles sichtbar; jeder Fehler in
     einem der Schritte 1-6 löst über `defer tx.Rollback(ctx)`
     automatisch einen vollständigen Rollback aus, kein Teilzustand
     bleibt sichtbar.

  **Zwei neue `classifyDomainError`-Substrings** ergänzt (vorher per
  `grep` auf Kollisionsfreiheit mit anderen, absichtlich `500` bleibenden
  Meldungen geprüft, keine gefunden): `"existiert bereits"` (für "Mandant
  mit dieser ID existiert bereits") und `"abgeleitet werden"` (für den
  Edge-Case "Mandanten-ID konnte nicht aus dem Firmennamen abgeleitet
  werden, bitte explizit angeben" — tritt nur auf, wenn der Firmenname
  nach dem Slugifizieren komplett leer würde, z. B. bei einem Namen nur
  aus Sonderzeichen).

  **Neuer End-to-End-Test** `TestPlatformTenantsCreateOnboardsNewTenant`
  (`server/internal/http/tenant_onboarding_integration_test.go`) — deckt
  den KOMPLETTEN Onboarding-Flow im echten Produktivpfad ab, nicht nur
  isolierte Funktionsaufrufe:
  1. `403` beim Versuch, den Endpunkt mit einer Rolle OHNE
     `admin.superuser` (`procurement`) aufzurufen.
  2. `201` für die eigentliche Mandanten-Anlage (`company_id`,
     `admin_user_id`, `admin_email` im Response korrekt).
  3. Erfolgreicher LOGIN als der neu angelegte Mandanten-Admin — beweist,
     dass der neue Zugang SOFORT einsatzfähig ist, nicht nur in der DB
     existiert.
  4. `GET /settings/company/` als der neue Nutzer liefert dessen EIGENES
     Firmenprofil (nicht das von `default`) — beweist, dass Subtask
     0.32.1s Company-Scoping-Fix korrekt mit dem neuen Onboarding-Flow
     zusammenspielt.
  5. Direkte SQL-Prüfung: die Anzahl kopierter `accounts`-Zeilen für den
     neuen Mandanten entspricht EXAKT der Anzahl bei `'default'`.
  6. Direkte SQL-Prüfung: Kontonummer `1400` existiert jetzt für BEIDE
     Mandanten unabhängig, ohne Kollision — der ursprüngliche
     0.32.2a-Blocker damit im ECHTEN Produktivpfad bewiesen (nicht nur
     durch die vorherige Direkt-SQL-Diagnose).
  7. `400` bei einem zweiten Anlageversuch mit identischer Mandanten-ID.
  8. `400` bei einem zweiten Anlageversuch mit identischer Admin-E-Mail.

  Verifiziert gegen frische, per `\dt` bestätigt leere DB: `go build
  ./...`, `go vet ./...`, `gofmt -l` clean. Neuer Test isoliert: PASS,
  alle 8 Teilschritte grün. Voller `go test ./...` (alle 15
  Nicht-Integrationspakete) ohne Fehler. Voller `go test
  ./internal/http/... -count=1` gegen frische DB: weiterhin **0**
  Fehlschläge.

  **Zusammenfassung Backlog 0.32** (drei Subtasks über mehrere Turns):
  0.32.1 (CompanyService company_id-Scoping, notwendige Voraussetzung),
  0.32.2a (kritischer `accounts`-PK-Architekturblocker, per
  `AskUserQuestion` dem Nutzer vorgelegt und auf dessen Wunsch sauber
  gefixt statt umgangen), 0.32.2b (der eigentliche Onboarding-Endpunkt).
  Aus einem ursprünglich bewusst als "zu groß für einen Qualitäts-Fix"
  zurückgestellten Fund wurde auf expliziten Nutzerwunsch ein
  vollständiges, end-to-end verifiziertes Feature — inklusive zweier
  während der Umsetzung entdeckter, tieferliegender Sicherheits-/
  Architekturprobleme (`CompanyService`-Cross-Tenant-Bug,
  `accounts`-PK-Kollision, `loadTaxCodes`-Cross-Tenant-Join), die ohne
  dieses Feature vermutlich noch lange unentdeckt geblieben wären, da sie
  erst bei einem ECHTEN zweiten Mandanten sichtbar werden.

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
