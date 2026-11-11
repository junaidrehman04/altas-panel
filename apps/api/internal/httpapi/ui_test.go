package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedUIAndUnknownAPI(t *testing.T) {
	app, _ := newTestApp(t, "development")
	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("ui status=%d type=%s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if strings.Contains(string(body), "root:") {
		t.Fatal("ui leaked a host file")
	}

	res, err = http.Get(srv.URL + "/api/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("api status=%d body=%s", res.StatusCode, body)
	}
	if strings.Contains(string(body), "<html") {
		t.Fatal("unknown API returned the UI")
	}
}
