package auth

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/db"
)

func TestTOTPMatchesRFC6238SHA1(t *testing.T) {
	key, err := hex.DecodeString("3132333435363738393031323334353637383930")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		at   int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
	}
	for _, tc := range cases {
		got := hotp(key, uint64(tc.at/30), 6)
		if got != tc.code {
			t.Fatalf("at %d code %s want %s", tc.at, got, tc.code)
		}
	}
}

func TestLoginLockoutAndTOTP(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "altas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc, err := NewService(sqlDB, Params{Memory: 8 * 1024, Time: 1, Threads: 1}, time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if _, _, err := svc.Register(ctx, "ada@altas.dev", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		_, err := svc.Login(ctx, "ada@altas.dev", "wrong-password", LoginOptions{})
		var cred CredentialError
		if !asCred(err, &cred) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	_, err = svc.Login(ctx, "ada@altas.dev", "wrong-password", LoginOptions{})
	var locked LockedError
	if !asLocked(err, &locked) || locked.RetryAfter < 14*time.Minute {
		t.Fatalf("lockout = %v", err)
	}
	now = now.Add(16 * time.Minute)
	userID := userID(t, svc, ctx)
	secret, uri, err := svc.BeginEnrollment(ctx, userID, "ada@altas.dev")
	if err != nil || secret == "" || uri == "" {
		t.Fatal(err)
	}
	code := codeAt(t, secret, now)
	codes, err := svc.ConfirmEnrollment(ctx, userID, code)
	if err != nil || len(codes) != 10 {
		t.Fatalf("confirm: %v codes %d", err, len(codes))
	}
	if _, err := svc.ConfirmEnrollment(ctx, userID, "000000"); err == nil {
		t.Fatal("second confirm should fail closed")
	}

	outcome, err := svc.Login(ctx, "ada@altas.dev", "correct-horse", LoginOptions{Remember: boolPtr(true)})
	if err != nil || !outcome.TOTPRequired || outcome.Challenge == "" || outcome.Session.Raw != "" {
		t.Fatalf("totp challenge = %+v err %v", outcome, err)
	}
	if _, _, _, err := svc.CompleteTOTP(ctx, outcome.Challenge, "000000", false); err == nil {
		t.Fatal("accepted a wrong code")
	}
	user, sess, trust, err := svc.CompleteTOTP(ctx, outcome.Challenge, codeAt(t, secret, now), true)
	if err != nil || !user.TOTPEnabled || sess.Raw == "" || !sess.Persistent || trust == "" {
		t.Fatalf("complete totp user=%+v sess=%+v trust=%q err=%v", user, sess, trust, err)
	}
	if !sess.ExpiresAt.Equal(now.Add(14*24*time.Hour).Truncate(time.Second)) && sess.ExpiresAt.Sub(now) < 13*24*time.Hour {
		t.Fatalf("remember ttl = %s", sess.ExpiresAt.Sub(now))
	}

	short, err := svc.Login(ctx, "ada@altas.dev", "correct-horse", LoginOptions{Remember: boolPtr(false), TrustToken: trust})
	if err != nil || short.TOTPRequired || short.Session.Persistent {
		t.Fatalf("trusted login = %+v err %v", short, err)
	}
	if short.Session.ExpiresAt.Sub(now) > 13*time.Hour {
		t.Fatalf("session cookie ttl = %s", short.Session.ExpiresAt.Sub(now))
	}
}

func asCred(err error, dest *CredentialError) bool {
	got, ok := err.(CredentialError)
	if ok && dest != nil {
		*dest = got
	}
	return ok
}

func asLocked(err error, dest *LockedError) bool {
	got, ok := err.(LockedError)
	if ok && dest != nil {
		*dest = got
	}
	return ok
}

func userID(t *testing.T, svc *Service, ctx context.Context) string {
	t.Helper()
	var id string
	if err := svc.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, "ada@altas.dev").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func codeAt(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	code := hotp(key, uint64(now.Unix()/30), 6)
	if !verifyTOTP(secret, code, now) {
		t.Fatal("generated code did not verify")
	}
	return code
}

func boolPtr(v bool) *bool { return &v }
