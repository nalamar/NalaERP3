-- Stellen als Vorlagen fuer erforderliche Qualifikationen (ADR 0031,
-- Backlog F.11.2). Zwei Tabellen:
--   - hr_positions: die Stellen des Mandanten ("Monteur", "Schweisser",
--     "Lagerist"), gepflegt wie der Qualifikationskatalog.
--   - hr_position_qualification_types: welche Arten eine Stelle verlangt.
--
-- Die Zuordnung Mitarbeiter -> Stellen und ihre Wirkung auf den Stand
-- folgen in F.11.3. Diese Migration legt KEINE Stellen an; ein
-- Vorschlagskatalog wird wie bei den Qualifikationsarten per
-- ausdruecklichem Aufruf uebernommen (F.11.4).
--
-- Der Code ist der stabile Schluessel (Abgleich mit dem
-- Vorschlagskatalog), der Name ist aenderbar. Beide eindeutig je Mandant,
-- der Name ohne Ruecksicht auf Gross-/Kleinschreibung - "Monteur" und
-- "monteur" waeren in einer Auswahlliste nicht zu unterscheiden.
--
-- Stellen werden nicht geloescht, sondern ueber aktiv=false stillgelegt
-- (ab F.11.3 verweisen Mitarbeiter auf sie).
CREATE TABLE IF NOT EXISTS hr_positions (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    code text NOT NULL,
    name text NOT NULL,
    aktiv boolean NOT NULL DEFAULT true,
    hinweis text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT hr_positions_company_code_key UNIQUE (company_id, code)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_positions_company_name ON hr_positions(company_id, lower(name));

-- Eine Art verlangt die Stelle; die Art muss zum selben Mandanten
-- gehoeren (geprueft im Service, wie bei den Anforderungen je
-- Mitarbeiter). ON DELETE RESTRICT auf die Art: Arten werden ohnehin nie
-- geloescht.
CREATE TABLE IF NOT EXISTS hr_position_qualification_types (
    position_id uuid NOT NULL REFERENCES hr_positions(id) ON DELETE CASCADE,
    type_id uuid NOT NULL REFERENCES hr_qualification_types(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (position_id, type_id)
);
CREATE INDEX IF NOT EXISTS idx_hr_position_qualification_types_type ON hr_position_qualification_types(type_id);

COMMENT ON TABLE hr_positions IS
    'Stellen je Mandant als Vorlagen fuer erforderliche Qualifikationen (ADR 0031). Stilllegen statt loeschen.';
COMMENT ON TABLE hr_position_qualification_types IS
    'Welche Qualifikationsarten eine Stelle verlangt (ADR 0031). Planung, aenderbar - kein Beleg.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS hr_position_qualification_types;
--   DROP TABLE IF EXISTS hr_positions;
-- DATENVERLUSTRISIKO (MITTEL): die vom Mandanten gepflegten Stellen und
-- ihre Zuordnung zu Qualifikationsarten gehen verloren. Keine Nachweise,
-- keine Anforderungen je Mitarbeiter und keine Mitarbeiterdaten betroffen;
-- keine andere Tabelle verweist auf die beiden (Stand F.11.2).
