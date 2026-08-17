package apihttp

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/auth"
)

// tenantCreateInput ist der Request-Body fuer POST /platform/tenants
// (Backlog 0.32, Subtask 0.32.2b: Mandanten-Onboarding).
type tenantCreateInput struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name"`
	AdminEmail       string `json:"admin_email"`
	AdminPassword    string `json:"admin_password"`
	AdminDisplayName string `json:"admin_display_name,omitempty"`
}

type tenantCreateResult struct {
	CompanyID   string `json:"company_id"`
	Name        string `json:"name"`
	AdminUserID string `json:"admin_user_id"`
	AdminEmail  string `json:"admin_email"`
}

var tenantSlugInvalidChars = regexp.MustCompile(`[^a-z0-9]+`)

// slugifyTenantID leitet aus einem Firmennamen eine URL-/ID-taugliche
// Mandanten-ID ab (Kleinbuchstaben, Ziffern, einzelne Bindestriche). Wird nur
// verwendet, wenn der Aufrufer keine explizite ID angibt.
func slugifyTenantID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = tenantSlugInvalidChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// createTenant legt atomar einen neuen Mandanten an: company_profiles-Zeile,
// eine Kopie des Kontenrahmens der 'default'-Company (siehe Backlog 0.32,
// Subtask 0.32.2a fuer die dafuer notwendige company_id-Primaerschluessel-
// Umstellung von accounts) sowie den ersten Admin-Benutzer samt admin-Rolle.
// tax_codes/roles/permissions sind global und brauchen kein Seeding;
// number_sequences initialisiert sich selbst beim ersten Next()-Aufruf
// (siehe settings/numbering.go).
func createTenant(ctx context.Context, pg *pgxpool.Pool, in tenantCreateInput) (*tenantCreateResult, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("Firmenname erforderlich")
	}
	companyID := strings.TrimSpace(in.ID)
	if companyID == "" {
		companyID = slugifyTenantID(name)
	}
	if companyID == "" {
		return nil, errors.New("Mandanten-ID konnte nicht aus dem Firmennamen abgeleitet werden, bitte explizit angeben")
	}

	email := strings.ToLower(strings.TrimSpace(in.AdminEmail))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("gueltige E-Mail-Adresse fuer den Erst-Administrator erforderlich")
	}
	passwordHash, err := auth.HashPassword(in.AdminPassword)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(in.AdminDisplayName)
	if displayName == "" {
		displayName = name + " Admin"
	}

	tx, err := pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM company_profiles WHERE id=$1)`, companyID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("Mandant mit dieser ID existiert bereits")
	}

	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email)=$1)`, email).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("Benutzer mit dieser E-Mail-Adresse bereits vorhanden")
	}

	if _, err := tx.Exec(ctx, `INSERT INTO company_profiles (id, name, updated_at) VALUES ($1, $2, now())`, companyID, name); err != nil {
		return nil, err
	}

	// Kontenrahmen-Vorlage: Kopie aller aktuellen 'default'-Konten fuer den
	// neuen Mandanten. parent_code ist in der Praxis immer NULL (kein
	// Anwendungscode setzt es je, siehe Backlog 0.32.2a), wird hier nur der
	// Vollstaendigkeit halber mitkopiert.
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounts (code, name, type, parent_code, tax_code, is_active, company_id)
		SELECT code, name, type, parent_code, tax_code, is_active, $1
		FROM accounts
		WHERE company_id = 'default'
	`, companyID); err != nil {
		return nil, err
	}

	userID := uuid.NewString()
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (
			id, email, password_hash, first_name, last_name, display_name,
			locale, timezone, is_active, is_locked, company_id, created_at, updated_at
		) VALUES ($1,$2,$3,'','',$4,'de-DE','Europe/Berlin',true,false,$5,$6,$6)
	`, userID, email, passwordHash, displayName, companyID, now); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, r.id FROM roles r WHERE r.code = 'admin'
	`, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &tenantCreateResult{
		CompanyID:   companyID,
		Name:        name,
		AdminUserID: userID,
		AdminEmail:  email,
	}, nil
}
