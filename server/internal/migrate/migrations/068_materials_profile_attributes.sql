-- Metallbau-spezifisches Artikel-/Profilattributschema (ADR 0006, Backlog
-- A.1.2). Vier neue, optionale Spalten direkt auf materials, konsistent mit
-- dem bestehenden Muster (kategorie, dichte, length_mm/width_mm/height_mm
-- sind ebenfalls eigene, nullable Spalten statt der generischen
-- attributes-jsonb-Spalte). Reine Additiv-Migration, kein Backfill noetig
-- (bestehende Zeilen erhalten NULL).
--
-- Validierung (RC-Klasse-Enum gegen DIN EN 1627, U-Wert-Plausibilitaet)
-- erfolgt bewusst NICHT hier per CHECK-Constraint, sondern im
-- Anwendungscode (Backlog A.1.3), analog zum bestehenden Muster fuer
-- kategorie (normalizeAndValidateCategory in server/internal/materials/service.go).
-- profilserie und brandschutzklasse bleiben laut ADR 0006 bewusst freier
-- Text ohne Enum (Begruendung dort).

ALTER TABLE materials
    ADD COLUMN IF NOT EXISTS profilserie text,
    ADD COLUMN IF NOT EXISTS rc_klasse text,
    ADD COLUMN IF NOT EXISTS u_wert numeric(6,3),
    ADD COLUMN IF NOT EXISTS brandschutzklasse text;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE materials
--       DROP COLUMN IF EXISTS profilserie,
--       DROP COLUMN IF EXISTS rc_klasse,
--       DROP COLUMN IF EXISTS u_wert,
--       DROP COLUMN IF EXISTS brandschutzklasse;
-- DATENVERLUSTRISIKO: Sobald echte Profildaten (Profilserie, RC-Klasse,
-- U-Wert, Brandschutzklasse) erfasst wurden, gehen diese bei einem
-- Downgrade unwiderruflich verloren.
