-- Nachtragsmanagement fuer bestehende Auftraege (ADR 0010, Backlog B.2.2).
-- Zwei neue Tabellen, keine bestehende Tabelle veraendert:
--   - sales_order_addenda: Nachtrag-Header (Status entwurf/beantragt/
--     angenommen/abgelehnt, Begruendung, eigene Summen). nachtrag_no ist
--     sequentiell PRO Auftrag (nicht ueber number_sequences), da ein
--     Nachtrag fachlich eine Unternummerierung des Auftrags ist.
--   - sales_order_addendum_items: Nachtrag-Positionen, identisches,
--     bewusst schlankes Spaltenset wie sales_order_items (kein
--     material_id, keine Hierarchie - das Original hat beides auch nicht).
--
-- Kein eigenes company_id auf beiden Tabellen - Scope wird ueber
-- sales_order_id -> sales_orders.company_id geerbt (Muster wie
-- sales_order_items selbst, ADR 0002).
--
-- Die effektive Auftragssumme (Grundauftrag + angenommene Nachtraege) wird
-- bewusst NICHT in sales_orders geschrieben (siehe ADR 0010) - keine
-- Schema-Aenderung an sales_orders noetig.

CREATE TABLE IF NOT EXISTS sales_order_addenda (
    id UUID PRIMARY KEY,
    sales_order_id UUID NOT NULL REFERENCES sales_orders(id) ON DELETE CASCADE,
    nachtrag_no int NOT NULL,
    status text NOT NULL DEFAULT 'entwurf',
    begruendung text NOT NULL DEFAULT '',
    currency char(3) NOT NULL DEFAULT 'EUR',
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    gross_amount numeric(18,4) NOT NULL DEFAULT 0,
    beantragt_am timestamptz,
    entschieden_am timestamptz,
    entschieden_von text,
    ablehnungsgrund text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_sales_order_addenda_status CHECK (status IN ('entwurf', 'beantragt', 'angenommen', 'abgelehnt')),
    UNIQUE (sales_order_id, nachtrag_no)
);
CREATE INDEX IF NOT EXISTS idx_sales_order_addenda_order ON sales_order_addenda(sales_order_id);
CREATE INDEX IF NOT EXISTS idx_sales_order_addenda_status ON sales_order_addenda(status);

CREATE TABLE IF NOT EXISTS sales_order_addendum_items (
    id UUID PRIMARY KEY,
    addendum_id UUID NOT NULL REFERENCES sales_order_addenda(id) ON DELETE CASCADE,
    position int NOT NULL,
    description text NOT NULL,
    qty numeric(18,4) NOT NULL DEFAULT 1,
    unit text NOT NULL DEFAULT 'Stk',
    unit_price numeric(18,4) NOT NULL DEFAULT 0,
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_code text REFERENCES tax_codes(code)
);
CREATE INDEX IF NOT EXISTS idx_sales_order_addendum_items_addendum ON sales_order_addendum_items(addendum_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS sales_order_addendum_items;
--   DROP TABLE IF EXISTS sales_order_addenda;
-- DATENVERLUSTRISIKO: Sobald echte Nachtraege (Header + Positionen sowie
-- deren Entscheidungshistorie) erfasst wurden, gehen diese bei einem
-- Downgrade unwiderruflich verloren. Kein Risiko fuer bestehende
-- sales_orders/sales_order_items - beide Tabellen bleiben unangetastet.
