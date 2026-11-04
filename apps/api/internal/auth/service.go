package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("reset link is invalid or expired")
	ErrUnauthenticated    = errors.New("unauthenticated")
)

// ValidationError is a request the client can correct.
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string { return e.Message }

// User is a panel account. Password hashes never leave the service.
type User struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

// Session is the raw cookie value plus its expiry. Only the hash is stored.
type Session struct {
	Raw       string
	ExpiresAt time.Time
}

// Service owns register, login, logout, and password reset.
type Service struct {
	db         *sql.DB
	params     Params
	sessionTTL time.Duration
	resetTTL   time.Duration
	now        func() time.Time
	dummy      string
}

func NewService(db *sql.DB, params Params, sessionTTL, resetTTL time.Duration) (*Service, error) {
	if params.Memory == 0 || params.Time == 0 || params.Threads == 0 {
		params = DefaultParams()
	}
	if params.SaltLen == 0 {
		params.SaltLen = 16
	}
	if params.KeyLen == 0 {
		params.KeyLen = 32
	}
	if sessionTTL <= 0 {
		sessionTTL = 7 * 24 * time.Hour
	}
	if resetTTL <= 0 {
		resetTTL = time.Hour
	}
	dummy, err := HashPassword("altas-timing-pad", params)
	if err != nil {
		return nil, err
	}
	return &Service{
		db:         db,
		params:     params,
		sessionTTL: sessionTTL,
		resetTTL:   resetTTL,
		now:        func() time.Time { return time.Now().UTC() },
		dummy:      dummy,
	}, nil
}

func (s *Service) Register(ctx context.Context, email, password string) (User, Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, Session{}, err
	}
	if err := validatePassword(password); err != nil {
		return User{}, Session{}, err
	}
	hash, err := HashPassword(password, s.params)
	if err != nil {
		return User{}, Session{}, err
	}
	now := s.now().Truncate(time.Second)
	user := User{ID: newID(), Email: email, CreatedAt: now}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		user.ID, user.Email, hash, formatTime(now), formatTime(now),
	)
	if err != nil {
		if isUnique(err) {
			return User{}, Session{}, ErrEmailTaken
		}
		return User{}, Session{}, err
	}
	sess, err := s.createSession(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, sess, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (User, Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, Session{}, err
	}
	if strings.TrimSpace(password) == "" {
		return User{}, Session{}, ValidationError{Message: "Password is required"}
	}
	var user User
	var hash, created string
	err = s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email = ?`, email,
	).Scan(&user.ID, &user.Email, &hash, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, _ = VerifyPassword(password, s.dummy)
			return User{}, Session{}, ErrInvalidCredentials
		}
		return User{}, Session{}, err
	}
	user.CreatedAt, err = parseTime(created)
	if err != nil {
		return User{}, Session{}, err
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		return User{}, Session{}, err
	}
	if !ok {
		return User{}, Session{}, ErrInvalidCredentials
	}
	sess, err := s.createSession(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, sess, nil
}

// Logout revokes the presented session. A missing cookie is a no-op.
func (s *Service) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`,
		formatTime(s.now()), hashToken(raw),
	)
	return err
}

func (s *Service) UserFromSession(ctx context.Context, raw string) (User, error) {
	if raw == "" {
		return User{}, ErrUnauthenticated
	}
	var user User
	var created string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?
	`, hashToken(raw), formatTime(s.now())).Scan(&user.ID, &user.Email, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUnauthenticated
		}
		return User{}, err
	}
	user.CreatedAt, err = parseTime(created)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// RequestReset stores a single-use token when the email is registered.
// The raw token is returned only to the caller; unknown emails return ok=false
// and no error so the HTTP layer can answer uniformly.
func (s *Service) RequestReset(ctx context.Context, email string) (raw string, ok bool, err error) {
	email, err = normalizeEmail(email)
	if err != nil {
		return "", false, err
	}
	var userID string
	err = s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	now := s.now()
	if _, err = s.db.ExecContext(ctx,
		`UPDATE password_resets SET used_at = ? WHERE user_id = ? AND used_at IS NULL`,
		formatTime(now), userID,
	); err != nil {
		return "", false, err
	}
	raw, err = newToken()
	if err != nil {
		return "", false, err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO password_resets (id, user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		newID(), userID, hashToken(raw), formatTime(now), formatTime(now.Add(s.resetTTL)),
	)
	if err != nil {
		return "", false, err
	}
	return raw, true, nil
}

// ResetPassword consumes a token, replaces the password, and revokes sessions.
func (s *Service) ResetPassword(ctx context.Context, rawToken, password string) error {
	if strings.TrimSpace(rawToken) == "" {
		return ErrInvalidToken
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	now := s.now()
	var resetID, userID string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id FROM password_resets
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
	`, hashToken(rawToken), formatTime(now)).Scan(&resetID, &userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidToken
		}
		return err
	}
	hash, err := HashPassword(password, s.params)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, formatTime(now), userID,
	); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE password_resets SET used_at = ? WHERE id = ? AND used_at IS NULL`,
		formatTime(now), resetID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInvalidToken
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
		formatTime(now), userID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) createSession(ctx context.Context, userID string) (Session, error) {
	raw, err := newToken()
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	exp := now.Add(s.sessionTTL).Truncate(time.Second)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		hashToken(raw), userID, formatTime(now), formatTime(exp),
	)
	if err != nil {
		return Session{}, err
	}
	return Session{Raw: raw, ExpiresAt: exp}, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return "", ValidationError{Message: "Enter a valid email address"}
	}
	return email, nil
}

func validEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	domain := email[at+1:]
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !strings.Contains(domain, ".") {
		return false
	}
	if strings.Contains(email[:at], "@") {
		return false
	}
	return true
}

func validatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < 8 {
		return ValidationError{Message: "Password must be at least 8 characters"}
	}
	if n > 128 {
		return ValidationError{Message: "Password must be at most 128 characters"}
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp: %w", err)
	}
	return t.UTC(), nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func isUnique(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
