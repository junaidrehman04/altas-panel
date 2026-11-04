package config

import (
	"os"
	"strings"
	"time"
)

// Config is the process configuration for the Altas API.
// Defaults target a local development run: HTTP on :8080, SQLite under
// data/, and password-reset links printed to stdout.
type Config struct {
	Addr         string
	DBPath       string
	Env          string
	WebOrigin    string
	SessionTTL   time.Duration
	ResetTTL     time.Duration
	ArgonMemory  uint32
	ArgonTime    uint32
	ArgonThreads uint8
}

func Load() Config {
	return Config{
		Addr:         getenv("ALTAS_ADDR", ":8080"),
		DBPath:       getenv("ALTAS_DB", "data/altas.db"),
		Env:          getenv("ALTAS_ENV", "development"),
		WebOrigin:    strings.TrimRight(getenv("ALTAS_WEB_ORIGIN", "http://127.0.0.1:5173"), "/"),
		SessionTTL:   durationEnv("ALTAS_SESSION_TTL", 7*24*time.Hour),
		ResetTTL:     durationEnv("ALTAS_RESET_TTL", time.Hour),
		ArgonMemory:  19 * 1024,
		ArgonTime:    2,
		ArgonThreads: 1,
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
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
