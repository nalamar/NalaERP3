-- Backlog 0.37: BankService (Bankabgleich/-matching) war an keinen
-- HTTP-Handler angebunden. Beim Ergaenzen der Routen (POST/GET
-- /bank-statements, POST /bank-statements/{id}/match) neue Permissions
-- nach demselben Muster wie 030_accounting_permissions.sql ergaenzt.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-bank-read', 'bank.read', 'Bankabgleich lesen', 'Kontoauszuege und Zahlungsabgleich anzeigen', 'finance'),
  ('perm-bank-write', 'bank.write', 'Bankabgleich schreiben', 'Kontoauszuege importieren und mit Rechnungen abgleichen', 'finance')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-finance', p.id
FROM permissions p
WHERE p.code IN ('bank.read', 'bank.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code IN ('bank.read', 'bank.write')
ON CONFLICT DO NOTHING;
