INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-quotes-approve', 'quotes.approve', 'Angebote freigeben', 'Freigabeanforderungen fuer Angebotspositionen genehmigen oder ablehnen', 'sales')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code = 'quotes.approve'
ON CONFLICT DO NOTHING;
