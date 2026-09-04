-- Bedarfsermittlung aus Angebot/Mindestbestand (ADR 0016, Backlog D.1.2).
-- Eine neue, optionale Spalte direkt auf materials, konsistent mit dem
-- bestehenden Muster additiver Artikel-Stammdatenfelder (A.1.2).
--
-- Anders als bei profilserie/rc_klasse/u_wert/brandschutzklasse (A.1.2,
-- dort bewusst KEIN CHECK-Constraint, Validierung im Anwendungscode) wird
-- hier ein CHECK direkt in der Migration ergaenzt: der Wertebereich
-- (nicht-negative Zahl oder NULL) ist von Anfang an klar definiert und
-- stabil, analog zur Begruendung in B.4.2 (invoice_type-CHECK).
--
-- NULL bleibt der Default fuer alle bestehenden UND neuen Zeilen - kein
-- Mindestbestand konfiguriert heisst kein automatischer Bedarf (siehe
-- ADR 0016).

ALTER TABLE materials
    ADD COLUMN IF NOT EXISTS mindestbestand numeric(18,6);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_materials_mindestbestand_non_negative'
    ) THEN
        ALTER TABLE materials
            ADD CONSTRAINT chk_materials_mindestbestand_non_negative
            CHECK (mindestbestand IS NULL OR mindestbestand >= 0);
    END IF;
END $$;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE materials
--       DROP CONSTRAINT IF EXISTS chk_materials_mindestbestand_non_negative,
--       DROP COLUMN IF EXISTS mindestbestand;
-- DATENVERLUSTRISIKO: Sobald echte Mindestbestand-Werte konfiguriert
-- wurden, gehen diese bei einem Downgrade unwiderruflich verloren.
