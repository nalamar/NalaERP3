-- Subtask 0.5.3: dediziertes 'admin.superuser'-Permission statt des
-- bisherigen impliziten Bypasses ueber 'users.manage' in requirePermission()
-- (server/internal/http/v1.go). 'users.manage' war als enges
-- "Benutzer/Rollen verwalten"-Recht gedacht (siehe 017_auth.sql), wurde vom
-- Code aber zusaetzlich als Universal-Bypass fuer JEDE andere Berechtigung
-- missbraucht - ein zukuenftiges "User-Admin"-Recht (z.B. aus der noch
-- ausstehenden User-Management-API, Subtask 0.5.4) haette ungewollt
-- systemweiten Vollzugriff erhalten.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-admin-superuser', 'admin.superuser', 'Systemweiter Vollzugriff', 'Uneingeschraenkter Zugriff auf alle Module (Notfall-/Wartungsrecht)', 'platform')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code = 'admin.superuser'
ON CONFLICT DO NOTHING;
