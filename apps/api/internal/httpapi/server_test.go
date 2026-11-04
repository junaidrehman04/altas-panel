package httpapi_test

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/config"
	"github.com/junaidrehman04/altas/apps/api/internal/db"
	"github.com/junaidrehman04/altas/apps/api/internal/httpapi"
)

func newTestApp(t *testing.T, env string) (http.Handler, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	buf := &bytes.Buffer{}
	logger := log.New(buf, "", 0)
	cfg := config.Config{
		Env:          env,
		WebOrigin:    "http://127.0.0.1:5173",
		SessionTTL:   24 * time.Hour,
		ResetTTL:     time.Hour,
		ArgonMemory:  8 * 1024,
		ArgonTime:    1,
		ArgonThreads: 1,
	}
	return httpapi.New(cfg, sqlDB, logger), buf
}

func postJSON(t *testing.T, client *http.Client, url string, body any) (*http.Response, []byte) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return res, data
}

func getJSON(t *testing.T, client *http.Client, url string) (*http.Response, []byte) {
	t.Helper()
	res, err := client.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return res, data
}

type userBody struct {
	User struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		IsAdmin   bool   `json:"is_admin"`
		CreatedAt string `json:"created_at"`
	} `json:"user"`
}

type errBody struct {
	Error string `json:"error"`
}

func decodeUser(t *testing.T, data []byte) userBody {
	t.Helper()
	var body userBody
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode user: %v body=%s", err, data)
	}
	return body
}

func TestRegisterLoginMeAndLogout(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	res, data := postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "Ada@Altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register status %d body %s", res.StatusCode, data)
	}
	created := decodeUser(t, data)
	if created.User.Email != "ada@altas.dev" || created.User.ID == "" || created.User.CreatedAt == "" {
		t.Fatalf("user payload = %+v", created.User)
	}

	res, data = getJSON(t, client, srv.URL+"/api/me")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me status %d body %s", res.StatusCode, data)
	}
	me := decodeUser(t, data)
	if me.User.ID != created.User.ID || me.User.Email != "ada@altas.dev" {
		t.Fatalf("me = %+v", me.User)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	logoutRes, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, logoutRes.Body)
	_ = logoutRes.Body.Close()
	if logoutRes.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status %d", logoutRes.StatusCode)
	}

	res, data = getJSON(t, client, srv.URL+"/api/me")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after logout status %d body %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"email":    "ada@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status %d body %s", res.StatusCode, data)
	}
	logged := decodeUser(t, data)
	if logged.User.ID != created.User.ID {
		t.Fatalf("login user id %s want %s", logged.User.ID, created.User.ID)
	}

	res, data = getJSON(t, client, srv.URL+"/api/me")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me after login status %d body %s", res.StatusCode, data)
	}
}

func TestSessionCookieIsHttpOnlyAndLax(t *testing.T) {
	handler, _ := newTestApp(t, "development")

	body := strings.NewReader(`{"email":"ops@altas.dev","password":"correct-horse"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}

	var session *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "altas_session" {
			session = c
		}
	}
	if session == nil || session.Value == "" {
		t.Fatal("missing altas_session cookie")
	}
	if !session.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v", session.SameSite)
	}
	if session.Secure {
		t.Fatal("Secure must be unset on plain HTTP")
	}
	if session.Path != "/" {
		t.Fatalf("path = %s", session.Path)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"ops@altas.dev","password":"correct-horse"}`))
	req.Header.Set("Content-Type", "application/json")
	req.TLS = &tls.ConnectionState{}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status %d body %s", rr.Code, rr.Body.String())
	}
	session = nil
	for _, c := range rr.Result().Cookies() {
		if c.Name == "altas_session" {
			session = c
		}
	}
	if session == nil || !session.Secure {
		t.Fatal("Secure must be set when the request is HTTPS")
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	res, data := postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "revoke@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", res.StatusCode, data)
	}

	base, err := http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	parsed := base.URL
	cookies := jar.Cookies(parsed)
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	raw := cookies[0].Value

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	logoutRes, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, logoutRes.Body)
	_ = logoutRes.Body.Close()

	revoked := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	revoked.AddCookie(&http.Cookie{Name: "altas_session", Value: raw})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, revoked)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestForgotPasswordResetAndLogin(t *testing.T) {
	handler, logs := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := &http.Client{}

	res, data := postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "reset@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/forgot-password", map[string]string{
		"email": "reset@altas.dev",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("forgot %d %s", res.StatusCode, data)
	}

	re := regexp.MustCompile(`http://127\.0\.0\.1:5173/reset-password\?token=([A-Za-z0-9_-]+)`)
	match := re.FindStringSubmatch(logs.String())
	if len(match) != 2 {
		t.Fatalf("reset url not logged: %q", logs.String())
	}
	token := match[1]

	res, data = postJSON(t, client, srv.URL+"/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "new-pass-phrase",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("reset %d %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"email":    "reset@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password status %d body %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"email":    "reset@altas.dev",
		"password": "new-pass-phrase",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("new password status %d body %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "another-pass-phrase",
	})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("reused token status %d body %s", res.StatusCode, data)
	}
}

