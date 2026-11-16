-- GAEB-Angaben an Angebotsgliederung und -positionen (ADR 0041, Fragen 2 und 3,
-- Backlog I.2.4a).
--
-- outline_no: Ordnungszahl aus dem LV, leer bei von Hand angelegten Gruppen
-- und Positionen. I.6 (D84) braucht sie je Position, auch nach einer Revision.
--
-- quote_items: Kurz- und Langtext getrennt, Positionsart, Pauschale und die
-- OZ der Grundposition (base_outline_no, nur bei Alternativpositionen).
-- Der Verweis geht bewusst ueber die OZ statt ueber die Positions-ID:
-- Update legt beim Speichern alle Positionen mit neuen IDs an, Revise
-- ebenso - ein Fremdschluessel ginge dabei verloren. description bleibt
-- (Kurz- und Langtext) fuer PDF, Auftrag und bestehende Oberflaechen.
-- Bestehende Zeilen: Art normal, keine OZ.

ALTER TABLE quote_item_groups
    ADD COLUMN IF NOT EXISTS outline_no text NOT NULL DEFAULT '';

ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS outline_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS short_text text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS long_text text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS item_kind text NOT NULL DEFAULT 'normal',
    ADD COLUMN IF NOT EXISTS is_lump_sum boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS base_outline_no text NOT NULL DEFAULT '';
ALTER TABLE quote_items
    DROP CONSTRAINT IF EXISTS quote_items_item_kind_check,
    ADD CONSTRAINT quote_items_item_kind_check CHECK (item_kind IN (
        'normal', 'grundposition', 'alternativposition', 'bedarfsposition_mit_gb', 'bedarfsposition_ohne_gb')),
    DROP CONSTRAINT IF EXISTS quote_items_base_outline_check,
    ADD CONSTRAINT quote_items_base_outline_check CHECK (
        item_kind = 'alternativposition' OR base_outline_no = '');

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quote_items
--       DROP CONSTRAINT IF EXISTS quote_items_base_outline_check,
--       DROP CONSTRAINT IF EXISTS quote_items_item_kind_check,
--       DROP COLUMN IF EXISTS base_outline_no,
--       DROP COLUMN IF EXISTS is_lump_sum,
--       DROP COLUMN IF EXISTS item_kind,
--       DROP COLUMN IF EXISTS long_text,
--       DROP COLUMN IF EXISTS short_text,
--       DROP COLUMN IF EXISTS outline_no;
--   ALTER TABLE quote_item_groups DROP COLUMN IF EXISTS outline_no;
-- DATENVERLUSTRISIKO: Ordnungszahlen, getrennte Kurz-/Langtexte, Positionsart,
-- Pauschale und Bezug zur Grundposition von Angeboten gehen beim Downgrade
-- verloren; Positionen, Texte (description), Preise und Gruppen bleiben.
-- Nach einem Downgrade zaehlen Alternativ- und Bedarfspositionen wieder zur
-- Angebotssumme, sobald I.2.4d umgesetzt ist.
