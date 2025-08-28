-- UTF-8, deutsch

CREATE TABLE IF NOT EXISTS materials (
  id text PRIMARY KEY,
  nummer text NOT NULL UNIQUE,
  bezeichnung text NOT NULL,
  typ text NOT NULL,
  norm text DEFAULT '',
  werkstoffnummer text DEFAULT '',
  einheit text NOT NULL,
  dichte numeric(18,6) DEFAULT 0,
  kategorie text DEFAULT '',
  attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
  aktiv boolean NOT NULL DEFAULT true,
  avg_purchase_price numeric(18,6) NOT NULL DEFAULT 0,
  currency char(3) NOT NULL DEFAULT 'EUR',
  purchase_total_qty numeric(18,6) NOT NULL DEFAULT 0,
  purchase_total_value numeric(18,6) NOT NULL DEFAULT 0,
  angelegt_am timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS warehouses (
  id text PRIMARY KEY,
  code text NOT NULL UNIQUE,
  name text NOT NULL
);

CREATE TABLE IF NOT EXISTS locations (
  id text PRIMARY KEY,
  warehouse_id text NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
  code text NOT NULL,
  name text NOT NULL,
  UNIQUE (warehouse_id, code)
);

CREATE TABLE IF NOT EXISTS batches (
  id text PRIMARY KEY,
  material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
  code text NOT NULL,
  supplier text DEFAULT '',
  angelegt_am timestamptz NOT NULL DEFAULT now(),
  UNIQUE (material_id, code)
);

CREATE TABLE IF NOT EXISTS stock_movements (
  id text PRIMARY KEY,
  material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
  warehouse_id text NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
  location_id text REFERENCES locations(id) ON DELETE SET NULL,
  batch_id text REFERENCES batches(id) ON DELETE SET NULL,
  quantity numeric(18,6) NOT NULL,
  uom text NOT NULL,
  movement_type text NOT NULL,
  reason text DEFAULT '',
  reference text DEFAULT '',
  at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_stock_movements_material ON stock_movements(material_id);
CREATE INDEX IF NOT EXISTS idx_stock_movements_wh_loc ON stock_movements(warehouse_id, location_id);

