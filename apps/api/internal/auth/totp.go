package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

var enrollFailures sync.Map

func enrollBump(userID string) int {
	current, _ := enrollFailures.LoadOrStore(userID, 0)
	n, _ := current.(int)
	n++
	enrollFailures.Store(userID, n)
	return n
}

const (
	totpDigits     = 6
	totpPeriod     = 30 * time.Second
	totpSkew       = 1
	maxLoginTries  = 5
	lockoutPeriod  = 15 * time.Minute
	challengeTTL   = 5 * time.Minute
	trustTTL       = 30 * 24 * time.Hour
	recoveryCount  = 10
	enrollAttempts = 8
)

// ErrLocked is returned when sign-in is inside a brute-force lockout.
var ErrLocked = errors.New("sign-in locked")

// LockedError tells the caller how long to wait.
type LockedError struct {
	RetryAfter time.Duration
}

func (e LockedError) Error() string { return "too many failed attempts" }

func (e LockedError) Unwrap() error { return ErrLocked }

// CredentialError is a failed password or code. AttemptsRemaining counts down to a lockout.
type CredentialError struct {
	AttemptsRemaining int
}

func (e CredentialError) Error() string { return ErrInvalidCredentials.Error() }

func (e CredentialError) Unwrap() error { return ErrInvalidCredentials }

// ErrInvalidCode is a rejected authenticator or recovery code.
var ErrInvalidCode = errors.New("invalid authentication code")

func (s *Service) loginLocked(ctx context.Context, email string) (bool, time.Duration, error) {
	var until string
	err := s.db.QueryRowContext(ctx, `SELECT locked_until FROM login_guards WHERE email = ?`, email).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) || strings.TrimSpace(until) == "" {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	t, err := parseTime(until)
	if err != nil {
		return false, 0, nil
	}
	wait := t.Sub(s.now())
	if wait <= 0 {
		return false, 0, nil
	}
	return true, wait, nil
}

func (s *Service) noteLoginFailure(ctx context.Context, email string) error {
	var failures int
	var until string
	err := s.db.QueryRowContext(ctx, `SELECT failures, locked_until FROM login_guards WHERE email = ?`, email).Scan(&failures, &until)
	if errors.Is(err, sql.ErrNoRows) {
		failures = 0
	} else if err != nil {
		return err
	}
	failures++
	if failures >= maxLoginTries {
		locked := s.now().Add(lockoutPeriod)
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO login_guards (email, failures, locked_until) VALUES (?, ?, ?)
			ON CONFLICT(email) DO UPDATE SET failures = excluded.failures, locked_until = excluded.locked_until
		`, email, failures, formatTime(locked))
		if err != nil {
			return err
		}
		_ = s.recordAuthEvent(ctx, "sign_in_locked", "Sign-in locked for 15 minutes after repeated failures for "+email)
		return LockedError{RetryAfter: lockoutPeriod}
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO login_guards (email, failures, locked_until) VALUES (?, ?, '')
		ON CONFLICT(email) DO UPDATE SET failures = excluded.failures, locked_until = ''
	`, email, failures)
	if err != nil {
		return err
	}
	return CredentialError{AttemptsRemaining: maxLoginTries - failures}
}

func (s *Service) clearLoginFailures(ctx context.Context, email string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM login_guards WHERE email = ?`, email)
	return err
}

func (s *Service) recordSignIn(ctx context.Context, userID, place string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET last_sign_in_at = ?, last_sign_in_place = ?, updated_at = ? WHERE id = ?
	`, formatTime(s.now()), place, formatTime(s.now()), userID)
	return err
}

func (s *Service) recordAuthEvent(ctx context.Context, kind, detail string) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO auth_events (at, kind, detail) VALUES (?, ?, ?)`, formatTime(s.now()), kind, detail); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM auth_events WHERE id NOT IN (SELECT id FROM auth_events ORDER BY id DESC LIMIT 200)
	`)
	return err
}

