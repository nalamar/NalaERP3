package auth

import (
    "context"
    "errors"
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
               COALESCE(display_name,''), locale, timezone, is_active, is_locked, last_login_at,
               password_changed_at, created_at, updated_at
          FROM users
         WHERE LOWER(email)=$1 OR LOWER(COALESCE(username,''))=$1
    `, login).Scan(
        &u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName,
        &u.DisplayName, &u.Locale, &u.Timezone, &u.IsActive, &u.IsLocked, &u.LastLoginAt,
        &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt,
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
               COALESCE(display_name,''), locale, timezone, is_active, is_locked, last_login_at,
               password_changed_at, created_at, updated_at
          FROM users
         WHERE id=$1
    `, userID).Scan(
        &u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName,
        &u.DisplayName, &u.Locale, &u.Timezone, &u.IsActive, &u.IsLocked, &u.LastLoginAt,
        &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt,
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
