-- Rolle Produktion darf die Lagerliste lesen (Backlog H.1.7, offene
-- Frage 9): bei der Freigabe eines Fertigungsauftrags waehlt sie das
-- Lager, aus dem reserviert wird (H.1.5). warehouses.read oeffnet nur
-- GET /warehouses und GET /warehouses/{id}/locations; Lager anlegen
-- (warehouses.write) und Bewegungen buchen bleiben bei der Rolle Lager.

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id = 'role-production'
  AND p.code = 'warehouses.read'
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions
--    WHERE role_id = 'role-production'
--      AND permission_id IN (SELECT id FROM permissions WHERE code = 'warehouses.read');
-- DATENVERLUSTRISIKO (NIEDRIG): es gehen keine Daten verloren, nur das
-- Recht. Achtung: das DOWN entfernt auch eine Zuordnung, die ein
-- Administrator vor dieser Migration selbst angelegt hatte. Danach gibt
-- die Rolle Produktion wieder ohne Reservierung frei.
