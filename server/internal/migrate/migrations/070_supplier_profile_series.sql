-- Systemlieferanten-Konzept: Bindung Lieferant (contacts) <-> Profilserie
-- (ADR 0008, Backlog A.3.2). Keine neue Lieferantentabelle - contacts mit
-- rolle IN ('supplier','both') sind bereits die etablierte Lieferantenquelle
-- (siehe purchase_orders.supplier_id -> contacts). Keine eigene
-- Profilserie-Katalogtabelle - profilserie bleibt bewusst freier Text,
-- analog zu materials.profilserie (ADR 0006).
--
-- Kein eigenes company_id - Scope wird ueber contact_id -> contacts.company_id
-- geerbt (Muster wie contact_addresses/contact_persons, ADR 0002).
--
-- Die fachliche Regel "nur Kontakte mit rolle IN ('supplier','both') duerfen
-- gebunden werden" wird bewusst NICHT per DB-CHECK erzwungen (Cross-Table-
-- Constraints brauchen in Postgres einen Trigger, hier unverhaeltnismaessig)
-- - Pruefung erfolgt im Anwendungscode (Backlog A.3.3).

CREATE TABLE IF NOT EXISTS supplier_profile_series (
    id text PRIMARY KEY,
    contact_id text NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    profilserie text NOT NULL,
    notiz text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_supplier_profile_series_profilserie CHECK (BTRIM(profilserie) <> ''),
    UNIQUE (contact_id, profilserie)
);
CREATE INDEX IF NOT EXISTS idx_supplier_profile_series_contact_id ON supplier_profile_series(contact_id);
CREATE INDEX IF NOT EXISTS idx_supplier_profile_series_profilserie ON supplier_profile_series(profilserie);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS supplier_profile_series;
-- DATENVERLUSTRISIKO: Sobald echte Lieferant-Profilserie-Bindungen erfasst
-- wurden, gehen diese bei einem Downgrade unwiderruflich verloren. Kein
-- Risiko fuer bereits bestehende Fachdaten anderer Domaenen (contacts selbst
-- bleibt unangetastet, nur die neue Tabelle wird entfernt).
