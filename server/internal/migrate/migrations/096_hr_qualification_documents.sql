-- Nachweisdokumente zu Qualifikationen (Backlog F.10.1, aus ADR 0027):
-- Scan des Zertifikats, unterschriebene Teilnehmerliste der Unterweisung.
-- Die Datei liegt in GridFS (wie contact_documents), hier steht der
-- Verweis.
--
-- Keine Loeschung: wie die Nachweise selbst haben die Dokumente
-- Beweisfunktion; ein falscher Nachweis wird storniert, seine Dokumente
-- bleiben nachvollziehbar daran haengen. ON DELETE CASCADE greift nur,
-- falls ein Nachweis je technisch entfernt wird (es gibt keinen Weg dazu).
--
-- Bewusst NICHT ueber den allgemeinen Download /documents/{id}
-- (documents.read) erreichbar - Zertifikate sind personenbezogene
-- Unterlagen; Zugriff nur ueber die Qualifikationsrechte.
CREATE TABLE IF NOT EXISTS hr_qualification_documents (
    id uuid PRIMARY KEY,
    qualification_id uuid NOT NULL REFERENCES hr_employee_qualifications(id) ON DELETE CASCADE,
    document_id text NOT NULL UNIQUE,
    filename text NOT NULL,
    content_type text NOT NULL,
    length bigint NOT NULL CHECK (length >= 0),
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    uploaded_by_user_id text REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_hr_qualification_documents_qualification ON hr_qualification_documents(qualification_id, uploaded_at);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS hr_qualification_documents;
-- DATENVERLUSTRISIKO (HOCH): die Verweise auf alle hochgeladenen
-- Nachweisdokumente gehen verloren; die Dateien bleiben verwaist in GridFS.
-- Vorher exportieren.
