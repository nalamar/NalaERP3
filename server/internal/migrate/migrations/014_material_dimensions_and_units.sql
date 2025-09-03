-- Add optional dimensions to materials and units master data

ALTER TABLE materials
  ADD COLUMN IF NOT EXISTS length_mm numeric(18,6),
  ADD COLUMN IF NOT EXISTS width_mm  numeric(18,6),
  ADD COLUMN IF NOT EXISTS height_mm numeric(18,6);

CREATE TABLE IF NOT EXISTS units (
  code text PRIMARY KEY,
  name text NOT NULL DEFAULT ''
);

-- Seed some common units if table is empty
INSERT INTO units (code, name)
SELECT x.code, x.name
FROM (VALUES
  ('pcs','Stück'),
  ('kg','Kilogramm'),
  ('g','Gramm'),
  ('m','Meter'),
  ('cm','Zentimeter'),
  ('mm','Millimeter'),
  ('l','Liter')
) AS x(code, name)
WHERE NOT EXISTS (SELECT 1 FROM units);

