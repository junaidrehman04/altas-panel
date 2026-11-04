package config

import (
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
	t.Setenv("ALTAS_UPDATE_API_BASE", "https://evil.example")
	if Load().UpdateAPIBase != "https://api.github.com" {
		t.Fatal("environment overrode the release API")
	}
}
