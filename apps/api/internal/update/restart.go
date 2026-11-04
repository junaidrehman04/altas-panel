package update

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrRestart means the panel process was not restarted.
var ErrRestart = errors.New("restart")

// Restarter restarts the installed panel after a binary replace.
type Restarter interface {
	Restart(ctx context.Context) error
}

type noopRestart struct{}

func (noopRestart) Restart(context.Context) error { return nil }

// Systemd restarts one unit through the systemctl binary.
// The unit name is a single token. The command is not a shell.
type Systemd struct {
	Unit     string
	lookPath func(string) (string, error)
	run      func(context.Context, string, ...string) error
}

// Restart runs `systemctl restart <unit>` with an absolute systemctl path.
func (s Systemd) Restart(ctx context.Context) error {
	unit := s.Unit
	if unit == "" {
		unit = "altas-panel"
	}
	if !ValidUnit(unit) {
		return ErrRestart
	}
	look := s.lookPath
	if look == nil {
		look = exec.LookPath
	}
	bin, err := look("systemctl")
	if err != nil || !filepath.IsAbs(bin) {
		return ErrRestart
	}
	if s.run != nil {
		return s.run(ctx, bin, "restart", unit)
	}
	cmd := exec.CommandContext(ctx, bin, "restart", unit)
	cmd.Stdin = nil
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return ErrRestart
	}
	return nil
}

// ValidUnit accepts a systemd unit name and rejects shell metacharacters.
func ValidUnit(unit string) bool {
	if unit == "" || len(unit) > 80 || strings.Contains(unit, "..") {
		return false
	}
	for i, r := range unit {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == ':' || r == '@':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
