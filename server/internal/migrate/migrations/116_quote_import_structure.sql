-- GAEB-Import mit Gliederung, Positionsart und getrennten Texten
-- (ADR 0040 Entscheidung 3 (a), Backlog I.2.2a).
--
-- quote_import_groups: Los/Titel/Untertitel eines Importlaufs, analog
-- quote_item_groups (071). Kein eigenes company_id - der Scope kommt ueber
-- import_id -> quote_imports -> projects.company_id (wie quote_import_items).
-- outline_no ist die Ordnungszahl des Bereichs, eindeutig je Importlauf;
-- bezeichnung darf leer sein (GAEB verlangt keinen Text, die Uebernahme ins
-- Angebot in I.2.4 setzt dann die Ordnungszahl ein).
--
-- quote_import_items: neue Spalten mit Defaults, bestehende Zeilen bleiben
-- gueltig (Art normal, keine Gruppe). description und is_optional bleiben
-- fuer den heutigen Client und die Uebernahme; description = Kurztext und
-- Langtext. base_position_no ist die OZ der Grundposition und genau bei
-- Alternativpositionen gesetzt. group_id SET NULL, damit das Loeschen einer
-- Gruppe keine Position mitnimmt.
--
-- quote_imports.vorbemerkungen: Vorbemerkungen des LV in Reihenfolge.

CREATE TABLE IF NOT EXISTS quote_import_groups (
    id uuid PRIMARY KEY,
    import_id uuid NOT NULL REFERENCES quote_imports(id) ON DELETE CASCADE,
    parent_group_id uuid REFERENCES quote_import_groups(id) ON DELETE CASCADE,
    outline_no text NOT NULL,
    kind text NOT NULL,
    bezeichnung text NOT NULL DEFAULT '',
    sort_order int NOT NULL DEFAULT 0,
    CONSTRAINT quote_import_groups_kind_check CHECK (kind IN ('los', 'titel', 'untertitel')),
    CONSTRAINT quote_import_groups_outline_check CHECK (BTRIM(outline_no) <> ''),
    CONSTRAINT quote_import_groups_outline_unique UNIQUE (import_id, outline_no)
);
CREATE INDEX IF NOT EXISTS idx_quote_import_groups_import_sort
    ON quote_import_groups (import_id, sort_order ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_quote_import_groups_parent
    ON quote_import_groups (parent_group_id);

ALTER TABLE quote_import_items
    ADD COLUMN IF NOT EXISTS short_text text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS long_text text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS item_kind text NOT NULL DEFAULT 'normal',
    ADD COLUMN IF NOT EXISTS is_lump_sum boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS base_position_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS group_id uuid REFERENCES quote_import_groups(id) ON DELETE SET NULL;
ALTER TABLE quote_import_items
    DROP CONSTRAINT IF EXISTS quote_import_items_item_kind_check,
    ADD CONSTRAINT quote_import_items_item_kind_check CHECK (item_kind IN (
        'normal', 'grundposition', 'alternativposition', 'bedarfsposition_mit_gb', 'bedarfsposition_ohne_gb')),
    DROP CONSTRAINT IF EXISTS quote_import_items_base_position_check,
    ADD CONSTRAINT quote_import_items_base_position_check CHECK (
        (item_kind = 'alternativposition') = (BTRIM(base_position_no) <> ''));
CREATE INDEX IF NOT EXISTS idx_quote_import_items_group_id ON quote_import_items (group_id);

ALTER TABLE quote_imports
    ADD COLUMN IF NOT EXISTS vorbemerkungen text[] NOT NULL DEFAULT '{}';

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quote_imports DROP COLUMN IF EXISTS vorbemerkungen;
--   ALTER TABLE quote_import_items
--       DROP CONSTRAINT IF EXISTS quote_import_items_base_position_check,
--       DROP CONSTRAINT IF EXISTS quote_import_items_item_kind_check,
--       DROP COLUMN IF EXISTS group_id,
--       DROP COLUMN IF EXISTS base_position_no,
--       DROP COLUMN IF EXISTS is_lump_sum,
--       DROP COLUMN IF EXISTS item_kind,
--       DROP COLUMN IF EXISTS long_text,
--       DROP COLUMN IF EXISTS short_text;
--   DROP TABLE IF EXISTS quote_import_groups;
-- DATENVERLUSTRISIKO: Gliederung, Vorbemerkungen, getrennte Kurz-/Langtexte,
-- Positionsart, Pauschale und Bezug zur Grundposition bereits importierter
-- LVs gehen beim Downgrade verloren. Die Importpositionen selbst bleiben
-- erhalten (description enthaelt Kurz- und Langtext, is_optional bleibt);
-- ein erneutes Verarbeiten der Quelldatei stellt die Angaben wieder her.
