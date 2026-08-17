package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidCredentials = errors.New("ungueltige anmeldedaten")
var ErrUserLocked = errors.New("benutzer ist gesperrt")
var ErrUserInactive = errors.New("benutzer ist deaktiviert")
var ErrSessionNotFound = errors.New("session nicht gefunden")
var ErrInvalidToken = errors.New("ungueltiges token")
var ErrRateLimited = errors.New("zu viele anmeldeversuche")
var ErrUserNotFound = errors.New("Benutzer nicht gefunden")

type Repository struct {
	pg *pgxpool.Pool
}

func NewRepository(pg *pgxpool.Pool) *Repository {
	return &Repository{pg: pg}
}

func (r *Repository) FindUserByLogin(ctx context.Context, login string) (*User, error) {
	login = strings.TrimSpace(strings.ToLower(login))
	if login == "" {
		return nil, ErrInvalidCredentials
	}
	var u User
	err := r.pg.QueryRow(ctx, `
        SELECT id, email, COALESCE(username,''), password_hash, COALESCE(first_name,''), COALESCE(last_name,''),
               COALESCE(display_name,''), locale, timezone, is_active, is_locked, COALESCE(company_id,''), branch_id,
               last_login_at, password_changed_at, created_at, updated_at
          FROM users
         WHERE LOWER(email)=$1 OR LOWER(COALESCE(username,''))=$1
    `, login).Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.DisplayName, &u.Locale, &u.Timezone, &u.IsActive, &u.IsLocked, &u.CompanyID, &u.BranchID,
		&u.LastLoginAt, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var u User
	err := r.pg.QueryRow(ctx, `
        SELECT id, email, COALESCE(username,''), password_hash, COALESCE(first_name,''), COALESCE(last_name,''),
               COALESCE(display_name,''), locale, timezone, is_active, is_locked, COALESCE(company_id,''), branch_id,
               last_login_at, password_changed_at, created_at, updated_at
          FROM users
         WHERE id=$1
    `, userID).Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.DisplayName, &u.Locale, &u.Timezone, &u.IsActive, &u.IsLocked, &u.CompanyID, &u.BranchID,
		&u.LastLoginAt, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) ListRoleCodesByUserID(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pg.Query(ctx, `
        SELECT ro.code
          FROM user_roles ur
          JOIN roles ro ON ro.id = ur.role_id
         WHERE ur.user_id=$1
         ORDER BY ro.code ASC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, nil
}

func (r *Repository) ListPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pg.Query(ctx, `
        SELECT DISTINCT p.code
          FROM user_roles ur
          JOIN role_permissions rp ON rp.role_id = ur.role_id
          JOIN permissions p ON p.id = rp.permission_id
         WHERE ur.user_id=$1
         ORDER BY p.code ASC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, nil
}

func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email)=LOWER($1))`, email).Scan(&exists)
	return exists, err
}

func (r *Repository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username)=LOWER($1))`, username).Scan(&exists)
	return exists, err
}

// CreateUser legt einen neuen Benutzer an (Subtask 0.5.4.1, ersetzt die
// bisher einzige Moeglichkeit per Direkt-SQL, siehe testutil.SeedAuthUser).
func (r *Repository) CreateUser(ctx context.Context, u User) (*User, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	var username any
	if u.Username != "" {
		username = u.Username
	}
	var branchID any
	if u.BranchID != nil && *u.BranchID != "" {
		branchID = *u.BranchID
	}
	_, err := r.pg.Exec(ctx, `
        INSERT INTO users (
            id, email, username, password_hash, first_name, last_name, display_name,
            locale, timezone, is_active, is_locked, company_id, branch_id, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)
    `, id, u.Email, username, u.PasswordHash, u.FirstName, u.LastName, u.DisplayName,
		u.Locale, u.Timezone, u.IsActive, u.IsLocked, u.CompanyID, branchID, now)
	if err != nil {
		return nil, err
	}
	return r.GetUserByID(ctx, id)
}

