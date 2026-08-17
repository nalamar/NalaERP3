package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"nalaerp3/internal/config"
)

type Service struct {
	repo        *Repository
	store       *SessionStore
	jwtSecret   []byte
	accessTTL   time.Duration
	sessionTTL  time.Duration
	rateLimiter *LoginRateLimiter
}

func NewService(repo *Repository, store *SessionStore, cfg *config.Config) *Service {
	return &Service{
		repo:       repo,
		store:      store,
		jwtSecret:  []byte(cfg.JWTSecret),
		accessTTL:  time.Duration(cfg.AccessTokenTTLMinutes) * time.Minute,
		sessionTTL: time.Duration(cfg.SessionTTLHours) * time.Hour,
	}
}

// WithRateLimiter ergaenzt einen optionalen Brute-Force-Schutz fuer Login
// (Subtask 0.5.2). Ohne Aufruf bleibt das Verhalten unveraendert (kein Limit).
func (s *Service) WithRateLimiter(l *LoginRateLimiter) *Service {
	s.rateLimiter = l
	return s
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*AuthResult, error) {
	now := time.Now().UTC()
	if blocked, err := s.rateLimiter.Blocked(ctx, in.IPAddress); err != nil {
		return nil, err
	} else if blocked {
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			EventType: "login.failed",
			IPAddress: in.IPAddress,
			UserAgent: in.UserAgent,
			Success:   false,
			Message:   "rate limit ueberschritten",
		})
		return nil, ErrRateLimited
	}
	user, err := s.repo.FindUserByLogin(ctx, in.Login)
	if err != nil {
		_ = s.rateLimiter.RegisterFailure(ctx, in.IPAddress)
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			EventType: "login.failed",
			IPAddress: in.IPAddress,
			UserAgent: in.UserAgent,
			Success:   false,
			Message:   "benutzer nicht gefunden",
		})
		return nil, err
	}
	if !user.IsActive {
		_ = s.rateLimiter.RegisterFailure(ctx, in.IPAddress)
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			UserID:    &user.ID,
			EventType: "login.failed",
			IPAddress: in.IPAddress,
			UserAgent: in.UserAgent,
			Success:   false,
			Message:   "benutzer deaktiviert",
		})
		return nil, ErrUserInactive
	}
	if user.IsLocked {
		_ = s.rateLimiter.RegisterFailure(ctx, in.IPAddress)
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			UserID:    &user.ID,
			EventType: "login.failed",
			IPAddress: in.IPAddress,
			UserAgent: in.UserAgent,
			Success:   false,
			Message:   "benutzer gesperrt",
		})
		return nil, ErrUserLocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		_ = s.rateLimiter.RegisterFailure(ctx, in.IPAddress)
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			UserID:    &user.ID,
			EventType: "login.failed",
			IPAddress: in.IPAddress,
			UserAgent: in.UserAgent,
			Success:   false,
			Message:   "ungueltiges passwort",
		})
		return nil, ErrInvalidCredentials
	}
	_ = s.rateLimiter.Reset(ctx, in.IPAddress)

	roles, err := s.repo.ListRoleCodesByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.repo.ListPermissionCodesByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	sessionID := uuid.NewString()
	session := Session{
		SessionID:  sessionID,
		UserID:     user.ID,
		Email:      user.Email,
		Roles:      roles,
		IssuedAt:   now,
		ExpiresAt:  now.Add(s.sessionTTL),
		LastSeenAt: now,
		IPAddress:  in.IPAddress,
		UserAgent:  in.UserAgent,
	}
	if err := s.store.Save(ctx, session, s.sessionTTL); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateLastLogin(ctx, user.ID, now); err != nil {
		return nil, err
	}

	tokens, err := s.issueTokenPair(user.ID, sessionID, roles, now)
	if err != nil {
		return nil, err
	}
	_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
		UserID:    &user.ID,
		EventType: "login.success",
		IPAddress: in.IPAddress,
		UserAgent: in.UserAgent,
		Success:   true,
		Message:   "anmeldung erfolgreich",
	})
	return &AuthResult{
		User:        *user,
		Roles:       roles,
		Permissions: permissions,
		Tokens:      tokens,
	}, nil
}

