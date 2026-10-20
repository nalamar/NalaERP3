-- Erster Admin bei Neuinstallation, Passwortwechsel erzwingen (Backlog
-- 0.49, Auftrag des Nutzers vom 2026-10-19: "Bei Neuinstallation Admin
-- User mit User: Admin Passwort: Admin und 'Muss Passwort bei ersten Login
-- aendern' anlegen").
--
-- must_change_password: solange true, verweigert der Server jede
-- geschuetzte Route ausser Passwortwechsel, /auth/me und Abmelden
-- (403 password_change_required). Gesetzt beim Bootstrap-Admin
-- (auth.EnsureBootstrapAdmin), geloescht durch den Passwortwechsel.
-- Bestehende Benutzer bleiben unberuehrt (false).
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password boolean NOT NULL DEFAULT false;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE users DROP COLUMN IF EXISTS must_change_password;
-- RISIKO: Ein Bootstrap-Admin, der sein Passwort noch nicht geaendert hat,
-- waere danach mit "Admin"/"Admin" ohne Sperre benutzbar - vorher das
-- Passwort aendern oder den Benutzer sperren.
