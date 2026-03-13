CREATE TABLE IF NOT EXISTS users (
  id text PRIMARY KEY,
  email text NOT NULL UNIQUE,
  username text UNIQUE,
  password_hash text NOT NULL,
  first_name text NOT NULL DEFAULT '',
  last_name text NOT NULL DEFAULT '',
  display_name text NOT NULL DEFAULT '',
  locale text NOT NULL DEFAULT 'de-DE',
  timezone text NOT NULL DEFAULT 'Europe/Berlin',
  is_active boolean NOT NULL DEFAULT true,
  is_locked boolean NOT NULL DEFAULT false,
  last_login_at timestamptz,
  password_changed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles (
  id text PRIMARY KEY,
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  is_system boolean NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS permissions (
  id text PRIMARY KEY,
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  context text NOT NULL
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id text NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  assigned_by text REFERENCES users(id) ON DELETE SET NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id text NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id text NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS auth_audit_log (
  id text PRIMARY KEY,
  user_id text REFERENCES users(id) ON DELETE SET NULL,
  event_type text NOT NULL,
  ip_address text NOT NULL DEFAULT '',
  user_agent text NOT NULL DEFAULT '',
  success boolean NOT NULL DEFAULT false,
  message text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_auth_audit_log_user_created_at
  ON auth_audit_log (user_id, created_at DESC);

INSERT INTO roles (id, code, name, description, is_system)
VALUES
  ('role-admin', 'admin', 'Administrator', 'Vollzugriff auf alle Module', true),
  ('role-sales', 'sales', 'Vertrieb', 'Vertrieb und Angebotswesen', true),
  ('role-procurement', 'procurement', 'Einkauf', 'Beschaffung und Bestellwesen', true),
  ('role-inventory', 'inventory', 'Lager', 'Material- und Lagerverwaltung', true),
  ('role-finance', 'finance', 'Finanzwesen', 'Rechnungen, Zahlungen und Controlling', true),
  ('role-hr', 'hr', 'HR', 'Personal- und Abwesenheitsverwaltung', true),
  ('role-production', 'production', 'Produktion', 'Arbeitsvorbereitung und Fertigung', true),
  ('role-fleet', 'fleet', 'Fuhrpark', 'Fahrzeuge und mobile Assets', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-users-manage', 'users.manage', 'Benutzer verwalten', 'Benutzer und Rollen pflegen', 'platform'),
  ('perm-settings-manage', 'settings.manage', 'Einstellungen verwalten', 'Systemweite Einstellungen pflegen', 'platform'),
  ('perm-contacts-read', 'contacts.read', 'Kontakte lesen', 'Kontakte anzeigen', 'masterdata'),
  ('perm-contacts-write', 'contacts.write', 'Kontakte schreiben', 'Kontakte bearbeiten', 'masterdata'),
  ('perm-materials-read', 'materials.read', 'Materialien lesen', 'Materialstamm anzeigen', 'inventory'),
  ('perm-materials-write', 'materials.write', 'Materialien schreiben', 'Materialstamm bearbeiten', 'inventory'),
  ('perm-warehouses-read', 'warehouses.read', 'Lager lesen', 'Lager anzeigen', 'inventory'),
  ('perm-warehouses-write', 'warehouses.write', 'Lager schreiben', 'Lager und Lagerorte bearbeiten', 'inventory'),
  ('perm-stock-movements-write', 'stock_movements.write', 'Bestandsbewegungen schreiben', 'Bestandsbewegungen buchen', 'inventory'),
  ('perm-purchase-orders-read', 'purchase_orders.read', 'Bestellungen lesen', 'Bestellungen anzeigen', 'procurement'),
  ('perm-purchase-orders-write', 'purchase_orders.write', 'Bestellungen schreiben', 'Bestellungen bearbeiten', 'procurement'),
  ('perm-projects-read', 'projects.read', 'Projekte lesen', 'Projekte anzeigen', 'projects'),
  ('perm-projects-write', 'projects.write', 'Projekte schreiben', 'Projekte bearbeiten', 'projects'),
  ('perm-documents-read', 'documents.read', 'Dokumente lesen', 'Dokumente und PDFs herunterladen', 'platform'),
  ('perm-quotes-read', 'quotes.read', 'Angebote lesen', 'Angebote anzeigen', 'sales'),
  ('perm-quotes-write', 'quotes.write', 'Angebote schreiben', 'Angebote bearbeiten', 'sales')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-sales', p.id
FROM permissions p
WHERE p.code IN ('contacts.read', 'contacts.write', 'projects.read', 'quotes.read', 'quotes.write', 'documents.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-procurement', p.id
FROM permissions p
WHERE p.code IN ('contacts.read', 'materials.read', 'purchase_orders.read', 'purchase_orders.write', 'projects.read', 'documents.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-inventory', p.id
FROM permissions p
WHERE p.code IN ('materials.read', 'materials.write', 'warehouses.read', 'warehouses.write', 'stock_movements.write', 'projects.read', 'documents.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-finance', p.id
FROM permissions p
WHERE p.code IN ('contacts.read', 'purchase_orders.read', 'projects.read', 'documents.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-production', p.id
FROM permissions p
WHERE p.code IN ('materials.read', 'projects.read', 'projects.write', 'documents.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code = 'settings.manage'
ON CONFLICT DO NOTHING;