// CreateUser legt einen neuen Benutzer innerhalb des Mandanten des Aufrufers
// an (Subtask 0.5.4.1). Rollenzuweisung ist bewusst nicht Teil dieser
// Funktion (eigene Subtask 0.5.4.3) - ein neu angelegter Nutzer hat
// zunaechst keine Rollen/Berechtigungen.
func (s *Service) CreateUser(ctx context.Context, in UserCreate, companyID string) (*User, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("gueltige E-Mail-Adresse erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("mandant erforderlich")
	}
	passwordHash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	exists, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("Benutzer mit dieser E-Mail-Adresse bereits vorhanden")
	}
	username := strings.TrimSpace(in.Username)
	if username != "" {
		taken, err := s.repo.UsernameExists(ctx, username)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, errors.New("Benutzername bereits vorhanden")
		}
	}
	locale := strings.TrimSpace(in.Locale)
	if locale == "" {
		locale = "de-DE"
	}
	timezone := strings.TrimSpace(in.Timezone)
	if timezone == "" {
		timezone = "Europe/Berlin"
	}
	firstName := strings.TrimSpace(in.FirstName)
	lastName := strings.TrimSpace(in.LastName)
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(firstName + " " + lastName)
	}
	u := User{
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
		FirstName:    firstName,
		LastName:     lastName,
		DisplayName:  displayName,
		Locale:       locale,
		Timezone:     timezone,
		IsActive:     true,
		IsLocked:     false,
		CompanyID:    companyID,
		BranchID:     in.BranchID,
	}
	return s.repo.CreateUser(ctx, u)
}

// SetUserLocked sperrt/entsperrt einen Benutzer innerhalb des Mandanten des
// Aufrufers (Subtask 0.5.4.2). Ein gesperrter Benutzer kann sich weder neu
// einloggen (Login prueft user.IsLocked) noch bestehende Access-Token weiter
// nutzen (AuthenticateAccessToken prueft user.IsLocked bei JEDEM Request neu
// gegen die DB, nicht nur beim Login) - die Sperre wirkt also spaetestens ab
// dem naechsten authentifizierten Request, nicht erst nach Tokenablauf.
func (s *Service) SetUserLocked(ctx context.Context, userID string, locked bool, companyID string) (*User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("benutzer-id erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("mandant erforderlich")
	}
	return s.repo.SetUserLocked(ctx, userID, locked, companyID)
}

// ListRoles liefert alle im System verfuegbaren Rollen (Subtask 0.5.4.3),
// z.B. fuer eine Rollenauswahl in einer kuenftigen Verwaltungs-UI.
func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.repo.ListRoles(ctx)
}

// ReplaceUserRoles ersetzt die komplette Rollenzuweisung eines Benutzers
// innerhalb des Mandanten des Aufrufers (Subtask 0.5.4.3, letzte Subtask
// von Task 0.5.4/Epic 0.5). Normalisiert die Eingabe (trimmt, verwirft
// Duplikate/Leerstrings), die eigentliche Existenz-/Mandanten- und
// Rollen-Code-Validierung passiert im Repository (dort ohnehin ein
// DB-Roundtrip noetig).
func (s *Service) ReplaceUserRoles(ctx context.Context, userID string, roleCodes []string, companyID string, assignedBy string) ([]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("benutzer-id erforderlich")
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("mandant erforderlich")
	}
	normalized := make([]string, 0, len(roleCodes))
	seen := make(map[string]bool, len(roleCodes))
	for _, code := range roleCodes {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		normalized = append(normalized, code)
	}
	return s.repo.ReplaceUserRoles(ctx, userID, normalized, companyID, assignedBy)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.parseToken(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}
	sess, err := s.store.Get(ctx, claims.SessionID)
	if err != nil {
		return nil, err
	}
	if sess.UserID != claims.Subject {
		return nil, ErrInvalidToken
	}
	now := time.Now().UTC()
	if now.After(sess.ExpiresAt) {
		_ = s.store.Delete(ctx, sess.SessionID, sess.UserID)
		return nil, ErrSessionNotFound
	}
	if err := s.store.Touch(ctx, *sess, time.Until(sess.ExpiresAt)); err != nil {
		return nil, err
	}
	pair, err := s.issueTokenPair(sess.UserID, sess.SessionID, sess.Roles, now)
	if err != nil {
		return nil, err
	}
	return &pair, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.parseToken(refreshToken, "refresh")
	if err != nil {
		return err
	}
	sess, err := s.store.Get(ctx, claims.SessionID)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, sess.SessionID, sess.UserID); err != nil {
		return err
	}
	_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
		UserID:    &sess.UserID,
		EventType: "logout",
		IPAddress: sess.IPAddress,
		UserAgent: sess.UserAgent,
		Success:   true,
		Message:   "abmeldung erfolgreich",
	})
	return nil
}

