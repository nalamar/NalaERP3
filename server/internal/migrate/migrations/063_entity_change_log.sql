-- Generisches Änderungsprotokoll über Fachobjekte (Task 0.3.3, letzter
-- Task von Epic 0.3 GoBD-Fundament).
--
-- Bisher existieren nur zwei domänenspezifische Protokolle:
-- project_import_changes (LogiKal-Import, Migration 006/012) und
-- quote_item_price_decisions (Quote-Preisentscheidungen, Migration 048) -
-- beide fest an ihre jeweilige Domäne gebunden. entity_change_log ist
-- stattdessen bewusst generisch: entity_type/entity_id statt einer festen
-- Fremdschlüsselbeziehung, damit dieselbe Tabelle künftig von beliebigen
-- Domänen genutzt werden kann, ohne dass jede Domäne ihre eigene
-- Protokolltabelle braucht.
--
-- Scope dieser Migration (Micro-Subtask 0.3.3.1): nur die generische
-- Infrastruktur. Die Anbindung weiterer Domänen (quotes, sales_orders,
-- purchase_orders, contacts, ...) folgt in separaten, noch offenen
-- Micro-Subtasks (siehe docs/backlog.md) - diese Tabelle ist bewusst so
-- generisch gestaltet, dass dafür keine weiteren Schemaänderungen nötig
-- sein sollten.

CREATE TABLE IF NOT EXISTS entity_change_log (
    id UUID PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    entity_type text NOT NULL,
    entity_id text NOT NULL,
    action text NOT NULL,
    actor_user_id text REFERENCES users(id) ON DELETE SET NULL,
    before_data jsonb,
    after_data jsonb,
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_entity_change_log_entity ON entity_change_log(entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_entity_change_log_company ON entity_change_log(company_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS entity_change_log;
-- DATENVERLUSTRISIKO: GoBD-relevante Nachweise (wer hat wann was geaendert)
-- gehen beim Rueckbau vollstaendig und unwiederbringlich verloren.
