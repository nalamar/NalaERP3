-- Materiallisten je Ausführungsvariante

-- Profile
CREATE TABLE IF NOT EXISTS single_elevation_profiles (
    id UUID PRIMARY KEY,
    single_elevation_id UUID NOT NULL,
    supplier_code TEXT NULL,
    article_code TEXT NULL,
    description TEXT NULL,
    length_mm REAL NULL,
    qty REAL NOT NULL DEFAULT 1,
    unit TEXT NULL,
    price_net REAL NULL,
    currency TEXT NULL,
    CONSTRAINT fk_prof_single FOREIGN KEY (single_elevation_id) REFERENCES project_single_elevations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_prof_single ON single_elevation_profiles (single_elevation_id);

-- Articles (Beschläge, Zubehör, etc.)
CREATE TABLE IF NOT EXISTS single_elevation_articles (
    id UUID PRIMARY KEY,
    single_elevation_id UUID NOT NULL,
    supplier_code TEXT NULL,
    article_code TEXT NULL,
    description TEXT NULL,
    qty REAL NOT NULL DEFAULT 1,
    unit TEXT NULL,
    price_net REAL NULL,
    currency TEXT NULL,
    CONSTRAINT fk_art_single FOREIGN KEY (single_elevation_id) REFERENCES project_single_elevations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_art_single ON single_elevation_articles (single_elevation_id);

-- Glass
CREATE TABLE IF NOT EXISTS single_elevation_glass (
    id UUID PRIMARY KEY,
    single_elevation_id UUID NOT NULL,
    configuration TEXT NULL,   -- z.B. 4/16/4
    description TEXT NULL,
    width_mm REAL NULL,
    height_mm REAL NULL,
    area_m2 REAL NULL,
    qty REAL NOT NULL DEFAULT 1,
    unit TEXT NULL,
    price_net REAL NULL,
    currency TEXT NULL,
    CONSTRAINT fk_gls_single FOREIGN KEY (single_elevation_id) REFERENCES project_single_elevations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_gls_single ON single_elevation_glass (single_elevation_id);

