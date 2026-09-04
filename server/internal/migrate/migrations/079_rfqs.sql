-- Anfrageprozess (RFQ) vor Bestellung (ADR 0017, Backlog D.2.2). Drei neue
-- Tabellen, keine bestehende Tabelle veraendert:
--   - rfqs: Anfrage-Header, eigenstaendiges mandantenweites Dokument mit
--     eigener company_id-Spalte (wie purchase_orders selbst, kein Kind
--     einer anderen Tabelle wie bei den Lager-/Bestandstabellen aus
--     C.1-C.3). status bewusst OHNE CHECK-Constraint - Konsistenz mit dem
--     unmittelbaren Schwester-Dokument purchase_orders, das ebenfalls nur
--     im Anwendungscode validiert (Statuses()/isIn()).
--   - rfq_items: Positionen (Material+Menge), Spaltenset bewusst identisch
--     zu purchase_order_items, aber ohne Preis (der entsteht erst durch
--     die Lieferanten-Offerte).
--   - rfq_supplier_quotes: Lieferanten-Offerten je Position, UNIQUE
--     (rfq_item_id, supplier_id) fuer Upsert-Verhalten (keine Historie -
--     eine Offerte vor der Bestellung ist kein GoBD-relevanter Beleg,
--     siehe ADR 0017).
--
-- Neuer number_sequences-Eintrag fuer Entity 'rfq', analog zum
-- bestehenden 'purchase_order'-Muster. WICHTIG (Fund bei dieser Subtask,
-- NICHT hier behoben): settings.NumberingService.Next() erzeugt bei
-- fehlendem number_sequences-Eintrag einen HARTKODIERTEN Fallback-Pattern
-- "PO-{YYYY}-{NNNN}" unabhaengig vom uebergebenen entity-Namen
-- (server/internal/settings/numbering.go:106) - ohne diesen expliziten
-- Seed wuerden RFQ-Nummern faelschlich mit "PO-" statt "RFQ-" beginnen.
-- Der Fallback-Bug selbst betrifft potenziell jede kuenftige neue Entity
-- und jeden kuenftigen zweiten Mandanten; separate, eigenstaendige
-- Korrektur ausserhalb des D.2-Scopes.

CREATE TABLE IF NOT EXISTS rfqs (
    id text PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    nummer text NOT NULL,
    status text NOT NULL DEFAULT 'offen',
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz,
    UNIQUE (nummer)
);

CREATE INDEX IF NOT EXISTS idx_rfqs_company_id ON rfqs(company_id);
CREATE INDEX IF NOT EXISTS idx_rfqs_status ON rfqs(status);

CREATE TABLE IF NOT EXISTS rfq_items (
    id text PRIMARY KEY,
    rfq_id text NOT NULL REFERENCES rfqs(id) ON DELETE CASCADE,
    position int NOT NULL,
    material_id text NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    description text NOT NULL DEFAULT '',
    qty numeric(18,6) NOT NULL,
    uom text NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rfq_items_rfq ON rfq_items(rfq_id);
CREATE INDEX IF NOT EXISTS idx_rfq_items_material ON rfq_items(material_id);

CREATE TABLE IF NOT EXISTS rfq_supplier_quotes (
    id text PRIMARY KEY,
    rfq_item_id text NOT NULL REFERENCES rfq_items(id) ON DELETE CASCADE,
    supplier_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    unit_price numeric(18,6) NOT NULL,
    currency char(3) NOT NULL DEFAULT 'EUR',
    delivery_date date,
    note text NOT NULL DEFAULT '',
    quoted_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (rfq_item_id, supplier_id)
);

CREATE INDEX IF NOT EXISTS idx_rfq_supplier_quotes_item ON rfq_supplier_quotes(rfq_item_id);
CREATE INDEX IF NOT EXISTS idx_rfq_supplier_quotes_supplier ON rfq_supplier_quotes(supplier_id);

INSERT INTO number_sequences (company_id, entity, pattern, next_value)
SELECT 'default', 'rfq', 'RFQ-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE company_id = 'default' AND entity = 'rfq');

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM number_sequences WHERE entity = 'rfq';
--   DROP TABLE IF EXISTS rfq_supplier_quotes;
--   DROP TABLE IF EXISTS rfq_items;
--   DROP TABLE IF EXISTS rfqs;
-- DATENVERLUSTRISIKO: Sobald echte Anfragen (Header, Positionen,
-- Lieferanten-Offerten) erfasst wurden, gehen diese bei einem Downgrade
-- unwiderruflich verloren. Kein Risiko fuer bestehende purchase_orders/
-- purchase_order_items/materials/contacts - alle bleiben unangetastet.
