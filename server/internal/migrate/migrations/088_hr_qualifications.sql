-- Qualifikationen, Unterweisungen, Beauftragungen (ADR 0027, Backlog F.2.2).
-- Drei Tabellen und drei Permissions:
--   - hr_qualification_types: der Katalog der Arten, je Mandant gepflegt
--     und erweiterbar.
--   - hr_employee_qualifications: die Nachweise - wer hat wann was
--     erworben, bis wann gilt es. Unveraenderlich, Fehler nur ueber Storno.
--   - hr_employee_qualification_requirements: welche Arten ein Mitarbeiter
--     fuer seine Stelle braucht.
--
-- Diese Migration legt KEINE Katalogeintraege an. Der Vorschlagskatalog
-- (Arbeitsschutz, Erste Hilfe, Stapler, ...) ist Code und wird je Mandant
-- durch einen ausdruecklichen Aufruf uebernommen (F.2.3) - was im Katalog
-- steht, gehoert dem Mandanten, und eine Migration soll ihm keine Daten
-- unterschieben.

-- 1) Katalog.
--
-- Eigene company_id: mandantenweite Stammdaten wie cost_centers.
--
-- Jede Art hat EINE Kategorie und HOECHSTENS EINE Frist. Ein Thema kann
-- deshalb aus mehreren Arten bestehen: "Staplerschein" sind die Ausbildung
-- (laeuft nicht ab), die jaehrliche Unterweisung (laeuft ab) und die
-- schriftliche Beauftragung (gilt bis Widerruf). Ein Eintrag mit einer
-- Frist koennte das nicht abbilden.
--
-- gueltigkeit_monate NULL = unbefristet. 0 oder negativ waere sinnlos und
-- ist ausgeschlossen; "unbefristet" wird als NULL ausgedrueckt, nicht als 0.
--
-- pflicht_fuer_alle: gilt fuer jeden aktiven Mitarbeiter des Mandanten
-- (z. B. die allgemeine Arbeitsschutzunterweisung), ohne dass sie jedem
-- einzeln als Anforderung zugeordnet werden muss.
--
-- Arten werden nicht geloescht, sondern ueber aktiv=false stillgelegt:
-- Nachweise verweisen auf sie.
CREATE TABLE IF NOT EXISTS hr_qualification_types (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    code text NOT NULL,
    name text NOT NULL,
    kategorie text NOT NULL,
    gueltigkeit_monate integer,
    pflicht_fuer_alle boolean NOT NULL DEFAULT false,
    aktiv boolean NOT NULL DEFAULT true,
    hinweis text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT hr_qualification_types_company_code_key UNIQUE (company_id, code),
    CONSTRAINT hr_qualification_types_code_nonempty CHECK (btrim(code) <> ''),
    CONSTRAINT hr_qualification_types_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT hr_qualification_types_kategorie_check
        CHECK (kategorie IN ('qualifikation', 'unterweisung', 'beauftragung')),
    CONSTRAINT hr_qualification_types_gueltigkeit_positive
        CHECK (gueltigkeit_monate IS NULL OR gueltigkeit_monate > 0)
);

COMMENT ON TABLE hr_qualification_types IS
    'Katalog der Qualifikationen, Unterweisungen und Beauftragungen je Mandant (ADR 0027). Erweiterbar; Eintraege werden deaktiviert, nicht geloescht.';
COMMENT ON COLUMN hr_qualification_types.gueltigkeit_monate IS
    'Frist in Monaten, nach der ein Nachweis dieser Art erneuert werden muss. NULL = unbefristet.';

