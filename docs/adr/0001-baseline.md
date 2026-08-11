# ADR 0001 — Baseline: Festgestellte Architekturentscheidungen des Ist-Stands

Datum: 2026-08-11
Status: dokumentiert (nachträglich, aus Code-Recherche — keine neue Entscheidung
dieser Session, siehe `docs/00-recon.md` / `docs/01-gap-analysis.md`)

## Kontext

aufgabe.md fordert vor jeder Codeänderung eine Bestandsaufnahme inkl. ADR für
den vorgefundenen Architektur-Ist-Stand. Diese ADR hält fest, welche
Architekturentscheidungen bereits im Repo verankert sind — nicht als
Empfehlung, sondern als belegte Beschreibung dessen, was vorgefunden wurde.
Änderungen an diesen Entscheidungen erfordern laut aufgabe.md §2 ("gilt
ausschließlich der vorgefundene [Stack]") und §7.6 ("keine Löschung/Umbenennung
ohne ADR") eine eigene, neue ADR mit ausdrücklicher Freigabe.

---

## Entscheidung 1: Technologie-Stack

**Optionen**: nicht neu bewertet — der Stack war beim Repo-Zugriff bereits
festgelegt und implementiert.

**Entscheidung (vorgefunden)**: Go 1.24 (API) + Flutter Web (Client) +
PostgreSQL 17 (relationale Fachdaten) + MongoDB 7 / GridFS (Binärdateien:
Dokumente, PDF-Assets, Bilder) + Redis 7 (Auth-Sessions).

**Beleg**: `server/go.mod:3`, `client/pubspec.yaml:6`, `docker-compose.yml:40,60,74`,
`README.md:23-25,30`.

**Konsequenzen**: aufgabe.md §2 verlangt exakt diesen Stack ("falls das Repo
bereits einen Stack festlegt, gilt ausschließlich der vorgefundene") — die
Vorgabe ist erfüllt, kein Konflikt. Jede künftige Abweichung braucht eine neue
ADR und ausdrückliche Freigabe.

---

## Entscheidung 2: Modularer Monolith statt Microservices

**Kontext**: Alle Fachdomänen (`contacts`, `materials`, `quotes`, `sales`,
`accounting`, `hr`, `projects`, `purchasing`, `settings`, `auth`) liegen als
Go-Packages unter einem einzigen Binary (`server/cmd/api/main.go`), ein
gemeinsamer HTTP-Router (`server/internal/http/v1.go`), eine gemeinsame
Postgres-Instanz.

**Entscheidung (vorgefunden)**: Modularer Monolith, keine Service-Grenzen auf
Prozess-/Deployment-Ebene.

**Beleg**: `server/internal/` (17 Packages, ein `go.mod`), `docker-compose.yml`
(ein `api`-Service).

**Konsequenzen**: Einfacher Build/Deploy, konsistente Transaktionen über
Domänengrenzen hinweg (z. B. `accounting.CreateFromQuoteTx` über `pgx.Tx`).
Kehrseite: keine unabhängige Skalierung/Deployment einzelner Domänen. Für ein
Mittelstands-ERP mit überschaubarer Last angemessen — keine Änderung
empfohlen ohne konkreten Anlass.

---

## Entscheidung 3: Eigenbau-JWT + Redis-Sessions statt Standard-Library/reines JWT

**Kontext**: Auth-Tokens werden nicht über eine geprüfte JWT-Bibliothek
signiert, sondern über manuelles HMAC-SHA256 + Base64URL
(`server/internal/auth/service.go:248-273`). Jede Token-Prüfung schlägt
zusätzlich eine Session in Redis nach (`server/internal/auth/store.go`) — das
System ist damit nicht rein stateless, sondern hybrid.

**Entscheidung (vorgefunden)**: Selbstgebautes Token-Signing + serverseitige
Session als Wahrheitsquelle für Gültigkeit/Widerruf.

**Beleg**: `server/internal/auth/service.go:212-273`, `store.go`.

**Konsequenzen**: Vorteil gegenüber reinem JWT: sofortiger Logout/Widerruf
möglich (Redis-Session löschen), da nicht rein stateless. Nachteil: Eigenbau
statt Standardbibliothek erhöht Wartungs-/Prüfaufwand, kein Key-Rotation-/
`kid`-Mechanismus. Siehe `docs/00-recon.md` Risiko #4 — Bewertung, ob dies
beibehalten oder auf eine Standardbibliothek migriert wird, ist eine offene
Frage (siehe Blocker-Fragen), keine Entscheidung dieser ADR.

---

## Entscheidung 4: Rollenbasierte Rechte über granulare Permission-Codes

**Kontext**: Statt reiner Rollenprüfung (`role == "admin"`) prüft jede
geschützte Route einen granularen Permission-Code (`materials.write`,
`quotes.approve`, …), Rollen sind Bündel von Permissions.

**Entscheidung (vorgefunden)**: Permission-Code-basierte Autorisierung,
Rollen als Zuordnungsebene (`user_roles`, `role_permissions`).

**Beleg**: `server/internal/migrate/migrations/017_auth.sql:19-47`,
`server/internal/http/v1.go:3625-3642` (`requirePermission`).

**Konsequenzen**: Granular und erweiterbar (neue Permission je neue Domäne,
z. B. `quotes.approve` für den Freigabe-Workflow, `030_accounting_permissions.sql`
für `invoices_out.*`). Bekannter Webfehler: `users.manage` wirkt als impliziter
globaler Bypass (`v1.go:3634`) — siehe Risiko #5 in `docs/00-recon.md`.

---

## Entscheidung 5: Single-Tenant-Datenmodell (trotz Mehrmandanten-Zielvorgabe)

**Kontext**: Keine Fach-/Transaktionstabelle trägt `company_id`/`branch_id`/
`tenant_id`. `company_profiles` ist als Singleton (`'default'`) angelegt.

**Entscheidung (vorgefunden, nicht explizit dokumentiert)**: Das System wurde
bislang implizit als Single-Tenant-ERP gebaut — eine Instanz bedient eine
Firma (ggf. mit mehreren Standorten über `company_branches`, aber ohne
Datentrennung).

**Beleg**: `server/internal/migrate/migrations/025_company_profile.sql:19-24`,
Abwesenheit von `tenant_id`/`company_id` in `contacts`, `projects`, `quotes`,
`sales_orders`, `invoices_out`, `purchase_orders`, `users` (durchsucht, 0
Treffer).

**Konsequenzen**: **Direkter Zielkonflikt** mit aufgabe.md §2
("Mehrmandanten-/Mehrstandortfähigkeit: ja — wenn ja, von Anfang an im
Datenmodell, nicht nachgerüstet"). Da bereits produktiver Code auf dem
Single-Tenant-Schema aufbaut, ist eine Nachrüstung mit Migrationsaufwand
verbunden (Spalten-Ergänzung + Backfill + Anpassung aller Queries/Handler).
Dies ist gemäß aufgabe.md §7.9 ("bei Zielkonflikt zwischen Regeln und
Backlog-Anweisung: nachfragen, nicht selbst entscheiden") eine der
Blocker-Fragen dieser Session, keine in dieser ADR getroffene Entscheidung.

---

## Entscheidung 6: Migrationsverzeichnis via `go:embed`, kein externes Migrationstool

**Kontext**: Migrationen sind reine `.sql`-Dateien, per `go:embed` eingebettet
und beim Serverstart sequenziell (alphabetisch sortiert) ausgeführt — kein
`golang-migrate`, kein `goose`, kein Versionsstempel-Tracking in einer
Migrations-Tabelle (soweit ersichtlich).

**Entscheidung (vorgefunden)**: Eigenbau-Migrationsrunner
(`server/internal/migrate/migrate.go`).

**Beleg**: `migrate.go:14-31`, `server/internal/app/server.go:56`.

**Konsequenzen**: Einfach, keine externe Abhängigkeit. Nachteil, real
eingetreten: da rein alphabetisch sortiert wird und kein Tool vor doppelten
Präfixen warnt, existieren im aktiven Verzeichnis zwei überlappende
Nummernketten (`docs/00-recon.md` Risiko #7). Ob ein Wechsel auf ein
etabliertes Migrationstool mit Versions-Tracking sinnvoll ist, ist eine
Backlog-Entscheidung, keine Entscheidung dieser ADR.

---

## Entscheidung 7: Kein Frontend-State-Management-Framework

**Kontext**: Der Flutter-Client verwendet ausschließlich `StatefulWidget` +
`setState` sowie einen einzelnen globalen `ValueNotifier` für den
Auth-Status — kein Provider/Riverpod/Bloc.

**Entscheidung (vorgefunden)**: Kein State-Management-Package,
Datenfluss über Konstruktor-Parameter/Callbacks und zentrale
`ApiClient`-Instanz.

**Beleg**: `client/pubspec.yaml:9-22` (keine solche Dependency),
`client/lib/api.dart:39,42` (`ValueNotifier` für Auth-Status).

**Konsequenzen**: Für den aktuellen Umfang (16 Seiten, klar getrennte
Domänen-Screens) handhabbar. Bei weiterem Wachstum (insbesondere Domäne I mit
plausibel komplexerem, geteiltem UI-State für Review-Queues) ggf. an Grenzen
stoßend — keine Entscheidung dieser ADR, nur Beobachtung für spätere
Architektur-Diskussion.

---

## Nicht in dieser ADR entschieden

Diese ADR beschreibt ausschließlich vorgefundene Entscheidungen. Sie trifft
**keine** neuen Architekturentscheidungen für Domäne I (GAEB-Parser-Ersatz,
LLM-Provider-Interface, Confidence-Modell) oder für die in
`docs/01-gap-analysis.md` benannten Lücken (Mandantenfähigkeit, GoBD-Storno,
DATEV-Export, E-Rechnung). Diese folgen als eigene ADRs, sobald die
entsprechenden Subtasks im Backlog angegangen werden.
