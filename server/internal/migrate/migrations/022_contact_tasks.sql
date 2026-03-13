CREATE TABLE IF NOT EXISTS contact_tasks (
    id TEXT PRIMARY KEY,
    contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    titel TEXT NOT NULL DEFAULT '',
    beschreibung TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    faellig_am TIMESTAMPTZ NULL,
    erledigt_am TIMESTAMPTZ NULL,
    erstellt_am TIMESTAMPTZ NOT NULL DEFAULT now(),
    aktualisiert_am TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_tasks_contact
ON contact_tasks(contact_id, status, faellig_am);
