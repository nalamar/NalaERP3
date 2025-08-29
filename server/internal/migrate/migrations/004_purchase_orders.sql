-- Bestellungen (Purchase Orders)

CREATE TABLE IF NOT EXISTS purchase_orders (
  id text PRIMARY KEY,
  supplier_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
  number text NOT NULL,
  order_date date NOT NULL DEFAULT CURRENT_DATE,
  currency char(3) NOT NULL DEFAULT 'EUR',
  status text NOT NULL DEFAULT 'draft', -- draft | ordered | received | canceled
  note text DEFAULT '',
  angelegt_am timestamptz NOT NULL DEFAULT now(),
  UNIQUE (number)
);

CREATE INDEX IF NOT EXISTS idx_po_supplier ON purchase_orders(supplier_id);
CREATE INDEX IF NOT EXISTS idx_po_status ON purchase_orders(status);

CREATE TABLE IF NOT EXISTS purchase_order_items (
  id text PRIMARY KEY,
  order_id text NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
  position int NOT NULL,
  material_id text NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
  description text NOT NULL DEFAULT '',
  qty numeric(18,6) NOT NULL,
  uom text NOT NULL,
  unit_price numeric(18,6) NOT NULL DEFAULT 0,
  currency char(3) NOT NULL DEFAULT 'EUR',
  delivery_date date
);

CREATE INDEX IF NOT EXISTS idx_poi_order ON purchase_order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_poi_material ON purchase_order_items(material_id);