-- 2) Nachweise.
--
-- Kein eigenes company_id: Scope ueber employee_id, wie hr_time_entries.
-- Dass Mitarbeiter und Art zum SELBEN Mandanten gehoeren, kann die
-- Datenbank hier nicht erzwingen (hr_employees.company_id ist noch
-- nullable, es gibt keinen zusammengesetzten Schluessel) - das prueft der
-- Service.
--
-- gueltig_bis wird beim Erfassen FESTGELEGT und gespeichert (angegeben
-- oder aus erworben_am plus Frist der Art berechnet), nicht bei jedem
-- Lesen neu aus der Art abgeleitet: aendert der Betrieb spaeter die Frist
-- einer Art, sollen bereits ausgestellte Nachweise nicht rueckwirkend
-- anders gelten. NULL = unbefristet. Der Tag selbst zaehlt noch als
-- gueltig.
--
-- Unveraenderlich wie die Zeiteintraege (086): ein Nachweis hat
-- Beweisfunktion ("wurde unterwiesen, und wann?"). Kein UPDATE der
-- Angaben, kein DELETE; ein Fehler wird storniert und neu erfasst. Eine
-- Wiederholung ist ein NEUER Nachweis, der alte bleibt stehen.
--
-- Beide Fremdschluessel mit ON DELETE RESTRICT: weder das Loeschen eines
-- Mitarbeiters noch das einer Art darf Nachweise still mitnehmen.
CREATE TABLE IF NOT EXISTS hr_employee_qualifications (
    id uuid PRIMARY KEY,
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE RESTRICT,
    type_id uuid NOT NULL REFERENCES hr_qualification_types(id) ON DELETE RESTRICT,
    erworben_am date NOT NULL,
    gueltig_bis date,
    aussteller text NOT NULL DEFAULT '',
    nachweis_nr text NOT NULL DEFAULT '',
    note text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT hr_employee_qualifications_status_check
        CHECK (status IN ('active', 'storniert')),
    CONSTRAINT hr_employee_qualifications_gueltig_after_erworben
        CHECK (gueltig_bis IS NULL OR gueltig_bis >= erworben_am),
    -- Wie in 086: ein Storno ist nur vollstaendig gueltig. Der Urheber ist
    -- bewusst nicht Teil des Checks (faellt per ON DELETE SET NULL auf NULL).
    CONSTRAINT hr_employee_qualifications_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

CREATE INDEX IF NOT EXISTS idx_hr_employee_qualifications_employee_type
    ON hr_employee_qualifications(employee_id, type_id, erworben_am DESC);
CREATE INDEX IF NOT EXISTS idx_hr_employee_qualifications_type ON hr_employee_qualifications(type_id);
-- Fuer die Liste "was laeuft ab": nur aktive Nachweise mit Ablaufdatum.
CREATE INDEX IF NOT EXISTS idx_hr_employee_qualifications_gueltig_bis
    ON hr_employee_qualifications(gueltig_bis) WHERE status = 'active' AND gueltig_bis IS NOT NULL;

COMMENT ON TABLE hr_employee_qualifications IS
    'Nachweise: Mitarbeiter hat eine Art an einem Tag erworben (ADR 0027). Unveraenderlich: Fehler nur ueber Storno, Wiederholung ist ein neuer Nachweis.';
COMMENT ON COLUMN hr_employee_qualifications.gueltig_bis IS
    'Letzter Tag der Gueltigkeit (einschliesslich). Beim Erfassen festgelegt und gespeichert. NULL = unbefristet.';

-- 3) Anforderungen je Mitarbeiter.
--
-- Vorgabe des Nutzers vom 2026-10-13: beim Anlegen eines Mitarbeiters legt
-- die Personalabteilung fest, welche Qualifikationen fuer die Stelle
-- noetig sind. Erst damit kann das System sagen, dass jemandem etwas
-- FEHLT - ein Nachweis allein sagt nur, was vorhanden ist.
--
-- Das ist Planung, kein Beleg: die Zuordnung ist aenderbar und darf
-- wieder entfernt werden (die Stelle aendert sich). Deshalb CASCADE vom
-- Mitarbeiter und ein gewoehnlicher DELETE - anders als bei den
-- Nachweisen. Die Art bleibt RESTRICT: Arten werden nie geloescht.
CREATE TABLE IF NOT EXISTS hr_employee_qualification_requirements (
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    type_id uuid NOT NULL REFERENCES hr_qualification_types(id) ON DELETE RESTRICT,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (employee_id, type_id)
);

CREATE INDEX IF NOT EXISTS idx_hr_employee_qualification_requirements_type
    ON hr_employee_qualification_requirements(type_id);

COMMENT ON TABLE hr_employee_qualification_requirements IS
    'Welche Arten ein Mitarbeiter fuer seine Stelle braucht (ADR 0027). Planung, aenderbar - kein Beleg.';

-- 4) Permissions, nach dem Muster der Zeiterfassung (086).
--
-- qualifications.self an ALLE Systemrollen: den eigenen Stand sehen darf
-- jeder. Erfassen darf er ihn nicht - ein Nachweis ist eine Aussage des
-- Betriebs. read/write (alle Mitarbeiter lesen; erfassen, stornieren,
-- Anforderungen und Katalog pflegen) an HR und Admin.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-qualifications-self', 'qualifications.self', 'Eigene Qualifikationen', 'Eigene Qualifikationen, Unterweisungen und deren Faelligkeit anzeigen', 'hr'),
  ('perm-qualifications-read', 'qualifications.read', 'Qualifikationen lesen', 'Qualifikationen und Faelligkeiten aller Mitarbeiter anzeigen', 'hr'),
  ('perm-qualifications-write', 'qualifications.write', 'Qualifikationen schreiben', 'Nachweise erfassen und stornieren, Anforderungen und Katalog pflegen', 'hr')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-sales', 'role-procurement', 'role-inventory', 'role-finance', 'role-hr', 'role-production', 'role-fleet')
  AND p.code = 'qualifications.self'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-hr')
  AND p.code IN ('qualifications.read', 'qualifications.write')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('qualifications.self', 'qualifications.read', 'qualifications.write'));
--   DELETE FROM permissions WHERE code IN ('qualifications.self', 'qualifications.read', 'qualifications.write');
--   DROP TABLE IF EXISTS hr_employee_qualification_requirements;
--   DROP TABLE IF EXISTS hr_employee_qualifications;
--   DROP TABLE IF EXISTS hr_qualification_types;
-- DATENVERLUSTRISIKO (HOCH): Mit hr_employee_qualifications gehen
-- saemtliche Nachweise ueber Unterweisungen und Qualifikationen samt
-- Storno-Historie unwiderruflich verloren. Das sind Aufzeichnungen mit
-- Beweisfunktion gegenueber Berufsgenossenschaft und Aufsicht - ein
-- Downgrade nach produktiver Nutzung ist ohne vorherigen Export nicht
-- zulaessig. Zusaetzlich gehen der vom Mandanten gepflegte Katalog und
-- die Anforderungen je Mitarbeiter verloren. Mitarbeiterstamm, Benutzer
-- und alle uebrigen Tabellen bleiben unberuehrt: keine andere Tabelle
-- verweist auf die drei.
