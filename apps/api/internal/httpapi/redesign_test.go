package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPanelEndpointsRequireSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	for _, path := range []string{
		"/api/panel/bootstrap",
		"/api/panel/edge?range=24h",
		"/api/panel/apm?range=1h",
		"/api/panel/hardware?range=7d",
		"/api/alerts",
		"/api/me/totp/setup",
	} {
		method := http.MethodGet
		var body *strings.Reader
		if strings.Contains(path, "totp") {
			method = http.MethodPost
			body = strings.NewReader(`{}`)
		} else {
			body = strings.NewReader("")
		}
		req := httptest.NewRequest(method, path, body)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("%s status %d body %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestEdgeRejectsBadProjectWithoutEcho(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := signedInClient(t, srv.URL)
	res, data := getJSON(t, client, srv.URL+"/api/panel/apm?range=24h&project=../secret")
	if res.StatusCode != http.StatusBadRequest || strings.Contains(string(data), "secret") {
		t.Fatalf("status %d body %s", res.StatusCode, data)
	}
}

func TestRememberExtendsSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	res, data := postJSON(t, client, srv.URL+"/api/auth/register", map[string]string{
		"email": "ada@altas.dev", "password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", res.StatusCode, data)
	}
	_, _ = postJSON(t, client, srv.URL+"/api/auth/logout", map[string]string{})
	res, data = postJSON(t, client, srv.URL+"/api/auth/login", map[string]any{
		"email": "ada@altas.dev", "password": "correct-horse", "remember": true,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login %d %s", res.StatusCode, data)
	}
	var maxAge int
	for _, cookie := range res.Cookies() {
		if cookie.Name == "altas_session" {
			maxAge = cookie.MaxAge
		}
	}
	if maxAge < int((13 * 24 * time.Hour).Seconds()) {
		t.Fatalf("remember max-age = %d", maxAge)
	}
}

func TestHardwarePayloadHasNoDemoHost(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := signedInClient(t, srv.URL)
	res, data := getJSON(t, client, srv.URL+"/api/panel/hardware?range=1h")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", res.StatusCode, data)
	}
	if strings.Contains(string(data), "northfield") || strings.Contains(string(data), "m.richter") || strings.Contains(string(data), "123456") {
		t.Fatalf("demo identity leaked: %s", data)
	}
	var body struct {
		Identity struct {
			ID string `json:"id"`
		} `json:"identity"`
		IPMI struct {
			Available bool `json:"available"`
		} `json:"ipmi"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if body.Identity.ID == "" || body.IPMI.Available {
		t.Fatalf("identity %+v", body)
	}
}