func (s *Service) beginChallenge(ctx context.Context, userID string, ttl time.Duration, persistent, remember bool, place string) (string, error) {
	raw, err := newToken()
	if err != nil {
		return "", err
	}
	rememberBit := 0
	if remember {
		rememberBit = 1
	}
	persistentBit := 0
	if persistent {
		persistentBit = 1
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO totp_challenges (token_hash, user_id, expires_at, attempts, remember, persistent, ttl_seconds, place)
		VALUES (?, ?, ?, 0, ?, ?, ?, ?)
	`, hashToken(raw), userID, formatTime(s.now().Add(challengeTTL)), rememberBit, persistentBit, int(ttl.Seconds()), place)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) loadChallenge(ctx context.Context, raw string) (userID string, attempts int, ttl time.Duration, persistent bool, place string, err error) {
	if strings.TrimSpace(raw) == "" {
		return "", 0, 0, false, "", ErrInvalidCode
	}
	var persistentBit, seconds int
	var expires string
	err = s.db.QueryRowContext(ctx, `
		SELECT user_id, expires_at, attempts, persistent, ttl_seconds, place
		FROM totp_challenges WHERE token_hash = ?
	`, hashToken(raw)).Scan(&userID, &expires, &attempts, &persistentBit, &seconds, &place)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, 0, false, "", ErrInvalidCode
	}
	if err != nil {
		return "", 0, 0, false, "", err
	}
	exp, err := parseTime(expires)
	if err != nil || !exp.After(s.now()) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM totp_challenges WHERE token_hash = ?`, hashToken(raw))
		return "", 0, 0, false, "", ErrInvalidCode
	}
	return userID, attempts, time.Duration(seconds) * time.Second, persistentBit != 0, place, nil
}

func (s *Service) failChallenge(ctx context.Context, raw, email string, attempts int) error {
	attempts++
	if attempts >= maxLoginTries {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM totp_challenges WHERE token_hash = ?`, hashToken(raw))
		locked := s.now().Add(lockoutPeriod)
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO login_guards (email, failures, locked_until) VALUES (?, ?, ?)
			ON CONFLICT(email) DO UPDATE SET failures = excluded.failures, locked_until = excluded.locked_until
		`, email, maxLoginTries, formatTime(locked))
		_ = s.recordAuthEvent(ctx, "totp_failed", "Authenticator locked for "+email)
		return LockedError{RetryAfter: lockoutPeriod}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE totp_challenges SET attempts = ? WHERE token_hash = ?`, attempts, hashToken(raw))
	if err != nil {
		return err
	}
	return CredentialError{AttemptsRemaining: maxLoginTries - attempts}
}

// CompleteTOTP checks a challenge and, on success, opens a session.
// The returned trust token is set only when trust is true.
func (s *Service) CompleteTOTP(ctx context.Context, challenge, code string, trust bool) (User, Session, string, error) {
	userID, attempts, ttl, persistent, place, err := s.loadChallenge(ctx, challenge)
	if err != nil {
		return User{}, Session{}, "", err
	}
	user, secret, email, err := s.userSecret(ctx, userID)
	if err != nil {
		return User{}, Session{}, "", err
	}
	if !verifyTOTP(secret, code, s.now()) {
		_ = s.recordAuthEvent(ctx, "totp_failed", "Rejected authenticator code for "+email)
		return User{}, Session{}, "", s.failChallenge(ctx, challenge, email, attempts)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM totp_challenges WHERE token_hash = ?`, hashToken(challenge))
	_ = s.clearLoginFailures(ctx, email)
	outcome, err := s.finishLogin(ctx, user, ttl, persistent, place)
	if err != nil {
		return User{}, Session{}, "", err
	}
	if !trust {
		return outcome.User, outcome.Session, "", nil
	}
	raw, err := s.storeTrust(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, "", err
	}
	return outcome.User, outcome.Session, raw, nil
}

