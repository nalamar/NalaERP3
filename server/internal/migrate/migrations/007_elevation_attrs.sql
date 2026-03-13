-- Zusätzliche Attribute je Position (Elevation)
ALTER TABLE project_elevations ADD COLUMN IF NOT EXISTS serie TEXT NULL;
ALTER TABLE project_elevations ADD COLUMN IF NOT EXISTS oberflaeche TEXT NULL;

