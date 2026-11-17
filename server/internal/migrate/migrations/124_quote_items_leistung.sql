-- Leistung an der Angebotsposition (ADR 0045 Frage 6, Backlog I.5.4).
--
-- Die Uebernahme eines Importlaufs setzt die bestaetigte bzw. geaenderte
-- Leistung aus der Zuordnung; der Preis bleibt 0 (Preise kommen nie aus
-- der Zuordnung). Speichern und Revision eines Angebots behalten sie. Wird
-- eine Leistung entfernt, bleibt die Position ohne Bezug.

ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS leistung_id text REFERENCES leistungen(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_quote_items_leistung ON quote_items (leistung_id) WHERE leistung_id IS NOT NULL;

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP INDEX IF EXISTS idx_quote_items_leistung;
--   ALTER TABLE quote_items DROP COLUMN IF EXISTS leistung_id;
-- DATENVERLUSTRISIKO (NIEDRIG): verloren geht nur der Bezug der
-- Angebotspositionen zur Leistung des Katalogs. Positionen, Texte, Mengen
-- und Preise bleiben; die Entscheidungen der Zuordnung stehen weiter in
-- quote_import_item_matches.
