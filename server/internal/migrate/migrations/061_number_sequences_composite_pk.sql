-- number_sequences pro Mandant statt global (ADR 0002, Task 0.2.2.3).
--
-- Migration 060 hat company_id bereits als NULLABLE Spalte ergaenzt und
-- alle bestehenden Zeilen auf 'default' zurueckgeschrieben, die
-- PK-Umstellung selbst aber bewusst auf NACH Task 0.2.2 verschoben (ein
-- zusammengesetzter PK erzwingt company_id NOT NULL, und der
-- Anwendungscode fuellte company_id vorher noch nicht - siehe
-- docs/adr/0002-mandanten-standort-scoping.md). Task 0.2.2.1
-- (Anwendungscode-Scoping aller Domaenen) ist jetzt abgeschlossen, und mit
-- Task 0.2.2.3 wird settings.NumberingService selbst auf company_id
-- umgestellt (siehe zugehoerige Anwendungscode-Aenderung) - diese
-- Migration ist der dazugehoerige Contract-Schritt.
--
-- Reihenfolge: NOT NULL erzwingen (alle Zeilen sind laut 060 bereits
-- 'default', kein Datenverlust), alten PK auf entity allein droppen, neuen
-- zusammengesetzten PK auf (company_id, entity) anlegen. Der bisherige
-- unterstuetzende Index idx_number_sequences_company_id wird ueberfluessig
-- (der neue PK deckt company_id als fuehrende Spalte ab) und wird entfernt.

ALTER TABLE number_sequences
    ALTER COLUMN company_id SET NOT NULL;

ALTER TABLE number_sequences
    DROP CONSTRAINT number_sequences_pkey;

ALTER TABLE number_sequences
    ADD PRIMARY KEY (company_id, entity);

DROP INDEX IF EXISTS idx_number_sequences_company_id;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE number_sequences DROP CONSTRAINT number_sequences_pkey;
--   CREATE INDEX idx_number_sequences_company_id ON number_sequences(company_id);
--   ALTER TABLE number_sequences ADD PRIMARY KEY (entity);
--   ALTER TABLE number_sequences ALTER COLUMN company_id DROP NOT NULL;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und eigene
-- Nummernkreise fuer denselben entity-Wert (z. B. 'quote') angelegt hat,
-- verletzt ein Rueckbau auf den alten entity-only-PK die Eindeutigkeit
-- (zwei Zeilen mit demselben entity, unterschiedlichem company_id) - der
-- DOWN-Schritt "ADD PRIMARY KEY (entity)" wuerde dann fehlschlagen und
-- muesste die Duplikate vorher manuell zusammenfuehren.
