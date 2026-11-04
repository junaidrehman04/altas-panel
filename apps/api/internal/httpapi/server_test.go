package httpapi_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
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
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status %d body %s", res.StatusCode, data)
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

func TestMeRequiresSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}
