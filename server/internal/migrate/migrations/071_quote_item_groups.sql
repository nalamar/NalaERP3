-- LV-Hierarchie (Los/Titel/Untertitel) im Positionsmodell (ADR 0009,
-- Backlog B.1.2). Separate Tabelle statt Vermischung mit quote_items:
-- Gruppenknoten (Los/Titel/Untertitel) sind reine Struktur-Ueberschriften,
-- keine bepreisten Positionen - eine Vermischung wuerde alle Preisspalten
-- fuer Gruppenzeilen bedeutungslos machen.
--
-- Kein eigenes company_id - Scope wird ueber quote_id -> quotes.company_id
-- geerbt (identisches Muster wie quote_items selbst, ADR 0002).
--
-- Wohlgeformtheits-Pruefung (welcher kind darf unter welchem Eltern-kind
-- stehen) erfolgt bewusst NICHT per DB-Trigger, sondern im Anwendungscode
-- (Backlog B.1.3), analog zu ADR 0008.

CREATE TABLE IF NOT EXISTS quote_item_groups (
    id UUID PRIMARY KEY,
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    parent_group_id UUID REFERENCES quote_item_groups(id) ON DELETE CASCADE,
    kind text NOT NULL,
    bezeichnung text NOT NULL,
    sort_order int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_quote_item_groups_kind CHECK (kind IN ('los', 'titel', 'untertitel')),
    CONSTRAINT chk_quote_item_groups_bezeichnung CHECK (BTRIM(bezeichnung) <> '')
);
CREATE INDEX IF NOT EXISTS idx_quote_item_groups_quote_id ON quote_item_groups(quote_id);
CREATE INDEX IF NOT EXISTS idx_quote_item_groups_parent_group_id ON quote_item_groups(parent_group_id);

-- quote_items.group_id ist bewusst NULLABLE mit ON DELETE SET NULL (NICHT
-- CASCADE): das Loeschen eines Gruppenknotens darf NIE bepreiste Positionen
-- mitloeschen, nur deren Gruppierung aufheben. group_id=NULL bedeutet
-- "ungruppierte Position" - jedes bereits bestehende Angebot bleibt damit
-- unveraendert gueltig, keine Datenmigration noetig.
ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS group_id UUID REFERENCES quote_item_groups(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_quote_items_group_id ON quote_items(group_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quote_items DROP COLUMN IF EXISTS group_id;
--   DROP TABLE IF EXISTS quote_item_groups;
-- DATENVERLUSTRISIKO: Sobald echte Los/Titel/Untertitel-Strukturen erfasst
-- wurden, gehen diese bei einem Downgrade unwiderruflich verloren. Die
-- bepreisten quote_items selbst sind davon NICHT betroffen (group_id wird
-- beim Downgrade nur entfernt, keine Zeile geloescht).
