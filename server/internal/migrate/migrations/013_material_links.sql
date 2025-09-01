-- Link von importierten Varianten-Materialzeilen zu Stammmaterialien

ALTER TABLE single_elevation_profiles
    ADD COLUMN IF NOT EXISTS material_id text NULL REFERENCES materials(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_prof_material ON single_elevation_profiles (material_id);

ALTER TABLE single_elevation_articles
    ADD COLUMN IF NOT EXISTS material_id text NULL REFERENCES materials(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_art_material ON single_elevation_articles (material_id);

ALTER TABLE single_elevation_glass
    ADD COLUMN IF NOT EXISTS material_id text NULL REFERENCES materials(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_gls_material ON single_elevation_glass (material_id);
