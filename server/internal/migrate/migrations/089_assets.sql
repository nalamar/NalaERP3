-- Betriebsmittel: Werkzeug, Leitern, IT, PSA (ADR 0028, Backlog F.3.2).
-- Drei Tabellen und drei Permissions:
--   - assets: der Bestand an EINZELSTUECKEN mit Inventarnummer, je Mandant.
--   - asset_assignments: Ausgabe an einen Mitarbeiter und Rueckgabe.
--     Unveraenderlich bis auf die einmalige Rueckgabe und das Storno.
--   - asset_inspections: wiederkehrende Pruefungen (DGUV Vorschrift 3,
--     Leitern, PSA gegen Absturz). Unveraenderlich, Fehler ueber Storno.
--
-- Verbrauchs-PSA (Handschuhe, Brillen, Schuhe - ohne Rueckgabe, ohne
-- Inventarnummer) gehoert NICHT hierher (Entscheidung des Nutzers vom
-- 2026-10-15), sondern spaeter ins Lager.
--
-- Nicht verwechseln: project_assets (016) sind Projekt-DATEIEN.

-- 1) Bestand.
--
-- Eigene company_id: mandantenweite Stammdaten wie cost_centers.
-- Inventarnummer eindeutig je Mandant.
--
-- Die Kategorie ist eine feste, grobe Liste (Gruppierung, Vorschlag eines
-- Pruefintervalls) - kein Katalog je Mandant wie bei den Qualifikationen.
-- Fahrzeuge gehoeren zum Fuhrpark (Epic G) und fehlen bewusst.
--
-- Es gibt KEINE Status-Spalte. "ausgegeben" folgt aus einer offenen
-- Ausgabe, "ausgemustert" aus ausgemustert_am, "gesperrt"/"ueberfaellig"
-- aus den Pruefungen. Eine gespeicherte Kopie davon koennte nur
-- auseinanderlaufen.
--
-- pruefintervall_monate NULL = keine wiederkehrende Pruefung (z. B. ein
-- Laptop). 0 oder negativ ist ausgeschlossen, wie gueltigkeit_monate in 088.
--
-- Assets werden nicht geloescht, sondern ausgemustert (Datum und Grund,
-- nur gemeinsam). Ausgaben und Pruefungen verweisen mit RESTRICT darauf.
--
-- Kostenstelle mit ON DELETE SET NULL wie projects.kostenstelle_id (082):
-- sie ist eine Zuordnung, kein Beleg.
CREATE TABLE IF NOT EXISTS assets (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    inventarnummer text NOT NULL,
    bezeichnung text NOT NULL,
    kategorie text NOT NULL,
    seriennummer text NOT NULL DEFAULT '',
    hersteller text NOT NULL DEFAULT '',
    modell text NOT NULL DEFAULT '',
    standort text NOT NULL DEFAULT '',
    anschaffungsdatum date,
    anschaffungspreis numeric(18,2),
    kostenstelle_id text REFERENCES cost_centers(id) ON DELETE SET NULL,
    pruefintervall_monate integer,
    notiz text NOT NULL DEFAULT '',
    ausgemustert_am date,
    ausgemustert_grund text NOT NULL DEFAULT '',
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT assets_company_inventarnummer_key UNIQUE (company_id, inventarnummer),
    CONSTRAINT assets_inventarnummer_nonempty CHECK (btrim(inventarnummer) <> ''),
    CONSTRAINT assets_bezeichnung_nonempty CHECK (btrim(bezeichnung) <> ''),
    CONSTRAINT assets_kategorie_check
        CHECK (kategorie IN ('werkzeug', 'elektrowerkzeug', 'messmittel', 'leiter', 'psa', 'it', 'maschine', 'sonstiges')),
    CONSTRAINT assets_anschaffungspreis_nonnegative
        CHECK (anschaffungspreis IS NULL OR anschaffungspreis >= 0),
    CONSTRAINT assets_pruefintervall_positive
        CHECK (pruefintervall_monate IS NULL OR pruefintervall_monate > 0),
    CONSTRAINT assets_ausmusterung_consistent
        CHECK (
            (ausgemustert_am IS NULL AND ausgemustert_grund = '')
            OR (ausgemustert_am IS NOT NULL AND btrim(ausgemustert_grund) <> '')
        )
);

CREATE INDEX IF NOT EXISTS idx_assets_company_kategorie ON assets(company_id, kategorie);
CREATE INDEX IF NOT EXISTS idx_assets_kostenstelle ON assets(kostenstelle_id);

