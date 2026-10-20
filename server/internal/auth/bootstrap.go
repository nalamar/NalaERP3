package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Erster Admin bei Neuinstallation und Passwortwechsel (Backlog 0.49).

// Bootstrap-Zugang. Bekannt und deshalb nur zusammen mit dem erzwungenen
// Passwortwechsel vertretbar: bis zum Wechsel ist jede geschuetzte Route
// gesperrt.
const (
	BootstrapAdminUsername = "Admin"
	BootstrapAdminPassword = "Admin"
	// .invalid ist als Top-Level-Domain reserviert (RFC 2606) - die
	// Adresse kann nie zustellbar sein.
	BootstrapAdminEmail = "admin@localhost.invalid"
	// MinPasswordLength gilt fuer jedes neu gesetzte Passwort.
	MinPasswordLength = 10
)

// bootstrapLockKey serialisiert den Bootstrap gleichzeitig startender
// Server (pg_advisory_xact_lock).
const bootstrapLockKey = 4900049

// EnsureBootstrapAdmin legt den ersten Admin an, wenn es UEBERHAUPT KEINEN
// Benutzer gibt. Bei vorhandenen Benutzern geschieht nichts - insbesondere
// wird kein Passwort zurueckgesetzt. created meldet, ob angelegt wurde.
func EnsureBootstrapAdmin(ctx context.Context, pg *pgxpool.Pool) (created bool, err error) {
	tx, err := pg.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, bootstrapLockKey); err != nil {
		return false, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	hash, err := HashPassword(BootstrapAdminPassword)
	if err != nil {
		return false, err
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
        INSERT INTO users (id, email, username, password_hash, first_name, last_name, display_name, locale, timezone,
                           is_active, is_locked, company_id, must_change_password, created_at, updated_at)
        VALUES ($1,$2,$3,$4,'','','Administrator','de-DE','Europe/Berlin',true,false,'default',true,$5,$5)
    `, id, BootstrapAdminEmail, BootstrapAdminUsername, hash, now); err != nil {
		return false, err
	}
	ct, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE code='admin'`, id)
	if err != nil {
		return false, err
	}
	if ct.RowsAffected() != 1 {
		return false, errors.New("Rolle admin nicht gefunden")
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Fehler des Passwortwechsels. Die Meldungen enthalten "ungültig" bzw.
// "erforderlich", damit sie ueber HTTP als 400 ankommen.
var (
	ErrWrongCurrentPassword = errors.New("Ungültiges aktuelles Passwort")
	ErrPasswordTooShort     = errors.New("Ungültiges neues Passwort: mindestens 10 Zeichen")
	ErrPasswordUnchanged    = errors.New("Ungültiges neues Passwort: es muss sich vom bisherigen unterscheiden")
)

// ChangePassword setzt ein neues Passwort, wenn das aktuelle stimmt, und
// hebt die Pflicht zum Wechsel auf.
func (s *Service) ChangePassword(ctx context.Context, userID, current, next string) error {
	if strings.TrimSpace(current) == "" || strings.TrimSpace(next) == "" {
		return errors.New("Aktuelles und neues Passwort erforderlich")
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{UserID: &user.ID, EventType: "password.change_failed", Success: false, Message: "aktuelles passwort falsch"})
		return ErrWrongCurrentPassword
	}
	if len([]rune(strings.TrimSpace(next))) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(strings.TrimSpace(next))) == nil {
		return ErrPasswordUnchanged
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.repo.SetPassword(ctx, user.ID, hash, time.Now().UTC()); err != nil {
		return err
	}
	_ = s.repo.InsertAuditEvent(ctx, AuditEvent{UserID: &user.ID, EventType: "password.changed", Success: true, Message: "passwort geaendert"})
	return nil
}
