-- Produktion: Fertigungsauftraege aus Projektpositionen (ADR 0034,
-- Backlog H.1.1). Entscheidungen des Nutzers vom 2026-10-31: alle sechs
-- Empfehlungen der ADR angenommen.
--
-- Ein Auftrag gehoert zu GENAU EINER Projektposition (Logikal-Position)
-- und einer Variante, mit Stueckzahl. Mehrere Auftraege je Position sind
-- erlaubt (Teillose); mehr als die Positionsmenge gibt nur einen Hinweis
-- (Service), keine Sperre.
--
-- Ein erneuter Logikal-Import aendert Positionen und LOESCHT Positionen
-- und Varianten (projects/import_logikal.go). Darum:
--   * Verweise auf Position und Variante mit ON DELETE SET NULL - der
--     Auftrag bleibt, wenn Logikal sie entfernt;
--   * beim FREIGEBEN werden Phase, Position, Variante, Masse und
--     Materialliste in den Auftrag kopiert (production_order_items) und
--     danach nie mehr geaendert; inhalt_hash haelt den Fingerabdruck der
--     Kopie fest, damit der Service "Position nach Freigabe geaendert"
--     erkennen kann.
-- Das Projekt selbst mit RESTRICT: ein Projekt mit Fertigungsauftraegen
-- laesst sich nicht loeschen (wie stock_reservations, 075).
--
-- Status linear: entwurf -> freigegeben -> in_arbeit -> fertig; storniert
-- (mit Grund) aus jedem Status vor "fertig". Jeder Uebergang passiert
-- hoechstens einmal, deshalb stehen Zeitpunkt und Benutzer je Uebergang
-- als Spalten am Auftrag (statt einer eigenen Verlaufstabelle). Nie
-- loeschen.
--
-- ausfuehrungsklasse (EN 1090, EXC1-EXC4): optional, in H.1 ohne Wirkung;
-- ab H.3 wird bei EXC2+ die Charge bei der Entnahme Pflicht.
CREATE TABLE IF NOT EXISTS production_orders (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    nummer text NOT NULL,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    elevation_id uuid REFERENCES project_elevations(id) ON DELETE SET NULL,
    single_elevation_id uuid REFERENCES project_single_elevations(id) ON DELETE SET NULL,
    stueckzahl integer NOT NULL,
    liefertermin date,
    ausfuehrungsklasse text,
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'entwurf',
    -- Kopie bei der Freigabe (vorher leer).
    phase_name text NOT NULL DEFAULT '',
    position_nummer text NOT NULL DEFAULT '',
    position_name text NOT NULL DEFAULT '',
    position_guid text NOT NULL DEFAULT '',
    variante_name text NOT NULL DEFAULT '',
    variante_guid text NOT NULL DEFAULT '',
    width_mm numeric(12,2),
    height_mm numeric(12,2),
    inhalt_hash text NOT NULL DEFAULT '',
    freigegeben_at timestamptz,
    freigegeben_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    begonnen_at timestamptz,
    begonnen_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    fertig_at timestamptz,
    fertig_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT production_orders_company_nummer_key UNIQUE (company_id, nummer),
    CONSTRAINT production_orders_nummer_nonempty CHECK (btrim(nummer) <> ''),
    CONSTRAINT production_orders_stueckzahl_check CHECK (stueckzahl > 0),
    CONSTRAINT production_orders_exc_check
        CHECK (ausfuehrungsklasse IS NULL OR ausfuehrungsklasse IN ('EXC1', 'EXC2', 'EXC3', 'EXC4')),
    CONSTRAINT production_orders_status_check
        CHECK (status IN ('entwurf', 'freigegeben', 'in_arbeit', 'fertig', 'storniert')),
    -- Zeitpunkte passend zum Status; storniert nur vor "fertig".
    CONSTRAINT production_orders_status_times CHECK (
        (status = 'entwurf' AND freigegeben_at IS NULL AND begonnen_at IS NULL AND fertig_at IS NULL)
        OR (status = 'freigegeben' AND freigegeben_at IS NOT NULL AND begonnen_at IS NULL AND fertig_at IS NULL)
        OR (status = 'in_arbeit' AND freigegeben_at IS NOT NULL AND begonnen_at IS NOT NULL AND fertig_at IS NULL)
        OR (status = 'fertig' AND freigegeben_at IS NOT NULL AND begonnen_at IS NOT NULL AND fertig_at IS NOT NULL)
        OR (status = 'storniert' AND fertig_at IS NULL AND (begonnen_at IS NULL OR freigegeben_at IS NOT NULL))
    ),
    CONSTRAINT production_orders_time_order CHECK (
        (begonnen_at IS NULL OR begonnen_at >= freigegeben_at)
        AND (fertig_at IS NULL OR fertig_at >= begonnen_at)
    ),
    CONSTRAINT production_orders_storno_consistent CHECK (
        (status <> 'storniert' AND storniert_at IS NULL AND storno_reason = '')
        OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
    ),
    -- Freigegeben heisst: die Kopie ist da.
    CONSTRAINT production_orders_copy_check CHECK (
        freigegeben_at IS NULL OR (inhalt_hash <> '' AND btrim(position_nummer) <> '')
    )
);

