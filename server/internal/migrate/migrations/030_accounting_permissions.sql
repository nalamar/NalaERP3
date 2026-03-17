INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-invoices-out-read', 'invoices_out.read', 'Ausgangsrechnungen lesen', 'Ausgangsrechnungen anzeigen', 'finance'),
  ('perm-invoices-out-write', 'invoices_out.write', 'Ausgangsrechnungen schreiben', 'Ausgangsrechnungen bearbeiten und Zahlungen buchen', 'finance')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-finance', p.id
FROM permissions p
WHERE p.code IN ('invoices_out.read', 'invoices_out.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code IN ('invoices_out.read', 'invoices_out.write')
ON CONFLICT DO NOTHING;