func TestForgotPasswordUnknownEmailStaysQuiet(t *testing.T) {
	handler, logs := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := &http.Client{}

	res, data := postJSON(t, client, srv.URL+"/api/auth/forgot-password", map[string]string{
		"email": "missing@altas.dev",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", res.StatusCode, data)
	}
	if strings.Contains(logs.String(), "reset-password?token=") {
		t.Fatalf("unknown email must not log a reset url: %s", logs.String())
	}
}

func TestProductionDoesNotLogResetURL(t *testing.T) {
	handler, logs := newTestApp(t, "production")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := &http.Client{}

	res, data := postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "prod@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", res.StatusCode, data)
	}
	res, data = postJSON(t, client, srv.URL+"/api/auth/forgot-password", map[string]string{
		"email": "prod@altas.dev",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("forgot %d %s", res.StatusCode, data)
	}
	if strings.Contains(logs.String(), "token=") {
		t.Fatalf("production log leaked reset url: %s", logs.String())
	}
}

func TestDuplicateEmailAndBadPassword(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := &http.Client{}

	res, data := postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "dup@altas.dev",
		"password": "short",
	})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("short password status %d body %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "dup@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", res.StatusCode, data)
	}
	res, data = postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "dup@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("closed registration status %d body %s", res.StatusCode, data)
	}
	var closed errBody
	if err := json.Unmarshal(data, &closed); err != nil {
		t.Fatal(err)
	}
	if closed.Error != registrationClosedError {
		t.Fatalf("closed registration error = %q", closed.Error)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"email":    "dup@altas.dev",
		"password": "wrong-password",
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad password status %d body %s", res.StatusCode, data)
	}
	var unknown, known errBody
	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"email":    "nobody@altas.dev",
		"password": "wrong-password",
	})
	if err := json.Unmarshal(data, &unknown); err != nil {
		t.Fatal(err)
	}
	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{
		"email":    "dup@altas.dev",
		"password": "wrong-password",
	})
	if err := json.Unmarshal(data, &known); err != nil {
		t.Fatal(err)
	}
	if unknown.Error == "" || unknown.Error != known.Error {
		t.Fatalf("login errors differ: unknown=%q known=%q", unknown.Error, known.Error)
	}
	_ = res
}

const registrationClosedError = "Public registration is closed. The first account is the panel admin; further accounts will be invite-only."

