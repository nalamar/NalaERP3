-- Seed numbering for projects
INSERT INTO number_sequences(entity, pattern, next_value)
SELECT 'project', 'PRJ-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE entity='project');

