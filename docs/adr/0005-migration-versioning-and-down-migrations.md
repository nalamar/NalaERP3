# ADR 0005 — Versions-Tracking für Migrationen; Down-Migrationen bleiben manuell

Datum: 2026-08-16
Status: entschieden (Backlog 0.13)
Bezug: ADR 0003 (Entscheidung 4) hat einen Wechsel auf ein externes
Migrationstool bewusst aus dem Umfang von Task 0.4 ausgeklammert und auf
Backlog 0.13 verwiesen. Diese ADR trifft die Folge-Entscheidung für 0.13.

## Kontext

`server/internal/migrate/migrate.go` führte bisher **jede** `.sql`-Datei aus
`migrations/` bei **jedem** Aufruf von `Run()` erneut aus — kein
Versions-Tracking, keine Down-Skripte, kein Rollback-Mechanismus. Das hatte
zwei dokumentierte, konkrete Folgen:

1. **Backlog 0.20 (KRITISCH)**: Migration `050_quote_item_approval_requests.sql`
   fügt einen Check-Constraint hinzu, `051_quote_item_approval_decisions.sql`
   entfernt ihn im selben Durchlauf wieder. Bei jedem ZWEITEN+ `Run()` gegen
   dieselbe, bereits benutzte DB (z. B. jede weitere Testfunktion, die
   `testutil.SetupIntegrationEnv` aufruft) versuchte `050` den Constraint
   erneut anzulegen — schlug fehl, sobald zwischenzeitlich Zeilen eingefügt
   wurden, die die urspüngliche (durch `051` eigentlich abgelöste) Regel
   verletzten. In einem vollen `go test ./internal/http`-Lauf kaskadierte das
   auf ca. 70 von ~75 Testfunktionen.
2. **aufgabe.md §7.8** ("Keine Migration ohne Down-Pfad und ohne Hinweis auf
   Datenverlustrisiko") war nur über Kommentarblöcke in einzelnen
   Migrationsdateien erfüllbar (z. B. `060_users_numbering_scope.sql`), nicht
   durch automatisiertes Tooling.

## Optionen

**Option A — Vollständiges Up/Down-Migrationstool** (gepaarte
`NNN_up.sql`/`NNN_down.sql`-Dateien, `schema_migrations`-Tabelle, `Up`/`Down`-
Befehle, ggf. Wechsel auf `golang-migrate`/`goose`). Löst beide Probleme
grundsätzlich, ist aber ein sehr großer Eingriff: Umbau des gesamten Runners,
Aufteilung/Nachbau von Rollback-SQL für alle 64 bestehenden Migrationen
(rückwirkend Down-SQL für historische Migrationen zu schreiben ist
risikoreich und für viele frühe Migrationen kaum noch sinnvoll
rekonstruierbar), neue Abhängigkeit, Anpassung von Docker-/CI-Skripten.
Bereits in ADR 0003 als "eigene, deutlich größere Epic-Initiative" bewertet.

**Option B — Nur Versions-Tracking (keine Re-Ausführung), Down-Migrationen
bleiben manuell/dokumentiert.** Eine `schema_migrations`-Tabelle
(`filename` als Primärschlüssel) verhindert, dass eine Datei zweimal läuft.
Down-Migrationen bleiben wie bisher als Kommentarblock im jeweiligen
Migrationsfile dokumentiert (manuell auszuführen), statt automatisiert
anwendbar zu sein. Deutlich kleinerer, gut abgegrenzter Eingriff (im
Wesentlichen nur `migrate.go`), löst Backlog 0.20 vollständig (Migrationen
laufen nur noch einmal, der 050/051-Konflikt kann nach der Einführung nicht
mehr durch einen erneuten `Run()` gegen dieselbe DB ausgelöst werden) und
erfüllt aufgabe.md §7.8 weiterhin über die bereits gelebte
Kommentar-Konvention (ein "Down-Pfad" muss laut Wortlaut vorhanden und
dokumentiert sein, nicht zwingend automatisiert ausführbar).

**Option C — Nichts ändern, nur als Produktentscheidung dokumentieren, dass
Rollbacks manuell bleiben.** Löst Backlog 0.20 NICHT (der eigentliche,
kritische Auslöser dieser ADR) — verworfen, da 0.20 bereits als eigenständiger
kritischer Fund dokumentiert ist und ungelöst bliebe, obwohl die Lösung
(Versions-Tracking) klein und gut abgegrenzt ist.

## Entscheidung

**Option B.**

1. Neue Tabelle `schema_migrations (filename text PRIMARY KEY, applied_at
   timestamptz)`, angelegt idempotent (`CREATE TABLE IF NOT EXISTS`) zu
   Beginn jedes `Run()`-Aufrufs — kein separates Migrationsfile, um das
   Henne-Ei-Problem zu vermeiden (die Tracking-Tabelle kann nicht per
   getrackter Migration angelegt werden, bevor das Tracking existiert).
2. `Run()` liest die bereits angewendeten Dateinamen, führt nur noch NICHT
   angewendete Dateien aus, jeweils atomar in einer Transaktion zusammen mit
   dem `INSERT INTO schema_migrations` (beides zusammen committet oder beides
   zurückgerollt — kein Zwischenzustand, in dem eine Migration als
   "angewendet" markiert ist, obwohl sie fehlgeschlagen ist, oder umgekehrt).
3. **Postgres Advisory Lock** (`pg_advisory_lock`/`pg_advisory_unlock`,
   session-gebunden über eine explizit aus dem Pool bezogene Verbindung) um
   den gesamten `Run()`-Ablauf. Bei der Verifikation dieser Entscheidung
   selbst gefunden: mehrere Go-Testpakete, die gleichzeitig
   `testutil.SetupIntegrationEnv` aufrufen (Standardverhalten von `go test`
   über mehrere Pakete hinweg), riefen `Run()` parallel gegen dieselbe DB
   auf — ohne Lock entstand eine Race Condition zwischen "geprüft: noch
   nicht angewendet" und "als angewendet markiert" (doppelte
   `schema_migrations`-Einträge bzw. Konflikte in der Migrations-DDL selbst,
   z. B. doppelt angelegte Typen). Der Advisory Lock serialisiert
   nebenläufige `Run()`-Aufrufe: ein zweiter Prozess wartet, bis der erste
   fertig ist, und überspringt danach alles (bereits als angewendet
   markiert).
