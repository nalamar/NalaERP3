# ADR 0003 — Umgang mit doppelten Nummernketten im Migrationsverzeichnis

Datum: 2026-08-17
Status: entschieden (Subtask 0.4.1)
Bezug: ADR 0001 (Entscheidung 6) hat das Risiko bereits als offene Frage
benannt ("Ob ein Wechsel auf ein etabliertes Migrationstool mit
Versions-Tracking sinnvoll ist, ist eine Backlog-Entscheidung, keine
Entscheidung dieser ADR") — diese ADR trifft die Folge-Entscheidung.

## Kontext

`server/internal/migrate/migrate.go` bettet alle `.sql`-Dateien aus
`migrations/` per `go:embed` ein und führt sie bei jedem Serverstart erneut
aus, sortiert per `sort.Strings` über den **vollen Dateinamen** (Nummer +
Unterstrich + Beschreibung), nicht nur über die Nummer. Es gibt **kein**
Versions-Tracking (keine `schema_migrations`-Tabelle o. ä.) — jede Datei
läuft bei jedem Start erneut, Sicherheit gegen Doppelausführung entsteht
ausschließlich durch idempotentes SQL (`CREATE TABLE IF NOT EXISTS`,
`ADD COLUMN IF NOT EXISTS`, …) in den Migrationsdateien selbst.

Im aktiven Verzeichnis `server/internal/migrate/migrations/` teilen sich
aktuell 14 Nummern-Präfixe jeweils zwei (bei `007` sogar drei) Dateien:

```
001: init.sql, projects.sql
002: material_documents.sql, project_phases.sql
003: contacts.sql, elevations.sql
004: purchase_orders.sql, single_elevations.sql
005: numbering.sql, single_elevation_materials.sql
006: import_logs.sql, pdf_templates.sql
007: alter_import_logs.sql, elevation_attrs.sql, projects.sql
008: project_phases.sql, seed_project_numbering.sql
013: alter_import_logs.sql, material_links.sql
014: material_dimensions_and_units.sql, seed_project_numbering.sql
017: accounting_basics.sql, auth.sql
018: contact_status.sql, journal_and_ar.sql
019: contact_commercial_fields.sql, opos_bank.sql
020: contact_duplicate_indexes.sql, hr_base.sql
```

Ab Nummer `021` ist die Kette durchgängig eindeutig bis `063` (Stand dieser
Session). Die Duplikate stammen erkennbar aus zwei parallel entstandenen
Nummernketten (vermutlich zwei unterschiedliche frühere Entwicklungsstränge:
eine "fachliche" Kette — Kontakte/Projekte/Elevationen/Import — und eine
zweite, ähnlich nummerierte Kette für einen anderen Bereich), die zu einem
Zeitpunkt zusammengeführt wurden, ohne die Nummern zu vereinheitlichen.

**Funktional stabil, aber fragil**: Da `sort.Strings` über den vollen
Dateinamen sortiert, ist die Ausführungsreihenfolge innerhalb einer
Nummerngruppe deterministisch (z. B. `007_alter_import_logs.sql` vor
`007_elevation_attrs.sql` vor `007_projects.sql`, weil `a` < `e` < `p`) und
wurde in dieser Session bereits dutzendfach gegen frische Datenbanken
erfolgreich verifiziert (jede Subtask dieser Session hat einen kompletten
Migrationslauf durchlaufen). Das Risiko ist kein akuter Bug, sondern eine
**Wartungsfalle**: wer eine neue Migration hinzufügt, muss die "nächste
freie Nummer" durch Nachsehen im Verzeichnis ermitteln — genau das ist in
der Vergangenheit bereits zweimal fehlgeschlagen (daher die 14 Duplikate).
Ohne Gegenmaßnahme wird es wieder passieren.

## Optionen

**Option A — Historische Duplikate umbenennen (Nummern vereinheitlichen).**
Alle ~20 betroffenen Dateien auf eine durchgängig eindeutige Kette
umnummerieren (z. B. `001`–`020` → `001`–`034`, Rest verschiebt sich).
Vorteil: das Verzeichnis wird auf einen Blick eindeutig. Nachteil: berührt
~20 bereits produktiv gelaufene, funktionierende Dateien ohne funktionalen
Nutzen (kein Versions-Tracking existiert, das durch den alten Dateinamen
referenziert würde — das Umbenennen ist technisch risikolos, aber auch
ohne jeden Laufzeit-Nutzen) und verschiebt zusätzlich die Nummern aller 43
nachfolgenden Dateien (`021`–`063`), was den Diff unnötig aufbläht und die
Nachvollziehbarkeit früherer Commits/State-Log-Einträge erschwert (state.md
referenziert Migrationsnummern wörtlich, z. B. "Migration 057").

**Option B — Nur Konvention für neue Migrationen ändern, Historie
unangetastet lassen.**
Die 14 bestehenden Duplikate bleiben wie sie sind (sie funktionieren
nachweislich). Für alle **neuen** Migrationen ab sofort ein Namensschema,
das Kollisionen strukturell unwahrscheinlich macht, statt sich auf
manuelles Nachsehen der "nächsten freien Nummer" zu verlassen.

**Option C — Wechsel auf ein etabliertes Migrationstool (`golang-migrate`,
`goose`) mit echtem Versions-Tracking.**
Würde das Problem grundsätzlicher lösen (inkl. Down-Migrationen, siehe
Backlog 0.13) und Kollisionen technisch verhindern (Tools dieser Art lehnen
doppelte Versionsnummern beim Erstellen i. d. R. ab). Deutlich größerer
Eingriff: neue Abhängigkeit, Umbau des gesamten Runners, Anpassung von
`server/internal/app/server.go`, aller Docker-/CI-Skripte, plus die
Notwendigkeit, für die bereits gelaufenen 63 Dateien einen Weg zu finden,
sie als "bereits angewendet" zu markieren (sonst würden sie beim
Tool-Wechsel erneut versucht). Klar außerhalb des Umfangs von "Verzeichnis
bereinigen" (Task 0.4) — wäre eine eigene, deutlich größere Epic-Initiative.

## Entscheidung

**Option B**, mit einem Baustein aus Option A in stark reduzierter Form:

1. **Historische Duplikate (`001`–`020`) bleiben unverändert.** Sie sind
   nachweislich funktionsfähig (dutzendfach diese Session gegen frische DB
   verifiziert), ein Umbenennen hat keinen Laufzeit-Nutzen (kein
   Versions-Tracking, das den alten Namen referenziert) und würde
   ausschließlich Risiko ohne Gegenwert einführen sowie bestehende
   Dokumentation (`docs/state.md`-Log-Einträge, die Migrationsnummern
   wörtlich zitieren) entwerten.
2. **Ab sofort gilt für neue Migrationen ein 3-stelliges, streng
   monoton wachsendes Nummernpräfix** (wie bisher: `NNN_beschreibung.sql`,
   3-stellig, zero-padded) — **aber** mit einer verbindlichen Regel: vor
   dem Anlegen einer neuen Migration MUSS `ls server/internal/migrate/migrations/
   | sort | tail -3` (oder äquivalent) geprüft werden, und die neue Nummer
   muss strikt größer sein als jede vorhandene. Diese Regel war implizit
   bereits die Absicht hinter dem bisherigen Schema — sie wird hiermit
   **explizit als Kontrakt** festgehalten (bisher nirgends dokumentiert,
   daher die Duplikate).
3. **Kein Wechsel auf Zeitstempel-Präfixe.** Erwogen (strukturell
   kollisionsfrei ohne Nachsehen), aber verworfen: das würde einen
   Stilbruch mitten in einer bislang durchgängig 3-stelligen Kette
   einführen und dem in `aufgabe.md` etablierten Muster (kurze,
   sprechende `NNN_beschreibung.sql`-Namen, wie in jeder Subtask dieser
   Session verwendet) widersprechen, ohne einen Vorteil zu bieten, den
   Punkt 2 nicht bereits abdeckt — solange die Regel "vorher nachsehen"
   tatsächlich befolgt wird. Sollte die Duplikat-Problematik trotz dieser
   ADR erneut auftreten, ist das ein Signal, Option C (echtes
   Migrationstool) neu zu bewerten.
4. **Kein Wechsel auf ein externes Migrationstool (Option C) im Rahmen
   dieser ADR.** Das bereits dokumentierte, verwandte Problem "keine
   Down-Migrationen" (Backlog 0.13) und "kein Versions-Tracking" bleiben
   bewusst offene, separate Backlog-Punkte — beide sind Voraussetzung für
   einen sinnvollen Tool-Wechsel und würden dessen Umfang so stark
   vergrößern, dass er nicht mehr in Task 0.4 ("Verzeichnis bereinigen")
   passt.

## Konsequenzen

- Keine Codeänderung an `server/internal/migrate/migrate.go` oder an
  bestehenden `.sql`-Dateien nötig — diese ADR ist eine reine
  Konventions-/Prozessentscheidung.
- Jede künftige Subtask, die eine neue Migration hinzufügt, muss die
  Nachsehen-Regel aus Entscheidung 2 befolgen (bereits gelebte Praxis in
  dieser Session ab Migration 054 — wird hiermit nur formalisiert).
- Das Risiko einer erneuten Kollision ist **nicht strukturell
  ausgeschlossen** (anders als bei Zeitstempeln oder einem echten Tool),
  sondern hängt weiterhin von der Disziplin der Ausführenden ab. Dieser
  Kompromiss wird bewusst eingegangen, um den Eingriff auf das für Task 0.4
  angemessene Maß zu begrenzen.
- Backlog 0.13 (keine Down-Migrationen) und ein mögliches künftiges
  "echtes Versions-Tracking"-Backlog-Item bleiben unverändert offen und
  unabhängig von dieser Entscheidung.
- Diese ADR deckt ausschließlich `server/internal/migrate/migrations/` ab.
  Das separate, verwaiste Verzeichnis `server/migrations/` (9 Dateien,
  nicht per `go:embed` eingebunden) ist Gegenstand von Subtask 0.4.2 und
  wird hier nicht behandelt.
