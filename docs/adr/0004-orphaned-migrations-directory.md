# ADR 0004 — Umgang mit dem verwaisten `server/migrations/`-Verzeichnis

Datum: 2026-08-17
Status: entschieden (Subtask 0.4.2)
Bezug: `docs/00-recon.md:101,281` (Ersterkennung: "verwaister Altstand,
vermutlich Merge-Artefakt"), ADR 0003 (Migrations-Nummernkollisionen —
diese ADR klärt einen Teil von deren Ursache).

## Kontext

Neben dem aktiven, per `go:embed` eingebundenen Migrationsverzeichnis
`server/internal/migrate/migrations/` (63 Dateien, Stand dieser Session)
existiert ein zweites Verzeichnis `server/migrations/` mit 9 Dateien
(`001_projects.sql` … `008_seed_project_numbering.sql`, inkl. der
Doppelnummer `007` für zwei Dateien).

**Befund (diese Subtask, per `diff`)**: Alle 9 Dateien sind **byte-identisch**
mit gleichnamigen Dateien in `server/internal/migrate/migrations/`:

```
001_projects.sql                    == internal/migrate/migrations/001_projects.sql
002_project_phases.sql              == internal/migrate/migrations/002_project_phases.sql
003_elevations.sql                  == internal/migrate/migrations/003_elevations.sql
004_single_elevations.sql           == internal/migrate/migrations/004_single_elevations.sql
005_single_elevation_materials.sql  == internal/migrate/migrations/005_single_elevation_materials.sql
006_import_logs.sql                 == internal/migrate/migrations/006_import_logs.sql
007_alter_import_logs.sql           == internal/migrate/migrations/007_alter_import_logs.sql
007_elevation_attrs.sql             == internal/migrate/migrations/007_elevation_attrs.sql
008_seed_project_numbering.sql      == internal/migrate/migrations/008_seed_project_numbering.sql
```

Das erklärt zugleich einen Teil der in ADR 0003 dokumentierten
Nummernkollisionen: diese 9 Dateien bilden genau die Hälfte der
Duplikat-Paare bei den Nummern `001`–`008`. Die naheliegende Erklärung: dies
ist der **alte Verzeichnisstandort** einer der beiden historisch parallel
entstandenen Migrationsketten, bevor sie — vermutlich weil `go:embed`
Dateien innerhalb des Go-Package-Baums verlangt (`server/migrations/` liegt
außerhalb von `server/internal/`, kann also nicht direkt eingebettet werden)
— nach `server/internal/migrate/migrations/` verschoben/kopiert wurde. Der
alte Ordner wurde dabei nicht gelöscht.

**Referenzprüfung**: `grep -rn "server/migrations"` über das gesamte Repo
(Go-Code, `*.yml`/`*.yaml`, `Dockerfile*`, `*.md`) findet außerhalb der
eigenen Recon-/Backlog-/ADR-Notizen dieser Session **keinen einzigen
Treffer** — insbesondere `server/internal/migrate/migrate.go` bettet
ausschließlich `migrations/*.sql` relativ zu seinem eigenen Package-Pfad
ein (`server/internal/migrate/migrations/`), niemals den Top-Level-Ordner.
`git log --oneline -- server/migrations/` zeigt einen einzelnen,
lange zurückliegenden Commit — seither unverändert, sauberer Git-Status.

## Optionen

**Option A — Verzeichnis löschen.**
Da alle 9 Dateien nachweislich byte-identische, bereits aktive Duplikate
sind und nirgends referenziert werden, trägt das Verzeichnis keine
Information, die nicht bereits an anderer Stelle (aktiv, eingebunden)
vorhanden ist. Risiko: aufgabe.md §7.6 verlangt eine ADR vor
Löschung/Umbenennung — genau das ist diese ADR.

**Option B — Verzeichnis als "Alt-Stand" markieren (z. B. `README.md`
oder `.migrations-archived` hinzufügen), aber nicht löschen.**
Konservativer, aber löst das eigentliche Problem nicht: ein verwaistes,
verwirrendes Verzeichnis bleibt im Repo bestehen und lädt dazu ein, erneut
missverstanden oder (schlimmer) versehentlich als "der eigentlich richtige"
Migrationsordner behandelt zu werden — genau das Gegenteil von "Verzeichnis
bereinigen" (Task 0.4).

**Option C — Verzeichnis unverändert lassen, nur dokumentieren.**
Bereits der Ist-Zustand vor dieser Subtask (dokumentiert in
`docs/00-recon.md`). Löst nichts, Task 0.4 wäre damit nicht erledigt.

## Entscheidung

**Option A — Verzeichnis `server/migrations/` vollständig löschen.**

Begründung: Es handelt sich nachweislich (byte-identischer `diff`, keine
Code-/Config-Referenz, sauberer Git-Status seit einem einzigen alten
Commit) um einen reinen Merge-/Verschiebungs-Rest ohne jeden eigenständigen
Inhalt oder Funktionswert. Eine Löschung entfernt kein Wissen, das nicht
bereits im aktiven, ausgeführten Verzeichnis vorhanden ist — die
identischen Dateien laufen über `server/internal/migrate/migrations/`
bereits bei jedem Serverstart. Anders als bei den in ADR 0003 belassenen
historischen Duplikaten (deren Umbenennen ohne Nutzen wäre) hat das
Löschen dieses Verzeichnisses einen klaren Nutzen: es beseitigt eine
Quelle für künftige Verwirrung (welches Verzeichnis ist "das echte"?) und
erfüllt den Auftrag von Task 0.4 ("Verzeichnis bereinigen") tatsächlich.

## Konsequenzen

- `server/migrations/` wird in dieser Subtask per `git rm -r` entfernt.
- Keine Auswirkung auf Laufzeitverhalten (Verzeichnis war nie referenziert,
  bestätigt durch repo-weite Suche).
- `docs/00-recon.md` bleibt als historisches Rechercheprotokoll unverändert
  (beschreibt den Ist-Stand zum Zeitpunkt der Recherche, keine laufende
  Dokumentation) — der Befund ist durch diese ADR aufgelöst.
- Damit ist Task 0.4 (Migrationsverzeichnis bereinigen) vollständig
  abgeschlossen: 0.4.1 hat die Nummernkollisionen im aktiven Verzeichnis
  bewertet und eine Konvention für die Zukunft festgelegt, 0.4.2 hat das
  zweite, verwaiste Verzeichnis vollständig entfernt.
