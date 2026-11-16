-- Recht zum Pflegen des Leistungskatalogs (Backlog I.4.4b). Lesen laeuft
-- ueber quotes.read (wer Angebote sieht, sieht die Leistungen); Anlegen,
-- Aendern und Import brauchen leistungen.write, weil der Katalog als
-- Stammdatum alle Zuordnungen beeinflusst. Standard: Admin und Vertrieb.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-leistungen-write', 'leistungen.write', 'Leistungskatalog pflegen', 'Leistungen anlegen, aendern, abschalten und importieren', 'sales')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-sales')
  AND p.code = 'leistungen.write'
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'leistungen.write');
--   DELETE FROM permissions WHERE code = 'leistungen.write';
-- Datenverlustrisiko: niedrig - nur die Zuordnung des Rechts zu Rollen geht
-- verloren (individuell vergebene Zuordnungen muessten neu gesetzt werden).
