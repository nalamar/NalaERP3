-- Preisliste als eigene Entitaet (ADR 0007, Backlog A.2.2). Zwei neue
-- Tabellen, keine bestehende Tabelle betroffen:
--   - price_lists: Header (Name, Lieferant als Freitext, Waehrung PRO
--     LISTE, Gueltigkeitszeitraum, aktiv). company_id von Anfang an NOT
--     NULL - anders als bei den 0.2.1.2-Migrationen gibt es hier keine
--     Bestandsdaten, die nachtraeglich befuellt werden muessten.
--   - price_list_items: Zeilen/Staffelpreise, Scope ueber price_list_id
--     geerbt (kein eigenes company_id, analog quote_items/
--     sales_order_items, siehe ADR 0002). Staffelpreis-Semantik: die Zeile
--     mit der groessten min_menge <= bestellte Menge gewinnt (Anwendung
--     in Backlog A.2.3).
--
-- Bewusst NICHT Teil dieser Migration: keine Aenderung an
-- quote_item_price_decisions oder der bestehenden Preisfindungskette in
-- quotes/service.go (siehe ADR 0007, Abgrenzung zu Domaene B).

CREATE TABLE IF NOT EXISTS price_lists (
    id text PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    name text NOT NULL,
    lieferant text NOT NULL DEFAULT '',
    currency char(3) NOT NULL DEFAULT 'EUR',
    gueltig_von date NOT NULL,
    gueltig_bis date,
    aktiv boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_price_lists_gueltigkeit CHECK (gueltig_bis IS NULL OR gueltig_bis >= gueltig_von)
);
CREATE INDEX IF NOT EXISTS idx_price_lists_company_id ON price_lists(company_id);

CREATE TABLE IF NOT EXISTS price_list_items (
    id text PRIMARY KEY,
    price_list_id text NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    min_menge numeric(18,6) NOT NULL DEFAULT 0,
    unit_price numeric(18,4) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_price_list_items_min_menge CHECK (min_menge >= 0),
    CONSTRAINT chk_price_list_items_unit_price CHECK (unit_price >= 0),
    UNIQUE (price_list_id, material_id, min_menge)
);
CREATE INDEX IF NOT EXISTS idx_price_list_items_price_list_id ON price_list_items(price_list_id);
CREATE INDEX IF NOT EXISTS idx_price_list_items_material_id ON price_list_items(material_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS price_list_items;
--   DROP TABLE IF EXISTS price_lists;
-- DATENVERLUSTRISIKO: Sobald echte Preislisten (Header + Staffelpreiszeilen)
-- erfasst wurden, gehen diese bei einem Downgrade unwiderruflich verloren.
-- Kein Risiko fuer bereits bestehende Fachdaten anderer Domaenen, da beide
-- Tabellen neu sind und von keiner bestehenden Tabelle referenziert werden.
