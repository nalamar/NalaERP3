-- Storno-Konzept fuer invoices_out/journal_entries (Task 0.3.1, GoBD-Fundament).
--
-- GoBD-Grundprinzip: ein einmal gebuchter Beleg (bzw. dessen Journalbuchung)
-- darf nachtraeglich weder geloescht noch inhaltlich veraendert werden. Eine
-- Korrektur erfolgt stattdessen ueber eine vollstaendige Umkehrbuchung
-- (Storno) als NEUE journal_entries-Zeile - die urspruengliche Buchung
-- bleibt unangetastet bestehen. invoices_out bekommt dafuer:
--   - storno_journal_entry_id: verweist auf die neu erzeugte Umkehrbuchung
--     (getrennt von journal_entry_id, das weiterhin auf die urspruengliche
--     Buchung zeigt - beide bleiben nachvollziehbar erhalten)
--   - storniert_am / storno_grund: Zeitpunkt und dokumentierte Begruendung,
--     GoBD verlangt eine nachvollziehbare Begruendung fuer jede Korrektur
--
-- Kein neuer CHECK-Constraint auf status: die Spalte war schon immer
-- freier Text (siehe 018_journal_and_ar.sql), der neue Wert 'storniert'
-- wird ausschliesslich im Anwendungscode (ARService.Storno) durchgesetzt.

ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS storno_journal_entry_id UUID REFERENCES journal_entries(id),
    ADD COLUMN IF NOT EXISTS storniert_am timestamptz,
    ADD COLUMN IF NOT EXISTS storno_grund text NOT NULL DEFAULT '';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE invoices_out DROP COLUMN IF EXISTS storno_grund;
--   ALTER TABLE invoices_out DROP COLUMN IF EXISTS storniert_am;
--   ALTER TABLE invoices_out DROP COLUMN IF EXISTS storno_journal_entry_id;
-- DATENVERLUSTRISIKO: Sobald Rechnungen real storniert wurden, geht bei
-- einem Downgrade die Dokumentation von Stornogrund/-zeitpunkt verloren
-- (GoBD-relevant: der Nachweis der Korrekturbegruendung waere dann nicht
-- mehr rekonstruierbar, die Umkehrbuchung selbst bliebe aber als normale
-- journal_entries-Zeile erhalten).
