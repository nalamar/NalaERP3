ALTER TABLE contacts
  ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';

UPDATE contacts
SET status = CASE
  WHEN aktiv = false THEN 'inactive'
  ELSE 'active'
END
WHERE status IS NULL OR status = '';

CREATE INDEX IF NOT EXISTS idx_contacts_status ON contacts(status);
