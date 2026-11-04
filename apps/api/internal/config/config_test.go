package config

import "testing"

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
