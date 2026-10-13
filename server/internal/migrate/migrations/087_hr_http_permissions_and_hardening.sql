-- HR-Bestand an HTTP anbinden (ADR 0025, Backlog F.4.2). Drei Dinge, die
-- vor der Anbindung von Mitarbeiterstamm und Urlaubsantraegen im Schema
-- stimmen muessen:
--   1) Personalnummer eindeutig JE MANDANT statt global.
--   2) Wer einen Urlaubsantrag entschieden hat, als Login festhalten.
--   3) Fuenf Permissions.

-- 1) Personalnummer je Mandant.
--
-- 020_hr_base.sql hat personalnummer als "TEXT UNIQUE" angelegt - lange
-- vor dem Mandanten-Scoping (059). Seitdem ist die Eindeutigkeit GLOBAL:
-- zwei Mandanten koennen nicht beide die Nummer "1001" vergeben, und ein
-- Mandant erfaehrt ueber die Ablehnung, welche Nummern ein anderer nutzt.
--
-- COALESCE(company_id, ''): company_id ist noch nullable (Expand-Contract
-- aus 059). In einem gewoehnlichen UNIQUE-Index gelten NULLs als
-- verschieden - Zeilen ohne Mandant waeren untereinander gar nicht mehr
-- eindeutig. Der Ausdruck behandelt "kein Mandant" als EINEN Mandanten;
-- fuer solche Altzeilen aendert sich damit nichts gegenueber vorher.
--
-- WHERE personalnummer IS NOT NULL: mehrere Mitarbeiter ohne
-- Personalnummer bleiben zulaessig, wie bisher.
--
-- Fuer bestehende Daten ungefaehrlich: was global eindeutig war, ist es
-- auch je Mandant. Der neue Index kann an Bestandsdaten nicht scheitern.
ALTER TABLE hr_employees DROP CONSTRAINT IF EXISTS hr_employees_personalnummer_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_employees_company_personalnummer
    ON hr_employees (COALESCE(company_id, ''), personalnummer)
    WHERE personalnummer IS NOT NULL;

-- 2) Entscheider eines Urlaubsantrags.
--
-- approver_id (020) verweist auf einen MITARBEITER. Wer entscheidet, muss
-- damit selbst einen Mitarbeiterdatensatz haben - ein Administrator ohne
-- einen solchen koennte nicht entscheiden, und der Nachweis hinge an einer
-- Stammdatenzeile statt am Login, der die Aktion ausgeloest hat.
-- decided_by_user_id haelt den Login fest, wie created_by_user_id /
-- storniert_by_user_id bei den Zeiteintraegen (086). approver_id bleibt
-- bestehen und wird weiter gefuellt, wenn der Entscheider einen
-- verknuepften Mitarbeiter hat.
--
-- Nullable: bestehende, bereits entschiedene Antraege haben die Angabe
-- nicht, und ein Wert liesse sich fuer sie nur erfinden.
-- ON DELETE SET NULL: das Loeschen eines Logins darf den Antrag nicht
-- mitreissen.
ALTER TABLE hr_leave_requests
    ADD COLUMN IF NOT EXISTS decided_by_user_id text REFERENCES users(id) ON DELETE SET NULL;

COMMENT ON COLUMN hr_leave_requests.decided_by_user_id IS
    'Login, der den Antrag genehmigt oder abgelehnt hat (ADR 0025). NULL bei offenen Antraegen und bei Entscheidungen vor Migration 087.';

-- 3) Permissions.
--
-- employees.read/write bewusst NUR an HR und Admin: der Mitarbeiterstamm
-- enthaelt personenbezogene Daten, Lesezugriff braucht einen Zweck. Ein
-- Modul, das spaeter Namen anzeigen muss, bekommt dafuer einen eigenen,
-- schmalen Lesepfad - nicht dieses Recht.
--
-- leave_requests.self an ALLE Systemrollen (eigene Antraege stellen und
-- lesen kann jeder, wie time_entries.self); read/decide an HR und Admin.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-employees-read', 'employees.read', 'Mitarbeiter lesen', 'Mitarbeiterstamm und Teams anzeigen', 'hr'),
  ('perm-employees-write', 'employees.write', 'Mitarbeiter schreiben', 'Mitarbeiter anlegen, aendern und mit Benutzern verknuepfen', 'hr'),
  ('perm-leave-requests-self', 'leave_requests.self', 'Eigene Urlaubsantraege', 'Eigene Urlaubsantraege stellen und anzeigen', 'hr'),
  ('perm-leave-requests-read', 'leave_requests.read', 'Urlaubsantraege lesen', 'Urlaubsantraege aller Mitarbeiter anzeigen', 'hr'),
  ('perm-leave-requests-decide', 'leave_requests.decide', 'Urlaubsantraege entscheiden', 'Urlaubsantraege genehmigen und ablehnen', 'hr')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-sales', 'role-procurement', 'role-inventory', 'role-finance', 'role-hr', 'role-production', 'role-fleet')
  AND p.code = 'leave_requests.self'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-hr')
  AND p.code IN ('employees.read', 'employees.write', 'leave_requests.read', 'leave_requests.decide')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('employees.read', 'employees.write', 'leave_requests.self', 'leave_requests.read', 'leave_requests.decide'));
--   DELETE FROM permissions WHERE code IN ('employees.read', 'employees.write', 'leave_requests.self', 'leave_requests.read', 'leave_requests.decide');
--   ALTER TABLE hr_leave_requests DROP COLUMN IF EXISTS decided_by_user_id;
--   DROP INDEX IF EXISTS idx_hr_employees_company_personalnummer;
--   ALTER TABLE hr_employees ADD CONSTRAINT hr_employees_personalnummer_key UNIQUE (personalnummer);
-- DATENVERLUSTRISIKO: Mit decided_by_user_id geht der Nachweis verloren,
-- welcher Login einen Urlaubsantrag entschieden hat (die Entscheidung
-- selbst, ihr Zeitpunkt und approver_id bleiben erhalten).
-- DOWNGRADE KANN SCHEITERN: Die letzte Anweisung stellt die GLOBALE
-- Eindeutigkeit wieder her. Haben zwei Mandanten inzwischen dieselbe
-- Personalnummer vergeben (genau das erlaubt diese Migration), schlaegt
-- sie fehl. Dann muessen die Nummern vor dem Downgrade bereinigt werden -
-- das ist eine fachliche Entscheidung, keine technische. Da die
-- Anweisungen einzeln ausgefuehrt werden, sind bei einem solchen Abbruch
-- die Schritte davor bereits wirksam.
