CREATE TABLE IF NOT EXISTS sales_orders (
    id UUID PRIMARY KEY,
    nummer text NOT NULL UNIQUE,
    source_quote_id UUID NOT NULL UNIQUE REFERENCES quotes(id) ON DELETE RESTRICT,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    contact_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'open',
    order_date date NOT NULL,
    currency char(3) NOT NULL DEFAULT 'EUR',
    note text NOT NULL DEFAULT '',
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    gross_amount numeric(18,4) NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sales_orders_project ON sales_orders(project_id);
CREATE INDEX IF NOT EXISTS idx_sales_orders_contact ON sales_orders(contact_id);
CREATE INDEX IF NOT EXISTS idx_sales_orders_status ON sales_orders(status);

CREATE TABLE IF NOT EXISTS sales_order_items (
    id UUID PRIMARY KEY,
    sales_order_id UUID NOT NULL REFERENCES sales_orders(id) ON DELETE CASCADE,
    position int NOT NULL,
    description text NOT NULL,
    qty numeric(18,4) NOT NULL DEFAULT 1,
    unit text NOT NULL DEFAULT 'Stk',
    unit_price numeric(18,4) NOT NULL DEFAULT 0,
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_code text REFERENCES tax_codes(code)
);

CREATE INDEX IF NOT EXISTS idx_sales_order_items_order ON sales_order_items(sales_order_id);

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS linked_sales_order_id UUID REFERENCES sales_orders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_quotes_linked_sales_order ON quotes(linked_sales_order_id);

INSERT INTO number_sequences(entity, pattern, next_value)
SELECT 'sales_order', 'AUF-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE entity='sales_order');

INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-sales-orders-read', 'sales_orders.read', 'Aufträge lesen', 'Aufträge anzeigen', 'sales'),
  ('perm-sales-orders-write', 'sales_orders.write', 'Aufträge schreiben', 'Aufträge anlegen und bearbeiten', 'sales')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-sales', p.id
FROM permissions p
WHERE p.code IN ('sales_orders.read', 'sales_orders.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role-admin', p.id
FROM permissions p
WHERE p.code IN ('sales_orders.read', 'sales_orders.write')
ON CONFLICT DO NOTHING;
