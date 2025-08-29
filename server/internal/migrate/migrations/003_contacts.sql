-- Kontakte (CRM): Firmen/Personen, Adressen, Ansprechpartner

CREATE TABLE IF NOT EXISTS contacts (
  id text PRIMARY KEY,
  typ text NOT NULL,                                -- org | person
  rolle text NOT NULL DEFAULT 'other',              -- customer | supplier | both | other
  name text NOT NULL,
  email text DEFAULT '',
  phone text DEFAULT '',
  vat_id text DEFAULT '',                           -- USt-IdNr.
  tax_no text DEFAULT '',                           -- Steuernummer
  waehrung char(3) NOT NULL DEFAULT 'EUR',
  aktiv boolean NOT NULL DEFAULT true,
  angelegt_am timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(name);
CREATE INDEX IF NOT EXISTS idx_contacts_rolle ON contacts(rolle);
CREATE INDEX IF NOT EXISTS idx_contacts_typ ON contacts(typ);

CREATE TABLE IF NOT EXISTS contact_addresses (
  id text PRIMARY KEY,
  contact_id text NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  art text NOT NULL DEFAULT 'other',                -- billing | shipping | other
  zeile1 text NOT NULL,
  zeile2 text DEFAULT '',
  plz text DEFAULT '',
  ort text DEFAULT '',
  land text DEFAULT '',
  is_primary boolean NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_contact_addresses_contact ON contact_addresses(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_addresses_primary ON contact_addresses(contact_id, is_primary);

CREATE TABLE IF NOT EXISTS contact_persons (
  id text PRIMARY KEY,
  contact_id text NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  anrede text DEFAULT '',
  vorname text DEFAULT '',
  nachname text DEFAULT '',
  position text DEFAULT '',
  email text DEFAULT '',
  phone text DEFAULT '',
  mobile text DEFAULT '',
  is_primary boolean NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_contact_persons_contact ON contact_persons(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_persons_primary ON contact_persons(contact_id, is_primary);

