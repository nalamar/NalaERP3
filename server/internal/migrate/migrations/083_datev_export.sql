-- DATEV-Export (ADR 0021, Backlog E.3.2). Fuenf additive Spalten auf
-- company_profiles fuer die im DATEV-EXTF-Kopfsatz (Format-Version 13,
-- Buchungsstapel) zwingend benoetigten, mandantenweiten Stammdaten, die im
-- bestehenden Datenmodell nirgends existieren (Beraternummer, Mandanten-
-- nummer, Kontenrahmen, Sachkontenlaenge, Wirtschaftsjahresbeginn-Monat).
-- Beraternummer/Mandantennummer bleiben nullable - der Export prueft beim
-- Aufruf explizit auf deren Vorhandensein (E.3.3), statt hier einen Dummy-
-- Wert zu erzwingen. Die uebrigen drei Felder bekommen Defaults, die zum
-- bereits vorhandenen Datenbestand passen (SKR04-Kontenrahmen-Seed in
-- 017_accounting_basics.sql, 4-stellige Kontonummern, deutsches
-- Kalenderjahr als mit Abstand haeufigster Wirtschaftsjahresbeginn).

ALTER TABLE company_profiles
    ADD COLUMN IF NOT EXISTS datev_berater_nr integer,
    ADD COLUMN IF NOT EXISTS datev_mandant_nr integer,
    ADD COLUMN IF NOT EXISTS datev_skr text NOT NULL DEFAULT '04',
    ADD COLUMN IF NOT EXISTS datev_sachkontenlaenge smallint NOT NULL DEFAULT 4,
    ADD COLUMN IF NOT EXISTS datev_fiscal_year_start_month smallint NOT NULL DEFAULT 1
        CHECK (datev_fiscal_year_start_month BETWEEN 1 AND 12);

-- Neue Permission fuer den DATEV-Export (analog zum Muster aus
-- 065_bank_permissions.sql/081_cost_centers.sql): keine bestehende
-- Permission passt - voller Ledger-Export ist eine eigene, sensible
-- Faehigkeit, keine Wiederverwendung von generischem accounts-/bank-
-- Lesezugriff.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-datev-export', 'datev.export', 'DATEV-Export', 'Buchungsstapel im DATEV-EXTF-Format exportieren', 'finance')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-finance', p.id
FROM permissions p
WHERE p.code = 'datev.export'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code = 'datev.export'
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id = 'perm-datev-export';
--   DELETE FROM permissions WHERE id = 'perm-datev-export';
--   ALTER TABLE company_profiles
--       DROP COLUMN IF EXISTS datev_berater_nr,
--       DROP COLUMN IF EXISTS datev_mandant_nr,
--       DROP COLUMN IF EXISTS datev_skr,
--       DROP COLUMN IF EXISTS datev_sachkontenlaenge,
--       DROP COLUMN IF EXISTS datev_fiscal_year_start_month;
-- DATENVERLUSTRISIKO: Sobald ein Mandant Beraternummer/Mandantennummer/
-- abweichenden Kontenrahmen/abweichende Sachkontenlaenge/abweichenden
-- Wirtschaftsjahresbeginn gepflegt hat, gehen diese Werte bei einem
-- Downgrade unwiderruflich verloren. Kein Risiko fuer bestehende
-- company_profiles-Kerndaten (Name/Adresse/Bankverbindung usw.) - diese
-- bleiben inhaltlich unangetastet.
