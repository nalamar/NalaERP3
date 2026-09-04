-- Kostenstellenmodell (ADR 0019, Backlog E.1.2). Eine neue
-- Stammdatentabelle, eine additive Spalte, zwei neue Permissions:
--   - cost_centers: mandantenweite Stammdaten (wie materials/warehouses),
--     code eindeutig je Mandant.
--   - journal_lines.kostenstelle_id: additive, nullable FK - Zuordnung
--     auf Zeilenebene (nicht journal_entries-Kopf), da eine einzelne
--     Buchung mehrere Kostenstellen gleichzeitig betreffen kann (siehe
--     ADR 0019). Bewusst optional, kein Zwang zur Zuordnung.

CREATE TABLE IF NOT EXISTS cost_centers (
    id text PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    code text NOT NULL,
    name text NOT NULL,
    aktiv boolean NOT NULL DEFAULT true,
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, code)
);

CREATE INDEX IF NOT EXISTS idx_cost_centers_company_id ON cost_centers(company_id);

ALTER TABLE journal_lines
    ADD COLUMN IF NOT EXISTS kostenstelle_id text REFERENCES cost_centers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_journal_lines_kostenstelle ON journal_lines(kostenstelle_id);

INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-cost-centers-read', 'cost_centers.read', 'Kostenstellen lesen', 'Kostenstellen anzeigen', 'finance'),
  ('perm-cost-centers-write', 'cost_centers.write', 'Kostenstellen schreiben', 'Kostenstellen anlegen und bearbeiten', 'finance')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-finance', p.id
FROM permissions p
WHERE p.code IN ('cost_centers.read', 'cost_centers.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code IN ('cost_centers.read', 'cost_centers.write')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('cost_centers.read', 'cost_centers.write'));
--   DELETE FROM permissions WHERE code IN ('cost_centers.read', 'cost_centers.write');
--   ALTER TABLE journal_lines DROP COLUMN IF EXISTS kostenstelle_id;
--   DROP TABLE IF EXISTS cost_centers;
-- DATENVERLUSTRISIKO: Sobald echte Kostenstellen angelegt und Buchungen
-- damit verknuepft wurden, gehen beide bei einem Downgrade unwiderruflich
-- verloren (die Buchungen selbst bleiben erhalten, nur die
-- Kostenstellen-Zuordnung faellt weg). Kein Risiko fuer bestehende
-- journal_entries/journal_lines-Daten - beide bleiben inhaltlich
-- unangetastet.
