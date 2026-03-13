CREATE TABLE IF NOT EXISTS contact_notes (
  id text PRIMARY KEY,
  contact_id text NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  titel text NOT NULL DEFAULT '',
  inhalt text NOT NULL DEFAULT '',
  erstellt_am timestamptz NOT NULL DEFAULT now(),
  aktualisiert_am timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_notes_contact
ON contact_notes(contact_id, erstellt_am DESC);
