-- Nachweise der Zulieferer nach EN 1090 (ADR 0035, Backlog H.4.1a).
-- Entscheidung 2026-11-03: alle sechs Empfehlungen als ANNAHME
-- ("fortsetzen" nach der Frage mit Empfehlung).
--
-- Der Betrieb fertigt selbst nicht nach EN 1090, kauft aber tragende
-- Teile bei zertifizierten Zulieferern. Das ERP DOKUMENTIERT die
-- Nachweise, es bewertet sie nicht; fehlende oder abgelaufene Nachweise
-- geben nur Hinweise (Service), keine Sperre.
--
-- Nachweise haben Beweisfunktion: nie loeschen, ein falscher Eintrag
-- wird mit Grund storniert (wie Qualifikationen, ADR 0027). Darum haengen
-- sie mit RESTRICT an Lieferant und Bestellung. Dass Lieferant,
-- Bestellung und Position zum selben Mandanten gehoeren, prueft der
-- Service (contacts.company_id ist nullable, Migration 054).

-- 1. Zertifikat am Lieferanten. exc_bis (hoechste zertifizierte
-- Ausfuehrungsklasse) genau bei EN 1090. gueltig_bis leer = ohne
-- Enddatum (die Stelle ueberwacht laufend); gesetzt gilt das Zertifikat
-- bis einschliesslich diesem Tag.
CREATE TABLE IF NOT EXISTS supplier_certificates (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    supplier_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    art text NOT NULL,
    nummer text NOT NULL DEFAULT '',
    stelle text NOT NULL DEFAULT '',
    exc_bis text,
    gueltig_von date,
    gueltig_bis date,
    notiz text NOT NULL DEFAULT '',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT supplier_certificates_art_check CHECK (art IN ('en1090', 'iso3834', 'sonstiges')),
    CONSTRAINT supplier_certificates_exc_check CHECK (
        -- IS NOT NULL ausdruecklich: NULL IN (...) ist NULL, und ein
        -- CHECK laesst NULL durch.
        (art = 'en1090' AND exc_bis IS NOT NULL AND exc_bis IN ('EXC1', 'EXC2', 'EXC3', 'EXC4'))
        OR (art <> 'en1090' AND exc_bis IS NULL)
    ),
    CONSTRAINT supplier_certificates_period_check CHECK (
        gueltig_von IS NULL OR gueltig_bis IS NULL OR gueltig_bis >= gueltig_von
    ),
    CONSTRAINT supplier_certificates_storno_consistent CHECK (
        (storniert_at IS NULL AND storno_reason = '')
        OR (storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
    )
);

CREATE INDEX IF NOT EXISTS idx_supplier_certificates_supplier
    ON supplier_certificates(company_id, supplier_id, art);

COMMENT ON TABLE supplier_certificates IS
    'Zertifikate der Zulieferer (ADR 0035 §1): EN 1090 mit EXC bis, Gueltigkeit, Storno statt Loeschen.';

-- Dateien zum Zertifikat (Scan), Datei in GridFS wie 096. Bewusst NICHT
-- ueber den allgemeinen Download /documents/{id} (ADR 0035 §6).
CREATE TABLE IF NOT EXISTS supplier_certificate_documents (
    id uuid PRIMARY KEY,
    certificate_id uuid NOT NULL REFERENCES supplier_certificates(id) ON DELETE RESTRICT,
    document_id text NOT NULL UNIQUE,
    filename text NOT NULL,
    content_type text NOT NULL,
    length bigint NOT NULL CHECK (length >= 0),
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    uploaded_by_user_id text REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_supplier_certificate_documents_certificate
    ON supplier_certificate_documents(certificate_id, uploaded_at);

-- 2. Nachweispflichtiges Material (ADR 0035 §2). Ohne Kennzeichen
-- aendert sich nichts.
ALTER TABLE materials
    ADD COLUMN IF NOT EXISTS nachweis_en1090 boolean NOT NULL DEFAULT false;

-- 5. Optionales Projekt an der Bestellung (ADR 0035 §5); RESTRICT wie
-- bei den Fertigungsauftraegen (107): die Nachweise eines Projekts
-- bleiben auffindbar.
ALTER TABLE purchase_orders
    ADD COLUMN IF NOT EXISTS project_id uuid REFERENCES projects(id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_purchase_orders_project
    ON purchase_orders(project_id) WHERE project_id IS NOT NULL;

-- 4. Lieferpapiere an der Bestellung (ADR 0035 §4): Leistungserklaerung,
-- Werkstoffzeugnisse nach EN 10204, Lieferschein. Optional mit Position
-- (SET NULL: eine Position laesst sich im Entwurf loeschen) und
-- Chargencode wie auf dem Papier.
CREATE TABLE IF NOT EXISTS purchase_order_documents (
    id uuid PRIMARY KEY,
    purchase_order_id text NOT NULL REFERENCES purchase_orders(id) ON DELETE RESTRICT,
    purchase_order_item_id text REFERENCES purchase_order_items(id) ON DELETE SET NULL,
    art text NOT NULL,
    charge text NOT NULL DEFAULT '',
    notiz text NOT NULL DEFAULT '',
    document_id text NOT NULL UNIQUE,
    filename text NOT NULL,
    content_type text NOT NULL,
    length bigint NOT NULL CHECK (length >= 0),
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    uploaded_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT purchase_order_documents_art_check CHECK (
        art IN ('leistungserklaerung', 'zeugnis_2_2', 'zeugnis_3_1', 'zeugnis_3_2', 'lieferschein', 'sonstiges')
    ),
    CONSTRAINT purchase_order_documents_storno_consistent CHECK (
        (storniert_at IS NULL AND storno_reason = '')
        OR (storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
    )
);
CREATE INDEX IF NOT EXISTS idx_purchase_order_documents_order
    ON purchase_order_documents(purchase_order_id, uploaded_at);
CREATE INDEX IF NOT EXISTS idx_purchase_order_documents_item
    ON purchase_order_documents(purchase_order_item_id) WHERE purchase_order_item_id IS NOT NULL;

COMMENT ON TABLE purchase_order_documents IS
    'Lieferpapiere der Bestellung (ADR 0035 §4): Leistungserklaerung, Zeugnisse EN 10204, Storno statt Loeschen.';

-- 6. Keine neuen Rechte: lesen mit purchase_orders.read, pflegen mit
-- purchase_orders.write, Kennzeichen am Material mit materials.write.

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS purchase_order_documents;
--   DROP INDEX IF EXISTS idx_purchase_orders_project;
--   ALTER TABLE purchase_orders DROP COLUMN IF EXISTS project_id;
--   ALTER TABLE materials DROP COLUMN IF EXISTS nachweis_en1090;
--   DROP TABLE IF EXISTS supplier_certificate_documents;
--   DROP TABLE IF EXISTS supplier_certificates;
-- DATENVERLUSTRISIKO (HOCH): alle erfassten Zertifikate, Lieferpapiere,
-- Kennzeichen am Material und Projekte an Bestellungen gehen verloren;
-- die Dateien bleiben verwaist in GridFS. Vorher exportieren.
