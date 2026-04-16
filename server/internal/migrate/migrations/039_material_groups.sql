CREATE TABLE IF NOT EXISTS material_groups (
    code text PRIMARY KEY,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO material_groups (code, name)
SELECT DISTINCT TRIM(kategorie), TRIM(kategorie)
FROM materials
WHERE TRIM(kategorie) <> ''
  AND NOT EXISTS (
      SELECT 1
      FROM material_groups mg
      WHERE mg.code = TRIM(materials.kategorie)
  );
