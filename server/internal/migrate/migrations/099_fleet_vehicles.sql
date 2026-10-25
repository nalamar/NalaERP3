-- Fuhrpark: Fahrzeugstamm (ADR 0032, Backlog G.1.2). Eine Tabelle und drei
-- Permissions. Termine (G.2), Fahrerzuordnung (G.3a), Fahrtenbuch (G.3c)
-- und Kosten (G.3d) folgen mit eigenen Migrationen.
--
-- Nur zulassungspflichtige Fahrzeuge und Anhaenger (Entscheidung des
-- Nutzers vom 2026-10-25). Stapler, Hubarbeitsbuehnen und andere
-- Maschinen ohne Zulassung bleiben Betriebsmittel (assets, 089).
--
-- Eigene company_id: mandantenweite Stammdaten wie assets.
--
-- Kennzeichen: gespeichert in Grossbuchstaben mit einfachen Leerzeichen
-- (der Service normalisiert). Eindeutig je Mandant nur unter den NICHT
-- abgemeldeten Fahrzeugen - nach einer Abmeldung kann dasselbe
-- Kennzeichen wieder vergeben werden, das alte Fahrzeug behaelt es in
-- seiner Historie.
--
-- FIN (Fahrzeug-Identifizierungsnummer, ISO 3779): optional, eindeutig je
-- Mandant - sie gehoert zum Fahrzeug und wechselt nicht.
--
-- Es gibt KEINE Status-Spalte: "abgemeldet" folgt aus abgemeldet_am,
-- "gesperrt"/"Termin ueberfaellig" kommen aus den Terminen (G.2).
--
-- Fahrzeuge werden nicht geloescht, sondern abgemeldet (Datum und Grund,
-- nur gemeinsam). Spaetere Tabellen verweisen mit RESTRICT darauf.
CREATE TABLE IF NOT EXISTS fleet_vehicles (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    kennzeichen text NOT NULL,
    art text NOT NULL,
    bezeichnung text NOT NULL DEFAULT '',
    hersteller text NOT NULL DEFAULT '',
    modell text NOT NULL DEFAULT '',
    fin text,
    erstzulassung date,
    zulaessige_gesamtmasse_kg integer,
    standort text NOT NULL DEFAULT '',
    kostenstelle_id text REFERENCES cost_centers(id) ON DELETE SET NULL,
    fahrtenbuch boolean NOT NULL DEFAULT false,
    notiz text NOT NULL DEFAULT '',
    abgemeldet_am date,
    abgemeldet_grund text NOT NULL DEFAULT '',
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fleet_vehicles_kennzeichen_nonempty CHECK (btrim(kennzeichen) <> ''),
    CONSTRAINT fleet_vehicles_art_check CHECK (art IN ('pkw', 'transporter', 'lkw', 'anhaenger')),
    CONSTRAINT fleet_vehicles_masse_check CHECK (zulaessige_gesamtmasse_kg IS NULL OR zulaessige_gesamtmasse_kg > 0),
    CONSTRAINT fleet_vehicles_abmeldung_check CHECK ((abgemeldet_am IS NULL) = (btrim(abgemeldet_grund) = '')),
    CONSTRAINT fleet_vehicles_company_fin_key UNIQUE (company_id, fin)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_fleet_vehicles_company_kennzeichen_aktiv
    ON fleet_vehicles(company_id, kennzeichen) WHERE abgemeldet_am IS NULL;

COMMENT ON TABLE fleet_vehicles IS
    'Fahrzeuge und Anhaenger je Mandant (ADR 0032). Abmelden statt loeschen; Status abgeleitet.';

-- Permissions.
--
-- fleet.self an ALLE Systemrollen: eigene Fahrzeugzuordnungen und Fahrten
-- sehen bzw. erfassen (ab G.3) darf jeder.
--
-- read/write NUR an Fuhrpark und Admin (Entscheidung des Nutzers vom
-- 2026-10-25), NICHT an die Personalabteilung: das Fahrtenbuch enthaelt
-- Bewegungsdaten.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-fleet-self', 'fleet.self', 'Eigene Fahrzeuge', 'Mir zugeordnete Fahrzeuge und eigene Fahrten anzeigen und erfassen', 'fleet'),
  ('perm-fleet-read', 'fleet.read', 'Fuhrpark lesen', 'Fahrzeuge, Termine, Zuordnungen, Fahrtenbuecher und Kosten anzeigen', 'fleet'),
  ('perm-fleet-write', 'fleet.write', 'Fuhrpark schreiben', 'Fahrzeuge pflegen und abmelden, Termine, Zuordnungen und Kosten erfassen, stornieren', 'fleet')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-sales', 'role-procurement', 'role-inventory', 'role-finance', 'role-hr', 'role-production', 'role-fleet')
  AND p.code = 'fleet.self'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-fleet')
  AND p.code IN ('fleet.read', 'fleet.write')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('fleet.self', 'fleet.read', 'fleet.write'));
--   DELETE FROM permissions WHERE code IN ('fleet.self', 'fleet.read', 'fleet.write');
--   DROP TABLE IF EXISTS fleet_vehicles;
-- DATENVERLUSTRISIKO (MITTEL): der gepflegte Fahrzeugstamm samt
-- Abmeldungen geht verloren. Keine andere Tabelle verweist darauf (Stand
-- G.1.2); spaetere Migrationen (Termine, Fahrtenbuch) muessen vorher
-- zurueckgenommen werden.