// CompleteRecovery consumes one recovery code for a pending challenge.
func (s *Service) CompleteRecovery(ctx context.Context, challenge, code string) (User, Session, error) {
	userID, attempts, ttl, persistent, place, err := s.loadChallenge(ctx, challenge)
	if err != nil {
		return User{}, Session{}, err
	}
	user, _, email, err := s.userSecret(ctx, userID)
	if err != nil {
		return User{}, Session{}, err
	}
	ok, err := s.consumeRecovery(ctx, userID, code)
	if err != nil {
		return User{}, Session{}, err
	}
	if !ok {
		_ = s.recordAuthEvent(ctx, "totp_failed", "Rejected recovery code for "+email)
		return User{}, Session{}, s.failChallenge(ctx, challenge, email, attempts)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM totp_challenges WHERE token_hash = ?`, hashToken(challenge))
	_ = s.clearLoginFailures(ctx, email)
	_ = s.recordAuthEvent(ctx, "recovery_used", "Recovery code used for "+email)
	outcome, err := s.finishLogin(ctx, user, ttl, persistent, place)
	if err != nil {
		return User{}, Session{}, err
	}
	return outcome.User, outcome.Session, nil
}

func (s *Service) userSecret(ctx context.Context, userID string) (User, string, string, error) {
	var user User
	var created, secret, lastAt, lastPlace string
	var admin, totpOn int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, is_admin, created_at, totp_enabled, totp_secret, last_sign_in_at, last_sign_in_place
		FROM users WHERE id = ?
	`, userID).Scan(&user.ID, &user.Email, &admin, &created, &totpOn, &secret, &lastAt, &lastPlace)
	if err != nil {
		return User{}, "", "", err
	}
	user.IsAdmin = admin != 0
	user.TOTPEnabled = totpOn != 0
	user.CreatedAt, err = parseTime(created)
	if err != nil {
		return User{}, "", "", err
	}
	user.LastSignInAt = parseOptionalTime(lastAt)
	user.LastSignInPlace = lastPlace
	return user, secret, user.Email, nil
}

func (s *Service) trustValid(ctx context.Context, raw, userID string) bool {
	if strings.TrimSpace(raw) == "" || userID == "" {
		return false
	}
	var expires string
	err := s.db.QueryRowContext(ctx, `
		SELECT expires_at FROM trust_devices WHERE token_hash = ? AND user_id = ?
	`, hashToken(raw), userID).Scan(&expires)
	if err != nil {
		return false
	}
	exp, err := parseTime(expires)
	return err == nil && exp.After(s.now())
}

func (s *Service) storeTrust(ctx context.Context, userID string) (string, error) {
	raw, err := newToken()
	if err != nil {
		return "", err
	}
	now := s.now()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO trust_devices (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)
	`, hashToken(raw), userID, formatTime(now.Add(trustTTL)), formatTime(now))
	if err != nil {
		return "", err
	}
	return raw, nil
}

// BeginEnrollment stores a pending secret. It is not active until ConfirmEnrollment.
func (s *Service) BeginEnrollment(ctx context.Context, userID, email string) (secret, otpauth string, err error) {
	secret, err = newTOTPSecret()
	if err != nil {
		return "", "", err
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE users SET totp_pending = ? WHERE id = ?`, secret, userID); err != nil {
		return "", "", err
	}
	enrollFailures.Delete(userID)
	label := url.QueryEscape("ALTAS:" + email)
	issuer := url.QueryEscape("ALTAS")
	otpauth = fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30", label, secret, issuer)
	return secret, otpauth, nil
}

