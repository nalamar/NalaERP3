-- Projektcontrolling (ADR 0020, Backlog E.2.2). Eine additive, nullable
-- Spalte auf projects - die fehlende Verknuepfung zwischen Projekten und
-- Kostenstellen (E.1), ohne die keine Ist-Kosten-Aggregation je Projekt
-- moeglich ist (siehe ADR 0020, Kontext). Bewusst optional: nicht jedes
-- Projekt braucht formales Kosten-Controlling, ein Zwang waere eine
-- unbelegte fachliche Vorgabe.

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS kostenstelle_id text REFERENCES cost_centers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_projects_kostenstelle ON projects(kostenstelle_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE projects DROP COLUMN IF EXISTS kostenstelle_id;
-- DATENVERLUSTRISIKO: Sobald echte Projekt-Kostenstellen-Zuordnungen
-- vorgenommen wurden, gehen diese bei einem Downgrade unwiderruflich
-- verloren. Kein Risiko fuer bestehende projects-/cost_centers-Daten -
-- beide bleiben inhaltlich unangetastet.