CREATE INDEX IF NOT EXISTS idx_production_orders_company_status
    ON production_orders(company_id, status, liefertermin);
CREATE INDEX IF NOT EXISTS idx_production_orders_project ON production_orders(project_id);
CREATE INDEX IF NOT EXISTS idx_production_orders_elevation
    ON production_orders(elevation_id) WHERE elevation_id IS NOT NULL;

COMMENT ON TABLE production_orders IS
    'Fertigungsauftraege je Projektposition (ADR 0034): Kopie der Position bei Freigabe, Status linear, Storno statt Loeschen.';

-- Fertigungsstueckliste: Kopie der Materialliste der Variante bei der
-- Freigabe (Profile mit Laenge, Artikel, Glas mit Massen). Ohne Preise -
-- die sind Kalkulationsdaten. menge_je_stueck wie aus Logikal fuer EIN
-- Element der Variante; der Bedarf des Auftrags ist menge_je_stueck *
-- stueckzahl (Service). Nie geaendert, nie geloescht.
CREATE TABLE IF NOT EXISTS production_order_items (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES production_orders(id) ON DELETE RESTRICT,
    pos integer NOT NULL,
    art text NOT NULL,
    supplier_code text NOT NULL DEFAULT '',
    article_code text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    configuration text NOT NULL DEFAULT '',
    length_mm numeric(12,2),
    width_mm numeric(12,2),
    height_mm numeric(12,2),
    area_m2 numeric(12,4),
    menge_je_stueck numeric(18,6) NOT NULL,
    unit text NOT NULL DEFAULT '',
    CONSTRAINT production_order_items_order_pos_key UNIQUE (order_id, pos),
    CONSTRAINT production_order_items_pos_check CHECK (pos > 0),
    CONSTRAINT production_order_items_art_check CHECK (art IN ('profil', 'artikel', 'glas')),
    CONSTRAINT production_order_items_menge_check CHECK (menge_je_stueck >= 0),
    -- Laenge nur bei Profilen, Glasmasse und Konfiguration nur bei Glas.
    CONSTRAINT production_order_items_length_check CHECK (length_mm IS NULL OR (art = 'profil' AND length_mm >= 0)),
    CONSTRAINT production_order_items_glass_check CHECK (
        art = 'glas' OR (width_mm IS NULL AND height_mm IS NULL AND area_m2 IS NULL AND configuration = '')
    ),
    CONSTRAINT production_order_items_dims_check CHECK (
        (width_mm IS NULL OR width_mm >= 0) AND (height_mm IS NULL OR height_mm >= 0) AND (area_m2 IS NULL OR area_m2 >= 0)
    )
);

COMMENT ON TABLE production_order_items IS
    'Fertigungsstueckliste: Kopie der Materialliste der Variante bei Freigabe (ADR 0034 §2), unveraenderlich.';

INSERT INTO number_sequences (company_id, entity, pattern, next_value)
SELECT 'default', 'production_order', 'FA-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE company_id = 'default' AND entity = 'production_order');

-- Permissions (ADR 0034 §6): lesen fuer Produktion, Administrator und
-- Vertrieb; schreiben (anlegen, freigeben, Status, Storno) fuer
-- Produktion und Administrator. Eine eigene Rolle "Projektleitung" gibt
-- es nicht (017_auth.sql) - sie bekommt das Recht, sobald es sie gibt.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-production-read', 'production.read', 'Fertigung lesen', 'Fertigungsauftraege und Fertigungsstuecklisten anzeigen', 'production'),
  ('perm-production-write', 'production.write', 'Fertigung schreiben', 'Fertigungsauftraege anlegen, freigeben, Status setzen und stornieren', 'production')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-production', 'role-sales')
  AND p.code = 'production.read'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-production')
  AND p.code = 'production.write'
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('production.read', 'production.write'));
--   DELETE FROM permissions WHERE code IN ('production.read', 'production.write');
--   DELETE FROM number_sequences WHERE entity = 'production_order';
--   DROP TABLE IF EXISTS production_order_items;
--   DROP TABLE IF EXISTS production_orders;
-- DATENVERLUSTRISIKO (HOCH): verloren gehen alle Fertigungsauftraege mit
-- Statusverlauf, Storno-Gruenden und der bei Freigabe kopierten
-- Fertigungsstueckliste. Die Kopie laesst sich NICHT aus dem Projekt
-- wiederherstellen, wenn Logikal seitdem neu importiert wurde - vorher
-- vollstaendig exportieren. Projekte, Positionen und Lager bleiben
-- unberuehrt; die Nummernfolge beginnt nach erneutem Anlegen wieder bei 1.
