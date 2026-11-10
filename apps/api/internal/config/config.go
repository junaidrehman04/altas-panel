package config

import (
	"os"
	"strings"
	"time"
)

// Config is the process configuration for the Altas API.
// Defaults target a local development run: HTTP on :8080, SQLite under
// data/, and password-reset links printed to stdout.
// Host proc paths and the Docker socket come from the operator, never from
// a request. The Docker probe is off unless ALTAS_DOCKER_PROBE is enabled.
type Config struct {
	Addr           string
	DBPath         string
	Env            string
	WebOrigin      string
	SessionTTL     time.Duration
	ResetTTL       time.Duration
	ArgonMemory    uint32
	ArgonTime      uint32
	ArgonThreads   uint8
	HostProc       string
	HostRoot       string
	DockerProbe    bool
	DockerSocket   string
	ProjectsRoot   string
	UpdateOwner    string
	UpdateRepo     string
	UpdateInterval time.Duration
	UpdateUnit     string
	UpdateRestart  string
	UpdateBinary   string
	UpdatePubKey   string
	UpdateAPIBase  string
	UpdateOS       string
	UpdateArch     string
	MetricsEvery   time.Duration
	GeoIPDB        string
	AccessLog      string
	Org            string
}

func Load() Config {
	return Config{
		Addr:           getenv("ALTAS_ADDR", ":8080"),
		DBPath:         getenv("ALTAS_DB", "data/altas.db"),
		Env:            getenv("ALTAS_ENV", "development"),
		WebOrigin:      strings.TrimRight(getenv("ALTAS_WEB_ORIGIN", "http://127.0.0.1:5173"), "/"),
		SessionTTL:     durationEnv("ALTAS_SESSION_TTL", 7*24*time.Hour),
		ResetTTL:       durationEnv("ALTAS_RESET_TTL", time.Hour),
		ArgonMemory:    19 * 1024,
		ArgonTime:      2,
		ArgonThreads:   1,
		HostProc:       getenv("ALTAS_HOST_PROC", "/proc"),
		HostRoot:       getenv("ALTAS_HOST_ROOT", "/"),
		DockerProbe:    boolEnv("ALTAS_DOCKER_PROBE", false),
		DockerSocket:   strings.TrimSpace(os.Getenv("ALTAS_DOCKER_SOCKET")),
		ProjectsRoot:   getenv("ALTAS_PROJECTS_ROOT", "/var/lib/altas/projects"),
		UpdateOwner:    updateNameEnv("ALTAS_UPDATE_OWNER", "junaidrehman04"),
		UpdateRepo:     updateNameEnv("ALTAS_UPDATE_REPO", "altas-panel"),
		UpdateInterval: updateInterval(),
		UpdateUnit:     updateUnit(),
		UpdateRestart:  updateRestart(),
		UpdateBinary:   strings.TrimSpace(os.Getenv("ALTAS_UPDATE_BINARY")),
		UpdatePubKey:   strings.TrimSpace(os.Getenv("ALTAS_UPDATE_PUBKEY")),
		// The release API is pinned. ALTAS_UPDATE_API_BASE is ignored on purpose.
		UpdateAPIBase: "https://api.github.com",
		MetricsEvery:  metricsInterval(),
		GeoIPDB:       strings.TrimSpace(os.Getenv("ALTAS_GEOIP_DB")),
		AccessLog:     strings.TrimSpace(os.Getenv("ALTAS_ACCESS_LOG")),
		Org:           orgEnv(),
	}
}

func orgEnv() string {
	value := strings.TrimSpace(os.Getenv("ALTAS_ORG"))
	if value == "" || len(value) > 80 || strings.ContainsAny(value, "\r\n\x00") {
		return ""
	}
	return value
}

func metricsInterval() time.Duration {
	every := durationEnv("ALTAS_METRICS_INTERVAL", 15*time.Second)
	if every < 5*time.Second || every > 5*time.Minute {
		return 15 * time.Second
	}
	return every
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func updateNameEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" || !validUpdateName(value) {
		return fallback
	}
	return value
}

func validUpdateName(name string) bool {
	if len(name) < 1 || len(name) > 39 || strings.Contains(name, "..") {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
			if i == 0 || i == len(name)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func updateInterval() time.Duration {
	interval := durationEnv("ALTAS_UPDATE_INTERVAL", 12*time.Hour)
	if interval < 6*time.Hour {
		return 6 * time.Hour
	}
	if interval > 24*time.Hour {
		return 24 * time.Hour
	}
	return interval
}

func updateUnit() string {
	unit := strings.TrimSpace(os.Getenv("ALTAS_UPDATE_UNIT"))
	if unit == "" || len(unit) > 80 || strings.Contains(unit, "..") {
		return "altas-panel"
	}
	for i, r := range unit {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == ':' || r == '@':
			if i == 0 {
				return "altas-panel"
			}
		default:
			return "altas-panel"
		}
	}
	return unit
}

func updateRestart() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ALTAS_UPDATE_RESTART"))) {
	case "", "systemd":
		return "systemd"
	case "none":
		return "none"
	default:
		return "none"
	}
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
