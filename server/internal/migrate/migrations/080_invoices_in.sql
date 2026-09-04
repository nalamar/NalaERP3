-- Eingangsrechnungsprüfung / 3-Way-Match (ADR 0018, Backlog D.3.2).
--
-- 1) stock_movements bekommt eine additive, nullable FK auf
--    purchase_order_items - der physische Wareneingang wird bereits
--    vollständig über stock_movements (movement_type='purchase')
--    abgebildet, eine eigene goods_receipts-Buchführung wäre eine zweite
--    Quelle der Wahrheit fuer denselben Sachverhalt (siehe ADR 0018,
--    Option A verworfen). ON DELETE SET NULL - ein geloeschtes
--    Bestellsystem-Objekt darf die physische Wareneingangs-Historie
--    nicht mitreissen.
-- 2) invoices_in/invoice_in_items: neue Kopf-/Positionstabellen fuer
--    Eingangsrechnungen, bewusst OHNE Buchungs-/Storno-/Freigabeworkflow
--    (kein CHECK auf status - aktuell nur der eine Wert 'erfasst',
--    Workflow-Erweiterung ist Sache eines spaeteren Epics). company_id
--    als eigene Spalte (top-level Dokument, wie invoices_out/
--    purchase_orders), purchase_order_id NULLABLE (nicht jede
--    Eingangsrechnung hat einen Bestellbezug).
-- 3) Neue Permissions invoices_in.read/write - genuin neue Domaene ohne
--    passende bestehende Permission (siehe ADR 0018).

ALTER TABLE stock_movements
    ADD COLUMN IF NOT EXISTS purchase_order_item_id text REFERENCES purchase_order_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_stock_movements_purchase_order_item ON stock_movements(purchase_order_item_id);

CREATE TABLE IF NOT EXISTS invoices_in (
    id text PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    supplier_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    purchase_order_id text REFERENCES purchase_orders(id) ON DELETE SET NULL,
    invoice_number text NOT NULL DEFAULT '',
    invoice_date date NOT NULL DEFAULT CURRENT_DATE,
    currency char(3) NOT NULL DEFAULT 'EUR',
    status text NOT NULL DEFAULT 'erfasst',
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invoices_in_company_id ON invoices_in(company_id);
CREATE INDEX IF NOT EXISTS idx_invoices_in_supplier ON invoices_in(supplier_id);
CREATE INDEX IF NOT EXISTS idx_invoices_in_purchase_order ON invoices_in(purchase_order_id);

CREATE TABLE IF NOT EXISTS invoice_in_items (
    id text PRIMARY KEY,
    invoice_in_id text NOT NULL REFERENCES invoices_in(id) ON DELETE CASCADE,
    purchase_order_item_id text REFERENCES purchase_order_items(id) ON DELETE SET NULL,
    description text NOT NULL DEFAULT '',
    qty numeric(18,6) NOT NULL,
    unit_price numeric(18,6) NOT NULL DEFAULT 0,
    currency char(3) NOT NULL DEFAULT 'EUR'
);

CREATE INDEX IF NOT EXISTS idx_invoice_in_items_invoice ON invoice_in_items(invoice_in_id);
CREATE INDEX IF NOT EXISTS idx_invoice_in_items_po_item ON invoice_in_items(purchase_order_item_id);

INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-invoices-in-read', 'invoices_in.read', 'Eingangsrechnungen lesen', 'Eingangsrechnungen und 3-Way-Match-Ergebnisse anzeigen', 'finance'),
  ('perm-invoices-in-write', 'invoices_in.write', 'Eingangsrechnungen schreiben', 'Eingangsrechnungen erfassen', 'finance')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-finance', p.id
FROM permissions p
WHERE p.code IN ('invoices_in.read', 'invoices_in.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-procurement', p.id
FROM permissions p
WHERE p.code IN ('invoices_in.read', 'invoices_in.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code IN ('invoices_in.read', 'invoices_in.write')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('invoices_in.read', 'invoices_in.write'));
--   DELETE FROM permissions WHERE code IN ('invoices_in.read', 'invoices_in.write');
--   DROP TABLE IF EXISTS invoice_in_items;
--   DROP TABLE IF EXISTS invoices_in;
--   ALTER TABLE stock_movements DROP COLUMN IF EXISTS purchase_order_item_id;
-- DATENVERLUSTRISIKO: Sobald echte Eingangsrechnungen erfasst wurden,
-- gehen diese bei einem Downgrade unwiderruflich verloren. Die
-- Wareneingangs-Verknuepfung auf stock_movements geht ebenfalls verloren
-- (die Bewegungen selbst bleiben erhalten, nur der PO-Bezug faellt weg).
-- Kein Risiko fuer bestehende purchase_orders/stock_movements/contacts -
-- alle bleiben inhaltlich unangetastet.