func (s *Service) AuthenticateAccessToken(ctx context.Context, accessToken string) (*Session, *User, []string, error) {
	claims, err := s.parseToken(accessToken, "access")
	if err != nil {
		return nil, nil, nil, err
	}
	sess, err := s.store.Get(ctx, claims.SessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	if sess.UserID != claims.Subject {
		return nil, nil, nil, ErrInvalidToken
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.store.Delete(ctx, sess.SessionID, sess.UserID)
		return nil, nil, nil, ErrSessionNotFound
	}
	user, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !user.IsActive || user.IsLocked {
		return nil, nil, nil, ErrInvalidCredentials
	}
	permissions, err := s.repo.ListPermissionCodesByUserID(ctx, user.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	return sess, user, permissions, nil
}

func (s *Service) issueTokenPair(userID, sessionID string, roles []string, now time.Time) (TokenPair, error) {
	accessClaims := Claims{
		Subject:   userID,
		SessionID: sessionID,
		TokenID:   uuid.NewString(),
		TokenType: "access",
		Roles:     roles,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.accessTTL),
	}
	refreshClaims := Claims{
		Subject:   userID,
		SessionID: sessionID,
		TokenID:   uuid.NewString(),
		TokenType: "refresh",
		Roles:     roles,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.sessionTTL),
	}
	accessToken, err := s.signToken(accessClaims)
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, err := s.signToken(refreshClaims)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresAt:    accessClaims.ExpiresAt,
		SessionID:    sessionID,
	}, nil
}

func (s *Service) signToken(cl Claims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"sub":   cl.Subject,
		"sid":   cl.SessionID,
		"jti":   cl.TokenID,
		"typ":   cl.TokenType,
		"roles": cl.Roles,
		"iat":   cl.IssuedAt.Unix(),
		"exp":   cl.ExpiresAt.Unix(),
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(hb) + "." + enc.EncodeToString(pb)
	mac := hmac.New(sha256.New, s.jwtSecret)
	_, _ = mac.Write([]byte(signingInput))
	sig := enc.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func (s *Service) parseToken(token, expectedType string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, s.jwtSecret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var raw struct {
		Subject   string   `json:"sub"`
		SessionID string   `json:"sid"`
		TokenID   string   `json:"jti"`
		TokenType string   `json:"typ"`
		Roles     []string `json:"roles"`
		IssuedAt  int64    `json:"iat"`
		ExpiresAt int64    `json:"exp"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, ErrInvalidToken
	}
	claims := &Claims{
		Subject:   raw.Subject,
		SessionID: raw.SessionID,
		TokenID:   raw.TokenID,
		TokenType: raw.TokenType,
		Roles:     raw.Roles,
		IssuedAt:  time.Unix(raw.IssuedAt, 0).UTC(),
		ExpiresAt: time.Unix(raw.ExpiresAt, 0).UTC(),
	}
	if claims.TokenType != expectedType {
		return nil, ErrInvalidToken
	}
	if time.Now().UTC().After(claims.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	if claims.Subject == "" || claims.SessionID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", errors.New("passwort erforderlich")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}
