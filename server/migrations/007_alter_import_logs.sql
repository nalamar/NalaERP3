-- Add before/after snapshots to support undo
ALTER TABLE project_import_changes
    ADD COLUMN IF NOT EXISTS before_data JSONB,
    ADD COLUMN IF NOT EXISTS after_data JSONB;