COMMENT ON TABLE assets IS
    'Betriebsmittel als Einzelstuecke mit Inventarnummer je Mandant (ADR 0028). Ausgemustert statt geloescht; Status wird abgeleitet, nicht gespeichert.';
COMMENT ON COLUMN assets.pruefintervall_monate IS
    'Abstand der wiederkehrenden Pruefung in Monaten. NULL = keine wiederkehrende Pruefung.';

-- 2) Ausgaben.
--
-- Kein eigenes company_id: Scope ueber asset_id. Dass Asset und
-- Mitarbeiter zum SELBEN Mandanten gehoeren, prueft der Service (wie in
-- 088 - hr_employees.company_id ist noch nullable).
--
-- Muster der Zeiteintraege (086): eine Ausgabe wird nicht geaendert und
-- nicht geloescht. Erlaubt sind genau zwei einmalige Uebergaenge -
-- Rueckgabe (zurueckgegeben_am, Zustand, wer zurueckgenommen hat) und
-- Storno. Die Historie "wer hatte die Leiter am Unfalltag?" bleibt
-- vollstaendig. Beide Fremdschluessel RESTRICT.
--
-- Tagesgenau (date): an einem Tag kann ein Stueck zurueckgegeben und an
-- den naechsten ausgegeben werden.
--
-- Keine Empfangsbestaetigung durch den Mitarbeiter (Entscheidung des
-- Nutzers vom 2026-10-15).
CREATE TABLE IF NOT EXISTS asset_assignments (
    id uuid PRIMARY KEY,
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE RESTRICT,
    ausgegeben_am date NOT NULL,
    zurueckgegeben_am date,
    rueckgabe_zustand text NOT NULL DEFAULT '',
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ausgegeben_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    zurueckgenommen_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_assignments_status_check
        CHECK (status IN ('active', 'storniert')),
    CONSTRAINT asset_assignments_return_after_issue
        CHECK (zurueckgegeben_am IS NULL OR zurueckgegeben_am >= ausgegeben_am),
    -- Ein Rueckgabezustand ohne Rueckgabe waere widerspruechlich.
    -- zurueckgenommen_by_user_id ist bewusst nicht Teil des Checks (faellt
    -- per ON DELETE SET NULL auf NULL).
    CONSTRAINT asset_assignments_zustand_only_when_returned
        CHECK (zurueckgegeben_am IS NOT NULL OR rueckgabe_zustand = ''),
    CONSTRAINT asset_assignments_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

-- Hoechstens eine offene Ausgabe je Asset - von der Datenbank garantiert,
-- nicht nur vom Code (zwei gleichzeitige Ausgaben koennten eine reine
-- Vorabpruefung beide passieren). Muster: idx_hr_time_entries_one_open.
CREATE UNIQUE INDEX IF NOT EXISTS idx_asset_assignments_one_open
    ON asset_assignments(asset_id) WHERE zurueckgegeben_am IS NULL AND status = 'active';

CREATE INDEX IF NOT EXISTS idx_asset_assignments_asset_date ON asset_assignments(asset_id, ausgegeben_am DESC);
CREATE INDEX IF NOT EXISTS idx_asset_assignments_employee ON asset_assignments(employee_id);

COMMENT ON TABLE asset_assignments IS
    'Ausgabe eines Betriebsmittels an einen Mitarbeiter (ADR 0028). Unveraenderlich bis auf einmalige Rueckgabe und Storno; hoechstens eine offene Ausgabe je Asset.';
COMMENT ON COLUMN asset_assignments.zurueckgegeben_am IS
    'NULL = der Mitarbeiter hat das Stueck noch.';

-- 3) Pruefungen.
--
-- Wie die Nachweise in 088: naechste_pruefung_am wird beim Erfassen
-- FESTGELEGT und gespeichert (angegeben oder aus geprueft_am plus
-- Pruefintervall des Assets berechnet). Aendert der Betrieb spaeter das
-- Intervall, gilt eine erfolgte Pruefung nicht rueckwirkend anders.
-- NULL = keine Folgepruefung vorgesehen.
--
-- ergebnis 'nicht_bestanden' sperrt das Asset fuer die Ausgabe (ADR 0028,
-- Entscheidung des Nutzers vom 2026-10-15) - das wertet der Service aus,
-- die Datenbank haelt nur das Ergebnis fest.
--
-- pruefer_name ist Pflicht: die Pruefung erfolgt durch eine befaehigte
-- Person, und wer das war, ist Teil der Aussage. Optional zusaetzlich der
-- Mitarbeiter, wenn intern geprueft wurde (RESTRICT wie bei den Ausgaben).
CREATE TABLE IF NOT EXISTS asset_inspections (
    id uuid PRIMARY KEY,
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    geprueft_am date NOT NULL,
    ergebnis text NOT NULL,
    naechste_pruefung_am date,
    pruefer_name text NOT NULL,
    pruefer_employee_id uuid REFERENCES hr_employees(id) ON DELETE RESTRICT,
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_inspections_ergebnis_check
        CHECK (ergebnis IN ('bestanden', 'maengel', 'nicht_bestanden')),
    CONSTRAINT asset_inspections_naechste_after_geprueft
        CHECK (naechste_pruefung_am IS NULL OR naechste_pruefung_am > geprueft_am),
    CONSTRAINT asset_inspections_pruefer_nonempty CHECK (btrim(pruefer_name) <> ''),
    CONSTRAINT asset_inspections_status_check
        CHECK (status IN ('active', 'storniert')),
    CONSTRAINT asset_inspections_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

CREATE INDEX IF NOT EXISTS idx_asset_inspections_asset_date ON asset_inspections(asset_id, geprueft_am DESC);
CREATE INDEX IF NOT EXISTS idx_asset_inspections_pruefer ON asset_inspections(pruefer_employee_id);
-- Fuer die Liste "was ist faellig": nur aktive Pruefungen mit Folgetermin.
CREATE INDEX IF NOT EXISTS idx_asset_inspections_naechste
    ON asset_inspections(naechste_pruefung_am) WHERE status = 'active' AND naechste_pruefung_am IS NOT NULL;

COMMENT ON TABLE asset_inspections IS
    'Wiederkehrende Pruefungen von Betriebsmitteln (ADR 0028). Unveraenderlich, Fehler nur ueber Storno.';
COMMENT ON COLUMN asset_inspections.naechste_pruefung_am IS
    'Faelligkeit der naechsten Pruefung. Beim Erfassen festgelegt und gespeichert. NULL = keine Folgepruefung.';

-- 4) Permissions.
--
-- assets.self an ALLE Systemrollen: sehen, was man selbst ausgegeben
-- bekommen hat, darf jeder.
--
-- read/write NUR an HR und Admin (Entscheidung des Nutzers vom
-- 2026-10-15: "Erstmal nur Personalabteilung, spaeter eventuell dem Lager
-- das Recht aber einraeumen koennen"). Admin wie in allen bisherigen
-- HR-Migrationen; er hat ohnehin admin.superuser. Das Lager spaeter
-- aufzunehmen ist eine Zuordnung in role_permissions - es gibt bisher
-- keine Oberflaeche dafuer, also eine eigene kleine Migration.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-assets-self', 'assets.self', 'Eigene Betriebsmittel', 'An mich ausgegebene Betriebsmittel anzeigen', 'assets'),
  ('perm-assets-read', 'assets.read', 'Betriebsmittel lesen', 'Bestand, Ausgaben, Pruefungen und Faelligkeiten aller Betriebsmittel anzeigen', 'assets'),
  ('perm-assets-write', 'assets.write', 'Betriebsmittel schreiben', 'Betriebsmittel pflegen und ausmustern, ausgeben und zuruecknehmen, Pruefungen erfassen, stornieren', 'assets')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-sales', 'role-procurement', 'role-inventory', 'role-finance', 'role-hr', 'role-production', 'role-fleet')
  AND p.code = 'assets.self'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-hr')
  AND p.code IN ('assets.read', 'assets.write')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('assets.self', 'assets.read', 'assets.write'));
--   DELETE FROM permissions WHERE code IN ('assets.self', 'assets.read', 'assets.write');
--   DROP TABLE IF EXISTS asset_inspections;
--   DROP TABLE IF EXISTS asset_assignments;
--   DROP TABLE IF EXISTS assets;
-- DATENVERLUSTRISIKO (HOCH): Mit asset_inspections gehen saemtliche
-- Pruefnachweise der Betriebsmittel (DGUV Vorschrift 3, Leitern, PSA gegen
-- Absturz) samt Storno-Historie verloren - Aufzeichnungen mit
-- Beweisfunktion gegenueber Berufsgenossenschaft und Aufsicht. Mit
-- asset_assignments geht die Historie verloren, wer welches Stueck wann
-- hatte. Ein Downgrade nach produktiver Nutzung ist ohne vorherigen Export
-- nicht zulaessig. Mitarbeiterstamm, Kostenstellen, Benutzer und alle
-- uebrigen Tabellen bleiben unberuehrt: keine andere Tabelle verweist auf
-- die drei.
