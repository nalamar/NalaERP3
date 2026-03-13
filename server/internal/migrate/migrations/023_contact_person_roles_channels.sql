ALTER TABLE contact_persons
ADD COLUMN IF NOT EXISTS rolle TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS bevorzugter_kanal TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_contact_persons_role
ON contact_persons(contact_id, rolle);
