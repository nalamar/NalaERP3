-- Zusatzattribute an Positionen: Serie und Oberfläche
ALTER TABLE project_elevations ADD COLUMN IF NOT EXISTS serie text;
ALTER TABLE project_elevations ADD COLUMN IF NOT EXISTS oberflaeche text;

