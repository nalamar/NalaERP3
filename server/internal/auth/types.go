package auth

import "time"

// User is the technical identity used for login and authorization.
type User struct {
    ID                string    `json:"id"`
    Email             string    `json:"email"`
    Username          string    `json:"username"`
    PasswordHash      string    `json:"-"`
    FirstName         string    `json:"first_name"`
    LastName          string    `json:"last_name"`
    DisplayName       string    `json:"display_name"`
    Locale            string    `json:"locale"`
    Timezone          string    `json:"timezone"`
    IsActive          bool      `json:"is_active"`
    IsLocked          bool      `json:"is_locked"`
    LastLoginAt       *time.Time `json:"last_login_at"`
    PasswordChangedAt *time.Time `json:"password_changed_at"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

type Role struct {
    ID          string `json:"id"`
    Code        string `json:"code"`
    Name        string `json:"name"`
    Description string `json:"description"`
    IsSystem    bool   `json:"is_system"`
}

type Permission struct {
    ID          string `json:"id"`
    Code        string `json:"code"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Context     string `json:"context"`
}

type Session struct {
    SessionID  string    `json:"session_id"`
    UserID     string    `json:"user_id"`
    Email      string    `json:"email"`
    Roles      []string  `json:"roles"`
    IssuedAt   time.Time `json:"issued_at"`
    ExpiresAt  time.Time `json:"expires_at"`
    LastSeenAt time.Time `json:"last_seen_at"`
    IPAddress  string    `json:"ip_address"`
    UserAgent  string    `json:"user_agent"`
}

type LoginInput struct {
    Login     string
    Password  string
    IPAddress string
    UserAgent string
}

type TokenPair struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    ExpiresAt    time.Time `json:"expires_at"`
    SessionID    string    `json:"session_id"`
}

type AuthResult struct {
    User        User      `json:"user"`
    Roles       []string  `json:"roles"`
    Permissions []string  `json:"permissions"`
    Tokens      TokenPair `json:"tokens"`
}

type Claims struct {
    Subject   string    `json:"sub"`
    SessionID string    `json:"sid"`
    TokenID   string    `json:"jti"`
    TokenType string    `json:"typ"`
    Roles     []string  `json:"roles"`
    IssuedAt  time.Time `json:"iat"`
    ExpiresAt time.Time `json:"exp"`
}

type AuditEvent struct {
    UserID    *string
    EventType string
    IPAddress string
    UserAgent string
    Success   bool
    Message   string
}
