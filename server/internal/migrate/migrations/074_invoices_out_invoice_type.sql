-- Abschlags-/Schlussrechnung nach VOB/B SS16 (ADR 0012, Backlog B.4.2).
-- Additive Spalte statt Vermischung mit invoices_out.status (das bleibt
-- der Buchungs-/Zahlungs-/Storno-Lebenszyklus, siehe 062_invoices_out_storno.sql).
--
-- Default 'rechnung' - JEDE bereits bestehende und jede kuenftige, nicht
-- auftragsgebundene Rechnung bleibt unveraendert neutral, 100%
-- rueckwaertskompatibel, keine Datenmigration noetig.
--
-- Anders als status (dort bewusst kein CHECK, siehe Kommentar in
-- 062_invoices_out_storno.sql - historisch organisch gewachsene
-- Werteliste) bekommt invoice_type als neue Spalte mit von Anfang an
-- klar definiertem, stabilem Wertesatz einen CHECK-Constraint (Muster
-- wie die meisten anderen neuen Spalten dieser Session).
--
-- Die Geschaeftsregeln (keine weitere Rechnung nach Schlussrechnung,
-- Schlussrechnung muss vollstaendig sein) werden bewusst NICHT hier per
-- Trigger erzwungen, sondern im Anwendungscode (Backlog B.4.3,
-- sales.Service.ConvertToInvoice) - analog zum in dieser Session
-- etablierten Prinzip (z. B. ADR 0008/0009).

ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS invoice_type text NOT NULL DEFAULT 'rechnung';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_invoices_out_invoice_type'
    ) THEN
        ALTER TABLE invoices_out
            ADD CONSTRAINT chk_invoices_out_invoice_type
            CHECK (invoice_type IN ('rechnung', 'abschlagsrechnung', 'schlussrechnung'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invoices_out_invoice_type ON invoices_out(invoice_type);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE invoices_out DROP CONSTRAINT IF EXISTS chk_invoices_out_invoice_type;
--   ALTER TABLE invoices_out DROP COLUMN IF EXISTS invoice_type;
-- DATENVERLUSTRISIKO: Sobald echte Abschlags-/Schlussrechnungen als solche
-- getypt wurden, geht diese Klassifizierung bei einem Downgrade verloren
-- (die Rechnungen selbst und ihre Betraege/Buchungen bleiben unberuehrt,
-- nur die VOB-Typisierung faellt weg).