func TestFirstAccountIsAdminAndClosesPublicRegistration(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := &http.Client{}

	res, data := getJSON(t, client, srv.URL+"/api/auth/registration")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("registration status %d body %s", res.StatusCode, data)
	}
	if !registrationOpen(t, data) {
		t.Fatal("registration must be open before any account exists")
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "Founder@Altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register status %d body %s", res.StatusCode, data)
	}
	created := decodeUser(t, data)
	if !created.User.IsAdmin || created.User.Email != "founder@altas.dev" {
		t.Fatalf("first user = %+v", created.User)
	}

	res, data = getJSON(t, client, srv.URL+"/api/auth/registration")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("registration after first user %d body %s", res.StatusCode, data)
	}
	if registrationOpen(t, data) {
		t.Fatal("registration must close after the first account")
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "second@altas.dev",
		"password": "short",
	})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("short password while closed status %d body %s", res.StatusCode, data)
	}

	res, data = postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email":    "second@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("second register status %d body %s", res.StatusCode, data)
	}
	var closed errBody
	if err := json.Unmarshal(data, &closed); err != nil {
		t.Fatal(err)
	}
	if closed.Error != registrationClosedError {
		t.Fatalf("error = %q", closed.Error)
	}
	if setCookie(res, "altas_session") != "" {
		t.Fatal("closed registration must not set a session cookie")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	authed := &http.Client{Jar: jar}
	res, data = postJSON(t, authed, srv.URL+"/api/auth/login", map[string]string{
		"email":    "founder@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status %d body %s", res.StatusCode, data)
	}
	logged := decodeUser(t, data)
	if !logged.User.IsAdmin || logged.User.ID != created.User.ID {
		t.Fatalf("login user = %+v", logged.User)
	}
	res, data = getJSON(t, authed, srv.URL+"/api/me")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me status %d body %s", res.StatusCode, data)
	}
	me := decodeUser(t, data)
	if !me.User.IsAdmin || me.User.Email != "founder@altas.dev" {
		t.Fatalf("me = %+v", me.User)
	}
}

func TestConcurrentFirstRegistrationAllowsOneAdmin(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	const n = 8
	var wg sync.WaitGroup
	codes := make([]int, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			payload := fmt.Sprintf(`{"email":"racer-%d@altas.dev","password":"correct-horse"}`, i)
			res, err := http.Post(srv.URL+"/api/auth/register", "application/json", strings.NewReader(payload))
			if err != nil {
				errs[i] = err
				return
			}
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, res.Body)
			codes[i] = res.StatusCode
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	created := 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusForbidden:
		default:
			t.Fatalf("unexpected status %d in %v", code, codes)
		}
	}
	if created != 1 {
		t.Fatalf("created %d accounts, want 1; codes %v", created, codes)
	}
}

func TestLegacyDatabaseKeepsFirstUserAsAdmin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.db")
	seedLegacyUsers(t, path)

	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var firstAdmin, secondAdmin int
	if err := sqlDB.QueryRow(`SELECT is_admin FROM users WHERE email = ?`, "first@altas.dev").Scan(&firstAdmin); err != nil {
		t.Fatalf("first admin flag: %v", err)
	}
	if err := sqlDB.QueryRow(`SELECT is_admin FROM users WHERE email = ?`, "second@altas.dev").Scan(&secondAdmin); err != nil {
		t.Fatalf("second admin flag: %v", err)
	}
	if firstAdmin != 1 || secondAdmin != 0 {
		t.Fatalf("admin flags first=%d second=%d", firstAdmin, secondAdmin)
	}

	cfg := config.Config{
		Env:          "development",
		WebOrigin:    "http://127.0.0.1:5173",
		SessionTTL:   time.Hour,
		ResetTTL:     time.Hour,
		ArgonMemory:  8 * 1024,
		ArgonTime:    1,
		ArgonThreads: 1,
	}
	handler := httpapi.New(cfg, sqlDB, log.New(io.Discard, "", 0))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	res, data := postJSON(t, &http.Client{}, srv.URL+"/api/auth/register", map[string]string{
		"email":    "third@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("legacy register status %d body %s", res.StatusCode, data)
	}
}

func registrationOpen(t *testing.T, data []byte) bool {
	t.Helper()
	var body struct {
		Open bool `json:"open"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode registration: %v body=%s", err, data)
	}
	return body.Open
}

func setCookie(res *http.Response, name string) string {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func sqlOpenLegacy(path string) (*sql.DB, error) {
	return sql.Open("sqlite", "file:"+filepath.ToSlash(path))
}

func seedLegacyUsers(t *testing.T, path string) {
	t.Helper()
	sqlDB, err := sqlOpenLegacy(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	_, err = sqlDB.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES
			('first', 'first@altas.dev', 'hash', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
			('second', 'second@altas.dev', 'hash', '2026-02-01T00:00:00Z', '2026-02-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMeRequiresSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}
