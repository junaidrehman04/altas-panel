package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForwardedClientUsesTheProxyHop(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.4, 10.0.0.8")
	if got := forwardedClient(req); got != "10.0.0.8" {
		t.Fatalf("forwarded = %s", got)
	}
}

func TestPublicPeerIgnoresForwardedFor(t *testing.T) {
	var got string
	handler := trustForwardedFromProxy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.RemoteAddr
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.2")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if got != "203.0.113.9:1234" {
		t.Fatalf("remote = %s", got)
	}
}

func TestLoopbackTrustsForwardedFor(t *testing.T) {
	var got string
	handler := trustForwardedFromProxy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.RemoteAddr
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.2, 10.1.1.1")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if got != "10.1.1.1:1234" {
		t.Fatalf("remote = %s", got)
	}
}