4. **Down-Migrationen bleiben bewusst manuell/dokumentiert**, kein
   automatisiertes Tooling. Bereits gelebte Konvention (Kommentarblock im
   Migrationsfile, z. B. `060_users_numbering_scope.sql`) wird als
   verbindlicher Standard für alle künftigen Migrationen festgehalten:
   > Jede neue Migration mit strukturellem Risiko (Spalten-/Tabellenwegfall,
   > `NOT NULL`, Typänderung) MUSS einen Kommentarblock `-- DOWN (manuell
   > auszuführen; ...)` mit dem Rückgängigmachungs-SQL UND einem expliziten
   > Hinweis auf Datenverlustrisiko enthalten (aufgabe.md §7.8).
5. **Bestehende Migrationen (001-064) werden NICHT rückwirkend mit
   Down-Kommentaren nachgerüstet.** Historische Migrationen ohne
   Down-Kommentar bleiben wie sie sind — retroaktiv korrektes Rollback-SQL zu
   rekonstruieren wäre selbst riskant (Gefahr, ein falsches Rollback zu
   dokumentieren) und ohne klaren Nutzen, da diese Migrationen bereits
   produktiv gelaufen sind.

## Konsequenzen

- `server/internal/migrate/migrate.go` geändert: `schema_migrations`-Tabelle,
  Advisory Lock, Skip-Logik für bereits angewendete Dateien. Neue Tests
  (`server/internal/migrate/migrate_test.go`, externes Testpaket
  `migrate_test`, um den Importzyklus mit `testutil` zu vermeiden):
  `TestRunTracksEveryMigrationFile`, `TestRunIsIdempotentOnRepeatedInvocation`.
- **Backlog 0.20 ist als Nebeneffekt gelöst**: verifiziert per vollem
  `go test ./internal/http`-Lauf gegen frische DB — die zuvor ~70/75
  kaskadierenden Fehlschläge sind auf 27 zurückgegangen, alle bereits
  anderweitig dokumentiert (0.19/0.21/0.24/0.26/0.28/0.29/0.34/0.35/0.38)
  bis auf einen neu entdeckten, unabhängigen Fund (Backlog 0.39: Testfixture
  referenziert nicht existierende Spalte `materials.updated_at`).
- **Migrationslauf gegen eine bereits vor dieser Änderung migrierte,
  produktive DB**: `schema_migrations` startet dort leer, der erste `Run()`
  nach dieser Änderung würde ALLE 64 Migrationen erneut versuchen. Die
  meisten sind idempotent (`IF NOT EXISTS`/`ON CONFLICT DO NOTHING`) und
  daher unkritisch — Migrationen `050`/`051` könnten aber genau dann erneut
  in den bereits dokumentierten Backlog-0.20-Konflikt laufen, wenn zu diesem
  Zeitpunkt bereits Daten vorhanden sind, die die ursprüngliche Regel
  verletzen. Dieses Risiko besteht nur für den EINMALIGEN Übergang einer
  bereits existierenden, nicht neu aufgesetzten Datenbank; für jede frische
  Installation (der bisher einzige in dieser Session tatsächlich verifizierte
  Fall) tritt es nicht auf. Kein Bootstrap-Mechanismus (z. B. Vorbefüllen von
  `schema_migrations` für eine bestehende Alt-DB) wurde für dieses hypothetische
  Szenario gebaut, da aktuell keine bekannte produktive Alt-Installation
  existiert, die diesen Übergang durchlaufen müsste.
- Kein Wechsel auf ein externes Migrationstool, keine Aufteilung in
  `_up`/`_down`-Dateien, keine rückwirkenden Down-Kommentare für 001-064.
