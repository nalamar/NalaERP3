-- Montagetermine (ADR 0038 Frage 6 (a), (b) vorbereitet, Backlog H.3.6a).
--
-- production_assembly_appointments: ein Termin fuer den Einbau auf der
-- Baustelle, Zeitraum von-bis (Tage, beide einschliesslich).
--   - project_id ist Pflicht, order_id optional: in (a) verlangt der
--     Service den Fertigungsauftrag und uebernimmt dessen Projekt; fuer
--     (b) (Termin fuer ein Projekt oder mehrere Auftraege) entfaellt nur
--     diese Pflicht im Service, ohne Migration. Mehrere Termine je
--     Auftrag sind erlaubt (Montage in Abschnitten).
--   - team_id und vehicle_id optional und SET NULL: Teams lassen sich
--     loeschen (DELETE /employees/teams/{id}) - der Termin bleibt dann
--     ohne Team stehen statt die Loeschung zu blockieren.
--   - baustelle als Freitext (keine Adressverwaltung am Projekt).
--   - Storno statt Loeschen, nur vollstaendig mit Begruendung (Muster
--     der Entnahmen, 113).
-- Hinweise (Team doppelt, Abwesenheit, Fahrzeug gesperrt, vor
-- Fertigstellung, nach Liefertermin) sind keine Sperren und werden nicht
-- gespeichert, sondern berechnet (H.3.6c).

CREATE TABLE IF NOT EXISTS production_assembly_appointments (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    order_id uuid REFERENCES production_orders(id) ON DELETE RESTRICT,
    von date NOT NULL,
    bis date NOT NULL,
    team_id uuid REFERENCES hr_teams(id) ON DELETE SET NULL,
    vehicle_id uuid REFERENCES fleet_vehicles(id) ON DELETE SET NULL,
    baustelle text NOT NULL DEFAULT '',
    notiz text NOT NULL DEFAULT '',
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    -- Zeitraum: bis nicht vor von, hoechstens 60 Tage (wie die Regeln).
    CONSTRAINT production_assembly_appointments_zeitraum_check CHECK (bis >= von AND bis - von <= 59),
    -- Storno nur vollstaendig: Zeitpunkt und Begruendung.
    CONSTRAINT production_assembly_appointments_storno_check CHECK (
        (storniert_at IS NULL AND storno_reason = '' AND storniert_by_user_id IS NULL)
        OR (storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
    )
);

CREATE INDEX IF NOT EXISTS idx_production_assembly_appointments_company_von
    ON production_assembly_appointments(company_id, von);
CREATE INDEX IF NOT EXISTS idx_production_assembly_appointments_team
    ON production_assembly_appointments(team_id, von) WHERE storniert_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_production_assembly_appointments_order ON production_assembly_appointments(order_id);
CREATE INDEX IF NOT EXISTS idx_production_assembly_appointments_project ON production_assembly_appointments(project_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS production_assembly_appointments;
-- DATENVERLUSTRISIKO (MITTEL): verloren gehen alle Montagetermine mit
-- Zeitraum, Team, Fahrzeug, Baustelle, Notiz und Stornos. Projekte,
-- Auftraege, Teams und Fahrzeuge bleiben unberuehrt.
