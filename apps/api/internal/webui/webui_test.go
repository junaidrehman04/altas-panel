package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeIndexAndRejectsEscape(t *testing.T) {
	index := ReadIndex()
	if !bytesContains(index, "ALTAS") && !bytesContains(index, "html") {
		t.Fatal("embedded index missing")
	}

	ok := httptest.NewRecorder()
	Serve(ok, httptest.NewRequest(http.MethodGet, "/", nil))
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "<") {
		t.Fatalf("index status=%d body=%q", ok.Code, ok.Body.String())
	}
	if ok.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff missing")
	}

	for _, p := range []string{"/../etc/passwd", "/..%2fetc/passwd", "/assets/../../etc/passwd", "/foo\\bar"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = p
		Serve(rec, req)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "root:") {
			t.Fatalf("path %s leaked", p)
		}
		if strings.Contains(rec.Body.String(), "root:") {
			t.Fatalf("path %s body leaked", p)
		}
	}

	missing := httptest.NewRecorder()
	Serve(missing, httptest.NewRequest(http.MethodGet, "/assets/missing-file.js", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing asset status=%d", missing.Code)
	}

	spa := httptest.NewRecorder()
	Serve(spa, httptest.NewRequest(http.MethodGet, "/edge", nil))
	if spa.Code != http.StatusOK || !strings.Contains(spa.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("spa status=%d type=%s", spa.Code, spa.Header().Get("Content-Type"))
	}
}

func bytesContains(b []byte, needle string) bool {
	return strings.Contains(string(b), needle)
}
