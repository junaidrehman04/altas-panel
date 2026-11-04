package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeriesAndAnalyticsRequireSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	for _, path := range []string{
		"/api/metrics/host/series?range=live&path=/etc/passwd",
		"/api/analytics/summary?range=24h&ip=203.0.113.55",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("%s status %d body %s", path, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		if strings.Contains(body, "/etc/passwd") || strings.Contains(body, "203.0.113.55") {
			t.Fatalf("%s reflected query: %s", path, body)
		}
	}
}

func TestSeriesAndAnalyticsRanges(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := signedInClient(t, srv.URL)

	res, data := getJSON(t, client, srv.URL+"/api/metrics/host/series?range=../etc/passwd")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad range status %d body %s", res.StatusCode, data)
	}
	if strings.Contains(string(data), "passwd") {
		t.Fatalf("bad range reflected input: %s", data)
	}

	res, data = getJSON(t, client, srv.URL+"/api/metrics/host/series?range=live")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("series status %d body %s", res.StatusCode, data)
	}
	var series struct {
		Range      string `json:"range"`
		Resolution string `json:"resolution"`
		Points     []any  `json:"points"`
	}
	if err := json.Unmarshal(data, &series); err != nil {
		t.Fatal(err)
	}
	if series.Range != "live" || series.Resolution != "15s" || series.Points == nil || len(series.Points) != 0 {
		t.Fatalf("series = %+v", series)
	}

	res, data = getJSON(t, client, srv.URL+"/api/analytics/summary?range=nope")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("analytics range status %d body %s", res.StatusCode, data)
	}

	res, data = getJSON(t, client, srv.URL+"/api/analytics/summary?range=24h&project=../secret")
	if res.StatusCode != http.StatusBadRequest || strings.Contains(string(data), "secret") {
		t.Fatalf("project filter status %d body %s", res.StatusCode, data)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/analytics/summary?range=month", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.55")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("summary status %d body %s", res.StatusCode, body)
	}
	if strings.Contains(string(body), "203.0.113.55") || strings.Contains(string(body), "203.0.113.0") {
		t.Fatalf("summary stored a client address: %s", body)
	}
	var summary struct {
		Range         string `json:"range"`
		Resolution    string `json:"resolution"`
		GeoConfigured bool   `json:"geo_configured"`
		Totals        struct {
			Requests int64    `json:"requests"`
			Visitors int64    `json:"visitors"`
			P95MS    *float64 `json:"p95_ms"`
		} `json:"totals"`
		Series    []any `json:"series"`
		TopHosts  []any `json:"top_hosts"`
		TopPaths  []any `json:"top_paths"`
		Countries []any `json:"countries"`
		Cities    []any `json:"cities"`
		Devices   []any `json:"devices"`
	}
	if err := json.Unmarshal(body, &summary); err != nil {
		t.Fatalf("decode: %v body %s", err, body)
	}
	if summary.Range != "month" || summary.Resolution != "1mo" || summary.GeoConfigured {
		t.Fatalf("summary meta = %+v", summary)
	}
	if summary.Totals.Requests < 1 || summary.Series == nil || summary.Countries == nil || len(summary.Countries) != 0 || len(summary.Cities) != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.TopHosts == nil || summary.TopPaths == nil || summary.Devices == nil {
		t.Fatalf("nil lists in %+v", summary)
	}
}