// SetUserLocked schaltet is_locked fuer einen Benutzer um (Subtask 0.5.4.2).
// Mandanten-scoped ueber die WHERE-Klausel: ein Zugriff auf einen Benutzer
// eines anderen Mandanten liefert ErrUserNotFound statt die Existenz des
// fremden Kontos zu verraten (konsistent mit dem Mandanten-Scoping-Muster
// aus Task 0.2 in den anderen Domaenen).
func (r *Repository) SetUserLocked(ctx context.Context, userID string, locked bool, companyID string) (*User, error) {
	tag, err := r.pg.Exec(ctx, `
        UPDATE users SET is_locked=$1, updated_at=now() WHERE id=$2 AND company_id=$3
    `, locked, userID, companyID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}
	return r.GetUserByID(ctx, userID)
}

func (r *Repository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.pg.Query(ctx, `SELECT id, code, name, description, is_system FROM roles ORDER BY code ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Role, 0)
	for rows.Next() {
		var ro Role
		if err := rows.Scan(&ro.ID, &ro.Code, &ro.Name, &ro.Description, &ro.IsSystem); err != nil {
			return nil, err
		}
		out = append(out, ro)
	}
	return out, nil
}

// ReplaceUserRoles ersetzt die komplette Rollenzuweisung eines Benutzers
// (Subtask 0.5.4.3). Mandanten-scoped (Zielbenutzer muss zum Mandanten des
// Aufrufers gehoeren, sonst ErrUserNotFound - gleiches Existenz-Leak-Schutz-
// Muster wie SetUserLocked), validiert alle Rollen-Codes VOR jeder
// Aenderung (alles-oder-nichts statt teilweise angewendeter Zuweisung bei
// einem Tippfehler in der Mitte der Liste), und fuehrt Delete+Insert
// atomar in einer Transaktion aus.
func (r *Repository) ReplaceUserRoles(ctx context.Context, userID string, roleCodes []string, companyID string, assignedBy string) ([]string, error) {
	var exists bool
	if err := r.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND company_id=$2)`, userID, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	if len(roleCodes) > 0 {
		rows, err := r.pg.Query(ctx, `SELECT code FROM roles WHERE code = ANY($1)`, roleCodes)
		if err != nil {
			return nil, err
		}
		found := make(map[string]bool, len(roleCodes))
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				rows.Close()
				return nil, err
			}
			found[code] = true
		}
		rows.Close()
		for _, code := range roleCodes {
			if !found[code] {
				return nil, fmt.Errorf("ungültiger Rollen-Code: %s", code)
			}
		}
	}

	tx, err := r.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
		return nil, err
	}
	if len(roleCodes) > 0 {
		var assignedByArg any
		if assignedBy != "" {
			assignedByArg = assignedBy
		}
		if _, err := tx.Exec(ctx, `
            INSERT INTO user_roles (user_id, role_id, assigned_at, assigned_by)
            SELECT $1, r.id, now(), $2 FROM roles r WHERE r.code = ANY($3)
        `, userID, assignedByArg, roleCodes); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.ListRoleCodesByUserID(ctx, userID)
}

func (r *Repository) UpdateLastLogin(ctx context.Context, userID string, at time.Time) error {
	_, err := r.pg.Exec(ctx, `UPDATE users SET last_login_at=$1, updated_at=$1 WHERE id=$2`, at, userID)
	return err
}

func (r *Repository) InsertAuditEvent(ctx context.Context, ev AuditEvent) error {
	id := uuid.NewString()
	_, err := r.pg.Exec(ctx, `
        INSERT INTO auth_audit_log (id, user_id, event_type, ip_address, user_agent, success, message)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, id, ev.UserID, ev.EventType, ev.IPAddress, ev.UserAgent, ev.Success, ev.Message)
	return err
}
