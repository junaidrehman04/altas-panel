package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadHostAndDockerDefaults(t *testing.T) {
	t.Setenv("ALTAS_ADDR", ":9090")
	t.Setenv("ALTAS_HOST_PROC", "/host/proc")
	t.Setenv("ALTAS_HOST_ROOT", "/host/root")
	t.Setenv("ALTAS_PROJECTS_ROOT", "/var/lib/altas/projects")
	t.Setenv("ALTAS_DOCKER_SOCKET", "")
	t.Setenv("ALTAS_DOCKER_PROBE", "yes")

	cfg := Load()
	if cfg.Addr != ":9090" || cfg.HostProc != "/host/proc" || cfg.HostRoot != "/host/root" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if !cfg.DockerProbe || cfg.DockerSocket != "" || cfg.ProjectsRoot != "/var/lib/altas/projects" {
		t.Fatalf("cfg = %+v", cfg)
	}

	t.Setenv("ALTAS_DOCKER_PROBE", "0")
	if Load().DockerProbe {
		t.Fatal("docker probe defaulted on")
	}
	t.Setenv("ALTAS_DOCKER_PROBE", "maybe")
	if Load().DockerProbe {
		t.Fatal("unknown probe value enabled the probe")
	}
}

func TestUpdateConfigIsPinnedAndRateLimited(t *testing.T) {
	t.Setenv("ALTAS_UPDATE_OWNER", "")
	t.Setenv("ALTAS_UPDATE_REPO", "")
	t.Setenv("ALTAS_UPDATE_INTERVAL", "")
	t.Setenv("ALTAS_UPDATE_UNIT", "")
	t.Setenv("ALTAS_UPDATE_RESTART", "")
	cfg := Load()
	if cfg.UpdateOwner != "junaidrehman04" || cfg.UpdateRepo != "altas-panel" {
		t.Fatalf("repo = %s/%s", cfg.UpdateOwner, cfg.UpdateRepo)
	}
	if cfg.UpdateAPIBase != "https://api.github.com" {
		t.Fatalf("api base = %s", cfg.UpdateAPIBase)
	}
	if cfg.UpdateInterval != 12*time.Hour || cfg.UpdateUnit != "altas-panel" || cfg.UpdateRestart != "systemd" {
		t.Fatalf("cfg = %+v", cfg)
	}

	t.Setenv("ALTAS_UPDATE_INTERVAL", "1m")
	if Load().UpdateInterval != 6*time.Hour {
		t.Fatalf("short interval = %s", Load().UpdateInterval)
	}
	t.Setenv("ALTAS_UPDATE_INTERVAL", "48h")
	if Load().UpdateInterval != 24*time.Hour {
		t.Fatalf("long interval = %s", Load().UpdateInterval)
	}
	t.Setenv("ALTAS_UPDATE_OWNER", "../etc")
	t.Setenv("ALTAS_UPDATE_REPO", "altas-panel/../../secret")
	cfg = Load()
	if cfg.UpdateOwner != "junaidrehman04" || cfg.UpdateRepo != "altas-panel" {
		t.Fatalf("escaped repo = %s/%s", cfg.UpdateOwner, cfg.UpdateRepo)
	}
	t.Setenv("ALTAS_ENV", "development")
	t.Setenv("ALTAS_UPDATE_API_BASE", "https://evil.example")
	if Load().UpdateAPIBase != "https://api.github.com" {
		t.Fatal("environment overrode the release API")
	}
	t.Setenv("ALTAS_ENV", "production")
	t.Setenv("ALTAS_UPDATE_API_BASE", "http://127.0.0.1:9")
	if Load().UpdateAPIBase != "https://api.github.com" {
		t.Fatal("production accepted a loopback release API")
	}
	t.Setenv("ALTAS_ENV", "development")
	if Load().UpdateAPIBase != "http://127.0.0.1:9" {
		t.Fatalf("development loopback base = %s", Load().UpdateAPIBase)
	}
	t.Setenv("ALTAS_UPDATE_API_BASE", "http://127.0.0.1:9/repos")
	if Load().UpdateAPIBase != "https://api.github.com" {
		t.Fatal("path on the release API was accepted")
	}
}

func TestLoadAnalyticsDefaults(t *testing.T) {
	t.Setenv("ALTAS_GEOIP_DB", "")
	t.Setenv("MAXMIND_LICENSE_KEY", "")
	t.Setenv("ALTAS_ACCESS_LOG", "")
	t.Setenv("ALTAS_METRICS_INTERVAL", "")
	cfg := Load()
	if cfg.GeoIPDB != "" || cfg.MaxMindLicenseKey != "" || cfg.AccessLog != "" || cfg.MetricsEvery != 15*time.Second {
		t.Fatal("analytics defaults were not empty")
	}
	if cfg.GeoDBPath() != "" {
		t.Fatal("geo path without a key")
	}
	t.Setenv("ALTAS_METRICS_INTERVAL", "1s")
	if Load().MetricsEvery != 15*time.Second {
		t.Fatal("short interval was accepted")
	}
	t.Setenv("ALTAS_METRICS_INTERVAL", "30s")
	if Load().MetricsEvery != 30*time.Second {
		t.Fatal("30s interval was rejected")
	}
}

func TestGeoDBPathUsesTheDataDirWhenAKeyIsSet(t *testing.T) {
	t.Setenv("ALTAS_DB", filepath.Join("data", "altas.db"))
	t.Setenv("ALTAS_GEOIP_DB", "")
	t.Setenv("MAXMIND_LICENSE_KEY", "test-key")
	path := Load().GeoDBPath()
	if !filepath.IsAbs(path) || filepath.Base(path) != "GeoLite2-Country.mmdb" {
		t.Fatalf("default geo path = %s", path)
	}
	if !strings.HasSuffix(filepath.Dir(path), "data") {
		t.Fatalf("default geo dir = %s", path)
	}
	t.Setenv("ALTAS_GEOIP_DB", "/var/lib/altas/custom.mmdb")
	if Load().GeoDBPath() != "/var/lib/altas/custom.mmdb" {
		t.Fatal("explicit geo path was replaced")
	}
	t.Setenv("ALTAS_GEOIP_DB", "")
	t.Setenv("MAXMIND_LICENSE_KEY", "bad\nkey")
	if Load().MaxMindLicenseKey != "" || Load().GeoDBPath() != "" {
		t.Fatal("newline in the license key was accepted")
	}
}