// ConfirmEnrollment enables TOTP after a valid code and returns recovery codes once.
func (s *Service) ConfirmEnrollment(ctx context.Context, userID, code string) ([]string, error) {
	var pending string
	if err := s.db.QueryRowContext(ctx, `SELECT totp_pending FROM users WHERE id = ?`, userID).Scan(&pending); err != nil {
		return nil, err
	}
	if strings.TrimSpace(pending) == "" || !verifyTOTP(pending, code, s.now()) {
		n := enrollBump(userID)
		if n >= enrollAttempts {
			_, _ = s.db.ExecContext(ctx, `UPDATE users SET totp_pending = '' WHERE id = ?`, userID)
			enrollFailures.Delete(userID)
			return nil, ValidationError{Message: "That code did not match. Start enrollment again."}
		}
		return nil, ErrInvalidCode
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET totp_secret = ?, totp_pending = '', totp_enabled = 1, updated_at = ? WHERE id = ?
	`, pending, formatTime(s.now()), userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	for _, hash := range hashes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (code_hash, user_id) VALUES (?, ?)`, hash, userID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	enrollFailures.Delete(userID)
	return codes, nil
}

// DisableTOTP turns authenticator sign-in off after the password and a current code.
func (s *Service) DisableTOTP(ctx context.Context, userID, password, code string) error {
	var hash, secret string
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT password_hash, totp_secret, totp_enabled FROM users WHERE id = ?`, userID).Scan(&hash, &secret, &enabled)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	if enabled == 0 {
		return nil
	}
	if !verifyTOTP(secret, code, s.now()) {
		used, err := s.consumeRecovery(ctx, userID, code)
		if err != nil {
			return err
		}
		if !used {
			return ErrInvalidCode
		}
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users SET totp_enabled = 0, totp_secret = '', totp_pending = '', updated_at = ? WHERE id = ?
	`, formatTime(s.now()), userID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM trust_devices WHERE user_id = ?`, userID)
	return err
}

func (s *Service) consumeRecovery(ctx context.Context, userID, code string) (bool, error) {
	hash := hashRecovery(code)
	if hash == "" {
		return false, nil
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE recovery_codes SET used_at = ? WHERE code_hash = ? AND user_id = ? AND used_at IS NULL
	`, formatTime(s.now()), hash, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RecoveryRemaining reports unused codes. It does not reveal the codes.
func (s *Service) RecoveryRemaining(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used_at IS NULL`, userID).Scan(&n)
	return n, err
}

func newTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

func verifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil || len(key) == 0 {
		return false
	}
	counter := uint64(now.Unix() / int64(totpPeriod.Seconds()))
	for delta := -totpSkew; delta <= totpSkew; delta++ {
		step := counter
		if delta < 0 {
			if counter < uint64(-delta) {
				continue
			}
			step = counter - uint64(-delta)
		} else {
			step = counter + uint64(delta)
		}
		got := hotp(key, step, totpDigits)
		if subtle.ConstantTimeCompare([]byte(got), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func hotp(key []byte, counter uint64, digits int) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, bin%mod)
}

func newRecoveryCodes() ([]string, []string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codes := make([]string, recoveryCount)
	hashes := make([]string, recoveryCount)
	buf := make([]byte, recoveryCount*8)
	if _, err := rand.Read(buf); err != nil {
		return nil, nil, err
	}
	for i := 0; i < recoveryCount; i++ {
		var b strings.Builder
		for j := 0; j < 8; j++ {
			if j == 4 {
				b.WriteByte('-')
			}
			b.WriteByte(alphabet[int(buf[i*8+j])%len(alphabet)])
		}
		codes[i] = b.String()
		hashes[i] = hashRecovery(codes[i])
	}
	return codes, hashes, nil
}

func hashRecovery(code string) string {
	clean := strings.ToUpper(strings.TrimSpace(code))
	clean = strings.ReplaceAll(clean, " ", "")
	if len(clean) == 8 {
		clean = clean[:4] + "-" + clean[4:]
	}
	if len(clean) != 9 || clean[4] != '-' {
		return ""
	}
	sum := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(sum[:])
}

// RecentAuthEvents returns the newest panel auth events. Details do not include client addresses.
func (s *Service) RecentAuthEvents(ctx context.Context, limit int) ([]AuthEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT at, kind, detail FROM auth_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuthEvent{}
	for rows.Next() {
		var ev AuthEvent
		if err := rows.Scan(&ev.At, &ev.Kind, &ev.Detail); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// AuthEvent is one sign-in audit row.
type AuthEvent struct {
	At     string
	Kind   string
	Detail string
}
